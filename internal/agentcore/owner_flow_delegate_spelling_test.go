package agentcore

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/disclosure"
)

// The Delegate path spells the target management-task:<id> (inbox_records.go),
// hyphen included, while the manifest's role is management_task. The pair
// check must see through the spelling, for the target as messageTargetDesk
// resolves it and for the task's return to management.
func TestOwnerFlowPairHoldsForTheExactDelegateSpelling(t *testing.T) {
	to := messageTargetDesk(disclosureCall{sink: disclosure.SinkDeskMessage, target: "management-task:task-1"})
	if !ownerDeskPair(deskmanifest.AdmitterName(), to) {
		t.Errorf("management to %q is not on the chain", to)
	}
	if !ownerDeskPair(to, deskmanifest.AdmitterName()) {
		t.Errorf("%q back to management is not on the chain", to)
	}
	for _, other := range []string{"management-task", "management_task"} {
		if !ownerDeskPair(deskmanifest.AdmitterName(), other) {
			t.Errorf("management to %q is not on the chain", other)
		}
	}
	// The fold does not widen: a hyphenated name that is no role is no pair.
	if ownerDeskPair(deskmanifest.AdmitterName(), "management-nothing") || ownerDeskPair(deskmanifest.Texture, "management-task") {
		t.Error("the hyphen fold widened the pairs")
	}
}
