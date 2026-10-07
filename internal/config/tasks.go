package config

import (
	"encoding/json"
	"fmt"

	"github.com/jasondeutsch/watts/internal/orchestration"
	workflowtemplates "github.com/jasondeutsch/watts/workflow-templates"
)

// DefaultWorkflow reads the bundled SDLC template. The JSON file is the source of truth.
func DefaultWorkflow() orchestration.Definition {
	contents, err := workflowtemplates.Files.ReadFile("sdlc.json")
	if err != nil {
		panic(err)
	}
	var definition orchestration.Definition
	if err := json.Unmarshal(contents, &definition); err != nil {
		panic(err)
	}
	return definition
}

func (c Config) ValidateWorkflow(definition orchestration.Definition) error {
	if err := definition.Validate(); err != nil {
		return err
	}
	for _, step := range definition.Steps {
		if step.Agent != "" {
			if _, ok := c.Agent(step.Agent); !ok {
				return fmt.Errorf("workflow step %q uses agent %q, which is not defined under agents", step.Name, step.Agent)
			}
			if !ReProvider.MatchString(step.Provider) {
				return fmt.Errorf("workflow step %q needs a valid provider", step.Name)
			}
			if !ReModel.MatchString(step.Model) {
				return fmt.Errorf("workflow step %q needs a valid model", step.Name)
			}
			if provider, ok := c.Providers[step.Provider]; ok && !(step.Provider == "ollama" && len(provider.Models) == 0) {
				found := false
				for _, model := range provider.Models {
					found = found || model.ID == step.Model
				}
				if !found {
					return fmt.Errorf("workflow step %q uses model %q, which providers.%s does not list", step.Name, step.Model, step.Provider)
				}
			}

		}
		if step.Capability != "" {
			if _, ok := c.Capabilities[step.Capability]; !ok {
				return fmt.Errorf("workflow step %q uses undefined capability %q", step.Name, step.Capability)
			}
		}
	}
	return nil
}

// WithWorkflowAgents returns a configuration view for the selected workflow without changing its persisted fields.
func (c Config) WithWorkflowAgents(definition orchestration.Definition) Config {
	c.agentWorkflow = &definition
	return c
}

// AgentWorkflow is the source of model selection for setup and interactive agent sessions.
func (c Config) AgentWorkflow() orchestration.Definition {
	if c.agentWorkflow != nil {
		return *c.agentWorkflow
	}
	if c.Workflow != nil {
		return *c.Workflow
	}
	return DefaultWorkflow()
}
