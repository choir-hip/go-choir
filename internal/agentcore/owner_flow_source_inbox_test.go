package agentcore

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/disclosure"
)

// Enforce flip S1 run 2 (design section 12): management's decline of a capture
// goes to the capture's source inbox (source:<id>), an inbox class no desk
// reads. A message from the admitter to a registered, addressable class that
// the admitter may send to discloses nothing to another custodian, and
// OwnerFlow covers it for its atoms. The class is resolved from the registry
// of inbox classes, not by a name.
func TestOwnerFlowCoversTheAdmittersDeclineToASourceInbox(t *testing.T) {
	to := messageTargetDesk(disclosureCall{sink: disclosure.SinkDeskMessage, target: captureSourceDesk + ":rss:bbc_world"})
	mgmt := deskmanifest.AdmitterName()
	if !ownerDeskPair(mgmt, to) {
		t.Errorf("management to %q (the source inbox) is not on the chain", to)
	}
	// Only the admitter, and only toward a class the admitter may write.
	for _, from := range []string{deskmanifest.Texture, deskmanifest.ResearcherName(), deskmanifest.PerformerName(), deskmanifest.Mail, "app:notes", "nonexistent"} {
		if ownerDeskPair(from, to) {
			t.Errorf("%q to the source inbox is on the chain", from)
		}
	}
	// A job inbox has no senders (not addressable); the source never sends as a desk.
	if ownerDeskPair(mgmt, jobInboxClass) {
		t.Error("management to a job inbox is on the chain")
	}
	if ownerDeskPair(to, mgmt) {
		t.Error("the source inbox as a sender is on the chain")
	}
	// An unregistered class is no pair.
	if ownerDeskPair(mgmt, "not-a-class") {
		t.Error("management to an unregistered name is on the chain")
	}
}

// Devin S1 run 2 note: the source inbox is the only registered class the
// admitter qualifies for today, so OwnerFlow toward an inbox class is exactly
// the decline path. A new qualifying class must be added here on purpose.
func TestOwnerFlowSourceInboxIsTheOnlyQualifyingClass(t *testing.T) {
	var got []string
	for _, c := range InboxClasses() {
		if admitterSendsToInboxClass(c.Name) {
			got = append(got, c.Name)
		}
	}
	if len(got) != 1 || got[0] != captureSourceDesk {
		t.Fatalf("qualifying inbox classes = %v, want [%s]", got, captureSourceDesk)
	}
}
