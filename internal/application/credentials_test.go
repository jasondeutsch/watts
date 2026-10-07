package application

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
)

func TestProviderGeneratesModelsAndPassesItsKey(t *testing.T) {
	ta := newProject(t)
	ta.initRepo(t)
	cfg := gatewayConfig(ta, t, "https://gw.example/v1")
	saveCfg(t, ta, cfg)

	require.NoError(t, ta.ConfigAgents(cfg, false))

	models := readFile(t, filepath.Join(ta.AgentDir(loadCfg(t, ta), "build"), "models.json"))
	for _, want := range []string{`"baseUrl": "https://gw.example/v1"`, `"apiKey": "$GW_KEY"`, `"authHeader": true`, `"contextWindow": 200000`, `"reasoning": true`} {
		assert.Contains(t, models, want,
			"models.json lacks %s:\n%s", want, models)
	}
	assert.Contains(t, models, "$GW_KEY",
		"the key must be an environment variable reference")

	t.Setenv("GW_KEY", "the-key-value")
	env, err := ta.PiEnv(cfg, "build", nil, false)
	require.NoError(t, err)
	assert.True(t, projectconfig.Contains(env, "GW_KEY=the-key-value"),
		"declaring the provider must be enough to pass its key variable into the agent")
	assert.Len(t, cfg.EnvPassthrough, 0,
		"the passthrough list should not need the provider's variable")

	t.Setenv("GW_KEY", "")
	os.Unsetenv("GW_KEY")
	env, _ = ta.PiEnv(cfg, "build", nil, false)
	for _, e := range env {
		assert.False(t, strings.HasPrefix(e, "GW_KEY="),
			"an unset variable must not appear")
	}
}
func TestAPIKeyCommandKeepsTheKeyOffDisk(t *testing.T) {
	ta := newProject(t)
	ta.initRepo(t)
	cfg := gatewayConfig(ta, t, "https://gw.example/v1")
	p := cfg.Providers["gw"]
	p.APIKeyEnv, p.APIKeyCommand = "", []string{"sh", "-c", "printf '%s%s' s3cr3t -token-123"}
	cfg.Providers["gw"] = p
	saveCfg(t, ta, cfg)

	require.NoError(t, ta.ConfigAgents(cfg, false))

	env, err := ta.PiEnv(cfg, "build", nil, false)
	require.False(t, err != nil || !projectconfig.Contains(env, "WATTS_APIKEY_GW=s3cr3t-token-123"),
		"the minted key must reach the agent in memory: %v %v", err, env)
	assert.Contains(t, readFile(t, filepath.Join(ta.AgentDir(loadCfg(t, ta), "build"), "models.json")), `"apiKey": "$WATTS_APIKEY_GW"`,
		"models.json must name the variable Watts fills")

	_ = filepath.WalkDir(ta.Root, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && !strings.Contains(path, "/.git/") {
			{
				data, _ := os.ReadFile(path)
				assert.NotContains(t, string(data), "s3cr3t-token-123",
					"the key was written to disk in %s", path)
			}
		}
		return nil
	})
	entries, _ := ta.EnvReport(cfg, "build", nil, false)
	var shown bool
	for _, e := range entries {
		shown = shown || (e.Name == "WATTS_APIKEY_GW" && strings.Contains(e.Source, "api_key_command"))
	}
	assert.True(t, shown,
		"the environment report must list the generated variable by name")

	p.APIKeyCommand = []string{"false"}
	cfg.Providers["gw"] = p
	{
		_, err := ta.PiEnv(cfg, "build", nil, false)
		assert.False(t, err == nil || !strings.Contains(err.Error(), "api_key_command"),
			"a failing key command must be reported: %v", err)
	}
}
func TestCredentialCheck(t *testing.T) {
	ta := newProject(t)
	ta.initRepo(t, "--provider", "example-gateway", "--build-model", "example/a", "--review-model", "example/b")
	cfg := loadCfg(t, ta)
	write := func(key string) {
		writeFile(t, filepath.Join(ta.AgentDir(loadCfg(t, ta), "build"), "models.json"),
			`{"providers":{"example-gateway":{"baseUrl":"https://x/v1","api":"openai-completions","apiKey":"`+key+`","models":[{"id":"example/a"},{"id":"example/b"}]}}}`)
	}
	levels := func(fs []Finding) string {
		var out []string
		for _, f := range fs {
			out = append(out, f.Level)
		}
		return strings.Join(out, ",")
	}
	os.Unsetenv("EXAMPLE_GATEWAY_API_KEY")

	write("$EXAMPLE_GATEWAY_API_KEY")
	fs := ta.CredentialCheck(cfg, "build", nil)
	assert.False(t, !HasFail(fs) || !strings.Contains(fs[0].Message, "EXAMPLE_GATEWAY_API_KEY"),
		"the $ form with the variable missing must be reported: %+v", fs)

	write("EXAMPLE_GATEWAY_API_KEY")
	fs = ta.CredentialCheck(cfg, "build", nil)
	var gotPass, gotSet bool
	for _, f := range fs {
		gotPass = gotPass || strings.Contains(f.Message, "is not in env_passthrough, so pi will not see it")
		gotSet = gotSet || strings.Contains(f.Message, "is not set in your shell")
	}
	assert.False(t, !gotPass || !gotSet,
		"both causes must be named: %+v", fs)
	{
		err := ta.RequireCredentials(cfg, "build", nil)
		assert.False(t, err == nil || !strings.Contains(ta.Output.String(), "watts config env add EXAMPLE_GATEWAY_API_KEY"),
			"a run must be refused with the fix named: %v\n%s", err, ta.Output)
	}

	cfg.EnvPassthrough = []string{"EXAMPLE_GATEWAY_API_KEY"}
	t.Setenv("EXAMPLE_GATEWAY_API_KEY", "k")
	{
		fs = ta.CredentialCheck(cfg, "build", nil)
		assert.Len(t, fs, 0,
			"a passed-through, set variable is fine: %+v", fs)
	}

	cfg.EnvPassthrough = nil
	{
		fs = ta.CredentialCheck(cfg, "build", []string{"EXAMPLE_GATEWAY_API_KEY"})
		assert.Len(t, fs, 0,
			"--env-passthrough must satisfy the check: %+v", fs)
	}

	literal := "abcdefghijklmnopqrstuvwxyz0123456789"
	write(literal)
	fs = ta.CredentialCheck(cfg, "build", nil)
	assert.False(t, levels(fs) != "warn" || strings.Contains(fs[0].Message, literal),
		"a literal key is a warning and is never printed: %+v", fs)

	write("EXAMPLE_GATEWAY_API_KEY")
	cfg.EnvPassthrough = []string{"EXAMPLE_GATEWAY_API_KEY"}
	setWorkflowModels(&cfg, "", "example/missing", "")
	{
		fs = ta.CredentialCheck(cfg, "build", nil)
		assert.False(t, !HasFail(fs) || !strings.Contains(fs[0].Message, "does not list"),
			"an unlisted model must fail: %+v", fs)
	}
}
