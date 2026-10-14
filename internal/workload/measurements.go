package workload

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"unicode/utf8"
)

// FileMeasurement describes physical volume, not executable lines or quality.
// A nil TextLines means the file is not UTF-8 text without NUL bytes.
type FileMeasurement struct {
	Bytes     int64  `json:"bytes"`
	TextLines *int64 `json:"text_lines,omitempty"`
}

type WorkspaceEvidence struct {
	Baseline              map[string]File            `json:"baseline"`
	Candidate             map[string]File            `json:"candidate"`
	MeasurementsVersion   int                        `json:"measurements_version,omitempty"`
	BaselineMeasurements  map[string]FileMeasurement `json:"baseline_measurements,omitempty"`
	CandidateMeasurements map[string]FileMeasurement `json:"candidate_measurements,omitempty"`
}

// MeasureFiles checks the captured content identity while counting its volume.
// It never retains source text, executes the product, or classifies languages.
func MeasureFiles(directory string, files map[string]File) (map[string]FileMeasurement, error) {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	measurements := make(map[string]FileMeasurement, len(files))
	for name, expected := range files {
		if !Relative(name) {
			return nil, fmt.Errorf("invalid measurement path %q", name)
		}
		data, mode, err := readRootFile(root, name, MaximumFileBytes)
		if err != nil {
			return nil, err
		}
		digest := sha256.Sum256(data)
		if hex.EncodeToString(digest[:]) != expected.SHA256 || uint32(mode) != expected.Mode {
			return nil, fmt.Errorf("workspace changed while measuring %s", name)
		}
		measurement := FileMeasurement{Bytes: int64(len(data))}
		if utf8.Valid(data) && !bytes.ContainsRune(data, '\x00') {
			lines := int64(bytes.Count(data, []byte{'\n'}))
			if len(data) > 0 && data[len(data)-1] != '\n' {
				lines++
			}
			measurement.TextLines = &lines
		}
		measurements[name] = measurement
	}
	return measurements, nil
}

// Measurement is an optional workload-reported observation carried in an event's
// details. Method, units, and scope keep unlike scores from being combined.
type Measurement struct {
	Name     string   `json:"name"`
	Kind     string   `json:"kind,omitempty"`
	Method   string   `json:"method"`
	Unit     string   `json:"unit"`
	Scope    string   `json:"scope"`
	Value    float64  `json:"value"`
	Baseline *float64 `json:"baseline,omitempty"`
}

func ParseMeasurement(event Event) (Measurement, error) {
	var measurement Measurement
	data, err := json.Marshal(event.Details)
	if err != nil {
		return measurement, err
	}
	// Pointer distinguishes a missing value from a measured zero.
	var wire struct {
		Measurement
		Value *float64 `json:"value"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&wire); err != nil {
		return measurement, fmt.Errorf("invalid measurement details")
	}
	measurement = wire.Measurement
	if wire.Value == nil || !finite(*wire.Value) || (measurement.Baseline != nil && !finite(*measurement.Baseline)) {
		return measurement, fmt.Errorf("measurement requires finite numeric values")
	}
	measurement.Value = *wire.Value
	for _, label := range []string{measurement.Name, measurement.Method, measurement.Unit, measurement.Scope} {
		if strings.TrimSpace(label) == "" || len(label) > 200 {
			return measurement, fmt.Errorf("measurement requires bounded name, method, unit, and scope")
		}
	}
	if len(measurement.Kind) > 200 {
		return measurement, fmt.Errorf("measurement kind is too long")
	}
	return measurement, nil
}

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }
