package apppackage

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
)

// Facts only the platform may state. A manifest that carries one of these keys
// anywhere a declared object is read is refused with ErrPublisherDeclared, so
// a publisher cannot say what its origin, level, class, hash, import set or
// review is. (Any other undeclared key is refused as unknown.)
var publisherDeclared = map[string]bool{
	"origin": true, "kind": true, "class": true, "level": true, "tier": true,
	"publisher": true, "owner": true, "owner_id": true, "track": true,
	"trust": true, "review": true, "reviewed": true, "approved": true,
	"behaviour_surface": true, "behavior_surface": true, "hash": true,
	"digest": true, "sha256": true, "source_digest": true, "artifact": true,
	"artifact_digest": true, "toolchain": true, "binary": true, "wasm": true,
	"import_set": true, "imports": true, "first_party": true,
}

// scanJSON refuses what encoding/json silently tolerates: a duplicate key at
// any depth (a last-wins reader and a first-wins reader would disagree),
// nesting past maxJSONDepth, and anything after the first value.
func scanJSON(raw []byte) error {
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	if err := scanValue(dec, 0); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return invalid("trailing data after the JSON value")
	}
	return nil
}

func scanValue(dec *json.Decoder, depth int) error {
	if depth > maxJSONDepth {
		return invalid("nested deeper than %d", maxJSONDepth)
	}
	tok, err := dec.Token()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidManifest, err)
	}
	d, ok := tok.(json.Delim)
	if !ok {
		return nil
	}
	switch d {
	case '{':
		seen := map[string]bool{}
		for dec.More() {
			k, err := dec.Token()
			if err != nil {
				return fmt.Errorf("%w: %v", ErrInvalidManifest, err)
			}
			key, _ := k.(string)
			if seen[key] {
				return invalid("duplicate key %q", key)
			}
			seen[key] = true
			if err := scanValue(dec, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for dec.More() {
			if err := scanValue(dec, depth+1); err != nil {
				return err
			}
		}
	}
	if _, err := dec.Token(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidManifest, err)
	}
	return nil
}

var rawMessageType = reflect.TypeOf(json.RawMessage{})

// checkKeys walks raw against the Go type t and requires every object key to be
// exactly a declared json name (encoding/json would also accept a different
// case). A key naming a platform fact is ErrPublisherDeclared. json.RawMessage
// fields are opaque and not walked.
func checkKeys(raw json.RawMessage, t reflect.Type, path string) error {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == rawMessageType {
		return nil
	}
	switch t.Kind() {
	case reflect.Slice:
		var items []json.RawMessage
		if json.Unmarshal(raw, &items) != nil {
			return nil // a type error; the decoder reports it
		}
		for i, it := range items {
			if err := checkKeys(it, t.Elem(), fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	case reflect.Struct:
		var obj map[string]json.RawMessage
		if json.Unmarshal(raw, &obj) != nil {
			return nil
		}
		fields := map[string]reflect.Type{}
		for i := 0; i < t.NumField(); i++ {
			tag := strings.Split(t.Field(i).Tag.Get("json"), ",")[0]
			if tag != "" && tag != "-" {
				fields[tag] = t.Field(i).Type
			}
		}
		keys := make([]string, 0, len(obj))
		for key := range obj {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		// Platform facts are named first, so the refusal does not depend on
		// which key sorts first.
		for _, key := range keys {
			if _, ok := fields[key]; !ok && publisherDeclared[strings.ToLower(key)] {
				return fmt.Errorf("%w: %s.%s", ErrPublisherDeclared, path, key)
			}
		}
		for _, key := range keys {
			val := obj[key]
			ft, ok := fields[key]
			if !ok {
				return invalid("%s: unknown field %q", path, key)
			}
			if err := checkKeys(val, ft, path+"."+key); err != nil {
				return err
			}
		}
	}
	return nil
}
