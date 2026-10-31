package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/tapefollow"
)

// Deployed acceptance for email sign-in, the per-run half
// (scripts/e2e-email-signin-staging.sh; docs/design/email-sign-in-2026-10-31.md,
// "Automated test (no person)"). It drives the Mac app's own native sign-in
// through the same local routes the renderer posts to (assetHandler ->
// handleEmailBegin / handleEmailVerify, the app's cookie jar), reads the code
// from the test account's inbox through the product mail API with the scoped
// key from the state file, then checks the session, a gated route and the
// follow sync with that session.
//
// It does nothing unless CHOIR_E2E_EMAIL_SIGNIN_STATE names the state file, so
// the ordinary test run skips it. Env: CHOIR_E2E_EMAIL_SIGNIN_STATE (state
// file), CHOIR_E2E_BACKEND (default https://choir.news), CHOIR_E2E_OUT (the
// output directory: steps.jsonl and the follow store).
//
// Nothing secret is logged: the address, code, key and cookies never reach t.Log,
// a step detail or an error string.

type e2eState struct {
	Stage      string `json:"stage"`
	AccountID  string `json:"account_id"`
	Address    string `json:"address"`
	InboxKey   string `json:"inbox_key"`
	ComputerID string `json:"computer_id"`
}

type e2eSteps struct {
	t     *testing.T
	file  *os.File
	fails int
}

func (s *e2eSteps) record(step string, ok bool, detail string) {
	status := "PASS"
	if !ok {
		status = "FAIL"
		s.fails++
	}
	line, _ := json.Marshal(map[string]any{"step": step, "ok": ok, "detail": detail})
	_, _ = s.file.Write(append(line, '\n'))
	fmt.Printf("%s %s: %s\n", status, step, detail)
}

