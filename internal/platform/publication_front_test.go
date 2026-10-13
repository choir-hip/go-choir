package platform

// Ways the Autopaper front list can fail (Cut 2 unit 2b, Reader W3a):
//
//  1. A story with several versions is listed once per version instead of
//     once, or its version count is wrong.
//  2. The order is the publications' creation order, not newest update
//     first: a story revised after another was opened must sort above it.
//  3. Paging repeats or drops a story across pages.
//  4. A hand-published Texture (publish_mode other than auto) is listed as
//     an Autopaper story.
//  5. The dek is the title, a heading or a citation marker instead of the
//     story's first paragraph.
//  6. Building the list calls a computer (a guest lookup or any outbound
//     HTTP), so the front page goes down with the paper computer.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

type frontFixture struct {
	svc   *Service
	store *Store
	// stories by doc id: their bare routes.
	routes map[string]string
}

func publishStoryRevision(t *testing.T, svc *Service, doc string, n int, title, content string, bodyDoc json.RawMessage) *PublishTextureResponse {
	t.Helper()
	resp, err := svc.PublishTexture(context.Background(), PublishTextureRequest{
		OwnerID:          "paper-owner",
		SourceDocID:      doc,
		SourceRevisionID: fmt.Sprintf("%s-rev-%d", doc, n),
		RevisionNumber:   n,
		Title:            title,
		Content:          content,
		BodyDoc:          bodyDoc,
		RequestedBy:      "paper-owner",
	})
	if err != nil {
		t.Fatalf("publish %s v%d: %v", doc, n, err)
	}
	return resp
}

// setPublicationUpdatedAt spaces the fixture's publications a second apart.
// og_objects.updated_at is a DATETIME (seconds), so publishes inside one
// test second tie; the fixture sets the times instead of sleeping.
func setPublicationUpdatedAt(t *testing.T, store *Store, ownerID, doc string, at time.Time) {
	t.Helper()
	ident, err := publicationIdentityFor(ownerID, doc)
	if err != nil {
		t.Fatalf("identity: %v", err)
	}
	if _, err := store.corpus().Exec(`UPDATE og_objects SET updated_at = ? WHERE canonical_id = ?`, at.UTC(), ident.CanonicalID); err != nil {
		t.Fatalf("set updated_at: %v", err)
	}
}

// newFrontFixture publishes three stories: harbor (three versions, last
// revised), bridge and canal (one version each, canal opened after bridge).
func newFrontFixture(t *testing.T) frontFixture {
	t.Helper()
	store, root := openTestPlatformStore(t)
	svc := NewService(store, filepath.Join(root, "artifacts"), filepath.Join(root, "signing.key"))
	routes := map[string]string{}

	harborDoc := json.RawMessage(`{"schema":"choir.texture_doc.v1","doc":{"type":"doc","attrs":{"id":"doc-harbor"},"content":[
		{"type":"heading","attrs":{"id":"h1","level":1},"content":[{"type":"text","text":"Harbor reopens"}]},
		{"type":"paragraph","attrs":{"id":"p1"},"content":[{"type":"text","text":"The harbor reopened at dawn after a week of storms."}]},
		{"type":"paragraph","attrs":{"id":"p2"},"content":[{"type":"text","text":"Ferries resume Monday."}]}]}}`)

	routes["doc-harbor"] = publishStoryRevision(t, svc, "doc-harbor", 1, "Harbor reopens", "The harbor may reopen.", nil).RoutePath
	routes["doc-bridge"] = publishStoryRevision(t, svc, "doc-bridge", 1, "Bridge closes", "# Bridge closes\n\nThe old bridge closed for repairs.\n\nDetours run north.", nil).RoutePath
	routes["doc-canal"] = publishStoryRevision(t, svc, "doc-canal", 1, "Canal dredged", "Dredging began on the canal.", nil).RoutePath
	publishStoryRevision(t, svc, "doc-harbor", 2, "Harbor reopens", "The harbor reopens at dawn.", nil)
	publishStoryRevision(t, svc, "doc-harbor", 3, "Harbor reopens", "", harborDoc)

	base := time.Now().UTC().Truncate(time.Second).Add(-time.Hour)
	setPublicationUpdatedAt(t, store, "paper-owner", "doc-bridge", base)
	setPublicationUpdatedAt(t, store, "paper-owner", "doc-canal", base.Add(time.Second))
	setPublicationUpdatedAt(t, store, "paper-owner", "doc-harbor", base.Add(2*time.Second))
	return frontFixture{svc: svc, store: store, routes: routes}
}

