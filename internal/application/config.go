package application

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/storage"
)

func (a *Service) ConfigPath() string { return filepath.Join(a.Root, projectconfig.Filename) }

// LoadConfig reads the project's watts.json and rejects unknown settings.
func (a *Service) LoadConfig() (projectconfig.Config, error) {
	data, err := os.ReadFile(a.ConfigPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return projectconfig.Config{}, errors.New("watts is not set up here (no watts.json found). Run: watts init in your project root")
		}
		return projectconfig.Config{}, err
	}
	cfg := projectconfig.Default()
	// Defaults apply when a section is omitted; explicitly supplied maps replace it.
	// Decoding directly into default maps would silently add agents or connections.
	var sections map[string]json.RawMessage
	if err := json.Unmarshal(data, &sections); err != nil {
		return projectconfig.Config{}, fmt.Errorf("%s: %w", a.Rel(a.ConfigPath()), err)
	}
	if _, provided := sections["providers"]; provided {
		cfg.Providers = nil
	}
	if _, provided := sections["agents"]; provided {
		cfg.Agents = nil
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return projectconfig.Config{}, fmt.Errorf("%s: %w", a.Rel(a.ConfigPath()), err)
	}
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
	if err := storage.WriteJSON(a.ConfigPath(), cfg); err != nil {
		return err
	}
	return nil
}
