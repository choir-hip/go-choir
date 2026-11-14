package computerevent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
)

// jsonbText spells the receipt the way PostgreSQL's jsonb output does: object
// keys ordered by length and then bytes, ", " between members and ": " after
// a key, strings escaped as PostgreSQL escapes them (quote, backslash, the
// five short control escapes and \u00XX for the other controls; nothing
// else). Receipts the platform stored while the column was jsonb read back in
// this spelling, and the stored text kept it when the column became TEXT.
// CompactHeadReceipt returns a jsonb-format value only when this rebuilds the
// stored bytes exactly.
func (r Receipt) jsonbText() ([]byte, error) {
	values, err := r.values(true)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var tree any
	if err := decoder.Decode(&tree); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := writeJSONB(&out, tree); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func writeJSONB(out *bytes.Buffer, v any) error {
	switch v := v.(type) {
	case nil:
		out.WriteString("null")
	case bool:
		out.WriteString(strconv.FormatBool(v))
	case json.Number:
		out.WriteString(v.String())
	case string:
		writeJSONBString(out, v)
	case []any:
		out.WriteByte('[')
		for i, item := range v {
			if i > 0 {
				out.WriteString(", ")
			}
			if err := writeJSONB(out, item); err != nil {
				return err
			}
		}
		out.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			if len(keys[i]) != len(keys[j]) {
				return len(keys[i]) < len(keys[j])
			}
			return keys[i] < keys[j]
		})
		out.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				out.WriteString(", ")
			}
			writeJSONBString(out, k)
			out.WriteString(": ")
			if err := writeJSONB(out, v[k]); err != nil {
				return err
			}
		}
		out.WriteByte('}')
	default:
		return fmt.Errorf("jsonb text: unsupported %T", v)
	}
	return nil
}

func writeJSONBString(out *bytes.Buffer, s string) {
	out.WriteByte('"')
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '"':
			out.WriteString(`\"`)
		case '\\':
			out.WriteString(`\\`)
		case '\b':
			out.WriteString(`\b`)
		case '\f':
			out.WriteString(`\f`)
		case '\n':
			out.WriteString(`\n`)
		case '\r':
			out.WriteString(`\r`)
		case '\t':
			out.WriteString(`\t`)
		default:
			if c < 0x20 {
				fmt.Fprintf(out, `\u%04x`, c)
			} else {
				out.WriteByte(c)
			}
		}
	}
	out.WriteByte('"')
}
