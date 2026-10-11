package workflowpackage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/jasondeutsch/watts/internal/render"
)

type Asset struct {
	Path string
	Data []byte
	Mode fs.FileMode
}

type Layer struct {
	Descriptor Descriptor
	Source     string
	Digest     string
	Root       string
	Templates  []render.File
	Assets     []Asset
	Values     []render.ValuesSource
	Schema     []byte
}

// Package retains exact local inputs. Later changes cannot alter this resolution.
type Package struct {
	Layers []Layer // Base first, consumer last.
}

func Load(filename string) (Package, error) {
	var result Package
	if err := result.load(filename, map[string]bool{}); err != nil {
		return result, err
	}
	return result, nil
}

func (pkg *Package) load(filename string, active map[string]bool) error {
	source, err := descriptorPath(filename)
	if err != nil {
		return err
	}
	if active[source] {
		return fmt.Errorf("package inheritance cycle at %s", source)
	}
	if len(active) >= 32 {
		return fmt.Errorf("package inheritance exceeds 32 layers")
	}
	active[source] = true
	defer delete(active, source)
	descriptor, err := ReadDescriptor(source)
	if err != nil {
		return err
	}
	if descriptor.Extends != nil {
		base := descriptor.Extends.Path
		if !filepath.IsAbs(base) {
			base = filepath.Join(filepath.Dir(source), base)
		}
		if err := pkg.load(base, active); err != nil {
			return fmt.Errorf("package %s extends: %w", descriptor.Name, err)
		}
	}
	for _, layer := range pkg.Layers {
		if layer.Descriptor.Name == descriptor.Name {
			return fmt.Errorf("duplicate package name %s in inheritance chain", descriptor.Name)
		}
	}
	layer, err := loadLayer(source, descriptor)
	if err != nil {
		return fmt.Errorf("package %s: %w", descriptor.Name, err)
	}
	pkg.Layers = append(pkg.Layers, layer)
	return nil
}

func loadLayer(source string, descriptor Descriptor) (Layer, error) {
	layer := Layer{Descriptor: descriptor, Source: source}
	root, err := os.OpenRoot(filepath.Dir(source))
	if err != nil {
		return layer, err
	}
	defer root.Close()
	digest := sha256.New()
	data, err := root.ReadFile(filepath.Base(source))
	if err != nil {
		return layer, err
	}
	writeDigest := func(name string, data []byte) {
		fmt.Fprintf(digest, "%d:%s:%d:", len(name), name, len(data))
		digest.Write(data)
	}
	writeDigest("descriptor", data)
	for _, filename := range descriptor.Values {
		if !fs.ValidPath(filename) {
			return layer, fmt.Errorf("values path %q must be inside its package", filename)
		}
		data, err := root.ReadFile(filename)
		if err != nil {
			return layer, fmt.Errorf("values %s: %w", filename, err)
		}
		writeDigest(filename, data)
		layer.Values = append(layer.Values, render.ValuesSource{Name: source + ":" + filename, Data: data})
	}
	if descriptor.Templates != "" {
		if !fs.ValidPath(descriptor.Templates) || descriptor.Templates == "." {
			return layer, fmt.Errorf("templates path must be a directory inside its package")
		}
		if err := fs.WalkDir(root.FS(), descriptor.Templates, func(filename string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.Type()&fs.ModeSymlink != 0 {
				return fmt.Errorf("template symlinks are unsupported: %s", filename)
			}
			if entry.IsDir() {
				return nil
			}
			if !strings.HasSuffix(filename, ".yaml.gotmpl") && !strings.HasSuffix(filename, ".tpl") && !strings.HasSuffix(filename, ".yaml") {
				return nil
			}
			data, err := root.ReadFile(filename)
			if err != nil {
				return err
			}
			writeDigest(filename, data)
			name := strings.TrimPrefix(filename, descriptor.Templates+"/")
			layer.Templates = append(layer.Templates, render.File{Name: path.Join(descriptor.Name, name), Data: data})
			return nil
		}); err != nil {
			return layer, err
		}
	}
	seenAssets := map[string]bool{}
	for _, filename := range descriptor.Assets {
		if !fs.ValidPath(filename) || filename == "." {
			return layer, fmt.Errorf("asset path %q must be inside its package", filename)
		}
		if err := fs.WalkDir(root.FS(), filename, func(filename string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.Type()&fs.ModeSymlink != 0 {
				return fmt.Errorf("asset symlinks are unsupported: %s", filename)
			}
			if entry.IsDir() || seenAssets[filename] {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("asset must be a regular file: %s", filename)
			}
			data, err := root.ReadFile(filename)
			if err != nil {
				return err
			}
			seenAssets[filename] = true
			writeDigest(filename, data)
			writeDigest(filename+":mode", []byte(fmt.Sprint(info.Mode().Perm())))
			layer.Assets = append(layer.Assets, Asset{Path: filename, Data: data, Mode: info.Mode().Perm()})
			return nil
		}); err != nil {
			return layer, err
		}
	}
	layer.Schema, err = root.ReadFile("values.schema.json")
	if err != nil && !os.IsNotExist(err) {
		return layer, err
	}
	if layer.Schema != nil {
		writeDigest("values.schema.json", layer.Schema)
	}
	layer.Digest = hex.EncodeToString(digest.Sum(nil))
	layer.Root = path.Join(".watts", "packages", descriptor.Name, layer.Digest)
	return layer, nil
}
