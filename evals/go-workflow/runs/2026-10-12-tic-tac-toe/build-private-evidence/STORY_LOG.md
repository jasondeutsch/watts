# Story Log — Tic-tac-toe

Records stories from PLAN.md as they are implemented. A story is marked DONE only
after all quality gates pass (`go build ./...`, `go vet ./...`, `go test ./...`,
`bash /inputs/resources/team/scripts/lint.sh`).

DONE US-001 - Project scaffolding and board rendering
- `go.mod` (module `tic-tac-toe`, `go 1.21`) at `/workspace`.
- `main.go` renders a numbered 3x3 ASCII board and prompts `Player X, enter a cell number (1-9): `.
- Gates: build, vet, test, lint all pass.

DONE US-002 - Turn management and move input
- Valid move places mark, re-displays board, toggles X/O.
- Occupied cell: error message, board re-display, same player re-prompted, board unchanged.
- Out-of-range (`0`, `10`, `11`, `-1`, ...) and non-numeric (`abc`, `5.0`, empty) inputs:
  error message, same player re-prompted, board unchanged.
- Implementation: `parseInput` + `playGame` loop with tests
  (`TestPlayGameOccupiedCellKeepsTurn`, `TestPlayGameInvalidInputKeepsTurn`,
  `TestPlayGameBoardUnchangedOnError`).

DONE US-003 - Win detection
- `checkWin` detects all eight winning lines for X and O.
- Win announces `Player X wins!` / `Player O wins!` and exits with code 0.
- Tests: `TestCheckWin` (all 8 lines x both players), `TestCheckWinNoWin`,
  `TestPlayGameXWinsDiagonal`, `TestPlayGameOWinsRow`.

DONE US-004 - Draw detection and EOF handling
- Full board with no winner prints `The game is a draw.` and exits 0.
- EOF at any prompt prints `Game abandoned.` and exits 0.
- Draw test uses a full board with no winning line to rule out false win detection
  (`TestPlayGameDraw`); EOF covered by `TestPlayGameEOFAbandoned` and
  `TestPlayGameEOFAfterMoves`.

DONE US-005 - Test suite and lint compliance
- `go test ./...` passes (14 tests, coverage 97.8%).
- `bash /inputs/resources/team/scripts/lint.sh` reports 0 issues.
- Coverage: rendering (`TestRender*`), win detection, draw detection, input parsing
  (`TestParseInput`), full-game simulations (`TestPlayGame*`).