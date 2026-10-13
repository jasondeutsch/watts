package manifest

import (
	"bytes"
	"fmt"
	"io"
	"maps"
	"reflect"
	"slices"

	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/jasondeutsch/watts/internal/workload"
	"gopkg.in/yaml.v3"
)

// LoadWorkflow accepts an inline execution definition or a self-contained bundle
// of named work resources. Both resolve to the same pinned runtime representation.
func LoadWorkflow(data []byte) (Document[orchestration.Definition], error) {
	var first Resource
	if err := yaml.Unmarshal(data, &first); err != nil {
		return Document[orchestration.Definition]{}, fmt.Errorf("invalid YAML manifest: %w", err)
	}
	var fields struct {
		Steps []map[string]any `yaml:"steps"`
	}
	_ = first.Spec.Decode(&fields)
	referenced := first.Kind != Workflow
	if first.Kind == Project {
		return Document[orchestration.Definition]{}, fmt.Errorf("expected kind Workflow, found Project")
	}
	for _, step := range fields.Steps {
		if _, ok := step["use"]; ok {
			referenced = true
		}
	}
	if !referenced {
		decoder := yaml.NewDecoder(bytes.NewReader(data))
		var node yaml.Node
		_ = decoder.Decode(&node)
		if err := decoder.Decode(&node); err != io.EOF {
			referenced = true
		}
	}
	if referenced {
		bundle, err := ParseResources(data)
		if err != nil {
			return Document[orchestration.Definition]{}, err
		}
		definition, err := bundle.Compile()
		document := Document[orchestration.Definition]{Kind: Workflow, SchemaVersion: SchemaVersion, Spec: definition}
		for _, resource := range bundle.Resources {
			if resource.Kind == Workflow {
				document.Name = resource.Name
			}
		}
		return document, err
	}
	return decodeDocument(data, Workflow, orchestration.Definition{})
}

// EncodeWorkflow creates a self-contained authoring bundle from resolved work.
// Credentials remain source references, including after task scaffolding.
func EncodeWorkflow(name string, definition orchestration.Definition) ([]byte, error) {
	workflow := WorkflowSpec{Version: definition.Version, Templates: definition.Templates, MaxTransitions: definition.MaxTransitions}
	documents := []any{}
	services := map[string]workload.MCPService{}
	for _, step := range definition.Steps {
		kind := Procedure
		contract := Contract{Container: step.Container, Inputs: step.Inputs, Outputs: step.Outputs, Checks: step.Checks, PreChecks: step.PreChecks, TimeoutSeconds: step.TimeoutSeconds, MaxAttempts: step.MaxAttempts, Environment: step.Environment}
		if len(step.SecretEnvironment) > 0 {
			secretName := "credentials-" + step.Name
			contract.Secrets = map[string]SecretKeyRef{}
			sources := map[string]string{}
			for target, source := range step.SecretEnvironment {
				sources[target] = source
				contract.Secrets[target] = SecretKeyRef{Name: secretName, Key: target}
			}
			documents = append(documents, Document[SecretSpec]{Kind: Secret, SchemaVersion: SchemaVersion, Name: secretName, Spec: SecretSpec{Env: sources}})
		}
		var spec any
		if step.Agent != "" {
			kind = Agent
			agent := AgentSpec{Contract: contract, Identity: step.Identity, Provider: step.Provider, Model: step.Model, Prompt: step.Prompt}
			agent.Tools, agent.Gates, agent.Skills, agent.GatePolicy = step.Tools, step.Gates, step.Skills, step.GatePolicy
			if agent.Identity == "" {
				agent.Identity = step.Agent
			}
			if settings := step.AgentSettings; settings != nil {
				agent.Thinking, agent.SkillsDirs, agent.BaseURL, agent.API = settings.Thinking, settings.SkillsDirs, settings.BaseURL, settings.API
				if settings.APIKeyEnv != "" {
					ref, ok := agent.Secrets[settings.APIKeyEnv]
					if !ok {
						return nil, fmt.Errorf("step %s: auth source is missing", step.Name)
					}
					agent.Auth = &Auth{Secret: ref, Env: settings.APIKeyEnv}
					delete(agent.Secrets, settings.APIKeyEnv)
				}
			}
			for _, serviceName := range slices.Sorted(maps.Keys(step.MCPServices)) {
				service := step.MCPServices[serviceName]
				if existing, ok := services[serviceName]; ok {
					if !reflect.DeepEqual(existing, service) {
						return nil, fmt.Errorf("conflicting MCP resource %s", serviceName)
					}
					continue
				}
				serviceSpec, err := encodeMCP(serviceName, service, step)
				if err != nil {
					return nil, err
				}
				documents = append(documents, Document[MCPSpec]{Kind: MCP, SchemaVersion: SchemaVersion, Name: serviceName, Spec: serviceSpec})
				services[serviceName] = service
			}
			spec = agent
		} else {
			execution := Execution{}
			switch {
			case step.Container != nil:
				execution.Type = "container"
			case step.Human:
				execution.Type = "approval"
			case step.Event:
				execution.Type = "event"
			}
			spec = ProcedureSpec{Contract: contract, Execution: execution}
		}
		documents = append(documents, Document[any]{Kind: kind, SchemaVersion: SchemaVersion, Name: step.Name, Spec: spec})
		workflow.Steps = append(workflow.Steps, Placement{Name: step.Name, Use: Reference{Kind: kind, Name: step.Name}, Parameters: step.Parameters, Transitions: step.Transitions, Next: step.Next, OnFailure: step.OnFailure, Optional: step.Optional})
	}
	var output bytes.Buffer
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if err := encoder.Encode(Document[WorkflowSpec]{Kind: Workflow, SchemaVersion: SchemaVersion, Name: name, Spec: workflow}); err != nil {
		return nil, err
	}
	for _, document := range documents {
		if err := encoder.Encode(document); err != nil {
			return nil, err
		}
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
