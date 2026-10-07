package devtools

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jasondeutsch/watts/cli/internal/arguments"
	"github.com/jasondeutsch/watts/cli/internal/terminal"
	projectconfig "github.com/jasondeutsch/watts/internal/config"
)

// GatewayHandler answers OpenAI-style requests with a canned reply, so Watts and Pi can be exercised
// without a real model or a real key. status lets a test provoke a failure such as 401.
func GatewayHandler(reply string, status int) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"mock","object":"model"}]}`))
	})
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		if status != 0 && status != 200 {
			http.Error(w, fmt.Sprintf(`{"code":%d,"message":"mock failure"}`, status), status)
			return
		}
		var req struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Model == "" {
			req.Model = "mock"
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-mock", "object": "chat.completion", "model": req.Model,
			"choices": []map[string]any{{"index": 0, "finish_reason": "stop", "message": map[string]string{"role": "assistant", "content": reply}}},
			"usage":   map[string]int{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
		})
	})
	return mux
}

func MockServer(a *terminal.Context, args []string) error {
	fl := arguments.NewFlags(a.ErrorOutput, "dev mock-server")
	addr := fl.String("addr", "127.0.0.1:8099", "address to listen on")
	reply := fl.String("reply", "mock reply", "text every completion returns")
	status := fl.Int("status", 200, "HTTP status to return instead of a completion (for example 401)")
	if _, err := arguments.Parse(fl, args); err != nil {
		return arguments.UsageError(err)
	}
	srv := &http.Server{Addr: *addr, Handler: GatewayHandler(*reply, *status), ReadHeaderTimeout: 5 * time.Second}
	a.Say("Mock gateway on http://%s/v1 (status %d). Press Ctrl-C to stop.", *addr, *status)
	a.Say("Declare it as a provider in %s with base_url http://%s/v1 and no key.", projectconfig.Filename, *addr)
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigc)
	go func() { <-sigc; _ = srv.Close() }()
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
