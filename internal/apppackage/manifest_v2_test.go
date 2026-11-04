package apppackage

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
		"unknown top-level key":   strings.Replace(g, `"opens"`, `"extra": 1, "opens"`, 1),
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
	if _, err := ParseManifest([]byte(strings.Replace(g, `"opens"`, `"x": `+deep+`, "opens"`, 1)), OriginFirstParty); !errors.Is(err, ErrInvalidManifest) {
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
	if err := os.WriteFile(filepath.Join(root, "big.md"), make([]byte, MaxSurfaceFileBytes+1), 0o644); err != nil {
		t.Fatal(err)
	}
	read := DirReader(root)
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
	if len(privileged) < 14 {
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
					v["opens"] = []any{}
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
		v["opens"] = []any{}
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
		v["opens"] = []any{}
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
	if _, err := m.BehaviourSurface(files.read); !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("hash over banned source: %v", err)
	}
}

type mapFiles map[string]string

func (f mapFiles) read(p string) ([]byte, error) {
	s, ok := f[p]
	if !ok {
		return nil, os.ErrNotExist
	}
	return []byte(s), nil
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
		h, err := m.BehaviourSurface(f.read)
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
	// theme geometry, the version, and files the manifest does not name.
	same := map[string]func(t *testing.T) string{
		"capability order": func(t *testing.T) string {
			return hash(t, func(v map[string]any) { c := v["capabilities"].([]any); c[0], c[1] = c[1], c[0] }, files)
		},
		"display": func(t *testing.T) string {
			return hash(t, func(v map[string]any) { v["display"].(map[string]any)["title"] = "Else" }, files)
		},
		"version":      func(t *testing.T) string { return hash(t, func(v map[string]any) { v["version"] = "9.9.9" }, files) },
		"unnamed file": func(t *testing.T) string { return hash(t, nil, file("README.md", "hello")) },
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
	if _, err := m.BehaviourSurface(mapFiles{"prompts/desk.md": "x"}.read); !errors.Is(err, ErrInvalidManifest) {
		t.Errorf("missing code file: %v", err)
	}
	if _, err := m.BehaviourSurface(nil); !errors.Is(err, ErrInvalidManifest) {
		t.Errorf("nil reader with desk files: %v", err)
	}
}

func TestBehaviourSurfaceOfAFrontEndOnlyPackageNeedsNoFiles(t *testing.T) {
	a, err := fixture(t).BehaviourSurface(nil)
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
		{"desk", func(v map[string]any) { delete(v, "app"); v["opens"] = []any{}; deskV(v) }, ClassDesk, true},
		{"data", func(v map[string]any) { delete(v, "app"); v["opens"] = []any{} }, ClassData, false},
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
	if _, err := parseAs(t, OriginFirstParty, func(v map[string]any) { delete(v, "app"); v["opens"] = []any{}; v["kinds"] = []any{} }); !errors.Is(err, ErrInvalidManifest) {
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
	big := make([]any, maxAccepts+1)
	for i := range big {
		big[i] = "x"
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
		if m.Class() != ClassApp || m.Level() != LevelDefault {
			t.Errorf("%s: class %s level %s", p, m.Class(), m.Level())
		}
		if _, err := m.BehaviourSurface(DirReader(filepath.Dir(p))); err != nil {
			t.Errorf("%s: %v", p, err)
		}
	}
}

// Review follow-ups (Devin, 4bb0af990).

// B2: a published package cannot take a name the shell already holds as an app
// id (static ids) or reserves (check-packages RESERVED_NAMES).
func TestB2PublishedNamesCannotTakeAShellAppName(t *testing.T) {
	names := []string{"audio", "pdf", "calendar", "settings", "pulse", "browser", "email", "autopaper", "desk", "prompt", "system", "owner", "management", "engineering", "research", "desktop", "shell"}
	for _, n := range names {
		_, err := parseAs(t, OriginPublished, func(v map[string]any) {
			v["name"] = n
			v["kinds"] = []any{}
			v["uses_kinds"] = []any{}
			v["opens"] = []any{}
			v["app"].(map[string]any)["aliases"] = []any{}
		})
		if !errors.Is(err, ErrInvalidManifest) {
			t.Errorf("published %q: %v", n, err)
		}
	}
	if len(ShellAppNames()) < 15 {
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
		s, err := m.BehaviourSurface(nil)
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
			v["opens"] = []any{map[string]any{"media_type": "application/x-p2btest", "extensions": []any{"p2b", "p3b"}}}
		},
		"opens type": func(v map[string]any) {
			v["opens"] = []any{map[string]any{"media_type": "application/x-other", "extensions": []any{"p2b"}}}
		},
		"opens off":   func(v map[string]any) { v["opens"] = []any{} },
		"app removed": func(v map[string]any) { delete(v, "app"); v["opens"] = []any{} },
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
		v["opens"] = []any{}
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
		v["opens"] = []any{map[string]any{"media_type": "a/b", "extensions": []any{"zz"}}}
	}); !errors.Is(err, ErrInvalidManifest) {
		t.Errorf("published opens: %v", err)
	}
	if _, err := parseAs(t, OriginPublished, plain); err != nil {
		t.Errorf("plain published app: %v", err)
	}
}
