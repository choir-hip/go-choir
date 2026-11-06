package apppackage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/promptroute"
)

// MA2.3 failure modes, written before the code: what a hostile manifest (or a
// careless publisher) could try. Each line is a refusal class and the test
// that pins it.
//
//	H1  Unknown or misspelled keys ride along and a later reader honours them.
//	    Strict decoding, exact-case keys (encoding/json folds case: "Name"),
//	    duplicate keys at any depth (parsers disagree on which wins), trailing
//	    data and oversize or deeply nested documents are all refused.
//	H2  A path escapes the package: "../x", "/etc/x", "a\\b", a dot-file, NUL,
//	    unicode lookalikes, a symlink in the tree, a binary (.so, .wasm, .exe).
//	    Paths are plain ASCII segments with a source extension; DirReader refuses
//	    symlinks and non-regular files.
//	H3  The publisher states a platform fact: origin, level, class, hash,
//	    behaviour_surface, import_set, review, publisher. Refused apart
//	    (ErrPublisherDeclared), at the top level and inside desks and grants.
//	H4  A privileged grant (ui.shell, desk.capsule.mutate, desk.selfdev.*, ...)
//	    from a published origin. Refused for every privileged name; first-party
//	    accepts. Also: a name outside the catalogue, a ui grant with no app, a
//	    desk grant with no desk, an argument on an argless grant, a wildcard host,
//	    a duplicate request, a catalogue family with a bad tail.
//	H5  A desk from a published origin before the Phase 6 sandbox (R24), however
//	    its prompt reads. Accepted on a platform that reports the sandbox.
//	H6  A desk takes another package's desk name, a system desk's or a shell name.
//	H7  Desk code imports unsafe, syscall, os/exec, net, cgo ("C"), reflect,
//	    runtime, embed, or a third-party module, or uses //go:linkname (R42).
//	H8  The behaviour changes and the hash does not: prompt, code, tool, schedule,
//	    accepts, kind schema, uses_kinds, capability argument (an egress host),
//	    desk name. Each changes the hash; authoring order does not.
//	H9  The class is declared rather than inferred.
//	H10 A published package takes the reserved -candidate suffix.
//	H11 A schedule faster than the floor; a desk with no prompt.
//	H12 The v1 reader still reads a v1 manifest (no shim).

func v2JSON(t *testing.T, mutate func(map[string]any)) []byte {
	t.Helper()
	return fixtureJSON(t, mutate)
}

func parseAs(t *testing.T, origin string, mutate func(map[string]any)) (Manifest, error) {
	t.Helper()
	return ParseManifest(v2JSON(t, mutate), origin)
}

func deskV(v map[string]any) {
	v["contributes"] = map[string]any{"desks": []any{map[string]any{
		"name": "p2btest", "prompt": "prompts/desk.md", "code": "desk/main.go",
	}}}
}

// H1
func TestH1StrictKeysDuplicatesAndLimits(t *testing.T) {
	good, err := os.ReadFile("testdata/p2btest.choir-package.json")
	if err != nil {
		t.Fatal(err)
	}
	g := string(good)
	cases := map[string]string{
		"unknown top-level key":   strings.Replace(g, `"kinds"`, `"extra": 1, "kinds"`, 1),
		"case-folded key":         strings.Replace(g, `"name": "p2btest"`, `"Name": "p2btest"`, 1),
		"case-folded nested key":  strings.Replace(g, `"entry"`, `"Entry"`, 1),
		"duplicate top-level key": strings.Replace(g, `"name": "p2btest",`, `"name": "p2btest", "name": "other",`, 1),
		"duplicate nested key":    strings.Replace(g, `"abi"`, `"entry": "App.svelte", "abi"`, 1),
		"trailing data":           g + `{}`,
		"trailing garbage":        g + `x`,
		"empty":                   ``,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseManifest([]byte(raw), OriginFirstParty); !errors.Is(err, ErrInvalidManifest) {
				t.Fatalf("err = %v", err)
			}
		})
	}
	if _, err := ParseManifest([]byte(strings.Repeat(" ", MaxManifestBytes+1)), OriginFirstParty); !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("oversize: %v", err)
	}
	deep := strings.Repeat(`[`, 40) + strings.Repeat(`]`, 40)
	if _, err := ParseManifest([]byte(strings.Replace(g, `"kinds"`, `"x": `+deep+`, "kinds"`, 1)), OriginFirstParty); !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("deep: %v", err)
	}
	// An unknown origin string from the caller is refused too.
	if _, err := ParseManifest(good, "installed"); !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("origin: %v", err)
	}
}

// H2
func TestH2PathContainment(t *testing.T) {
	bad := []string{
		"../App.svelte", "/etc/passwd.svelte", "a/../b.svelte", "a\\b.svelte", ".hidden/App.svelte",
		"a//b.svelte", "a/./b.svelte", "App.svelte\x00.go", "Åpp.svelte", "App.svelte ", "a/b/c/d/e/f/g/h/i.svelte",
		"App.so", "App.wasm", "App.exe", "App.svelte.so", "..svelte", ".svelte", "a..b/App.svelte", strings.Repeat("a", 300) + ".svelte",
	}
	for _, p := range bad {
		if _, err := parseAs(t, OriginFirstParty, func(v map[string]any) { v["app"].(map[string]any)["entry"] = p }); !errors.Is(err, ErrInvalidManifest) {
			t.Errorf("entry %q: err = %v", p, err)
		}
	}
	for _, p := range []string{"prompts/../x.md", "/x.md", "x.bin", "x.so", "../x.txt", "x.md/../../y.md", "p/.x.md"} {
		_, err := parseAs(t, OriginFirstParty, func(v map[string]any) {
			deskV(v)
			v["contributes"].(map[string]any)["desks"].([]any)[0].(map[string]any)["prompt"] = p
		})
		if !errors.Is(err, ErrInvalidManifest) {
			t.Errorf("prompt %q: err = %v", p, err)
		}
	}
	for _, p := range []string{"x.so", "x.wasm", "../m.go", "m.go/../n.go", "m.GO"} {
		for _, field := range []string{"code", "tools"} {
			_, err := parseAs(t, OriginFirstParty, func(v map[string]any) {
				deskV(v)
				d := v["contributes"].(map[string]any)["desks"].([]any)[0].(map[string]any)
				if field == "code" {
					d["code"] = p
				} else {
					d["tools"] = []any{p}
				}
			})
			if !errors.Is(err, ErrInvalidManifest) {
				t.Errorf("%s %q: err = %v", field, p, err)
			}
		}
	}
	// The same file named as code and as a tool is refused.
	_, err := parseAs(t, OriginFirstParty, func(v map[string]any) {
		deskV(v)
		v["contributes"].(map[string]any)["desks"].([]any)[0].(map[string]any)["tools"] = []any{"desk/main.go"}
	})
	if !errors.Is(err, ErrInvalidManifest) {
		t.Errorf("duplicate file: %v", err)
	}
}

