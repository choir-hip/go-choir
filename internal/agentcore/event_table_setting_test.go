package agentcore

// F18: the store.event_table switch (event-table design 4.2, deploy 2). The
// writer stamps only when both keys hold: the host named this computer
// (vmctl's CHOIR_EVENT_TABLE_COMPUTERS, a grant on the root-only credential
// disk at cold boot) and the owner set store.event_table on the computer's
// tape. Ways it can fail, written before the code:
//   - the default is on, or a store error reads as on;
//   - a guest process, a desk or a model turns it on: through
//     choir.ProductAPI, through an in-guest (loopback) HTTP caller, or with
//     no authenticated owner;
//   - the owner's setting alone turns the writer on for a computer the host
//     did not name (every account owner could then make deploy 1 their
//     computer's rollback floor), or a grant for another computer does;
//   - a POST is not applied until a restart, or the persisted setting is not
//     read at boot, so a crash boot silently returns to og_objects (or the
//     reverse);
//   - turning it off hides events already in choir_events;
//   - the observability response changes on a computer the switch is off on.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/eventtablegrant"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const eventTableOwner = "owner-telemetry"

func eventTableRuntime(t *testing.T, computerID string, granted bool) *Runtime {
	t.Helper()
	rt := telemetryRuntime(t, computerID)
	dir := t.TempDir()
	if granted {
		raw, _ := json.Marshal(eventtablegrant.Grant{ComputerID: computerID, Status: eventtablegrant.GrantGranted, Basis: eventtablegrant.BasisComputer, GrantedAt: "2026-10-22T12:00:00Z"})
		if err := os.WriteFile(filepath.Join(dir, eventtablegrant.GrantFile), raw, 0o400); err != nil {
			t.Fatal(err)
		}
	}
	rt.eventTableGrantDir = dir
	return rt
}

