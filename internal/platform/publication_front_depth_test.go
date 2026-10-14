package platform

// Ways a front row's depth (source_count, words) can fail, written before
// the code:
//
//  1. Two citations of one capture count as two sources, so the row says 2
//     where the reader's toolbar says "Sources 1".
//  2. An embedded (transcluded) capture is not counted, though the reader
//     lists it as a source.
//  3. A body with no citations reports source_count absent (unknown)
//     instead of 0, or a version with no structured body reports 0 instead
//     of absent, so the front page penalises a story it cannot measure.
//  4. Words count attribute text or miss text inside headings and lists;
//     a Chinese or Japanese body counts as a handful of "words" and reads as
//     a stub.
//  5. A corrupt body_doc fails the front list instead of leaving depth
//     unknown.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
	"github.com/yusefmosiah/go-choir/internal/sources"
	"github.com/yusefmosiah/go-choir/internal/texturewrite"
)

func TestFrontRowCarriesDepthCountedLikeTheReader(t *testing.T) {
	svc, artifactsRoot := newVersionTestService(t)
	ctx := context.Background()
	ogSvc := objectgraph.NewService(objectgraph.Config{Durable: svc.ogStore()})
	bodies := &CaptureBodyStore{root: CaptureBodiesRoot(artifactsRoot)}
	fetched := time.Date(2026, 10, 14, 8, 0, 0, 0, time.UTC)
	written, err := sourcegraph.WriteCaptures(ctx, ogSvc, bodies, []sourcegraph.CaptureItem{
		{Item: sources.Item{ID: "item-afp", SourceID: "rss:afp", SourceType: sources.SourceTypeRSS, Title: "AFP",
			URL: "https://afp.example/mauritius", Body: "Police shot a woman during protests.\n\nA curfew covers ten regions.", FetchedAt: fetched}},
		{Item: sources.Item{ID: "item-bbc", SourceID: "rss:bbc", SourceType: sources.SourceTypeRSS, Title: "BBC",
			URL: "https://bbc.example/mauritius", Body: "Water shortages drove the protests.\n\nThe government met unions.", FetchedAt: fetched}},
	}, sourcegraph.CaptureWriteConfig{OwnerID: "paper-owner", ComputerID: "computer-paper", Now: time.Now().UTC()})
	if err != nil || len(written.Captured) != 2 {
		t.Fatalf("write captures: %+v %v", written, err)
	}
	afp, bbc := written.Captured[0].CaptureID, written.Captured[1].CaptureID
	captures := &storeCaptures{t: t, og: svc.ogStore(), bodies: bodies}
	res, err := texturewrite.Apply(ctx, texturewrite.Input{
		OwnerID: "paper-owner", DocID: "doc-mauritius", Seed: "rev-a",
		Base: texturewrite.PlainDoc("doc-mauritius", "rev-0", "# Mauritius"),
		// AFP cited twice (one source), BBC embedded (a source).
		Script: "@insert end\nPolice shot a woman {{cite " + afp + " \"Police shot a woman\"}}; a curfew followed {{cite " + afp + " \"A curfew covers ten regions\"}}.\n\n{{embed " + bbc + " \"Water shortages drove the protests\"}}",
		Resolver: captures,
	})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	bodyDoc, _ := json.Marshal(res.Doc)
	entities, _ := json.Marshal(res.Entities)
	if _, err := svc.PublishTexture(ctx, PublishTextureRequest{
		OwnerID: "paper-owner", SourceDocID: "doc-mauritius", SourceRevisionID: "rev-a", RevisionNumber: 1,
		Title: "Mauritius curfew", BodyDoc: bodyDoc, SourceEntities: entities, RequestedBy: "paper-owner",
		PublishMode: PublishModeAuto,
	}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	// A cited body with no citations, and a projected-text publish.
	plain := json.RawMessage(`{"schema":"choir.texture_doc.v1","doc":{"type":"doc","attrs":{"id":"doc-canal"},"content":[
		{"type":"heading","attrs":{"id":"h1","level":1},"content":[{"type":"text","text":"Canal dredged"}]},
		{"type":"paragraph","attrs":{"id":"p1"},"content":[{"type":"text","text":"Dredging began on the canal."}]}]}}`)
	publishStoryRevision(t, svc, "doc-canal", 1, "Canal dredged", "", plain)
	publishStoryRevision(t, svc, "doc-bridge", 1, "Bridge closes", "The old bridge closed.", nil)

	page, err := svc.ListFrontStories(ctx, 0, "")
	if err != nil {
		t.Fatalf("front: %v", err)
	}
	byTitle := map[string]FrontStory{}
	for _, row := range page.Stories {
		byTitle[row.Title] = row
	}
	story := byTitle["Mauritius curfew"]
	if story.SourceCount == nil || *story.SourceCount != 2 {
		t.Fatalf("Mauritius source_count = %v, want 2 (AFP cited twice is one source, BBC embedded is one)", story.SourceCount)
	}
	if story.Words == nil || *story.Words < 8 {
		t.Fatalf("Mauritius words = %v, want the body's words", story.Words)
	}
	canal := byTitle["Canal dredged"]
	if canal.SourceCount == nil || *canal.SourceCount != 0 || canal.Words == nil || *canal.Words != 7 {
		t.Fatalf("canal depth = %v/%v, want 0 sources and 7 words (heading and paragraph)", canal.SourceCount, canal.Words)
	}
	bridge := byTitle["Bridge closes"]
	if bridge.SourceCount != nil || bridge.Words != nil {
		t.Fatalf("bridge depth = %v/%v, want absent (no structured body)", bridge.SourceCount, bridge.Words)
	}
	raw, _ := json.Marshal(bridge)
	if strings.Contains(string(raw), "source_count") || strings.Contains(string(raw), `"words"`) {
		t.Fatalf("bridge row JSON carries depth fields: %s", raw)
	}
	raw, _ = json.Marshal(canal)
	if !strings.Contains(string(raw), `"source_count":0`) {
		t.Fatalf("canal row JSON drops a zero source_count: %s", raw)
	}
}

func TestFrontDepthUnknownOnCorruptBodyAndCJKWords(t *testing.T) {
	if s, w := frontDepth(json.RawMessage(`{not json`), nil); s != nil || w != nil {
		t.Fatalf("corrupt body depth = %v/%v, want unknown", s, w)
	}
	if s, w := frontDepth(json.RawMessage(`null`), nil); s != nil || w != nil {
		t.Fatalf("null body depth = %v/%v, want unknown", s, w)
	}
	// An anchor with no entity keys by its capture; a blank anchor is skipped.
	anchors := []PublicationAnchor{
		{AnchorID: "a1"}, // no ref: not a citation
	}
	anchors[0].Ref.Doc = ""
	withDoc := PublicationAnchor{AnchorID: "a2"}
	withDoc.Ref.Doc = "obj:capture-1"
	again := PublicationAnchor{AnchorID: "a3"}
	again.Ref.Doc = "obj:capture-1"
	anchors = append(anchors, withDoc, again)
	body := json.RawMessage(`{"schema":"choir.texture_doc.v1","doc":{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"城市议会通过交通预算"}]}]}}`)
	s, w := frontDepth(body, anchors)
	if s == nil || *s != 1 {
		t.Fatalf("sources = %v, want 1 (two anchors on one capture)", s)
	}
	if w == nil || *w != 5 {
		t.Fatalf("CJK words = %v, want 5 (ten ideographs, two per word)", w)
	}
	if got := countWords("Mixed 中文 text, 3 words"); got != 5 {
		t.Fatalf("countWords mixed = %d, want 5", got)
	}
}
