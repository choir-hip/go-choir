package sourcegraph

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/sources"
)

type memoryBodies map[string][]byte

func (m memoryBodies) PutBody(body []byte) (string, error) {
	ref := fmt.Sprintf("mem:%d", len(m))
	m[ref] = append([]byte(nil), body...)
	return ref, nil
}

// W1b failure modes (docs/definitions/choir-autopaper-w1-content-in-the-graph-2026-10-11.md):
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
	items := []sources.Item{
		{ID: "item-1", SourceID: "rss:example", SourceType: sources.SourceTypeRSS, Title: "One", Body: "Body one", URL: "https://example.com/1", Published: fetched.Add(-time.Hour), FetchedAt: fetched, Language: "en"},
		{ID: "item-2", SourceID: "rss:example", SourceType: sources.SourceTypeRSS, Title: "Two", Body: "Body two", URL: "https://example.com/2", FetchedAt: fetched},
		{ID: "item-3", SourceID: "rss:example", Title: "No body", URL: "https://example.com/3", FetchedAt: fetched},
		{ID: "item-4", SourceID: "rss:example", Title: "No URL", Body: "x", URL: "ftp://example.com/4", FetchedAt: fetched},
	}
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
		body, ok := bodies[meta.ContentBlobID]
		if !ok {
			t.Fatalf("metadata names a body that was never stored: %s", meta.ContentBlobID)
		}
		if meta.SourceID != "rss:example" || meta.ItemID == "" || string(body) == "" {
			t.Fatalf("metadata = %+v body = %q", meta, body)
		}
	}
	second, err := WriteCaptures(ctx, graph, bodies, items, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if second.New != 0 || second.Repeated != 2 {
		t.Fatalf("second poll = %+v, want both items recognized as repeats", second)
	}
}
