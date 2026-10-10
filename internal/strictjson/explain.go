// Package strictjson explains strict-decode failures to the model that wrote
// the JSON. encoding/json reports `unknown field "x"` without saying where x
// sits, so a desk whose field was valid at the top level but misplaced in a
// nested object could not tell what was wrong.
package strictjson

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// Explain returns a sentence naming the first unknown field in body, its
// path, and the fields valid in the object that holds it, for decoding into
// target's type with DisallowUnknownFields. It returns "" when body is not
// JSON or has no unknown field.
func Explain(body []byte, target any) string {
	var doc any
	if err := json.Unmarshal(body, &doc); err != nil {
		return ""
	}
	return walk(doc, reflect.TypeOf(target), "")
}

func walk(value any, t reflect.Type, path string) string {
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == nil {
		return ""
	}
	switch t.Kind() {
	case reflect.Struct:
		object, ok := value.(map[string]any)
		if !ok {
			return ""
		}
		fields := structFields(t)
		keys := make([]string, 0, len(object))
		for key := range object {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			field, known := fields[key]
			if !known {
				field, known = fields[strings.ToLower(key)]
			}
			if !known {
				where := "the top level"
				if path != "" {
					where = path
				}
				names := make([]string, 0, len(fields))
				for name := range fields {
					names = append(names, name)
				}
				sort.Strings(names)
				return fmt.Sprintf("unknown field %q at %s; valid there: %s", key, where, strings.Join(names, ", "))
			}
			if found := walk(object[key], field, join(path, key)); found != "" {
				return found
			}
		}
	case reflect.Slice, reflect.Array:
		items, ok := value.([]any)
		if !ok {
			return ""
		}
		for i, item := range items {
			if found := walk(item, t.Elem(), path+"["+strconv.Itoa(i)+"]"); found != "" {
				return found
			}
		}
	case reflect.Map:
		object, ok := value.(map[string]any)
		if !ok {
			return ""
		}
		keys := make([]string, 0, len(object))
		for key := range object {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if found := walk(object[key], t.Elem(), join(path, key)); found != "" {
				return found
			}
		}
	}
	return ""
}

// structFields maps each JSON name of t (embedded structs flattened) to its
// field type. Lower-cased names are added too: encoding/json matches keys
// case-insensitively.
func structFields(t reflect.Type) map[string]reflect.Type {
	out := map[string]reflect.Type{}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("json")
		name, _, _ := strings.Cut(tag, ",")
		if name == "-" {
			continue
		}
		if field.Anonymous && name == "" {
			embedded := field.Type
			for embedded.Kind() == reflect.Pointer {
				embedded = embedded.Elem()
			}
			if embedded.Kind() == reflect.Struct {
				for key, value := range structFields(embedded) {
					out[key] = value
				}
				continue
			}
		}
		if !field.IsExported() {
			continue
		}
		if name == "" {
			name = field.Name
		}
		out[name] = field.Type
	}
	for key, value := range out {
		if lower := strings.ToLower(key); lower != key {
			if _, exists := out[lower]; !exists {
				out[lower] = value
			}
		}
	}
	return out
}

func join(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}
