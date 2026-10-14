package workerstate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func retained(t *testing.T, store Store, project, task, run, attempt string) *Lease {
	t.Helper()
	lease, err := store.Begin(project, task, run, attempt)
	require.NoError(t, err)
	root := store.AttemptDir(project, task, run, attempt)
	require.NoError(t, os.MkdirAll(root, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(root, "candidate.txt"), []byte(attempt), 0600))
	require.NoError(t, lease.RetainEvidence())
	return lease
}
func TestRetentionKeepsEveryLatestAttemptAndOtherTasks(t *testing.T) {
	store := Store{Root: t.TempDir()}
	project := t.TempDir()
	task := "tasks/a"
	durable := filepath.Join(project, task, "SPEC.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(durable), 0755))
	require.NoError(t, os.WriteFile(durable, []byte("durable document"), 0644))
	retained(t, store, project, task, "old", "build-1").Close()
	require.NoError(t, store.Complete(project, task, "old"))
	retained(t, store, project, task, "new", "build-1").Close()
	retained(t, store, project, task, "new", "build-2").Close()
	require.NoError(t, store.Complete(project, task, "new"))
	retained(t, store, project, "tasks/b", "other", "build-1").Close()
	require.NoError(t, store.Complete(project, "tasks/b", "other"))
	require.NoError(t, store.Prune(project, task, KeepAll))
	require.DirExists(t, store.ExecutionDir(project, task, "old"))
	restarted := Store{Root: store.Root}
	require.NoError(t, restarted.PruneProject(project, LastRun))
	require.NoError(t, restarted.PruneProject(project, LastRun))
	require.NoDirExists(t, store.ExecutionDir(project, task, "old"))
	require.FileExists(t, filepath.Join(store.AttemptDir(project, task, "new", "build-1"), "candidate.txt"))
	require.FileExists(t, filepath.Join(store.AttemptDir(project, task, "new", "build-2"), "candidate.txt"))
	require.DirExists(t, store.ExecutionDir(project, "tasks/b", "other"))
	contents, err := os.ReadFile(durable)
	require.NoError(t, err)
	require.Equal(t, "durable document", string(contents))
}
func TestActiveIncompleteAndUnknownExecutionsAreProtected(t *testing.T) {
	store := Store{Root: t.TempDir()}
	project := t.TempDir()
	task := "tasks/a"
	active := retained(t, store, project, task, "active", "build-1")
	defer active.Close()
	require.NoError(t, store.Complete(project, task, "active"))
	incomplete, err := store.Begin(project, task, "incomplete", "build-1")
	require.NoError(t, err)
	incomplete.Close()
	require.NoError(t, store.Complete(project, task, "incomplete"))
	retained(t, store, project, task, "waiting", "build-1").Close() // nonterminal approval/retry wait
	retained(t, store, project, task, "corrupt", "build-1").Close()
	require.NoError(t, os.WriteFile(filepath.Join(store.ExecutionDir(project, task, "corrupt"), "execution.json"), []byte("broken"), 0600))
	retained(t, store, project, task, "new", "build-1").Close()
	require.NoError(t, store.Complete(project, task, "new"))
	require.NoError(t, store.Prune(project, task, LastRun))
	for _, run := range []string{"active", "incomplete", "waiting", "corrupt", "new"} {
		require.DirExists(t, store.ExecutionDir(project, task, run), run)
	}
}
func TestRestartDoesNotReorderOlderExecutions(t *testing.T) {
	store := Store{Root: t.TempDir()}
	project := t.TempDir()
	task := "tasks/a"
	retained(t, store, project, task, "old", "build-1").Close()
	require.NoError(t, store.Complete(project, task, "old"))
	retained(t, store, project, task, "new", "build-1").Close()
	require.NoError(t, store.Complete(project, task, "new"))
	retained(t, store, project, task, "old", "build-2").Close()
	require.NoError(t, store.Prune(project, task, LastRun))
	require.NoDirExists(t, store.ExecutionDir(project, task, "old"))
	require.DirExists(t, store.ExecutionDir(project, task, "new"))
}
func TestRuntimeSymlinksCannotRedirectCleanup(t *testing.T) {
	store := Store{Root: t.TempDir()}
	protected := t.TempDir()
	sentinel := filepath.Join(protected, "SPEC.md")
	require.NoError(t, os.WriteFile(sentinel, []byte("keep"), 0644))
	require.NoError(t, os.Symlink(protected, filepath.Join(store.Root, "runtime")))
	require.ErrorContains(t, store.PruneProject(t.TempDir(), LastRun), "symlink")
	_, err := store.Begin(t.TempDir(), "task", "run", "attempt")
	require.ErrorContains(t, err, "symlink")
	require.FileExists(t, sentinel)
}
func TestProjectsAndConcurrentTaskLeasesDoNotShareState(t *testing.T) {
	store := Store{Root: t.TempDir()}
	a, b := t.TempDir(), t.TempDir()
	first := retained(t, store, a, "tasks/a", "run", "build-1")
	defer first.Close()
	second := retained(t, store, a, "tasks/b", "run", "build-1")
	defer second.Close()
	third := retained(t, store, b, "tasks/a", "run", "build-1")
	defer third.Close()
	_, err := store.Begin(a, "tasks/a", "run", "build-2")
	require.Error(t, err)
	require.NotEqual(t, store.ExecutionDir(a, "tasks/a", "run"), store.ExecutionDir(b, "tasks/a", "run"))
}

func TestLostCounterRecoversExecutionOrder(t *testing.T) {
	store := Store{Root: t.TempDir()}
	project := t.TempDir()
	task := "tasks/a"
	for _, run := range []string{"first", "second"} {
		retained(t, store, project, task, run, "build-1").Close()
		require.NoError(t, store.Complete(project, task, run))
	}
	require.NoError(t, os.Remove(filepath.Join(store.TaskDir(project, task), "ordinal.json")))
	retained(t, store, project, task, "third", "build-1").Close()
	require.NoError(t, store.Complete(project, task, "third"))
	require.NoError(t, store.Prune(project, task, LastRun))
	require.NoDirExists(t, store.ExecutionDir(project, task, "first"))
	require.NoDirExists(t, store.ExecutionDir(project, task, "second"))
	require.DirExists(t, store.ExecutionDir(project, task, "third"))
}
