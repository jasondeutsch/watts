package workload

import (
	"fmt"
	"path"
	"regexp"
	"strings"
)

// Tool is a named capability, not a workflow stage. Command arguments are fixed.
type Tool struct {
	Name        string       `json:"name" yaml:"name"`
	Description string       `json:"description,omitempty" yaml:"description,omitempty"`
	Command     *CommandTool `json:"command,omitempty" yaml:"command,omitempty"`
	MCP         *MCPTool     `json:"mcp,omitempty" yaml:"mcp,omitempty"`
}
type CommandTool struct {
	Argv           []string `json:"argv" yaml:"argv"`
	WorkingDir     string   `json:"working_dir,omitempty" yaml:"working_dir,omitempty"`
	TimeoutSeconds int      `json:"timeout_seconds,omitempty" yaml:"timeout_seconds,omitempty"`
}
type MCPTool struct {
	Ref        string   `json:"ref" yaml:"ref"`
	Exposure   string   `json:"exposure,omitempty" yaml:"exposure,omitempty"`
	AllowTools []string `json:"allow_tools,omitempty" yaml:"allow_tools,omitempty"`
}
type Gate struct {
	Name   string          `json:"name" yaml:"name"`
	Tool   string          `json:"tool" yaml:"tool"`
	Expect GateExpectation `json:"expect" yaml:"expect"`
}
type GateExpectation struct {
	ExitCode int `json:"exit_code" yaml:"exit_code"`
}

// MaxRepairs excludes the initial execution. Omitted policy allows two repairs.
type GatePolicy struct {
	MaxRepairs int `json:"max_repairs" yaml:"max_repairs"`
}
type Skill struct {
	Name   string      `json:"name" yaml:"name"`
	Source SkillSource `json:"source" yaml:"source"`
}
type SkillSource struct {
	Resource  string `json:"resource,omitempty" yaml:"resource,omitempty"`
	Path      string `json:"path,omitempty" yaml:"path,omitempty"`
	ImagePath string `json:"image_path,omitempty" yaml:"image_path,omitempty"`
}

var capabilityName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

