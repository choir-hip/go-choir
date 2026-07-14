package promotion

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func adoptionWithRollbackProfile(t *testing.T, profile map[string]any) types.AppAdoptionRecord {
	t.Helper()
	raw, err := json.Marshal(profile)
	if err != nil {
		t.Fatalf("marshal rollback profile: %v", err)
	}
	return types.AppAdoptionRecord{RollbackProfileJSON: raw}
}

func TestPromoteFreshnessCAS(t *testing.T) {
	base := "refs/computers/c1/active@v1"
	moved := "refs/computers/c1/active@v2"

	t.Run("fresh base passes", func(t *testing.T) {
		rec := adoptionWithRollbackProfile(t, map[string]any{
			"previous_active_source_ref":  base,
			"lineage_ref_at_verification": base,
		})
		lineage := types.ComputerSourceLineageRecord{ActiveSourceRef: base}
		if err := promoteFreshnessCAS(rec, lineage); err != nil {
			t.Fatalf("fresh base must pass CAS: %v", err)
		}
	})

	t.Run("moved foreground is rejected", func(t *testing.T) {
		rec := adoptionWithRollbackProfile(t, map[string]any{
			"previous_active_source_ref":  base,
			"lineage_ref_at_verification": base,
		})
		lineage := types.ComputerSourceLineageRecord{ActiveSourceRef: moved}
		err := promoteFreshnessCAS(rec, lineage)
		if err == nil {
			t.Fatal("moved foreground must fail CAS")
		}
		if !strings.Contains(err.Error(), "re-verify") {
			t.Fatalf("CAS error must direct to re-verify, got: %v", err)
		}
	})

	t.Run("legacy record without captured base skips check", func(t *testing.T) {
		rec := adoptionWithRollbackProfile(t, map[string]any{
			"previous_active_source_ref": base,
		})
		lineage := types.ComputerSourceLineageRecord{ActiveSourceRef: moved}
		if err := promoteFreshnessCAS(rec, lineage); err != nil {
			t.Fatalf("legacy record must skip CAS: %v", err)
		}
	})
}
