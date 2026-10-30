package updater

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

func testSigningKey(t *testing.T) computerevent.SigningKey {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "guest-core", KeyID: "updater-test"}, PrivateKey: privateKey}
}

// G7 of docs/archive/reports/p5-host-build-layered-apply-2026-10-15.md: a layered
// release that declares the commit its binary was built from is healthy only
// when the serving process reports that commit. A base binary still serving
// (the overlay failed, the wrapper fell back) must not read as applied.
func TestHTTPHealthProberRequiresLayeredCodeCommit(t *testing.T) {
	const digest = "1111111111111111111111111111111111111111111111111111111111111111"
	manifest := ReleaseManifest{Marker: "m", EventSchemaVersion: 1, ReducerVersion: 1,
		LayeringEntrypoint: "x-autoputer/bin/autoputer", CodeCommit: "c8fb78349781faef5ab648623919d394171d37ca"}
	serve := func(commit string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "ok", "self_development_marker": "m", "event_schema_version": 1, "reducer_version": 1,
				"release_digest": digest, "build": map[string]any{"commit": commit},
			})
		}))
	}
	base := serve("e83735f2a0000000000000000000000000000000")
	defer base.Close()
	prober := HTTPHealthProber{URL: base.URL, Attempts: 2, Interval: time.Millisecond}
	if _, err := prober.Probe(context.Background(), digest, manifest); err == nil || !strings.Contains(err.Error(), "identity mismatch") {
		t.Fatalf("base binary serving a layered release probed healthy: %v", err)
	}
	layered := serve(manifest.CodeCommit)
	defer layered.Close()
	prober.URL = layered.URL
	if _, err := prober.Probe(context.Background(), digest, manifest); err != nil {
		t.Fatalf("layered binary serving its own commit: %v", err)
	}
	// A plain release declares no code commit and keeps the old contract.
	plain := manifest
	plain.LayeringEntrypoint, plain.CodeCommit = "", ""
	prober.URL = base.URL
	if _, err := prober.Probe(context.Background(), digest, plain); err != nil {
		t.Fatalf("plain release: %v", err)
	}
}

// R1: a restore that changed the exec identity restarts the runtime into
// current; with no current release there is nothing to restart into.
func TestRestartCurrentRestartsIntoCurrentOnly(t *testing.T) {
	root := filepath.Join(t.TempDir(), "updater")
	service := &fakeServiceManager{}
	engine, err := New(root, "computer-test", "realization-test", service, fakeHealthProber{}, testReceiptSigner{key: testSigningKey(t)})
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.RestartCurrent(context.Background()); err == nil || service.restarts != 0 {
		t.Fatalf("restart without a current release: err=%v restarts=%d", err, service.restarts)
	}
	release := filepath.Join(root, "releases", strings.Repeat("a", 64))
	if err := os.MkdirAll(release, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(release, "release-manifest.json"), []byte("{}"), 0o444); err != nil {
		t.Fatal(err)
	}
	if err := swapCurrentPointer(root, release); err != nil {
		t.Fatal(err)
	}
	if err := engine.RestartCurrent(context.Background()); err != nil || service.restarts != 1 {
		t.Fatalf("restart current: err=%v restarts=%d", err, service.restarts)
	}
}

// The layered mint lists the layer's own subtree (its frontend) from the
// closure, so the manifest names exactly what the release dir serves.
func TestExtractClosureSubtreeListsLayerFiles(t *testing.T) {
	blob, err := os.ReadFile(filepath.Join("testdata", "closure-layerdir.nar"))
	if err != nil {
		t.Skipf("closure fixture absent: %v", err)
	}
	target := t.TempDir()
	files, err := ExtractClosureSubtree(blob, "3l816rvxd4qv5xdf3r9lnm1mr1a0cgb1-layerdir", "share", target)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Path != "share/go-choir/build.json" || files[0].Mode != 0o444 {
		t.Fatalf("files = %+v", files)
	}
	raw, err := os.ReadFile(filepath.Join(target, "share", "go-choir", "build.json"))
	if err != nil || !strings.Contains(string(raw), "c8fb78349781faef5ab648623919d394171d37ca") {
		t.Fatalf("extracted = %q %v", raw, err)
	}
	if sum, _ := fileSHA256(filepath.Join(target, "share", "go-choir", "build.json")); sum != files[0].SHA256 {
		t.Fatalf("digest %s != %s", sum, files[0].SHA256)
	}
	bin, err := ExtractClosureSubtree(blob, "3l816rvxd4qv5xdf3r9lnm1mr1a0cgb1-layerdir", "bin", t.TempDir())
	if err != nil || len(bin) != 1 || bin[0].Mode != 0o555 {
		t.Fatalf("bin = %+v %v", bin, err)
	}
	for _, bad := range []struct{ base, sub string }{
		{"3l816rvxd4qv5xdf3r9lnm1mr1a0cgb1-layerdir", "frontend"},
		{"absent-layerdir", "share"},
		{"3l816rvxd4qv5xdf3r9lnm1mr1a0cgb1-layerdir", "../etc"},
	} {
		if _, err := ExtractClosureSubtree(blob, bad.base, bad.sub, t.TempDir()); err == nil {
			t.Fatalf("%s/%s extracted", bad.base, bad.sub)
		}
	}
}
