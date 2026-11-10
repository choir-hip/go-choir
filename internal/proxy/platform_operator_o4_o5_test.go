package proxy

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// The platform operator's schedule (O4) and restart (O5) acts at the proxy
// (docs/design/platform-operator-account-2026-11-06.md section 9: S1, S2, S6,
// X1 to X7, X9). The world is the one in platform_operator_computers_test.go.

func scheduleURL(id, query string) string {
	u := operatorPrefix + id + "/guest" + schedulesGuest
	if query != "" {
		u += "?" + query
	}
	return u
}

func restartURL(id string) string { return operatorPrefix + id + "/restart" }

func (w *operatorWorld) vmctlSaw(path string) int {
	w.mu.Lock()
	defer w.mu.Unlock()
	n := 0
	for _, p := range w.vmctlPaths {
		if p == path {
			n++
		}
	}
	return n
}

func (w *operatorWorld) lastGuest() guestSeen {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.guestCalls[len(w.guestCalls)-1]
}

// S1 S2: only the list, a start with the four fields and a stop with source_id
// and reason are forwarded; the forwarded start body is rebuilt.
func TestOperatorScheduleForwardsOnlyTheThreeActs(t *testing.T) {
	w := newOperatorWorld(t, opAccount)
	s := w.session(opAccount)
	if rec := w.do(http.MethodGet, scheduleURL(paperComputer, ""), "", s); rec.Code != http.StatusOK {
		t.Fatalf("list = %d %s", rec.Code, rec.Body)
	}
	// A start: the forwarded body is the parsed fields, not the client's bytes.
	rec := w.do(http.MethodPost, scheduleURL(paperComputer, ""), `{"source_id":"rss:a","interval_minutes":30,"max_items":4,"reason":"slow"}`, s)
	if rec.Code != http.StatusOK {
		t.Fatalf("start = %d %s", rec.Code, rec.Body)
	}
	last := w.lastGuest()
	if last.Method != http.MethodPost || last.Path != schedulesGuest || last.Header.Get("X-Choir-Platform-Operator") != opAccount {
		t.Fatalf("forwarded %+v", last)
	}
	var fwd map[string]any
	if err := json.Unmarshal([]byte(last.Body), &fwd); err != nil || fwd["source_id"] != "rss:a" || len(fwd) != 4 {
		t.Fatalf("forwarded body %q", last.Body)
	}
	// A stop.
	if rec := w.do(http.MethodDelete, scheduleURL(paperComputer, "source_id=rss%3Aa&reason=paused"), "", s); rec.Code != http.StatusOK {
		t.Fatalf("stop = %d %s", rec.Code, rec.Body)
	}
	before := w.guestCount()
	for name, c := range map[string]struct{ method, url, body string }{
		"feed_url":              {http.MethodPost, scheduleURL(paperComputer, ""), `{"feed_url":"https://x.example/f.xml"}`},
		"fixture_run":           {http.MethodPost, scheduleURL(paperComputer, ""), `{"source_id":"rss:a","fixture_run":true}`},
		"unknown field":         {http.MethodPost, scheduleURL(paperComputer, ""), `{"source_id":"rss:a","x":1}`},
		"two documents":         {http.MethodPost, scheduleURL(paperComputer, ""), `{"source_id":"rss:a"}{"feed_url":"u"}`},
		"no source":             {http.MethodPost, scheduleURL(paperComputer, ""), `{}`},
		"long reason":           {http.MethodPost, scheduleURL(paperComputer, ""), `{"source_id":"rss:a","reason":"` + strings.Repeat("x", 300) + `"}`},
		"stop with extra query": {http.MethodDelete, scheduleURL(paperComputer, "source_id=rss%3Aa&computer_id=x"), ""},
		"stop with no source":   {http.MethodDelete, scheduleURL(paperComputer, "reason=r"), ""},
		"list with a query":     {http.MethodGet, scheduleURL(paperComputer, "owner=x"), ""},
		"PUT":                   {http.MethodPut, scheduleURL(paperComputer, ""), `{"source_id":"rss:a"}`},
		"PATCH":                 {http.MethodPatch, scheduleURL(paperComputer, ""), `{}`},
		"trailing path":         {http.MethodPost, scheduleURL(paperComputer, "") + "/rss:a", `{"source_id":"rss:a"}`},
	} {
		rec := w.do(c.method, c.url, c.body, s)
		if rec.Code < 400 {
			t.Fatalf("%s was forwarded: %d %s", name, rec.Code, rec.Body)
		}
	}
	if w.guestCount() != before {
		t.Fatalf("a refused schedule request reached the guest (%d -> %d)", before, w.guestCount())
	}
}

