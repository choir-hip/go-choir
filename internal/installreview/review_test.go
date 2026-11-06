package installreview

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/apppackage"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/modelcatalog"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The install review pipeline (MA7.5, trust draft section 7), failure modes
// written before the code:
//
//	R1  a release whose ceiling is widened (an extended grant, a desk, or an
//	    update that adds a grant) installs with no verdict;
//	R2  a forged verdict: a record that claims the pipeline's agent and an
//	    accept, but carries no seal, a seal from another key, or a seal over
//	    other bytes, is honoured;
//	R3  a verdict for a different digest (another release, another tree, the
//	    same release with another manifest) is honoured;
//	R4  a rejected verdict is overridden by an older accept;
//	R5  a seat is sized by taste rather than by the risk table, a seat runs on
//	    a model the roster does not name, or the roster names a model outside
//	    Devin and DeepSeek;
//	R6  an unavailable seat (no provider wired) yields a verdict anyway;
//	R7  a seat's answer that is not the typed verdict is read as an accept;
//	R8  a default-level fresh install is made to wait on a model review.

func manifestWith(t *testing.T, caps string, desks string) apppackage.Manifest {
	t.Helper()
	raw := `{"schema":"choir.package/v2","name":"demo-next","version":"1.0.0",
"display":{"title":"Demo","icon":"D","description":"Demo"},
"opens":[],"kinds":[],"uses_kinds":[],"contributes":{"desks":[` + desks + `]},"capabilities":[` + caps + `]}`
	var m apppackage.Manifest
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func subjectOf(m apppackage.Manifest) Subject {
	return Subject{
		ReleaseID: strings.Repeat("a", 64), PatchSHA256: strings.Repeat("b", 64), ManifestSHA256: strings.Repeat("c", 64),
		BehaviourSurface: strings.Repeat("d", 64), Package: "demo-next", Publisher: "owner-p", Manifest: m,
	}
}

func TestWidenedCeiling(t *testing.T) {
	plain := manifestWith(t, `{"grant":"ui.captures.read"}`, "")
	if s := subjectOf(plain); s.Widened() {
		t.Fatalf("R8: a default-level fresh install reads as widened: %v", s.Added())
	}
	ext := manifestWith(t, `{"grant":"ui.read.kind","args":["notes"]}`, "")
	if s := subjectOf(ext); !s.Widened() {
		t.Fatal("R1: an extended grant is not a widened ceiling")
	}
	// An update that adds a default grant widens the ceiling it replaces.
	upd := subjectOf(manifestWith(t, `{"grant":"ui.captures.read"},{"grant":"desk.request","args":["texture"]}`, ""))
	upd.HasPrior, upd.PriorCeiling = true, Ceiling(plain)
	if !upd.Widened() || len(upd.Added()) != 1 || upd.Added()[0] != "desk.request(texture)" {
		t.Fatalf("R1: an update adding desk.request: widened=%v added=%v", upd.Widened(), upd.Added())
	}
	// The same ceiling on update is not widened; a narrowed one is not either.
	same := subjectOf(plain)
	same.HasPrior, same.PriorCeiling = true, Ceiling(plain)
	if same.Widened() {
		t.Fatal("an update with the same ceiling reads as widened")
	}
	// Argument changes count: the same grant to another kind is a new ceiling.
	other := subjectOf(manifestWith(t, `{"grant":"ui.read.kind","args":["mail"]}`, ""))
	other.HasPrior, other.PriorCeiling = true, Ceiling(ext)
	if !other.Widened() {
		t.Fatal("R1: ui.read.kind(notes) -> ui.read.kind(mail) is not widened")
	}
}

func TestPlanSizesSeatsByTheRiskTable(t *testing.T) {
	cases := []struct {
		name   string
		caps   string
		strong bool
		seats  []string
		risk   types.ImpactClass
	}{
		{"I1 thin", `{"grant":"ui.read.kind","args":["notes"]}`, false, []string{SeatDevin}, types.ImpactI1},
		{"I1 strong", `{"grant":"ui.read.kind","args":["notes"]}`, true, nil, types.ImpactI1},
		{"I3 thin", `{"grant":"ui.net.egress","args":["api.example.com"]}`, false, []string{SeatDevin, SeatDeepSeek}, types.ImpactI3},
		{"I3 strong", `{"grant":"ui.net.egress","args":["api.example.com"]}`, true, []string{SeatDevin}, types.ImpactI3},
	}
	for _, c := range cases {
		p := PlanFor(subjectOf(manifestWith(t, c.caps, "")), c.strong)
		if p.Refusal != "" || p.Risk != c.risk || strings.Join(p.Seats, ",") != strings.Join(c.seats, ",") {
			t.Fatalf("R5 %s: plan %+v, want risk %s seats %v", c.name, p, c.risk, c.seats)
		}
	}
	// I4 from a Library release is refused outright (first-party only).
	p := PlanFor(subjectOf(manifestWith(t, `{"grant":"ui.shell"}`, "")), true)
	if p.Refusal == "" {
		t.Fatalf("an I4 grant from a Library release is planned: %+v", p)
	}
	// An unknown grant fails closed to I4.
	p = PlanFor(subjectOf(manifestWith(t, `{"grant":"ui.mystery"}`, "")), true)
	if p.Risk != types.ImpactI4 || p.Refusal == "" {
		t.Fatalf("an unknown grant is not I4: %+v", p)
	}
}

func TestRosterIsDevinAndDeepSeekOnly(t *testing.T) {
	roster := Roster(nil)
	if len(roster) != 2 || roster[SeatDevin] == nil || roster[SeatDeepSeek] == nil {
		t.Fatalf("R5: roster %v", roster)
	}
	ds := roster[SeatDeepSeek].(*ProviderSeat)
	if !strings.Contains(strings.ToLower(ds.Model()), "deepseek") || ds.Provider() == "openrouter" {
		t.Fatalf("R5: the DeepSeek seat runs %s on %s", ds.Model(), ds.Provider())
	}
	if _, ok := modelcatalog.Lookup(ds.Provider(), ds.Model()); !ok {
		t.Fatalf("R5: the DeepSeek seat's model %s/%s is not in the catalog", ds.Provider(), ds.Model())
	}
	// R6: no Devin provider is wired: the seat says so and gives no verdict.
	_, err := roster[SeatDevin].Review(context.Background(), Packet{})
	if !errors.Is(err, ErrSeatUnavailable) {
		t.Fatalf("R6: the Devin seat answered %v", err)
	}
	// R6: a seat with no caller is unavailable, never an accept.
	if _, err := ds.Review(context.Background(), Packet{}); !errors.Is(err, ErrSeatUnavailable) {
		t.Fatalf("R6: the DeepSeek seat with no provider answered %v", err)
	}
}

type fakeCaller struct {
	text string
	req  provideriface.ToolLoopRequest
}

func (f *fakeCaller) CallWithTools(_ context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	f.req = req
	return &provideriface.ToolLoopResponse{Text: f.text, Model: req.Model}, nil
}

func TestProviderSeatReadsOnlyTheTypedVerdict(t *testing.T) {
	packet := Packet{Subject: subjectOf(manifestWith(t, `{"grant":"ui.read.kind","args":["notes"]}`, "")), Risk: types.ImpactI1}
	good := `{"accept":true,"findings":[],"blocking":false,"predictions":[{"question":"` + StandardQuestions[0] + `","p_yes":0.9},{"question":"` + StandardQuestions[1] + `","p_yes":0.85}]}`
	caller := &fakeCaller{text: "```json\n" + good + "\n```"}
	seat := NewProviderSeat(SeatDeepSeek, DeepSeekProvider, DeepSeekModel, caller)
	v, err := seat.Review(context.Background(), packet)
	if err != nil || !v.Accept || len(v.Predictions) != 2 {
		t.Fatalf("typed verdict: %+v %v", v, err)
	}
	if caller.req.Provider != DeepSeekProvider || caller.req.Model != DeepSeekModel {
		t.Fatalf("R5: the seat called %s/%s", caller.req.Provider, caller.req.Model)
	}
	for _, bad := range []string{
		"Looks fine to me, accept.",
		`{"accept":true}`, // no predictions
		`{"accept":true,"predictions":[{"question":"` + StandardQuestions[0] + `","p_yes":1.4},{"question":"` + StandardQuestions[1] + `","p_yes":0.5}]}`,
		`{"accept":true,"blocking":true,"predictions":[{"question":"` + StandardQuestions[0] + `","p_yes":0.9},{"question":"` + StandardQuestions[1] + `","p_yes":0.9}]}`,
	} {
		caller.text = bad
		if v, err := seat.Review(context.Background(), packet); err == nil {
			t.Fatalf("R7: %q read as %+v", bad, v)
		}
	}
}

type testSigner struct {
	key computerevent.SigningKey
}

func newTestSigner(t *testing.T) *testSigner {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return &testSigner{key: computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "guest-core", KeyID: "k1"}, PrivateKey: priv}}
}

