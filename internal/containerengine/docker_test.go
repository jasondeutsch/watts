package containerengine

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jasondeutsch/watts/internal/workload"
	"github.com/stretchr/testify/require"
)

// Uses an ordinary image with no Watts adapter or language-model dependency.
func TestDockerExecutionAndReattachment(t *testing.T) {
	if os.Getenv("WATTS_DOCKER_INTEGRATION") != "1" {
		t.Skip("set WATTS_DOCKER_INTEGRATION=1 with ubuntu:latest available locally")
	}
	engine, err := Connect()
	require.NoError(t, err)
	t.Cleanup(func() { _ = engine.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	image, err := engine.Pin(ctx, "ubuntu:latest")
	require.NoError(t, err)
	root := t.TempDir()
	var output bytes.Buffer
	request := Request{Identity: root, Spec: workload.Spec{Image: image, Result: "exit-code", Command: []string{"sh", "-c", "printf x >> /outputs/count; echo running; test ! -w /workspace"}}, Stdout: &output, Stderr: &output}
	for name, target := range map[string]*string{"inputs": &request.InputDir, "outputs": &request.OutputDir, "runtime": &request.RuntimeDir, "workspace": &request.WorkspaceDir} {
		*target = filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(*target, 0755))
	}
	exit, err := engine.Run(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, exit.Code)
	require.Zero(t, *exit.Code)
	t.Cleanup(func() { _ = engine.Remove(context.Background(), exit.ContainerID) })
	again, err := engine.Run(ctx, request)
	require.NoError(t, err)
	require.Equal(t, exit.ContainerID, again.ContainerID)
	data, err := os.ReadFile(filepath.Join(request.OutputDir, "count"))
	require.NoError(t, err)
	require.Equal(t, "x", string(data), "a completed execution must never be restarted")
	request.Spec.Command = []string{"sh", "-c", "echo changed"}
	_, err = engine.Run(ctx, request)
	require.ErrorContains(t, err, "conflicts")
	request.Identity += "/invalid-command"
	request.Spec.Command = []string{"/no-such-executable"}
	failed, err := engine.Run(ctx, request)
	require.Error(t, err)
	require.NotEmpty(t, failed.Error)
	require.Nil(t, failed.Code, "startup failure must not be recorded as exit zero")
	t.Cleanup(func() { _ = engine.Remove(context.Background(), failed.ContainerID) })
}

type startNotification struct{ started chan struct{} }

func (notification startNotification) Write(data []byte) (int, error) {
	select {
	case notification.started <- struct{}{}:
	default:
	}
	return len(data), nil
}

func TestDockerCancellationStopsOwnedContainer(t *testing.T) {
	if os.Getenv("WATTS_DOCKER_INTEGRATION") != "1" {
		t.Skip("set WATTS_DOCKER_INTEGRATION=1 with ubuntu:latest available locally")
	}
	engine, err := Connect()
	require.NoError(t, err)
	t.Cleanup(func() { _ = engine.Close() })
	root := t.TempDir()
	started := make(chan struct{}, 1)
	request := Request{Identity: root, Spec: workload.Spec{Image: "ubuntu:latest", Command: []string{"sh", "-c", "echo started; exec sleep 60"}}, Stdout: startNotification{started}, Stderr: io.Discard}
	for name, target := range map[string]*string{"inputs": &request.InputDir, "outputs": &request.OutputDir, "runtime": &request.RuntimeDir, "workspace": &request.WorkspaceDir} {
		*target = filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(*target, 0755))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	type execution struct {
		exit Exit
		err  error
	}
	finished := make(chan execution, 1)
	go func() {
		exit, err := engine.Run(ctx, request)
		finished <- execution{exit, err}
	}()
	select {
	case <-started:
		cancel()
	case result := <-finished:
		t.Fatalf("container exited before startup: %v", result.err)
	case <-ctx.Done():
		t.Fatal("container did not report startup")
	}
	result := <-finished
	require.ErrorIs(t, result.err, context.Canceled)
	require.NotEmpty(t, result.exit.ContainerID)
	t.Cleanup(func() { _ = engine.Remove(context.Background(), result.exit.ContainerID) })
	inspected, err := engine.client.ContainerInspect(context.Background(), result.exit.ContainerID)
	require.NoError(t, err)
	require.False(t, inspected.State.Running, "cancellation must not leave a workload running")
}
