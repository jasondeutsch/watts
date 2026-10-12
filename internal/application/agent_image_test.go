package application

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/jasondeutsch/watts/internal/workload"
	"github.com/stretchr/testify/require"
)

// Real Pi and Ralph run inside the image against a deterministic fake provider.
// No paid calls, host agent installation, or user project is involved.
func TestAgentImageContract(t *testing.T) {
	requireDocker(t)
	for _, mode := range []string{"pi", "ralph", "provider-error"} {
		t.Run(mode, func(t *testing.T) {
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				var body struct {
					Messages []struct {
						Role string `json:"role"`
					} `json:"messages"`
				}
				require.NoError(t, json.NewDecoder(request.Body).Decode(&body))
				require.Equal(t, "Bearer fixture-key", request.Header.Get("Authorization"))
				if mode == "provider-error" {
					response.Header().Set("Content-Type", "application/json")
					response.WriteHeader(429)
					fmt.Fprint(response, `{"error":{"message":"fixture provider rate limited","code":429}}`)
					return
				}
				hasTool := false
				for _, message := range body.Messages {
					if message.Role == "tool" {
						hasTool = true
					}
				}
				delta := map[string]any{"role": "assistant", "content": "Hello"}
				finish := "stop"
				if mode == "ralph" {
					if hasTool {
						delta["content"] = "<promise>DONE</promise>"
					} else {
						delete(delta, "content")
						finish = "tool_calls"
						delta["tool_calls"] = []any{map[string]any{"index": 0, "id": "write-1", "type": "function", "function": map[string]any{"name": "write", "arguments": `{"path":"/workspace/hello.txt","content":"hello"}`}}}
					}
				}
				writeFixtureCompletion(response, delta, finish)
			}))
			listener, err := net.Listen("tcp", "0.0.0.0:0")
			require.NoError(t, err)
			server.Listener = listener
			server.Start()
			defer server.Close()
			port := listener.Addr().(*net.TCPAddr).Port
			options := `{}`
			files := map[string]string{}
			if mode == "ralph" {
				options = `{"ralph":true}`
				files["ralph.md"] = "---\nmax_iterations: 1\ntimeout: 30\ncompletion_promise: DONE\ncompletion_gate: required\nstop_on_error: true\n---\nWrite hello.txt in /workspace, then emit <promise>DONE</promise>.\n"
			}
			files["launcher.json"] = options
			step := orchestration.Step{Name: "agent", Agent: "fixture", Identity: "fixture", Provider: "fixture", Model: "fixture-model", Prompt: "Say hello", Container: &workload.Spec{Image: "watts/pi-ralph:0.1.0", Command: []string{"node", "/opt/watts/launcher.mjs"}, Network: "bridge", Workspace: "write", Files: files}, AgentSettings: &orchestration.AgentSettings{BaseURL: fmt.Sprintf("http://host.docker.internal:%d/v1", port), API: "openai-completions", APIKeyEnv: "FIXTURE_PROVIDER_KEY"}, SecretEnvironment: map[string]string{"FIXTURE_PROVIDER_KEY": "FIXTURE_SOURCE_KEY"}}
			if mode == "ralph" {
				step.Tools = []workload.Tool{{Name: "verify", Command: &workload.CommandTool{Argv: []string{"sh", "-c", "test -f /workspace/hello.txt"}}}}
				step.Gates = []workload.Gate{{Name: "hello-exists", Tool: "verify"}}
				step.GatePolicy = &workload.GatePolicy{MaxRepairs: 0}
			}
			step.Skills = []workload.Skill{{Name: "spec-guidance", Source: workload.SkillSource{ImagePath: "/opt/watts/kit/skills/spec-template"}}}
			t.Setenv("FIXTURE_SOURCE_KEY", "fixture-key")
			app, request := boundTemporalTask(t, step)
			var output strings.Builder
			app.Output = &output
			app.ErrorOutput = &output
			result, err := executeBound(t, app, request)
			if mode == "provider-error" {
				require.ErrorContains(t, err, "fixture provider rate limited")
				require.NoFileExists(t, filepath.Join(app.Root, "hello.txt"))
				return
			}
			require.NoError(t, err, output.String())
			require.Equal(t, "success", result.Outcome)
			events, err := workload.ReadEvents(filepath.Join(filepath.Dir(app.stepInputPath(request)), "workload-events.jsonl"))
			require.NoError(t, err)
			usage := workload.Summarize(events)
			require.NotNil(t, usage)
			require.NotNil(t, usage.Input)
			require.Greater(t, *usage.Input, int64(0))
			require.Nil(t, usage.Cost)
			if mode == "ralph" {
				require.Equal(t, "hello", readFile(t, filepath.Join(app.Root, "hello.txt")))
				require.NoDirExists(t, filepath.Join(app.Root, ".ralph-runner"))
			}
			data, err := os.ReadFile(app.WorkflowBindingPath(request.Input.Task))
			require.NoError(t, err)
			require.NotContains(t, string(data), "fixture-key")
		})
	}
}

