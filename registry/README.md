# Workload images

Example image implementations of the [Watts container contract](../docs/container-workloads.md):

- [Pi/Ralph](pi_ralph/README.md): image-owned launcher, model/MCP/skill translation, and nested telemetry.
- [Go generalist](go_generalist/README.md): a team image extending Pi/Ralph with Go, gopls, and golangci-lint.

Watts does not require a shared base image or wrapper for ordinary command
Procedures. An Agent or outcome-producing Procedure must implement the common
result interface. Runtime-specific dependencies and code stay in these images.
This directory contains image source, not a remote image or package registry.
