# Workload images

Example image implementations of the [Watts container contract](../docs/container-workloads.md):

- [Pi/Ralph](pi_ralph/README.md): image-owned launcher, model/MCP/skill translation, and nested telemetry.
- [Golang coder](coder_golang/README.md): a team image extending Pi/Ralph with Go, gopls, and golangci-lint.

Watts does not require a shared base image or wrapper for ordinary command
Procedures. An Agent or outcome-producing Procedure must implement the common
result interface. Runtime-specific dependencies and code stay in these images.
This directory contains image source, not a remote image or package registry.

## Build locally

From the Watts checkout, run `make images`. Docker Buildx is required (bundled
with current Docker Desktop). This uses a [Bake dependency graph](https://docs.docker.com/build/bake/contexts/#using-a-target-as-a-build-context):
`coder-golang` consumes `pi-ralph` through the named context `watts-agent-base`.
It never resolves that dependency through Docker Hub. Both outputs are loaded
into the active Docker context's Engine with tags `watts/pi-ralph:0.1.0` and
`watts/coder-golang:0.1.0` and verified for the Engine's native Linux architecture.
Upstream Node/Go images and apt/npm/Go packages still require network access.

The script prints the context, builder driver, platform, build step failures, and
loaded images. It selects the context-owned `docker` driver for this invocation,
without changing your selected builder/context. Remote and `docker-container`
builders are outside this supported setup path. A conflicting
`DOCKER_DEFAULT_PLATFORM` fails before building; this command builds for the
worker's engine, not multiple architectures at once.

A builder cache is not the Engine image store. Here the base is a graph dependency
and `--load` exports both final images; a separate `--load` followed by an
independent builder resolving a tag does not provide the same guarantee.
Do not run the Go Dockerfile as an independent `docker build`: its named base
context is supplied by Bake.

If the CLI cannot discover a bundled Buildx plugin, enable/install it or supply
its executable path for this invocation: `BUILDX=/path/to/docker-buildx make images`.
No plugin, builder, or Docker configuration is installed or changed by this script.
`WATTS_IMAGE_TAG=local-check make images` changes output tags for an isolated
verification build; update authored image references if you intend to use them.

Watts uses the Docker SDK environment, not the CLI's context selection. For a
nondefault local context, export its Unix Engine endpoint as `DOCKER_HOST` before
both building and starting Watts. `wctl doctor --compiled <artifact>` verifies
that the worker engine has the selected images. Different engine stores do not
share local tags automatically. Use one local Engine for this prototype.
