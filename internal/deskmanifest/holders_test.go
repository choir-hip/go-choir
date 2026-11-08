package deskmanifest

import "testing"

// MA4.11: code that compared a profile to a desk literal asks the registry's
// grant holders. On the built-in table the answers equal the old literals
// (stored events and inboxes keep their meaning); with no holder they match
// nothing.
func TestHolderLookupsEqualTheBuiltInNames(t *testing.T) {
	if got := AdmitterName(); got != Management {
		t.Fatalf("AdmitterName = %q, want %q", got, Management)
	}
	if got := PerformerName(); got != Engineering {
		t.Fatalf("PerformerName = %q, want %q", got, Engineering)
	}
	if got := ResearcherName(); got != Research {
		t.Fatalf("ResearcherName = %q, want %q", got, Research)
	}
	if got := AdmitterInbox("o1"); got != "management:o1" {
		t.Fatalf("AdmitterInbox = %q", got)
	}
	if got := PerformerInbox("a1"); got != "engineering:a1" {
		t.Fatalf("PerformerInbox = %q", got)
	}
	if got := ResearchInbox("d1"); got != "research:d1" {
		t.Fatalf("ResearchInbox = %q", got)
	}
}

func TestHolderPredicatesAreExact(t *testing.T) {
	for name, want := range map[string]bool{"management": true, "management_task": false, "engineering": false, "research": false, "texture": false, "conductor": false, "": false, "nobody": false} {
		if got := AdmitsAssignments(name); got != want {
			t.Errorf("AdmitsAssignments(%q) = %v", name, got)
		}
	}
	for name, want := range map[string]bool{"research": true, "research_web": false, "research_texture": false, "research_child": false, "management": false, "": false} {
		if got := SpawnsResearch(name); got != want {
			t.Errorf("SpawnsResearch(%q) = %v", name, got)
		}
	}
	if !IsAdmitterInbox("management:o1") || IsAdmitterInbox("engineering:o1") || IsAdmitterInbox("management") {
		t.Error("IsAdmitterInbox must match only <admitter>:<x>")
	}
	if !IsResearchInbox("research:d1") || IsResearchInbox("research_web:d1") || IsResearchInbox("texture:d1") {
		t.Error("IsResearchInbox must match only <researcher>:<x>")
	}
}

// With no holder the names are values nothing stored can carry, so a
// comparison or an inbox match fails closed instead of matching "".
func TestNoHolderFailsClosed(t *testing.T) {
	name := holderName("no.such.grant")
	if name == "" || name == Management || name == Engineering || name == Research {
		t.Fatalf("no-holder name %q must be a value no desk carries", name)
	}
}
