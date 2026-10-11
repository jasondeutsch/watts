# PLAN: Minimal Go tic-tac-toe

Task documents: tasks/2026-10-10-tic-tac-toe-eval/
Implementation root: the project directory containing watts.yaml

Application source, tests, dependencies, and build files belong in the project root and its normal source directories. The task directory stores workflow documents and evidence only. Do not create a separate application or module under the task directory.
Spec ref: SPEC.md v1
Risk tier at Planning: 0, from SPEC.md

*The planning agent drafts this file. The plan-review workflow gate records human approval before implementation starts.*

## Architecture and approach

A fresh, self-contained Go 1.26 module in `package main` at the project root (`/Users/j/watts_project/test`), standard library only. No existing Go code, `go.mod`, or `go.sum` exists in the project root today, so the first story creates the module (`go mod init tictactoe`) and proves the toolchain against the team lint configuration.

Precisely two source files in the project root:

- `main.go` — everything: a 9-element `[9]string` cell array, small unexported functions for rendering (`renderBoard`), move validation (`validMove`), winner/draw detection (`checkWin`, `isDraw`), and a `playGame` function accepting `io.Reader` and `io.Writer` that contains the game loop. The `main()` entry point calls `playGame(os.Stdin, os.Stdout)`. No exported types, no constructors, no sentinel error variables — the program stays small enough to read as one unit.
- `main_test.go` — table-driven unit tests for the pure functions plus integration tests that call `playGame` directly with a `strings.Reader` as stdin and a `bytes.Buffer` as stdout, asserting exact output and avoiding subprocess compilation.

Cell numbering follows keypad order: cell index 0 = position 1 (top-left), cell index 8 = position 9 (bottom-right). The board is stored as `[9]string` where each element is `"1"`..`"9"` for free cells or `"X"`/`"O"` for taken cells. Free cells are shown by their number so the board doubles as a position reference.

The game loop uses `bufio.Scanner` over the provided `io.Reader`. `Scanner.Scan()` returns `false` at clean EOF (EOF signal / Ctrl-D) with `Scanner.Err() == nil` — this is the standard Go scannerEOF contract. The code checks `Scan()` result directly and does not call `Scanner.Err()` expecting `io.EOF`.

Concrete UI contract:

- Board render (free cells numbered, taken cells marked):
  ```
   X | 2 | 3
  ---+---+---
   4 | O | 6
  ---+---+---
   7 | 8 | 9
  ```
- Prompt: `Player X (1-9): ` and `Player O (1-9): `.
- Invalid-input messages: `Invalid cell: must be a number from 1 to 9` (non-numeric and out-of-range), `Cell 5 is already taken` (occupied), each followed by a re-prompt of the same player.
- Win: `Player X wins!` / `Player O wins!`. Draw: `It's a draw!`. EOF: `Goodbye.` printed, exit status 0.

## Data model and interface changes

No external interfaces, schemas, or API contracts change; this is a new standalone CLI. Internal state in `package main`:

- `cells [9]string` — the board, local variable in `playGame` and passed to helper functions.
- `player string` (`"X"` or `"O"`), current player.
- Helper function signatures in `package main`:
  - `func renderBoard(cells [9]string) string`
  - `func validMove(cells [9]string, cell int) (bool, string)` — returns (ok, errorMsg)
  - `func checkWin(cells [9]string) (string, bool)` — returns (winnerMark, won)
  - `func isDraw(cells [9]string) bool`
  - `func playGame(r io.Reader, w io.Writer) int` — returns exit code for test assertion.

No exported identifiers, no constructors, no sentinel errors. All state lives in memory for the current process; there is no persistence.

## Systems touched

Cross-checked against the risk-tiering skill, not from memory.

- Touches: new files in the project root — `go.mod`, `go.sum` (if any), `main.go`, `main_test.go`. Reads stdin, writes stdout/stderr of the terminal process only.
- Does not touch: any existing source (none exists), `watts.yaml`, `values.yaml`, `Makefile`, `.watts/`, `tasks/`, `templates/`, any networking, filesystem writes, identity, credentials, or persistence.

