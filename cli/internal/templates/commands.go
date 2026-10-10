// Package templates adapts standalone package rendering to the CLI.
package templates

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/jasondeutsch/watts/cli/internal/arguments"
	"github.com/jasondeutsch/watts/cli/internal/terminal"
	"github.com/jasondeutsch/watts/internal/manifest"
	"github.com/jasondeutsch/watts/internal/render"
	"gopkg.in/yaml.v3"
)

type valuesFiles []string

func (files *valuesFiles) String() string { return strings.Join(*files, ", ") }
func (files *valuesFiles) Set(name string) error {
	if name == "" {
		return fmt.Errorf("values path must not be empty")
	}
	*files = append(*files, name)
	return nil
}

func Template(a *terminal.Context, args []string) error {
	flags := arguments.NewFlags(a.ErrorOutput, "template")
	var overrides valuesFiles
	var output string
	flags.Var(&overrides, "f", "values YAML file; repeat for additional overrides (later files win)")
	flags.Var(&overrides, "values", "values YAML file; repeat for additional overrides (later files win)")
	flags.StringVar(&output, "o", "", "output directory; existing files are never overwritten (default: stdout)")
	flags.StringVar(&output, "output", "", "output directory; existing files are never overwritten (default: stdout)")
	flags.Usage = func() {
		fmt.Fprintln(a.ErrorOutput, "Usage: watts template <package-dir> [-f <values.yaml>]... [-o <output-dir>]")
		flags.PrintDefaults()
	}
	positional, err := arguments.Parse(flags, args)
	if err != nil {
		return arguments.UsageError(err)
	}
	directory, err := arguments.One(positional, "package directory")
	if err != nil {
		return arguments.UsageError(err)
	}
	packageRoot, err := os.OpenRoot(directory)
	if err != nil {
		return fmt.Errorf("open package %s: %w", directory, err)
	}
	defer packageRoot.Close()
	var sources []render.ValuesSource
	for _, name := range overrides {
		data, err := os.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read values %s: %w", name, err)
		}
		sources = append(sources, render.ValuesSource{Name: name, Data: data})
	}
	result, err := render.Render(packageRoot.FS(), sources...)
	if err != nil {
		return err
	}
	if err := validateResources(result.Files); err != nil {
		return err
	}
	if output != "" {
		return writeFiles(output, result.Files)
	}
	return printFiles(a.Output, result.Files)
}

func printFiles(output io.Writer, files []render.File) error {
	encoder := yaml.NewEncoder(output)
	encoder.SetIndent(2)
	for _, file := range files {
		decoder := yaml.NewDecoder(bytes.NewReader(file.Data))
		for {
			var document yaml.Node
			if err := decoder.Decode(&document); err == io.EOF {
				break
			} else if err != nil {
				return err
			}
			if err := encoder.Encode(&document); err != nil {
				return err
			}
		}
	}
	return encoder.Close()
}

// Reserve every output before writing so conflicts cannot truncate existing
// files or leave a subset of the rendered files behind. Root confines paths;
// O_EXCL also protects against a file appearing after the preflight checks.
func writeFiles(directory string, files []render.File) (err error) {
	if err := os.MkdirAll(directory, 0755); err != nil {
		return fmt.Errorf("output %s: %w", directory, err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return fmt.Errorf("output %s: %w", directory, err)
	}
	defer root.Close()
	var reserved []*os.File
	defer func() {
		for index, file := range reserved {
			_ = file.Close()
			if err != nil {
				_ = root.Remove(files[index].Name)
			}
		}
	}()
	for _, file := range files {
		if !fs.ValidPath(file.Name) || file.Name == "." {
			return fmt.Errorf("invalid output path %q", file.Name)
		}
		if err := root.MkdirAll(path.Dir(file.Name), 0755); err != nil {
			return fmt.Errorf("output %s: %w", file.Name, err)
		}
		handle, err := root.OpenFile(file.Name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err != nil {
			return fmt.Errorf("output %s (existing files are never overwritten): %w", file.Name, err)
		}
		reserved = append(reserved, handle)
	}
	for index, file := range files {
		if _, err := reserved[index].Write(file.Data); err != nil {
			return fmt.Errorf("write %s: %w", file.Name, err)
		}
		if err := reserved[index].Close(); err != nil {
			return fmt.Errorf("close %s: %w", file.Name, err)
		}
	}
	return nil
}

// Generic YAML remains supported. When a package declares resource kinds,
// validate the complete bundle and its references before emitting any output.
func validateResources(files []render.File) error {
	var sources [][]byte
	resources := false
	for _, file := range files {
		sources = append(sources, file.Data)
		decoder := yaml.NewDecoder(bytes.NewReader(file.Data))
		for {
			var header struct {
				Kind string `yaml:"kind"`
			}
			if err := decoder.Decode(&header); err == io.EOF {
				break
			} else if err != nil {
				return err
			}
			resources = resources || header.Kind != ""
		}
	}
	if !resources {
		return nil
	}
	data, err := manifest.JoinDocuments(sources...)
	if err != nil {
		return err
	}
	bundle, err := manifest.ParseResources(data)
	if err != nil {
		return err
	}
	return bundle.Validate()
}
