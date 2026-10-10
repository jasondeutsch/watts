package application

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/manifest"
)

func (a *Service) ConfigPath() string { return filepath.Join(a.Root, projectconfig.Filename) }

// LoadConfig reads the project's watts.yaml and rejects unknown settings.
func (a *Service) LoadConfig() (projectconfig.Config, error) {
	data, err := os.ReadFile(a.ConfigPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return projectconfig.Config{}, errors.New("watts is not set up here (no watts.yaml found). Run: watts init in your project root")
		}
		return projectconfig.Config{}, err
	}
	cfg := projectconfig.Default()
	// Defaults apply when a section is omitted; explicitly supplied maps replace it.
	// Decoding directly into default maps would silently add agents or connections.
	sections, err := manifest.Decode(data, manifest.Project, map[string]any{})
	if err != nil {
		return projectconfig.Config{}, fmt.Errorf("%s: %w", a.Rel(a.ConfigPath()), err)
	}
	if sections.Spec == nil {
		return projectconfig.Config{}, fmt.Errorf("%s: manifest requires a spec mapping", a.Rel(a.ConfigPath()))
	}
	if _, provided := sections.Spec["providers"]; provided {
		cfg.Providers = nil
	}
	if _, provided := sections.Spec["agents"]; provided {
		cfg.Agents = nil
	}

	document, err := manifest.Decode(data, manifest.Project, cfg)
	if err != nil {
		return projectconfig.Config{}, fmt.Errorf("%s: %w", a.Rel(a.ConfigPath()), err)
	}
	cfg = document.Spec
	if err := cfg.Validate(); err != nil {
		return projectconfig.Config{}, fmt.Errorf("%s: %w", a.Rel(a.ConfigPath()), err)
	}
	selected, err := a.ResolveWorkflow(cfg, "")
	if err != nil {
		return projectconfig.Config{}, err
	}
	return cfg.WithWorkflowAgents(selected.Definition), nil
}

func (a *Service) SaveConfig(cfg projectconfig.Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if cfg.EnvPassthrough == nil {
		cfg.EnvPassthrough = []string{}
	}
	name := filepath.Base(a.Root)
	if data, err := os.ReadFile(a.ConfigPath()); err == nil {
		if document, err := manifest.Decode(data, manifest.Project, map[string]any{}); err == nil {
			name = document.Name
		}
	}
	if err := manifest.Write(a.ConfigPath(), manifest.Project, name, cfg); err != nil {
		return err
	}
	return nil
}
