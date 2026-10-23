package platform

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
	"github.com/yusefmosiah/go-choir/internal/server"
)

// Tests for GET /internal/platform/artifact-gc/tape/extra-refs, the read-only
// instrument for the sweep's database-side keep set
// (docs/problems/tape-extra-refs-no-read-route-2026-10-22.md).

type tapeExtraRefsAnswer struct {
	Digests     []string  `json:"digests"`
	Count       int       `json:"count"`
	Queries     int       `json:"queries"`
	RowsScanned int       `json:"rows_scanned"`
	GeneratedAt time.Time `json:"generated_at"`
}

func tapeExtraRefsDigest(c byte) string { return platformTestDigest(c) }

// seedExtraRefs puts a digest in every one of the five columns, one digest
// twice (a duplicate), two glued digests in one value, and a 63-character run
// that is not a digest. It returns the expected sorted unique set.
func (f *tapeFixture) seedExtraRefs() []string {
	f.t.Helper()
	now := time.Now().UTC()
	a, b, c, d, e, g := tapeExtraRefsDigest('a'), tapeExtraRefsDigest('b'), tapeExtraRefsDigest('c'), tapeExtraRefsDigest('d'), tapeExtraRefsDigest('e'), tapeExtraRefsDigest('f')
	exec := func(q string, args ...any) {
		f.t.Helper()
		if _, err := f.store.db.Exec(q, args...); err != nil {
			f.t.Fatal(err)
		}
	}
	exec(`INSERT INTO computer_version_code_closures (code_ref, source_commit, closure_json, created_at) VALUES (?,?,?,?)`,
		"code-1", "abc", `{"artifacts":[{"uri":"x","sha256":"`+a+`"},{"uri":"y","sha256":"`+b+`"}]}`, now)
	exec(`INSERT INTO computer_version_artifact_programs (artifact_program_ref, program_json, created_at) VALUES (?,?,?)`,
		"prog-1", `{"entries":[{"content_sha256":"`+b+`"}]}`, now) // b again: duplicate
	exec(`INSERT INTO computer_checkpoints (computer_id,idempotency_key,request_commitment,checkpoint_digest,checkpoint_artifact_ref,checkpoint_json,receipt_json,receipt_digest,created_at) VALUES (?,?,?,?,?,?,?,?,?)`,
		"c-1", "k-1", a, c, "ref", `{"ref":"`+c+d+`"}`, `{"seen":"`+e+`","short":"`+strings.Repeat("9", 63)+`"}`, a, now)
	exec(`INSERT INTO computer_route_projection_certificates (computer_id,idempotency_key,request_commitment,certificate_digest,certificate_json,receipt_json,receipt_digest,expires_at,created_at) VALUES (?,?,?,?,?,?,?,?,?)`,
		"c-1", "k-2", a, a, `{"x":"`+g+`"}`, `{}`, a, now.Add(time.Hour), now)
	want := []string{a, b, c, d, e, g}
	sort.Strings(want)
	return want
}

func (f *tapeFixture) callExtraRefs(method, remote string, mutate func(*http.Request)) *httptest.ResponseRecorder {
	f.t.Helper()
	req := httptest.NewRequest(method, "/internal/platform/artifact-gc/tape/extra-refs", nil)
	req.RemoteAddr = remote
	if mutate != nil {
		mutate(req)
	}
	rec := httptest.NewRecorder()
	hostauth.Middleware(http.HandlerFunc(NewHandler(f.svc).HandleTapeExtraRefs)).ServeHTTP(rec, req)
	return rec
}

func internalMarker(r *http.Request) { r.Header.Set("X-Internal-Caller", "true") }

