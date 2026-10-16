---
name: acl-style
description: Write, review, or refactor Watts ACL packages and modules for readable, consistent source. Use for .acl and .wvars authoring, not application code or compiled task definitions.
---

# ACL style

Apply these conventions when authoring or reviewing Watts packages. Preserve
execution behavior during a style-only change. Follow explicit team conventions
where they differ; language requirements always take precedence over style.

Read [the ACL language specification](../../../docs/language-spec.md) when checking
supported fields, evaluation scope, or routing. Resolve this link relative to the
skill, not the target project. If using a standalone copy, locate the matching
Watts specification rather than guessing from Terraform syntax.

## Formatting

Use two-space indentation, double-quoted labels, aligned adjacent assignments,
and a final newline. Separate declarations and logical sections with blank lines.
Put attributes before nested blocks. Prefer `#` comments that explain intent.
These conventions are inspired by [HashiCorp's Terraform style guide](https://developer.hashicorp.com/terraform/language/style).

Use multiline declaration and contract blocks, even for short bodies. Prefer
blocks for fixed contracts (`container`, `command`, `source`, `expect`); use object
attributes for maps (`environment`, `files`, `resources`, `variables`, `templates`)
and for imported object values such as `command = module.go-tools.outputs.test`.
Do not replace a typed reference with copied configuration just to obtain a block.

Keep short scalar lists on one line. Expand long artifact dependency lists to one
item per line, with trailing commas. Use `file()` for substantial prompts and
`jsonencode()` for JSON adapter configuration instead of hand-escaped strings.

## Names and organization

ACL declarations and stages use lower-case hyphenated names, such as `plan-review`.
Variables, locals, and output aliases use `snake_case`. Names describe responsibility;
avoid repeating a declaration type already present in its address. Do not rename
public declarations or exports merely for style without checking their consumers.

For a package with several executors, use this layout where it improves navigation:

- `watts.acl`: package identity, version, declared assets, variable files, entrypoint.
- `variables.acl`: input variables, alphabetically ordered.
- `modules.acl`: reusable local module dependencies and their supplied inputs.
- `agents.acl`, `procedures.acl`: executor contracts.
- `workflow.acl`: stage sequence, named artifact bindings, and outcome routes.
- `values.wvars`: nonsecret project defaults.
- `modules/<name>/`: local modules; keep small modules in one `watts.acl`.

These filenames are conventions, not dependency or override rules. ACL loads all
immediate `.acl` files as one module. Do not split a workflow declaration across
files. Keep Agent declarations in their principal workflow order when helpful;
**never alphabetize stage blocks**: their order controls default execution.

## Contracts that read in execution order

For an Agent, start with identity/default references, model settings when explicit,
prompt, input/output artifacts, and execution limits/environment. Follow with its
container, skills, callable tools, completion gates, and gate repair policy.
Place tools before gates that reference them. Keep a tool's description before its
command or MCP block, and a gate's tool reference before its expected result.

For a container, put image and command first, then working directory, workspace,
network/user/result policy, resource limits, private files, and resource mounts.
For a command, use `argv`, `working_dir`, then `timeout_seconds`. In a workflow,
put overall limits/scaffolds before stages; inside a stage put executor, inputs,
outputs, and parameters before routing blocks. Use consistent ordering rather
than rearranging contracts into unrelated alphabetical fields.

An Agent-owned quality gate belongs with that Agent's tools. Do not create a
separate stage merely to satisfy a formatting pattern. Keep deterministic
Procedures and approval/event Procedures distinguishable through `execution.type`.

## Inputs, modules, and connections

Give each variable a `type` and a concise `description`; put `default` last when
needed. Keep team defaults in the declared `.wvars` file and module fallback
values in variable declarations. Describe units and adapter-specific meanings.
Expose inputs that consumers need to vary; avoid making every literal a variable.

Use local modules for reusable contracts with a clear responsibility. Put `source`
and an exact `version` before the module's `variables` map. Export useful values
or references with `output "name" { value = ... }`; ACL outputs do not accept Terraform's
`description`, `sensitive`, or `type` attributes. Avoid unnecessary locals and
extra modules that only hide a single straightforward assignment.

Connect stages through `stage.<name>.outputs.<alias>` where an earlier named output
exists. Use declaration references for executors, credential sources, and route
targets rather than reconstructing qualified names. Keep outcome routes readable;
backward or same-stage `on` routes require `max_iterations`. Do not change routing,
timeouts, permissions, retry limits, or gate policy as part of cosmetic cleanup.

Declare credential source names through `secret` and bind its keys at consumers.
Keep credential values out of source, variable files, and prompts. Compilation
embeds values and declared assets; it does not provide a Terraform `sensitive`
flag. Agent adapters, tools, skills, and MCP transport policies remain explicit.

## Check the result

Compile the package using the available `wctl`, printing the complete artifact
without creating files:

```sh
wctl exec ./path/to/package --dry-run
```

Compilation validates source and contracts without executing workloads. There is
no `wctl fmt` command currently; an HCL-aware formatter may normalize whitespace,
but review ACL ordering and contracts separately. Do not prescribe `terraform fmt`,
Terraform state, providers, meta-arguments, or remote registries for ACL.

For a refactor, compare the resolved workflow before and after. Stage order,
commands, permissions, gates, model configuration, artifacts, and route limits
should remain equivalent. Source digests and captured resource roots can change
with formatting; compare captured asset contents rather than requiring identical
package hashes. JSON configuration strings may differ in key order while decoding
to the same adapter input. Leave compiled artifacts and existing task snapshots
untouched; change authored source and compile a new artifact.
