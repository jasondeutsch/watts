# YAML package rendering

`render` is independent of the CLI, project settings, Watts resource kinds, and Temporal. It uses Go `text/template`, selected Sprig functions, yaml.v3, and JSON Schema validation. It does not depend on Helm or any other Watts package.

## Package layout

```text
package/
├── values.yaml                 # Optional defaults
├── values.schema.json          # Optional JSON Schema for resolved values
└── templates/
    ├── workflow.yaml.gotmpl    # Rendered to workflow.yaml
    ├── config.yaml             # Literal YAML, copied without templating
    └── _helpers.tpl            # Named helpers, no output file
```

Nested template directories are supported. Other file extensions are ignored. No package metadata or dependency registry is required. This is rendering support only; resource compilation and execution are separate concerns.

## API

```go
result, err := render.Render(packageFS,
    render.ValuesSource{Name: "team.yaml", Data: teamOverrides},
    render.ValuesSource{Name: "project.yaml", Data: projectOverrides},
)
```

The caller supplies an `fs.FS` rooted at the package and optional override bytes. Defaults load first; overrides apply in argument order. Nested mappings merge recursively. Lists and scalars replace existing values, including `false`, zero, empty strings, and null. Null is a value rather than a deletion directive. Each values source must contain exactly one YAML mapping. Inputs are not rewritten.

The result contains resolved values and files in lexical source-path order. Output paths are relative to `templates/`; `.gotmpl` is removed. Duplicate output paths fail. Whitespace-only template results are skipped; a package producing no YAML files fails. Rendered YAML must contain mappings and have valid syntax without duplicate keys. Multiple YAML documents are supported in a rendered file; the caller decides whether its resource format permits them.

If `values.schema.json` exists, validate resolved values before rendering. Local schema fragments such as `#/$defs/model` work; external schema loading is disabled. JSON Schema supplies value validation, not Watts manifest validation. Callers validate rendered resource kinds, required fields, references, and workflow rules before accepting the output.

## Template interface

Templates receive `.Values`. Missing map entries fail rendering. Use `get` or `hasKey` for optional entries rather than relying on a missing direct lookup followed by `default`.

```yaml
name: {{ .Values.name | quote }}
spec: {{ .Values.configuration | toYaml | nindent 2 }}
```

Go's native `if`, `range`, `define`, and `template` actions are available. `include` executes a named helper and returns its text for pipelines; `toYaml` serializes a value using yaml.v3.

The Sprig function list is explicit: `quote`, `squote`, `default`, `empty`, `coalesce`, `ternary`, `indent`, `nindent`, `trim`, `trimSuffix`, `trimPrefix`, `lower`, `upper`, `replace`, `contains`, `hasPrefix`, `hasSuffix`, `join`, `list`, `dict`, `get`, `hasKey`, `keys`, `sortAlpha`, `toJson`, and `fail`. Sort the result of `keys` explicitly when using it to generate output. Helm-specific context and full Helm compatibility are not provided; `tpl` and `required` are not exposed.

## Boundaries

The package performs no writes, shell execution, environment reads, or network requests. Secrets must remain references in values; callers must not pass credential contents. Functions based on time, randomness, or host access are not exposed. Repeated rendering with identical inputs produces identical output when templates use stable ordering, including sorting lists returned by `keys`.

Template symlinks are rejected, but an arbitrary `fs.FS` implementation can access whatever its caller permits. The caller must enforce the filesystem boundary, including the package root and values/schema files. Output size is capped at 4 MiB per template/helper execution; this is not a CPU, memory, or total-package sandbox. Templates are trusted code for rendering purposes. For untrusted packages, use a separately constrained process. Error messages and resolved values may include caller-supplied data and should not be treated as secret-safe logs.

## CLI integration

`watts template <package-dir> [-f <values.yaml>]... [-o <output-dir>]` is the CLI adapter. It opens the package through `os.Root`, loads override files relative to the invocation directory, and calls this API. Without `-o`, it prints a YAML document stream; with `-o`, it preserves relative output paths and refuses to overwrite files. Validation finishes before output begins. Rendering failures return exit status 1; argument errors return 2.

The command validates YAML structure and the optional values schema. It does not validate Watts resource semantics or start a workflow. Normal project/workflow loaders perform those checks when the rendered manifests are used. Workflow loading behavior has not changed; render templates explicitly before selecting their output.
