package proxy

// Ways an op-ed's links can be forged or lost at the proxy (docs/evidence/c2-opeds-failure-list-2026-11-10.md,
// X2, X16):
//
//  - a body names parents or a responds_to the revision never carried;
//  - a fork edited by its owner (a revision with no links of its own) is
//    published without the link its first revision recorded;
//  - the paper's response (the desk's revision, which inherited the op-ed
//    link) is published without citing the op-ed.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func provenanceWithSource(t *testing.T, versionID, hash string) json.RawMessage {
	t.Helper()
	raw, err := types.Provenance{SchemaVersion: types.ProvenanceSchemaVersion, ParentLinks: []types.ParentLink{
		{Role: types.ParentLinkBase, RevisionID: "rev-0"},
		{Role: types.ParentLinkSource, PublicationVersionID: versionID, ManifestHash: hash, RoutePath: "/pub/texture/story/v1"},
	}}.CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestOpedLinksComeFromTheRevisionsOwnProvenance(t *testing.T) {
	var posted []platform.PublishTextureRequest
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req platform.PublishTextureRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		posted = append(posted, req)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(platform.PublishTextureResponse{RoutePath: "/pub/texture/my-view-abc", PermalinkPath: "/pub/texture/my-view-abc/v1", Revision: 1, State: "published"})
	}))
	defer corpusd.Close()
	revisions := map[string]autoputerTextureRevision{
		// The fork's first revision carries the source link; the owner's later edit does not.
		"rev-1": {RevisionID: "rev-1", DocID: "doc-1", OwnerID: "user-1", VersionNumber: 1, RevisionHash: "h1", Content: "Fork text.",
			Provenance: provenanceWithSource(t, "pub-story-v1", "manifest-1")},
		"rev-2": {RevisionID: "rev-2", DocID: "doc-1", OwnerID: "user-1", VersionNumber: 2, RevisionHash: "h2", Content: "Edited fork text.", ParentRevisionID: "rev-1"},
		"rev-3": {RevisionID: "rev-3", DocID: "doc-1", OwnerID: "user-1", VersionNumber: 3, RevisionHash: "h3", Content: "Edited again.", ParentRevisionID: "rev-2"},
	}
	autoputer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/texture/documents/doc-1":
			_ = json.NewEncoder(w).Encode(autoputerTextureDocument{DocID: "doc-1", OwnerID: "user-1", Title: "My view", CurrentRevisionID: "rev-3"})
		case strings.HasPrefix(r.URL.Path, "/api/texture/revisions/"):
			rev, ok := revisions[strings.TrimPrefix(r.URL.Path, "/api/texture/revisions/")]
			if !ok {
				http.NotFound(w, r)
				return
			}
			_ = json.NewEncoder(w).Encode(rev)
		default:
			http.NotFound(w, r)
		}
	}))
	defer autoputer.Close()
	h, priv := opinionTestHandler(t, autoputer.URL, corpusd.URL)
	post := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "https://choir.news/api/platform/texture/opinions", strings.NewReader(body))
		req.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWTWithEmail(priv, "user-1", "reader.person@example.com")})
		w := httptest.NewRecorder()
		h.HandleAPI(w, req)
		return w
	}
	// A body cannot carry the links.
	for _, body := range []string{
		`{"doc_id":"doc-1","responds_to":{"publication_version_id":"pub-x-v1","manifest_hash":"m"}}`,
		`{"doc_id":"doc-1","parents":[{"publication_version_id":"pub-x-v1","manifest_hash":"m"}]}`,
	} {
		if w := post(body); w.Code != http.StatusBadRequest {
			t.Fatalf("body %s = %d, want 400", body, w.Code)
		}
	}
	if len(posted) != 0 {
		t.Fatalf("corpusd was reached: %+v", posted)
	}
	// The head revision has no links; the walk finds the fork's first revision's.
	if w := post(`{"doc_id":"doc-1","slug":"my-view"}`); w.Code != http.StatusCreated {
		t.Fatalf("publish = %d %s", w.Code, w.Body)
	}
	if len(posted) != 1 {
		t.Fatalf("corpusd calls = %d", len(posted))
	}
	got := posted[0]
	// The proxy sends parents only; corpusd derives responds_to from the verified ones.
	if got.RespondsTo != nil {
		t.Fatalf("responds_to = %+v, want none from the proxy", got.RespondsTo)
	}
	if len(got.Parents) != 1 || got.Parents[0].PublicationVersionID != "pub-story-v1" {
		t.Fatalf("parents = %+v", got.Parents)
	}
}

// The paper's own response: a story revision whose provenance carries the
// op-ed link is published with the op-ed as a parent.
func TestStoryResponseCitesTheOpedFromProvenance(t *testing.T) {
	links := derivePublishLinks(provenanceWithSource(t, "pub-oped-v1", "manifest-op"), platform.PublishModeAuto)
	if len(links.Parents) != 1 || links.Parents[0].PublicationVersionID != "pub-oped-v1" || links.RespondsTo != nil {
		t.Fatalf("links = %+v", links)
	}
	none := derivePublishLinks(json.RawMessage(`{"schema_version":1}`), platform.PublishModeOpinion)
	if len(none.Parents) != 0 || none.RespondsTo != nil {
		t.Fatalf("a revision without links gave %+v", none)
	}
	if bad := derivePublishLinks(json.RawMessage(`not json`), platform.PublishModeOpinion); len(bad.Parents) != 0 {
		t.Fatalf("garbage provenance gave %+v", bad)
	}
}
