# Go generalist team workflow

This example is being built for a team developing Go business applications and web services. It currently extends Watts' bundled SDLC with golangci-lint. The builder uses a team Go skill; The builder also has a [gopls MCP setup](mcp/README.md).

## Adopt the example

Initialize your Go project with Watts, then copy this directory to `resources/examples/go_generalist` inside that project. Keep its layout: the workflow and script use these project-relative paths. Merge these fields into your project's existing watts.json, preserving other agent settings:

```json
{
  "default_workflow": "resources/examples/go_generalist/workflows/go-generalist.json",
  "mcps": {
    "gopls": { "command": "gopls", "args": ["mcp"], "cwd": "." }
  },
  "agents": {
    "build": {
      "skills_dirs": ["resources/examples/go_generalist/skills"]
    }
  }
}
```

Install golangci-lint v2 on the worker's PATH, using a release compatible with your project's Go version. Use the same release locally and in CI. See the [installation instructions](https://golangci-lint.run/docs/welcome/install/).

Create a new task to adopt the workflow; existing submitted tasks retain their pinned definitions. The stages are specification, spec review, planning, plan review, build, lint, and independent review. Review rework returns to build, so lint runs again before the next review.

## Lint policy

[.golangci.yml](.golangci.yml) selects the team's linters and formatting checks. It uses the golangci-lint v2 configuration format and resolves paths relative to go.mod, without relying on Git. Edit this file to change the team's policy.

From the project root, run:

```sh
bash resources/examples/go_generalist/scripts/lint.sh
```

The script explicitly selects this configuration, checks all packages including tests, and preserves golangci-lint's exit status and diagnostic output. It does not automatically fix files. The planning agent includes the same command in story quality gates; the explicit lint workflow stage checks again after build and before review.

Lint failure blocks progression and records output in the stage's attempt log. Inspect the findings, correct the code, and retry with `watts task run <task>`. Missing tools and configuration errors also fail visibly rather than being skipped.

The script expects a single Go module at the project root. Multi-module workspace support is not part of this initial example. Linting supplements tests and review; it does not establish functional correctness.

## Builder skill

[go-builder](skills/go-builder/SKILL.md) provides implementation, error handling, service reliability, and testing guidance adapted from [samber/cc-skills-golang](https://github.com/samber/cc-skills-golang). Its source revision and MIT notice are retained alongside it.

After configuring the build role's `skills_dirs`, run `watts config apply` to copy the skill into its private Pi directory, then create a new task. The review role does not receive this directory. Specification and planning stages also use the build role, so the skill limits implementation work to the approved task phase. The build prompt explicitly directs the builder to read the skill.

## MCP tools

Follow [Builder MCP setup](mcp/README.md) to install gopls, declare its connection in watts.json, and verify a tool call. The portable [MCP configuration](mcp/mcp.json) demonstrates how teams add servers without changing Watts. The build stage selects gopls with `"mcps": ["gopls"]`; the other stages do not select it.
