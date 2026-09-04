package store

import (
	"testing"
)

func boolPtr(v bool) *bool { return &v }

// TestTextureRevisionPresenceConflict pins the revision-presence guard matrix,
// including the replay-only tolerance for an op that preserves the projected
// revision byte-for-byte (2026-09-03 seq 138612: sleep-after-non-revision
// turn for a mutation created with its revision attached).
func TestTextureRevisionPresenceConflict(t *testing.T) {
	cases := []struct {
		name     string
		existing string
		require  *bool
		opRev    string
		replay   bool
		conflict bool
	}{
		{"nil guard never conflicts", "rev-1", nil, "rev-1", false, false},
		{"require true with revision agrees", "rev-1", boolPtr(true), "rev-1", false, false},
		{"require false without revision agrees", "", boolPtr(false), "", false, false},
		{"live require true without revision conflicts", "", boolPtr(true), "", false, true},
		{"live require false with revision conflicts", "rev-1", boolPtr(false), "rev-1", false, true},
		{"replay require true without revision still conflicts", "", boolPtr(true), "", true, true},
		{"replay require false clearing revision still conflicts", "rev-1", boolPtr(false), "", true, true},
		{"replay require false swapping revision still conflicts", "rev-1", boolPtr(false), "rev-2", true, true},
		{"replay require false preserving revision tolerated", "rev-1", boolPtr(false), "rev-1", true, false},
		{"replay tolerance is exact match only", "rev-1", boolPtr(false), " rev-1 ", true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := textureRevisionPresenceConflict(tc.existing, tc.require, tc.opRev, tc.replay); got != tc.conflict {
				t.Fatalf("textureRevisionPresenceConflict(%q, %v, %q, replay=%v) = %v, want %v",
					tc.existing, tc.require, tc.opRev, tc.replay, got, tc.conflict)
			}
		})
	}
}

func TestSleepAfterTurnRequireRevision(t *testing.T) {
	if sleepAfterTurnRequireRevision(nil) {
		t.Fatal("missing row derives false")
	}
	if !sleepAfterTurnRequireRevision(&AgentMutation{RevisionID: "f1511357-seq-138612"}) {
		t.Fatal("revision-carrying row derives true (preserve through sleep)")
	}
	if sleepAfterTurnRequireRevision(&AgentMutation{RevisionID: ""}) {
		t.Fatal("row without revision derives false")
	}
	if sleepAfterTurnRequireRevision(&AgentMutation{RevisionID: "  "}) {
		t.Fatal("whitespace revision derives false")
	}
}
