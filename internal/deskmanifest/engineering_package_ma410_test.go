package deskmanifest

import (
	"strings"
	"testing"
)

// MA4.10: engineering as a package, last (D8). The failure modes, written
// before the code:
//
//  1. a second desk performs assignments, or none does and a check falls back
//     to a default desk -> perform.assignment has one holder and
//     AssignmentPerformer reports false, never a name, with no holder;
//  2. an assignment inbox is built or parsed from a literal desk name -> the
//     inbox helpers read the performer, and an empty id, a foreign prefix or a
//     bare desk name is not an assignment inbox;
//  3. the capsule mutates for a role by its name -> MutatesCapsule asks the
//     capsule.mutate grant, so management, research, a role row and an unknown
//     name all fail closed;
//  4. a package asks for perform.assignment or capsule.mutate -> refused.

func TestEngineeringHoldsThePerformAssignmentGrant(t *testing.T) {
	d, ok := AssignmentPerformer()
	if !ok || d.Name != Engineering {
		t.Fatalf("AssignmentPerformer = %q, %v; want engineering", d.Name, ok)
	}
	if !GrantPrivileged(GrantPerformAssignment) {
		t.Error("perform.assignment is not first-party only")
	}
	if h, ok := GrantHolder(GrantPerformAssignment); !ok || h.Name != Engineering {
		t.Errorf("GrantHolder(perform.assignment) = %q, %v", h.Name, ok)
	}
	for _, v := range d.Verbs {
		if g, _ := VerbGrant(v); g == GrantPerformAssignment {
			t.Errorf("verb %s is carried by perform.assignment", v)
		}
	}
	if d.Instance != InstanceAssignment {
		t.Errorf("the performer's instance is %q, want assignment", d.Instance)
	}
}

func TestNoOtherDeskPerformsAssignments(t *testing.T) {
	for _, name := range Names() {
		if name == Engineering {
			continue
		}
		if Holds(name, GrantPerformAssignment) {
			t.Errorf("%s holds perform.assignment", name)
		}
	}
	second := Desk{Name: "second-performer", Kind: KindSystem, Carrier: CarrierNone,
		Taint: TaintPrivileged, Restart: RestartContract, Grants: []Grant{G(GrantPerformAssignment)}}
	if err := register(second); err == nil || !strings.Contains(err.Error(), "one holder") {
		mu.Lock()
		delete(registry, second.Name)
		mu.Unlock()
		t.Errorf("a second performer registered: %v", err)
	}
	base, err := NewAppDesk(AppDeskSpec{Name: "app:sneak", Verbs: []string{"Cast"}, Taint: TaintPrivileged, UID: 62902, WorkerMaxBytes: 64 << 20})
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range []string{GrantPerformAssignment, "capsule.mutate"} {
		row := base.clone()
		row.Grants = append(row.Grants, G(g))
		if err := RegisterApp(row); err == nil {
			_ = RetireApp(row.Name)
			t.Errorf("an app desk registered holding %s", g)
		}
	}
}

func TestAssignmentInboxHelpersReadThePerformer(t *testing.T) {
	inbox, ok := AssignmentInbox("a1")
	if !ok || inbox != Engineering+":a1" {
		t.Fatalf("AssignmentInbox = %q, %v", inbox, ok)
	}
	if id, ok := AssignmentIDFromInbox("  " + inbox + " "); !ok || id != "a1" {
		t.Errorf("AssignmentIDFromInbox = %q, %v", id, ok)
	}
	for _, bad := range []string{"", Engineering, Engineering + ":", "texture:a1", "management:a1", "a1"} {
		if id, ok := AssignmentIDFromInbox(bad); ok || id != "" {
			t.Errorf("AssignmentIDFromInbox(%q) = %q, %v", bad, id, ok)
		}
		if IsAssignmentInbox(bad) {
			t.Errorf("IsAssignmentInbox(%q)", bad)
		}
	}
	for _, empty := range []string{"", "  "} {
		if s, ok := AssignmentInbox(empty); ok || s != "" {
			t.Errorf("AssignmentInbox(%q) = %q, %v", empty, s, ok)
		}
	}
	if !IsAssignmentInbox(inbox) {
		t.Error("a real assignment inbox is refused")
	}
}

func TestMutatesCapsuleIsTheCapsuleMutateGrant(t *testing.T) {
	if !MutatesCapsule(Engineering) {
		t.Error("engineering does not mutate a capsule")
	}
	for _, name := range []string{Management, Research, Texture, ManagementTask, ResearchWeb, "", "no-such-desk"} {
		if MutatesCapsule(name) {
			t.Errorf("%q mutates a capsule", name)
		}
	}
	if !ReadsCapsules(Research) || ReadsCapsules(Engineering) || ReadsCapsules(Management) {
		t.Error("capsule.read is not what ReadsCapsules answers")
	}
	if ReadsCapsules("") || ReadsCapsules("no-such-desk") {
		t.Error("an unknown desk reads capsules")
	}
}
