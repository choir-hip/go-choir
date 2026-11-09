package vmctl

import "testing"

// Sandbox pause (docs/design/sandbox-pause-resume-2026-11-09.md, r3): the
// guest's "pausing" and "paused, awaiting ack" refusals carry holds_new_work,
// so the host keeps asking once a minute, under the pre-ack TTL, instead of
// backing off. The bodies below are the ones the guest writes
// (agentcore.plannedRestartBusyResponse); the guest side pins the field.
//
// Ways this fails: a refusal without the field backs the sweep off past the
// TTL, so the pause is unpaused before the host asks again, for ever.
func TestGuestPausingRefusalsKeepTheOneMinuteCadence(t *testing.T) {
	for name, body := range map[string]string{
		"pausing":             `{"error":"live capsules are pausing for the restart","outcome":"busy","reason":"pausing","allow_busy":true,"running_runs":1,"selfdev_active_operations":1,"live_capsules":1,"holds_new_work":true}`,
		"paused awaiting ack": `{"error":"a paused capsule is not yet acknowledgeable","outcome":"busy","reason":"paused_awaiting_ack","allow_busy":true,"live_capsules":1,"holds_new_work":true}`,
	} {
		if !guestRefusalHoldsNewWork(body) {
			t.Errorf("%s: the sweep would back off", name)
		}
	}
	if guestRefusalHoldsNewWork(`{"outcome":"busy","live_capsules":1}`) {
		t.Error("an ordinary busy refusal holds new work")
	}
}
