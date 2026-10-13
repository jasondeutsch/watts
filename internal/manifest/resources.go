package manifest

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/jasondeutsch/watts/internal/workload"
	"gopkg.in/yaml.v3"
)

const Procedure = "Procedure"
const Agent = "Agent"
const ConfigMap = "ConfigMap"
const Secret = "Secret"
const MCP = "MCP"

// Reference identifies an authoring resource, not a running execution.
type Reference struct {
	Kind string `yaml:"kind"`
	Name string `yaml:"name"`
}

type Placement struct {
	DependsOn        []string          `yaml:"depends_on,omitempty"`
	TransitionLimits map[string]int    `yaml:"transition_limits,omitempty"`
	Name             string            `yaml:"name"`
	Use              Reference         `yaml:"use"`
	Parameters       map[string]any    `yaml:"parameters,omitempty"`
	Transitions      map[string]string `yaml:"transitions,omitempty"`
	Next             string            `yaml:"next,omitempty"`
	OnFailure        string            `yaml:"on_failure,omitempty"`
	Optional         bool              `yaml:"optional,omitempty"`
}

type WorkflowSpec struct {
	Version        int               `yaml:"version,omitempty"`
	Templates      map[string]string `yaml:"templates,omitempty"`
	MaxTransitions int               `yaml:"max_transitions,omitempty"`
	Steps          []Placement       `yaml:"steps"`
}

type SecretKeyRef struct {
	Name string `yaml:"name" json:"name"`
	Key  string `yaml:"key" json:"key"`
}

// Contract is shared by Agent and Procedure. It does not contain routing or model configuration.
type Contract struct {
	Container      *workload.Spec          `yaml:"container,omitempty"`
	Inputs         []string                `yaml:"inputs,omitempty"`
	Outputs        []string                `yaml:"outputs,omitempty"`
	PreChecks      []orchestration.Check   `yaml:"pre_checks,omitempty"`
	Checks         []orchestration.Check   `yaml:"checks,omitempty"`
	TimeoutSeconds int                     `yaml:"timeout_seconds,omitempty"`
	MaxAttempts    int                     `yaml:"max_attempts,omitempty"`
	Environment    map[string]string       `yaml:"environment,omitempty"`
	Secrets        map[string]SecretKeyRef `yaml:"secrets,omitempty"`
}

type Execution struct {
	Type string `yaml:"type"`
}

type ProcedureSpec struct {
	Contract  `yaml:",inline"`
	Execution Execution `yaml:"execution"`
	ConfigRef string    `yaml:"config_ref,omitempty"`
}

type Auth struct {
	Secret SecretKeyRef `yaml:"secret"`
	Env    string       `yaml:"env"`
}

type AgentSpec struct {
	Tools      []workload.Tool      `yaml:"tools,omitempty"`
	Gates      []workload.Gate      `yaml:"gates,omitempty"`
	Skills     []workload.Skill     `yaml:"skills,omitempty"`
	GatePolicy *workload.GatePolicy `yaml:"gate_policy,omitempty"`
	Contract   `yaml:",inline"`
	Identity   string   `yaml:"identity"`
	Provider   string   `yaml:"provider"`
	Model      string   `yaml:"model"`
	Prompt     string   `yaml:"prompt"`
	Auth       *Auth    `yaml:"auth,omitempty"`
	BaseURL    string   `yaml:"base_url,omitempty"`
	API        string   `yaml:"api,omitempty"`
	Thinking   string   `yaml:"thinking,omitempty"`
	SkillsDirs []string `yaml:"skills_dirs,omitempty"`
	ConfigRef  string   `yaml:"config_ref,omitempty"`
}

type ConfigMapSpec struct {
	Data map[string]any `yaml:"data"`
}

// Secret values are source environment-variable names, never credentials.
type SecretSpec struct {
	Env map[string]string `yaml:"env"`
}

