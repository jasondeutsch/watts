// Package render renders YAML workflow packages without knowing Watts resource
// kinds, project configuration, orchestration, or CLI state.
package render

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"gopkg.in/yaml.v3"
)

// ValuesSource is a caller-supplied override file. Name identifies errors;
// Data contains a single YAML mapping. Sources apply in argument order.
type ValuesSource struct {
	Name string
	Data []byte
}

// File is a rendered YAML file relative to templates/.
type File struct {
	Name string
	Data []byte
}

// Result contains ordered outputs and the resolved non-secret parameter values.
type Result struct {
	Files  []File
	Values map[string]any
}

// Render reads optional values.yaml and values.schema.json, then renders files
// under templates/. It performs no writes or network requests. The caller owns
// the filesystem boundary and resource-specific validation. Template authors
// must be trusted; this API is not an execution sandbox.
func Render(packageFS fs.FS, overrides ...ValuesSource) (Result, error) {
	values, err := loadValues(packageFS, overrides)
	if err != nil {
		return Result{}, err
	}
	if err := validateValues(packageFS, values); err != nil {
		return Result{}, err
	}
	templates, err := fs.Sub(packageFS, "templates")
	if err != nil {
		return Result{}, fmt.Errorf("templates: %w", err)
	}
	var sources []File
	err = fs.WalkDir(templates, ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("template symlinks are unsupported: %s", name)
		}
		if entry.IsDir() {
			return nil
		}
		if !strings.HasSuffix(name, ".yaml.gotmpl") && !strings.HasSuffix(name, ".tpl") && !strings.HasSuffix(name, ".yaml") {
			return nil
		}
		data, err := fs.ReadFile(templates, name)
		if err != nil {
			return err
		}
		sources = append(sources, File{Name: name, Data: data})
		return nil
	})
	if err != nil {
		return Result{}, fmt.Errorf("load templates: %w", err)
	}
	engine := template.New("package").Option("missingkey=error")
	engine.Funcs(functions(engine))
	for _, source := range sources {
		if strings.HasSuffix(source.Name, ".yaml") {
			continue
		}
		if _, err := engine.New(source.Name).Parse(string(source.Data)); err != nil {
			return Result{}, fmt.Errorf("parse %s: %w", source.Name, err)
		}
	}
	result := Result{Values: values}
	for _, source := range sources {
		if strings.HasSuffix(source.Name, ".tpl") {
			continue
		}
		output := source.Data
		name := strings.TrimSuffix(source.Name, ".gotmpl")
		if strings.HasSuffix(source.Name, ".gotmpl") {
			buffer := &limitedBuffer{}
			if err := engine.ExecuteTemplate(buffer, source.Name, map[string]any{"Values": values}); err != nil {
				return Result{}, fmt.Errorf("render %s: %w", source.Name, err)
			}
			output = buffer.Bytes()
		}
		if len(bytes.TrimSpace(output)) == 0 {
			continue
		}
		if err := validateYAML(output); err != nil {
			return Result{}, fmt.Errorf("rendered %s: %w", source.Name, err)
		}
		for _, file := range result.Files {
			if file.Name == name {
				return Result{}, fmt.Errorf("duplicate output path %s", name)
			}
		}
		result.Files = append(result.Files, File{Name: name, Data: output})
	}
	if len(result.Files) == 0 {
		return Result{}, errors.New("package produced no YAML files")
	}
	return result, nil
}

func functions(engine *template.Template) template.FuncMap {
	available := sprig.TxtFuncMap()
	allowed := template.FuncMap{}
	// An explicit list prevents future Sprig additions from expanding capabilities.
	for _, name := range []string{"quote", "squote", "default", "empty", "coalesce", "ternary", "indent", "nindent", "trim", "trimSuffix", "trimPrefix", "lower", "upper", "replace", "contains", "hasPrefix", "hasSuffix", "join", "list", "dict", "get", "hasKey", "keys", "sortAlpha", "toJson", "fail"} {
		allowed[name] = available[name]
	}
	allowed["toYaml"] = func(value any) (string, error) {
		var output bytes.Buffer
		encoder := yaml.NewEncoder(&output)
		encoder.SetIndent(2)
		if err := encoder.Encode(value); err != nil {
			return "", err
		}
		if err := encoder.Close(); err != nil {
			return "", err
		}
		return strings.TrimSuffix(output.String(), "\n"), nil
	}
	allowed["include"] = func(name string, value any) (string, error) {
		output := &limitedBuffer{}
		if err := engine.ExecuteTemplate(output, name, value); err != nil {
			return "", err
		}
		return output.String(), nil
	}
	return allowed
}

const maxOutputBytes = 4 * 1024 * 1024

type limitedBuffer struct{ bytes.Buffer }

func (buffer *limitedBuffer) Write(data []byte) (int, error) {
	if len(data) > maxOutputBytes-buffer.Len() {
		return 0, errors.New("rendered output exceeds 4 MiB limit")
	}
	return buffer.Buffer.Write(data)
}

func (buffer *limitedBuffer) WriteString(value string) (int, error) {
	return buffer.Write([]byte(value))
}
