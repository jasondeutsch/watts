# PLAN: Tic-tac-toe web UI

Task documents: /runtime/task
Implementation root: /workspace

Application source, tests, dependencies, and build files belong in the project root and its normal source directories. The task directory stores workflow documents and evidence only. Do not create a separate application or module under the task directory.
Spec ref: SPEC.md v2
Risk tier at Planning: 1

*The planning agent drafts this file. The plan-review workflow gate records human approval before implementation starts.*

## Architecture and approach

Add a loopback HTTP server (`httptest.Server`-friendly) to the existing `main` package so the same `board`, `checkWin`, `isDraw`, and `parseInput` functions power a browser UI. No new package is necessary or desired; the web handler lives in a new `web.go` (and `web_test.go`) at the project root, conditionally started via a `-addr` flag or subcommand so the default `go run .` continues to start the terminal game.

- **Server**: A plain `net/http` server in a new `startWeb` function. Serving three routes:
  - `GET /` — returns the HTML page (embed or `html/template`).
  - `POST /move` — accepts `{"cell":N}` (1–9), validates with `parseInput`, checks occupancy with `b[pos] != empty`, applies the move, returns JSON `{board:[...], turn:"X"|"O"|"win"|"draw"}`.
  - `POST /reset` — clears the board, returns the initial empty board JSON.
- **State**: A single global (or `sync.Mutex`-guarded) `gameState` struct holding the `board`, current player, and game-over flag. The `sync.Mutex` serialises concurrent requests as required by the spec.
- **HTML/CSS/JS**: A minimal hand-written HTML page served inline (or as an `embed`-ed string) with zero external dependencies. CSS uses flexbox/grid for responsive layout. JS fetches `POST /move` and `POST /reset` and updates the DOM. The JS calls `parseInt` and validates 1–9 client-side before sending, but the server is authoritative.
- **Entry point**: `main` checks for `os.Args` containing a `"web"` subcommand or an `-addr` flag. If absent, runs the existing terminal game unchanged. The terminal game code does not import `net/http`. Documentation explains both modes.
- **Testing**: `web_test.go` starts a test server, issues `httptest.NewRequest` / `http.Client` calls, validates JSON responses and board state transitions. Reuses existing `board` construction helpers.

No frontend framework, no CDN, no new Go module dependencies. Only `net/http`, `encoding/json`, `embed`, `html/template`, `sync` from the standard library.

## Data model and interface changes

New server-side `gameState` struct (internal to `web.go`, not exported):

```go
type gameState struct {
    mu     sync.Mutex
    board  board
    player rune     // current player ('X' or 'O')
    over   bool     // true when someone won or board is full
}
```

New JSON message types for the move/reset endpoints (request/response structs, not exported if not needed beyond marshalling).

`main` entry point gains a `-addr` flag (default empty, terminal mode when empty) or a `web` subcommand.

HTTP API:

- `POST /move` — request `{"cell":N}`, response `{"board":["","X","O",...],"turn":"X"|"O"|"X_wins"|"O_wins"|"draw","over":true|false}`
- `POST /reset` — response same shape as an initial empty board

## Systems touched

Cross-checked against the risk-tiering skill, not from memory.

- Touches: `main.go` entry point (add flag/subcommand detection), new `web.go` / `web_test.go` project root files, `go.mod` (no dependency changes), team build/lint/test scripts run from workspace root.
- Does not touch: the existing `ttt` binary (it is not rebuilt), any authentication, credential handling, database, production services, deployment configuration, identity/entitlement logic, existing test behaviour (`main_test.go` unchanged), any task-directory or runtime metadata.

## Technical decisions and alternatives considered

| Decision | Chosen approach | Alternatives rejected |
|---|---|---|
| Package location | Same `main` package, new `web.go` / `web_test.go` at project root. | New `web` sub‑package would require exporting game types; the existing functions are unexported and the spec says reuse them directly. Adding a package under `cmd/web/` would create a separate `main` with duplication. Same‑package keeps everything in one build and avoids refactoring exports. |
| Entry‑point switching | Inspect `os.Args` for `"web"` subcommand (or `-addr` flag). If absent, run the terminal game exactly as today. | Always starting the server (e.g. in a goroutine) would change the terminal‑only behaviour and add `net/http` imports to the terminal path. |
| State guarding | `sync.Mutex` on the `gameState` struct. | Channel‑based serialisation would be overkill for a single shared board; an atomic swap of a whole board pointer is possible but mutex is clearer and trivially correct. |
| HTML delivery | `//go:embed` of a single `index.html` file into a string variable. | `html/template` for server‑side rendering adds complexity; the JS can render the board from JSON. Embedding avoids a separate file read at runtime. |
| Win detection in UI | The server sends `turn: "X_wins"|"O_wins"|"draw"` as a string; the JS displays the outcome text. JS does not re‑implement `checkWin`. | Having the JS independently check win would duplicate game logic and violate AC‑5. |
| Race handling | Every public method of `gameState` locks the mutex before reading or writing. The board never changes between evaluation and placement because they happen inside one locked method. | Unlocked reads would let concurrent requests observe intermediate state. |

## Testing strategy

- **Unit tests** (existing `main_test.go`): untouched; still cover terminal game rendering, parsing, win/draw detection, occupied-cell and invalid-input turn retention, EOF behaviour.
- **Web handler tests** (`web_test.go`): use `httptest.NewServer` to exercise `POST /move` and `POST /reset` end‑to-end. Cover:
  - Valid moves advancing turn and updating the board.
  - Occupied cells returning an error without changing state or turn.
  - Malformed JSON, missing cell field, out‑of‑range cell numbers.
  - Reset after partial game and after a completed game.
  - Moves attempted after win or draw are rejected.
  - Concurrent requests produce a consistent board (serialisation).
  - `GET /` returns 200 and the HTML page.
- **Not tested**: visual layout / CSS pixel rendering; manual review of narrow viewport behaviour suffices for a local‑dev UI.

## Observability

The HTTP server logs minimal request‑level output via `log.Printf` on `POST /move` and `POST /reset` (cell received, outcome, or error). Standard `net/http` server errors (port in use, etc.) are printed to stderr by `http.ListenAndServe`. No metrics instrumentation; the change is a local dev convenience, not a production service.

## Security and privacy considerations

- Loopback‑only default address (`127.0.0.1:8080`). The `-addr` flag allows a different address; documented as a developer convenience, not a production deployment.
- No credentials, tokens, or secrets are stored or transmitted.
- Input validation: `parseInput` rejects out‑of‑range and non‑numeric input; the server also validates the JSON payload structure before calling game functions.
- No `text/template` or unsafe HTML rendering that could introduce XSS — the HTML template has no user‑supplied text; board cells are either a space rune or `'X'`/`'O'`, rendered by JS class names.

## Performance considerations

Negligible. One local user, one game at a time, serialised by mutex. The server starts in milliseconds and serves sub‑kilobyte JSON payloads. No caching, pooling, or goroutine tuning is needed.

## Risk tier recheck

Run the risk-tiering skill again against what this plan actually describes, not what SPEC.md assumed before the technical approach was known.

Risk tier after planning: 1
Changed from Planning and Requirements: no

Rationale: The plan adds a loopback HTTP listener, a new entry‑point branch, and shared‑state serialisation in the same `main` package. A mistake in the web handler could silently alter game behaviour (accept a move on a won board, fail to advance turn, etc.) without touching identity, credentials, entitlements, or production infrastructure. This matches the Tier‑1 description: *"a mistake could silently change routing or behaviour without violating an entitlement"*. No Tier‑2 triggers are present — no auth, no claims, no admission policy, no credential handling. Tier 1 is correct.

## Rollback and migration

Trivially revertable: delete `web.go`, `web_test.go`, and the `os.Args` branch in `main()`, then re‑run `go build ./...` and `go test ./...` to confirm the terminal game remains intact. No database, no schema, no persistent state outside the single in‑memory board. The `ttt` binary is never touched.

## Dependencies and sequencing

No external dependencies. No other in‑flight work blocks on or conflicts with this change. The plan's own stories are sequential — each builds on the last — so they run in order within a single task.

## Quality gates

Every story must pass these before it counts as done. The builder runs them from the workspace root (`/workspace`). The reviewer and the Watts checks run them again independently.

- `bash /inputs/resources/team/scripts/build.sh`
- `go test ./...`
- `go vet ./...`
- `bash /inputs/resources/team/scripts/lint.sh`

## User stories

*The build loop works through these in order. Keep the heading format exactly (`### US-001: Title`). Each story must be small enough for one agent iteration, with checkable acceptance criteria, a `Covers:` line naming SPEC criteria, and a `Depends on:` line. Every SPEC criterion must be covered by some story. Put the riskiest or approach-proving story first.*

### US-001: HTTP server serves the web UI and static assets

Covers: AC-1
Depends on: none

**As a** developer
**I want** the application to start a loopback HTTP server and serve the game UI when passing a `web` subcommand or `-addr` flag
**So that** the game is reachable from a browser without a frontend build step or CDN

