package application

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/kit"
	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/jasondeutsch/watts/internal/storage"
)

type WorkflowBinding struct {
	WorkflowID string                         `json:"workflow_id"`
	RunID      string                         `json:"run_id,omitempty"`
	Settings   projectconfig.TemporalSettings `json:"settings"`
	Input      orchestration.Input            `json:"input"`
}

func (app *Service) WorkflowBindingPath(taskPath string) string {
	return filepath.Join(app.StateDir(taskPath), "temporal.json")
}

func (app *Service) HasTemporalTask(taskPath string) bool {
	_, err := os.Stat(app.WorkflowBindingPath(taskPath))
	return !errors.Is(err, os.ErrNotExist)
}

func (app *Service) LoadWorkflowBinding(taskPath string) (WorkflowBinding, error) {
	var binding WorkflowBinding
	data, err := os.ReadFile(app.WorkflowBindingPath(taskPath))
	if err != nil {
		return binding, err
	}
	if err = json.Unmarshal(data, &binding); err != nil {
		return binding, fmt.Errorf("invalid Temporal task record: %w", err)
	}
	if binding.WorkflowID == "" || binding.Input.Task != taskPath {
		return binding, errors.New("Temporal task record does not match this task")
	}
	return binding, nil
}

// WorkflowConfigPath locates the runtime settings frozen at first submission.
func (app *Service) WorkflowConfigPath(taskPath string) string {
	return filepath.Join(app.StateDir(taskPath), "workflow-config.json")
}

func (app *Service) CreateWorkflowBinding(config projectconfig.Config, taskPath string, flags *RunOptions) (WorkflowBinding, error) {
	selected, err := app.TaskWorkflow(config, taskPath)
	if err != nil {
		return WorkflowBinding{}, err
	}
	definition := selected.Definition
	if err := definition.Validate(); err != nil {
		return WorkflowBinding{}, err
	}
	for i := range definition.Steps {
		if definition.Steps[i].MaxAttempts == 0 && config.Limits.MaxAttempts > 0 {
			definition.Steps[i].MaxAttempts = config.Limits.MaxAttempts
		}
		if definition.Steps[i].TimeoutSeconds == 0 && config.Limits.MaxMinutes > 0 {
			definition.Steps[i].TimeoutSeconds = config.Limits.MaxMinutes * 60
		}
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return WorkflowBinding{}, err
	}
	// Store the runtime configuration locally. Provider credentials are fetched by the worker;
	// neither credential values nor runtime settings are workflow-history inputs.
	if err = storage.WriteJSONAtomically(app.WorkflowConfigPath(taskPath), config); err != nil {
		return WorkflowBinding{}, err
	}
	binding := WorkflowBinding{
		WorkflowID: "watts-" + kit.Digest([]byte(app.Root + "\x00" + taskPath))[:32],
		Settings:   app.ResolvedTemporalSettings(config),
		Input: orchestration.Input{
			Task:         taskPath,
			Definition:   definition,
			ConfigSHA256: kit.Digest(append(data, '\n')),
			Offline:      flags.Offline,
			Pass:         flags.Pass,
		},
	}
	if err = storage.WriteJSONAtomically(app.WorkflowBindingPath(taskPath), binding); err != nil {
		return WorkflowBinding{}, err
	}
	return binding, nil
}
