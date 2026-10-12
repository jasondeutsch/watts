package application

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/jasondeutsch/watts/internal/storage"
)

// Local activity evidence lives beside task artifacts. Temporal history is authoritative for
// workflow progress, retries, routing and completion.
const StateDirName = ".watts-state"

func (a *Service) StateDir(taskPath string) string {
	return filepath.Join(a.Root, filepath.FromSlash(taskPath), StateDirName)
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
					snap[a.Rel(p)] = storage.Digest(data)
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
