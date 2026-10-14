# Secret protection

Declare credentials with ACL `secret` source bindings, then reference those keys
from Agent authentication, container secrets, or MCP credentials. Compilation
records environment **names**, never resolves credential values. The worker
resolves values for an execution and keeps its redaction set in memory.

## Capture and publication

Watts replaces known secret values with `[REDACTED]` before writing container
stdout/stderr to the terminal or attempt logs. Each stream retains incomplete
matches across chunks and flushes when execution ends. Diagnostic tails therefore
feed sanitized errors into worker/SDK logs and Temporal history. Result feedback,
event details, tool evidence, and rendered completion reports are sanitized too;
JSON numeric usage counters retain their values.

Declared task outputs and candidate source containing a known credential are
rejected before publication. Watts does not silently rewrite code or documents
that passed gates. Remove the credential from the candidate and retry through the
workflow. Private container workspaces are not safe export sources.

`wctl task new` and `wctl start` add these rules to the project's `.gitignore`,
preserving existing contents:

```gitignore
/.watts/
**/.watts-state/
```

These protect service logs, private state, and task evidence from ordinary Git
adds. Authored `watts/` ACL packages and intentional task reports remain visible.
Watts does not run Git, untrack files, or change commits. An already tracked file
remains tracked despite ignore rules. Private worker state is separate from
shareable reports; see the [runtime layout](package-runtime-layout.md).

## Image responsibilities

The generic environment includes `WATTS_SECRET_ENV_NAMES`: a JSON array of
container environment names carrying declared secrets. It contains no values.
An image adapter can obtain its in-memory redaction context from these bindings.
Adapters must sanitize tool results **before model ingestion**, and sanitize
telemetry and tool evidence before writing them. Core log filtering cannot undo a
credential already sent to a provider or written by a workload.

The supplied Pi/Ralph image installs tool-result and provider-context hooks in
both the main Agent and nested Ralph Agents. Its declared command tools filter
stdout/stderr before truncation, evidence persistence, and model feedback. The
launcher also filters child-process console output and telemetry. Rebuild this
image and derived images to use those protections. Existing tasks pin image
identity; recompiling and creating a new task selects the updated image.

## Limits and sharing evidence

This matches declared values and common JSON/string, HTML, and URL-escaped forms.
It cannot guarantee recognition of unknown keys, fragments, arbitrary encodings,
or deliberately transformed credentials. Environment credentials are available
to the executing workload; redaction is not a sandbox, network policy, or complete
data-loss prevention mechanism. A workload can still write raw credentials to
its own private files. The container engine itself receives credential-bearing
environment configuration and may retain workload logs. Protect engine access
and private runtime storage accordingly.

Share newly generated sanitized reports and deliberately selected sanitized
attempt evidence. Do not archive entire `.watts/`, `.watts-state/`, container
runtime directories, or agent session directories. Before publishing an eval
bundle, scan every included file for the relevant credential values and review
its contents. Retain stage outcomes, failures, usage counts, and approvals; removing
a secret is not a reason to rewrite the run's outcome. A report's final redaction
pass can use only currently available declared values, so rotated or undeclared
historical credentials require separate cleanup. There is no automatic historical
archive scrubber.

## Existing disclosure

1. Revoke or rotate the exposed credential with its provider.
2. Identify affected logs, task evidence, exports, container storage, and Git
   commits without pasting the credential into an issue or chat.
3. Prepare sanitized evidence copies for sharing and restrict/remove unsafe
   private copies according to your retention policy.
4. Untrack private runtime files explicitly. Remove the secret from every commit
   being pushed; adding `.gitignore` or deleting the latest working-tree file
   alone does not remove it from earlier commits. Coordinate any shared-history
   rewrite with collaborators.
5. Retry the push after cleanup. Do not bypass push protection for a real key.

Tests use synthetic credentials: every stream split, escaped/nested data, command
failure, report rendering, SDK activity logs, Pi/Ralph model context, actual
Temporal history, and Git tracking. They make no paid model calls.
