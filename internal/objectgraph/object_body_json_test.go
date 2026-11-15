package objectgraph

// Release 1 of docs/design/object-bodies-inline-on-the-tape-2026-11-15.md
// (the reader). This file is written before the code. The failure list it
// answers, from the design's sections 2, 3 and 8, restricted to what the
// reader alone can fail:
//
//  R1. An object op carrying "body_json" decodes with an empty Body (an old
//      type ignores the unknown field): the inline form is lost. The reader
//      must set Body to the raw bytes of body_json. (failure 1)
//  R2. "body_json" is carried verbatim, not re-encoded: whitespace, key order,
//      duplicate keys, huge numbers, raw U+2028 and escapes survive bytewise.
//      (failures 4, 7, 8)
//  R3. A "body" key (even "" or null) and "body_json" both present: the
//      reader refuses, it never prefers one. (section 2)
//  R4. "body_json":null: the reader refuses, never reads it as a body or as an
//      absent field. (section 2, failure 6)
//  R5. An empty object body_json ({}) decodes to the two bytes "{}", not to an
//      empty body; a zero-length body (field absent, or "body":"") decodes to
//      a zero-length Body. (failure 6)
//  R6. The base64 form still decodes exactly as before, forever: a fixture
//      from each historical shape (with and without body, with every other
//      field) decodes equal to what encoding/json produced before. If
//      UnmarshalJSON drops the "body" branch this fails. (failure 28)
//  R7. ContentHash is the same function of the bytes in either form, so a
//      decoded inline object and the same object decoded from base64 have
//      equal ContentHash. (section 3)
//  R8. The default marshal of a decoded inline object emits "body", not
//      "body_json", and no MarshalJSON exists on Object: HTTPStore.PutObject
//      keeps emitting base64. (failure 17)
//  R9. The decode copies: mutating the input buffer after Unmarshal does not
//      change Body. (the design's "copy of its raw bytes")
//  R10. A JSON null for the whole object stays a no-op, as before.
//  R11. A reused destination: decoding an inline op into an Object that
//      already holds a Body replaces the body, and a base64 op does too.
//  R12. An old binary (a type without the reader) reads an inline op as a
//      bodyless object. Simulated with a copy of the pre-change struct; this
//      is why batch version 3 is the fence (failure 1).
//  R13. HeaderRow embeds Object and would inherit the promoted UnmarshalJSON,
//      which would silently drop ObjID. No production source decodes a
//      HeaderRow from JSON. (failure 17)
//  R14. Every field other than the body still decodes under the reader
//      (timestamps, tombstone, superseded_by, metadata raw bytes).

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func inlineOp(bodyField string) []byte {
	return []byte(`{"canonical_id":"obj:k:o:1","object_kind":"k","owner_id":"o","content_hash":"sha256:x",` +
		bodyField + `,"metadata":{"a":1},"created_at":"2026-11-15T01:02:03Z","updated_at":"2026-11-15T01:02:04Z"}`)
}

func decodeObject(t *testing.T, raw []byte) (Object, error) {
	t.Helper()
	var obj Object
	err := json.Unmarshal(raw, &obj)
	return obj, err
}

func TestReaderInlineBodyIsCarriedVerbatim(t *testing.T) { // R1, R2
	bodies := []string{
		`{"event_id":"e1","n":1}`,
		`{"b":1,"a":2}`,
		`{"k":1,"k":2}`,
		`{"big":12345678901234567890,"exp":1e400}`,
		`{"u":"  \ud800 <>&"}`,
		"{\"raw\": 1,\n \"ws\" : [ 1 , 2 ]}",
		`[1,2,3]`,
		`"a bare string"`,
		`12`,
		`true`,
	}
	for _, body := range bodies {
		obj, err := decodeObject(t, inlineOp(`"body_json":`+body))
		if err != nil {
			t.Fatalf("body %q: %v", body, err)
		}
		if string(obj.Body) != body {
			t.Fatalf("body %q decoded as %q", body, obj.Body)
		}
	}
}

