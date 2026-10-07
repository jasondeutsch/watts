package web

import (
	"context"
	"github.com/jasondeutsch/watts/internal/application"
	"net"
	"net/http"
	"time"
)

// Serve runs the task UI on an already-bound listener until cancellation.
func Serve(ctx context.Context, app *application.Service, listener net.Listener) error {
	monitor := newMonitor(app)
	defer monitor.close()
	server := &http.Server{Handler: monitor.routes(), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdown)
	}()
	err := server.Serve(listener)
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}
