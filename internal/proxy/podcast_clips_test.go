package proxy

// Podcast clips at the proxy (docs/design/podcast-clips-2026-11-15.md 4.1 and
// 4.3). Ways it can fail, written first:
//   - the public clips route asks for a sign-in, or forwards a query corpusd
//     did not get from the allowed parameters, or a stranger's thousand
//     routes reach corpusd;
//   - an operator write route answers an anonymous caller, or a signed-in
//     account that is not on the platform operator roster, or reaches
//     corpusd before the roster said yes;
//   - the episode listing forwards parameters other than need, since, limit;
//   - a method the route does not take reaches corpusd.

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/auth"
	"github.com/yusefmosiah/go-choir/internal/platform"
)

func TestPublicPodcastClipsRouteNeedsNoSignInAndForwardsOnlyItsQuery(t *testing.T) {
	var mu sync.Mutex
	var got []string
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		got = append(got, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery+" internal="+r.Header.Get("X-Internal-Caller"))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"clips":[]}`))
	}))
	defer corpusd.Close()
	autoputer := httptest.NewServer(http.NewServeMux())
	defer autoputer.Close()
	h, _ := opinionTestHandler(t, autoputer.URL, corpusd.URL)

	req := httptest.NewRequest(http.MethodGet, "https://choir.news/api/platform/podcast-clips?route=/a&route=/b&limit=7&secret=1", nil)
	w := httptest.NewRecorder()
	h.HandleAPI(w, req)
	if w.Code != http.StatusOK || w.Body.String() != `{"clips":[]}` {
		t.Fatalf("anonymous read = %d %s", w.Code, w.Body)
	}
	if cc := w.Header().Get("Cache-Control"); !strings.Contains(cc, "max-age=60") {
		t.Fatalf("Cache-Control = %q", cc)
	}
	mu.Lock()
	call := got[0]
	mu.Unlock()
	q := url.Values{"route": {"/a", "/b"}, "limit": {"7"}}.Encode()
	if call != "GET "+platform.PodcastClipsPath+"?"+q+" internal=true" {
		t.Fatalf("corpusd saw %q", call)
	}

	many := url.Values{}
	for i := 0; i < podcastPublicMaxRoutes+1; i++ {
		many.Add("route", fmt.Sprintf("/r%d", i))
	}
	w = httptest.NewRecorder()
	h.HandleAPI(w, httptest.NewRequest(http.MethodGet, "https://choir.news/api/platform/podcast-clips?"+many.Encode(), nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("too many routes = %d", w.Code)
	}
	w = httptest.NewRecorder()
	h.HandleAPI(w, httptest.NewRequest(http.MethodGet, "https://choir.news/api/platform/podcast-clips?limit=x", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("bad limit = %d", w.Code)
	}
	w = httptest.NewRecorder()
	h.HandleAPI(w, httptest.NewRequest(http.MethodPost, "https://choir.news/api/platform/podcast-clips", strings.NewReader("{}")))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST = %d", w.Code)
	}
	mu.Lock()
	n := len(got)
	mu.Unlock()
	if n != 1 {
		t.Fatalf("corpusd got %d calls, want 1", n)
	}
}

func TestOperatorPodcastRoutesPassTheRosterGate(t *testing.T) {
	var mu sync.Mutex
	var podcastCalls []string
	var bodies []string
	operator := false
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == platform.PlatformOperatorCheckRoute {
			_, _ = fmt.Fprintf(w, `{"operator":%v}`, operator)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		podcastCalls = append(podcastCalls, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)
		bodies = append(bodies, string(raw))
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer corpusd.Close()
	autoputer := httptest.NewServer(http.NewServeMux())
	defer autoputer.Close()
	h, _ := opinionTestHandler(t, autoputer.URL, corpusd.URL)
	h.SetAPIKeyValidator(ownerActsAPIKeys{})

	call := func(method, path, body string, key bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "https://choir.news"+path, strings.NewReader(body))
		if key {
			req.Header.Set("Authorization", "Bearer "+auth.APIKeyPrefix+"test-secret")
		}
		w := httptest.NewRecorder()
		h.HandleAPI(w, req)
		return w
	}
	count := func() int {
		mu.Lock()
		defer mu.Unlock()
		return len(podcastCalls)
	}
	routes := []struct{ method, path string }{
		{http.MethodPost, "/api/platform/operator/podcasts/episodes"},
		{http.MethodGet, "/api/platform/operator/podcasts/episodes"},
		{http.MethodPut, "/api/platform/operator/podcasts/transcripts"},
		{http.MethodPut, "/api/platform/operator/podcasts/clips"},
		{http.MethodPut, "/api/platform/operator/podcasts/feeds/listed"},
	}
	for _, rt := range routes {
		if w := call(rt.method, rt.path, `{}`, false); w.Code != http.StatusUnauthorized {
			t.Fatalf("anonymous %s %s = %d", rt.method, rt.path, w.Code)
		}
		if w := call(rt.method, rt.path, `{}`, true); w.Code != http.StatusForbidden {
			t.Fatalf("a key whose account is not an operator, %s %s = %d %s", rt.method, rt.path, w.Code, w.Body)
		}
	}
	if count() != 0 {
		t.Fatalf("corpusd's podcast routes were reached %d times before the roster said yes", count())
	}

	mu.Lock()
	operator = true
	mu.Unlock()
	if w := call(http.MethodPost, "/api/platform/operator/podcasts/episodes", `{"episodes":[]}`, true); w.Code != http.StatusOK {
		t.Fatalf("operator POST episodes = %d %s", w.Code, w.Body)
	}
	if w := call(http.MethodGet, "/api/platform/operator/podcasts/episodes?need=transcript&limit=5&since=2026-11-15T00:00:00Z&evil=1", "", true); w.Code != http.StatusOK {
		t.Fatalf("operator GET episodes = %d %s", w.Code, w.Body)
	}
	if w := call(http.MethodPut, "/api/platform/operator/podcasts/transcripts", `{"episode_id":"x"}`, true); w.Code != http.StatusOK {
		t.Fatalf("operator PUT transcripts = %d", w.Code)
	}
	if w := call(http.MethodPut, "/api/platform/operator/podcasts/clips", `{"clips":[]}`, true); w.Code != http.StatusOK {
		t.Fatalf("operator PUT clips = %d", w.Code)
	}
	if w := call(http.MethodPut, "/api/platform/operator/podcasts/feeds/listed", `{"feeds":[]}`, true); w.Code != http.StatusOK {
		t.Fatalf("operator PUT feeds/listed = %d", w.Code)
	}
	if w := call(http.MethodDelete, "/api/platform/operator/podcasts/clips", ``, true); w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("DELETE clips = %d", w.Code)
	}
	mu.Lock()
	defer mu.Unlock()
	want := []string{
		"POST " + platform.PodcastEpisodesPath + "?",
		"GET " + platform.PodcastEpisodesPath + "?" + url.Values{"need": {"transcript"}, "limit": {"5"}, "since": {"2026-11-15T00:00:00Z"}}.Encode(),
		"PUT " + platform.PodcastTranscriptsPath + "?",
		"PUT " + platform.PodcastClipsWritePath + "?",
		"PUT " + platform.PodcastFeedsListedPath + "?",
	}
	if strings.Join(podcastCalls, "\n") != strings.Join(want, "\n") {
		t.Fatalf("corpusd saw\n%s\nwant\n%s", strings.Join(podcastCalls, "\n"), strings.Join(want, "\n"))
	}
	if bodies[0] != `{"episodes":[]}` || bodies[2] != `{"episode_id":"x"}` {
		t.Fatalf("bodies = %q", bodies)
	}
}

// Subscriptions at the proxy: a signed-in report passes a salted hash, never
// the account; an anonymous one passes none; the transcript needs a sign-in;
// the popular list is public.
func TestPodcastSubscriptionRoutes(t *testing.T) {
	var mu sync.Mutex
	var seen []string
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		seen = append(seen, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery+" reporter="+r.Header.Get(platform.PodcastReporterHeader)+" body="+string(body))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer corpusd.Close()
	autoputer := httptest.NewServer(http.NewServeMux())
	defer autoputer.Close()
	h, _ := opinionTestHandler(t, autoputer.URL, corpusd.URL)
	h.SetAPIKeyValidator(ownerActsAPIKeys{})
	call := func(method, path, body string, key bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "https://choir.news"+path, strings.NewReader(body))
		if key {
			req.Header.Set("Authorization", "Bearer "+auth.APIKeyPrefix+"test-secret")
		}
		w := httptest.NewRecorder()
		h.HandleAPI(w, req)
		return w
	}
	last := func() string {
		mu.Lock()
		defer mu.Unlock()
		return seen[len(seen)-1]
	}
	feeds := `{"feeds":[{"feed_url":"https://a.example.test/f.xml","title":"A"}]}`
	n := len(seen)
	if w := call(http.MethodPost, "/api/platform/podcast-feeds", feeds, false); w.Code != http.StatusUnauthorized || len(seen) != n {
		t.Fatalf("an anonymous report must be refused before corpusd: %d (%d calls)", w.Code, len(seen)-n)
	}
	badCookie := httptest.NewRequest(http.MethodPost, "https://choir.news/api/platform/podcast-feeds", strings.NewReader(feeds))
	badCookie.AddCookie(&http.Cookie{Name: "choir_access", Value: "not-a-token"})
	wBad := httptest.NewRecorder()
	h.HandleAPI(wBad, badCookie)
	if wBad.Code != http.StatusUnauthorized || len(seen) != n {
		t.Fatalf("a presented, invalid credential is a 401, not an anonymous report: %d", wBad.Code)
	}
	if w := call(http.MethodPost, "/api/platform/podcast-feeds", feeds, true); w.Code != http.StatusOK {
		t.Fatalf("signed-in report: %d %s", w.Code, w.Body)
	}
	got := last()
	if !strings.Contains(got, "reporter=sub-") || strings.Contains(got, "user-real") {
		t.Fatalf("the reporter is not a hash: %s", got)
	}
	if w := call(http.MethodGet, "/api/platform/podcast-feeds", "", false); w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET feeds = %d", w.Code)
	}
	big := `{"feeds":[],"x":"` + strings.Repeat("a", 70<<10) + `"}`
	before := len(seen)
	if w := call(http.MethodPost, "/api/platform/podcast-feeds", big, false); w.Code == http.StatusOK || len(seen) != before {
		t.Fatalf("an oversized report reached corpusd: %d", w.Code)
	}
	// Popular is public; the transcript is not.
	if w := call(http.MethodGet, "/api/platform/podcast-popular?limit=5", "", false); w.Code != http.StatusOK || !strings.Contains(last(), "/podcast-popular?limit=5") {
		t.Fatalf("popular: %d %s", w.Code, last())
	}
	n = len(seen)
	if w := call(http.MethodGet, "/api/platform/podcast-transcript?feed=https://a.example.test/f.xml&guid=g", "", false); w.Code != http.StatusUnauthorized || len(seen) != n {
		t.Fatalf("anonymous transcript = %d", w.Code)
	}
	if w := call(http.MethodGet, "/api/platform/podcast-transcript?feed=https://a.example.test/f.xml&guid=g", "", true); w.Code != http.StatusOK || !strings.Contains(last(), "/podcast-transcript?feed=https%3A%2F%2Fa.example.test%2Ff.xml&guid=g") || !strings.Contains(last(), "reporter=sub-") {
		t.Fatalf("signed-in transcript = %d %s", w.Code, last())
	}
	if w := call(http.MethodGet, "/api/platform/podcast-transcript?feed=x", "", true); w.Code != http.StatusBadRequest {
		t.Fatalf("transcript without a guid = %d", w.Code)
	}
	// The clips route forwards feed filters.
	if w := call(http.MethodGet, "/api/platform/podcast-clips?feed=https://a.example.test/f.xml", "", false); w.Code != http.StatusOK || !strings.Contains(last(), "feed=https%3A%2F%2Fa.example.test%2Ff.xml") {
		t.Fatalf("clips feed filter: %d %s", w.Code, last())
	}
}
