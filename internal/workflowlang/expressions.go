package workflowlang

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"reflect"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/function/stdlib"
	ctyjson "github.com/zclconf/go-cty/cty/json"
)

func functions(read func(string) ([]byte, error)) map[string]function.Function {
	return map[string]function.Function{
		"merge": stdlib.MergeFunc, "concat": stdlib.ConcatFunc, "coalesce": stdlib.CoalesceFunc, "join": stdlib.JoinFunc,
		"format": stdlib.FormatFunc, "length": stdlib.LengthFunc, "jsonencode": stdlib.JSONEncodeFunc, "jsondecode": stdlib.JSONDecodeFunc,
		"file": function.New(&function.Spec{Params: []function.Parameter{{Name: "path", Type: cty.String}}, Type: function.StaticReturnType(cty.String), Impl: func(args []cty.Value, _ cty.Type) (cty.Value, error) {
			name := args[0].AsString()
			if !fs.ValidPath(name) || name == "." {
				return cty.NilVal, fmt.Errorf("file path must be inside the package")
			}
			data, err := read(name)
			if err != nil {
				return cty.NilVal, err
			}
			return cty.StringVal(string(data)), nil
		}}),
	}
}
func evaluate(expression hcl.Expression, ctx *hcl.EvalContext) (cty.Value, error) {
	value, diagnostics := expression.Value(ctx)
	if diagnostics.HasErrors() {
		return cty.NilVal, diagnostics
	}
	if !value.IsWhollyKnown() || value.IsNull() {
		return cty.NilVal, fmt.Errorf("%s: value must be known and non-null at compile time", expression.Range())
	}
	return value, nil
}
func decodeValue(value cty.Value) (any, error) {
	data, err := ctyjson.Marshal(value, value.Type())
	if err != nil {
		return nil, err
	}
	var result any
	err = json.Unmarshal(data, &result)
	return result, err
}
func attributes(body *hclsyntax.Body, ctx *hcl.EvalContext) (map[string]any, error) {
	values := map[string]any{}
	for _, name := range sortedKeys(body.Attributes) {
		attribute := body.Attributes[name]
		value, err := evaluate(attribute.Expr, ctx)
		if err != nil {
			return nil, err
		}
		decoded, err := decodeValue(value)
		if err != nil {
			return nil, err
		}
		values[name] = decoded
	}
	return values, nil
}

// Resource bodies are checked against the existing typed workload contracts.
// Nested blocks and object expressions obey the same schema, never arbitrary YAML.
func bodyFields(body *hclsyntax.Body, schema reflect.Type, ctx *hcl.EvalContext) (map[string]any, error) {
	fields := schemaFields(schema)
	result, err := attributes(body, ctx)
	if err != nil {
		return nil, err
	}
	for name := range result {
		if _, ok := fields[name]; !ok {
			return nil, fmt.Errorf("%s: unsupported argument %q", body.Attributes[name].Range(), name)
		}
	}
	aliases := map[string]string{"tool": "tools", "gate": "gates", "skill": "skills", "check": "checks", "pre_check": "pre_checks"}
	for _, block := range body.Blocks {
		name := block.Type
		if alias, ok := aliases[name]; ok {
			name = alias
		}
		field, ok := fields[name]
		if !ok {
			return nil, fmt.Errorf("%s: unsupported block %q", block.TypeRange, block.Type)
		}
		repeated := field.Kind() == reflect.Slice
		nested := field
		if repeated {
			nested = field.Elem()
		}
		for nested.Kind() == reflect.Pointer {
			nested = nested.Elem()
		}
		if nested.Kind() != reflect.Struct {
			return nil, fmt.Errorf("%s: %s must be an argument", block.TypeRange, block.Type)
		}
		value, err := bodyFields(block.Body, nested, ctx)
		if err != nil {
			return nil, err
		}
		if repeated {
			if _, named := schemaFields(nested)["name"]; named {
				if len(block.Labels) != 1 {
					return nil, fmt.Errorf("%s: %s requires one name label", block.TypeRange, block.Type)
				}
				if _, exists := value["name"]; exists {
					return nil, fmt.Errorf("%s: name is supplied by the block label", block.TypeRange)
				}
				value["name"] = block.Labels[0]
			} else if len(block.Labels) != 0 {
				return nil, fmt.Errorf("%s: %s does not accept labels", block.TypeRange, block.Type)
			}
			if existing, ok := result[name]; ok {
				if _, attr := body.Attributes[name]; attr {
					return nil, fmt.Errorf("%s: %s supplied as both an argument and blocks", block.TypeRange, name)
				}
				result[name] = append(existing.([]any), value)
			} else {
				result[name] = []any{value}
			}
		} else {
			if len(block.Labels) != 0 {
				return nil, fmt.Errorf("%s: %s does not accept labels", block.TypeRange, block.Type)
			}
			if _, exists := result[name]; exists {
				return nil, fmt.Errorf("%s: duplicate %s", block.TypeRange, name)
			}
			result[name] = value
		}
	}
	return result, nil
}
func schemaFields(schema reflect.Type) map[string]reflect.Type {
	for schema.Kind() == reflect.Pointer {
		schema = schema.Elem()
	}
	fields := map[string]reflect.Type{}
	for i := 0; i < schema.NumField(); i++ {
		field := schema.Field(i)
		tag := strings.Split(field.Tag.Get("yaml"), ",")
		if len(tag) > 1 && tag[1] == "inline" {
			for name, t := range schemaFields(field.Type) {
				fields[name] = t
			}
			continue
		}
		if tag[0] != "" && tag[0] != "-" {
			fields[tag[0]] = field.Type
		}
	}
	return fields
}
func object(values map[string]cty.Value) cty.Value {
	if len(values) == 0 {
		return cty.EmptyObjectVal
	}
	return cty.ObjectVal(values)
}
func reference(kind, name string) cty.Value {
	return object(map[string]cty.Value{"kind": cty.StringVal(kind), "name": cty.StringVal(name)})
}
func referenceValue(value any) (string, string, error) {
	fields, ok := value.(map[string]any)
	if !ok {
		return "", "", fmt.Errorf("executor must reference an Agent or Procedure")
	}
	kind, _ := fields["kind"].(string)
	name, _ := fields["name"].(string)
	if kind == "" || name == "" {
		return "", "", fmt.Errorf("invalid resource reference")
	}
	return kind, name, nil
}
