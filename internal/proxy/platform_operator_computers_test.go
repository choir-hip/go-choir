package proxy

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/yusefmosiah/go-choir/internal/auth"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// The platform operator's acts on platform computers, through the HTTP
// handlers, with the real corpusd roster handler behind the check and a fake
// vmctl and guest. Failure list (docs/design/platform-operator-account-2026-11-06.md,
// "Failure list (build)"); each test names the items it covers.

const (
	opAccount      = "1111aaaa-2222-3333-4444-55555555beef" // example id: the operator
	otherAccount   = "2222bbbb-3333-4444-5555-66666666cafe" // example id: an ordinary account
	paperComputer  = "computer-1355b83bced1613eb048ee28d66688a8"
	paperOwner     = "e5962310-0000-4000-8000-000000000001" // example id: the paper's owner account
	userComputer   = "computer-7cc96dc670daf6eccd65919e9f70162a"
	testTierID     = "computer-a875023a6379c41f6ddb438f02e76a02"
	operatorPrefix = "/api/platform/operator/computers/"
)

type opKeys map[string]*auth.APIKey // token -> key

func (k opKeys) GetAPIKeyByHash(_ context.Context, hash string) (*auth.APIKey, error) {
	for token, key := range k {
		sum := sha256.Sum256([]byte(token))
		if hex.EncodeToString(sum[:]) == hash {
			return key, nil
		}
	}
	return nil, sql.ErrNoRows
}
func (k opKeys) TouchAPIKeyLastUsed(context.Context, string) error { return nil }
func (k opKeys) GetUserByID(id string) (*auth.User, error) {
	return &auth.User{ID: id, Email: "x@example.com"}, nil
}

type guestSeen struct {
	Method, Path, RawQuery string
	Header                 http.Header
	Body                   string
}

type operatorWorld struct {
	t          *testing.T
	h          *Handler
	priv       ed25519.PrivateKey
	mu         sync.Mutex
	guestCalls []guestSeen
	lookups    []string
	checks     int
	state      string // the paper's vmctl state
	guestCode  int
	guestBody  string
	corpusdUp  bool
	corpusdBad string // when set, the check route answers this body with 200
	oldGuest   bool   // a guest build from before operator receipts: it ignores the marker
}

