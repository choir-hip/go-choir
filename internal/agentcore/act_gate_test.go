package agentcore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/actgate"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/lease"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/trust"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// MA7.8 in the runtime. Ways the act gate and the breaker could fail
// (written first):
//
//  1. The gate is only in tests: the act path (commitActIntent) does not
//     consult it, so a package desk's material act executes with no record.
//  2. A gated act writes its record or its envelope anyway.
//  3. The gated act is not a tape fact, or management is not told, or the
//     refusal does not name the owner's tap.
//  4. An owner approval does not let the retry pass, or lets it pass twice; a
//     denial is forgotten; the answers are not trust evidence.
//  5. A first-party desk is blocked (the paper stops), or spends a trust
//     derivation per act; a non-material act of a package desk is gated.
//  6. A forged frame does not trip the breaker: no lease ends, the class
//     stays autonomous, nothing reaches the tape.
//  7. The trip is lost at a restart: the next trust view does not carry it,
//     so autonomy returns without new evidence.

type actGateFixture struct {
	*disclosureFixture
}

func newActGateFixture(t *testing.T, slug string, uid uint32) *actGateFixture {
	t.Helper()
	f := &actGateFixture{newDisclosureFixture(t, slug, uid)}
	actgate.ResetCounts()
	t.Cleanup(actgate.ResetCounts)
	return f
}

func (f *actGateFixture) reduction(exec toolregistry.ExecutionContext) *rlmCallReduction {
	return &rlmCallReduction{mb: f.rt, ledger: f.s, rec: exec.RunRecord, scope: ReductionScope{
		RunID: exec.RunID, OwnerID: f.owner, ComputerID: f.computer, DeskAgentID: exec.AgentID, FromAgentID: exec.AgentID, CellID: "cell-" + exec.RunID,
	}}
}

// precommit stages a Precommit of class impact.
func precommitIntent(id string, impact types.ImpactClass) yaegikernel.StagedIntent {
	w := 0.5
	body, _ := json.Marshal(types.CommitmentPrecommit{Question: "will the publish land", Distribution: map[string]float64{"yes": 0.7, "no": 0.3},
		Weight: &w, ImpactClass: impact})
	return yaegikernel.StagedIntent{LocalID: id, Kind: yaegikernel.IntentPrecommit, Precommit: string(body)}
}

func (f *actGateFixture) gatedActs(t *testing.T) []types.EventRecord {
	t.Helper()
	var out []types.EventRecord
	for _, e := range f.events(t) {
		var p disclosurePayload
		if json.Unmarshal(e.Payload, &p) == nil && p.Sink == actGateSink {
			out = append(out, e)
		}
	}
	return out
}

func (f *actGateFixture) managementNotices(t *testing.T) []types.CommitmentRecord {
	t.Helper()
	recs, err := f.s.ListCommitmentRecords(context.Background(), f.owner, f.computer, "", 1000)
	if err != nil {
		t.Fatal(err)
	}
	var out []types.CommitmentRecord
	for _, r := range recs {
		if r.Provenance.AgentID == actGateAgentID {
			out = append(out, r)
		}
	}
	return out
}

// 1, 2, 3: a package desk's I2 act with no record is held on the act path.
func TestAPackageDesksMaterialActIsGatedOnTheActPath(t *testing.T) {
	f := newActGateFixture(t, "gated", 62851)
	exec := f.run(t, "run-gated", f.desk, "make the act")
	r := f.reduction(exec)
	_, err := r.commitActIntent(context.Background(), precommitIntent("i1", types.ImpactI2))
	if err == nil || !strings.Contains(err.Error(), "act gated") || !strings.Contains(err.Error(), "/api/disclosures/") {
		t.Fatalf("a thin package desk's I2 act = %v", err)
	}
	if recs, _ := f.s.ListCommitmentRecords(context.Background(), f.owner, f.computer, "", 1000); len(recs) != len(f.managementNotices(t)) {
		t.Fatalf("the gated act wrote a record: %d records, %d notices", len(recs), len(f.managementNotices(t)))
	}
	gated := f.gatedActs(t)
	if len(gated) != 1 || gated[0].Phase != disclosurePhasePending {
		t.Fatalf("gated facts = %v", gated)
	}
	var p disclosurePayload
	_ = json.Unmarshal(gated[0].Payload, &p)
	if p.Gate == nil || p.Gate.Decision != actgate.Gated || p.Gate.Class.Impact != types.ImpactI2 || p.Desk != f.desk {
		t.Fatalf("payload = %+v", p)
	}
	if n := f.managementNotices(t); len(n) != 1 || n[0].Inbox != persistentManagementAgentID(f.owner) || !strings.Contains(n[0].Directive.Body, gated[0].EventID) {
		t.Fatalf("management notices = %+v", n)
	}
	if actgate.Counts()["gated|I2|enforced"] != 1 {
		t.Fatalf("counts = %v", actgate.Counts())
	}
	// The owner sees it as a sentence.
	h := &APIHandler{rt: f.rt}
	req := httptest.NewRequest(http.MethodGet, "/api/disclosures", nil)
	req.Header.Set("X-Authenticated-User", f.owner)
	w := httptest.NewRecorder()
	h.HandleDisclosures(w, req)
	if !strings.Contains(w.Body.String(), "The act gate holds it") {
		t.Fatalf("owner list = %s", w.Body.String())
	}
}