func TestE2EEmailSignInStaging(t *testing.T) {
	statePath := os.Getenv("CHOIR_E2E_EMAIL_SIGNIN_STATE")
	if statePath == "" {
		t.Skip("CHOIR_E2E_EMAIL_SIGNIN_STATE not set")
	}
	backend := os.Getenv("CHOIR_E2E_BACKEND")
	if backend == "" {
		backend = "https://choir.news"
	}
	out := os.Getenv("CHOIR_E2E_OUT")
	if out == "" {
		out = t.TempDir()
	}
	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state file: %v", err)
	}
	var st e2eState
	if err := json.Unmarshal(raw, &st); err != nil || st.Stage != "done" || st.Address == "" || st.InboxKey == "" {
		t.Fatalf("state file is not a finished bootstrap (stage %q)", st.Stage)
	}
	f, err := os.Create(filepath.Join(out, "steps.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	steps := &e2eSteps{t: t, file: f}
	defer func() {
		if steps.fails > 0 {
			t.Fail()
		}
	}()

	session, err := newDesktopSession(backend)
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	handler := assetHandler(session)
	post := func(path string, body any) (int, map[string]any) {
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		var m map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &m)
		return rec.Code, m
	}

	mail := &e2eInbox{backend: backend, key: st.InboxKey, http: &http.Client{Timeout: 30 * time.Second}}
	seen, err := mail.ids()
	if err != nil {
		steps.record("I0", false, "the scoped key cannot read the test inbox: "+err.Error())
		return
	}
	steps.record("I0", true, fmt.Sprintf("scoped key reads the test inbox (%d earlier messages)", len(seen)))

	// B1 begin through the app's native route.
	code, body := post(desktopEmailBeginPath, map[string]string{"email": st.Address})
	attempt, _ := body["attempt_id"].(string)
	if code != http.StatusOK || attempt == "" {
		steps.record("B1", false, fmt.Sprintf("native begin answered %d", code))
		return
	}
	steps.record("B1", true, "native begin answered 200 with an attempt id")

	// M1 the newest sign-in mail, read with the scoped key.
	var signinCode string
	deadline := time.Now().Add(4 * time.Minute)
	for time.Now().Before(deadline) {
		if c, err := mail.newestCode("Your Choir sign-in code", seen); err == nil && c != "" {
			signinCode = c
			break
		}
		time.Sleep(5 * time.Second)
	}
	if signinCode == "" {
		steps.record("M1", false, "no new sign-in mail reached the test inbox within 4 minutes")
		return
	}
	steps.record("M1", true, "new sign-in mail read from the inbox; 8-digit code extracted")

	// V0 a wrong code is refused (the attempt survives one miss).
	wrong := "00000000"
	if signinCode == wrong {
		wrong = "00000001"
	}
	if code, _ := post(desktopEmailVerifyPath, map[string]string{"attempt_id": attempt, "code": wrong}); code == http.StatusBadRequest || code == http.StatusForbidden {
		steps.record("V0", true, fmt.Sprintf("a wrong code is refused (%d)", code))
	} else {
		steps.record("V0", false, fmt.Sprintf("a wrong code answered %d, want 400/403", code))
	}

	// V1 the right code signs in.
	code, body = post(desktopEmailVerifyPath, map[string]string{"attempt_id": attempt, "code": signinCode})
	if auth, _ := body["authenticated"].(bool); code != http.StatusOK || !auth {
		steps.record("V1", false, fmt.Sprintf("native verify answered %d", code))
		return
	}
	steps.record("V1", true, "native verify answered 200 authenticated")

	// V2 the code is single use.
	if code, _ := post(desktopEmailVerifyPath, map[string]string{"attempt_id": attempt, "code": signinCode}); code != http.StatusOK {
		steps.record("V2", true, fmt.Sprintf("replaying the code is refused (%d)", code))
	} else {
		steps.record("V2", false, "replaying the code answered 200")
	}

	// S1 the session: the user is the test account and the access token's
	// method claim is an email method.
	var sess struct {
		Authenticated bool `json:"authenticated"`
		User          struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	scode, sbody := session.get(t, "/auth/session")
	_ = json.Unmarshal(sbody, &sess)
	method := accessMethod(session, backend)
	if scode == http.StatusOK && sess.Authenticated && sess.User.ID == st.AccountID && strings.HasPrefix(method, "email_") {
		steps.record("S1", true, "GET /auth/session shows the test account; access token method "+method)
	} else {
		steps.record("S1", false, fmt.Sprintf("session status %d authenticated=%v same_account=%v method=%q", scode, sess.Authenticated, sess.User.ID == st.AccountID, method))
	}

	// G1 a gated route refuses the email session.
	gb, _ := json.Marshal(map[string]any{"label": "e2e must-not-exist", "scopes": []string{"read:runtime"}})
	greq, _ := http.NewRequest(http.MethodPost, session.endpoint("/auth/api-keys").String(), bytes.NewReader(gb))
	greq.Header.Set("Content-Type", "application/json")
	gresp, gerr := session.client.Do(greq)
	if gerr != nil {
		steps.record("G1", false, "request failed: "+gerr.Error())
	} else {
		io.Copy(io.Discard, io.LimitReader(gresp.Body, 1<<16))
		gresp.Body.Close()
		if gresp.StatusCode == http.StatusForbidden {
			steps.record("G1", true, "POST /auth/api-keys with the email session answered 403")
		} else {
			steps.record("G1", false, fmt.Sprintf("POST /auth/api-keys with the email session answered %d, want 403", gresp.StatusCode))
		}
	}

	// F1 the follow sync with this session, to the computer's head: the
	// desktop's own discovery and the same syncer it runs.
	sync := newDesktopSync(session)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	id, err := sync.currentComputer(ctx)
	if err != nil {
		steps.record("F1", false, "current computer: "+err.Error())
		return
	}
	if st.ComputerID != "" && id != st.ComputerID {
		steps.record("F1", false, "the session's computer is not the one the bootstrap recorded")
		return
	}
	dir := filepath.Join(out, "store", id)
	store, err := tapefollow.Open(dir, id, nil)
	if err != nil {
		steps.record("F1", false, "open store: "+err.Error())
		return
	}
	defer store.Close()
	syncer := &tapefollow.Syncer{Store: store, Client: &tapefollow.Client{Backend: session.backend.String(), HTTP: sync.client}}
	pulls := 0
	for {
		more, err := syncer.Step(ctx)
		if err != nil {
			steps.record("F1", false, fmt.Sprintf("follow step %d: %v", pulls+1, err))
			return
		}
		pulls++
		if !more {
			break
		}
	}
	vs, err := store.VerifyAll()
	if err != nil {
		steps.record("F2", false, "verify the local copy: "+err.Error())
		return
	}
	steps.record("F1", true, fmt.Sprintf("followed %s to the server head in %d pulls", id, pulls))
	steps.record("F2", true, fmt.Sprintf("local copy verifies as one tape: head_seq %d, %d bundles, %d bytes", vs.HeadSeq, len(vs.Bundles), vs.Bytes))
	if sline := syncer.Status(); sline.SourceHeadSeq > 0 && sline.HeadSeq >= sline.SourceHeadSeq {
		steps.record("F3", true, fmt.Sprintf("local head %d is the server head %d", sline.HeadSeq, sline.SourceHeadSeq))
	} else if sline.SourceHeadSeq == 0 {
		steps.record("F3", true, fmt.Sprintf("local head %d; the server reported no newer head", sline.HeadSeq))
	} else {
		steps.record("F3", false, fmt.Sprintf("local head %d behind server head %d", sline.HeadSeq, sline.SourceHeadSeq))
	}
}

// get is a small GET through the session's jar.
func (s *desktopSession) get(t *testing.T, path string) (int, []byte) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, s.endpoint(path).String(), nil)
	req.Header.Set("Accept", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return 0, nil
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, b
}

