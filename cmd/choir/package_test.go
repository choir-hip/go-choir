package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
)

// Library S2 at the client (docs/design/library-package-propagation-2026-10-26.md
// 14): `choir package show` verifies the record against the id the owner
// named, and `choir package publish` sends the owner's act to the publish
// route with the operation and the package, nothing else.

func testPackageRecord(t *testing.T) (string, json.RawMessage) {
	t.Helper()
	parent := pkgrelease.Blob{SHA256: strings.Repeat("6", 64), Bytes: 9}
	record := pkgrelease.Record{
		Schema: pkgrelease.Schema, Package: pkgrelease.Package{Name: "reader", ManifestVersion: "1.0.0"}, Track: "o0",
		Parent:      pkgrelease.Parent{Kind: pkgrelease.ParentBase, Commit: strings.Repeat("1", 40), Tree: strings.Repeat("2", 40)},
		ParentPatch: &parent,
		Source: pkgrelease.Source{
			Root: "frontend/src/packages/reader", Tree: strings.Repeat("3", 40),
			TreePatch: pkgrelease.Blob{SHA256: strings.Repeat("7", 64), Bytes: 9}, ReviewPatch: pkgrelease.Blob{SHA256: strings.Repeat("8", 64), Bytes: 9},
			ChangedPaths: []string{"frontend/src/packages/reader/App.svelte"}, BuiltOn: strings.Repeat("1", 40),
		},
		Manifest:  pkgrelease.Manifest{SHA256: strings.Repeat("4", 64), ABI: "choir.app-frontend/svelte-inline/v1"},
		Publisher: pkgrelease.Publisher{OwnerID: "o0", ComputerID: "c0"},
		Operation: pkgrelease.Operation{OperationID: "op-0", BundleDigest: strings.Repeat("b", 64), DecisionEvent: strings.Repeat("e", 64)},
	}
	id, err := record.ReleaseID()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := record.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	return id, raw
}

func testPackageID(t *testing.T) string {
	id, _ := testPackageRecord(t)
	return id
}

func testPackageReply(t *testing.T) string {
	id, raw := testPackageRecord(t)
	out, _ := json.Marshal(map[string]any{"release_id": id, "record": raw})
	return string(out)
}

func servePackageHost(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	t.Setenv(hostEnvVar, srv.URL)
	t.Setenv(apiKeyEnvVar, "choir_sk_parity")
	t.Setenv("CHOIR_COMPUTER", "")
	t.Setenv("CHOIR_API_KEY_FILE", "")
	t.Setenv(timeoutEnvVar, "")
}

func TestPackageShowVerifiesTheRecordAgainstItsID(t *testing.T) {
	id, raw := testPackageRecord(t)
	serve := func(record json.RawMessage) {
		servePackageHost(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			out, _ := json.Marshal(map[string]any{"release_id": id, "record": record, "listing": map[string]any{"seq": 1}})
			_, _ = w.Write(out)
		})
	}
	serve(bytes.Replace(raw, []byte(`"op-0"`), []byte(`"op-9"`), 1))
	var stdout, stderr bytes.Buffer
	if code := run([]string{"package", "show", "--release", id}, &stdout, &stderr); code == 0 {
		t.Fatalf("a record that does not verify was shown as verified: %s", stdout.String())
	}
	serve(raw)
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"package", "show", "--release", id}, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), `"record_verified": true`) {
		t.Fatalf("show exit %d: %s %s", code, stdout.String(), stderr.String())
	}
}

func TestPackagePublishSendsTheOperationAndPackage(t *testing.T) {
	var path string
	var posted []byte
	servePackageHost(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.Method + " " + r.URL.Path
		posted, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"release_id":"r"}`))
	})
	var stdout, stderr bytes.Buffer
	if code := run([]string{"package", "publish", "--computer", "c1", "--operation", "op-1", "--package", "reader"}, &stdout, &stderr); code != 0 {
		t.Fatalf("publish exit %d: %s", code, stderr.String())
	}
	var body map[string]any
	if path != "POST /api/computers/c1/packages/releases" || json.Unmarshal(posted, &body) != nil || body["operation_id"] != "op-1" || body["package"] != "reader" || len(body) != 2 {
		t.Fatalf("publish request = %s %s", path, posted)
	}
	for _, args := range [][]string{
		{"package", "history", "--package", "reader"},
		{"package", "search", "--query", "read"},
	} {
		stdout.Reset()
		if code := run(args, &stdout, &stderr); code != 0 || !strings.HasPrefix(path, "GET /api/catalog/packages") {
			t.Fatalf("%v exit %d path %s: %s", args, code, path, stderr.String())
		}
	}
}
