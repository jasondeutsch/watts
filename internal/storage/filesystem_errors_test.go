package storage

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHostFilesystemDiagnosticsRetainOperationAndCause(t *testing.T) {
	for _, test := range []struct {
		cause    error
		category string
	}{
		{syscall.EROFS, "read-only filesystem"}, {syscall.EACCES, "permission denied"},
		{os.ErrNotExist, "path not found"}, {syscall.ENOTDIR, "not a directory"},
	} {
		t.Run(test.category, func(t *testing.T) {
			original := fmt.Errorf("prepare attempt: %w", &os.PathError{Op: "mkdir", Path: "/home/person/.watts/runtime/private", Err: test.cause})
			annotated := DescribeHostError(original)
			require.ErrorContains(t, annotated, "host filesystem: mkdir")
			require.ErrorContains(t, annotated, test.category)
			require.ErrorContains(t, annotated, "/home/person/.watts/runtime/private")
			require.ErrorIs(t, annotated, test.cause)
			var pathError *os.PathError
			require.ErrorAs(t, annotated, &pathError)
			require.Equal(t, annotated, DescribeHostError(annotated), "do not duplicate diagnostics")
		})
	}
	link := &os.LinkError{Op: "rename", Old: "/home/person/.watts/config/.state", New: "/home/person/.watts/config/settings.json", Err: syscall.EROFS}
	require.ErrorContains(t, DescribeHostError(link), "host filesystem: rename")
	require.ErrorContains(t, DescribeHostError(link), "read-only filesystem")
	require.ErrorIs(t, DescribeHostError(link), syscall.EROFS)
	require.Nil(t, DescribeHostError(nil))
	container := errors.New("container main exited 1: mkdir /runtime/task: Read-only file system")
	require.Same(t, container, DescribeHostError(container), "do not infer host context from workload text")
}
