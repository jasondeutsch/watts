# Pi with Ralph runtime image

Base image for an Agent using Pi with Ralph as a tool. Includes Node.js 22.22.1, Pi 1.0.3, Ralph 2.1.0, CA certificates, and ripgrep. Runs as the unprivileged `node` user with `/workspace` as its working directory. No model, provider, credentials, workflow, or team skills are baked into it.

Build from the Watts repository:

```sh
docker build -t watts/pi-ralph:0.1.0 registry/pi_ralph
```

Inspect the installed runtime without calling a model:

```sh
docker run --rm watts/pi-ralph:0.1.0 --version
docker run --rm watts/pi-ralph:0.1.0 list
```

Pi loads Ralph from its installed local package path; runtime startup does not need to download the extension. `PI_CODING_AGENT_DIR` defaults to `/home/node/.pi/agent`. A future adapter that replaces this directory or its settings must retain the local Ralph package binding from `settings.json`. Private state must be writable by the container user and isolated per attempt.

The entrypoint is `pi`; arguments are passed directly to it. This is a runtime image, not a Watts worker. Container orchestration, adapter configuration, credentials, workspace isolation, usage capture, cancellation, and durable output publication remain proposed integration work. Running this image alone does not establish the Watts execution contract or prevent Ralph from creating workspace-relative registries.

## Team dependencies

Extend this image to supply the workload's required tools. It does not contain Go, golangci-lint, or gopls and therefore is not yet the complete Go generalist image. Skills, instructions, provider/model configuration, and credential references come from resolved Agent manifests at launch.

Use existing image build tools outside Watts. Version arguments permit deliberate upgrades; validate compatible Pi/Ralph versions together. Direct software versions are pinned, but the Node base image is pinned by digest; apt packages and npm transitive dependencies are not fully locked. Publish and reference a tested image digest for immutable execution. No image has been published to a remote registry by adding these files.
