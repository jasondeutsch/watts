# PLAN: Tic-tac-toe web UI

Task documents: /runtime/task
Implementation root: /workspace

Application source, tests, dependencies, and build files belong in the project root and its normal source directories. The task directory stores workflow documents and evidence only. Do not create a separate application or module under the task directory.
Spec ref: SPEC.md v2
Risk tier at Planning: 1

*The planning agent drafts this file. The plan-review workflow gate records human approval before implementation starts.*

## Architecture and approach

Extend the existing single-package `main` Go application in `/workspace` (module `tic-tac-toe`, Go 1.21) with an optional HTTP server, preserving the existing terminal game as the default mode.

**Web mode activation**: A `-addr` flag on the `ttt` binary. When absent, the program runs the existing terminal game (backward compatible default). When present (e.g., `-addr=localhost:8080`), the program starts an HTTP server on that address. A convenience shell wrapper or documented invocation without the flag falls back to the terminal mode. The flag name `-addr` is chosen for consistency with Go's `net/http` conventions; the default value is empty string, meaning terminal mode.

**Server architecture**: A single `main`-package file `server.go` (or `http.go`) adds:
- `http.Handler`-based routing with no third-party router (standard library `http.ServeMux`).
- A `GET /` handler that serves the single-page HTML UI.
- A `GET /state` handler that returns the current board, whose-turn, and game-over status as JSON. This is the read-only state retrieval endpoint that enables page refresh and second-tab hydration without resetting the board.
- A `POST /move` handler that accepts a JSON body `{"cell": N}` (N = 0–8), validates via existing `parseInput`, applies the move via existing `checkWin`/`isDraw`/board logic, and returns the updated state as JSON.
- A `POST /reset` handler that clears the board and restores X to move.
- A shared mutex protecting the in-memory `board` and current-player state so concurrent requests do not race.

**State retrieval (AC-1, AC-6, Assumptions)**: `GET /state` returns the complete game state as JSON:
```json
{
  "board": [["X"," ","O"],[" ","X"," "],[" "," "," "]],
  "currentPlayer": "O",
  "winner": "",
  "gameOver": false
}
```
On a page load or refresh, the browser JS calls `GET /state` to hydrate its display from the server-held state, so refresh or a second tab sees the current game (not a new board). A failed `GET /state` (network error, non-200 response) leaves the UI in a loading/error state without changing the board.

**Frontend assets**: A single `index.html` file embedded via `//go:embed` in the Go binary, containing inline `<style>` and `<script>` blocks. No separate CSS/JS files to serve. The JavaScript uses `fetch()` for AJAX calls, DOM APIs for rendering, and no third-party libraries.

**Embedded HTML/JS strategy**: Use Go 1.16+ `//go:embed` to embed `index.html` directly. The HTML contains:
- A responsive grid layout using CSS Grid or Flexbox.
- A clickable 3×3 board (buttons or clickable divs).
- A status display showing whose turn it is or the outcome.
- A reset button.
- JavaScript that calls `GET /state` on page load, `POST /move` on cell click, and `POST /reset` on reset.

**Existing code**: `main.go`, `main_test.go`, `go.mod` remain untouched structurally. The `main` function gains a conditional branch when `-addr` is set. The existing `board`, `newBoard`, `checkWin`, `isDraw`, `parseInput` functions are reused directly — HTTP handlers call the same functions the terminal game uses.

**Executable**: The existing `ttt` binary in `/workspace` remains untouched. The build script writes the new binary to `/runtime/build/`; it is the same source tree rebuilt (the `go.mod` module name is `tic-tac-toe`). No third-party dependencies are added.

## Data model and interface changes

**New exported/internal types** (in `main` package):
- `gameState` struct: `{ board board; currentPlayer rune; winner rune; gameOver bool }` (unexported, server-internal).
- `stateResponse` struct: JSON-serializable shape returning `board` as `[][]string`, `currentPlayer` as string, `winner` as string, `gameOver` as bool.
- `moveRequest` struct: JSON body with `Cell int` field.
- `moveResponse` struct: same shape as `stateResponse`, returned after every move and reset.
- A `sync.Mutex` held on the game state for concurrent safety.

**New `var`**: `gameState` instance (package-level or closure-captured), initialized by the server setup.

**API contract**:

