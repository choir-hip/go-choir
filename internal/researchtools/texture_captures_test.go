package researchtools

// Texture cites from captures (Cut 2 2a, A2): a {{cite}} handle resolves
// through the capture read path research's choir.Read writes to. Ways it can
// fail, written first:
//   - a capture id resolves to only the first page of its blocks, so a quote
//     in a later block reads as "not found";
//   - a URL captured twice resolves to the older capture;
//   - a URL with no capture, an unknown capture id, or a capture written
//     before Cut 2 (flat text, no cids) cites anyway instead of refusing as
//     not captured;
//   - an unreachable capture store reads as "not captured", hiding the
//     outage behind a model-facing refusal;
//   - the resolved blocks are not the capture's own cids, so the Ref names a
//     block no reader can find.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
	"github.com/yusefmosiah/go-choir/internal/texturedoc"
	"github.com/yusefmosiah/go-choir/internal/texturewrite"
)

type fakeCapture struct {
	id, url, docHash, fetchedAt string
	blocks                      []sourcefetch.Block
	flatBody                    string
}

// fakeCorpus serves corpusd's capture read and object list routes from
// memory, paging blocks the way corpusd does.
func fakeCorpus(t *testing.T, captures ...fakeCapture) *httptest.Server {
	t.Helper()
	byID := map[string]fakeCapture{}
	for _, c := range captures {
		byID[c.id] = c
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/internal/platform/objects":
			var objs []objectgraph.Object
			for _, c := range captures {
				meta, _ := json.Marshal(objectgraph.WebCaptureMetadata{SchemaVersion: objectgraph.WebCaptureSchemaVersion, ContentBlobID: "sha256:raw", ExtractedTextBlobID: "sha256:text", Extractor: "x1", URL: c.url, CanonicalURL: c.url, FetchedAt: c.fetchedAt, DocHash: c.docHash, BlocksBlobID: map[bool]string{true: "sha256:b", false: ""}[c.flatBody == ""]})
				objs = append(objs, objectgraph.Object{CanonicalID: c.id, ObjectKind: objectgraph.WebCaptureObjectKind, OwnerID: "owner", Metadata: meta})
			}
			_ = json.NewEncoder(w).Encode(objs)
		case strings.HasPrefix(r.URL.Path, "/internal/platform/captures/"):
			c, ok := byID[strings.TrimPrefix(r.URL.Path, "/internal/platform/captures/")]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"error":"capture not found"}`))
				return
			}
			out := map[string]any{"capture_id": c.id, "url": c.url, "title": "Court ruling", "doc_hash": c.docHash, "block_count": len(c.blocks)}
			if c.flatBody != "" {
				out["body"] = c.flatBody
				_ = json.NewEncoder(w).Encode(out)
				return
			}
			from, _ := strconv.Atoi(r.URL.Query().Get("from_block"))
			// One block per page, so a resolver that reads one page misses the rest.
			out["from_block"], out["blocks"] = from, c.blocks[from:from+1]
			if from+1 < len(c.blocks) {
				out["next_block"], out["truncated"] = from+1, true
			}
			_ = json.NewEncoder(w).Encode(out)
		default:
			http.NotFound(w, r)
		}
	}))
}

func courtBlocks() []sourcefetch.Block {
	doc := sourcefetch.ExtractBlocks([]byte(`<html><head><title>Court ruling</title></head><body><article>
<h1>Ruling</h1>
<p>The court ruled that the merger may proceed after a long review.</p>
<p>The appeal was filed on Monday by the regulator, who called the ruling premature.</p>
</article></body></html>`))
	return doc.Blocks
}

func captureID(t *testing.T, key string) string {
	t.Helper()
	id, err := objectgraph.BuildCanonicalID(objectgraph.WebCaptureObjectKind, "owner", objectgraph.StableSuffixFromKey(key))
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestTextureCapturesResolveACaptureIDToEveryBlock(t *testing.T) {
	blocks := courtBlocks()
	if len(blocks) < 3 {
		t.Fatalf("fixture extracted %d blocks", len(blocks))
	}
	id := captureID(t, "court#h1")
	srv := fakeCorpus(t, fakeCapture{id: id, url: "https://example.org/court", docHash: "h1", fetchedAt: "2026-10-13T01:00:00Z", blocks: blocks})
	defer srv.Close()
	src := NewTextureCaptures(NewCorpusdCaptureSink(srv.URL), "computer")
	got, err := src.CaptureBlocks(context.Background(), "owner", id)
	if err != nil {
		t.Fatalf("CaptureBlocks: %v", err)
	}
	if got.Doc != id || got.Version != "h1" || got.URL != "https://example.org/court" || len(got.Blocks) != len(blocks) {
		t.Fatalf("capture = %+v", got)
	}
	for i, b := range got.Blocks {
		if b.ID != blocks[i].ID || b.Text != blocks[i].Text {
			t.Fatalf("block %d = %+v, want the capture's own cid and text %+v", i, b, blocks[i])
		}
	}
}

func TestTextureCapturesResolveAURLToItsNewestCapture(t *testing.T) {
	blocks := courtBlocks()
	older, newer := captureID(t, "court#old"), captureID(t, "court#new")
	srv := fakeCorpus(t,
		fakeCapture{id: newer, url: "https://example.org/court", docHash: "new", fetchedAt: "2026-10-13T02:00:00Z", blocks: blocks},
		fakeCapture{id: older, url: "https://example.org/court", docHash: "old", fetchedAt: "2026-10-12T02:00:00Z", blocks: blocks[:1]},
	)
	defer srv.Close()
	got, err := NewTextureCaptures(NewCorpusdCaptureSink(srv.URL), "computer").CaptureBlocks(context.Background(), "owner", "https://example.org/court")
	if err != nil || got.Doc != newer || got.Version != "new" {
		t.Fatalf("URL resolved to %+v, %v; want the newest capture %s", got, err, newer)
	}
}

func TestTextureCapturesRefuseWhatIsNotCaptured(t *testing.T) {
	legacy := captureID(t, "legacy")
	srv := fakeCorpus(t, fakeCapture{id: legacy, url: "https://example.org/legacy", fetchedAt: "2026-10-01T00:00:00Z", flatBody: "flat text with no block ids"})
	defer srv.Close()
	src := NewTextureCaptures(NewCorpusdCaptureSink(srv.URL), "computer")
	for _, handle := range []string{"https://example.org/never-read", captureID(t, "unknown"), legacy, "https://example.org/legacy"} {
		if _, err := src.CaptureBlocks(context.Background(), "owner", handle); !errors.Is(err, texturewrite.ErrNotCaptured) {
			t.Errorf("%s = %v, want ErrNotCaptured", handle, err)
		}
	}
	down := NewTextureCaptures(&CorpusdCaptureSink{BaseURL: "http://127.0.0.1:1", Client: &http.Client{Timeout: time.Second}}, "computer")
	if _, err := down.CaptureBlocks(context.Background(), "owner", captureID(t, "x")); err == nil || errors.Is(err, texturewrite.ErrNotCaptured) {
		t.Fatalf("an unreachable store = %v, want an error that is not ErrNotCaptured", err)
	}
}

// End to end through the Write materializer: a cite of a real capture
// resolves to that capture's block and quote; a cite of an uncaptured URL
// is refused at its line.
func TestTextureWriteCitesARealCaptureBlock(t *testing.T) {
	blocks := courtBlocks()
	id := captureID(t, "court#h1")
	srv := fakeCorpus(t, fakeCapture{id: id, url: "https://example.org/court", docHash: "h1", fetchedAt: "2026-10-13T01:00:00Z", blocks: blocks})
	defer srv.Close()
	src := NewTextureCaptures(NewCorpusdCaptureSink(srv.URL), "computer")
	base := texturedoc.StructuredTextureDoc{Schema: texturedoc.SchemaV1, Doc: texturedoc.Node{Type: "doc", Attrs: map[string]any{"id": "root"}, Content: []texturedoc.Node{
		{Type: "paragraph", Attrs: map[string]any{"id": "p1"}, Content: []texturedoc.Node{{Type: "text", Text: "Opening."}}},
	}}}
	handles := map[string]texturewrite.Source{"r1.c1": {Handle: "r1.c1", URL: "https://example.org/court"}}
	res, err := texturewrite.Apply(context.Background(), texturewrite.Input{OwnerID: "owner", DocID: "d", Seed: "s", Base: base, Handles: handles, Resolver: src,
		Script: "@insert end\nThe regulator appealed {{cite r1.c1 \"appeal was filed on Monday\"}}."})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	var cited texturedoc.Node
	for _, n := range res.Doc.Doc.Content[1].Content {
		if n.Type == "source_ref" {
			cited = n
		}
	}
	ref, ok := texturewrite.RefOf(cited)
	if !ok || ref.Doc != id || ref.Version != "h1" || ref.CID != blocks[2].ID || ref.Quote != "appeal was filed on Monday" {
		t.Fatalf("ref = %+v, want block %s of %s", ref, blocks[2].ID, id)
	}
	_, err = texturewrite.Apply(context.Background(), texturewrite.Input{OwnerID: "owner", DocID: "d", Seed: "s", Base: base, Resolver: src,
		Script: "@insert end\nOk.\n\nX {{cite https://example.org/never-read \"words\"}}."})
	if err == nil || !strings.Contains(err.Error(), "line 4") || !strings.Contains(err.Error(), "not captured") {
		t.Fatalf("uncaptured cite = %v, want a line-4 not-captured refusal", err)
	}
}
