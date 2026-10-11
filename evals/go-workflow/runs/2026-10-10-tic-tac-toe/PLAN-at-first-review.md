# PLAN: Minimal Go tic-tac-toe

Task documents: tasks/2026-10-10-tic-tac-toe-eval/
Implementation root: the project directory containing watts.yaml

Application source, tests, dependencies, and build files belong in the project root and its normal source directories. The task directory stores workflow documents and evidence only. Do not create a separate application or module under the task directory.
Spec ref: SPEC.md v1
Risk tier at Planning: 0, from SPEC.md

*The planning agent drafts this file. The plan-review workflow gate records human approval before implementation starts.*

## Architecture and approach

A fresh, self-contained Go 1.26 module in `package main` at the project root (`/Users/j/watts_project/test`), standard library only. No existing Go code, `go.mod`, or `go.sum` exists in the project root today, so the first story creates the module (`go mod init tictactoe`) and proves the toolchain against the team lint configuration.

Three small source files plus tests, all in the project root:

- `main.go` — entry point and the interactive REPL: prints the board, prompts the current player, reads a line from stdin with `bufio.Scanner`, parses with `strconv.Atoi`, dispatches invalid input to a re-prompt without changing the turn, applies valid moves, announces win/draw, and handles EOF with a goodbye message and exit status 0.
- `board.go` — the pure board model: `Board` holding 9 cells, `Place` validation (out-of-range and occupied with sentinel errors), and `String()` rendering the 3×3 ASCII grid with the cell's number (1–9) shown for free cells and `X`/`O` for taken cells.
- `game.go` — the pure turn/game state machine: current player, `ApplyMove` (validate → place → switch player), `Winner` over the eight lines, and draw detection.

Spec's "one `main.go` plus a test file" is a stated preference, not a constraint; splitting the pure logic (board, game state) from the I/O REPL in `main.go` keeps every acceptance criterion unit-testable without a terminal, while staying tiny and standard-library-only. The riskiest part — stdin interaction, EOF behavior, and subprocess testing — is isolated in `main.go` and covered by the last story, so everything it depends on is already green.

Concrete UI contract (resolving the spec-review feedback that board format and EOF wording are open):

- Board render (free cells numbered, taken cells marked):
  ```
   X | 2 | 3
  ---+---+---
   4 | O | 6
  ---+---+---
   7 | 8 | 9
  ```
- Prompt: `Player X (1-9): ` and `Player O (1-9): ` (input is numeric only, so symbol case is moot).
- Invalid-input messages: `invalid cell 12: must be a number from 1 to 9` (out of range / non-numeric) and `cell 5 is already taken` (occupied), each followed by a re-prompt of the same player.
- Win: `Player X wins!` / `Player O wins!`. Draw: `It's a draw.`. EOF: `Goodbye.` printed, exit status 0.

## Data model and interface changes

No external interfaces, schemas, or API contracts change; this is a new standalone CLI. Internal Go types (all in `package main`):

- `type Mark rune` with constants `MarkEmpty Mark = 0`, `MarkX Mark = 'X'`, `MarkO Mark = 'O'`.
- `type Board struct { cells [9]Mark }` — cell index 0 = top-left, 8 = bottom-right; keyed by cell number 1–9 via `cell-1`. Methods: `NewBoard() Board`, `MarkAt(cell int) Mark`, `Place(cell int, m Mark) error`, `String() string`, `full() bool` (unexported).
- Sentinel errors (from `errors.New`): `errOutOfRange`, `errOccupied`.
- `type Game struct { board Board; turn Mark }` with `NewGame() *Game`, `CurrentPlayer() Mark`, `ApplyMove(cell int) error` (returns the sentinel errors, switches turn only on success), `Winner() (Mark, bool)`, `IsDraw() bool`.

Maintenance of state: board lives only in memory for the current process; there is no persistence.

## Systems touched

Cross-checked against the risk-tiering skill, not from memory.

- Touches: new files in the project root — `go.mod`, `go.sum` (if any), `main.go`, `board.go`, `game.go`, `board_test.go`, `game_test.go`, `main_test.go`. Reads stdin, writes stdout/stderr of the terminal process only.
- Does not touch: any existing source (none exists), `watts.yaml`, `values.yaml`, `Makefile`, `.watts/`, `tasks/`, `templates/`, any networking, filesystem writes, identity, credentials, or persistence.

