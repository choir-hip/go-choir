package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/yusefmosiah/go-choir/internal/appids"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Written before the code (CLAUDE.md, Testing). The approval page may close the
// phone's sign-in sheet (docs/reference/exact/auth-session.md): after an approval
// it navigates to a callback the SERVER builds. How that could fail:
//
//  R1 an approve that names an app of ours gets no return_to, or a wrong one.
//  R2 a return target the client made up (a URL, another scheme, a path, a host
//     with userinfo, a script) is followed or echoed: an open redirect.
//  R3 a state that could break out of the query (&, #, %, a space, a newline), or
//     is empty or huge, is echoed.
//  R4 (review, 2026-11-23) the hint refuses an approval. A bad return or state must
//     never stop the decision: it is recorded, the key is collected by the poll,
//     and return_to is simply absent. Nor may it count as a guessed code, nor tell
//     a caller anything about a code (an unknown code is the same 404 with or
//     without a hint).
//  R5 the callback carries something that is not a signal: the device code, the
//     user code, the label, the user id, a key.
//  R6 the page without a return changes: no return_to, same body as before.
//  R7 the gates move: an email session, an API key, a form post and a missing
//     session are refused exactly as before, with or without a return.
//  R8 the allowlist drifts from the apps that exist (mobile/AutoradioLab/app.json:
//     the default id and each variant's), so a build's callback scheme is refused
//     or an unknown one is admitted.
//  R9 a deny answers no return_to (the app learns of it by its poll), and it mints nothing.
//  R11 (2026-11-23, the store build) the list lacks the production app's id,
//     news.choir.autoradio (so its sign-in sheet never returns), or it admits a near
//     miss of that id or of the two lab ids. Exactly those three are answered.
//  R10 an over-long hint cannot decide anything: 400 before the lookup.

const goodState = "k3VqS0m9Zt_xA1b2C3d4-E5f"

func approveBody(code, decision string, extra map[string]string) map[string]string {
	m := map[string]string{"user_code": code, "decision": decision}
	for k, v := range extra {
		m[k] = v
	}
	return m
}

func TestDeviceApproveReturnsTheAppsOwnCallback(t *testing.T) {
	h, priv := testHandlerEnv(t)
	u, _ := h.store.CreateUser("return-user", "return@example.com")
	for _, scheme := range []string{"news.choir.autoradio", "news.choir.autoradio.lab", "news.choir.autoradio.lab.one"} {
		b := deviceBegin(t, h, "Autoradio", "203.0.113.9")
		rec := deviceDecide(h, passkeyCookieReq(http.MethodPost, "/auth/device/approve",
			approveBody(b.UserCode, "approve", map[string]string{"return": scheme, "state": goodState}), priv, u.ID))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: approve %d: %s", scheme, rec.Code, rec.Body.String())
		}
		body := decodeBody(t, rec)
		want := scheme + ":/signed-in?state=" + goodState
		if body["return_to"] != want { // R1
			t.Fatalf("%s: return_to %v, want %q", scheme, body["return_to"], want)
		}
		if body["status"] != "approved" {
			t.Fatalf("status %v", body["status"])
		}
		// R5: nothing else in the body that could be redeemed, and the callback is exactly scheme, path, state.
		raw := rec.Body.String()
		for _, secret := range []string{b.DeviceCode, b.UserCode, strings.ReplaceAll(b.UserCode, "-", ""), u.ID, "choir_sk_", APIKeyPrefix} {
			if secret != "" && strings.Contains(raw, secret) {
				t.Fatalf("approve body carries %q: %s", secret, raw)
			}
		}
		keys := []string{}
		for k := range body {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		if strings.Join(keys, ",") != "label,return_to,status" {
			t.Fatalf("approve body fields %v", keys)
		}
		// The key is still collected only by the poller that began the flow.
		rewindPoll(t, h)
		if code, m := devicePoll(h, b.DeviceCode); code != http.StatusOK || !strings.HasPrefix(m["api_key"].(string), APIKeyPrefix) {
			t.Fatalf("collect %d %v", code, m)
		}
	}
}

