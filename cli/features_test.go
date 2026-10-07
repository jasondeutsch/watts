package cli

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"

	"github.com/jasondeutsch/watts/cli/internal/devtools"
	"github.com/jasondeutsch/watts/internal/application"
	projectconfig "github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/kit"
	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/jasondeutsch/watts/internal/storage"
)

// customPi puts a pi stand-in first on PATH. body runs for every call that is not --version,
// list or install, with PI_CODING_AGENT_DIR and the arguments available.
func customPi(t *testing.T, body string) {
	t.Helper()
	bin := t.TempDir()
	pi := "#!/usr/bin/env bash\ncase \"${1:-}\" in\n  --version) echo \"pi 1.0.1\"; exit 0 ;;\n  list) echo \"" + application.RalphPackage + "\"; exit 0 ;;\n  install) exit 0 ;;\nesac\n" + body + "\n"
	for name, content := range map[string]string{"pi": pi, "node": "#!/bin/sh\necho v22.22.1\n", "npm": "#!/bin/sh\nexit 0\n"} {
		require.NoError(t, os.WriteFile(filepath.Join(bin, name), []byte(content), 0o755))
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// sessionLine is a session record entry like the ones Pi writes.
func sessionScript(model string) string {
	return `mkdir -p "$PI_CODING_AGENT_DIR/sessions/s"
printf '%s\n' '{"type":"message","timestamp":"2026-10-05T16:14:10.323Z","message":{"role":"assistant","model":"` + model + `","content":[{"type":"text","text":"working on it"},{"type":"toolCall","name":"bash","arguments":{"command":"ls"}}]}}' >> "$PI_CODING_AGENT_DIR/sessions/s/a.jsonl"`
}
func loadCfg(t *testing.T, ta *testApp) projectconfig.Config {
	t.Helper()
	cfg, err := ta.LoadConfig()
	require.NoError(t, err)

	return cfg
}
func saveCfg(t *testing.T, ta *testApp, cfg projectconfig.Config) {
	t.Helper()
	{
		err := ta.SaveConfig(cfg)
		require.NoError(t, err,
			"save: %v", err)
	}
}
func TestFreshInitDefaults(t *testing.T) {
	needTools(t, "awk")
	ta := newProject(t)
	{
		code, out, errs := runCLI(t, ta.Root, "init", "--no-install")
		require.Equal(t, 0, code,
			"init: %d %s%s", code, out, errs)
	}

	cfg := loadCfg(t, ta)
	require.Equal(t, "tasks", cfg.TasksDir,
		"default tasks_dir: %q", cfg.TasksDir)
	{
		code, out, errs := runCLI(t, ta.Root, "task", "new", "demo")
		require.Equal(t, 0, code,
			"task new: %d %s%s", code, out, errs)
	}

	matches, _ := filepath.Glob(filepath.Join(ta.Root, "tasks", "*-demo"))
	require.Len(t, matches, 1,
		"the task must be created in tasks: %v", matches)

	ralph := readFile(t, filepath.Join(matches[0], "RALPH.md"))
	assert.False(t, strings.Contains(ralph, "__TASKS__") || !strings.Contains(ralph, "tasks/"+filepath.Base(matches[0])),
		"the loop prompt must point at the configured tasks folder:\n%s", ralph)

	if code, _, errs := runTaskScript(t, ta.Root, "preflight.sh", filepath.Base(matches[0])); !strings.Contains(errs, "APPROVAL") && code == 0 {
		t.Logf("check by bare name: %d %s", code, errs)
	}
}
func TestWattsDirIsRebuildable(t *testing.T) {
	ta := newProject(t)
	{
		code, out, errs := runCLI(t, ta.Root, "init", "--no-install")
		require.Equal(t, 0, code,
			"init: %d %s%s", code, out, errs)
	}

	cfg := loadCfg(t, ta)
	setWorkflowModels(&cfg, "gw", "", "")
	setWorkflowModels(&cfg, "", "m1", "m2")
	cfg.Providers = map[string]projectconfig.Provider{"gw": {BaseURL: "https://gw.example/v1", APIKeyEnv: "GW_KEY", Models: []projectconfig.ProviderModel{{ID: "m1"}, {ID: "m2"}}}}
	cfg.SkillsDirs = []string{"team-skills"}
	writeFile(t, filepath.Join(ta.Root, "team-skills", "my-skill", "SKILL.md"), "---\nname: my-skill\n---\n")
	saveCfg(t, ta, cfg)

	require.NoError(t, ta.ConfigAgents(cfg, false))

	before := readFile(t, filepath.Join(ta.Root, "watts.json"))
	want := readFile(t, filepath.Join(ta.Root, ".watts/pi-agent-build/models.json"))

	require.NoError(t, os.RemoveAll(filepath.Join(ta.Root, ".watts")))

	{
		code, out, errs := runCLI(t, ta.Root, "init", "--no-install")
		require.Equal(t, 0, code,
			"re-init after deleting .watts: %d %s%s", code, out, errs)
	}
	assert.Equal(t, before, readFile(t, filepath.Join(ta.Root, "watts.json")),
		"deleting .watts and running init must not change the configuration")
	assert.Equal(t, want, readFile(t, filepath.Join(ta.Root, ".watts/pi-agent-build/models.json")),
		"models.json must be regenerated from watts.json")
	assert.False(t, !exists(filepath.Join(ta.Root, ".watts/pi-agent-review/skills/my-skill/SKILL.md")) || !exists(filepath.Join(ta.Root, ".watts/kit/scripts/lib.sh")),
		"the kit and the project skills must come back")
}
func gatewayConfig(ta *testApp, t *testing.T, base string) projectconfig.Config {
	t.Helper()
	cfg := loadCfg(t, ta)
	setWorkflowModels(&cfg, "gw", "", "")
	setWorkflowModels(&cfg, "", "m1", "m2")
	cfg.Providers = map[string]projectconfig.Provider{"gw": {Name: "Gateway", BaseURL: base, APIKeyEnv: "GW_KEY",
		Models: []projectconfig.ProviderModel{{ID: "m1", Reasoning: true, ContextWindow: 200000, MaxTokens: 16384}, {ID: "m2"}}}}
	return cfg
}
func TestAgentEnvShowsNamesNeverValues(t *testing.T) {
	ta := newProject(t)
	ta.initRepo(t, "--env-passthrough", "MY_SECRET", "--env-passthrough", "NOT_SET_HERE")
	t.Setenv("MY_SECRET", "hunter2-value")
	os.Unsetenv("NOT_SET_HERE")
	code, out, errs := runCLI(t, ta.Root, "agent", "env", "build")
	require.Equal(t, 0, code,
		"agent env: %d %s", code, errs)
	assert.False(t, !strings.Contains(out, "MY_SECRET") || !strings.Contains(out, "NOT_SET_HERE") || !strings.Contains(out, "not set in your shell"),
		"the report must list names and say which are unset:\n%s", out)
	assert.NotContains(t, out, "hunter2-value",
		"a value was printed")

	code, out, _ = runCLI(t, ta.Root, "agent", "env", "review", "--json")
	var parsed struct {
		Agent     string                 `json:"agent"`
		Variables []application.EnvEntry `json:"variables"`
	}
	assert.False(t, code != 0 || json.Unmarshal([]byte(out), &parsed) != nil || parsed.Agent != "review" || len(parsed.Variables) < 10 || strings.Contains(out, "hunter2-value"),
		"json report: %d\n%s", code, out)
	{
		code, _, _ := runCLI(t, ta.Root, "agent", "env", "nobody")
		assert.NotEqual(t, 0, code,
			"an unknown agent must fail")
	}
}
func TestDoctorLiveAndJSON(t *testing.T) {
	ta := newProject(t)
	ta.initRepo(t)
	var gotAuth, gotModel string
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		var req struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotModel = req.Model
		devtools.GatewayHandler("hi", 200).ServeHTTP(w, r)
	}))
	defer ok.Close()
	deny := httptest.NewServer(devtools.GatewayHandler("", 401))
	defer deny.Close()

	cfg := gatewayConfig(ta, t, ok.URL+"/v1")
	saveCfg(t, ta, cfg)

	require.NoError(t, ta.ConfigAgents(cfg, false))

	t.Setenv("GW_KEY", "live-key")
	{
		c := ta.LiveCheck(cfg, "build")
		require.Equal(t, "ok", c.Level,
			"a 200 must pass: %+v", c)
	}
	assert.False(t, gotAuth != "Bearer live-key" || gotModel != "m1",
		"the call must use the agent's key and model: %q %q", gotAuth, gotModel)

	cfg.Providers["gw"] = projectconfig.Provider{BaseURL: deny.URL + "/v1", APIKeyEnv: "GW_KEY", Models: cfg.Providers["gw"].Models}
	saveCfg(t, ta, cfg)

	require.NoError(t, ta.ConfigAgents(cfg, true))

	{
		c := ta.LiveCheck(cfg, "build")
		assert.False(t, c.Level != "fail" || !strings.Contains(c.Detail, "rejected the key"),
			"a 401 must be reported as a rejected key: %+v", c)
	}

	stubBin(t, "pi 1.0.1")
	code, out, _ := runCLI(t, ta.Root, "doctor", "--json")
	var parsed struct {
		OK     bool                `json:"ok"`
		Checks []application.Check `json:"checks"`
	}
	{
		err := json.Unmarshal([]byte(out), &parsed)
		require.False(t, err != nil || len(parsed.Checks) == 0,
			"doctor --json must print JSON: %v\n%s", err, out)
	}
	assert.False(t, parsed.OK || code != 1,
		"the extension is not installed, so doctor must fail: ok=%v code=%d", parsed.OK, code)
}
func TestRunUsesTemporalWithDefaultConnectionSettings(t *testing.T) {
	ta, task := taskRepo(t)
	customPi(t, `echo unexpected > pi-was-started`)
	code, out, errs := runCLI(t, ta.Root, "task", "run", task, "--dry-run")
	require.False(t, code != 0 || !strings.Contains(out, "localhost:7233") || !strings.Contains(out, "specification"),
		"resume: %d %s%s", code, out, errs)
	require.False(t, exists(filepath.Join(ta.Root, "pi-was-started")),
		"resume fell back to launching Pi directly")
}
func TestDryRunStartsNothing(t *testing.T) {
	needTools(t, "awk")
	ta, task := taskRepo(t)
	log := filepath.Join(t.TempDir(), "pi.log")
	customPi(t, `echo called >> `+log)
	t.Setenv("MY_SECRET", "do-not-print")
	{
		code, _, _ := runCLI(t, ta.Root, "config", "env", "add", "MY_SECRET")
		require.Equal(t, 0, code,
			"env add")
	}

	code, out, errs := runCLI(t, ta.Root, "task", "run", task, "--dry-run")
	require.Equal(t, 0, code,
		"dry run: %d %s%s", code, out, errs)

	for _, want := range []string{"Temporal workflow", "localhost:7233", "build", "review"} {
		assert.Contains(t, out, want,
			"dry run output lacks %q:\n%s", want, out)
	}
	assert.NotContains(t, out, "do-not-print",
		"a value was printed")
	assert.False(t, exists(log),
		"a dry run must not start Pi")
	assert.False(t, exists(filepath.Join(ta.StateDir(task), "state.json")),
		"a dry run must not record a stage")
}

