package apppackage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// P2b failure modes for the manifest, the schema subset and the registration,
// written before the code (CLAUDE.md "Testing"). The route's are in
// internal/autoputer/app_package_objects_test.go.
//
//	M1  a manifest with an unknown field, trailing data, the wrong schema or
//	    origin, or a desk, backend or grant before their slices is accepted.
//	M2  a package name that is a built-in desk, a desk of the manifest, a shell
//	    name or malformed is accepted: its kinds would be written by a desk it
//	    does not own.
//	M3  a kind outside choir.app.<name>., declared twice, or with a bad identity
//	    or schema version is accepted.
//	M4  a schema that uses a keyword the validator does not enforce, a keyword
//	    on the wrong type, no type, a non-object top level or a depth bomb is
//	    compiled: the manifest then promises a constraint nothing checks.
//	S1  a body of the wrong type, missing a required field, too long, above the
//	    maximum, outside the enum, with a bad nested field or array item, or
//	    that is not one JSON object is accepted; an extra field is refused (it
//	    must pass: extension).
//	G1  the generated table drifts from the manifests under
//	    frontend/src/packages (the node --check is CI's; this is the same check
//	    from Go, so go test catches it without node).
//	G2  RegisterManifest registers a kind whose schema then differs from the one
//	    it registered earlier, or registers kinds of an invalid manifest.

func fixture(t *testing.T) Manifest {
	t.Helper()
	raw, err := os.ReadFile("testdata/p2btest.choir-package.json")
	if err != nil {
		t.Fatal(err)
	}
	m, err := ParseManifest(raw)
	if err != nil {
		t.Fatalf("fixture manifest: %v", err)
	}
	return m
}

func fixtureJSON(t *testing.T, mutate func(map[string]any)) []byte {
	t.Helper()
	raw, err := os.ReadFile("testdata/p2btest.choir-package.json")
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	mutate(v)
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestParseManifestAcceptsTheFixture(t *testing.T) {
	m := fixture(t)
	if m.Name != "p2btest" || len(m.Kinds) != 2 || m.App.Aliases[0] != "p2b" {
		t.Fatalf("manifest = %+v", m)
	}
}

// M1
func TestParseManifestRefusesWhatP1AndTheSlicesDoNotAdmit(t *testing.T) {
	cases := map[string]func(map[string]any){
		"an unknown top-level field": func(v map[string]any) { v["legacy_kinds"] = []any{} },
		"the wrong schema":           func(v map[string]any) { v["schema"] = "choir.app-package/v2" },
		"an installed origin":        func(v map[string]any) { v["origin"] = "installed" },
		"no origin":                  func(v map[string]any) { delete(v, "origin") },
		"a desk":                     func(v map[string]any) { v["desk"] = map[string]any{"verbs": []any{}} },
		"a backend":                  func(v map[string]any) { v["backend"] = map[string]any{"kind": "service"} },
		"a grant":                    func(v map[string]any) { v["grants"] = []any{"net:*"} },
		"a bad version":              func(v map[string]any) { v["version"] = "1.0" },
		"an empty title":             func(v map[string]any) { v["display"] = map[string]any{"title": " "} },
		"the wrong abi":              func(v map[string]any) { v["app"].(map[string]any)["abi"] = "choir.app-frontend/v1" },
		"an escaping entry":          func(v map[string]any) { v["app"].(map[string]any)["entry"] = "../App.svelte" },
		"an alias equal to the name": func(v map[string]any) { v["app"].(map[string]any)["aliases"] = []any{"p2btest"} },
		"a repeated alias":           func(v map[string]any) { v["app"].(map[string]any)["aliases"] = []any{"a1", "a1"} },
		"an extension claimed twice": func(v map[string]any) {
			v["opens"] = []any{map[string]any{"media_type": "a/b", "extensions": []any{"x", "x"}}}
		},
		"an upper-case extension": func(v map[string]any) {
			v["opens"] = []any{map[string]any{"media_type": "a/b", "extensions": []any{"PDF"}}}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseManifest(fixtureJSON(t, mutate)); !errors.Is(err, ErrInvalidManifest) {
				t.Fatalf("err = %v, want ErrInvalidManifest", err)
			}
		})
	}
	good, _ := os.ReadFile("testdata/p2btest.choir-package.json")
	if _, err := ParseManifest(append(good, []byte(`{"x":1}`)...)); !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("trailing data: err = %v", err)
	}
	if _, err := ParseManifest([]byte(`{`)); !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("truncated: err = %v", err)
	}
}

