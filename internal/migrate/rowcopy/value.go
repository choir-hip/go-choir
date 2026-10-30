// Package rowcopy is the generic one-shot table copier behind the data
// system cutover migrators (docs/design/data-system-build-sheet-2026-10-30.md,
// decision 12): it copies every table present in both a MySQL-protocol
// source (a Dolt server or the embedded Dolt driver) and a SQLite or
// PostgreSQL target whose schema already exists, intersecting column names,
// then verifies each table with a row count and a digest.
//
// The package is driver-agnostic (database/sql only); the callers
// (internal/migrate/guestdolt and internal/migrate/hostcopy) own the driver
// imports.
//
// Digest. Each row is encoded canonically (see canon) and hashed with
// SHA-256; a table's digest is the sum of its row hashes modulo 2^256. The sum
// is order independent on purpose: MySQL, SQLite and PostgreSQL order text
// keys differently, so a digest that depended on primary-key order could
// differ across engines for equal data. The source side hashes the value that
// is written to the target; the target side hashes what it reads back, so a
// mismatch means the target stored something other than what was sent.
package rowcopy

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// class is the logical type of a column, decided from the target's declared
// type. Conversion and canonical encoding both follow the target class.
type class int

const (
	classText class = iota
	classInt
	classReal
	classNumeric
	classBool
	classBytes
	classJSON
	classTime
)

func (c class) String() string {
	return [...]string{"text", "int", "real", "numeric", "bool", "bytes", "json", "time"}[c]
}

// canonical-encoding tags.
const (
	tagNull byte = iota
	tagInt
	tagReal
	tagText
	tagBytes
	tagTime
)

// sourceClass maps a MySQL/Dolt information_schema data_type.
func sourceClass(dataType string) class {
	switch strings.ToLower(dataType) {
	case "tinyint", "smallint", "mediumint", "int", "integer", "bigint", "bit", "year":
		return classInt
	case "float", "double", "real":
		return classReal
	case "decimal", "numeric":
		return classNumeric
	case "datetime", "timestamp", "date":
		return classTime
	case "json":
		return classJSON
	case "binary", "varbinary", "tinyblob", "blob", "mediumblob", "longblob":
		return classBytes
	default: // char, varchar, *text, enum, set, time
		return classText
	}
}

// sqliteClass maps a SQLite declared column type.
func sqliteClass(decl string) class {
	d := strings.ToUpper(decl)
	switch {
	case strings.Contains(d, "JSON"):
		return classJSON
	case strings.Contains(d, "DATE") || strings.Contains(d, "TIME"):
		return classTime
	case strings.Contains(d, "BOOL"):
		return classInt
	case strings.Contains(d, "INT"):
		return classInt
	case strings.Contains(d, "CHAR") || strings.Contains(d, "CLOB") || strings.Contains(d, "TEXT"):
		return classText
	case strings.Contains(d, "BLOB") || d == "":
		return classBytes
	case strings.Contains(d, "REAL") || strings.Contains(d, "FLOA") || strings.Contains(d, "DOUB"):
		return classReal
	case strings.Contains(d, "NUM") || strings.Contains(d, "DEC"):
		return classNumeric
	}
	return classText
}

// pgClass maps a PostgreSQL information_schema data_type.
func pgClass(dataType string) class {
	switch strings.ToLower(dataType) {
	case "smallint", "integer", "bigint":
		return classInt
	case "boolean":
		return classBool
	case "double precision", "real":
		return classReal
	case "numeric":
		return classNumeric
	case "jsonb", "json":
		return classJSON
	case "bytea":
		return classBytes
	case "timestamp with time zone", "timestamp without time zone", "date":
		return classTime
	}
	return classText
}

