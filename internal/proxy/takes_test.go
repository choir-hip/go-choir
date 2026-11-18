package proxy

// Takes at the proxy (docs/reference/AUTORADIO-TAKES.md). Ways it can fail,
// written first:
//   1. an anonymous caller reads, publishes, deletes or reports;
//   2. the publisher or listener key comes from the request body instead of
//      the credential, so one account acts as another;
//   3. an outcome is accepted from a browser session (rule 1: it is the
//      listener's computer's act), or from a key that names no computer;
//   4. a drill take or a drill list is asked for by a caller that is not a
//      drill computer;
//   5. the multipart carries a field the route does not know, or audio over
//      the bound, and still reaches corpusd;
//   6. the audio route answers a body that is not audio/mp4;
//   7. an operator route answers a caller that is not the operator.

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/auth"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

const takesDrillComputer = "computer-59c902c5d46caecc0a180a8557164e72"

// takesKeys is an API-key store with one key bound to a computer.
type takesKeys struct{ computer string }

func (k takesKeys) GetAPIKeyByHash(context.Context, string) (*auth.APIKey, error) {
	return &auth.APIKey{ID: "key-t", UserID: "user-key", Scopes: []string{"admin"}, ComputerID: k.computer}, nil
}
func (takesKeys) TouchAPIKeyLastUsed(context.Context, string) error { return nil }
func (takesKeys) GetUserByID(id string) (*auth.User, error) {
	if id != "user-key" {
		return nil, sql.ErrNoRows
	}
	return &auth.User{ID: id, Email: "k@example.com"}, nil
}

type takesCall struct {
	method, path, query, ctype string
	body                       []byte
}

func takesHarness(t *testing.T, audioReply func(w http.ResponseWriter)) (func(string) func(*http.Request), func(*http.Request, ...func(*http.Request)) *httptest.ResponseRecorder, func() []takesCall) {
	t.Helper()
	var mu sync.Mutex
	var calls []takesCall
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		mu.Lock()
		calls = append(calls, takesCall{r.Method, r.URL.Path, r.URL.RawQuery, r.Header.Get("Content-Type"), raw})
		mu.Unlock()
		if r.URL.Path == platform.TakesAudioPath && audioReply != nil {
			audioReply(w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(corpusd.Close)
	autoputer := httptest.NewServer(http.NewServeMux())
	t.Cleanup(autoputer.Close)
	vm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/internal/vmctl/touch" {
			_, _ = w.Write([]byte(`{"touched":true}`))
			return
		}
		c := r.URL.Query().Get("computer_id")
		_, _ = w.Write([]byte(`{"computer_id":"` + c + `","user_id":"user-key","desktop_id":"primary","kind":"interactive","state":"active","epoch":1}`))
	}))
	t.Cleanup(vm.Close)
	h, priv := opinionTestHandler(t, autoputer.URL, corpusd.URL)
	h.vmctlClient = vmctl.NewClient(vm.URL)
	h.SetAPIKeyValidator(takesKeys{computer: takesDrillComputer})
	send := func(req *http.Request, mutate ...func(*http.Request)) *httptest.ResponseRecorder {
		for _, m := range mutate {
			m(req)
		}
		w := httptest.NewRecorder()
		h.HandleAPI(w, req)
		return w
	}
	cookie := func(user string) func(*http.Request) {
		return func(r *http.Request) {
			r.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWTWithEmail(priv, user, "reader@example.com")})
		}
	}
	return cookie, send, func() []takesCall { mu.Lock(); defer mu.Unlock(); return append([]takesCall(nil), calls...) }
}

func takesKey(r *http.Request) { r.Header.Set("Authorization", "Bearer "+auth.APIKeyPrefix+"secret") }

