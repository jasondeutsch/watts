package application

import (
	"os"
	"path/filepath"
	"testing"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/stretchr/testify/require"
)

func TestStageMCPSelectionRestoresInteractiveConfiguration(t *testing.T) {
	project := newProject(t)
	project.initRepo(t)
	cfg := loadCfg(t, project)
	cfg.MCPs = map[string]projectconfig.MCPServer{"gopls": {Command: "gopls", Args: []string{"mcp"}}, "other": {Command: "other"}}
	path := filepath.Join(project.AgentDir(cfg, "build"), "mcp.json")
	original := `{"mcpServers":{"personal":{"command":"personal"}}}`
	writeFile(t, path, original)
	restore, err := project.configureStageMCPs(cfg, orchestration.Step{Agent: "build", MCPs: []string{"gopls"}})
	require.NoError(t, err)
	contents := readFile(t, path)
	require.Contains(t, contents, `"gopls"`)
	require.Contains(t, contents, `"direct"`)
	require.NotContains(t, contents, "personal")
	require.NotContains(t, contents, "other")
	require.NoError(t, restore())
	require.Equal(t, original, readFile(t, path))
	restore, err = project.configureStageMCPs(cfg, orchestration.Step{Agent: "build"})
	require.NoError(t, err)
	require.Contains(t, readFile(t, path), `"mcpServers": {}`)
	require.NoError(t, restore())
	require.NoError(t, os.Remove(path))
	restore, err = project.configureStageMCPs(cfg, orchestration.Step{Agent: "build"})
	require.NoError(t, err)
	require.NoError(t, restore())
	_, err = os.Stat(path)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestWorkflowRejectsUnknownAndNonAgentMCPs(t *testing.T) {
	cfg := projectconfig.Default()
	definition := orchestration.Definition{Steps: []orchestration.Step{{Name: "build", Agent: "build", Prompt: "build", Provider: "ollama", Model: "example", MCPs: []string{"missing"}}}}
	require.ErrorContains(t, cfg.ValidateWorkflow(definition), "undefined mcp")
	definition.Steps[0] = orchestration.Step{Name: "lint", Command: "true", MCPs: []string{"gopls"}}
	require.ErrorContains(t, definition.Validate(), "require an agent")
}