func (s *testSigner) PublicKey(context.Context) (computerevent.SignerRef, ed25519.PublicKey, error) {
	return s.key.SignerRef, s.key.PrivateKey.Public().(ed25519.PublicKey), nil
}

func (s *testSigner) SignReceipt(_ context.Context, kind, issuer string, fields map[string]any, at time.Time) (computerevent.Receipt, error) {
	return computerevent.NewSignedReceipt(kind, issuer, fields, []computerevent.SigningKey{s.key}, at)
}

func sealedVerdict(t *testing.T, signer *testSigner, s Subject, accept bool, at time.Time) types.CommitmentRecord {
	t.Helper()
	plan := PlanFor(s, false)
	seatV := SeatVerdict{Accept: accept, Predictions: []Prediction{{StandardQuestions[0], 0.9}, {StandardQuestions[1], 0.8}}}
	seat := SeatRecord(s, plan, SeatDevin, "devin", seatV, "computer-1", at)
	rec := VerdictRecord(s, plan, nil, []types.CommitmentRecord{seat}, accept, "computer-1", at)
	sealed, err := Seal(context.Background(), signer, rec, "computer-1", at)
	if err != nil {
		t.Fatal(err)
	}
	return sealed
}

func TestGateHonoursOnlyASealedVerdictForThisDigest(t *testing.T) {
	signer := newTestSigner(t)
	_, pub, _ := signer.PublicKey(context.Background())
	s := subjectOf(manifestWith(t, `{"grant":"ui.read.kind","args":["notes"]}`, ""))
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)

	// R1: no records at all.
	if d := Gate(nil, s, "computer-1", pub); d.Allowed || d.Reason != ReasonRequired {
		t.Fatalf("R1: no verdict: %+v", d)
	}
	good := sealedVerdict(t, signer, s, true, now)
	if d := Gate([]types.CommitmentRecord{good}, s, "computer-1", pub); !d.Allowed || d.VerdictID != good.RecordID {
		t.Fatalf("a sealed accept for this digest: %+v", d)
	}

	// R2: forgeries.
	unsealed := good
	unsealed.SubjectRefs = cloneRefs(good.SubjectRefs)
	delete(unsealed.SubjectRefs, RefSeal)
	other := newTestSigner(t)
	otherKey := sealedVerdict(t, other, s, true, now)
	tampered := good
	tampered.SubjectRefs = cloneRefs(good.SubjectRefs)
	tampered.Prediction.Hypothesis = "install review verdict: accept (edited)"
	flipped := sealedVerdict(t, signer, s, false, now)
	flipped.SubjectRefs = cloneRefs(flipped.SubjectRefs)
	flipped.SubjectRefs[RefVerdict] = VerdictAccept
	wrongComputer := good
	for name, forged := range map[string]types.CommitmentRecord{"unsealed": unsealed, "other key": otherKey, "tampered body": tampered, "reject relabelled": flipped} {
		d := Gate([]types.CommitmentRecord{forged}, s, "computer-1", pub)
		if d.Allowed || d.Reason != ReasonRequired || len(d.Ignored) != 1 {
			t.Fatalf("R2 %s: %+v", name, d)
		}
	}
	if d := Gate([]types.CommitmentRecord{wrongComputer}, s, "computer-2", pub); d.Allowed {
		t.Fatalf("R2: a verdict sealed for another computer: %+v", d)
	}

	// R3: a sealed accept for a different digest.
	for name, mutate := range map[string]func(*Subject){
		"release":  func(x *Subject) { x.ReleaseID = strings.Repeat("e", 64) },
		"tree":     func(x *Subject) { x.PatchSHA256 = strings.Repeat("e", 64) },
		"manifest": func(x *Subject) { x.ManifestSHA256 = strings.Repeat("e", 64) },
		"surface":  func(x *Subject) { x.BehaviourSurface = strings.Repeat("e", 64) },
	} {
		want := s
		mutate(&want)
		if d := Gate([]types.CommitmentRecord{good}, want, "computer-1", pub); d.Allowed {
			t.Fatalf("R3 %s: a verdict for another digest admitted: %+v", name, d)
		}
	}

	// R4: the latest sealed verdict wins; a reject after an accept refuses.
	reject := sealedVerdict(t, signer, s, false, now.Add(time.Minute))
	if d := Gate([]types.CommitmentRecord{good, reject}, s, "computer-1", pub); d.Allowed || d.Reason != ReasonRejected {
		t.Fatalf("R4: reject after accept: %+v", d)
	}
	later := sealedVerdict(t, signer, s, true, now.Add(2*time.Minute))
	if d := Gate([]types.CommitmentRecord{reject, good, later}, s, "computer-1", pub); !d.Allowed || d.VerdictID != later.RecordID {
		t.Fatalf("R4: a new accept after a reject: %+v", d)
	}
}