## Technical decisions and alternatives considered

- **Module layout under `package main` in the project root** versus a library package or a module under the task directory. Chosen: project-root `package main` with three small files, because SPEC's constraints put source in the project root and forbid an application module in the task directory, and the non-goals rule out packaging.
- **Splitting `board.go` / `game.go` out of `main.go`** versus SPEC's preferred single `main.go` + one test file. Chosen: three files, because pure functions over the board and turn state can then be tested without spawning a process or faking stdin; the deviation is noted because SPEC calls the single-file shape only a preference. The REPL stays in `main.go` alone.
- **`bufio.Scanner` + `strconv.Atoi`** versus `bufio.Reader.ReadString` or fmt scanning. Chosen: Scanner handles line-buffered Enter-pressed input (SPEC assumption), Atoi gives a clean non-numeric parse error, and `Scanner.Err()` surfaces `io.EOF` for the AC-8 goodbye path.
- **Sentinel errors for validation** versus a single generic error string. Chosen: sentinel errors so the REPL and tests can distinguish and message AC-3 (out of range/non-numeric) from AC-4 (occupied) without string matching (team guidance: `errors.Is`, not message comparison).
- **Rendering free cells as their 1–9 numbers** versus dashes/dots. Chosen: numbers, per the resolved spec-review question ("show cell numbers 1–9 for positions that are still available"); it doubles as the position keypad map.
- **Gates without a separate `go vet ./...`**. The resolved spec-review feedback set the gate list as `go build ./...` + `go test ./...` + the team golangci-lint script; `govet` is enabled in `.golangci.yml`, so vet coverage is inside the lint gate. A standalone `go vet` run is not redundant and is not part of AC-9's wording, so it is not a story gate.
- Rejected: any third-party dependency (testify included) — SPEC constrains app code and the test file to the standard library; table-driven `testing` subtests cover the same ground.

## Testing strategy

- **Unit tests (fast, pure, no I/O)** in `board_test.go` and `game_test.go`, table-driven with named subtests:
  - Rendering: empty board shows all numbers 1–9; mixed board shows X/O and numbers in the right cells.
  - `Place`/`MarkAt`: valid fills; out-of-range at 0, -1, 10; occupied rejection with `errors.Is`; placement on a board that already has a winner leaves state unchanged.
  - Turn logic: `ApplyMove` alternates X→O→X; failed moves (out of range, occupied, non-numeric parse earlier in REPL) keep the same player's turn.
  - Win/draw: all eight lines (3 rows, 3 columns, 2 diagonals) for X and O; draw with all nine filled; no false positive on a game that is neither won nor full; no win through a blocked line.
- **Integration tests (subprocess)** in `main_test.go`: build the binary once into a temp dir (TestMain + `os/exec`), then run it with scripted stdin via `strings.NewReader` for: a complete X win, a complete draw, invalid inputs (non-numeric, out-of-range, occupied) followed by recovery on the same player's turn, and immediate EOF, asserting exact output and exit codes (0 on EOF/game end).
- **Explicitly left untested**: terminal-specific behavior such as cursor control or colored output — the program emits plain text and the subprocess tests capture that text, so there is nothing terminal-specific to test. There is no concurrency or persistence to test.
- Every story ends green on all quality gates; lint findings are fixed without weakening `.golangci.yml`.

## Observability

A single-session terminal game: its stdout text is the entire observable behavior, captured verbatim by the subprocess integration tests. Error messages for invalid input go to stdout as part of gameplay (SPEC requires displaying them); only unexpected internal failures would surface as non-zero exit codes. There is nothing to log, measure, or alert on; the acceptance criteria themselves (win/draw/invalid/EOF messages and exit status) are the observability contract.

## Security and privacy considerations

No data is handled beyond a line of stdin typed by a local player; no secrets, credentials, identity, or access logic. The program writes only to the terminal. Nothing requires review beyond the Tier 0 assessment. The only input attack surface is a malformed line, which is handled by `Atoi`'s parse error plus the sentinel-error paths; buffer-size limits on a scanned line (Scanner default 64 KiB) bound any pathological input.

