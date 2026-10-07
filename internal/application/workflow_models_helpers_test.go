package application

import projectconfig "github.com/jasondeutsch/watts/internal/config"

func setWorkflowModels(cfg *projectconfig.Config, provider, build, review string) {
	definition := cfg.AgentWorkflow()
	for index := range definition.Steps {
		step := &definition.Steps[index]
		if step.Agent == "" {
			continue
		}
		if provider != "" {
			step.Provider = provider
		}
		if step.Agent == "review" {
			if review != "" {
				step.Model = review
			}
		} else if build != "" {
			step.Model = build
		}
	}
	cfg.Workflow = &definition
	*cfg = cfg.WithWorkflowAgents(definition)
}
