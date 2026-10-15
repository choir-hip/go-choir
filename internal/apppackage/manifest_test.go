package apppackage

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

const testDigest = "sha256:1111111111111111111111111111111111111111111111111111111111111111"

func validManifest() Manifest {
	return Manifest{
		Schema:  ManifestSchemaV1,
		Name:    "hello",
		Version: "0.1.0",
		Display: Display{Title: "Hello", Icon: "👋", Description: "fixture"},
		Source:  SourceRef{Digest: testDigest},
		Frontend: Frontend{
			Entry:  "frontend/index.js",
			ABI:    FrontendABIV1,
			Window: WindowSpec{Singleton: true, Width: 520, Height: 360},
		},
		Build:   BuildSpec{Recipe: RecipeStaticV1, Outputs: []string{"frontend/index.js"}},
		Data:    DataSpec{Store: DataStoreKVV1, SchemaVersion: 1, ReadsSchemaVersions: []int{1}},
		Restore: RestoreSpec{DataScope: DataScopeComputerTape, OnPurge: OnPurgeDeleteData},
		Grants:  []json.RawMessage{},
	}
}

func TestManifestValid(t *testing.T) {
	m := validManifest()
	if err := m.Validate(); err != nil {
		t.Fatalf("valid manifest refused: %v", err)
	}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseManifest(raw)
	if err != nil {
		t.Fatalf("round trip refused: %v", err)
	}
	a, err := m.Digest()
	if err != nil {
		t.Fatal(err)
	}
	b, err := parsed.Digest()
	if err != nil {
		t.Fatal(err)
	}
	if a != b || !IsDigest(a) {
		t.Fatalf("manifest digest not stable across round trip: %s vs %s", a, b)
	}
}

func TestManifestDataNoneIsValid(t *testing.T) {
	m := validManifest()
	m.Data = DataSpec{Store: DataStoreNone}
	if err := m.Validate(); err != nil {
		t.Fatalf("data-less manifest refused: %v", err)
	}
}

func TestManifestBackendShapeIsValid(t *testing.T) {
	// F18 is enforced at install, not here: a manifest may declare the
	// backend it will need once S7-2 lands.
	m := validManifest()
	m.Backend = &Backend{Kind: BackendKindService, Entry: "backend/hello", ABI: BackendABIV1}
	m.Build.Outputs = []string{"backend/hello", "frontend/index.js"}
	if err := m.Validate(); err != nil {
		t.Fatalf("backend-declaring manifest refused: %v", err)
	}
}

func TestManifestRejects(t *testing.T) {
	cases := map[string]func(*Manifest){
		"schema":         func(m *Manifest) { m.Schema = "choir.app-package/v2" },
		"name-empty":     func(m *Manifest) { m.Name = "" },
		"name-shape":     func(m *Manifest) { m.Name = "Hello_World" },
		"reserved-name":  func(m *Manifest) { m.Name = "texture" },
		"version":        func(m *Manifest) { m.Version = "1.0" },
		"title":          func(m *Manifest) { m.Display.Title = " " },
		"icon-long":      func(m *Manifest) { m.Display.Icon = "123456789" },
		"source-digest":  func(m *Manifest) { m.Source.Digest = "sha256:ABC" },
		"entry-dotdot":   func(m *Manifest) { m.Frontend.Entry = "../index.js"; m.Build.Outputs = []string{"../index.js"} },
		"entry-absolute": func(m *Manifest) { m.Frontend.Entry = "/index.js"; m.Build.Outputs = []string{"/index.js"} },
		"entry-backslash": func(m *Manifest) {
			m.Frontend.Entry = `frontend\index.js`
			m.Build.Outputs = []string{`frontend\index.js`}
		},
		"entry-not-output":    func(m *Manifest) { m.Frontend.Entry = "frontend/other.js" },
		"frontend-abi":        func(m *Manifest) { m.Frontend.ABI = "svelte4" },
		"backend-kind":        func(m *Manifest) { m.Backend = &Backend{Kind: "daemon", Entry: "b", ABI: BackendABIV1} },
		"backend-not-output":  func(m *Manifest) { m.Backend = &Backend{Kind: BackendKindService, Entry: "b", ABI: BackendABIV1} },
		"recipe":              func(m *Manifest) { m.Build.Recipe = "make/v1" },
		"outputs-empty":       func(m *Manifest) { m.Build.Outputs = nil },
		"outputs-unsorted":    func(m *Manifest) { m.Build.Outputs = []string{"frontend/index.js", "a.js"} },
		"outputs-duplicate":   func(m *Manifest) { m.Build.Outputs = []string{"frontend/index.js", "frontend/index.js"} },
		"data-store":          func(m *Manifest) { m.Data.Store = "sqlite" },
		"data-window":         func(m *Manifest) { m.Data.ReadsSchemaVersions = []int{2} },
		"data-zero":           func(m *Manifest) { m.Data.SchemaVersion = 0; m.Data.ReadsSchemaVersions = []int{0} },
		"data-none-version":   func(m *Manifest) { m.Data = DataSpec{Store: DataStoreNone, SchemaVersion: 1} },
		"restore-scope":       func(m *Manifest) { m.Restore.DataScope = "external" },
		"restore-on-purge":    func(m *Manifest) { m.Restore.OnPurge = "archive" },
		"grants":              func(m *Manifest) { m.Grants = []json.RawMessage{json.RawMessage(`{"host":"example.com"}`)} },
		"window-negative":     func(m *Manifest) { m.Frontend.Window.Width = -1 },
		"description-too-big": func(m *Manifest) { m.Display.Description = strings.Repeat("x", 201) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			m := validManifest()
			mutate(&m)
			err := m.Validate()
			if err == nil {
				t.Fatalf("manifest with %s accepted", name)
			}
			if !errors.Is(err, ErrInvalidManifest) {
				t.Fatalf("error %v is not ErrInvalidManifest", err)
			}
		})
	}
	t.Run("unknown-field", func(t *testing.T) {
		raw, _ := json.Marshal(validManifest())
		var generic map[string]any
		_ = json.Unmarshal(raw, &generic)
		generic["permissions"] = []string{"all"}
		raw, _ = json.Marshal(generic)
		if _, err := ParseManifest(raw); !errors.Is(err, ErrInvalidManifest) {
			t.Fatalf("unknown field accepted: %v", err)
		}
	})
	t.Run("trailing-data", func(t *testing.T) {
		raw, _ := json.Marshal(validManifest())
		raw = append(raw, []byte(`{}`)...)
		if _, err := ParseManifest(raw); !errors.Is(err, ErrInvalidManifest) {
			t.Fatalf("trailing data accepted: %v", err)
		}
	})
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

