package application

import (
	"errors"
	"os"
	"path/filepath"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/jasondeutsch/watts/internal/storage"
)

// configureStageMCPs replaces ambient role servers for this activity only.
// The workspace lock serializes activities. Ralph children inherit this agent directory.
func (app *Service) configureStageMCPs(cfg projectconfig.Config, step orchestration.Step) (func() error, error) {
	path := filepath.Join(app.AgentDir(cfg, step.Agent), "mcp.json")
	previous, err := os.ReadFile(path)
	missing := errors.Is(err, os.ErrNotExist)
	if err != nil && !missing {
		return nil, err
	}
	servers := map[string]projectconfig.MCPServer{}
	for _, name := range step.MCPs {
		server := cfg.MCPs[name]
		if server.Exposure == "" {
			server.Exposure = "direct"
		}
		servers[name] = server
	}
	if err := storage.WriteJSONAtomically(path, struct {
		Servers map[string]projectconfig.MCPServer `json:"mcpServers"`
	}{servers}); err != nil {
		return nil, err
	}
	return func() error {
		if missing {
			err := os.Remove(path)
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		return os.WriteFile(path, previous, 0600)
	}, nil
}
