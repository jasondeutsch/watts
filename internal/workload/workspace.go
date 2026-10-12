package workload

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type File struct {
	SHA256 string `json:"sha256"`
	Mode   uint32 `json:"mode"`
}

func excluded(name string, excludes []string) bool {
	for _, entry := range excludes {
		if name == entry || strings.HasPrefix(name, entry+"/") {
			return true
		}
	}
	return false
}
func Snapshot(root string, excludes []string) (map[string]File, error) {
	info, err := os.Lstat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("workspace root must be a directory, not a symlink")
	}
	source, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer source.Close()
	return snapshotRoot(source, excludes)
}
func snapshotRoot(source *os.Root, excludes []string) (map[string]File, error) {
	files := map[string]File{}
	err := fs.WalkDir(source.FS(), ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if name == "." {
			return nil
		}
		if excluded(name, excludes) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("workspace symlink %s is unsupported; use regular files", name)
		}
		if entry.IsDir() {
			return nil
		}
		data, mode, err := readRootFile(source, name, MaximumFileBytes)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		files[name] = File{SHA256: hex.EncodeToString(sum[:]), Mode: uint32(mode)}
		return nil
	})
	return files, err
}
func Copy(source, target string, excludes []string) (map[string]File, error) {
	files, err := Snapshot(source, excludes)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(target, 0700); err != nil {
		return nil, err
	}
	src, err := os.OpenRoot(source)
	if err != nil {
		return nil, err
	}
	defer src.Close()
	dst, err := os.OpenRoot(target)
	if err != nil {
		return nil, err
	}
	defer dst.Close()
	for name, file := range files {
		data, err := readRootData(src, name)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != file.SHA256 {
			return nil, fmt.Errorf("workspace changed while copying %s", name)
		}
		if err = dst.MkdirAll(filepath.Dir(name), 0755); err != nil {
			return nil, err
		}
		if err = dst.WriteFile(name, data, os.FileMode(file.Mode)); err != nil {
			return nil, err
		}
	}
	return files, nil
}

func ResourceHash(project, name string) (string, error) {
	if !Relative(filepath.ToSlash(name)) {
		return "", fmt.Errorf("invalid resource path %q", name)
	}
	root, err := os.OpenRoot(project)
	if err != nil {
		return "", err
	}
	defer root.Close()
	info, err := root.Lstat(name)
	if err != nil {
		return "", err
	}
	var descriptor any
	if info.Mode().IsRegular() {
		data, mode, err := readRootFile(root, name, MaximumFileBytes)
		if err != nil {
			return "", err
		}
		sum := sha256.Sum256(data)
		descriptor = File{SHA256: hex.EncodeToString(sum[:]), Mode: uint32(mode)}
	} else if info.IsDir() {
		source, err := root.OpenRoot(name)
		if err != nil {
			return "", err
		}
		defer source.Close()
		files, err := snapshotRoot(source, nil)
		if err != nil {
			return "", err
		}
		descriptor = files
	} else {
		return "", fmt.Errorf("resource %s must be a regular file or directory", name)
	}
	data, err := json.Marshal(descriptor)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// Publish applies a candidate only when each destination still matches its
// baseline or already matches this candidate (idempotent interrupted publication).
// Caller holds the project lock; external edits remain explicit conflicts.
func Publish(project, candidate string, baseline map[string]File, excludes []string) error {
	after, err := Snapshot(candidate, nil)
	if err != nil {
		return err
	}
	beforeApply, err := Snapshot(project, excludes)
	if err != nil {
		return err
	}
	names := map[string]bool{}
	for name := range baseline {
		names[name] = true
	}
	for name := range after {
		names[name] = true
	}
	var changed []string
	for name := range names {
		if excluded(name, excludes) {
			return fmt.Errorf("candidate changed excluded path %s", name)
		}
		if baseline[name] == after[name] {
			continue
		}
		if beforeApply[name] != baseline[name] && beforeApply[name] != after[name] {
			return fmt.Errorf("workspace conflict at %s; candidate retained for inspection", name)
		}
		changed = append(changed, name)
	}
	sort.Strings(changed)
	source, err := os.OpenRoot(candidate)
	if err != nil {
		return err
	}
	defer source.Close()
	destination, err := os.OpenRoot(project)
	if err != nil {
		return err
	}
	defer destination.Close()
	for _, name := range changed {
		current, currentMode, readErr := readRootFile(destination, name, MaximumFileBytes)
		if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
			return readErr
		}
		hash := ""
		if readErr == nil {
			sum := sha256.Sum256(current)
			hash = hex.EncodeToString(sum[:])
		}
		currentFile := File{}
		if readErr == nil {
			currentFile = File{SHA256: hash, Mode: uint32(currentMode)}
		}
		if currentFile != baseline[name] && currentFile != after[name] {
			return fmt.Errorf("workspace changed during publication at %s", name)
		}
		file, exists := after[name]
		if !exists {
			if err = destination.Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			continue
		}
		data, mode, err := readRootFile(source, name, MaximumFileBytes)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != file.SHA256 || uint32(mode) != file.Mode {
			return fmt.Errorf("candidate changed during publication at %s", name)
		}
		if err = destination.MkdirAll(filepath.Dir(name), 0755); err != nil {
			return err
		}
		if err = writeRootFile(destination, name, data, os.FileMode(file.Mode)); err != nil {
			return err
		}
	}
	return nil
}

func readRootData(root *os.Root, name string) ([]byte, error) {
	data, _, err := readRootFile(root, name, MaximumFileBytes)
	return data, err
}

// CopyResource captures a file or directory relative to a trusted project root.
// OpenRoot prevents parent symlinks from exposing files outside the project.
func CopyResource(project, name, target string) error {
	if !Relative(filepath.ToSlash(name)) {
		return fmt.Errorf("invalid resource path %q", name)
	}
	root, err := os.OpenRoot(project)
	if err != nil {
		return err
	}
	defer root.Close()
	info, err := root.Lstat(name)
	if err != nil {
		return err
	}
	if info.Mode().IsRegular() {
		data, mode, err := readRootFile(root, name, MaximumFileBytes)
		if err != nil {
			return err
		}
		if err = os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return err
		}
		return WriteFile(filepath.Dir(target), filepath.Base(target), data, mode)
	}
	if !info.IsDir() {
		return fmt.Errorf("resource %s must be a regular file or directory", name)
	}
	source, err := root.OpenRoot(name)
	if err != nil {
		return err
	}
	defer source.Close()
	files, err := snapshotRoot(source, nil)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(target, 0700); err != nil {
		return err
	}
	destination, err := os.OpenRoot(target)
	if err != nil {
		return err
	}
	defer destination.Close()
	for name, file := range files {
		data, mode, err := readRootFile(source, name, MaximumFileBytes)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != file.SHA256 || uint32(mode) != file.Mode {
			return fmt.Errorf("resource changed while capturing %s", name)
		}
		if err = writeRootFile(destination, name, data, os.FileMode(file.Mode)); err != nil {
			return err
		}
	}
	return nil
}
