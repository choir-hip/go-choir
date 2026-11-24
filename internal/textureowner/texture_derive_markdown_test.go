package textureowner

// "derive_from_markdown" on POST /api/texture/documents/{id}/revisions (revise_edit.go): the owner sends the text he typed and the parent revision; the server derives body_doc (texturedoc.MergeMarkdown),
// keeping what the text did not touch. Failure list, written before the code:
//   D1  the field is refused today (an unknown field: 400): a text edit of a structured revision with a source_ref saves, keeps the ref and its entity, and the stored content is the projection;
//   D2  a client that sends the field AND its own body_doc or source_entities is refused (one author of the body);
//   D3  a document with no revision takes the plain text whole;
//   D4  the owner route's draft rewrite does not act on derived content (a table-shaped draft is stored as the projection says).

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/texturedoc"
)

func deriveFixture(t *testing.T) (docID, revID string, h *Handler) {
	t.Helper()
	_, handler := testAPISetup(t)
	w := httptest.NewRecorder()
	handler.HandleTextureCreateDocument(w, textureRequest(t, http.MethodPost, "/api/texture/documents", textureCreateDocRequest{Title: "Derive"}))
	var created textureCreateDocResponse
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil || w.Code != http.StatusCreated {
		t.Fatalf("create doc %d %v", w.Code, err)
	}
	return created.DocID, "", handler
}

func postRevision(t *testing.T, h *Handler, docID string, req textureCreateRevisionRequest) (*httptest.ResponseRecorder, textureRevisionResponse) {
	t.Helper()
	w := httptest.NewRecorder()
	h.HandleTextureRevisions(w, textureRequest(t, http.MethodPost, "/api/texture/documents/"+docID+"/revisions", req))
	var rev textureRevisionResponse
	if w.Code == http.StatusCreated {
		if err := json.NewDecoder(w.Body).Decode(&rev); err != nil {
			t.Fatal(err)
		}
	}
	return w, rev
}

func TestDeriveFromMarkdownKeepsTheParentsSources(t *testing.T) { // D1
	docID, _, h := deriveFixture(t)
	bodyDoc, entities := runtimeStructuredRevisionFixture(t)
	w, first := postRevision(t, h, docID, textureCreateRevisionRequest{BodyDoc: bodyDoc, SourceEntities: entities})
	if w.Code != http.StatusCreated {
		t.Fatalf("parent %d %s", w.Code, w.Body.String())
	}
	w, second := postRevision(t, h, docID, textureCreateRevisionRequest{Content: "Grounded[1]. More words.", ParentRevisionID: first.RevisionID, DeriveFromMarkdown: true})
	if w.Code != http.StatusCreated {
		t.Fatalf("derive %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(second.Content, "More words") || !strings.Contains(string(second.BodyDoc), `"source_ref"`) || !strings.Contains(string(second.SourceEntities), "source_entity_id") {
		t.Fatalf("the derived revision lost its source: content=%q body=%s entities=%s", second.Content, second.BodyDoc, second.SourceEntities)
	}
	var doc texturedoc.StructuredTextureDoc
	var ents []texturedoc.SourceEntity
	if json.Unmarshal(second.BodyDoc, &doc) != nil || json.Unmarshal(second.SourceEntities, &ents) != nil {
		t.Fatal("the stored body does not decode")
	}
	if p, err := texturedoc.Project(doc, ents); err != nil || p.Text != second.Content {
		t.Fatalf("stored content %q is not the projection %q (%v)", second.Content, p.Text, err)
	}
}

func TestDeriveFromMarkdownRefusesAClientBody(t *testing.T) { // D2
	docID, _, h := deriveFixture(t)
	bodyDoc, entities := runtimeStructuredRevisionFixture(t)
	w, _ := postRevision(t, h, docID, textureCreateRevisionRequest{Content: "x", DeriveFromMarkdown: true, BodyDoc: bodyDoc})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("body_doc with derive: %d %s", w.Code, w.Body.String())
	}
	w, _ = postRevision(t, h, docID, textureCreateRevisionRequest{Content: "x", DeriveFromMarkdown: true, SourceEntities: entities})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("source_entities with derive: %d %s", w.Code, w.Body.String())
	}
}

func TestDeriveFromMarkdownOnADocumentWithNoRevision(t *testing.T) { // D3
	docID, _, h := deriveFixture(t)
	w, rev := postRevision(t, h, docID, textureCreateRevisionRequest{Content: "Hello\nThis is my first time using texture editing on autoradio 1", DeriveFromMarkdown: true})
	if w.Code != http.StatusCreated {
		t.Fatalf("derive %d %s", w.Code, w.Body.String())
	}
	if rev.Content != "Hello\nThis is my first time using texture editing on autoradio 1" || !strings.Contains(string(rev.BodyDoc), `"hard_break"`) {
		t.Fatalf("content=%q body=%s", rev.Content, rev.BodyDoc)
	}
}

func TestDeriveFromMarkdownIsNotRewrittenByTheDraftStabilizer(t *testing.T) { // D4
	docID, _, h := deriveFixture(t)
	table := "Intro\n\n| a | b |\n|---|---|\n| 1 | 2 |"
	w, first := postRevision(t, h, docID, textureCreateRevisionRequest{Content: table, DeriveFromMarkdown: true})
	if w.Code != http.StatusCreated {
		t.Fatalf("first %d %s", w.Code, w.Body.String())
	}
	collapsed := "Intro\n\na b 1 2"
	w, second := postRevision(t, h, docID, textureCreateRevisionRequest{Content: collapsed, ParentRevisionID: first.RevisionID, DeriveFromMarkdown: true})
	if w.Code != http.StatusCreated {
		t.Fatalf("second %d %s", w.Code, w.Body.String())
	}
	if second.Content != collapsed {
		t.Fatalf("the draft stabilizer rewrote derived content: %q", second.Content)
	}
}