| Endpoint   | Method | Body                        | Response                  |
|------------|--------|-----------------------------|---------------------------|
| `/`        | GET    | —                           | `text/html` (embedded page) |
| `/state`   | GET    | —                           | `200 application/json` (current game state) |
| `/move`    | POST   | `{"cell": <int 0-8>}`       | `200` updated state, `400` bad request / occupied / game over |
| `/reset`   | POST   | —                           | `200` initial state       |

## Systems touched

Cross-checked against the risk-tiering skill, not from memory.

- Touches:
  - `/workspace/main.go` — minor `main` branching for web mode (condition on `-addr` flag).
  - New file `/workspace/server.go` — HTTP server, handlers, embedded assets.
  - New file `/workspace/server_test.go` — HTTP handler tests.
  - New file `/workspace/index.html` — embedded web UI.
  - `go.mod` — Go version and module name remain unchanged.
  - Project build via `go build ./...` and `go vet ./...`.
  - Team scripts `/inputs/resources/team/scripts/build.sh`, `lint.sh`.
  - The `ttt` binary in `/workspace` — must remain unchanged (not replaced).
- Does not touch:
  - Authentication, credentials, identity, or JWT.
  - Database, external services, or cloud resources.
  - Existing `main_test.go` terminal-game tests (they remain and must still pass).
  - Team lint configuration `/inputs/resources/team/.golangci.yml`.
  - Any Watts runtime metadata or task evidence outside the task directory.
  - Any other repositories, modules, or packages outside `/workspace`.

## Technical decisions and alternatives considered

### Web mode activation: `-addr` flag (chosen) vs subcommand vs env var
- **Chosen**: A `-addr` flag on the existing binary. Default (no flag) = terminal mode. When `-addr=localhost:8080` is provided, the HTTP server starts. This is the simplest Go idiom, requires no `os.Args` subcommand parsing, and is consistent with standard `net/http` usage.
- **Rejected**: Subcommand (`ttt web`, `ttt serve`) — adds more parsing complexity, deviates from the single-binary terminal default.
- **Rejected**: Environment variable only (`WEB_ADDR`) — less discoverable, not consistent with Go flag conventions.
- **Rejected**: Always-on web mode — violates the requirement to preserve the terminal default.

### Server-held game state (chosen) vs client-only state
- **Chosen**: Authoritative game state lives in a single in-memory `gameState` struct behind a `sync.Mutex` in the Go HTTP server. The browser is a stateless view. A page refresh or second tab retrieves state via `GET /state` and renders it. This is consistent with the resolved OQ-1 decision in SPEC.md.
- **Rejected**: Client-only state — a page refresh would lose the game, violating the server-held state requirement.

### Embedded HTML vs separate static directory
- **Chosen**: `//go:embed` bundles `index.html` into the Go binary. No file-serving directory, no CDN, no build step. The server registers a single `GET /` handler that serves the embedded bytes with `Content-Type: text/html`.
- **Rejected**: Separate `static/` directory served via `http.FileServer` — requires the binary to know its own location relative to assets at runtime, adds complexity, and is less portable.

### No third-party dependencies
- The implementation uses only Go standard library: `net/http`, `encoding/json`, `embed`, `sync`, `flag`, `fmt`, `strings`. This satisfies the no-new-dependencies constraint.

## Testing strategy

**Unit tests** (in `server_test.go`):
- `TestGetState_Initial` — `GET /state` returns empty board, X to move, no winner, not game over.
- `TestGetState_AfterMoves` — Make a valid move via `POST /move`, then `GET /state` returns the updated board with correct current player.
- `TestGetState_RefreshHydration` — Simulate a second request (as from a refresh or second tab) and verify the state is identical (not reset).
- `TestGetState_AfterFailedRequest` — Send a malformed `POST /move` (rejected), then `GET /state` returns the original board unchanged.
- `TestMove_ValidCell` — `POST /move {"cell": 0}` returns updated state, board shows X at position 0, current player becomes O.
- `TestMove_OccupiedCell` — `POST /move` to same cell twice; second returns 400 and state unchanged.
- `TestMove_OutOfRange` — `POST /move {"cell": 99}` returns 400.
- `TestMove_BadPayload` — `POST /move` with non-JSON body, missing `cell` field, string value, etc. return 400.
- `TestMove_AfterWin` — Play to a win, then attempt another move via `POST /move`; returns 400 with game-over status.
- `TestMove_AfterDraw` — Play to a draw, then attempt another move; returns 400 with game-over status.
- `TestReset` — After moves, `POST /reset` returns the initial state (empty board, X to move, no winner).
- `TestReset_AfterTwice` — `POST /reset` twice; second reset also returns initial state.
- `TestGetState_NoResetOnRefresh` — Verify multiple `GET /state` calls in sequence return the same state without mutating it.

