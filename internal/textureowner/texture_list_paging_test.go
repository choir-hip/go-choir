package textureowner

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// GET /api/texture/documents hard-coded 50 and ignored ?limit, so scripts
// asking for 100 or 500 were silently truncated and documents past the newest
// 50 could not be listed at all (docs/archive/problems/surface-broken-calls-2026-10-14.md,
// item 7).
//
// Failure modes pinned: limit ignored; no way past the first page; a page
// repeats or skips a document; the default page is unbounded; a bad limit
// or cursor is silently accepted.
func TestTextureListDocumentsHonoursLimitAndPages(t *testing.T) {
	_, handler := testAPISetup(t)

	const total = 55
	for i := 0; i < total; i++ {
		w := httptest.NewRecorder()
		handler.HandleTextureCreateDocument(w, textureRequest(t, http.MethodPost, "/api/texture/documents", textureCreateDocRequest{Title: fmt.Sprintf("doc %02d", i)}))
		if w.Code != http.StatusCreated {
			t.Fatalf("create doc %d status = %d body=%s", i, w.Code, w.Body.String())
		}
	}

	list := func(query string) (int, textureListDocsResponse) {
		t.Helper()
		w := httptest.NewRecorder()
		handler.HandleTextureListDocuments(w, textureRequest(t, http.MethodGet, "/api/texture/documents"+query, nil))
		var resp textureListDocsResponse
		if w.Code == http.StatusOK {
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("decode list %q: %v", query, err)
			}
		}
		return w.Code, resp
	}

	// A limit above the old hard-coded 50 is honoured.
	if code, resp := list("?limit=500"); code != http.StatusOK || len(resp.Documents) != total || resp.NextCursor != "" {
		t.Fatalf("limit=500: status=%d documents=%d next=%q, want %d and no next page", code, len(resp.Documents), resp.NextCursor, total)
	}

	// The default page stays 50 and says there is more.
	code, first := list("")
	if code != http.StatusOK || len(first.Documents) != textureDocumentListDefaultLimit || first.NextCursor == "" {
		t.Fatalf("default: status=%d documents=%d next=%q, want %d and a next page", code, len(first.Documents), first.NextCursor, textureDocumentListDefaultLimit)
	}

	// Paging by 20 visits every document once, newest first.
	seen := map[string]bool{}
	var pages []int
	var lastUpdated string
	cursor := ""
	for page := 0; page < 10; page++ {
		query := "?limit=20"
		if cursor != "" {
			query += "&cursor=" + cursor
		}
		code, resp := list(query)
		if code != http.StatusOK {
			t.Fatalf("page %d status=%d", page, code)
		}
		pages = append(pages, len(resp.Documents))
		for _, doc := range resp.Documents {
			if seen[doc.DocID] {
				t.Fatalf("page %d repeats %s", page, doc.DocID)
			}
			seen[doc.DocID] = true
			if lastUpdated != "" && doc.UpdatedAt > lastUpdated {
				t.Fatalf("page %d not newest first: %s after %s", page, doc.UpdatedAt, lastUpdated)
			}
			lastUpdated = doc.UpdatedAt
		}
		if resp.NextCursor == "" {
			break
		}
		cursor = resp.NextCursor
	}
	if len(seen) != total || fmt.Sprint(pages) != "[20 20 15]" {
		t.Fatalf("paging saw %d documents in pages %v, want %d in [20 20 15]", len(seen), pages, total)
	}

	// Above the maximum the page is clamped, and the cursor still says more.
	if code, resp := list(fmt.Sprintf("?limit=%d", textureDocumentListMaxLimit+1)); code != http.StatusOK || len(resp.Documents) != total {
		t.Fatalf("limit above max: status=%d documents=%d", code, len(resp.Documents))
	}

	for _, query := range []string{"?limit=0", "?limit=-1", "?limit=many", "?cursor=not-a-cursor", "?cursor=e30"} {
		if code, _ := list(query); code != http.StatusBadRequest {
			t.Errorf("%s status=%d, want 400", query, code)
		}
	}
}
