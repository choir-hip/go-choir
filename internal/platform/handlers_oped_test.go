package platform

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// X4, X9, X19 over HTTP: a caller bound to nothing is refused; the feed is the
// owner the caller is bound to; a verdict for another owner's story is 403.
func TestOpedRoutesAreScopedToTheBoundOwner(t *testing.T) {
	f := newOpedFixture(t)
	ref := f.ref()
	op, err := f.publishOped(t, "reader-1", "writer-aa11bb", "doc-fork", 1, &ref, ref)
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(f.svc)
	do := func(method, path string, body any, internal bool) *httptest.ResponseRecorder {
		var rdr *bytes.Reader
		if body != nil {
			raw, _ := json.Marshal(body)
			rdr = bytes.NewReader(raw)
		} else {
			rdr = bytes.NewReader(nil)
		}
		req := httptest.NewRequest(method, path, rdr)
		if internal {
			req.Header.Set("X-Internal-Caller", "true")
		}
		w := httptest.NewRecorder()
		switch {
		case path[:len(opedFeedPath)] == opedFeedPath:
			h.HandleInternalOpeds(w, req)
		case path[:len(noticeFeedPath)] == noticeFeedPath:
			h.HandleInternalNotices(w, req)
		default:
			h.HandleInternalOpedVerdicts(w, req)
		}
		return w
	}
	// Unbound callers.
	for _, c := range []struct{ method, path string }{{"GET", opedFeedPath + "?owner_id=paper-owner"}, {"GET", noticeFeedPath + "?owner_id=paper-owner"}, {"POST", opedVerdictsPath}} {
		if w := do(c.method, c.path, OpedVerdictPost{OwnerID: "paper-owner"}, false); w.Code != http.StatusForbidden {
			t.Fatalf("%s %s unbound = %d, want 403", c.method, c.path, w.Code)
		}
	}
	// The proxy relays the bound owner.
	w := do("GET", opedFeedPath+"?owner_id=paper-owner", nil, true)
	var feed OpedFeed
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &feed) != nil || len(feed.Items) != 1 {
		t.Fatalf("paper feed = %d %s", w.Code, w.Body)
	}
	w = do("GET", opedFeedPath+"?owner_id=reader-2", nil, true)
	if json.Unmarshal(w.Body.Bytes(), &feed) != nil || len(feed.Items) != 0 {
		t.Fatalf("another owner's feed = %s", w.Body)
	}
	if w := do("GET", opedFeedPath+"?owner_id=paper-owner&limit=x", nil, true); w.Code != http.StatusBadRequest {
		t.Fatalf("bad limit = %d", w.Code)
	}
	// A verdict for a story that is not the caller's.
	post := OpedVerdictPost{OwnerID: "reader-2", VersionID: op.PublicationVersionID, Run: 1, Verdict: "accepted", VerdictHash: "h", TapeRecordID: "r"}
	if w := do("POST", opedVerdictsPath, post, true); w.Code != http.StatusForbidden {
		t.Fatalf("foreign verdict = %d, want 403", w.Code)
	}
	post.OwnerID = "paper-owner"
	if w := do("POST", opedVerdictsPath, post, true); w.Code != http.StatusOK {
		t.Fatalf("paper verdict = %d %s", w.Code, w.Body)
	}
}
