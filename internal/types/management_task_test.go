package types

import "testing"

// The two spellings (management-task written, management_task the role):
// readers accept both and the writer emits one, with no data rewrite.
func TestManagementTaskSpellingsResolveToOne(t *testing.T) {
	for _, inbox := range []string{"management-task:t1", "management_task:t1", " management_task:t1 "} {
		id, ok := ManagementTaskIDOfInbox(inbox)
		if !ok || id != "t1" {
			t.Errorf("ManagementTaskIDOfInbox(%q) = %q, %v", inbox, id, ok)
		}
	}
	if got := ManagementTaskInbox("t1"); got != "management-task:t1" {
		t.Errorf("writer spelling = %q", got)
	}
	if got := NormalizeManagementTaskRef("management_task:t1"); got != "management-task:t1" {
		t.Errorf("NormalizeManagementTaskRef = %q", got)
	}
	if got := NormalizeManagementTaskRef("management:o1"); got != "management:o1" {
		t.Errorf("unrelated ref changed: %q", got)
	}
	if !IsTaskInboxDesk("management_task") || !IsTaskInboxDesk("management-task") || IsTaskInboxDesk("management") {
		t.Error("IsTaskInboxDesk")
	}
}

func TestManagementTaskIDOfRefTakesEverySpellingAndNeverDoubles(t *testing.T) {
	for _, ref := range []string{"mt-1", "management-task:mt-1", "management_task:mt-1", "  management_task:mt-1 "} {
		if got := ManagementTaskIDOfRef(ref); got != "mt-1" {
			t.Errorf("ManagementTaskIDOfRef(%q) = %q", ref, got)
		}
		if got := ManagementTaskInbox(ManagementTaskIDOfRef(ref)); got != "management-task:mt-1" {
			t.Errorf("inbox from %q = %q", ref, got)
		}
	}
}
