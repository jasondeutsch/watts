package taskreport

import (
	"testing"

	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/jasondeutsch/watts/internal/workload"
	"github.com/stretchr/testify/require"
)

func TestUsageChartsKeepReworkAndCacheAccountingSeparate(t *testing.T) {
	pointer := func(value int64) *int64 { return &value }
	cost := 0.0
	metrics := func(input, output int64) Metrics {
		result := Metrics{}
		result.add(&workload.Usage{Input: pointer(input), Output: pointer(output), CacheRead: pointer(input), CacheWrite: pointer(0), Cost: &cost})
		return result
	}
	first, repeat, build := metrics(80, 20), metrics(40, 10), metrics(200, 100)
	draft := Metrics{}
	draft.add(&workload.Usage{Input: pointer(120), Output: pointer(30), CacheRead: pointer(120), CacheWrite: pointer(0), Cost: &cost})
	report := Report{Stages: []Stage{
		{Definition: orchestration.Step{Name: "draft", Agent: "generic"}, Attempts: 2, Metrics: draft},
		{Definition: orchestration.Step{Name: "approval", Human: true}, Attempts: 1},
		{Definition: orchestration.Step{Name: "build", Agent: "generic"}, Attempts: 1, Metrics: build},
	}, Attempts: []Attempt{
		{Sequence: 1, StageResult: orchestration.StageResult{Step: "draft", Attempt: 1, Result: orchestration.Result{Outcome: "failure"}}, Metrics: first},
		{Sequence: 2, StageResult: orchestration.StageResult{Step: "approval", Attempt: 1}},
		{Sequence: 3, StageResult: orchestration.StageResult{Step: "draft", Attempt: 2}, Metrics: repeat},
		{Sequence: 4, StageResult: orchestration.StageResult{Step: "build", Attempt: 1}, Metrics: build},
	}}
	panels := buildUsagePanels(report, map[string]stageTiming{"draft": {Seconds: 120, Known: 2}, "approval": {Seconds: 60, Known: 1}, "build": {Seconds: 30, Known: 1}}, Dashboard{})
	require.Len(t, panels, 8)
	require.Equal(t, "150", panels[2].Rows[0].Value) // cache reads are not added again
	require.Equal(t, "0", panels[4].Rows[0].Value)
	require.False(t, panels[4].Rows[0].Unknown)
	require.Equal(t, "$0.000000", panels[7].Rows[0].Value)
	require.False(t, panels[7].Rows[0].Unknown)
	rework := panels[5].Rows[0]
	require.InDelta(t, 100.0/300*100, rework.Series[0].Width, 1e-8)
	require.InDelta(t, 50.0/300*100, rework.Series[1].Width, 1e-8)
	require.Len(t, panels[6].Rows, 3) // human timing is retained, without model usage
	require.Equal(t, "human", panels[6].Rows[1].Series[0].Class)
	plot := buildAttemptUsagePlot(report.Attempts)
	require.Len(t, plot.Points, 3)
	require.Equal(t, []int{1, 3, 4}, []int{plot.Points[0].Sequence, plot.Points[1].Sequence, plot.Points[2].Sequence})
	require.Contains(t, plot.Points[0].Detail, "failure")
	require.Equal(t, "100", plot.Points[0].Value)
	require.InDelta(t, 170, plot.Points[2].InputHeight+plot.Points[2].OutputHeight, 1e-8)
}

func TestUsageChartsKeepMissingAndPartialObservationsVisible(t *testing.T) {
	input, output := int64(100), int64(10)
	known, missing := Metrics{}, Metrics{}
	known.add(&workload.Usage{Input: &input, Output: &output})
	missing.add(nil)
	partial := Metrics{}
	partial.add(&workload.Usage{Input: &input, Output: &output})
	partial.add(nil)
	report := Report{Stages: []Stage{{Definition: orchestration.Step{Name: "draft", Agent: "generic"}, Attempts: 2, Metrics: partial}}, Attempts: []Attempt{
		{Sequence: 1, StageResult: orchestration.StageResult{Step: "draft", Attempt: 1}, Metrics: known},
		{Sequence: 2, StageResult: orchestration.StageResult{Step: "draft", Attempt: 2}, Metrics: missing},
	}}
	panels := buildUsagePanels(report, map[string]stageTiming{"draft": {Seconds: 20, Known: 1}}, Dashboard{})
	require.Equal(t, "100 (partial 1/2)", panels[0].Rows[0].Value)
	require.Equal(t, "partial", panels[0].Rows[0].Series[0].Class)
	require.True(t, panels[3].Rows[0].Unknown)
	require.Equal(t, "Unknown", panels[3].Rows[0].Value)
	require.Nil(t, panels[3].Scale)
	require.Equal(t, "partial", panels[5].Rows[0].Series[0].Class)
	require.Len(t, panels[5].Rows[0].Series, 1) // incomplete first/repeat cannot be split accurately
	require.Contains(t, panels[6].Rows[0].Value, "partial 1/2")
	require.True(t, panels[7].Rows[0].Unknown)
	plot := buildAttemptUsagePlot(report.Attempts)
	require.True(t, plot.Points[1].Unknown)
	require.Equal(t, "Unknown", plot.Points[1].Value)
	require.False(t, plot.Points[0].Unknown)
}
