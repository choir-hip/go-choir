package agentcore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// Enforce flip r4 (section 9), FM11, written first. The owner-act stamp on a
// source schedule is the terminal fact of the publish-auto provenance walk, so
// it must separate the owner's request from a desk's in-process ProductAPI
// call:
//   - a schedule created through the owner route carries created_by_owner_act;
//   - a schedule a desk creates through ProductAPI (empty RemoteAddr) carries
//     none, and neither does a poll or a desk update add one;
//   - the affirm route stamps an existing schedule for the owner, and refuses
//     the in-process caller.

func ownerScheduleRequest(method, target, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body)) // httptest's remote address is a real one
	req.Header.Set("X-Authenticated-User", "owner-stamp")
	return req
}

func getStoredSchedule(t *testing.T, rt *Runtime, source string) store.SourceSchedule {
	t.Helper()
	s, err := rt.store.GetSourceSchedule(scheduleCustodian(context.Background()), "owner-stamp", source)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestOwnerRouteStampsAScheduleAndPollsCarryItForward(t *testing.T) {
	rt, _, _, _ := sourceScheduleFixture(t, "rss:stamp-a", nil)
	h := &APIHandler{rt: rt}
	rec := httptest.NewRecorder()
	h.HandleSourceSchedules(rec, ownerScheduleRequest(http.MethodPost, "/api/autopaper/sources/schedules", `{"source_id":"rss:stamp-a"}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("owner create = %d %s", rec.Code, rec.Body)
	}
	s := getStoredSchedule(t, rt, "rss:stamp-a")
	if s.CreatedByOwnerAct == "" || s.CreatedByOwnerActAt.IsZero() {
		t.Fatalf("the owner route wrote no stamp: %+v", s)
	}
	stamp := s.CreatedByOwnerAct
	due := sourcePollDue{SourceID: "rss:stamp-a", Generation: s.Generation, Due: s.NextDue.Unix()}
	b, _ := json.Marshal(due)
	if err := rt.HandleSourcePollDue(context.Background(), "owner-stamp", rt.cfg.ComputerID, string(b)); err != nil {
		t.Fatal(err)
	}
	if got := getStoredSchedule(t, rt, "rss:stamp-a"); got.CreatedByOwnerAct != stamp {
		t.Fatalf("a poll changed the stamp %q -> %q", stamp, got.CreatedByOwnerAct)
	}
}

func TestFM11DeskProductAPICreateAndAffirmWriteNoStamp(t *testing.T) {
	rt, _, _, _ := sourceScheduleFixture(t, "rss:stamp-b", nil)
	h := &APIHandler{rt: rt}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/autopaper/sources/schedules", h.HandleSourceSchedules)
	mux.HandleFunc("/api/autopaper/sources/schedules/", h.HandleSourceSchedules)
	rt.SetProductAPIHandler(mux, "")
	ctx := withDeskReader(context.Background(), agentprofile.Management)

	if _, err := rt.productAPIRequest(ctx, "owner-stamp", "", "POST", "/api/autopaper/sources/schedules", json.RawMessage(`{"source_id":"rss:stamp-b"}`)); err != nil {
		t.Fatalf("desk create: %v", err)
	}
	if s := getStoredSchedule(t, rt, "rss:stamp-b"); s.CreatedByOwnerAct != "" {
		t.Fatalf("a desk's ProductAPI create wrote the owner stamp %q", s.CreatedByOwnerAct)
	}
	// A desk update of the same schedule adds none.
	if _, err := rt.productAPIRequest(ctx, "owner-stamp", "", "POST", "/api/autopaper/sources/schedules", json.RawMessage(`{"source_id":"rss:stamp-b","interval_minutes":5}`)); err != nil {
		t.Fatalf("desk update: %v", err)
	}
	if s := getStoredSchedule(t, rt, "rss:stamp-b"); s.CreatedByOwnerAct != "" {
		t.Fatal("a desk update added the owner stamp")
	}
	// A desk affirm, by ProductAPI or by an in-process request, stamps nothing.
	_, _ = rt.productAPIRequest(ctx, "owner-stamp", "", "POST", "/api/autopaper/sources/schedules/rss:stamp-b/affirm", nil)
	req := ownerScheduleRequest(http.MethodPost, "/api/autopaper/sources/schedules/rss:stamp-b/affirm", "")
	req.RemoteAddr = ""
	rec := httptest.NewRecorder()
	h.HandleSourceSchedules(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("an in-process affirm = %d, want 403", rec.Code)
	}
	if s := getStoredSchedule(t, rt, "rss:stamp-b"); s.CreatedByOwnerAct != "" {
		t.Fatal("a desk affirm wrote the owner stamp")
	}
	// The owner's affirm stamps it, once.
	rec = httptest.NewRecorder()
	h.HandleSourceSchedules(rec, ownerScheduleRequest(http.MethodPost, "/api/autopaper/sources/schedules/rss:stamp-b/affirm", ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("owner affirm = %d %s", rec.Code, rec.Body)
	}
	first := getStoredSchedule(t, rt, "rss:stamp-b").CreatedByOwnerAct
	if first == "" {
		t.Fatal("the owner's affirm wrote no stamp")
	}
	rec = httptest.NewRecorder()
	h.HandleSourceSchedules(rec, ownerScheduleRequest(http.MethodPost, "/api/autopaper/sources/schedules/rss:stamp-b/affirm", ""))
	if got := getStoredSchedule(t, rt, "rss:stamp-b").CreatedByOwnerAct; got != first {
		t.Fatalf("a second affirm replaced the stamp %q -> %q", first, got)
	}
	// An unknown schedule is not found.
	rec = httptest.NewRecorder()
	h.HandleSourceSchedules(rec, ownerScheduleRequest(http.MethodPost, "/api/autopaper/sources/schedules/rss:none/affirm", ""))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("affirm of an unknown schedule = %d", rec.Code)
	}
}
