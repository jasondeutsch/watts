package application

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/manifest"
	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/jasondeutsch/watts/internal/workspace"
	workflowtemplates "github.com/jasondeutsch/watts/workflow-templates"
)

const builtinWorkflow = "sdlc"

// WorkflowSelection identifies a reusable project template and its definition.
type WorkflowSelection struct {
	Name       string                   `json:"name"`
	Definition orchestration.Definition `json:"definition"`
}

func decodeWorkflow(data []byte, value *orchestration.Definition) error {
	document, err := manifest.Decode(data, manifest.Workflow, orchestration.Definition{})
	if err != nil {
		return err
	}
	*value = document.Spec
	return nil
}

// ResolveWorkflow selects a bundled base template or a project-supplied YAML file.
func (app *Service) ResolveWorkflow(cfg projectconfig.Config, name string) (WorkflowSelection, error) {
	if name == "" && cfg.Workflow != nil {
		if err := cfg.ValidateWorkflow(*cfg.Workflow); err != nil {
			return WorkflowSelection{}, err
		}
		return WorkflowSelection{Definition: *cfg.Workflow}, nil
	}
	if name == "" {
		name = cfg.DefaultWorkflow
	}
	if name == "" {
		name = builtinWorkflow
	}
	var contents []byte
	var err error
	if strings.Contains(name, "/") || strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
		if err := projectconfig.ValidRelPath("default_workflow", name); err != nil {
			return WorkflowSelection{}, err
		}
		contents, err = manifest.ReadWorkflowSource(filepath.Join(app.Root, filepath.FromSlash(name)))
	} else {
		if !orchestration.ValidName(name) {
			return WorkflowSelection{}, fmt.Errorf("invalid workflow template %q", name)
		}
		contents, err = workflowtemplates.Files.ReadFile(name + ".yaml")
	}
	if err != nil {
		return WorkflowSelection{}, fmt.Errorf("workflow %s: %w", name, err)
	}
	var definition orchestration.Definition
	if err := decodeWorkflow(contents, &definition); err != nil {
		return WorkflowSelection{}, fmt.Errorf("workflow %s: %w", name, err)
	}
	if err := cfg.ValidateWorkflow(definition); err != nil {
		return WorkflowSelection{}, err
	}
	return WorkflowSelection{Name: name, Definition: definition}, nil
}

func (app *Service) TaskWorkflowPath(taskPath string) string {
	return filepath.Join(app.Root, taskPath, "workflow.yaml")
}

func (app *Service) TaskWorkflow(cfg projectconfig.Config, taskPath string) (WorkflowSelection, error) {
	data, err := os.ReadFile(app.TaskWorkflowPath(taskPath))
	if err != nil {
		return WorkflowSelection{}, err
	}
	var definition orchestration.Definition
	if err := decodeWorkflow(data, &definition); err != nil {
		return WorkflowSelection{}, fmt.Errorf("%s: %w", app.Rel(app.TaskWorkflowPath(taskPath)), err)
	}
	if err := cfg.ValidateWorkflow(definition); err != nil {
		return WorkflowSelection{}, err
	}
	return WorkflowSelection{Definition: definition}, nil
}

func (app *Service) CreateTask(cfg projectconfig.Config, slug, workflowName string) (string, error) {
	if slug == "" || strings.Trim(slug, "abcdefghijklmnopqrstuvwxyz0123456789-") != "" {
		return "", errors.New("slug may contain only lowercase letters, digits and dashes")
	}
	selected, err := app.ResolveWorkflow(cfg, workflowName)
	if err != nil {
		return "", err
	}
	taskPath := filepath.ToSlash(filepath.Join(cfg.TasksDir, time.Now().Format("2006-01-02")+"-"+slug))
	if err := os.MkdirAll(app.TasksPath(cfg), 0755); err != nil {
		return "", err
	}
	if err := os.Mkdir(filepath.Join(app.Root, taskPath), 0755); err != nil {
		return "", err
	}
	replacer := strings.NewReplacer("__NAME__", filepath.Base(taskPath), "__TASKS__", cfg.TasksDir, "__KIT__", ".watts/kit", "__AGENT__", selected.Definition.AgentIdentity("build"), "__REVIEWER__", selected.Definition.AgentIdentity("review"), "tasks/<date>-<slug>/", taskPath+"/")
	for artifact, contents := range selected.Definition.Templates {
		target, err := workspace.TaskArtifactOutput(app.Root, taskPath, artifact)
		if err != nil {
			return "", err
		}
		if err := os.WriteFile(target, []byte(replacer.Replace(contents)), 0644); err != nil {
			return "", err
		}
	}
	if selected.Definition.CaptureBaseline {
		var snapshot bytes.Buffer
		if err := app.KitScriptTo(cfg, &snapshot, "snapshot.sh", taskPath, "base"); err != nil {
			return "", err
		}
		if err := os.WriteFile(filepath.Join(app.Root, taskPath, "BASE"), snapshot.Bytes(), 0644); err != nil {
			return "", err
		}
	}
	taskDefinition := selected.Definition
	taskDefinition.Templates = nil // Scaffolding has already created these artifacts.
	if err := manifest.Write(app.TaskWorkflowPath(taskPath), manifest.Workflow, filepath.Base(taskPath), taskDefinition); err != nil {
		return "", err
	}
	app.say("Created %s. Customize workflow.yaml, then run: watts task run %s", taskPath, taskPath)
	return taskPath, nil
}
