{{ define "watts.agent-capabilities" }}
{{ with (get . "tools") }}
  tools:
{{ range . }}
    - name: {{ .name | quote }}
{{ if hasKey . "description" }}
      description: {{ .description | quote }}
{{ end }}
{{ with (get . "command") }}
      command:
        argv: {{ .argv | toYaml | nindent 10 }}
{{ if hasKey . "working_dir" }}
        working_dir: {{ .working_dir | quote }}
{{ end }}
{{ if hasKey . "timeout_seconds" }}
        timeout_seconds: {{ .timeout_seconds }}
{{ end }}
{{ end }}
{{ with (get . "mcp") }}
      mcp:
        ref: {{ .ref | quote }}
{{ end }}
{{ end }}
{{ end }}
{{ with (get . "gates") }}
  gates:
{{ range . }}
    - name: {{ .name | quote }}
      tool: {{ .tool | quote }}
      expect:
        exit_code: {{ .expect.exit_code }}
{{ end }}
{{ end }}
{{ with (get . "gate_policy") }}
  gate_policy:
    max_repairs: {{ .max_repairs }}
{{ end }}
{{ with (get . "skills") }}
  skills:
{{ range . }}
    - name: {{ .name | quote }}
      source:
{{ if hasKey .source "image_path" }}
        image_path: {{ .source.image_path | quote }}
{{ end }}
{{ if hasKey .source "resource" }}
        resource: {{ .source.resource | quote }}
        path: {{ .source.path | quote }}
{{ end }}
{{ end }}
{{ end }}
{{ end }}
