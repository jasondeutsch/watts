# PLAN: <name>

Task documents: tasks/<date>-<slug>/
Implementation root: the project directory containing watts.json

Application source, tests, dependencies, and build files belong in the project root and its normal source directories. The task directory stores workflow documents and evidence only. Do not create a separate application or module under the task directory.
Spec ref: SPEC.md v<N>
Risk tier at Planning: <0 / 1 / 2, from SPEC.md>

*The planning agent drafts this file. The plan-review workflow gate records human approval before implementation starts.*

## Architecture and approach

What's the actual technical approach. Which existing components this extends versus touches versus leaves alone. Specific enough that two engineers reading this would build the same thing.

-

## Data model and interface changes

Concrete types, schemas, API contracts, or interfaces that change. Write "none" explicitly if purely internal, don't leave it blank.

-

## Systems touched

Cross-checked against the risk-tiering skill, not from memory.

- Touches:
- Does not touch:

## Technical decisions and alternatives considered

What got chosen, what got rejected, and why. Not allowed to collapse silently for anything above Tier 0.

-

## Testing strategy

What gets covered by unit tests, what by integration tests, what's explicitly left untested and why that's acceptable here.

-

## Observability

What gets logged, measured, or alerted once this ships. How will anyone know it's working, how will anyone know it broke.

-

## Security and privacy considerations

Data handled, auth or access implications, anything that needs review beyond what the risk tier alone captures.

-

## Performance considerations

Expected load, any latency or memory budget, what happens under failure or at the edge of that budget.

-

## Risk tier recheck

Run the risk-tiering skill again against what this plan actually describes, not what SPEC.md assumed before the technical approach was known.

Risk tier after planning: <0 / 1 / 2>
Changed from Planning and Requirements: <yes, no>
If yes, why:

## Rollback and migration

How this gets undone if it's wrong. Required above Tier 0.

-

## Dependencies and sequencing

What this blocks on, what blocks on this, whether the order conflicts with other work already in flight.

-

## Quality gates

Commands every story must pass before it counts as done, one backticked command per bullet. Real commands for this repository, run from the repository root, not placeholders. The builder runs them, and the reviewer and the Watts checks run them again independently.

- `go build ./...`
- `go vet ./...`
- `go test ./...`

## User stories

*The build loop works through these in order. Keep the heading format exactly (`### US-001: Title`). Each story must be small enough for one agent iteration, with checkable acceptance criteria, a `Covers:` line naming SPEC criteria, and a `Depends on:` line. Every SPEC criterion must be covered by some story. Put the riskiest or approach-proving story first.*

### US-001: <short title>

Covers: AC-1
Depends on: none

**As a** <user type>
**I want** <capability>
**So that** <benefit>

#### Acceptance Criteria
- [ ] <checkable criterion>
- [ ] <checkable criterion>

### US-002: <short title>

Covers: AC-2
Depends on: US-001

**As a** <user type>
**I want** <capability>
**So that** <benefit>

#### Acceptance Criteria
- [ ] <checkable criterion>

## Open questions for the author agent

Anything ambiguous enough to come back as a question rather than get resolved by the agent's own judgment. Answer these here, before Development starts, don't let them surface for the first time as a rejection.

-

## Review

An agent drafts this document. The human approves or rejects it at the plan-review workflow gate with `watts task decide <task> plan-review`, optionally using `--reject --feedback 'changes needed'`.
