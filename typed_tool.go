package agnogo

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// TypedTool creates a ToolDef from a strongly-typed function.
// The input struct's fields become tool parameters, and the output is auto-serialized to JSON.
//
// Struct tags:
//   - json:"name"       → parameter name (required)
//   - desc:"..."        → parameter description
//   - required:"true"   → marks parameter as required
//   - enum:"a,b,c"      → allowed values
//
// Example:
//
//	type WeatherInput struct {
//	    City string `json:"city" desc:"City name" required:"true"`
//	    Unit string `json:"unit" desc:"Temperature unit" enum:"C,F"`
//	}
//	type WeatherOutput struct {
//	    Temp float64 `json:"temperature"`
//	    Desc string  `json:"description"`
//	}
//	tool := agnogo.TypedTool[WeatherInput, WeatherOutput]("weather", "Get weather", getWeather)
func TypedTool[In, Out any](name, desc string, fn func(ctx context.Context, in In) (Out, error)) ToolDef {
	params := structToParams[In]()

	return ToolDef{
		Name:   name,
		Desc:   desc,
		Params: params,
		Fn: func(ctx context.Context, args map[string]string) (string, error) {
			in, err := argsToStruct[In](args)
			if err != nil {
				return "", fmt.Errorf("typed tool %q: unmarshal input: %w", name, err)
			}

			out, err := fn(ctx, in)
			if err != nil {
				return "", err
			}

			return marshalOutput(out)
		},
	}
}

// structToParams uses reflect to inspect struct fields and build a Params map.
// Panics if T is not a struct, ensuring fast failure at registration time.
func structToParams[T any]() Params {
	t := reflect.TypeFor[T]()
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		panic(fmt.Sprintf("agnogo.TypedTool: input type %s must be a struct", t))
	}

	params := make(Params, t.NumField())
	for i := range t.NumField() {
		f := t.Field(i)

		// Skip unexported fields.
		if !f.IsExported() {
			continue
		}

		// Derive parameter name from the json tag; skip fields with json:"-".
		jsonTag := f.Tag.Get("json")
		if jsonTag == "-" {
			continue
		}
		paramName, _, _ := strings.Cut(jsonTag, ",")
		if paramName == "" {
			paramName = f.Name
		}

		p := Param{
			Type: goTypeToJSONSchema(f.Type),
			Desc: f.Tag.Get("desc"),
		}

		if f.Tag.Get("required") == "true" {
			p.Required = true
		}

		if enumTag := f.Tag.Get("enum"); enumTag != "" {
			p.Enum = strings.Split(enumTag, ",")
		}

		params[paramName] = p
	}

	return params
}

// goTypeToJSONSchema maps a Go reflect.Type to a JSON Schema type string.
func goTypeToJSONSchema(t reflect.Type) string {
	switch t.Kind() {
	case reflect.String:
		return "string"
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return "number"
	default:
		return "string"
	}
}

// argsToStruct converts a map[string]string to a typed struct by round-tripping
// through JSON. This lets encoding/json handle type coercion (e.g. string "42"
// quoted in JSON will unmarshal into an int field via json.Number).
func argsToStruct[T any](args map[string]string) (T, error) {
	var zero T

	// Build a JSON object where values that look like numbers or booleans
	// are emitted as their raw JSON form so they unmarshal into numeric/bool fields.
	obj := make(map[string]json.RawMessage, len(args))
	for k, v := range args {
		obj[k] = toRawJSON(v)
	}

	data, err := json.Marshal(obj)
	if err != nil {
		return zero, err
	}

	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		return zero, err
	}
	return result, nil
}

// toRawJSON converts a string value to a json.RawMessage. Numeric and boolean
// values are kept unquoted so they deserialize into the correct Go type.
func toRawJSON(v string) json.RawMessage {
	// Try to detect numbers and booleans by attempting a round-trip.
	// If json.Unmarshal accepts it as a valid JSON token that is not a string,
	// use it directly.
	if v == "true" || v == "false" || v == "null" {
		return json.RawMessage(v)
	}

	// Try as a number: valid JSON numbers unmarshal into json.Number.
	var n json.Number
	if err := json.Unmarshal([]byte(v), &n); err == nil {
		return json.RawMessage(v)
	}

	// Default: quote as a JSON string.
	b, _ := json.Marshal(v)
	return json.RawMessage(b)
}

// marshalOutput serializes the function output. If the output is already a
// string, it is returned directly without JSON wrapping.
func marshalOutput(out any) (string, error) {
	// Fast path: if Out is a string, return it directly.
	if s, ok := out.(string); ok {
		return s, nil
	}

	b, err := json.Marshal(out)
	if err != nil {
		return "", fmt.Errorf("marshal output: %w", err)
	}
	return string(b), nil
}
