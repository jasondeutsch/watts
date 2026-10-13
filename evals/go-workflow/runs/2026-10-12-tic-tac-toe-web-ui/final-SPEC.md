# SPEC: Tic-tac-toe web UI

## Status
Draft
Version: v2
Supersedes: none; this is the same task's draft refined at the spec step (v1 was the initial draft from the user's request)
Owner: project user; draft refined by the specification agent
Last updated: 2026-10-12

## Intent
Add a minimal browser interface to the existing Go tic-tac-toe game so two people can play locally by clicking cells. This is an incremental change to the current application, not a replacement project. Preserve the terminal game while making the same rules available through a web UI.

## Background and context
The project root (`/workspace`) already contains `go.mod` (module `tic-tac-toe`, Go 1.21), `main.go`, `main_test.go`, and an existing executable named `ttt`. Observed current behavior in `main.go`:

- `board` is a 3×3 rune grid; `newBoard` starts all cells empty.
- `playGame` runs the terminal loop over stdin/stdout with a numbered board, alternating X/O turns, and prompts after each move.
- `parseInput` converts a single digit 1–9 to a position (0–8) and rejects empty, multi-character, and out-of-range input.
- `checkWin` evaluates all eight winning lines (3 rows, 3 columns, 2 diagonals); `isDraw` detects a full board with no winner.
- `main` invokes the terminal game and exits 0 on win, draw, or EOF (abandoned game).

`main_test.go` establishes regression coverage for numbered rendering, all eight winning lines for both players, draw detection, input parsing boundaries, X/O wins, draws, EOF behavior, occupied-cell and invalid-input turn retention, and boards staying unchanged on rejected moves.

The team quality gates under `/inputs/resources/team/scripts/` build to private runtime storage (`build.sh` writes to `/runtime/build`), run `golangci-lint` with the pinned team config (`lint.sh`, config at `/inputs/resources/team/.golangci.yml`), and validate the spec/plan/story/verdict artifacts (`check.mjs`). The `ttt` executable must remain untouched; this task must not replace it with a container-built binary.

This task adds a loopback HTTP server and browser assets in the same module so the same game rules are reachable from a browser without changing the terminal experience.

## Acceptance criteria
- AC-1: When the documented web-mode Go command starts, the application shall serve the game UI and its assets locally without a frontend build step or remote CDN dependencies. Default web access shall be loopback-only, with a documented way to select the listen address.
- AC-2: When a new game begins, the web UI shall display an empty clickable 3-by-3 board and show that X moves first. When the user selects a valid empty cell, the application shall place the current player's mark and, unless the game has ended, advance the turn to the other player.
- AC-3: If the user selects an occupied or invalid cell, then the application shall reject the move without changing the board or the current player's turn. When either player completes a winning line or the board fills without a winner, the UI shall display the correct outcome and reject further moves until the user selects reset.
- AC-4: When the user selects reset, the application shall clear the board, remove the previous outcome, and restore X as the current player. The UI shall remain usable on narrow/mobile viewports and shall provide readable button labels and visible turn and outcome feedback.
- AC-5: The web interface shall reuse the existing Go game-rule functions rather than introduce a separate independent JavaScript rules engine. The terminal game and its existing tests shall continue to work, including invalid moves retaining turns, wins/draws, and EOF behavior.
- AC-6: The change shall include tests of the new HTTP and game behavior covering valid moves and turn advancement, occupied cells, malformed or out-of-range cells, bad request payloads, reset, and completed-game handling, including moves attempted after a win or draw. The configured build, test, vet, lint, and story-completion gates shall pass without weakening checks or substituting the pinned verification tools.
- AC-7: Application source, tests, and web assets shall live in the project root and normal source directories. Task documents and evidence shall remain in the task directory. Verification build products/caches shall remain under private runtime storage; the existing `ttt` executable shall remain unchanged. The change shall document how to run the terminal and web modes.

## Explicit non-goals
- No AI opponent, remote multiplayer, authentication, database, persistence across server restarts, deployment, or framework migration.
- No frontend framework or JavaScript package manager/build pipeline is needed.
- No changes to previous Watts task/eval records or runtime-owned metadata to force success.
- No new third-party dependencies; the implementation should need only the Go standard library.

## Assumptions
- Two humans play one local game, alternating at the browser; independent accounts, multi-game isolation, and simultaneous games in multiple tabs are not required.
- A small standard-library Go HTTP server and local HTML/CSS/JavaScript are an appropriate implementation. Keep dependencies minimal.
- The UI targets a modern browser with standard ES6+ and CSS layout support; no legacy-browser support is required.
- The authoritative game state lives in the Go HTTP server as one shared in-memory board. A page refresh or second tab retrieves and resumes the same current game. Concurrent requests are serialized so the board does not change between evaluation and placement. Resetting the game clears the board and restores X to move.

## Constraints
- Preserve existing game behavior and prefer reusing concrete project functions over introducing a generic game framework.
- Do not create a new application or module under the task directory.
- Use the image's declared tools and quality gates. Do not install replacements or change team lint policy to suppress findings.
- Do not read, expose, or embed provider credentials in application files or served responses.

## Open questions
- OQ-1: (resolved) Where should the authoritative game state live, and what should happen after a page refresh or in a second tab?
  Decision: The Go HTTP server holds a single shared in-memory game. A page refresh or second tab retrieves and resumes the current board. Concurrent requests are serialized so the board does not change between evaluation and placement. Reset clears the board and restores X to move. See the Assumptions section for the recorded resolution.

  Other options considered: browser-held state (page refresh starts a new game). The server-held approach was chosen because it keeps the board authoritative on the backend where the existing game-rule functions live and avoids a multi-tab isolation contract that this task does not need.

- No other material ambiguities were identified for the stated local-game scope. If review surfaces further ambiguity, it will be recorded here rather than turned into a requirement.

## Risk-relevant context
- Touches: local HTTP listener and request handling, browser assets served from the module, application entry point for web mode, game-state transitions shared with the terminal mode, new HTTP tests, and run documentation.
- Does not touch: authentication, credential handling, production services, databases, deployment, external accounts, historical task evidence, or the existing `ttt` binary.
- Requests may arrive concurrently while one game state is shared; the web handler must be race-free and must serialize state changes against the terminal game's rule functions.

## Risk tier
1

Rationale: the change adds a network-visible entry point and incremental game-state/logic changes on a developer machine; a mistake could silently change behavior but cannot touch identities, credentials, or entitlements, and there is no production deployment.

## Review

An agent drafts this document. The declared spec-review Procedure then requires a human decision — or an explicitly delegated coordinator decision — tied to the reviewed artifact hashes. This v2 draft refines v1 against project evidence (`main.go`, `main_test.go`, `go.mod`, and the team gate scripts); no prior rejection feedback was available in this environment to incorporate. On rejection, this draft is revised in place using the feedback while preserving the original intent. A signature in this document does not approve the workflow.