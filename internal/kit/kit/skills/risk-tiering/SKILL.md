---
name: risk-tiering
description: Classifies a task into Tier 0, 1, or 2 based on what it touches in this team's systems. Use during Planning and Requirements to assign the initial tier, and again during Design and during Testing and Verification to recheck it.
---

# Risk Tiering

Risk tiers guide review depth. Workflow definitions declare approval policy explicitly; the default SDLC has specification and plan review gates at every tier.

Three tiers. Assign based on what the change actually touches, not how large or small the diff looks.

## Tier 0, low risk

Dependency bumps. Documentation changes. Non-functional refactors and formatting. UI-only changes to a dashboard component itself, not the data it reads.

## Tier 1, checkpoint logged before merge

Governance field value changes in a model or service registry. Cache or fallback chain tuning in a routing layer. Non-security pipeline stages, response formatting, post-processing. Protocol adapters that don't touch authentication.

## Tier 2, human approval gate before execution starts

Anything touching JWT claim validation or issuance, anything resembling model or resource access-control claims, budget attribution headers. Admission policy logic itself, not per-org configuration data, the policy. Entitlement primitive logic and hard limit enforcement. Planning-phase logic that produces an execution plan gating upstream calls. Identity or provisioning logic. Anything resembling credential handling.

## The general rule for anything not listed above

- If a mistake could let one identity spend or access what another identity is entitled to: Tier 2.
- If a mistake could silently change routing or behavior without violating an entitlement: Tier 1.
- If a mistake is cosmetic, immediately visible, and trivially reverted: Tier 0.

## When to recheck

Assign the initial tier during Planning and Requirements. Recheck it during Design, once the actual shape of the solution is known. Recheck it again during Testing and Verification if anything was discovered mid-task that changes the picture. A risk tier can move up mid-task. It never moves down silently.
