package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
)

type traceSink struct {
	owner, computer string
	reads           []string
}

func (s *traceSink) WriteCaptures(context.Context, string, string, []sourcegraph.CaptureItem) (sourcegraph.CaptureWriteResult, error) {
	return sourcegraph.CaptureWriteResult{}, fmt.Errorf("not used")
}

func (s *traceSink) ReadCapture(_ context.Context, id string, _, _ int) (json.RawMessage, error) {
	s.reads = append(s.reads, id)
	return json.RawMessage(`{"capture_id":"` + id + `","url":"https://example.com/a","extractor":"html1+nt1","doc_hash":"sha256:x","block_count":1,"raw_bytes":10,"blocks":[{"id":"c1","kind":"paragraph","text":"Prices  rose."}]}`), nil
}

func (s *traceSink) ListCaptures(_ context.Context, owner, computer string, _ int) ([]objectgraph.Object, error) {
	s.owner, s.computer = owner, computer
	return []objectgraph.Object{{CanonicalID: "cap-in", ObjectKind: objectgraph.WebCaptureObjectKind, CreatedAt: time.Date(2026, 10, 13, 9, 30, 0, 0, time.UTC)}}, nil
}

// GET /api/captures serves the trace bundle's captures layer (Cut 2
// amendment A5). Failure modes pinned: an unauthenticated caller reads
// captures; the route lists another computer's captures (it must ask for
// this computer's); ids named by the trajectory are not read; a malformed
// window is accepted.
func TestCapturesRouteServesTheTraceWindow(t *testing.T) {
	sink := &traceSink{}
	rt := &Runtime{researchDeps: &researchtools.Dependencies{Captures: sink}}
	rt.cfg.ComputerID = "computer-1"
	handler := &APIHandler{rt: rt}
	get := func(query string, auth bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/captures"+query, nil)
		if auth {
			req.Header.Set("X-Authenticated-User", "owner-1")
		}
		rec := httptest.NewRecorder()
		handler.HandleCaptures(rec, req)
		return rec
	}
	if rec := get("", false); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated = %d", rec.Code)
	}
	if rec := get("?since=yesterday", true); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad window = %d", rec.Code)
	}
	rec := get("?since=2026-10-13T09:00:00Z&until=2026-10-13T10:00:00Z&id=cap-named", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("captures = %d %s", rec.Code, rec.Body)
	}
	var out researchtools.TraceCaptures
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if sink.owner != "owner-1" || sink.computer != "computer-1" || out.ComputerID != "computer-1" {
		t.Fatalf("listed owner %q computer %q; out %+v", sink.owner, sink.computer, out)
	}
	if len(out.Captures) != 2 || out.Captures[0].CaptureID != "cap-in" || out.Captures[1].CaptureID != "cap-named" || out.Captures[0].Blocks[0].Normtext != "Prices rose." {
		t.Fatalf("captures = %+v", out.Captures)
	}
}
