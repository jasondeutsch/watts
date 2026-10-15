package application

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/manifest"
	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/jasondeutsch/watts/internal/storage"
	"github.com/jasondeutsch/watts/internal/workflowlang"
	"github.com/jasondeutsch/watts/internal/workspace"
)

// TaskDefinition is a resolved executor contract with optional package provenance.
type TaskDefinition struct {
	Name       string                   `json:"name"`
	Definition orchestration.Definition `json:"definition"`
	Captured   *workflowlang.Package
	Package    *workflowlang.Metadata `json:"package,omitempty"`
}

func decodeWorkflow(data []byte, value *orchestration.Definition) error {
	document, err := manifest.Decode(data, manifest.Workflow, orchestration.Definition{})
	if err != nil {
		return err
	}
	*value = document.Spec
	return nil
}

func (app *Service) TaskWorkflowPath(taskPath string) string {
	if isCompiledTask(app.Root, taskPath) {
		return compiledTaskPath(app.Root, taskPath)
	}
	return filepath.Join(app.Root, taskPath, "workflow.yaml")
}

func (app *Service) TaskWorkflow(cfg projectconfig.Config, taskPath string) (TaskDefinition, error) {
	data, err := os.ReadFile(app.TaskWorkflowPath(taskPath))
	if err != nil {
		return TaskDefinition{}, err
	}
	var definition orchestration.Definition
	if isCompiledTask(app.Root, taskPath) {
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&definition); err != nil {
			return TaskDefinition{}, err
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			return TaskDefinition{}, fmt.Errorf("compiled task definition must contain one JSON document")
		}
		expected, err := os.ReadFile(filepath.Join(app.StateDir(taskPath), "definition.sha256"))
		if err != nil {
			return TaskDefinition{}, fmt.Errorf("compiled task fingerprint unavailable: %w", err)
		}
		canonical, err := json.Marshal(definition)
		if err != nil {
			return TaskDefinition{}, err
		}
		if strings.TrimSpace(string(expected)) != storage.Digest(canonical) {
			return TaskDefinition{}, fmt.Errorf("compiled task definition was modified; recompile source and create a new task")
		}

	} else if err := decodeWorkflow(data, &definition); err != nil {
		return TaskDefinition{}, fmt.Errorf("%s: %w", app.Rel(app.TaskWorkflowPath(taskPath)), err)
	}
	if err := cfg.ValidateWorkflow(definition); err != nil {
		return TaskDefinition{}, err
	}
	return TaskDefinition{Definition: definition}, nil
}

func (app *Service) createTask(cfg projectconfig.Config, slug string, selected TaskDefinition) (string, error) {
	slug, err := taskSlug(slug)
	if err != nil {
		return "", err
	}
	if err := app.CheckLegacySettings(); err != nil {
		return "", err
	}
	if err := app.ProtectRuntimeFiles(); err != nil {
		return "", err
	}
	if err := app.RegisterWorkspace(cfg); err != nil {
		return "", err
	}
	if err := os.MkdirAll(app.TasksPath(cfg), 0755); err != nil {
		return "", err
	}
	taskPath, err := createTaskDirectory(app.Root, cfg.TasksDir, slug, time.Now())
	if err != nil {
		return "", err
	}
	replacer := strings.NewReplacer("__NAME__", filepath.Base(taskPath), "__TASKS__", cfg.TasksDir, "tasks/<date>-<slug>/", taskPath+"/")
	for artifact, contents := range selected.Definition.Templates {
		target, err := workspace.TaskArtifactOutput(app.Root, taskPath, artifact)
		if err != nil {
			return "", err
		}
		if err := os.WriteFile(target, []byte(replacer.Replace(contents)), 0644); err != nil {
			return "", err
		}
	}
	taskDefinition := selected.Definition
	taskDefinition.Steps = append([]orchestration.Step(nil), selected.Definition.Steps...)
	if selected.Captured != nil {
		capturedDir := filepath.Join(app.StateDir(taskPath), "package")
		if err := os.MkdirAll(capturedDir, 0700); err != nil {
			return "", err
		}
		if err := selected.Captured.Materialize(capturedDir); err != nil {
			return "", err
		}
		// Bind logical compiler resources to this task's durable captured package.
		for i := range taskDefinition.Steps {
			if taskDefinition.Steps[i].Container == nil {
				continue
			}
			spec := *taskDefinition.Steps[i].Container
			resources := map[string]string{}
			for name, source := range spec.Resources {
				resources[name] = source // Project-relative resources are captured at submission.
				for _, asset := range selected.Captured.Assets {
					if source == asset.Root || strings.HasPrefix(source, asset.Root+"/") {
						resources[name] = filepath.ToSlash(filepath.Join(taskPath, StateDirName, "package", source))
						break
					}
				}
			}
			spec.Resources = resources
			taskDefinition.Steps[i].Container = &spec
		}
	}
	taskDefinition.Templates = nil // Scaffolding has already created these artifacts.
	if err := storage.WriteJSONAtomically(compiledTaskPath(app.Root, taskPath), taskDefinition); err != nil {
		return "", err
	}
	canonical, err := json.Marshal(taskDefinition)
	if err != nil {
		return "", err
	}
	if err := storage.WriteFileAtomically(filepath.Join(app.StateDir(taskPath), "definition.sha256"), []byte(storage.Digest(canonical)+"\n")); err != nil {
		return "", err
	}

	if err := app.WritePackageMetadata(taskPath, selected.Package); err != nil {
		return "", err
	}
	app.say("Created %s. Draft SPEC.md, then run: wctl exec %s", taskPath, taskPath)
	return taskPath, nil
}

// Mkdir arbitrates concurrent creation. Advance one millisecond on a same-name
// collision so repeated requests never reuse an existing task directory.
func createTaskDirectory(root, tasksDir, slug string, timestamp time.Time) (string, error) {
	for {
		name := strings.ReplaceAll(timestamp.UTC().Format("2006-01-02t150405.000z"), ".", "-") + "-" + slug
		taskPath := filepath.ToSlash(filepath.Join(tasksDir, name))
		err := os.Mkdir(filepath.Join(root, taskPath), 0755)
		if errors.Is(err, os.ErrExist) {
			timestamp = timestamp.Add(time.Millisecond)
			continue
		}
		if err != nil {
			return "", err
		}
		return taskPath, nil
	}
}

// Normalize new human-entered names once. Existing task lookup preserves case
// and never passes through this function.
func taskSlug(name string) (string, error) {
	var result strings.Builder
	separator := false
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			if separator && result.Len() > 0 {
				result.WriteByte('-')
			}
			result.WriteRune(r)
			separator = false
		case r == ' ' || r == '-' || r == '_':
			separator = true
		default:
			return "", fmt.Errorf("task name %q contains unsupported characters; use letters, digits, spaces, underscores or dashes", name)
		}
	}
	if result.Len() == 0 || result.Len() > 100 {
		return "", fmt.Errorf("task name must produce between 1 and 100 letters, digits or dashes")
	}
	return result.String(), nil
}
