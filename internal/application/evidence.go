package application

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/kit"
	"github.com/jasondeutsch/watts/internal/storage"
	"github.com/jasondeutsch/watts/internal/workspace"
)

// Local activity evidence lives beside task artifacts. Temporal history is authoritative for
// workflow progress, retries, routing and completion.
const StateDirName = ".watts-state"

func (a *Service) StateDir(taskPath string) string {
	return filepath.Join(a.Root, filepath.FromSlash(taskPath), StateDirName)
}

type agentEvidence struct {
	runtime *activityRuntime
	agent   projectconfig.ResolvedAgent
	started time.Time
	args    []string
}

func (runtime *activityRuntime) beginAgentEvidence(args []string) (*agentEvidence, error) {
	agent, ok := runtime.config.Agent(runtime.request.Step.Agent)
	if !ok {
		return nil, fmt.Errorf("unknown agent %q", runtime.request.Step.Agent)
	}
	if runtime.request.Step.Identity != "" {
		agent.Identity = runtime.request.Step.Identity
	}
	return &agentEvidence{runtime: runtime, agent: agent, started: time.Now(), args: args}, nil
}

func (evidence *agentEvidence) finish(runErr error) error {
	runtime, project := evidence.runtime, evidence.runtime.project
	ended := time.Now()
	var problems []string
	session, found := project.ReadSession(runtime.config, evidence.agent.Name, evidence.started)
	if found {
		fmt.Fprintf(runtime.output, "\nRun summary from the session record (not from the agent's own report):\n  session: %s\n  models seen: %s\n  assistant messages: %d, tool calls: %d, error entries: %d\n", project.Rel(session.File), strings.Join(session.Models, ", "), session.Assistant, session.ToolCalls, session.Errors)
		if mismatch := ModelMismatch(evidence.agent.Model, session.Models); mismatch != "" {
			problems = append(problems, mismatch)
		}
		if len(session.ErrorMessages) > 0 {
			detail := "agent error: " + strings.Join(session.ErrorMessages, "; ")
			problems = append(problems, detail)
			runErr = errors.Join(runErr, errors.New(detail))
		}
	} else {
		fmt.Fprintf(runtime.output, "\nRun summary: no session record was found for the %s agent since this run started.\n", evidence.agent.Name)
		if handled, err := evidence.ralphEvidence(); handled {
			if err != nil {
				problems = append(problems, err.Error())
			}
		} else if len(evidence.args) > 1 && strings.HasPrefix(strings.TrimSpace(evidence.args[1]), "/") {
			problems = append(problems, "agent command produced no session record; the extension did not provide evidence that the agent ran")
		}
	}
	for _, problem := range problems {
		fmt.Fprintf(runtime.output, "FAILED: %s\n", problem)
	}
	if len(problems) > 0 && runErr == nil {
		runErr = workspace.ExitError{Code: 1, Message: problems[0]}
	}
	if err := evidence.writeProvenance(runErr, session, found, ended); err != nil {
		return errors.Join(runErr, err)
	}
	return runErr
}

func ModelMismatch(want string, seen []string) string {
	if len(seen) == 0 {
		return ""
	}
	match := func(m string) bool {
		return m == want || strings.HasSuffix(m, "/"+want) || strings.HasSuffix(want, "/"+m)
	}
	for _, m := range seen {
		if match(m) {
			return ""
		}
	}
	return fmt.Sprintf("the session record shows model %s, but %s is configured", strings.Join(seen, ", "), want)
}

// writeProvenance records how a run was produced, so a difference between two runs can be
// explained. It holds names and hashes, never secret values.
func (h *agentEvidence) writeProvenance(runErr error, sess SessionInfo, haveSess bool, ended time.Time) error {
	a := h.runtime.project
	request := h.runtime.request
	exit := 0
	if runErr != nil {
		exit = 1
		var ee workspace.ExitError
		if errors.As(runErr, &ee) {
			exit = ee.Code
		}
	}
	var envNames []string
	if entries, err := a.EnvReport(h.runtime.config, h.agent.Name, nil, false); err == nil {
		for _, e := range entries {
			envNames = append(envNames, e.Name)
		}
	}
	rec := map[string]any{
		"stage": request.Step.Name, "attempt": request.Attempt, "agent": h.agent.Name,
		"provider": h.agent.Provider, "model": h.agent.Model, "thinking": h.agent.Thinking, "identity": h.agent.Identity,
		"started_at": h.started.UTC().Format(time.RFC3339), "ended_at": ended.UTC().Format(time.RFC3339),
		"exit_code": exit, "watts_version": a.Release, "pi_version": PiVersion(),
		"pi_args": h.args, "prompt_sha256": kit.Digest([]byte(strings.Join(h.args, "\x00"))),
		"env_names": envNames, "config_sha256": request.Input.ConfigSHA256,
		"limits":          h.runtime.config.Limits,
		"forbidden_paths": h.agent.Forbidden,
	}
	if haveSess {
		rec["session_file"] = a.Rel(sess.File)
		rec["session_models"] = sess.Models
	}
	name := fmt.Sprintf("%s-%d.json", request.Step.Name, request.Attempt)
	return storage.WriteJSONAtomically(filepath.Join(a.StateDir(request.Input.Task), "runs", name), rec)
}

// snapshotPaths hashes every file under the given paths (relative to the repository, globs
// allowed). It works the same whether or not the paths are tracked or ignored by git.
func (a *Service) SnapshotPaths(patterns []string) map[string]string {
	snap := map[string]string{}
	for _, pat := range patterns {
		matches, err := filepath.Glob(filepath.Join(a.Root, filepath.FromSlash(pat)))
		if err != nil {
			continue
		}
		for _, m := range matches {
			_ = filepath.WalkDir(m, func(p string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}
				if data, err := os.ReadFile(p); err == nil {
					snap[a.Rel(p)] = kit.Digest(data)
				}
				return nil
			})
		}
	}
	return snap
}

func DiffSnapshots(before, after map[string]string) []string {
	var changed []string
	for p, h := range after {
		if before[p] != h {
			changed = append(changed, p)
		}
	}
	for p := range before {
		if _, ok := after[p]; !ok {
			changed = append(changed, p)
		}
	}
	sort.Strings(changed)
	return changed
}

func Sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
