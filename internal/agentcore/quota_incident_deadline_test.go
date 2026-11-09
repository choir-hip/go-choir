package agentcore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// docs/problems/paper-quota-incidents-2026-11-09.md: management-task runs that
// slept through an every-model quota incident were cancelled "activation
// budget exceeded: progress deadline reached". Failure modes: the incident
// sleep is charged to the progress deadline; the time is never given back
// when the incident ends; a research child (no run record) suspends its
// parent's deadline; the ledger's holds are absent from observability.

func TestQuotaIncidentSleepDoesNotSpendTheProgressDeadline(t *testing.T) {
	rt := &Runtime{}
	rec := &types.RunRecord{RunID: "run-q", OwnerID: "owner", ComputerID: "c", AgentID: "a"}
	hook := rt.providerQuotaIncidentHook(rec)
	deadline, cancel := newActivationDeadline(context.Background(), 150*time.Millisecond)
	defer cancel()

	wait := types.NewProviderQuotaWait("run", "synthetic", "m", time.Now(), time.Now().Add(time.Minute), false, time.Minute, "every model out")
	wait.Stamp(time.Now())
	hook(deadline, wait, "")
	time.Sleep(300 * time.Millisecond)
	if err := deadline.Err(); err != nil {
		t.Fatalf("deadline ended during the incident sleep: %v", err)
	}
	hook(deadline, nil, "cleared")
	if err := deadline.Err(); err != nil {
		t.Fatalf("deadline ended as the incident cleared: %v", err)
	}
	if d, _ := deadline.Deadline(); time.Until(d) < 100*time.Millisecond {
		t.Fatalf("deadline has %s left after the incident; the parked time was not given back", time.Until(d))
	}
	// Without an incident the same budget still ends the activation.
	other, cancel2 := newActivationDeadline(context.Background(), 50*time.Millisecond)
	defer cancel2()
	time.Sleep(150 * time.Millisecond)
	if other.Err() == nil {
		t.Fatal("an activation with no incident outlived its deadline")
	}
}

func TestResearchChildIncidentDoesNotSuspendTheParentDeadline(t *testing.T) {
	rt := &Runtime{}
	hook := rt.providerQuotaIncidentHookFor("owner", "c", "research-child:x", "a", "research child x", nil)
	deadline, cancel := newActivationDeadline(context.Background(), time.Hour)
	defer cancel()
	wait := types.NewProviderQuotaWait("run", "synthetic", "m", time.Now(), time.Now().Add(time.Minute), false, time.Minute, "every model out")
	hook(deadline, wait, "")
	if _, _, moved := deadline.endsAt(); moved {
		t.Fatal("a research child's incident suspended the parent's deadline")
	}
	hook(deadline, nil, "cleared")
}

func TestObservabilityShowsTheQuotaLedgerHolds(t *testing.T) {
	rt := telemetryRuntime(t, "computer-quota-ledger")
	member := provideriface.LLMSelection{Provider: "synthetic", Model: "hf:deepseek-ai/DeepSeek-V4.1-Flash-1017"}
	rt.quotaLedger.MarkSourced(member, time.Now().Add(5*time.Minute), true)
	req := httptest.NewRequest(http.MethodGet, "/api/runtime/observability", nil)
	req.Header.Set("X-Authenticated-User", "owner-telemetry")
	rec := httptest.NewRecorder()
	(&APIHandler{rt: rt}).HandleGuestObservability(rec, req)
	var obs struct {
		QuotaLedger []struct {
			Member string    `json:"member"`
			Until  time.Time `json:"until"`
			Source string    `json:"source"`
		} `json:"quota_ledger"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &obs); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("observability = %d %s", rec.Code, rec.Body.String())
	}
	if len(obs.QuotaLedger) != 1 || obs.QuotaLedger[0].Member != "synthetic/hf:deepseek-ai/deepseek-v4.1-flash-1017" ||
		obs.QuotaLedger[0].Source != "estimated" || !obs.QuotaLedger[0].Until.After(time.Now()) {
		t.Fatalf("quota_ledger = %+v, want the held member, its end and that it is estimated", obs.QuotaLedger)
	}
}

// A restart keeps what the computer knew about providers being down: the
// restarted runtime's ledger, and the observability field, show the holds.
func TestQuotaLedgerHoldsSurviveARuntimeRestart(t *testing.T) {
	cfg := provideriface.Config{StorePath: filepath.Join(t.TempDir(), "state.db")}
	member := provideriface.LLMSelection{Provider: "synthetic", Model: "m1"}
	first := New(cfg, nil, nil, nil)
	first.quotaLedger.Mark(member, time.Now().Add(time.Hour))
	first.quotaLedger.DemoteHard(provideriface.LLMSelection{Provider: "zai", Model: "glm"}, time.Now())

	second := New(cfg, nil, nil, nil)
	holds := second.quotaLedger.Holds(time.Now())
	if len(holds) != 2 || holds[0].Member != "synthetic/m1" || holds[1].Source != "hard_failure" {
		t.Fatalf("holds after a restart = %+v, want both restored", holds)
	}
}
