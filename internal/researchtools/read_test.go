package researchtools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

type readResult struct {
	CaptureID  string `json:"capture_id"`
	New        bool   `json:"new"`
	URL        string `json:"url"`
	Title      string `json:"title"`
	MediaType  string `json:"media_type"`
	Extractor  string `json:"extractor"`
	DocHash    string `json:"doc_hash"`
	RawBytes   int    `json:"raw_bytes"`
	BlockCount int    `json:"block_count"`
	Blocks     []struct {
		ID   string `json:"id"`
		Kind string `json:"kind"`
		Page int    `json:"page"`
		Text string `json:"text"`
	} `json:"blocks"`
	NextBlock int `json:"next_block"`
	Headings  []struct {
		ID    string `json:"id"`
		Index int    `json:"index"`
		Text  string `json:"text"`
	} `json:"headings"`
	Truncated bool `json:"truncated"`
}

// choir.Read (Cut 2 unit 2a) fetches a page into a capture through the one
// capture writer and returns what research needs to cite it. Ways it can
// fail, written before the code:
//
//   - The page is fetched outside the address guard, so a cell can read the
//     guest's own loopback services.
//   - Read returns page text but no capture, so a quote cites a page that
//     was never captured (C8 census: 46% of quotes).
//   - The blocks Read shows carry ids other than the stored capture's, so a
//     citation built from Read names no block.
//   - Re-reading an unchanged page makes a second capture, or re-reading a
//     changed page returns the old one.
//   - A PDF is refused or read without page numbers.
//   - An image, an error page or an empty page is captured as a document.
//   - A long page floods the cell, or is cut with no way to read the rest.
func TestReadFetchesAPageIntoABlockCapture(t *testing.T) {
	prev := sourcefetch.SetAllowPrivateNetworkForTests(true)
	defer sourcefetch.SetAllowPrivateNetworkForTests(prev)
	article, err := os.ReadFile("../sourcefetch/testdata/article.html")
	if err != nil {
		t.Fatal(err)
	}
	pdf, err := os.ReadFile("../content/testdata/rainfall-two-pages.pdf")
	if err != nil {
		t.Fatal(err)
	}
	var edited atomic.Bool
	var long strings.Builder
	long.WriteString("<html><head><title>Long</title></head><body><article>")
	for i := 1; i <= 120; i++ {
		if i%10 == 1 {
			fmt.Fprintf(&long, "<h2>Section %d</h2>", i/10+1)
		}
		fmt.Fprintf(&long, "<p>Paragraph %d says something long enough to count as readable text in the page body.</p>", i)
	}
	long.WriteString("</article></body></html>")
	pages := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/harbor":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			page := string(article)
			if edited.Load() {
				page = strings.Replace(page, "voted 7 to 2", "voted 6 to 3", 1)
			}
			fmt.Fprint(w, page)
		case "/report.pdf":
			w.Header().Set("Content-Type", "application/pdf")
			_, _ = w.Write(pdf)
		case "/long":
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, long.String())
		case "/image.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("\x89PNG\r\n\x1a\nxxxx"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer pages.Close()

	mem := objectgraph.NewMemoryStore()
	shared := objectgraph.NewService(objectgraph.Config{Memory: mem, Durable: mem})
	defer shared.Close()
	og := platform.NewObjectGraphHandler(shared, mem)
	if err := og.ConfigureGuestBinding(paperGuest{}); err != nil {
		t.Fatal(err)
	}
	og.ConfigureCaptureBodies(t.TempDir())
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.RemoteAddr = "10.200.1.2:40000"
		if r.URL.Path == "/internal/platform/captures" {
			og.HandleCaptures(w, r)
			return
		}
		og.HandleCaptureRead(w, r)
	}))
	defer corpusd.Close()

	deps := Dependencies{HTTP: sourcefetch.Client(10 * time.Second), Captures: NewCorpusdCaptureSink(corpusd.URL)}
	ctx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{
		RunID: "research-run", OwnerID: "owner-1", ComputerID: "computer-1", Profile: agentprofile.Research,
	})
	read := func(path string) (readResult, error) {
		t.Helper()
		raw, err := deps.HostEgress(ctx, "read", json.RawMessage(`{"url":"`+pages.URL+path+`"}`))
		if err != nil {
			return readResult{}, err
		}
		var out readResult
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("decode %s: %v", raw, err)
		}
		return out, nil
	}

	first, err := read("/harbor")
	if err != nil {
		t.Fatal(err)
	}
	want := sourcefetch.ExtractBlocks(article)
	if first.CaptureID == "" || !first.New || first.MediaType != "text/html" || first.RawBytes != len(article) ||
		first.BlockCount != len(want.Blocks) || first.DocHash != want.DocHash() || first.Extractor != sourcefetch.ExtractorVersion || first.Truncated {
		t.Fatalf("first read = %+v", first)
	}
	if len(first.Blocks) != len(want.Blocks) || first.Blocks[0].Text != "Harbor Council Approves Flood Wall" {
		t.Fatalf("blocks = %+v", first.Blocks)
	}
	// The ids Read shows are the stored capture's.
	stored, err := deps.Captures.ReadCapture(context.Background(), first.CaptureID, 0, 100000)
	if err != nil {
		t.Fatal(err)
	}
	var capture platform.CaptureReadResponse
	if err := json.Unmarshal(stored, &capture); err != nil {
		t.Fatal(err)
	}
	for i, b := range first.Blocks {
		if capture.Blocks[i].ID != b.ID || want.Blocks[i].ID != b.ID {
			t.Fatalf("block %d id %q; stored %q; cid %q", i, b.ID, capture.Blocks[i].ID, want.Blocks[i].ID)
		}
	}

	again, err := read("/harbor")
	if err != nil || again.New || again.CaptureID != first.CaptureID {
		t.Fatalf("unchanged re-read = %+v, %v; want the first capture", again, err)
	}
	edited.Store(true)
	changed, err := read("/harbor")
	if err != nil || !changed.New || changed.CaptureID == first.CaptureID {
		t.Fatalf("changed re-read = %+v, %v; want a new capture", changed, err)
	}

	report, err := read("/report.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if report.MediaType != "application/pdf" || report.Extractor != sourcefetch.PDFExtractorVersion || report.RawBytes != len(pdf) || report.BlockCount == 0 {
		t.Fatalf("pdf read = %+v", report)
	}
	pagesSeen := map[int]bool{}
	for _, b := range report.Blocks {
		pagesSeen[b.Page] = true
	}
	if !pagesSeen[1] || !pagesSeen[2] {
		t.Fatalf("pdf blocks lack page numbers: %+v", report.Blocks)
	}

	longRead, err := read("/long")
	if err != nil {
		t.Fatal(err)
	}
	if !longRead.Truncated || longRead.NextBlock == 0 || longRead.NextBlock != len(longRead.Blocks) || longRead.BlockCount != 132 {
		t.Fatalf("long read = truncated %v next %d shown %d of %d", longRead.Truncated, longRead.NextBlock, len(longRead.Blocks), longRead.BlockCount)
	}
	if len(longRead.Headings) != 12 || longRead.Headings[11].Text != "Section 12" || longRead.Headings[11].Index < longRead.NextBlock {
		t.Fatalf("long read headings = %+v", longRead.Headings)
	}
	more, err := deps.HostEgress(ctx, "read_capture", json.RawMessage(fmt.Sprintf(`{"capture_id":%q,"from_block":%d}`, longRead.CaptureID, longRead.NextBlock)))
	if err != nil {
		t.Fatal(err)
	}
	var rest platform.CaptureReadResponse
	if err := json.Unmarshal(more, &rest); err != nil || rest.FromBlock != longRead.NextBlock || len(rest.Blocks) == 0 {
		t.Fatalf("read_capture from_block = %+v, %v", rest, err)
	}

	for path, wantErr := range map[string]string{"/image.png": "image/png", "/missing": "404"} {
		if _, err := read(path); err == nil || !strings.Contains(err.Error(), wantErr) {
			t.Fatalf("read %s error = %v, want it to name %q", path, err, wantErr)
		}
	}
	if _, err := deps.HostEgress(ctx, "read", json.RawMessage(`{"url":"file:///etc/passwd"}`)); err == nil {
		t.Fatal("read accepted a file URL")
	}
	if _, err := deps.HostEgress(context.Background(), "read", json.RawMessage(`{"url":"`+pages.URL+`/harbor"}`)); err == nil {
		t.Fatal("read ran without an owner")
	}

	sourcefetch.SetAllowPrivateNetworkForTests(false)
	if _, err := read("/harbor"); err == nil || !strings.Contains(err.Error(), "forbidden address") {
		t.Fatalf("read reached a loopback server: %v", err)
	}
}
