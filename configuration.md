# Configuration

Setup, isolation, models, and how the loop is wired. You install the `watts` CLI once per machine and run `watts init` once per repository. Everything Watts needs is then in that repository's `.watts/` directory.

## Prerequisites

- Node.js 22.22.1 or later. [pi-ralph-loop 2.1.0](https://pi.dev/packages/@lnilluv/pi-ralph-loop) requires it. Pi 1.0 itself needs 22.19 or later.
- [Pi](https://pi.dev) 1.0 or later. Version 1.0.1 was current on npm when this was written. pi-ralph-loop needs Pi 0.85.1 or later, so 1.x satisfies it.
- `bash` (3.2 or later, so the macOS default works), `awk`, `find`, `xargs` and `sha256sum` or `shasum`. The check scripts use them. Watts does not use git or any other version control, so none is needed. Go 1.27 or later is needed only to build the CLI.
- Optional: [Ollama](https://pi.dev/docs/latest/models#configure-a-compatible-endpoint) for local models. A hosted provider works instead.
- Windows: the check scripts are bash, so use WSL. See [Run Pi on Windows](https://pi.dev/docs/latest/windows).

`watts doctor` checks all of this and tells you what is missing.

## Install

Once per machine, build and install the CLI. It is a single Go binary with the kit and Temporal SDK compiled in:

```
go install github.com/jasondeutsch/watts@latest
```

Or from a checkout: `go build -o watts . && mv watts /usr/local/bin/`. Then, once per repository:

```
cd your-repo
watts init
```

`watts init` initializes the current directory, even when a parent directory is already a Watts project. It writes `watts.json` there, and into `.watts/` the kit (`internal/kit/kit/scripts`, `internal/kit/kit/templates`, `internal/kit/kit/skills`, copied out of the binary) and the private agent directories. On a first run in a terminal it asks where task folders should live (default `tasks`); pass `--tasks-dir` to answer ahead of time. It writes local ignore files inside Watts-owned folders for run state, and installs the pinned extension if Pi is already present. Nothing else in your project is touched. Flags: `--tasks-dir`, `--env-passthrough NAME`, `--no-install`, and `--force`. Running it in a repository where `watts.json` exists uses that file's values as the defaults, so deleting `.watts` and running `watts init` again restores everything. Running it again creates missing assets and preserves existing kit files and a `models.json` you wrote.

If Pi is not installed, `watts install pi` runs the npm install (`npm install -g --ignore-scripts @earendil-works/pi-coding-agent@^1.0.0`) and skips it when 1.x is present. You can also use the [official script](https://earendil.com/posts/pi-1-0/) (`curl -fsSL https://pi.dev/install.sh | sh`). See the [Quickstart](https://pi.dev/docs/latest/quickstart). `watts install` does Pi, the agent directories, the extension and `doctor` in one go.

The extension is pinned on purpose: Watts maintains the Pi version range (`^1.0.0`) and exact Ralph extension version (`2.1.0`) in its source. The extension is installed as `pi install npm:@lnilluv/pi-ralph-loop@2.1.0`. A versioned npm source is [pinned](https://pi.dev/docs/latest/packages#install-and-manage-packages), so an unreviewed update cannot change your loop. Pi packages run code with your permissions, so read the source before bumping the version.

## What lives where

```
watts.json              the configuration (the one file worth keeping and sharing)
tasks/            task folders (the name is tasks_dir)
.watts/                  generated and disposable: delete it and run `watts init` to rebuild
  .gitignore             keeps the local state below out of any version control the project may use
  kit/                   check scripts, loop prompt templates, the five skills
  pi-agent-build/        private Pi config for the build agent (local)
  pi-agent-review/       private Pi config for the review agent (local)
  home-build/  home-review/   private HOME directories (local)
```

Watts does not use git or any other version control, so it makes no difference whether the project is tracked, and where these folders are stored is up to you. Watts never checks whether your working tree is clean, never switches branches and never edits your `.gitignore`. Sharing `watts.json` shares the setup. Initialization creates the bundled assets needed by workflows; existing project assets are left untouched.

## The configuration file: `watts.json`

Change a flat setting with `watts config set <key> <value>` and apply it with `watts config apply`, or edit the file. Unknown keys are rejected, so a typo is not ignored. `watts config show` prints the file, and `watts config show --resolved` prints every agent with its defaults filled in and where each value came from.

- `agents.<name>.thinking` (default `medium` when omitted)
- `providers.ollama.base_url` (default `http://localhost:11434/v1`)
- `env_passthrough`: the names of environment variables copied from your shell into every agent (see Isolation).
- `offline`: sets `PI_OFFLINE=1`
- `tasks_dir`: where task folders live, relative to the repository (default `tasks`). It cannot be inside `.watts` or `.git`.
- `snapshot_exclude`: extra paths, relative to the project, that snapshots skip (see Snapshots).
- `limits.max_minutes` and `limits.max_attempts`: see Runs
- `providers`, `agents`, `workflow`, `skills_dirs`, `pi_settings`, `forbidden_paths`: described below

## Private agent directories

`watts config apply` creates the agents `build` and `review` (and any custom agent, see below), each with its own directories under `.watts/`. It never replaces a `models.json` you wrote unless you pass `--force`, and it merges into `settings.json`, so installed packages survive:

- `.watts/pi-agent-<role>/` is that role's Pi [agent directory](https://pi.dev/docs/latest/configuration), selected through the [`PI_CODING_AGENT_DIR`](https://pi.dev/docs/latest/environment-variables#pi-process-configuration) variable. It holds `settings.json`, `models.json` (generated for Ollama and for providers declared in `watts.json`), `auth.json` if you log in, the installed packages, and a copy of the five skills.
- `.watts/home-<role>/` is a private `HOME` for the agent's local execution environment.

Workflow stages select their own models. Pi's [settings](https://pi.dev/docs/latest/settings) set `defaultProvider`, `defaultModel` and `defaultThinkingLevel` per directory, and the loop's child processes inherit the variable, so each role gets its model with the workflow stage also selecting its provider and model explicitly on the command line.

The generated `settings.json` also sets `defaultProjectTrust: "never"` and `enableInstallTelemetry: false`. Never trusting projects means Pi ignores `.pi/` in your repository (project skills, settings, extensions, MCP config), which is the safe default for unattended runs, because [project trust cannot prompt in non-interactive modes](https://pi.dev/docs/latest/security). The skills are installed at user level in the agent directory, which is not subject to project trust. Pi [discovers skills recursively](https://pi.dev/docs/latest/skills), and `watts config apply` refreshes them from `.watts/kit/skills` every time.

## Models

Each agent stage declares its provider and model in workflow.json:

```json
{
  "name": "build",
  "agent": "build",
  "provider": "ollama",
  "model": "devstral-small-2:24b",
  "prompt": "/ralph --path ./{task}"
}
```

The bundled templates use `openrouter` and `deepseek/deepseek-v4-flash`. Export `OPENROUTER_API_KEY` before starting the worker. Edit the project default workflow for future tasks, or the task's workflow.json before submission. Stages may use different models even when they share an agent role. The worker applies the current stage's settings to Pi and passes its provider and model explicitly at launch. Using the same model for build and review is allowed.

Ollama (optional):

- `ollama pull <model>` for each tag, and keep the server running. `watts config apply` writes `models.json` in the [documented shape](https://pi.dev/docs/latest/models#configure-a-compatible-endpoint): `baseUrl http://localhost:11434/v1`, `api openai-completions`, and a dummy `apiKey`. Change `providers.ollama.base_url` in `watts.json`. If you edit `models.json` yourself, `watts config apply` leaves your file alone (use `watts config apply --force` to regenerate it).
- Ollama's default context window is small for agent work. Raise it on the server (for example `OLLAMA_CONTEXT_LENGTH=32768`) and check with `ollama ps`. Pi cannot do this for you, and a truncated context causes odd failures.
- Local model quality varies. Run the smoke test in the [runbook](runbook.md) before trusting anything.

Hosted provider:

```
watts init
```

Set `provider` to `anthropic` and `model` to your chosen model ID on each relevant workflow stage. Then give Pi the credential, either by passing one environment variable through the isolation wrapper (`watts config env add ANTHROPIC_API_KEY`, see below) or by running `watts agent pi` and `/login`, which stores it in that role's private `auth.json`. Pi resolves credentials in a [fixed order](https://pi.dev/docs/latest/models#authenticate): a runtime key, then `auth.json`, then `models.json`, then environment variables. Browse model IDs in the [catalog](https://pi.dev/models) and provider setup in [Providers](https://pi.dev/docs/latest/providers). Other local options: [llama.cpp](https://pi.dev/docs/latest/llama-cpp).

## Providers and credentials

A provider that is not Ollama is declared in `watts.json`, and Watts writes `models.json` for it. The key is never stored: name the shell variable that holds it, or give a command that prints it.

```
{
  "providers": {
    "example-gateway": {
      "name": "Example Inference Gateway",
      "base_url": "https://inference.example.com/v1",
      "api_key_env": "EXAMPLE_GATEWAY_API_KEY",
      "models": [
        { "id": "example/coding-model", "reasoning": true, "context_window": 200000, "max_tokens": 16384 },
        { "id": "example/review-model", "context_window": 128000, "max_tokens": 16384 }
      ]
    }
  }
}
```

- `api_key_env` names a variable in your shell. Watts passes it into the agent automatically and generates `apiKey` as `$NAME`, for example `$OPENROUTER_API_KEY`. The key value is never stored in models.json.
- `api_key_command` is a command (an array, for example `["my-key-minter", "--ttl", "1h"]`) that Watts runs outside the sandbox at launch. Its output goes to the agent in a variable named `WATTS_APIKEY_<PROVIDER>` and is never written to disk. Use it for short-lived keys. Set `api_key_env` or `api_key_command`, not both.
- Every model an agent uses must be listed under its provider.

Before any agent starts, Watts checks that it can actually authenticate: it reads the agent's `models.json`, follows the `apiKey` it names through `env_passthrough` to your shell, and refuses to start with the real cause, for example "EXAMPLE_GATEWAY_API_KEY is not in env_passthrough, so pi will not see it" or "EXAMPLE_GATEWAY_API_KEY is not set in your shell". It never prints a key. `watts doctor` runs the same check, and `watts doctor --live` also sends one tiny completion to each gateway from outside the sandbox, so a rejected key (HTTP 401) is told apart from a key that never reached Pi.

If you wrote a `models.json` yourself, Watts leaves it alone and checks it the same way. A literal key in that file is reported as a warning, because it is a secret at rest inside the repository tree.

## Tasks and approvals

Tasks live in `tasks_dir`. A task argument can be the full folder (`tasks/2026-10-03-http-retry`) or just its name (`2026-10-03-http-retry`).

The default SDLC workflow drafts SPEC.md and PLAN.md as agent stages with intervening human gates. Approve with `watts task decide <task> spec-review` and `watts task decide <task> plan-review`. Reject with `--reject --feedback "changes needed"` to route back to drafting. Decisions approve artifact hashes and record the human identity from USER. Agents cannot submit human decisions from inside a Watts agent process.

Each task contains an editable workflow.json copied from its project template. First submission freezes that definition and runtime configuration. Project template edits do not affect existing tasks; task-local edits after submission do not affect running executions.


## Snapshots

Watts does not use git or any other version control, and it behaves the same whether or not the project is tracked by something. Where a commit would be the record of the work, Watts keeps snapshots in `<task>/.watts-state`: a manifest of the SHA-256 of every project file, plus a copy of each file up to 1 MB, stored by hash so unchanged files cost nothing. Snapshots skip `.git`, `.watts`, `watts.json`, the tasks folder, `node_modules`, `.venv` and anything in `snapshot_exclude`. Each check keeps its meaning:

- `watts task new` records a baseline snapshot in `BASE`. It creates no branch and never looks at the state of your working tree.
- When a story is finished the build agent runs `bash .watts/kit/scripts/snapshot.sh <task>` and logs `DONE US-00N <snapshot-id>` in `STORY_LOG.md`. A story counts only if that snapshot exists, was taken by the agent, and is not older than the baseline.
- Before review starts, Watts takes a `review-start` snapshot. A verdict is refused if any project file changed since then, so the reviewer can write only its verdict. Rework after a rejection needs a newer agent snapshot than the verdict.
- The reviewer's diff is `bash .watts/kit/scripts/evidence.sh <task> diff` (added, removed and modified files, with a unified diff for text files up to 1 MB).
- Approvals, stage records, limits, the protected-path check and the model check work as described elsewhere in this document.

Snapshots retain file hashes and copies for task evidence. They do not restore files or authenticate who made a change. Stored copies grow with the project, and every snapshot hashes the included files. Task documents and rendered RALPH.md prompts are task-owned artifacts.

## Temporal backend

Temporal is required for all workflow execution. A `temporal` object only overrides connection defaults. Start a service and `watts start`, then submit with `watts task run <task>`. See [temporal.md](temporal.md) and [workflows.md](workflows.md).

## Agents, skills and workflows

`build` and `review` always exist. Add your own under `agents`, and every setting not given falls back to the top level:

- `path`: the agent directory (Pi's agent directory), anywhere inside the repository. The default is `.watts/pi-agent-<name>`. Put a custom agent outside `.watts` so it can be kept and shared.
- `thinking`
- `skills_dirs`: extra directories of skills, added at the top level for every agent or on one agent. Each folder with a `SKILL.md` inside is copied into the agent's skills.
- `pi_settings`: values merged into the agent's Pi `settings.json`, at the top level for every agent or on one agent. This is the place for anything Pi documents for its settings file, including MCP servers. Watts does not interpret these keys.
- `forbidden_paths`: paths the agent may not change, in addition to `watts.json` and `.watts/kit`, which are always protected.

`watts agent new <name>` creates a custom agent in `.watts.agents/<name>` from a documented example (`.watts/kit/examples/custom-agent`) and registers it. `watts agent list` shows every agent.

Bundled base templates live in the source repository's `workflow-templates/`. Every new task receives the project default as its editable workflow.json. Set `default_workflow` in watts.json to a project-relative JSON file path to override SDLC. Edit the project template and per-task files directly. Agents declare their identities in workflow stages; provider and model selection also belong to workflow stages. Stages execute in listed order unless an explicit transition routes them elsewhere. See [workflows.md](workflows.md).

## Runs: state, limits and guards

Temporal is required for workflow execution. The optional `temporal` settings object overrides localhost:7233, the default namespace and the project-derived queue. Start a service and `watts start`; then submit with `watts task run`. Task definitions and runtime settings are pinned at first submission. `task status --json` includes every attempt's outcomes and feedback. `task run` starts or retries a task; `task cancel` cancels the execution.

Stage budgets and `max_transitions` bound retries and rework. Agent activities check protected paths and the model recorded in Pi's session. Local provenance, logs and plugin requests/results live under `<task>/.watts-state`. `task run --dry-run` shows the pinned definition without submitting. `watts agent follow [agent]` displays a session feed from a separate terminal. [temporal.md](temporal.md) describes restart recovery and stored state.

## Isolation: what the CLI does

Pi processes launched by worker activities, interactive `watts agent pi`, and extension installation use the application service:

- The environment is not inherited. Only a short list is set: `PATH`, `TERM`, `LANG`, the private `HOME`, `PI_CODING_AGENT_DIR`, `PI_SKIP_VERSION_CHECK=1`, `PI_TELEMETRY=0`, `WATTS_ROLE` (the agent's name). Your shell's API keys never reach Pi, and Pi's provider auto-detection from environment variables finds nothing.
- `HOME` is private, so `~/.ssh`, `~/.aws`, `~/.config/gh` and similar are not found by default.
- `PI_CODING_AGENT_DIR` is private, so your normal `~/.pi/agent/auth.json`, extensions and settings are not used.
- `offline` (or `--offline` on a command) adds `PI_OFFLINE=1`, which disables automatic network activity such as model catalog refreshes.
- Print-mode runs (`-p`, which is how loops start) get an empty stdin. Pi's print mode waits for stdin to close when it is not a terminal, so a loop started from a script would otherwise hang.

Pass exactly what you need, by name. `watts agent env [agent]` shows what an agent receives (names only, never values, with a note for any name that is not set in your shell), which answers "why can't Pi see my variable" directly. Setting a variable inline on the `watts` command line does not help, because the environment is built from scratch and only listed names are copied: `watts config env add ANTHROPIC_API_KEY GOCACHE GOMODCACHE`, or `--env-passthrough NAME` on a single command. Only names that are set in your shell are copied. Toolchains that keep caches under `HOME` (Go, npm, pip) start cold in the private home, so pass their cache variables or accept the first-run cost. Behind a corporate proxy that re-signs TLS, pass the certificate and proxy variables too, or `npm` fails with `SELF_SIGNED_CERT_IN_CHAIN` (`watts config env add NODE_EXTRA_CA_CERTS SSL_CERT_FILE HTTPS_PROXY`).

What this is not: a sandbox. Absolute paths to your real home are still readable, and Pi's tools run with the permissions of the process. The `protected_files` guardrail in the loop prompts blocks the agent's `write` and `edit` tools, not reads. For a real boundary see [sandbox.md](sandbox.md) and [Isolate Pi](https://pi.dev/docs/latest/containerization).

You do not need a provider-suppression extension. Per the extension's documentation the loop's children disable extension discovery, so such an extension would not load inside iterations, while a cleared environment works at the process boundary. If you still use one in your own interactive Pi, note that it lives in `~/.pi/agent`, which Watts deliberately does not read.

## How the loop runs

[pi-ralph-loop](https://pi.dev/packages/@lnilluv/pi-ralph-loop) adds `/ralph` slash commands to Pi. Facts that shape the design:

- `/ralph` is a Pi command, not a shell program. Watts starts it non-interactively as `pi -p "/ralph --path ./tasks/<name>"` through the wrapper, which is the documented way to run it from a script.
- Each iteration runs in a fresh child `pi --mode rpc` ([RPC mode](https://pi.dev/docs/latest/rpc)) with a fresh context. Progress lives in the code, in Watts snapshots, and in files in the task folder. The child loads the Ralph extension and disables normal extension discovery, so your other extensions, including MCP, are not expected to load inside iterations.
- The `commands` in `RALPH.md` run before the agent edits files each iteration, and their output is injected as evidence. Commands marked `acceptance: true` are rerun after the agent emits its completion promise, and any non-`ok` outcome keeps the loop going. That is how the checks cannot be talked around.
- The promise (`DONE` for build, `REVIEWED` for review) is only accepted when the completion gate is satisfied: `required_outputs` exist, `OPEN_QUESTIONS.md` has no unresolved P0 or P1 items, and every acceptance command passes.
- A loop stops on completion, `max_iterations`, `no-progress-exhaustion`, a timeout, or `/ralph-stop` and `/ralph-cancel`. The prompts set 30 iterations of up to 900 seconds for the build loop and 10 for the review loop.

The templates in `templates/` set, and you can tune:

- Guardrails: `block_commands` (no Git commands), `protected_files` (secrets, SPEC.md, PLAN.md, BASE, `review/`, the kit, `.watts/`), and a `shell_policy` allowlist. The allowlist is on for the reviewer and commented out for the builder. Enable it for Tier 1 and 2 builds and fill in your toolchain.
- Allowlist rules, confirmed in the extension's source: each pattern is anchored as `^(?:pattern)$`, so it must match the whole command, arguments included, which means `go test` alone does not allow `go test ./...`. The allowlist also governs the loop's own `commands`, which is why they are plain scripts (`scripts/evidence.sh`, `check-*.sh`, `gates.sh`) with no shell operators. The patterns use `[^;&|<>`$]*` for arguments so `;`, `&`, `|`, redirects, backticks and `$(...)` cannot ride along. Enabling the builder's allowlist without allowing your own toolchain will block the agent, so add your build and test commands.
- `protected_files` globs are matched with minimatch against the file's path, so `**/SPEC.md` covers every task folder. They block the agent's write and edit tools, not reads, and not writes made through an allowed shell command.
- Add your source patterns (for example `**/*.go`) to the reviewer's `protected_files`, and your test commands to its allowlist.
- `OPEN_QUESTIONS.md` convention: one question per line, prefixed `P0:`, `P1:` or `P2:`, deleted when resolved, `None.` when empty. The extension documents the gate but not the exact format it parses, so confirm it with the first run.

`watts task status <task>` reads Temporal state and attempt history. `watts task cancel <task>` cancels the workflow and active subprocess groups. Activity logs live under `<task>/.watts-state/logs/`.

## MCP

Pi 1.0 has [built-in MCP support](https://pi.dev/docs/latest/mcp). In Watts it applies to interactive sessions (`watts agent pi`), not to loop iterations. Servers go in the build role's private agent directory (`watts agent pi` uses it), because `defaultProjectTrust: never` ignores project-level `.pi/mcp.json`:

```
watts agent pi -- mcp add context7 -- npx -y @upstash/context7-mcp
watts agent pi -- mcp list
```

Context7 is only an example (a hosted documentation service, so queries leave your machine, and the package name was written from memory, so check its own instructions). Any `${VAR}` in an MCP config needs that variable in `env_passthrough`. By default Pi exposes MCP tools only through its [codemode](https://pi.dev/docs/latest/codemode) script tool, which small local models may handle poorly, and `"exposure": "direct"` on a server changes that. See the [MCP docs](https://pi.dev/docs/latest/mcp) for transports, OAuth, and the `pi mcp add` options.

## Command reference

`watts help` lists everything. Flags can go before or after the arguments.

- Setup: `init`, `doctor [--live] [--json]`, `install` (`all`, `pi`, `ralph`), `version`
- Config: `config show [--resolved]`, `config set <key> <value>`, `config apply [--force]`, `config env [add|remove NAME...]`
- Agents: `agent pi [--agent name] [-- pi args]`, `agent list`, `agent env [name] [--json]`, `agent new <name>`, `agent follow [name] [--once] [--tail N]`, `agent clean --force`
- Tasks: `task new <slug>`, `task run [task]`, `task status [task] [--json]`, `task decide [task] [stage] [--reject] [--feedback text]`, `task cancel [task]`
- Development: `dev mock-server [--addr] [--reply] [--status]`
- Submission flags: `--env-passthrough NAME`, `--offline`, `--dry-run`; retry also supports `--ignore-attempt-limit`

A task argument is a folder such as `tasks/2026-10-03-http-retry`, given relative to the repository root or to your current directory. Exit codes: 0 success, 1 a failed check or error, 2 a usage error, with task submission returning before activities finish. Inspect activity failures through `task status`.

What the CLI does itself and what stays in bash: setup, isolation, config, installs and diagnostic checks live in internal Go packages; the CLI handles command wiring, prompts and output. The check scripts in `.watts/kit/scripts` stay bash because the loop prompts call them by path from inside the loop, where there is no `watts` binary on the isolated PATH.

## Troubleshooting

- A bare `/ralph ...` arrives as ordinary chat: the extension is not loaded. Run `watts doctor`, then `/reload` or restart Pi, as the [extension's README](https://pi.dev/packages/@lnilluv/pi-ralph-loop) advises. `pi --help` does not list extension commands.
- `No API key found` or a 401 from the gateway: run `watts doctor --live` and `watts agent env`. The usual cause is that the key variable is not in `env_passthrough` (or, for a declared provider, not set in your shell), so Pi never received it. Watts now stops before starting with exactly that message.
- `doctor` says pi-ralph-loop is missing for a role: `watts install ralph`. If npm reports `SELF_SIGNED_CERT_IN_CHAIN`, `watts config env add NODE_EXTRA_CA_CERTS` first, as above.
- A non-interactive `pi -p ...` hangs: Pi's print mode waits for stdin to close when it is not a terminal. `watts` already gives `-p` runs an empty stdin. If you call Pi yourself from a script, add `< /dev/null`.
- No model or provider: check `watts config show`, apply changes with `watts config apply`, then `watts agent pi` and `/model`. For hosted providers check `env_passthrough` or `/login`. If Pi seems to ignore your own `models.json`, remember that the loops read `.watts/pi-agent-<role>/models.json`, not `~/.pi/agent`.
- Weird truncation or looping with Ollama: raise the context length.
- `[blocked by guardrail: ...]` in a transcript: working as intended. If it blocks legitimate work, adjust `RALPH.md`.
- Start over: `watts agent clean --force` deletes the private agent directories (logins, packages, sessions) and keeps the kit and config, then `watts install`.

## References

[Pi documentation](https://pi.dev/docs/latest), [Quickstart](https://pi.dev/docs/latest/quickstart), [Configuration](https://pi.dev/docs/latest/configuration), [Settings](https://pi.dev/docs/latest/settings), [Environment variables](https://pi.dev/docs/latest/environment-variables), [Models](https://pi.dev/docs/latest/models), [Providers](https://pi.dev/docs/latest/providers), [Packages](https://pi.dev/docs/latest/packages), [Skills](https://pi.dev/docs/latest/skills), [Security](https://pi.dev/docs/latest/security), [Containerization](https://pi.dev/docs/latest/containerization), [CLI](https://pi.dev/docs/latest/cli), [RPC](https://pi.dev/docs/latest/rpc), [MCP](https://pi.dev/docs/latest/mcp), [Pi 1.0](https://earendil.com/posts/pi-1-0/), [Pi Durable](https://earendil.com/posts/pi-durable/), [pi-ralph-loop](https://pi.dev/packages/@lnilluv/pi-ralph-loop), [its repository](https://github.com/lnilluv/pi-ralph-loop).

`watts task status` without a task selects the project's only task. If there are multiple tasks, it lists the choices and requires an explicit task argument.

Agent thinking and provider connections are configured separately:

```json
{
  "version": 1,
  "agents": {
    "build": { "thinking": "medium" },
    "review": { "thinking": "high" }
  },
  "providers": {
    "ollama": { "base_url": "http://localhost:11434/v1", "api": "openai-completions" }
  }
}
```

Ollama models can be omitted from the connection: Watts generates their definitions from workflow stages. Other custom providers require a model list. Provider and model selection remain in each workflow step. Pi and Ralph versions are managed by Watts and are not project configuration fields or init flags.
