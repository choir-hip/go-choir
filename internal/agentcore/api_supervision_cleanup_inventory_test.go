package agentcore

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/store"
)

func TestSupervisionCleanupInventoryRequiresAuthenticationAndStableComputer(t *testing.T) {
	_, handler := testAPISetup(t)
	t.Setenv("CHOIR_COMPUTER_ID", "computer-inventory")
	t.Setenv("CHOIR_REALIZATION_ID", "realization-inventory")

	unauthenticated := runtimeHandlerRequest(t, handler.HandleSupervisionCleanupInventory, http.MethodGet, "/api/acceptance/supervision-cleanup-inventory", "", "")
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d", unauthenticated.Code)
	}
	response := runtimeHandlerRequest(t, handler.HandleSupervisionCleanupInventory, http.MethodGet, "/api/acceptance/supervision-cleanup-inventory", "", "owner-inventory")
	if response.Code != http.StatusOK {
		t.Fatalf("inventory status = %d body=%s", response.Code, response.Body.String())
	}
	var inventory store.SupervisionCleanupInventory
	if err := json.Unmarshal(response.Body.Bytes(), &inventory); err != nil {
		t.Fatal(err)
	}
	if inventory.Schema != store.SupervisionCleanupInventorySchemaV1 || inventory.OwnerID != "owner-inventory" || inventory.ComputerID != "computer-inventory" || inventory.RealizationID != "realization-inventory" {
		t.Fatalf("inventory identity = %#v", inventory)
	}
	if inventory.Events == nil || inventory.Commands == nil || inventory.RecordSets == nil {
		t.Fatalf("inventory empty collections must be encoded: %#v", inventory)
	}
}
