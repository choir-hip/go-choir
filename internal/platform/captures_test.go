package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/sources"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

type fakeGuestLookup map[string]*vmctl.GuestOwnership

func (f fakeGuestLookup) LookupGuestContext(_ context.Context, remoteAddr string) (*vmctl.GuestOwnership, error) {
	host := strings.Split(remoteAddr, ":")[0]
	if o, ok := f[host]; ok {
		return o, nil
	}
	return &vmctl.GuestOwnership{Found: false}, nil
}

// W1b2 (docs/definitions/choir-autopaper-w1-content-in-the-graph-2026-10-11.md):
// captures are shared content in Store B. Failure modes pinned: a body
// reaches a Dolt row; a guest writes captures owned by another computer; the
// metadata names a body that is not on disk; a re-poll writes duplicates; a
// second computer cannot read a capture's body; an oversized request is
// accepted.
func TestCaptureRouteWritesSharedCapturesWithBodiesOnHostDisk(t *testing.T) {
	mem := objectgraph.NewMemoryStore()
	svc := objectgraph.NewService(objectgraph.Config{Memory: mem, Durable: mem})
	defer svc.Close()
	h := NewObjectGraphHandler(svc, mem)
	paper := &vmctl.GuestOwnership{Found: true, UserID: "paper-owner", ComputerID: "computer-paper"}
	reader := &vmctl.GuestOwnership{Found: true, UserID: "reader-owner", ComputerID: "computer-reader"}
	if err := h.ConfigureGuestBinding(fakeGuestLookup{"10.200.1.2": paper, "10.200.9.9": reader}); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	h.ConfigureCaptureBodies(root)

	fetched := time.Date(2026, 10, 11, 10, 0, 0, 0, time.UTC)
	items := []sources.Item{
		{ID: "item-1", SourceID: "rss:example", SourceType: sources.SourceTypeRSS, Title: "One", Body: "Body one", URL: "https://example.com/1", FetchedAt: fetched},
		{ID: "item-2", SourceID: "rss:example", SourceType: sources.SourceTypeRSS, Title: "Two", Body: "Body two", URL: "https://example.com/2", FetchedAt: fetched},
	}
	post := func(from, owner, computer string, items []sources.Item) *httptest.ResponseRecorder {
		t.Helper()
		raw, _ := json.Marshal(CaptureWriteRequest{OwnerID: owner, ComputerID: computer, Items: items})
		req := httptest.NewRequest(http.MethodPost, "/internal/platform/captures", bytes.NewReader(raw))
		req.RemoteAddr = from + ":40000"
		rec := httptest.NewRecorder()
		h.HandleCaptures(rec, req)
		return rec
	}

	if rec := post("10.200.1.2", "reader-owner", "computer-reader", items); rec.Code != http.StatusForbidden {
		t.Fatalf("guest wrote captures for another computer: %d %s", rec.Code, rec.Body)
	}
	rec := post("10.200.1.2", "paper-owner", "computer-paper", items)
	if rec.Code != http.StatusOK {
		t.Fatalf("capture write = %d %s", rec.Code, rec.Body)
	}
	var first CaptureWriteResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &first); err != nil {
		t.Fatal(err)
	}
	if first.New != 2 || len(first.Captured) != 2 {
		t.Fatalf("first write = %+v", first)
	}
	obj, err := svc.GetObject(context.Background(), first.Captured[0].CaptureID)
	if err != nil {
		t.Fatal(err)
	}
	if len(obj.Body) != 0 {
		t.Fatalf("capture body is in the graph row: %q", obj.Body)
	}
	meta, err := objectgraph.WebCaptureMetadataFromObject(obj)
	if err != nil {
		t.Fatal(err)
	}
	sha := strings.TrimPrefix(meta.ContentBlobID, "sha256:")
	if _, err := os.Stat(filepath.Join(root, sha[:2], sha)); err != nil {
		t.Fatalf("metadata names a body not on disk: %v", err)
	}

	again := post("10.200.1.2", "paper-owner", "computer-paper", items)
	var second CaptureWriteResponse
	_ = json.Unmarshal(again.Body.Bytes(), &second)
	if second.New != 0 || second.Repeated != 2 {
		t.Fatalf("re-poll = %+v, want two repeats", second)
	}

	get := httptest.NewRequest(http.MethodGet, "/internal/platform/captures/bodies/"+sha, nil)
	get.RemoteAddr = "10.200.9.9:40000"
	body := httptest.NewRecorder()
	h.HandleCaptureBody(body, get)
	if body.Code != http.StatusOK || body.Body.String() != "Body one" && body.Body.String() != "Body two" {
		t.Fatalf("second computer body read = %d %q", body.Code, body.Body)
	}
	stranger := httptest.NewRequest(http.MethodGet, "/internal/platform/captures/bodies/"+sha, nil)
	stranger.RemoteAddr = "10.200.5.5:40000"
	denied := httptest.NewRecorder()
	h.HandleCaptureBody(denied, stranger)
	if denied.Code != http.StatusForbidden {
		t.Fatalf("unbound caller read a body: %d", denied.Code)
	}

	big := make([]sources.Item, maxCaptureItemsPerRequest+1)
	for i := range big {
		big[i] = items[0]
	}
	if rec := post("10.200.1.2", "paper-owner", "computer-paper", big); rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized request = %d", rec.Code)
	}
}