## Performance considerations

One human move per turn; per-move work is constant (parse, validate, render 9 cells, check 8 lines). Memory is a single 9-cell board. No concurrency, no external calls, no latency budget. There are no failure modes beyond stdin ending (handled, exit 0) and assertion of over-long lines (Scanner token-too-long error → treated as invalid input with re-prompt).

## Risk tier recheck

Run the risk-tiering skill again against what this plan actually describes, not what SPEC.md assumed before the technical approach was known.

Risk tier after planning: 0
Changed from Planning and Requirements: no
If yes, why: n/a — the plan adds nothing SPEC.md did not already describe: a new, self-contained, read-stdin/write-stdout terminal program with no filesystem, network, identity, credentials, or entitlement logic. Any mistake is cosmetic or immediately visible and trivially reverted, per the Tier 0 rule.

## Rollback and migration

Required above Tier 0: n/a (Tier 0). If wrong, delete the new files (`go.mod`, `go.sum`, `main.go`, `board.go`, `game.go`, `*_test.go`); nothing pre-existing is modified, so the project root is restored exactly. No data migration, no schema, no deployment.

## Dependencies and sequencing

Nothing blocks this task: the project root is an empty Go workspace with no other work in flight. This task blocks nothing else. Within the task, stories are strictly sequential (`US-00N` depends on all earlier stories) because the REPL story needs board, validation, and game-state logic to exist and pass before it can be integration-tested. The lint script resolves its own `.golangci.yml` relative to the go-generalist package; no setup beyond having `golangci-lint` v2 on PATH (verified present: v2.11.3) is required.

## Quality gates

Commands every story must pass before it counts as done, one backticked command per bullet. Real commands for this repository, run from the repository root, not placeholders. The builder runs them, and the reviewer and the Watts checks run them again independently.

- `go build ./...`
- `go test ./...`
- `bash .watts/packages/go-generalist/f3e80902dcdbad2520bb679945135271090e3f72df99467ff306fed2d1f0491f/scripts/lint.sh`

## User stories

*The build loop works through these in order. Keep the heading format exactly (`### US-001: Title`). Each story must be small enough for one agent iteration, with checkable acceptance criteria, a `Covers:` line naming SPEC criteria, and a `Depends on:` line. Every SPEC criterion must be covered by some story. Put the riskiest or approach-proving story first.*

### US-001: Scaffold the Go module with board model and rendering

Covers: AC-1, AC-9
Depends on: none

**As a** developer setting up the task
**I want** the Go module initialized with a board type that renders a numbered empty grid
**So that** the project root has a buildable, lint-clean foundation proving the toolchain before any gameplay logic.

#### Acceptance Criteria
- [ ] `go.mod` exists in the project root with module path `tictactoe` and the Go 1.26 toolchain directive; no third-party dependencies.
- [ ] `board.go` defines `Mark`, `MarkEmpty`/`MarkX`/`MarkO`, `Board` with a 9-cell state, `NewBoard`, `MarkAt`, `Place`, and `String`.
- [ ] `String()` on a fresh board renders exactly the 3×3 grid with cells labeled 1–9 in keypad order (top-left = 1, bottom-right = 9).
- [ ] `main.go` compiles with a runnable entry point that exercises the render (temporary; the full REPL arrives in US-005).
- [ ] `board_test.go` table-driven tests cover empty-board rendering and cell numbering.
- [ ] All quality gates pass from the project root (`go build ./...`, `go test ./...`, lint.sh) with zero findings.

### US-002: Move placement with out-of-range and occupied rejection

Covers: AC-3, AC-4
Depends on: US-001

**As a** player
**I want** `Place` to reject a cell outside 1–9 or an already-taken cell with distinguishable errors
**So that** the game loop can display the right error and re-prompt the same player.

#### Acceptance Criteria
- [ ] `Place` fills an empty cell in range and returns nil; `MarkAt` reflects the mark.
- [ ] `Place` on cell 0, -1, 10, or 11 returns `errOutOfRange` and leaves the board unchanged.
- [ ] `Place` on an already-taken cell returns `errOccupied` and leaves the board unchanged; tests assert with `errors.Is`, not message text.
- [ ] Rejecting a move does not change the board state at all.
- [ ] Table-driven unit tests cover valid, boundary (1 and 9), out-of-range, and occupied cases.
- [ ] All quality gates pass from the project root.

