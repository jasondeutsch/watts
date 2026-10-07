package application

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
)

// customPi puts a pi stand-in first on PATH. body runs for every call that is not --version,
// list or install, with PI_CODING_AGENT_DIR and the arguments available.
func customPi(t *testing.T, body string) {
	t.Helper()
	bin := t.TempDir()
	pi := "#!/usr/bin/env bash\ncase \"${1:-}\" in\n  --version) echo \"pi 1.0.1\"; exit 0 ;;\n  list) echo \"" + RalphPackage + "\"; exit 0 ;;\n  install) exit 0 ;;\nesac\n" + body + "\n"
	for name, content := range map[string]string{"pi": pi, "node": "#!/bin/sh\necho v22.22.1\n", "npm": "#!/bin/sh\nexit 0\n"} {
		require.NoError(t, os.WriteFile(filepath.Join(bin, name), []byte(content), 0o755))
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// sessionLine is a session record entry like the ones Pi writes.
func sessionScript(model string) string {
	return `mkdir -p "$PI_CODING_AGENT_DIR/sessions/s"
printf '%s\n' '{"type":"message","timestamp":"2026-10-05T16:14:10.323Z","message":{"role":"assistant","model":"` + model + `","content":[{"type":"text","text":"working on it"},{"type":"toolCall","name":"bash","arguments":{"command":"ls"}}]}}' >> "$PI_CODING_AGENT_DIR/sessions/s/a.jsonl"`
}
func loadCfg(t *testing.T, ta *testApp) projectconfig.Config {
	t.Helper()
	cfg, err := ta.LoadConfig()
	require.NoError(t, err)

	return cfg
}
func saveCfg(t *testing.T, ta *testApp, cfg projectconfig.Config) {
	t.Helper()
	{
		err := ta.SaveConfig(cfg)
		require.NoError(t, err,
			"save: %v", err)
	}
}
func gatewayConfig(ta *testApp, t *testing.T, base string) projectconfig.Config {
	t.Helper()
	cfg := loadCfg(t, ta)
	setWorkflowModels(&cfg, "gw", "", "")
	setWorkflowModels(&cfg, "", "m1", "m2")
	cfg.Providers = map[string]projectconfig.Provider{"gw": {Name: "Gateway", BaseURL: base, APIKeyEnv: "GW_KEY",
		Models: []projectconfig.ProviderModel{{ID: "m1", Reasoning: true, ContextWindow: 200000, MaxTokens: 16384}, {ID: "m2"}}}}
	return cfg
}

// taskRepo returns an initialised repository with a Tier 0 task whose plan has two stories.
func taskRepo(t *testing.T) (*testApp, string) {
	t.Helper()
	ta := newProject(t)
	ta.initRepo(t)
	{
		_, err := ta.CreateTask(loadCfg(t, ta), "demo", "")
		require.NoError(t, err)
	}

	matches, _ := filepath.Glob(filepath.Join(ta.Root, "tasks", "*-demo"))
	require.Len(t, matches, 1,
		"task folder not created: %v", matches)

	task := "tasks/" + filepath.Base(matches[0])
	writeFile(t, filepath.Join(ta.Root, task, "SPEC.md"), `# SPEC: demo

## Status
Draft
Version: v1

## Acceptance criteria
- AC-1: While idle, when polled, the system shall answer.
- AC-2: While busy, when polled, the system shall queue.

## Risk tier
0

## Approved by
<name, date>
`)
	writeFile(t, filepath.Join(ta.Root, task, "PLAN.md"), `# PLAN: demo

Spec ref: SPEC.md v1

## Risk tier recheck

Risk tier after planning: 0

## Quality gates

- `+"`true`"+`

## User stories

### US-001: First

Covers: AC-1
Depends on: none

### US-002: Second

Covers: AC-2
Depends on: US-001

## Approved by

<name, date>
`)

	return ta, task
}

// gitTripwire puts a fake git first on PATH. It records every call and fails it, so a test can
// prove that Watts never ran git. It returns the file the calls are written to.
func gitTripwire(t *testing.T) string {
	t.Helper()
	bin := t.TempDir()
	log := filepath.Join(bin, "git-was-called.txt")
	script := "#!/bin/sh\necho \"$@\" >> \"" + log + "\"\nexit 127\n"

	require.NoError(t, os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o755))

	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Cleanup(func() {
		assert.NoFileExists(t, log, "Watts must never run Git")
	})
	return log
}
