package manifest

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// JoinDocuments preserves YAML nodes while combining file streams without
// introducing empty documents around existing separators.
func JoinDocuments(sources ...[]byte) ([]byte, error) {
	var output bytes.Buffer
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	for _, data := range sources {
		decoder := yaml.NewDecoder(bytes.NewReader(data))
		for {
			var node yaml.Node
			if err := decoder.Decode(&node); err == io.EOF {
				break
			} else if err != nil {
				return nil, err
			}
			if err := encoder.Encode(&node); err != nil {
				return nil, err
			}
		}
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

// ReadWorkflowSource loads one YAML file or the YAML files directly in a
// directory. Authoring bundles are self-contained; arbitrary includes are not used.
func ReadWorkflowSource(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		root, err := os.OpenRoot(filepath.Dir(path))
		if err != nil {
			return nil, err
		}
		defer root.Close()
		return root.ReadFile(filepath.Base(path))
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return nil, err
	}
	var sources [][]byte
	for _, entry := range entries {
		if entry.IsDir() || (!strings.HasSuffix(entry.Name(), ".yaml") && !strings.HasSuffix(entry.Name(), ".yml")) {
			continue
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return nil, fmt.Errorf("resource symlinks are unsupported: %s", entry.Name())
		}
		data, err := root.ReadFile(entry.Name())
		if err != nil {
			return nil, err
		}
		sources = append(sources, data)
	}
	if len(sources) == 0 {
		return nil, fmt.Errorf("workflow directory contains no YAML resources")
	}
	return JoinDocuments(sources...)
}
