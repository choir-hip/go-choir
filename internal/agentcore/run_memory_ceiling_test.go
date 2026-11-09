package agentcore

// Every request stays within 100,000 prompt tokens (owner 2026-11-09): Bedrock
// bills a Haiku 5.5 request over 100k at 5x for the whole request. The
// compaction threshold gets a constant ceiling: 90,000 for every desk, 180,000
// for engineering. Ways this could fail:
//  - a 1M-window model still compacts at 70% of 1M (700k) instead of 90k;
//  - the ceiling is applied to only one branch (explicit cfg threshold above
//    it is not clamped, or the window-ratio branch is);
//  - the engineering desk is held to 90k, or other desks get 180k;
//  - a small-window model is raised to the ceiling instead of keeping its
//    lower ratio threshold;
//  - a threshold below the ceiling (cfg 1, tests) is changed;
//  - the "never compact twice in a row" guard or the forced overflow path
//    breaks (compaction at the new threshold loops, or force no longer works);
//  - pressure is compared without the prompt overhead and reserve.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const ceilingTestBigModel = "us.anthropic.claude-haiku-5-5"

func ceilingManager(t *testing.T, profile string, cfgThreshold int, model string, overhead int) (*runMemoryManager, types.RunRecord) {
	t.Helper()
	_, s := testRuntime(t)
	now := time.Now().UTC()
	rec := types.RunRecord{
		RunID: "run-ceiling-" + profile, AgentID: profile + ":ceiling", OwnerID: "owner-1", ComputerID: "autoputer-test",
		AgentProfile: profile, AgentRole: profile, State: types.RunRunning, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.CreateRun(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	m := newRunMemoryManager(s, &rec, provideriface.Config{RunMemoryContextThresholdTokens: cfgThreshold, RunMemoryKeepRecentTokens: 1}, func(types.EventKind, string, json.RawMessage) {})
	m.withPromptBudget(model, overhead)
	return m, rec
}

func ceilingSeed(t *testing.T, m *runMemoryManager) {
	t.Helper()
	big := strings.Repeat("evidence ", 400)
	var initial []json.RawMessage
	for _, role := range []string{"user", "assistant", "user", "assistant"} {
		msg, _ := json.Marshal(map[string]any{"role": role, "content": []any{map[string]string{"type": "text", "text": role + " " + big}}})
		initial = append(initial, msg)
	}
	if _, err := m.initialize(context.Background(), initial); err != nil {
		t.Fatal(err)
	}
}

func TestRunMemoryThresholdCeiling(t *testing.T) {
	eng := agentprofile.Engineering
	if got := agentProfileForRun(&types.RunRecord{AgentProfile: eng}); got != deskmanifest.PerformerName() {
		t.Fatalf("engineering profile %q is not the performer %q", got, deskmanifest.PerformerName())
	}
	cases := []struct {
		name    string
		profile string
		cfg     int
		model   string
		want    int
	}{
		{"1M window, other desk", agentprofile.Research, 0, ceilingTestBigModel, 90_000},
		{"1M window, engineering", eng, 0, ceilingTestBigModel, 180_000},
		{"cfg above ceiling is clamped", agentprofile.Research, 500_000, ceilingTestBigModel, 90_000},
		{"cfg above ceiling, engineering", eng, 500_000, ceilingTestBigModel, 180_000},
		{"cfg below ceiling kept", agentprofile.Research, 1, ceilingTestBigModel, 1},
		{"unknown model: 70% of 200k default, clamped", agentprofile.Research, 0, "no-such-model", 90_000},
		{"smaller threshold kept", agentprofile.Research, 50_000, ceilingTestBigModel, 50_000},
	}
	for _, c := range cases {
		m, _ := ceilingManager(t, c.profile, c.cfg, c.model, 0)
		if got := m.effectiveContextThresholdTokens(); got != c.want {
			t.Errorf("%s: threshold %d, want %d", c.name, got, c.want)
		}
	}
}

func countCompactions(t *testing.T, m *runMemoryManager, rec types.RunRecord) int {
	t.Helper()
	entries, err := m.store.ListRunMemoryEntries(context.Background(), rec.OwnerID, rec.RunID)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range entries {
		if e.Kind == types.RunMemoryEntryCompaction {
			n++
		}
	}
	return n
}

// pressure = messages + overhead + reserve; the overhead is sized so that the
// total lands just under or just over the ceiling.
func TestRunMemoryCompactsAtCeilingNotBefore(t *testing.T) {
	probe, probeRec := ceilingManager(t, agentprofile.Research, 0, ceilingTestBigModel, 0)
	ceilingSeed(t, probe)
	entries, _ := probe.store.ListRunMemoryEntries(context.Background(), probeRec.OwnerID, probeRec.RunID)
	base := probe.estimatePromptPressureTokens(entries)

	for _, c := range []struct {
		profile string
		target  int
		want    bool
	}{
		{agentprofile.Research, 89_000, false},
		{agentprofile.Research, 91_000, true},
		{agentprofile.Research, 120_000, true},
		{agentprofile.Engineering, 120_000, false},
		{agentprofile.Engineering, 179_000, false},
		{agentprofile.Engineering, 181_000, true},
	} {
		m, rec := ceilingManager(t, c.profile, 0, ceilingTestBigModel, c.target-base)
		ceilingSeed(t, m)
		compacted, err := m.compactIfNeeded(context.Background(), "threshold", false)
		if err != nil {
			t.Fatal(err)
		}
		wantN := 0
		if c.want {
			wantN = 1
		}
		if compacted != c.want || countCompactions(t, m, rec) != wantN {
			t.Fatalf("%s at %d tokens: compacted=%v, want %v", c.profile, c.target, compacted, c.want)
		}
		if c.want {
			// Never compact twice in a row.
			again, err := m.compactIfNeeded(context.Background(), "threshold", false)
			if err != nil || again {
				t.Fatalf("%s compacted twice in a row: %v %v", c.profile, again, err)
			}
		}
	}
}
