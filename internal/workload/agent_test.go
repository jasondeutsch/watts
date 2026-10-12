package workload

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAgentCapabilitiesRejectInvalidBindings(t *testing.T) {
	tool := Tool{Name: "lint", Command: &CommandTool{Argv: []string{"lint"}}}
	gate := Gate{Name: "lint-clean", Tool: "lint"}
	validate := func(tools []Tool, gates []Gate, skills []Skill, policy *GatePolicy) error {
		return ValidateAgentCapabilities(tools, gates, skills, policy, map[string]string{"team": "team"}, map[string]bool{"gopls": true})
	}
	require.NoError(t, validate([]Tool{tool, {Name: "symbols", MCP: &MCPTool{Ref: "gopls"}}}, []Gate{gate}, []Skill{{Name: "team", Source: SkillSource{Resource: "team", Path: "skills/go"}}}, nil))
	require.NoError(t, validate(nil, nil, nil, &GatePolicy{MaxRepairs: 2})) // A shared policy may be inactive when gates are disabled.
	for name, run := range map[string]func() error{
		"duplicate tool": func() error { return validate([]Tool{tool, tool}, nil, nil, nil) },
		"two implementations": func() error {
			both := tool
			both.MCP = &MCPTool{Ref: "gopls"}
			return validate([]Tool{both}, nil, nil, nil)
		},
		"missing command": func() error { return validate([]Tool{{Name: "empty"}}, nil, nil, nil) },
		"escaped directory": func() error {
			return validate([]Tool{{Name: "bad", Command: &CommandTool{Argv: []string{"true"}, WorkingDir: "/workspace/../inputs"}}}, nil, nil, nil)
		},
		"negative timeout": func() error {
			return validate([]Tool{{Name: "bad", Command: &CommandTool{Argv: []string{"true"}, TimeoutSeconds: -1}}}, nil, nil, nil)
		},
		"unknown mcp":       func() error { return validate([]Tool{{Name: "bad", MCP: &MCPTool{Ref: "unknown"}}}, nil, nil, nil) },
		"unknown gate tool": func() error { return validate(nil, []Gate{gate}, nil, nil) },
		"duplicate gate":    func() error { return validate([]Tool{tool}, []Gate{gate, gate}, nil, nil) },
		"mcp gate": func() error {
			return validate([]Tool{{Name: "lint", MCP: &MCPTool{Ref: "gopls"}}}, []Gate{gate}, nil, nil)
		},
		"unbounded repair": func() error { return validate([]Tool{tool}, []Gate{gate}, nil, &GatePolicy{MaxRepairs: 11}) },
		"escaped skill": func() error {
			return validate(nil, nil, []Skill{{Name: "bad", Source: SkillSource{Resource: "team", Path: "../secret"}}}, nil)
		},
		"mutable image skill": func() error {
			return validate(nil, nil, []Skill{{Name: "bad", Source: SkillSource{ImagePath: "/runtime/skills"}}}, nil)
		},
		"mixed skill source": func() error {
			return validate(nil, nil, []Skill{{Name: "bad", Source: SkillSource{ImagePath: "/opt/skills", Resource: "team", Path: "skills"}}}, nil)
		},
	} {
		t.Run(name, func(t *testing.T) { require.Error(t, run()) })
	}
}
func TestGateEvidenceMustCoverFinalCandidate(t *testing.T) {
	code := 0
	candidate := map[string]File{"main.go": {SHA256: "source", Mode: 0644}}
	gate := Gate{Name: "tests-pass", Tool: "test"}
	report := GateReport{Version: 1, Execution: "run", Workspace: candidate, Results: []GateResult{{Name: gate.Name, Tool: gate.Tool, Status: "passed", ExitCode: &code, Evidence: "tools/call.json"}}}
	require.NoError(t, report.Validate("run", []Gate{gate}, candidate))
	require.ErrorContains(t, report.Validate("other", []Gate{gate}, candidate), "execution")
	require.ErrorContains(t, report.Validate("run", []Gate{gate}, map[string]File{"main.go": {SHA256: "later-edit", Mode: 0644}}), "stale")
	missing := report
	missing.Results = nil
	require.ErrorContains(t, missing.Validate("run", []Gate{gate}, candidate), "every required gate")
	failed := report
	failed.Results = append([]GateResult{}, report.Results...)
	failed.Results[0].Status = "failed"
	require.ErrorContains(t, failed.Validate("run", []Gate{gate}, candidate), "did not pass")
	duplicate := report
	duplicate.Results = append(append([]GateResult{}, report.Results...), report.Results...)
	require.Error(t, duplicate.Validate("run", []Gate{gate}, candidate))
}