// S6: a guest that does not record operator receipts is never changed.
func TestOperatorScheduleWriteRefusedOnAGuestThatDoesNotRecordReceipts(t *testing.T) {
	w := newOperatorWorld(t, opAccount)
	w.oldGuest = true
	s := w.session(opAccount)
	if rec := w.do(http.MethodPost, scheduleURL(paperComputer, ""), `{"source_id":"rss:a"}`, s); rec.Code != http.StatusConflict {
		t.Fatalf("start on an old guest = %d %s", rec.Code, rec.Body)
	}
	if rec := w.do(http.MethodDelete, scheduleURL(paperComputer, "source_id=rss%3Aa"), "", s); rec.Code != http.StatusConflict {
		t.Fatalf("stop on an old guest = %d %s", rec.Code, rec.Body)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, c := range w.guestCalls {
		if c.Method != http.MethodGet {
			t.Fatalf("a write reached an old guest: %+v", c)
		}
	}
}

// A9 for schedules: a read key lists and cannot start or stop.
func TestOperatorScheduleKeepsTheKeysScopes(t *testing.T) {
	w := newOperatorWorld(t, opAccount)
	if rec := w.do(http.MethodGet, scheduleURL(paperComputer, ""), "", bearer("choir_sk_ro")); rec.Code != http.StatusOK {
		t.Fatalf("read key list = %d", rec.Code)
	}
	before := w.guestCount()
	if rec := w.do(http.MethodPost, scheduleURL(paperComputer, ""), `{"source_id":"rss:a"}`, bearer("choir_sk_ro")); rec.Code != http.StatusForbidden {
		t.Fatalf("read key start = %d", rec.Code)
	}
	if rec := w.do(http.MethodDelete, scheduleURL(paperComputer, "source_id=rss%3Aa"), "", bearer("choir_sk_ro")); rec.Code != http.StatusForbidden {
		t.Fatalf("read key stop = %d", rec.Code)
	}
	if w.guestCount() != before {
		t.Fatal("a read key's schedule write reached the guest")
	}
}

// X1: the one gate. Nothing reaches vmctl's refresh or the guest on a refusal;
// the route takes POST and a reason.
func TestOperatorRestartAdmissionIsTheGate(t *testing.T) {
	w := newOperatorWorld(t, opAccount)
	body := `{"reason":"pick up the policy"}`
	for name, c := range map[string]struct {
		url    string
		mutate func(*http.Request)
		want   int
	}{
		"anonymous":           {restartURL(paperComputer), nil, http.StatusUnauthorized},
		"ordinary session":    {restartURL(paperComputer), w.session(otherAccount), http.StatusForbidden},
		"ordinary admin key":  {restartURL(paperComputer), bearer("choir_sk_other"), http.StatusForbidden},
		"computer-bound key":  {restartURL(paperComputer), bearer("choir_sk_bound"), http.StatusForbidden},
		"read key":            {restartURL(paperComputer), bearer("choir_sk_ro"), http.StatusForbidden},
		"user computer":       {restartURL(userComputer), w.session(opAccount), http.StatusForbidden},
		"test tier computer":  {restartURL(testTierID), w.session(opAccount), http.StatusForbidden},
		"upper case id":       {restartURL(strings.ToUpper(paperComputer)), w.session(opAccount), http.StatusForbidden},
		"owner's id":          {restartURL(paperOwner), w.session(opAccount), http.StatusForbidden},
		"extra path":          {restartURL(paperComputer) + "/now", w.session(opAccount), http.StatusNotFound},
		"guest-segment alias": {operatorPrefix + paperComputer + "/guest/restart", w.session(opAccount), http.StatusForbidden},
	} {
		rec := w.do(http.MethodPost, c.url, body, c.mutate)
		if rec.Code != c.want {
			t.Errorf("%s = %d %s, want %d", name, rec.Code, rec.Body, c.want)
		}
	}
	if rec := w.do(http.MethodGet, restartURL(paperComputer), "", w.session(opAccount)); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET restart = %d", rec.Code)
	}
	if w.vmctlSaw("/internal/vmctl/deploy-refresh") != 0 || w.guestCount() != 0 {
		t.Fatal("a refused restart reached vmctl's refresh or the guest")
	}
}

