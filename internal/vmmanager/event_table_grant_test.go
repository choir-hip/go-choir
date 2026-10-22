package vmmanager

// The host selector of store.event_table (event-table design 4.2, deploy 2).
// Ways it can fail, written before the code:
//   - an empty selector writes anything onto any credential disk (deploy 2
//     must be byte-identical for every computer until FP0 names one);
//   - a computer the selector does not name is granted;
//   - class:test or "*" grants a computer: the selector names ComputerIDs
//     only, because turning the writer on makes deploy 1 that
//     computer's rollback floor, a per-computer platform decision;
//   - the grant names a computer other than the one it is written for.

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/eventtablegrant"
)

func TestEventTableGrantNamesComputersOnly(t *testing.T) {
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	env := func(v string) func(string) string {
		return func(k string) string {
			if k == eventTableComputersEnv {
				return v
			}
			return ""
		}
	}
	off := loadEventTableConfig(env(""))
	for _, tc := range []struct{ id, class string }{{"computer-a", "test"}, {"computer-a", "primary"}, {"", "test"}} {
		if files, status := off.eventTableMaterial(tc.id, tc.class, now); files != nil || status != "" {
			t.Fatalf("an empty selector granted %s/%s: %s", tc.id, tc.class, status)
		}
	}
	cfg := loadEventTableConfig(env(" computer-a, class:test *\ncomputer-b "))
	for _, tc := range []struct {
		id, class string
		granted   bool
	}{
		{"computer-a", "primary", true},
		{"computer-b", "critical_protected", true},
		{"computer-c", "test", false},
		{"", "test", false},
	} {
		files, status := cfg.eventTableMaterial(tc.id, tc.class, now)
		if !tc.granted {
			if files != nil || status != "" {
				t.Fatalf("%s/%s granted: %s", tc.id, tc.class, status)
			}
			continue
		}
		var g eventtablegrant.Grant
		if err := json.Unmarshal(files[eventtablegrant.GrantFile], &g); err != nil {
			t.Fatal(err)
		}
		if g.ComputerID != tc.id || g.Status != eventtablegrant.GrantGranted || g.Basis != eventtablegrant.BasisComputer || g.GrantedAt != "2026-10-22T12:00:00Z" || len(files) != 1 || status == "" {
			t.Fatalf("%s grant = %+v (%s)", tc.id, g, status)
		}
	}
}
