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
	if err := register(Desk{Name: Texture, Kind: KindSystem, Carrier: CarrierNone, Sandbox: Sandbox{Profile: "none"}, Taint: TaintPrivileged, Restart: RestartContract}); err == nil {
		t.Fatal("Register accepted a duplicate of a built-in desk")
	}
	if err := register(Desk{Name: "scratch", Kind: KindSystem, Taint: TaintPrivileged}); err == nil {
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
	want(ProfileNames(), "conductor", "engineering", "management", "management_task", "research", "texture")
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

func TestRegisterRefusesReservedProfileNames(t *testing.T) {
	for _, n := range []string{"verifier", "verifier_multimodal", "processor", "reconciler"} {
		d := Desk{Name: n, Kind: KindSystem, Carrier: CarrierNone, Sandbox: Sandbox{Profile: "none"}, Taint: TaintPrivileged, Restart: RestartContract}
		if err := register(d); err == nil {
			t.Errorf("register accepted the reserved name %s", n)
		}
	}
}

// RegisterApp is the only door for a row from outside the built-in table; each
// row below carries one authority an app desk does not have, and each is
// refused, never trimmed.
func TestRegisterAppRefusesAuthorityBeyondAnApp(t *testing.T) {
	base := func() Desk {
		d, err := NewAppDesk(AppDeskSpec{Name: "app:strict", Verbs: []string{"Cast", "Done"}, Taint: TaintUntrustedInput, UID: 62600, WorkerMaxBytes: 64 * mib})
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	cases := map[string]func(*Desk){
		"exec verb":        func(d *Desk) { d.Verbs = append(d.Verbs, "Exec") },
		"write_file verb":  func(d *Desk) { d.Verbs = append(d.Verbs, "WriteFile") },
		"product api verb": func(d *Desk) { d.Verbs = append(d.Verbs, "ProductAPI") },
		"capsule verbs":    func(d *Desk) { d.CapsuleVerbs = []string{"exec"} },
		"modules":          func(d *Desk) { d.Modules = []string{ModuleUpdatesRead} },
		"prompt":           func(d *Desk) { d.Prompt = true },
		"identity":         func(d *Desk) { d.Identity = IdentityOwner },
		"own channel":      func(d *Desk) { d.OwnChannel = true },
		"cell deadline":    func(d *Desk) { d.CellDeadline = time.Second },
		"profile off":      func(d *Desk) { d.Profile = false },
		"system uid":       func(d *Desk) { d.Sandbox.UID = 61999 },
		"capsule carrier":  func(d *Desk) { d.Carrier = CarrierCapsule },
		"no memory budget": func(d *Desk) { d.Memory.WorkerMaxBytes = 0 },
		"huge memory":      func(d *Desk) { d.Memory.WorkerMaxBytes = 1 << 40 },
		"capsule memory":   func(d *Desk) { d.Memory.CapsuleRequestedBytes = 1 << 30 },
		"bad name":         func(d *Desk) { d.Name = "app:Bad Name" },
		"system kind":      func(d *Desk) { d.Kind = KindSystem },
		"role kind":        func(d *Desk) { d.Kind = KindRole },
	}
	for name, mutate := range cases {
		d := base()
		mutate(&d)
		if err := RegisterApp(d); err == nil {
			RetireApp(d.Name)
			t.Errorf("%s: RegisterApp accepted the row", name)
		}
	}
	if _, ok := Lookup("app:strict"); ok {
		t.Fatal("a refused row resolved")
	}
	if err := RegisterApp(base()); err != nil {
		t.Fatalf("the canonical row was refused: %v", err)
	}
	defer RetireApp("app:strict")
	if err := RegisterApp(base()); err == nil {
		t.Fatal("RegisterApp accepted a duplicate")
	}
}

func TestAppDeskRefusesReservedSlugs(t *testing.T) {
	for _, slug := range []string{"texture", "management", "engineering", "research", "conductor", "email", "mail", "calendar", "slides", "management-task"} {
		_, err := NewAppDesk(AppDeskSpec{Name: "app:" + slug, Verbs: []string{"Cast"}, Taint: TaintUntrustedInput, UID: 62601, WorkerMaxBytes: 64 * mib})
		if err == nil {
			t.Errorf("NewAppDesk accepted the reserved slug %s", slug)
		}
	}
}

func TestProfileReadDoesNotClone(t *testing.T) {
	if p, r := Profile(ManagementTask); !p || !r {
		t.Fatalf("management_task profile/role = %v %v", p, r)
	}
	if p, r := Profile(Texture); !p || r {
		t.Fatalf("texture profile/role = %v %v", p, r)
	}
	for _, role := range []string{ResearchWeb, ResearchTexture, "research_child"} {
		if p, _ := Profile(role); p {
			t.Fatalf("%s is a profile", role)
		}
	}
	if p, _ := Profile("nobody"); p {
		t.Fatal("unknown is a profile")
	}
}
