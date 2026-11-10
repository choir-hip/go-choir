package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJevCallLineNamesTheSiteAndBytes(t *testing.T) {
	var buf strings.Builder
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)
	tr := &JevTransport{enabled: true, apiKey: "k", httpClient: &http.Client{Transport: &statusRoundTripper{status: http.StatusOK}},
		receipts: newJevReceiptWriter(filepath.Join(t.TempDir(), "r.jsonl"))}
	req := JudgmentRequest{State: json.RawMessage(`{"a":1}`), Questions: map[string]json.RawMessage{"q": json.RawMessage(`{"type":"choice","choices":["a","b"]}`)}}
	for header, want := range map[string]string{"intake_labels": "site=intake_labels", "": "site=unknown", "Bad Site!": "site=unknown", strings.Repeat("a", 49): "site=unknown"} {
		buf.Reset()
		r, _ := http.NewRequest(http.MethodPost, "/", nil)
		if header != "" {
			r.Header.Set(JevSiteHeader, header)
		}
		_, _ = tr.Decide(withJevSite(context.Background(), r), "c", req)
		line := buf.String()
		if !strings.Contains(line, "gateway: jev call ") || !strings.Contains(line, want+" ") || !strings.Contains(line, fmt.Sprintf("bytes=%d ", jevRequestBytes(req))) {
			t.Fatalf("header %q: line = %q; want %s and bytes", header, line, want)
		}
	}
}
