package render

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

func loadValues(packageFS fs.FS, overrides []ValuesSource) (map[string]any, error) {
	values := map[string]any{}
	data, err := fs.ReadFile(packageFS, "values.yaml")
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("values.yaml: %w", err)
	}
	sources := overrides
	if err == nil {
		sources = append([]ValuesSource{{Name: "values.yaml", Data: data}}, overrides...)
	}
	for _, source := range sources {
		var incoming map[string]any
		if err := decodeYAML(source.Data, &incoming); err != nil {
			return nil, fmt.Errorf("values %s: %w", source.Name, err)
		}
		if incoming == nil {
			return nil, fmt.Errorf("values %s: expected a YAML mapping", source.Name)
		}
		mergeValues(values, incoming)
	}
	return values, nil
}

// Maps merge recursively; lists and scalars replace, including false, zero,
// empty strings, and explicit null. No implicit list concatenation or deletion.
func mergeValues(destination, source map[string]any) {
	for key, value := range source {
		incoming, isMap := value.(map[string]any)
		if !isMap {
			destination[key] = value
			continue
		}
		existing, ok := destination[key].(map[string]any)
		if !ok {
			existing = map[string]any{}
			destination[key] = existing
		}
		mergeValues(existing, incoming)
	}
}

func decodeYAML(data []byte, value any) error {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		if err != nil {
			return err
		}
		return errors.New("expected exactly one YAML document")
	}
	return nil
}

func validateYAML(data []byte) error {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	count := 0
	for {
		var document map[string]any
		err := decoder.Decode(&document)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if document == nil {
			return errors.New("expected a YAML mapping, found an empty document")
		}
		count++
	}
	if count == 0 {
		return errors.New("expected a YAML document")
	}
	return nil
}

func validateValues(packageFS fs.FS, values map[string]any) error {
	data, err := fs.ReadFile(packageFS, "values.schema.json")
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("values.schema.json: %w", err)
	}
	schemaDocument, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("values.schema.json: %w", err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.UseLoader(noExternalSchemas{})
	const location = "https://watts.invalid/values.schema.json"
	if err := compiler.AddResource(location, schemaDocument); err != nil {
		return fmt.Errorf("values.schema.json: %w", err)
	}
	schema, err := compiler.Compile(location)
	if err != nil {
		return fmt.Errorf("values.schema.json: %w", err)
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return fmt.Errorf("values must be JSON-compatible: %w", err)
	}
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	if err := schema.Validate(instance); err != nil {
		return fmt.Errorf("values validation: %w", err)
	}
	return nil
}

type noExternalSchemas struct{}

func (noExternalSchemas) Load(url string) (any, error) {
	return nil, fmt.Errorf("external schema references are unsupported: %s", url)
}