func TestAgentCommandToolsAndGateRepair(t *testing.T) {
	requireDocker(t)
	for _, mode := range []string{"repair", "exhausted"} {
		t.Run(mode, func(t *testing.T) {
			var mu sync.Mutex
			calls := 0
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body struct {
					Messages []struct {
						Role    string `json:"role"`
						Content any    `json:"content"`
					} `json:"messages"`
					Tools []struct {
						Function struct {
							Name string `json:"name"`
						} `json:"function"`
					} `json:"tools"`
				}
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				declared := false
				for _, tool := range body.Tools {
					if tool.Function.Name == "record" {
						declared = true
					}
				}
				require.True(t, declared, "declared command tool was not exposed to Pi")
				mu.Lock()
				calls++
				mu.Unlock()
				repair := false
				hasTool := false
				for _, message := range body.Messages {
					if message.Role == "user" && strings.Contains(fmt.Sprint(message.Content), "Required gate repair feedback") {
						repair = true
					}
					if message.Role == "tool" {
						hasTool = true
					}
				}
				delta := map[string]any{"role": "assistant", "content": "Done"}
				finish := "stop"
				if !hasTool {
					name, args := "record", `{}`
					if repair && mode == "repair" {
						name, args = "write", `{"path":"/workspace/fixed.txt","content":"fixed"}`
					}
					delete(delta, "content")
					finish = "tool_calls"
					delta["tool_calls"] = []any{map[string]any{"index": 0, "id": "fixture-call", "type": "function", "function": map[string]any{"name": name, "arguments": args}}}
				}
				writeFixtureCompletion(w, delta, finish)
			}))
			listener, err := net.Listen("tcp", "0.0.0.0:0")
			require.NoError(t, err)
			server.Listener = listener
			server.Start()
			defer server.Close()
			tool := workload.Tool{Name: "record", Command: &workload.CommandTool{Argv: []string{"sh", "-c", "printf called > /workspace/called.txt"}}}
			verify := workload.Tool{Name: "verify", Command: &workload.CommandTool{Argv: []string{"sh", "-c", "test -f /workspace/fixed.txt"}, TimeoutSeconds: 10}}
			repairs := 1
			if mode == "exhausted" {
				repairs = 0
			}
			step := orchestration.Step{Name: "agent", Agent: "fixture", Identity: "fixture", Provider: "fixture", Model: "fixture-model", Prompt: "Use record then finish", Tools: []workload.Tool{tool, verify}, Gates: []workload.Gate{{Name: "fixed-required", Tool: "verify"}}, GatePolicy: &workload.GatePolicy{MaxRepairs: repairs}, Container: &workload.Spec{Image: "watts/pi-ralph:0.1.0", Command: []string{"node", "/opt/watts/launcher.mjs"}, Network: "bridge", Workspace: "write"}, AgentSettings: &orchestration.AgentSettings{BaseURL: fmt.Sprintf("http://host.docker.internal:%d/v1", listener.Addr().(*net.TCPAddr).Port), API: "openai-completions", APIKeyEnv: "FIXTURE_PROVIDER_KEY"}, SecretEnvironment: map[string]string{"FIXTURE_PROVIDER_KEY": "FIXTURE_SOURCE_KEY"}}
			t.Setenv("FIXTURE_SOURCE_KEY", "fixture-key")
			app, request := boundTemporalTask(t, step)
			var output strings.Builder
			app.Output = &output
			app.ErrorOutput = &output
			result, err := executeBound(t, app, request)
			evidence := filepath.Dir(app.stepInputPath(request))
			if mode == "exhausted" {
				require.ErrorContains(t, err, "Required gates failed after 0 repair(s)")
				require.NoFileExists(t, filepath.Join(app.Root, "called.txt"))
				require.FileExists(t, filepath.Join(evidence, "workload-gates.json"))
			} else {
				require.NoError(t, err, output.String())
				require.Equal(t, "success", result.Outcome)
				require.Equal(t, "called", readFile(t, filepath.Join(app.Root, "called.txt")))
				require.Equal(t, "fixed", readFile(t, filepath.Join(app.Root, "fixed.txt")))
				var report workload.GateReport
				require.NoError(t, json.Unmarshal([]byte(readFile(t, filepath.Join(evidence, "workload-gates.json"))), &report))
				require.Equal(t, "passed", report.Results[0].Status)
				files, err := filepath.Glob(filepath.Join(evidence, "tools", "*.json"))
				require.NoError(t, err)
				require.GreaterOrEqual(t, len(files), 3)
				mu.Lock()
				require.GreaterOrEqual(t, calls, 4)
				mu.Unlock()
			}
		})
	}
}

func writeFixtureCompletion(response http.ResponseWriter, delta map[string]any, finish string) {
	response.Header().Set("Content-Type", "text/event-stream")
	first, _ := json.Marshal(map[string]any{"id": "completion", "object": "chat.completion.chunk", "model": "fixture-model", "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": nil}}})
	last, _ := json.Marshal(map[string]any{"id": "completion", "object": "chat.completion.chunk", "model": "fixture-model", "choices": []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": finish}}, "usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 2, "total_tokens": 12}})
	fmt.Fprintf(response, "data: %s\n\ndata: %s\n\ndata: [DONE]\n\n", first, last)
}
