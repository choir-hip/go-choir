package yaegikernel

import "testing"

// Desk cell confinement (docs/problems/desk-boundary-residuals-2026-10-21.md).
//
// Ways the policy half could fail, written first:
//  1. a desk role maps to no identity, or two desks share one, so a cell
//     of one desk can signal or read another desk's worker;
//  2. an unknown role (a typo, a new desk) silently gets some identity
//     instead of refusing;
//  3. a malformed CHOIR_DESK_CELL_CONFINEMENT value weakens the policy
//     instead of strengthening it.

func TestDeskCellIdentityIsOnePerDesk(t *testing.T) {
	uid := func(role string) uint32 {
		id, ok := DeskCellIdentity(role)
		if !ok || id == 0 {
			t.Fatalf("DeskCellIdentity(%q) = %d, %v; want a non-zero identity", role, id, ok)
		}
		return id
	}
	// A task shares management's identity and a research child research's;
	// the three desks are distinct from one another.
	if uid("management") != uid(SessionRoleManagementTask) || uid("research") != uid(SessionRoleResearchChild) {
		t.Error("a role must run as its desk's identity")
	}
	if uid("management") == uid("texture") || uid("texture") == uid("research") || uid("management") == uid("research") {
		t.Error("two desks share one cell identity")
	}
	for _, role := range []string{"", "engineering", "Management", "root", "texture ", "conductor", "email", "app:never-registered"} {
		if id, ok := DeskCellIdentity(role); ok {
			t.Errorf("DeskCellIdentity(%q) = %d, want no identity (refuse)", role, id)
		}
	}
}

func TestDeskCellConfinementModeFromEnv(t *testing.T) {
	cases := map[string]CellConfinementMode{
		"":         CellConfinementAuto,
		"auto":     CellConfinementAuto,
		" auto ":   CellConfinementAuto,
		"required": CellConfinementRequired,
		"REQUIRED": CellConfinementRequired,
		"off":      CellConfinementRequired, // no opt-out: unknown values strengthen
		"requried": CellConfinementRequired,
		"best":     CellConfinementRequired,
	}
	for value, want := range cases {
		got := DeskCellConfinementModeFromEnv(func(k string) string {
			if k == DeskCellConfinementEnv {
				return value
			}
			return ""
		})
		if got != want {
			t.Errorf("mode for %q = %q, want %q", value, got, want)
		}
	}
}
