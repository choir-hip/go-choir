package textureowner

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// X7 (modular architecture plan, section 8.4) is counted, not fixed.
// Management's ProductAPI GET /api/texture/ serves the owner's Texture routes
// in-process, on the run's context. The owner's route declares the owner as
// reader only when the request names no actor, so the same route read as the
// owner is an E1 read and read for management is a crossing into Texture's
// documents. A built-in kind only observes until MA3.7 narrows the allowlist
// to list and summary routes, so the read is served; this test is MA3.7's
// target.
func TestManagementProductAPIReadOfTextureRoutesCountsAsManagementReadingTexture(t *testing.T) {
	_, handler := testAPISetup(t)
	create := httptest.NewRecorder()
	handler.HandleTextureCreateDocument(create, textureRequest(t, http.MethodPost, "/api/texture/documents", textureCreateDocRequest{Title: "x7"}))
	if create.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", create.Code, create.Body.String())
	}
	defer objectgraph.SetReaderGateMode(objectgraph.WriterGateObserve)()
	key := "choir.texture_document|" + string(objectgraph.WriterManagement) + "|wrong_reader"

	list := func(reader objectgraph.Writer) int {
		req := textureRequest(t, http.MethodGet, "/api/texture/documents", nil)
		if reader != "" {
			req = req.WithContext(objectgraph.WithReader(context.Background(), reader))
		}
		w := httptest.NewRecorder()
		handler.HandleTextureListDocuments(w, req)
		return w.Code
	}
	before := objectgraph.ReaderGateCounts()
	if code := list(""); code != http.StatusOK {
		t.Fatalf("owner list: %d", code)
	}
	if code := list(objectgraph.WriterTexture); code != http.StatusOK {
		t.Fatalf("texture list: %d", code)
	}
	if got := objectgraph.ReaderGateCounts()[key]; got != before[key] {
		t.Fatalf("the owner's and Texture's reads were counted as management's: %v", got-before[key])
	}
	if code := list(objectgraph.WriterManagement); code != http.StatusOK {
		t.Fatalf("management's read (observed, not refused): %d", code)
	}
	if objectgraph.ReaderGateCounts()[key] <= before[key] {
		t.Fatalf("counter %q did not move (counts: %v)", key, objectgraph.ReaderGateCounts())
	}
}