**Existing tests**: All tests in `main_test.go` must continue to pass (no behavioral change to terminal game).

**Integration**: Test server startup and shutdown, though these are covered implicitly by HTTP handler tests that start a `httptest.NewServer`.

## Observability

- The HTTP server logs each request with method, path, and status code via a small `loggingMiddleware` or `http.Handler` wrapper writing to `os.Stderr`.
- On startup (web mode), a single log line prints the listen address.
- Terminal mode is unchanged — no new logging.
- No metrics system, no structured logging dependency (standard library `log` package suffices).

## Security and privacy considerations

- Loopback-only default (`localhost:8080`). Users must explicitly override `-addr` to expose the server to other hosts.
- No authentication, no session cookies, no user data stored.
- Request payloads are validated for shape and bounds before touching game state.
- The `sync.Mutex` prevents races, not a security boundary.
- Credentials are never read, exposed, or embedded (no credential handling at all).

## Performance considerations

- A single, in-memory game board for two local players; concurrency is limited to maybe a handful of requests at once.
- `sync.Mutex` serializes all state mutations; uncontended cost is negligible.
- No database, no I/O beyond HTTP request/response; latency is sub-millisecond.
- Edge case: a client that spams requests will be serialized and served sequentially; no resource exhaustion beyond standard Go HTTP limits.

## Risk tier recheck

Run the risk-tiering skill again against what this plan actually describes, not what SPEC.md assumed before the technical approach was known.

Risk tier after planning: 1
Changed from Planning and Requirements: no

**Rationale**: The plan adds a loopback HTTP server and embedded static assets in the same Go module. The HTTP listener is network-visible only on `localhost` by default, with an optional address override. It handles only game-state requests (state, move, reset) and serves a static HTML page. No authentication, credential handling, database, external service, identity, or entitlement logic is involved. A mistake could silently change game behavior (Tier 1 concern) but cannot let one identity access another's resources, touch credentials, or affect production systems. The risk tier remains 1.

## Rollback and migration

- This change is additive to the existing codebase. To roll back:
  1. Revert `main.go` changes (remove `-addr` flag branching) and delete `server.go`, `server_test.go`, `index.html`.
  2. Restore the original `ttt` binary (it remains on disk, unchanged by this task).
  3. Run existing tests to confirm terminal-only behavior.
- No data migration is needed (no persistent storage).
- No coordinated rollback with other services is required.

## Dependencies and sequencing

- Blocks on: nothing.
- Blocked by: nothing.
- No other work in flight for this repository.
- The single module (`tic-tac-toe`) means all code compiles and tests together; no sequencing risk.

## Quality gates

Commands every story must pass before it counts as done, one backticked command per bullet. Real commands for this repository, run from the repository root, not placeholders. The builder runs them, and the reviewer and the Watts checks run them again independently.

- `bash /inputs/resources/team/scripts/build.sh`
- `go test ./...`
- `go vet ./...`
- `bash /inputs/resources/team/scripts/lint.sh`

## User stories

*The build loop works through these in order. Keep the heading format exactly (`### US-001: Title`). Each story must be small enough for one agent iteration, with checkable acceptance criteria, a `Covers:` line naming SPEC criteria, and a `Depends on:` line. Every SPEC criterion must be covered by some story. Put the riskiest or approach-proving story first.*

### US-001: Web mode startup with state retrieval

Covers: AC-1, AC-6
Depends on: none

**As a** player
**I want** to start the application in web mode using `-addr=localhost:8080` and for the server to serve the UI and expose a read-only state endpoint
**So that** I can load the UI from a browser and hydrate the display from the server-held board without resetting it

#### Acceptance Criteria
- [ ] When the binary is started with `-addr=localhost:8080`, `GET /` returns `200 OK` with `Content-Type: text/html` (the embedded page). No frontend build step or CDN is used.
- [ ] When the binary is started without `-addr`, the terminal game runs as before (backward compatible).
- [ ] `GET /state` returns `200 OK` with JSON containing an empty 3×3 board, `currentPlayer: "X"`, `winner: ""`, `gameOver: false`.
- [ ] After a `POST /move` places a mark, `GET /state` returns the updated board with the correct current player. Multiple sequential `GET /state` calls return identical state (no mutation).
- [ ] After a failed request (e.g., malformed payload), `GET /state` returns the unchanged prior board.
- [ ] The default listen address is localhost (loopback-only); the `-addr` flag allows override.
- [ ] Quality gates pass (`bash /inputs/resources/team/scripts/build.sh`, `go test ./...`, `go vet ./...`, `bash /inputs/resources/team/scripts/lint.sh`).

