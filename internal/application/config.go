package application

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
	"gopkg.in/yaml.v3"
)

// ServiceSettingsPath is local operational state, not portable package authoring.
func (app *Service) ServiceSettingsPath() string { return app.Wd("settings.yaml") }

func (app *Service) LoadServiceConfig() (projectconfig.Config, error) {
	cfg := projectconfig.Default()
	data, err := os.ReadFile(app.ServiceSettingsPath())
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return cfg, fmt.Errorf("service settings: %w", err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		return cfg, fmt.Errorf("service settings require one YAML document")
	}
	if cfg.Workflow != nil {
		return cfg, errors.New("workflow authoring belongs in a package, not service settings")
	}
	return cfg, cfg.Validate()
}

// SaveConfig persists worker preferences without generating or rewriting packages.
func (app *Service) SaveConfig(cfg projectconfig.Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(app.Wd(), 0700); err != nil {
		return err
	}
	return os.WriteFile(app.ServiceSettingsPath(), data, 0600)
}
