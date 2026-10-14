package taskreport

import (
	"fmt"
	"math"
	"os"
	"time"

	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/jasondeutsch/watts/internal/workload"
)

type ChartRow struct {
	Label, Value, Detail string
	Width                float64
}

type Observation struct {
	workload.Measurement
	Stage                   string
	Attempt                 int
	BeforeWidth, AfterWidth float64
}

func (observation Observation) BaselineText() string {
	if observation.Baseline == nil {
		return "Unknown"
	}
	return fmt.Sprintf("%.2f", *observation.Baseline)
}

func (observation Observation) DeltaText() string {
	if observation.Baseline == nil {
		return "Unknown"
	}
	delta := observation.Value - *observation.Baseline
	if math.IsNaN(delta) || math.IsInf(delta, 0) {
		return "Unknown"
	}
	return fmt.Sprintf("%+.2f", delta)
}

type GateCoverage struct{ Passed, Failed, Known, Expected int }

type Dashboard struct {
	Elapsed, ApprovalTime, WorkloadTime      string
	ApprovalWidth, WorkloadWidth             float64
	ReworkAttempts, Decisions, Rejections    int
	RepeatTokens                             Metrics
	RepeatShare                              string
	Gates                                    GateCoverage
	Changes                                  WorkspaceChanges
	TokensPerNetLine, LinesPerThousandTokens string
	FileRows                                 []ChartRow
	UsagePanels                              []UsagePanel
	AttemptUsage                             AttemptUsagePlot
	Observations                             []Observation
	Complexity                               string
}

func buildDashboard(report Report, evidence *os.Root) Dashboard {
	dashboard := Dashboard{Elapsed: "Unknown", ApprovalTime: "Unknown", WorkloadTime: "Unknown", RepeatShare: "Unknown", Complexity: "Not reported"}
	stageIndex := make(map[string]Stage, len(report.Stages))
	for _, stage := range report.Stages {
		stageIndex[stage.Definition.Name] = stage
	}
	timings := map[string]stageTiming{}
	visits := map[string]int{}
	approvalSeconds, workloadSeconds := 0.0, 0.0
	allTimesKnown := true
	for _, attempt := range report.Attempts {
		definition := stageIndex[attempt.Step].Definition
		visits[attempt.Step]++
		if visits[attempt.Step] > 1 && !definition.Human {
			dashboard.ReworkAttempts++
			if definition.Agent != "" {
				addMetrics(&dashboard.RepeatTokens, attempt.Metrics)
			}
		}
		if definition.Human {
			dashboard.Decisions++
			if attempt.Outcome == "rejected" {
				dashboard.Rejections++
			}
		}
		seconds, known := attemptSeconds(attempt.StartedAt, attempt.FinishedAt)
		if known {
			timing := timings[attempt.Step]
			timing.Seconds += seconds
			timing.Known++
			timings[attempt.Step] = timing
			if definition.Human {
				approvalSeconds += seconds
			} else {
				workloadSeconds += seconds
			}
		} else {
			allTimesKnown = false
		}
		countGates(&dashboard.Gates, definition, attempt.Gates)
		if definition.Container != nil && definition.Container.Workspace == "write" {
			dashboard.Changes.add(evidence, attempt)
		}
		for _, measurement := range attempt.Measurements {
			observation := Observation{Measurement: measurement, Stage: attempt.Step, Attempt: attempt.Attempt}
			maximum := math.Abs(measurement.Value)
			if measurement.Baseline != nil {
				maximum = math.Max(maximum, math.Abs(*measurement.Baseline))
				observation.BeforeWidth = percentage(math.Abs(*measurement.Baseline), maximum)
			}
			observation.AfterWidth = percentage(math.Abs(measurement.Value), maximum)
			dashboard.Observations = append(dashboard.Observations, observation)
			if measurement.Kind == "complexity" {
				dashboard.Complexity = fmt.Sprintf("%.2f %s", measurement.Value, measurement.Unit)
			}
		}
	}
	if seconds, known := attemptSeconds(report.State.StartedAt, report.State.FinishedAt); known {
		dashboard.Elapsed = durationText(seconds)
	}
	if allTimesKnown {
		dashboard.ApprovalTime, dashboard.WorkloadTime = durationText(approvalSeconds), durationText(workloadSeconds)
		dashboard.ApprovalWidth = percentage(approvalSeconds, approvalSeconds+workloadSeconds)
		dashboard.WorkloadWidth = percentage(workloadSeconds, approvalSeconds+workloadSeconds)
	}
	if complete(report.Metrics.Total) && report.Metrics.Total.Value == 0 {
		dashboard.RepeatShare = "N/A"
	} else if complete(report.Metrics.Total) && complete(dashboard.RepeatTokens.Total) {
		dashboard.RepeatShare = fmt.Sprintf("%.1f%%", percentage(float64(dashboard.RepeatTokens.Total.Value), float64(report.Metrics.Total.Value)))
	} else if complete(report.Metrics.Total) && dashboard.RepeatTokens.Total.Expected == 0 {
		dashboard.RepeatShare = "0.0%"
	}
	dashboard.TokensPerNetLine, dashboard.LinesPerThousandTokens = dashboard.Changes.ratios(report.Metrics.Total)
	if dashboard.Changes.KnownAttempts > 0 {
		maximum := float64(max(dashboard.Changes.Added, dashboard.Changes.Modified, dashboard.Changes.Removed))
		for _, item := range []struct {
			name  string
			count int
		}{{"Added", dashboard.Changes.Added}, {"Modified", dashboard.Changes.Modified}, {"Removed", dashboard.Changes.Removed}} {
			dashboard.FileRows = append(dashboard.FileRows, ChartRow{Label: item.name, Value: fmt.Sprint(item.count), Width: percentage(float64(item.count), maximum)})
		}
	}
	dashboard.UsagePanels = buildUsagePanels(report, timings, dashboard)
	dashboard.AttemptUsage = buildAttemptUsagePlot(report.Attempts)
	return dashboard
}

