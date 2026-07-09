// YAML loading with the same strictness as the reference: top level must be
// a mapping; absent file is a usage error. Keys are normalised to strings
// (YAML 1.1 parsers may resolve bare `on:` as a boolean key — the
// conformance dialect parser handles both spellings).

package core

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func normaliseYAML(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = normaliseYAML(val)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[fmt.Sprint(k)] = normaliseYAML(val)
		}
		return out
	case []any:
		for i := range t {
			t[i] = normaliseYAML(t[i])
		}
		return t
	default:
		return v
	}
}

func ParseYAMLMap(data []byte, name string) (map[string]any, error) {
	var v any
	if err := yaml.Unmarshal(data, &v); err != nil {
		return nil, Usagef("%s: invalid YAML: %v", name, err)
	}
	if v == nil {
		return map[string]any{}, nil
	}
	m, ok := normaliseYAML(v).(map[string]any)
	if !ok {
		return nil, Usagef("%s: top level must be a mapping", name)
	}
	return m, nil
}

func LoadYAML(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, Usagef("cannot read %s: %v", path, err)
	}
	return ParseYAMLMap(data, path)
}

// -- typed accessors over the normalised maps ----------------------------------

func getMap(m map[string]any, key string) map[string]any {
	if v, ok := m[key].(map[string]any); ok {
		return v
	}
	return map[string]any{}
}

func getStr(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getBool(m map[string]any, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

func getList(m map[string]any, key string) []any {
	if v, ok := m[key].([]any); ok {
		return v
	}
	return nil
}

// strList converts a []any of strings; ok=false if any element is not a
// string (callers turn that into their own validation error).
func strList(v any) ([]string, bool) {
	items, ok := v.([]any)
	if !ok {
		if v == nil {
			return nil, true
		}
		return nil, false
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		s, ok := item.(string)
		if !ok {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}

// schemaVersionIs handles YAML/JSON numeric typing (int vs float64).
func schemaVersionIs(m map[string]any, want int) bool {
	switch v := m["schema_version"].(type) {
	case int:
		return v == want
	case float64:
		return v == float64(want)
	}
	return false
}
