// Package templates adapts standalone package rendering to the CLI.
package templates

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jasondeutsch/watts/cli/internal/arguments"
	"github.com/jasondeutsch/watts/cli/internal/terminal"
	"github.com/jasondeutsch/watts/internal/manifest"
	"github.com/jasondeutsch/watts/internal/render"
	"github.com/jasondeutsch/watts/internal/workflowpackage"
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
	if len(args) > 0 && args[0] == "new" {
		return New(a, args[1:])
	}
	flags := arguments.NewFlags(a.ErrorOutput, "template")
	var overrides valuesFiles
	var output string
	flags.Var(&overrides, "f", "values YAML file; repeat for additional overrides (later files win)")
	flags.Var(&overrides, "values", "values YAML file; repeat for additional overrides (later files win)")
	flags.StringVar(&output, "o", "", "generated output directory; replaced on each render (default: stdout)")
	flags.StringVar(&output, "output", "", "generated output directory; replaced on each render (default: stdout)")
	flags.Usage = func() {
		fmt.Fprintln(a.ErrorOutput, "Usage: watts template <package> [-f <values.yaml>]... [-o <output-dir>]")
		fmt.Fprintln(a.ErrorOutput, "       watts template new <name>")
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
	var sources []render.ValuesSource
	for _, name := range overrides {
		data, err := os.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read values %s: %w", name, err)
		}
		sources = append(sources, render.ValuesSource{Name: name, Data: data})
	}
	result, inputs, err := renderInput(directory, sources)
	if err != nil {
		return err
	}
	if err := validateResources(result.Files); err != nil {
		return err
	}
	if output != "" {
		source := directory
		if len(inputs) > 0 {
			source = inputs[0]
		}
		if err := validateOutputLocation(source, output, append(overrides, inputs...)); err != nil {
			return err
		}
		return writeFiles(output, result.Files)
	}
	return printFiles(a.Output, result.Files)
}

// New scaffolds authoring files; rendering remains a separate operation.
func New(a *terminal.Context, args []string) error {
	flags := arguments.NewFlags(a.ErrorOutput, "template new")
	flags.Usage = func() {
		fmt.Fprintln(a.ErrorOutput, "Usage: watts template new <name>")
	}
	positional, err := arguments.Parse(flags, args)
	if err != nil {
		return arguments.UsageError(err)
	}
	directory, err := arguments.One(positional, "package name")
	if err != nil {
		return arguments.UsageError(err)
	}
	if err := workflowpackage.Scaffold(directory); err != nil {
		return err
	}
	fmt.Fprintf(a.Output, "Created %s with watts.yaml, values.yaml, and templates for Workflow, Agent, Procedure, ConfigMap, and Secret.\n", directory)
	return nil
}

// A file explicitly selects a descriptor; a directory uses its conventional
// descriptor when present. Generic template directories remain valid inputs.
func renderInput(input string, overrides []render.ValuesSource) (render.Result, []string, error) {
	info, err := os.Stat(input)
	if err != nil {
		return render.Result{}, nil, err
	}
	descriptor := !info.IsDir()
	if info.IsDir() {
		_, err := os.Stat(filepath.Join(input, workflowpackage.Filename))
		descriptor = err == nil
	}
	if descriptor {
		pkg, err := workflowpackage.Load(input)
		if err != nil {
			return render.Result{}, nil, err
		}
		result, err := pkg.Render(overrides...)
		if err != nil {
			return render.Result{}, nil, err
		}
		var inputs []string
		for _, layer := range pkg.Layers {
			inputs = append(inputs, layer.Source)
			root := filepath.Dir(layer.Source)
			for _, name := range append(append([]string{}, layer.Descriptor.Values...), layer.Descriptor.Assets...) {
				inputs = append(inputs, filepath.Join(root, name))
			}
			if layer.Descriptor.Templates != "" {
				inputs = append(inputs, filepath.Join(root, layer.Descriptor.Templates))
			}
		}
		return render.Result{Files: []render.File{{Name: "workflow.yaml", Data: result.Data}}, Values: result.Values}, inputs, nil
	}
	root, err := os.OpenRoot(input)
	if err != nil {
		return render.Result{}, nil, err
	}
	defer root.Close()
	result, err := render.Render(root.FS(), overrides...)
	return result, nil, err
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