func TestH2DirReaderRefusesSymlinksAndEscapes(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.md"), []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ok.md"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "secret.md"), filepath.Join(root, "link.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "dir")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "big.md"), make([]byte, MaxSourceFileBytes+1), 0o644); err != nil {
		t.Fatal(err)
	}
	src := DirSource(root)
	read := src.Read
	if b, err := read("ok.md"); err != nil || string(b) != "ok" {
		t.Fatalf("ok: %q %v", b, err)
	}
	for _, p := range []string{"link.md", "dir/secret.md", "../x.md", "/abs.md", "big.md", "missing.md", "a/../ok.md"} {
		if _, err := read(p); err == nil {
			t.Errorf("DirReader read %q", p)
		}
	}
}

// H3
func TestH3PublisherDeclaredFieldsAreRefusedApart(t *testing.T) {
	top := []string{"origin", "kind", "class", "level", "publisher", "owner", "track", "trust", "review", "behaviour_surface", "hash", "digest", "import_set", "imports", "toolchain", "artifact", "binary", "tier", "first_party"}
	for _, k := range top {
		for _, origin := range []string{OriginFirstParty, OriginPublished} {
			_, err := parseAs(t, origin, func(v map[string]any) { v[k] = "first_party" })
			if !errors.Is(err, ErrPublisherDeclared) || !errors.Is(err, ErrInvalidManifest) {
				t.Errorf("top-level %q (%s): err = %v", k, origin, err)
			}
		}
	}
	nested := map[string]func(v map[string]any, k string){
		"desk": func(v map[string]any, k string) {
			deskV(v)
			v["contributes"].(map[string]any)["desks"].([]any)[0].(map[string]any)[k] = "x"
		},
		"capability": func(v map[string]any, k string) {
			v["capabilities"] = []any{map[string]any{"grant": "ui.captures.read", k: "privileged"}}
		},
		"app":       func(v map[string]any, k string) { v["app"].(map[string]any)[k] = "x" },
		"contribs":  func(v map[string]any, k string) { v["contributes"] = map[string]any{"desks": []any{}, k: "x"} },
		"kind-item": func(v map[string]any, k string) { v["kinds"].([]any)[0].(map[string]any)[k] = "x" },
	}
	for where, mut := range nested {
		for _, k := range []string{"level", "import_set", "behaviour_surface", "origin"} {
			_, err := parseAs(t, OriginFirstParty, func(v map[string]any) { mut(v, k) })
			if !errors.Is(err, ErrPublisherDeclared) {
				t.Errorf("%s.%s: err = %v", where, k, err)
			}
		}
	}
	// An undeclared key that is not a platform fact is merely unknown.
	_, err := parseAs(t, OriginFirstParty, func(v map[string]any) { v["whatever"] = 1 })
	if !errors.Is(err, ErrInvalidManifest) || errors.Is(err, ErrPublisherDeclared) {
		t.Errorf("unknown: %v", err)
	}
}

func withCaps(c ...Capability) func(map[string]any) {
	return func(v map[string]any) {
		deskV(v)
		var list []any
		for _, x := range c {
			e := map[string]any{"grant": x.Grant}
			if x.Args != nil {
				e["args"] = x.Args
			}
			list = append(list, e)
		}
		v["capabilities"] = list
	}
}

// H4
func TestH4EachPrivilegedGrantRefusedFromAPublishedOrigin(t *testing.T) {
	var privileged []string
	for name, s := range grantCatalogue {
		if s.level != LevelPrivileged {
			continue
		}
		if s.family {
			name += ".apply"
		}
		privileged = append(privileged, name)
	}
	if len(privileged) < 13 { // 14 before the model floor was deleted (ec8b4e90d)
		t.Fatalf("catalogue lost privileged grants: %v", privileged)
	}
	for _, name := range privileged {
		spec, _ := lookupGrant(name)
		c := Capability{Grant: name}
		switch spec.args {
		case argOne, argMany:
			c.Args = []string{"example"}
		}
		// ui.shell needs an app and no desk; the others need a desk.
		mut := withCaps(c)
		if strings.HasPrefix(name, "ui.") {
			mut = func(v map[string]any) { v["capabilities"] = []any{map[string]any{"grant": name}} }
		}
		if _, err := parseAs(t, OriginFirstParty, mut); err != nil {
			t.Errorf("first party refused %s: %v", name, err)
		}
		// Granting the sandbox must not rescue a privileged grant.
		m, err := ParseManifest(v2JSON(t, mut), OriginFirstParty)
		if err != nil {
			continue
		}
		m.Origin = OriginPublished
		if err := m.ValidateOn(Platform{DeskSandbox: true}); !errors.Is(err, ErrPrivilegedGrant) {
			t.Errorf("published %s on a sandboxed platform: err = %v", name, err)
		}
		if _, err := parseAs(t, OriginPublished, mut); err == nil {
			t.Errorf("published %s accepted", name)
		}
	}
}

