# SPEC: Minimal Go tic-tac-toe

## Status
Draft
Version: v1
Supersedes: none
Owner: *(unassigned)*
Last updated: 2026-10-10

## Intent

Build a minimal terminal tic-tac-toe game in Go that two human players can play on the same machine. The game implements the classic rules (alternating turns, win-or-draw detection, invalid-move rejection) and runs entirely from the command line using only the Go standard library. The result should be a small, readable codebase that can serve as a reference or teaching example.

## Background and context

No existing tic-tac-toe code exists in this project. The project is a fresh Go workspace created from the "go-generalist" Watts template (Go 1.26.1, golangci-lint configured). The team needs a self-contained, two-player terminal game with no networking, no persistence, and no external dependencies beyond what Go ships with.

The standard library `testing` package (available since Go 0.19, present in Go 1.26.1) provides the test harness. The board cell numbering follows the standard physical keypad / telephone layout: 1–9, left-to-right, top-to-bottom.

*Observed fact: Go 1.26.1 is installed on the worker PATH. The lint configuration (`.golangci.yml`) enables errcheck, govet, ineffassign, staticcheck, unused linters and the gofmt formatter.*

## Acceptance criteria

- AC-1: When the program starts, the system shall display an empty 3×3 board and prompt Player X to enter a cell number (1–9).
- AC-2: While a game is in progress, after a valid move the system shall redraw the board with the player's mark in the chosen cell, then display the next player's prompt.
- AC-3: If a player enters a number outside 1–9, then the system shall reject the move without changing the turn, display an error message, and re-prompt the same player.
- AC-4: If a player enters a number for an already-occupied cell, then the system shall reject the move without changing the turn, display an error message, and re-prompt the same player.
- AC-5: If a player enters non-numeric input, then the system shall reject the move without changing the turn, display an error message, and re-prompt the same player.
- AC-6: When a player completes a row, column, or diagonal with their mark, the system shall announce the winner and terminate the game.
- AC-7: When all nine cells are filled with no winner, the system shall announce a draw and terminate the game.
- AC-8: If the input stream ends (EOF signal) at any prompt, the system shall print a goodbye message and exit with a zero status code.
- AC-9: The program and its tests shall compile with `go build ./...` and pass `go test ./...` and the project's lint gate without errors, warnings, or lint violations.

## Explicit non-goals

- No AI opponent or computer player of any kind.
- No web server, HTTP API, or network listener.
- No persistence (no save files, no database, no board state across restarts).
- No configuration files, CLI flags, environment variables for game setup, or dependency injection framework.
- No graphical UI beyond terminal text output; no terminal UI library outside the Go standard library.
- No undo, replay, or move history.
- No multi-game session or score tracking.
- No support for more than two players or non-standard board sizes.
- No packaging, containerisation, or deployment artefacts.
- No separate application module created inside the task directory; the task directory holds only workflow documents, story logs, review artefacts, and execution evidence.

## Assumptions

- The game runs in a UTF-8-capable terminal. The program will use ASCII characters for the board (e.g., `X`, `O`, `-` or `.` for empty cells).
- Standard input is line-buffered; the player presses Enter after typing a cell number. Non-numeric input means input that cannot be parsed as an integer.
- The standard `testing` package (Go standard library) is an acceptable test harness. The program depends only on the Go standard library.

## Constraints

- Go standard library only for application code. The test file may import the standard `testing` package.
- Source and build files (`go.mod`, application `.go` files, test `.go` files) belong in the project root directory (where `watts.yaml` lives), not inside the task directory.
- The implementation must pass `go build ./...`, `go test ./...`, and `bash .watts/packages/go-generalist/f3e80902dcdbad2520bb679945135271090e3f72df99467ff306fed2d1f0491f/scripts/lint.sh` (golangci-lint v2 with the team's `.golangci.yml` configuration).
- All lint findings must be resolved without weakening the lint configuration.
- Keep the implementation small and readable; one `main.go` plus a test file is the preferred structure.

## Open questions

1. What board display format does the team prefer? The standard 3×3 grid with borders (e.g., `---+---`) or a simpler representation such as three lines of space-separated marks? Either works; this spec assumes a bordered grid for clarity.
2. What message text is expected on EOF ("Goodbye.", empty line + exit, or another convention)? The spec assumes `"Goodbye."` on its own line followed by clean exit, but this is easily adjusted.
3. Should the program accept lowercase `x` and `o` in addition to uppercase `X` and `O` in any context? The spec assumes uppercase only; the program outputs uppercase marks.
4. Is running `go build ./...` and `go test ./...` from the project root sufficient, or does the lint gate replace the `go vet ./...` step? The workflow template listed `go vet ./...` originally, but the configured lint script (golangci-lint) includes govet already. This spec uses the actual configured gates; please confirm.

## Risk-relevant context

- Touches: A new terminal application that reads from stdin and writes to stdout. No filesystem writes, no networking, no identity or access logic.
- Does not touch: Existing project source code, authentication, authorisation, persistence, credentials, secrets, configuration, budget logic, entitlement logic, pipeline stages, service registries, governance fields, or any production-adjacent system.

## Risk tier
0

*Justification: A mistake is cosmetic or immediately visible (wrong board, missed win, crash on bad input) and trivially reverted — the entire program is a single-prompt terminal game with no side effects.*

## Review

An agent drafts this document. The human approves or rejects it at the spec-review workflow gate with `watts task decide <task> spec-review`, optionally using `--reject --feedback 'changes needed'`.