## Technical decisions and alternatives considered

- **Single `main.go`** versus splitting into `board.go`/`game.go` as in the prior plan draft. Chosen: one file, per the spec-review feedback explicitly rejecting split files. The entire program fits under 250 lines and remains readable as one unit; helper functions are grouped by concern (rendering, validation, win/draw, loop) with blank-line comments.
- **`[9]string` cell storage** versus a `Mark` type with constants. Chosen: simple strings over a custom type to avoid exported APIs that the feedback rejected. The strings `"1"`..`"9"` double as the display value and the "empty" sentinel; `"X"` and `"O"` are the taken marks. This eliminates any need for a `Mark` enum, `NewBoard` constructor, or sentinel error variables.
- **Direct `playGame` testability** (accepting `io.Reader`/`io.Writer`) versus subprocess compilation tests. Chosen: direct calls, per the feedback's explicit instruction — "unit tests need not be split across files and subprocess compilation is unnecessary when the game loop is directly testable." Each test creates a `strings.Reader` with scripted input, call `playGame`, and checks `bytes.Buffer` output and the return code.
- **`bufio.Scanner`** versus `fmt.Fscan` or `bufio.Reader.ReadString`. Chosen: Scanner handles line-buffered Enter-pressed input (SPEC assumption) and the `Scan()`-returns-`false` + `Err()==nil` pattern correctly identifies clean EOF without requiring a `io.EOF` check on `Scanner.Err`.
- **Returning `(ok bool, errMsg string)` from `validMove`** versus sentinel errors via `errors.New`/`errors.Is`. Chosen: plain string messages per the feedback's "do not introduce sentinel errors unless essential." The caller can distinguish out-of-range from occupied by checking the string, or by calling separate boolean-check helpers. A `strings.Contains` or prefix check is acceptable for the two distinct cases since this is a small teaching program.
- **Winner detection via hardcoded win lines** `[8][3]int` array inside `checkWin`. Eight lines (3 rows, 3 columns, 2 diagonals) are small enough to enumerate table-driven.
- **Two stories** versus five in the prior plan draft. Chosen: exactly two, per the feedback's explicit direction, with all nine ACs covered across them.
- Rejected: any third-party dependency (testify included) — SPEC constrains app code and the test file to the standard library; table-driven `testing` subtests cover the same ground.

## Testing strategy

- **Unit tests (fast, pure, no I/O)** in `main_test.go`, table-driven with named subtests:
  - `TestRenderBoard`: empty board shows all numbers 1–9; mixed board shows X/O and numbers in the right cells.
  - `TestValidMove`: valid cell in range (1–9) returns (true, ""); out-of-range (0, -1, 10, etc.) returns (false, msg); occupied cell returns (false, msg) distinct from out-of-range.
  - `TestCheckWin`: all eight lines (3 rows, 3 columns, 2 diagonals) for X and O; no false positive on an in-progress board or a blocked line.
  - `TestIsDraw`: full board with no winner returns true; board with winner returns false; in-progress board returns false.
- **Integration tests via `playGame`** in the same `main_test.go`, bringing in full game scenarios by providing scripted stdin as `strings.Reader` and capturing stdout via `bytes.Buffer`:
  - A complete X win (moves: 1, 9, 2, 8, 3 → X wins).
  - A complete draw (moves filling all cells without a winner).
  - Invalid inputs (non-numeric like `abc`, out-of-range like `12`, occupied cell) followed by recovery on the same player's turn.
  - Immediate EOF at the first prompt.
  - Assert exact output messages and exit status (0 on game end and EOF).
- **Explicitly left untested**: terminal-specific behavior such as cursor control or colored output — the program emits plain text. There is no concurrency or persistence to test.
- Every story ends green on all quality gates; lint findings are fixed without weakening `.golangci.yml`.

