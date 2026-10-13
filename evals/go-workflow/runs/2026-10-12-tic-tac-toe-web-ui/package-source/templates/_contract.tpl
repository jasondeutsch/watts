{{ define "watts.contract" }}
{{ $settings := . }}
{{ if hasKey $settings "inputs" }}
  inputs: {{ $settings.inputs | toYaml | nindent 4 }}
{{ end }}
{{ if hasKey $settings "outputs" }}
  outputs: {{ $settings.outputs | toYaml | nindent 4 }}
{{ end }}
{{ if hasKey $settings "environment" }}
  environment: {{ $settings.environment | toYaml | nindent 4 }}
{{ end }}
{{ if hasKey $settings "secrets" }}
  secrets: {{ $settings.secrets | toYaml | nindent 4 }}
{{ end }}
{{ if hasKey $settings "timeout_seconds" }}
  timeout_seconds: {{ $settings.timeout_seconds }}
{{ end }}
{{ if hasKey $settings "max_attempts" }}
  max_attempts: {{ $settings.max_attempts }}
{{ end }}
{{ with (get $settings "container") }}
  container:
{{ if hasKey . "image" }}
    image: {{ .image | quote }}
{{ end }}
{{ if hasKey . "command" }}
    command: {{ .command | toYaml | nindent 6 }}
{{ end }}
{{ if hasKey . "working_dir" }}
    working_dir: {{ .working_dir | quote }}
{{ end }}
{{ if hasKey . "user" }}
    user: {{ .user | quote }}
{{ end }}
{{ if hasKey . "network" }}
    network: {{ .network | quote }}
{{ end }}
{{ if hasKey . "workspace" }}
    workspace: {{ .workspace | quote }}
{{ end }}
{{ if hasKey . "result" }}
    result: {{ .result | quote }}
{{ end }}
{{ if hasKey . "memory_mb" }}
    memory_mb: {{ .memory_mb }}
{{ end }}
{{ if hasKey . "cpus" }}
    cpus: {{ .cpus }}
{{ end }}
{{ if hasKey . "files" }}
    files: {{ .files | toYaml | nindent 6 }}
{{ end }}
{{ if hasKey . "resources" }}
    resources: {{ .resources | toYaml | nindent 6 }}
{{ end }}
{{ end }}
{{ with (get $settings "pre_checks") }}
  pre_checks:
{{ range . }}
    - command: {{ .command | toYaml | nindent 8 }}
{{ end }}
{{ end }}
{{ with (get $settings "checks") }}
  checks:
{{ range . }}
    - command: {{ .command | toYaml | nindent 8 }}
{{ end }}
{{ end }}
{{ end }}