func postEventTable(t *testing.T, rt *Runtime, value bool, mutate func(*http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/computer-settings", strings.NewReader(fmt.Sprintf(`{"key":"store.event_table","value":%v}`, value)))
	req.Header.Set("X-Authenticated-User", eventTableOwner)
	if mutate != nil {
		mutate(req)
	}
	rec := httptest.NewRecorder()
	(&APIHandler{rt: rt}).HandleComputerSettings(rec, req)
	return rec
}

func appendToolResult(t *testing.T, rt *Runtime, id string) {
	t.Helper()
	rec := &types.EventRecord{EventID: id, RunID: "run-event-table", OwnerID: eventTableOwner, Timestamp: time.Now().UTC(), Kind: types.EventToolResult}
	if err := rt.store.AppendEvent(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
}

func TestEventTableSettingIsOffByDefaultAndAnOwnerAct(t *testing.T) {
	rt := eventTableRuntime(t, "computer-event-table-default", true)
	rt.configureEventTable(context.Background())
	if rt.store.EventTableWritesOn() {
		t.Fatal("the writer is on with no setting stored")
	}
	if _, known := store.KnownComputerSettings[store.SettingEventTable]; !known || store.KnownComputerSettings[store.SettingEventTable] {
		t.Fatal("store.event_table is not a known setting with default false")
	}
	// No authenticated owner, and an in-guest caller with a forged header:
	// both refused, nothing stored.
	if rec := postEventTable(t, rt, true, func(r *http.Request) { r.Header.Del("X-Authenticated-User") }); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated set = %d", rec.Code)
	}
	if rec := postEventTable(t, rt, true, func(r *http.Request) { r.RemoteAddr = "127.0.0.1:41234" }); rec.Code != http.StatusUnauthorized {
		t.Fatalf("an in-guest loopback caller set it: %d %s", rec.Code, rec.Body)
	}
	if rt.store.EventTableWritesOn() {
		t.Fatal("a refused request turned the writer on")
	}
	if _, err := rt.store.GetComputerSetting(context.Background(), eventTableOwner, rt.cfg.ComputerID, store.SettingEventTable); err == nil {
		t.Fatal("a refused request stored the setting")
	}
	// A desk or model reaches product routes only through choir.ProductAPI,
	// whose allowlist refuses the settings route.
	if err := validateProductAPIRoute(http.MethodPost, "/api/computer-settings"); err == nil {
		t.Fatal("choir.ProductAPI may set computer settings; store.event_table is an owner act")
	}
}

func TestEventTableSettingNeedsTheHostGrant(t *testing.T) {
	rt := eventTableRuntime(t, "computer-event-table-ungranted", false)
	rt.configureEventTable(context.Background())
	if rec := postEventTable(t, rt, true, nil); rec.Code != http.StatusOK {
		t.Fatalf("set = %d %s", rec.Code, rec.Body)
	}
	if rt.store.EventTableWritesOn() {
		t.Fatal("the owner's setting turned the writer on without the host grant")
	}
	st := rt.eventTableStatus()
	if st.HostGranted || !st.Setting || st.Writing || !strings.Contains(st.Reason, "not granted") {
		t.Fatalf("status = %+v", st)
	}
	appendToolResult(t, rt, "ungranted-1")
	if got := rt.store.TelemetryStats().TapeOpsByTable["choir_events"]; got != 0 {
		t.Fatalf("an ungranted computer stamped %d ops", got)
	}

	// A grant on the disk for another computer does not count.
	other := eventTableRuntime(t, "computer-event-table-misplaced", false)
	raw, _ := json.Marshal(eventtablegrant.Grant{ComputerID: "computer-someone-else", Status: eventtablegrant.GrantGranted, Basis: eventtablegrant.BasisComputer})
	if err := os.WriteFile(filepath.Join(other.eventTableGrantDir, eventtablegrant.GrantFile), raw, 0o400); err != nil {
		t.Fatal(err)
	}
	other.configureEventTable(context.Background())
	if rec := postEventTable(t, other, true, nil); rec.Code != http.StatusOK {
		t.Fatalf("set = %d %s", rec.Code, rec.Body)
	}
	if other.store.EventTableWritesOn() {
		t.Fatal("a grant written for another computer turned this one on")
	}
}

func TestEventTableSettingAppliesAtOnceAndAtBoot(t *testing.T) {
	ctx := context.Background()
	t.Setenv("CHOIR_OWNER_ID", eventTableOwner)
	rt := eventTableRuntime(t, "computer-event-table-live", true)
	rt.configureEventTable(ctx)
	appendToolResult(t, rt, "before-1")

	if rec := postEventTable(t, rt, true, nil); rec.Code != http.StatusOK {
		t.Fatalf("set on = %d %s", rec.Code, rec.Body)
	}
	if !rt.store.EventTableWritesOn() {
		t.Fatal("the setting did not take effect without a restart")
	}
	appendToolResult(t, rt, "on-1")
	appendToolResult(t, rt, "on-2")
	if got := rt.store.TelemetryStats().TapeOpsByTable["choir_events"]; got != 2 {
		t.Fatalf("stamped ops = %d, want 2", got)
	}

	// A boot (the crash case: nothing in memory) reads the stored setting
	// and the grant before the first event.
	rt.store.SetEventTableWrites(false)
	rt.configureEventTable(ctx)
	if !rt.store.EventTableWritesOn() {
		t.Fatal("the persisted setting was not applied at boot")
	}

	// Off is the live rollback: new events go back to og_objects, and the
	// events already in choir_events stay readable.
	if rec := postEventTable(t, rt, false, nil); rec.Code != http.StatusOK {
		t.Fatalf("set off = %d %s", rec.Code, rec.Body)
	}
	if rt.store.EventTableWritesOn() {
		t.Fatal("off did not take effect at once")
	}
	appendToolResult(t, rt, "after-1")
	if got := rt.store.TelemetryStats().TapeOpsByTable["choir_events"]; got != 2 {
		t.Fatalf("stamped ops after off = %d, want still 2", got)
	}
	events, err := rt.store.ListEventsAfter(ctx, "run-event-table", 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, e := range events {
		ids = append(ids, e.EventID)
	}
	if got := strings.Join(ids, ","); got != "before-1,on-1,on-2,after-1" {
		t.Fatalf("after turning it off the run reads %s", got)
	}
	rt.configureEventTable(ctx)
	if rt.store.EventTableWritesOn() {
		t.Fatal("a boot after the off write turned the writer on")
	}
}

func TestEventTableObservabilityIsUnchangedWhenOff(t *testing.T) {
	read := func(rt *Runtime) map[string]json.RawMessage {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/runtime/observability", nil)
		req.Header.Set("X-Authenticated-User", eventTableOwner)
		rec := httptest.NewRecorder()
		(&APIHandler{rt: rt}).HandleGuestObservability(rec, req)
		var obs map[string]json.RawMessage
		if err := json.Unmarshal(rec.Body.Bytes(), &obs); err != nil {
			t.Fatalf("observability = %d %s", rec.Code, rec.Body)
		}
		return obs
	}
	off := eventTableRuntime(t, "computer-event-table-obs-off", false)
	off.configureEventTable(context.Background())
	if _, present := read(off)["event_table"]; present {
		t.Fatal("observability carries event_table on a computer that is neither granted nor set")
	}
	on := eventTableRuntime(t, "computer-event-table-obs-on", true)
	on.configureEventTable(context.Background())
	if rec := postEventTable(t, on, true, nil); rec.Code != http.StatusOK {
		t.Fatalf("set = %d %s", rec.Code, rec.Body)
	}
	raw, present := read(on)["event_table"]
	if !present {
		t.Fatal("observability does not show the switch on a granted computer")
	}
	var st EventTableStatus
	if err := json.Unmarshal(raw, &st); err != nil || !st.HostGranted || !st.Setting || !st.Writing {
		t.Fatalf("event_table = %s (%v)", raw, err)
	}
}