## Observability

A single-session terminal game: its stdout text is the entire observable behaviour, captured verbatim by the direct `playGame` integration tests. Error messages for invalid input go to stdout as part of gameplay. There is nothing to log, measure, or alert on; the acceptance criteria themselves (win/draw/invalid/EOF messages and exit status) are the observability contract.

## Security and privacy considerations

No data is handled beyond a line of stdin typed by a local player; no secrets, credentials, identity, or access logic. The program writes only to the terminal. Nothing requires review beyond the Tier 0 assessment. The only input attack surface is a malformed line, which is handled by `strconv.Atoi` parse error plus the valid-move validation; buffer-size limits on a scanned line (Scanner default 64 KiB) bound any pathological input.

## Performance considerations

One human move per turn; per-move work is constant (parse, validate, render 9 cells, check 8 lines). Memory is a single 9-cell array. No concurrency, no external calls, no latency budget. There are no failure modes beyond stdin ending (handled, exit 0) and assertion of over-long lines (Scanner token-too-long error → treated as invalid input with re-prompt).

## Risk tier recheck

Run the risk-tiering skill again against what this plan actually describes, not what SPEC.md assumed before the technical approach was known.

Risk tier after planning: 0
Changed from Planning and Requirements: no
If yes, why: n/a — the plan adds nothing SPEC.md did not already describe: a new, self-contained, read-stdin/write-stdout terminal program with no filesystem, network, identity, credentials, or entitlement logic. Any mistake is cosmetic or immediately visible and trivially reverted, per the Tier 0 rule.

## Rollback and migration

Required above Tier 0: n/a (Tier 0). If wrong, delete the new files (`go.mod`, `go.sum`, `main.go`, `main_test.go`); nothing pre-existing is modified, so the project root is restored exactly. No data migration, no schema, no deployment.

## Dependencies and sequencing

Nothing blocks this task: the project root is an empty Go workspace with no other work in flight. This task blocks nothing else. Within the task, US-001 (complete game) is built first and must pass all quality gates; US-002 (focused tests) then adds the full test suite and re-runs all gates. The lint script resolves its own `.golangci.yml` relative to the go-generalist package; no setup beyond having `golangci-lint` v2 on PATH (verified present: v2.11.3) is required.

## Quality gates

Commands every story must pass before it counts as done, one backticked command per bullet. Real commands for this repository, run from the repository root, not placeholders. The builder runs them, and the reviewer and the Watts checks run them again independently.

- `go build ./...`
- `go test ./...`
- `bash .watts/packages/go-generalist/f3e80902dcdbad2520bb679945135271090e3f72df99467ff306fed2d1f0491f/scripts/lint.sh`

## User stories

*The build loop works through these in order. Keep the heading format exactly (`### US-001: Title`). Each story must be small enough for one agent iteration, with checkable acceptance criteria, a `Covers:` line naming SPEC criteria, and a `Depends on:` line. Every SPEC criterion must be covered by some story. Put the riskiest or approach-proving story first.*

### US-001: Minimal complete game

Covers: AC-1, AC-2, AC-3, AC-4, AC-5, AC-6, AC-7, AC-8, AC-9
Depends on: none

**As a** player
**I want** a complete, playable terminal tic-tac-toe game that I can start and finish in one session
**So that** two humans on the same machine can play a full game with win, draw, invalid-input rejection, and clean EOF handling

