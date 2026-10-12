# Tic-tac-toe — minimal terminal game for two human players

## Status
Draft
Version: v1
Owner: <to be assigned>
Last updated: <date>

## Intent
Two local human players need a terminal-based tic-tac-toe game they can run without installing anything beyond Go. The game prints the board each turn, accepts moves by cell number, enforces the rules, and announces the outcome — a win, a draw, or an abandoned session (EOF) — then stops. No AI, web server, persistence, or configuration framework is needed.

## Background and context
The workspace is empty; no Go project exists yet. This is a greenfield implementation of one of the simplest possible interactive terminal games. The human request supplies all the rules, constraints, and scope boundaries needed. The implementation is expected to be a single `main.go` plus focused test files, with a `go.mod` at the project root. A lint gate (configured elsewhere) will run alongside `go test ./...`.

## Acceptance criteria

- AC-1: When the program starts, the system shall print an empty 3×3 board labelled with cell numbers 1–9 and prompt Player X for a move.
- AC-2: When a player enters a valid unoccupied cell number (1–9), the system shall place that player's mark, update the board display, and switch to the other player's turn.
- AC-3: If a player enters a cell number that is already occupied, then the system shall print an error message, display the current board again, and keep the same player's turn without changing the board state.
- AC-4: If a player enters a number outside the range 1–9, then the system shall print an error message and prompt the same player again without changing the board state.
- AC-5: If a player enters nonnumeric input (text, special characters, etc.), then the system shall print an error message and prompt the same player again without changing the board state.
- AC-6: After any move, if the board contains three matching marks in any row, column, or diagonal, the system shall announce the winning player and exit.
- AC-7: After any move, if the board is full with no win, the system shall announce a draw and exit.
- AC-8: When the program reads EOF (e.g., Ctrl+D / Ctrl+Z) at any input prompt, the system shall print an exit message and terminate cleanly with a zero exit code.
- AC-9: When the program runs `go test ./...`, all tests shall pass.
- AC-10: When the configured lint gate runs against the project, it shall report zero errors.

## Explicit non-goals

- No AI opponent or computer player.
- No network, web server, or API.
- No persistence (file storage, database, leaderboard, save/load).
- No configuration files, environment variables, or CLI flags beyond what Go's standard library supplies.
- No external application dependencies beyond the Go standard library.
- No concurrency, goroutines, or channels.
- No GUI, ncurses, or interactive TUI framework — terminal line I/O only.

## Assumptions

- The players are co-located at the same terminal and take turns at the same keyboard.
- The `go.mod` file will declare the module path `tic-tac-toe` (or a name specified by the developer).
- A configured linter (e.g., `golangci-lint`) is expected to pass; no specific linter config is supplied in this spec.
- Go is already installed in the build environment.

## Constraints

- Use only the Go standard library; no third-party packages.
- The implementation must be readable and small — ideally one `main.go` plus separate test files.
- Application files (`go.mod`, `*.go`) must be created in the project root (`/workspace`), not in the task directory (`/runtime/task`).
- Zero lint errors per the project's configured lint gate.

## Open questions

None. The human request, the acceptance criteria above, and the explicit non-goals cover all known decisions. Cell display format (e.g., ASCII grid with `|` and `-`) and exact error-message wording are left to the implementation, as they do not affect correctness.

## Risk-relevant context

- Touches: A new, standalone Go terminal application with no network access, no file I/O beyond terminal stdin/stdout, and no external dependencies.
- Does not touch: Any existing production code, credentials, entitlements, identity, network services, persistent storage, or configuration infrastructure.

## Risk tier
0

## Review

An agent drafts this document. The human approves or rejects it at the spec-review workflow gate with `wctl task decide <task> spec-review`, optionally using `--reject --feedback 'changes needed'`.