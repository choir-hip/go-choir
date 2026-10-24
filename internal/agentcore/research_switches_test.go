package agentcore

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Per-computer compute-saving switches (FP0 direction 2026-10-24):
// research.run_memory_elision (E2) and research.search_caps (E3), beside the
// fleet env vars CHOIR_RUN_MEMORY_ELISION and CHOIR_DESK_SEARCH_*. Ways this
// can fail, written before the code:
//   - F1. The setting is ignored: stored true, elision still off / 40 results.
//   - F2. The setting is on by default, or a missing/false setting reads on.
//   - F3. The env var stops working once the setting exists (env stays the
//     fleet switch): env on with the setting off must still be on.
//   - F4. A setting read error reads as on (it must fail closed).
//   - F5. The change needs a restart: the owner's POST must reach the next
//     run without one.
//   - F6. A guest desk sets it: the route stays owner-only (no auth, an
//     in-guest loopback caller, choir.ProductAPI are all refused).
//   - F7. The setting turns elision on for a desk with no retrieval verb.
//   - F8. The setting overrides an explicit env tuning of the caps, or the
//     KEEP/BATCH/MIN_CHARS knobs stop applying with the setting on.
//   - F9. A key typo is stored (the allowlist refuses unknown keys).
//   - F10. One computer's setting turns another's on (per computer).

const switchOwner = "owner-research-switches"

func clearSwitchEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"CHOIR_RUN_MEMORY_ELISION", "CHOIR_RUN_MEMORY_ELISION_KEEP", "CHOIR_RUN_MEMORY_ELISION_BATCH", "CHOIR_RUN_MEMORY_ELISION_MIN_CHARS",
		"CHOIR_DESK_SEARCH_MAX_RESULTS", "CHOIR_DESK_SEARCH_SNIPPET_CHARS",
	} {
		t.Setenv(name, "")
	}
}

func postSwitch(t *testing.T, rt *Runtime, key string, value bool, mutate func(*http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/computer-settings", strings.NewReader(fmt.Sprintf(`{"key":%q,"value":%v}`, key, value)))
	req.Header.Set("X-Authenticated-User", switchOwner)
	if mutate != nil {
		mutate(req)
	}
	rec := httptest.NewRecorder()
	(&APIHandler{rt: rt}).HandleComputerSettings(rec, req)
	return rec
}

var researchRun = &types.RunRecord{AgentID: "research:doc", AgentProfile: agentprofile.Research}

func TestResearchSwitchSettingsAreKnownDefaultOffAndOwnerOnly(t *testing.T) {
	clearSwitchEnv(t)
	rt, _ := testRuntime(t)
	for _, key := range []string{store.SettingRunMemoryElision, store.SettingSearchCaps} {
		if v, known := store.KnownComputerSettings[key]; !known || v {
			t.Fatalf("%s is not a known setting with default false", key)
		}
		if rec := postSwitch(t, rt, key, true, func(r *http.Request) { r.Header.Del("X-Authenticated-User") }); rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s: unauthenticated set = %d", key, rec.Code)
		}
		if rec := postSwitch(t, rt, key, true, func(r *http.Request) { r.RemoteAddr = "127.0.0.1:41234" }); rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s: an in-guest loopback caller set it: %d %s", key, rec.Code, rec.Body)
		}
	}
	if rt.runMemoryElisionSettingOn() || rt.searchCapsSettingOn() {
		t.Fatal("a refused request or a missing setting turned a switch on")
	}
	if err := validateProductAPIRoute(http.MethodPost, "/api/computer-settings"); err == nil {
		t.Fatal("choir.ProductAPI may set computer settings; these are owner acts")
	}
	if rec := postSwitch(t, rt, "research.run_memory_elison", true, nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("a typo key = %d, want 400", rec.Code)
	}
}

func TestRunMemoryElisionPolicyFollowsSettingOrEnv(t *testing.T) {
	clearSwitchEnv(t)
	rt := &Runtime{}
	if rt.runMemoryElisionPolicy(researchRun).Enabled {
		t.Fatal("elision is on with the setting off and no env")
	}
	rt.runMemoryElisionSetting.Store(true)
	p := rt.runMemoryElisionPolicy(researchRun)
	if !p.Enabled || p.KeepRecent != defaultElisionKeepRecent || p.Batch != defaultElisionBatch || p.MinChars != defaultElisionMinChars || p.Retrieval != "choir.RunMemoryEntry" {
		t.Fatalf("setting on: %+v", p)
	}
	// F7: a desk with no retrieval verb is never elided.
	if rt.runMemoryElisionPolicy(&types.RunRecord{AgentID: "texture:x", AgentProfile: agentprofile.Texture}).Enabled {
		t.Fatal("the setting enabled elision for a desk that cannot read an entry back")
	}
	// F8: the tuning knobs still apply with the setting on.
	t.Setenv("CHOIR_RUN_MEMORY_ELISION_KEEP", "5")
	t.Setenv("CHOIR_RUN_MEMORY_ELISION_BATCH", "10")
	t.Setenv("CHOIR_RUN_MEMORY_ELISION_MIN_CHARS", "700")
	if p := rt.runMemoryElisionPolicy(researchRun); p.KeepRecent != 5 || p.Batch != 10 || p.MinChars != 700 {
		t.Fatalf("knobs ignored with the setting on: %+v", p)
	}
	// F3: env on with the setting off is on.
	rt.runMemoryElisionSetting.Store(false)
	t.Setenv("CHOIR_RUN_MEMORY_ELISION", "1")
	if !rt.runMemoryElisionPolicy(researchRun).Enabled {
		t.Fatal("env on with the setting off is not on")
	}
	// A nil runtime is the env-only policy.
	var none *Runtime
	if !none.runMemoryElisionPolicy(researchRun).Enabled {
		t.Fatal("nil runtime lost the env switch")
	}
}

