package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jasondeutsch/watts/internal/orchestration"
)

// TaskArtifactOutput creates task-local parent directories without following symlinks.
func TaskArtifactOutput(projectRoot, taskPath, artifact string) (string, error) {
	if !orchestration.ValidArtifact(artifact) {
		return "", fmt.Errorf("invalid artifact path %q", artifact)
	}
	current := filepath.Join(projectRoot, filepath.FromSlash(taskPath))
	components := strings.Split(artifact, "/")
	for index, component := range components {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			if index < len(components)-1 {
				if err := os.Mkdir(current, 0755); err != nil {
					return "", err
				}
			}
			continue
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("artifact output %s must not follow a symlink", artifact)
		}
		if index < len(components)-1 && !info.IsDir() {
			return "", fmt.Errorf("artifact parent %s is not a directory", current)
		}
	}
	return current, nil
}
