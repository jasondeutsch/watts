package application

import (
	"context"
	"io"
	"strings"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"github.com/jasondeutsch/watts/internal/orchestration"
)

func (app *Service) NewTemporalWorker(temporalClient client.Client, queue string) worker.Worker {
	// Keep the SDK heartbeat throttling defaults: the throttle interval also
	// sets the heartbeat RPC deadline, so a short override can cancel healthy
	// activities when the server or local machine is briefly busy.
	workflowWorker := worker.New(temporalClient, queue, worker.Options{MaxConcurrentActivityExecutionSize: 1, WorkerStopTimeout: 10 * time.Second})
	workflowWorker.RegisterWorkflowWithOptions(orchestration.Run, workflow.RegisterOptions{Name: orchestration.WorkflowName})
	workflowWorker.RegisterActivityWithOptions(app.ExecuteWorkflowStage, activity.RegisterOptions{Name: orchestration.ExecuteActivity})
	workflowWorker.RegisterActivityWithOptions(app.CaptureApprovalArtifacts, activity.RegisterOptions{Name: orchestration.HashActivity})
	workflowWorker.RegisterActivityWithOptions(app.ValidateApprovalArtifacts, activity.RegisterOptions{Name: orchestration.ValidateActivity})
	return workflowWorker
}

// Avoid importing interactive terminal state into worker processes.
func EmptyWorkerInput() io.Reader { return strings.NewReader("") }

func (app *Service) RunTemporalWorker(ctx context.Context) error {
	config, err := app.LoadConfig()
	if err != nil {
		return err
	}
	settings := app.ResolvedTemporalSettings(config)
	dialCtx, dialCancel := context.WithTimeout(ctx, 10*time.Second)
	defer dialCancel()
	temporalClient, err := app.ConnectTemporalWorker(dialCtx, settings)
	if err != nil {
		return err
	}
	defer temporalClient.Close()
	workflowWorker := app.NewTemporalWorker(temporalClient, settings.TaskQueue)
	if err = workflowWorker.Start(); err != nil {
		return err
	}
	logger := app.WorkerLogger()
	defer func() {
		logger.Info("Worker stopping", "next_action", "Active work is stopping; Temporal retains workflow history")
		workflowWorker.Stop()
		logger.Info("Worker stopped", "next_action", "Run watts start to resume processing")
	}()
	logger.Info("Worker ready", "address", settings.Address, "namespace", settings.Namespace, "queue", settings.TaskQueue)
	<-ctx.Done()
	return nil
}
