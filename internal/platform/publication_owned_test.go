package platform

// Ways the owner's read of one doc's publication can fail (the tracer blind
// spot, docs/archive/problems/tracer-paper-blind-to-test-publications-2026-10-13.md):
//
//  1. Another owner's publication of the same doc id is returned: the read
//     must be scoped by the owner, never by the doc id alone.
//  2. A doc that was never published reads as an error other than not
//     found, so a caller cannot tell "not yet" from "broken".
//  3. A story with several versions reports the wrong version count or
//     latest revision.
//  4. A story the front list leaves out (an owner off the Autopaper
//     allowlist, or a hand-published Texture) is not returned at all, which
//     is the blind spot this read exists to close; or on_front disagrees
//     with what the front list shows.
//  5. The internal route serves a caller that is not host-internal, or
//     takes a request without owner_id or doc_id.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestOwnedPublicationReadsOneOwnersPublication(t *testing.T) {
	store, root := openTestPlatformStore(t)
	svc := NewService(store, filepath.Join(root, "artifacts"), filepath.Join(root, "signing.key"))
	ctx := context.Background()

	// Only paper-owner is on the front; test-owner's stories are not.
	svc.SetAutopaperOwners([]string{"paper-owner"})
	first := publishAs(t, svc, "test-owner", PublishModeAuto, "doc-1", 1, "Test story", "First version.", nil)
	publishAs(t, svc, "test-owner", PublishModeAuto, "doc-1", 2, "Test story", "Second version.", nil)
	publishAs(t, svc, "paper-owner", PublishModeAuto, "doc-1", 1, "Paper story", "Another owner's doc-1.", nil)
	publishAs(t, svc, "test-owner", PublishModeOwner, "doc-hand", 1, "Hand note", "Published by hand.", nil)

	got, err := svc.OwnedPublication(ctx, "test-owner", "doc-1")
	if err != nil {
		t.Fatalf("owned publication: %v", err)
	}
	if got.PublicationID != first.PublicationID || got.Title != "Test story" {
		t.Fatalf("owned publication = %+v, want test-owner's %s", got, first.PublicationID)
	}
	if got.VersionCount != 2 || got.LatestRevision != 2 {
		t.Fatalf("versions = (%d, v%d), want (2, v2)", got.VersionCount, got.LatestRevision)
	}
	if got.Route.Bare != first.RoutePath || got.Route.Permalink != versionPermalink(first.RoutePath, 2) {
		t.Fatalf("route = %+v, want bare %s", got.Route, first.RoutePath)
	}
	if got.PublishMode != PublishModeAuto || got.OnFront {
		t.Fatalf("mode/on_front = (%q, %v), want (auto, false) for an owner off the allowlist", got.PublishMode, got.OnFront)
	}

	paper, err := svc.OwnedPublication(ctx, "paper-owner", "doc-1")
	if err != nil {
		t.Fatalf("paper owner's publication: %v", err)
	}
	if paper.PublicationID == got.PublicationID || paper.Title != "Paper story" || !paper.OnFront {
		t.Fatalf("paper owner's doc-1 = %+v, want its own listed story", paper)
	}

	hand, err := svc.OwnedPublication(ctx, "test-owner", "doc-hand")
	if err != nil {
		t.Fatalf("hand-published: %v", err)
	}
	if hand.PublishMode != PublishModeOwner || hand.OnFront || hand.VersionCount != 1 {
		t.Fatalf("hand-published = %+v, want mode owner, off the front, one version", hand)
	}

	if _, err := svc.OwnedPublication(ctx, "test-owner", "doc-never"); !errors.Is(err, ErrPublicationNotFound) {
		t.Fatalf("never published: err = %v, want ErrPublicationNotFound", err)
	}
	if _, err := svc.OwnedPublication(ctx, "other-owner", "doc-hand"); !errors.Is(err, ErrPublicationNotFound) {
		t.Fatalf("another owner's doc: err = %v, want ErrPublicationNotFound", err)
	}
}

func TestInternalOwnedPublicationRoute(t *testing.T) {
	store, root := openTestPlatformStore(t)
	svc := NewService(store, filepath.Join(root, "artifacts"), filepath.Join(root, "signing.key"))
	handler := NewHandler(svc)
	pub := publishAs(t, svc, "user-1", PublishModeAuto, "doc-1", 1, "Story", "Body.", nil)

	get := func(query string, internal bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/internal/platform/publications/owned"+query, nil)
		if internal {
			req.Header.Set("X-Internal-Caller", "true")
		}
		w := httptest.NewRecorder()
		handler.HandleInternalOwnedPublication(w, req)
		return w
	}

	if w := get("?owner_id=user-1&doc_id=doc-1", false); w.Code != http.StatusForbidden {
		t.Fatalf("external caller: got %d, want 403", w.Code)
	}
	for _, q := range []string{"?doc_id=doc-1", "?owner_id=user-1", ""} {
		if w := get(q, true); w.Code != http.StatusBadRequest {
			t.Fatalf("query %q: got %d, want 400", q, w.Code)
		}
	}
	if w := get("?owner_id=user-1&doc_id=doc-none", true); w.Code != http.StatusNotFound {
		t.Fatalf("unpublished doc: got %d, want 404", w.Code)
	}
	w := get("?owner_id=user-1&doc_id=doc-1", true)
	if w.Code != http.StatusOK {
		t.Fatalf("owned read: got %d body %s", w.Code, w.Body.String())
	}
	var got OwnedPublication
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.PublicationID != pub.PublicationID || got.DocID != "doc-1" || got.VersionCount != 1 || got.Route.Bare != pub.RoutePath {
		t.Fatalf("owned read = %+v", got)
	}
}
