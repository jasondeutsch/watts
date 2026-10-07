# Watts

Watts is a proof of concept. Its release version remains **0.1.0** as the implementation evolves.

## License

This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, you can obtain one at https://mozilla.org/MPL/2.0/.

Watts is licensed under [MPL 2.0](LICENSE). Third-party dependencies and attributed material retain their respective licenses.

A governed way to get coding work done by AI agents on [Pi 1.0](https://earendil.com/posts/pi-1-0/). A human decides what should be built and approves it. An agent builds it in a loop, a separate agent reviews it, deterministic scripts check the facts, and a human reviews the final changes.

The stack is small: [Pi](https://pi.dev) 1.0 as the agent, the [`@lnilluv/pi-ralph-loop`](https://pi.dev/packages/@lnilluv/pi-ralph-loop) extension as the loop, a small Go CLI, `watts`, for the commands, and [Ollama](https://pi.dev/docs/latest/models#configure-a-compatible-endpoint) or any hosted provider for the model. Ollama is optional.

Docs: [temporal.md](temporal.md) (durable workflows, workers and human decisions), [configuration.md](configuration.md) (install, config, isolation), [runbook.md](runbook.md) (the procedure per task), [sandbox.md](sandbox.md) (tentative stronger isolation). Source: `main.go` calls the thin `cli/` adapter for argument parsing, terminal interaction and output. Its private command packages are organized by domain; see [cli/readme.md](cli/readme.md). `internal/application` owns task and agent operations; `internal/config`, `internal/kit`, `internal/storage`, `internal/workspace` and `internal/orchestration` own configuration, embedded assets, persistence, local execution and durable workflows. The kit is compiled from `internal/kit/kit/`.

Diagram: one [C4-PlantUML](https://github.com/plantuml-stdlib/C4-PlantUML) diagram of the structure and the numbered flow of one task, source [diagrams/watts.puml](diagrams/watts.puml), with a compact workflow view in [diagrams/watts.svg](diagrams/watts.svg). Render the C4 source with `plantuml -tsvg diagrams/watts.puml` (needs [PlantUML](https://plantuml.com) and Graphviz).

## Quick start

To have an interactive agent guide or run a task, give it the [Watts coordinator skill](resources/skills/watts-coordinator/SKILL.md). Teams can provide their own external context and integrations; the skill documents the CLI, task files, monitoring, recovery, and human review boundaries. See [resources/readme.md](resources/readme.md) for using it with your agent.

The task dashboard lives in [`web/`](web/readme.md), beside `cli/`. Run `watts start` from your project to start its worker and open the dashboard in your default browser. Use `watts start -d` for background operation and `watts stop` to stop both.

`watts start` stays in the foreground; Ctrl+C stops the worker and UI. `watts start -d` detaches, and `watts stop` stops the service for the current project. The UI URL is printed at startup. Temporal must already be running. Rebuild Watts after changing the source.

Once per machine, install the CLI (Go 1.27 or later builds it):

```
go install github.com/jasondeutsch/watts@latest
```

Once per project, from the repository root:

```
watts init                                       # writes watts.json, .watts/ (kit, private Pi dirs) and a few .gitignore lines
watts doctor                                     # add --live to also make one real call to each gateway
```

Start the local Temporal service from the Watts source directory (requires Docker and Docker Compose v2):

```sh
docker compose up -d --wait
```

The API runs at `localhost:7233`, and the [Web UI](http://localhost:8233) at port 8233. Workflow history persists in a Docker volume. Run `watts start` from your project in a separate terminal. See [temporal.md](temporal.md) for stopping, restarting and external-server configuration. Temporal is required for every task execution. Then, per task:

```
watts task new http-retry
watts task run <task> --dry-run                  # preview the task-local workflow
watts task run <task>                            # starts specification drafting
watts task status <task>
watts task decide <task> spec-review             # approve the generated specification
watts task decide <task> plan-review             # approve the generated plan
watts task status <task>                         # completed when all stages pass
```

You do not copy a Watts directory into your project. `watts init` adds `watts.json`, `.watts/` and nothing else except those `.gitignore` lines. `watts help` lists every command, grouped, and each group (`watts task`, `watts agent`, `watts config`) lists its own subcommands.

`watts task status <task>` reads durable execution state. `watts task run [task]` starts an unsubmitted task or retries its failed stage. Repeating it while a task is active reports the current execution. Every execution uses Temporal; connection settings default to localhost:7233.

Base workflows live in [workflow-templates/](workflow-templates/). Set `default_workflow` in watts.json to a project workflow file, or use the bundled SDLC default. Each task receives its own workflow.json for direct editing. Stages run in listed order, with any rework routes explicitly declared. See [workflows.md](workflows.md).

Tests use Testify assertions. CLI integration tests cover dispatch and output; application and orchestration tests live beside their implementations and cover execution, pinned definitions and durable workflow behavior. The embedded check scripts have their own shell suite. The live Temporal suite covers restart recovery, decisions, retries and cancellation. Real-model prompt following and model quality require separate evaluation.

## What `watts init` puts in your repository

```
watts.json         the configuration: provider connections, agent directories, default workflow, limits (keep this with project workflow files)
.watts/             generated, disposable, rebuilt from watts.json by `watts init`
  kit/              check scripts, loop prompt templates, the five skills (keep it only to pin the scripts for your team)
  pi-agent-*/ home-*/   private Pi state for the agents (local)
workflow.json       optional project default, selected by default_workflow
tasks/       created by `watts task new`, one folder per task, kept as the audit trail (the name is tasks_dir)
```

Watts does not use git or any other version control, whether or not your project is tracked: it keeps its own snapshots (see Snapshots in [configuration.md](configuration.md)), and it never checks the state of your working tree, switches branches or edits your `.gitignore`. `watts.json` and project workflow templates configure execution. Nothing else has to be committed: you may ignore `.watts`, `watts.json` and the tasks folder in git and lose nothing, and deleting `.watts` and running `watts init` again rebuilds it. Committing `watts.json` gives your whole team and CI the same setup,.

## The default SDLC workflow

1. Specification: an agent refines the task’s SPEC.md. The spec-review human gate approves it or returns feedback to specification.
2. Planning: an agent reads the approved specification and writes PLAN.md. The plan-review human gate approves both artifacts or returns feedback to planning.
3. Build: an agent implements the plan through `/ralph`, with deterministic checks before and after execution.
4. Review: the review agent checks the resulting work and writes a verdict. A rejection returns to build.

Every executable SDLC step explicitly declares its agent, prompt, inputs, outputs and checks in the task's workflow.json. Human review gates are explicit workflow steps. The bundled definition pauses for both document reviews at every risk tier; customize the task workflow before submission to change that policy. `watts agent pi` remains an interactive agent session for exploration and authentication; starting a task runs drafting through its worker.

When status reports `completed`, review the final changes and verdict. Watts does not merge changes.

## The task folder

Each task is a folder under `tasks/`, kept permanently as the audit trail:

```
tasks/2026-10-03-http-retry/
  workflow.json                    task-local definition
  SPEC.md  PLAN.md  BASE            what and why, how and the stories, the baseline snapshot
  RALPH.md                          build loop prompt and settings
  STORY_LOG.md                      DONE US-001 <snapshot id>, written by the builder, verified by script
  OPEN_QUESTIONS.md  RALPH_PROGRESS.md
  review/
    RALPH.md  VERDICT.md            review loop prompt and the verdict
    history/                        earlier verdicts
  retrospective.md
```

Loop run state (`.ralph-runner/`, `.watts-state/`) ignores itself through a `.gitignore` inside the task folder. A project holds many tasks, each with its own folder. Follow-up work is a new task with a new SPEC.md whose Supersedes line points at the old one.

## How governance works

Judgment is left to humans and to the reviewer model. Facts are checked by scripts, because scripts do not get tired or flattered:

- Human gates approve the exact input hashes through Temporal updates. The worker records local artifact approval receipts for deterministic checks; changing a document invalidates its matching approval. Use `watts task decide <task> <stage> --reject --feedback "changes needed"` to return it to its drafting agent.
- Plan structure is checked by `check-plan.sh`: quality gates listed, stories numbered, every acceptance criterion covered by a story.
- Story claims are verified by `check-stories.sh`: a story counts as done only if STORY_LOG.md names a snapshot that really exists and was taken by the agent.
- A verdict is checked by `check-verdict.sh`: well-formed, newer than the agent's latest snapshot, no project files changed since the review started, and a PASS is refused if approvals, plan, stories, or gates are red.
- The build loop's completion is gated by the extension itself: its [completion gate](https://pi.dev/packages/@lnilluv/pi-ralph-loop) reruns the `acceptance: true` commands after the agent claims it is done, and any failure keeps the loop going.
- Agents run under the identities `watts-agent` and `watts-reviewer`, and snapshots are labelled agent, human or review-start, so the record shows who took what.

Tiers, from the `risk-tiering` skill (its text is authoritative):

- Tier 0, fully autonomous: dependency bumps, docs, non-functional refactors and formatting, UI-only changes.
- Tier 1, checkpoint before merge: governance field changes, cache or fallback tuning, non-security pipeline stages, adapters that do not touch authentication.
- Tier 2, human approval before execution: anything touching JWT claims, access-control claims, budget attribution, admission policy, entitlements and hard limits, identity or provisioning, credentials. Tier 2 work should use an interactive Pi session with a human at every step, not the unattended loop.

The default workflow requires human specification and planning reviews before Build. Risk tiers guide review depth; approval policy is explicit in the workflow.

## The five skills

Skills are instruction packs that Pi loads on demand ([Skills](https://pi.dev/docs/latest/skills)). They ship in `internal/kit/kit/skills/`, `watts init` copies them to `.watts/kit/skills/`, and `watts config apply` installs them into the private agent directories.

- `spec-template`: SPEC.md with numbered EARS acceptance criteria, non-goals, risk tier, approval. ([EARS](https://en.wikipedia.org/wiki/Easy_Approach_to_Requirements_Syntax))
- `plan-template`: PLAN.md with approach, tier recheck, quality gates, and the user stories the loop works through.
- `risk-tiering`: classifies a task into Tier 0, 1, or 2.
- `admission-rubric`: the reviewer's four steps: spec fidelity, approvals, tests, the three failure modes (no output, fails CI, passes CI but wrong).
- `retrospective-log`: the end-of-task record.

## What this does not do

- It is not a sandbox. `watts` runs Pi with a cleared environment, a private HOME and a private Pi config directory, which keeps ambient keys and your normal logins out. Absolute paths to your real files are still reachable, and Pi's tools [run with the permissions of the Pi process](https://pi.dev/docs/latest/security). For a real boundary use [Pi's isolation guide](https://pi.dev/docs/latest/containerization) or [sandbox.md](sandbox.md).
- The loop's iterations run in a child Pi that, per the extension's documentation, disables normal extension discovery. Extensions you have installed, including MCP, are not expected to load inside iterations.
- Risk tiers are enforced by checks and discipline. Nothing stops a determined agent from editing a file through a shell command, which is why Tier 1 loops should enable the shell allowlist in `RALPH.md` and why a human reads every diff.
- The reviewer is a different process and, if configured, a different model. It is not infallible.

## Pi Durable, and where it fits

[Pi Durable](https://earendil.com/posts/pi-durable/) shipped alongside Pi 1.0 as an experimental package for building long-running agentic applications. It does not replace the Pi coding agent and Watts does not use it. It is a framework: a harness that stores conversations and tasks in a pluggable backend (memory, SQLite or JSONL), checkpoints every step, and after a crash resumes unfinished work from the last checkpoint. The API may still change.

What it would change for Watts, if you ever need it:

- Durability. pi-ralph-loop keeps run state on disk (`.ralph-runner/`, `RALPH_PROGRESS.md`) and `/ralph-resume` starts a new run from the same `RALPH.md`. Pi Durable checkpoints each model request and tool call, reruns tools marked `replay: "safe"` after a crash, and tells the model when an unsafe tool call was interrupted.
- Approval gates. A `beforeTool` hook can ask a human and store the answer in a memo, so a restart finds the stored answer instead of asking again. That is a real gate, where Watts has a pre-flight check.
- The reviewer. A subagent is a separate conversation with its own model, read-only tools and its own working directory, owned by the call that started it.
- Surfaces. Many people and clients can watch and steer the same conversation, which suits a Slack or GitHub front end.

Not worth it yet: it is experimental, it means writing an application on its [harness API](https://github.com/earendil-works/pi/blob/main/packages/durable/README.md) instead of using an extension, and the loop you have is enough for an engineer driving one task at a time. What would carry over unchanged: the skills, the templates, the tier rules, and the logic in the check scripts. Revisit it if you need unattended multi-day runs, remote execution environments, or multi-user steering.

## Further reading

Pi
- [Pi 1.0 announcement](https://earendil.com/posts/pi-1-0/) and [Pi Durable announcement](https://earendil.com/posts/pi-durable/)
- [Documentation index](https://pi.dev/docs/latest), [Quickstart](https://pi.dev/docs/latest/quickstart), [How Pi works](https://pi.dev/docs/latest/how-pi-works)
- [Run Pi safely](https://pi.dev/docs/latest/security), [Isolate Pi](https://pi.dev/docs/latest/containerization), [Configure Pi](https://pi.dev/docs/latest/configuration)
- [Settings](https://pi.dev/docs/latest/settings), [Environment variables](https://pi.dev/docs/latest/environment-variables), [CLI](https://pi.dev/docs/latest/cli), [Choose a model](https://pi.dev/docs/latest/models)
- [Skills](https://pi.dev/docs/latest/skills), [Packages](https://pi.dev/docs/latest/packages), [Connect MCP servers](https://pi.dev/docs/latest/mcp), [Codemode](https://pi.dev/docs/latest/codemode)
- [RPC mode](https://pi.dev/docs/latest/rpc), the interface pi-ralph-loop uses for its child iterations
- [Source code](https://github.com/earendil-works/pi) and [npm package](https://www.npmjs.com/package/@earendil-works/pi-coding-agent)

pi-ralph-loop
- [Package page](https://pi.dev/packages/@lnilluv/pi-ralph-loop) (full README), [repository](https://github.com/lnilluv/pi-ralph-loop), [npm](https://www.npmjs.com/package/@lnilluv/pi-ralph-loop)
- [Prompt patterns](https://cdn.jsdelivr.net/npm/@lnilluv/pi-ralph-loop@2.1.0/skills/ralph-loop/references/prompt-patterns.md) and [config cookbook](https://cdn.jsdelivr.net/npm/@lnilluv/pi-ralph-loop@2.1.0/skills/ralph-loop/references/config-cookbook.md)

Pi Durable
- [README](https://github.com/earendil-works/pi/blob/main/packages/durable/README.md) and the [examples](https://github.com/earendil-works/pi/tree/main/packages/durable/test/examples)

Background
- [Agent Skills specification](https://agentskills.io/specification), the format Pi skills follow
- [EARS, the Easy Approach to Requirements Syntax](https://en.wikipedia.org/wiki/Easy_Approach_to_Requirements_Syntax)
- [Docker Sandboxes](https://docs.docker.com/ai/sandboxes/)

## Task commands

The MVP has five task commands: `new`, `run`, `status`, `decide`, and `cancel`. For all commands operating on an existing task, omit the task argument when the project has exactly one task. With multiple tasks, supply its folder or name; Watts lists the choices instead of guessing. `decide [task] [stage]` uses the pending human review stage when the stage is omitted. Creating a task always requires a slug.

Workflow definitions and model selection are frozen at first submission. `run` retries the existing definition; it does not pick up edits made after submission.

## TODO

- Support a single Watts service that manages workers across multiple registered projects, routing each project's Temporal queue to its own project root, configuration, and agent environment. Users should not need to start a separate worker process for every project.
