// Package taskreport renders a portable view of recorded workflow execution.
package taskreport

import (
	"crypto/sha256"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"math"
	"os"
	"path"
	"sort"
	"time"

	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/jasondeutsch/watts/internal/workload"
)

//go:embed report.html
var templates embed.FS

var page = template.Must(template.New("report.html").Funcs(template.FuncMap{
	"timestamp": func(t time.Time) string {
		if t.IsZero() {
			return "Unknown"
		}
		return t.UTC().Format(time.RFC3339Nano)
	},
	"elapsed": func(start, end time.Time) string {
		if start.IsZero() || end.IsZero() {
			return "Unknown"
		}
		return end.Sub(start).Round(time.Millisecond).String()
	},
}).ParseFS(templates, "report.html"))

// Counter keeps a reported subtotal while exposing missing observations.
type Counter struct {
	Value           int64
	Known, Expected int
}

func (c Counter) String() string {
	if c.Expected == 0 {
		return "N/A"
	}
	if c.Known == 0 {
		return "Unknown"
	}
	if c.Known != c.Expected {
		return fmt.Sprintf("%s (partial %d/%d)", integerText(c.Value), c.Known, c.Expected)
	}
	return integerText(c.Value)
}
func (c *Counter) add(value *int64) {
	c.Expected++
	if value != nil && *value >= 0 && *value <= math.MaxInt64-c.Value {
		c.Value += *value
		c.Known++
	}
}

type Metrics struct {
	Input, Output, CacheRead, CacheWrite, Total Counter
	Cost                                        float64
	CostKnown, Expected                         int
}

func (m *Metrics) add(u *workload.Usage) {
	if u == nil {
		u = &workload.Usage{}
	}
	m.Input.add(u.Input)
	m.Output.add(u.Output)
	m.CacheRead.add(u.CacheRead)
	m.CacheWrite.add(u.CacheWrite)
	var total *int64
	if u.Input != nil && u.Output != nil && *u.Input >= 0 && *u.Output >= 0 && *u.Input <= math.MaxInt64-*u.Output {
		n := *u.Input + *u.Output
		total = &n
	}
	m.Total.add(total)
	m.Expected++
	if u.Cost != nil && *u.Cost >= 0 && !math.IsInf(*u.Cost, 0) && !math.IsNaN(*u.Cost) && !math.IsInf(m.Cost+*u.Cost, 0) {
		m.Cost += *u.Cost
		m.CostKnown++
	}
}
func (m Metrics) CostText() string {
	if m.Expected == 0 {
		return "N/A"
	}
	if m.CostKnown == 0 {
		return "Unknown"
	}
	if m.CostKnown != m.Expected {
		return fmt.Sprintf("$%.6f (partial %d/%d)", m.Cost, m.CostKnown, m.Expected)
	}
	return fmt.Sprintf("$%.6f", m.Cost)
}

type Attempt struct {
	orchestration.StageResult
	Sequence     int
	Metrics      Metrics
	Models       []string
	Warnings     []string
	Gates        []workload.GateResult
	Measurements []workload.Measurement
	Evidence     string
	Log          string
}
type Stage struct {
	Definition orchestration.Step
	Attempts   int
	Metrics    Metrics
}
type Report struct {
	orchestration.ReportInput
	DefinitionSHA256 string
	Attempts         []Attempt
	Stages           []Stage
	Metrics          Metrics
	Failures         int
	Rework           int
	Dashboard        Dashboard
}

