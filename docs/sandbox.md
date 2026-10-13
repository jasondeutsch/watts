# Execution isolation

Watts executes Agent and executable Procedure workloads in a container execution
environment. Currently, only the local Docker Engine backend is supported. The
controls below describe that backend; a future Podman or Kubernetes backend must
implement and verify its own isolation controls. See [the workload contract](container-workloads.md)
for mounts, access, and backend status.

The application is copied into an isolated candidate. It is read-only unless the
manifest explicitly permits writes. Task inputs/resources are read-only; outputs
and workload-private runtime state are separately writable. The live project,
worker home, Docker socket, and unrelated credentials are not mounted. The image
root filesystem is read-only, the user is nonroot, capabilities are dropped, and
privilege escalation is disabled. Network access defaults to `none`.

Validated changes are published with baseline conflict checks, confined filesystem
operations, and atomic file replacement. External edits remain explicit conflicts.
A workspace lock serializes Watts publication, not external editors. Cancellation
stops the owned container; it does not reverse already accepted changes.

This is a prototype boundary, not hardened multi-tenant isolation. Containers share
a kernel; `bridge` permits outbound networking without an allowlist. A workload
can access and print its explicitly scoped credentials. Enforce appropriate host,
Docker daemon, image, and provider trust. Strong network policy, microVMs, workload
identities, and distinct-node artifact delivery remain proposed work.

See [wiss-013](../issues.md#wiss-013-containerized-workloads) and
[local developer workspaces](../issues.md#wiss-017-sandboxed-execution-in-a-developers-local-environment).

Research inputs include [Docker Sandboxes](https://docs.docker.com/ai/sandboxes/),
[OpenShell](https://docs.nvidia.com/openshell/about/overview), and
[Gondolin](https://github.com/earendil-works/gondolin). These are not supported Watts backends.