func (command CommandTool) Validate() error {
	if len(command.Argv) == 0 || strings.TrimSpace(command.Argv[0]) == "" {
		return fmt.Errorf("command tool needs argv")
	}
	for _, arg := range command.Argv {
		if strings.ContainsRune(arg, 0) {
			return fmt.Errorf("command tool argv contains NUL")
		}
	}
	if command.TimeoutSeconds < 0 || command.TimeoutSeconds > 86400 {
		return fmt.Errorf("tool timeout_seconds must be between 0 and 86400")
	}
	return ValidateWorkingDirectory(command.WorkingDir)
}
func ValidateWorkingDirectory(dir string) error {
	if dir != "" && dir != "/workspace" && dir != "/runtime" && (!strings.HasPrefix(dir, "/workspace/") && !strings.HasPrefix(dir, "/runtime/") || path.Clean(dir) != dir || strings.ContainsRune(dir, 0)) {
		return fmt.Errorf("working_dir must be inside /workspace or /runtime")
	}
	return nil
}
func ValidateAgentCapabilities(tools []Tool, gates []Gate, skills []Skill, policy *GatePolicy, resources map[string]string, mcpNames map[string]bool) error {
	names := map[string]Tool{}
	for _, tool := range tools {
		if !capabilityName.MatchString(tool.Name) {
			return fmt.Errorf("invalid tool name %q", tool.Name)
		}
		if _, exists := names[tool.Name]; exists {
			return fmt.Errorf("duplicate tool %s", tool.Name)
		}
		if (tool.Command == nil) == (tool.MCP == nil) {
			return fmt.Errorf("tool %s needs exactly one command or mcp", tool.Name)
		}
		if tool.Command != nil {
			if err := tool.Command.Validate(); err != nil {
				return fmt.Errorf("tool %s: %w", tool.Name, err)
			}
		}
		if tool.MCP != nil && !mcpNames[tool.MCP.Ref] {
			return fmt.Errorf("tool %s references unknown MCP %s", tool.Name, tool.MCP.Ref)
		}
		names[tool.Name] = tool
	}
	seen := map[string]bool{}
	for _, gate := range gates {
		if !capabilityName.MatchString(gate.Name) || seen[gate.Name] {
			return fmt.Errorf("invalid or duplicate gate name %q", gate.Name)
		}
		seen[gate.Name] = true
		tool, exists := names[gate.Tool]
		if !exists {
			return fmt.Errorf("gate %s references unknown tool %s", gate.Name, gate.Tool)
		}
		if tool.Command == nil {
			return fmt.Errorf("gate %s requires a command tool; MCP gate predicates are not supported", gate.Name)
		}
		if gate.Expect.ExitCode < 0 || gate.Expect.ExitCode > 255 {
			return fmt.Errorf("gate %s exit_code must be between 0 and 255", gate.Name)
		}
	}
	if policy != nil && (policy.MaxRepairs < 0 || policy.MaxRepairs > 10) {
		return fmt.Errorf("gate_policy.max_repairs must be between 0 and 10")
	}
	seen = map[string]bool{}
	for _, skill := range skills {
		if !capabilityName.MatchString(skill.Name) || seen[skill.Name] {
			return fmt.Errorf("invalid or duplicate skill name %q", skill.Name)
		}
		seen[skill.Name] = true
		source := skill.Source
		if source.ImagePath != "" {
			for _, mount := range []string{"/inputs", "/outputs", "/runtime", "/workspace", "/tmp"} {
				if source.ImagePath == "/" || source.ImagePath == mount || strings.HasPrefix(source.ImagePath, mount+"/") {
					return fmt.Errorf("skill %s image_path must be outside workload mounts", skill.Name)
				}
			}
			if source.Resource != "" || source.Path != "" || !path.IsAbs(source.ImagePath) || path.Clean(source.ImagePath) != source.ImagePath || strings.ContainsAny(source.ImagePath, "\\\x00") {
				return fmt.Errorf("skill %s needs one image_path or resource/path source", skill.Name)
			}
		} else if _, exists := resources[source.Resource]; !exists || !Relative(source.Path) {
			return fmt.Errorf("skill %s has invalid resource/path source", skill.Name)
		}
	}
	return nil
}

// GateReport binds authoritative completion checks to the final candidate.
// It is executor evidence, not a security attestation against a malicious image.
type GateReport struct {
	TaskOutputs map[string]File `json:"task_outputs"`
	Version     int             `json:"version"`
	Execution   string          `json:"execution"`
	Workspace   map[string]File `json:"workspace"`
	Results     []GateResult    `json:"results"`
}
type GateResult struct {
	Name     string `json:"name"`
	Tool     string `json:"tool"`
	Status   string `json:"status"`
	ExitCode *int   `json:"exit_code"`
	Evidence string `json:"evidence"`
}

func (report GateReport) Validate(execution string, gates []Gate, candidate map[string]File) error {
	if report.Version != 1 || report.Execution != execution {
		return fmt.Errorf("gate report has incompatible version or execution")
	}
	if len(report.Workspace) != len(candidate) {
		return fmt.Errorf("gate evidence is stale: workspace changed")
	}
	for name, file := range candidate {
		if checked, ok := report.Workspace[name]; !ok || checked != file {
			return fmt.Errorf("gate evidence is stale: workspace changed at %s", name)
		}
	}
	if len(report.Results) != len(gates) {
		return fmt.Errorf("gate report does not cover every required gate")
	}
	results := map[string]GateResult{}
	for _, result := range report.Results {
		if _, exists := results[result.Name]; exists {
			return fmt.Errorf("duplicate gate result %s", result.Name)
		}
		if !Relative(result.Evidence) || !strings.HasPrefix(result.Evidence, "tools/") {
			return fmt.Errorf("gate %s has invalid evidence path", result.Name)
		}
		results[result.Name] = result
	}
	for _, gate := range gates {
		result, ok := results[gate.Name]
		if !ok || result.Tool != gate.Tool || result.Status != "passed" || result.ExitCode == nil || *result.ExitCode != gate.Expect.ExitCode {
			return fmt.Errorf("required gate %s did not pass", gate.Name)
		}
	}
	return nil
}
