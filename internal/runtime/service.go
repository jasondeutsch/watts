// Package runtime manages the project worker and task UI as one service.
package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"syscall"
	"time"

	"github.com/jasondeutsch/watts/internal/application"
	"github.com/jasondeutsch/watts/internal/storage"
	"github.com/jasondeutsch/watts/internal/workerstate"
	"github.com/jasondeutsch/watts/web"
)

type record struct {
	PID int    `json:"pid"`
	URL string `json:"url"`
}

func paths(app *application.Service) (string, string, error) {
	directory, err := app.ProjectWorkerDir()
	if err != nil {
		return "", "", err
	}
	return filepath.Join(directory, "service.lock"), filepath.Join(directory, "service.json"), nil
}
func logPath(app *application.Service) (string, error) {
	directory, err := app.ProjectWorkerDir()
	return filepath.Join(directory, "service.log"), err
}
func lock(app *application.Service) (*os.File, error) {
	path, _, err := paths(app)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		return nil, fmt.Errorf("Watts is already running for this project: %w", err)
	}
	return file, nil
}
func release(file *os.File) { syscall.Flock(int(file.Fd()), syscall.LOCK_UN); file.Close() }

func Start(ctx context.Context, app *application.Service, detached bool) (err error) {
	defer func() { err = storage.DescribeHostError(err) }()
	if err := app.CheckLegacySettings(); err != nil {
		return err
	}
	if err := checkLegacyWorker(app); err != nil {
		return err
	}
	if err := app.ProtectRuntimeFiles(); err != nil {
		return err
	}
	cfg, err := app.LoadServiceConfig()
	if err != nil {
		return err
	}
	if err = app.RegisterWorkspace(cfg); err != nil {
		return err
	}
	store, err := workerstate.Default()
	if err != nil {
		return err
	}
	if err = store.PruneProject(app.Root, cfg.RuntimeRetention); err != nil {
		return fmt.Errorf("runtime retention: %w", err)
	}
	if detached {
		return detach(ctx, app)
	}
	file, err := lock(app)
	if err != nil {
		return err
	}
	defer release(file)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer listener.Close()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	settings := app.ResolvedTemporalSettings(cfg)
	dialContext, dialCancel := context.WithTimeout(ctx, 10*time.Second)
	client, err := app.ConnectTemporalWorker(dialContext, settings)
	dialCancel()
	if err != nil {
		return err
	}
	defer client.Close()
	worker := app.NewTemporalWorker(client, settings.TaskQueue)
	if err = worker.Start(); err != nil {
		return err
	}
	logger := app.WorkerLogger()
	defer func() {
		logger.Info("Worker stopping", "next_action", "Active work is stopping; Temporal retains workflow history")
		worker.Stop()
		logger.Info("Worker stopped", "next_action", "Run wctl start to resume processing")
	}()
	done := make(chan error, 1)
	go func() { done <- web.Serve(ctx, app, listener) }()
	_, path, err := paths(app)
	if err != nil {
		return err
	}
	state := record{PID: os.Getpid(), URL: "http://" + listener.Addr().String()}
	data, _ := json.Marshal(state)
	if err = os.WriteFile(path, data, 0600); err != nil {
		return err
	}
	defer os.Remove(path)
	logger.Info("Worker ready", "address", settings.Address, "namespace", settings.Namespace, "queue", settings.TaskQueue, "web_ui", state.URL)
	if err = openBrowser(state.URL); err != nil {
		fmt.Fprintf(app.ErrorOutput, "Could not open browser: %v. Open %s manually.\n", err, state.URL)
	}
	select {
	case err = <-done:
		cancel()
		return err
	case <-ctx.Done():
		cancel()
		return <-done
	}
}
func openBrowser(url string) error {
	switch goruntime.GOOS {
	case "darwin":
		return exec.Command("open", url).Run()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Run()
	default:
		return exec.Command("xdg-open", url).Run()
	}
}
func detach(ctx context.Context, app *application.Service) error {
	file, err := lock(app)
	if err != nil {
		return err
	}
	_, path, err := paths(app)
	if err != nil {
		return err
	}
	os.Remove(path)
	release(file)
	binary, err := os.Executable()
	if err != nil {
		return err
	}
	logName, err := logPath(app)
	if err != nil {
		return err
	}
	log, err := os.OpenFile(logName, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer log.Close()
	command := exec.Command(binary, "start")
	command.Dir = app.Root
	command.Stdout = log
	command.Stderr = log
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err = command.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.NewTimer(20 * time.Second)
	defer timeout.Stop()
	for {
		select {
		case err = <-done:
			return fmt.Errorf("detached startup failed: %v; see %s", err, logName)
		case <-ticker.C:
			data, e := os.ReadFile(path)
			var state record
			if e == nil && json.Unmarshal(data, &state) == nil && state.PID == command.Process.Pid {
				fmt.Fprintf(app.Output, "Watts started detached (PID %d): %s\nLog: %s\n", state.PID, state.URL, logName)
				return nil
			}
		case <-ctx.Done():
			command.Process.Signal(syscall.SIGTERM)
			return ctx.Err()
		case <-timeout.C:
			command.Process.Signal(syscall.SIGTERM)
			return fmt.Errorf("startup timed out; see %s", logName)
		}
	}
}
func Stop(ctx context.Context, app *application.Service) error {
	file, err := lock(app)
	if err == nil {
		release(file)
		return errors.New("Watts is not running for this project")
	}
	if !errors.Is(err, syscall.EWOULDBLOCK) {
		return err
	}
	_, path, err := paths(app)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var state record
	if err = json.Unmarshal(data, &state); err != nil {
		return err
	}
	if state.PID <= 1 {
		return errors.New("invalid service PID")
	}
	if err = syscall.Kill(state.PID, syscall.SIGTERM); err != nil {
		return err
	}
	deadline := time.NewTimer(20 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			file, err = lock(app)
			if err == nil {
				release(file)
				fmt.Fprintln(app.Output, "Watts stopped.")
				return nil
			}
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return fmt.Errorf("Watts has not stopped yet; inspect the worker service log")
		}
	}
}

// Never start a second worker beside a still-running project-local service.
// Legacy state is inspected only for this guard; it is not an execution fallback.
func checkLegacyWorker(app *application.Service) error {
	name := filepath.Join(app.Root, ".watts", "service.lock")
	file, err := os.OpenFile(name, os.O_RDWR, 0)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()
	if err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return fmt.Errorf("stop the old project-local worker before starting this version: %w", err)
	}
	_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	return nil
}
