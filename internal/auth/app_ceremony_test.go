package auth

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
)

// A software authenticator: enough of a platform passkey to run the phone
// app's whole ceremony against the real handlers, with the origin and the
// relying party the deployed server has.

const (
	testRP     = "choir.news"
	testOrigin = "https://choir.news"
	testApp    = "news.choir.autoradio.lab"
)

func appEnv(t *testing.T) *Handler {
	t.Helper()
	h, _ := testHandlerEnv(t)
	cfg := *h.config
	cfg.RPID = testRP
	cfg.RPOrigins = []string{testOrigin}
	wa, err := webauthn.New(&webauthn.Config{RPID: testRP, RPDisplayName: "go-choir test", RPOrigins: []string{testOrigin}})
	if err != nil {
		t.Fatal(err)
	}
	h.config = &cfg
	h.webauthn = wa
	return h
}

func cborHead(major byte, n int) []byte {
	switch {
	case n < 24:
		return []byte{major<<5 | byte(n)}
	case n < 256:
		return []byte{major<<5 | 24, byte(n)}
	default:
		return []byte{major<<5 | 25, byte(n >> 8), byte(n)}
	}
}
func cborText(s string) []byte  { return append(cborHead(3, len(s)), s...) }
func cborBytes(b []byte) []byte { return append(cborHead(2, len(b)), b...) }
func cborInt(n int) []byte {
	if n >= 0 {
		return cborHead(0, n)
	}
	return cborHead(1, -1-n)
}

type softPasskey struct {
	key    *ecdsa.PrivateKey
	id     []byte
	handle []byte // the user handle, set when the credential is created
	count  uint32
}