// 4: an approval lets the same act pass once; a denial holds; both are E6.
func TestTheOwnersAnswerToAGatedActPassesOnceAndIsEvidence(t *testing.T) {
	f := newActGateFixture(t, "tapped", 62852)
	exec := f.run(t, "run-tap", f.desk, "x")
	r := f.reduction(exec)
	in := precommitIntent("i1", types.ImpactI3)
	rec := commitmentRecordForIntent(r.scope, in)
	if err := r.gateAct(context.Background(), in, rec); err == nil {
		t.Fatal("not gated")
	}
	id := f.gatedActs(t)[0].EventID
	if _, err := f.rt.answerDisclosure(context.Background(), f.owner, id, true); err != nil {
		t.Fatal(err)
	}
	if err := r.gateAct(context.Background(), in, rec); err != nil {
		t.Fatalf("the approved act did not pass: %v", err)
	}
	if err := r.gateAct(context.Background(), in, rec); err == nil {
		t.Fatal("the approval passed the act twice")
	}
	// A different act is not covered by that approval; deny it.
	other := precommitIntent("i2", types.ImpactI3)
	other.Precommit = strings.Replace(other.Precommit, "publish", "release", 1)
	orec := commitmentRecordForIntent(r.scope, other)
	_ = r.gateAct(context.Background(), other, orec)
	var otherID string
	for _, e := range f.gatedActs(t) {
		var p disclosurePayload
		_ = json.Unmarshal(e.Payload, &p)
		if e.Phase == disclosurePhasePending && strings.Contains(p.Summary, "release") {
			otherID = e.EventID
		}
	}
	if otherID == "" {
		t.Fatal("the second act was not gated")
	}
	if _, err := f.rt.answerDisclosure(context.Background(), f.owner, otherID, false); err != nil {
		t.Fatal(err)
	}
	if err := r.gateAct(context.Background(), other, orec); err == nil || !strings.Contains(err.Error(), "denied") {
		t.Fatalf("denied act = %v", err)
	}
	in2, err := f.rt.trustInputs(context.Background(), f.owner, f.computer)
	if err != nil {
		t.Fatal(err)
	}
	acts := map[string]int{}
	for _, a := range in2.OwnerActs {
		if a.Desk == f.desk && a.Class.Impact == types.ImpactI3 {
			acts[a.Act]++
		}
	}
	if acts["approve"] != 1 || acts["reject"] != 1 {
		t.Fatalf("owner acts = %+v", in2.OwnerActs)
	}
}

// 5: a first-party desk's material act passes and is counted; a package
// desk's I1 act is not gated.
func TestFirstPartyAndNonMaterialActsPass(t *testing.T) {
	f := newActGateFixture(t, "plain", 62853)
	mgmt := f.run(t, "run-mgmt", agentprofile.Management, "x")
	if err := f.reduction(mgmt).gateAct(context.Background(), precommitIntent("i1", types.ImpactI4), commitmentRecordForIntent(f.reduction(mgmt).scope, precommitIntent("i1", types.ImpactI4))); err != nil {
		t.Fatalf("a first-party desk was gated: %v", err)
	}
	app := f.run(t, "run-app", f.desk, "x")
	r := f.reduction(app)
	if err := r.gateAct(context.Background(), precommitIntent("i2", types.ImpactI1), commitmentRecordForIntent(r.scope, precommitIntent("i2", types.ImpactI1))); err != nil {
		t.Fatalf("an I1 act was gated: %v", err)
	}
	if len(f.gatedActs(t)) != 0 {
		t.Fatal("a passing act reached the tape")
	}
	if _, fed := f.rt.leaseTrustFed[f.owner]; fed {
		t.Fatal("a first-party or non-material act derived a trust view")
	}
	if actgate.Counts()["material|I4|observed"] != 1 {
		t.Fatalf("counts = %v", actgate.Counts())
	}
}

