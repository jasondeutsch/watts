# SPEC: Tic-tac-toe web UI

## Status
Draft
Version: v1
Supersedes: none; this is a new task extending the existing application
Owner: project user; initial draft prepared by Codex from the user's request
Last updated: 2026-10-12

## Intent
Add a minimal browser interface to the existing Go tic-tac-toe game so two people can play locally by clicking cells. This is an incremental change to the current application, not a replacement project. Preserve its terminal game while making the same rules available through a web UI.

## Background and context
The project already contains main.go, main_test.go, and go.mod. It implements a numbered terminal board, alternating X/O turns, all eight winning lines, draws, invalid-input handling, and EOF behavior. Existing tests establish regression behavior. A macOS executable named ttt is also present; this task must not replace it with a container-built binary.

## Acceptance criteria
- AC-1: When the documented web-mode Go command starts, the application shall serve the game UI and its assets locally without a frontend build step or remote CDN dependencies. Default web access shall be loopback-only, with a documented way to select the listen address.
- AC-2: When a new game begins, the UI shall display an empty clickable 3-by-3 board and indicate that X moves first. When a valid cell is selected, the application shall place the current player's mark and advance the turn unless the game has ended.
- AC-3: When a move targets an occupied or invalid cell, the application shall reject it without changing the board or turn. When either player completes any winning line or the board fills without a winner, the UI shall show the correct outcome and prevent further moves until reset.
- AC-4: When the user selects reset, the application shall clear the board, remove the previous outcome, and restore X as the current player. The UI shall remain usable on narrow/mobile viewports and provide understandable button labels and visible turn/outcome feedback.
- AC-5: The web interface shall reuse the existing Go game-rule functions rather than introduce a separate independent JavaScript rules engine. The terminal game and its existing tests shall continue to work, including invalid moves retaining turns, wins/draws, and EOF behavior.
- AC-6: The change shall include meaningful tests of new HTTP/game behavior, including malformed or invalid moves, reset, and completed-game handling. The configured build, test, vet, lint, and story-completion gates shall pass without weakening checks or substituting the pinned verification tools.
- AC-7: Application source, tests, and web assets shall live in the project root and normal source directories. Task documents and evidence shall remain in the task directory. Verification build products/caches shall remain under private runtime storage; the existing ttt executable shall remain unchanged. Document how to run the terminal and web modes.

## Explicit non-goals
- No AI opponent, remote multiplayer, authentication, database, persistence across server restarts, deployment, or framework migration.
- No frontend framework or JavaScript package manager/build pipeline is needed.
- No changes to previous Watts task/eval records or runtime-owned metadata to force success.

## Assumptions
- Two humans play one local game, alternating at the browser; independent accounts or multi-game isolation are not required.
- A small standard-library Go HTTP server and local HTML/CSS/JavaScript are an appropriate implementation. Keep dependencies minimal.
- The workflow's specification and planning Agents will refine these notes and create the implementation plan; this draft is not approval.

## Constraints
- Preserve existing game behavior and prefer reusing concrete project functions over introducing a generic game framework.
- Do not create a new application or module under the task directory.
- Use the image's declared tools and quality gates. Do not install replacements or change team lint policy to suppress findings.
- Do not read, expose, or embed provider credentials in application files or served responses.

## Open questions
None currently identified for the stated local-game scope. If refinement finds a material ambiguity, record it for review rather than inventing additional features.

## Risk-relevant context
- Touches: local HTTP interface, browser assets, application entry point, game-state transitions, tests, and run instructions.
- Does not touch: authentication, production services, databases, deployment, external accounts, or historical task evidence.

## Risk tier
1 — a local HTTP entry point and incremental code changes; no production deployment.

## Review
The specification Agent refines this draft. The declared spec-review Procedure then requires a human decision or an explicitly delegated coordinator decision tied to the reviewed artifact hashes.