// zeroValue is what a NULL becomes in a NOT NULL target column.
func zeroValue(c class) any {
	switch c {
	case classInt:
		return int64(0)
	case classReal:
		return float64(0)
	case classNumeric:
		return "0"
	case classBool:
		return false
	case classBytes:
		return []byte{}
	case classJSON:
		return "null"
	case classTime:
		return time.Time{}
	}
	return ""
}

// convResult carries side counts out of convert.
type convResult struct {
	coerced  bool // NULL replaced by the zero value of a NOT NULL column
	zeroDate bool // a zero date became NULL
}

// convert turns a value scanned from the source into the Go value written to
// the target column of class tc. kindPG selects PostgreSQL-specific checks.
func convert(v any, tc class, notNull bool, kindPG bool) (any, convResult, error) {
	var res convResult
	if v == nil {
		if notNull {
			res.coerced = true
			return zeroValue(tc), res, nil
		}
		return nil, res, nil
	}
	switch tc {
	case classInt:
		n, err := toInt64(v)
		return n, res, err
	case classBool:
		n, err := toInt64(v)
		return n != 0, res, err
	case classReal:
		f, err := toFloat64(v)
		return f, res, err
	case classNumeric:
		s, err := toText(v)
		if err != nil {
			return nil, res, err
		}
		return strings.TrimSpace(s), res, nil
	case classBytes:
		switch x := v.(type) {
		case []byte:
			if x == nil {
				x = []byte{}
			}
			return x, res, nil
		case string:
			return []byte(x), res, nil
		}
		return nil, res, fmt.Errorf("cannot store %T as bytes", v)
	case classJSON:
		s, err := toText(v)
		if err != nil {
			return nil, res, err
		}
		if kindPG {
			if err := checkPGText(s); err != nil {
				return nil, res, err
			}
		}
		return s, res, nil
	case classTime:
		t, zero, err := toTime(v)
		if err != nil {
			return nil, res, err
		}
		if zero {
			if notNull {
				return time.Time{}, res, nil
			}
			res.zeroDate = true
			return nil, res, nil
		}
		return t, res, nil
	default: // text
		if t, ok := v.(time.Time); ok {
			if t.IsZero() {
				res.zeroDate = true
				return "", res, nil
			}
			return t.UTC().Format(time.RFC3339Nano), res, nil
		}
		s, err := toText(v)
		if err != nil {
			return nil, res, err
		}
		if kindPG {
			if err := checkPGText(s); err != nil {
				return nil, res, err
			}
		}
		return s, res, nil
	}
}

// checkPGText rejects what PostgreSQL text cannot hold, with a message the
// operator can act on (the COPY error would not name the row).
func checkPGText(s string) error {
	if strings.IndexByte(s, 0) >= 0 {
		return &badTextError{reason: "NUL byte"}
	}
	if !utf8.ValidString(s) {
		return &badTextError{reason: "invalid UTF-8"}
	}
	return nil
}

// badTextError is text PostgreSQL cannot store. Never stripped silently: a
// real run fails naming the table, column and primary key; a dry run counts.
type badTextError struct{ reason string }

func (e *badTextError) Error() string {
	return "text contains " + e.reason + ", which PostgreSQL cannot store"
}

func toText(v any) (string, error) {
	switch x := v.(type) {
	case string:
		return x, nil
	case []byte:
		return string(x), nil
	case int64:
		return strconv.FormatInt(x, 10), nil
	case uint64:
		return strconv.FormatUint(x, 10), nil
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64), nil
	case bool:
		if x {
			return "1", nil
		}
		return "0", nil
	}
	return "", fmt.Errorf("cannot store %T as text", v)
}

