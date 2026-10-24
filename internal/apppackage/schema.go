package apppackage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"unicode/utf8"
)

// The kind schema language: a small JSON Schema subset (design section 2.1,
// open decision 5). A kind's schema travels with its package, and the body is
// validated against it before the write.
//
// Keywords: type, required, properties, items, enum, maxLength, maximum, and
// description (an annotation). Anything else is refused at compile time, so a
// manifest cannot rely on a constraint this validator does not enforce.
//
// Objects admit fields the schema does not name. A user extends a kind in a
// kind of their own (I27), and a later schema version adds fields, so the
// validator checks what the schema says and no more.

const (
	maxSchemaDepth = 8
	maxSchemaNodes = 256
	maxEnumValues  = 64
)

var schemaTypes = map[string]bool{
	"object": true, "array": true, "string": true, "number": true,
	"integer": true, "boolean": true, "null": true,
}

// Schema is a compiled kind schema.
type Schema struct {
	Type       string
	Required   []string
	Properties map[string]*Schema
	Items      *Schema
	Enum       []json.RawMessage
	MaxLength  *int
	Maximum    *float64
}

// ErrSchema refuses a schema the subset does not accept.
var ErrSchema = errors.New("app package: invalid kind schema")

// ErrBody refuses a body that does not satisfy its kind's schema.
var ErrBody = errors.New("app package: body does not satisfy the kind schema")

// CompileSchema compiles raw into a Schema. The top level must be an object.
func CompileSchema(raw []byte) (*Schema, error) {
	nodes := 0
	s, err := compileNode(raw, "$", 0, &nodes)
	if err != nil {
		return nil, err
	}
	if s.Type != "object" {
		return nil, fmt.Errorf("%w: the top level must be type object, not %q", ErrSchema, s.Type)
	}
	return s, nil
}

func schemaErr(path, format string, args ...any) error {
	return fmt.Errorf("%w: %s: %s", ErrSchema, path, fmt.Sprintf(format, args...))
}

func compileNode(raw []byte, path string, depth int, nodes *int) (*Schema, error) {
	if depth > maxSchemaDepth {
		return nil, schemaErr(path, "nested deeper than %d", maxSchemaDepth)
	}
	if *nodes++; *nodes > maxSchemaNodes {
		return nil, schemaErr(path, "more than %d schema nodes", maxSchemaNodes)
	}
	var fields map[string]json.RawMessage
	if err := strictJSON(raw, &fields); err != nil || fields == nil {
		return nil, schemaErr(path, "must be a JSON object")
	}
	s := &Schema{}
	var typeSeen bool
	for key, val := range fields {
		switch key {
		case "description":
			var d string
			if json.Unmarshal(val, &d) != nil {
				return nil, schemaErr(path, "description must be a string")
			}
		case "type":
			if err := json.Unmarshal(val, &s.Type); err != nil || !schemaTypes[s.Type] {
				return nil, schemaErr(path, "type must be one of object, array, string, number, integer, boolean, null")
			}
			typeSeen = true
		case "required":
			if err := json.Unmarshal(val, &s.Required); err != nil {
				return nil, schemaErr(path, "required must be a list of strings")
			}
		case "properties":
			var props map[string]json.RawMessage
			if err := strictJSON(val, &props); err != nil || props == nil {
				return nil, schemaErr(path, "properties must be an object")
			}
			s.Properties = map[string]*Schema{}
			for _, name := range sortedKeys(props) {
				child, err := compileNode(props[name], path+".properties."+name, depth+1, nodes)
				if err != nil {
					return nil, err
				}
				s.Properties[name] = child
			}
		case "items":
			child, err := compileNode(val, path+".items", depth+1, nodes)
			if err != nil {
				return nil, err
			}
			s.Items = child
		case "enum":
			if err := json.Unmarshal(val, &s.Enum); err != nil || len(s.Enum) == 0 || len(s.Enum) > maxEnumValues {
				return nil, schemaErr(path, "enum must be a list of 1 to %d values", maxEnumValues)
			}
		case "maxLength":
			var n int
			if err := json.Unmarshal(val, &n); err != nil || n < 0 || n > 1<<20 {
				return nil, schemaErr(path, "maxLength must be an integer from 0 to %d", 1<<20)
			}
			s.MaxLength = &n
		case "maximum":
			var f float64
			if err := json.Unmarshal(val, &f); err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
				return nil, schemaErr(path, "maximum must be a number")
			}
			s.Maximum = &f
		default:
			return nil, schemaErr(path, "keyword %q is not in the supported subset", key)
		}
	}
	if !typeSeen {
		return nil, schemaErr(path, "type is required")
	}
	// A keyword on the wrong type would be silently unenforced; refuse it.
	switch {
	case (len(s.Required) > 0 || s.Properties != nil) && s.Type != "object":
		return nil, schemaErr(path, "required and properties apply to type object, not %q", s.Type)
	case s.Items != nil && s.Type != "array":
		return nil, schemaErr(path, "items applies to type array, not %q", s.Type)
	case s.MaxLength != nil && s.Type != "string":
		return nil, schemaErr(path, "maxLength applies to type string, not %q", s.Type)
	case s.Maximum != nil && s.Type != "number" && s.Type != "integer":
		return nil, schemaErr(path, "maximum applies to type number or integer, not %q", s.Type)
	}
	seen := map[string]bool{}
	for _, name := range s.Required {
		if name == "" || seen[name] {
			return nil, schemaErr(path, "required names must be non-empty and unique")
		}
		seen[name] = true
	}
	for _, v := range s.Enum {
		if err := s.checkType(v, path+".enum"); err != nil {
			return nil, schemaErr(path, "enum value %s does not have type %s", v, s.Type)
		}
	}
	return s, nil
}

