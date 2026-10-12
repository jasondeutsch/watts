package workload

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadFileRejectsEscapesAndOversizedArtifacts(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "secret"), []byte("private"), 0600))
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "parent")))
	require.NoError(t, os.Symlink(filepath.Join(outside, "secret"), filepath.Join(root, "direct")))
	for _, name := range []string{"../secret", "direct", "parent/secret"} {
		_, _, err := ReadFile(root, name, 1024)
		require.Error(t, err, name)
	}
	require.NoError(t, os.WriteFile(filepath.Join(root, "large"), []byte("12345"), 0600))
	_, _, err := ReadFile(root, "large", 4)
	require.Error(t, err)
	data, _, err := ReadFile(root, "large", 5)
	require.NoError(t, err)
	require.Equal(t, "12345", string(data))
}

func TestPublishDetectsExternalEditsAndAllowsRecovery(t *testing.T) {
	project, candidate := t.TempDir(), t.TempDir()
	name := filepath.Join(project, "main.go")
	require.NoError(t, os.WriteFile(name, []byte("baseline"), 0644))
	baseline, err := Copy(project, candidate, nil)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(candidate, "main.go"), []byte("candidate"), 0644))
	require.NoError(t, os.WriteFile(name, []byte("human edit"), 0644))
	require.ErrorContains(t, Publish(project, candidate, baseline, nil), "conflict")
	data, err := os.ReadFile(name)
	require.NoError(t, err)
	require.Equal(t, "human edit", string(data))
	require.NoError(t, os.WriteFile(name, []byte("baseline"), 0644))
	require.NoError(t, Publish(project, candidate, baseline, nil))
	require.NoError(t, Publish(project, candidate, baseline, nil))
}

func TestPublishRejectsSymlinksAndExcludedOutputs(t *testing.T) {
	project, candidate := t.TempDir(), t.TempDir()
	require.NoError(t, os.Symlink(t.TempDir(), filepath.Join(candidate, "escape")))
	require.ErrorContains(t, Publish(project, candidate, nil, nil), "symlink")
	require.NoError(t, os.Remove(filepath.Join(candidate, "escape")))
	require.NoError(t, os.WriteFile(filepath.Join(candidate, "watts.yaml"), []byte("changed"), 0644))
	require.ErrorContains(t, Publish(project, candidate, nil, []string{"watts.yaml"}), "excluded")
}

func TestSnapshotRejectsSymlinkRoot(t *testing.T) {
	link := filepath.Join(t.TempDir(), "linked-root")
	require.NoError(t, os.Symlink(t.TempDir(), link))
	_, err := Snapshot(link, nil)
	require.ErrorContains(t, err, "symlink")
}
