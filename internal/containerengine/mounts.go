package containerengine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jasondeutsch/watts/internal/storage"
)

// validateHostMounts runs before contacting Docker. Bind sources are existing
// host directories; container-only paths are targets, never host destinations.
// Rejecting absent sources also prevents daemon-side directory creation from
// hiding a worker/engine path mismatch.
func (request Request) validateHostMounts() error {
	for _, source := range []struct{ name, path string }{
		{"inputs", request.InputDir}, {"outputs", request.OutputDir},
		{"runtime", request.RuntimeDir}, {"workspace", request.WorkspaceDir},
	} {
		if !filepath.IsAbs(source.path) || filepath.Clean(source.path) == string(filepath.Separator) {
			return fmt.Errorf("host %s mount source %q must be an absolute private directory", source.name, source.path)
		}
		if containerOnlyPath(filepath.Clean(source.path)) {
			return fmt.Errorf("host %s mount source %q is a container-only path; use the attempt directory under the host's $HOME/.watts/runtime", source.name, source.path)
		}
		resolved, err := filepath.EvalSymlinks(source.path)
		if err != nil {
			return storage.DescribeHostError(err)
		}
		if containerOnlyPath(resolved) {
			return fmt.Errorf("host %s mount source %q resolves to container-only path %q", source.name, source.path, resolved)
		}
		info, err := os.Stat(resolved)
		if err != nil {
			return storage.DescribeHostError(err)
		}
		if !info.IsDir() {
			return fmt.Errorf("host %s mount source %q must be a directory", source.name, source.path)
		}
	}
	return nil
}

func containerOnlyPath(directory string) bool {
	for _, target := range []string{"/inputs", "/outputs", "/runtime", "/workspace"} {
		if directory == target || strings.HasPrefix(directory, target+"/") {
			return true
		}
	}
	return false
}