const approvedSpec = `# SPEC: demo

## Status
Approved
Version: v1

## Acceptance criteria
- AC-1: While idle, when polled, the system shall answer.

## Risk tier
1

## Approved by
Jane Doe, 2026-10-05
`
const approvedPlan = `# PLAN: demo

Spec ref: SPEC.md v1

## Risk tier recheck

Risk tier after planning: 1

## Quality gates

- ` + "`true`" + `

## User stories

### US-001: First

Covers: AC-1
Depends on: none

## Approved by

Jane Doe, 2026-10-05
`

func TestApprovalNeedsNothingStored(t *testing.T) {
	needTools(t, "awk")
	ta := newProject(t)
	t.Setenv("USER", "Jane Doe")
	ta.initRepo(t, "--tasks-dir", "tasks")
	{
		code, out, errs := runCLI(t, ta.Root, "task", "new", "demo")
		require.Equal(t, 0, code,
			"task new: %d %s%s", code, out, errs)
	}

	matches, _ := filepath.Glob(filepath.Join(ta.Root, "tasks", "*-demo"))
	task := "tasks/" + filepath.Base(matches[0])
	writeFile(t, filepath.Join(ta.Root, task, "SPEC.md"), approvedSpec)
	writeFile(t, filepath.Join(ta.Root, task, "PLAN.md"), approvedPlan)
	{
		code, out, errs := runTaskScript(t, ta.Root, "preflight.sh", task)
		require.False(t, code == 0 || !strings.Contains(out+errs, "watts task decide"),
			"before approval the check must say how to approve: %d\n%s%s", code, out, errs)
	}
	{
		code, out, errs := recordTaskApproval(t, ta.Root, task)
		require.Equal(t, 0, code,
			"approve: %d %s%s", code, out, errs)
	}

	record := readFile(t, filepath.Join(ta.Root, task, "APPROVAL"))
	assert.False(t, !strings.Contains(record, "approved_by Jane Doe") || strings.Count(record, "\n") < 5,
		"the approval record: %q", record)
	{
		code, out, errs := runTaskScript(t, ta.Root, "preflight.sh", task)
		require.False(t, code != 0 || !strings.Contains(out, "matches its recorded approval") || !strings.Contains(out, "PREFLIGHT: OK"),
			"an approved task must pass: %d\n%s%s", code, out, errs)
	}

	appendFile(t, filepath.Join(ta.Root, task, "SPEC.md"), "\nan edit after approval\n")
	{
		code, out, errs := runTaskScript(t, ta.Root, "preflight.sh", task)
		assert.False(t, code == 0 || !strings.Contains(out+errs, "has changed since it was approved"),
			"a changed spec must fail: %d\n%s%s", code, out, errs)
	}

	t.Setenv("WATTS_ROLE", "build")
	{
		code, _, errs := recordTaskApproval(t, ta.Root, task)
		assert.False(t, code == 0 || !strings.Contains(errs, "cannot be given from inside an agent"),
			"approval from inside an agent must be refused: %d %s", code, errs)
	}
}
func appendFile(t *testing.T, path, text string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	require.NoError(t, err)

	defer f.Close()
	{
		_, err := f.WriteString(text)
		require.NoError(t, err)
	}
}
func TestAStrayFileInTheProjectDoesNotMatter(t *testing.T) {
	needTools(t, "awk")
	ta, task := taskRepo(t)
	writeFile(t, filepath.Join(ta.Root, "stray.txt"), "dirty")
	writeFile(t, filepath.Join(ta.Root, ".watts/scratch.txt"), "x")
	{
		code, out, errs := runTaskScript(t, ta.Root, "preflight.sh", task)
		assert.False(t, code != 0 || !strings.Contains(out, "PREFLIGHT: OK"),
			"uncommitted or untracked files must not matter: %d\n%s%s", code, out, errs)
	}
}
func TestAgentNewAndWorkflowRun(t *testing.T) {
	needTools(t, "awk")
	ta, task := taskRepo(t)
	code, out, errs := runCLI(t, ta.Root, "agent", "new", "security-review")
	require.Equal(t, 0, code,
		"agent new: %d %s%s", code, out, errs)

	dir := filepath.Join(ta.Root, ".watts.agents", "security-review")
	for _, f := range []string{"README.md", "skills/example-skill/SKILL.md"} {
		assert.True(t, exists(filepath.Join(dir, f)),
			"the scaffold lacks %s", f)
	}
	assert.False(t, exists(filepath.Join(dir, "watts.example.json")),
		"the config example is a reference and must not be copied into the agent")
	assert.False(t, !exists(filepath.Join(dir, "settings.json")) || !exists(filepath.Join(dir, "skills", "example-skill", "SKILL.md")),
		"the agent directory must be configured")

	cfg := loadCfg(t, ta)
	assert.False(t, cfg.Agents["security-review"].Path != ".watts.agents/security-review",
		"the agent must be registered: %+v", cfg.Agents)
	{
		code, _, _ := runCLI(t, ta.Root, "agent", "new", "security-review")
		assert.NotEqual(t, 0, code,
			"creating the same agent twice must fail")
	}
	{
		code, out, _ := runCLI(t, ta.Root, "agent", "list")
		assert.False(t, code != 0 || !strings.Contains(out, "security-review"),
			"agent list: %d %s", code, out)
	}

	cfg.Workflow = &orchestration.Definition{Steps: []orchestration.Step{
		{Name: "mark", Command: "printf '%s' \"$WATTS_TASK\" > ran-command.txt"},
		{Name: "audit", Agent: "security-review", Provider: "ollama", Model: "qwen2.5-coder:7b", Prompt: "Audit {task} for secrets"},
		{Name: "flaky", Command: "false", Optional: true},
	}}
	saveCfg(t, ta, cfg)
	customPi(t, `echo "$*" > "$PI_CODING_AGENT_DIR/prompt.txt"`)
	task = "tasks/custom"

	require.NoError(t, os.MkdirAll(filepath.Join(ta.Root, task), 0755))

	require.NoError(t, storage.WriteJSON(ta.TaskWorkflowPath(task), cfg.Workflow))
	binding, err := ta.CreateWorkflowBinding(cfg, task, &application.RunOptions{})
	require.NoError(t, err)

	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterActivityWithOptions(ta.ExecuteWorkflowStage, activity.RegisterOptions{Name: orchestration.ExecuteActivity})
	env.ExecuteWorkflow(orchestration.Run, binding.Input)

	require.NoError(t, env.GetWorkflowError())

	var state orchestration.State

	require.NoError(t, env.GetWorkflowResult(&state))

	require.False(t, state.Status != "completed" || state.Stages[2].Status != "skipped",
		"custom workflow=%+v", state)
	assert.Equal(t, task, readFile(t, filepath.Join(ta.Root, "ran-command.txt")),
		"command did not receive WATTS_TASK")
	assert.Contains(t, readFile(t, filepath.Join(dir, "prompt.txt")), "Audit "+task+" for secrets",
		"agent prompt did not expand the task")
	{
		code, _, errs := runCLI(t, ta.Root, "task", "run", task, "--step", "nope")
		assert.False(t, code == 0 || !strings.Contains(errs, "flag provided but not defined"),
			"step bypass: %d %s", code, errs)
	}

	cfg.Workflow.Steps = []orchestration.Step{{Name: "boom", Command: "false"}}
	saveCfg(t, ta, cfg)
	{
		code, out, errs := runCLI(t, ta.Root, "task", "run", task, "--dry-run")
		assert.False(t, code != 0 || !strings.Contains(out, "audit") || strings.Contains(out, "boom"),
			"pinned workflow preview: %d %s%s", code, out, errs)
	}
}
func TestExampleAgentIsShippedAndDocumented(t *testing.T) {
	for _, f := range []string{"README.md", "watts.example.json", "skills/example-skill/SKILL.md"} {
		{
			_, err := fs.ReadFile(kit.Files, "kit/examples/custom-agent/"+f)
			assert.NoError(t, err,
				"the example agent lacks %s: %v", f, err)
		}
	}
	data, _ := fs.ReadFile(kit.Files, "kit/examples/custom-agent/watts.example.json")
	var c projectconfig.Config
	c = projectconfig.Default()
	var example struct {
		Agents   map[string]projectconfig.AgentConfig `json:"agents"`
		Workflow *orchestration.Definition            `json:"workflow"`
	}

	require.NoError(t, json.Unmarshal(data, &example))

	c.Agents, c.Workflow = example.Agents, example.Workflow
	{
		err := c.Validate()
		assert.NoError(t, err,
			"the example configuration must itself be valid: %v", err)
	}
}
func TestFollowReadsTheSessionRecord(t *testing.T) {
	ta := newProject(t)
	ta.initRepo(t)
	lines := []string{
		`{"type":"model_change","timestamp":"2026-10-05T16:14:09.000Z","model":"qwen3-coder:30b"}`,
		`{"type":"message","timestamp":"2026-10-05T16:14:10.323Z","message":{"role":"assistant","content":[{"type":"thinking","thinking":"hmm"},{"type":"text","text":"Reading   the\nspec"},{"type":"toolCall","name":"read","arguments":{"path":"SPEC.md"}}]}}`,
		`{"type":"message","timestamp":"2026-10-05T16:14:11.000Z","message":{"role":"toolResult","content":[{"type":"text","text":"file contents"}]}}`,
		`{"type":"unrelated"}`,
	}
	writeFile(t, filepath.Join(ta.AgentDir(loadCfg(t, ta), "build"), "sessions", "x", "s.jsonl"), strings.Join(lines, "\n")+"\n")
	code, out, errs := runCLI(t, ta.Root, "agent", "follow", "build", "--once")
	require.Equal(t, 0, code,
		"follow: %d %s", code, errs)

	for _, want := range []string{"model: qwen3-coder:30b", "assistant: Reading the spec", "call read", "SPEC.md", "result: file contents"} {
		assert.Contains(t, out, want,
			"the feed lacks %q:\n%s", want, out)
	}
	assert.False(t, strings.Contains(out, "hmm") || strings.Contains(out, "unrelated"),
		"thinking and unknown entries are left out:\n%s", out)
	{
		code, _, errs := runCLI(t, ta.Root, "agent", "follow", "review", "--once")
		assert.False(t, code == 0 || !strings.Contains(errs, "no session record"),
			"with no session there is nothing to follow: %d %s", code, errs)
	}
	assert.Equal(t, "", application.FormatEntry([]byte("not json")),
		"garbage lines are skipped")
}