func TestH4GrantShapeRefusals(t *testing.T) {
	cases := map[string][]Capability{
		"not in the catalogue":     {{Grant: "desk.root"}},
		"no domain":                {{Grant: "act"}},
		"v1 spelling":              {{Grant: "net:*"}},
		"argument on argless":      {{Grant: "desk.act", Args: []string{"x"}}},
		"missing argument":         {{Grant: "desk.net.egress"}},
		"two arguments for one":    {{Grant: "desk.net.egress", Args: []string{"a.example", "b.example"}}},
		"wildcard host":            {{Grant: "desk.net.egress", Args: []string{"*"}}},
		"url as host":              {{Grant: "desk.net.egress", Args: []string{"https://a.example/x"}}},
		"upper-case argument":      {{Grant: "desk.net.egress", Args: []string{"A.example"}}},
		"duplicate request":        {{Grant: "desk.act"}, {Grant: "desk.act"}},
		"duplicate with arg order": {{Grant: "desk.request", Args: []string{"a1", "b1"}}, {Grant: "desk.request", Args: []string{"b1", "a1"}}},
		"family bare":              {{Grant: "desk.selfdev"}},
		"family bad tail":          {{Grant: "desk.selfdev.A-b"}},
		"family two levels":        {{Grant: "desk.selfdev.a.b"}},
		"ui grant, no app":         {{Grant: "ui.captures.read"}},
	}
	for name, caps := range cases {
		t.Run(name, func(t *testing.T) {
			mut := withCaps(caps...)
			if name == "ui grant, no app" {
				mut = func(v map[string]any) {
					delete(v, "app")
					setOpens(v, []any{})
					deskV(v)
					v["capabilities"] = []any{map[string]any{"grant": "ui.captures.read"}}
				}
			}
			if _, err := parseAs(t, OriginFirstParty, mut); !errors.Is(err, ErrInvalidManifest) {
				t.Fatalf("err = %v", err)
			}
		})
	}
	// A desk grant with no desk.
	_, err := parseAs(t, OriginFirstParty, func(v map[string]any) { v["capabilities"] = []any{map[string]any{"grant": "desk.act"}} })
	if !errors.Is(err, ErrInvalidManifest) {
		t.Errorf("desk grant without desk: %v", err)
	}
	// A request count over the cap.
	var many []any
	for i := 0; i < maxCapabilities+1; i++ {
		many = append(many, map[string]any{"grant": "ui.captures.read"})
	}
	if _, err := parseAs(t, OriginFirstParty, func(v map[string]any) { v["capabilities"] = many }); !errors.Is(err, ErrInvalidManifest) {
		t.Errorf("too many: %v", err)
	}
}

func TestLevelsAreDerivedFromTheCatalogue(t *testing.T) {
	m := fixture(t)
	if m.Level() != LevelDefault || m.ReviewRequired() {
		t.Fatalf("empty: %s %v", m.Level(), m.ReviewRequired())
	}
	m, err := parseAs(t, OriginFirstParty, func(v map[string]any) {
		v["capabilities"] = []any{map[string]any{"grant": "ui.captures.read"}, map[string]any{"grant": "ui.net.egress", "args": []any{"api.example.com"}}}
	})
	if err != nil || m.Level() != LevelExtended || !m.ReviewRequired() {
		t.Fatalf("extended: %v %s %v", err, m.Level(), m.ReviewRequired())
	}
	m, err = parseAs(t, OriginFirstParty, func(v map[string]any) {
		v["capabilities"] = []any{map[string]any{"grant": "ui.net.egress", "args": []any{"a.example.com"}}, map[string]any{"grant": "ui.shell"}}
	})
	if err != nil || m.Level() != LevelPrivileged {
		t.Fatalf("privileged: %v %s", err, m.Level())
	}
	// Any desk needs review, even with no grant.
	m, err = parseAs(t, OriginFirstParty, deskV)
	if err != nil || m.Level() != LevelDefault || !m.ReviewRequired() {
		t.Fatalf("desk: %v %s %v", err, m.Level(), m.ReviewRequired())
	}
}

// H5
func TestH5DesksFromAPublishedOriginWaitForTheSandbox(t *testing.T) {
	if _, err := parseAs(t, OriginPublished, deskV); !errors.Is(err, ErrDeskNeedsSandbox) {
		t.Fatalf("published desk: %v", err)
	}
	if _, err := parseAs(t, OriginFirstParty, deskV); err != nil {
		t.Fatalf("first-party desk: %v", err)
	}
	m, err := ParseManifest(v2JSON(t, func(v map[string]any) {
		deskV(v)
		setOpens(v, []any{})
		v["app"].(map[string]any)["aliases"] = []any{}
	}), OriginFirstParty)
	if err != nil {
		t.Fatal(err)
	}
	m.Origin = OriginPublished
	if err := m.ValidateOn(Platform{DeskSandbox: true}); err != nil {
		t.Fatalf("published desk on a sandboxed platform: %v", err)
	}
	if err := m.ValidateOn(Platform{}); !errors.Is(err, ErrDeskNeedsSandbox) {
		t.Fatalf("published desk, no sandbox: %v", err)
	}
	if CurrentPlatform().DeskSandbox {
		t.Fatal("this build has no Phase 6 sandbox yet")
	}
	// A front-end-only published package is unaffected (R16).
	if _, err := parseAs(t, OriginPublished, func(v map[string]any) {
		v["kinds"] = []any{}
		v["uses_kinds"] = []any{}
		setOpens(v, []any{})
		v["app"].(map[string]any)["aliases"] = []any{}
	}); err != nil {
		t.Fatalf("published front-end package: %v", err)
	}
}

// H6
func TestH6DeskNames(t *testing.T) {
	for _, name := range []string{"management", "texture", "other", "p2btest_x", "p2b", "P2btest", "p2btest-"} {
		_, err := parseAs(t, OriginFirstParty, func(v map[string]any) {
			deskV(v)
			v["contributes"].(map[string]any)["desks"].([]any)[0].(map[string]any)["name"] = name
		})
		if !errors.Is(err, ErrInvalidManifest) {
			t.Errorf("desk name %q: %v", name, err)
		}
	}
	if _, err := parseAs(t, OriginFirstParty, func(v map[string]any) {
		deskV(v)
		d := v["contributes"].(map[string]any)["desks"].([]any)
		v["contributes"].(map[string]any)["desks"] = append(d, map[string]any{"name": "p2btest", "prompt": "b.md"})
	}); !errors.Is(err, ErrInvalidManifest) {
		t.Errorf("duplicate desk: %v", err)
	}
	if _, err := parseAs(t, OriginFirstParty, func(v map[string]any) {
		deskV(v)
		d := v["contributes"].(map[string]any)["desks"].([]any)
		v["contributes"].(map[string]any)["desks"] = append(d, map[string]any{"name": "p2btest-scribe", "prompt": "b.md"})
	}); err != nil {
		t.Errorf("suffix desk: %v", err)
	}
}

