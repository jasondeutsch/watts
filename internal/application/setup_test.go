package application

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/kit"
	"github.com/jasondeutsch/watts/internal/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalIgnoresStayInsideWatts(t *testing.T) {
	ta := newProject(t)
	gi := filepath.Join(ta.Root, ".gitignore")
	writeFile(t, gi, "node_modules/\nbin")

	require.NoError(t, ta.WriteLocalIgnores())

	{
		body := readFile(t, ta.Wd(".gitignore"))
		assert.False(t, !strings.Contains(body, "pi-agent-*/") || !strings.Contains(body, "home-*/"),
			".watts/.gitignore must ignore the local agent directories: %q", body)
	}
	assert.Equal(t, "node_modules/\nbin", readFile(t, gi),
		"the project's own .gitignore must never be touched")

	require.NoError(t, ta.WriteLocalIgnores())
}
func TestConfigAgents(t *testing.T) {
	ta := newProject(t)
	ta.initRepo(t)
	cfg, err := ta.LoadConfig()
	require.NoError(t, err)

	for _, role := range projectconfig.BuiltinAgents {
		agent, home := ta.AgentDir(loadCfg(t, ta), role), ta.HomeDir(role)
		var s map[string]any

		require.NoError(t, json.Unmarshal([]byte(readFile(t, filepath.Join(agent, "settings.json"))), &s))

		assert.False(t, s["defaultProvider"] != "openrouter" || s["defaultProjectTrust"] != "never" || s["enableInstallTelemetry"] != false,
			"%s settings wrong: %v", role, s)

		resolved, _ := cfg.Agent(role)
		wantModel := resolved.Model
		assert.Equal(t, wantModel, s["defaultModel"],
			"%s role must use %s, got %v", role, wantModel, s["defaultModel"])

		var m ModelsFile

		require.NoError(t, json.Unmarshal([]byte(readFile(t, filepath.Join(agent, "models.json"))), &m))

		p := m.Providers["openrouter"]
		assert.False(t, p.BaseURL != cfg.Providers["openrouter"].BaseURL || p.API != "openai-completions" || p.APIKey == "" || len(p.Models) != 1,
			"models.json for %s wrong: %+v", role, p)
		assert.False(t, exists(filepath.Join(home, ".gitconfig")),
			"%s must not have a generated Git configuration", role)

		for _, skill := range kit.EmbeddedSkillNames() {
			assert.True(t, exists(filepath.Join(agent, "skills", skill, "SKILL.md")),
				"%s agent lacks skill %s", role, skill)
		}
	}
}

