---
name: admission-rubric
description: Applies a four-step admission and verification check to a finished task's diff before it is allowed to count as done. Use only when reviewing work produced by a different agent instance, never to review your own output, during Testing and Verification.
---

# Admission Rubric

Apply these four steps in this exact order. Stop and reject at the first failure, do not continue to later steps once one has failed, and do not pass something forward with a caveat instead of a clear pass or reject.

You review one whole task at a time. Your task folder is named in your instructions (`<tasks dir>/<name>/`). Read only that folder's SPEC.md, PLAN.md, and STORY_LOG.md, and ignore the other folders under the tasks directory. The review covers everything that changed since the task started. BASE names the baseline snapshot, and the diff is `bash <kit>/scripts/evidence.sh <task> diff` (use `diffstat` for the list of files); the story log holds snapshot ids. Story ids repeat across tasks, so always limit searches to this task's folder and diff. Do not modify any file except your verdict file.

The review loop has already run deterministic checks and shows their output as evidence: the approval check, the story check, the quality gates, and a verdict-file check. Treat them as inputs, not as a substitute for judgment. A clean check does not prove the work is right, and the verdict-file check refuses a PASS when any of them is red.

## Step 1: Spec fidelity

Read SPEC.md in full, including Explicit Non-Goals, and read every story in PLAN.md. Check the diff line by line against the acceptance criteria, the stories, and the Explicit Non-Goals.

- Is every acceptance criterion (AC-1, AC-2, and so on) actually satisfied by the diff, and exercised by a test where one is called for? Name each one in your reasoning.
- Does the diff avoid everything listed under Explicit Non-Goals?
- If the author resolved an ambiguity the spec didn't cover, was that resolution reasonable, or did it invent scope nobody asked for?
- Does each story's snapshot match its story, or was work moved, merged, or skipped?

Reject here if any of the above fails. This step catches the most dangerous failure mode there is, code that runs and passes its own tests but doesn't do what was actually asked, so do not skip ahead to the test suite before finishing this step.

## Step 2: Required approvals

Check Risk tier in SPEC.md and the recheck in PLAN.md, and use the higher of the two.

- Tier 0: require the approval checks declared by the workflow, then proceed to step 3.
- Tier 1: run `bash <kit>/scripts/check-approval.sh <task>` and require `APPROVAL: OK`. The workflow human gates record the reviewed content hashes; any changed document needs another review.
- Tier 2: require those artifact approvals before implementation and inspect the risk-relevant evidence carefully.

Reject if the required approval for the stated tier isn't actually present.

## Step 3: Test suite

Look at the quality gates output in your evidence, which was run just now, and at the commands in PLAN.md's Quality gates section.

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
- `VERDICT: REJECT: <step number and name>: <specific reason>`

A rejection goes back to the author with that reason, never forward with a note attached. The build loop shows the latest verdict to the builder on its next run and refuses to finish until the builder has recorded a newer agent snapshot after fixing it.
