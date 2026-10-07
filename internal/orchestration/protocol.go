package orchestration

const (
	WorkflowName     = "watts.workflow.v1"
	ExecuteActivity  = "watts.execute.v1"
	HashActivity     = "watts.hash.v1"
	ValidateActivity = "watts.validate-approval.v1"
	StatusQuery      = "status"
	DecisionUpdate   = "decision"
	RetryUpdate      = "retry"
	EventUpdate      = "event"
)

type Artifact struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}
type Input struct {
	Task         string     `json:"task"`
	Definition   Definition `json:"definition"`
	ConfigSHA256 string     `json:"config_sha256"`
	Offline      bool       `json:"offline,omitempty"`
	Pass         []string   `json:"pass,omitempty"`
}
type ActivityInput struct {
	Input     Input
	Step      Step
	Attempt   int
	Artifacts []Artifact
	Decision  *Decision    `json:"decision,omitempty"`
	Previous  *StageResult `json:"previous,omitempty"`
}
type Result struct {
	Artifacts []Artifact     `json:"artifacts,omitempty"`
	Outcome   string         `json:"outcome,omitempty"`
	Feedback  string         `json:"feedback,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
}
type Stage struct {
	Name       string     `json:"name"`
	Status     string     `json:"status"`
	Attempts   int        `json:"attempts"`
	LastError  string     `json:"last_error,omitempty"`
	Artifacts  []Artifact `json:"artifacts,omitempty"`
	ApprovedBy string     `json:"approved_by,omitempty"`
	Outcome    string     `json:"outcome,omitempty"`
}
type State struct {
	Status      string        `json:"status"`
	Current     string        `json:"current,omitempty"`
	Stages      []Stage       `json:"stages"`
	Transitions int           `json:"transitions"`
	History     []StageResult `json:"history,omitempty"`
}
type Decision struct {
	Step      string     `json:"step"`
	Attempt   int        `json:"attempt"`
	By        string     `json:"by"`
	Feedback  string     `json:"feedback,omitempty"`
	Reject    bool       `json:"reject,omitempty"`
	Artifacts []Artifact `json:"artifacts"`
}
type Retry struct {
	Step    string
	Attempt int
	Force   bool
}

func sameArtifacts(a, b []Artifact) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// StageResult retains every attempt, including rework, and supplies context to the next stage.
type StageResult struct {
	Step    string `json:"step"`
	Attempt int    `json:"attempt"`
	Result
	Error string `json:"error,omitempty"`
}

// Event is bound to one waiting stage attempt. It cannot jump directly to an arbitrary stage.
type Event struct {
	ID       string         `json:"id"`
	Step     string         `json:"step"`
	Attempt  int            `json:"attempt"`
	Outcome  string         `json:"outcome"`
	Feedback string         `json:"feedback,omitempty"`
	Data     map[string]any `json:"data,omitempty"`
}
