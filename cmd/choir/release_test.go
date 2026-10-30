package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/release"
)

// P8 at the client (docs/archive/reports/p6-p8-release-adopt-2026-10-15.md): the
// owner adopts exactly the record that hashes to the id they named. A host
// answer that does not verify is refused before the adoption request is
// sent (A1 at the client), and the adoption sends the verified record.

func testReleaseRecord(t *testing.T) (string, json.RawMessage) {
	t.Helper()
	patch := []byte("diff --git a/x b/x\n")
	record := release.Record{
		Schema:    release.Schema,
		Publisher: release.Publisher{ComputerID: "c0", OwnerID: "o0"},
		Operation: release.Operation{
			OperationID: "op-0", BundleDigest: strings.Repeat("b", 64), ReleaseDigest: strings.Repeat("c", 64),
			CodeRef: "code:sha256:" + strings.Repeat("d", 64), DecisionEvent: strings.Repeat("e", 64),
		},
		Source: release.Source{
			BaseCommit: strings.Repeat("1", 40), PatchSHA256: computerevent.DigestBytes(patch), PatchBytes: len(patch),
			TreeHash: strings.Repeat("2", 40), ChangedPaths: []string{"x"},
		},
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

func testReleaseID(t *testing.T) string {
	id, _ := testReleaseRecord(t)
	return id
}

func testReleaseReply(t *testing.T) string {
	id, raw := testReleaseRecord(t)
	out, _ := json.Marshal(map[string]any{"release_id": id, "record": raw})
	return string(out)
}

func TestReleaseAdoptSendsOnlyAVerifiedRecord(t *testing.T) {
	id, raw := testReleaseRecord(t)
	var posted []byte
	serve := func(record json.RawMessage) {
		posted = nil
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.Method == http.MethodPost {
				posted, _ = io.ReadAll(r.Body)
				_, _ = w.Write([]byte(`{"adoption_id":"a"}`))
				return
			}
			out, _ := json.Marshal(map[string]any{"release_id": id, "record": record})
			_, _ = w.Write(out)
		}))
		t.Cleanup(srv.Close)
		t.Setenv(hostEnvVar, srv.URL)
		t.Setenv(apiKeyEnvVar, "choir_sk_parity")
		t.Setenv("CHOIR_COMPUTER", "")
		t.Setenv("CHOIR_API_KEY_FILE", "")
		t.Setenv(timeoutEnvVar, "")
	}
	serve(bytes.Replace(raw, []byte(`"op-0"`), []byte(`"op-9"`), 1))
	var stdout, stderr bytes.Buffer
	if code := run([]string{"release", "adopt", "--computer", "c1", "--release", id}, &stdout, &stderr); code == 0 || posted != nil {
		t.Fatalf("a record that does not verify was adopted: exit %d, posted %s", code, posted)
	}

	serve(raw)
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"release", "adopt", "--computer", "c1", "--release", id}, &stdout, &stderr); code != 0 {
		t.Fatalf("adopt exit %d: %s", code, stderr.String())
	}
	var body struct {
		ReleaseID string          `json:"release_id"`
		Record    json.RawMessage `json:"record"`
	}
	if json.Unmarshal(posted, &body) != nil || body.ReleaseID != id || canonicalJSON(body.Record) != canonicalJSON(raw) {
		t.Fatalf("adoption request = %s", posted)
	}
}
