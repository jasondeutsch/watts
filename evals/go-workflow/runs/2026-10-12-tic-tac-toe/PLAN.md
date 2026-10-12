# PLAN: Tic-tac-toe — minimal terminal game for two human players

Task documents: /runtime/task/
Implementation root: /workspace

Application source, tests, dependencies, and build files belong in the project root and its normal source directories. The task directory stores workflow documents and evidence only. Do not create a separate application or module under the task directory.
Spec ref: SPEC.md v1
Risk tier at Planning: 0

*The planning agent drafts this file. The plan-review workflow gate records human approval before implementation starts.*

## Architecture and approach

A single Go module (`go.mod` with module path `tic-tac-toe`) at `/workspace`. The binary is built from a single `main.go` that reads from stdin and writes to stdout/stderr. The game state, board rendering, move validation, and win/draw detection live in internal functions callable from tests without launching the binary.

The control flow is:

1. Print the empty numbered board and prompt Player X.
2. Read a line from stdin.
3. On EOF, print an exit message and return 0.
4. Parse and validate the input; on error, print the message and re-prompt the same player.
5. Place the mark, check for win/draw, announce if terminal, otherwise toggle the active player and repeat.

Board state is represented as a `[9]rune` (or equivalent) slice indexed 0–8 corresponding to cells 1–9. A function `render(board [9]rune, showNumbers bool) string` produces the grid display. A function `checkWin(board [9]rune) (winner rune, won bool)` scans the eight winning lines. A function `isDraw(board [9]rune) bool` checks whether all cells are occupied with no winner.

No separate packages are needed beyond `main`. Internal functions are tested via `*_test.go` files in the same package.

## Data model and interface changes

None — greenfield project with no existing interfaces or data model.

Internal types (all in package `main`):
- `board [9]rune` — cell values, unicode `'X'`, `'O'`, or `' '`/`0` for empty.
- `render(board, bool) string` — produces the ASCII grid.
- `checkWin([9]rune) (rune, bool)` — win detection.
- `isDraw([9]rune) bool` — draw detection.
- `parseInput(string) (int, error)` — input parsing and validation.

## Systems touched

Cross-checked against the risk-tiering skill, not from memory.

- Touches: A new Go directory under `/workspace`. No existing production code, no network, no storage, no credentials.
- Does not touch: Any existing service, database, CI pipeline, deployment target, secrets store, identity provider, or configuration system.

## Technical decisions and alternatives considered

This is a Tier 0 task so this section can be concise, but it is not collapsed.

- **Standard library only, no third-party packages.** The spec constrains this; it is correct because the application does nothing that warrants an external dependency.
- **Single package `main` rather than a library package.** The program is a single binary with no reuse outside itself. Internal functions are tested directly in `main_test.go`. This avoids unnecessary package boundaries that would add no value.
- **`[9]rune` array rather than a slice or 2-D grid.** A flat array is idiomatic for a fixed-size board; cell-to-index mapping (1–9 → 0–8) is trivial. A 2-D `[3][3]rune` would work equally well but adds syntactic friction for win-line scanning.
- **Input via `bufio.Scanner` on `os.Stdin` rather than `fmt.Scan` or raw terminal read.** `bufio.Scanner` handles line boundaries consistently, reports EOF without ambiguity, and is part of the standard library. `fmt.Scan` is harder to test and more error-prone on unexpected input.

## Testing strategy

Unit tests only, same package `main`, file `main_test.go` (and optionally `board_test.go` or similar splits). Integration testing is unnecessary for this tier — stdin/stdout can be exercised via `strings.NewReader` and `bytes.Buffer` in unit tests.

Specific test targets:
- `TestRender` — empty board, board mid-game, board full.
- `TestCheckWin` — all eight winning lines for both players; no-window scenarios.
- `TestIsDraw` — full board with no winner; board not full; board with a winner.
- `TestParseInput` — valid 1–9, out-of-range, non-numeric, empty string.
- `TestPlayGame` — full-game simulations via substituting stdin and capturing stdout.

The lint gate is not a testing strategy concern but a quality gate; it checks formatting and static analysis.

## Observability

Not applicable. The program is a terminal game with no persistent runtime, no network service, and no production deployment. Its only observable output is the board display and result messages printed to stdout/stderr during interactive play.

## Security and privacy considerations

None. The program reads only from stdin, writes only to stdout/stderr, performs no network I/O, stores no data, and handles no credentials or personal information.

## Performance considerations

Negligible load — one user at a time, at human typing speed. A single-threaded, synchronous read-loop uses trivial CPU and memory (< 1 MB heap). No edge of budget worth describing.

## Risk tier recheck

Run the risk-tiering skill again against what this plan actually describes, not what SPEC.md assumed before the technical approach was known.

A newly-created Go terminal application in a standalone workspace:

- **Does not touch** any existing production code, credentials, entitlements, identity, network services, persistent storage, or configuration infrastructure.
- **A mistake is cosmetic, immediately visible (wrong board output, wrong winner, wrong draw), and trivially reverted (delete the directory).** A mistake cannot cause cross-identity spending or access violation because no identities exist. A mistake cannot silently change routing or behavior because the program has no network or storage side effects.
- **Risk classification matches Tier 0 exactly** per the skill's rubric ("cosmetic, immediately visible, and trivially reverted").