// 6, 7: a forged frame trips the breaker: the desk's lease ends at once, its
// classes drop, the trip is on the tape, management is told, and the trust
// view reads it back as an open breaker.
func TestAForgedFrameTripsTheBreaker(t *testing.T) {
	f := newActGateFixture(t, "forger", 62854)
	exec := f.run(t, "run-forge", f.desk, "x")
	held, err := f.rt.RequestLease(context.Background(), exec.RunRecord, lease.Scope{CaptureIDs: []string{"cap-1"}}, "read one capture", 0, leaseFields())
	if err != nil {
		t.Fatal(err)
	}
	f.rt.recordDeskRefusal(context.Background(), exec, "session-1", yaegikernel.FrameRefusal{Role: f.desk, Action: yaegikernel.ActionReadCapture,
		Reason: "slot token mismatch", WorkerPID: 4242, Forged: true})
	for _, l := range f.rt.leases.Live(f.owner) {
		if l.LeaseID == held.LeaseID {
			t.Fatal("the breaker did not end the desk's lease")
		}
	}
	if _, dropped := f.rt.actGate.Dropped(f.desk, trust.ActionClass{Grant: "desk.request", Impact: types.ImpactI2}); !dropped {
		t.Fatal("the breaker did not drop the desk's classes")
	}
	trips, err := f.s.ListAcknowledgedEventsByChannel(context.Background(), f.owner, trustBreakerChannel(f.computer), 100)
	if err != nil || len(trips) != 1 || trips[0].Kind != types.EventTrustBreaker || trips[0].Phase != breakerCauseForgedFrame {
		t.Fatalf("trips = %v, %v", trips, err)
	}
	if n := f.managementNotices(t); len(n) != 1 || !strings.Contains(n[0].Directive.Body, "circuit breaker") {
		t.Fatalf("notices = %+v", n)
	}
	if actgate.Counts()["trip|forged_frame"] != 1 {
		t.Fatalf("counts = %v", actgate.Counts())
	}
	// 7: the view derived from the tape (what a restarted computer reads)
	// carries the trip as an open breaker in the desk's grants.
	in, err := f.rt.trustInputs(context.Background(), f.owner, f.computer)
	if err != nil {
		t.Fatal(err)
	}
	view := trust.Derive(in, trust.DefaultParams())
	open := false
	for _, row := range view.Rows {
		if row.Subject == (trust.Subject{Kind: trust.SubjectAgent, ID: f.desk}) && row.Class.Grant == "desk.request" && row.Breaker {
			open = true
		}
	}
	if !open {
		t.Fatalf("the view does not carry the trip: %+v", view.Rows)
	}
	// A fresh gate (a restart) fed that view gates the desk's material act.
	g := actgate.New(nil)
	g.Observe(f.owner, view.Rows)
	if v := g.Decide(f.owner, []string{f.desk}, "", trust.ActionClass{Grant: "desk.request", Impact: types.ImpactI2}, true); v.Decision != actgate.Gated || !v.Breaker {
		t.Fatalf("after a restart: %+v", v)
	}
}

// A package desk whose record in the class clears tau acts without the gate,
// and the same desk after a trip is gated again.
func TestAStrongRecordIsAutonomousUntilTheBreakerTrips(t *testing.T) {
	f := newActGateFixture(t, "trusted", 62855)
	exec := f.run(t, "run-trusted", f.desk, "x")
	r := f.reduction(exec)
	in := precommitIntent("i1", types.ImpactI2)
	rec := commitmentRecordForIntent(r.scope, in)
	class := trust.ActionOf(rec)
	// The view the runtime would derive, pushed as the feed pushes it; the
	// feed's clock is set so this act does not derive again.
	f.rt.actGate.Observe(f.owner, []trust.Row{{Subject: trust.Subject{Kind: trust.SubjectAgent, ID: f.desk}, Class: class, LCB: 0.93, N: 120}})
	f.rt.leaseTrustFed = map[string]time.Time{f.owner: time.Now()}
	if err := r.gateAct(context.Background(), in, rec); err != nil {
		t.Fatalf("a strong record was gated: %v", err)
	}
	f.rt.tripBreaker(context.Background(), f.owner, f.computer, f.desk, class, "test", exec.RunID, "")
	if err := r.gateAct(context.Background(), in, rec); err == nil || !strings.Contains(err.Error(), "circuit breaker") {
		t.Fatalf("after a trip = %v", err)
	}
}
