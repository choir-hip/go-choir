package agentcore

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
)

// dispatch.wide_slots (the paper's owner, 2026-10-27). Ways it can fail:
//   - the owner's act stores the setting but the dispatcher keeps 8 until a
//     restart;
//   - turning it off leaves 12 (off must be the rollback);
//   - a boot with the setting on starts at 8;
//   - a boot with no setting stored starts above 8.
func TestDispatchWideSlotsSettingSetsTheDispatcherCap(t *testing.T) {
	rt, s := testRuntime(t)
	owner, computer := "owner-slots", rt.cfg.ComputerID
	t.Setenv("CHOIR_OWNER_ID", owner)

	var got []int
	rt.SetSharedSlotsActor(func(n int) { got = append(got, n) })
	if len(got) != 0 {
		t.Fatalf("a boot with no setting changed the cap: %v", got)
	}

	handler := &APIHandler{rt: rt}
	post := func(value string) {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/api/computer-settings", strings.NewReader(`{"key":"dispatch.wide_slots","value":`+value+`}`))
		req.Header.Set("X-Authenticated-User", owner)
		rec := httptest.NewRecorder()
		handler.HandleComputerSettings(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("set %s = %d %s", value, rec.Code, rec.Body)
		}
	}
	post("true")
	if len(got) != 1 || got[0] != 12 {
		t.Fatalf("on: cap calls = %v, want [12]", got)
	}
	post("false")
	if len(got) != 2 || got[1] != 0 {
		t.Fatalf("off: cap calls = %v, want [12 0] (0 is the dispatcher's default)", got)
	}

	// A boot with the setting stored on starts at 12.
	if err := s.PutComputerSetting(context.Background(), store.ComputerSetting{OwnerID: owner, ComputerID: computer, Key: store.SettingDispatchWideSlots, Value: true, UpdatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	var boot []int
	rt.SetSharedSlotsActor(func(n int) { boot = append(boot, n) })
	if len(boot) != 1 || boot[0] != 12 {
		t.Fatalf("boot with the setting on: cap calls = %v, want [12]", boot)
	}
}
