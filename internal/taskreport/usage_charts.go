package taskreport

import (
	"fmt"
	"math"
)

// Usage panels share a row/scale contract so units and missing evidence are
// consistent across token, cache, timing, cost, and rework views.
type UsageSeries struct {
	Label, Class string
	Width        float64
}
type UsageRow struct {
	Label, Value, Detail string
	Series               []UsageSeries
	Unknown              bool
}
type UsagePanel struct {
	Title, Unit, Caption string
	Legend               []UsageSeries
	Rows                 []UsageRow
	Scale                []string
}
type PlotTick struct {
	Label string
	Y     float64
}
type AttemptUsagePoint struct {
	Stage, Value, Detail                                   string
	Sequence, Attempt                                      int
	X, Center, Width, Y, InputY, InputHeight, OutputHeight float64
	Unknown, Partial                                       bool
}
type AttemptUsagePlot struct {
	Width  int
	Points []AttemptUsagePoint
	Ticks  []PlotTick
}

type stageTiming struct {
	Seconds float64
	Known   int
}

type counterPanelDefinition struct {
	Title, SeriesLabel, Class, Caption string
	SelectCounter                      func(Metrics) Counter
}

func buildUsagePanels(report Report, timings map[string]stageTiming, dashboard Dashboard) []UsagePanel {
	definitions := []counterPanelDefinition{
		{
			Title: "Input tokens", SeriesLabel: "input", Class: "input",
			Caption:       "Input across all recorded attempts, including rework. Provider cache/input accounting varies.",
			SelectCounter: func(m Metrics) Counter { return m.Input },
		},
		{
			Title: "Output tokens", SeriesLabel: "output", Class: "output",
			Caption:       "Output across all recorded attempts, including rework. Each panel has its own token scale.",
			SelectCounter: func(m Metrics) Counter { return m.Output },
		},
		{
			Title: "Total tokens", SeriesLabel: "input + output", Class: "total",
			Caption:       "Input + output across recorded attempts. Cache counters are not added again; partial totals are gray.",
			SelectCounter: func(m Metrics) Counter { return m.Total },
		},
		{
			Title: "Cache reads", SeriesLabel: "cache read", Class: "cache-read",
			Caption:       "Provider-reported cache reads; not added again to input + output. Counts do not establish dollar savings.",
			SelectCounter: func(m Metrics) Counter { return m.CacheRead },
		},
		{
			Title: "Cache writes", SeriesLabel: "cache write", Class: "cache-write",
			Caption:       "Provider-reported cache writes; not added again to input + output. A reported zero differs from unknown.",
			SelectCounter: func(m Metrics) Counter { return m.CacheWrite },
		},
	}
	panels := make([]UsagePanel, 0, len(definitions)+3)
	for _, definition := range definitions {
		panels = append(panels, counterPanel(report, definition))
	}
	return append(panels, reworkPanel(report), timingPanel(report, timings, dashboard), costPanel(report))
}

func counterPanel(report Report, definition counterPanelDefinition) UsagePanel {
	panel := UsagePanel{Title: definition.Title, Unit: "tokens", Caption: definition.Caption, Legend: []UsageSeries{{Label: definition.SeriesLabel, Class: definition.Class}}}
	maximum := 0.0
	for _, stage := range report.Stages {
		maximum = math.Max(maximum, float64(definition.SelectCounter(stage.Metrics).Value))
	}
	for _, stage := range report.Stages {
		counter := definition.SelectCounter(stage.Metrics)
		if counter.Expected == 0 {
			continue
		}
		color := definition.Class
		if !complete(counter) {
			color = "partial"
		}
		panel.Rows = append(panel.Rows, UsageRow{Label: stage.Definition.Name, Value: counter.String(), Unknown: counter.Known == 0,
			Detail: fmt.Sprintf("%s · %d attempts · %s %s tokens · %d/%d observations", stage.Definition.Name, stage.Attempts, counter.String(), definition.SeriesLabel, counter.Known, counter.Expected),
			Series: []UsageSeries{{Class: color, Width: percentage(float64(counter.Value), maximum)}}})
	}
	panel.Scale = scaleLabels(maximum, compactNumber)
	return panel
}

