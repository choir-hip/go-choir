package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
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

// W1b2 (docs/archive/definitions/choir-autopaper-w1-content-in-the-graph-2026-10-11.md):
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
	items := []sourcegraph.CaptureItem{
		{Item: sources.Item{ID: "item-1", SourceID: "rss:example", SourceType: sources.SourceTypeRSS, Title: "One", Body: "Body one", URL: "https://example.com/1", FetchedAt: fetched}},
		{Item: sources.Item{ID: "item-2", SourceID: "rss:example", SourceType: sources.SourceTypeRSS, Title: "Two", Body: "Body two", URL: "https://example.com/2", FetchedAt: fetched}},
	}
	post := func(from, owner, computer string, items []sourcegraph.CaptureItem) *httptest.ResponseRecorder {
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

	big := make([]sourcegraph.CaptureItem, maxCaptureItemsPerRequest+1)
	for i := range big {
		big[i] = items[0]
	}
	if rec := post("10.200.1.2", "paper-owner", "computer-paper", big); rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized request = %d", rec.Code)
	}
}

// A capture write and a verdict push are both a bound guest's write and
// meet one rule (B12): the host-internal caller (loopback with the marker)
// or the guest vmctl binds to the owner and computer the write names.
func TestCaptureWriteAdmitsItsBoundGuestAndInternalCallers(t *testing.T) {
	mem := objectgraph.NewMemoryStore()
	svc := objectgraph.NewService(objectgraph.Config{Memory: mem, Durable: mem})
	defer svc.Close()
	h := NewObjectGraphHandler(svc, mem)
	if err := h.ConfigureGuestBinding(fakeGuestLookup{
		"10.200.44.2": {Found: true, UserID: "paper-owner", ComputerID: "computer-paper"},
		"10.200.9.9":  {Found: true, UserID: "paper-owner", ComputerID: "computer-other"},
	}); err != nil {
		t.Fatal(err)
	}
	h.ConfigureCaptureBodies(t.TempDir())
	fetched := time.Date(2026, 10, 13, 12, 35, 8, 0, time.UTC)
	for i, c := range []struct {
		label    string
		from     string
		internal bool
		want     int
	}{
		{"the bound guest that owns the captures", "10.200.44.2", false, http.StatusOK},
		{"the host-internal caller", "127.0.0.1", true, http.StatusOK},
		{"a guest bound to another computer", "10.200.9.9", false, http.StatusForbidden},
		{"an unbound outside caller", "203.0.113.9", false, http.StatusForbidden},
		{"an outside caller claiming to be internal", "203.0.113.9", true, http.StatusForbidden},
		{"a loopback caller without the internal marker", "127.0.0.1", false, http.StatusForbidden},
	} {
		items := []sourcegraph.CaptureItem{{Item: sources.Item{ID: fmt.Sprintf("item-%d", i), SourceID: "rss:npr_world", SourceType: sources.SourceTypeRSS,
			Title: "Item", Body: "Body", URL: fmt.Sprintf("https://example.com/%d", i), FetchedAt: fetched}}}
		raw, _ := json.Marshal(CaptureWriteRequest{OwnerID: "paper-owner", ComputerID: "computer-paper", Items: items})
		req := httptest.NewRequest(http.MethodPost, "/internal/platform/captures", bytes.NewReader(raw))
		req.RemoteAddr = c.from + ":40000"
		if c.internal {
			req.Header.Set("X-Internal-Caller", "true")
		}
		rec := httptest.NewRecorder()
		h.HandleCaptures(rec, req)
		if rec.Code != c.want {
			t.Fatalf("%s = %d %s; want %d", c.label, rec.Code, rec.Body, c.want)
		}
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

// Two computers of one owner post the same new items at once against Store
// B's Dolt adapter. Each capture is created once: it keeps one writer's
// computer, the other writer counts a repeat (or fails), and no request
// replaces the other's capture
// (problems/w1-panel-capture-boundary-and-lost-items-2026-10-11.md). The
// bodies are the same text: a capture's identity is its URL and doc_hash,
// so different text would be two captures.
func TestConcurrentSameOwnerCaptureWritesCreateOnce(t *testing.T) {
	store, _ := openTestPlatformStore(t)
	og := NewObjectGraphStore(store)
	svc := objectgraph.NewService(objectgraph.Config{Durable: og})
	defer svc.Close()
	h := NewObjectGraphHandler(svc, og)
	a := &vmctl.GuestOwnership{Found: true, UserID: "paper-owner", ComputerID: "computer-a"}
	b := &vmctl.GuestOwnership{Found: true, UserID: "paper-owner", ComputerID: "computer-b"}
	if err := h.ConfigureGuestBinding(fakeGuestLookup{"10.200.1.2": a, "10.200.1.3": b}); err != nil {
		t.Fatal(err)
	}
	h.ConfigureCaptureBodies(t.TempDir())

	fetched := time.Date(2026, 10, 11, 10, 0, 0, 0, time.UTC)
	itemsFor := func(computer string) []sourcegraph.CaptureItem {
		var items []sourcegraph.CaptureItem
		for i := 0; i < 8; i++ {
			items = append(items, sourcegraph.CaptureItem{Item: sources.Item{ID: fmt.Sprintf("race-%d", i), SourceID: "rss:example", SourceType: sources.SourceTypeRSS,
				Title: "Race", Body: fmt.Sprintf("race body %d", i), URL: fmt.Sprintf("https://example.com/race/%d", i), FetchedAt: fetched}})
		}
		return items
	}
	type outcome struct {
		code   int
		result struct {
			New        int      `json:"new"`
			Repeated   int      `json:"repeated"`
			CaptureIDs []string `json:"capture_ids"`
		}
	}
	post := func(from, computer string) outcome {
		raw, _ := json.Marshal(CaptureWriteRequest{OwnerID: "paper-owner", ComputerID: computer, Items: itemsFor(computer)})
		req := httptest.NewRequest(http.MethodPost, "/internal/platform/captures", bytes.NewReader(raw))
		req.RemoteAddr = from + ":40000"
		rec := httptest.NewRecorder()
		h.HandleCaptures(rec, req)
		var out outcome
		out.code = rec.Code
		_ = json.Unmarshal(rec.Body.Bytes(), &out.result)
		return out
	}
	var wg sync.WaitGroup
	var got [2]outcome
	for i, g := range []struct{ ip, computer string }{{"10.200.1.2", "computer-a"}, {"10.200.1.3", "computer-b"}} {
		wg.Add(1)
		go func(i int, ip, computer string) { defer wg.Done(); got[i] = post(ip, computer) }(i, g.ip, g.computer)
	}
	wg.Wait()

	created := map[string]string{}
	for i, computer := range []string{"computer-a", "computer-b"} {
		if got[i].code != http.StatusOK {
			continue // a conflicting commit fails the request; it must not overwrite
		}
		for _, id := range got[i].result.CaptureIDs {
			if prev, dup := created[id]; dup {
				t.Fatalf("capture %s reported new by %s and %s", id, prev, computer)
			}
			created[id] = computer
		}
	}
	for id, computer := range created {
		obj, err := og.GetObject(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if obj.ComputerID != computer {
			t.Fatalf("capture %s was created by %s but now names %s", id, computer, obj.ComputerID)
		}
	}
	if got[0].code == http.StatusOK && got[1].code == http.StatusOK && len(created) != 8 {
		t.Fatalf("both writes succeeded but %d of 8 captures were reported new: %+v", len(created), got)
	}
}

// The capture read route serves a block capture (Cut 2 unit 2a). Ways it
// can fail, written before the code: it serves the raw bytes as body text
// now that ContentBlobID names them; it returns blocks without their ids,
// so a reader cannot cite one; a long page is cut with no way to read the
// rest; a raw request over the bound is accepted; a capture written before
// Cut 2 (one flat-text blob, no blocks) no longer reads.
func TestCaptureReadServesBlocksAndKeepsRawBytes(t *testing.T) {
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
	page, err := os.ReadFile("../sourcefetch/testdata/wiki-table.html")
	if err != nil {
		t.Fatal(err)
	}
	fetched := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	doc := sourcefetch.ExtractBlocks(page)
	item := sourcegraph.ReadCaptureItem("https://en.example.org/wiki/Cities", page, "text/html", doc, fetched)
	raw, _ := json.Marshal(CaptureWriteRequest{OwnerID: "paper-owner", ComputerID: "computer-paper", Items: []sourcegraph.CaptureItem{item}})
	req := httptest.NewRequest(http.MethodPost, "/internal/platform/captures", bytes.NewReader(raw))
	req.RemoteAddr = "10.200.1.2:40000"
	rec := httptest.NewRecorder()
	h.HandleCaptures(rec, req)
	var wrote CaptureWriteResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &wrote); rec.Code != http.StatusOK || err != nil || wrote.New != 1 {
		t.Fatalf("write = %d %s", rec.Code, rec.Body)
	}
	id := wrote.Captured[0].CaptureID

	read := func(query string) CaptureReadResponse {
		t.Helper()
		get := httptest.NewRequest(http.MethodGet, "/internal/platform/captures/"+id+query, nil)
		get.RemoteAddr = "10.200.9.9:40000"
		rec := httptest.NewRecorder()
		h.HandleCaptureRead(rec, get)
		var out CaptureReadResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &out); rec.Code != http.StatusOK || err != nil {
			t.Fatalf("read %s = %d %s", query, rec.Code, rec.Body)
		}
		return out
	}
	first := read("?max_chars=300")
	if first.BlockCount != len(doc.Blocks) || first.DocHash != doc.DocHash() || first.Extractor != sourcefetch.ExtractorVersion ||
		first.RawBytes != len(page) || first.MediaType != "text/html" || first.Body != "" {
		t.Fatalf("read = %+v", first)
	}
	if len(first.Blocks) == 0 || len(first.Blocks) >= len(doc.Blocks) || first.NextBlock != len(first.Blocks) || !first.Truncated {
		t.Fatalf("a 300-char read returned %d of %d blocks, next %d", len(first.Blocks), len(doc.Blocks), first.NextBlock)
	}
	for i, b := range first.Blocks {
		if b.ID != doc.Blocks[i].ID || b.Text != doc.Blocks[i].Text || b.Kind != doc.Blocks[i].Kind {
			t.Fatalf("block %d = %+v, want %+v", i, b, doc.Blocks[i])
		}
	}
	rest := read(fmt.Sprintf("?from_block=%d&max_chars=100000", first.NextBlock))
	if rest.FromBlock != first.NextBlock || len(first.Blocks)+len(rest.Blocks) != len(doc.Blocks) || rest.NextBlock != 0 || rest.Truncated {
		t.Fatalf("rest = from %d, %d blocks, next %d", rest.FromBlock, len(rest.Blocks), rest.NextBlock)
	}
	if rest.Blocks[len(rest.Blocks)-1].ID != doc.Blocks[len(doc.Blocks)-1].ID {
		t.Fatalf("last block = %+v", rest.Blocks[len(rest.Blocks)-1])
	}

	obj, _ := svc.GetObject(context.Background(), id)
	meta, _ := objectgraph.WebCaptureMetadataFromObject(obj)
	sha := strings.TrimPrefix(meta.ContentBlobID, "sha256:")
	stored, err := os.ReadFile(filepath.Join(root, sha[:2], sha))
	if err != nil || !bytes.Equal(stored, page) {
		t.Fatalf("raw bytes on disk = %d bytes, %v", len(stored), err)
	}

	// A capture written before Cut 2: one flat-text blob, no blocks.
	ref, err := h.captureBodies.PutBody([]byte("Legacy flat body"))
	if err != nil {
		t.Fatal(err)
	}
	legacy, _, err := svc.CreateWebCaptureIfAbsent(context.Background(), objectgraph.CreateWebCaptureRequest{OwnerID: "paper-owner", ComputerID: "computer-paper",
		IdentityKey: "autopaper:capture:legacy", URL: "https://example.com/legacy", FetchedAt: fetched, ContentBlobID: ref, ExtractedTextBlobID: ref, Now: fetched})
	if err != nil {
		t.Fatal(err)
	}
	id = legacy.CanonicalID
	if got := read(""); got.Body != "Legacy flat body" || len(got.Blocks) != 0 {
		t.Fatalf("legacy read = %+v", got)
	}

	item.Raw = make([]byte, maxCaptureRawBytes+1)
	raw, _ = json.Marshal(CaptureWriteRequest{OwnerID: "paper-owner", ComputerID: "computer-paper", Items: []sourcegraph.CaptureItem{item}})
	req = httptest.NewRequest(http.MethodPost, "/internal/platform/captures", bytes.NewReader(raw))
	req.RemoteAddr = "10.200.1.2:40000"
	rec = httptest.NewRecorder()
	h.HandleCaptures(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized raw bytes = %d %s", rec.Code, rec.Body)
	}
}
