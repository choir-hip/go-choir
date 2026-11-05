package store

import (
	"context"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// X1 (modular architecture plan, section 8.4) is counted, not fixed. Research's
// corpus search runs Store.SearchDocuments, which lists every owner Texture
// document: a read of Texture's kind by a desk that is not its custodian. A
// built-in kind only observes until MA3.7 replaces the read with a
// texture.search lease, so the search works; but the counters must read it as
// what it is, desk:research reading choir.texture_document. This test is
// MA3.7's target: when the read becomes a refusal, the search turns from a
// counted crossing into ErrWrongReader.
func TestResearchCorpusSearchCountsAsResearchReadingTexturesDocuments(t *testing.T) {
	s := openTestStore(t)
	receipt := mustCommitInbox(t, s, "cell-records:x1-1", nil, openingCast("x1-1:cast:c1"))
	docID := strings.TrimPrefix(receipt.Records[0].Inbox, "texture:")
	defer objectgraph.SetReaderGateMode(objectgraph.WriterGateObserve)()
	objectgraph.ResetReaderGateCounts()

	key := "choir.texture_document|" + string(objectgraph.WriterResearch) + "|wrong_reader"
	for name, reader := range map[string]objectgraph.Writer{
		"texture (custodian)": objectgraph.WriterTexture,
		"owner":               objectgraph.WriterOwner,
		"kernel":              objectgraph.WriterKernel,
	} {
		ctx := objectgraph.WithReader(context.Background(), reader)
		if results, err := s.SearchDocuments(ctx, "harbor storm", storyOwner, 10); err != nil || len(results) != 1 {
			t.Fatalf("%s: search = %+v, %v", name, results, err)
		}
	}
	if got := objectgraph.ReaderGateCounts(); len(got) != 0 {
		t.Fatalf("a reader the 8.2 table admits was counted: %v", got)
	}

	ctx := objectgraph.WithReader(context.Background(), objectgraph.WriterResearch)
	results, err := s.SearchDocuments(ctx, "harbor storm", storyOwner, 10)
	if err != nil || len(results) != 1 || results[0].DocID != docID {
		t.Fatalf("research's search (observed, not refused) = %+v, %v; want the story %s", results, err, docID)
	}
	if objectgraph.ReaderGateCounts()[key] == 0 {
		t.Fatalf("counter %q = 0 (counts: %v)", key, objectgraph.ReaderGateCounts())
	}

	// The scoped handle declares the reader once, for the typed methods.
	objectgraph.ResetReaderGateCounts()
	if _, err := s.SearchDocuments(s.As(objectgraph.WriterResearch).Ctx(context.Background()), "harbor storm", storyOwner, 10); err != nil {
		t.Fatal(err)
	}
	if objectgraph.ReaderGateCounts()[key] == 0 {
		t.Fatalf("a research handle did not declare research: %v", objectgraph.ReaderGateCounts())
	}
	objectgraph.ResetReaderGateCounts()
	if _, err := s.As(objectgraph.WriterTexture).Graph().GetObject(context.Background(), "choir.texture_document:none"); err == nil {
		t.Fatal("a missing object was found")
	}
	if got := objectgraph.ReaderGateCounts(); len(got) != 0 {
		t.Fatalf("texture's own handle was counted: %v", got)
	}
}
