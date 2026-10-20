package platform

// Ways the expansion-source read can fail (story expansion design phase 1,
// section 4.1, step 2), written before the code. The read answers one
// question for the proxy: which paper document does this public route
// publish, so a signed-in owner's "more research" can reach that document's
// Texture desk. The proxy builds its call to the paper from this answer
// alone, never from the request body (design F4), so a wrong answer is a
// wrong document revised.
//
//  1. A hand-published Texture, or a story from an owner off the Autopaper
//     allowlist, is returned: an expansion request would reach a document
//     that is not the paper's.
//  2. A held, unpublished or unknown route is told apart from a non-story
//     route (a probe learns which routes exist); all of them must be the
//     same not-found error, and the internal route the same 404 body.
//  3. The source of an older version is returned instead of the latest, so
//     the request identity (publication + source revision) points at a
//     revision Texture has already moved past, or a per-version limit is
//     keyed to the wrong version.
//  4. A /vN permalink resolves to that older version's source instead of the
//     story's latest.
//  5. The owner returned is the caller's or the route's text rather than the
//     publication's owner, or another owner's doc with the same doc id is
//     mixed in.
//  6. The internal route serves a caller that is not host-internal, serves a
//     POST, or takes a request without a route.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestExpansionSourceNamesTheLatestVersionsDocument(t *testing.T) {
	store, root := openTestPlatformStore(t)
	svc := NewService(store, filepath.Join(root, "artifacts"), filepath.Join(root, "signing.key"))
	ctx := context.Background()
	svc.SetAutopaperOwners([]string{"paper-owner"})

	publishAs(t, svc, "paper-owner", PublishModeAuto, "doc-1", 1, "Harbor ferries", "First version.", nil)
	latest := publishAs(t, svc, "paper-owner", PublishModeAuto, "doc-1", 3, "Harbor ferries", "Third version.", nil)
	// Another owner's story with the same doc id must never be mixed in.
	publishAs(t, svc, "tracer-owner", PublishModeAuto, "doc-1", 1, "Tracer story", "Tracer.", nil)

	got, err := svc.ExpansionSource(ctx, latest.RoutePath)
	if err != nil {
		t.Fatalf("expansion source: %v", err)
	}
	if got.OwnerID != "paper-owner" || got.SourceDocID != "doc-1" || got.PublicationID != latest.PublicationID {
		t.Fatalf("source = %+v, want paper-owner's doc-1 (%s)", got, latest.PublicationID)
	}
	if got.LatestRevision != 3 || got.SourceRevisionID != "doc-1-rev-3" {
		t.Fatalf("latest = (v%d, %q), want (v3, doc-1-rev-3)", got.LatestRevision, got.SourceRevisionID)
	}
	if got.Route != latest.RoutePath {
		t.Fatalf("route = %q, want bare %q", got.Route, latest.RoutePath)
	}

	// An older version's permalink names the same story's latest source.
	older, err := svc.ExpansionSource(ctx, versionPermalink(latest.RoutePath, 1))
	if err != nil {
		t.Fatalf("expansion source by permalink: %v", err)
	}
	if older.LatestRevision != 3 || older.SourceRevisionID != "doc-1-rev-3" || older.Route != latest.RoutePath {
		t.Fatalf("permalink source = %+v, want the latest (v3) under the bare route", older)
	}
}

func TestExpansionSourceRefusesWhatIsNotAPaperStory(t *testing.T) {
	store, root := openTestPlatformStore(t)
	svc := NewService(store, filepath.Join(root, "artifacts"), filepath.Join(root, "signing.key"))
	ctx := context.Background()
	svc.SetAutopaperOwners([]string{"paper-owner"})

	hand := publishAs(t, svc, "paper-owner", PublishModeOwner, "doc-hand", 1, "Hand note", "By hand.", nil)
	offList := publishAs(t, svc, "tracer-owner", PublishModeAuto, "doc-2", 1, "Tracer story", "Tracer.", nil)

	for name, route := range map[string]string{
		"hand-published": hand.RoutePath,
		"off allowlist":  offList.RoutePath,
		"unknown":        "/pub/texture/never-published-abc123",
		"not a route":    "/somewhere/else",
	} {
		if _, err := svc.ExpansionSource(ctx, route); !errors.Is(err, ErrPublicationNotFound) {
			t.Fatalf("%s: err = %v, want ErrPublicationNotFound", name, err)
		}
	}
}

func TestInternalExpansionSourceRoute(t *testing.T) {
	store, root := openTestPlatformStore(t)
	svc := NewService(store, filepath.Join(root, "artifacts"), filepath.Join(root, "signing.key"))
	handler := NewHandler(svc)
	pub := publishAs(t, svc, "user-1", PublishModeAuto, "doc-1", 1, "Story", "Body.", nil)
	hand := publishAs(t, svc, "user-1", PublishModeOwner, "doc-hand", 1, "Hand note", "By hand.", nil)

	call := func(method, query string, internal bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/internal/platform/publications/expansion-source"+query, nil)
		if internal {
			req.Header.Set("X-Internal-Caller", "true")
		}
		w := httptest.NewRecorder()
		handler.HandleInternalExpansionSource(w, req)
		return w
	}

	if w := call(http.MethodGet, "?route="+pub.RoutePath, false); w.Code != http.StatusForbidden {
		t.Fatalf("external caller: got %d, want 403", w.Code)
	}
	if w := call(http.MethodPost, "?route="+pub.RoutePath, true); w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST: got %d, want 405", w.Code)
	}
	if w := call(http.MethodGet, "", true); w.Code != http.StatusBadRequest {
		t.Fatalf("no route: got %d, want 400", w.Code)
	}
	unknown := call(http.MethodGet, "?route=/pub/texture/never-abc", true)
	notStory := call(http.MethodGet, "?route="+hand.RoutePath, true)
	if unknown.Code != http.StatusNotFound || notStory.Code != http.StatusNotFound {
		t.Fatalf("unknown=%d notStory=%d, want 404 and 404", unknown.Code, notStory.Code)
	}
	if unknown.Body.String() != notStory.Body.String() {
		t.Fatalf("404 bodies differ (a probe can tell them apart): %q vs %q", unknown.Body.String(), notStory.Body.String())
	}
	w := call(http.MethodGet, "?route="+pub.RoutePath, true)
	if w.Code != http.StatusOK {
		t.Fatalf("source read: got %d body %s", w.Code, w.Body.String())
	}
	var got ExpansionSource
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.OwnerID != "user-1" || got.SourceDocID != "doc-1" || got.LatestRevision != 1 || got.SourceRevisionID != "doc-1-rev-1" {
		t.Fatalf("source = %+v", got)
	}
}
