package main

// `choir texture read` shows a document's embeds so a coding agent can find
// the evidence digests and fetch them (station TE,
// docs/design/texture-evidence-embeds-te-2026-10-25.md section 5). Ways it
// could fail, written before the code:
//   - the embeds are buried in the revision's body JSON, which read does not
//     even return, so the agent never learns there is evidence;
//   - a row lacks the digest, the operation or the computer, so the agent
//     cannot call `choir self-dev visual` for the bytes;
//   - the fetch line names another operation or computer than the embed;
//   - a document without embeds, or with no revision, changes shape;
//   - a revision read failure fails the read of the document.

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTextureReadListsEmbedsWithTheirFetchCommand(t *testing.T) {
	digest := strings.Repeat("c", 64)
	revisionStatus := http.StatusOK
	body := `{"schema":"choir.texture_doc.v1","doc":{"type":"doc","attrs":{"id":"d"},"content":[` +
		`{"type":"paragraph","attrs":{"id":"p1"},"content":[{"type":"text","text":"Evidence."}]},` +
		`{"type":"embed","attrs":{"id":"embed-1","kind":"capsule_evidence","target":{"digest":"` + digest + `","operation_id":"selfdev-op-1","computer_id":"computer-1","route":"/reader","width":1280,"height":800,"blank":true},"caption":"Blank screenshot of /reader","alt":"Screenshot of /reader"}}]}}`
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/texture/documents/doc-ev":
			_, _ = io.WriteString(w, `{"doc_id":"doc-ev","title":"Reader change","current_revision_id":"rev-1"}`)
		case "/api/texture/documents/doc-plain":
			_, _ = io.WriteString(w, `{"doc_id":"doc-plain","title":"Plain"}`)
		case "/api/texture/revisions/rev-1":
			w.WriteHeader(revisionStatus)
			_, _ = io.WriteString(w, `{"revision_id":"rev-1","doc_id":"doc-ev","body_doc":`+body+`}`)
		default:
			t.Errorf("unexpected request %s", r.URL.String())
			http.NotFound(w, r)
		}
	}))
	defer stub.Close()
	read := func(doc string) (int, map[string]any, string) {
		var out, errOut bytes.Buffer
		code := run([]string{"texture", "read", "--host=" + stub.URL, doc}, &out, &errOut)
		var got map[string]any
		_ = json.Unmarshal(out.Bytes(), &got)
		return code, got, out.String() + errOut.String()
	}

	code, got, raw := read("doc-ev")
	if code != 0 || got["doc_id"] != "doc-ev" || got["title"] != "Reader change" {
		t.Fatalf("read: code %d %s", code, raw)
	}
	embeds, _ := got["embeds"].([]any)
	if len(embeds) != 1 {
		t.Fatalf("embeds = %v (%s)", got["embeds"], raw)
	}
	row, _ := embeds[0].(map[string]any)
	want := map[string]any{
		"block_id": "embed-1", "kind": "capsule_evidence", "digest": digest, "operation_id": "selfdev-op-1",
		"computer_id": "computer-1", "route": "/reader", "width": float64(1280), "height": float64(800), "blank": true,
		"revision_id": "rev-1", "asset_url": "/api/assets/sha256/" + digest,
		"fetch": "choir self-dev visual --computer computer-1 --operation selfdev-op-1 --out <dir>",
	}
	for k, v := range want {
		if row[k] != v {
			t.Fatalf("embed row %s = %v, want %v (%s)", k, row[k], v, raw)
		}
	}

	if code, got, raw := read("doc-plain"); code != 0 || got["embeds"] != nil || got["doc_id"] != "doc-plain" {
		t.Fatalf("a document without a revision: code %d %s", code, raw)
	}
	revisionStatus = http.StatusInternalServerError
	if code, got, raw := read("doc-ev"); code != 0 || got["doc_id"] != "doc-ev" || got["embeds_error"] == nil {
		t.Fatalf("a revision read failure: code %d %s", code, raw)
	}
}
