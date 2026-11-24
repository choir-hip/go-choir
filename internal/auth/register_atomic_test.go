package auth

import (
	"encoding/json"
	"fmt"
	"github.com/go-webauthn/webauthn/webauthn"
	"net/http"
	"sync"
	"testing"
	"time"
)

// R3 (review round 2, both seats): consuming the challenge, the "no passkey
// yet unless recovery began it" check and the credential insert are one atomic
// step. Concurrent finishes cannot both win, and a read error refuses.

// finishTogether posts every request at once and returns the status codes.
func finishTogether(h http.HandlerFunc, bodies []any) []int {
	codes := make([]int, len(bodies))
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range bodies {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			codes[i] = postJSON(h, bodies[i]).Code
		}(i)
	}
	close(start)
	wg.Wait()
	return codes
}

func countOf(codes []int, want int) int {
	n := 0
	for _, c := range codes {
		if c == want {
			n++
		}
	}
	return n
}

// Two live register/begin challenges for one address, both finished at once on
// the app route: one account, one passkey, one key, one 200 and the rest 409.
func TestConcurrentAppRegisterFinishesAdmitExactlyOne(t *testing.T) {
	h := appEnv(t)
	for round := 0; round < 25; round++ {
		email := fmt.Sprintf("race%d@example.com", round)
		var bodies []any
		for i := 0; i < 4; i++ {
			begin := decodeMap(t, registerBeginFor(t, h, email))
			bodies = append(bodies, appFinishBody(testApp, newSoftPasskey(t).create(t, begin, testOrigin)))
		}
		codes := finishTogether(h.HandleAppRegisterFinish, bodies)
		user, _ := h.store.GetUserByEmail(email)
		creds, _ := h.store.GetCredentialsByUserID(user.ID)
		if countOf(codes, 200) != 1 || len(creds) != 1 || countKeys(t, h, user.ID) != 1 {
			t.Fatalf("round %d: codes %v, credentials %d, keys %d; want one 200, one credential, one key",
				round, codes, len(creds), countKeys(t, h, user.ID))
		}
	}
}

// The web finish is the same step.
func TestConcurrentWebRegisterFinishesAdmitExactlyOne(t *testing.T) {
	h := appEnv(t)
	for round := 0; round < 25; round++ {
		email := fmt.Sprintf("webrace%d@example.com", round)
		var bodies []any
		for i := 0; i < 4; i++ {
			begin := decodeMap(t, registerBeginFor(t, h, email))
			bodies = append(bodies, rawJSON(newSoftPasskey(t).create(t, begin, testOrigin)))
		}
		codes := finishTogether(h.HandleRegisterFinish, bodies)
		user, _ := h.store.GetUserByEmail(email)
		creds, _ := h.store.GetCredentialsByUserID(user.ID)
		if countOf(codes, 200) != 1 || len(creds) != 1 {
			t.Fatalf("round %d: codes %v, credentials %d; want one 200 and one credential", round, codes, len(creds))
		}
	}
}

// One challenge answered twice at once (a recovery challenge, which may add a
// passkey to an account that has one): single use, so exactly one finish wins.
func TestOneChallengeFinishedTwiceAtOnceSucceedsOnce(t *testing.T) {
	h := appEnv(t)
	for round := 0; round < 25; round++ {
		email := fmt.Sprintf("once%d@example.com", round)
		if rec := createAccount(t, h, email, newSoftPasskey(t), testOrigin); rec.Code != 200 {
			t.Fatal(rec.Body.String())
		}
		user, _ := h.store.GetUserByEmail(email)
		opts := saveRecoveryChallenge(t, h, user, ChallengeSourceRecovery)
		bodies := []any{
			rawJSON(newSoftPasskey(t).create(t, opts, testOrigin)),
			rawJSON(newSoftPasskey(t).create(t, opts, testOrigin)),
			rawJSON(newSoftPasskey(t).create(t, opts, testOrigin)),
		}
		codes := finishTogether(h.HandleRegisterFinish, bodies)
		creds, _ := h.store.GetCredentialsByUserID(user.ID)
		if countOf(codes, 200) != 1 || len(creds) != 2 {
			t.Fatalf("round %d: codes %v, credentials %d; want one 200 and two credentials", round, codes, len(creds))
		}
	}
}