#### Acceptance Criteria
- [ ] The module is initialized (`go.mod` with `tictactoe` module path, Go 1.26 toolchain directive, no third-party dependencies) and `go build ./...` produces a working binary.
- [ ] `main.go` contains all game logic as small unexported functions: `renderBoard`, `validMove`, `checkWin`, `isDraw`, `playGame(io.Reader, io.Writer)`, and `main()`.
- [ ] On start, the program prints the empty 3×3 board with cells numbered 1–9 and prompts `Player X (1-9): `.
- [ ] After a valid move, the board is redrawn with the player's mark in the chosen cell, then the next player is prompted.
- [ ] Out-of-range input (0, -1, 10, etc.) prints `Invalid cell: must be a number from 1 to 9` and re-prompts the same player without changing the turn.
- [ ] Occupied-cell input prints `Cell N is already taken` and re-prompts the same player without changing the turn.
- [ ] Non-numeric input prints `Invalid cell: must be a number from 1 to 9` and re-prompts the same player.
- [ ] A completed row, column, or diagonal prints `Player X wins!` or `Player O wins!` and the game ends.
- [ ] A full board with no winner prints `It's a draw!` and the game ends.
- [ ] EOF at any prompt prints `Goodbye.` and the program exits with status 0.
- [ ] The game loop functions (`renderBoard`, `validMove`, `checkWin`, `isDraw`, `playGame`) all compile and are reachable; no exported types, constructors, or sentinel errors exist.
- [ ] All quality gates pass from the project root (`go build ./...`, `go test ./...`, lint.sh) with zero findings.

### US-002: Focused behavioral tests and final verification

Covers: AC-1, AC-2, AC-3, AC-4, AC-5, AC-6, AC-7, AC-8, AC-9
Depends on: US-001

**As a** developer verifying correctness
**I want** a complete test suite covering every acceptance criterion with table-driven unit tests and integration tests calling `playGame` directly
**So that** the program is verified to handle all valid and invalid game states without subprocess compilation

#### Acceptance Criteria
- [ ] `TestRenderBoard` covers: empty board prints all numbers 1–9; mixed board prints X/O and remaining numbers in the right grid positions.
- [ ] `TestValidMove` covers: valid in-range (1–9) returns ok; out-of-range (0, -1, 10, 11, etc.) returns false; occupied cell returns false with message distinct from out-of-range message.
- [ ] `TestCheckWin` covers: all eight winning lines (3 rows, 3 columns, 2 diagonals) for both X and O — each as a named subtest, not looped over a counter — plus a false-negative test for an in-progress board and a blocked/no-win board.
- [ ] `TestIsDraw` covers: full board with no winner returns true; board with a winner returns false; in-progress board with free cells returns false.
- [ ] Integration tests in `TestPlayGame` cover:
  - A complete X win (moves 1, 9, 2, 8, 3) asserting exact output sequences and exit code 0.
  - A complete draw (moves filling the board without a winner) asserting exact output and exit code 0.
  - Non-numeric input ("abc") followed by recovery on the same turn and then a valid move.
  - Out-of-range input ("12") followed by recovery.
  - Occupied-cell input (two moves to same cell) followed by recovery.
  - Immediate EOF at the first prompt asserting `Goodbye.` and exit code 0.
- [ ] No subprocess compilation or `os/exec` is used; all tests call `playGame` directly with `strings.Reader`/`bytes.Buffer`.
- [ ] Every acceptance criterion (AC-1 through AC-9) from SPEC.md is covered by at least one test assertion.
- [ ] All quality gates pass from the project root (`go build ./...`, `go test ./...`, lint.sh) with zero findings.

## Open questions for the author agent

None. All ambiguity from the spec-review and plan-review rounds is resolved here: board format (3×3 ASCII grid, free cells numbered 1–9), prompt and outcome wording (fixed strings above), invalid-input handling (two distinct messages via string return from `validMove`), EOF wording and exit status (`Goodbye.`, 0 via `Scan()` false + `Err()==nil` contract), file count (one `main.go` + one `main_test.go` + `go.mod` only), and the gate list (`go build ./...` + `go test ./...` + team lint script). No exported APIs, no sentinel errors, no subprocess tests. Any contradiction discovered during build goes to the human gate per the workflow, not worked around.

## Review

An agent drafts this document. The human approves or rejects it at the plan-review workflow gate with `watts task decide <task> plan-review`, optionally using `--reject --feedback 'changes needed'`.