package cli

import (
	"github.com/jasondeutsch/watts/internal/storage"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jasondeutsch/watts/cli/internal/setup"
	projectconfig "github.com/jasondeutsch/watts/internal/config"
)

func TestInitFlagsPersistAndRerunKeepsConfig(t *testing.T) {
	ta := newProject(t)
	ta.initRepo(t, "--env-passthrough", "NODE_EXTRA_CA_CERTS,HTTPS_PROXY")
	cfg, err := ta.LoadConfig()
	require.NoError(t, err)
	require.False(t, len(cfg.EnvPassthrough) != 2,
		"flags not saved: %+v", cfg)

	ta.reset()
	ta.initRepo(t)
	cfg2, _ := ta.LoadConfig()
	require.False(t, len(cfg2.EnvPassthrough) != 2,
		"init without flags must keep the saved config: %+v", cfg2)

	other := newProject(t)
	{
		err := setup.Init(other.Context, []string{"--no-install", "--thinking", "extreme"})
		require.Error(t, err,
			"an invalid thinking level must be rejected")
	}
	require.False(t, exists(other.Wd()),
		"nothing may be written when the flags are invalid")
}
func TestDoctor(t *testing.T) {
	needTools(t, "timeout", "awk")
	ta := newProject(t)
	ta.initRepo(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer srv.Close()
	cfg, _ := ta.LoadConfig()
	cfg.Providers["ollama"] = projectconfig.Provider{BaseURL: srv.URL + "/v1"}

	require.NoError(t, ta.SaveConfig(cfg))

	stubBin(t, "pi 1.0.1")

	ta.reset()
	err := ta.Doctor(cfg, true)
	require.False(t, err == nil || !strings.Contains(ta.Output.String(), "watts install ralph"),
		"expected a failure naming install-ralph:\n%s", ta.Output)
	assert.Contains(t, ta.Output.String(), "Ollama reachable",
		"the Ollama check should pass against the test server:\n%s", ta.Output)

	require.NoError(t, ta.InstallRalph(cfg, nil))

	ta.reset()
	{
		err := ta.Doctor(cfg, true)
		require.NoError(t, err,
			"doctor should pass:\n%s", ta.Output)
	}
	assert.Contains(t, ta.Output.String(), "DOCTOR: OK",
		"missing OK line:\n%s", ta.Output)

	stubBin(t, "pi 0.99.2")
	ta.reset()
	{
		err := ta.Doctor(cfg, true)
		require.False(t, err == nil || !strings.Contains(ta.Output.String(), "too old"),
			"an old Pi must fail:\n%s", ta.Output)
	}

	stubBin(t, "pi 1.0.1")

	cfg.Providers["ollama"] = projectconfig.Provider{BaseURL: "http://127.0.0.1:1/v1"}
	ta.reset()
	_ = ta.Doctor(cfg, true)
	assert.Contains(t, ta.Output.String(), "warn:  Ollama not reachable",
		"expected an Ollama warning:\n%s", ta.Output)

	ta.reset()
	{
		err := ta.Doctor(projectconfig.Config{}, false)
		require.False(t, err == nil || !strings.Contains(ta.Output.String(), "watts init"),
			"a missing config must fail and name watts init:\n%s", ta.Output)
	}
}

func TestInitTargetsCurrentDirectoryInsideParentProject(t *testing.T) {
	for _, marker := range []string{projectconfig.Filename, ".watts"} {
		t.Run(marker, func(t *testing.T) {
			parent := t.TempDir()
			if marker == projectconfig.Filename {
				cfg := projectconfig.Default()
				cfg.Agents["build"] = projectconfig.AgentConfig{Thinking: "high"}
				require.NoError(t, storage.WriteJSON(filepath.Join(parent, marker), cfg))
			} else {
				require.NoError(t, os.Mkdir(filepath.Join(parent, marker), 0755))
				writeFile(t, filepath.Join(parent, marker, "sentinel"), "parent assets")
			}
			child := filepath.Join(parent, "child")
			require.NoError(t, os.Mkdir(child, 0755))
			parentConfig, _ := os.ReadFile(filepath.Join(parent, projectconfig.Filename))
			code, out, errors := runCLI(t, child, "init", "--no-install")
			require.Equal(t, 0, code, out+errors)
			require.FileExists(t, filepath.Join(child, projectconfig.Filename))
			require.FileExists(t, filepath.Join(child, ".watts", "kit", "scripts", "lib.sh"))
			require.FileExists(t, filepath.Join(child, ".watts", "pi-agent-build", "settings.json"))
			if marker == projectconfig.Filename {
				require.Equal(t, string(parentConfig), readFile(t, filepath.Join(parent, projectconfig.Filename)))
				require.NoDirExists(t, filepath.Join(parent, ".watts"))
			} else {
				require.NoFileExists(t, filepath.Join(parent, projectconfig.Filename))
				require.Equal(t, "parent assets", readFile(t, filepath.Join(parent, ".watts", "sentinel")))
				require.NoDirExists(t, filepath.Join(parent, ".watts", "kit"))
			}
			// Existing-project commands continue to discover the child's root from below it.
			nested := filepath.Join(child, "src")
			require.NoError(t, os.Mkdir(nested, 0755))
			code, out, errors = runCLI(t, nested, "config", "show")
			require.Equal(t, 0, code, errors)
			require.Contains(t, out, `"thinking": "medium"`)
			code, out, errors = runCLI(t, child, "task", "new", "project-root-check")
			require.Equal(t, 0, code, out+errors)
			tasks, err := os.ReadDir(filepath.Join(child, "tasks"))
			require.NoError(t, err)
			require.Len(t, tasks, 1)
			require.FileExists(t, filepath.Join(child, "tasks", tasks[0].Name(), "workflow.json"))
			require.NoDirExists(t, filepath.Join(parent, "tasks"))
		})
	}
}