// Captures stay off the generic object routes
// (problems/w1-panel-capture-boundary-and-lost-items-2026-10-11.md): a
// guest cannot write one there, and no caller puts a capture body in a row.
func TestGenericObjectRoutesRefuseCaptures(t *testing.T) {
	mem := objectgraph.NewMemoryStore()
	svc := objectgraph.NewService(objectgraph.Config{Memory: mem, Durable: mem})
	defer svc.Close()
	h := NewObjectGraphHandler(svc, mem)
	paper := &vmctl.GuestOwnership{Found: true, UserID: "paper-owner", ComputerID: "computer-paper"}
	if err := h.ConfigureGuestBinding(fakeGuestLookup{"10.200.1.2": paper}); err != nil {
		t.Fatal(err)
	}
	send := func(method, from string, internal bool, body any) int {
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(method, "/internal/platform/objects", bytes.NewReader(raw))
		req.RemoteAddr = from + ":40000"
		if internal {
			req.Header.Set("X-Internal-Caller", "true")
		}
		rec := httptest.NewRecorder()
		h.HandleObjects(rec, req)
		return rec.Code
	}
	create := map[string]any{"kind": "choir.web_capture", "owner_id": "paper-owner", "computer_id": "computer-paper", "identity_key": "k", "metadata": map[string]any{"url": "https://example.com"}}
	if code := send(http.MethodPost, "10.200.1.2", false, create); code != http.StatusForbidden {
		t.Fatalf("guest create of a capture = %d, want 403", code)
	}
	withBody := map[string]any{"kind": "choir.web_capture", "owner_id": "paper-owner", "computer_id": "computer-paper", "identity_key": "k", "body": []byte("inline body"), "metadata": map[string]any{"url": "https://example.com"}}
	if code := send(http.MethodPost, "127.0.0.1", true, withBody); code != http.StatusBadRequest {
		t.Fatalf("internal create of a capture with a body = %d, want 400", code)
	}
	id, _ := objectgraph.BuildCanonicalID(objectgraph.WebCaptureObjectKind, "paper-owner", objectgraph.StableSuffixFromKey("k"))
	meta, _ := objectgraph.NormalizeMetadata(map[string]any{"url": "https://example.com"})
	put := objectgraph.Object{CanonicalID: id, ObjectKind: objectgraph.WebCaptureObjectKind, OwnerID: "paper-owner", ComputerID: "computer-paper",
		Body: []byte("x"), Metadata: meta, ContentHash: objectgraph.ContentHash(objectgraph.WebCaptureObjectKind, []byte("x"), meta)}
	if code := send(http.MethodPut, "10.200.1.2", false, put); code != http.StatusForbidden {
		t.Fatalf("guest put of a valid capture = %d, want 403", code)
	}
	if _, err := svc.GetObject(t.Context(), id); err == nil {
		t.Fatal("a capture was written through the generic route")
	}
}
