package containerengine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strconv"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/jasondeutsch/watts/internal/workload"
)

type Engine struct{ client *client.Client }

func Connect() (*Engine, error) {
	c, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &Engine{client: c}, nil
}
func (engine *Engine) Close() error                    { return engine.client.Close() }
func (engine *Engine) Check(ctx context.Context) error { _, err := engine.client.Ping(ctx); return err }
func (engine *Engine) Pin(ctx context.Context, ref string) (string, error) {
	data, _, err := engine.client.ImageInspectWithRaw(ctx, ref)
	if err != nil {
		return "", fmt.Errorf("image %s unavailable: %w; build or pull it explicitly on the worker engine", ref, err)
	}
	return data.ID, nil
}

// Platform describes the workload image, not the machine hosting Docker.
func (engine *Engine) Platform(ctx context.Context, ref string) (string, string, error) {
	image, _, err := engine.client.ImageInspectWithRaw(ctx, ref)
	if err != nil {
		return "", "", err
	}
	return image.Os, image.Architecture, nil
}

type Request struct {
	Identity                                      string
	Spec                                          workload.Spec
	InputDir, OutputDir, RuntimeDir, WorkspaceDir string
	Environment                                   []string
	Stdout, Stderr                                io.Writer
}
type Exit struct {
	ContainerID string `json:"container_id"`
	ImageID     string `json:"image_id"`
	Code        *int64 `json:"exit_code,omitempty"`
	Error       string `json:"error,omitempty"`
	FinishedAt  string `json:"finished_at"`
}

func Name(identity string) string {
	sum := sha256.Sum256([]byte(identity))
	return "watts-" + hex.EncodeToString(sum[:])[:32]
}