func TestReaderRefusesBothBodyForms(t *testing.T) { // R3
	both := inlineOp(`"body":"` + base64.StdEncoding.EncodeToString([]byte(`{"a":1}`)) + `","body_json":{"a":1}`)
	if _, err := decodeObject(t, both); err == nil {
		t.Fatal("an op carrying body and body_json decoded; the reader must refuse and never prefer one")
	}
	// The check is on the presence of the "body" key, not on the length of the
	// decoded body: "body":"" and "body":null beside a body_json are two
	// bodies spelled, and the reader never prefers one.
	for _, bodyField := range []string{`"body":""`, `"body":null`} {
		if _, err := decodeObject(t, inlineOp(bodyField+`,"body_json":{"a":1}`)); err == nil {
			t.Fatalf("%s beside body_json decoded; the key's presence is what the reader refuses", bodyField)
		}
		if _, err := decodeObject(t, inlineOp(`"body_json":{"a":1},`+bodyField)); err == nil {
			t.Fatalf("body_json before %s decoded; key order must not matter", bodyField)
		}
	}
	// Duplicate envelope keys: any occurrence of "body" is a present key, and
	// a later "body_json":null is still the refused null.
	for _, fields := range []string{
		`"body":"x","body":null,"body_json":{"a":1}`,
		`"body_json":{"a":1},"body_json":null`,
	} {
		if _, err := decodeObject(t, inlineOp(fields)); err == nil {
			t.Fatalf("duplicate keys %s decoded", fields)
		}
	}
}

func TestReaderRefusesBodyJSONNull(t *testing.T) { // R4
	for _, raw := range []string{
		`"body_json":null`,
		`"body_json":null,"body":""`,
	} {
		if obj, err := decodeObject(t, inlineOp(raw)); err == nil {
			t.Fatalf("%s decoded (Body=%q); null must be an error", raw, obj.Body)
		}
	}
}

func TestReaderEmptyAndZeroLengthBodies(t *testing.T) { // R5
	obj, err := decodeObject(t, inlineOp(`"body_json":{}`))
	if err != nil {
		t.Fatal(err)
	}
	if string(obj.Body) != `{}` {
		t.Fatalf("empty object body_json decoded as %q, want {}", obj.Body)
	}
	for _, raw := range []string{`"x":1`, `"body":""`} {
		obj, err := decodeObject(t, inlineOp(raw))
		if err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
		if len(obj.Body) != 0 {
			t.Fatalf("%s: Body = %q, want zero length", raw, obj.Body)
		}
	}
	// "body_json":"" is the JSON string with no characters (two bytes), a body.
	obj, err = decodeObject(t, inlineOp(`"body_json":""`))
	if err != nil {
		t.Fatal(err)
	}
	if string(obj.Body) != `""` {
		t.Fatalf(`"body_json":"" decoded as %q`, obj.Body)
	}
}

func TestReaderBase64FormIsUnchanged(t *testing.T) { // R6, R14
	body := []byte("not json at all \x00\xff and <html>")
	raw := inlineOp(`"body":"` + base64.StdEncoding.EncodeToString(body) + `"`)
	obj, err := decodeObject(t, raw)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(obj.Body, body) {
		t.Fatalf("base64 body decoded as %q", obj.Body)
	}
	// Every other field, against the stdlib decode of an identical struct
	// that has no UnmarshalJSON (the pre-change behavior).
	type plain Object
	var want plain
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	if obj.CanonicalID != want.CanonicalID || obj.ObjectKind != want.ObjectKind || obj.OwnerID != want.OwnerID ||
		obj.ContentHash != want.ContentHash || !bytes.Equal(obj.Body, want.Body) ||
		string(obj.Metadata) != string(want.Metadata) || !obj.CreatedAt.Equal(want.CreatedAt) || !obj.UpdatedAt.Equal(want.UpdatedAt) {
		t.Fatalf("decode differs from the pre-change decode:\n got %+v\nwant %+v", obj, want)
	}
	full := []byte(`{"canonical_id":"c","object_kind":"k","owner_id":"o","computer_id":"cmp","version_id":"v","content_hash":"h",` +
		`"metadata":{"z":1},"created_at":"2026-11-15T01:02:03Z","updated_at":"2026-11-15T01:02:04Z","tombstone":true,"superseded_by":"s"}`)
	obj, err = decodeObject(t, full)
	if err != nil {
		t.Fatal(err)
	}
	if obj.ComputerID != "cmp" || obj.VersionID != "v" || !obj.Tombstone || obj.SupersededBy != "s" || string(obj.Metadata) != `{"z":1}` {
		t.Fatalf("other fields lost: %+v", obj)
	}
	if len(obj.Body) != 0 {
		t.Fatalf("bodyless op decoded a body: %q", obj.Body)
	}
}

