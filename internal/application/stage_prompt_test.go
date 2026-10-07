package application

import (
	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestExtensionCommandKeepsItsArgumentGrammar(t *testing.T) {
	app := &Service{Root: t.TempDir()}
	request := orchestration.ActivityInput{Previous: &orchestration.StageResult{Step: "planning"}}
	require.Equal(t, "/ralph --path ./tasks/example", app.stagePrompt("/ralph --path ./tasks/example", request))
}
func TestPlainPromptReceivesPreviousResultContext(t *testing.T) {
	app := &Service{Root: t.TempDir()}
	request := orchestration.ActivityInput{Previous: &orchestration.StageResult{Step: "planning"}}
	require.Contains(t, app.stagePrompt("Implement the task", request), "Read the previous workflow result")
}
