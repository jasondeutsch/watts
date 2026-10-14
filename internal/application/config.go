package application

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/storage"
	"github.com/jasondeutsch/watts/internal/workerstate"
)

// ServiceSettingsPath is local operational state, not portable package authoring.
func (app *Service) ServiceSettingsPath() (string, error) {
	store, err := workerstate.Default()
	if err != nil {
		return "", err
	}
	if err = store.CheckDirectory(filepath.Dir(store.SettingsPath())); err != nil {
		return "", err
	}
	return store.SettingsPath(), nil
}

func (app *Service) LoadServiceConfig() (projectconfig.Config, error) {
	cfg := projectconfig.Default()
	path, err := app.ServiceSettingsPath()
	if err != nil {
		return cfg, err
	}
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return cfg, fmt.Errorf("worker settings must be a regular file")
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return cfg, fmt.Errorf("service settings: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return cfg, fmt.Errorf("service settings require one JSON document")
	}
	if cfg.Workflow != nil {
		return cfg, errors.New("workflow authoring belongs in a package, not service settings")
	}
	return cfg, validateGlobalSettings(cfg)
}

// SaveConfig persists worker preferences without generating or rewriting packages.
func (app *Service) SaveConfig(cfg projectconfig.Config) error {
	if err := validateGlobalSettings(cfg); err != nil {
		return err
	}
	path, err := app.ServiceSettingsPath()
	if err != nil {
		return err
	}
	return storage.WriteJSONAtomically(path, cfg)
}

func validateGlobalSettings(cfg projectconfig.Config) error {
	if cfg.Temporal != nil && cfg.Temporal.TaskQueue != "" {
		return fmt.Errorf("global task_queue overrides are not supported; omit task_queue to keep project workers isolated")
	}
	return cfg.Validate()
}

// Refuse silent substitution of defaults for project preferences during setup.
// Inspection of pinned task evidence and global configuration remains available.
func (app *Service) CheckLegacySettings() error {
	if _, err := os.Lstat(filepath.Join(app.Root, ".watts", "settings.yaml")); err == nil {
		return fmt.Errorf("project-local .watts/settings.yaml is no longer used; stop the old worker, move supported preferences to ~/.watts/config/settings.json, and archive the old settings; task evidence is unchanged")
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}
