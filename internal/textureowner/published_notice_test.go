package textureowner

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// A computer that publishes must hear what happens to what it published (the
// notices on its work, the op-eds on its stories). Publishing is done by the
// proxy, not by the computer, so the proxy tells the computer after a
// successful publish and the computer arms its polls.

type nullFeed struct{}

func (nullFeed) Opeds(context.Context, string, int) (agentcore.OpedFeedWire, error) {
	return agentcore.OpedFeedWire{}, nil
}
func (nullFeed) Notices(context.Context, string, int) (agentcore.NoticeFeedWire, error) {
	return agentcore.NoticeFeedWire{}, nil
}
func (nullFeed) ReadVersion(context.Context, string) (agentcore.PublicationVersionWire, error) {
	return agentcore.PublicationVersionWire{}, nil
}
func (nullFeed) PostOpedVerdict(context.Context, agentcore.OpedVerdictWire) error { return nil }

func postPublished(handler *Handler, docID, owner, method string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/api/texture/documents/"+docID+"/published", nil)
	if owner != "" {
		req.Header.Set("X-Authenticated-User", owner)
	}
	w := httptest.NewRecorder()
	handler.HandleTextureRouter(w, req)
	return w
}

func TestPublishedNoticeArmsThePublishersPolls(t *testing.T) {
	core, handler := testAPISetup(t)
	core.SetPlatformFeed(nullFeed{})
	start := startObservationLifecycle(t, core.Store())
	doc := start.InitialDocument.DocID
	rows := func() []store.PlatformPoll {
		polls, err := core.Store().ListPlatformPolls(objectgraph.WithCustodianReader(context.Background(), "choir.platform_poll"), start.OwnerID)
		if err != nil {
			t.Fatal(err)
		}
		return polls
	}
	if len(rows()) != 0 {
		t.Fatal("polls existed before any publish")
	}
	if w := postPublished(handler, doc, "", http.MethodPost); w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated = %d", w.Code)
	}
	if w := postPublished(handler, doc, start.OwnerID, http.MethodGet); w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET = %d", w.Code)
	}
	if w := postPublished(handler, doc, "someone-else", http.MethodPost); w.Code != http.StatusNotFound {
		t.Fatalf("another owner's document = %d, want 404", w.Code)
	}
	if len(rows()) != 0 {
		t.Fatal("a refused call armed polls")
	}
	for i := 0; i < 2; i++ { // idempotent
		if w := postPublished(handler, doc, start.OwnerID, http.MethodPost); w.Code != http.StatusNoContent {
			t.Fatalf("publish notice %d = %d %s", i, w.Code, w.Body)
		}
	}
	got := map[string]string{}
	for _, p := range rows() {
		got[p.Name] = p.State
	}
	if got[store.PlatformPollOpedIntake] != store.PlatformPollActive || got[store.PlatformPollNoticeIntake] != store.PlatformPollActive || len(got) != 2 {
		t.Fatalf("polls = %v, want both intakes active", got)
	}
	core.Stop()
}
