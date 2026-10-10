# Task monitor

Run from your project directory:

```sh
watts start
# Or run in the background:
watts start -d
# Stop the worker and UI:
watts stop
```

Startup opens your default browser automatically. The UI binds to an available loopback port and prints its URL. Detached service logs are in `.watts/service.log`.

The sidebar discovers all immediate task folders in the configured `tasks_dir`, refreshes every five seconds, and lets you switch tasks. Incomplete tasks remain selectable and show their configuration error. The selected task streams snapshots every two seconds with stage status, attempts, workflow routes (including conditional branches and loops), agent configuration, failure feedback, and local stage log tails. Log output depends on when the worker writes it; this does not stream individual model tokens.

Submitted tasks display their frozen workflow definition, even if `workflow.yaml` has since been edited. Execution state comes from Temporal through its Go SDK. If Temporal or the worker cannot answer, the monitor explicitly marks status unavailable and still displays the workflow and local logs. Unsubmitted tasks show their task workflow and pending stages.

The monitor offers a retry button for tasks waiting for retry. Retry uses the submitted configuration and explicitly authorizes another attempt. The worker and UI stop together. Temporal remains a separately managed dependency.

Run its tests with `go test ./web`.
