package yaegikernel

import "testing"

// Desk cell confinement (docs/problems/desk-cells-run-as-guest-root-and-egress-is-open-2026-10-14.md).
//
// Ways the policy half could fail, written first:
//  1. a desk role maps to no identity, or two desks share one, so a cell
//     of one desk can signal or read another desk's worker;
//  2. an unknown role (a typo, a new desk) silently gets some identity
//     instead of refusing;
//  3. a malformed CHOIR_DESK_CELL_CONFINEMENT value weakens the policy
//     instead of strengthening it.

func TestDeskCellUserIsOnePerDesk(t *testing.T) {
	cases := map[string]string{
		"management":              "choir-cell-management",
		SessionRoleManagementTask: "choir-cell-management",
		"texture":                 "choir-cell-texture",
		"research":                "choir-cell-research",
		SessionRoleResearchChild:  "choir-cell-research",
	}
	for role, want := range cases {
		if got := DeskCellUser(role); got != want {
			t.Errorf("DeskCellUser(%q) = %q, want %q", role, got, want)
		}
	}
	for _, role := range []string{"", "engineering", "Management", "root", "texture "} {
		if got := DeskCellUser(role); got != "" {
			t.Errorf("DeskCellUser(%q) = %q, want no identity (refuse)", role, got)
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