func TestAgentThinkingAndSharedProviderConnection(t *testing.T) {
	app := newProject(t)
	app.initRepo(t)
	cfg, err := app.LoadConfig()
	require.NoError(t, err)
	cfg.Agents["build"] = projectconfig.AgentConfig{Thinking: "low"}
	cfg.Agents["review"] = projectconfig.AgentConfig{Thinking: "high"}
	setWorkflowModels(&cfg, "ollama", "local:7b", "local:7b")
	cfg.Providers["ollama"] = projectconfig.Provider{BaseURL: "http://localhost:1234/v1", APIKeyEnv: "LOCAL_MODEL_KEY"}
	require.NoError(t, app.SaveConfig(cfg))
	require.NoError(t, app.ConfigAgents(cfg, true))
	for role, thinking := range map[string]string{"build": "low", "review": "high"} {
		var settings map[string]any
		require.NoError(t, json.Unmarshal([]byte(readFile(t, filepath.Join(app.AgentDir(cfg, role), "settings.json"))), &settings))
		require.Equal(t, thinking, settings["defaultThinkingLevel"])
		var models ModelsFile
		require.NoError(t, json.Unmarshal([]byte(readFile(t, filepath.Join(app.AgentDir(cfg, role), "models.json"))), &models))
		require.Equal(t, "http://localhost:1234/v1", models.Providers["ollama"].BaseURL)
		require.Equal(t, "$LOCAL_MODEL_KEY", models.Providers["ollama"].APIKey)
		require.NotEmpty(t, models.Providers["ollama"].Models)
	}
	saved, err := manifest.Decode([]byte(readFile(t, app.ConfigPath())), manifest.Project, map[string]any{})
	require.NoError(t, err)
	for _, key := range []string{"thinking", "ollama_url", "pi_range", "ralph_version"} {
		require.NotContains(t, saved.Spec, key)
	}
}
func TestConfigAgentsPreservesUserWork(t *testing.T) {
	ta := newProject(t)
	ta.initRepo(t)
	cfg, _ := ta.LoadConfig()
	agent := ta.AgentDir(loadCfg(t, ta), "build")

	writeFile(t, filepath.Join(agent, "settings.json"), `{"packages":["npm:@lnilluv/pi-ralph-loop@2.1.0"],"theme":"dark","defaultModel":"stale"}`)

	require.NoError(t, ta.ConfigAgents(cfg, false))

	var s map[string]any
	_ = json.Unmarshal([]byte(readFile(t, filepath.Join(agent, "settings.json"))), &s)
	assert.False(t, s["theme"] != "dark" || s["packages"] == nil,
		"settings merge lost your keys: %v", s)
	resolved, _ := cfg.Agent("build")
	assert.Equal(t, resolved.Model, s["defaultModel"],
		"managed keys must be refreshed, got %v", s["defaultModel"])

	mine := `{"providers":{"ollama":{"baseUrl":"http://my-box:11434/v1","api":"openai-completions","apiKey":"x","models":[{"id":"my-model"}]}}}`
	writeFile(t, filepath.Join(agent, "models.json"), mine)
	ta.reset()

	require.NoError(t, ta.ConfigAgents(cfg, false))

	require.Equal(t, mine, readFile(t, filepath.Join(agent, "models.json")),
		"watts config overwrote a models.json the user wrote")
	assert.Contains(t, ta.ErrorOutput.String(), "edited by you",
		"expected a warning, got %q", ta.ErrorOutput.String())

	require.NoError(t, ta.ConfigAgents(cfg, true))

	require.NotContains(t, readFile(t, filepath.Join(agent, "models.json")), "my-model",
		"--force must regenerate models.json")

	setWorkflowModels(&cfg, "ollama", "another:7b", "local:7b")

	require.NoError(t, ta.ConfigAgents(cfg, false))

	require.Contains(t, readFile(t, filepath.Join(agent, "models.json")), "another:7b",
		"an unedited generated models.json must follow config changes")

	writeFile(t, filepath.Join(agent, "settings.json"), "{broken")
	{
		err := ta.ConfigAgents(cfg, false)
		require.False(t, err == nil || !strings.Contains(err.Error(), "settings.json"),
			"expected an invalid JSON error, got %v", err)
	}
}
func TestConfigAgentsHostedProviderWritesNoModelsFile(t *testing.T) {
	ta := newProject(t)
	ta.initRepo(t, "--provider", "anthropic", "--build-model", "m1", "--review-model", "m2")
	for _, role := range projectconfig.BuiltinAgents {
		assert.False(t, exists(filepath.Join(ta.AgentDir(loadCfg(t, ta), role), "models.json")),
			"%s: a hosted provider needs no models.json", role)
	}
	var s map[string]any
	_ = json.Unmarshal([]byte(readFile(t, filepath.Join(ta.AgentDir(loadCfg(t, ta), "review"), "settings.json"))), &s)
	assert.False(t, s["defaultProvider"] != "anthropic" || s["defaultModel"] != "m2",
		"review settings wrong: %v", s)
}
func TestResolveTask(t *testing.T) {
	ta := newProject(t)
	writeFile(t, filepath.Join(ta.Root, "tasks", "t1", "BASE"), "x")
	sub := filepath.Join(ta.Root, "sub")

	require.NoError(t, os.MkdirAll(sub, 0o755))

	outside := t.TempDir()

	for _, c := range []struct{ cwd, arg string }{
		{ta.Root, "tasks/t1"}, {ta.Root, "./tasks/t1/"}, {ta.Root, filepath.Join(ta.Root, "tasks", "t1")},
		{sub, "../tasks/t1"}, {sub, "tasks/t1"},
	} {
		ta.WorkingDirectory = c.cwd
		got, err := ta.ResolveTask(c.arg)
		assert.False(t, err != nil || got != "tasks/t1",
			"resolveTask(%q) from %s = %q, %v", c.arg, c.cwd, got, err)
	}
	ta.WorkingDirectory = ta.Root
	for _, arg := range []string{"", "tasks/missing", outside, "..", ".", "../elsewhere"} {
		{
			got, err := ta.ResolveTask(arg)
			assert.Error(t, err,
				"resolveTask(%q) = %q, expected an error", arg, got)
		}
	}
}
func TestPiEnvIsolation(t *testing.T) {
	ta := newProject(t)
	ta.initRepo(t)
	cfg, _ := ta.LoadConfig()
	t.Setenv("SECRET_TEST_KEY", "leak-me")
	t.Setenv("ANTHROPIC_API_KEY", "sk-test")
	t.Setenv("GH_TOKEN", "ghp_x")
	t.Setenv("TERM", "")
	t.Setenv("LANG", "")
	t.Setenv("GIT_AUTHOR_NAME", "must-not-inherit")

	env, err := ta.PiEnv(cfg, "build", nil, false)
	require.NoError(t, err)

	joined := strings.Join(env, "\n")
	assert.NotContains(t, joined, "GIT_",
		"Watts must not supply Git variables: %s", joined)

	for _, leak := range []string{"SECRET_TEST_KEY", "ANTHROPIC_API_KEY", "GH_TOKEN", "leak-me", "sk-test"} {
		assert.NotContains(t, joined, leak,
			"environment leaked %s", leak)
	}
	want := []string{
		"HOME=" + ta.HomeDir("build"), "PI_CODING_AGENT_DIR=" + ta.AgentDir(loadCfg(t, ta), "build"),
		"PI_SKIP_VERSION_CHECK=1", "PI_TELEMETRY=0", "TERM=xterm-256color", "LANG=en_US.UTF-8",
	}
	for _, w := range want {
		assert.True(t, projectconfig.Contains(env, w),
			"missing %s in %v", w, env)
	}
	assert.False(t, projectconfig.Contains(env, "PI_OFFLINE=1"),
		"PI_OFFLINE must be off by default")
	assert.Len(t, env, 9,
		"expected exactly 9 variables, got %d: %v", len(env), env)
	assert.True(t, projectconfig.Contains(env, "WATTS_ROLE=build"),
		"the agent must be told its own name in WATTS_ROLE")

	cfg.EnvPassthrough = []string{"ANTHROPIC_API_KEY", "NOT_SET_ANYWHERE"}
	env, _ = ta.PiEnv(cfg, "review", []string{"GH_TOKEN", "ANTHROPIC_API_KEY"}, true)
	joined = strings.Join(env, "\n")
	assert.False(t, !projectconfig.Contains(env, "ANTHROPIC_API_KEY=sk-test") || !projectconfig.Contains(env, "GH_TOKEN=ghp_x"),
		"passed variables missing: %v", env)
	assert.False(t, strings.Contains(joined, "NOT_SET_ANYWHERE") || strings.Contains(joined, "SECRET_TEST_KEY"),
		"unset or unnamed variables must not appear: %v", env)
	assert.False(t, !projectconfig.Contains(env, "PI_OFFLINE=1") || !projectconfig.Contains(env, "WATTS_ROLE=review"),
		"offline flag or reviewer identity missing: %v", env)

	count := 0
	for _, e := range env {
		if strings.HasPrefix(e, "ANTHROPIC_API_KEY=") {
			count++
		}
	}
	assert.Equal(t, 1, count,
		"a name listed twice must appear once, got %d", count)

	for _, bad := range [][]string{{"HOME"}, {"PI_OFFLINE"}, {"A B"}, {"x;y"}} {
		{
			_, err := ta.PiEnv(cfg, "build", bad, false)
			assert.Error(t, err,
				"piEnv accepted %v", bad)
		}
	}
	{
		_, err := ta.PiEnv(cfg, "admin", nil, false)
		assert.Error(t, err,
			"an unknown role must be rejected")
	}
}
func TestIsPrintMode(t *testing.T) {
	for args, want := range map[string]bool{"-p x": true, "--print x": true, "list": false, "install npm:x": false, "": false} {
		{
			got := IsPrintMode(strings.Fields(args))
			assert.Equal(t, want, got,
				"isPrintMode(%q) = %v", args, got)
		}
	}
}
func TestRunPiWithStub(t *testing.T) {
	needTools(t, "timeout")
	ta := newProject(t)
	ta.initRepo(t)
	cfg, _ := ta.LoadConfig()
	logFile := stubBin(t, "pi 1.0.1")
	t.Setenv("SECRET_TEST_KEY", "leak-me")
	ta.Input = strings.NewReader("pending input")

	require.NoError(t, ta.RunPi(cfg, "build", nil, false, "-p", "/ralph-list"))

	require.NoError(t, ta.RunPi(cfg, "review", nil, false, "--interactive-check"))

	log := readFile(t, logFile)
	runs := strings.Split(strings.TrimSuffix(strings.TrimSpace(log), "---"), "---\n")
	require.Len(t, runs, 2,
		"expected 2 runs, got %d:\n%s", len(runs), log)
	assert.False(t, !strings.Contains(runs[0], "ARGS: -p /ralph-list") || !strings.Contains(runs[0], "STDIN_BYTES: 0"),
		"print mode must get an empty stdin:\n%s", runs[0])
	assert.Contains(t, runs[1], "STDIN_BYTES: 13",
		"interactive mode must keep stdin:\n%s", runs[1])
	assert.False(t, !strings.Contains(runs[0], "WATTS_ROLE=build") || !strings.Contains(runs[1], "WATTS_ROLE=review"),
		"each role must run under its own identity")
	assert.Contains(t, runs[0], "PI_CODING_AGENT_DIR="+ta.AgentDir(loadCfg(t, ta), "build"),
		"the private agent directory must be selected")
	assert.False(t, strings.Contains(log, "SECRET_TEST_KEY") || strings.Contains(log, "leak-me"),
		"the secret reached the Pi process")
	assert.Contains(t, runs[0], "PWD: "+ta.Root,
		"Pi must run in the repository root")

	require.NoError(t, os.RemoveAll(ta.AgentDir(loadCfg(t, ta), "build")))

	{
		err := ta.RunPi(cfg, "build", nil, false, "-p", "x")
		require.False(t, err == nil || !strings.Contains(err.Error(), "watts config apply"),
			"expected a pointer to watts config, got %v", err)
	}
}
