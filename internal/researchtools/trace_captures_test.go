package researchtools

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/normtext"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
)

// The trace bundle's captures layer (Cut 2 amendment A5) lists the
// computer's captures in the trace window with each block's cid and
// normalized text, so the C8 quote census can rerun on bundles. Ways it can
// fail, written before the code:
//
//   - Captures outside the window, or another computer's, are listed as this
//     trajectory's; a capture the trajectory cited but did not create in the
//     window (an unchanged re-read, another computer's poll) is missing.
//   - Blocks lack their cid or normtext, or normtext is not normtext's.
//   - Whether raw bytes were kept is not said (C8 asks for it separately).
//   - A huge page makes the bundle unbounded, silently; or the list hits its
//     cap and the window is reported complete.
//   - A capture written before Cut 2 (no blocks) fails the whole layer.
func TestTraceCapturesListsTheWindowWithCIDsAndNormtext(t *testing.T) {
	ctx := context.Background()
	mem := objectgraph.NewMemoryStore()
	shared := objectgraph.NewService(objectgraph.Config{Memory: mem, Durable: mem})
	defer shared.Close()
	og := platform.NewObjectGraphHandler(shared, mem)
	if err := og.ConfigureGuestBinding(paperGuest{}); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	og.ConfigureCaptureBodies(root)
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.RemoteAddr = "10.200.1.2:40000"
		if r.URL.Path == "/internal/platform/objects" {
			og.HandleObjects(w, r)
			return
		}
		og.HandleCaptureRead(w, r)
	}))
	defer corpusd.Close()
	sink := NewCorpusdCaptureSink(corpusd.URL)
	bodies := bodyStoreFor(t, og)

	article, err := os.ReadFile("../sourcefetch/testdata/article.html")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	write := func(computer, pageURL string, raw []byte, at time.Time) string {
		t.Helper()
		doc := sourcefetch.ExtractBlocks(raw)
		res, err := sourcegraph.WriteCaptures(ctx, shared, bodies, []sourcegraph.CaptureItem{sourcegraph.ReadCaptureItem(pageURL, raw, "text/html", doc, at)},
			sourcegraph.CaptureWriteConfig{OwnerID: "owner-1", ComputerID: computer, Now: at})
		if err != nil || res.New != 1 {
			t.Fatalf("write %s = %+v, %v", pageURL, res, err)
		}
		return res.Captured[0].CaptureID
	}
	before := write("computer-1", "https://example.com/before", article, start.Add(-2*time.Hour))
	inWindow := write("computer-1", "https://example.com/harbor", article, start.Add(time.Minute))
	var huge strings.Builder
	huge.WriteString("<html><body><article>")
	for i := 0; i < 3000; i++ {
		fmt.Fprintf(&huge, "<p>Paragraph %d of a very long page that goes on and on about the harbor and its walls.</p>", i)
	}
	huge.WriteString("</article></body></html>")
	big := write("computer-1", "https://example.com/huge", []byte(huge.String()), start.Add(2*time.Minute))
	other := write("computer-2", "https://example.com/other", article, start.Add(3*time.Minute))
	ref, err := bodies.PutBody([]byte("Legacy flat body"))
	if err != nil {
		t.Fatal(err)
	}
	legacy, _, err := shared.CreateWebCaptureIfAbsent(ctx, objectgraph.CreateWebCaptureRequest{OwnerID: "owner-1", ComputerID: "computer-1",
		IdentityKey: "autopaper:capture:legacy", URL: "https://example.com/legacy", FetchedAt: start, ContentBlobID: ref, ExtractedTextBlobID: ref,
		Now: start.Add(4 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}

	got, err := TraceCapturesInWindow(ctx, sink, "owner-1", "computer-1", start, start.Add(time.Hour), []string{before, "not-a-capture"})
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]TraceCapture{}
	for _, c := range got.Captures {
		byID[c.CaptureID] = c
	}
	if _, ok := byID[other]; ok {
		t.Fatal("another computer's capture is listed")
	}
	for _, id := range []string{before, inWindow, big, legacy.CanonicalID} {
		if _, ok := byID[id]; !ok {
			t.Fatalf("capture %s missing; got %d captures", id, len(got.Captures))
		}
	}
	if !got.WindowComplete {
		t.Fatalf("window = %+v, want complete", got)
	}
	harbor := byID[inWindow]
	want := sourcefetch.ExtractBlocks(article)
	if harbor.URL != "https://example.com/harbor" || harbor.Extractor != sourcefetch.ExtractorVersion || harbor.DocHash != want.DocHash() ||
		harbor.BlockCount != len(want.Blocks) || !harbor.RawPresent || harbor.RawBytes != len(article) || len(harbor.Blocks) != len(want.Blocks) || harbor.BlocksTruncated {
		t.Fatalf("harbor = %+v", harbor)
	}
	for i, b := range harbor.Blocks {
		if b.ID != want.Blocks[i].ID || b.Text != want.Blocks[i].Text || b.Normtext != normtext.Normalize(want.Blocks[i].Text) {
			t.Fatalf("block %d = %+v", i, b)
		}
	}
	if huge := byID[big]; !huge.BlocksTruncated || len(huge.Blocks) == 0 || len(huge.Blocks) >= huge.BlockCount || !strings.Contains(huge.Note, "characters") {
		t.Fatalf("huge capture = %d of %d blocks, truncated %v, note %q", len(huge.Blocks), huge.BlockCount, huge.BlocksTruncated, huge.Note)
	}
	if old := byID[legacy.CanonicalID]; old.RawPresent || len(old.Blocks) != 0 || old.Note == "" {
		t.Fatalf("legacy capture = %+v", old)
	}
	if !strings.Contains(got.Note, "not-a-capture") {
		t.Fatalf("an unreadable named id is not noted: %q", got.Note)
	}
}

// bodyStoreFor writes blobs where the handler reads them, so test captures
// written directly through sourcegraph are readable through corpusd.
func bodyStoreFor(t *testing.T, og *platform.ObjectGraphHandler) sourcegraph.BodyStore {
	t.Helper()
	store := og.CaptureBodyStore()
	if store == nil {
		t.Fatal("capture bodies not configured")
	}
	return store
}