// M2
func TestParseManifestRefusesANameThatIsADeskOrTheShell(t *testing.T) {
	for _, name := range []string{
		"texture", "management", "engineering", "research", "autoradio", // built-in desk writers
		"conductor", "email", "super", "researcher", "co-super", // desks of the manifest table and frozen V1 spellings
		"shell", "owner", "choir", "system", // shell names
		"A", "a", "has_underscore", "x.y", strings.Repeat("a", 33),
	} {
		t.Run(name, func(t *testing.T) {
			raw := fixtureJSON(t, func(v map[string]any) {
				v["name"] = name
				// Distinct kinds, so no case can fail on "declared twice": the
				// name rule is the only thing left to refuse it.
				for i, k := range v["kinds"].([]any) {
					k.(map[string]any)["kind"] = fmt.Sprintf("choir.app.%s.note%d", name, i)
				}
			})
			_, err := ParseManifest(raw)
			if !errors.Is(err, ErrInvalidManifest) {
				t.Fatalf("name %q: err = %v, want ErrInvalidManifest", name, err)
			}
			if strings.Contains(err.Error(), "declared twice") {
				t.Fatalf("name %q was refused for the wrong reason: %v", name, err)
			}
		})
	}
}

// M3
func TestParseManifestRefusesBadKinds(t *testing.T) {
	kind := func(v map[string]any, i int) map[string]any { return v["kinds"].([]any)[i].(map[string]any) }
	cases := map[string]func(map[string]any){
		"a built-in kind":            func(v map[string]any) { kind(v, 0)["kind"] = "choir.texture_revision" },
		"another package's kind":     func(v map[string]any) { kind(v, 0)["kind"] = "choir.app.mail.message" },
		"a prefix-lookalike package": func(v map[string]any) { kind(v, 0)["kind"] = "choir.app.p2btest-x.note" },
		"a nested kind name":         func(v map[string]any) { kind(v, 0)["kind"] = "choir.app.p2btest.a.b" },
		"a kind declared twice":      func(v map[string]any) { kind(v, 1)["kind"] = kind(v, 0)["kind"] },
		"owner edits by content":     func(v map[string]any) { kind(v, 0)["identity"] = "content_hash"; kind(v, 0)["owner_edits"] = true },
		"an unknown identity":        func(v map[string]any) { kind(v, 0)["identity"] = "random" },
		"schema version zero":        func(v map[string]any) { kind(v, 0)["schema_version"] = 0 },
		"reads without the version":  func(v map[string]any) { kind(v, 0)["reads_schema_versions"] = []any{2} },
		"reads unsorted": func(v map[string]any) {
			kind(v, 0)["reads_schema_versions"] = []any{2, 1}
			kind(v, 0)["schema_version"] = 2
		},
		"an unknown kind field":     func(v map[string]any) { kind(v, 0)["writer"] = "owner" },
		"no schema":                 func(v map[string]any) { delete(kind(v, 0), "schema") },
		"a uses_kinds with a colon": func(v map[string]any) { v["uses_kinds"] = []any{"a:b"} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseManifest(fixtureJSON(t, mutate)); !errors.Is(err, ErrInvalidManifest) {
				t.Fatalf("err = %v, want ErrInvalidManifest", err)
			}
		})
	}
}