// R7: a guest that does not record operator receipts is never written to.
func TestOperatorWriteRefusedOnAGuestThatDoesNotRecordReceipts(t *testing.T) {
	w := newOperatorWorld(t, opAccount)
	w.oldGuest = true
	rec := w.do(http.MethodPut, policyURL(paperComputer), "# owner-edited\n", w.session(opAccount))
	if rec.Code != http.StatusConflict {
		t.Fatalf("R7 old guest PUT = %d %s", rec.Code, rec.Body)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, c := range w.guestCalls {
		if c.Method != http.MethodGet {
			t.Fatalf("R7 a write reached an old guest: %+v", c)
		}
	}
}

func newOperatorWorld(t *testing.T, roster string) *operatorWorld {
	t.Helper()
	w := &operatorWorld{t: t, state: "active", guestCode: http.StatusOK, guestBody: `{"ok":true}`, corpusdUp: true}
	ph := &platform.Handler{}
	if _, err := ph.ConfigurePlatformOperatorUsers(roster); err != nil && roster != "malformed" {
		t.Fatal(err)
	}
	if roster == "malformed" {
		_, _ = ph.ConfigurePlatformOperatorUsers(opAccount + ",NOT-AN-ID")
	}
	corpusd := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		w.mu.Lock()
		w.checks++
		up, bad := w.corpusdUp, w.corpusdBad
		w.mu.Unlock()
		if !up {
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}
		if bad != "" {
			_, _ = rw.Write([]byte(bad))
			return
		}
		ph.HandlePlatformOperatorCheck(rw, r)
	}))
	t.Cleanup(corpusd.Close)
	guest := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		w.mu.Lock()
		w.guestCalls = append(w.guestCalls, guestSeen{r.Method, r.URL.Path, r.URL.RawQuery, r.Header.Clone(), string(raw)})
		code, body := w.guestCode, w.guestBody
		w.mu.Unlock()
		rw.Header().Set("Content-Type", "application/json")
		if !w.oldGuest {
			rw.Header().Set("X-Choir-Operator-Receipts", "1")
		}
		rw.Header().Set("Set-Cookie", "guest=secret")
		rw.Header().Set("X-Guest-Internal", "leak")
		rw.WriteHeader(code)
		_, _ = rw.Write([]byte(body))
	}))
	t.Cleanup(guest.Close)
	vm := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		w.mu.Lock()
		w.lookups = append(w.lookups, r.URL.Query().Get("computer_id")+"|user="+r.URL.Query().Get("user_id"))
		state := w.state
		w.mu.Unlock()
		id := r.URL.Query().Get("computer_id")
		if id != paperComputer && id != testTierID {
			rw.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(rw).Encode(map[string]any{"computer_id": id, "user_id": paperOwner, "desktop_id": "primary",
			"kind": "interactive", "state": state, "computer_url": guest.URL, "epoch": 3, "warmness_class": "public_platform"})
	}))
	t.Cleanup(vm.Close)
	h, priv := opinionTestHandler(t, guest.URL, corpusd.URL)
	h.vmctlClient = vmctl.NewClient(vm.URL)
	h.SetAPIKeyValidator(opKeys{
		"choir_sk_rw":    {ID: "k-rw", UserID: opAccount, Scopes: []string{"read:runtime", "write:runtime"}},
		"choir_sk_ro":    {ID: "k-ro", UserID: opAccount, Scopes: []string{"read:runtime"}},
		"choir_sk_bound": {ID: "k-bound", UserID: opAccount, Scopes: []string{"admin"}, ComputerID: paperComputer},
		"choir_sk_other": {ID: "k-other", UserID: otherAccount, Scopes: []string{"admin"}},
	})
	w.h, w.priv = h, priv
	return w
}

func (w *operatorWorld) do(method, path, body string, mutate func(*http.Request)) *httptest.ResponseRecorder {
	w.t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, "https://choir.news"+path, rd)
	if mutate != nil {
		mutate(req)
	}
	rec := httptest.NewRecorder()
	w.h.HandleAPI(rec, req)
	return rec
}

func (w *operatorWorld) session(user string) func(*http.Request) {
	return func(r *http.Request) {
		r.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(w.priv, user)})
	}
}

func bearer(token string) func(*http.Request) {
	return func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+token) }
}

func (w *operatorWorld) guestCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.guestCalls)
}

func (w *operatorWorld) lookupCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.lookups)
}

func policyURL(id string) string { return operatorPrefix + id + "/guest" + policyGuest }

