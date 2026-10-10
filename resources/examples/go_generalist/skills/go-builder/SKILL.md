---
name: go-builder
description: Implement and test Go business application and web service changes in the Watts Go generalist workflow, following the approved spec, plan, and team lint policy.
license: MIT
---

# Go builder

Use the task's approved SPEC.md and PLAN.md to determine scope. Follow existing project conventions unless the task explicitly changes them. Application source, tests, and modules belong in the project root containing watts.yaml; task directories hold workflow documents and evidence.

## Readable implementation

- Prefer concrete types and small functions with clear responsibilities. Introduce interfaces at consumer boundaries when substitution or testing requires them; avoid speculative abstractions and framework changes.
- Use names that explain the domain and purpose. Handle errors and exceptional cases early so the normal path stays easy to read.
- Use keyed struct literals. Initialize maps before writing; choose nil versus empty slices according to the API contract rather than applying one rule everywhere.
- Keep changes within the approved stories. Do not introduce a library, directory structure, or architecture merely because a skill mentions it.

## Errors and service boundaries

- Handle returned errors or propagate them with useful operation context. Wrap with `%w` when callers need the cause; inspect chains with `errors.Is` and `errors.As` rather than comparing message text.
- Avoid duplicate logging at every layer: return contextual errors to the boundary responsible for reporting them. Expected business failures are returned errors, not panics.
- Preserve cancellation and deadlines across database calls, HTTP clients, and goroutines. Give background work an explicit owner and shutdown path.
- Validate incoming data and map business failures to the API's declared response contract. Keep internal errors and secrets out of client responses and logs.
- For changed persistence behavior, check transaction boundaries, rollback paths, and query parameterization. For changed concurrent behavior, check races, blocking, and goroutine cleanup.

## Go code intelligence

When gopls MCP is available, use workspace discovery and package APIs to understand the project, symbol references to assess affected callers, and diagnostics after edits. For a rename, inspect and apply the returned edits, then run checks. These tools supplement the approved plan and do not replace lint or tests. If the configured tools are unavailable, report that explicitly; do not claim they were used.

## Tests that constrain behavior

- Test acceptance criteria and meaningful failure paths, including boundary inputs and cancellation where relevant. Assert observable behavior rather than reproducing implementation logic.
- Use named table-driven subtests when cases share a useful structure. Keep tests independently runnable; parallelize only when their resources are isolated.
- Use the standard `testing` harness. For this team example, use Testify `require` for prerequisites and `assert` for independent expectations. Prefer small fakes at external boundaries over extensive mock expectations.
- Use temporary directories and explicit fixtures. Tests should not depend on personal credentials, live production services, wall-clock sleeps, or execution order. Document how optional integration tests are enabled.
- Run the plan's checks. Use race detection for concurrent changes when the toolchain supports it. Report checks that were not run and why.

## Lint and completion

Run `bash resources/examples/go_generalist/scripts/lint.sh` from the project root along with the plan's build and test commands. The example's .golangci.yml is the team policy; lint findings must be resolved before declaring a story complete.

Do not weaken lint configuration or add broad suppressions to make a change pass. Explain any narrowly scoped suppression. Formatting and lint success supplement tests and independent review; neither proves the acceptance criteria.

Follow the task's Ralph instructions for snapshots and story evidence. Report actual command results, not assumed success. Bring contradictions in approved documents back to the human gate instead of inventing requirements or approving documents yourself.

## Source and team customization

This is a focused adaptation of selected guidance from samber/cc-skills-golang. See [SOURCES.md](SOURCES.md) for the revision and [LICENSE](LICENSE) for the upstream MIT notice. Team additions cover Watts artifact placement, its lint script, and evidence requirements. Edit this skill as team conventions evolve; it does not require upstream's library choices or other skills.