func countGates(coverage *GateCoverage, definition orchestration.Step, results []workload.GateResult) {
	for _, gate := range definition.Gates {
		coverage.Expected++
		matches := []workload.GateResult{}
		for _, result := range results {
			if result.Name == gate.Name && result.Tool == gate.Tool {
				matches = append(matches, result)
			}
		}
		if len(matches) != 1 {
			continue
		}
		switch matches[0].Status {
		case "passed":
			if matches[0].ExitCode != nil && *matches[0].ExitCode == gate.Expect.ExitCode {
				coverage.Known++
				coverage.Passed++
			}
		case "failed":
			coverage.Known++
			coverage.Failed++
		}
	}
}

func complete(counter Counter) bool { return counter.Expected > 0 && counter.Known == counter.Expected }
func percentage(value, total float64) float64 {
	if total <= 0 {
		return 0
	}
	return math.Min(100, math.Max(0, value/total*100))
}
func durationText(seconds float64) string {
	if seconds >= 60 {
		return fmt.Sprintf("%.1f min", seconds/60)
	}
	return fmt.Sprintf("%.1f s", seconds)
}
func attemptSeconds(start, end time.Time) (float64, bool) {
	if start.IsZero() || end.IsZero() || end.Before(start) {
		return 0, false
	}
	return end.Sub(start).Seconds(), true
}
func addMetrics(target *Metrics, source Metrics) {
	for _, pair := range [][2]*Counter{{&target.Input, &source.Input}, {&target.Output, &source.Output}, {&target.Total, &source.Total}} {
		pair[0].Expected += pair[1].Expected
		if pair[1].Value <= math.MaxInt64-pair[0].Value {
			pair[0].Value += pair[1].Value
			pair[0].Known += pair[1].Known
		}
	}
}
