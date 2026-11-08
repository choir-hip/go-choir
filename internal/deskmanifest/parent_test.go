package deskmanifest

import "testing"

// The enforce flip (section 7.3, 8.2): a role's parent is the holder of the
// sole-holder grant its Parent field names, never a name prefix.
func TestParentOfResolvesByGrantHolder(t *testing.T) {
	for role, grant := range map[string]string{
		ManagementTask:  GrantAdmitAssignment,
		ResearchWeb:     GrantResearchSpawn,
		ResearchTexture: GrantResearchSpawn,
	} {
		p, ok := ParentOf(role)
		if !ok {
			t.Fatalf("%s has no parent", role)
		}
		h, _ := GrantHolder(grant)
		if p.Name != h.Name || p.Kind != KindSystem {
			t.Errorf("%s parent = %s, want holder of %s (%s)", role, p.Name, grant, h.Name)
		}
	}
	for _, name := range []string{Management, Texture, Research, Engineering, Conductor, Mail, "nonexistent"} {
		if _, ok := ParentOf(name); ok {
			t.Errorf("%s is no role and must have no parent", name)
		}
	}
}

func TestParentIsRefusedOnNonRolesAndNonSoleGrants(t *testing.T) {
	d := Desk{Name: "x", Kind: KindSystem, Parent: GrantAdmitAssignment, Restart: RestartContract, Taint: TaintPrivileged}
	if err := validateStructure(d); err == nil {
		t.Error("a system desk named a parent grant")
	}
	r := Desk{Name: "x", Kind: KindRole, Parent: "act.cast", Restart: RestartContract, Taint: TaintPrivileged,
		Carrier: CarrierDeskWorker, Sandbox: Sandbox{Profile: "desk_cell", UID: 61009}, Memory: Memory{WorkerMaxBytes: 1 << 20}}
	if err := validateStructure(r); err == nil {
		t.Error("a role named a non-sole-holder grant as its parent")
	}
}

func TestParentIsPartOfAuthorityEquality(t *testing.T) {
	a, _ := Lookup(ManagementTask)
	b := a
	b.Parent = GrantResearchSpawn
	if a.Equal(b) {
		t.Error("Equal ignores Parent")
	}
}