### US-003: Win and draw detection

Covers: AC-6, AC-7
Depends on: US-001

**As a** player
**I want** the game state to detect a winning line or a full board with no winner
**So that** the program can announce the outcome and terminate.

#### Acceptance Criteria
- [ ] `Winner()` detects every row, column, and both diagonals for X and for O (eight lines × both marks).
- [ ] `Winner()` returns false for an in-progress board and for a board with no unbroken line (e.g., a blocked line).
- [ ] Draw detection: a fully filled board with no winner reports a draw; a board with a winner reports the winner, not a draw.
- [ ] Table-driven unit tests enumerate all winning lines explicitly rather than looping over a counter in the implementation.
- [ ] All quality gates pass from the project root.

### US-004: Turn state machine — valid move applies and switches player, invalid keeps turn

Covers: AC-2
Depends on: US-002, US-003

**As a** player
**I want** a `Game` that alternates X/O on valid moves, applies the mark, and keeps the same player's turn on rejected moves
**So that** AC-2's redraw-and-next-prompt behavior and AC-3/AC-4's re-prompt behavior have a correct, testable core before the REPL exists.

#### Acceptance Criteria
- [ ] `NewGame` starts with Player X's turn and an empty board.
- [ ] `ApplyMove` with a valid cell places the mark and switches the turn (X→O→X…).
- [ ] `ApplyMove` with an out-of-range or occupied cell returns the sentinel error, does not place a mark, and does not change the current player.
- [ ] A move that completes a line makes `Winner()` non-empty; a move filling the last free cell without a line makes `IsDraw()` true.
- [ ] Unit tests cover turn alternation, error-keeps-turn, win-on-move, and draw-on-move transitions.
- [ ] All quality gates pass from the project root.

### US-005: Interactive game loop in main.go with prompts, invalid-input handling, outcome and EOF

Covers: AC-1, AC-2, AC-3, AC-4, AC-5, AC-6, AC-7, AC-8
Depends on: US-004

**As a** player starting the program
**I want** a complete terminal session — start board, alternating prompts, redraw after each valid move, error + re-prompt on bad input, win/draw announcement, and a clean goodbye on EOF
**So that** the game is fully playable by two humans on the same machine using only the standard library.

#### Acceptance Criteria
- [ ] On startup, prints the empty board then `Player X (1-9): ` (AC-1).
- [ ] After a valid move, redraws the board with the mark placed, then prompts the next player (AC-2).
- [ ] Non-numeric input (e.g., `abc`, blank line with spaces, `1.5`) prints an error, does not change the turn, and re-prompts the same player (AC-5).
- [ ] Out-of-range (0, -1, 10, 12) and occupied cells print their specific error, do not change the turn, and re-prompt the same player (AC-3, AC-4).
- [ ] A completed row, column, or diagonal prints `Player <X|O> wins!` and terminates (AC-6); a full board with no winner prints `It's a draw.` and terminates (AC-7).
- [ ] EOF at any prompt (including the first) prints `Goodbye.` and exits with status 0 (AC-8).
- [ ] `main_test.go` subprocess tests cover: an X win, a draw, each invalid-input class followed by recovery on the same player's turn, and immediate EOF — asserting exact messages and exit codes.
- [ ] All quality gates pass from the project root.

## Open questions for the author agent

None. All ambiguity from the spec-review rounds is resolved here in the plan: board format (3×3 ASCII grid, free cells numbered 1–9), prompt and outcome wording (fixed strings above), invalid-input handling (numeric cells only, errors via sentinels), EOF wording and exit status (`Goodbye.`, 0), and the gate list (`go build` + `go test` + team lint script; vet covered by the lint gate's `govet`). Any contradiction discovered during build goes to the human gate per the workflow, not worked around.

## Review

An agent drafts this document. The human approves or rejects it at the plan-review workflow gate with `watts task decide <task> plan-review`, optionally using `--reject --feedback 'changes needed'`.