// TestReservedNamesCoverStaticRegistry keeps the Go reserved-name list in
// step with the static frontend registry (F2): a package may never take a
// static app's id.
func TestReservedNamesCoverStaticRegistry(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "frontend", "src", "lib", "apps", "registry.ts"))
	if err != nil {
		t.Fatal(err)
	}
	ids := regexp.MustCompile(`(?m)^\s+id: '([a-z0-9-]+)'`).FindAllStringSubmatch(string(raw), -1)
	if len(ids) < 10 {
		t.Fatalf("found only %d static app ids; the registry shape changed, update this test", len(ids))
	}
	for _, id := range ids {
		if !IsReservedName(id[1]) {
			t.Errorf("static app id %q is not a reserved package name", id[1])
		}
	}
	if !IsReservedName("texture") {
		t.Error("TEXTURE_APP_ID (texture) is not reserved")
	}
}

// TestHelloFixtureSourceDigest binds the in-repo fixture's manifest to its
// source tree (F10): editing packages/hello without updating the manifest's
// source digest fails here.
func TestHelloFixtureSourceDigest(t *testing.T) {
	dir := filepath.Join(repoRoot(t), "packages", "hello")
	raw, err := os.ReadFile(filepath.Join(dir, ManifestFile))
	if err != nil {
		t.Fatal(err)
	}
	m, err := ParseManifest(raw)
	if err != nil {
		t.Fatalf("fixture manifest invalid: %v", err)
	}
	if err := VerifySource(dir, m); err != nil {
		got, _ := SourceDigest(dir)
		t.Fatalf("fixture source digest: %v (tree digest is %s)", err, got)
	}
}

func TestSourceDigestIgnoresManifestAndDotfiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "frontend/index.js", "export function mount() {}\n")
	before, err := SourceDigest(dir)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, ManifestFile, "{}")
	writeFile(t, dir, ".DS_Store", "junk")
	after, err := SourceDigest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("manifest or dotfile changed the tree digest: %s -> %s", before, after)
	}
	writeFile(t, dir, "frontend/index.js", "export function mount() { return {}; }\n")
	changed, err := SourceDigest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if changed == before {
		t.Fatal("a source edit did not change the tree digest")
	}
}

func TestSourceDigestRefusesSymlink(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "frontend/index.js", "x")
	if err := os.Symlink("/etc/passwd", filepath.Join(dir, "frontend", "leak.js")); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	if _, err := SourceDigest(dir); err == nil {
		t.Fatal("symlink in a package source tree accepted")
	}
}

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