// Run reattaches by execution identity. A stopped container is evidence, never
// restarted implicitly. Only this attempt's isolated directories are mounted.
func (engine *Engine) Run(ctx context.Context, request Request) (result Exit, runErr error) {
	defer func() {
		if runErr != nil {
			result.Error = runErr.Error()
		}
	}()
	if request.Identity == "" {
		return result, fmt.Errorf("container needs an execution identity")
	}
	if request.Stdout == nil {
		request.Stdout = io.Discard
	}
	if request.Stderr == nil {
		request.Stderr = io.Discard
	}
	defer func() {
		if runErr != nil && result.ContainerID != "" {
			cleanup, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			seconds := 3
			if err := engine.client.ContainerStop(cleanup, result.ContainerID, container.StopOptions{Timeout: &seconds}); err != nil {
				runErr = errors.Join(runErr, err, engine.client.ContainerKill(cleanup, result.ContainerID, "SIGKILL"))
			}
		}
	}()
	spec := request.Spec
	if err := spec.Validate(); err != nil {
		return result, err
	}
	identity := Name(request.Identity)
	descriptor, _ := json.Marshal(struct {
		Spec                              workload.Spec
		Input, Output, Runtime, Workspace string
	}{spec, request.InputDir, request.OutputDir, request.RuntimeDir, request.WorkspaceDir})
	digest := sha256.Sum256(descriptor)
	fingerprint := hex.EncodeToString(digest[:])
	existing, err := engine.client.ContainerInspect(ctx, identity)
	if err != nil && !errdefs.IsNotFound(err) {
		return result, err
	}
	if err == nil && (existing.Config.Labels["watts.execution"] != request.Identity || existing.Config.Labels["watts.contract"] != fingerprint) {
		return result, fmt.Errorf("container identity conflicts with another execution")
	}
	if errdefs.IsNotFound(err) {
		if os.Getuid() == 0 && spec.User == "" {
			for _, directory := range []string{request.InputDir, request.OutputDir, request.RuntimeDir, request.WorkspaceDir} {
				if err = prepareNonrootMount(directory); err != nil {
					return result, err
				}
			}
		}
		user := spec.User
		if user == "" {
			user = strconv.Itoa(os.Getuid()) + ":" + strconv.Itoa(os.Getgid())
			if os.Getuid() <= 0 {
				user = "1000:1000"
			}
		}
		network := spec.Network
		if network == "" {
			network = "none"
		}
		directory := spec.WorkingDir
		if directory == "" {
			directory = "/workspace"
		}
		configuration := &container.Config{Image: spec.Image, Entrypoint: spec.Command, Cmd: []string{}, WorkingDir: directory, User: user, Env: request.Environment, Labels: map[string]string{"watts.execution": request.Identity, "watts.contract": fingerprint}}
		host := &container.HostConfig{ReadonlyRootfs: true, NetworkMode: container.NetworkMode(network), CapDrop: []string{"ALL"}, SecurityOpt: []string{"no-new-privileges"}, Tmpfs: map[string]string{"/tmp": "rw,noexec,nosuid,size=67108864"}, Resources: container.Resources{Memory: spec.MemoryMB * 1024 * 1024, NanoCPUs: int64(spec.CPUs * 1e9)}, Mounts: []mount.Mount{
			{Type: mount.TypeBind, Source: request.InputDir, Target: "/inputs", ReadOnly: true},
			{Type: mount.TypeBind, Source: request.OutputDir, Target: "/outputs"},
			{Type: mount.TypeBind, Source: request.RuntimeDir, Target: "/runtime"},
			{Type: mount.TypeBind, Source: request.WorkspaceDir, Target: "/workspace", ReadOnly: spec.Workspace != "write"},
		}}
		created, createErr := engine.client.ContainerCreate(ctx, configuration, host, nil, nil, identity)
		if createErr != nil {
			return result, createErr
		}
		result.ContainerID = created.ID
		if err = engine.client.ContainerStart(ctx, created.ID, container.StartOptions{}); err != nil {
			return result, err
		}
		existing, err = engine.client.ContainerInspect(ctx, created.ID)
		if err != nil {
			return result, err
		}
	} else if existing.State.Status == "created" {
		result.ContainerID = existing.ID
		if err = engine.client.ContainerStart(ctx, existing.ID, container.StartOptions{}); err != nil {
			return result, err
		}
	}
	result.ContainerID = existing.ID
	result.ImageID = existing.Image
	logs, err := engine.client.ContainerLogs(ctx, existing.ID, container.LogsOptions{ShowStdout: true, ShowStderr: true, Follow: true})
	if err != nil {
		return result, err
	}
	defer logs.Close()
	logDone := make(chan error, 1)
	go func() { _, err := stdcopy.StdCopy(request.Stdout, request.Stderr, logs); logDone <- err }()
	status, failures := engine.client.ContainerWait(ctx, existing.ID, container.WaitConditionNotRunning)
	select {
	case <-ctx.Done():
		logs.Close()
		<-logDone
		return result, ctx.Err()
	case err := <-failures:
		if err != nil {
			logs.Close()
			<-logDone
			return result, err
		}
	case exit := <-status:
		result.Code = &exit.StatusCode
		if exit.Error != nil {
			logs.Close()
			<-logDone
			return result, fmt.Errorf("container wait: %s", exit.Error.Message)
		}
	}
	if err := <-logDone; err != nil && !errors.Is(err, context.Canceled) {
		return result, err
	}
	inspected, err := engine.client.ContainerInspect(ctx, existing.ID)
	if err != nil {
		return result, err
	}
	result.FinishedAt = inspected.State.FinishedAt
	return result, nil
}

// A root worker still launches a nonroot workload. Ownership changes are
// confined to attempt mounts; Lchown never follows a workload-created symlink.
func prepareNonrootMount(directory string) error {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return err
	}
	defer root.Close()
	return fs.WalkDir(root.FS(), ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return root.Lchown(name, 1000, 1000)
	})
}
func (engine *Engine) Remove(ctx context.Context, id string) error {
	err := engine.client.ContainerRemove(ctx, id, container.RemoveOptions{})
	if errdefs.IsNotFound(err) {
		return nil
	}
	return err
}
