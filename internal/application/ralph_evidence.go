package application

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Ralph launches nested Pi processes with --no-session. Its run-token-bound
// terminal event is the evidence contract instead of the normal Pi session.
func (evidence *agentEvidence) ralphEvidence() (bool, error) {
	if len(evidence.args) < 2 || !strings.HasPrefix(strings.TrimSpace(evidence.args[1]), "/ralph --path ") {
		return false, nil
	}
	task := evidence.runtime.request.Input.Task
	path := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(evidence.args[1]), "/ralph --path "))
	path = filepath.Clean(path)
	if path != filepath.Clean(task) && path != filepath.Join(task, "review") {
		return true, fmt.Errorf("Ralph evidence path does not match this task: %s", path)
	}
	root, err := os.OpenRoot(filepath.Join(evidence.runtime.project.Root, path))
	if err != nil {
		return true, err
	}
	defer root.Close()
	data, err := root.ReadFile(".ralph-runner/status.json")
	if err != nil {
		return true, fmt.Errorf("Ralph produced no runner status: %w", err)
	}
	var status struct {
		LoopToken        string    `json:"loopToken"`
		Status           string    `json:"status"`
		StartedAt        time.Time `json:"startedAt"`
		CurrentIteration int       `json:"currentIteration"`
	}
	if err = json.Unmarshal(data, &status); err != nil {
		return true, err
	}
	if status.LoopToken == "" || status.StartedAt.Before(evidence.started.Add(-2*time.Second)) {
		return true, fmt.Errorf("Ralph runner evidence is from an earlier attempt")
	}
	if status.Status != "complete" {
		return true, fmt.Errorf("Ralph runner ended with status %s at iteration %d", status.Status, status.CurrentIteration)
	}
	file, err := root.Open(".ralph-runner/events.jsonl")
	if err != nil {
		return true, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 4<<20)
	for scanner.Scan() {
		var event struct {
			Type      string `json:"type"`
			LoopToken string `json:"loopToken"`
			Status    string `json:"status"`
		}
		if json.Unmarshal(scanner.Bytes(), &event) == nil && event.Type == "runner.finished" && event.LoopToken == status.LoopToken && event.Status == "complete" {
			fmt.Fprintf(evidence.runtime.output, "Ralph run %s completed %d iterations; evidence: %s/.ralph-runner\n", status.LoopToken, status.CurrentIteration, path)
			return true, nil
		}
	}
	if err = scanner.Err(); err != nil {
		return true, err
	}
	return true, fmt.Errorf("Ralph has no successful terminal event for this attempt")
}
