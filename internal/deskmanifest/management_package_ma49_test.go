package deskmanifest

import (
	"strings"
	"testing"
)

// MA4.9: management as a package (D9). Its powers are grants computed at
// registration, first-party only; its persistent identity is its per_owner
// instance. The failure modes, written before the code:
//
//  1. a desk other than management holds platform.update, product.api or
//     admit.assignment (a package asks for them, or a second system row
//     takes one) -> refused at NewAppDesk / RegisterApp / register;
//  2. two owners share one management identity (an empty owner falling back
//     to the bare desk name, an owner id with stray space minting a second
//     id) -> OwnerInstance has no instance without an owner and trims;
//  3. who admits engineering work is a union or a default desk when the
//     grant has no holder -> GrantHolder reports false, never a default;
//  4. a per_owner desk whose declared identity disagrees with its inbox ->
//     refused at validation.

func TestManagementHoldsThePlatformUpdateProductAPIAndAdmissionGrants(t *testing.T) {
	d, ok := Lookup(Management)
	if !ok {
		t.Fatal("management is not registered")
	}
	for _, g := range []string{GrantPlatformUpdate, GrantAdmitAssignment, "product.api"} {
		if !d.HasGrant(g) {
			t.Errorf("management lacks %s", g)
		}
		if !GrantPrivileged(g) {
			t.Errorf("%s is not first-party only", g)
		}
	}
	for _, g := range []string{GrantPlatformUpdate, GrantAdmitAssignment} {
		holder, ok := GrantHolder(g)
		if !ok || holder.Name != Management {
			t.Errorf("GrantHolder(%s) = %q, %v; want management", g, holder.Name, ok)
		}
	}
	// product.api is not a sole-holder grant: GrantHolder does not answer it.
	if _, ok := GrantHolder("product.api"); ok {
		t.Error("GrantHolder answered a grant that is not sole-holder")
	}
	// The two new grants carry no verb: they are host authority, not cell calls.
	for _, v := range d.Verbs {
		if g, _ := VerbGrant(v); g == GrantPlatformUpdate || g == GrantAdmitAssignment {
			t.Errorf("verb %s is carried by %s", v, g)
		}
	}
}

func TestNoOtherDeskHoldsManagementsPowers(t *testing.T) {
	for _, name := range Names() {
		if name == Management {
			continue
		}
		d, _ := Lookup(name)
		for _, g := range []string{GrantPlatformUpdate, GrantAdmitAssignment, "product.api"} {
			if d.HasGrant(g) {
				t.Errorf("%s holds %s", name, g)
			}
		}
		if Holds(name, "product.api") {
			t.Errorf("Holds(%s, product.api)", name)
		}
	}
	if Holds("no-such-desk", "product.api") {
		t.Error("an unknown desk holds a grant")
	}
	// A package desk cannot ask for ProductAPI's verb.
	if _, err := NewAppDesk(AppDeskSpec{Name: "app:sneak", Verbs: []string{"ProductAPI"}, Taint: TaintPrivileged, UID: 62901, WorkerMaxBytes: 64 << 20}); err == nil {
		t.Error("an app desk was given ProductAPI")
	}
	// A package desk row carrying the platform update or admission grant is
	// refused: RegisterApp rebuilds the row from its spec and compares grants.
	base, err := NewAppDesk(AppDeskSpec{Name: "app:sneak", Verbs: []string{"Cast"}, Taint: TaintPrivileged, UID: 62901, WorkerMaxBytes: 64 << 20})
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range []string{GrantPlatformUpdate, GrantAdmitAssignment, "product.api"} {
		row := base.clone()
		row.Grants = append(row.Grants, G(g))
		if err := RegisterApp(row); err == nil {
			_ = RetireApp(row.Name)
			t.Errorf("an app desk registered holding %s", g)
		}
	}
	// A second system row taking a sole-holder grant is refused at register.
	for _, g := range []string{GrantPlatformUpdate, GrantAdmitAssignment} {
		second := Desk{Name: "second-holder-" + strings.ReplaceAll(g, ".", "-"), Kind: KindSystem, Carrier: CarrierNone,
			Taint: TaintPrivileged, Restart: RestartContract, Grants: []Grant{G(g)}}
		if err := register(second); err == nil || !strings.Contains(err.Error(), "one holder") {
			mu.Lock()
			delete(registry, second.Name)
			mu.Unlock()
			t.Errorf("a second holder of %s registered: %v", g, err)
		}
	}
}

func TestPerOwnerInstanceIsThePersistentIdentityAndNeverShared(t *testing.T) {
	d, _ := Lookup(Management)
	if d.Instance != InstancePerOwner || d.Identity != IdentityOwner {
		t.Fatalf("management instance %q identity %q, want per_owner and its derived owner identity", d.Instance, d.Identity)
	}
	a, okA := d.OwnerInstance("owner-a")
	b, okB := d.OwnerInstance("owner-b")
	if !okA || !okB || a == b {
		t.Fatalf("two owners share an identity: %q %q", a, b)
	}
	if a != Management+":owner-a" {
		t.Errorf("instance = %q", a)
	}
	if again, _ := d.OwnerInstance("  owner-a\n"); again != a {
		t.Errorf("a padded owner id minted a second identity %q", again)
	}
	for _, empty := range []string{"", "  "} {
		if id, ok := d.OwnerInstance(empty); ok || id != "" {
			t.Errorf("no owner gave instance %q", id)
		}
	}
	tex, _ := Lookup(Texture)
	if _, ok := tex.OwnerInstance("owner-a"); ok {
		t.Error("a subject desk has an owner instance")
	}
	// A per_owner desk may not declare an identity its inbox does not have.
	bad := Desk{Name: "x", Kind: KindSystem, Carrier: CarrierNone, Taint: TaintPrivileged, Restart: RestartContract,
		Instance: InstancePerOwner, Identity: IdentitySubject}
	if err := validateStructure(bad); err == nil {
		t.Error("a per_owner desk declared a subject identity")
	}
}