func takePost(t *testing.T, audio []byte, fields map[string]string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("audio", "take.m4a")
	_, _ = fw.Write(audio)
	for k, v := range fields {
		_ = mw.WriteField(k, v)
	}
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "https://choir.news/api/platform/takes", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func TestTakesRoutesNeedSignIn(t *testing.T) {
	_, send, calls := takesHarness(t, nil)
	for _, c := range []struct{ method, path string }{
		{http.MethodPost, "/api/platform/takes"}, {http.MethodGet, "/api/platform/takes"}, {http.MethodGet, "/api/platform/takes?mine=1"},
		{http.MethodGet, "/api/platform/takes/record?publisher=voice-abcdef"},
		{http.MethodGet, "/api/platform/takes/take-0123456789abcdef/audio"}, {http.MethodDelete, "/api/platform/takes/take-0123456789abcdef"},
		{http.MethodPost, "/api/platform/takes/take-0123456789abcdef/outcome"}, {http.MethodPost, "/api/platform/operator/takes/yank"},
	} {
		w := send(httptest.NewRequest(c.method, "https://choir.news"+c.path, strings.NewReader("{}")))
		if w.Code != http.StatusUnauthorized {
			t.Errorf("anonymous %s %s = %d, want 401", c.method, c.path, w.Code)
		}
	}
	if got := calls(); len(got) != 0 {
		t.Fatalf("corpusd was reached by anonymous callers: %+v", got)
	}
}

func TestTakePublishNamesPublisherFromCredentialOnly(t *testing.T) {
	cookie, send, calls := takesHarness(t, nil)
	audio := []byte("fake-m4a-bytes")
	// A field that names a publisher is refused whole.
	w := send(takePost(t, audio, map[string]string{"transcript": "hi", "duration_seconds": "10", "publisher": "voice-aaaaaa"}), cookie("user-1"))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("publisher field = %d %s", w.Code, w.Body)
	}
	if w = send(takePost(t, audio, map[string]string{"transcript": "hi", "duration_seconds": "10", "sha256": strings.Repeat("0", 64)}), cookie("user-1")); w.Code != http.StatusBadRequest {
		t.Fatalf("a wrong sha256 = %d", w.Code)
	}
	if w = send(takePost(t, bytes.Repeat([]byte("a"), platform.TakeMaxBytes+1), map[string]string{"transcript": "hi", "duration_seconds": "10"}), cookie("user-1")); w.Code != http.StatusBadRequest {
		t.Fatalf("over-size audio = %d", w.Code)
	}
	if got := calls(); len(got) != 0 {
		t.Fatalf("a refused publish reached corpusd: %+v", got)
	}
	w = send(takePost(t, audio, map[string]string{"transcript": "hi there", "duration_seconds": "10", "responds_to": `{"route":"/pub/texture/a","title":"A"}`}), cookie("user-1"))
	if w.Code != http.StatusOK {
		t.Fatalf("publish = %d %s", w.Code, w.Body)
	}
	got := calls()
	if len(got) != 1 || got[0].path != platform.TakesPath || got[0].method != http.MethodPost {
		t.Fatalf("corpusd calls = %+v", got)
	}
	var in platform.TakeIn
	if err := json.Unmarshal(got[0].body, &in); err != nil {
		t.Fatal(err)
	}
	if in.PublisherKey != types.TakePublisherKey("user-1") || in.Publisher != types.TakePublisherHandle("user-1") || in.Drill || in.Transcript != "hi there" || in.RespondsTo == nil || in.RespondsTo.Title != "A" {
		t.Fatalf("corpusd saw %+v", in)
	}
	if strings.Contains(string(got[0].body), "user-1") || strings.Contains(string(got[0].body), "reader@example.com") {
		t.Fatal("the user id or the email reached corpusd's take body")
	}
	// A key bound to the drill computer publishes drill data.
	w = send(takePost(t, audio, map[string]string{"transcript": "proof", "duration_seconds": "10"}), takesKey)
	if w.Code != http.StatusOK {
		t.Fatalf("drill publish = %d %s", w.Code, w.Body)
	}
	_ = json.Unmarshal(calls()[1].body, &in)
	if !in.Drill {
		t.Fatal("a take published by the drill computer's key is not drill data")
	}
}

func TestTakeListMineRecordDeleteUseTheCallerNotTheQuery(t *testing.T) {
	cookie, send, calls := takesHarness(t, nil)
	get := func(path string, m ...func(*http.Request)) *httptest.ResponseRecorder {
		return send(httptest.NewRequest(http.MethodGet, "https://choir.news"+path, nil), m...)
	}
	if w := get("/api/platform/takes?limit=x", cookie("user-1")); w.Code != http.StatusBadRequest {
		t.Fatalf("bad limit = %d", w.Code)
	}
	get("/api/platform/takes?limit=7&publisher_key=evil&include_drill=1", cookie("user-1"))
	get("/api/platform/takes?mine=1&publisher_key=evil", cookie("user-1"))
	get("/api/platform/takes?limit=5", takesKey)
	get("/api/platform/takes/record?publisher=voice-abcdef&as=evil", cookie("user-1"))
	if w := get("/api/platform/takes/record?publisher=bob@example.com", cookie("user-1")); w.Code != http.StatusBadRequest {
		t.Fatalf("a non-handle publisher = %d", w.Code)
	}
	send(httptest.NewRequest(http.MethodDelete, "https://choir.news/api/platform/takes/take-0123456789abcdef", nil), cookie("user-1"))
	if w := send(httptest.NewRequest(http.MethodDelete, "https://choir.news/api/platform/takes/not-a-take", nil), cookie("user-1")); w.Code != http.StatusNotFound {
		t.Fatalf("a malformed id = %d", w.Code)
	}
	got := calls()
	if len(got) != 5 {
		t.Fatalf("%d corpusd calls, want 5: %+v", len(got), got)
	}
	if got[0].query != "limit=7" {
		t.Errorf("list query = %q: a caller's include_drill or publisher_key was forwarded", got[0].query)
	}
	q, _ := url.ParseQuery(got[1].query)
	if q.Get("publisher_key") != types.TakePublisherKey("user-1") || len(q) != 1 {
		t.Errorf("mine query = %q", got[1].query)
	}
	if q, _ = url.ParseQuery(got[2].query); q.Get("include_drill") != "1" || q.Get("limit") != "5" {
		t.Errorf("a drill computer's list query = %q", got[2].query)
	}
	if q, _ = url.ParseQuery(got[3].query); q.Get("as") != types.TakePublisherKey("user-1") || q.Get("publisher") != "voice-abcdef" {
		t.Errorf("record query = %q", got[3].query)
	}
	var del map[string]string
	_ = json.Unmarshal(got[4].body, &del)
	if got[4].path != platform.TakesDeletePath || del["take_id"] != "take-0123456789abcdef" || del["publisher_key"] != types.TakePublisherKey("user-1") {
		t.Errorf("delete = %+v %s", got[4], got[4].body)
	}
}

