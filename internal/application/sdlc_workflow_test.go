package application

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/jasondeutsch/watts/internal/storage"
)

func TestSDLCManagesDraftingReviewsAndFeedback(t *testing.T) {
	project := newProject(t)
	project.initRepo(t)
	cfg := loadCfg(t, project)
	task, err := project.CreateTask(cfg, "server", "")
	require.NoError(t, err)
	require.Equal(t, "tasks", filepath.Dir(task))
	writeFile(t, filepath.Join(project.Root, task, "SPEC.md"), "Create a Hello World HTTP server in Go")
	definition := projectconfig.DefaultWorkflow()
	// Exercise drafting and both gates through the real worker activities, then a stubbed build.
	definition.Steps = definition.Steps[:5]
	require.NoError(t, storage.WriteJSON(project.TaskWorkflowPath(task), definition))
	customPi(t, sessionScript("deepseek/deepseek-v4-flash")+"\n"+`case "$2" in
 *spec-template*)
 cp "$WATTS_STEP_INPUT" "$WATTS_TASK/spec-input.json"
 cat > "$WATTS_TASK/SPEC.md" <<'SPEC'
# SPEC: Hello World
## Status
Draft
Version: v1
## Acceptance criteria
- AC-1: When a client requests /, the server shall return Hello World.
## Risk tier
1
SPEC
 ;;
 *plan-template*)
 cp "$WATTS_STEP_INPUT" "$WATTS_TASK/plan-input.json"
 cat > "$WATTS_TASK/PLAN.md" <<'PLAN'
# PLAN: Hello World
Spec ref: SPEC.md v1
Risk tier after planning: 1
## Quality gates
- `+"`true`"+`
## User stories
### US-001: Serve Hello World
Covers: AC-1
Depends on: none
#### Acceptance Criteria
- [ ] GET / returns Hello World.
PLAN
 ;;
 *)
 printf 'package main\n' > main.go
 snapshot=$(bash .watts/kit/scripts/snapshot.sh "$WATTS_TASK")
 printf 'DONE US-001 %s\n' "$snapshot" > "$WATTS_TASK/STORY_LOG.md"
 printf 'None.\n' > "$WATTS_TASK/OPEN_QUESTIONS.md"
 ;;
esac`)
	binding, err := project.CreateWorkflowBinding(cfg, task, &RunOptions{})
	require.NoError(t, err)
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterActivityWithOptions(project.ExecuteWorkflowStage, activity.RegisterOptions{Name: orchestration.ExecuteActivity})
	env.RegisterActivityWithOptions(project.CaptureApprovalArtifacts, activity.RegisterOptions{Name: orchestration.HashActivity})
	env.RegisterActivityWithOptions(project.ValidateApprovalArtifacts, activity.RegisterOptions{Name: orchestration.ValidateActivity})
	for index, action := range []struct {
		step     string
		reject   bool
		feedback string
	}{
		{"spec-review", true, "State the response content type"},
		{"spec-review", false, ""},
		{"plan-review", true, "Add a handler test"},
		{"plan-review", false, ""},
	} {
		env.RegisterDelayedCallback(func() {
			value, err := env.QueryWorkflow(orchestration.StatusQuery)
			require.NoError(t, err)
			var state orchestration.State
			require.NoError(t, value.Get(&state))
			require.Equal(t, "waiting_approval", state.Status)
			require.Equal(t, action.step, state.Current)
			var stage orchestration.Stage
			for _, candidate := range state.Stages {
				if candidate.Name == action.step {
					stage = candidate
				}
			}
			env.UpdateWorkflowNoRejection(orchestration.DecisionUpdate, action.step+time.Duration(index).String(), t,
				orchestration.Decision{Step: stage.Name, Attempt: stage.Attempts, By: "Human", Reject: action.reject, Feedback: action.feedback, Artifacts: stage.Artifacts})
		}, time.Duration(index+1)*time.Second)
	}
	env.ExecuteWorkflow(orchestration.Run, binding.Input)
	require.NoError(t, env.GetWorkflowError())
	var state orchestration.State
	require.NoError(t, env.GetWorkflowResult(&state))
	require.Equal(t, "completed", state.Status)
	assert.Equal(t, 2, state.Stages[0].Attempts)
	assert.Equal(t, 2, state.Stages[2].Attempts)
	assert.Contains(t, readFile(t, filepath.Join(project.Root, task, "spec-input.json")), "State the response content type")
	assert.Contains(t, readFile(t, filepath.Join(project.Root, task, "plan-input.json")), "Add a handler test")
	require.NoError(t, project.KitScript(cfg, "check-approval.sh", task))
	writeFile(t, filepath.Join(project.Root, task, "PLAN.md"), "changed after approval")
	require.Error(t, project.KitScript(cfg, "check-approval.sh", task))
}

func TestAgentPreconditionsStopBeforePi(t *testing.T) {
	project := newProject(t)
	project.initRepo(t)
	cfg := loadCfg(t, project)
	task, err := project.CreateTask(cfg, "empty", "")
	require.NoError(t, err)
	marker := filepath.Join(project.Root, "pi-called")
	customPi(t, "touch pi-called")
	definition := projectconfig.DefaultWorkflow()
	definition.Steps[0].PreChecks = []orchestration.Check{{Script: "check-spec"}}
	require.NoError(t, storage.WriteJSON(project.TaskWorkflowPath(task), definition))
	binding, err := project.CreateWorkflowBinding(cfg, task, &RunOptions{})
	require.NoError(t, err)
	step := binding.Input.Definition.Steps[0]
	request := orchestration.ActivityInput{Input: binding.Input, Step: step, Attempt: 1}
	runtime, err := project.prepareActivity(context.Background(), request)
	require.NoError(t, err)
	_, err = runtime.prepareInput()
	require.NoError(t, err)
	_, err = runtime.execute()
	require.ErrorContains(t, err, "precondition check check-spec")
	_, err = os.Stat(marker)
	require.True(t, os.IsNotExist(err))
}
