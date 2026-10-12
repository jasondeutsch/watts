package application

import (
	"testing"

	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/stretchr/testify/require"
)

func TestDelegationRequiresActorAuthorityAndReviewedSnapshot(t *testing.T) {
	for _, options := range []DecisionOptions{
		{Actor: "coordinator"},
		{AuthorizedBy: "human"},
		{Actor: "coordinator", AuthorizedBy: "human"},
		{Actor: "coordinator\nforged", AuthorizedBy: "human", ReviewedStatus: &TemporalTaskStatus{}},
	} {
		require.Error(t, options.validate())
	}
	require.NoError(t, (DecisionOptions{}).validate(), "direct human decisions remain supported")
	require.NoError(t, (DecisionOptions{Actor: "coordinator", AuthorizedBy: "human", ReviewedStatus: &TemporalTaskStatus{}}).validate())
}

func TestReviewedApprovalBindsExecutionAttemptAndArtifacts(t *testing.T) {
	stage := orchestration.Stage{Name: "approve", Status: "waiting_approval", Attempts: 2, Artifacts: []orchestration.Artifact{{Path: "SPEC.md", SHA256: "reviewed-hash"}}}
	binding := WorkflowBinding{WorkflowID: "workflow", RunID: "run", Input: orchestration.Input{Task: "tasks/example"}}
	view := TemporalTaskStatus{Task: binding.Input.Task, WorkflowID: binding.WorkflowID, RunID: binding.RunID, State: orchestration.State{Status: "waiting_approval", Current: stage.Name, Stages: []orchestration.Stage{stage}}}
	require.NoError(t, reviewedApproval(view, binding, stage))
	for _, mutate := range []func(*TemporalTaskStatus){
		func(v *TemporalTaskStatus) { v.Task = "tasks/other" },
		func(v *TemporalTaskStatus) { v.WorkflowID = "other-workflow" },
		func(v *TemporalTaskStatus) { v.RunID = "other-run" },
		func(v *TemporalTaskStatus) { v.State.Status = "running" },
		func(v *TemporalTaskStatus) { v.State.Current = "other-stage" },
		func(v *TemporalTaskStatus) { v.State.Stages[0].Attempts-- },
		func(v *TemporalTaskStatus) {
			v.State.Stages[0].Artifacts = []orchestration.Artifact{{Path: "SPEC.md", SHA256: "stale-hash"}}
		},
	} {
		candidate := view
		candidate.State.Stages = append([]orchestration.Stage(nil), view.State.Stages...)
		mutate(&candidate)
		require.Error(t, reviewedApproval(candidate, binding, stage))
	}
}
