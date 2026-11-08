package store

import (
	"context"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// The boot migration lists each obligation-bearing kind on its custodian's
// behalf. Read undeclared, the reader gate refuses the Texture revision list
// at enforce, the migration aborts at that kind, the run kind after it is
// never listed, and the marker never lands, so every boot repeats the pass (F5
// drill, 2026-11-08: a fresh computer counted texture_revision undeclared at
// boot).
func TestMigrateActorWakeOutboxReadsEachKindAsItsCustodian(t *testing.T) {
	defer objectgraph.SetModeForTest(objectgraph.WriterGateEnforce)()
	s := openTestStore(t)
	objectgraph.ResetReaderGateCounts()
	if _, err := s.MigrateActorWakeOutbox(context.Background()); err != nil {
		t.Fatalf("migration at enforce: %v", err)
	}
	for k, n := range objectgraph.ReaderGateCounts() {
		if strings.Contains(k, "undeclared_reader") || strings.Contains(k, "wrong_reader") {
			t.Errorf("migration read refused: %s = %d", k, n)
		}
	}
}