func sortedKeys(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// strictJSON decodes one JSON value and refuses trailing data.
func strictJSON(raw []byte, target any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	if err := dec.Decode(target); err != nil {
		return err
	}
	if dec.More() {
		return errors.New("trailing data after the JSON value")
	}
	var extra json.RawMessage
	if err := dec.Decode(&extra); err == nil {
		return errors.New("trailing data after the JSON value")
	}
	return nil
}

// Validate checks body against the schema. The body must be one JSON value.
func (s *Schema) Validate(body []byte) error {
	var v any
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return fmt.Errorf("%w: %v", ErrBody, err)
	}
	if dec.More() {
		return fmt.Errorf("%w: trailing data after the JSON value", ErrBody)
	}
	var extra json.RawMessage
	if err := dec.Decode(&extra); err == nil {
		return fmt.Errorf("%w: trailing data after the JSON value", ErrBody)
	}
	return s.check(v, "$")
}

func bodyErr(path, format string, args ...any) error {
	return fmt.Errorf("%w: %s: %s", ErrBody, path, fmt.Sprintf(format, args...))
}

func jsonTypeOf(v any) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case string:
		return "string"
	case json.Number:
		if _, err := t.Int64(); err == nil {
			return "integer"
		}
		if f, err := t.Float64(); err == nil && f == math.Trunc(f) && !math.IsInf(f, 0) {
			return "integer"
		}
		return "number"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	}
	return "unknown"
}

// checkType checks only the type of an enum member given as raw JSON.
func (s *Schema) checkType(raw json.RawMessage, path string) error {
	var v any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return err
	}
	return s.typeMatches(v, path)
}

func (s *Schema) typeMatches(v any, path string) error {
	got := jsonTypeOf(v)
	switch {
	case got == s.Type:
	case s.Type == "number" && got == "integer":
	default:
		return bodyErr(path, "is %s, want %s", got, s.Type)
	}
	return nil
}

func (s *Schema) check(v any, path string) error {
	if err := s.typeMatches(v, path); err != nil {
		return err
	}
	if len(s.Enum) > 0 {
		ok := false
		for _, e := range s.Enum {
			var ev any
			dec := json.NewDecoder(bytes.NewReader(e))
			dec.UseNumber()
			if dec.Decode(&ev) == nil && jsonEqual(ev, v) {
				ok = true
				break
			}
		}
		if !ok {
			return bodyErr(path, "is not one of the allowed values")
		}
	}
	switch t := v.(type) {
	case string:
		if s.MaxLength != nil && utf8.RuneCountInString(t) > *s.MaxLength {
			return bodyErr(path, "is longer than %d characters", *s.MaxLength)
		}
	case json.Number:
		if s.Maximum != nil {
			f, err := t.Float64()
			if err != nil || f > *s.Maximum {
				return bodyErr(path, "is above the maximum %v", *s.Maximum)
			}
		}
	case map[string]any:
		for _, name := range s.Required {
			if _, ok := t[name]; !ok {
				return bodyErr(path, "is missing required field %q", name)
			}
		}
		for _, name := range sortedAnyKeys(t) {
			if child, ok := s.Properties[name]; ok {
				if err := child.check(t[name], path+"."+name); err != nil {
					return err
				}
			}
		}
	case []any:
		if s.Items != nil {
			for i, item := range t {
				if err := s.Items.check(item, fmt.Sprintf("%s[%d]", path, i)); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func sortedAnyKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func jsonEqual(a, b any) bool {
	ab, errA := json.Marshal(a)
	bb, errB := json.Marshal(b)
	return errA == nil && errB == nil && string(ab) == string(bb)
}