Risk tier after planning: 0
Changed from Planning and Requirements: no
If yes, why: n/a

## Rollback and migration

Not required at Tier 0. If the implementation is wrong, delete `/workspace/go.mod`, `/workspace/*.go`, and `/workspace/*_test.go`, then start again.

## Dependencies and sequencing

No sequencing conflicts — this is a standalone greenfield project with no other work in flight that touches `/workspace`.

## Quality gates

Commands every story must pass before it counts as done, one backticked command per bullet. Real commands for this repository, run from the repository root, not placeholders. The builder runs them, and the reviewer and the Watts checks run them again independently.

- `go build ./...`
- `go vet ./...`
- `go test ./...`
- `bash /inputs/resources/team/scripts/lint.sh`

Lint findings must be fixed without weakening the lint configuration.

## User stories

*The build loop works through these in order. Keep the heading format exactly (`### US-001: Title`). Each story must be small enough for one agent iteration, with checkable acceptance criteria, a `Covers:` line naming SPEC criteria, and a `Depends on:` line. Every SPEC criterion must be covered by some story. Put the riskiest or approach-proving story first.*

### US-001: Project scaffolding and board rendering

Covers: AC-1
Depends on: none

**As a** developer
**I want** an initialised Go module with a `main.go` that compiles and, when run, prints the empty numbered board and prompts Player X
**So that** the project skeleton is in place and the first acceptance criterion is satisfied

#### Acceptance Criteria
- [ ] `go.mod` exists at `/workspace` with module path `tic-tac-toe` and uses `go 1.21` (or later as available in the build image).
- [ ] `main.go` compiles with `go build ./...` and `go vet ./...`.
- [ ] Running `go run .` prints a 3×3 board labelled with cell numbers 1–9 using an ASCII grid (e.g., `|` and `-`) and the prompt `Player X, enter a cell number (1–9): ` (exact wording up to implementation).
- [ ] All quality gates pass.

### US-002: Turn management and move input

Covers: AC-2, AC-3, AC-4, AC-5
Depends on: US-001

**As a** player
**I want** to enter a cell number and see my mark placed, then the turn switch to the other player
**So that** the basic game loop works interactively

#### Acceptance Criteria
- [ ] When a player enters a valid unoccupied cell number (1–9), the board updates with that player's mark and the display is re-printed.
- [ ] After a valid move, the active player toggles between X and O.
- [ ] When a player enters an occupied cell number, an error message is printed, the board is re-displayed, and the same player is prompted again without changing the board.
- [ ] When a player enters a number outside 1–9, an error message is printed and the same player is prompted again.
- [ ] When a player enters non-numeric input, an error message is printed and the same player is prompted again.
- [ ] All quality gates pass.

### US-003: Win detection

Covers: AC-6
Depends on: US-002

**As a** player
**I want** the game to detect when I have three in a row, column, or diagonal
**So that** the winner is announced and the program exits

#### Acceptance Criteria
- [ ] All eight winning lines (three rows, three columns, two diagonals) are detected for either player.
- [ ] When a move creates a win, the winning player is announced (e.g., `Player X wins!`) and the program exits with exit code 0.
- [ ] A move that does not create a win does not trigger the announcement.
- [ ] All quality gates pass.

### US-004: Draw detection and EOF handling

Covers: AC-7, AC-8
Depends on: US-003

**As a** player
**I want** the game to detect a full board with no winner and to handle an EOF gracefully
**So that** the game always terminates with a clear message, whether completed or abandoned

#### Acceptance Criteria
- [ ] When the board is full with no winner, a draw message is printed and the program exits with exit code 0.
- [ ] When EOF (Ctrl+D / Ctrl+Z) is read at any input prompt, an exit message is printed and the program exits with exit code 0.
- [ ] A draw after the last possible winning move is not overridden by a false win detection.
- [ ] All quality gates pass.

### US-005: Test suite and lint compliance

Covers: AC-9, AC-10
Depends on: US-004

**As a** developer
**I want** complete unit tests and a clean lint pass
**So that** every acceptance criterion is verifiable by automated test and the project passes the configured lint gate

#### Acceptance Criteria
- [ ] `go test ./...` passes with no failures (AC-9).
- [ ] `bash /inputs/resources/team/scripts/lint.sh` reports zero errors (AC-10).
- [ ] Tests cover: board rendering, win detection, draw detection, input parsing, and at least one full-game simulation.
- [ ] Test coverage is meaningful (not trivially empty assertions).
- [ ] All quality gates pass.

## Open questions for the author agent

None. The spec is unambiguous about scope, rules, cell numbering, and constraints. The plan resolves all implementation decisions (single package, `bufio.Scanner`, flat `[9]rune` board). No ambiguity needs the author agent to pause and ask.

---

## Review

An agent drafts this document. The human approves or rejects it at the plan-review workflow gate with `wctl task decide <task> plan-review`, optionally using `--reject --feedback 'changes needed'`.