// A1 A2 A3 A4 A5 A6 A7 A8 A9: who is admitted.
func TestOperatorAdmissionFailsClosed(t *testing.T) {
	w := newOperatorWorld(t, opAccount)
	get := policyURL(paperComputer)

	// A1 anonymous.
	if rec := w.do(http.MethodGet, get, "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("A1 anonymous = %d", rec.Code)
	}
	// A4 an ordinary account, session and admin key alike.
	if rec := w.do(http.MethodGet, get, "", w.session(otherAccount)); rec.Code != http.StatusForbidden {
		t.Fatalf("A4 session = %d", rec.Code)
	}
	if rec := w.do(http.MethodGet, get, "", bearer("choir_sk_other")); rec.Code != http.StatusForbidden {
		t.Fatalf("A4 admin key of a non-operator = %d", rec.Code)
	}
	// A5 near matches of the operator id are not the operator.
	for _, near := range []string{strings.ToUpper(opAccount), " " + opAccount, opAccount + " ", opAccount[:8], opAccount + "0"} {
		if rec := w.do(http.MethodGet, get, "", func(r *http.Request) {
			r.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(w.priv, near)})
		}); rec.Code != http.StatusForbidden {
			t.Fatalf("A5 near match %q = %d", near, rec.Code)
		}
	}
	// A7 an email session of the operator account.
	for _, am := range []string{"email_code", "email_link"} {
		emailSession := func(req *http.Request) {
			token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.MapClaims{"sub": opAccount, "iat": time.Now().Unix(), "exp": time.Now().Add(5 * time.Minute).Unix(), "scope": "access", "am": am})
			signed, err := token.SignedString(w.priv)
			if err != nil {
				t.Fatal(err)
			}
			req.AddCookie(&http.Cookie{Name: "choir_access", Value: signed})
		}
		if rec := w.do(http.MethodGet, get, "", emailSession); rec.Code != http.StatusForbidden {
			t.Fatalf("A7 %s = %d", am, rec.Code)
		}
	}
	// A8 a key bound to a computer, even with admin.
	if rec := w.do(http.MethodGet, get, "", bearer("choir_sk_bound")); rec.Code != http.StatusForbidden {
		t.Fatalf("A8 bound key = %d", rec.Code)
	}
	if w.guestCount() != 0 {
		t.Fatalf("a refused request reached the guest: %d", w.guestCount())
	}

	// A9 the key keeps its scopes: a read key reads and cannot write.
	if rec := w.do(http.MethodGet, get, "", bearer("choir_sk_ro")); rec.Code != http.StatusOK {
		t.Fatalf("A9 read key GET = %d %s", rec.Code, rec.Body)
	}
	before := w.guestCount()
	if rec := w.do(http.MethodPut, get, "# owner-edited\n", bearer("choir_sk_ro")); rec.Code != http.StatusForbidden {
		t.Fatalf("A9 read key PUT = %d", rec.Code)
	}
	if rec := w.do(http.MethodPost, operatorPrefix+paperComputer+"/guest/api/model-policy/verify", `{}`, bearer("choir_sk_ro")); rec.Code != http.StatusForbidden {
		t.Fatalf("A9 read key verify = %d", rec.Code)
	}
	if w.guestCount() != before {
		t.Fatal("A9 a read key's write reached the guest")
	}
	// The operator's own passkey session and write key are admitted.
	if rec := w.do(http.MethodPut, get, "# owner-edited\n", bearer("choir_sk_rw")); rec.Code != http.StatusOK {
		t.Fatalf("operator write key PUT = %d %s", rec.Code, rec.Body)
	}
	if rec := w.do(http.MethodGet, get, "", w.session(opAccount)); rec.Code != http.StatusOK {
		t.Fatalf("operator session GET = %d", rec.Code)
	}
}

// A2 A3 A6: the roster is empty, malformed, or corpusd does not answer yes.
func TestOperatorRosterFailsClosed(t *testing.T) {
	for _, roster := range []string{"", "none", "malformed"} {
		w := newOperatorWorld(t, roster)
		if rec := w.do(http.MethodGet, policyURL(paperComputer), "", w.session(opAccount)); rec.Code != http.StatusForbidden {
			t.Fatalf("roster %q admitted the operator: %d", roster, rec.Code)
		}
		if rec := w.do(http.MethodGet, platformOperatorComputersPath, "", w.session(opAccount)); rec.Code != http.StatusForbidden {
			t.Fatalf("roster %q: status admitted the operator: %d", roster, rec.Code)
		}
		if w.guestCount() != 0 || w.lookupCount() != 0 {
			t.Fatalf("roster %q: a refused request reached vmctl or the guest", roster)
		}
	}
	// A6 corpusd down, garbage, or "true"-ish text that is not the typed answer.
	w := newOperatorWorld(t, opAccount)
	w.corpusdUp = false
	if rec := w.do(http.MethodGet, policyURL(paperComputer), "", w.session(opAccount)); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("A6 corpusd 500 = %d", rec.Code)
	}
	w.corpusdUp = true
	for _, bad := range []string{"not json", `{"operator":"true"}`, `{}`, `{"operator":1}`} {
		w.corpusdBad = bad
		if rec := w.do(http.MethodGet, policyURL(paperComputer), "", w.session(opAccount)); rec.Code == http.StatusOK {
			t.Fatalf("A6 corpusd answered %q and the proxy admitted", bad)
		}
	}
	// No corpusd configured at all.
	w2 := newOperatorWorld(t, opAccount)
	w2.h.cfg.CorpusdURL = ""
	if rec := w2.do(http.MethodGet, policyURL(paperComputer), "", w2.session(opAccount)); rec.Code == http.StatusOK {
		t.Fatalf("A6 no corpusd URL admitted: %d", rec.Code)
	}
	if w.guestCount() != 0 || w2.guestCount() != 0 {
		t.Fatal("a refused request reached the guest")
	}
}

