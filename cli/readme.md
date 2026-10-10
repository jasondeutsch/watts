# CLI organization

`cmd/cli/main.go` is the executable entry point and calls `cli.Run`. `Run` is the public entry point of this package. The root package owns the command tree, help, dispatch and process exit status. Command implementations live under `cli/internal` so they are private to the CLI:

| Package | Responsibility |
| --- | --- |
| `setup` | Initialization, worker startup, installation and diagnostics commands |
| `settings` | Configuration display, edits, application and environment settings |
| `agents` | Interactive Pi, agent discovery, creation, cleanup and session feeds |
| `tasks` | Task lifecycle, execution, status and human decisions |
| `templates` | Standalone YAML package rendering and terminal/file output |
| `devtools` | Mock model gateway |
| `arguments` | Flag parsing, argument validation and common execution flags |
| `terminal` | Streams, prompts, diagnostic output, JSON and execution previews |

Command packages depend on the shared argument and terminal packages and on application services. They do not import one another or the dispatcher. The shared packages do not import command packages. `internal/application` and the other core packages cannot import any CLI package.

Keep commands focused on translating arguments into application calls and rendering the result. Configuration rules, persistence, orchestration and process execution belong in the core `internal` packages. Dependency tests enforce these boundaries recursively. The root integration tests exercise the complete CLI through `Run`; command and presentation tests can live beside their implementation.

Application behavior and live Temporal tests live in `internal/application`; deterministic workflow tests live in `internal/orchestration`. Tests use Testify `assert` for independent expectations and `require` for prerequisites. CLI tests retain end-to-end command coverage without reexporting core types through test aliases.