func newSoftPasskey(t *testing.T) *softPasskey {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	id := make([]byte, 16)
	_, _ = rand.Read(id)
	return &softPasskey{key: k, id: id}
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func rpHash() []byte { h := sha256.Sum256([]byte(testRP)); return h[:] }

func (p *softPasskey) create(t *testing.T, options map[string]any, origin string) []byte {
	t.Helper()
	pk := options["publicKey"].(map[string]any)
	challenge := pk["challenge"].(string)
	if u, ok := pk["user"].(map[string]any); ok {
		p.handle, _ = base64.RawURLEncoding.DecodeString(u["id"].(string))
	}
	x := p.key.PublicKey.X.FillBytes(make([]byte, 32))
	y := p.key.PublicKey.Y.FillBytes(make([]byte, 32))
	var cose []byte
	cose = append(cose, cborHead(5, 5)...)
	cose = append(cose, cborInt(1)...)
	cose = append(cose, cborInt(2)...)
	cose = append(cose, cborInt(3)...)
	cose = append(cose, cborInt(-7)...)
	cose = append(cose, cborInt(-1)...)
	cose = append(cose, cborInt(1)...)
	cose = append(cose, cborInt(-2)...)
	cose = append(cose, cborBytes(x)...)
	cose = append(cose, cborInt(-3)...)
	cose = append(cose, cborBytes(y)...)
	// flags: user present, user verified, attested data, backup eligible, backed up (a synced passkey)
	auth := append(rpHash(), 0x01|0x04|0x40|0x08|0x10, 0, 0, 0, 0)
	auth = append(auth, make([]byte, 16)...)
	auth = append(auth, byte(len(p.id)>>8), byte(len(p.id)))
	auth = append(auth, p.id...)
	auth = append(auth, cose...)
	var att []byte
	att = append(att, cborHead(5, 3)...)
	att = append(att, cborText("fmt")...)
	att = append(att, cborText("none")...)
	att = append(att, cborText("attStmt")...)
	att = append(att, cborHead(5, 0)...)
	att = append(att, cborText("authData")...)
	att = append(att, cborBytes(auth)...)
	cd := fmt.Sprintf(`{"type":"webauthn.create","challenge":%q,"origin":%q,"crossOrigin":false}`, challenge, origin)
	out, _ := json.Marshal(map[string]any{
		"id": b64(p.id), "rawId": b64(p.id), "type": "public-key",
		"response":               map[string]any{"clientDataJSON": b64([]byte(cd)), "attestationObject": b64(att)},
		"clientExtensionResults": map[string]any{},
	})
	return out
}

func (p *softPasskey) assert(t *testing.T, options map[string]any, origin string) []byte {
	t.Helper()
	pk := options["publicKey"].(map[string]any)
	challenge := pk["challenge"].(string)
	p.count++
	auth := append(rpHash(), 0x01|0x04|0x08|0x10, byte(p.count>>24), byte(p.count>>16), byte(p.count>>8), byte(p.count))
	cd := fmt.Sprintf(`{"type":"webauthn.get","challenge":%q,"origin":%q,"crossOrigin":false}`, challenge, origin)
	cdh := sha256.Sum256([]byte(cd))
	digest := sha256.Sum256(append(append([]byte{}, auth...), cdh[:]...))
	sig, err := ecdsa.SignASN1(rand.Reader, p.key, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	out, _ := json.Marshal(map[string]any{
		"id": b64(p.id), "rawId": b64(p.id), "type": "public-key",
		"response": map[string]any{
			"clientDataJSON": b64([]byte(cd)), "authenticatorData": b64(auth), "signature": b64(sig), "userHandle": b64(p.handle),
		},
		"clientExtensionResults": map[string]any{},
	})
	return out
}

func postJSON(h http.HandlerFunc, body any) *httptest.ResponseRecorder {
	var b []byte
	switch v := body.(type) {
	case []byte:
		b = v
	default:
		b, _ = json.Marshal(v)
	}
	req := httptest.NewRequest(http.MethodPost, "/x", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

func decodeMap(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("not JSON (%d): %s", rec.Code, rec.Body.String())
	}
	return m
}

func appFinishBody(app string, credential []byte) map[string]any {
	return map[string]any{"app": app, "credential": json.RawMessage(credential)}
}

func createAccount(t *testing.T, h *Handler, email string, p *softPasskey, origin string) *httptest.ResponseRecorder {
	t.Helper()
	begin := postJSON(h.HandleRegisterBegin, map[string]string{"email": email})
	if begin.Code != 200 {
		return begin
	}
	cred := p.create(t, decodeMap(t, begin), origin)
	return postJSON(h.HandleAppRegisterFinish, appFinishBody(testApp, cred))
}

func assertNoCookies(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if len(rec.Result().Cookies()) != 0 {
		t.Fatalf("an app finish set cookies: %v", rec.Result().Cookies())
	}
}

// F1 and F2: the app creates an account with a passkey, and the same passkey
// signs in with no email (a discoverable credential); each ends in a key with
// the device grant, for the right account, and no cookie.
func TestAppCreatesAnAccountAndSignsInWithoutAnEmail(t *testing.T) {
	h := appEnv(t)
	p := newSoftPasskey(t)
	rec := createAccount(t, h, "someone@example.com", p, testOrigin)
	if rec.Code != 200 {
		t.Fatalf("create account = %d: %s", rec.Code, rec.Body.String())
	}
	assertNoCookies(t, rec)
	created := decodeMap(t, rec)
	secret, _ := created["api_key"].(string)
	if !strings.HasPrefix(secret, APIKeyPrefix) {
		t.Fatalf("no key in the answer: %v", created)
	}
	ak, err := h.store.GetAPIKeyByHash(t.Context(), hashAPIKeySecret(secret))
	if err != nil {
		t.Fatalf("the minted key is not usable: %v", err)
	}
	user, _ := h.store.GetUserByEmail("someone@example.com")
	if ak.UserID != user.ID || ak.ID != created["key_id"] {
		t.Fatalf("key belongs to %q, want %q", ak.UserID, user.ID)
	}
	if got, want := strings.Join(ak.Scopes, ","), strings.Join(AllScopes(), ","); got != want {
		t.Fatalf("scopes = %v, want the device grant %v", ak.Scopes, AllScopes())
	}
	if ak.ExpiresAt != nil || ak.ComputerID != "" {
		t.Fatalf("the key should be owner-wide and without an expiry: %+v", ak)
	}
	if !strings.Contains(ak.Label, testApp) {
		t.Fatalf("label %q does not name the app", ak.Label)
	}

	// Sign in: no email, a discoverable credential.
	begin := postJSON(h.HandleAppLoginBegin, map[string]string{})
	if begin.Code != 200 {
		t.Fatalf("login begin = %d: %s", begin.Code, begin.Body.String())
	}
	opts := decodeMap(t, begin)
	pk := opts["publicKey"].(map[string]any)
	if l, _ := pk["allowCredentials"].([]any); len(l) != 0 {
		t.Fatalf("a discoverable ceremony names no credentials: %v", l)
	}
	cred := p.assert(t, opts, testOrigin)
	fin := postJSON(h.HandleAppLoginFinish, appFinishBody(testApp, cred))
	if fin.Code != 200 {
		t.Fatalf("login finish = %d: %s", fin.Code, fin.Body.String())
	}
	assertNoCookies(t, fin)
	again := decodeMap(t, fin)
	ak2, err := h.store.GetAPIKeyByHash(t.Context(), hashAPIKeySecret(again["api_key"].(string)))
	if err != nil || ak2.UserID != user.ID || ak2.ID == ak.ID {
		t.Fatalf("sign-in key: %+v %v (want a new key for the same account)", ak2, err)
	}

	// F3: a replay of the same finish finds no challenge and mints nothing.
	n := countKeys(t, h, user.ID)
	replay := postJSON(h.HandleAppLoginFinish, appFinishBody(testApp, cred))
	if replay.Code != 400 || countKeys(t, h, user.ID) != n {
		t.Fatalf("replay = %d, keys %d -> %d", replay.Code, n, countKeys(t, h, user.ID))
	}
}

func countKeys(t *testing.T, h *Handler, userID string) int {
	t.Helper()
	var n int
	if err := h.store.db.QueryRow("SELECT COUNT(*) FROM api_keys WHERE user_id = ?", userID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// F4: the app is named from a fixed list; free text and a missing name mint nothing.
func TestAppFinishNamesAnAppFromTheFixedList(t *testing.T) {
	h := appEnv(t)
	p := newSoftPasskey(t)
	begin := postJSON(h.HandleRegisterBegin, map[string]string{"email": "a@example.com"})
	cred := p.create(t, decodeMap(t, begin), testOrigin)
	for _, app := range []string{"", "evil.example", "news.choir.autoradio.lab ", "NEWS.CHOIR.AUTORADIO.LAB", "news.choir.autoradio.labx"} {
		rec := postJSON(h.HandleAppRegisterFinish, appFinishBody(app, cred))
		if rec.Code != 400 {
			t.Fatalf("app %q = %d, want 400", app, rec.Code)
		}
	}
	var n int
	_ = h.store.db.QueryRow("SELECT COUNT(*) FROM api_keys").Scan(&n)
	if n != 0 {
		t.Fatalf("%d keys were minted for refused apps", n)
	}
	// The same credential still works: a refused app did not consume the challenge.
	if rec := postJSON(h.HandleAppRegisterFinish, appFinishBody("news.choir.autoradio", cred)); rec.Code != 200 {
		t.Fatalf("release id = %d: %s", rec.Code, rec.Body.String())
	}
}

// F5: no origin is added for the app: only the relying party's own https
// origin is accepted, on both ceremonies.
func TestAppCeremonyAcceptsOnlyTheRelyingPartyOrigin(t *testing.T) {
	h := appEnv(t)
	for _, origin := range []string{"https://evil.example", "http://choir.news", "https://choir.news.evil.example", "android:apk-key-hash:abc", "news.choir.autoradio.lab:/x"} {
		p := newSoftPasskey(t)
		if rec := createAccount(t, h, "o"+fmt.Sprint(len(origin))+"@example.com", p, origin); rec.Code == 200 {
			t.Fatalf("origin %q accepted for creation", origin)
		}
	}
	p := newSoftPasskey(t)
	if rec := createAccount(t, h, "good@example.com", p, testOrigin); rec.Code != 200 {
		t.Fatalf("good origin refused: %s", rec.Body.String())
	}
	opts := decodeMap(t, postJSON(h.HandleAppLoginBegin, map[string]string{}))
	bad := p.assert(t, opts, "https://evil.example")
	if rec := postJSON(h.HandleAppLoginFinish, appFinishBody(testApp, bad)); rec.Code == 200 {
		t.Fatal("a wrong origin signed in")
	}
}

// F6: a passkey the server never stored, and a challenge used on the wrong
// ceremony, mint nothing.
func TestAppLoginRefusesUnknownPasskeysAndCrossedChallenges(t *testing.T) {
	h := appEnv(t)
	stranger := newSoftPasskey(t)
	stranger.handle = []byte("no-such-user")
	opts := decodeMap(t, postJSON(h.HandleAppLoginBegin, map[string]string{}))
	if rec := postJSON(h.HandleAppLoginFinish, appFinishBody(testApp, stranger.assert(t, opts, testOrigin))); rec.Code != 400 {
		t.Fatalf("unknown passkey = %d", rec.Code)
	}
	// A registration challenge cannot finish a login, and a login challenge cannot finish a registration.
	p := newSoftPasskey(t)
	reg := decodeMap(t, postJSON(h.HandleRegisterBegin, map[string]string{"email": "c@example.com"}))
	cred := p.create(t, reg, testOrigin)
	p.handle = []byte("x")
	fake := p.assert(t, reg, testOrigin)
	if rec := postJSON(h.HandleAppLoginFinish, appFinishBody(testApp, fake)); rec.Code != 400 {
		t.Fatalf("registration challenge used for login = %d", rec.Code)
	}
	login := decodeMap(t, postJSON(h.HandleAppLoginBegin, map[string]string{}))
	crossed := p.create(t, login, testOrigin)
	if rec := postJSON(h.HandleAppRegisterFinish, appFinishBody(testApp, crossed)); rec.Code != 400 {
		t.Fatalf("login challenge used for registration = %d", rec.Code)
	}
	if rec := postJSON(h.HandleAppRegisterFinish, appFinishBody(testApp, cred)); rec.Code != 200 {
		t.Fatalf("the real registration = %d: %s", rec.Code, rec.Body.String())
	}
}

// F7: sign-up closed is the waitlist answer, at begin and again at finish,
// and mints no key.
func TestAppCreateAnswersTheWaitlistWhenSignUpIsClosed(t *testing.T) {
	h := appEnv(t)
	first := newSoftPasskey(t)
	if rec := createAccount(t, h, "first@example.com", first, testOrigin); rec.Code != 200 {
		t.Fatalf("first = %d: %s", rec.Code, rec.Body.String())
	}
	h.config.RegistrationCap = 1
	rec := postJSON(h.HandleRegisterBegin, map[string]string{"email": "second@example.com"})
	if rec.Code != 403 || decodeMap(t, rec)["code"] != "registration_waitlist" {
		t.Fatalf("begin over the cap = %d: %s", rec.Code, rec.Body.String())
	}
	// Begun below the cap, finished above it.
	h.config.RegistrationCap = 2
	second := newSoftPasskey(t)
	begin := decodeMap(t, postJSON(h.HandleRegisterBegin, map[string]string{"email": "third@example.com"}))
	h.config.RegistrationCap = 1
	fin := postJSON(h.HandleAppRegisterFinish, appFinishBody(testApp, second.create(t, begin, testOrigin)))
	if fin.Code != 403 || decodeMap(t, fin)["code"] != "registration_waitlist" {
		t.Fatalf("finish over the cap = %d: %s", fin.Code, fin.Body.String())
	}
	var n int
	_ = h.store.db.QueryRow("SELECT COUNT(*) FROM api_keys").Scan(&n)
	if n != 1 {
		t.Fatalf("%d keys, want only the first account's", n)
	}
	// An existing account still signs in at the cap.
	opts := decodeMap(t, postJSON(h.HandleAppLoginBegin, map[string]string{}))
	if rec := postJSON(h.HandleAppLoginFinish, appFinishBody(testApp, first.assert(t, opts, testOrigin))); rec.Code != 200 {
		t.Fatalf("sign-in at the cap = %d", rec.Code)
	}
}

// F8: an email that already has an account says so (the app tells the person to sign in).
func TestAppCreateWithAnExistingEmailIsAConflict(t *testing.T) {
	h := appEnv(t)
	if rec := createAccount(t, h, "dup@example.com", newSoftPasskey(t), testOrigin); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	if rec := postJSON(h.HandleRegisterBegin, map[string]string{"email": "dup@example.com"}); rec.Code != 409 {
		t.Fatalf("duplicate begin = %d", rec.Code)
	}
}

// F9: what strangers can cause is bounded: the in-memory challenge table has a
// ceiling, and expired challenges are not used.
func TestAppChallengeTableIsBounded(t *testing.T) {
	h := appEnv(t)
	for i := 0; i < appChallengeMax; i++ {
		if rec := postJSON(h.HandleAppLoginBegin, map[string]string{}); rec.Code != 200 {
			t.Fatalf("begin %d = %d", i, rec.Code)
		}
	}
	if rec := postJSON(h.HandleAppLoginBegin, map[string]string{}); rec.Code != 503 {
		t.Fatalf("begin past the ceiling = %d, want 503", rec.Code)
	}
	var c appChallenges
	s := webauthn.SessionData{Challenge: "old"}
	if !c.put(s, timeAgo(10)) {
		t.Fatal("put")
	}
	if _, ok := c.take("old", timeNow()); ok {
		t.Fatal("an expired challenge was used")
	}
}

func timeNow() time.Time        { return time.Now().UTC() }
func timeAgo(min int) time.Time { return time.Now().UTC().Add(-time.Duration(min) * time.Minute) }