func toInt64(v any) (int64, error) {
	switch x := v.(type) {
	case int64:
		return x, nil
	case int:
		return int64(x), nil
	case int32:
		return int64(x), nil
	case int16:
		return int64(x), nil
	case int8:
		return int64(x), nil
	case uint64:
		if x > math.MaxInt64 {
			return 0, fmt.Errorf("unsigned value %d exceeds int64; store the column as text", x)
		}
		return int64(x), nil
	case uint32:
		return int64(x), nil
	case uint16:
		return int64(x), nil
	case uint8:
		return int64(x), nil
	case bool:
		if x {
			return 1, nil
		}
		return 0, nil
	case float64:
		if x == math.Trunc(x) && math.Abs(x) < 1<<63 {
			return int64(x), nil
		}
		return 0, fmt.Errorf("non-integral value %v for an integer column", x)
	case []byte:
		return parseInt(string(x))
	case string:
		return parseInt(x)
	}
	return 0, fmt.Errorf("cannot store %T as an integer", v)
}

func parseInt(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return n, nil
	}
	if u, err := strconv.ParseUint(s, 10, 64); err == nil {
		return 0, fmt.Errorf("unsigned value %d exceeds int64; store the column as text", u)
	}
	return 0, fmt.Errorf("value %q is not an integer", clip(s))
}

func toFloat64(v any) (float64, error) {
	switch x := v.(type) {
	case float64:
		return x, nil
	case float32:
		return float64(x), nil
	case []byte:
		return strconv.ParseFloat(strings.TrimSpace(string(x)), 64)
	case string:
		return strconv.ParseFloat(strings.TrimSpace(x), 64)
	}
	n, err := toInt64(v)
	return float64(n), err
}

var timeLayouts = []string{
	"2006-01-02 15:04:05.999999999",
	"2006-01-02T15:04:05.999999999Z07:00",
	"2006-01-02 15:04:05.999999999Z07:00",
	"2006-01-02 15:04:05.999999999-07:00",
	"2006-01-02 15:04:05.999999999 -0700 MST",
	"2006-01-02",
}

// toTime parses a source or read-back time. zero reports a MySQL zero date or
// Go's zero time.
func toTime(v any) (t time.Time, zero bool, err error) {
	switch x := v.(type) {
	case time.Time:
		if x.IsZero() || x.Year() < 1 {
			return time.Time{}, true, nil
		}
		return x.UTC(), false, nil
	case []byte:
		return parseTimeText(string(x))
	case string:
		return parseTimeText(x)
	}
	return time.Time{}, false, fmt.Errorf("cannot store %T as a time", v)
}

func parseTimeText(s string) (time.Time, bool, error) {
	s = strings.TrimSpace(s)
	if s == "" || strings.HasPrefix(s, "0000-00-00") {
		return time.Time{}, true, nil
	}
	for _, l := range timeLayouts {
		if t, err := time.ParseInLocation(l, s, time.UTC); err == nil {
			if t.IsZero() {
				return time.Time{}, true, nil
			}
			return t.UTC(), false, nil
		}
	}
	return time.Time{}, false, fmt.Errorf("unrecognized time %q", clip(s))
}

func clip(s string) string {
	if len(s) > 40 {
		return s[:40] + "..."
	}
	return s
}

// digest accumulates the order-independent table digest.
type digest struct {
	sum   [4]uint64 // little-endian 256-bit accumulator
	count int64
	buf   bytes.Buffer
}

// beginRow starts a row encoding.
func (d *digest) beginRow() { d.buf.Reset() }

// col appends one column to the current row. v is the value written (source
// side) or read back (target side).
func (d *digest) col(c class, v any) error {
	return canon(&d.buf, c, v)
}

// endRow hashes the row and adds it into the accumulator.
func (d *digest) endRow() {
	h := sha256.Sum256(d.buf.Bytes())
	var carry uint64
	for i := 0; i < 4; i++ {
		w := binary.LittleEndian.Uint64(h[i*8:])
		s := d.sum[i] + w
		var c1, c2 uint64
		if s < w {
			c1 = 1
		}
		s2 := s + carry
		if s2 < s {
			c2 = 1
		}
		d.sum[i] = s2
		carry = c1 + c2
	}
	d.count++
}

