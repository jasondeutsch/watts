package orchestration

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"
)

type Definition struct {
	CaptureBaseline bool              `json:"capture_baseline,omitempty" yaml:"capture_baseline,omitempty"`
	Version         int               `json:"version,omitempty" yaml:"version,omitempty"`
	Steps           []Step            `json:"steps" yaml:"steps"`
	MaxTransitions  int               `json:"max_transitions,omitempty" yaml:"max_transitions,omitempty"`
	Templates       map[string]string `json:"templates,omitempty" yaml:"templates,omitempty"`
}

type Check struct {
	Script string   `json:"script" yaml:"script"`
	Args   []string `json:"args,omitempty" yaml:"args,omitempty"`
}

type AgentSettings struct {
	Thinking   string   `json:"thinking,omitempty" yaml:"thinking,omitempty"`
	SkillsDirs []string `json:"skills_dirs,omitempty" yaml:"skills_dirs,omitempty"`
	BaseURL    string   `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	API        string   `json:"api,omitempty" yaml:"api,omitempty"`
	APIKeyEnv  string   `json:"api_key_env,omitempty" yaml:"api_key_env,omitempty"`
	Sandbox    string   `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
}

type Step struct {
	AgentSettings     *AgentSettings    `json:"agent_settings,omitempty" yaml:"agent_settings,omitempty"`
	Environment       map[string]string `json:"environment,omitempty" yaml:"environment,omitempty"`
	SecretEnvironment map[string]string `json:"secret_environment,omitempty" yaml:"secret_environment,omitempty"`
	MCPs              []string          `json:"mcps,omitempty" yaml:"mcps,omitempty"`
	Name              string            `json:"name" yaml:"name"`
	Capability        string            `json:"capability,omitempty" yaml:"capability,omitempty"`
	Parameters        map[string]any    `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	Event             bool              `json:"event,omitempty" yaml:"event,omitempty"`
	ResultFile        string            `json:"result_file,omitempty" yaml:"result_file,omitempty"`
	Transitions       map[string]string `json:"transitions,omitempty" yaml:"transitions,omitempty"`
	Identity          string            `json:"identity,omitempty" yaml:"identity,omitempty"`
	Provider          string            `json:"provider,omitempty" yaml:"provider,omitempty"`
	Model             string            `json:"model,omitempty" yaml:"model,omitempty"`
	Agent             string            `json:"agent,omitempty" yaml:"agent,omitempty"`
	Prompt            string            `json:"prompt,omitempty" yaml:"prompt,omitempty"`
	Command           string            `json:"command,omitempty" yaml:"command,omitempty"`
	Check             string            `json:"check,omitempty" yaml:"check,omitempty"`
	Args              []string          `json:"args,omitempty" yaml:"args,omitempty"`
	Human             bool              `json:"human,omitempty" yaml:"human,omitempty"`
	Optional          bool              `json:"optional,omitempty" yaml:"optional,omitempty"`
	Inputs            []string          `json:"inputs,omitempty" yaml:"inputs,omitempty"`
	Outputs           []string          `json:"outputs,omitempty" yaml:"outputs,omitempty"`
	PreChecks         []Check           `json:"pre_checks,omitempty" yaml:"pre_checks,omitempty"`
	Checks            []Check           `json:"checks,omitempty" yaml:"checks,omitempty"`
	Next              string            `json:"next,omitempty" yaml:"next,omitempty"`
	OnFailure         string            `json:"on_failure,omitempty" yaml:"on_failure,omitempty"`
	TimeoutSeconds    int               `json:"timeout_seconds,omitempty" yaml:"timeout_seconds,omitempty"`
	MaxAttempts       int               `json:"max_attempts,omitempty" yaml:"max_attempts,omitempty"`
}

// AgentIdentity returns the identity declared by a workflow's agent stages.
func (definition Definition) AgentIdentity(agent string) string {
	for _, step := range definition.Steps {
		if step.Agent == agent && step.Identity != "" {
			return step.Identity
		}
	}
	return agent
}

func ValidName(name string) bool { return namePattern.MatchString(name) && name != "end" }

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

func ValidArtifact(artifactPath string) bool {
	if artifactPath == "" || strings.ContainsAny(artifactPath, "\\\x00") || path.IsAbs(artifactPath) {
		return false
	}
	if path.Clean(artifactPath) != artifactPath || artifactPath == "." || artifactPath == ".." || strings.HasPrefix(artifactPath, "../") {
		return false
	}
	return artifactPath != ".watts-state" && !strings.HasPrefix(artifactPath, ".watts-state/")
}

func (definition Definition) Validate() error {
	if definition.Version != 0 && definition.Version != 1 {
		return fmt.Errorf("unsupported workflow version %d", definition.Version)
	}
	if len(definition.Steps) == 0 {
		return errors.New("workflow must contain at least one step")
	}
	if definition.MaxTransitions < 0 {
		return errors.New("workflow.max_transitions must not be negative")
	}
	for artifact := range definition.Templates {
		if !ValidArtifact(artifact) {
			return fmt.Errorf("invalid template artifact path %q", artifact)
		}
	}
	names := map[string]bool{}
	for _, step := range definition.Steps {
		if !namePattern.MatchString(step.Name) || step.Name == "end" {
			return fmt.Errorf("invalid workflow step name %q", step.Name)
		}
		if names[step.Name] {
			return fmt.Errorf("workflow step %q appears twice", step.Name)
		}
		names[step.Name] = true
		if err := step.validateExecution(); err != nil {
			return err
		}
	}
	for _, step := range definition.Steps {
		targets := []string{step.Next, step.OnFailure}
		for outcome, target := range step.Transitions {
			if !ValidName(outcome) || target == "" {
				return fmt.Errorf("step %s has invalid outcome transition %q", step.Name, outcome)
			}
			targets = append(targets, target)
		}
		for _, transition := range targets {
			if transition != "" && transition != "end" && !names[transition] {
				return fmt.Errorf("step %s references unknown transition %q", step.Name, transition)
			}
		}
	}
	return definition.validateReachability()
}

func (definition Definition) successTransition(stepIndex int) string {
	if definition.Steps[stepIndex].Next != "" {
		return definition.Steps[stepIndex].Next
	}
	if stepIndex+1 < len(definition.Steps) {
		return definition.Steps[stepIndex+1].Name
	}
	return "end"
}

// validateExecution checks one stage without depending on graph traversal.
func (step Step) validateExecution() error {
	executorCount := 0
	for _, executor := range []string{step.Agent, step.Command, step.Check, step.Capability} {
		if executor != "" {
			executorCount++
		}
	}
	if step.Human || step.Event {
		executorCount++
	}
	if executorCount != 1 {
		return fmt.Errorf("step %s must set exactly one of agent, command, check, capability, human or event", step.Name)
	}
	if len(step.MCPs) > 0 && step.Agent == "" {
		return fmt.Errorf("step %s: mcps require an agent executor", step.Name)
	}
	seenMCPs := map[string]bool{}
	for _, name := range step.MCPs {
		if !ValidName(name) || seenMCPs[name] {
			return fmt.Errorf("step %s has invalid or duplicate mcp %q", step.Name, name)
		}
		seenMCPs[name] = true
	}
	if step.Identity != "" && (step.Agent == "" || strings.ContainsAny(step.Identity, "\x00\r\n")) {
		return fmt.Errorf("step %s has an invalid agent identity", step.Name)
	}
	if step.Agent != "" && strings.TrimSpace(step.Prompt) == "" {
		return fmt.Errorf("step %s needs a prompt", step.Name)
	}
	if step.TimeoutSeconds < 0 || step.MaxAttempts < 0 {
		return fmt.Errorf("step %s has a negative timeout or attempt limit", step.Name)
	}
	if step.Capability != "" && !ValidName(step.Capability) {
		return fmt.Errorf("step %s has invalid capability name %q", step.Name, step.Capability)
	}
	if step.Event && (len(step.Transitions) == 0 || step.Optional || step.ResultFile != "" || len(step.Outputs) > 0 || len(step.Checks)+len(step.PreChecks) > 0) {
		return fmt.Errorf("event step %s needs transitions and cannot have outputs, checks, result_file or optional", step.Name)
	}
	if step.Human && step.Event {
		return fmt.Errorf("step %s cannot be both human and event", step.Name)
	}
	if step.ResultFile != "" && (!ValidArtifact(step.ResultFile) || step.Human || step.Event || step.Capability != "") {
		return fmt.Errorf("step %s has invalid result_file", step.Name)
	}
	if step.Human && (step.Optional || len(step.Inputs) == 0) {
		return fmt.Errorf("human step %s needs inputs and cannot be optional", step.Name)
	}
	if step.Human && (len(step.Outputs) > 0 || len(step.Checks)+len(step.PreChecks) > 0) {
		return fmt.Errorf("human step %s approves inputs; put outputs and completion checks in an execution step", step.Name)
	}
	for _, artifactPath := range append(append([]string{}, step.Inputs...), step.Outputs...) {
		if !ValidArtifact(artifactPath) {
			return fmt.Errorf("step %s has invalid task artifact path %q", step.Name, artifactPath)
		}
	}
	for _, completionCheck := range append(append([]Check{}, step.PreChecks...), step.Checks...) {
		if !namePattern.MatchString(strings.TrimSuffix(completionCheck.Script, ".sh")) {
			return fmt.Errorf("step %s has invalid completion check %q", step.Name, completionCheck.Script)
		}
	}
	if step.Check != "" && !namePattern.MatchString(strings.TrimSuffix(step.Check, ".sh")) {
		return fmt.Errorf("step %s has invalid check %q", step.Name, step.Check)
	}
	return nil
}

func (definition Definition) validateReachability() error {
	// Reject unreachable stages, including accidental early termination.
	reached := map[string]bool{}
	var visit func(int)
	visit = func(i int) {
		if i >= len(definition.Steps) || reached[definition.Steps[i].Name] {
			return
		}
		step := definition.Steps[i]
		reached[step.Name] = true
		targets := []string{step.OnFailure}
		if len(step.Transitions) == 0 {
			targets = append(targets, definition.successTransition(i))
		}
		for _, target := range step.Transitions {
			targets = append(targets, target)
		}
		for _, target := range targets {
			for candidateIndex, candidateStep := range definition.Steps {
				if candidateStep.Name == target {
					visit(candidateIndex)
				}
			}
		}
	}
	visit(0)
	for _, step := range definition.Steps {
		if !reached[step.Name] {
			return fmt.Errorf("workflow step %s is unreachable", step.Name)
		}
	}
	return nil
}

// Target interprets a completed executor outcome. An explicit transition table is exhaustive.
func (definition Definition) Target(index int, outcome string) (string, error) {
	step := definition.Steps[index]
	if len(step.Transitions) == 0 {
		return definition.successTransition(index), nil
	}
	target, ok := step.Transitions[outcome]
	if !ok {
		return "", fmt.Errorf("step %s returned undeclared outcome %q", step.Name, outcome)
	}
	return target, nil
}
