package application

import (
	"fmt"
	"os"
	"strings"

	"github.com/jasondeutsch/watts/internal/redact"
	"github.com/jasondeutsch/watts/internal/workload"
)

// Bindings contain source names; their values live only in the execution process.
func (runtime *activityRuntime) secretRedaction() *redact.Set {
	var values []string
	for _, source := range runtime.request.Step.SecretEnvironment {
		values = append(values, os.Getenv(source))
	}
	return redact.New(values...)
}

// Published source and task documents must not contain credentials. Rewriting
// them would change the candidate against which gates and approvals were checked.
func (attempt attemptWorkspace) rejectSecretPublication(outputs []string, workspace bool) error {
	roots := map[string][]string{attempt.output: {}}
	for _, name := range outputs {
		roots[attempt.output] = append(roots[attempt.output], "task/"+name)
	}
	if workspace {
		files, err := workload.Snapshot(attempt.candidate, nil)
		if err != nil {
			return err
		}
		for name := range files {
			roots[attempt.candidate] = append(roots[attempt.candidate], name)
		}
	}
	for root, names := range roots {
		for _, name := range names {
			if name != attempt.secrets.Text(name) {
				return fmt.Errorf("publication refused: declared secret in workload artifact name")
			}
			data, _, err := workload.ReadFile(root, name, workload.MaximumFileBytes)
			if err != nil {
				return err
			}
			if string(data) != attempt.secrets.Text(string(data)) {
				return fmt.Errorf("publication refused: declared secret in workload artifact; remove the credential and retry")
			}
		}
	}
	return nil
}

// Ignore runtime state without invoking Git or ignoring authored ACL and reports.
// Existing tracked files and history require separate, explicit incident cleanup.
func (app *Service) ProtectRuntimeFiles() error {
	root, err := os.OpenRoot(app.Root)
	if err != nil {
		return err
	}
	defer root.Close()
	data, mode, err := workload.ReadFile(app.Root, ".gitignore", workload.MaximumFileBytes)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if os.IsNotExist(err) {
		mode = 0644
	}
	const block = "\n# Watts private runtime and task state\n/.watts/\n**/.watts-state/\n"
	if strings.Contains(string(data), block) {
		return nil
	}
	// O_NOFOLLOW through Root prevents escaping the project; reject symlinks even
	// within it to avoid modifying an unrelated authored file.
	if info, err := root.Lstat(".gitignore"); err == nil && !info.Mode().IsRegular() {
		return fmt.Errorf(".gitignore must be a regular file")
	}
	return workload.WriteFile(app.Root, ".gitignore", append(data, []byte(block)...), mode)
}
