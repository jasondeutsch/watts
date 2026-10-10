package application

import (
	"context"
	"fmt"
	"strings"

	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	temporallog "go.temporal.io/sdk/log"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/kit"
	"github.com/jasondeutsch/watts/internal/orchestration"
)

func (app *Service) ResolvedTemporalSettings(config projectconfig.Config) projectconfig.TemporalSettings {
	settings := projectconfig.TemporalSettings{}
	if config.Temporal != nil {
		settings = *config.Temporal
	}
	if settings.Address == "" {
		settings.Address = "localhost:7233"
	}
	if settings.Namespace == "" {
		settings.Namespace = "default"
	}
	if settings.TaskQueue == "" {
		settings.TaskQueue = "watts-" + kit.Digest([]byte(app.Root))[:16]
	}
	return settings
}

func ConnectTemporal(ctx context.Context, settings projectconfig.TemporalSettings) (client.Client, error) {
	return connectTemporal(ctx, settings, nil)
}

// ConnectTemporalWorker directs SDK and workflow logs to the worker's output.
func (app *Service) ConnectTemporalWorker(ctx context.Context, settings projectconfig.TemporalSettings) (client.Client, error) {
	return connectTemporal(ctx, settings, app.WorkerLogger())
}

func connectTemporal(ctx context.Context, settings projectconfig.TemporalSettings, logger temporallog.Logger) (client.Client, error) {
	temporalClient, err := client.DialContext(ctx, client.Options{HostPort: settings.Address, Namespace: settings.Namespace, Logger: logger})
	if err != nil {
		return nil, fmt.Errorf("cannot connect to Temporal at %s (namespace %s): %w", settings.Address, settings.Namespace, err)
	}
	return temporalClient, nil
}

func QueryTemporalWorkflowState(ctx context.Context, temporalClient client.Client, binding WorkflowBinding) (orchestration.State, error) {
	var state orchestration.State
	execution, err := temporalClient.DescribeWorkflowExecution(ctx, binding.WorkflowID, binding.RunID)
	if err != nil {
		return state, err
	}
	// A completed execution's result is durable and does not require a running worker to query.
	if execution.WorkflowExecutionInfo.Status != enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING {
		err = temporalClient.GetWorkflow(ctx, binding.WorkflowID, binding.RunID).Get(ctx, &state)
		if err != nil {
			state.Status = strings.ToLower(strings.TrimPrefix(execution.WorkflowExecutionInfo.Status.String(), "WORKFLOW_EXECUTION_STATUS_"))
			return state, nil
		}
		return state, nil
	}
	value, err := temporalClient.QueryWorkflow(ctx, binding.WorkflowID, binding.RunID, orchestration.StatusQuery)
	if err != nil {
		return state, fmt.Errorf("cannot query workflow status; check that the project worker is running: %w", err)
	}
	err = value.Get(&state)
	return state, err
}
