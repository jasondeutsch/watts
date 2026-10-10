package render_test

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/jasondeutsch/watts/internal/render"
	"github.com/stretchr/testify/require"
)

func packageFiles(files map[string]string) fstest.MapFS {
	result := fstest.MapFS{}
	for name, data := range files {
		result[name] = &fstest.MapFile{Data: []byte(data)}
	}
	return result
}

func TestRenderValuesHelpersAndLiteralYAML(t *testing.T) {
	packageFS := packageFiles(map[string]string{
		"values.yaml":            "builder:\n  model: default-model\n  enabled: true\n  attempts: 3\n  tools: [lint, test]\n  notes: original\n",
		"templates/_helpers.tpl": `{{define "builder.name"}}go-builder{{end}}`,
		"templates/agent.yaml.gotmpl": `kind: Agent
name: {{ include "builder.name" . | quote }}
spec: {{ .Values.builder | toYaml | nindent 2 }}
`,
		"templates/z-config.yaml": "kind: ConfigMap\nname: static\n",
	})
	override := render.ValuesSource{Name: "team.yaml", Data: []byte("builder:\n  model: team-model\n  enabled: false\n  attempts: 0\n  tools: [gopls]\n  notes: null\n")}
	first, err := render.Render(packageFS, override)
	require.NoError(t, err)
	require.Len(t, first.Files, 2)
	require.Equal(t, "agent.yaml", first.Files[0].Name)
	require.Equal(t, "z-config.yaml", first.Files[1].Name)
	require.Contains(t, string(first.Files[0].Data), `name: "go-builder"`)
	builder := first.Values["builder"].(map[string]any)
	require.Equal(t, "team-model", builder["model"])
	require.Equal(t, false, builder["enabled"])
	require.Equal(t, 0, builder["attempts"])
	require.Equal(t, []any{"gopls"}, builder["tools"])
	require.Nil(t, builder["notes"])
	second, err := render.Render(packageFS, override)
	require.NoError(t, err)
	require.Equal(t, first, second)
	require.Equal(t, "builder:\n  model: default-model\n  enabled: true\n  attempts: 3\n  tools: [lint, test]\n  notes: original\n", string(packageFS["values.yaml"].Data))
}

func TestOverridesApplyInOrder(t *testing.T) {
	packageFS := packageFiles(map[string]string{"templates/result.yaml.gotmpl": "value: {{.Values.value | quote}}\n"})
	result, err := render.Render(packageFS,
		render.ValuesSource{Name: "one.yaml", Data: []byte("value: first\n")},
		render.ValuesSource{Name: "two.yaml", Data: []byte("value: second\n")})
	require.NoError(t, err)
	require.Equal(t, "value: \"second\"\n", string(result.Files[0].Data))
}

func TestConditionalsLoopsAndMultilineText(t *testing.T) {
	packageFS := packageFiles(map[string]string{
		"values.yaml":                    "enabled: false\nsteps: [build, review]\nprompt: |\n  Read SPEC.md.\n  Keep \"quotes\" and \\slashes.\n",
		"templates/disabled.yaml.gotmpl": "{{if .Values.enabled}}name: enabled{{end}}",
		"templates/workflow.yaml.gotmpl": `steps:
{{range .Values.steps}}  - {{ . | quote }}
{{end}}prompt: {{ .Values.prompt | toYaml | nindent 2 }}
`,
	})
	result, err := render.Render(packageFS)
	require.NoError(t, err)
	require.Len(t, result.Files, 1)
	require.Contains(t, string(result.Files[0].Data), "  - \"build\"\n  - \"review\"")
	require.Contains(t, string(result.Files[0].Data), `Keep "quotes" and \slashes.`)
}

func TestRenderErrorsIdentifySource(t *testing.T) {
	for _, test := range []struct{ name, source, message string }{
		{"missing value", "name: {{.Values.missing}}", "missing"},
		{"syntax", "name: {{if}}", "parse broken.yaml.gotmpl"},
		{"duplicate keys", "name: a\nname: b", "already defined"},
		{"invalid YAML", "name: [", "rendered broken.yaml.gotmpl"},
		{"scalar output", "hello", "cannot unmarshal"},
		{"empty document", "---\n", "empty document"},
		{"environment access", "name: {{env \"SECRET\"}}", `function "env" not defined`},
		{"random access", "name: {{randAlphaNum 5}}", `function "randAlphaNum" not defined`},
		{"clock access", "name: {{now}}", `function "now" not defined`},
		{"network access", "name: {{getHostByName \"example.com\"}}", `function "getHostByName" not defined`},
		{"output limit", `name: {{range 5}}{{printf "%*s" 999999 "x"}}{{end}}`, "4 MiB limit"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := render.Render(packageFiles(map[string]string{"templates/broken.yaml.gotmpl": test.source}))
			require.ErrorContains(t, err, test.message)
		})
	}
}

func TestValuesErrors(t *testing.T) {
	for _, source := range []string{"one: a\none: b\n", "one: a\n---\none: b\n", "- one\n", "null\n", ""} {
		_, err := render.Render(packageFiles(map[string]string{"values.yaml": source, "templates/result.yaml": "name: result\n"}))
		require.ErrorContains(t, err, "values.yaml")
	}
}

func TestValuesSchema(t *testing.T) {
	schema := `{"type":"object","required":["model"],"additionalProperties":false,"properties":{"model":{"$ref":"#/$defs/model"}},"$defs":{"model":{"type":"string","minLength":1}}}`
	packageFS := packageFiles(map[string]string{
		"values.schema.json":          schema,
		"values.yaml":                 "model: coding-model\n",
		"templates/agent.yaml.gotmpl": "model: {{.Values.model | quote}}\n",
	})
	_, err := render.Render(packageFS)
	require.NoError(t, err)
	_, err = render.Render(packageFS, render.ValuesSource{Name: "bad.yaml", Data: []byte("model: 42\n")})
	require.ErrorContains(t, err, "values validation")
	packageFS["values.schema.json"].Data = []byte(`{"$ref":"file:///private/credentials.json"}`)
	_, err = render.Render(packageFS)
	require.ErrorContains(t, err, "external schema references are unsupported")
}

func TestPackageBoundaries(t *testing.T) {
	t.Run("duplicate output", func(t *testing.T) {
		_, err := render.Render(packageFiles(map[string]string{"templates/a.yaml": "name: a", "templates/a.yaml.gotmpl": "name: b"}))
		require.ErrorContains(t, err, "duplicate output path a.yaml")
	})
	t.Run("missing templates", func(t *testing.T) {
		_, err := render.Render(packageFiles(map[string]string{}))
		require.ErrorContains(t, err, "load templates")
	})
	t.Run("symlink", func(t *testing.T) {
		packageFS := packageFiles(map[string]string{"templates/a.yaml": "name: a"})
		packageFS["templates/a.yaml"].Mode = fs.ModeSymlink
		_, err := render.Render(packageFS)
		require.ErrorContains(t, err, "symlinks are unsupported")
	})
}
