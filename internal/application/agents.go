package application

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/kit"
)

type CreateAgentOptions struct {
	Name string
	Path string
}

func (a *Service) CreateAgent(cfg projectconfig.Config, options CreateAgentOptions) (string, error) {
	name := options.Name
	var err error
	if _, exists := cfg.Agent(name); exists {
		return "", fmt.Errorf("an agent named %q already exists", name)
	}
	if !projectconfig.ReAgent.MatchString(name) {
		return "", fmt.Errorf("agent name %q must be lowercase letters, digits and dashes, starting with a letter", name)
	}
	target := options.Path
	if target == "" {
		target = ".watts.agents/" + name
	}
	if cfg.Agents == nil {
		cfg.Agents = map[string]projectconfig.AgentConfig{}
	}
	cfg.Agents[name] = projectconfig.AgentConfig{Path: target}
	if err := cfg.Validate(); err != nil {
		return "", err
	}
	abs := filepath.Join(a.Root, filepath.FromSlash(target))
	if _, err := os.Stat(abs); err == nil {
		return "", fmt.Errorf("%s already exists; pick another --path or remove it", target)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return "", err
	}
	const root = "kit/examples/custom-agent"
	err = fs.WalkDir(kit.Files, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		taskPath := strings.TrimPrefix(strings.TrimPrefix(p, root), "/")
		if taskPath == "" || taskPath == "watts.example.yaml" {
			return nil
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(abs, taskPath), 0o755)
		}
		data, err := fs.ReadFile(kit.Files, p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(abs, taskPath), data, 0o644)
	})
	if err != nil {
		return "", err
	}
	if err := a.SaveConfig(cfg); err != nil {
		return "", err
	}
	if err := a.ConfigAgents(cfg, false); err != nil {
		return "", err
	}
	return target, nil
}

// CleanAgents removes only generated/private agent state after the caller authorizes it.
func (a *Service) CleanAgents() ([]string, error) {
	var removed []string
	cfg, err := a.LoadConfig()
	names := projectconfig.BuiltinAgents
	if err == nil {
		names = cfg.AgentNames()
	}
	for _, role := range names {
		for _, d := range []string{a.AgentDir(cfg, role), a.HomeDir(role)} {
			if err := os.RemoveAll(d); err != nil {
				return nil, err
			}
			removed = append(removed, a.Rel(d))
		}
	}
	_ = os.Remove(a.Wd(".generated.json"))
	return removed, nil
}
