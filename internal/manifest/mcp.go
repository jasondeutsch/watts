package manifest

import (
	"crypto/sha256"
	"fmt"
	"maps"

	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/jasondeutsch/watts/internal/workload"
)

type MCPSpec struct {
	workload.MCPService `yaml:",inline"`
	ConfigRef           string                     `yaml:"config_ref,omitempty"`
	Secrets             map[string]SecretKeyRef    `yaml:"secrets,omitempty"`
	HeaderSecrets       map[string]MCPHeaderSecret `yaml:"header_secrets,omitempty"`
}

type MCPHeaderSecret struct {
	Secret SecretKeyRef `yaml:"secret"`
	Prefix string       `yaml:"prefix,omitempty"`
}

func compileMCP(resource Resource, secrets map[string]map[string]string) (workload.MCPService, error) {
	var spec MCPSpec
	if err := decodeSpec(resource, &spec); err != nil {
		return workload.MCPService{}, err
	}
	service := spec.MCPService
	if len(spec.Secrets) > 0 {
		service.SecretEnvironment = map[string]string{}
	}
	for target, ref := range spec.Secrets {
		source, ok := secrets[ref.Name][ref.Key]
		if !ok {
			return service, fmt.Errorf("MCP %s references missing Secret %s key %s", resource.Name, ref.Name, ref.Key)
		}
		service.SecretEnvironment[target] = source
	}
	if len(spec.HeaderSecrets) > 0 {
		service.SecretHeaders = map[string]workload.MCPSecret{}
	}
	for header, binding := range spec.HeaderSecrets {
		source, ok := secrets[binding.Secret.Name][binding.Secret.Key]
		if !ok {
			return service, fmt.Errorf("MCP %s references missing Secret %s key %s", resource.Name, binding.Secret.Name, binding.Secret.Key)
		}
		service.SecretHeaders[header] = workload.MCPSecret{Environment: source, Prefix: binding.Prefix}
	}
	return service, service.Validate()
}

// Resolve only services explicitly selected by this consumer. Credential targets
// are scoped so two servers may both use API_KEY without sharing credentials.
func bindMCPs(step *orchestration.Step, services map[string]workload.MCPService) error {
	for _, tool := range step.Tools {
		if tool.MCP == nil {
			continue
		}
		name := tool.MCP.Ref
		service, ok := services[name]
		if !ok {
			return fmt.Errorf("Agent %s references unknown MCP %s", step.Name, name)
		}
		if _, exists := step.MCPServices[name]; exists {
			return fmt.Errorf("Agent %s selects MCP %s more than once", step.Name, name)
		}
		if err := service.ValidateConsumer(step.Container, *tool.MCP); err != nil {
			return fmt.Errorf("MCP %s: %w", name, err)
		}
		service.SecretEnvironment = maps.Clone(service.SecretEnvironment)
		service.SecretHeaders = maps.Clone(service.SecretHeaders)
		bind := func(target, source string) (string, error) {
			digest := sha256.Sum256([]byte(name + ":" + target))
			alias := fmt.Sprintf("MCP_%X", digest[:12])
			if _, exists := step.Environment[alias]; exists {
				return "", fmt.Errorf("MCP %s conflicts with Agent environment %s", name, alias)
			}
			if old, exists := step.SecretEnvironment[alias]; exists && old != source {
				return "", fmt.Errorf("MCP %s has conflicting credential binding", name)
			}
			if step.SecretEnvironment == nil {
				step.SecretEnvironment = map[string]string{}
			}
			step.SecretEnvironment[alias] = source
			return alias, nil
		}
		for target, source := range service.SecretEnvironment {
			alias, err := bind("env:"+target, source)
			if err != nil {
				return err
			}
			service.SecretEnvironment[target] = alias
		}
		for header, secret := range service.SecretHeaders {
			alias, err := bind("header:"+header, secret.Environment)
			if err != nil {
				return err
			}
			secret.Environment = alias
			service.SecretHeaders[header] = secret
		}
		if step.MCPServices == nil {
			step.MCPServices = map[string]workload.MCPService{}
		}
		step.MCPServices[name] = service
	}
	return nil
}

// Restore authoring references from pinned launch bindings without reading secrets.
func encodeMCP(name string, service workload.MCPService, step orchestration.Step) (MCPSpec, error) {
	spec := MCPSpec{MCPService: service}
	secretName := "credentials-" + step.Name
	for target, alias := range service.SecretEnvironment {
		if step.SecretEnvironment[alias] == "" {
			return spec, fmt.Errorf("MCP %s has no launch binding for %s", name, target)
		}
		if spec.Secrets == nil {
			spec.Secrets = map[string]SecretKeyRef{}
		}
		spec.Secrets[target] = SecretKeyRef{Name: secretName, Key: alias}
	}
	for header, binding := range service.SecretHeaders {
		if step.SecretEnvironment[binding.Environment] == "" {
			return spec, fmt.Errorf("MCP %s has no launch binding for header %s", name, header)
		}
		if spec.HeaderSecrets == nil {
			spec.HeaderSecrets = map[string]MCPHeaderSecret{}
		}
		spec.HeaderSecrets[header] = MCPHeaderSecret{Secret: SecretKeyRef{Name: secretName, Key: binding.Environment}, Prefix: binding.Prefix}
	}
	spec.SecretEnvironment, spec.SecretHeaders = nil, nil
	return spec, nil
}
