# Sandbox (tentative)

Status: tentative and untested. Watts is not a sandbox: the `watts` CLI clears the environment and gives Pi a private home and config directory (see [runbook configuration](runbook.md#configure-the-project)), which stops ambient credentials and default credential locations leaking in. It does not stop a misbehaving agent from reading a file by absolute path or from touching anything else you can. Pi's own documentation says the same: tools and extensions [run with the permissions of the Pi process](https://pi.dev/docs/latest/security), and project trust is not a sandbox.

Use a real boundary for Tier 1 and Tier 2 work, for unattended runs, or whenever the repository holds anything sensitive.

## Options

Pi's [Isolate Pi](https://pi.dev/docs/latest/containerization) guide compares four ways to isolate Pi, and all of them isolate the whole process, so the loop's child iterations and their extensions run inside the boundary too:

- Plain Docker: the simplest container boundary. Credentials are passed into the container.
- [Docker Sandboxes](https://docs.docker.com/ai/sandboxes/): a managed microVM. The proxy can keep the real provider key on the host and substitute it on the way out, so the key never enters the VM.
- [OpenShell](https://docs.nvidia.com/openshell/about/overview): local or remote sandboxes with filesystem, process, network and credential policies.
- A Gondolin extension: runs only the built-in tools in a micro-VM and leaves the Pi process on the host. Its commands inherit the host environment, so it is not a credential boundary, and extension tools still run on the host.

For Watts, prefer one of the first three.

## Docker Sandboxes with the Pi kit

Pi's guide documents a ready-made kit. Check the [Pi kit README](https://github.com/docker/sbx-kits-contrib/tree/main/pi) for the current commands, because this is a young product.

1. Configure credentials before creating the sandbox, and do not run `/login` inside it, because that writes the real credential into the VM. For an Anthropic API key: `sbx secret set anthropic`. Other providers are in the kit README.
2. From the repository root (the folder that gets mounted), start Pi in the sandbox: `sbx run --kit "docker.io/sbx/pi-kit:latest" pi`.
3. Install the loop extension inside it, pinned, and the Watts skills, for example with `sbx exec <sandbox-name> -- pi install npm:@lnilluv/pi-ralph-loop@2.1.0`. The sandbox needs network access to the npm registry for this step.
4. Run a loop non-interactively: `sbx exec <sandbox-name> -- pi -p "/ralph --path ./tasks/<name>"`. Pi's guide documents `sbx exec <name> -- pi -p "..."` for non-interactive runs.
5. For a local Ollama on the host, the sandbox's network policy has to allow it. Docker's documentation describes allowing `localhost` ports with `sbx policy allow network`, and the proxy translates `host.docker.internal`. Treat a policy rule as global to your sbx setup until you confirm otherwise with `sbx policy ls`.

What does not carry over automatically:

- `watts` is a single static binary, so it is easy to put in a sandbox image: copy it in (or `go install` it there) and run `watts init` inside the sandboxed checkout (only `watts.yaml` needs to travel with the repository; everything under `.watts` is rebuilt). Inside a sandbox the sandbox is the isolation, so the CLI's cleared environment and private agent directories are belt and braces, not the boundary. Credentials for the provider should come from the sandbox's proxy, not from `watts config env add`.
- The check scripts under `.watts/kit/scripts` need `bash`, `awk`, `find`, `xargs` and `sha256sum` or `shasum` in the sandbox image. Workflow stages run their declared checks inside the execution environment.
- Watts needs no version-control repository or Git identity in the sandbox. Build and review use task snapshots and role names; Watts does not configure Git.
- Work on a separate filesystem copy of your project when you need isolation from your original files. Inspect the Watts snapshot diff on the host before copying changes back. Choose the mount mode according to the sandbox provider’s documentation.

## Plain Docker

Pi's guide gives a `Dockerfile.pi` and a `docker run` that mounts only the working folder and a named volume for Pi's own config. Add `bash`, copy the `watts` binary in, run `watts init` in the mounted repository, and use `watts` inside the container. Never mount your host `~/.pi/agent`, because it carries your Pi credentials and extensions.

## Questions to settle before trusting any of this

- Does `pi -p "/ralph ..."` run to completion through `sbx exec`, and do its child iterations see the extension and skills?
- Which files can the agent still reach through the mount, and which network destinations are allowed?
- Where do credentials live, and does a loop transcript ever contain them?

## References

[Isolate Pi](https://pi.dev/docs/latest/containerization), [Run Pi safely](https://pi.dev/docs/latest/security), [Docker Sandboxes](https://docs.docker.com/ai/sandboxes/), [Pi kit for Docker Sandboxes](https://github.com/docker/sbx-kits-contrib/tree/main/pi), [OpenShell](https://docs.nvidia.com/openshell/about/overview), [Gondolin](https://github.com/earendil-works/gondolin).