type Resource struct {
	Kind          string    `yaml:"kind"`
	SchemaVersion int       `yaml:"schema_version"`
	Name          string    `yaml:"name"`
	Spec          yaml.Node `yaml:"spec"`
}

type Bundle struct{ Resources []Resource }

func ParseResources(data []byte) (Bundle, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	bundle := Bundle{}
	seen := map[Reference]bool{}
	for {
		var resource Resource
		err := decoder.Decode(&resource)
		if err == io.EOF {
			break
		}
		if err != nil {
			return bundle, fmt.Errorf("resource document %d: %w", len(bundle.Resources)+1, err)
		}
		switch resource.Kind {
		case Workflow, Procedure, Agent, ConfigMap, Secret, MCP:
		default:
			return bundle, fmt.Errorf("unsupported resource kind %q", resource.Kind)
		}
		if resource.SchemaVersion != SchemaVersion {
			return bundle, fmt.Errorf("%s %s: unsupported schema_version %d", resource.Kind, resource.Name, resource.SchemaVersion)
		}
		if !regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`).MatchString(resource.Name) || resource.Name == "end" {
			return bundle, fmt.Errorf("invalid %s resource name %q", resource.Kind, resource.Name)
		}
		if resource.Spec.Kind != yaml.MappingNode {
			return bundle, fmt.Errorf("%s %s requires a spec mapping", resource.Kind, resource.Name)
		}
		ref := Reference{resource.Kind, resource.Name}
		if seen[ref] {
			return bundle, fmt.Errorf("duplicate resource %s %s", ref.Kind, ref.Name)
		}
		seen[ref] = true
		bundle.Resources = append(bundle.Resources, resource)
	}
	if len(bundle.Resources) == 0 {
		return bundle, fmt.Errorf("resource bundle is empty")
	}
	return bundle, nil
}

func decodeSpec(resource Resource, target any) error {
	data, err := yaml.Marshal(&resource.Spec)
	if err != nil {
		return err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("%s %s: %w", resource.Kind, resource.Name, err)
	}
	return nil
}

// CompileWorkflow resolves a self-contained resource bundle into the existing
// Temporal execution contract. Secret source names are pinned; values are not read.
func CompileWorkflow(data []byte) (orchestration.Definition, error) {
	bundle, err := ParseResources(data)
	if err != nil {
		return orchestration.Definition{}, err
	}
	return bundle.Compile()
}

func (bundle Bundle) Validate() error {
	_, err := bundle.resolve(false)
	return err
}

func (bundle Bundle) Compile() (orchestration.Definition, error) { return bundle.resolve(true) }

func (bundle Bundle) resolve(requireWorkflow bool) (orchestration.Definition, error) {
	configs := map[string]map[string]any{}
	secrets := map[string]map[string]string{}
	work := map[Reference]orchestration.Step{}
	var workflow *WorkflowSpec
	for _, resource := range bundle.Resources {
		switch resource.Kind {
		case ConfigMap:
			var spec ConfigMapSpec
			if err := decodeSpec(resource, &spec); err != nil {
				return orchestration.Definition{}, err
			}
			if spec.Data == nil {
				return orchestration.Definition{}, fmt.Errorf("ConfigMap %s requires data", resource.Name)
			}
			configs[resource.Name] = spec.Data
		case Secret:
			var spec SecretSpec
			if err := decodeSpec(resource, &spec); err != nil {
				return orchestration.Definition{}, err
			}
			if len(spec.Env) == 0 {
				return orchestration.Definition{}, fmt.Errorf("Secret %s requires env sources", resource.Name)
			}
			for key, source := range spec.Env {
				if key == "" || !workload.EnvironmentSource(source) {
					return orchestration.Definition{}, fmt.Errorf("Secret %s has invalid environment source", resource.Name)
				}
			}
			secrets[resource.Name] = spec.Env
		}
	}
	services := map[string]workload.MCPService{}
	for _, original := range bundle.Resources {
		if original.Kind != MCP {
			continue
		}
		resource := original
		if err := applyConfig(&resource, configs); err != nil {
			return orchestration.Definition{}, err
		}
		service, err := compileMCP(resource, secrets)
		if err != nil {
			return orchestration.Definition{}, fmt.Errorf("MCP %s: %w", resource.Name, err)
		}
		services[resource.Name] = service
	}
	for _, original := range bundle.Resources {
		resource := original
		switch resource.Kind {
		case Workflow:
			if workflow != nil {
				return orchestration.Definition{}, fmt.Errorf("bundle must contain exactly one Workflow")
			}
			var spec WorkflowSpec
			if err := decodeSpec(resource, &spec); err != nil {
				return orchestration.Definition{}, err
			}
			workflow = &spec
		case Procedure, Agent:
			if err := applyConfig(&resource, configs); err != nil {
				return orchestration.Definition{}, err
			}
			step, err := compileWork(resource, secrets, services)
			if err != nil {
				return orchestration.Definition{}, err
			}
			work[Reference{resource.Kind, resource.Name}] = step
		}
	}
	if workflow == nil {
		if !requireWorkflow {
			return orchestration.Definition{}, nil
		}
		return orchestration.Definition{}, fmt.Errorf("bundle requires one Workflow")
	}
	definition := orchestration.Definition{Version: workflow.Version, Templates: workflow.Templates, MaxTransitions: workflow.MaxTransitions}
	for _, placement := range workflow.Steps {
		if placement.Use.Kind != Agent && placement.Use.Kind != Procedure {
			return definition, fmt.Errorf("step %s must use Agent or Procedure, found %s", placement.Name, placement.Use.Kind)
		}
		step, ok := work[placement.Use]
		if !ok {
			return definition, fmt.Errorf("step %s references unknown %s %s", placement.Name, placement.Use.Kind, placement.Use.Name)
		}
		step.Name = placement.Name
		if step.Agent != "" {
			step.Agent = placement.Name
		}
		step.DependsOn, step.TransitionLimits = placement.DependsOn, placement.TransitionLimits
		step.Parameters = placement.Parameters
		step.Transitions = placement.Transitions
		step.Next = placement.Next
		step.OnFailure = placement.OnFailure
		step.Optional = placement.Optional
		definition.Steps = append(definition.Steps, step)
	}
	return definition, definition.Validate()
}

func applyConfig(resource *Resource, configs map[string]map[string]any) error {
	var fields map[string]any
	if err := resource.Spec.Decode(&fields); err != nil {
		return err
	}
	value, provided := fields["config_ref"]
	if !provided {
		return nil
	}
	name, ok := value.(string)
	if !ok {
		return fmt.Errorf("%s %s: config_ref must be a name", resource.Kind, resource.Name)
	}
	base, ok := configs[name]
	if !ok {
		return fmt.Errorf("%s %s references unknown ConfigMap %s", resource.Kind, resource.Name, name)
	}
	merged := map[string]any{}
	for key, value := range base {
		if key == "config_ref" {
			return fmt.Errorf("ConfigMap %s must not contain config_ref", name)
		}
		merged[key] = value
	}
	for key, value := range fields {
		merged[key] = value
	}
	return resource.Spec.Encode(merged)
}

func compileWork(resource Resource, secrets map[string]map[string]string, services map[string]workload.MCPService) (orchestration.Step, error) {
	step := orchestration.Step{Name: resource.Name}
	var contract Contract
	if resource.Kind == Agent {
		var spec AgentSpec
		if err := decodeSpec(resource, &spec); err != nil {
			return step, err
		}
		if strings.TrimSpace(spec.Identity) == "" || strings.TrimSpace(spec.Provider) == "" || strings.TrimSpace(spec.Model) == "" {
			return step, fmt.Errorf("Agent %s requires identity, provider and model", resource.Name)
		}
		step.Agent = resource.Name
		step.Tools, step.Gates, step.Skills, step.GatePolicy = spec.Tools, spec.Gates, spec.Skills, spec.GatePolicy
		step.Identity = spec.Identity
		step.Provider, step.Model, step.Prompt = spec.Provider, spec.Model, spec.Prompt
		step.AgentSettings = &orchestration.AgentSettings{Thinking: spec.Thinking, SkillsDirs: spec.SkillsDirs, BaseURL: spec.BaseURL, API: spec.API}
		contract = spec.Contract
		if spec.Auth != nil {
			if contract.Secrets == nil {
				contract.Secrets = map[string]SecretKeyRef{}
			}
			if _, exists := contract.Secrets[spec.Auth.Env]; exists {
				return step, fmt.Errorf("Agent %s: duplicate auth binding", resource.Name)
			}
			contract.Secrets[spec.Auth.Env] = spec.Auth.Secret
			step.AgentSettings.APIKeyEnv = spec.Auth.Env
		}
	} else {
		var spec ProcedureSpec
		if err := decodeSpec(resource, &spec); err != nil {
			return step, err
		}
		contract = spec.Contract
		execution := spec.Execution
		switch execution.Type {
		case "container":
			// Container supplies the executable contract.
		case "approval":
			step.Human = true
		case "event":
			step.Event = true
		default:
			return step, fmt.Errorf("Procedure %s: unsupported execution type %q", resource.Name, execution.Type)
		}

	}
	step.Container = contract.Container
	if !step.Human && !step.Event && step.Container == nil {
		return step, fmt.Errorf("%s %s needs a container specification", resource.Kind, resource.Name)
	}
	step.Inputs, step.Outputs = contract.Inputs, contract.Outputs
	step.PreChecks, step.Checks = contract.PreChecks, contract.Checks
	step.TimeoutSeconds, step.MaxAttempts = contract.TimeoutSeconds, contract.MaxAttempts
	step.Environment = contract.Environment
	if len(contract.Secrets) > 0 {
		step.SecretEnvironment = map[string]string{}
	}
	for target, value := range contract.Environment {
		if !validEnvironment(target) || strings.ContainsRune(value, 0) {
			return step, fmt.Errorf("%s %s: invalid environment binding", resource.Kind, resource.Name)
		}
	}
	for target, ref := range contract.Secrets {
		if !validEnvironment(target) {
			return step, fmt.Errorf("%s %s: invalid secret target %q", resource.Kind, resource.Name, target)
		}
		if _, exists := contract.Environment[target]; exists {
			return step, fmt.Errorf("%s %s: conflicting environment binding %s", resource.Kind, resource.Name, target)
		}
		source, ok := secrets[ref.Name][ref.Key]
		if !ok {
			return step, fmt.Errorf("%s %s references missing Secret %s key %s", resource.Kind, resource.Name, ref.Name, ref.Key)
		}
		step.SecretEnvironment[target] = source
	}
	if contract.TimeoutSeconds < 0 || contract.MaxAttempts < 0 {
		return step, fmt.Errorf("%s %s: negative execution limit", resource.Kind, resource.Name)
	}
	if step.Agent != "" && strings.TrimSpace(step.Prompt) == "" {
		return step, fmt.Errorf("Agent %s requires a prompt", resource.Name)
	}
	if step.Container == nil && !step.Human && !step.Event && step.Agent == "" {
		return step, fmt.Errorf("Procedure %s requires a container", resource.Name)
	}

	if step.Agent != "" {
		if err := bindMCPs(&step, services); err != nil {
			return step, err
		}
		mcps := map[string]bool{}
		for name := range step.MCPServices {
			mcps[name] = true
		}
		if err := workload.ValidateAgentCapabilities(step.Tools, step.Gates, step.Skills, step.GatePolicy, step.Container.Resources, mcps); err != nil {
			return step, fmt.Errorf("Agent %s: %w", resource.Name, err)
		}
	}
	return step, nil
}
