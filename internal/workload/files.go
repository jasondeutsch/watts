package workload

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const MaximumFileBytes = 64 << 20

// ReadFile confines an untrusted artifact to its mounted directory, including
// symlinks in parent components. Missing files remain distinguishable errors.
func ReadFile(directory, name string, limit int64) ([]byte, os.FileMode, error) {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, 0, err
	}
	defer root.Close()
	return readRootFile(root, name, limit)
}

func readRootFile(root *os.Root, name string, limit int64) ([]byte, os.FileMode, error) {
	if !Relative(filepath.ToSlash(name)) || limit <= 0 {
		return nil, 0, fmt.Errorf("invalid bounded file read %q", name)
	}
	info, err := root.Lstat(name)
	if err != nil {
		return nil, 0, err
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, 0, fmt.Errorf("%s must be a regular file of at most %d bytes", name, limit)
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, 0, err
	}
	defer file.Close()
	info, err = file.Stat()
	if err != nil {
		return nil, 0, err
	}
	if !info.Mode().IsRegular() {
		return nil, 0, fmt.Errorf("%s is not a regular file", name)
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, 0, err
	}
	if int64(len(data)) > limit {
		return nil, 0, fmt.Errorf("%s exceeds %d bytes", name, limit)
	}
	return data, info.Mode().Perm(), nil
}

// WriteFile publishes through a unique, exclusively created temporary file.
// Root confines both creation and rename; a failed write never truncates a target.
func WriteFile(directory, name string, data []byte, mode os.FileMode) error {
	if !Relative(filepath.ToSlash(name)) {
		return fmt.Errorf("invalid output path %q", name)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return err
	}
	defer root.Close()
	return writeRootFile(root, name, data, mode)
}
func writeRootFile(root *os.Root, name string, data []byte, mode os.FileMode) error {
	parent := filepath.Dir(name)
	if err := root.MkdirAll(parent, 0755); err != nil {
		return err
	}
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	temporary := filepath.Join(parent, ".watts-publish-"+hex.EncodeToString(nonce))
	file, err := root.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode.Perm())
	if err != nil {
		return err
	}
	defer root.Remove(temporary)
	_, writeErr := file.Write(data)
	if writeErr == nil {
		writeErr = file.Chmod(mode.Perm())
	}
	if writeErr == nil {
		writeErr = file.Sync()
	}
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	return root.Rename(temporary, name)
}