// T1 T2 T3 T5: the operator reaches platform computers and nothing else.
func TestOperatorReachesOnlyPlatformComputers(t *testing.T) {
	w := newOperatorWorld(t, opAccount)
	session := w.session(opAccount)
	for name, id := range map[string]string{
		"T1 an ordinary user's computer":       userComputer,
		"T2 a test-tier computer":              testTierID,
		"T3 upper case":                        strings.ToUpper(paperComputer),
		"T3 trailing space":                    paperComputer + "%20",
		"T3 leading space":                     "%20" + paperComputer,
		"T3 prefix of the real id":             paperComputer[:20],
		"T3 id plus suffix":                    paperComputer + "0",
		"T3 traversal":                         "..%2f" + paperComputer,
		"T3 empty":                             "",
		"T3 the owner's user id as computer":   paperOwner,
		"T3 the operator's own id as computer": opAccount,
	} {
		rec := w.do(http.MethodGet, operatorPrefix+id+"/guest"+policyGuest, "", session)
		if rec.Code == http.StatusOK || rec.Code < 400 {
			t.Fatalf("%s: %d %s", name, rec.Code, rec.Body)
		}
	}
	if w.guestCount() != 0 || w.lookupCount() != 0 {
		t.Fatalf("a non-platform computer reached vmctl (%d) or a guest (%d)", w.lookupCount(), w.guestCount())
	}
	// T5 neither a header nor a query retargets the act: the path decides.
	rec := w.do(http.MethodGet, policyURL(paperComputer)+"?computer_id="+userComputer, "", func(r *http.Request) {
		session(r)
		r.Header.Set("X-Choir-Computer", userComputer)
		r.Header.Set("X-Choir-Desktop", "other")
	})
	if rec.Code != http.StatusForbidden { // a query on the policy file is not on the allowlist
		t.Fatalf("T5 query = %d", rec.Code)
	}
	rec = w.do(http.MethodGet, policyURL(paperComputer), "", func(r *http.Request) {
		session(r)
		r.Header.Set("X-Choir-Computer", userComputer)
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("T5 header = %d", rec.Code)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.lookups) != 1 || !strings.HasPrefix(w.lookups[0], paperComputer+"|user=") || strings.Contains(w.lookups[0], "user=e5") {
		t.Fatalf("T5 vmctl was asked %v; want the paper by id with no user join", w.lookups)
	}
	if got := w.guestCalls[len(w.guestCalls)-1].Header.Get("X-Authenticated-Computer"); got != paperComputer {
		t.Fatalf("T5 the guest was told computer %q", got)
	}
	// The operator's ordinary routes did not widen: their own computer routes
	// still demand ownership, and the operator marker is not an owner act.
}

// T4: the roster says platform but vmctl does not agree.
func TestOperatorTargetNeedsVmctlAgreement(t *testing.T) {
	w := newOperatorWorld(t, opAccount)
	w.h.vmctlClient = nil
	if rec := w.do(http.MethodGet, policyURL(paperComputer), "", w.session(opAccount)); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("T4 no vmctl = %d", rec.Code)
	}
	w = newOperatorWorld(t, opAccount)
	w.state = "stopped" // a stopped guest with no wake path in the test: the act must not be faked
	rec := w.do(http.MethodGet, policyURL(paperComputer), "", w.session(opAccount))
	if rec.Code == http.StatusOK && w.guestCount() == 0 {
		t.Fatal("T4 answered OK with no guest call")
	}
}