// Render reads only selected evidence files, never environment values, prompts,
// raw logs or event messages. Configured capabilities are distinct from usage.
func Render(w io.Writer, evidence *os.Root, request orchestration.ReportInput) error {
	definition, err := json.Marshal(request.Input.Definition)
	if err != nil {
		return fmt.Errorf("workflow fingerprint: %w", err)
	}
	report := Report{ReportInput: request, DefinitionSHA256: fmt.Sprintf("%x", sha256.Sum256(definition))}
	indices := map[string]int{}
	for _, step := range request.Input.Definition.Steps {
		indices[step.Name] = len(report.Stages)
		report.Stages = append(report.Stages, Stage{Definition: step})
	}
	for i, record := range request.State.History {
		index, exists := indices[record.Step]
		if !exists || !orchestration.ValidName(record.Step) || record.Attempt < 1 {
			return fmt.Errorf("invalid attempt %s/%d", record.Step, record.Attempt)
		}
		stage := &report.Stages[index]
		stage.Attempts++
		if stage.Attempts > 1 {
			report.Rework++
		}
		attempt := Attempt{StageResult: record, Sequence: i + 1}
		if record.Error != "" || record.Outcome == "failure" {
			report.Failures++
		}
		directory := fmt.Sprintf(".watts-state/steps/%s-%d", record.Step, record.Attempt)
		attempt.Evidence = retainedLink(evidence, directory+"/result.json")
		attempt.Log = retainedLink(evidence, fmt.Sprintf(".watts-state/logs/%s-%d.log", record.Step, record.Attempt))
		if stage.Definition.Agent != "" {
			var usage *workload.Usage
			if err := readJSON(evidence, directory+"/usage.json", &usage); err != nil {
				attempt.Warnings = append(attempt.Warnings, "Usage evidence unavailable: "+err.Error())
			}
			if usage == nil {
				attempt.Warnings = append(attempt.Warnings, "Agent usage was not reported; totals may be incomplete.")
			}
			attempt.Metrics.add(usage)
			stage.Metrics.add(usage)
			report.Metrics.add(usage)
		}
		if stage.Definition.Agent != "" || stage.Definition.Container != nil {
			var events []workload.Event
			if err := readJSON(evidence, directory+"/events.json", &events); err != nil {
				attempt.Warnings = append(attempt.Warnings, "Workload observations unavailable: "+err.Error())
			} else {
				models := map[string]bool{}
				for _, event := range events {
					if event.Type == "usage" && event.Usage != nil && event.Usage.Model != "" {
						models[event.Usage.Provider+" / "+event.Usage.Model] = true
					}
					if event.Type == "measurement" {
						measurement, err := workload.ParseMeasurement(event)
						if err != nil {
							attempt.Warnings = append(attempt.Warnings, "Workload measurement unavailable: "+err.Error())
						} else {
							attempt.Measurements = append(attempt.Measurements, measurement)
						}
					}
				}
				for model := range models {
					attempt.Models = append(attempt.Models, model)
				}
				sort.Strings(attempt.Models)
			}
		}
		if len(stage.Definition.Gates) > 0 {
			var gates workload.GateReport
			if err := readJSON(evidence, directory+"/workload-gates.json", &gates); err != nil {
				attempt.Warnings = append(attempt.Warnings, "Gate evidence unavailable: "+err.Error())
			} else {
				attempt.Gates = gates.Results
				for j := range attempt.Gates {
					if workload.Relative(attempt.Gates[j].Evidence) {
						attempt.Gates[j].Evidence = path.Join(directory, attempt.Gates[j].Evidence)
					} else {
						attempt.Gates[j].Evidence = ""
					}
				}
			}
		}
		// Artifact links only address task-local files; unsafe paths stay visible as text.
		for j := range attempt.Artifacts {
			if !orchestration.ValidArtifact(attempt.Artifacts[j].Path) {
				attempt.Artifacts[j].Path = ""
			}
		}
		report.Attempts = append(report.Attempts, attempt)
	}
	report.Dashboard = buildDashboard(report, evidence)
	return page.Execute(w, report)
}

func readJSON(root *os.Root, name string, target any) error {
	file, err := root.Open(name)
	if err != nil {
		return fmt.Errorf("%s: unavailable", path.Base(name))
	}
	defer file.Close()
	const limit = 16 << 20
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil || len(data) > limit {
		return fmt.Errorf("%s: unreadable or too large", path.Base(name))
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("%s: invalid JSON", path.Base(name))
	}
	return nil
}

func retainedLink(root *os.Root, name string) string {
	info, err := root.Lstat(name)
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}
	return name
}
