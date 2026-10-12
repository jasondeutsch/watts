package application

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
)

func TestWorkerLogsAttemptDiagnosticsAndChecks(t *testing.T) {
	requireDocker(t)
	step := containerStep("printf 'compiler: undefined symbol\\n' >&2; exit 1")
	step.Name = "build"
	project, request := boundTemporalTask(t, step)
	var log bytes.Buffer
	var suite testsuite.WorkflowTestSuite
	app := &Service{Root: project.Root, Output: &log}
	suite.SetLogger(app.WorkerLogger())
	env := suite.NewTestActivityEnvironment()
	env.RegisterActivity(project.ExecuteWorkflowStage)
	_, err := env.ExecuteActivity(project.ExecuteWorkflowStage, request)
	require.ErrorContains(t, err, "compiler: undefined symbol")
	text := log.String()
	require.Contains(t, text, `msg="Attempt execution started"`)
	require.Contains(t, text, "stage=build attempt=1")
	require.Contains(t, text, `msg="Container executing"`)
	require.Contains(t, text, `msg="Attempt execution failed"`)
	require.Contains(t, text, "compiler: undefined symbol")
	require.Contains(t, text, "next_action=")
	require.Contains(t, text, "build-1.log")
	require.NotContains(t, text, "RecordActivityTaskHeartbeat")
	require.Contains(t, readFile(t, project.StateDir(request.Input.Task)+"/logs/build-1.log"), "compiler: undefined symbol")
}

func TestWorkerLoggerKeepsDebugNoiseOut(t *testing.T) {
	var output bytes.Buffer
	logger := (&Service{Root: "/work/project", Output: &output}).WorkerLogger()
	logger.Debug("heartbeat")
	logger.Info("Worker ready", "queue", "project-queue")
	logger.Warn("Worker stopping")
	text := output.String()
	require.NotContains(t, text, "heartbeat")
	require.Contains(t, text, "time=")
	require.Contains(t, text, "level=INFO")
	require.Contains(t, text, "project=/work/project")
	require.Contains(t, text, "queue=project-queue")
	require.Contains(t, text, "level=WARN")
}