// T6 F1 F2 F3 F4: only the allowlisted routes, and what the guest sees.
func TestOperatorForwardsOnlyTheAllowlistAndBuildsTheRequestFromNothing(t *testing.T) {
	w := newOperatorWorld(t, opAccount)
	session := w.session(opAccount)
	base := operatorPrefix + paperComputer + "/guest"

	// T6 refused routes never reach the guest.
	revision := "20261109T120000.000001Z.toml"
	for name, c := range map[string]struct{ method, path string }{
		"DELETE policy":            {http.MethodDelete, base + policyGuest},
		"PATCH policy":             {http.MethodPatch, base + policyGuest},
		"POST policy":              {http.MethodPost, base + policyGuest},
		"PUT revision":             {http.MethodPut, base + "/api/files/System/model-policy.revisions/" + revision},
		"GET another file":         {http.MethodGet, base + "/api/files/System/other.toml"},
		"GET the mailbox":          {http.MethodGet, base + "/api/files/Mail"},
		"GET bad revision name":    {http.MethodGet, base + "/api/files/System/model-policy.revisions/../model-policy.toml"},
		"GET revision traversal":   {http.MethodGet, base + "/api/files/System/model-policy.revisions/..%2fmodel-policy.toml"},
		"GET files root":           {http.MethodGet, base + "/api/files"},
		"GET encoded slash":        {http.MethodGet, base + "/api/files/System%2fmodel-policy.toml"},
		"GET dot segment":          {http.MethodGet, base + "/api/files/System/./model-policy.toml"},
		"GET double slash":         {http.MethodGet, base + "/api/files//System/model-policy.toml"},
		"PUT policy with query":    {http.MethodPut, base + policyGuest + "?x=1"},
		"resolve bad role":         {http.MethodGet, base + "/api/model-policy/resolve?role=a/b"},
		"resolve extra query":      {http.MethodGet, base + "/api/model-policy/resolve?role=texture&x=1"},
		"other model-policy route": {http.MethodGet, base + "/api/model-policy/other"},
		"agent api":                {http.MethodPost, base + "/api/agent/submit"},
		"schedules write":          {http.MethodPost, base + "/api/autopaper/sources/schedules"},
		"no guest segment":         {http.MethodGet, operatorPrefix + paperComputer},
		"wrong segment":            {http.MethodGet, operatorPrefix + paperComputer + "/host" + policyGuest},
	} {
		rec := w.do(c.method, c.path, "x", session)
		if rec.Code < 400 {
			t.Fatalf("T6 %s = %d", name, rec.Code)
		}
	}
	if w.guestCount() != 0 {
		t.Fatalf("T6 a refused route reached the guest: %+v", w.guestCalls)
	}

	// F1 F2 F3: an allowed PUT. The client tries to smuggle identity and
	// credentials; the guest sees the paper's owner, the operator marker and
	// nothing the client sent.
	rec := w.do(http.MethodPut, base+policyGuest, "# owner-edited\n[defaults]\nmodel = \"x\"\n", func(r *http.Request) {
		bearer("choir_sk_rw")(r)
		r.Header.Set("X-Authenticated-User", "attacker")
		r.Header.Set("X-Choir-Platform-Operator", "99999999-9999-9999-9999-999999999999")
		r.Header.Set("X-Internal-Caller", "true")
		r.Header.Set("X-Choir-Internal-Token", "stolen")
		r.Header.Set("Cookie", "choir_access=abc")
		r.Header.Set("Content-Type", "application/octet-stream")
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("allowed PUT = %d %s", rec.Code, rec.Body)
	}
	w.mu.Lock()
	seen := w.guestCalls[len(w.guestCalls)-1]
	w.mu.Unlock()
	if seen.Method != http.MethodPut || seen.Path != policyGuest || !strings.Contains(seen.Body, "owner-edited") {
		t.Fatalf("guest saw %+v", seen)
	}
	if got := seen.Header.Get("X-Authenticated-User"); got != paperOwner {
		t.Fatalf("F2 guest identity = %q, want the paper's owner", got)
	}
	if got := seen.Header.Values("X-Choir-Platform-Operator"); len(got) != 1 || got[0] != opAccount {
		t.Fatalf("F2 operator marker = %v", got)
	}
	for _, name := range []string{"Authorization", "Cookie", "X-Internal-Caller", "X-Choir-Internal-Token", "X-Authenticated-Email", "X-Choir-Computer"} {
		if v := seen.Header.Values(name); len(v) != 0 {
			t.Fatalf("F1 header %s reached the guest: %v", name, v)
		}
	}
	// F4 no header of the guest's reaches the client.
	if rec.Header().Get("Set-Cookie") != "" || rec.Header().Get("X-Guest-Internal") != "" {
		t.Fatalf("F4 guest headers leaked: %v", rec.Header())
	}
	// A read of an allowed revision and a resolve go through.
	if rec := w.do(http.MethodGet, base+"/api/files/System/model-policy.revisions/"+revision, "", session); rec.Code != http.StatusOK {
		t.Fatalf("revision read = %d", rec.Code)
	}
	if rec := w.do(http.MethodGet, base+"/api/model-policy/resolve?role=texture", "", session); rec.Code != http.StatusOK {
		t.Fatalf("resolve = %d", rec.Code)
	}
}

// O2: the status route, read only.
func TestOperatorStatusListsPlatformComputers(t *testing.T) {
	w := newOperatorWorld(t, opAccount)
	w.guestBody = `{"model":"pool:default"}`
	if rec := w.do(http.MethodGet, platformOperatorComputersPath, "", w.session(otherAccount)); rec.Code != http.StatusForbidden {
		t.Fatalf("non-operator status = %d", rec.Code)
	}
	if rec := w.do(http.MethodPost, platformOperatorComputersPath, "", w.session(opAccount)); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d", rec.Code)
	}
	rec := w.do(http.MethodGet, platformOperatorComputersPath, "", bearer("choir_sk_ro"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d %s", rec.Code, rec.Body)
	}
	var out struct {
		Computers []operatorComputerStatus `json:"computers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Computers) != 1 || out.Computers[0].ComputerID != paperComputer || out.Computers[0].Tier != "platform" || out.Computers[0].State != "active" {
		t.Fatalf("status = %+v", out.Computers)
	}
	if len(out.Computers[0].Policy) != len(operatorPolicyRoles()) || len(out.Computers[0].Schedules) == 0 {
		t.Fatalf("status lacks policy or schedules: %+v", out.Computers[0])
	}
	for _, c := range w.guestCalls {
		if c.Method != http.MethodGet {
			t.Fatalf("status wrote: %+v", c)
		}
	}
	// A stopped computer is reported, not woken.
	w2 := newOperatorWorld(t, opAccount)
	w2.state = "stopped"
	rec = w2.do(http.MethodGet, platformOperatorComputersPath, "", w2.session(opAccount))
	if rec.Code != http.StatusOK || w2.guestCount() != 0 || !strings.Contains(rec.Body.String(), `"state":"stopped"`) || !strings.Contains(rec.Body.String(), "errors") {
		t.Fatalf("stopped status = %d guest calls %d body %s", rec.Code, w2.guestCount(), rec.Body)
	}
}

// The operator's ordinary routes did not widen: the operator naming a platform
// computer on the ordinary /api route still demands ownership.
func TestOperatorMarkerIsNotAnOrdinaryOwnerAct(t *testing.T) {
	w := newOperatorWorld(t, opAccount)
	rec := w.do(http.MethodGet, "/api/files/System/model-policy.toml", "", func(r *http.Request) {
		bearer("choir_sk_rw")(r)
		r.Header.Set("X-Choir-Computer", paperComputer)
		r.Header.Set("X-Choir-Platform-Operator", opAccount)
	})
	if rec.Code == http.StatusOK {
		t.Fatalf("operator read the paper through the ordinary route: %d", rec.Code)
	}
	if w.guestCount() != 0 {
		t.Fatal("the ordinary route reached the paper's guest")
	}
}
