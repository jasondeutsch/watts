package taskreport

import (
	"fmt"
	"os"
	"sort"

	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/jasondeutsch/watts/internal/workload"
)

type WorkspaceChanges struct {
	NetBytes                        int64
	KnownAttempts, ExpectedAttempts int
	Added, Modified, Removed        int
	KnownFiles, ExpectedFiles       int
	NetTextLines, NewFileTextLines  int64
	NonTextFiles                    int
}

func (changes *WorkspaceChanges) add(root *os.Root, attempt Attempt) {
	changes.ExpectedAttempts++
	directory := fmt.Sprintf(".watts-state/steps/%s-%d/", attempt.Step, attempt.Attempt)
	var receipt struct {
		Execution string               `json:"execution"`
		Result    orchestration.Result `json:"result"`
	}
	if readJSON(root, directory+"accepted.json", &receipt) != nil || receipt.Execution == "" || receipt.Result.Outcome != attempt.Outcome || attempt.Error != "" {
		return
	}
	var snapshot workload.WorkspaceEvidence
	if readJSON(root, directory+"workspace.json", &snapshot) != nil || snapshot.Baseline == nil || snapshot.Candidate == nil {
		return
	}
	changes.KnownAttempts++
	for _, name := range changedPaths(snapshot) {
		_, beforeExists := snapshot.Baseline[name]
		_, afterExists := snapshot.Candidate[name]
		switch {
		case !beforeExists:
			changes.Added++
		case !afterExists:
			changes.Removed++
		default:
			changes.Modified++
		}
		changes.ExpectedFiles++
		before, beforeKnown := snapshot.BaselineMeasurements[name]
		after, afterKnown := snapshot.CandidateMeasurements[name]
		if snapshot.MeasurementsVersion != 1 || (beforeExists && !validFileMeasurement(before, beforeKnown)) || (afterExists && !validFileMeasurement(after, afterKnown)) {
			continue
		}
		changes.KnownFiles++
		changes.NetBytes += after.Bytes - before.Bytes
		beforeLines, afterLines := int64(0), int64(0)
		if before.TextLines != nil {
			beforeLines = *before.TextLines
		}
		if after.TextLines != nil {
			afterLines = *after.TextLines
		}
		changes.NetTextLines += afterLines - beforeLines
		if !beforeExists {
			changes.NewFileTextLines += afterLines
		}
		if (beforeExists && before.TextLines == nil) || (afterExists && after.TextLines == nil) {
			changes.NonTextFiles++
		}
	}
}

func validFileMeasurement(measurement workload.FileMeasurement, known bool) bool {
	return known && measurement.Bytes >= 0 && measurement.Bytes <= workload.MaximumFileBytes && (measurement.TextLines == nil || (*measurement.TextLines >= 0 && *measurement.TextLines <= measurement.Bytes))
}
func (changes WorkspaceChanges) FilesText() string {
	if changes.ExpectedAttempts == 0 {
		return "N/A"
	}
	if changes.KnownAttempts == 0 {
		return "Unknown"
	}
	value := fmt.Sprint(changes.Added + changes.Modified + changes.Removed)
	if changes.KnownAttempts != changes.ExpectedAttempts {
		value += fmt.Sprintf(" (partial %d/%d)", changes.KnownAttempts, changes.ExpectedAttempts)
	}
	return value
}
func (changes WorkspaceChanges) LinesText() string {
	if changes.ExpectedAttempts == 0 {
		return "N/A"
	}
	if changes.KnownAttempts != changes.ExpectedAttempts || changes.KnownFiles != changes.ExpectedFiles {
		return "Unknown"
	}
	return fmt.Sprintf("%+d", changes.NetTextLines)
}
func (changes WorkspaceChanges) NewLinesText() string {
	if changes.ExpectedAttempts == 0 {
		return "N/A"
	}
	if changes.KnownAttempts != changes.ExpectedAttempts || changes.KnownFiles != changes.ExpectedFiles {
		return "Unknown"
	}
	return fmt.Sprint(changes.NewFileTextLines)
}
func (changes WorkspaceChanges) ratios(tokens Counter) (string, string) {
	if changes.ExpectedAttempts == 0 || tokens.Expected == 0 {
		return "N/A", "N/A"
	}
	if !complete(tokens) || changes.KnownAttempts != changes.ExpectedAttempts || changes.KnownFiles != changes.ExpectedFiles {
		return "Unknown", "Unknown"
	}
	if tokens.Value == 0 {
		return "N/A", "N/A"
	}
	perLine := "N/A (no net increase)"
	if changes.NetTextLines > 0 {
		perLine = fmt.Sprintf("%.1f", float64(tokens.Value)/float64(changes.NetTextLines))
	}
	return perLine, fmt.Sprintf("%+.2f", float64(changes.NetTextLines)*1000/float64(tokens.Value))
}

func (changes WorkspaceChanges) NonTextChangesText() string {
	if changes.ExpectedAttempts == 0 {
		return "N/A"
	}
	if changes.KnownAttempts != changes.ExpectedAttempts || changes.KnownFiles != changes.ExpectedFiles {
		return "Unknown"
	}
	return fmt.Sprint(changes.NonTextFiles)
}
func (changes WorkspaceChanges) BytesText() string {
	if changes.ExpectedAttempts == 0 {
		return "N/A"
	}
	if changes.KnownAttempts != changes.ExpectedAttempts || changes.KnownFiles != changes.ExpectedFiles {
		return "Unknown"
	}
	return fmt.Sprintf("%+d B", changes.NetBytes)
}

// Stable path ordering is useful for both reports and their retained evidence.
func changedPaths(snapshot workload.WorkspaceEvidence) []string {
	names := map[string]bool{}
	for name := range snapshot.Baseline {
		names[name] = true
	}
	for name := range snapshot.Candidate {
		names[name] = true
	}
	paths := []string{}
	for name := range names {
		if snapshot.Baseline[name] != snapshot.Candidate[name] {
			paths = append(paths, name)
		}
	}
	sort.Strings(paths)
	return paths
}