### US-002: Web game — valid moves, turn advancement, and board display

Covers: AC-2, AC-5
Depends on: US-001

**As a** player
**I want** to click an empty cell to place my mark and see the board update and the turn advance
**So that** I can play a standard game of tic-tac-toe in the browser

#### Acceptance Criteria
- [ ] When the game starts, the UI displays an empty 3×3 board and shows "Player X's turn".
- [ ] Clicking an empty cell places the current player's mark (X or O) on the board.
- [ ] After a valid move, the display updates to show the next player's turn.
- [ ] The move is validated server-side using the same `checkWin` / `isDraw` / `parseInput` functions that the terminal game uses (no separate JS rules engine).
- [ ] Existing terminal game tests (`main_test.go`) still pass.
- [ ] Quality gates pass.

### US-003: Rejected moves — occupied cells, invalid input, game-over protection

Covers: AC-3, AC-5, AC-6
Depends on: US-002

**As a** player
**I want** invalid or illegal moves to be rejected without changing the board or turn
**So that** only valid moves affect game state

#### Acceptance Criteria
- [ ] Clicking an occupied cell returns an error (400); the board and turn remain unchanged.
- [ ] `POST /move` with an out-of-range cell number (e.g., 42) returns 400; the board and turn remain unchanged.
- [ ] `POST /move` with a malformed body (e.g., non-JSON, missing `cell` field, string value) returns 400; the board and turn remain unchanged.
- [ ] After a win (three-in-a-row), any further `POST /move` returns 400 with `gameOver: true`.
- [ ] After a draw (board full, no winner), any further `POST /move` returns 400 with `gameOver: true`.
- [ ] The UI reflects the error (the cell remains unmarked, turn unchanged) and may display an informational message.
- [ ] Quality gates pass.

### US-004: Win and draw outcomes displayed, then blocking further moves

Covers: AC-3, AC-5
Depends on: US-003

**As a** player
**I want** the UI to announce the winner or a draw when the game ends and to prevent further moves
**So that** the outcome is clearly communicated

#### Acceptance Criteria
- [ ] When either player completes a winning line, the UI displays the winner (e.g., "Player X wins!") and indicates the game is over.
- [ ] When the board fills without a winner, the UI displays "The game is a draw."
- [ ] After a win or draw, clicking any cell has no effect (server returns 400 for cell clicks; UI does not change the board).
- [ ] The reset button is still functional.
- [ ] Quality gates pass.

### US-005: Reset clears board and restores X to move

Covers: AC-4
Depends on: US-004

**As a** player
**I want** to click a reset button to start a new game
**So that** I can play another round without reloading the page

#### Acceptance Criteria
- [ ] `POST /reset` clears the board, restores X as the current player, and removes any outcome message.
- [ ] After reset, the UI displays an empty board and "Player X's turn".
- [ ] The reset button is visible and labeled clearly (e.g., "Reset Game").
- [ ] Quality gates pass.

### US-006: Responsive layout, visible feedback, and run documentation

Covers: AC-4, AC-7
Depends on: US-005

**As a** player
**I want** the UI to be usable on narrow or mobile viewports and to provide clear visual feedback
**So that** I can play comfortably on any device

#### Acceptance Criteria
- [ ] The board and controls render correctly on a viewport as narrow as 360px wide, without horizontal scrolling or overlapping elements.
- [ ] All buttons and interactive elements have accessible labels.
- [ ] Turn indicator and outcome display are visually distinct and readable.
- [ ] The project includes or updates a doc comment or README explaining how to run the terminal game (`go run .` or the `ttt` binary) and the web mode (`go run . -addr=localhost:8080` or `ttt -addr=localhost:8080`).
- [ ] The existing `ttt` binary in `/workspace` is not replaced or modified by the build.
- [ ] Quality gates pass.

## Open questions for the author agent

None. SPEC.md's resolved OQ-1 (server-held state with GET /state hydration) is addressed by the state retrieval endpoint and the stories above. No remaining ambiguities.

## Review

An agent drafts this document. The human approves or rejects it at the plan-review workflow gate with `wctl task decide <task> plan-review`, optionally using `--reject --feedback 'changes needed'`.