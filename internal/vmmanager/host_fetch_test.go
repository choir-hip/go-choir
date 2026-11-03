package vmmanager

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/hostfetch"
)

// Ways the switch could fail: it grants with an empty list (not off by
// default); it grants a VM it does not name; it honors a class the gateway
// cannot check (the gateway admits by VM ID, the credential identity); the
// grant names the wrong computer.
func TestHostFetchGrantIsPerListedVM(t *testing.T) {
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	env := func(v string) func(string) string {
		return func(k string) string {
			if k == hostFetchVMsEnv {
				return v
			}
			return ""
		}
	}
	off := loadHostFetchConfig(env(""))
	if files, _ := off.hostFetchMaterial("computer-a", "vm-a", now); files != nil {
		t.Fatal("an empty list granted")
	}
	cfg := loadHostFetchConfig(env("vm-a, class:test"))
	if files, _ := cfg.hostFetchMaterial("computer-b", "vm-b", now); files != nil {
		t.Fatal("class:test or an unlisted VM granted")
	}
	if files, _ := cfg.hostFetchMaterial("", "vm-a", now); files != nil {
		t.Fatal("a VM with no ComputerID granted")
	}
	files, status := cfg.hostFetchMaterial("computer-a", "vm-a", now)
	var g hostfetch.Grant
	if err := json.Unmarshal(files[hostfetch.GrantFile], &g); err != nil {
		t.Fatal(err)
	}
	if g.ComputerID != "computer-a" || g.Status != hostfetch.GrantGranted || g.Basis != hostfetch.BasisVM || g.GrantedAt != "2026-10-22T12:00:00Z" || len(files) != 1 || status == "" {
		t.Fatalf("grant = %+v (%s)", g, status)
	}
}