func TestReaderContentHashEqualAcrossForms(t *testing.T) { // R7
	body := `{"event_id":"e1","text":"a b","n":[1,2]}`
	inline, err := decodeObject(t, inlineOp(`"body_json":`+body))
	if err != nil {
		t.Fatal(err)
	}
	b64, err := decodeObject(t, inlineOp(`"body":"`+base64.StdEncoding.EncodeToString([]byte(body))+`"`))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(inline.Body, b64.Body) {
		t.Fatalf("bodies differ: %q vs %q", inline.Body, b64.Body)
	}
	hi := ContentHash(inline.ObjectKind, inline.Body, inline.Metadata)
	hb := ContentHash(b64.ObjectKind, b64.Body, b64.Metadata)
	if hi != hb {
		t.Fatalf("ContentHash differs across forms: %s vs %s", hi, hb)
	}
}

func TestDefaultMarshalOfDecodedInlineObjectEmitsBody(t *testing.T) { // R8
	inline, err := decodeObject(t, inlineOp(`"body_json":{"a":1}`))
	if err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(inline)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out, []byte("body_json")) || !bytes.Contains(out, []byte(`"body":"`+base64.StdEncoding.EncodeToString([]byte(`{"a":1}`))+`"`)) {
		t.Fatalf("default marshal = %s; PutObject must keep emitting base64 body", out)
	}
	pt := func(v any) bool { _, ok := v.(json.Marshaler); return ok }
	if pt(Object{}) || pt(&Object{}) {
		t.Fatal("Object has a MarshalJSON; release 1 changes only the reader")
	}
}

func TestReaderCopiesTheInput(t *testing.T) { // R9
	raw := inlineOp(`"body_json":{"a":"original"}`)
	obj, err := decodeObject(t, raw)
	if err != nil {
		t.Fatal(err)
	}
	for i := range raw {
		raw[i] = 'X'
	}
	if string(obj.Body) != `{"a":"original"}` {
		t.Fatalf("Body aliases the input buffer: %q", obj.Body)
	}
}

func TestReaderNullObjectIsNoOpAndReusedDestinationReplaces(t *testing.T) { // R10, R11
	var obj Object
	if err := json.Unmarshal([]byte(`null`), &obj); err != nil {
		t.Fatal(err)
	}
	if obj.CanonicalID != "" || obj.Body != nil {
		t.Fatalf("null changed the object: %+v", obj)
	}
	obj = Object{Body: []byte("old")}
	if err := json.Unmarshal(inlineOp(`"body_json":{"n":2}`), &obj); err != nil {
		t.Fatal(err)
	}
	if string(obj.Body) != `{"n":2}` {
		t.Fatalf("inline over a reused object: %q", obj.Body)
	}
	if err := json.Unmarshal(inlineOp(`"body":"`+base64.StdEncoding.EncodeToString([]byte("new"))+`"`), &obj); err != nil {
		t.Fatal(err)
	}
	if string(obj.Body) != "new" {
		t.Fatalf("base64 over a reused object: %q", obj.Body)
	}
	var objs []Object
	list := `[` + string(inlineOp(`"body_json":{"a":1}`)) + `,` + string(inlineOp(`"body":"`+base64.StdEncoding.EncodeToString([]byte("zz"))+`"`)) + `]`
	if err := json.Unmarshal([]byte(list), &objs); err != nil || len(objs) != 2 || string(objs[0].Body) != `{"a":1}` || string(objs[1].Body) != "zz" {
		t.Fatalf("mixed list: %v %+v", err, objs)
	}
}

