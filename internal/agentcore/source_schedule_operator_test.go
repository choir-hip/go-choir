package agentcore

// The platform operator's schedule acts on the guest
// (docs/design/platform-operator-account-2026-11-06.md section 9, items S1 to
// S9). Ways it could fail, written before the code:
//   - a start or stop lands with no receipt, or the receipt is written after;
//   - a receipt that cannot be recorded still lets the schedule change;
//   - the marker is believed in-process (a desk could forge a receipt naming
//     an operator) or with a second value;
//   - a marked request adds a feed or a fixture schedule, which are the
//     owner's own acts;
//   - the receipt carries an email, or a key the store's vocabulary guard
//     refuses;
//   - a list or an ordinary owner act grows a receipt.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const opScheduleAccount = "1111aaaa-2222-3333-4444-55555555beef"

func operatorScheduleRequest(method, target, body string) *http.Request {
	req := ownerScheduleRequest(method, target, body)
	req.Header.Set(server.PlatformOperatorHeader, opScheduleAccount)
	return req
}

func scheduleReceipts(t *testing.T, rt *Runtime) []types.EventRecord {
	t.Helper()
	evs, err := rt.store.ListEventsByOwner(context.Background(), "owner-stamp", 200)
	if err != nil {
		t.Fatal(err)
	}
	var out []types.EventRecord
	for _, ev := range evs {
		if ev.Kind == types.EventPlatformComputerSchedule {
			out = append(out, ev)
		}
	}
	return out
}

func scheduleExists(rt *Runtime, source string) bool {
	_, err := rt.store.GetSourceSchedule(scheduleCustodian(context.Background()), "owner-stamp", source)
	return err == nil
}

// S3 S7: the receipt is on the tape before the schedule exists, names the
// account, and the answer carries its id.
func TestOperatorScheduleActIsRecordedBeforeItLands(t *testing.T) {
	rt, _, _, _ := sourceScheduleFixture(t, "rss:op-a", nil)
	h := &APIHandler{rt: rt}
	recorded := 0
	h.operatorReceipt = func(ctx context.Context, ownerID string, kind types.EventKind, payload map[string]any) (string, error) {
		recorded++
		if scheduleExists(rt, "rss:op-a") && payload["act"] == "schedule.start" {
			t.Error("the receipt was recorded after the schedule landed")
		}
		return rt.recordOperatorReceipt(ctx, ownerID, kind, payload)
	}
	rec := httptest.NewRecorder()
	h.HandleSourceSchedules(rec, operatorScheduleRequest(http.MethodPost, "/api/autopaper/sources/schedules", `{"source_id":"rss:op-a","interval_minutes":30,"reason":"slow the feed"}`))
	if rec.Code != http.StatusOK || recorded != 1 {
		t.Fatalf("operator start = %d %s (receipts %d)", rec.Code, rec.Body, recorded)
	}
	if rec.Header().Get("X-Choir-Operator-Receipts") != "1" {
		t.Fatal("the guest did not say it records operator receipts")
	}
	evs := scheduleReceipts(t, rt)
	if len(evs) != 1 {
		t.Fatalf("receipts = %d", len(evs))
	}
	if !strings.Contains(rec.Body.String(), evs[0].EventID) {
		t.Fatalf("the answer does not name the receipt %s: %s", evs[0].EventID, rec.Body)
	}
	p := string(evs[0].Payload)
	for _, want := range []string{opScheduleAccount, "schedule.start", "rss:op-a", "slow the feed"} {
		if !strings.Contains(p, want) {
			t.Fatalf("receipt lacks %q: %s", want, p)
		}
	}
	if strings.Contains(p, "@") {
		t.Fatalf("receipt carries an address-like value: %s", p)
	}
	for _, banned := range []string{`"role"`, `"profile"`, `"agent_profile"`, `"agent_role"`, `"authority_profile"`, `"requested_by_profile"`, `"actor_profile"`, `"target_profile"`} {
		if strings.Contains(p, banned) {
			t.Fatalf("receipt uses the out-of-vocabulary key %s", banned)
		}
	}
	// The stop leaves its own receipt.
	rec = httptest.NewRecorder()
	h.HandleSourceSchedules(rec, operatorScheduleRequest(http.MethodDelete, "/api/autopaper/sources/schedules?source_id=rss:op-a&reason=paused", ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("operator stop = %d %s", rec.Code, rec.Body)
	}
	if evs := scheduleReceipts(t, rt); len(evs) != 2 {
		t.Fatalf("receipts after the stop = %d", len(evs))
	}
	if s := getStoredSchedule(t, rt, "rss:op-a"); s.State != store.SourceScheduleStopped {
		t.Fatalf("schedule state = %s", s.State)
	}
}

// S3: a receipt that cannot be recorded leaves the schedule untouched.
func TestOperatorScheduleActNeedsItsReceipt(t *testing.T) {
	rt, _, _, _ := sourceScheduleFixture(t, "rss:op-b", nil)
	h := &APIHandler{rt: rt}
	h.operatorReceipt = func(context.Context, string, types.EventKind, map[string]any) (string, error) {
		return "", errors.New("tape unavailable")
	}
	rec := httptest.NewRecorder()
	h.HandleSourceSchedules(rec, operatorScheduleRequest(http.MethodPost, "/api/autopaper/sources/schedules", `{"source_id":"rss:op-b"}`))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("start with no receipt = %d %s", rec.Code, rec.Body)
	}
	if scheduleExists(rt, "rss:op-b") {
		t.Fatal("the schedule landed with no receipt")
	}
}

