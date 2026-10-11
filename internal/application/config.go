package application

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/manifest"
	"github.com/jasondeutsch/watts/internal/workflowpackage"
	"gopkg.in/yaml.v3"
)

func (app *Service) ConfigPath() string { return app.PackagePath() }

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
	if cfg.Workflow != nil || cfg.DefaultWorkflow != "" {
		return cfg, errors.New("workflow authoring belongs in a package, not service settings")
	}
	return cfg, cfg.Validate()
}

// LoadConfig derives runtime settings from an explicitly selected package. The
// descriptor remains the sole authoring entry point; runtime details stay local.
func (app *Service) LoadConfig() (projectconfig.Config, error) {
	cfg, err := app.LoadServiceConfig()
	if err != nil {
		return cfg, err
	}
	if _, err := os.Stat(app.PackagePath()); errors.Is(err, os.ErrNotExist) {
		return cfg, errors.New("no package descriptor found; run watts init or select an explicit --package path")
	} else if err != nil {
		return cfg, err
	}
	selected, err := app.ResolvePackage(cfg)
	if err != nil {
		return cfg, err
	}
	app.resolvedPackage = &selected
	app.packageDefinitionHash = definitionHash(selected.Definition)
	cfg.Workflow = &selected.Definition
	return cfg.WithWorkflowAgents(selected.Definition), nil
}

// SaveConfig writes local runtime preferences without adding fields to the
// descriptor. Explicit workflow edits through the application API become source.
func (app *Service) SaveConfig(cfg projectconfig.Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if (cfg.Workflow != nil && definitionHash(*cfg.Workflow) != app.packageDefinitionHash) || cfg.DefaultWorkflow != "" {
		if err := app.writeWorkflowSource(cfg, false); err != nil {
			return err
		}
	} else if err := app.InitializePackage(cfg); err != nil {
		return err
	}
	cfg.Workflow, cfg.DefaultWorkflow = nil, ""
	if cfg.EnvPassthrough == nil {
		cfg.EnvPassthrough = []string{}
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(app.Wd(), 0700); err != nil {
		return err
	}
	return os.WriteFile(app.ServiceSettingsPath(), data, 0600)
}

func (app *Service) writeWorkflowSource(cfg projectconfig.Config, createOnly bool) error {
	definition := projectconfig.DefaultWorkflow()
	if cfg.Workflow != nil {
		definition = *cfg.Workflow
	}
	if cfg.DefaultWorkflow != "" {
		selected, err := app.ResolveWorkflow(cfg, cfg.DefaultWorkflow)
		if err != nil {
			return err
		}
		definition = selected.Definition
	}
	name := strings.ToLower(filepath.Base(app.Root))
	name = regexp.MustCompile(`[^a-z0-9-]+`).ReplaceAllString(name, "-")
	if name == "" || name[0] < 'a' || name[0] > 'z' {
		name = "project-" + name
	}
	if existing, err := workflowpackage.ReadDescriptor(app.PackagePath()); err == nil {
		name = existing.Name
	}
	root := filepath.Dir(app.PackagePath())
	if err := os.MkdirAll(filepath.Join(root, "templates"), 0755); err != nil {
		return err
	}
	definition = cfg.WithResolvedWorkSettings(definition)
	data, err := manifest.EncodeWorkflow("sdlc", definition)
	if err != nil {
		return err
	}
	target := filepath.Join(root, "templates", "workflow.yaml")
	if err := os.WriteFile(target, data, 0644); err != nil {
		return err
	}
	descriptor := workflowpackage.Descriptor{Name: name, Version: "0.1.0", Templates: "templates"}
	data, err = yaml.Marshal(descriptor)
	if err != nil {
		return err
	}
	flags := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	if createOnly {
		flags = os.O_CREATE | os.O_WRONLY | os.O_EXCL
	}
	file, err := os.OpenFile(app.PackagePath(), flags, 0644)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(data)
	return errors.Join(writeErr, file.Close())
}
