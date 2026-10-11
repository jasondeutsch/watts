# Case: tic-tac-toe — minimal terminal game

- Case version: `0.1.0`
- Difficulty: small

## Request

Create a minimal terminal tic-tac-toe game in Go for two local human players. Use only the Go standard library. Players X and O alternate selecting cells 1–9. Print the board and whose turn it is. Reject occupied cells, out-of-range moves, and nonnumeric input without changing turns. Detect all rows, columns, diagonals, and draws; announce the result and stop. Exit cleanly on EOF. No AI opponent, web server, persistence, configuration framework, or external application dependencies. Keep the implementation small and readable, preferably one main.go plus focused tests. Create go.mod and application files in the project root, not the task directory. Verify wins, draws, invalid moves, and EOF with meaningful tests. Run go test ./... and the configured lint gate.

## Starting project and boundaries

`test` initially contains only a Makefile launching Pi. Watts setup and package configuration are infrastructure, not implementation. No starting Go module or application code. Preserve the Makefile. Application files belong in the project root; task artifacts remain in tasks/.

## Independent acceptance checks

- Two-player numbered-cell terminal interaction and alternating valid moves.
- All eight win lines for either player; no premature win.
- Full-board draw terminates with a draw message.
- Occupied, out-of-range, and nonnumeric input rejected without losing a turn.
- EOF exits cleanly.
- Standard-library-only Go module; tests and lint pass.
- Small readable implementation without unrelated functionality.

## Intervention policy

Use all default Go generalist stages and human spec/plan gates. Record approvals, rejection, retries, infrastructure setup, and manual changes. Do not hand-write or repair application code. Stop and record unresolved provider/runtime failures rather than substitute a direct implementation. Human review scores remain unscored until human review.
