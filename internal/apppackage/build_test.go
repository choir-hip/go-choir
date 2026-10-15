package apppackage

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// staticFixture writes a package source tree whose manifest carries its true
// source digest, and returns the directory.
func staticFixture(t *testing.T, name string, mutate func(*Manifest)) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, dir, "frontend/index.js", "export function mount(target) { target.textContent = '"+name+"'; return { destroy() {} }; }\n")
	digest, err := SourceDigest(dir)
	if err != nil {
		t.Fatal(err)
	}
	m := validManifest()
	m.Name = name
	m.Source.Digest = digest
	if mutate != nil {
		mutate(&m)
	}
	raw, err := MarshalManifest(m)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, ManifestFile, string(raw))
	return dir
}

func TestBuildStaticDeterministic(t *testing.T) {
	dir := staticFixture(t, "hello", nil)
	m1, b1, files1, err := BuildStatic(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, b2, _, err := BuildStatic(dir)
	if err != nil {
		t.Fatal(err)
	}
	d1, err := b1.Digest()
	if err != nil {
		t.Fatal(err)
	}
	d2, _ := b2.Digest()
	if d1 != d2 {
		t.Fatalf("static build not deterministic: %s vs %s", d1, d2)
	}
	md, _ := m1.Digest()
	if b1.ManifestDigest != md || b1.SourceDigest != m1.Source.Digest || b1.Recipe != RecipeStaticV1 {
		t.Fatalf("build does not bind its manifest: %+v", b1)
	}
	if err := b1.Validate(); err != nil {
		t.Fatalf("static build invalid: %v", err)
	}
	file, ok := b1.File("frontend/index.js")
	if !ok || file.MediaType != "text/javascript" || file.Size != int64(len(files1["frontend/index.js"])) {
		t.Fatalf("entry file record wrong: %+v ok=%v", file, ok)
	}
}

func TestBuildStaticRefusesSourceDrift(t *testing.T) {
	dir := staticFixture(t, "hello", nil)
	writeFile(t, dir, "frontend/index.js", "export function mount() { /* edited after the manifest */ }\n")
	if _, _, _, err := BuildStatic(dir); !errors.Is(err, ErrSourceMismatch) {
		t.Fatalf("drifted source built: %v", err)
	}
}

func TestBuildStaticRefusesOtherRecipe(t *testing.T) {
	dir := staticFixture(t, "hello", func(m *Manifest) { m.Build.Recipe = RecipeViteModuleV1 })
	if _, _, _, err := BuildStatic(dir); err == nil {
		t.Fatal("static builder accepted a vite-module recipe")
	}
}

func TestBuildStoreRoundTripAndTamper(t *testing.T) {
	dir := staticFixture(t, "hello", nil)
	_, build, files, err := BuildStatic(dir)
	if err != nil {
		t.Fatal(err)
	}
	store := BuildStore{Root: t.TempDir()}
	digest, err := store.Put(build, files)
	if err != nil {
		t.Fatal(err)
	}
	if !store.Has(digest) {
		t.Fatal("stored build not found")
	}
	// Put is idempotent for the same build.
	if again, err := store.Put(build, files); err != nil || again != digest {
		t.Fatalf("idempotent put: %s %v", again, err)
	}
	gotBuild, gotFiles, err := store.Lookup(digest)
	if err != nil {
		t.Fatal(err)
	}
	if d, _ := gotBuild.Digest(); d != digest || string(gotFiles["frontend/index.js"]) != string(files["frontend/index.js"]) {
		t.Fatal("lookup did not return the stored build")
	}
	body, record, err := store.Open(digest, "frontend/index.js")
	if err != nil || string(body) != string(files["frontend/index.js"]) || record.Path != "frontend/index.js" {
		t.Fatalf("open: %v", err)
	}
	if _, _, err := store.Open(digest, "frontend/missing.js"); !errors.Is(err, ErrBuildFileNotFound) {
		t.Fatalf("open of a path outside the receipt: %v", err)
	}
	if _, _, err := store.Open(digest, "../build.json"); err == nil {
		t.Fatal("open escaped the build directory")
	}

	// F12: bytes changed on disk are refused, not served.
	hex := digest[len("sha256:"):]
	path := filepath.Join(store.Root, "builds", hex, "files", "frontend", "index.js")
	if err := os.WriteFile(path, []byte("export function mount() { steal(); }"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Open(digest, "frontend/index.js"); !errors.Is(err, ErrDigestMismatch) {
		t.Fatalf("tampered bytes served: %v", err)
	}
}

func TestBuildStoreRefusesTamper(t *testing.T) {
	dir := staticFixture(t, "hello", nil)
	_, build, files, err := BuildStatic(dir)
	if err != nil {
		t.Fatal(err)
	}
	store := BuildStore{Root: t.TempDir()}
	files["frontend/index.js"] = []byte("export function mount() { steal(); }")
	if _, err := store.Put(build, files); !errors.Is(err, ErrDigestMismatch) {
		t.Fatalf("put accepted bytes that differ from the receipt: %v", err)
	}
	if _, err := store.Put(build, map[string][]byte{}); err == nil {
		t.Fatal("put accepted a build with missing files")
	}
}

func TestBuildStoreLookupMissing(t *testing.T) {
	store := BuildStore{Root: t.TempDir()}
	if _, _, err := store.Lookup(testDigest); !errors.Is(err, ErrBuildNotFound) {
		t.Fatalf("missing build: %v", err)
	}
	if _, _, err := store.Lookup("not-a-digest"); err == nil {
		t.Fatal("malformed digest accepted")
	}
}

func TestBuildStoreList(t *testing.T) {
	store := BuildStore{Root: t.TempDir()}
	builds, err := store.List()
	if err != nil || len(builds) != 0 {
		t.Fatalf("empty store list: %v %v", builds, err)
	}
	_, build, files, err := BuildStatic(staticFixture(t, "hello", nil))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Put(build, files); err != nil {
		t.Fatal(err)
	}
	builds, err = store.List()
	if err != nil || len(builds) != 1 || builds[0].Package != "hello" {
		t.Fatalf("list: %+v %v", builds, err)
	}
}