// S1: a marked request adds neither a feed nor a fixture schedule.
func TestOperatorScheduleRefusesFeedURLAndFixture(t *testing.T) {
	rt, _, _, _ := sourceScheduleFixture(t, "rss:op-c", nil)
	h := &APIHandler{rt: rt}
	for _, body := range []string{
		`{"feed_url":"https://news.example/feed.xml"}`,
		`{"source_id":"rss:op-c","fixture_run":true}`,
		`{"source_id":"rss:op-c","extra":1}`,
	} {
		rec := httptest.NewRecorder()
		h.HandleSourceSchedules(rec, operatorScheduleRequest(http.MethodPost, "/api/autopaper/sources/schedules", body))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s = %d %s", body, rec.Code, rec.Body)
		}
	}
	if n := len(scheduleReceipts(t, rt)); n != 0 {
		t.Fatalf("a refused request left %d receipts", n)
	}
	if scheduleExists(rt, "rss:op-c") {
		t.Fatal("a refused request started a schedule")
	}
}

// S4: a list and an ordinary owner act grow no receipt.
func TestOperatorScheduleListAndOwnerActsGrowNoReceipt(t *testing.T) {
	rt, _, _, _ := sourceScheduleFixture(t, "rss:op-d", nil)
	h := &APIHandler{rt: rt}
	rec := httptest.NewRecorder()
	h.HandleSourceSchedules(rec, operatorScheduleRequest(http.MethodGet, "/api/autopaper/sources/schedules", ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("operator list = %d %s", rec.Code, rec.Body)
	}
	rec = httptest.NewRecorder()
	h.HandleSourceSchedules(rec, ownerScheduleRequest(http.MethodPost, "/api/autopaper/sources/schedules", `{"source_id":"rss:op-d"}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("owner start = %d %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("X-Choir-Operator-Receipts") != "" {
		t.Fatal("an owner act claimed operator receipts")
	}
	if n := len(scheduleReceipts(t, rt)); n != 0 {
		t.Fatalf("receipts = %d, want none", n)
	}
}

// S5: the marker is not believed in-process, or with a second value.
func TestOperatorScheduleMarkerIsNotBelievedInProcess(t *testing.T) {
	rt, _, _, _ := sourceScheduleFixture(t, "rss:op-e", nil)
	h := &APIHandler{rt: rt}
	req := operatorScheduleRequest(http.MethodPost, "/api/autopaper/sources/schedules", `{"source_id":"rss:op-e"}`)
	req.RemoteAddr = "" // a desk's in-process ProductAPI call
	rec := httptest.NewRecorder()
	h.HandleSourceSchedules(rec, req)
	if rec.Code != http.StatusForbidden && rec.Code != http.StatusUnauthorized {
		t.Fatalf("in-process marked start = %d %s", rec.Code, rec.Body)
	}
	req = operatorScheduleRequest(http.MethodPost, "/api/autopaper/sources/schedules", `{"source_id":"rss:op-e"}`)
	req.Header.Add(server.PlatformOperatorHeader, opScheduleAccount)
	rec = httptest.NewRecorder()
	h.HandleSourceSchedules(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("two-valued marker = %d %s", rec.Code, rec.Body)
	}
	if scheduleExists(rt, "rss:op-e") {
		t.Fatal("a refused marker still started a schedule")
	}
}

// S8: a stop of an unknown source says so, and the answer is a 404.
func TestOperatorScheduleStopOfUnknownSource(t *testing.T) {
	rt, _, _, _ := sourceScheduleFixture(t, "rss:op-f", nil)
	h := &APIHandler{rt: rt}
	rec := httptest.NewRecorder()
	h.HandleSourceSchedules(rec, operatorScheduleRequest(http.MethodDelete, "/api/autopaper/sources/schedules?source_id=rss:nope", ""))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("stop of unknown = %d %s", rec.Code, rec.Body)
	}
}

// S7: the event kind is accepted by the store.
func TestPlatformOperatorScheduleReceiptEventIsAppended(t *testing.T) {
	rt, _ := testAPISetup(t)
	rec, err := rt.EmitProductEvent(context.Background(), "paper-owner", "primary", types.EventPlatformComputerSchedule, map[string]any{
		"operator_account_id": opScheduleAccount, "act": "schedule.start", "source_id": "rss:x", "interval_minutes": 30, "max_items": 5, "reason": "r",
	})
	if err != nil || rec.Kind != types.EventPlatformComputerSchedule {
		t.Fatalf("event = %+v err %v", rec, err)
	}
}

// X5: the restart receipt route records only for the host peer's operator
// marker, needs a reason, and answers with the receipt id.
func TestRestartReceiptRouteBelievesOnlyTheHostMarker(t *testing.T) {
	rt, _, _, _ := sourceScheduleFixture(t, "rss:op-g", nil)
	h := &APIHandler{rt: rt}
	post := func(mark bool, body string, mutate func(*http.Request)) *httptest.ResponseRecorder {
		req := ownerScheduleRequest(http.MethodPost, OperatorRestartReceiptRoute, body)
		if mark {
			req.Header.Set(server.PlatformOperatorHeader, opScheduleAccount)
		}
		if mutate != nil {
			mutate(req)
		}
		rec := httptest.NewRecorder()
		h.HandleOperatorRestartReceipt(rec, req)
		return rec
	}
	rec := post(true, `{"reason":"pick up the policy"}`, nil)
	if rec.Code != http.StatusOK || rec.Header().Get("X-Choir-Operator-Receipts") != "1" {
		t.Fatalf("operator receipt = %d %s", rec.Code, rec.Body)
	}
	var out struct {
		Receipt string `json:"receipt"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.Receipt == "" {
		t.Fatalf("answer = %s", rec.Body)
	}
	evs, _ := rt.store.ListEventsByOwner(context.Background(), "owner-stamp", 50)
	found := false
	for _, ev := range evs {
		if ev.Kind == types.EventPlatformComputerRestart && ev.EventID == out.Receipt && strings.Contains(string(ev.Payload), opScheduleAccount) {
			found = true
		}
	}
	if !found {
		t.Fatal("no restart receipt on the tape")
	}
	for name, got := range map[string]*httptest.ResponseRecorder{
		"no marker":      post(false, `{"reason":"r"}`, nil),
		"in process":     post(true, `{"reason":"r"}`, func(r *http.Request) { r.RemoteAddr = "" }),
		"no reason":      post(true, `{}`, nil),
		"extra field":    post(true, `{"reason":"r","force":true}`, nil),
		"second marker":  post(true, `{"reason":"r"}`, func(r *http.Request) { r.Header.Add(server.PlatformOperatorHeader, opScheduleAccount) }),
		"malformed mark": post(true, `{"reason":"r"}`, func(r *http.Request) { r.Header.Set(server.PlatformOperatorHeader, "someone@example.com") }),
	} {
		if got.Code < 400 {
			t.Errorf("%s = %d %s", name, got.Code, got.Body)
		}
	}
	n := 0
	evs, _ = rt.store.ListEventsByOwner(context.Background(), "owner-stamp", 50)
	for _, ev := range evs {
		if ev.Kind == types.EventPlatformComputerRestart {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("receipts = %d, want 1", n)
	}
}
