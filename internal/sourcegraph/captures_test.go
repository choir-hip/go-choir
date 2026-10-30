package sourcegraph

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
	"github.com/yusefmosiah/go-choir/internal/sources"
)

type memoryBodies map[string][]byte

func (m memoryBodies) PutBody(body []byte) (string, error) {
	ref := fmt.Sprintf("mem:%d", len(m))
	m[ref] = append([]byte(nil), body...)
	return ref, nil
}

func feedItems(items ...sources.Item) []CaptureItem {
	out := make([]CaptureItem, len(items))
	for i, item := range items {
		out[i] = CaptureItem{Item: item}
	}
	return out
}

// W1b failure modes (docs/archive/definitions/choir-autopaper-w1-content-in-the-graph-2026-10-11.md):
// a re-poll writes duplicate captures; a body lands in og_objects.body; the
// metadata names bytes that were never written; an item without a body or an
// http URL becomes a capture anyway.
func TestWriteCapturesKeepsBodiesOutOfTheGraphAndSkipsRepeats(t *testing.T) {
	ctx := context.Background()
	mem := objectgraph.NewMemoryStore()
	graph := objectgraph.NewService(objectgraph.Config{Memory: mem, Durable: mem})
	defer graph.Close()
	bodies := memoryBodies{}
	fetched := time.Date(2026, 10, 11, 10, 0, 0, 0, time.UTC)
	items := feedItems(
		sources.Item{ID: "item-1", SourceID: "rss:example", SourceType: sources.SourceTypeRSS, Title: "One", Body: "Body one", URL: "https://example.com/1", Published: fetched.Add(-time.Hour), FetchedAt: fetched, Language: "en"},
		sources.Item{ID: "item-2", SourceID: "rss:example", SourceType: sources.SourceTypeRSS, Title: "Two", Body: "Body two", URL: "https://example.com/2", FetchedAt: fetched},
		sources.Item{ID: "item-3", SourceID: "rss:example", Title: "No body", URL: "https://example.com/3", FetchedAt: fetched},
		sources.Item{ID: "item-4", SourceID: "rss:example", Title: "No URL", Body: "x", URL: "ftp://example.com/4", FetchedAt: fetched},
	)
	cfg := CaptureWriteConfig{OwnerID: "owner-1", ComputerID: "computer-1", Now: fetched}
	first, err := WriteCaptures(ctx, graph, bodies, items, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if first.New != 2 || first.Skipped != 2 || first.Repeated != 0 || len(first.CaptureIDs) != 2 {
		t.Fatalf("first poll = %+v", first)
	}
	for _, id := range first.CaptureIDs {
		obj, err := graph.GetObject(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if len(obj.Body) != 0 {
			t.Fatalf("capture %s carries its body in the graph: %q", id, obj.Body)
		}
		meta, err := objectgraph.WebCaptureMetadataFromObject(obj)
		if err != nil {
			t.Fatal(err)
		}
		for _, ref := range []string{meta.ContentBlobID, meta.ExtractedTextBlobID, meta.BlocksBlobID} {
			if _, ok := bodies[ref]; !ok {
				t.Fatalf("metadata names a blob that was never stored: %q (%+v)", ref, meta)
			}
		}
		if meta.SourceID != "rss:example" || meta.ItemID == "" || meta.Extractor != sourcefetch.TextExtractorVersion || meta.BlockCount != 1 {
			t.Fatalf("metadata = %+v", meta)
		}
		// A feed item's raw bytes are its body as polled.
		if raw := string(bodies[meta.ContentBlobID]); raw != "Body one" && raw != "Body two" {
			t.Fatalf("raw blob = %q", raw)
		}
	}
	second, err := WriteCaptures(ctx, graph, bodies, items, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if second.New != 0 || second.Repeated != 2 || len(second.Repeats) != 2 {
		t.Fatalf("second poll = %+v, want both items recognized as repeats", second)
	}
}

// Captures as block documents (Cut 2 unit 2a). Ways it can fail, written
// before the code:
//
//   - The raw bytes are lost: one flat-text body is both the content and the
//     extracted text (the C8 census found no raw bytes anywhere).
//   - A changed re-capture of the same URL is dropped as a repeat because
//     identity is the item id, so research can never cite the page as it
//     reads now.
//   - Every re-capture is new because identity is the raw bytes, so ad and
//     nonce churn floods Store B; "changed" must be decided by doc_hash.
//   - The writer trusts block ids the caller sent, so a block's id is not
//     the cid of its text and a citation addresses the wrong words.
//   - A page with "[12]" footnotes and links is refused as a document.
//   - A repeat's capture id is not returned, so a reader cannot cite it.
func TestWriteCapturesBlockDocumentKeepsRawAndIdentityIsDocHash(t *testing.T) {
	ctx := context.Background()
	mem := objectgraph.NewMemoryStore()
	graph := objectgraph.NewService(objectgraph.Config{Memory: mem, Durable: mem})
	defer graph.Close()
	bodies := memoryBodies{}
	cfg := CaptureWriteConfig{OwnerID: "owner-1", ComputerID: "computer-1", Now: time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)}
	page, err := os.ReadFile("../sourcefetch/testdata/article.html")
	if err != nil {
		t.Fatal(err)
	}
	read := func(raw string) CaptureItem {
		doc := sourcefetch.ExtractBlocks([]byte(raw))
		return ReadCaptureItem("https://example.com/harbor", []byte(raw), "text/html", doc, cfg.Now)
	}

	first, err := WriteCaptures(ctx, graph, bodies, []CaptureItem{read(string(page))}, cfg)
	if err != nil || first.New != 1 || len(first.Captured) != 1 {
		t.Fatalf("first read = %+v, %v", first, err)
	}
	firstID := first.Captured[0].CaptureID
	obj, err := graph.GetObject(ctx, firstID)
	if err != nil {
		t.Fatal(err)
	}
	meta, err := objectgraph.WebCaptureMetadataFromObject(obj)
	if err != nil {
		t.Fatal(err)
	}
	want := sourcefetch.ExtractBlocks(page)
	if string(bodies[meta.ContentBlobID]) != string(page) || meta.RawBytes != len(page) || meta.MediaType != "text/html" {
		t.Fatalf("raw bytes not kept: blob %d bytes, meta %+v", len(bodies[meta.ContentBlobID]), meta)
	}
	if string(bodies[meta.ExtractedTextBlobID]) != want.Projection() {
		t.Fatalf("projection blob = %q", bodies[meta.ExtractedTextBlobID])
	}
	var stored CaptureBlocks
	if err := json.Unmarshal(bodies[meta.BlocksBlobID], &stored); err != nil {
		t.Fatal(err)
	}
	if stored.Schema != CaptureBlocksSchema || stored.Extractor != sourcefetch.ExtractorVersion || stored.DocHash != want.DocHash() ||
		meta.DocHash != want.DocHash() || meta.Extractor != sourcefetch.ExtractorVersion || meta.BlockCount != len(want.Blocks) || len(stored.Blocks) != len(want.Blocks) {
		t.Fatalf("block JSON = %+v, meta = %+v", stored, meta)
	}
	for i := range want.Blocks {
		if stored.Blocks[i].ID != want.Blocks[i].ID || stored.Blocks[i].Text != want.Blocks[i].Text {
			t.Fatalf("stored block %d = %+v, want %+v", i, stored.Blocks[i], want.Blocks[i])
		}
	}

	// Nonce and ad churn: new raw bytes, same blocks, a repeat that names
	// the first capture.
	churned := strings.Replace(strings.Replace(string(page), `"nonce": "a8f3"`, `"nonce": "0b9e"`, 1), "Buy one mattress", "Half-price sofas", 1)
	again, err := WriteCaptures(ctx, graph, bodies, []CaptureItem{read(churned)}, cfg)
	if err != nil || again.New != 0 || again.Repeated != 1 || len(again.Repeats) != 1 || again.Repeats[0].CaptureID != firstID {
		t.Fatalf("churned re-read = %+v, %v; want a repeat of %s", again, err, firstID)
	}

	// The page's text changed: a second capture of the same URL.
	edited := strings.Replace(string(page), "voted 7 to 2", "voted 6 to 3", 1)
	changed, err := WriteCaptures(ctx, graph, bodies, []CaptureItem{read(edited)}, cfg)
	if err != nil || changed.New != 1 || changed.Captured[0].CaptureID == firstID {
		t.Fatalf("changed re-read = %+v, %v; want a new capture", changed, err)
	}
	if changed.Captured[0].DocHash == first.Captured[0].DocHash {
		t.Fatal("changed capture kept the first doc_hash")
	}
	if _, err := graph.GetObject(ctx, firstID); err != nil {
		t.Fatalf("the first capture is gone: %v", err)
	}

	// Ids the caller sends are recomputed from the text.
	forged := read(string(page))
	forged.Blocks[0].ID = "not-a-cid"
	forged.URL = "https://example.com/forged"
	out, err := WriteCaptures(ctx, graph, bodies, []CaptureItem{forged}, cfg)
	if err != nil || out.New != 1 {
		t.Fatalf("forged = %+v, %v", out, err)
	}
	obj, _ = graph.GetObject(ctx, out.Captured[0].CaptureID)
	meta, _ = objectgraph.WebCaptureMetadataFromObject(obj)
	_ = json.Unmarshal(bodies[meta.BlocksBlobID], &stored)
	if stored.Blocks[0].ID != want.Blocks[0].ID {
		t.Fatalf("stored id = %q, want the cid %q", stored.Blocks[0].ID, want.Blocks[0].ID)
	}

	// A page with footnote markers and links is a valid world document.
	wiki, err := os.ReadFile("../sourcefetch/testdata/wiki-table.html")
	if err != nil {
		t.Fatal(err)
	}
	doc := sourcefetch.ExtractBlocks(wiki)
	out, err = WriteCaptures(ctx, graph, bodies, []CaptureItem{ReadCaptureItem("https://en.example.org/wiki/Cities", wiki, "text/html", doc, cfg.Now)}, cfg)
	if err != nil || out.New != 1 || len(out.Errors) != 0 {
		t.Fatalf("footnoted page = %+v, %v", out, err)
	}
}

// L2a A5 (defects 5 and the Jev input plumbing,
// docs/problems/autopaper-intake-defects-2026-10-14.md). Ways the captured
// item report can fail, written before the code:
//   - the language is the source's but the script guess is missing, or is
//     read from the full article body instead of the title and summary;
//   - a repeat reports different fields from the first write, so an
//     arrival built from either is not the same record;
//   - the feed summary or outlet is dropped between the poll and the
//     report, so a later labeller has only the title.
func TestWriteCapturesReportsLanguageScriptSummaryAndOutlet(t *testing.T) {
	ctx := context.Background()
	mem := objectgraph.NewMemoryStore()
	graph := objectgraph.NewService(objectgraph.Config{Memory: mem, Durable: mem})
	defer graph.Close()
	fetched := time.Date(2026, 10, 14, 2, 0, 0, 0, time.UTC)
	items := feedItems(
		sources.Item{ID: "item-ja", SourceID: "rss:mixed", SourceType: sources.SourceTypeRSS, Title: "東京で地震", Summary: "震度5弱を観測しました",
			Body: "An English article body that is much longer than the title and summary combined, so it would win a majority.",
			URL:  "https://mixed.example/ja", FetchedAt: fetched, Language: "en", Outlet: "Mixed Wire"},
		sources.Item{ID: "item-en", SourceID: "rss:mixed", SourceType: sources.SourceTypeRSS, Title: "Markets steady", Body: "Markets were steady on Tuesday.",
			URL: "https://mixed.example/en", FetchedAt: fetched, Language: "en", Outlet: "Mixed Wire"},
	)
	cfg := CaptureWriteConfig{OwnerID: "owner-1", ComputerID: "computer-1", Now: fetched}
	first, err := WriteCaptures(ctx, graph, memoryBodies{}, items, cfg)
	if err != nil || len(first.Captured) != 2 {
		t.Fatalf("first write = %+v, %v", first, err)
	}
	ja, en := first.Captured[0], first.Captured[1]
	if ja.Language != "en" || ja.Script != sources.ScriptKana || ja.Summary != "震度5弱を観測しました" || ja.Outlet != "Mixed Wire" {
		t.Fatalf("japanese item = %+v; want source language en, script kana from title+summary, summary and outlet", ja)
	}
	if en.Language != "en" || en.Script != sources.ScriptLatin || en.Outlet != "Mixed Wire" {
		t.Fatalf("english item = %+v", en)
	}
	again, err := WriteCaptures(ctx, graph, memoryBodies{}, items, cfg)
	if err != nil || len(again.Repeats) != 2 {
		t.Fatalf("repeat write = %+v, %v", again, err)
	}
	if again.Repeats[0] != ja || again.Repeats[1] != en {
		t.Fatalf("repeats report %+v, first write reported %+v", again.Repeats, first.Captured)
	}
}