// The answer equals what the sweep keeps from the database: the digests
// tapeExtraRefs adds, extracted by the sweep's own regexp, de-duplicated and
// sorted; queries is the five the sweep runs.
func TestTapeExtraRefsRouteReturnsTheSweepsKeepSet(t *testing.T) {
	f := newTapeFixture(t)
	want := f.seedExtraRefs()

	// The sweep's own view of the same rows.
	sweepSet := map[string]struct{}{}
	rows := 0
	if err := f.svc.tapeExtraRefs(f.ctx, func(text string) {
		rows++
		for _, m := range hex64RE.FindAllString(text, -1) {
			sweepSet[m] = struct{}{}
		}
	}); err != nil {
		t.Fatal(err)
	}
	if len(sweepSet) != len(want) {
		t.Fatalf("fixture: sweep keeps %d digests, expected %d", len(sweepSet), len(want))
	}

	rec := f.callExtraRefs(http.MethodGet, "127.0.0.1:5555", internalMarker)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("content type = %q", ct)
	}
	var got tapeExtraRefsAnswer
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("answer is not JSON: %v: %s", err, rec.Body.String())
	}
	if strings.Join(got.Digests, ",") != strings.Join(want, ",") {
		t.Fatalf("digests = %v, want %v", got.Digests, want)
	}
	if got.Count != len(want) || got.Queries != len(tapeExtraRefQueries) || got.Queries != 5 {
		t.Fatalf("count/queries = %d/%d", got.Count, got.Queries)
	}
	if got.RowsScanned != rows || got.RowsScanned != 5 {
		t.Fatalf("rows_scanned = %d, sweep read %d (want 5)", got.RowsScanned, rows)
	}
	if got.GeneratedAt.IsZero() || time.Since(got.GeneratedAt) > time.Minute {
		t.Fatalf("generated_at = %v", got.GeneratedAt)
	}
	hex := regexp.MustCompile(`^[0-9a-f]{64}$`)
	for _, d := range got.Digests {
		if !hex.MatchString(d) {
			t.Fatalf("not a 64-hex digest: %q", d)
		}
	}
	// A read: no sweep job, nothing written to the tape or the database.
	if _, ok := f.svc.TapeSweepJobStatus(""); ok {
		t.Fatal("the read route started a sweep job")
	}
}

