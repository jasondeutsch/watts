package application

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunIsCheckedAgainstWhatWasRecorded(t *testing.T) {
	needTools(t, "awk")
	ta, task := taskRepo(t)

	customPi(t, `echo "tampered" >> watts.yaml
`+sessionScript("deepseek/deepseek-v4-flash"))
	code, out, errs := executeAgentActivity(t, ta, task, "build", false)
	assert.False(t, code == 0 || !strings.Contains(out+errs, "changed paths it may not change: watts.yaml"),
		"a change to watts.yaml must fail the stage: %d\n%s%s", code, out, errs)
	assert.False(t, !strings.Contains(out, "models seen: deepseek/deepseek-v4-flash") || !strings.Contains(out, "tool calls: 1"),
		"the summary comes from the session record:\n%s", out)

	_ = os.WriteFile(filepath.Join(ta.Root, "watts.yaml"), []byte(strings.ReplaceAll(readFile(t, filepath.Join(ta.Root, "watts.yaml")), "tampered\n", "")), 0o644)

	customPi(t, sessionScript("some-other-model"))
	_ = os.RemoveAll(filepath.Join(ta.AgentDir(loadCfg(t, ta), "build"), "sessions"))
	code, out, errs = executeAgentActivity(t, ta, task, "build", false)
	assert.False(t, code == 0 || !strings.Contains(out+errs, "the session record shows model some-other-model, but deepseek/deepseek-v4-flash is configured"),
		"a wrong model must fail the stage: %d\n%s%s", code, out, errs)
}
func TestTimeLimitStopsARun(t *testing.T) {
	ta := newProject(t)
	ta.initRepo(t)
	customPi(t, "sleep 20")
	cfg := loadCfg(t, ta)
	cfg.Limits.MaxMinutes = 1
	ta.TimeoutUnit = 100 * time.Millisecond
	start := time.Now()
	err := ta.RunPi(cfg, "build", nil, false, "-p", "x")
	require.False(t, err == nil || !strings.Contains(err.Error(), "time limit"),
		"expected the time limit to stop the run, got %v", err)
	assert.False(t, time.Since(start) > 10*time.Second,
		"the run was not stopped promptly")
}

func TestProviderErrorFailsStageEvenWhenPiExitsSuccessfully(t *testing.T) {
	ta, task := taskRepo(t)
	customPi(t, strings.Replace(sessionScript("deepseek/deepseek-v4-flash"), `"role":"assistant"`, `"role":"assistant","stopReason":"error","errorMessage":"Provider finish_reason: error"`, 1))
	code, out, diagnostics := executeAgentActivity(t, ta, task, "build", false)
	require.NotZero(t, code)
	require.Contains(t, out+diagnostics, "agent error: Provider finish_reason: error")
}
func TestSessionSummary(t *testing.T) {
	ta := newProject(t)
	ta.initRepo(t)
	writeFile(t, filepath.Join(ta.AgentDir(loadCfg(t, ta), "build"), "sessions", "x", "s.jsonl"),
		`{"type":"model_change","model":"m1"}
{"type":"message","message":{"role":"assistant","model":"m1","stopReason":"error","content":[{"type":"toolCall","name":"a"},{"type":"toolCall","name":"b"}]}}
{"type":"message","message":{"role":"toolResult","isError":true,"content":[]}}
`)
	info, ok := ta.ReadSession(loadCfg(t, ta), "build", time.Time{})
	assert.False(t, !ok || info.Assistant != 1 || info.ToolCalls != 2 || info.Errors != 2 || !reflect.DeepEqual(info.Models, []string{"m1"}),
		"session summary: %v %+v", ok, info)
	assert.False(t, ModelMismatch("m1", []string{"m1"}) != "" || ModelMismatch("m1", []string{"other/m1"}) != "" || ModelMismatch("m1", nil) != "",
		"matching models, with or without a provider prefix, are fine")
	assert.NotEqual(t, "", ModelMismatch("m1", []string{"m2"}),
		"a different model must be reported")
}
