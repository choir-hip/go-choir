package agentcore

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/actgate"
	"github.com/yusefmosiah/go-choir/internal/trust"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Desk Casts reach the trust view (d@e, 2026-11-06: 18 desk_cast-plane
// settlements and no trust row). Ways it could fail, written down first:
//
//  1. The view reads only sub-call Casts, so a judged desk Cast's
//     settlement points at no loaded record and is dropped in silence: no
//     row for the sender, and the act gate and the lease policy see none.
//  2. The Cast is read but not in the judge's shape, so the settlement's
//     verdicts index the wrong questions (the p_success question is the
//     judge's last) and no skill is scored.
//  3. A settlement whose Cast is gone is still dropped in silence.
//  4. The Cast is counted twice (as a request and as a Cast).
func TestDeskCastSettlementsReachTheTrustViewAndTheActGate(t *testing.T) {
	f := deskJudgeFixture(t)
	ctx := context.Background()
	// A material Cast (I2), so the act gate decides in its class.
	cast := deskCast("tf-cast-1", deskTex, deskRes, "Find the primary filing for the merger.", 0.8)
	cast.ImpactClass = types.ImpactI2
	f.commitInbox(t, "tf-cast", nil, cast)
	f.commitInbox(t, "tf-final", f.readAll(t, deskRes), deskReport("tf-final-1", deskRes, deskTex, "The filing is at https://example.test/filing.", true))
	f.rt.waitSubCallJudge()
	if !f.exists(judgeSettlementID("tf-cast-1")) {
		t.Fatal("the judge did not settle the desk Cast")
	}
	// An orphan: a desk_cast-plane settlement whose Cast was never written,
	// through the same append path the judge uses.
	orphan := f.record(t, judgeSettlementID("tf-cast-1"))
	orphan.RecordID = judgeSettlementID("tf-gone-1")
	orphan.ParentID = "tf-gone-1"
	sb := *orphan.SubCall
	sb.CastID = "tf-gone-1"
	orphan.SubCall = &sb
	if err := f.rt.appendSubCallRecord(ctx, f.owner, f.computer, orphan); err != nil {
		t.Fatal(err)
	}

	in, err := f.rt.trustInputs(ctx, f.owner, f.computer)
	if err != nil {
		t.Fatal(err)
	}
	view := trust.Derive(in, trust.DefaultParams())
	class := trust.ClassOf(cast)
	if !actgate.Material(class) {
		t.Fatalf("the Cast's class %s is not material", class)
	}
	var sender *trust.Row
	callee := false
	for i, r := range view.Rows {
		if r.Subject.Kind == trust.SubjectAgent && r.Subject.ID == deskTex && r.Class == class {
			sender = &view.Rows[i]
		}
		if r.Subject.Kind == trust.SubjectAgent && r.Subject.ID == "research" && r.Class == class {
			callee = true
		}
	}
	if sender == nil || sender.N != 1 || sender.Evidence[trust.E1] != 1 {
		t.Fatalf("no single-item E1 row for the sender %s in %s: %+v", deskTex, class, view.Rows)
	}
	if !callee {
		t.Fatalf("no E5 row for the recipient: %+v", view.Rows)
	}
	missing := false
	for _, e := range view.Excluded {
		if e.RecordID == "tf-gone-1" && e.Reason == trust.ExcludedCastMissing {
			missing = true
		}
		if e.RecordID == "tf-cast-1" {
			t.Fatalf("the settled desk Cast was excluded: %+v", e)
		}
	}
	if !missing {
		t.Fatalf("the orphan settlement is not excluded as cast_missing: %+v", view.Excluded)
	}

	// The act gate and the lease policy read the row through the runtime's
	// own feed.
	f.rt.feedLeaseTrust(f.owner, f.computer)
	v := f.rt.actGate.Decide(f.owner, []string{deskTex}, "", class, true)
	if strings.Contains(v.Reason, "no record") || v.N != 1 {
		t.Fatalf("the act gate did not read the desk's row: %+v", v)
	}
	if v.Decision != actgate.Gated || v.LCB >= v.Tau {
		t.Fatalf("one settled Cast cleared tau: %+v", v)
	}
	if age, ok := f.rt.leases.TrustAge(f.owner); !ok || age > time.Minute {
		t.Fatalf("the lease policy holds no fresh trust view: %v %v", age, ok)
	}
}
