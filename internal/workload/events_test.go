package workload

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUsageEvidenceIsDeduplicatedWithoutInventingMissingCounters(t *testing.T) {
	file := filepath.Join(t.TempDir(), "events.jsonl")
	line := `{"id":"message-1","type":"usage","usage":{"input_tokens":20,"output_tokens":0}}` + "\n"
	require.NoError(t, os.WriteFile(file, []byte(line+line), 0600))
	events, err := ReadEvents(file)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.EqualValues(t, 20, *events[0].Usage.Input)
	require.EqualValues(t, 0, *events[0].Usage.Output)
	require.Nil(t, events[0].Usage.CacheRead)
	require.Nil(t, events[0].Usage.Cost)

	require.NoError(t, os.WriteFile(file, []byte(line+`{"id":"message-1","type":"usage","usage":{"input_tokens":21}}`), 0600))
	_, err = ReadEvents(file)
	require.ErrorContains(t, err, "conflicting event id")
}

func TestMissingTelemetryRemainsUnknown(t *testing.T) {
	events, err := ReadEvents(filepath.Join(t.TempDir(), "absent.jsonl"))
	require.NoError(t, err)
	require.Nil(t, events)
}
