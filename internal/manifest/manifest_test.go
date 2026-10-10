package manifest_test

import (
	"strings"
	"testing"

	"github.com/jasondeutsch/watts/internal/manifest"
	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/stretchr/testify/require"
)

const validWorkflow = `kind: Workflow
schema_version: 1
name: example
spec:
  steps:
    - name: build
      command: "true"
`

func TestRejectInvalidManifests(t *testing.T) {
	for _, test := range []struct{ name, source, message string }{
		{"wrong kind", strings.Replace(validWorkflow, "kind: Workflow", "kind: Project", 1), "expected kind"},
		{"unsupported schema", strings.Replace(validWorkflow, "schema_version: 1", "schema_version: 2", 1), "unsupported schema_version"},
		{"missing schema", strings.Replace(validWorkflow, "schema_version: 1", "", 1), "schema_version"},
		{"missing name", strings.Replace(validWorkflow, "name: example", "", 1), "name must not be empty"},
		{"envelope typo", "typo: true\n" + validWorkflow, "line 1"},
		{"nested typo", validWorkflow + "      timeot_seconds: 10\n", "timeot_seconds"},
		{"duplicate key", validWorkflow + "      command: false\n", "already defined"},
		{"invalid syntax", "kind: [", "invalid YAML"},
		{"multiple documents", validWorkflow + "---\n" + validWorkflow, "duplicate resource"},
		{"wrong field type", strings.Replace(validWorkflow, "schema_version: 1", "schema_version: nope", 1), "invalid YAML"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := manifest.Decode([]byte(test.source), manifest.Workflow, orchestration.Definition{})
			require.ErrorContains(t, err, test.message)
		})
	}
}

func TestMultilineInstructionsRoundTrip(t *testing.T) {
	definition := orchestration.Definition{Steps: []orchestration.Step{{Name: "build", Agent: "build", Identity: "builder", Provider: "openrouter", Model: "coding-model", Prompt: "Read SPEC.md.\nPreserve \\\n and \"quotes\".\n"}}}
	data, err := manifest.Encode(manifest.Workflow, "example", definition)
	require.NoError(t, err)
	require.Contains(t, string(data), "prompt: |")
	document, err := manifest.Decode(data, manifest.Workflow, orchestration.Definition{})
	require.NoError(t, err)
	require.Equal(t, definition.Steps[0].Prompt, document.Spec.Steps[0].Prompt)
}