// preChangeObject is the Object type as it was before the reader (no
// UnmarshalJSON, no body_json): the simulation of an old binary, as the F5
// test of event_table_old_binary_test.go simulates one.
type preChangeObject struct {
	CanonicalID string          `json:"canonical_id"`
	ObjectKind  ObjectKind      `json:"object_kind"`
	OwnerID     string          `json:"owner_id"`
	ContentHash string          `json:"content_hash"`
	Body        []byte          `json:"body,omitempty"`
	Metadata    json.RawMessage `json:"metadata"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

func TestOldBinarySimulationReadsInlineOpAsBodylessObject(t *testing.T) { // R12
	raw := inlineOp(`"body_json":{"event_id":"e1"}`)
	var old preChangeObject
	if err := json.Unmarshal(raw, &old); err != nil {
		t.Fatal(err)
	}
	if len(old.Body) != 0 || old.CanonicalID == "" {
		t.Fatalf("simulation premise: old type decoded body=%q id=%q; an old binary must see a bodyless object, which is why version 3 must refuse it", old.Body, old.CanonicalID)
	}
	var now Object
	if err := json.Unmarshal(raw, &now); err != nil || string(now.Body) != `{"event_id":"e1"}` {
		t.Fatalf("the reader must not: %v %q", err, now.Body)
	}
}

// headerRowDecodes returns the position of every Unmarshal, Decode or
// UnmarshalJSON call in file whose argument mentions HeaderRow or names a
// variable declared with it, however the call is split across lines.
func headerRowDecodes(file *ast.File) []token.Pos {
	mentions := func(n ast.Node) bool {
		found := false
		ast.Inspect(n, func(m ast.Node) bool {
			if id, ok := m.(*ast.Ident); ok && id.Name == "HeaderRow" {
				found = true
			}
			return !found
		})
		return found
	}
	var hits []token.Pos
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || (sel.Sel.Name != "Unmarshal" && sel.Sel.Name != "Decode" && sel.Sel.Name != "UnmarshalJSON") {
			return true
		}
		// Only the destination matters: Unmarshal(data, dest) and
		// Decode(dest). A source such as ref.Metadata, or a field of a
		// HeaderRow as the destination, decodes into something else.
		if len(call.Args) == 0 {
			return true
		}
		dest := call.Args[len(call.Args)-1]
		if sel.Sel.Name == "Decode" {
			dest = call.Args[0]
		}
		for {
			switch d := dest.(type) {
			case *ast.UnaryExpr:
				dest = d.X
				continue
			case *ast.ParenExpr:
				dest = d.X
				continue
			case *ast.IndexExpr:
				dest = d.X
				continue
			}
			break
		}
		hit := false
		switch d := dest.(type) {
		case *ast.Ident:
			if d.Name == "HeaderRow" {
				hit = true
			} else if d.Obj != nil && d.Obj.Decl != nil {
				if decl, ok := d.Obj.Decl.(ast.Node); ok && mentions(decl) {
					hit = true
				}
			}
		case *ast.SelectorExpr:
			// a field of something, not the something
		default:
			hit = mentions(dest)
		}
		if hit {
			hits = append(hits, call.Pos())
		}
		return true
	})
	return hits
}

func TestHeaderRowIsNeverJSONDecoded(t *testing.T) { // R13
	// The promoted UnmarshalJSON would decode into the embedded Object and
	// leave ObjID alone, so a JSON decode of a HeaderRow is a bug; fence the
	// sources on the parsed AST, since the type itself cannot refuse (a named
	// field instead of the embed would touch every header.Field use in the scan
	// and the callbacks).
	root := filepath.Join("..", "..")
	fset := token.NewFileSet()
	var offenders []string
	for _, dir := range []string{"internal", "cmd"} {
		_ = filepath.Walk(filepath.Join(root, dir), func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			src, rerr := os.ReadFile(path)
			if rerr != nil || !bytes.Contains(src, []byte("HeaderRow")) {
				return nil
			}
			file, perr := parser.ParseFile(fset, path, src, 0)
			if perr != nil {
				offenders = append(offenders, path+": does not parse: "+perr.Error())
				return nil
			}
			for _, pos := range headerRowDecodes(file) {
				offenders = append(offenders, fset.Position(pos).String())
			}
			return nil
		})
	}
	if len(offenders) > 0 {
		t.Fatalf("a HeaderRow is JSON-decoded (it would drop ObjID through the promoted Object.UnmarshalJSON):\n%s", strings.Join(offenders, "\n"))
	}
}

// The fence must catch what a line match missed: a decode split across lines,
// one through a variable declared with the type, and a slice of it; and it
// must leave a decode of a plain Object alone.
func TestHeaderRowFenceSeesSplitAndIndirectDecodes(t *testing.T) {
	const src = `package p
func f(data []byte) {
	var row HeaderRow
	_ = json.Unmarshal(
		data,
		&row,
	)
	var kept []HeaderRow
	dec.Decode(&kept)
	var other Object
	_ = json.Unmarshal(data, &other)
}`
	file, err := parser.ParseFile(token.NewFileSet(), "p.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	if hits := headerRowDecodes(file); len(hits) != 2 {
		t.Fatalf("the AST match found %d HeaderRow decodes in the probe source, want 2", len(hits))
	}
}
