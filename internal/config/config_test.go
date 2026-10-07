package config

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jasondeutsch/watts/internal/orchestration"
)

func TestDefaultConfigIsValid(t *testing.T) {
	require.NoError(t, Default().Validate())
}

func TestConfigSet(t *testing.T) {
	good := []struct {
		key  string
		vals []string
		chk  func(Config) bool
	}{
		{"offline", []string{"true"}, func(c Config) bool { return c.Offline }},
		{"env_passthrough", []string{"A_KEY,B_KEY", "C_KEY"}, func(c Config) bool { return strings.Join(c.EnvPassthrough, ",") == "A_KEY,B_KEY,C_KEY" }},
		{"env_passthrough", []string{"A", "A"}, func(c Config) bool { return len(c.EnvPassthrough) == 1 }},
		{"env_passthrough", nil, func(c Config) bool { return c.EnvPassthrough != nil && len(c.EnvPassthrough) == 0 }},
	}
	for _, g := range good {
		c := Default()
		if err := c.Set(g.key, g.vals); err != nil {
			assert.Fail(t, fmt.Sprintf("set %s %v: %v", g.key, g.vals, err))
			continue
		}
		assert.True(t, g.chk(c),
			"set %s %v: value not applied: %+v", g.key, g.vals, c)
	}

	bad := []struct {
		key  string
		vals []string
		want string
	}{
		{"nope", []string{"x"}, "unknown key"},
		{"thinking", []string{"extreme"}, "thinking"},
		{"ollama_url", []string{"ftp://x"}, "ollama_url"},
		{"pi_range", []string{"$(id)"}, "pi_range"},
		{"ralph_version", []string{"latest"}, "unknown key"},
		{"ralph_version", []string{"^2.1.0"}, "unknown key"},
		{"offline", []string{"maybe"}, "true or false"},
		{"env_passthrough", []string{"HOME"}, "cannot be passed"},
		{"env_passthrough", []string{"PI_CODING_AGENT_DIR"}, "cannot be passed"},
		{"env_passthrough", []string{"1BAD"}, "valid environment variable"},
		{"env_passthrough", []string{"A;B"}, "valid environment variable"},
	}
	for _, b := range bad {
		c := Default()
		err := c.Set(b.key, b.vals)
		if err == nil {
			assert.Fail(t, fmt.Sprintf("set %s %v: expected an error", b.key, b.vals))
			continue
		}
		assert.Contains(t, err.Error(), b.want,
			"set %s %v: error %q does not mention %q", b.key, b.vals, err, b.want)
	}
}

func TestConfigValidationRules(t *testing.T) {
	good := Default()

	require.NoError(t, good.Validate())

	cases := []struct {
		name string
		edit func(c *Config)
		want string
	}{
		{"tasks dir escapes", func(c *Config) { c.TasksDir = "../x" }, "tasks_dir"},
		{"tasks dir absolute", func(c *Config) { c.TasksDir = "/abs" }, "tasks_dir"},
		{"tasks dir inside .watts", func(c *Config) { c.TasksDir = ".watts/tasks" }, "disposable"},
		{"tasks dir with space", func(c *Config) { c.TasksDir = "my tasks" }, "tasks_dir"},
		{"empty tasks dir", func(c *Config) { c.TasksDir = "" }, "tasks_dir"},
		{"reserved env name", func(c *Config) { c.EnvPassthrough = []string{"HOME"} }, "set by Watts itself"},
		{"negative limit", func(c *Config) { c.Limits.MaxMinutes = -1 }, "negative"},
		{"provider without models", func(c *Config) {
			c.Providers = map[string]Provider{"gw": {BaseURL: "https://x.example/v1"}}
		}, "at least one model"},
		{"key env and command", func(c *Config) {
			c.Providers = map[string]Provider{"gw": {BaseURL: "https://x.example/v1", APIKeyEnv: "K", APIKeyCommand: []string{"echo"}, Models: []ProviderModel{{ID: "m"}}}}
		}, "not both"},
		{"model not offered", func(c *Config) {
			definition := DefaultWorkflow()
			definition.Steps[0].Provider = "gw"
			c.Workflow = &definition
			c.Providers = map[string]Provider{"gw": {BaseURL: "https://x.example/v1", Models: []ProviderModel{{ID: "other"}}}}
		}, "does not list"},
		{"agent path escapes", func(c *Config) { c.Agents = map[string]AgentConfig{"sec": {Path: "../x"}} }, "relative path"},
		{"agents share a directory", func(c *Config) {
			c.Agents = map[string]AgentConfig{"sec": {Path: ".watts/pi-agent-build"}}
		}, "share the directory"},
		{"step with two kinds", func(c *Config) {
			c.Workflow = &orchestration.Definition{Steps: []orchestration.Step{{Name: "x", Command: "true", Check: "verify"}}}
		}, "exactly one"},
		{"step with unknown agent", func(c *Config) {
			c.Workflow = &orchestration.Definition{Steps: []orchestration.Step{{Name: "x", Agent: "ghost", Provider: "ollama", Model: "qwen2.5-coder:7b", Prompt: "go"}}}
		}, "not defined"},
		{"duplicate step", func(c *Config) {
			c.Workflow = &orchestration.Definition{Steps: []orchestration.Step{{Name: "x", Command: "true"}, {Name: "x", Command: "true"}}}
		}, "twice"},
	}
	for _, tc := range cases {
		c := Default()
		tc.edit(&c)
		{
			err := c.Validate()
			assert.False(t, err == nil || !strings.Contains(err.Error(), tc.want),
				"%s: want an error containing %q, got %v", tc.name, tc.want, err)
		}
	}
	c := Default()
	definition := DefaultWorkflow()
	c.Workflow = &definition
	{
		err := c.Validate()
		assert.NoError(t, err,
			"using the same review model must not block validation: %v", err)
	}
}