func storyIDs(page *FrontPage) []string {
	out := make([]string, 0, len(page.Stories))
	for _, s := range page.Stories {
		out = append(out, s.Title)
	}
	return out
}

// Failure modes 1, 2 and 5.
func TestFrontListsEachStoryOnceNewestFirst(t *testing.T) {
	f := newFrontFixture(t)
	page, err := f.svc.ListFrontStories(context.Background(), 0, "")
	if err != nil {
		t.Fatalf("front: %v", err)
	}
	got := storyIDs(page)
	want := []string{"Harbor reopens", "Canal dredged", "Bridge closes"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("front order = %v, want %v", got, want)
	}
	if page.NextCursor != "" {
		t.Fatalf("one page holds every story, next cursor = %q", page.NextCursor)
	}
	harbor := page.Stories[0]
	if harbor.VersionCount != 3 || harbor.LatestRevision != 3 {
		t.Fatalf("harbor versions = %d latest = %d, want 3 and 3", harbor.VersionCount, harbor.LatestRevision)
	}
	if harbor.Route.Bare != f.routes["doc-harbor"] || harbor.Route.Permalink != f.routes["doc-harbor"]+"/v3" {
		t.Fatalf("harbor route = %+v, bare %q", harbor.Route, f.routes["doc-harbor"])
	}
	if harbor.Dek != "The harbor reopened at dawn after a week of storms." {
		t.Fatalf("harbor dek = %q, want its first paragraph", harbor.Dek)
	}
	if harbor.UpdatedAt.IsZero() || harbor.PublicationID == "" {
		t.Fatalf("harbor row = %+v", harbor)
	}
	bridge := page.Stories[2]
	if bridge.VersionCount != 1 || bridge.Route.Permalink != f.routes["doc-bridge"]+"/v1" {
		t.Fatalf("bridge row = %+v", bridge)
	}
	if bridge.Dek != "The old bridge closed for repairs." {
		t.Fatalf("bridge dek = %q, want its first paragraph after the heading", bridge.Dek)
	}

	// The latest version a row names is the one the bare link serves.
	bundle := readBundle(t, f.svc, harbor.Route.Bare)
	if bundle.Nav.Latest != harbor.LatestRevision || bundle.Payload.Revision != harbor.LatestRevision {
		t.Fatalf("bare link serves v%d (latest %d), row says v%d", bundle.Payload.Revision, bundle.Nav.Latest, harbor.LatestRevision)
	}
}

// Failure mode 3.
func TestFrontPagesWithoutRepeatsOrGaps(t *testing.T) {
	f := newFrontFixture(t)
	var titles []string
	cursor := ""
	for i := 0; i < 5; i++ {
		page, err := f.svc.ListFrontStories(context.Background(), 1, cursor)
		if err != nil {
			t.Fatalf("page %d: %v", i, err)
		}
		if len(page.Stories) != 1 {
			t.Fatalf("page %d has %d stories, want 1", i, len(page.Stories))
		}
		titles = append(titles, page.Stories[0].Title)
		cursor = page.NextCursor
		if cursor == "" {
			break
		}
	}
	want := []string{"Harbor reopens", "Canal dredged", "Bridge closes"}
	if fmt.Sprint(titles) != fmt.Sprint(want) {
		t.Fatalf("paged titles = %v, want %v", titles, want)
	}
	if _, err := f.svc.ListFrontStories(context.Background(), 1, "not-a-cursor"); err != ErrInvalidFrontCursor {
		t.Fatalf("bad cursor err = %v, want ErrInvalidFrontCursor", err)
	}
}

