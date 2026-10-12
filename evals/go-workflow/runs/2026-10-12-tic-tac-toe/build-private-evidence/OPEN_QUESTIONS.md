# Open Questions

Logged at implementation time. No blocking questions remain: the spec
(SPEC.md v1) and plan (PLAN.md) resolved all decisions, and the plan-review
gate was approved (outcome: approved, approved_by: j, attempt 1).

## P0

- [x] None. No P0 question must block completion.

## P1

- [x] None. No P1 question must block completion.

## Resolved during implementation

- Cell display format (ASCII grid with `|` and `-`) and exact error-message
  wording were left to the implementation by the spec and are now fixed in
  `main.go` (e.g., `That cell is already occupied.`, `invalid input: %q is
  not a number between 1 and 9`, `Game abandoned.`).
- Module path `tic-tac-toe` with `go 1.21`, single package `main`,
  `bufio.Scanner` for input, `[9]rune` board — as decided in PLAN.md and
  implemented in `/workspace`.