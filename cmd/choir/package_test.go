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
	// The second step: the owner confirms the paths the preview showed.
	if code := run([]string{"package", "publish", "--computer", "c1", "--operation", "op-1", "--package", "reader", "--confirm-paths", strings.Repeat("f", 64)}, &stdout, &stderr); code != 0 {
		t.Fatalf("publish exit %d: %s", code, stderr.String())
	}
	if json.Unmarshal(posted, &body) != nil || body["confirm_paths"] != strings.Repeat("f", 64) {
		t.Fatalf("a confirmed publish request = %s", posted)
	}
	// The command says plainly that publishing is public.
	for _, cmd := range packageCommands() {
		if cmd.name() == "package publish" {
			text := strings.ToLower(cmd.summary + " " + cmd.desc)
			for _, want := range []string{"public", "every signed-in owner", "install"} {
				if !strings.Contains(text, want) {
					t.Fatalf("package publish's help %q does not say %q", text, want)
				}
			}
		}
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

// Withdraw and reinstate are owner acts on PATCH
// /api/catalog/records/<id>/listing; share is retired (owner, 2026-11-03: a
// published release is public), so it sends nothing and says why; history
// names one track and a page.
func TestPackageListingVerbsSendTheOwnersChange(t *testing.T) {
	id := strings.Repeat("a", 64)
	var path string
	var posted []byte
	servePackageHost(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.Method + " " + r.URL.RequestURI()
		posted, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"release_id":"r","changed":true}`))
	})
	for _, c := range []struct {
		args []string
		want map[string]any
	}{
		{[]string{"package", "withdraw", "--computer", "c1", "--release", id, "--reason", "broken"},
			map[string]any{"computer_id": "c1", "state": "withdrawn", "reason": "broken"}},
		{[]string{"package", "reinstate", "--computer", "c1", "--release", id},
			map[string]any{"computer_id": "c1", "state": "listed"}},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(c.args, &stdout, &stderr); code != 0 {
			t.Fatalf("%v exit %d: %s", c.args, code, stderr.String())
		}
		var body map[string]any
		if path != "PATCH /api/catalog/records/"+id+"/listing" || json.Unmarshal(posted, &body) != nil {
			t.Fatalf("%v request = %s %s", c.args, path, posted)
		}
		want, _ := json.Marshal(c.want)
		got, _ := json.Marshal(body)
		if string(want) != string(got) {
			t.Fatalf("%v body = %s, want %s", c.args, got, want)
		}
	}
	// Share is a no-op that explains: nothing is sent, whatever it names.
	for _, args := range [][]string{
		{"package", "share", "--computer", "c1", "--release", id, "--owners", "owner-b", "--confirm-paths", strings.Repeat("f", 64)},
		{"package", "share", "--computer", "c1", "--release", id, "--public"},
		{"package", "share", "--computer", "c1", "--release", id, "--private"},
		{"package", "share"},
	} {
		path = ""
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 0 {
			t.Fatalf("%v exit %d: %s", args, code, stderr.String())
		}
		out := stdout.String()
		if path != "" || !strings.Contains(out, "public") || !strings.Contains(out, "withdraw") {
			t.Fatalf("%v sent %q, said %s", args, path, out)
		}
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"package", "history", "--package", "reader", "--track", "owner-b", "--limit", "2", "--before-seq", "3"}, &stdout, &stderr); code != 0 {
		t.Fatalf("history exit %d: %s", code, stderr.String())
	}
	for _, want := range []string{"GET /api/catalog/packages/reader/releases?", "track=owner-b", "limit=2", "before_seq=3"} {
		if !strings.Contains(path, want) {
			t.Fatalf("history request %s lacks %q", path, want)
		}
	}
}