func TestDeviceDenyReturnsNoCallback(t *testing.T) { // R9
	h, priv := testHandlerEnv(t)
	u, _ := h.store.CreateUser("deny-user", "deny@example.com")
	b := deviceBegin(t, h, "Autoradio", "")
	rec := deviceDecide(h, passkeyCookieReq(http.MethodPost, "/auth/device/approve",
		approveBody(b.UserCode, "deny", map[string]string{"return": "news.choir.autoradio.lab", "state": goodState}), priv, u.ID))
	if rec.Code != http.StatusOK {
		t.Fatalf("deny %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	if _, has := body["return_to"]; has || body["status"] != "denied" {
		t.Fatalf("deny body %v: a denial answers no return_to", body)
	}
	rewindPoll(t, h)
	if code, m := devicePoll(h, b.DeviceCode); code != http.StatusBadRequest || m["error"] != "access_denied" {
		t.Fatalf("poll after deny: %d %v", code, m)
	}
}

// R10 (review, 2026-11-23): a hint that pushes the body past readJSON's 1 KiB
// cap is refused as a bad body (400) before the code is looked up: no decision,
// no miss counted, nothing about the code. The page never sends such a hint
// (device-return.js bounds it); this pins what the server does if a client does.
func TestDeviceApproveOverlongHintIsABadBodyAndDecidesNothing(t *testing.T) {
	h, priv := testHandlerEnv(t)
	u, _ := h.store.CreateUser("long-user", "long@example.com")
	b := deviceBegin(t, h, "x", "")
	rec := deviceDecide(h, passkeyCookieReq(http.MethodPost, "/auth/device/approve",
		approveBody(b.UserCode, "approve", map[string]string{"return": "news.choir.autoradio.lab", "state": strings.Repeat("a", 2000)}), priv, u.ID))
	if rec.Code != http.StatusBadRequest || strings.Contains(rec.Body.String(), "return_to") {
		t.Fatalf("over-long hint: %d %s", rec.Code, rec.Body.String())
	}
	if n := len(h.device.misses[u.ID]); n != 0 {
		t.Fatalf("%d misses counted for a refused body", n)
	}
	if code, m := devicePoll(h, b.DeviceCode); code != http.StatusBadRequest || m["error"] != "authorization_pending" {
		t.Fatalf("after the refused body: %d %v", code, m)
	}
}

func TestDeviceApproveRecordsTheDecisionAndOmitsAReturnItDoesNotKnow(t *testing.T) {
	h, priv := testHandlerEnv(t)
	u, _ := h.store.CreateUser("refuse-user", "refuse@example.com")
	bad := []struct{ name, ret, state string }{
		{"a web URL", "https://evil.example/cb", goodState},
		{"a web URL with our scheme in it", "https://news.choir.autoradio.lab/signed-in", goodState},
		{"another scheme", "evil.app", goodState},
		{"our scheme with a host", "news.choir.autoradio.lab://evil", goodState},
		{"our scheme with a path", "news.choir.autoradio.lab:/other", goodState},
		{"our scheme with userinfo", "news.choir.autoradio.lab@evil.example", goodState},
		{"a prefix of ours", "news.choir.autoradi", goodState},
		{"the production id with a suffix not in the list", "news.choir.autoradio.evil", goodState},
		{"the production id with a letter added", "news.choir.autoradiox", goodState},
		{"the production id upper case", "News.Choir.Autoradio", goodState},
		{"the production id with a path", "news.choir.autoradio:/signed-in", goodState},
		{"the production id with a colon", "news.choir.autoradio:", goodState},
		{"the production id with a host", "news.choir.autoradio://evil", goodState},
		{"the production id with userinfo", "news.choir.autoradio@evil.example", goodState},
		{"the production id with a trailing dot", "news.choir.autoradio.", goodState},
		{"the production id with a space", "news.choir.autoradio ", goodState},
		{"empty", "", goodState},
		{"ours with a suffix not in the list", "news.choir.autoradio.lab.two", goodState},
		{"a script", "javascript:alert(1)", goodState},
		{"upper case", "News.Choir.Autoradio.Lab", goodState},
		{"a space", "news.choir.autoradio.lab ", goodState},
		{"a return with no state", "news.choir.autoradio.lab", ""},
		{"a state with no return", "", goodState},
		{"a state that ends the query", "news.choir.autoradio.lab", goodState + "&code=x"},
		{"a state with a fragment", "news.choir.autoradio.lab", goodState + "#x"},
		{"a state with a percent", "news.choir.autoradio.lab", "abc%2Fdefghijklmnopqrstuvwx"},
		{"a state with a space", "news.choir.autoradio.lab", "abcdefghijklmnop qrstuvwx"},
		{"a state with a newline", "news.choir.autoradio.lab", "abcdefghijklmnop\nqrstuvwx"},
		{"a short state", "news.choir.autoradio.lab", "abc"},
		{"a long state", "news.choir.autoradio.lab", strings.Repeat("a", 129)},
	}
	for i, c := range bad {
		b := deviceBegin(t, h, "Autoradio", fmt.Sprintf("203.0.113.%d", 10+i))
		rec := deviceDecide(h, passkeyCookieReq(http.MethodPost, "/auth/device/approve",
			approveBody(b.UserCode, "approve", map[string]string{"return": c.ret, "state": c.state}), priv, u.ID))
		if rec.Code != http.StatusOK { // R4: the hint never refuses an approval
			t.Fatalf("%s: status %d, want 200: %s", c.name, rec.Code, rec.Body.String())
		}
		body := decodeBody(t, rec)
		if _, has := body["return_to"]; has || body["status"] != "approved" || len(body) != 2 { // R2, R3: nothing built from a value we do not accept
			t.Fatalf("%s: body %v", c.name, body)
		}
		for _, echoed := range []string{c.ret, c.state} {
			if len(echoed) > 3 && strings.Contains(rec.Body.String(), echoed) {
				t.Fatalf("%s: the response echoes the client's value %q: %s", c.name, echoed, rec.Body.String())
			}
		}
		// The decision was recorded: the poller that began the flow collects its key.
		rewindPoll(t, h)
		if code, m := devicePoll(h, b.DeviceCode); code != http.StatusOK || !strings.HasPrefix(fmt.Sprint(m["api_key"]), APIKeyPrefix) {
			t.Fatalf("%s: collect %d %v", c.name, code, m)
		}
	}
	if h.deviceGuessesExhausted(u.ID) || len(h.device.misses[u.ID]) != 0 {
		t.Fatalf("a bad hint counted as a miss: %v", h.device.misses[u.ID])
	}
}

// R4: no oracle. An unknown code is the same 404 with a good hint, a bad hint or none, and counts the same miss; the hint is never answered.
func TestDeviceApproveUnknownCodeIsTheSameWithOrWithoutAHint(t *testing.T) {
	h, priv := testHandlerEnv(t)
	u, _ := h.store.CreateUser("oracle-user", "oracle@example.com")
	hints := []map[string]string{nil,
		{"return": "news.choir.autoradio.lab", "state": goodState},
		{"return": "evil.app", "state": "x"}}
	for i, hint := range hints {
		rec := deviceDecide(h, passkeyCookieReq(http.MethodPost, "/auth/device/approve", approveBody("BCDF-GHJK", "approve", hint), priv, u.ID))
		if rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), "return_to") {
			t.Fatalf("hint %d: %d %s", i, rec.Code, rec.Body.String())
		}
		if got := len(h.device.misses[u.ID]); got != i+1 {
			t.Fatalf("hint %d: %d misses, want %d", i, got, i+1)
		}
	}
}