func reworkPanel(report Report) UsagePanel {
	panel := UsagePanel{Title: "First visit vs. repeat visits", Unit: "tokens", Caption: "Input + output. Later visits include feedback loops and retries; approval visits are excluded. Partial totals are gray.", Legend: []UsageSeries{{Label: "first", Class: "input"}, {Label: "repeat", Class: "repeat"}}}
	first, repeat := map[string]Metrics{}, map[string]Metrics{}
	seen := map[string]bool{}
	for _, attempt := range report.Attempts {
		if attempt.Metrics.Total.Expected == 0 {
			continue
		}
		target := first[attempt.Step]
		if seen[attempt.Step] {
			target = repeat[attempt.Step]
		}
		addMetrics(&target, attempt.Metrics)
		if seen[attempt.Step] {
			repeat[attempt.Step] = target
		} else {
			first[attempt.Step] = target
		}
		seen[attempt.Step] = true
	}
	maximum := 0.0
	for _, stage := range report.Stages {
		maximum = math.Max(maximum, float64(stage.Metrics.Total.Value))
	}
	for _, stage := range report.Stages {
		if stage.Metrics.Total.Expected == 0 {
			continue
		}
		initial, later := first[stage.Definition.Name].Total, repeat[stage.Definition.Name].Total
		series := []UsageSeries{{Class: "input", Width: percentage(float64(initial.Value), maximum)}, {Class: "repeat", Width: percentage(float64(later.Value), maximum)}}
		if !complete(stage.Metrics.Total) {
			series = []UsageSeries{{Class: "partial", Width: percentage(float64(stage.Metrics.Total.Value), maximum)}}
		}
		laterText := later.String()
		if later.Expected == 0 {
			laterText = "0 (no repeat visit)"
		}
		panel.Rows = append(panel.Rows, UsageRow{Label: stage.Definition.Name, Value: stage.Metrics.Total.String(), Unknown: stage.Metrics.Total.Known == 0, Series: series,
			Detail: fmt.Sprintf("%s · first %s · repeat %s tokens", stage.Definition.Name, initial.String(), laterText)})
	}
	panel.Scale = scaleLabels(maximum, compactNumber)
	return panel
}

func timingPanel(report Report, timings map[string]stageTiming, dashboard Dashboard) UsagePanel {
	panel := UsagePanel{Title: "Recorded stage time", Unit: "elapsed", Legend: []UsageSeries{{Label: "workload", Class: "input"}, {Label: "approval", Class: "human"}}, Caption: fmt.Sprintf("Workload stages: %s · approval stages: %s. Includes tools and waiting; not inference speed or active human time.", dashboard.WorkloadTime, dashboard.ApprovalTime)}
	maximum := 0.0
	for _, timing := range timings {
		maximum = math.Max(maximum, timing.Seconds)
	}
	for _, stage := range report.Stages {
		if stage.Attempts == 0 {
			continue
		}
		timing := timings[stage.Definition.Name]
		value, class := durationText(timing.Seconds), "input"
		if stage.Definition.Human {
			class = "human"
		}
		if timing.Known == 0 {
			value = "Unknown"
		} else if timing.Known != stage.Attempts {
			value += fmt.Sprintf(" (partial %d/%d)", timing.Known, stage.Attempts)
			class = "partial"
		}
		panel.Rows = append(panel.Rows, UsageRow{Label: stage.Definition.Name, Value: value, Detail: fmt.Sprintf("%s · %d recorded visits · %s", stage.Definition.Name, stage.Attempts, value), Unknown: timing.Known == 0, Series: []UsageSeries{{Class: class, Width: percentage(timing.Seconds, maximum)}}})
	}
	panel.Scale = scaleLabels(maximum, durationText)
	return panel
}

