package vmmanager

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/hostfetch"
)

// Ways the switch could fail: it grants with an empty list (not off by
// default); it grants a computer it does not name; class:test grants a
// non-test computer; the grant names the wrong computer.
func TestHostFetchGrantIsPerComputerOrClassTest(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	env := func(v string) func(string) string {
		return func(k string) string {
			if k == hostFetchComputersEnv {
				return v
			}
			return ""
		}
	}
	off := loadHostFetchConfig(env(""))
	if files, _ := off.hostFetchMaterial("computer-a", "test", now); files != nil {
		t.Fatal("an empty list granted a test computer")
	}
	cfg := loadHostFetchConfig(env("computer-a, class:test"))
	cases := []struct{ id, class, basis string }{
		{"computer-a", "primary", hostfetch.BasisComputer},
		{"computer-b", "test", hostfetch.BasisClassTest},
		{"computer-c", "primary", ""},
		{"computer-d", "critical_protected", ""},
		{"", "test", ""},
	}
	for _, tc := range cases {
		files, status := cfg.hostFetchMaterial(tc.id, tc.class, now)
		if tc.basis == "" {
			if files != nil || status != "" {
				t.Fatalf("%s/%s granted: %s", tc.id, tc.class, status)
			}
			continue
		}
		var g hostfetch.Grant
		if err := json.Unmarshal(files[hostfetch.GrantFile], &g); err != nil {
			t.Fatal(err)
		}
		if g.ComputerID != tc.id || g.Status != hostfetch.GrantGranted || g.Basis != tc.basis || g.GrantedAt != "2026-10-21T12:00:00Z" || len(files) != 1 {
			t.Fatalf("%s grant = %+v", tc.id, g)
		}
	}
}
