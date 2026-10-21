package vmmanager

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/mirrorgrant"
)

// Station FP10 F12: the mirror switch turns on only for a computer the host
// names (or class:test), only with a known ecosystem, and is off by default.
func TestCapsuleMirrorGrantIsPerComputerAndOffByDefault(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	env := func(computers, ecosystems, only string) func(string) string {
		return func(k string) string {
			return map[string]string{
				capsuleMirrorComputersEnv:  computers,
				capsuleMirrorEcosystemsEnv: ecosystems,
				capsuleMirrorsOnlyEnv:      only,
			}[k]
		}
	}
	for _, off := range []CapsuleMirrorConfig{
		loadCapsuleMirrorConfig(env("", "", "")),
		loadCapsuleMirrorConfig(env("", "npm,pypi", "1")),
		loadCapsuleMirrorConfig(env("computer-a,class:test", "", "")),
		loadCapsuleMirrorConfig(env("computer-a,class:test", "maven", "")),
	} {
		for _, class := range []string{"primary", "test"} {
			if files, status := off.capsuleMirrorMaterial("computer-a", class, now); files != nil || status != "" {
				t.Fatalf("%+v granted computer-a (%s): %s", off, class, status)
			}
		}
	}

	cfg := loadCapsuleMirrorConfig(env("computer-a, class:test", "pypi npm,git", "true"))
	cases := []struct{ id, class, basis string }{
		{"computer-a", "primary", mirrorgrant.BasisComputer},
		{"computer-b", "test", mirrorgrant.BasisClassTest},
		{"computer-c", "primary", ""},
		{"computer-d", "critical_protected", ""},
		{"", "test", ""},
	}
	for _, tc := range cases {
		files, status := cfg.capsuleMirrorMaterial(tc.id, tc.class, now)
		if tc.basis == "" {
			if files != nil || status != "" {
				t.Fatalf("%s/%s granted: %s", tc.id, tc.class, status)
			}
			continue
		}
		var g mirrorgrant.Grant
		if err := json.Unmarshal(files[mirrorgrant.GrantFile], &g); err != nil {
			t.Fatal(err)
		}
		if g.ComputerID != tc.id || g.Status != mirrorgrant.GrantGranted || g.Basis != tc.basis || !g.MirrorsOnly ||
			!reflect.DeepEqual(g.Ecosystems, []string{"git", "npm", "pypi"}) || g.GrantedAt != "2026-10-21T12:00:00Z" || len(files) != 1 {
			t.Fatalf("%s grant = %+v", tc.id, g)
		}
	}
	// Mirrors only is its own switch: off unless set.
	routeOnly := loadCapsuleMirrorConfig(env("computer-a", "npm", ""))
	files, _ := routeOnly.capsuleMirrorMaterial("computer-a", "primary", now)
	var g mirrorgrant.Grant
	if err := json.Unmarshal(files[mirrorgrant.GrantFile], &g); err != nil || g.MirrorsOnly {
		t.Fatalf("mirrors only without the switch: %+v %v", g, err)
	}
}
