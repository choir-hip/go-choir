package textureowner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// X7 (modular architecture plan, section 8.4; MA3.7a) is resolved. Management's
// ProductAPI reaches the Texture list route in-process, on the run's context.
// The route answers a desk from the story index Texture publishes, not from
// Texture's documents, so the same list read as management is no crossing, and
// a list read as the owner or as Texture still lists every document.
func TestManagementProductAPIListOfTextureRoutesCrossesNothing(t *testing.T) {
	_, handler := testAPISetup(t)
	create := httptest.NewRecorder()
	handler.HandleTextureCreateDocument(create, textureRequest(t, http.MethodPost, "/api/texture/documents", textureCreateDocRequest{Title: "x7"}))
	if create.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", create.Code, create.Body.String())
	}
	defer objectgraph.SetModeForTest(objectgraph.WriterGateObserve)()
	objectgraph.ResetReaderGateCounts()

	list := func(reader objectgraph.Writer) (int, int) {
		req := textureRequest(t, http.MethodGet, "/api/texture/documents", nil)
		if reader != "" {
			req = req.WithContext(objectgraph.WithReader(context.Background(), reader))
		}
		w := httptest.NewRecorder()
		handler.HandleTextureListDocuments(w, req)
		var body struct {
			Documents []json.RawMessage `json:"documents"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		return w.Code, len(body.Documents)
	}
	if code, n := list(""); code != http.StatusOK || n != 1 {
		t.Fatalf("owner list: %d, %d documents; want 200 and the owner's document", code, n)
	}
	// Texture's own runs list the index too: a story run has no need of the
	// owner's other documents, which a public publish would carry (2026-11-08).
	if code, n := list(objectgraph.WriterTexture); code != http.StatusOK || n != 0 {
		t.Fatalf("texture list: %d, %d documents; want 200 from the index", code, n)
	}
	// The desk lists the index: stories only, and the owner's plain document
	// is Texture's, not management's.
	if code, n := list(objectgraph.WriterManagement); code != http.StatusOK || n != 0 {
		t.Fatalf("management list: %d, %d documents; want 200 from the index (no story yet)", code, n)
	}
	for _, kind := range []string{"choir.texture_document", "choir.texture_revision", "choir.story_library_head"} {
		key := kind + "|" + string(objectgraph.WriterManagement) + "|wrong_reader"
		if got := objectgraph.ReaderGateCounts()[key]; got != 0 {
			t.Fatalf("counter %q = %d; management's list must cross nothing", key, got)
		}
	}
}
