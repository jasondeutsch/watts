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
	"github.com/jasondeutsch/watts/web"
)

type record struct {
	PID int    `json:"pid"`
	URL string `json:"url"`
}

func paths(app *application.Service) (string, string) {
	return filepath.Join(app.Root, ".watts", "service.lock"), filepath.Join(app.Root, ".watts", "service.json")
}
func lock(app *application.Service) (*os.File, error) {
	path, _ := paths(app)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		return nil, errors.New("Watts is already running for this project")
	}
	return file, nil
}
func release(file *os.File) { syscall.Flock(int(file.Fd()), syscall.LOCK_UN); file.Close() }

func Start(ctx context.Context, app *application.Service, detached bool) error {
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
	cfg, err := app.LoadConfig()
	if err != nil {
		return err
	}
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
		logger.Info("Worker stopped", "next_action", "Run watts start to resume processing")
	}()
	done := make(chan error, 1)
	go func() { done <- web.Serve(ctx, app, listener) }()
	_, path := paths(app)
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
	_, path := paths(app)
	os.Remove(path)
	release(file)
	binary, err := os.Executable()
	if err != nil {
		return err
	}
	log, err := os.OpenFile(filepath.Join(app.Root, ".watts", "service.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
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
			return fmt.Errorf("detached startup failed: %v; see .watts/service.log", err)
		case <-ticker.C:
			data, e := os.ReadFile(path)
			var state record
			if e == nil && json.Unmarshal(data, &state) == nil && state.PID == command.Process.Pid {
				fmt.Fprintf(app.Output, "Watts started detached (PID %d): %s\nLog: .watts/service.log\n", state.PID, state.URL)
				return nil
			}
		case <-ctx.Done():
			command.Process.Signal(syscall.SIGTERM)
			return ctx.Err()
		case <-timeout.C:
			command.Process.Signal(syscall.SIGTERM)
			return errors.New("startup timed out; see .watts/service.log")
		}
	}
}
func Stop(ctx context.Context, app *application.Service) error {
	file, err := lock(app)
	if err == nil {
		release(file)
		return errors.New("Watts is not running for this project")
	}
	_, path := paths(app)
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
			return errors.New("Watts has not stopped yet; inspect .watts/service.log")
		}
	}
}
