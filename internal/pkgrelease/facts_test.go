package pkgrelease

import (
	"encoding/json"
	"testing"
)

// L1: kept facts are read back as recorded; anything else falls back to the
// record's summary and says it was not recorded (never a claimed ceiling).
func TestFactsFromMetadataReadsKeptFactsOrFallsBackToTheSummary(t *testing.T) {
	kept, _ := json.Marshal(map[string]any{"facts": map[string]any{"kind": "pack", "tier": "T0", "level": "default", "recorded": true, "ceiling": []any{map[string]any{"grant": "ui.captures.read", "level": "default"}}}})
	f := FactsFromMetadata(kept, Manifest{Class: "app", Level: "default"})
	if f.Kind != "pack" || !f.Recorded || len(f.Ceiling) != 1 || f.Slots == nil || f.Facets == nil {
		t.Fatalf("kept facts = %+v", f)
	}
	// Metadata never promotes facts the host did not mark recorded.
	unmarked, _ := json.Marshal(map[string]any{"facts": map[string]any{"kind": "pack", "tier": "T0", "level": "default"}})
	if f := FactsFromMetadata(unmarked, Manifest{Class: "data", Level: "default"}); f.Recorded {
		t.Fatalf("unmarked facts read as recorded: %+v", f)
	}
	for class, want := range map[string]string{"data": "pack", "app": "app", "desk": "app", "app+desk": "app", "provider": "provider"} {
		f := FactsFromMetadata(json.RawMessage(`{"description":"x"}`), Manifest{Class: class, Level: "extended"})
		if string(f.Kind) != want || f.Recorded || f.Level != "extended" || f.Ceiling == nil || len(f.Ceiling) != 0 {
			t.Errorf("%s: %+v", class, f)
		}
	}
	if f := FactsFromMetadata(json.RawMessage(`{"facts":"junk"}`), Manifest{Class: "data"}); f.Recorded || f.Kind != "pack" {
		t.Fatalf("junk facts = %+v", f)
	}
}