func TestDeskSearchLimitsFollowTheSettingOrEnv(t *testing.T) {
	for _, tc := range []struct {
		name, results, snippet string
		setting                bool
		wantResults, wantChars int
	}{
		{name: "setting off, no env", wantResults: 40, wantChars: 0},
		{name: "setting on, no env", setting: true, wantResults: 12, wantChars: 300},
		{name: "env on, setting off", results: "12", snippet: "300", wantResults: 12, wantChars: 300},
		{name: "env override beats the setting", setting: true, results: "20", snippet: "100", wantResults: 20, wantChars: 100},
		{name: "env snippet 0 (whole) beats the setting", setting: true, snippet: "0", wantResults: 12, wantChars: 0},
		{name: "garbage env falls to the setting", setting: true, results: "lots", snippet: "-3", wantResults: 12, wantChars: 300},
		{name: "garbage env, setting off", results: "lots", snippet: "-3", wantResults: 40, wantChars: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CHOIR_DESK_SEARCH_MAX_RESULTS", tc.results)
			t.Setenv("CHOIR_DESK_SEARCH_SNIPPET_CHARS", tc.snippet)
			if got := deskSearchMaxResults(tc.setting); got != tc.wantResults {
				t.Fatalf("max results = %d, want %d", got, tc.wantResults)
			}
			if got := deskSearchSnippetChars(tc.setting); got != tc.wantChars {
				t.Fatalf("snippet chars = %d, want %d", got, tc.wantChars)
			}
		})
	}
}

func TestResearchSwitchSettingsApplyLiveAndAtBoot(t *testing.T) {
	clearSwitchEnv(t)
	rt, _ := testRuntime(t)
	t.Setenv("CHOIR_OWNER_ID", switchOwner)
	ctx := context.Background()
	rt.configureResearchSwitchSettings(ctx)
	if rt.runMemoryElisionSettingOn() || rt.searchCapsSettingOn() {
		t.Fatal("a boot with no stored setting reads on")
	}
	// F5: the owner's POST reaches the next run with no restart.
	for key, on := range map[string]func() bool{
		store.SettingRunMemoryElision: func() bool { return rt.runMemoryElisionPolicy(researchRun).Enabled },
		store.SettingSearchCaps:       func() bool { return deskSearchMaxResults(rt.searchCapsSettingOn()) == 12 },
	} {
		if on() {
			t.Fatalf("%s: on before the owner set it", key)
		}
		if rec := postSwitch(t, rt, key, true, nil); rec.Code != http.StatusOK {
			t.Fatalf("%s: set = %d %s", key, rec.Code, rec.Body)
		}
		if !on() {
			t.Fatalf("%s: the owner's change did not apply live", key)
		}
	}
	// F10: another computer's switch is not this computer's.
	if rt.computerSettingOn(ctx, switchOwner, "another-computer", store.SettingSearchCaps) {
		t.Fatal("a setting on one computer reads on another")
	}
	// A restart reads the stored settings.
	rt.runMemoryElisionSetting.Store(false)
	rt.searchCapsSetting.Store(false)
	rt.configureResearchSwitchSettings(ctx)
	if !rt.runMemoryElisionSettingOn() || !rt.searchCapsSettingOn() {
		t.Fatal("the stored settings were not read at boot")
	}
	// Off is the rollback, also live.
	for _, key := range []string{store.SettingRunMemoryElision, store.SettingSearchCaps} {
		if rec := postSwitch(t, rt, key, false, nil); rec.Code != http.StatusOK {
			t.Fatalf("%s: unset = %d %s", key, rec.Code, rec.Body)
		}
	}
	if rt.runMemoryElisionPolicy(researchRun).Enabled || deskSearchMaxResults(rt.searchCapsSettingOn()) != 40 {
		t.Fatal("off did not roll back live")
	}
	// A same-value POST re-applies (the apply is idempotent).
	rt.searchCapsSetting.Store(true)
	if rec := postSwitch(t, rt, store.SettingSearchCaps, false, nil); rec.Code != http.StatusOK || rt.searchCapsSettingOn() {
		t.Fatalf("same-value POST left the flag on: %d", rec.Code)
	}
}

func TestResearchSwitchSettingReadErrorFailsClosed(t *testing.T) {
	clearSwitchEnv(t)
	rt, s := testRuntime(t)
	t.Setenv("CHOIR_OWNER_ID", switchOwner)
	ctx := context.Background()
	for _, key := range []string{store.SettingRunMemoryElision, store.SettingSearchCaps} {
		if err := s.PutComputerSetting(ctx, store.ComputerSetting{OwnerID: switchOwner, ComputerID: rt.cfg.ComputerID, Key: key, Value: true}); err != nil {
			t.Fatal(err)
		}
	}
	rt.configureResearchSwitchSettings(ctx)
	if !rt.runMemoryElisionSettingOn() || !rt.searchCapsSettingOn() {
		t.Fatal("stored true not read")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	rt.configureResearchSwitchSettings(ctx)
	if rt.runMemoryElisionSettingOn() || rt.searchCapsSettingOn() {
		t.Fatal("a store read error read as on; it must fail closed")
	}
	// No owner id also reads off.
	t.Setenv("CHOIR_OWNER_ID", "")
	rt.configureResearchSwitchSettings(ctx)
	if rt.runMemoryElisionSettingOn() || rt.searchCapsSettingOn() {
		t.Fatal("no owner id read as on")
	}
}
