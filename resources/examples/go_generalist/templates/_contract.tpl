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
{{ if hasKey $settings "result_file" }}
  result_file: {{ $settings.result_file | quote }}
{{ end }}
{{ with (get $settings "pre_checks") }}
  pre_checks:
{{ range . }}
    - script: {{ .script | quote }}
{{ with (get . "args") }}
      args: {{ . | toYaml | nindent 8 }}
{{ end }}
{{ end }}
{{ end }}
{{ with (get $settings "checks") }}
  checks:
{{ range . }}
    - script: {{ .script | quote }}
{{ with (get . "args") }}
      args: {{ . | toYaml | nindent 8 }}
{{ end }}
{{ end }}
{{ end }}
{{ end }}