// R7: a form post (the cross-site shape) is refused with a hint present, and nothing is decided.
func TestDeviceApproveFormPostWithAReturnIsRefused(t *testing.T) {
	h, priv := testHandlerEnv(t)
	u, _ := h.store.CreateUser("form-user", "form@example.com")
	b := deviceBegin(t, h, "x", "")
	form := httptest.NewRequest(http.MethodPost, "/auth/device/approve", bytes.NewBufferString(
		`{"user_code":"`+b.UserCode+`","decision":"approve","return":"news.choir.autoradio.lab","state":"`+goodState+`"}`))
	form.Header.Set("Content-Type", "text/plain")
	form.AddCookie(&http.Cookie{Name: AccessTokenCookieName, Value: issueTestAccessJWTForHandler(priv, u.ID)})
	rec := deviceDecide(h, form)
	if rec.Code != http.StatusUnsupportedMediaType || strings.Contains(rec.Body.String(), "return_to") {
		t.Fatalf("text/plain with a return: %d %s", rec.Code, rec.Body.String())
	}
	if code, m := devicePoll(h, b.DeviceCode); code != http.StatusBadRequest || m["error"] != "authorization_pending" {
		t.Fatalf("after the refused form post: %d %v", code, m)
	}
}

func TestDeviceApproveWithoutAReturnIsAsBefore(t *testing.T) { // R6
	h, priv := testHandlerEnv(t)
	u, _ := h.store.CreateUser("plain-user", "plain@example.com")
	b := deviceBegin(t, h, "laptop", "")
	rec := deviceDecide(h, passkeyCookieReq(http.MethodPost, "/auth/device/approve", approveBody(b.UserCode, "approve", nil), priv, u.ID))
	if rec.Code != http.StatusOK {
		t.Fatalf("%d: %s", rec.Code, rec.Body.String())
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if _, has := raw["return_to"]; has || len(raw) != 2 {
		t.Fatalf("plain approve body changed: %s", rec.Body.String())
	}
	// begin and lookup do not mention a return either.
	if strings.Contains(b.VerificationURIComplete, "return") || strings.Contains(b.VerificationURIComplete, "state") {
		t.Fatalf("begin's link changed: %s", b.VerificationURIComplete)
	}
}

func TestDeviceApproveGatesHoldWithAReturn(t *testing.T) { // R7
	h, priv := testHandlerEnv(t)
	u, _ := h.store.CreateUser("gate2-user", "gate2@example.com")
	b := deviceBegin(t, h, "x", "")
	body := approveBody(b.UserCode, "approve", map[string]string{"return": "news.choir.autoradio.lab", "state": goodState})

	if rec := deviceDecide(h, jsonReq(http.MethodPost, "/auth/device/approve", body, "")); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d", rec.Code)
	}
	_, secret, _ := h.store.CreateAPIKey(t.Context(), u.ID, "full", AllScopes(), nil)
	req := jsonReq(http.MethodPost, "/auth/device/approve", body, "")
	req.Header.Set("Authorization", "Bearer "+secret)
	if rec := deviceDecide(h, req); rec.Code != http.StatusUnauthorized {
		t.Fatalf("api key: %d", rec.Code)
	}
	rec := deviceDecide(h, emailSessionCookieReq(http.MethodPost, "/auth/device/approve", body, priv, u.ID))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("email session: %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "return_to") || strings.Contains(rec.Body.String(), goodState) {
		t.Fatalf("an email session was handed a callback: %s", rec.Body.String())
	}
	// The email session decided nothing: the code is pending and a passkey session can still approve it.
	if code, m := devicePoll(h, b.DeviceCode); code != http.StatusBadRequest || m["error"] != "authorization_pending" {
		t.Fatalf("after refused sessions: %d %v", code, m)
	}
}

