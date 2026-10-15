package workflowlang

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func coderVariables(t *testing.T) (string, map[string]cty.Value) {
	t.Helper()
	source := filepath.Join("..", "..", "resources", "examples", "coder_golang")
	data, err := os.ReadFile(filepath.Join(source, "values.wvars"))
	require.NoError(t, err)
	values := map[string]cty.Value{}
	require.NoError(t, decodeVariables(data, "values.wvars", values))
	return source, values
}

func coderStep(t *testing.T, pkg Package, name string) orchestration.Step {
	t.Helper()
	for _, step := range pkg.Definition.Steps {
		if step.Name == name {
			return step
		}
	}
	t.Fatalf("missing step %s", name)
	return orchestration.Step{}
}

func TestCoderAgentConfigurationThroughConsumerModule(t *testing.T) {
	source, values := coderVariables(t)
	profiles := values["providers"].AsValueMap()
	profiles["corporate"] = cty.ObjectVal(map[string]cty.Value{
		"api":         cty.StringVal("openai-completions"),
		"api_key_env": cty.StringVal("CORPORATE_API_KEY"),
		"base_url":    cty.StringVal("https://models.example.test/v1"),
	})
	values["providers"] = cty.ObjectVal(profiles)
	agents := values["agents"].AsValueMap()
	review := agents["review"].AsValueMap()
	review["provider"] = cty.StringVal("corporate")
	review["model"] = cty.StringVal("review-model")
	review["thinking"] = cty.StringVal("high")
	review["image"] = cty.StringVal("example/reviewer:0.1.0")
	review["context_window"] = cty.NumberIntVal(64000)
	review["max_output_tokens"] = cty.NumberIntVal(8000)
	review["reasoning"] = cty.False
	agents["review"] = cty.ObjectVal(review)
	values["agents"] = cty.ObjectVal(agents)

	root := t.TempDir()
	absolute, err := filepath.Abs(source)
	require.NoError(t, err)
	write(t, root, Filename, `package "team" {
 version = "0.1.0"
 entrypoint = module.coder.workflow.coding
}
variable "agents" { type = any }
variable "globals" { type = any }
variable "providers" { type = any }
module "coder" {
 source = `+stringQuote(absolute)+`
 version = "0.1.0"
 variables = { agents = var.agents, globals = var.globals, providers = var.providers }
}
`)
	pkg, err := Load(root, Options{Variables: values})
	require.NoError(t, err)
	build := coderStep(t, pkg, "build")
	reviewer := coderStep(t, pkg, "review")
	require.Equal(t, "openrouter", build.Provider)
	require.Equal(t, "https://openrouter.ai/api/v1", build.AgentSettings.BaseURL)
	require.Equal(t, map[string]string{"OPENROUTER_API_KEY": "OPENROUTER_API_KEY"}, build.SecretEnvironment)
	require.Equal(t, "corporate", reviewer.Provider)
	require.Equal(t, "review-model", reviewer.Model)
	require.Equal(t, "high", reviewer.AgentSettings.Thinking)
	require.Equal(t, "https://models.example.test/v1", reviewer.AgentSettings.BaseURL)
	require.Equal(t, map[string]string{"CORPORATE_API_KEY": "CORPORATE_API_KEY"}, reviewer.SecretEnvironment)
	require.Equal(t, "example/reviewer:0.1.0", reviewer.Container.Image)
	var launcher struct {
		ModelConfig struct {
			ContextWindow int
			MaxTokens     int
			Reasoning     bool
		} `json:"model_config"`
	}
	require.NoError(t, json.Unmarshal([]byte(reviewer.Container.Files["launcher.json"]), &launcher))
	require.Equal(t, 64000, launcher.ModelConfig.ContextWindow)
	require.Equal(t, 8000, launcher.ModelConfig.MaxTokens)
	require.False(t, launcher.ModelConfig.Reasoning)

	// Only declarations that explicitly select the shared image change.
	values["globals"] = cty.ObjectVal(map[string]cty.Value{"image": cty.StringVal("example/shared:0.1.0")})
	changed, err := Load(root, Options{Variables: values})
	require.NoError(t, err)
	require.Equal(t, agentContract(reviewer), agentContract(coderStep(t, changed, "review")))
	for _, name := range []string{"specification", "planning", "build"} {
		before, after := coderStep(t, pkg, name), coderStep(t, changed, name)
		require.Equal(t, "example/shared:0.1.0", after.Container.Image)
		after.Container.Image = before.Container.Image
		require.Equal(t, agentContract(before), agentContract(after))
	}

	// Compiled contracts preserve per-Agent configuration without reading secrets.
	t.Setenv("CORPORATE_API_KEY", "fixture-secret-never-captured")
	data, err := json.Marshal(pkg)
	require.NoError(t, err)
	require.NotContains(t, string(data), "fixture-secret-never-captured")
	artifact := filepath.Join(t.TempDir(), "workflow.json")
	require.NoError(t, os.WriteFile(artifact, data, 0600))
	captured, err := ReadCompiled(artifact)
	require.NoError(t, err)
	require.Equal(t, reviewer, coderStep(t, captured, "review"))
}

func stringQuote(value string) string { data, _ := json.Marshal(value); return string(data) }

func TestCoderRejectsMissingAgentBindings(t *testing.T) {
	for _, name := range []string{"provider", "agent", "model"} {
		t.Run(name, func(t *testing.T) {
			source, values := coderVariables(t)
			agents := values["agents"].AsValueMap()
			switch name {
			case "provider":
				review := agents["review"].AsValueMap()
				review["provider"] = cty.StringVal("missing-provider")
				agents["review"] = cty.ObjectVal(review)
			case "agent":
				delete(agents, "review")
			case "model":
				review := agents["review"].AsValueMap()
				delete(review, "model")
				agents["review"] = cty.ObjectVal(review)
			}
			values["agents"] = cty.ObjectVal(agents)
			_, err := Load(source, Options{Variables: values})
			require.Error(t, err)
			require.Contains(t, err.Error(), map[string]string{"provider": "Invalid index", "agent": "review", "model": "model"}[name])
		})
	}
}

// Package captures receive new content addresses when variable inputs change.
// Compare executor behavior independently of that provenance address.
func agentContract(step orchestration.Step) orchestration.Step {
	container := *step.Container
	container.Resources = nil
	step.Container = &container
	return step
}