#### Acceptance Criteria
- [ ] When `go run . web` (or equivalent) is executed, the application starts an HTTP server on `127.0.0.1:8080` by default
- [ ] `GET /` returns an HTML page with status 200 that contains a 3×3 grid container and no embedded credentials
- [ ] The HTML asset is delivered without network requests to external hosts (check for absence of `<script src="http` or `<link href="http` tags)
- [ ] When the terminal game starts (no subcommand / flag), the HTTP server is not started and existing terminal behaviour is preserved
- [ ] The `-addr` flag accepts an alternative listen address, and this is documented
- [ ] All quality gates pass from `/workspace` root

### US-002: Web UI displays the board and handles valid moves

Covers: AC-2, AC-5
Depends on: US-001

**As a** player
**I want** to see an empty board with X to move first, and be able to click a cell to place my mark
**So that** I can play a tic-tac-toe game in the browser

#### Acceptance Criteria
- [ ] On initial load, the UI shows an empty 3×3 board and `"X to move"` (or equivalent) turn indicator
- [ ] Clicking an empty cell sends `POST /move` with `{"cell":N}`, the server places the current player's mark, and the UI updates to show the mark
- [ ] After a valid move the turn indicator switches to the other player (`"O to move"`)
- [ ] The server uses the existing `checkWin`, `isDraw`, `newBoard`, and `parseInput` functions — not duplicate JS game logic
- [ ] The terminal game and its existing tests continue to pass unchanged

### US-003: Web UI handles invalid moves and game-over states

Covers: AC-3, AC-5
Depends on: US-002

**As a** player
**I want** the UI to reject invalid moves and display the outcome when the game ends
**So that** the game correctly enforces the rules

#### Acceptance Criteria
- [ ] Clicking an occupied cell returns an error and does not change the board or the turn indicator
- [ ] Sending a cell number outside 1–9 or a malformed JSON body returns an error without changing state
- [ ] When a player completes a winning line, the UI displays which player won and stops accepting further moves (server rejects any `POST /move` with `over: true`)
- [ ] When the board fills with no winner, the UI displays "Draw" and stops accepting further moves
- [ ] The server calls `checkWin` and `isDraw` from the existing game — no reimplementation
- [ ] The terminal game's occupied-cell, invalid-input, win, and draw behaviour is unchanged

### US-004: Reset functionality and responsive UI

Covers: AC-4
Depends on: US-003

**As a** player
**I want** a reset button that clears the board and restarts the game, and a layout that works on narrow screens
**So that** I can play multiple rounds and use the UI on a phone

#### Acceptance Criteria
- [ ] A reset button is visible on the page at all times (during active play, after win, after draw)
- [ ] Clicking reset sends `POST /reset`, clears the board, removes the outcome message, and restores `"X to move"`
- [ ] After a win or draw, clicking reset re‑enables board interaction
- [ ] The UI is usable on viewports as narrow as 320 px (CSS media query or flexible layout)
- [ ] Buttons have readable labels (not unlabelled icons without text equivalents)

### US-005: Tests and documentation

Covers: AC-6, AC-7
Depends on: US-004

**As a** developer
**I want** automated tests covering the web handler and documentation explaining how to run both modes
**So that** regressions are caught and other developers can use the application

#### Acceptance Criteria
- [ ] `web_test.go` covers: valid moves advancing turn, occupied cells returning error without state change, malformed or out‑of‑range cell input, reset during play and after game‑over, moves rejected after win or draw
- [ ] All four quality gates (`build.sh`, `go test ./...`, `go vet ./...`, `lint.sh`) pass from the workspace root
- [ ] The existing `ttt` executable at workspace root is unchanged (verified by checksum or re‑build not overwriting it)
- [ ] `README` or equivalent documentation at `/workspace` explains how to run the terminal game (`go run .`) and the web game (`go run . web` or similar), and how to set a custom listen address
- [ ] No application source or test files are created under `/runtime/task`

## Open questions for the author agent

- Which subcommand/flag format to use? Decision: `-addr` flag. When `-addr` is present and non‑empty, `main` starts the web server on that address; otherwise it runs the terminal game. This is simpler than a subcommand and avoids `flag` interfering with terminal mode. Example: `go run . -addr 127.0.0.1:8080`.
- Embedded HTML: single file with inline CSS and JS or separate embedded files? Decision: a single `//go:embed index.html` file in the project root keeps things simple. The HTML is small enough to maintain in one file.

## Review

An agent drafts this document. The human approves or rejects it at the plan-review workflow gate with `wctl task decide <task> plan-review`, optionally using `--reject --feedback 'changes needed'`.