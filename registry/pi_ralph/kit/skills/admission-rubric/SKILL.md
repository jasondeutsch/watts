---
name: admission-rubric
description: Applies a four-step admission and verification check to a finished task's diff before it is allowed to count as done. Use only when reviewing work produced by a different agent instance, never to review your own output, during Testing and Verification.
---

# Admission Rubric

Apply these four steps in this exact order. Stop and reject at the first failure, do not continue to later steps once one has failed, and do not pass something forward with a caveat instead of a clear pass or reject.

Review the task documents in the private task directory named in your instructions. Inspect BUILD_CHANGES.json when supplied, the application in /workspace, SPEC.md, PLAN.md, and STORY_LOG.md. Check only this task's scope. Write only the declared review artifacts; the application workspace is read-only.

Deterministic checks are evidence, not a substitute for judgment. Approval policy is enforced by the workflow; do not invent approval files or signatures. If required evidence is absent, state that explicitly rather than assuming it exists.

## Step 1: Spec fidelity

Read SPEC.md in full, including Explicit Non-Goals, and read every story in PLAN.md. Check the diff line by line against the acceptance criteria, the stories, and the Explicit Non-Goals.

- Is every acceptance criterion (AC-1, AC-2, and so on) actually satisfied by the diff, and exercised by a test where one is called for? Name each one in your reasoning.
- Does the diff avoid everything listed under Explicit Non-Goals?
- If the author resolved an ambiguity the spec didn't cover, was that resolution reasonable, or did it invent scope nobody asked for?
- Does each story's recorded work match its story, or was work moved, merged, or skipped?

Reject here if any of the above fails. This step catches the most dangerous failure mode there is, code that runs and passes its own tests but doesn't do what was actually asked, so do not skip ahead to the test suite before finishing this step.

## Step 2: Required approvals

Use the higher risk tier from SPEC.md and PLAN.md to guide review depth. Watts
records and enforces the human approval gates declared by the selected workflow.
The supplied `coder_golang` workflow requires spec and plan approval at every tier.

Inspect approval evidence when it is supplied by the workflow, checking that it
identifies the reviewed artifacts and attempt. Keep additional risk-specific
approval requirements explicit in the workflow. If required evidence is missing
or inconsistent, state that in the review. Do not invent approval files, recreate
approval checks with an undeclared script, or treat a document signature as a
Watts approval. A review Agent does not submit human decisions.

## Step 3: Test suite

Run the review Agent's declared checks against the candidate and inspect the
actual results alongside PLAN.md's Quality gates section. Use supplied build
evidence as context; do not assume a check ran or passed because it is listed in
the plan. Report checks you could not run and why.

- Does it build and run without error?
- Do existing tests still pass?
- Does new or changed behavior have an actual test, not just a claim that it works?

If a gate is red, reject. If you can run a gate yourself and doubt the evidence, do so.

## Step 4: The three failure modes, checked explicitly

- No output produced at all: reject, the task wasn't completed.
- Fails CI: reject, back to the author.
- Passes CI but doesn't match spec: this is what step 1 should already have caught. If it's still possible to fail this way after step 1, that's a sign step 1 wasn't thorough enough, redo it rather than letting this pass because the tests are green.

## Output

Write your verdict to `<task folder>/review/VERDICT.md`, with your reasoning first, covering each acceptance criterion and each story. The last line of the file must be exactly one of these, because the checks read it and treat anything else as invalid:

- `VERDICT: PASS`
- `VERDICT: REJECT`

For a rejection, put the failed step, specific findings, and required changes in
the body above that final line. Do not append an explanation to the verdict line;
the supplied checker requires exactly PASS or REJECT.

The image maps REJECT to `changes-required`. In `coder_golang`, the workflow
routes that outcome back to build with review feedback, subject to its configured
rework limit. Follow the selected workflow's routes; do not launch another stage
or edit its state yourself.
