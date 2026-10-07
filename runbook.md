# Running a task

All task execution uses Temporal. With Docker running, start the local server from the Watts source directory:

```sh
docker compose up -d --wait
```

Then start a Watts worker from your project in a separate terminal:

```sh
watts start
```

The server keeps its database in a Docker volume. `docker compose down` preserves it; `docker compose down -v` deletes workflow history.

See [temporal.md](temporal.md) for connection settings and recovery, and [workflows.md](workflows.md) for workflow templates and capabilities.

For a bundled SDLC task:

```sh
watts task new http-retry
# Describe the task in <task>/SPEC.md; edit <task>/workflow.json as needed.
watts task run <task> --dry-run
watts task run <task>
watts task status <task>
# Wait for spec-review, inspect SPEC.md, then:
watts task decide <task> spec-review
# Wait for plan-review, inspect PLAN.md, then:
watts task decide <task> plan-review
```

The worker drafts SPEC.md, waits for the explicit specification review, drafts PLAN.md, and waits for the explicit plan review. It then builds and independently reviews the implementation. To request a revision, use `watts task decide <task> spec-review --reject --feedback "Clarify the timeout requirement"`, or the corresponding plan-review command. Feedback reaches the next drafting attempt. Agent-authored signatures do not grant approval.

`task run` submits and returns; the worker executes independently. Repeating it retries a failed stage or reports the existing execution if it is active or complete. Status and completion come from Temporal, while local evidence remains beside task artifacts. `watts agent follow build` or `review` displays an agent's session from another terminal. The Temporal development UI normally runs at localhost:8233.

To use a custom default, write a project workflow file and set `default_workflow` to its path in watts.json. Create the task with `watts task new <slug>`, then edit its workflow.json directly before running it. The template controls which artifacts are scaffolded.

When a stage waits for action:

- `waiting_approval`: inspect the declared inputs, then use `watts task decide <task> <stage>` or add `--reject --feedback "changes needed"`.
- `waiting_event`: an external integration must deliver the event through Temporal; event delivery is not exposed in the MVP CLI.
- `waiting_retry`: inspect the error and evidence, then use `watts task run [task]`. Add `--ignore-attempt-limit` only to authorize an additional attempt beyond the budget.
- An active execution needs its worker. Restarting the worker resumes durable orchestration; it does not erase activity effects.

Use `watts task cancel <task>` to cancel the execution and stop active activity subprocess groups. Inspect any surviving process reported by the workspace journal before retrying. Task status reports `completed` when all workflow stages pass.

Run `go test ./...` for unit, CLI, SDK workflow/activity and embedded script tests. `WATTS_TEMPORAL_INTEGRATION=1 go test ./internal/application -run TestTemporalLive -v` exercises a real development service. Model quality and an agent following prompts require a separate real-model exercise.

`run`, `status`, `decide`, and `cancel` accept an omitted task when exactly one exists. With multiple tasks, supply a task folder or name. `watts task decide [task]` approves the pending review; use `--reject --feedback "changes needed"` to request revisions.