// R8: the allowlist is the set of apps that exist. A callback scheme is an app's id.
func TestDeviceReturnSchemesAreTheAppsInAppJSON(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "mobile", "AutoradioLab", "app.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		App      struct{ ID string } `json:"app"`
		Variants map[string]struct {
			IDSuffix string `json:"idSuffix"`
			Env      map[string]string
		} `json:"variants"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	want := []string{m.App.ID}
	for _, v := range m.Variants {
		want = append(want, m.App.ID+v.IDSuffix)
	}
	sort.Strings(want)
	got := append([]string(nil), DeviceReturnSchemes...)
	sort.Strings(got)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("DeviceReturnSchemes %v, app.json ids %v", got, want)
	}
	if DeviceReturnPath != "/signed-in" {
		t.Fatalf("path %q", DeviceReturnPath)
	}
	// Every id the app builds with is one the passkey routes accept (and the
	// apple-app-site-association file names), except the first lab app: it is
	// signed by a free team that cannot carry Associated Domains, so it can
	// never use a passkey and the app says so.
	const freeTeamLab = "news.choir.autoradio.lab"
	for _, id := range want {
		if id != freeTeamLab && !appids.Known(id) {
			t.Fatalf("app.json id %q is not in appids.PasskeyApps", id)
		}
	}
	if appids.Known(freeTeamLab) {
		t.Fatalf("%s is on a free team and cannot use passkeys; it must not be accepted", freeTeamLab)
	}
}

// R11: the exact set. The list is the three declared apps and nothing else, and
// the callback for each is its own id.
func TestDeviceReturnToIsExactForEachDeclaredAppOnly(t *testing.T) {
	for _, s := range []string{"news.choir.autoradio", "news.choir.autoradio.lab", "news.choir.autoradio.lab.one"} {
		if got, want := deviceReturnTo(s, goodState), s+":/signed-in?state="+goodState; got != want {
			t.Fatalf("%s: %q, want %q", s, got, want)
		}
	}
	for _, s := range []string{"", "news.choir.autoradio.evil", "news.choir.autoradiox", "News.Choir.Autoradio",
		"NEWS.CHOIR.AUTORADIO", "news.choir.autoradio:/signed-in", "news.choir.autoradio:", "news.choir.autoradio.lab.two",
		"news.choir.autoradio.lab.one.x", " news.choir.autoradio", "news.choir.autoradio\n", "news.choir"} {
		if got := deviceReturnTo(s, goodState); got != "" {
			t.Fatalf("%q was accepted: %q", s, got)
		}
	}
	if len(DeviceReturnSchemes) != 3 {
		t.Fatalf("DeviceReturnSchemes %v", DeviceReturnSchemes)
	}
}
