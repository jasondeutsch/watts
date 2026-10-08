# Watts runbook

This guide covers project setup, configuration, and running a task. See the [workflow reference](workflows.md) for stage definitions and capabilities, and the [Temporal reference](temporal.md) for server operations and execution internals.

## Set up a project

Install Watts from the source checkout with `go install .` and put Go's binary directory on your PATH. Building requires Go 1.27 or later. Agent execution requires Node.js 22.22.1 or later, Pi, and the Ralph extension; `watts install` installs the supported Pi and Ralph versions. The bundled checks require Bash, awk, find, xargs, and sha256sum or shasum. Use WSL on Windows.

Start the local Temporal server from the Watts checkout with Docker running:

```sh
docker compose up -d --wait
```

Then initialize the directory containing your application:

```sh
cd /path/to/project
watts init
watts install
export OPENROUTER_API_KEY='your-key'
watts doctor
watts start -d
```

`init` initializes the current directory, even if a parent is already a Watts project. It creates `watts.json`, project workflow templates, and the local kit and agent directories. It asks where tasks should live; use `watts init --tasks-dir tasks` to specify that without a prompt. Existing kit assets and user-written model files are preserved.

`watts start -d` runs the project worker and Watts web UI in the background and opens the UI in your default browser. Use `watts start` for foreground operation and `watts stop` to stop the managed service. After rebuilding Watts, restart the service to use the new binary. Credentials must be available in the shell that starts the worker.