func costPanel(report Report) UsagePanel {
	panel := UsagePanel{Title: "Reported cost", Unit: "USD", Caption: "Reported model cost only. Missing billing stays unknown; partial subtotals are gray. Compute and human costs are not inferred.", Legend: []UsageSeries{{Label: "reported", Class: "cost"}}}
	maximum := 0.0
	for _, stage := range report.Stages {
		maximum = math.Max(maximum, stage.Metrics.Cost)
	}
	for _, stage := range report.Stages {
		if stage.Metrics.Expected == 0 {
			continue
		}
		class := "cost"
		if stage.Metrics.CostKnown != stage.Metrics.Expected {
			class = "partial"
		}
		panel.Rows = append(panel.Rows, UsageRow{Label: stage.Definition.Name, Value: stage.Metrics.CostText(), Unknown: stage.Metrics.CostKnown == 0,
			Detail: fmt.Sprintf("%s · %d/%d model attempts reported cost", stage.Definition.Name, stage.Metrics.CostKnown, stage.Metrics.Expected), Series: []UsageSeries{{Class: class, Width: percentage(stage.Metrics.Cost, maximum)}}})
	}
	panel.Scale = scaleLabels(maximum, func(value float64) string { return fmt.Sprintf("$%.4f", value) })
	return panel
}

func scaleLabels(maximum float64, format func(float64) string) []string {
	if maximum <= 0 {
		return nil
	}
	return []string{format(0), format(maximum / 2), format(maximum)}
}
func compactNumber(value float64) string {
	switch {
	case value >= 1e6:
		return fmt.Sprintf("%.2fM", value/1e6)
	case value >= 1e3:
		return fmt.Sprintf("%.1fk", value/1e3)
	default:
		return fmt.Sprintf("%.0f", value)
	}
}

func buildAttemptUsagePlot(attempts []Attempt) AttemptUsagePlot {
	plot := AttemptUsagePlot{}
	maximum := 0.0
	for _, attempt := range attempts {
		if attempt.Metrics.Total.Expected == 0 {
			continue
		}
		maximum = math.Max(maximum, float64(attempt.Metrics.Total.Value))
		plot.Points = append(plot.Points, AttemptUsagePoint{Stage: attempt.Step, Sequence: attempt.Sequence, Attempt: attempt.Attempt, Value: attempt.Metrics.Total.String(), Unknown: attempt.Metrics.Total.Known == 0, Partial: !complete(attempt.Metrics.Total), Detail: fmt.Sprintf("%s · attempt %d · %s tokens · %s", attempt.Step, attempt.Attempt, attempt.Metrics.Total.String(), attempt.Outcome)})
	}
	plot.Width = max(900, len(plot.Points)*115+80)
	if len(plot.Points) == 0 {
		return plot
	}
	slot := float64(plot.Width-80) / float64(len(plot.Points))
	index := 0
	for _, attempt := range attempts {
		if attempt.Metrics.Total.Expected == 0 {
			continue
		}
		point := &plot.Points[index]
		point.Width = math.Min(64, slot*0.65)
		point.Center = 60 + slot*(float64(index)+0.5)
		point.X = point.Center - point.Width/2
		height := percentage(float64(attempt.Metrics.Total.Value), maximum) * 1.7
		point.Y = 210 - height
		point.InputHeight = height
		if !point.Partial {
			point.InputHeight = percentage(float64(attempt.Metrics.Input.Value), maximum) * 1.7
			point.OutputHeight = percentage(float64(attempt.Metrics.Output.Value), maximum) * 1.7
		}
		point.InputY = 210 - point.InputHeight
		index++
	}
	if maximum > 0 {
		for i := 0; i <= 4; i++ {
			plot.Ticks = append(plot.Ticks, PlotTick{Label: compactNumber(maximum * float64(i) / 4), Y: 210 - float64(i)*42.5})
		}
	}
	return plot
}
