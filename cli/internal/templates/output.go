package templates

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/jasondeutsch/watts/internal/render"
)

// Resolve existing ancestors too, so a symlinked parent cannot hide overlap.
func resolvedPath(name string) (string, error) {
	absolute, err := filepath.Abs(name)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if errors.Is(err, fs.ErrNotExist) {
		parent, parentErr := resolvedPath(filepath.Dir(absolute))
		return filepath.Join(parent, filepath.Base(absolute)), parentErr
	}
	return resolved, err
}

func containsPath(directory, name string) bool {
	relative, err := filepath.Rel(directory, name)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func validateOutputLocation(packageDirectory, outputDirectory string, values []string) error {
	output, err := resolvedPath(outputDirectory)
	if err != nil {
		return err
	}
	inputs := append([]string{packageDirectory}, values...)
	for _, input := range inputs {
		source, err := resolvedPath(input)
		if err != nil {
			return err
		}
		if containsPath(output, source) || containsPath(source, output) {
			return fmt.Errorf("generated output must not overlap input %s", input)
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	if containsPath(output, cwd) {
		return fmt.Errorf("generated output must not contain the working directory")
	}
	return nil
}

// Output is a generated directory, not a collection of independently edited
// files. Stage the complete result before replacing it, including stale YAML.
func writeFiles(directory string, files []render.File) error {
	directory = filepath.Clean(directory)
	parent, name := filepath.Dir(directory), filepath.Base(directory)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return err
	}
	root, err := os.OpenRoot(parent)
	if err != nil {
		return err
	}
	defer root.Close()
	exists := false
	if info, err := root.Lstat(name); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("generated output %s must be a directory, not a file or symlink", directory)
		}
		exists = true
		if err := fs.WalkDir(root.FS(), name, func(entryPath string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			if !entry.Type().IsRegular() || (path.Ext(entryPath) != ".yaml" && path.Ext(entryPath) != ".yml") {
				return fmt.Errorf("generated output contains non-YAML file or symlink: %s", entryPath)
			}
			return nil
		}); err != nil {
			return err
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	staging, err := os.MkdirTemp(parent, ".watts-template-")
	if err != nil {
		return err
	}
	stagedName := filepath.Base(staging)
	defer root.RemoveAll(stagedName)
	for _, file := range files {
		if !fs.ValidPath(file.Name) || file.Name == "." {
			return fmt.Errorf("invalid output path %q", file.Name)
		}
		target := path.Join(stagedName, file.Name)
		if err := root.MkdirAll(path.Dir(target), 0755); err != nil {
			return err
		}
		if err := root.WriteFile(target, file.Data, 0644); err != nil {
			return err
		}
	}
	if err := root.Chmod(stagedName, 0755); err != nil {
		return err
	}
	backup := stagedName + "-previous"
	if exists {
		if err := root.Rename(name, backup); err != nil {
			return err
		}
	}
	if err := root.Rename(stagedName, name); err != nil {
		if exists {
			err = errors.Join(err, root.Rename(backup, name))
		}
		return fmt.Errorf("publish generated output: %w", err)
	}
	if exists {
		return root.RemoveAll(backup)
	}
	return nil
}