The Temporal UI is separate, normally at [localhost:8233](http://localhost:8233). Stopping the Watts worker preserves workflow state. `docker compose down` preserves the Temporal database; `docker compose down -v` deletes its history.

## Configure the project

`watts.json` contains project infrastructure and agent execution settings. Each task's `workflow.json` selects its stages, agents, identities, providers, and models.

```sh
watts config show
watts config show --resolved
watts config apply
```

Edit nested settings directly in `watts.json`, then apply them. `config apply` merges Pi settings and refreshes installed skills. It preserves an existing `models.json`; use `watts config apply --force` when you intend to regenerate that file. Unknown configuration fields are rejected.

| Setting | Purpose |
| --- | --- |
| `tasks_dir` | Project-relative task directory; default `tasks` |
| `default_workflow` | Project-relative workflow template copied into new tasks |
| `providers` | Provider endpoints, credential sources, and model definitions |
| `agents` | Agent directories, thinking levels, skills, Pi settings, and protected paths |
| `skills_dirs`, `pi_settings`, `forbidden_paths` | Shared settings extended or overridden by each agent |
| `env_passthrough` | Additional environment variable names available to agents |
| `offline` | Sets Pi's offline mode |
| `limits.max_minutes`, `limits.max_attempts` | Default activity timeout and stage attempt budget |
| `snapshot_exclude` | Additional project-relative paths excluded from snapshots |
| `temporal` | Connection overrides: address, namespace, and task queue |
| `capabilities` | Executable plugins used by workflow stages |

An inline `workflow` can replace `default_workflow`; do not set both. Prefer a separate template for readability. See [workflows.md](workflows.md) for customization.

### Providers and models

The bundled workflows use OpenRouter's `deepseek/deepseek-v4-flash`. Change `provider` and `model` on the relevant workflow stages before submitting a task. Build and review may use the same model.

A custom provider declaration looks like this:

```json
{
  "providers": {
    "example": {
      "base_url": "https://inference.example.com/v1",
      "api": "openai-completions",
      "api_key_env": "EXAMPLE_API_KEY",
      "models": [
        { "id": "example/coding-model", "context_window": 128000, "max_tokens": 16384 }
      ]
    }
  }
}
```

`api_key_env` names a variable; it does not contain the secret. Watts automatically passes that provider's variable to its agent and generates Pi's model credential reference as `$EXAMPLE_API_KEY`. Export the value before starting the worker. Alternatively, `api_key_command` accepts an argument array for a command that prints a credential at launch. Set one credential source, not both. Keep secrets out of configuration, prompts, and commands recorded in workflow history.

Custom providers require a model list. Ollama may omit that list: Watts derives model definitions from workflow stages. For Ollama, configure `providers.ollama.base_url`, pull the selected model, and keep Ollama running. Ensure the server's context window is large enough for the task.

`watts doctor` checks local dependencies and credentials. `watts doctor --live` also makes a small provider request to distinguish configuration problems from authentication or provider failures.

### Agent settings and environment

Built-in roles are `build` and `review`. Configure thinking under `agents.<name>.thinking`; the default is `medium`. Custom roles can be registered with `watts agent new <name>` and selected by workflow stages.

Each role uses its own Pi directory, normally `.watts/pi-agent-<name>/`, and private HOME, `.watts/home-<name>/`. Your normal `~/.pi/agent` settings and credentials are separate. Generated Pi settings disable project trust, so project-level `.pi` configuration is ignored. Add skills with `skills_dirs` and Pi settings with `pi_settings`, globally or per agent.

Agents receive a constructed environment rather than your whole shell environment. To expose additional variables:

```sh
watts config env add GOCACHE GOMODCACHE HTTPS_PROXY
watts agent env build
```

The environment diagnostic prints names, not values. Submission flags pass variable names; they do not transfer credential values from the submitting terminal to an already-running worker. Restart the worker after changing its shell environment.

Private directories and protected-path checks are guardrails, not a filesystem sandbox. See [sandbox.md](sandbox.md) for the boundary and isolation options.

## Create and run a task

```sh
watts task new http-retry
```

Use the task path printed by that command in the examples below. Write the request as rough notes in `<task>/SPEC.md`; the specification agent refines it. SPEC describes what to build and why. PLAN describes how to implement and verify it. Customize `<task>/workflow.json` before submitting if needed.

```sh
watts task run <task> --dry-run
watts task run <task>
watts task status <task>
```

`run` submits and returns; completion happens asynchronously. The default workflow drafts the specification, waits for human approval, drafts the plan, waits for human approval, builds, and reviews. Implementation belongs in the project containing `watts.json`; the task directory holds documents and execution evidence.

At each human gate, inspect the document before deciding:

```sh
watts task decide <task> spec-review
watts task decide <task> plan-review
```

To request a revision at the pending gate:

```sh
watts task decide <task> spec-review --reject --feedback "Clarify the timeout requirement"
```

Approval records the input file hashes. Changed inputs invalidate the pending approval; rejection routes through the workflow's rework transition. Agent-authored signatures do not grant human approval, and Watts agent processes cannot submit human decisions.

First submission pins the workflow definition and runtime configuration. Editing the template or task workflow afterward does not alter that execution. Create a new task to use a changed definition or configuration.

Task commands accept a folder or task name. You may omit it when the project has exactly one task; otherwise supply it explicitly.

## Inspect failures and retry

Use the Watts web UI to inspect the stage, attempts, error, and output. For an agent or terminal caller:

```sh
watts task status <task> --json
watts agent follow build
```

Task status reports workflow state and attempt history. Agent follow displays a role's session feed. Activity output is stored in `<task>/.watts-state/logs/<stage>-<attempt>.log`; executor requests and results are under `.watts-state/steps/`. Detached service output is in `.watts/service.log`.

| State | Action |
| --- | --- |
| `waiting_approval` | Review inputs and use `task decide` |
| `waiting_retry` | Inspect the failed attempt, fix project inputs or the external cause, then use `task run` |
| `waiting_event` | Deliver the expected event through Temporal; the MVP CLI has no event command |
| Active | Ensure the project worker is running; repeated submission does not duplicate execution |
| `completed` | Inspect the implementation and final evidence |
| Terminal failed, cancelled, or terminated | Inspect evidence and create a new task; the execution cannot be reopened |

Retry the current failed stage:

```sh
watts task run <task>
```

If its attempt budget is exhausted, explicitly authorize another attempt after inspecting the failure:

```sh
watts task run <task> --ignore-attempt-limit
```

Retries retain prior attempts and pinned configuration. Activities do not automatically retry through Temporal; workflow transitions and explicit retries control rework. Stage `timeout_seconds` and `max_attempts` override project limits. If neither supplies a value, activities default to one hour and three attempts. Human approval waits do not expire.

Cancel with `watts task cancel <task>`. Cancellation stops active subprocess groups through the worker. File edits are not rolled back. If worker loss leaves a process recorded in the workspace journal, inspect and stop that process before retrying. See [Temporal recovery](temporal.md#decisions-and-recovery).

## Files and evidence

| Location | Contents |
| --- | --- |
| `watts.json` | Project settings |
| `workflows/` | Project workflow templates |
| `<tasks_dir>/<task>/` | SPEC, PLAN, workflow definition, and other declared artifacts |
| `<task>/.watts-state/` | Snapshots, approvals, pinned settings, logs, and attempt evidence |
| `.watts/kit/` | Installed checks, templates, and skills |
| `.watts/pi-agent-*/`, `.watts/home-*/` | Private agent settings, credentials, packages, and sessions |
| `.watts/service.*` | Managed worker/UI service records and output |

Snapshots record project file hashes and copies of files up to 1 MB. The bundled build loop logs completed stories against snapshots; review checks compare against the review-start snapshot. Snapshots exclude Watts state, task folders, `.git`, dependencies such as `node_modules` and `.venv`, and `snapshot_exclude` entries. They provide evidence, not rollback or authenticated authorship. Watts requires no Git repository or clean branch.

Retain task evidence, project files, and the Temporal database. Treat private agent directories as state too: deleting them removes logins and sessions. Reinitialization can restore bundled assets, but cannot restore that evidence or authentication.

## Troubleshooting

- **Missing credentials or HTTP 401:** run `watts doctor --live` and `watts agent env <agent>`. Check the provider's variable in the worker's launch environment and the private `models.json` credential reference. Restart the worker after exporting a missing variable.
- **Provider error:** inspect the complete attempt log. Rate limits, model availability, and upstream failures require a provider-side fix or a new task with different pinned settings.
- **Timeout:** check activity output and provider responsiveness. Increase the stage timeout or project limit before submitting a new task. Local models may need more time, memory, or a smaller workload.
- **Missing Ralph extension:** run `watts install ralph`, then restart the service. `watts doctor` checks installation per role.
- **Guardrail blocks legitimate work:** inspect the task's RALPH.md and workflow checks. A successful process exit alone does not satisfy required outputs or completion checks.
- **No progress after worker restart:** inspect task status and the workspace process journal. The worker must use the task's project queue and have access to the correct files.

For developer tests, run `go test ./...`. For repeatable workflow quality evaluation, see [the Go workflow eval](evals/go-workflow/README.md). The [coordinator skill](resources/skills/watts-coordinator/SKILL.md) explains how an external agent can guide the task lifecycle.
