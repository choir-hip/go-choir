package yaegikernel

import (
	"strings"
	"testing"
)

// The deployed forged-frame probe, run here against this tree: every desk's
// served/refused partition is the pinned table; every forged case is refused;
// a wrong slot token ends the worker; no refused frame reached a handler. The
// staging script judges the same receipt from the guest.
func TestFrameBoundarySelfTestReceipt(t *testing.T) {
	rec := FrameBoundarySelfTest()
	if len(rec.Roles) != len(FrameBoundaryRoles) {
		t.Fatalf("roles = %d", len(rec.Roles))
	}
	for _, r := range rec.Roles {
		if r.Error != "" {
			t.Fatalf("%s: %s", r.Role, r.Error)
		}
		want := map[string]bool{}
		for _, a := range pinnedFrameActions[r.Role] {
			want[string(a)] = true
		}
		for _, a := range r.Served {
			if !want[a] {
				t.Errorf("%s: %s was served, not in the pinned table", r.Role, a)
			}
			delete(want, a)
		}
		for a := range want {
			t.Errorf("%s: %s is in the pinned table but was refused", r.Role, a)
		}
		claim := map[string]string{}
		if r.Role != "management" {
			claim["claimed_other_desk"] = r.ClaimedOtherDesk
		}
		for name, v := range claim {
			if !strings.HasPrefix(v, "refused") {
				t.Errorf("%s: %s = %q, want a refusal", r.Role, name, v)
			}
		}
		for name, v := range map[string]string{"wrong_token": r.WrongToken, "missing_token": r.MissingToken,
			"foreign_token": r.ForeignToken, "stale_epoch": r.StaleEpoch, "bad_protocol": r.BadProtocol, "bad_version_header": r.BadVersionHeader} {
			if !strings.HasPrefix(v, "refused") {
				t.Errorf("%s: %s = %q, want a refusal", r.Role, name, v)
			}
		}
		if !r.WrongTokenKilled {
			t.Errorf("%s: a wrong slot token did not end the worker", r.Role)
		}
		if r.RefusedReachedHandler != 0 {
			t.Errorf("%s: %d refused frames reached a handler", r.Role, r.RefusedReachedHandler)
		}
		if r.RefusalsReported == 0 {
			t.Errorf("%s: no refusal was reported to the host", r.Role)
		}
	}
}

// The frame probe names every action a frame can carry and none other: the
// emit and message actions were deleted with their channels (MA1.1), so a
// frame naming one fails protocol validation, not only the role table.
func TestFrameBoundaryActionsAreTheLiveSetOnly(t *testing.T) {
	want := map[BrokerAction]bool{}
	for _, a := range allBrokerActions {
		want[a] = true
	}
	got := map[BrokerAction]bool{}
	for _, a := range FrameBoundaryActions {
		if got[a] {
			t.Errorf("FrameBoundaryActions names %s twice", a)
		}
		got[a] = true
		if !want[a] {
			t.Errorf("FrameBoundaryActions names %s, which the pinned set does not", a)
		}
	}
	for a := range want {
		if !got[a] {
			t.Errorf("FrameBoundaryActions omits %s", a)
		}
	}
	for _, deleted := range []BrokerAction{"emit", "message"} {
		req := BrokerRequest{ProtocolVersion: ProtocolVersion, RequestID: "r", HandleRef: "h", Epoch: 1, Action: deleted}
		if err := req.Validate(); err == nil {
			t.Errorf("a frame naming the deleted action %q passed validation", deleted)
		}
	}
	for _, a := range allBrokerActions {
		req := BrokerRequest{ProtocolVersion: ProtocolVersion, RequestID: "r", HandleRef: "h", Epoch: 1, Action: a}
		if err := req.Validate(); err != nil {
			t.Errorf("live action %s failed validation: %v", a, err)
		}
	}
}