// Hex is the table digest.
func (d *digest) Hex() string {
	var out [32]byte
	for i := 0; i < 4; i++ {
		binary.BigEndian.PutUint64(out[(3-i)*8:], d.sum[i])
	}
	return hex.EncodeToString(out[:])
}

func putLen(b *bytes.Buffer, n int) {
	var tmp [binary.MaxVarintLen64]byte
	b.Write(tmp[:binary.PutUvarint(tmp[:], uint64(n))])
}

// canon appends the canonical encoding of v as class c. It accepts every Go
// type a driver may return for the class (a SQLite DATETIME may come back as
// time.Time or text, a PostgreSQL jsonb as string or []byte, and so on).
func canon(b *bytes.Buffer, c class, v any) error {
	if v == nil {
		b.WriteByte(tagNull)
		return nil
	}
	switch c {
	case classInt, classBool:
		n, err := toInt64(v)
		if err != nil {
			return err
		}
		b.WriteByte(tagInt)
		b.WriteString(strconv.FormatInt(n, 10))
		b.WriteByte(0)
	case classReal:
		f, err := toFloat64(v)
		if err != nil {
			return err
		}
		b.WriteByte(tagReal)
		b.WriteString(strconv.FormatFloat(f, 'g', -1, 64))
		b.WriteByte(0)
	case classNumeric:
		s, err := toText(v)
		if err != nil {
			return err
		}
		s = normalizeNumeric(s)
		b.WriteByte(tagText)
		putLen(b, len(s))
		b.WriteString(s)
	case classBytes:
		var raw []byte
		switch x := v.(type) {
		case []byte:
			raw = x
		case string:
			raw = []byte(x)
		default:
			return fmt.Errorf("cannot encode %T as bytes", v)
		}
		b.WriteByte(tagBytes)
		putLen(b, len(raw))
		b.Write(raw)
	case classJSON:
		s, err := toText(v)
		if err != nil {
			return err
		}
		s = canonicalJSON(s)
		b.WriteByte(tagText)
		putLen(b, len(s))
		b.WriteString(s)
	case classTime:
		t, zero, err := toTime(v)
		if err != nil {
			return err
		}
		if zero {
			b.WriteByte(tagNull)
			return nil
		}
		s := t.UTC().Truncate(time.Microsecond).Format("2006-01-02T15:04:05.000000Z")
		b.WriteByte(tagTime)
		b.WriteString(s)
		b.WriteByte(0)
	default:
		s, err := toText(v)
		if err != nil {
			return err
		}
		b.WriteByte(tagText)
		putLen(b, len(s))
		b.WriteString(s)
	}
	return nil
}

func normalizeNumeric(s string) string {
	s = strings.TrimSpace(s)
	if strings.ContainsAny(s, "eE") {
		if f, ok := new(big.Float).SetPrec(256).SetString(s); ok {
			s = f.Text('f', -1)
		}
	}
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimSuffix(s, ".")
	}
	if s == "" || s == "-" {
		return "0"
	}
	return s
}

// canonicalJSON re-encodes valid JSON with sorted keys, so PostgreSQL's jsonb
// normalization (key order, whitespace) does not read as a difference. Text
// that is not JSON is returned unchanged.
func canonicalJSON(s string) string {
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return s
	}
	if dec.More() {
		return s
	}
	out, err := json.Marshal(normalizeJSONNumbers(v))
	if err != nil {
		return s
	}
	return string(out)
}

func normalizeJSONNumbers(v any) any {
	switch x := v.(type) {
	case map[string]any:
		for k, e := range x {
			x[k] = normalizeJSONNumbers(e)
		}
		return x
	case []any:
		for i, e := range x {
			x[i] = normalizeJSONNumbers(e)
		}
		return x
	case json.Number:
		s := string(x)
		if strings.ContainsAny(s, "eE") {
			return json.Number(normalizeNumeric(s))
		}
		return x
	}
	return v
}