// An empty database is an empty list, not null: the operator's jq must see
// []; the repack tool treats an empty keep set as an error unless told.
func TestTapeExtraRefsRouteEmptyIsAnEmptyList(t *testing.T) {
	f := newTapeFixture(t)
	rec := f.callExtraRefs(http.MethodGet, "127.0.0.1:5555", internalMarker)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"digests":[]`) {
		t.Fatalf("empty answer = %d %s", rec.Code, rec.Body.String())
	}
}

// A failing query is an error status and no digests: a partial list must not
// look like a keep set.
func TestTapeExtraRefsRouteFailureReturnsNoPartialList(t *testing.T) {
	f := newTapeFixture(t)
	f.seedExtraRefs()
	if _, err := f.store.db.Exec(`DROP TABLE computer_route_projection_certificates`); err != nil {
		t.Fatal(err)
	}
	rec := f.callExtraRefs(http.MethodGet, "127.0.0.1:5555", internalMarker)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `"digests"`) || regexp.MustCompile(`[0-9a-f]{64}`).MatchString(rec.Body.String()) {
		t.Fatalf("error answer carries digests: %s", rec.Body.String())
	}
}

// Over the cap the route says so and sends nothing, rather than a truncated
// list.
func TestTapeExtraRefsRouteOverTheCapReturnsNoList(t *testing.T) {
	f := newTapeFixture(t)
	f.seedExtraRefs()
	f.svc.tapeSweep.extraRefsMax = 3
	rec := f.callExtraRefs(http.MethodGet, "127.0.0.1:5555", internalMarker)
	if rec.Code != http.StatusInsufficientStorage {
		t.Fatalf("status = %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `"digests"`) || regexp.MustCompile(`[0-9a-f]{64}`).MatchString(rec.Body.String()) {
		t.Fatalf("over-cap answer carries digests: %s", rec.Body.String())
	}
	f.svc.tapeSweep.extraRefsMax = 6
	if rec := f.callExtraRefs(http.MethodGet, "127.0.0.1:5555", internalMarker); rec.Code != http.StatusOK {
		t.Fatalf("at the cap = %d %s", rec.Code, rec.Body.String())
	}
}

// Same authority as the tape sweep route: loopback, the marker and the
// host-internal credential; a guest (a tap address, or no marker) is refused,
// and the route takes GET only.
func TestTapeExtraRefsRouteIsHostInternalOnly(t *testing.T) {
	f := newTapeFixture(t)
	f.seedExtraRefs()
	const token = "tape-extra-refs-token-0123456789abcdef0123456789"
	restore := hostauth.SetForTesting(hostauth.ModeEnforce, token)
	defer restore()
	withCredential := func(r *http.Request) { internalMarker(r); r.Header.Set(hostauth.Header, token) }

	cases := []struct {
		name   string
		method string
		remote string
		mut    func(*http.Request)
		want   int
	}{
		{"guest tap address with marker and credential", http.MethodGet, "10.200.7.7:4000", withCredential, http.StatusForbidden},
		{"loopback without the marker", http.MethodGet, "127.0.0.1:5555", func(r *http.Request) { r.Header.Set(hostauth.Header, token) }, http.StatusForbidden},
		{"loopback with the marker but no credential", http.MethodGet, "127.0.0.1:5555", internalMarker, http.StatusForbidden},
		{"loopback with a wrong credential", http.MethodGet, "127.0.0.1:5555", func(r *http.Request) { internalMarker(r); r.Header.Set(hostauth.Header, "wrong") }, http.StatusForbidden},
		{"POST is refused", http.MethodPost, "127.0.0.1:5555", withCredential, http.StatusMethodNotAllowed},
		{"host caller with the credential", http.MethodGet, "127.0.0.1:5555", withCredential, http.StatusOK},
	}
	for _, tc := range cases {
		rec := f.callExtraRefs(tc.method, tc.remote, tc.mut)
		if rec.Code != tc.want {
			t.Errorf("%s: status = %d, want %d: %s", tc.name, rec.Code, tc.want, rec.Body.String())
		}
		if tc.want != http.StatusOK && regexp.MustCompile(`[0-9a-f]{64}`).MatchString(rec.Body.String()) {
			t.Errorf("%s: a refused call saw digests: %s", tc.name, rec.Body.String())
		}
	}
}

// The route is reachable at its exact path through the real registration
// (server.NewServer + RegisterRoutes + the host-credential middleware), GET
// only. The other tests call the handler directly, so a typo in the
// registered pattern would 404 the operator and still pass them.
func TestTapeExtraRefsRouteIsRegisteredAtItsExactPath(t *testing.T) {
	f := newTapeFixture(t)
	want := f.seedExtraRefs()
	const token = "tape-extra-refs-token-0123456789abcdef0123456789"
	restore := hostauth.SetForTesting(hostauth.ModeEnforce, token)
	defer restore()
	srv := server.NewServer("corpusd-test", "0")
	RegisterRoutes(srv, NewHandler(f.svc))
	call := func(method, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		req.RemoteAddr = "127.0.0.1:5555"
		internalMarker(req)
		req.Header.Set(hostauth.Header, token)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		return rec
	}
	const path = "/internal/platform/artifact-gc/tape/extra-refs"
	rec := call(http.MethodGet, path)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d %s", path, rec.Code, rec.Body.String())
	}
	var got tapeExtraRefsAnswer
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("answer is not JSON: %v: %s", err, rec.Body.String())
	}
	if strings.Join(got.Digests, ",") != strings.Join(want, ",") {
		t.Fatalf("digests = %v, want %v", got.Digests, want)
	}
	if rec := call(http.MethodPost, path); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST %s = %d, want 405: %s", path, rec.Code, rec.Body.String())
	}
	// A neighbouring path must not reach the extra-refs handler.
	if rec := call(http.MethodGet, path+"x"); rec.Code == http.StatusOK {
		t.Fatalf("GET %sx reached a handler: %s", path, rec.Body.String())
	}
}