// X1: a restart needs a reason, bounded, and no other field.
func TestOperatorRestartNeedsAReason(t *testing.T) {
	w := newOperatorWorld(t, opAccount)
	s := w.session(opAccount)
	for _, body := range []string{``, `{}`, `{"reason":""}`, `{"reason":"   "}`, `{"reason":"` + strings.Repeat("x", 300) + `"}`,
		`{"reason":"r","force":true}`, `{"reason":"r"}{"reason":"s"}`, `not json`} {
		if rec := w.do(http.MethodPost, restartURL(paperComputer), body, s); rec.Code != http.StatusBadRequest {
			t.Errorf("body %.40q = %d %s", body, rec.Code, rec.Body)
		}
	}
	if w.vmctlSaw("/internal/vmctl/deploy-refresh") != 0 || w.guestCount() != 0 {
		t.Fatal("a restart with no usable reason reached vmctl or the guest")
	}
}

// X2 X5 X7: the receipt is asked first, then the planned owner update, and no
// other vmctl endpoint (stop, start, restart, unhold) is ever called.
func TestOperatorRestartIsThePlannedOwnerUpdate(t *testing.T) {
	w := newOperatorWorld(t, opAccount)
	rec := w.do(http.MethodPost, restartURL(paperComputer), `{"reason":"pick up the policy"}`, w.session(opAccount))
	if rec.Code != http.StatusOK {
		t.Fatalf("restart = %d %s", rec.Code, rec.Body)
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out["outcome"] != "refreshed" || out["receipt"] != "evt-restart-1" {
		t.Fatalf("answer = %s", rec.Body)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	refreshes := 0
	for _, p := range w.vmctlPaths {
		switch {
		case p == "/internal/vmctl/deploy-refresh":
			refreshes++
		case strings.Contains(p, "lookup") || strings.Contains(p, "ownership") || strings.Contains(p, "computers"):
			// the roster lookup the gate makes
		default:
			t.Errorf("an operator restart called vmctl %s", p)
		}
	}
	if refreshes != 1 {
		t.Fatalf("deploy-refresh calls = %d, want 1", refreshes)
	}
	var receiptCall *guestSeen
	for i := range w.guestCalls {
		if w.guestCalls[i].Path == "/api/platform/operator/restart-receipt" {
			receiptCall = &w.guestCalls[i]
		}
	}
	if receiptCall == nil || receiptCall.Method != http.MethodPost || receiptCall.Header.Get("X-Choir-Platform-Operator") != opAccount ||
		!strings.Contains(receiptCall.Body, "pick up the policy") {
		t.Fatalf("receipt call = %+v", receiptCall)
	}
}

// X3: an outcome that restarted nothing is returned with its code, and there is
// no force.
func TestOperatorRestartRefusalsAreReturnedWithTheirCodes(t *testing.T) {
	cases := []struct {
		name, body string
		code       int
		want       int
		wantCode   string
	}{
		{"current", `{"outcome":"current","target":"abc"}`, 200, http.StatusOK, ""},
		{"live capsule", `{"outcome":"deferred","reason":"busy","reason_code":"live_capsule","refusal":{"live_capsules":1}}`, 409, http.StatusConflict, "live_capsule"},
		{"selfdev active", `{"outcome":"deferred","reason":"busy","reason_code":"selfdev_active","refusal":{"selfdev_active_operations":1}}`, 409, http.StatusConflict, "selfdev_active"},
		{"awaiting decision", `{"outcome":"deferred","reason":"busy","reason_code":"selfdev_awaiting_decision"}`, 409, http.StatusConflict, "selfdev_awaiting_decision"},
		{"held", `{"outcome":"skipped","reason":"held","reason_code":"held"}`, 409, http.StatusConflict, "held"},
		{"ring waits", `{"outcome":"skipped","reason":"ring","reason_code":"ring_waits"}`, 409, http.StatusConflict, "ring_waits"},
		{"in flight", `{"outcome":"deferred","reason":"refresh in flight","reason_code":"refresh_in_flight"}`, 409, http.StatusConflict, "refresh_in_flight"},
		{"failed", `{"outcome":"failed","reason":"boot","reason_code":"failed"}`, 500, http.StatusInternalServerError, "failed"},
	}
	for _, c := range cases {
		w := newOperatorWorld(t, opAccount)
		w.refreshCode, w.refreshBody = c.code, c.body
		rec := w.do(http.MethodPost, restartURL(paperComputer), `{"reason":"r"}`, w.session(opAccount))
		if rec.Code != c.want {
			t.Errorf("%s = %d %s, want %d", c.name, rec.Code, rec.Body, c.want)
			continue
		}
		var out map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		if c.wantCode != "" && out["reason_code"] != c.wantCode {
			t.Errorf("%s: reason_code = %v in %s", c.name, out["reason_code"], rec.Body)
		}
		if out["receipt"] != "evt-restart-1" {
			t.Errorf("%s: the answer does not carry the receipt: %s", c.name, rec.Body)
		}
		if w.vmctlSaw("/internal/vmctl/deploy-refresh") != 1 {
			t.Errorf("%s: deploy-refresh calls = %d, want exactly 1 (no retry, no force)", c.name, w.vmctlSaw("/internal/vmctl/deploy-refresh"))
		}
	}
}

// X5: no receipt, no restart.
func TestOperatorRestartNeedsItsReceiptFirst(t *testing.T) {
	w := newOperatorWorld(t, opAccount)
	w.receiptCode = http.StatusInternalServerError
	rec := w.do(http.MethodPost, restartURL(paperComputer), `{"reason":"r"}`, w.session(opAccount))
	if rec.Code < 400 {
		t.Fatalf("restart with a failing receipt = %d %s", rec.Code, rec.Body)
	}
	if w.vmctlSaw("/internal/vmctl/deploy-refresh") != 0 {
		t.Fatal("vmctl was asked to restart with no receipt")
	}
}

// X5: an old guest that does not record receipts is not restarted.
func TestOperatorRestartRefusedOnAGuestThatDoesNotRecordReceipts(t *testing.T) {
	w := newOperatorWorld(t, opAccount)
	w.oldGuest = true
	rec := w.do(http.MethodPost, restartURL(paperComputer), `{"reason":"r"}`, w.session(opAccount))
	if rec.Code != http.StatusConflict {
		t.Fatalf("restart on an old guest = %d %s", rec.Code, rec.Body)
	}
	if w.vmctlSaw("/internal/vmctl/deploy-refresh") != 0 {
		t.Fatal("an old guest was restarted with no receipt")
	}
}

// X6: a computer that is not active is not woken to be restarted.
func TestOperatorRestartDoesNotWakeAStoppedComputer(t *testing.T) {
	w := newOperatorWorld(t, opAccount)
	w.state = "stopped"
	rec := w.do(http.MethodPost, restartURL(paperComputer), `{"reason":"r"}`, w.session(opAccount))
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "not_active") {
		t.Fatalf("restart of a stopped computer = %d %s", rec.Code, rec.Body)
	}
	if w.vmctlSaw("/internal/vmctl/deploy-refresh") != 0 || w.guestCount() != 0 {
		t.Fatal("a stopped computer was woken or refreshed")
	}
}

// The proof computer: the operator reaches d@e (the roster's operator_proof_computers
// entry) with the same gate and receipts, and still nothing else.
func TestOperatorReachesTheProofComputerAndNoOrdinaryComputer(t *testing.T) {
	const proof = "computer-59c902c5d46caecc0a180a8557164e72"
	w := newOperatorWorld(t, opAccount)
	w.extraComputers = map[string]bool{proof: true}
	s := w.session(opAccount)
	if rec := w.do(http.MethodGet, scheduleURL(proof, ""), "", s); rec.Code != http.StatusOK {
		t.Fatalf("list on the proof computer = %d %s", rec.Code, rec.Body)
	}
	if rec := w.do(http.MethodPost, restartURL(proof), `{"reason":"operator verb proof"}`, s); rec.Code != http.StatusOK {
		t.Fatalf("restart on the proof computer = %d %s", rec.Code, rec.Body)
	}
	w.extraComputers[userComputer] = true
	if rec := w.do(http.MethodGet, scheduleURL(userComputer, ""), "", s); rec.Code != http.StatusForbidden {
		t.Fatalf("an ordinary computer = %d %s", rec.Code, rec.Body)
	}
	// The default `status` roster stays the paper alone.
	rec := w.do(http.MethodGet, platformOperatorComputersPath, "", s)
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), proof) {
		t.Fatalf("status lists the proof computer: %d %s", rec.Code, rec.Body)
	}
}
