package config

import (
	"fmt"

	"github.com/jasondeutsch/watts/internal/manifest"
	"github.com/jasondeutsch/watts/internal/orchestration"
	workflowtemplates "github.com/jasondeutsch/watts/workflow-templates"
)

// DefaultWorkflow reads the bundled SDLC template. The YAML file is the source of truth.
func DefaultWorkflow() orchestration.Definition {
	contents, err := workflowtemplates.Files.ReadFile("sdlc.yaml")
	if err != nil {
		panic(err)
	}
	var definition orchestration.Definition
	document, err := manifest.Decode(contents, manifest.Workflow, definition)
	if err != nil {
		panic(err)
	}
	return document.Spec
}

func (c Config) ValidateWorkflow(definition orchestration.Definition) error {
	c = c.WithWorkflowAgents(definition)
	validation := c
	validation.Workflow = nil
	if err := validation.Validate(); err != nil {
		return err
	}
	if err := definition.Validate(); err != nil {
		return err
	}
	for _, step := range definition.Steps {
		for _, name := range step.MCPs {
			if _, ok := c.MCPs[name]; !ok {
				return fmt.Errorf("workflow step %q uses undefined mcp %q", step.Name, name)
			}
		}
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
	agents := map[string]AgentConfig{}
	for name, agent := range c.Agents {
		agents[name] = agent
	}
	mcps := map[string]MCPServer{}
	for name, server := range c.MCPs {
		mcps[name] = server
	}
	capabilities := map[string]Capability{}
	for name, capability := range c.Capabilities {
		capabilities[name] = capability
	}
	for _, step := range definition.Steps {
		if len(step.CapabilityCommand) > 0 {
			capabilities[step.Capability] = Capability{Command: step.CapabilityCommand}
		}
	}
	providers := map[string]Provider{}
	for name, provider := range c.Providers {
		providers[name] = provider
	}
	for _, step := range definition.Steps {
		if step.Agent == "" {
			continue
		}
		if _, exists := agents[step.Agent]; !exists && !Contains(BuiltinAgents, step.Agent) && step.AgentSettings == nil {
			continue
		}
		agent := agents[step.Agent]
		if settings := step.AgentSettings; settings != nil {
			for name, server := range settings.MCPServers {
				mcps[name] = server
			}
			if settings.Thinking != "" {
				agent.Thinking = settings.Thinking
			}
			if len(settings.SkillsDirs) > 0 {
				agent.SkillsDirs = settings.SkillsDirs
			}
			provider := providers[step.Provider]
			if settings.BaseURL != "" {
				provider.BaseURL = settings.BaseURL
				found := false
				for _, model := range provider.Models {
					found = found || model.ID == step.Model
				}
				if !found {
					provider.Models = append(append([]ProviderModel(nil), provider.Models...), ProviderModel{ID: step.Model, ContextWindow: 1048576, MaxTokens: 16384})
				}
			}
			if settings.API != "" {
				provider.API = settings.API
			}
			if settings.APIKeyEnv != "" {
				provider.APIKeyEnv = settings.APIKeyEnv
				provider.APIKeyCommand = nil
			}
			if settings.BaseURL != "" || settings.API != "" || settings.APIKeyEnv != "" {
				providers[step.Provider] = provider
			}
		}
		agents[step.Agent] = agent
	}
	c.Agents = agents
	c.Providers = providers
	c.MCPs = mcps
	c.Capabilities = capabilities
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

// WithResolvedWorkSettings captures local runtime defaults on work resources
// when application callers explicitly author a workflow definition.
func (c Config) WithResolvedWorkSettings(definition orchestration.Definition) orchestration.Definition {
	definition.Steps = append([]orchestration.Step(nil), definition.Steps...)
	for index := range definition.Steps {
		step := &definition.Steps[index]
		if step.Capability != "" {
			step.CapabilityCommand = append([]string(nil), c.Capabilities[step.Capability].Command...)
		}
		if step.Agent == "" {
			continue
		}
		settings := orchestration.AgentSettings{Sandbox: "local"}
		if step.AgentSettings != nil {
			settings = *step.AgentSettings
		}
		agent, _ := c.Agent(step.Agent)
		settings.Thinking = agent.Thinking
		settings.SkillsDirs = agent.SkillsDirs
		provider := c.Providers[step.Provider]
		settings.BaseURL = provider.BaseURL
		settings.API = provider.API
		if provider.APIKeyEnv != "" && settings.APIKeyEnv == "" {
			settings.APIKeyEnv = provider.APIKeyEnv
			bindings := map[string]string{}
			for target, source := range step.SecretEnvironment {
				bindings[target] = source
			}
			bindings[provider.APIKeyEnv] = provider.APIKeyEnv
			step.SecretEnvironment = bindings
		}
		servers := map[string]orchestration.MCPServer{}
		for name, server := range settings.MCPServers {
			servers[name] = server
		}
		for _, name := range step.MCPs {
			if server, ok := c.MCPs[name]; ok {
				servers[name] = server
			}
		}
		if len(servers) > 0 {
			settings.MCPServers = servers
		}
		step.AgentSettings = &settings
	}
	return definition
}
