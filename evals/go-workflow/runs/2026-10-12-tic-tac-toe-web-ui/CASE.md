# Case: Add a web UI to an existing tic-tac-toe application

- Case ID/version: `tic-tac-toe-web-ui` / `0.1.0`
- Eval/rubric: `go-workflow` / `0.1.0`
- Configuration: `coder-golang-low-tools-gates`
- Project: `/Users/j/watts_project/test`; this is an incremental change, not a fresh project.
- User request: “create another eval. utilize the same test/ repo. this time lets add a web ui for our tic-tac-toe game”

## Starting conditions

The project contains a terminal tic-tac-toe game with existing game-rule/CLI tests, a standard-library-only Go module, and a macOS executable named `ttt`. Baseline sources, settings, and hashes are retained alongside this record. Existing task history remains unchanged. The consumer package's base path is updated from the renamed `go_generalist` directory to `coder_golang` before task creation; this is setup, not an application repair.

## Acceptance criteria

1. Serve an interactive browser UI from a documented Go command using local assets and no frontend build step or external CDN.
2. Display a usable 3×3 board, current turn, and clear game outcome; support local alternating X/O play with X first.
3. Reject occupied/out-of-range moves without advancing the turn; detect all winning lines and draws, and prevent further moves after completion.
4. Reset to an empty board with X to move.
5. Reuse the existing Go game rules and preserve terminal behavior and the existing regression tests. Keep the application minimal and dependency-light.
6. Provide meaningful tests for new HTTP/game behavior, including invalid requests and completed-game handling; build, tests, vet, and configured lint must pass.
7. Keep source, tests, and web assets in normal project locations. Compilation output stays private; task documents/evidence stay under the task directory. Preserve the existing `ttt` artifact rather than silently replacing it with a container binary.

## Scope and verification

A local Go HTTP server with ordinary HTML/CSS/JavaScript is the assumed minimal implementation. No authentication, AI opponent, remote multiplayer, database, deployment, or frontend framework is needed. Browser behavior, responsive layout, turn/win/draw/reset handling, CLI regression, tests/vet/lint, and retained gate evidence will be independently checked after publication. Product styling is assessed qualitatively; no human rubric score is fabricated.

## Intervention policy

Use the configured six-stage workflow and its human gates. Preserve declared provider/model and source policy. Coordinator may prepare initial SPEC.md, inspect evidence, and perform authorized operational setup/recovery; it does not hand-edit implementation or fabricate decisions. Delegated approval from the previous case is not silently extended to this case; obtain the applicable decision/delegation before advancing human gates. Any rejection, retry, dependency repair, or policy change is recorded. An overall 45-minute observation cutoff excludes explicit human approval waits; if reached, record the pending state rather than discard the execution. Unknown costs and missing usage remain unknown.
