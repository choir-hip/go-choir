package coagentpacket

import (
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// M11 rerun 10 (2026-10-10 07:38–07:40Z): Texture spent 16 reducer
// round-trips guessing update_coagent enum values ("repair", "command",
// "zzznotype", mutation_class "reversible", "capsule", ...) because each
// rejection said only "is not supported". A rejected enum must name the
// values it accepts, so one correction suffices.
func TestEnumRejectionsNameAllowedValues(t *testing.T) {
	cases := []struct {
		name  string
		err   error
		allow []string
	}{
		{"action type", validateCoagentPacketAction(types.CoagentPacketAction{Type: "repair", Objective: "x"}, false), coagentActionTypes},
		{"mutation class", validateCoagentPacketAction(types.CoagentPacketAction{Type: "produce_diff", Objective: "x", Safety: types.CoagentPacketActionSafety{MutationClass: "capsule", Network: "forbidden", FileMutation: "allowed"}}, true), mutationClasses},
		{"network", validateCoagentPacketAction(types.CoagentPacketAction{Type: "produce_diff", Objective: "x", Safety: types.CoagentPacketActionSafety{MutationClass: "orange", Network: "maybe", FileMutation: "allowed"}}, true), coagentActionSafetyModes},
		{"file mutation", validateCoagentPacketAction(types.CoagentPacketAction{Type: "produce_diff", Objective: "x", Safety: types.CoagentPacketActionSafety{MutationClass: "orange", Network: "forbidden", FileMutation: "true"}}, true), coagentActionSafetyModes},
		{"stance", validateCoagentPacketClaim(types.CoagentPacketClaim{Text: "x", Stance: "agrees"}, nil), coagentClaimStances},
		{"recommended surface", validateCoagentPacketClaim(types.CoagentPacketClaim{Text: "x", RecommendedSurface: "popup"}, nil), coagentClaimRecommendedSurfaces},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.err == nil {
				t.Fatal("expected a rejection")
			}
			for _, value := range tc.allow {
				if !strings.Contains(tc.err.Error(), value) {
					t.Fatalf("rejection %q does not name allowed value %q", tc.err, value)
				}
			}
		})
	}
	if err := Validate(types.CoagentSourcePacketPayload{SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: "repair"}); err == nil || !strings.Contains(err.Error(), "execution_request") {
		t.Fatalf("packet kind rejection should name allowed kinds, got %v", err)
	}
}

// Cut 2 amendment A2: research names the capture a source came from
// (choir.Read's capture_id) beside its URL. Failure mode pinned: the field
// is dropped when the packet is normalized, so the quote cannot be joined to
// the capture it was copied from.
func TestNormalizeKeepsTheSourceCaptureID(t *testing.T) {
	packet := Normalize(types.CoagentSourcePacketPayload{Sources: []types.CoagentPacketSource{{
		SourceID: "s1", Kind: "web_source",
		Target: types.CoagentPacketSourceTarget{URI: "https://example.com/a", CaptureID: "  choir.web_capture:owner:abc  "},
	}}})
	if got := packet.Sources[0].Target.CaptureID; got != "choir.web_capture:owner:abc" {
		t.Fatalf("capture_id = %q", got)
	}
}
