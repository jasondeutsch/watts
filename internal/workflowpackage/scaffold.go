package workflowpackage

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	resourcetemplates "github.com/jasondeutsch/watts/resource-templates"
	"gopkg.in/yaml.v3"
)

// Scaffold creates a new authoring package. Existing directories are never changed.
func Scaffold(directory string) (err error) {
	name := filepath.Base(filepath.Clean(directory))
	if !packageName.MatchString(name) || name == "end" {
		return fmt.Errorf("invalid package name %q: use lowercase letters, digits, and hyphens, starting with a letter", name)
	}
	if err := os.MkdirAll(filepath.Dir(directory), 0755); err != nil {
		return err
	}
	if err := os.Mkdir(directory, 0755); err != nil {
		return fmt.Errorf("create package %s (existing directories are never overwritten): %w", directory, err)
	}
	defer func() {
		if err != nil {
			if cleanupErr := os.RemoveAll(directory); cleanupErr != nil {
				err = fmt.Errorf("%w; remove incomplete package: %v", err, cleanupErr)
			}
		}
	}()
	return fs.WalkDir(resourcetemplates.Files, ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		target := filepath.Join(directory, filepath.FromSlash(path))
		if entry.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, err := resourcetemplates.Files.ReadFile(path)
		if err != nil {
			return err
		}
		if path == Filename || path == "values.yaml" {
			var document map[string]any
			if err := yaml.Unmarshal(data, &document); err != nil {
				return err
			}
			if path == Filename {
				document["name"] = name
			} else {
				workflow, ok := document["workflow"].(map[string]any)
				if !ok {
					return fmt.Errorf("starter values must define workflow")
				}
				workflow["name"] = name
			}
			data, err = yaml.Marshal(document)
			if err != nil {
				return err
			}
		}
		return os.WriteFile(target, data, 0644)
	})
}