func TestTakeOutcomeIsAComputersAct(t *testing.T) {
	cookie, send, calls := takesHarness(t, nil)
	post := func(m ...func(*http.Request)) *httptest.ResponseRecorder {
		return send(httptest.NewRequest(http.MethodPost, "https://choir.news/api/platform/takes/take-0123456789abcdef/outcome",
			strings.NewReader(`{"outcome":"heard","claim_id":"claim-aaaaaaaaaaaa"}`)), m...)
	}
	if w := post(cookie("user-1")); w.Code != http.StatusForbidden {
		t.Fatalf("a browser session reported an outcome: %d", w.Code)
	}
	if got := calls(); len(got) != 0 {
		t.Fatalf("corpusd reached: %+v", got)
	}
	if w := post(takesKey); w.Code != http.StatusOK {
		t.Fatalf("a computer's key = %d %s", w.Code, w.Body)
	}
	var in platform.TakeOutcomeIn
	_ = json.Unmarshal(calls()[0].body, &in)
	if in.ListenerComputer != takesDrillComputer || in.ListenerUserKey != types.TakePublisherKey("user-key") || in.Outcome != "heard" || in.TakeID != "take-0123456789abcdef" {
		t.Fatalf("corpusd saw %+v", in)
	}
	if w := send(httptest.NewRequest(http.MethodPost, "https://choir.news/api/platform/takes/take-0123456789abcdef/outcome",
		strings.NewReader(`{"outcome":"heard","claim_id":"c","listener_computer_id":"x"}`)), takesKey); w.Code != http.StatusBadRequest {
		t.Fatalf("a body naming the listener = %d", w.Code)
	}
}

func TestTakeAudioRelaysOnlyAudio(t *testing.T) {
	reply := "audio"
	cookie, send, _ := takesHarness(t, func(w http.ResponseWriter) {
		if reply == "audio" {
			w.Header().Set("Content-Type", "audio/mp4")
			_, _ = w.Write([]byte("bytes"))
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<script>"))
	})
	get := func() *httptest.ResponseRecorder {
		return send(httptest.NewRequest(http.MethodGet, "https://choir.news/api/platform/takes/take-0123456789abcdef/audio", nil), cookie("user-1"))
	}
	if w := get(); w.Code != http.StatusOK || w.Header().Get("Content-Type") != "audio/mp4" || w.Body.String() != "bytes" || !strings.Contains(w.Header().Get("Cache-Control"), "private") {
		t.Fatalf("audio = %d %q %q", w.Code, w.Header().Get("Content-Type"), w.Body)
	}
	reply = "html"
	if w := get(); w.Code != http.StatusBadGateway {
		t.Fatalf("a non-audio body = %d, want 502", w.Code)
	}
}

func TestTakeYankIsTheOperatorsAct(t *testing.T) {
	cookie, send, calls := takesHarness(t, nil)
	w := send(httptest.NewRequest(http.MethodPost, "https://choir.news/api/platform/operator/takes/yank", strings.NewReader(`{"take_id":"take-0123456789abcdef"}`)), cookie("user-1"))
	if w.Code != http.StatusForbidden {
		t.Fatalf("a non-passkey session yanked: %d", w.Code)
	}
	for _, c := range calls() {
		if c.path == platform.TakesYankPath {
			t.Fatalf("the yank reached corpusd: %+v", c)
		}
	}
}
