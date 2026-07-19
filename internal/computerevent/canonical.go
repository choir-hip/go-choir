package computerevent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"unicode/utf16"
	"unicode/utf8"
)

// CanonicalJSON returns RFC 8785 JSON for the JSON-compatible value v.
// Choir's signed protocols intentionally use only strings, booleans, null,
// arrays, objects, and integral numbers. Floating-point inputs are rejected so
// platform-dependent number rendering cannot enter a signature preimage.
func CanonicalJSON(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("canonical json: marshal: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var decoded any
	if err := dec.Decode(&decoded); err != nil {
		return nil, fmt.Errorf("canonical json: decode: %w", err)
	}
	var out bytes.Buffer
	if err := appendCanonical(&out, decoded); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func appendCanonical(out *bytes.Buffer, v any) error {
	switch value := v.(type) {
	case nil:
		out.WriteString("null")
	case bool:
		if value {
			out.WriteString("true")
		} else {
			out.WriteString("false")
		}
	case string:
		return appendCanonicalString(out, value)
	case json.Number:
		text := value.String()
		if !isCanonicalInteger(text) {
			return fmt.Errorf("canonical json: non-integral number %q is forbidden", text)
		}
		out.WriteString(text)
	case []any:
		out.WriteByte('[')
		for i, item := range value {
			if i > 0 {
				out.WriteByte(',')
			}
			if err := appendCanonical(out, item); err != nil {
				return err
			}
		}
		out.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Slice(keys, func(i, j int) bool { return utf16Less(keys[i], keys[j]) })
		out.WriteByte('{')
		for i, key := range keys {
			if i > 0 {
				out.WriteByte(',')
			}
			if err := appendCanonicalString(out, key); err != nil {
				return err
			}
			out.WriteByte(':')
			if err := appendCanonical(out, value[key]); err != nil {
				return err
			}
		}
		out.WriteByte('}')
	default:
		return fmt.Errorf("canonical json: unsupported value %T", value)
	}
	return nil
}

func appendCanonicalString(out *bytes.Buffer, value string) error {
	if !utf8.ValidString(value) {
		return fmt.Errorf("canonical json: invalid UTF-8 string")
	}
	out.WriteByte('"')
	for _, r := range value {
		switch r {
		case '"', '\\':
			out.WriteByte('\\')
			out.WriteRune(r)
		case '\b':
			out.WriteString(`\b`)
		case '\t':
			out.WriteString(`\t`)
		case '\n':
			out.WriteString(`\n`)
		case '\f':
			out.WriteString(`\f`)
		case '\r':
			out.WriteString(`\r`)
		default:
			if r < 0x20 {
				out.WriteString(`\u`)
				out.WriteString(fmt.Sprintf("%04x", r))
			} else {
				out.WriteRune(r)
			}
		}
	}
	out.WriteByte('"')
	return nil
}

func isCanonicalInteger(value string) bool {
	if value == "0" {
		return true
	}
	if value == "" {
		return false
	}
	start := 0
	if value[0] == '-' {
		if len(value) == 1 || value[1] == '0' {
			return false
		}
		start = 1
	} else if value[0] == '0' {
		return false
	}
	for i := start; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return true
}

func utf16Less(a, b string) bool {
	left := utf16.Encode([]rune(a))
	right := utf16.Encode([]rune(b))
	for i := 0; i < len(left) && i < len(right); i++ {
		if left[i] != right[i] {
			return left[i] < right[i]
		}
	}
	return len(left) < len(right)
}

func canonicalInt(value uint64) string {
	return strconv.FormatUint(value, 10)
}
