// Package workflowlang compiles HCL-backed ACL packages into executor contracts.
// It does not run workloads, contact providers, or read credential values.
package workflowlang

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/zclconf/go-cty/cty"
)

const Filename = "watts.acl"

type Options struct {
	Workflow      string
	VariableFiles []string
	Variables     map[string]cty.Value
}
type Dependency struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Source    string `json:"source"`
	Version   string `json:"version,omitempty"`
	Digest    string `json:"digest,omitempty"`
	Transport string `json:"transport,omitempty"`
	Lifecycle string `json:"lifecycle,omitempty"`
}
type Asset struct {
	Path string      `json:"path"`
	Data []byte      `json:"data"`
	Mode fs.FileMode `json:"mode"`
	Root string      `json:"root"`
}
type Package struct {
	SchemaVersion int                      `json:"schema_version"`
	Name          string                   `json:"name"`
	Version       string                   `json:"version"`
	Digest        string                   `json:"digest"`
	Source        string                   `json:"source"`
	SourcePaths   []string                 `json:"source_paths"`
	Dependencies  []Dependency             `json:"dependencies"`
	Definition    orchestration.Definition `json:"definition"`
	Assets        []Asset                  `json:"assets,omitempty"`
}

// Load resolves local modules and captures inputs without modifying the project.
func Load(source string, options Options) (Package, error) {
	resolver := moduleResolver{active: map[string]bool{}}
	module, err := resolver.resolveModule(source, "", options)
	if err != nil {
		return Package{}, err
	}
	definition, err := module.compileDefinition()
	if err != nil {
		return Package{}, err
	}
	pkg := Package{SchemaVersion: 1, Name: module.name, Version: module.version, Source: module.source, SourcePaths: module.paths, Dependencies: module.dependencies, Assets: module.assets, Definition: definition}
	for _, step := range definition.Steps {
		if step.Container != nil {
			pkg.Dependencies = append(pkg.Dependencies, Dependency{Kind: "image", Name: step.Name, Source: step.Container.Image})
		}
		for _, name := range sortedKeys(step.MCPServices) {
			service := step.MCPServices[name]
			pkg.Dependencies = append(pkg.Dependencies, Dependency{Kind: "mcp", Name: name, Source: name, Transport: service.Connection.Transport, Lifecycle: service.Lifecycle})
		}
	}
	// Shared modules may carry the same content-addressed assets more than once.
	unique := map[string]Asset{}
	for _, asset := range pkg.Assets {
		key := asset.Root + "/" + asset.Path
		if prior, exists := unique[key]; exists && (!bytes.Equal(prior.Data, asset.Data) || prior.Mode != asset.Mode) {
			return Package{}, fmt.Errorf("conflicting captured asset %s", key)
		}
		unique[key] = asset
	}
	pkg.Assets = nil
	for _, key := range sortedKeys(unique) {
		pkg.Assets = append(pkg.Assets, unique[key])
	}
	paths := map[string]bool{}
	for _, name := range pkg.SourcePaths {
		paths[name] = true
	}
	pkg.SourcePaths = sortedKeys(paths)
	pkg.Digest, err = pkg.fingerprint()
	if err != nil {
		return Package{}, err
	}
	return pkg, nil
}

func digest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for name := range values {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	return keys
}

func sourcePath(source string) (string, error) {
	info, err := os.Stat(source)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		source = filepath.Join(source, Filename)
	} else if !strings.HasSuffix(source, ".acl") {
		return "", fmt.Errorf("workflow sources must be ACL files or package directories")
	}
	absolute, err := filepath.Abs(source)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(absolute)
}

// Materialize publishes captured assets under content-addressed, package-private paths.
func (pkg Package) Materialize(capturedDirectory string) error {
	root, err := os.OpenRoot(capturedDirectory)
	if err != nil {
		return err
	}
	defer root.Close()
	for _, asset := range pkg.Assets {
		target := path.Join(asset.Root, asset.Path)
		if existing, err := root.ReadFile(target); err == nil {
			info, statErr := root.Stat(target)
			if statErr != nil {
				return statErr
			}
			if !bytes.Equal(existing, asset.Data) || info.Mode().Perm() != asset.Mode.Perm() {
				return fmt.Errorf("package asset was modified: %s", target)
			}
			continue
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := root.MkdirAll(path.Dir(target), 0755); err != nil {
			return err
		}
		file, err := root.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, asset.Mode)
		if err != nil {
			return err
		}
		_, writeErr := file.Write(asset.Data)
		closeErr := file.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func collectAssets(root *os.Root, declared []string) ([]Asset, error) {
	var assets []Asset
	seen := map[string]bool{}
	for _, directory := range declared {
		if !fs.ValidPath(directory) || directory == "." {
			return nil, fmt.Errorf("asset path %q must be inside its package", directory)
		}
		err := fs.WalkDir(root.FS(), directory, func(name string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.Type()&fs.ModeSymlink != 0 {
				return fmt.Errorf("asset symlink is unsupported: %s", name)
			}
			if entry.IsDir() || seen[name] {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("asset is not a regular file: %s", name)
			}
			data, err := root.ReadFile(name)
			if err != nil {
				return err
			}
			seen[name] = true
			assets = append(assets, Asset{Path: name, Data: data, Mode: info.Mode().Perm()})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Slice(assets, func(i, j int) bool { return assets[i].Path < assets[j].Path })
	return assets, nil
}

func IsSource(source string) bool {
	if info, err := os.Stat(source); err == nil && info.IsDir() {
		_, err = os.Stat(filepath.Join(source, Filename))
		return err == nil
	}
	return strings.HasSuffix(source, ".acl")
}
