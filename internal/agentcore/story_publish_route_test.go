package agentcore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Where a computer sends its story versions and verdicts (B12). A guest's
// platform URL (choir.platform_url, CorpusdURL) is corpusd itself, and
// corpusd has no story publish route: publishing reads the document and
// revision back from the computer, which only the proxy does. Ways it can
// fail, written before the fix:
//   - a version posts to the platform URL and meets corpusd's 404, so every
//     story revision is a gap once auto-publish fires;
//   - verdicts post to the proxy only, so a guest booted without a proxy
//     URL never reaches the overlay;
//   - a computer without a proxy URL publishes nothing and says nothing.

func fakeHostService(t *testing.T, routes ...string) (*httptest.Server, chan string) {
	t.Helper()
	hits := make(chan string, 8)
	mux := http.NewServeMux()
	for _, route := range routes {
		route := route
		mux.HandleFunc(route, func(w http.ResponseWriter, r *http.Request) {
			hits <- route
			w.Header().Set("Content-Type", "application/json")
			if route == types.StoryPublishPath {
				var req types.StoryPublishRequest
				_ = json.NewDecoder(r.Body).Decode(&req)
				_ = json.NewEncoder(w).Encode(map[string]any{"revision": 2, "permalink_path": "/p/" + req.DocID + "/v2"})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"recorded": 1})
		})
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, hits
}

func TestStoryVersionsGoToTheProxyAndVerdictsToTheComputersPlatformURL(t *testing.T) {
	rt, _ := testRuntime(t)
	// corpusd serves the verdict and capture routes, never the publish one.
	corpusd, corpusdHits := fakeHostService(t, types.StoryVerdictsPath, "/internal/platform/captures")
	proxy, proxyHits := fakeHostService(t, types.StoryPublishPath, types.StoryVerdictsPath)
	rt.cfg.CorpusdURL, rt.cfg.ProxyURL = corpusd.URL, proxy.URL

	publisher := rt.storyPublisher()
	if publisher == nil {
		t.Fatal("no publisher with a platform and a proxy URL")
	}
	ctx := context.Background()
	if err := publisher.PublishVersion(ctx, StoryVersion{OwnerID: "o", ComputerID: "c", DocID: "doc-1", RevisionID: "rev-2", Revision: 2}); err != nil {
		t.Fatalf("publish version: %v", err)
	}
	if got := <-proxyHits; got != types.StoryPublishPath {
		t.Fatalf("version reached the proxy at %s", got)
	}
	if err := publisher.PublishVerdicts(ctx, types.StoryVerdictsRequest{OwnerID: "o", ComputerID: "c", Verdicts: []types.StoryVerdict{{Key: "k", Verdict: types.SupportSupported, Judge: "management"}}}); err != nil {
		t.Fatalf("publish verdicts: %v", err)
	}
	if got := <-corpusdHits; got != types.StoryVerdictsPath {
		t.Fatalf("verdicts reached corpusd at %s", got)
	}
	select {
	case got := <-proxyHits: // hang-guard: non-blocking
		t.Fatalf("verdicts also went to the proxy at %s", got)
	default:
	}
}

func TestStoryVersionWithoutAProxyURLIsAGapThatSaysSo(t *testing.T) {
	rt, _ := testRuntime(t)
	corpusd, _ := fakeHostService(t, types.StoryVerdictsPath)
	rt.cfg.CorpusdURL, rt.cfg.ProxyURL = corpusd.URL, ""
	publisher := rt.storyPublisher()
	if publisher == nil {
		t.Fatal("verdicts need a publisher even without a proxy URL")
	}
	err := publisher.PublishVersion(context.Background(), StoryVersion{DocID: "doc-1", RevisionID: "rev-2", Revision: 2})
	if err == nil {
		t.Fatal("a version published with no proxy URL to publish through")
	}
	if err := publisher.PublishVerdicts(context.Background(), types.StoryVerdictsRequest{OwnerID: "o", ComputerID: "c"}); err != nil {
		t.Fatalf("verdicts without a proxy URL: %v", err)
	}
}