// H7
func TestH7DeskSourceImportGate(t *testing.T) {
	allowed := fixture(t).ImportSet()
	ok := "package main\nimport (\"fmt\"; \"math/big\"; \"choir\")\nfunc main(){ _ = fmt.Sprint(big.NewInt(1)) }\n"
	if err := CheckGoSource("a.go", []byte(ok), allowed); err != nil {
		t.Fatalf("pure imports refused: %v", err)
	}
	bad := map[string]string{
		"unsafe":        "package main\nimport \"unsafe\"\n",
		"syscall":       "package main\nimport \"syscall\"\n",
		"os/exec":       "package main\nimport \"os/exec\"\n",
		"os":            "package main\nimport \"os\"\n",
		"net":           "package main\nimport _ \"net\"\n",
		"net/http":      "package main\nimport h \"net/http\"\n",
		"cgo":           "package main\nimport \"C\"\n",
		"reflect":       "package main\nimport \"reflect\"\n",
		"runtime":       "package main\nimport \"runtime\"\n",
		"embed":         "package main\nimport _ \"embed\"\n",
		"plugin":        "package main\nimport \"plugin\"\n",
		"third party":   "package main\nimport \"github.com/x/y\"\n",
		"outside set":   "package main\nimport \"compress/gzip\"\n",
		"dot import":    "package main\nimport . \"unsafe\"\n",
		"linkname":      "package main\n//go:linkname f runtime.f\nfunc f()\n",
		"not go":        "this is not go",
		"broken import": "package main\nimport (\n",
	}
	for name, src := range bad {
		if err := CheckGoSource("a.go", []byte(src), allowed); !errors.Is(err, ErrInvalidManifest) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	// The gate runs inside the hash: no hash is issued for refused source.
	m, err := ParseManifest(v2JSON(t, deskV), OriginFirstParty)
	if err != nil {
		t.Fatal(err)
	}
	files := mapFiles{"prompts/desk.md": "be careful", "desk/main.go": "package main\nimport \"os/exec\"\n"}
	if _, err := m.BehaviourSurface(files); !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("hash over banned source: %v", err)
	}
}

type mapFiles map[string]string

func (f mapFiles) Read(p string) ([]byte, error) {
	s, ok := f[p]
	if !ok {
		return nil, os.ErrNotExist
	}
	return []byte(s), nil
}

func (f mapFiles) List() ([]string, error) {
	out := make([]string, 0, len(f))
	for k := range f {
		out = append(out, k)
	}
	return out, nil
}

func cloneFiles(f mapFiles) mapFiles {
	out := mapFiles{}
	for k, v := range f {
		out[k] = v
	}
	return out
}