// Failure mode 4: the story filter seam reads publish_mode when present.
func TestFrontSkipsHandPublishedTextures(t *testing.T) {
	f := newFrontFixture(t)
	publishStoryRevision(t, f.svc, "doc-essay", 1, "An essay", "Hand published.", nil)
	ident, err := publicationIdentityFor("paper-owner", "doc-essay")
	if err != nil {
		t.Fatalf("identity: %v", err)
	}
	og := f.svc.ogStore()
	obj, err := og.GetObject(context.Background(), ident.CanonicalID)
	if err != nil {
		t.Fatalf("get publication: %v", err)
	}
	var meta map[string]any
	if err := json.Unmarshal(obj.Metadata, &meta); err != nil {
		t.Fatalf("decode: %v", err)
	}
	meta["publish_mode"] = "manual"
	obj.Metadata, _ = json.Marshal(meta)
	if err := og.PutObject(context.Background(), obj); err != nil {
		t.Fatalf("put publication: %v", err)
	}
	page, err := f.svc.ListFrontStories(context.Background(), 0, "")
	if err != nil {
		t.Fatalf("front: %v", err)
	}
	for _, s := range page.Stories {
		if s.Title == "An essay" {
			t.Fatalf("hand-published Texture listed: %+v", s)
		}
	}
	if len(page.Stories) != 3 {
		t.Fatalf("front lists %d stories, want 3", len(page.Stories))
	}
}

type failingGuestLookup struct{ t *testing.T }

func (g failingGuestLookup) LookupGuestContext(context.Context, string) (*vmctl.GuestOwnership, error) {
	g.t.Fatalf("front list looked up a guest computer")
	return nil, nil
}

type failingTransport struct{ t *testing.T }

func (f failingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	f.t.Fatalf("front list made an outbound request to %s", r.URL)
	return nil, fmt.Errorf("no outbound requests")
}

// Failure mode 6: the corpusd route answers from Store B with no guest
// lookup and no outbound HTTP.
func TestFrontRouteNeverCallsAComputer(t *testing.T) {
	f := newFrontFixture(t)
	handler := NewHandler(f.svc)
	if err := handler.ConfigureGuestBinding(failingGuestLookup{t: t}); err != nil {
		t.Fatalf("configure guest binding: %v", err)
	}
	saved := http.DefaultTransport
	http.DefaultTransport = failingTransport{t: t}
	defer func() { http.DefaultTransport = saved }()

	denied := httptest.NewRecorder()
	handler.HandleInternalFrontStories(denied, httptest.NewRequest(http.MethodGet, "/internal/platform/publications/front", nil))
	if denied.Code != http.StatusForbidden {
		t.Fatalf("missing internal caller: got %d", denied.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/internal/platform/publications/front?limit=2", nil)
	req.Header.Set("X-Internal-Caller", "true")
	w := httptest.NewRecorder()
	handler.HandleInternalFrontStories(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("front status %d body %s", w.Code, w.Body.String())
	}
	var page FrontPage
	if err := json.NewDecoder(w.Body).Decode(&page); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(page.Stories) != 2 || page.NextCursor == "" || page.Stories[0].VersionCount != 3 {
		t.Fatalf("front page = %+v", page)
	}

	bad := httptest.NewRequest(http.MethodGet, "/internal/platform/publications/front?cursor=%21", nil)
	bad.Header.Set("X-Internal-Caller", "true")
	badW := httptest.NewRecorder()
	handler.HandleInternalFrontStories(badW, bad)
	if badW.Code != http.StatusBadRequest {
		t.Fatalf("bad cursor status %d", badW.Code)
	}
}
