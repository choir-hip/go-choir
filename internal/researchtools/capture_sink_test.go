package researchtools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
	"github.com/yusefmosiah/go-choir/internal/sources"
)

// A poll large enough to pass corpusd's 64 MiB request bound in one request
// is split into requests under the batch bounds, with each body trimmed to
// corpusd's 1 MiB and raw feed JSON dropped before sending
// (problems/w1-panel-capture-boundary-and-lost-items-2026-10-11.md).
func TestCaptureSinkSplitsALargePollUnderCorpusdsBound(t *testing.T) {
	var requests, items int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if len(raw) > 2*captureBatchBytes {
			t.Errorf("request of %d bytes exceeds the batch bound", len(raw))
		}
		var req struct {
			Items []sources.Item `json:"items"`
		}
		if err := json.Unmarshal(raw, &req); err != nil {
			t.Fatal(err)
		}
		if len(req.Items) > captureBatchItems {
			t.Errorf("request carries %d items", len(req.Items))
		}
		var out sourcegraph.CaptureWriteResult
		for _, item := range req.Items {
			if len(item.Body) > captureBodyBytes || item.RawJSON != "" {
				t.Errorf("item %s was not trimmed: body %d raw %d", item.URL, len(item.Body), len(item.RawJSON))
			}
			out.New++
			out.CaptureIDs = append(out.CaptureIDs, item.URL)
		}
		requests++
		items += len(req.Items)
		_ = json.NewEncoder(w).Encode(out)
	}))
	defer srv.Close()

	var poll []sourcegraph.CaptureItem
	for i := 0; i < 30; i++ {
		poll = append(poll, sourcegraph.CaptureItem{Item: sources.Item{
			URL:     fmt.Sprintf("https://example.test/%d", i),
			Title:   "item",
			Body:    strings.Repeat("x", 1<<20+512),
			RawJSON: strings.Repeat("r", 4096),
		}})
	}
	got, err := NewCorpusdCaptureSink(srv.URL).WriteCaptures(context.Background(), "owner-1", "computer-1", poll)
	if err != nil {
		t.Fatal(err)
	}
	if got.New != 30 || len(got.CaptureIDs) != 30 || items != 30 {
		t.Fatalf("aggregate = %+v, server saw %d items", got, items)
	}
	if requests < 30/(captureBatchBytes>>20) {
		t.Fatalf("30 MiB of bodies went out in %d requests", requests)
	}
	if poll[0].RawJSON == "" || len(poll[0].Body) <= captureBodyBytes {
		t.Fatal("the sink changed the caller's items")
	}
}
