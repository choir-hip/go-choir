package apppackage

import (
	"errors"
	"testing"
)

// Shell-surface providers (plan section 11, H1 to H4). Ways a provider
// manifest can be wrong, each refused:
//
//	P1 a service that is not a shell slot;
//	P2 an abi that is not choir.service/<service>/v1;
//	P3 an entry that is not <Name>.surface.svelte at the package root;
//	P4 a provider without ui.shell (it draws shell UI: tier T2);
//	P5 a provider with a window (one or the other);
//	P6 the same service twice;
//	P7 a priority above 50 (rank.high needs a platform review record);
//	P8 a published provider (ui.shell is privileged: first party only);
//	P9 tier stated by the publisher (a platform fact).
func providerV(v map[string]any) {
	delete(v, "app")
	v["opens"] = []any{}
	v["capabilities"] = []any{map[string]any{"grant": "ui.shell"}}
	v["provides"] = []any{map[string]any{
		"service": "launcher", "abi": "choir.service/launcher/v1", "entry": "Launcher.surface.svelte", "requires_sdk": "^1.5",
	}}
}

func provide(v map[string]any) map[string]any { return v["provides"].([]any)[0].(map[string]any) }

func TestProvidersAreFirstPartyShellSurfaces(t *testing.T) {
	m, err := parseAs(t, OriginFirstParty, providerV)
	if err != nil {
		t.Fatalf("a good provider: %v", err)
	}
	if m.Class() != ClassProvider || m.Level() != LevelPrivileged {
		t.Fatalf("class %s level %s", m.Class(), m.Level())
	}
	bad := map[string]func(map[string]any){
		"P1 not a shell slot": func(v map[string]any) {
			provide(v)["service"] = "texture"
			provide(v)["abi"] = "choir.service/texture/v1"
		},
		"P2 wrong abi":           func(v map[string]any) { provide(v)["abi"] = "choir.service/launcher/v2" },
		"P3 entry not a surface": func(v map[string]any) { provide(v)["entry"] = "Launcher.svelte" },
		"P3 entry in a folder":   func(v map[string]any) { provide(v)["entry"] = "x/Launcher.surface.svelte" },
		"P3 entry escapes":       func(v map[string]any) { provide(v)["entry"] = "../Launcher.surface.svelte" },
		"P4 no ui.shell":         func(v map[string]any) { v["capabilities"] = []any{} },
		"P6 twice": func(v map[string]any) {
			v["provides"] = append(v["provides"].([]any), map[string]any{"service": "launcher", "abi": "choir.service/launcher/v1", "entry": "Other.surface.svelte"})
		},
		"P7 priority":    func(v map[string]any) { provide(v)["priority"] = 51 },
		"P9 tier stated": func(v map[string]any) { provide(v)["tier"] = "T2" },
		"requires_sdk":   func(v map[string]any) { provide(v)["requires_sdk"] = "1.5" },
	}
	for name, mutate := range bad {
		if _, err := parseAs(t, OriginFirstParty, func(v map[string]any) { providerV(v); mutate(v) }); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	// P5: a window and a surface in one package.
	if _, err := parseAs(t, OriginFirstParty, func(v map[string]any) {
		app := v["app"]
		providerV(v)
		v["app"] = app
	}); err == nil {
		t.Error("P5 window and surface: accepted")
	}
	// P8: no other publisher draws shell UI.
	if _, err := parseAs(t, OriginPublished, providerV); !errors.Is(err, ErrPrivilegedGrant) && !errors.Is(err, ErrInvalidManifest) {
		t.Errorf("P8 published provider: %v", err)
	}
}

func TestProvidesMoveTheBehaviourSurface(t *testing.T) {
	m1, err := parseAs(t, OriginFirstParty, providerV)
	if err != nil {
		t.Fatal(err)
	}
	m2, err := parseAs(t, OriginFirstParty, func(v map[string]any) { providerV(v); provide(v)["priority"] = 10 })
	if err != nil {
		t.Fatal(err)
	}
	src := MapSource(map[string]string{"Launcher.surface.svelte": "<p>launcher</p>"})
	h1, err := m1.BehaviourSurface(src)
	if err != nil {
		t.Fatal(err)
	}
	h2, err := m2.BehaviourSurface(src)
	if err != nil {
		t.Fatal(err)
	}
	if h1 == h2 {
		t.Fatal("a provider's priority does not move its behaviour surface")
	}
	if _, err := m1.BehaviourSurface(nil); err == nil {
		t.Fatal("a provider hashed with no source")
	}
}