// M4
func TestCompileSchemaRefusesWhatItCannotEnforce(t *testing.T) {
	deep := `{"type":"string"}`
	for i := 0; i < maxSchemaDepth+2; i++ {
		deep = `{"type":"object","properties":{"a":` + deep + `}}`
	}
	cases := map[string]string{
		"an unsupported keyword":    `{"type":"object","additionalProperties":false}`,
		"a pattern":                 `{"type":"object","properties":{"a":{"type":"string","pattern":"^x"}}}`,
		"no type":                   `{"required":["a"]}`,
		"an unknown type":           `{"type":"date"}`,
		"a non-object top level":    `{"type":"array","items":{"type":"string"}}`,
		"required on a string":      `{"type":"object","properties":{"a":{"type":"string","required":["x"]}}}`,
		"items on an object":        `{"type":"object","items":{"type":"string"}}`,
		"maxLength on a number":     `{"type":"object","properties":{"a":{"type":"number","maxLength":3}}}`,
		"maximum on a string":       `{"type":"object","properties":{"a":{"type":"string","maximum":3}}}`,
		"an enum of the wrong type": `{"type":"object","properties":{"a":{"type":"string","enum":[1]}}}`,
		"an empty enum":             `{"type":"object","properties":{"a":{"type":"string","enum":[]}}}`,
		"a repeated required name":  `{"type":"object","required":["a","a"]}`,
		"a negative maxLength":      `{"type":"object","properties":{"a":{"type":"string","maxLength":-1}}}`,
		"a non-object schema":       `[]`,
		"null":                      `null`,
		"trailing data":             `{"type":"object"} {}`,
		"a depth bomb":              deep,
		"a properties list":         `{"type":"object","properties":[]}`,
		"a non-string description":  `{"type":"object","description":3}`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := CompileSchema([]byte(raw)); !errors.Is(err, ErrSchema) {
				t.Fatalf("err = %v, want ErrSchema", err)
			}
		})
	}
	wide := `{"type":"object","properties":{`
	for i := 0; i <= maxSchemaNodes; i++ {
		if i > 0 {
			wide += ","
		}
		wide += `"p` + strings.Repeat("x", 1) + string(rune('a'+i%26)) + string(rune('a'+(i/26)%26)) + `":{"type":"string"}`
	}
	wide += `}}`
	if _, err := CompileSchema([]byte(wide)); !errors.Is(err, ErrSchema) {
		t.Fatalf("a node bomb: err = %v, want ErrSchema", err)
	}
}

