// Package manifest encodes human-authored, typed YAML resources.
// Runtime bindings and executor protocols remain JSON.
package manifest

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const SchemaVersion = 1
const Project = "Project"
const Workflow = "Workflow"

type Document[T any] struct {
	Kind          string `yaml:"kind"`
	SchemaVersion int    `yaml:"schema_version"`
	Name          string `yaml:"name"`
	Spec          T      `yaml:"spec"`
}

func Decode[T any](data []byte, kind string, defaults T) (Document[T], error) {
	document := Document[T]{Spec: defaults}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&document); err != nil {
		return document, fmt.Errorf("invalid YAML manifest: %w", err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		if err != nil {
			return document, fmt.Errorf("invalid YAML manifest: %w", err)
		}
		return document, fmt.Errorf("manifest must contain exactly one YAML document")
	}
	if document.Kind != kind {
		return document, fmt.Errorf("expected kind %q, found %q", kind, document.Kind)
	}
	if document.SchemaVersion != SchemaVersion {
		return document, fmt.Errorf("unsupported schema_version %d (expected %d)", document.SchemaVersion, SchemaVersion)
	}
	if strings.TrimSpace(document.Name) == "" {
		return document, fmt.Errorf("manifest name must not be empty")
	}
	return document, nil
}

func Encode[T any](kind, name string, spec T) ([]byte, error) {
	if kind != Project && kind != Workflow {
		return nil, fmt.Errorf("unsupported manifest kind %q", kind)
	}
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("manifest name must not be empty")
	}
	var output bytes.Buffer
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if err := encoder.Encode(Document[T]{Kind: kind, SchemaVersion: SchemaVersion, Name: name, Spec: spec}); err != nil {
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func Write[T any](path, kind, name string, spec T) error {
	data, err := Encode(kind, name, spec)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".watts-manifest-*.yaml")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err = file.Chmod(0644); err == nil {
		_, err = file.Write(data)
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(file.Name(), path)
}
