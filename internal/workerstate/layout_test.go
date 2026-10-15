package workerstate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultUsesHostHomeAndRejectsContainerHome(t *testing.T) {
	for _, home := range []string{"/", "relative-home", "/runtime", "/runtime/task", "/runtime/home", ""} {
		t.Run(home, func(t *testing.T) {
			t.Setenv("HOME", home)
			_, err := Default()
			require.ErrorContains(t, err, "host HOME")
		})
	}
	home := filepath.Join(t.TempDir(), "Organization Home")
	require.NoError(t, os.Mkdir(home, 0700))
	t.Setenv("HOME", home)
	store, err := Default()
	require.NoError(t, err)
	require.Equal(t, filepath.Join(home, ".watts"), store.Root)
	require.NoDirExists(t, store.Root, "resolving paths must not create state")
	require.NoError(t, store.RegisterProject(t.TempDir(), "tasks"))
	require.DirExists(t, filepath.Join(home, ".watts", "runtime", "projects"))
}

func TestHostHomeCannotAliasFilesystemRoot(t *testing.T) {
	alias := filepath.Join(t.TempDir(), "home")
	require.NoError(t, os.Symlink(string(filepath.Separator), alias))
	t.Setenv("HOME", alias)
	_, err := Default()
	require.ErrorContains(t, err, "host HOME")
	require.ErrorContains(t, err, "resolves")
}
