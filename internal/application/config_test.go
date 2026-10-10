package application

import (
	"path/filepath"
	"strings"
	"testing"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfigErrors(t *testing.T) {
	ta := newProject(t)
	{
		_, err := ta.LoadConfig()
		require.False(t, err == nil || !strings.Contains(err.Error(), "watts init"),
			"missing config should point at watts init, got %v", err)
	}

	writeFile(t, ta.ConfigPath(), `kind: Project
schema_version: 1
name: test
spec:
  version: 1
  provder: ollama
`)
	{
		_, err := ta.LoadConfig()
		require.False(t, err == nil || !strings.Contains(err.Error(), "provder"),
			"a typo in a key must be rejected, got %v", err)
	}

	writeFile(t, ta.ConfigPath(), `kind: Project
schema_version: 1
name: test
spec:
  version: 1
  thinking: extreme
`)
	{
		_, err := ta.LoadConfig()
		require.False(t, err == nil || !strings.Contains(err.Error(), "thinking"),
			"an invalid value must be rejected, got %v", err)
	}

	writeFile(t, ta.ConfigPath(), `kind: Project
schema_version: 1
name: test
spec:
  version: 2
`)
	{
		_, err := ta.LoadConfig()
		require.Error(t, err,
			"an unknown config version must be rejected")
	}

	writeFile(t, ta.ConfigPath(), `not json`)
	{
		_, err := ta.LoadConfig()
		require.Error(t, err,
			"invalid YAML must be rejected")
	}
}
func TestConfigRoundTripAndPartialFile(t *testing.T) {
	ta := newProject(t)
	cfg := projectconfig.Default()
	setWorkflowModels(&cfg, "anthropic", "", "")
	cfg.EnvPassthrough = []string{"ANTHROPIC_API_KEY"}

	require.NoError(t, ta.SaveConfig(cfg))

	got, err := ta.LoadConfig()
	require.NoError(t, err)
	require.False(t, got.Workflow.Steps[0].Provider != "anthropic" || len(got.EnvPassthrough) != 1 || got.EnvPassthrough[0] != "ANTHROPIC_API_KEY",
		"round trip lost data: %+v", got)

	writeFile(t, ta.ConfigPath(), `kind: Project
schema_version: 1
name: test
spec:
  version: 1
`)
	got, err = ta.LoadConfig()
	require.NoError(t, err)
	require.False(t, got.Agents["build"].Thinking != "medium",
		"defaults not applied: %+v", got)
}
func TestVersionHelpers(t *testing.T) {
	ge := []struct {
		a, b string
		want bool
	}{
		{"22.22.1", "22.22.1", true}, {"22.22.2", "22.22.1", true}, {"23.0.0", "22.22.1", true},
		{"22.22.0", "22.22.1", false}, {"22.19.9", "22.22.1", false}, {"1.0.1", "1.0.0", true},
		{"0.99.2", "1.0.0", false}, {"10.0.0", "9.9.9", true}, {"1.0", "1.0.0", true},
	}
	for _, c := range ge {
		assert.Equal(t, c.want, VersionAtLeast(c.a, c.b),
			"verGE(%s, %s) != %v", c.a, c.b, c.want)
	}
	for in, want := range map[string]string{"v22.22.2\n": "22.22.2", "pi 1.0.1": "1.0.1", "1.0.1": "1.0.1", "none": ""} {
		{
			got := FindVersion(in)
			assert.Equal(t, want, got,
				"findVersion(%q) = %q, want %q", in, got, want)
		}
	}
}
func TestConfigurationRequiresProjectFile(t *testing.T) {
	ta := newProject(t)

	require.NoError(t, storage.WriteJSON(filepath.Join(ta.Root, ".watts", "config.json"), projectconfig.Default()))

	{
		_, err := ta.LoadConfig()
		require.Error(t, err,
			"configuration must come from the project watts.yaml")
	}
}
