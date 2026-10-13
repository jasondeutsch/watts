package workload

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
)

// MCPService is a resolved tool service, independent of the Agent's client library.
// Stdio commands run in the consuming workload image, never on the worker host.
type MCPService struct {
	Description       string               `json:"description,omitempty" yaml:"description,omitempty"`
	Connection        MCPConnection        `json:"connection" yaml:"connection"`
	Lifecycle         string               `json:"lifecycle" yaml:"lifecycle"`
	Tools             MCPToolPolicy        `json:"tools" yaml:"tools"`
	Requirements      MCPRequirements      `json:"requirements,omitempty" yaml:"requirements,omitempty"`
	Environment       map[string]string    `json:"environment,omitempty" yaml:"environment,omitempty"`
	Headers           map[string]string    `json:"headers,omitempty" yaml:"headers,omitempty"`
	SecretEnvironment map[string]string    `json:"secret_environment,omitempty" yaml:"-"`
	SecretHeaders     map[string]MCPSecret `json:"secret_headers,omitempty" yaml:"-"`
}

type MCPConnection struct {
	Transport      string       `json:"transport" yaml:"transport"`
	Command        *CommandTool `json:"command,omitempty" yaml:"command,omitempty"`
	URL            string       `json:"url,omitempty" yaml:"url,omitempty"`
	TimeoutSeconds int          `json:"timeout_seconds" yaml:"timeout_seconds"`
}

type MCPToolPolicy struct {
	Discovery string   `json:"discovery" yaml:"discovery"`
	Allow     []string `json:"allow,omitempty" yaml:"allow,omitempty"`
}

type MCPRequirements struct {
	OS        []string `json:"os,omitempty" yaml:"os,omitempty"`
	Arch      []string `json:"arch,omitempty" yaml:"arch,omitempty"`
	Workspace string   `json:"workspace,omitempty" yaml:"workspace,omitempty"`
	Network   bool     `json:"network,omitempty" yaml:"network,omitempty"`
}

// Environment is a launch-time binding name, not a credential value.
type MCPSecret struct {
	Environment string `json:"environment"`
	Prefix      string `json:"prefix,omitempty"`
}

func (service MCPService) Validate() error {
	if err := service.validateConnection(); err != nil {
		return err
	}
	if err := service.Tools.Validate(); err != nil {
		return err
	}
	if err := service.validateBindings(); err != nil {
		return err
	}
	return service.Requirements.Validate()
}

func (service MCPService) validateConnection() error {
	connection := service.Connection
	if connection.TimeoutSeconds < 1 || connection.TimeoutSeconds > 3600 {
		return fmt.Errorf("connection.timeout_seconds must be between 1 and 3600")
	}
	switch connection.Transport {
	case "stdio":
		if connection.Command == nil || connection.URL != "" || service.Lifecycle != "attempt" {
			return fmt.Errorf("stdio MCP needs a command, lifecycle: attempt, and no URL")
		}
		if err := connection.Command.Validate(); err != nil {
			return err
		}
		if connection.Command.TimeoutSeconds != 0 {
			return fmt.Errorf("use connection.timeout_seconds for MCP requests, not command.timeout_seconds")
		}
		if len(service.Headers)+len(service.SecretHeaders) > 0 {
			return fmt.Errorf("stdio MCP cannot have HTTP headers")
		}
	case "streamable-http":
		endpoint, err := url.Parse(connection.URL)
		if err != nil || endpoint.Host == "" || endpoint.User != nil || endpoint.Fragment != "" || !mcpLiteral(connection.URL) || (endpoint.Scheme != "http" && endpoint.Scheme != "https") {
			return fmt.Errorf("streamable-http MCP needs an HTTP(S) URL without credentials or fragment")
		}
		if connection.Command != nil || service.Lifecycle != "external" {
			return fmt.Errorf("streamable-http MCP requires lifecycle: external and no command")
		}
		if len(service.Environment)+len(service.SecretEnvironment) > 0 {
			return fmt.Errorf("remote MCP cannot have server environment bindings")
		}
	default:
		return fmt.Errorf("unsupported MCP transport %q; use stdio or streamable-http", connection.Transport)
	}
	return nil
}

func (policy MCPToolPolicy) Validate() error {
	if policy.Discovery != "all" && policy.Discovery != "allowlist" {
		return fmt.Errorf("tools.discovery must be all or allowlist")
	}
	if (policy.Discovery == "allowlist") != (len(policy.Allow) > 0) {
		return fmt.Errorf("tools.allow is required only for allowlist discovery")
	}
	if err := ValidateMCPToolNames(policy.Allow); err != nil {
		return err
	}
	return nil
}

