package agentcore

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// R4: the operator receipt is a tape event the store accepts and keeps. The
// kind is registered in the event tables (internal/types) and carries no
// role-bearing field, so the serving vocabulary fence has nothing to refuse
// (the open vocabfence defect is a role value, not an event kind).
func TestPlatformOperatorReceiptEventIsAppended(t *testing.T) {
	t.Parallel()
	rt, _ := testAPISetup(t)
	rec, err := rt.EmitProductEvent(context.Background(), "paper-owner", "primary", types.EventPlatformComputerModelPolicy, map[string]any{
		"operator_account_id": "1111aaaa-2222-3333-4444-55555555beef",
		"act":                 "model_policy.write",
		"path":                "System/model-policy.toml",
		"revision":            "20261109T120000.000001Z.toml",
		"old_sha256":          "aa",
		"new_sha256":          "bb",
		"bytes":               12,
	})
	if err != nil {
		t.Fatalf("receipt event refused by the store: %v", err)
	}
	if rec.Kind != types.EventPlatformComputerModelPolicy || rec.OwnerID != "paper-owner" {
		t.Fatalf("event = %+v", rec)
	}
	if types.EventPlatformComputerModelPolicy.Durability() != types.DurabilityAcknowledged || types.EventPlatformComputerModelPolicy.Storage() != types.StorageObjectGraph {
		t.Fatal("the receipt must be acknowledged and in the object graph")
	}
}
