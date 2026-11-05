package llmcost

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func progress(run, desk, model string, in, out int) types.EventRecord {
	payload := map[string]any{"phase": "tool_loop", "llm_model": model, "input_tokens": in, "output_tokens": out}
	if desk != "" {
		payload["desk"] = desk
	}
	raw, _ := json.Marshal(payload)
	return types.EventRecord{EventID: run + desk, RunID: run, Kind: types.EventRunProgress, Timestamp: time.Unix(1700000000, 0), Payload: raw}
}

// MA3.9: spend groups by desk and by package, a call whose event names no desk
// takes the run's, and what cannot be told is its own group. Nothing caps it.
func TestSpendGroupsByDeskAndPackage(t *testing.T) {
	entries := ExtractCostEntries([]types.EventRecord{
		progress("r1", "app:notes", "gpt-4o", 1000, 100),
		progress("r2", "app:notes", "gpt-4o", 2000, 200),
		progress("r3", "management", "gpt-4o", 500, 50),
		progress("r4", "", "gpt-4o", 700, 70),
		progress("r5", "", "gpt-4o", 900, 90),
	})
	if entries[0].Desk != "app:notes" {
		t.Fatalf("event desk lost: %+v", entries[0])
	}
	AttributeDesks(entries, func(run string) string {
		if run == "r4" {
			return "app:video"
		}
		return ""
	})
	s := Aggregate(entries)
	if got := s.ByDesk["app:notes"].CallCount; got != 2 {
		t.Fatalf("by desk = %+v", s.ByDesk)
	}
	if s.ByPackage["notes"].TotalInputTokens != 3000 || s.ByPackage["video"].CallCount != 1 || s.ByPackage[SystemPackage].CallCount != 1 {
		t.Fatalf("by package = %+v", s.ByPackage)
	}
	if s.ByPackage[Unattributed].CallCount != 1 || s.ByDesk[Unattributed].CallCount != 1 {
		t.Fatalf("unattributed = %+v", s.ByPackage)
	}
	if s.CallCount != 5 {
		t.Fatalf("call count = %d", s.CallCount)
	}
}
