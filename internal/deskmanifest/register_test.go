package deskmanifest

import (
	"reflect"
	"testing"
	"time"
)

// MA2.5 failure modes, written first:
//  1. a built-in desk loads by a path other than Register, so a special case
//     survives in code;
//  2. a lookup answers differently from the closed list it replaced (a name
//     gains or loses profile, prompt, identity, deadline or module);
//  3. Register accepts a duplicate or an invalid row;
//  4. a retired or never-registered desk still resolves (must be deny).

func TestRegisterRefusesDuplicateAndInvalid(t *testing.T) {
	if err := Register(Desk{Name: Texture, Kind: KindSystem, Carrier: CarrierNone, Sandbox: Sandbox{Profile: "none"}, Taint: TaintPrivileged, Restart: RestartContract}); err == nil {
		t.Fatal("Register accepted a duplicate of a built-in desk")
	}
	if err := Register(Desk{Name: "scratch", Kind: KindSystem, Taint: TaintPrivileged}); err == nil {
		t.Fatal("Register accepted a row with no restart contract")
	}
	if _, ok := Lookup("scratch"); ok {
		t.Fatal("a refused row resolved")
	}
}

func TestEveryBuiltinDeskResolvesThroughRegistry(t *testing.T) {
	for _, d := range system {
		got, ok := Lookup(d.Name)
		if !ok {
			t.Fatalf("built-in desk %s is not in the registry", d.Name)
		}
		if !got.Equal(d) {
			t.Errorf("registry row for %s differs from the built-in table", d.Name)
		}
	}
	if len(Names()) < len(system) {
		t.Fatalf("registry has %d desks, table %d", len(Names()), len(system))
	}
}

// The closed lists the registry replaced, pinned as literals.
func TestRegistryFlagsEqualTheClosedLists(t *testing.T) {
	want := func(got []string, list ...string) {
		t.Helper()
		// Other tests in the package leave app desks registered.
		var builtin []string
		for _, n := range got {
			if d, _ := Lookup(n); d.Kind != KindApp {
				builtin = append(builtin, n)
			}
		}
		got = builtin
		if len(got) == 0 && len(list) == 0 {
			return
		}
		if !reflect.DeepEqual(got, list) {
			t.Errorf("got %v, want %v", got, list)
		}
	}
	// agentprofile.Canonical and modelpolicy.NormalizeRole: the V2 live
	// vocabulary plus the management task role; the research child is a role
	// that borrows its parent's policy and is not a profile.
	want(ProfileNames(), "conductor", "email", "engineering", "management", "management_task", "research", "texture")
	// promptstore.promptRoles.
	want(PromptNames(), "conductor", "engineering", "management", "research", "texture")
	// deskCarrierLive plus the management task's registry.
	want(CellCarrierNames(), "management", "management_task", "research", "texture")

	ident := map[string]Identity{Management: IdentityOwner, Conductor: IdentityOwner, Texture: IdentitySubject}
	for _, name := range Names() {
		d, _ := Lookup(name)
		if d.Identity != ident[name] {
			t.Errorf("%s identity = %q, want %q", name, d.Identity, ident[name])
		}
		if d.OwnChannel != (name == Management) {
			t.Errorf("%s OwnChannel = %v", name, d.OwnChannel)
		}
		wantDeadline := time.Duration(0)
		if name == Management || name == ManagementTask {
			wantDeadline = 300 * time.Second
		}
		if d.CellDeadline != wantDeadline {
			t.Errorf("%s CellDeadline = %v, want %v", name, d.CellDeadline, wantDeadline)
		}
		if d.HasModule(ModuleUpdatesRead) != (name == Management || name == Texture) {
			t.Errorf("%s updates.read = %v", name, d.HasModule(ModuleUpdatesRead))
		}
	}
}

func TestAppDeskIsAProfileUntilRetired(t *testing.T) {
	d, err := NewAppDesk(AppDeskSpec{Name: "app:ma25", Verbs: []string{"Cast"}, Taint: TaintUntrustedInput, UID: 62500, WorkerMaxBytes: 64 * mib})
	if err != nil {
		t.Fatal(err)
	}
	if err := RegisterApp(d); err != nil {
		t.Fatal(err)
	}
	if got, _ := Lookup("app:ma25"); !got.Profile {
		t.Fatal("a registered app desk is not a profile")
	}
	if err := RetireApp("app:ma25"); err != nil {
		t.Fatal(err)
	}
	if _, ok := Lookup("app:ma25"); ok {
		t.Fatal("a retired app desk still resolves")
	}
}