// A read error on the credentials is a refusal, never "no passkey yet": a
// recovery finish whose account's credentials cannot be read answers 500 and
// adds nothing.
func TestRecoveryFinishRefusesWhenTheCredentialsCannotBeRead(t *testing.T) {
	h := appEnv(t)
	email := "unreadable@example.com"
	if rec := createAccount(t, h, email, newSoftPasskey(t), testOrigin); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	user, _ := h.store.GetUserByEmail(email)
	begin := saveRecoveryChallenge(t, h, user, ChallengeSourceRecovery)
	body := rawJSON(newSoftPasskey(t).create(t, begin, testOrigin))
	// Hide the credentials behind a view of every column in which one value
	// cannot be scanned (a text where an integer is read), so the read fails and
	// an insert still goes through to the real table.
	for _, q := range []string{
		`ALTER TABLE credentials RENAME TO credentials_real`,
		`CREATE VIEW credentials AS SELECT id, user_id, public_key, attestation_type, transport, 'bad' AS sign_count, aaguid, flags, name, prf_capable, prf_salt, created_at, last_used_at FROM credentials_real`,
		`CREATE TRIGGER credentials_ins INSTEAD OF INSERT ON credentials BEGIN
			INSERT INTO credentials_real (id, user_id, public_key, attestation_type, transport, sign_count, aaguid, flags, name, created_at)
			VALUES (NEW.id, NEW.user_id, NEW.public_key, NEW.attestation_type, NEW.transport, 0, NEW.aaguid, NEW.flags, NEW.name, NEW.created_at); END`,
	} {
		if _, err := h.store.DB().Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	rec := postJSON(h.HandleRegisterFinish, body)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("finish with unreadable credentials = %d, want 500: %s", rec.Code, rec.Body.String())
	}
	var n int
	if err := h.store.DB().QueryRow(`SELECT COUNT(*) FROM credentials_real WHERE user_id = ?`, user.ID).Scan(&n); err != nil || n != 1 {
		t.Fatalf("credentials = %d (%v); want the one that was there", n, err)
	}
}

func beginOptionsFor(t *testing.T, h *Handler, user *User, source string) any {
	t.Helper()
	waUser, err := newWebAuthnUser(user, nil)
	if err != nil {
		t.Fatal(err)
	}
	creation, session, err := h.webauthn.BeginRegistration(waUser)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := jsonMarshal(session)
	now := time.Now().UTC()
	if err := h.store.SaveChallengeState(&ChallengeState{
		ID: session.Challenge, UserID: user.ID, Challenge: session.Challenge, Type: "registration",
		Source: source, WebAuthnSessionData: string(raw), CreatedAt: now, ExpiresAt: now.Add(SessionChallengeTTL),
	}); err != nil {
		t.Fatal(err)
	}
	return creation
}

// saveRecoveryChallenge stores a challenge of the given source for user and
// returns its options.
func saveRecoveryChallenge(t *testing.T, h *Handler, user *User, source string) map[string]any {
	t.Helper()
	return decodeMap(t, jsonRecorder(t, beginOptionsFor(t, h, user, source)))
}

func rawJSON(b []byte) json.RawMessage { return json.RawMessage(b) }

// A stranger cannot grow the app-login challenge table without bound, a whole
// IPv6 /64 is one source, and the ceiling never evicts a fresh challenge for an
// older one.
func TestAppChallengeTableIsBoundedAndNeverOutlivesItsCeiling(t *testing.T) {
	var c appChallenges
	now := timeNow()
	// One host walking addresses inside its /64 holds eight challenges.
	for i := 0; i < 500; i++ {
		c.put(webauthn.SessionData{Challenge: fmt.Sprintf("v6-%d", i)}, fmt.Sprintf("2001:db8:1:2::%x", i+1), now)
	}
	if n := len(c.bySource["2001:db8:1:2::/64"]); n != appChallengePerAddr || len(c.m) != appChallengePerAddr {
		t.Fatalf("a /64 holds %d challenges (table %d), want %d", n, len(c.m), appChallengePerAddr)
	}
	// Many distinct sources fill the ceiling and no further.
	for i := 0; i < appChallengeMax+50; i++ {
		c.put(webauthn.SessionData{Challenge: fmt.Sprintf("c-%d", i)}, fmt.Sprintf("10.%d.%d.%d", i>>16&255, i>>8&255, i&255), now)
	}
	if len(c.m) != appChallengeMax || c.order.Len() != appChallengeMax {
		t.Fatalf("table = %d (queue %d), want ceiling %d", len(c.m), c.order.Len(), appChallengeMax)
	}
	if _, ok := c.take(fmt.Sprintf("c-%d", appChallengeMax+49), now); !ok {
		t.Fatal("the newest challenge was dropped at the ceiling")
	}
	if _, ok := c.take("c-0", now); ok {
		t.Fatal("the oldest challenge survived the ceiling")
	}
	// Time empties it from the front.
	c.put(webauthn.SessionData{Challenge: "late"}, "192.0.2.1", now.Add(2*SessionChallengeTTL))
	if len(c.m) != 1 {
		t.Fatalf("expired challenges kept: %d", len(c.m))
	}
}
