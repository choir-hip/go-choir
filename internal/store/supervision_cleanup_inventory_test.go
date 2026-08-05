package store

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

func TestSupervisionCleanupInventoryReportsCanonicalAndProjectionResidue(t *testing.T) {
	productStore := openTestStore(t)
	head := storeBootstrapSupervisionComputer(t, productStore)
	transaction := storeSupervisionOpenTransaction(t, head.CanonicalEventHead)
	request := storeSupervisionRequest(t, transaction, head)
	finalizeSupervisionRequest(t, productStore, request)

	inventory, err := productStore.SupervisionCleanupInventory(context.Background(), transaction.OwnerID, transaction.ComputerID)
	if err != nil {
		t.Fatal(err)
	}
	if inventory.Schema != SupervisionCleanupInventorySchemaV1 || inventory.Head == nil || inventory.Head.Sequence != 2 {
		t.Fatalf("inventory head = %#v", inventory.Head)
	}
	if len(inventory.Events) != 2 || inventory.Events[1].EventKind != computerevent.EventSupervisionTransaction || inventory.Events[1].Status != "finalized" {
		t.Fatalf("inventory events = %#v", inventory.Events)
	}
	if len(inventory.Events[1].MutationKinds) != 3 || inventory.ProjectionImportEvents != 0 {
		t.Fatalf("inventory mutation summary = %#v imports=%d", inventory.Events[1].MutationKinds, inventory.ProjectionImportEvents)
	}
	if len(inventory.Commands) != 1 || inventory.Commands[0].Status != "finalized" || inventory.Commands[0].EventDigest != inventory.Events[1].EventDigest {
		t.Fatalf("inventory commands = %#v", inventory.Commands)
	}
	foundState, foundDocument, foundRevision := false, false, false
	for _, recordSet := range inventory.RecordSets {
		if recordSet.Source != "og_objects" || recordSet.Scope != transaction.ComputerID || recordSet.Count == 0 || len(recordSet.Digest) != 64 {
			continue
		}
		switch recordSet.Kind {
		case string(ogKindSupervisionState):
			foundState = true
		case string(ogKindTexDoc):
			foundDocument = true
		case string(ogKindTexRev):
			foundRevision = true
		}
	}
	if !foundState || !foundDocument || !foundRevision {
		t.Fatalf("inventory record sets = %#v", inventory.RecordSets)
	}
}