func (service MCPService) validateBindings() error {
	for target, value := range service.Environment {
		if !EnvironmentTarget(target) || !mcpLiteral(value) {
			return fmt.Errorf("invalid MCP environment %s; use Secret bindings for interpolation", target)
		}
		if _, exists := service.SecretEnvironment[target]; exists {
			return fmt.Errorf("conflicting MCP environment binding %s", target)
		}
	}
	for target, binding := range service.SecretEnvironment {
		if !EnvironmentTarget(target) || !EnvironmentSource(binding) {
			return fmt.Errorf("invalid MCP secret environment binding %s", target)
		}
	}
	seenHeaders := map[string]bool{}
	for header, value := range service.Headers {
		key := strings.ToLower(header)
		if seenHeaders[key] {
			return fmt.Errorf("duplicate MCP header %s", header)
		}
		seenHeaders[key] = true
		if !validMCPHeader(header) || !mcpLiteral(value) || strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("invalid MCP header %s; use header_secrets for credentials", header)
		}
		for secretHeader := range service.SecretHeaders {
			if strings.EqualFold(header, secretHeader) {
				return fmt.Errorf("conflicting MCP header %s", header)
			}
		}
	}
	for header, secret := range service.SecretHeaders {
		key := strings.ToLower(header)
		if !validMCPHeader(header) || seenHeaders[key] || !EnvironmentSource(secret.Environment) || !mcpLiteral(secret.Prefix) || strings.ContainsAny(secret.Prefix, "\r\n") {
			return fmt.Errorf("invalid MCP secret header %s", header)
		}
		seenHeaders[key] = true
	}
	return nil
}

func (requirements MCPRequirements) Validate() error {
	for _, operatingSystem := range requirements.OS {
		if operatingSystem != "linux" && operatingSystem != "darwin" && operatingSystem != "windows" {
			return fmt.Errorf("unsupported MCP operating system %q", operatingSystem)
		}
	}
	for _, arch := range requirements.Arch {
		if arch != "amd64" && arch != "arm64" {
			return fmt.Errorf("unsupported MCP architecture %q", arch)
		}
	}
	switch requirements.Workspace {
	case "", "none", "read-only", "write":
	default:
		return fmt.Errorf("MCP workspace requirement must be none, read-only or write")
	}
	return nil
}

func (service MCPService) ValidateConsumer(container *Spec, selection MCPTool) error {
	if container == nil {
		return fmt.Errorf("MCP requires a consuming Agent container")
	}
	if (service.Connection.Transport == "streamable-http" || service.Requirements.Network) && container.Network != "bridge" {
		return fmt.Errorf("MCP needs container network: bridge")
	}
	if service.Requirements.Workspace == "write" && container.Workspace != "write" {
		return fmt.Errorf("MCP needs a writable workspace")
	}
	if selection.Exposure != "" && selection.Exposure != "direct" && selection.Exposure != "deferred" && selection.Exposure != "codemode" {
		return fmt.Errorf("MCP exposure must be direct, deferred or codemode")
	}
	if selection.AllowTools != nil && len(selection.AllowTools) == 0 {
		return fmt.Errorf("allow_tools must contain at least one tool when provided")
	}
	if err := ValidateMCPToolNames(selection.AllowTools); err != nil {
		return err
	}
	if service.Tools.Discovery == "allowlist" {
		for _, name := range selection.AllowTools {
			if !slices.Contains(service.Tools.Allow, name) {
				return fmt.Errorf("MCP consumer cannot expose disallowed tool %s", name)
			}
		}
	}
	return nil
}

func ValidateMCPToolNames(names []string) error {
	seen := map[string]bool{}
	for _, name := range names {
		if strings.TrimSpace(name) == "" || strings.ContainsAny(name, "*\x00\r\n") || seen[name] {
			return fmt.Errorf("invalid or duplicate MCP tool name %q", name)
		}
		seen[name] = true
	}
	return nil
}

func mcpLiteral(value string) bool {
	return !strings.ContainsRune(value, 0) && !strings.Contains(value, "${") && !strings.HasPrefix(strings.TrimSpace(value), "!")
}

func (service MCPService) ValidatePlatform(operatingSystem, architecture string) error {
	for _, requirement := range []struct {
		name, actual string
		allowed      []string
	}{
		{"os", operatingSystem, service.Requirements.OS},
		{"arch", architecture, service.Requirements.Arch},
	} {
		if len(requirement.allowed) > 0 && !slices.Contains(requirement.allowed, requirement.actual) {
			return fmt.Errorf("requires %s %v; workload image uses %s", requirement.name, requirement.allowed, requirement.actual)
		}
	}
	return nil
}

func validMCPHeader(name string) bool {
	if name == "" {
		return false
	}
	for _, char := range name {
		if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || strings.ContainsRune("!#$%&'*+-.^_`|~", char)) {
			return false
		}
	}
	return true
}
