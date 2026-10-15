package storage

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"syscall"
)

// HostFilesystemError identifies an OS operation performed by Watts on the host,
// rather than a diagnostic returned by the container process or Docker daemon.
// The original error remains available through errors.Is/As.
type HostFilesystemError struct {
	Operation string
	Path      string
	Category  string
	Cause     error
}

func (err *HostFilesystemError) Error() string {
	return fmt.Sprintf("host filesystem: %s %q (%s): %v", err.Operation, err.Path, err.Category, err.Cause)
}
func (err *HostFilesystemError) Unwrap() error { return err.Cause }

// DescribeHostError annotates actual local path errors only. It does not guess
// context or errno from workload/daemon error strings, or replace an earlier
// diagnostic when errors have been joined or wrapped.
func DescribeHostError(err error) error {
	var described *HostFilesystemError
	if errors.As(err, &described) {
		return err
	}
	var operation, path string
	var cause error
	var pathError *os.PathError
	var linkError *os.LinkError
	switch {
	case errors.As(err, &pathError):
		operation, path, cause = pathError.Op, pathError.Path, pathError.Err
	case errors.As(err, &linkError):
		operation, path, cause = linkError.Op, linkError.Old+" -> "+linkError.New, linkError.Err
	default:
		return err
	}
	category := "filesystem operation failed"
	switch {
	case errors.Is(cause, syscall.EROFS):
		category = "read-only filesystem"
	case errors.Is(cause, fs.ErrPermission):
		category = "permission denied"
	case errors.Is(cause, fs.ErrNotExist):
		category = "path not found"
	case errors.Is(cause, syscall.ENOTDIR):
		category = "not a directory"
	}
	return &HostFilesystemError{Operation: operation, Path: path, Category: category, Cause: err}
}
