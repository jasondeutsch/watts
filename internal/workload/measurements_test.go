package workload

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFileMeasurementsPreserveVolumeWithoutSourceContent(t *testing.T) {
	directory := t.TempDir()
	for name, data := range map[string][]byte{
		"source.txt": []byte("one\n\nlast"), "empty.txt": {},
		"windows.txt": []byte("first\r\nsecond\r\n"), "binary": {0, 1, 2}, "invalid-utf8": {255},
	} {
		require.NoError(t, os.WriteFile(filepath.Join(directory, name), data, 0600))
	}
	files, err := Snapshot(directory, nil)
	require.NoError(t, err)
	measurements, err := MeasureFiles(directory, files)
	require.NoError(t, err)
	require.Equal(t, int64(9), measurements["source.txt"].Bytes)
	require.Equal(t, int64(3), *measurements["source.txt"].TextLines)
	require.Equal(t, int64(0), *measurements["empty.txt"].TextLines)
	require.Equal(t, int64(2), *measurements["windows.txt"].TextLines)
	require.Nil(t, measurements["binary"].TextLines)
	require.Nil(t, measurements["invalid-utf8"].TextLines)
	encoded, err := json.Marshal(measurements)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "one\\n")
	require.NoError(t, os.WriteFile(filepath.Join(directory, "source.txt"), []byte("changed"), 0600))
	_, err = MeasureFiles(directory, files)
	require.ErrorContains(t, err, "changed while measuring")
}

func TestReportedMeasurementRequiresExplicitMethodAndValue(t *testing.T) {
	details := map[string]any{"name": "Complexity", "kind": "complexity", "method": "team-check/v1", "unit": "points", "scope": "changed functions", "baseline": 0.0, "value": 0.0}
	measurement, err := ParseMeasurement(Event{Type: "measurement", Details: details})
	require.NoError(t, err)
	require.Equal(t, 0.0, measurement.Value)
	require.NotNil(t, measurement.Baseline)
	require.Equal(t, 0.0, *measurement.Baseline)
	delete(details, "value")
	_, err = ParseMeasurement(Event{Type: "measurement", Details: details})
	require.ErrorContains(t, err, "finite numeric values")
	details["value"] = 12
	details["method"] = " "
	_, err = ParseMeasurement(Event{Type: "measurement", Details: details})
	require.ErrorContains(t, err, "method")
}
