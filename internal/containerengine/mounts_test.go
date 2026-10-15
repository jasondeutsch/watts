package containerengine

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jasondeutsch/watts/internal/workload"
	"github.com/stretchr/testify/require"
)

func TestMountSourcesRejectContainerPathsBeforeDocker(t *testing.T) {
	root := t.TempDir()
	base := Request{Identity: "fixture", InputDir: root, OutputDir: root, RuntimeDir: root, WorkspaceDir: root, Spec: workload.Spec{Image: "fixture:0.1.0", Command: []string{"true"}}}
	require.NoError(t, base.validateHostMounts())
	for _, invalid := range []string{"/runtime/task", "/inputs", "/outputs/task", "/workspace", "/", "relative"} {
		request := base
		request.RuntimeDir = invalid
		// A nil client proves rejection happens before contacting Docker.
		_, err := (&Engine{}).Run(context.Background(), request)
		require.ErrorContains(t, err, "host runtime mount source")
	}
	request := base
	request.RuntimeDir = filepath.Join(root, "missing")
	require.ErrorIs(t, request.validateHostMounts(), os.ErrNotExist)
	require.NoDirExists(t, request.RuntimeDir, "validation must not create host paths")
	filename := filepath.Join(root, "file")
	require.NoError(t, os.WriteFile(filename, nil, 0600))
	request.RuntimeDir = filename
	require.ErrorContains(t, request.validateHostMounts(), "must be a directory")
}