// accessMethod reads the unverified "am" claim of the jar's access cookie.
func accessMethod(s *desktopSession, backend string) string {
	u, err := url.Parse(backend)
	if err != nil {
		return ""
	}
	for _, c := range s.jar.Cookies(u) {
		if c.Name != "choir_access" {
			continue
		}
		parts := strings.Split(c.Value, ".")
		if len(parts) != 3 {
			return ""
		}
		payload, err := base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil {
			return ""
		}
		var claims struct {
			AM string `json:"am"`
		}
		_ = json.Unmarshal(payload, &claims)
		return claims.AM
	}
	return ""
}

// e2eInbox reads the test account's mail with the scoped key (Bearer, no jar).
type e2eInbox struct {
	backend string
	key     string
	http    *http.Client
}

type e2eMessage struct {
	ID        string `json:"id"`
	Subject   string `json:"subject"`
	CreatedAt string `json:"created_at"`
}

func (m *e2eInbox) get(path string, dst any) error {
	req, _ := http.NewRequest(http.MethodGet, strings.TrimRight(m.backend, "/")+path, nil)
	req.Header.Set("Authorization", "Bearer "+m.key)
	resp, err := m.http.Do(req)
	if err != nil {
		return fmt.Errorf("request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
		return fmt.Errorf("mail API answered %d", resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(dst)
}

func (m *e2eInbox) list() ([]e2eMessage, error) {
	var all []e2eMessage
	for _, folder := range []string{"inbox", "quarantine"} {
		var body struct {
			Messages []e2eMessage `json:"messages"`
		}
		if err := m.get("/api/email/messages?folder="+folder+"&limit=100", &body); err != nil {
			return nil, err
		}
		all = append(all, body.Messages...)
	}
	return all, nil
}

func (m *e2eInbox) ids() (map[string]bool, error) {
	msgs, err := m.list()
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, msg := range msgs {
		seen[msg.ID] = true
	}
	return seen, nil
}

var e2eCodePattern = regexp.MustCompile(`Your code:\s*(\d{8})`)

// newestCode is the 8-digit code of the newest unseen mail whose subject
// contains subject, or "".
func (m *e2eInbox) newestCode(subject string, seen map[string]bool) (string, error) {
	msgs, err := m.list()
	if err != nil {
		return "", err
	}
	var best *e2eMessage
	for i := range msgs {
		msg := &msgs[i]
		if seen[msg.ID] || !strings.Contains(msg.Subject, subject) {
			continue
		}
		if best == nil || msg.CreatedAt > best.CreatedAt {
			best = msg
		}
	}
	if best == nil {
		return "", nil
	}
	var detail struct {
		TextBody string `json:"text_body"`
	}
	if err := m.get("/api/email/messages/"+best.ID, &detail); err != nil {
		return "", err
	}
	if match := e2eCodePattern.FindStringSubmatch(detail.TextBody); match != nil {
		return match[1], nil
	}
	return "", nil
}
