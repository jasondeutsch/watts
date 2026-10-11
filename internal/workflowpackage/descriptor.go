// Package workflowpackage resolves local, composable workflow packages.
// It has no dependency on project configuration, CLI state, or Temporal.
package workflowpackage

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/jasondeutsch/watts/internal/manifest"
	"gopkg.in/yaml.v3"
)

const Filename = "watts.yaml"

type Base struct {
	Path string `yaml:"path"`
}

// Descriptor identifies a package independently of the descriptor filename.
type Descriptor struct {
	Name        string               `yaml:"name" json:"name"`
	Version     string               `yaml:"version" json:"version"`
	Description string               `yaml:"description,omitempty" json:"description,omitempty"`
	Extends     *Base                `yaml:"extends,omitempty" json:"-"`
	Values      []string             `yaml:"values,omitempty" json:"-"`
	Templates   string               `yaml:"templates,omitempty" json:"-"`
	Assets      []string             `yaml:"assets,omitempty" json:"-"`
	Replaces    []manifest.Reference `yaml:"replaces,omitempty" json:"-"`
}

var packageName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

func ReadDescriptor(filename string) (Descriptor, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return Descriptor{}, err
	}
	var descriptor Descriptor
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&descriptor); err != nil {
		return descriptor, fmt.Errorf("descriptor %s: %w", filename, err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		return descriptor, fmt.Errorf("descriptor %s must contain one YAML document", filename)
	}
	if !packageName.MatchString(descriptor.Name) || descriptor.Name == "end" {
		return descriptor, fmt.Errorf("descriptor %s requires a valid package name", filename)
	}
	if descriptor.Version == "" {
		return descriptor, fmt.Errorf("package %s requires version", descriptor.Name)
	}
	if descriptor.Templates == "" && descriptor.Extends == nil {
		return descriptor, fmt.Errorf("package %s requires templates or extends", descriptor.Name)
	}
	if descriptor.Extends != nil && descriptor.Extends.Path == "" {
		return descriptor, fmt.Errorf("package %s: extends requires path", descriptor.Name)
	}
	return descriptor, nil
}

func descriptorPath(filename string) (string, error) {
	info, err := os.Stat(filename)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		filename = filepath.Join(filename, Filename)
	}
	absolute, err := filepath.Abs(filename)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(absolute)
}
