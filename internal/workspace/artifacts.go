package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jasondeutsch/watts/internal/orchestration"
)

func HashTaskArtifacts(projectRoot, taskPath string, paths []string) ([]orchestration.Artifact, error) {
	root := filepath.Join(projectRoot, filepath.FromSlash(taskPath))
	var result []orchestration.Artifact
	for _, artifactPath := range paths {
		if !orchestration.ValidArtifact(artifactPath) {
			return nil, fmt.Errorf("invalid artifact path %q", artifactPath)
		}
		resolved, err := filepath.EvalSymlinks(filepath.Join(root, filepath.FromSlash(artifactPath)))
		if err != nil {
			return nil, fmt.Errorf("artifact %s: %w", artifactPath, err)
		}
		inside, err := filepath.Rel(root, resolved)
		if err != nil || inside == ".." || strings.HasPrefix(inside, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("artifact %s escapes the task folder", artifactPath)
		}
		taskState, err := os.Stat(resolved)
		if err != nil {
			return nil, err
		}
		if !taskState.Mode().IsRegular() {
			return nil, fmt.Errorf("artifact %s must be a regular file", artifactPath)
		}
		hash, err := hashFile(resolved)
		if err != nil {
			return nil, err
		}
		result = append(result, orchestration.Artifact{Path: artifactPath, SHA256: hash})
	}
	return result, nil
}

func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
