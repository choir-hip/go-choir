package proxy

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// A computer publishes its own stories (Cut 2 2b, automatic publishing).
// Ways it can fail, written before the code:
//   - the guest has no publish path at all (the owner's route needs a
//     browser session), or a second publish path that skips the owner
//     path's reads and checks;
//   - a guest publishes another owner's or another computer's document
//     (corpusd trusts the proxy as an internal caller, so the proxy binds
//     the guest);
//   - an unbound caller publishes;
//   - the version number comes from the caller rather than the revision;
//   - a guest writes verdicts for another owner;
//   - the guest's publish is not marked publish_mode auto, so its story
//     never reaches the Autopaper front; or the guest names the owner or
//     the mode in its body and the proxy forwards it.

func TestGuestPublishesItsStoryThroughTheOwnerPath(t *testing.T) {
	var mu sync.Mutex
	var published []platform.PublishTextureRequest
	var verdicts []types.StoryVerdictsRequest
	corpusMux := http.NewServeMux()
	corpusMux.HandleFunc("/internal/platform/publications/texture", func(w http.ResponseWriter, r *http.Request) {
		var req platform.PublishTextureRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		mu.Lock()
		published = append(published, req)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(platform.PublishTextureResponse{PublicationID: "pub-1", RoutePath: "/pub/texture/harbor-pub1",
			PermalinkPath: "/pub/texture/harbor-pub1/v" + strings.TrimPrefix(req.SourceRevisionID, "rev-"), Revision: req.RevisionNumber, State: "published"})
	})
	corpusMux.HandleFunc(types.StoryVerdictsPath, func(w http.ResponseWriter, r *http.Request) {
		var req types.StoryVerdictsRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		mu.Lock()
		verdicts = append(verdicts, req)
		mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]int{"recorded": len(req.Verdicts)})
	})
	var relations []types.StoryRelationsRequest
	corpusMux.HandleFunc(types.StoryRelationsPath, func(w http.ResponseWriter, r *http.Request) {
		var req types.StoryRelationsRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		mu.Lock()
		relations = append(relations, req)
		mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]int{"recorded": len(req.Relations)})
	})
	corpusd := httptest.NewServer(corpusMux)
	defer corpusd.Close()

	autoputer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Authenticated-User") != "paper-owner" {
			t.Errorf("autoputer read as %q", r.Header.Get("X-Authenticated-User"))
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/texture/documents/doc-story":
			_ = json.NewEncoder(w).Encode(autoputerTextureDocument{DocID: "doc-story", OwnerID: "paper-owner", Title: "Harbor reopens", CurrentRevisionID: "rev-3"})
		case "/api/texture/revisions/rev-2":
			_ = json.NewEncoder(w).Encode(autoputerTextureRevision{RevisionID: "rev-2", DocID: "doc-story", OwnerID: "paper-owner", VersionNumber: 1,
				RevisionHash: "h2", Content: "The harbor reopens Monday.", Metadata: json.RawMessage(`{"texture_citation_checks":[]}`)})
		default:
			http.NotFound(w, r)
		}
	}))
	defer autoputer.Close()

	guests := map[string]vmctl.GuestOwnership{
		"10.200.1.2": {Found: true, UserID: "paper-owner", ComputerID: "computer-paper", ComputerURL: autoputer.URL},
	}
	vmctlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			RemoteAddr string `json:"remote_addr"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		_ = json.NewEncoder(w).Encode(guests[strings.Split(req.RemoteAddr, ":")[0]])
	}))
	defer vmctlServer.Close()

	handler, _, _ := testProxyEnv(t)
	handler.cfg.CorpusdURL = corpusd.URL
	handler.vmctlClient = vmctl.NewClient(vmctlServer.URL)
	srv := server.NewServer("proxy-test", "0")
	RegisterRoutes(srv, handler)
	send := func(path, from string, body any) *httptest.ResponseRecorder {
		t.Helper()
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
		req.RemoteAddr = from + ":40000"
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		return rec
	}

	if rec := send(types.StoryPublishPath, "10.200.9.9", types.StoryPublishRequest{DocID: "doc-story", RevisionID: "rev-2"}); rec.Code != http.StatusForbidden {
		t.Fatalf("an unbound caller published: %d %s", rec.Code, rec.Body)
	}
	rec := send(types.StoryPublishPath, "10.200.1.2", types.StoryPublishRequest{DocID: "doc-story", RevisionID: "rev-2"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("guest publish = %d %s", rec.Code, rec.Body)
	}
	var resp platform.PublishTextureResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil || resp.Revision != 2 {
		t.Fatalf("guest publish response = %+v (%v); want v2", resp, err)
	}
	mu.Lock()
	if len(published) != 1 || published[0].OwnerID != "paper-owner" || published[0].SourceDocID != "doc-story" ||
		published[0].SourceRevisionID != "rev-2" || published[0].RevisionNumber != 2 || published[0].Title != "Harbor reopens" ||
		!strings.Contains(string(published[0].Metadata), "texture_citation_checks") || published[0].PublishMode != platform.PublishModeAuto {
		t.Fatalf("corpusd publish = %+v", published)
	}
	mu.Unlock()
	// The guest names only the document and revision: an owner or a mode
	// in its body is refused, never forwarded.
	for _, body := range []map[string]string{
		{"doc_id": "doc-story", "revision_id": "rev-2", "owner_id": "reader-owner"},
		{"doc_id": "doc-story", "revision_id": "rev-2", "publish_mode": "owner"},
	} {
		if rec := send(types.StoryPublishPath, "10.200.1.2", body); rec.Code != http.StatusBadRequest {
			t.Fatalf("guest body %v: %d %s", body, rec.Code, rec.Body)
		}
	}
	// A revision the owner's computer does not have is not published.
	if rec := send(types.StoryPublishPath, "10.200.1.2", types.StoryPublishRequest{DocID: "doc-story", RevisionID: "rev-9"}); rec.Code < 400 {
		t.Fatalf("a missing revision published: %d", rec.Code)
	}

	verdict := []types.StoryVerdict{{Key: "sp-1", Verdict: types.SupportSupported, Judge: "management"}}
	if rec := send(types.StoryVerdictsPath, "10.200.1.2", types.StoryVerdictsRequest{OwnerID: "reader-owner", ComputerID: "computer-paper", Verdicts: verdict}); rec.Code != http.StatusForbidden {
		t.Fatalf("a guest wrote another owner's verdicts: %d %s", rec.Code, rec.Body)
	}
	if rec := send(types.StoryVerdictsPath, "10.200.1.2", types.StoryVerdictsRequest{OwnerID: "paper-owner", ComputerID: "computer-paper", Verdicts: verdict}); rec.Code != http.StatusOK {
		t.Fatalf("guest verdicts = %d %s", rec.Code, rec.Body)
	}
	// The judged pair relations of the related_story edge pass under the same
	// bound-guest rule: another owner's are refused, the guest's own reach
	// corpusd.
	rel := []types.StoryRelation{{StoryA: "obj:choir.publication:cGFwZXItb3duZXI:a", StoryB: "obj:choir.publication:cGFwZXItb3duZXI:b", Relation: types.StoryRelationSameIncident}}
	if rec := send(types.StoryRelationsPath, "10.200.1.2", types.StoryRelationsRequest{OwnerID: "reader-owner", ComputerID: "computer-paper", Relations: rel}); rec.Code != http.StatusForbidden {
		t.Fatalf("a guest wrote another owner's relations: %d %s", rec.Code, rec.Body)
	}
	if rec := send(types.StoryRelationsPath, "10.200.1.2", types.StoryRelationsRequest{OwnerID: "paper-owner", ComputerID: "computer-other", Relations: rel}); rec.Code != http.StatusForbidden {
		t.Fatalf("a guest wrote another computer's relations: %d %s", rec.Code, rec.Body)
	}
	if rec := send(types.StoryRelationsPath, "10.200.1.2", types.StoryRelationsRequest{OwnerID: "paper-owner", ComputerID: "computer-paper", Relations: rel}); rec.Code != http.StatusOK {
		t.Fatalf("guest relations = %d %s", rec.Code, rec.Body)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(verdicts) != 1 || verdicts[0].OwnerID != "paper-owner" || len(verdicts[0].Verdicts) != 1 {
		t.Fatalf("corpusd verdicts = %+v", verdicts)
	}
	if len(relations) != 1 || relations[0].OwnerID != "paper-owner" || len(relations[0].Relations) != 1 {
		t.Fatalf("corpusd relations = %+v", relations)
	}
}
