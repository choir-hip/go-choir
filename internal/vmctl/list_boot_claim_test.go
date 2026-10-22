package vmctl

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The list shows the wake-orphan fix's row fields
// (docs/problems/wake-during-deploy-vmctl-restart-orphans-the-vm-2026-10-22.md,
// "Observed in the staging proof"). The proof plan reads boot_claim,
// start_interruptions and failure_warmed_at from the ownership row; without
// them in the list it can read them only with SQL.
//
// Failure modes pinned here:
//   - the list omits a field the row has (the proof cannot read it);
//   - the list shows an empty field for a row without it (an absent claim
//     and an empty one look alike, and every row grows noise);
//   - the claim's kind, epoch or claimed_at is dropped or mislabelled.
func TestListShowsBootClaimStartInterruptionsAndFailureWarm(t *testing.T) {
	reg := NewOwnershipRegistry("")
	addOwnership(reg, &VMOwnership{
		VMID: "vm-claimed", UserID: "claimed", ComputerID: "computer-claimed", DesktopID: PrimaryDesktopID,
		Kind: VMKindInteractive, State: VMStateStopped, StoppedBy: stoppedByStartInterruptedRepeatedly,
		BootClaim:          &BootClaim{Kind: "boot", Epoch: 7, ClaimedAt: "2026-10-22T12:15:00Z", Counted: true},
		StartInterruptions: 2,
		FailureWarmedAt:    "2026-10-22T12:15:01Z",
	})
	addOwnership(reg, &VMOwnership{
		VMID: "vm-plain", UserID: "plain", ComputerID: "computer-plain", DesktopID: PrimaryDesktopID,
		Kind: VMKindInteractive, State: VMStateStopped, StoppedBy: "idle",
	})
	h := NewHandler(reg)

	req := httptest.NewRequest(http.MethodGet, "/internal/vmctl/list", nil)
	req.Header.Set("X-Internal-Caller", "true")
	rec := httptest.NewRecorder()
	h.HandleList(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list = %d %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Ownerships []map[string]json.RawMessage `json:"ownerships"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	rows := map[string]map[string]json.RawMessage{}
	for _, row := range body.Ownerships {
		var id string
		_ = json.Unmarshal(row["vm_id"], &id)
		rows[id] = row
	}

	claimed := rows["vm-claimed"]
	if claimed == nil {
		t.Fatalf("list has no vm-claimed: %s", rec.Body.String())
	}
	var claim map[string]any
	if err := json.Unmarshal(claimed["boot_claim"], &claim); err != nil {
		t.Fatalf("boot_claim = %s: %v", claimed["boot_claim"], err)
	}
	if claim["kind"] != "boot" || claim["epoch"] != float64(7) || claim["claimed_at"] != "2026-10-22T12:15:00Z" {
		t.Fatalf("boot_claim = %v", claim)
	}
	if got := string(claimed["start_interruptions"]); got != "2" {
		t.Fatalf("start_interruptions = %q", got)
	}
	if got := string(claimed["failure_warmed_at"]); got != `"2026-10-22T12:15:01Z"` {
		t.Fatalf("failure_warmed_at = %q", got)
	}

	plain := rows["vm-plain"]
	if plain == nil {
		t.Fatalf("list has no vm-plain: %s", rec.Body.String())
	}
	for _, field := range []string{"boot_claim", "start_interruptions", "failure_warmed_at"} {
		if v, ok := plain[field]; ok {
			t.Fatalf("vm-plain carries %s = %s; want it omitted", field, v)
		}
	}
}