// S1
// S1: an enum of numbers compares values, not spellings. 2.0, 2e0 and 20e-1 are
// the integer 2 and match enum [2], as typeMatches already admits them.
func TestSchemaEnumComparesNumbersAsNumbers(t *testing.T) {
	schema, err := CompileSchema([]byte(`{"type":"object","properties":{"n":{"type":"number","enum":[2,1.5]}}}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{`{"n":2}`, `{"n":2.0}`, `{"n":2e0}`, `{"n":20e-1}`, `{"n":1.5}`, `{"n":1.50}`, `{"n":15e-1}`} {
		if err := schema.Validate([]byte(body)); err != nil {
			t.Errorf("%s: %v, want it in the enum", body, err)
		}
	}
	for _, body := range []string{`{"n":3}`, `{"n":2.5}`, `{"n":"2"}`} {
		if err := schema.Validate([]byte(body)); !errors.Is(err, ErrBody) {
			t.Errorf("%s: err = %v, want ErrBody", body, err)
		}
	}
}

func TestSchemaValidateChecksTheBody(t *testing.T) {
	note := fixture(t).Kinds[0]
	schema, err := CompileSchema(note.Schema)
	if err != nil {
		t.Fatal(err)
	}
	ok := map[string]string{
		"the minimal body":         `{"title":"a","priority":1}`,
		"every field":              `{"title":"a","priority":5,"state":"open","tags":["x","y"],"meta":{"by":"me"}}`,
		"a field it does not name": `{"title":"a","priority":1,"extension":{"x":1}}`,
		"an integer written 2.0":   `{"title":"a","priority":2.0}`,
		"a maximum-length title":   `{"title":"` + strings.Repeat("é", 20) + `","priority":0}`,
	}
	for name, body := range ok {
		if err := schema.Validate([]byte(body)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	bad := map[string]string{
		"not JSON":             `{`,
		"an array":             `[]`,
		"null":                 `null`,
		"a string":             `"x"`,
		"trailing data":        `{"title":"a","priority":1} {}`,
		"a missing required":   `{"title":"a"}`,
		"the wrong type":       `{"title":1,"priority":1}`,
		"a float for integer":  `{"title":"a","priority":1.5}`,
		"a string for integer": `{"title":"a","priority":"1"}`,
		"too long":             `{"title":"` + strings.Repeat("é", 21) + `","priority":1}`,
		"above the maximum":    `{"title":"a","priority":6}`,
		"outside the enum":     `{"title":"a","priority":1,"state":"gone"}`,
		"null for a string":    `{"title":null,"priority":1}`,
		"a bad array item":     `{"title":"a","priority":1,"tags":["x",2]}`,
		"a bad nested field":   `{"title":"a","priority":1,"meta":{"by":3}}`,
		"a missing nested":     `{"title":"a","priority":1,"meta":{}}`,
	}
	for name, body := range bad {
		if err := schema.Validate([]byte(body)); !errors.Is(err, ErrBody) {
			t.Errorf("%s: err = %v, want ErrBody", name, err)
		}
	}
}

// G2
func TestRegisterManifestRegistersTheKindsAndTheirSchemas(t *testing.T) {
	m := fixture(t)
	if err := RegisterManifest(m); err != nil {
		t.Fatal(err)
	}
	if err := RegisterManifest(m); err != nil {
		t.Fatalf("re-registration of the same manifest: %v", err)
	}
	note := objectgraph.ObjectKind("choir.app.p2btest.note")
	reg, ok := objectgraph.LookupPackageKind(note)
	if !ok || reg.Writer != "desk:p2btest" || !reg.OwnerEdits || reg.Schema != "pkg:p2btest@1" {
		t.Fatalf("registration = %+v %v", reg, ok)
	}
	if _, ok := SchemaFor(note); !ok {
		t.Fatal("no schema recorded")
	}
	ledger, _ := objectgraph.LookupPackageKind("choir.app.p2btest.ledger")
	if ledger.OwnerEdits {
		t.Fatal("the ledger kind admits owner edits")
	}

	// The same kind and version with a different schema is a conflict.
	changed := fixtureJSON(t, func(v map[string]any) {
		v["kinds"].([]any)[0].(map[string]any)["schema"] = map[string]any{"type": "object"}
	})
	m2, err := ParseManifest(changed)
	if err != nil {
		t.Fatal(err)
	}
	if err := RegisterManifest(m2); !errors.Is(err, ErrSchemaConflict) {
		t.Fatalf("err = %v, want ErrSchemaConflict", err)
	}
	got, _ := SchemaFor(note)
	if len(got.Properties) == 0 {
		t.Fatal("a refused schema replaced the registered one")
	}

	// An invalid manifest registers nothing.
	bad := fixture(t)
	bad.Name = "p2bbad"
	bad.Kinds = []Kind{{Kind: "choir.texture_revision", Identity: "external_key", SchemaVersion: 1, ReadsSchemaVersions: []int{1}, Schema: bad.Kinds[0].Schema}}
	if err := RegisterManifest(bad); !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("err = %v, want ErrInvalidManifest", err)
	}
	if len(objectgraph.PackageKindsOf("p2bbad")) != 0 {
		t.Fatal("an invalid manifest left a registration")
	}
}

func TestRegisterManifestConcurrentlyWithOneSchema(t *testing.T) {
	m := fixture(t)
	m.Name = "p2bconc"
	for i := range m.Kinds {
		m.Kinds[i].Kind = strings.Replace(m.Kinds[i].Kind, "p2btest", "p2bconc", 1)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- RegisterManifest(m) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if n := len(objectgraph.PackageKindsOf("p2bconc")); n != 2 {
		t.Fatalf("kinds = %d, want 2", n)
	}
}

func TestRegisterFirstPartyIsIdempotentAndRegistersTheTable(t *testing.T) {
	saved := firstPartyPackages
	t.Cleanup(func() { firstPartyPackages = saved })
	m := fixture(t)
	pkg := firstPartyPackage{Name: "p2bfirst"}
	for _, k := range m.Kinds {
		pkg.Kinds = append(pkg.Kinds, firstPartyKind{
			Kind: strings.Replace(k.Kind, "p2btest", "p2bfirst", 1), Identity: k.Identity, Versioned: k.Versioned, OwnerEdits: k.OwnerEdits,
			SchemaVersion: k.SchemaVersion, ReadsSchemaVersions: k.ReadsSchemaVersions, Schema: string(k.Schema),
		})
	}
	firstPartyPackages = []firstPartyPackage{pkg}
	for i := 0; i < 2; i++ {
		if err := RegisterFirstParty(); err != nil {
			t.Fatal(err)
		}
	}
	if n := len(objectgraph.PackageKindsOf("p2bfirst")); n != 2 {
		t.Fatalf("kinds = %d, want 2", n)
	}
	// A broken entry is named and does not stop the others.
	firstPartyPackages = []firstPartyPackage{
		{Name: "texture", Kinds: []firstPartyKind{{Kind: "choir.app.texture.x", Identity: "external_key", SchemaVersion: 1, Schema: `{"type":"object"}`}}},
		pkg,
	}
	err := RegisterFirstParty()
	if err == nil || !strings.Contains(err.Error(), "texture") {
		t.Fatalf("err = %v, want a refusal naming the texture package", err)
	}
	if len(objectgraph.PackageKindsOf("texture")) != 0 {
		t.Fatal("a package named like a desk registered a kind")
	}
}

// G1: the committed table is the one the manifests generate.
func TestGeneratedTableMatchesTheFirstPartyManifests(t *testing.T) {
	dir := filepath.Join("..", "..", "frontend", "src", "packages")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		if len(firstPartyPackages) != 0 {
			t.Fatalf("firstparty_gen.go holds %d packages but %s does not exist", len(firstPartyPackages), dir)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	var want []firstPartyPackage
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name(), ManifestFile))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		m, err := ParseManifest(raw)
		if err != nil {
			t.Fatalf("%s: %v", e.Name(), err)
		}
		if m.Name != e.Name() {
			t.Fatalf("%s: name %q is not the directory name", e.Name(), m.Name)
		}
		if len(m.Kinds) == 0 {
			continue
		}
		p := firstPartyPackage{Name: m.Name}
		for _, k := range m.Kinds {
			p.Kinds = append(p.Kinds, firstPartyKind{
				Kind: k.Kind, Identity: k.Identity, Versioned: k.Versioned, OwnerEdits: k.OwnerEdits,
				SchemaVersion: k.SchemaVersion, ReadsSchemaVersions: k.ReadsSchemaVersions, Schema: canonicalSchema(t, k.Schema),
			})
		}
		want = append(want, p)
	}
	if len(want) == 0 && len(firstPartyPackages) == 0 {
		return
	}
	if !reflect.DeepEqual(want, firstPartyPackages) {
		t.Fatalf("firstparty_gen.go is stale; run node frontend/scripts/gen-package-kinds.mjs\nwant %+v\ngot  %+v", want, firstPartyPackages)
	}
}

func canonicalSchema(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	// Go sorts map keys, as the generator does, and JSON.stringify leaves < > &
	// alone, so the encoder must too.
	var buf strings.Builder
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		t.Fatal(err)
	}
	return strings.TrimSuffix(buf.String(), "\n")
}

// M2, the generated path: RegisterFirstParty reaches the seam without the
// manifest's name check, so the seam alone must refuse a reserved name.
func TestRegisterFirstPartyRefusesAReservedDeskName(t *testing.T) {
	saved := firstPartyPackages
	defer func() { firstPartyPackages = saved }()
	firstPartyPackages = []firstPartyPackage{{Name: "super", Kinds: []firstPartyKind{{
		Kind: "choir.app.super.note", Identity: "external_key", Versioned: true, OwnerEdits: true,
		SchemaVersion: 1, Schema: `{"type":"object"}`,
	}}}}
	if err := RegisterFirstParty(); err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("RegisterFirstParty = %v, want a refusal naming the reserved name", err)
	}
	if _, ok := objectgraph.LookupPackageKind("choir.app.super.note"); ok {
		t.Fatal("a reserved-name kind is registered")
	}
}