func TestVerdictRecordsArePrecommitments(t *testing.T) {
	s := subjectOf(manifestWith(t, `{"grant":"ui.net.egress","args":["api.example.com"]}`, ""))
	plan := PlanFor(s, false)
	at := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	seat := SeatRecord(s, plan, SeatDeepSeek, DeepSeekModel, SeatVerdict{Accept: true, Predictions: []Prediction{{StandardQuestions[0], 0.9}, {StandardQuestions[1], 0.7}}}, "c", at)
	if seat.Kind != types.CommitmentKindPrecommit || seat.Precommit == nil || seat.Precommit.Distribution["yes"] != 0.7 ||
		seat.Precommit.ImpactClass != types.ImpactI3 || len(seat.Prediction.Questions) != 2 || seat.Provenance.ModelID != DeepSeekModel ||
		seat.SubjectRefs[RefRole] != RoleSeat || seat.SubjectRefs[RefSeat] != SeatDeepSeek || seat.Provenance.ResolvedAt != "" {
		t.Fatalf("seat record is not an open precommitment: %+v", seat)
	}
	if !strings.HasPrefix(seat.Provenance.AgentID, "system:") || seat.SubjectRefs[RefResolvesBy] == "" || seat.Precommit.Weight == nil {
		t.Fatalf("seat record agent %q refs %v", seat.Provenance.AgentID, seat.SubjectRefs)
	}
}

func TestFlowSentencesNameSourceToSink(t *testing.T) {
	m := manifestWith(t, `{"grant":"ui.read.kind","args":["notes"]},{"grant":"ui.net.egress","args":["api.example.com"]}`, "")
	flow := Flow(m)
	if len(flow) != 1 || !strings.Contains(flow[0], "notes") || !strings.Contains(flow[0], "api.example.com") {
		t.Fatalf("flow %v", flow)
	}
	if f := Flow(manifestWith(t, `{"grant":"ui.read.kind","args":["notes"]}`, "")); len(f) != 0 {
		t.Fatalf("a read with no sink has a flow: %v", f)
	}
}