// H8
func TestH8BehaviourSurfaceHashIsStableAndMovesWithEachCoveredInput(t *testing.T) {
	baseMut := func(v map[string]any) {
		deskV(v)
		d := v["contributes"].(map[string]any)["desks"].([]any)[0].(map[string]any)
		d["tools"] = []any{"desk/tool.go"}
		d["schedules"] = []any{map[string]any{"id": "tick", "every_seconds": 3600}}
		d["accepts"] = []any{"summarize a thread"}
		v["capabilities"] = []any{
			map[string]any{"grant": "desk.act"},
			map[string]any{"grant": "desk.net.egress", "args": []any{"api.example.com"}},
		}
	}
	files := mapFiles{
		"prompts/desk.md": "be careful",
		"desk/main.go":    "package main\nimport \"fmt\"\nfunc main(){ fmt.Println(1) }\n",
		"desk/tool.go":    "package main\nfunc tool(){}\n",
	}
	hash := func(t *testing.T, mut func(map[string]any), f mapFiles) string {
		t.Helper()
		m, err := ParseManifest(v2JSON(t, func(v map[string]any) {
			baseMut(v)
			if mut != nil {
				mut(v)
			}
		}), OriginFirstParty)
		if err != nil {
			t.Fatal(err)
		}
		h, err := m.BehaviourSurface(f)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	base := hash(t, nil, files)
	if len(base) != 64 || base != hash(t, nil, files) {
		t.Fatalf("hash unstable or malformed: %s", base)
	}
	desk := func(v map[string]any) map[string]any {
		return v["contributes"].(map[string]any)["desks"].([]any)[0].(map[string]any)
	}
	file := func(k, val string) mapFiles { f := cloneFiles(files); f[k] = val; return f }
	changes := map[string]func(t *testing.T) string{
		"prompt content": func(t *testing.T) string { return hash(t, nil, file("prompts/desk.md", "be reckless")) },
		"code content": func(t *testing.T) string {
			return hash(t, nil, file("desk/main.go", "package main\nimport \"fmt\"\nfunc main(){ fmt.Println(2) }\n"))
		},
		"tool content": func(t *testing.T) string { return hash(t, nil, file("desk/tool.go", "package main\nfunc tool2(){}\n")) },
		"prompt path": func(t *testing.T) string {
			f := cloneFiles(files)
			f["prompts/other.md"] = f["prompts/desk.md"]
			return hash(t, func(v map[string]any) { desk(v)["prompt"] = "prompts/other.md" }, f)
		},
		"schedule period": func(t *testing.T) string {
			return hash(t, func(v map[string]any) { desk(v)["schedules"].([]any)[0].(map[string]any)["every_seconds"] = 7200 }, files)
		},
		"schedule id": func(t *testing.T) string {
			return hash(t, func(v map[string]any) { desk(v)["schedules"].([]any)[0].(map[string]any)["id"] = "tock" }, files)
		},
		"accepts": func(t *testing.T) string {
			return hash(t, func(v map[string]any) { desk(v)["accepts"] = []any{"summarize a thread", "plan"} }, files)
		},
		"egress host": func(t *testing.T) string {
			return hash(t, func(v map[string]any) {
				v["capabilities"].([]any)[1].(map[string]any)["args"] = []any{"evil.example.com"}
			}, files)
		},
		"a capability": func(t *testing.T) string {
			return hash(t, func(v map[string]any) { v["capabilities"] = v["capabilities"].([]any)[:1] }, files)
		},
		"uses_kinds": func(t *testing.T) string {
			return hash(t, func(v map[string]any) { v["uses_kinds"] = []any{"choir.content_item", "choir.other"} }, files)
		},
		"kind schema": func(t *testing.T) string {
			return hash(t, func(v map[string]any) {
				v["kinds"].([]any)[1].(map[string]any)["schema"] = map[string]any{"type": "object", "required": []any{"entry", "more"}}
			}, files)
		},
		"kind identity": func(t *testing.T) string {
			return hash(t, func(v map[string]any) { v["kinds"].([]any)[1].(map[string]any)["owner_edits"] = true }, files)
		},
		"unnamed file": func(t *testing.T) string { return hash(t, nil, file("README.md", "hello")) },
		"kind removed": func(t *testing.T) string {
			return hash(t, func(v map[string]any) { v["kinds"] = v["kinds"].([]any)[:1] }, files)
		},
		"code removed": func(t *testing.T) string { return hash(t, func(v map[string]any) { delete(desk(v), "code") }, files) },
		"second desk": func(t *testing.T) string {
			return hash(t, func(v map[string]any) {
				c := v["contributes"].(map[string]any)
				c["desks"] = append(c["desks"].([]any), map[string]any{"name": "p2btest-b", "prompt": "prompts/desk.md"})
			}, files)
		},
	}
	seen := map[string]string{base: "base"}
	for name, f := range changes {
		got := f(t)
		if got == base {
			t.Errorf("%s did not change the hash", name)
		}
		if prev, dup := seen[got]; dup {
			t.Errorf("%s collides with %s", name, prev)
		}
		seen[got] = name
	}
	// Not covered: authoring order, whitespace and key order, display, window and
	// theme geometry, and the version.
	same := map[string]func(t *testing.T) string{
		"capability order": func(t *testing.T) string {
			return hash(t, func(v map[string]any) { c := v["capabilities"].([]any); c[0], c[1] = c[1], c[0] }, files)
		},
		"display": func(t *testing.T) string {
			return hash(t, func(v map[string]any) { v["display"].(map[string]any)["title"] = "Else" }, files)
		},
		"version": func(t *testing.T) string { return hash(t, func(v map[string]any) { v["version"] = "9.9.9" }, files) },
		"kinds order": func(t *testing.T) string {
			return hash(t, func(v map[string]any) { k := v["kinds"].([]any); k[0], k[1] = k[1], k[0] }, files)
		},
	}
	for name, f := range same {
		if got := f(t); got != base {
			t.Errorf("%s moved the hash", name)
		}
	}
	// A missing file is an error, not an empty hash input.
	m, _ := ParseManifest(v2JSON(t, deskV), OriginFirstParty)
	if _, err := m.BehaviourSurface(mapFiles{"prompts/desk.md": "x"}); !errors.Is(err, ErrInvalidManifest) {
		t.Errorf("missing code file: %v", err)
	}
	if _, err := m.BehaviourSurface(nil); !errors.Is(err, ErrInvalidManifest) {
		t.Errorf("nil reader with desk files: %v", err)
	}
}

func TestBehaviourSurfaceOfAFrontEndOnlyPackageNeedsNoFiles(t *testing.T) {
	a, err := fixture(t).BehaviourSurface(mapFiles{"App.svelte": "x"})
	if err != nil || len(a) != 64 {
		t.Fatalf("%v %q", err, a)
	}
}

func TestImportSetMatchesTheYaegiDefaultPlusChoir(t *testing.T) {
	// internal/yaegikernel is not imported (a heavy dependency); the list is
	// the 16 of DefaultSafeStdlibPackages plus choir. Count and spot-check.
	set := fixture(t).ImportSet()
	if len(set) != 17 {
		t.Fatalf("import set = %v", set)
	}
	for _, banned := range []string{"os", "os/exec", "net", "net/http", "unsafe", "syscall", "reflect", "runtime"} {
		for _, s := range set {
			if s == banned {
				t.Errorf("%s is in the base import set", banned)
			}
		}
	}
	// The set returned is a copy.
	set[0] = "os"
	if fixture(t).ImportSet()[0] == "os" {
		t.Error("ImportSet aliases the base set")
	}
}

// H9
func TestH9ClassIsInferred(t *testing.T) {
	cases := []struct {
		name string
		mut  func(map[string]any)
		want Class
		code bool
	}{
		{"app", func(v map[string]any) {}, ClassApp, true},
		{"app+desk", deskV, ClassAppDesk, true},
		{"desk", func(v map[string]any) { delete(v, "app"); setOpens(v, []any{}); deskV(v) }, ClassDesk, true},
		{"data", func(v map[string]any) { delete(v, "app"); setOpens(v, []any{}) }, ClassData, false},
	}
	for _, c := range cases {
		m, err := parseAs(t, OriginFirstParty, c.mut)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if m.Class() != c.want || m.CodeBearing() != c.code {
			t.Errorf("%s: class %s code %v", c.name, m.Class(), m.CodeBearing())
		}
	}
	// Nothing at all is refused.
	if _, err := parseAs(t, OriginFirstParty, func(v map[string]any) { delete(v, "app"); setOpens(v, []any{}); v["kinds"] = []any{} }); !errors.Is(err, ErrInvalidManifest) {
		t.Errorf("empty package: %v", err)
	}
	// opens without an app.
	if _, err := parseAs(t, OriginFirstParty, func(v map[string]any) { delete(v, "app") }); !errors.Is(err, ErrInvalidManifest) {
		t.Errorf("opens without app: %v", err)
	}
}

// H10, H11
func TestH10H11CandidateSuffixSchedulesAndPrompts(t *testing.T) {
	if _, err := parseAs(t, OriginPublished, func(v map[string]any) { v["name"] = "p2btest-candidate"; v["kinds"] = []any{} }); !errors.Is(err, ErrInvalidManifest) {
		t.Errorf("-candidate: %v", err)
	}
	for name, mut := range map[string]func(d map[string]any){
		"fast schedule":     func(d map[string]any) { d["schedules"] = []any{map[string]any{"id": "x", "every_seconds": 1}} },
		"zero schedule":     func(d map[string]any) { d["schedules"] = []any{map[string]any{"id": "x", "every_seconds": 0}} },
		"negative schedule": func(d map[string]any) { d["schedules"] = []any{map[string]any{"id": "x", "every_seconds": -60}} },
		"duplicate schedule": func(d map[string]any) {
			d["schedules"] = []any{map[string]any{"id": "x", "every_seconds": 60}, map[string]any{"id": "x", "every_seconds": 90}}
		},
		"no prompt":          func(d map[string]any) { delete(d, "prompt") },
		"accepts not a list": func(d map[string]any) { d["accepts"] = map[string]any{"a": 1} },
		"accepts of numbers": func(d map[string]any) { d["accepts"] = []any{1, 2} },
	} {
		_, err := parseAs(t, OriginFirstParty, func(v map[string]any) {
			deskV(v)
			mut(v["contributes"].(map[string]any)["desks"].([]any)[0].(map[string]any))
		})
		if !errors.Is(err, ErrInvalidManifest) {
			t.Errorf("%s: %v", name, err)
		}
	}
	big := make([]any, promptroute.MaxAccepts+1)
	for i := range big {
		big[i] = fmt.Sprintf("intent %d", i)
	}
	if _, err := parseAs(t, OriginFirstParty, func(v map[string]any) {
		deskV(v)
		v["contributes"].(map[string]any)["desks"].([]any)[0].(map[string]any)["accepts"] = big
	}); !errors.Is(err, ErrInvalidManifest) {
		t.Errorf("accepts over cap: %v", err)
	}
}

// H12
func TestH12TheV1ReaderIsGone(t *testing.T) {
	v1 := `{"schema":"choir.app-package/v1","name":"files","version":"1.0.0","origin":"first_party",
	"display":{"title":"F","icon":"","description":""},
	"app":{"entry":"App.svelte","abi":"choir.app-frontend/svelte-inline/v1","aliases":[],"launcher":{},"window":{},"auth":{},"theme":{}},
	"opens":[],"kinds":[],"uses_kinds":[],"desk":null,"backend":null,"grants":[]}`
	for _, origin := range []string{OriginFirstParty, OriginPublished} {
		if _, err := ParseManifest([]byte(v1), origin); !errors.Is(err, ErrInvalidManifest) {
			t.Errorf("v1 accepted from %s: %v", origin, err)
		}
	}
	// Even with its schema line fixed, the v1 fields are refused.
	fixed := strings.Replace(v1, "choir.app-package/v1", "choir.package/v2", 1)
	if _, err := ParseManifest([]byte(fixed), OriginFirstParty); !errors.Is(err, ErrPublisherDeclared) {
		t.Errorf("v1 body under the v2 schema: %v", err)
	}
	var probe map[string]any
	if json.Unmarshal([]byte(v1), &probe) != nil {
		t.Fatal("fixture is not JSON")
	}
}

func TestInTreeManifestsParseAndHash(t *testing.T) {
	dirs, err := filepath.Glob("../../frontend/src/packages/*/choir-package.json")
	if err != nil || len(dirs) == 0 {
		t.Fatalf("no in-tree manifests: %v", err)
	}
	for _, p := range dirs {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		m, err := ParseManifest(raw, OriginFirstParty)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		// Only the two apps that call shell functions hold ui.shell, a privileged
		// grant; a shell-surface provider holds it by definition (H1 to H4).
		privilegedOK := m.Name == "settings" || m.Name == "compute-monitor"
		if m.Class() == ClassProvider {
			if m.Level() != LevelPrivileged {
				t.Errorf("%s: provider at level %s", p, m.Level())
			}
		} else if m.Class() != ClassApp || (m.Level() != LevelDefault && !(privilegedOK && m.Level() == LevelPrivileged)) {
			t.Errorf("%s: class %s level %s", p, m.Class(), m.Level())
		}
		if _, err := m.BehaviourSurface(DirSource(filepath.Dir(p))); err != nil {
			t.Errorf("%s: %v", p, err)
		}
	}
}

// Review follow-ups (Devin, 4bb0af990).

// B2: a published package cannot take a name the shell already holds as an app
// id (static ids) or reserves (check-packages RESERVED_NAMES).
func TestB2PublishedNamesCannotTakeAShellAppName(t *testing.T) {
	names := []string{"texture", "pdf", "epub", "email", "desk", "prompt", "system", "owner", "management", "engineering", "research", "desktop", "shell"}
	for _, n := range names {
		_, err := parseAs(t, OriginPublished, func(v map[string]any) {
			v["name"] = n
			v["kinds"] = []any{}
			v["uses_kinds"] = []any{}
			setOpens(v, []any{})
			v["app"].(map[string]any)["aliases"] = []any{}
		})
		if !errors.Is(err, ErrInvalidManifest) {
			t.Errorf("published %q: %v", n, err)
		}
	}
	if len(ShellAppNames()) < 10 {
		t.Fatalf("the generated shell app name list is short: %v", ShellAppNames())
	}
}

// N1: a directive after the first declaration is scanned, and a file that does
// not fully parse is refused.
func TestN1GoSourceIsFullyParsed(t *testing.T) {
	allowed := fixture(t).ImportSet()
	for name, src := range map[string]string{
		"late linkname": "package main\nfunc f(){}\n//go:linkname g runtime.g\nfunc g()\n",
		"late embed":    "package main\nfunc f(){}\n//go:embed x.txt\nvar s string\n",
		"late cgo":      "package main\nfunc f(){}\n//go:cgo_import_dynamic x\n",
		"broken body":   "package main\nimport \"fmt\"\nfunc f( {\n",
		"unclosed":      "package main\nfunc f(){\n",
	} {
		if err := CheckGoSource("a.go", []byte(src), allowed); !errors.Is(err, ErrInvalidManifest) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if err := CheckGoSource("a.go", []byte("package main\nimport \"fmt\"\nfunc f(){ fmt.Println(1) }\n"), allowed); err != nil {
		t.Errorf("good source: %v", err)
	}
}

// N2: the app block, opens and aliases move the behaviour surface.
func TestN2AppBlockOpensAndAliasesMoveTheHash(t *testing.T) {
	h := func(mut func(map[string]any)) string {
		t.Helper()
		m, err := parseAs(t, OriginFirstParty, mut)
		if err != nil {
			t.Fatal(err)
		}
		s, err := m.BehaviourSurface(mapFiles{"App.svelte": "x"})
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	app := func(v map[string]any) map[string]any { return v["app"].(map[string]any) }
	base := h(func(map[string]any) {})
	for name, mut := range map[string]func(map[string]any){
		"entry":        func(v map[string]any) { app(v)["entry"] = "Other.svelte" },
		"requires_sdk": func(v map[string]any) { app(v)["requires_sdk"] = "^1.2" },
		"alias":        func(v map[string]any) { app(v)["aliases"] = []any{"p2b", "extra"} },
		"opens ext": func(v map[string]any) {
			setOpens(v, []any{map[string]any{"media_type": "application/x-p2btest", "extensions": []any{"p2b", "p3b"}}})
		},
		"opens type": func(v map[string]any) {
			setOpens(v, []any{map[string]any{"media_type": "application/x-other", "extensions": []any{"p2b"}}})
		},
		"opens off":   func(v map[string]any) { setOpens(v, []any{}) },
		"app removed": func(v map[string]any) { delete(v, "app"); setOpens(v, []any{}) },
	} {
		if h(mut) == base {
			t.Errorf("%s did not move the hash", name)
		}
	}
	// Window geometry and theme are presentation, not behaviour.
	if h(func(v map[string]any) { app(v)["window"] = map[string]any{"singleton": false, "heavy": true} }) != base {
		t.Error("window moved the hash")
	}
}

// N3: opens and aliases have no consumer for an installed package, so a
// published manifest may not carry them until one is wired.
func TestN3PublishedManifestsCarryNoOpensOrAliases(t *testing.T) {
	plain := func(v map[string]any) {
		v["kinds"] = []any{}
		v["uses_kinds"] = []any{}
		setOpens(v, []any{})
		v["app"].(map[string]any)["aliases"] = []any{}
	}
	if _, err := parseAs(t, OriginPublished, func(v map[string]any) { v["kinds"] = []any{}; v["uses_kinds"] = []any{} }); !errors.Is(err, ErrInvalidManifest) {
		t.Errorf("published opens and aliases: %v", err)
	}
	if _, err := parseAs(t, OriginPublished, func(v map[string]any) { plain(v); v["app"].(map[string]any)["aliases"] = []any{"zzz"} }); !errors.Is(err, ErrInvalidManifest) {
		t.Errorf("published alias: %v", err)
	}
	if _, err := parseAs(t, OriginPublished, func(v map[string]any) {
		plain(v)
		setOpens(v, []any{map[string]any{"media_type": "a/b", "extensions": []any{"zz"}}})
	}); !errors.Is(err, ErrInvalidManifest) {
		t.Errorf("published opens: %v", err)
	}
	if _, err := parseAs(t, OriginPublished, plain); err != nil {
		t.Errorf("plain published app: %v", err)
	}
}

// Second-seat #2: the hash covers the bytes of every file of the release tree.
func TestSurfaceHashCoversEveryFileOfTheTree(t *testing.T) {
	m := fixture(t)
	files := mapFiles{"App.svelte": "<p>a</p>", "lib/x.js": "export const x = 1", "choir-package.json": "{}"}
	h := func(f mapFiles) string {
		t.Helper()
		got, err := m.BehaviourSurface(f)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	base := h(files)
	for name, mod := range map[string]func(mapFiles){
		"entry bytes":  func(f mapFiles) { f["App.svelte"] = "<p>evil</p>" },
		"imported js":  func(f mapFiles) { f["lib/x.js"] = "export const x = 2" },
		"added file":   func(f mapFiles) { f["lib/y.css"] = "a{}" },
		"removed file": func(f mapFiles) { delete(f, "lib/x.js") },
		"renamed file": func(f mapFiles) { f["lib/z.js"] = f["lib/x.js"]; delete(f, "lib/x.js") },
	} {
		f := cloneFiles(files)
		mod(f)
		if h(f) == base {
			t.Errorf("%s did not move the hash", name)
		}
	}
	// The manifest file is covered field by field, not as bytes.
	f := cloneFiles(files)
	f["choir-package.json"] = `{"display":"changed"}`
	if h(f) != base {
		t.Error("the manifest's bytes moved the hash")
	}
	// An app package with no source to hash is an error, not an empty input.
	if _, err := m.BehaviourSurface(nil); !errors.Is(err, ErrInvalidManifest) {
		t.Errorf("nil source with an app: %v", err)
	}
	// The domain is versioned: the doc shape changed with this coverage.
	if SurfaceDomain != "choir.behaviour-surface/v2" {
		t.Errorf("domain = %s", SurfaceDomain)
	}
}

// The derived import set is not in the hash: a platform release that widens it
// must not move every package's surface.
func TestImportSetIsNotInTheHash(t *testing.T) {
	m := fixture(t)
	files := mapFiles{"App.svelte": "x"}
	before, _ := m.BehaviourSurface(files)
	saved := baseImportSet
	baseImportSet = append(append([]string{}, saved...), "text/tabwriter")
	defer func() { baseImportSet = saved }()
	after, _ := m.BehaviourSurface(files)
	if before != after {
		t.Fatal("the import set moved the surface")
	}
}

// Second-seat non-blocking: the app block is validated in Go for any manifest.
func TestAppBlockIsValidatedInGo(t *testing.T) {
	app := func(v map[string]any) map[string]any { return v["app"].(map[string]any) }
	cases := map[string]func(map[string]any){
		"launcher missing field": func(v map[string]any) { app(v)["launcher"] = map[string]any{"menu": true} },
		"launcher extra": func(v map[string]any) {
			app(v)["launcher"] = map[string]any{"menu": true, "desktopIcon": false, "order": 1, "x": 1}
		},
		"launcher wrong type": func(v map[string]any) {
			app(v)["launcher"] = map[string]any{"menu": "yes", "desktopIcon": false, "order": 1}
		},
		"window missing": func(v map[string]any) { app(v)["window"] = map[string]any{"singleton": true} },
		"window bad geometry": func(v map[string]any) {
			app(v)["window"] = map[string]any{"singleton": true, "heavy": false, "desktop": map[string]any{"width": -1}}
		},
		"auth unknown":         func(v map[string]any) { app(v)["auth"] = map[string]any{"preview": "public-write"} },
		"theme surface":        func(v map[string]any) { t := app(v)["theme"].(map[string]any); t["surface"] = "neon" },
		"theme attr not data-": func(v map[string]any) { t := app(v)["theme"].(map[string]any); t["shellDataAttr"] = "onclick" },
		"theme attr injection": func(v map[string]any) { t := app(v)["theme"].(map[string]any); t["shellDataAttr"] = "data-x onload=1" },
		"theme class":          func(v map[string]any) { t := app(v)["theme"].(map[string]any); t["contentClass"] = "A b" },
		"block absent":         func(v map[string]any) { delete(app(v), "theme") },
		"block null":           func(v map[string]any) { app(v)["auth"] = nil },
	}
	for name, mut := range cases {
		for _, origin := range []string{OriginFirstParty, OriginPublished} {
			if _, err := parseAs(t, origin, func(v map[string]any) { mut(v); setOpens(v, []any{}); app(v)["aliases"] = []any{} }); !errors.Is(err, ErrInvalidManifest) {
				t.Errorf("%s (%s): %v", name, origin, err)
			}
		}
	}
}

// One platform decision: a record and a manifest ask the same question.
func TestDeskAdmissionIsOneDecision(t *testing.T) {
	if (Platform{}).AdmitsDesksFrom(OriginPublished) || !(Platform{DeskSandbox: true}).AdmitsDesksFrom(OriginPublished) ||
		!(Platform{}).AdmitsDesksFrom(OriginFirstParty) {
		t.Fatal("desk admission is wrong")
	}
}

// The manifest's declared-ceiling catalogue (desk.*) and the runtime grant
// catalogue (deskmanifest) name the same powers and agree on who may hold them
// (MA4.1): a grant the runtime marks first-party only is privileged in the
// manifest, and one a package may hold is not. A power the runtime no longer
// has (the management task's model floor, deleted in ec8b4e90d) is in neither.
func TestManifestAndRuntimeGrantCataloguesAgreeOnPrivilege(t *testing.T) {
	pairs := map[string]string{
		"desk.act.cancel":        "act.cancel",
		"desk.captures.read":     "captures.read",
		"desk.intake.read":       "intake.read",
		"desk.cancel.assignment": "cancel.assignment",
		"desk.intake.dispose":    "intake.dispose",
		"desk.precommit.rule":    "precommit.rule",
		"desk.run.deadline":      "run.deadline",
		"desk.product.api":       "product.api",
		"desk.delegate.task":     "delegate.task",
		"desk.capsule.mutate":    "capsule.mutate",
		"desk.world.read":        "world.read",
		"desk.platform.update":   "platform.update",
		"desk.admit.assignment":  "admit.assignment",
	}
	for manifestName, runtimeName := range pairs {
		level, ok := GrantLevel(manifestName)
		if !ok {
			t.Errorf("%s is not in the manifest catalogue", manifestName)
			continue
		}
		privileged := level == LevelPrivileged
		if got := deskmanifest.GrantPrivileged(runtimeName); got != privileged {
			t.Errorf("%s is privileged=%v in the manifest and %s privileged=%v at runtime", manifestName, privileged, runtimeName, got)
		}
	}
	if _, ok := GrantLevel("desk.model.floor"); ok {
		t.Error("desk.model.floor names a power that was deleted")
	}
}

// MA5 review r1 B1: the front end's grant vocabulary (frontend/src/lib/sdk-surface/grants.json,
// which check-packages and the SDK lock's sections.grants read) is this
// catalogue, name for name; and the manifest the front-end gate accepts for a
// submitPrompt use (the ui.shell grant) parses here too.
func TestFrontEndGrantVocabularyIsTheCatalogue(t *testing.T) {
	raw, err := os.ReadFile("../../frontend/src/lib/sdk-surface/grants.json")
	if err != nil {
		t.Fatal(err)
	}
	var front struct {
		Grants map[string]struct {
			Level  string `json:"level"`
			Args   string `json:"args"`
			Family bool   `json:"family"`
		} `json:"grants"`
	}
	if err := json.Unmarshal(raw, &front); err != nil {
		t.Fatal(err)
	}
	argName := map[argRule]string{argNone: "none", argOne: "one", argMany: "many"}
	if len(front.Grants) != len(grantCatalogue) {
		t.Errorf("grants.json has %d grants, the catalogue %d", len(front.Grants), len(grantCatalogue))
	}
	for name, spec := range grantCatalogue {
		f, ok := front.Grants[name]
		if !ok {
			t.Errorf("%s is in the catalogue and not in grants.json", name)
			continue
		}
		if f.Level != string(spec.level) || f.Args != argName[spec.args] || f.Family != spec.family {
			t.Errorf("%s: grants.json %+v, catalogue %+v", name, f, spec)
		}
	}
}

func TestSubmitPromptGrantManifestParses(t *testing.T) {
	raw, err := os.ReadFile("../../frontend/tests/fixtures/ui-shell-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	m, err := ParseManifest(raw, OriginFirstParty)
	if err != nil {
		t.Fatal(err)
	}
	if m.Level() != LevelPrivileged {
		t.Errorf("level %s, want privileged", m.Level())
	}
	if _, err := ParseManifest(raw, OriginPublished); err == nil {
		t.Error("a published package declaring ui.shell must be refused")
	}
}
