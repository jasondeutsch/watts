package workflowpackage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/jasondeutsch/watts/internal/manifest"
	"github.com/jasondeutsch/watts/internal/render"
	"gopkg.in/yaml.v3"
)

type Identity struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Digest  string `json:"digest"`
	Source  string `json:"source"`
}

type Metadata struct {
	Name    string     `json:"name"`
	Version string     `json:"version"`
	Digest  string     `json:"digest"`
	Layers  []Identity `json:"layers"`
}

type Result struct {
	Data     []byte
	Values   map[string]any
	Metadata Metadata
}

func (pkg Package) Render(overrides ...render.ValuesSource) (Result, error) {
	if len(pkg.Layers) == 0 {
		return Result{}, fmt.Errorf("package contains no layers")
	}
	var valuesSources []render.ValuesSource
	var templates []render.File
	for _, layer := range pkg.Layers {
		valuesSources = append(valuesSources, layer.Values...)
		for _, source := range layer.Templates {
			source.Context = map[string]any{"Package": map[string]any{"Name": layer.Descriptor.Name, "Version": layer.Descriptor.Version, "Root": layer.Root}}
			templates = append(templates, source)
		}
	}
	values, err := render.MergeValues(append(valuesSources, overrides...)...)
	if err != nil {
		return Result{}, err
	}
	for _, layer := range pkg.Layers {
		if layer.Schema != nil {
			if err := render.ValidateSchema(layer.Schema, values); err != nil {
				return Result{}, fmt.Errorf("package %s: %w", layer.Descriptor.Name, err)
			}
		}
	}
	rendered, err := render.RenderSources(templates, values)
	if err != nil {
		return Result{}, err
	}
	var resources []manifest.Resource
	positions := map[manifest.Reference]int{}
	for _, layer := range pkg.Layers {
		var documents [][]byte
		for _, file := range rendered.Files {
			if strings.HasPrefix(file.Name, layer.Descriptor.Name+"/") {
				documents = append(documents, []byte(strings.ReplaceAll(string(file.Data), "{package}", layer.Root)))
			}
		}
		if len(documents) == 0 {
			if len(layer.Descriptor.Replaces) > 0 {
				return Result{}, fmt.Errorf("package %s declares replacements without resources", layer.Descriptor.Name)
			}
			continue
		}
		data, err := manifest.JoinDocuments(documents...)
		if err != nil {
			return Result{}, err
		}
		bundle, err := manifest.ParseResources(data)
		if err != nil {
			return Result{}, fmt.Errorf("package %s: %w", layer.Descriptor.Name, err)
		}
		allowed := map[manifest.Reference]bool{}
		for _, ref := range layer.Descriptor.Replaces {
			if allowed[ref] {
				return Result{}, fmt.Errorf("package %s repeats replacement %s %s", layer.Descriptor.Name, ref.Kind, ref.Name)
			}
			if _, exists := positions[ref]; !exists {
				return Result{}, fmt.Errorf("package %s cannot replace missing %s %s", layer.Descriptor.Name, ref.Kind, ref.Name)
			}
			allowed[ref] = true
		}
		for _, resource := range bundle.Resources {
			ref := manifest.Reference{Kind: resource.Kind, Name: resource.Name}
			if index, exists := positions[ref]; exists {
				if !allowed[ref] {
					return Result{}, fmt.Errorf("package %s duplicates %s %s; declare it in replaces", layer.Descriptor.Name, ref.Kind, ref.Name)
				}
				resources[index] = resource
				delete(allowed, ref)
			} else {
				positions[ref] = len(resources)
				resources = append(resources, resource)
			}
		}
		if len(allowed) > 0 {
			return Result{}, fmt.Errorf("package %s declares a replacement but does not provide its resource", layer.Descriptor.Name)
		}
	}
	if _, err := (manifest.Bundle{Resources: resources}).Compile(); err != nil {
		return Result{}, err
	}
	var documents [][]byte
	for _, resource := range resources {
		data, err := yaml.Marshal(resource)
		if err != nil {
			return Result{}, err
		}
		documents = append(documents, data)
	}
	data, err := manifest.JoinDocuments(documents...)
	if err != nil {
		return Result{}, err
	}
	consumer := pkg.Layers[len(pkg.Layers)-1].Descriptor
	metadata := Metadata{Name: consumer.Name, Version: consumer.Version}
	digest := sha256.New()
	for _, layer := range pkg.Layers {
		metadata.Layers = append(metadata.Layers, Identity{Name: layer.Descriptor.Name, Version: layer.Descriptor.Version, Digest: layer.Digest, Source: layer.Source})
		digest.Write([]byte(layer.Digest))
	}
	encodedValues, err := yaml.Marshal(values)
	if err != nil {
		return Result{}, err
	}
	digest.Write(encodedValues)
	digest.Write(data)
	metadata.Digest = hex.EncodeToString(digest.Sum(nil))
	return Result{Data: data, Values: values, Metadata: metadata}, nil
}
