package lease_test

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/lease"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Failure modes the lease slice (MA3.4) must not have, written before the
// code:
//
//  1. a desk with no lease reads another package's kind;
//  2. a lease outlives its run (the run ends, the lease still admits);
//  3. a lease is used by another run of the same desk;
//  4. a crash leaves a lease active at the next boot;
//  5. the owner's revoke leaves the next read working;
//  6. a lease request is not a scored record (no p_success or weight), or a
//     refused request leaves no trace;
//  7. a lease widens beyond the manifest ceiling: at issue, or at use when a
//     lease record is forged on the tape;
//  8. a lease for one kind opens a different kind;
//  9. an uninstall or retirement of the holder leaves its lease live;
//  10. a lease reads past its expiry.

const (
	ownerPkg  = "leaseowner"
	readerPkg = "leasereader"
	ownerID   = "owner-lease-test"
	computer  = "computer-lease-test"
	clipKind  = objectgraph.ObjectKind("choir.app.leaseowner.clip")
	secret    = objectgraph.ObjectKind("choir.app.leaseowner.secret")
)

type fixture struct {
	t      *testing.T
	st     *store.Store
	svc    *lease.Service
	reader objectgraph.Writer
}

func registerApp(t *testing.T, slug string, uid uint32, leaseKinds []string) {
	t.Helper()
	d, err := deskmanifest.NewAppDesk(deskmanifest.AppDeskSpec{
		Name: "app:" + slug, Verbs: []string{"Cast", "ReadCapture"}, Taint: deskmanifest.TaintUntrustedInput,
		UID: uid, WorkerMaxBytes: 64 << 20, LeaseKinds: leaseKinds, LeaseCaptures: len(leaseKinds) > 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := deskmanifest.RegisterApp(d); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = deskmanifest.RetireApp("app:" + slug) })
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	for _, k := range []objectgraph.ObjectKind{clipKind, secret} {
		err := objectgraph.RegisterPackageKind(ownerPkg, objectgraph.KindRegistration{
			Kind: k, IdentityMode: objectgraph.IdentityExternalKey, OwnerEdits: true,
			Schema: "pkg:" + ownerPkg + "@1", Writer: objectgraph.Writer("desk:" + ownerPkg),
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	registerApp(t, ownerPkg, 62801, nil)
	// The reader's ceiling names the clip kind and no other.
	registerApp(t, readerPkg, 62802, []string{string(clipKind)})
	svc := lease.New(st.Leases())
	t.Cleanup(svc.Close)
	for _, k := range []objectgraph.ObjectKind{clipKind, secret} {
		if _, err := st.PutPackageObject(context.Background(), k, ownerID, "o1", json.RawMessage(`{"k":"v"}`)); err != nil {
			t.Fatal(err)
		}
	}
	reader, ok := objectgraph.DeskWriter("app:" + readerPkg)
	if !ok {
		t.Fatal("reader desk has no writer")
	}
	return &fixture{t: t, st: st, svc: svc, reader: reader}
}

func (f *fixture) ctx(runID string) context.Context {
	return objectgraph.WithLeases(objectgraph.WithReader(context.Background(), f.reader), f.svc, runID)
}

func (f *fixture) read(runID string, kind objectgraph.ObjectKind) error {
	_, err := f.st.GetPackageObject(f.ctx(runID), kind, ownerID, "o1")
	return err
}

func fields(p, w float64) *types.RequestFields {
	return &types.RequestFields{PSuccess: &p, Weight: &w, ImpactClass: types.ImpactI1, Settlement: types.SettlementRecipient}
}

func (f *fixture) request(runID string, kinds []string, fl *types.RequestFields) (lease.Record, error) {
	return f.svc.Request(context.Background(), lease.Request{
		OwnerID: ownerID, ComputerID: computer, Desk: "app:" + readerPkg, RunID: runID, AgentID: "agent-" + runID,
		Scope: lease.Scope{Kinds: kinds}, Purpose: "edit the clip", Fields: fl,
	})
}

// (a) A desk without a lease is refused; with one it is served.
func TestADeskWithoutALeaseIsRefused(t *testing.T) {
	f := newFixture(t)
	if err := f.read("run-a", clipKind); !errors.Is(err, objectgraph.ErrWrongReader) {
		t.Fatalf("a desk with no lease read the kind: %v", err)
	}
	if _, err := f.request("run-a", []string{string(clipKind)}, fields(0.8, 0.3)); err != nil {
		t.Fatal(err)
	}
	if err := f.read("run-a", clipKind); err != nil {
		t.Fatalf("a leased read was refused: %v", err)
	}
	// The lease is the run's: another run of the same desk holds nothing.
	if err := f.read("run-other", clipKind); !errors.Is(err, objectgraph.ErrWrongReader) {
		t.Fatalf("another run used the lease: %v", err)
	}
}

// (b) A lease expires at its run's end, and at its own expiry.
func TestALeaseExpiresAtItsRunsEnd(t *testing.T) {
	f := newFixture(t)
	if _, err := f.request("run-b", []string{string(clipKind)}, fields(0.8, 0.3)); err != nil {
		t.Fatal(err)
	}
	if err := f.read("run-b", clipKind); err != nil {
		t.Fatal(err)
	}
	if n := f.svc.EndRun(context.Background(), "run-b", lease.ReasonRunEnded); n != 1 {
		t.Fatalf("EndRun ended %d leases", n)
	}
	if err := f.read("run-b", clipKind); !errors.Is(err, objectgraph.ErrWrongReader) {
		t.Fatalf("a lease outlived its run: %v", err)
	}
	recs, err := f.st.Leases().ListLeases(context.Background(), ownerID, computer)
	if err != nil || len(recs) != 1 || recs[0].State != lease.StateEnded || recs[0].EndReason != lease.ReasonRunEnded {
		t.Fatalf("tape = %+v, %v", recs, err)
	}

	// An expiry shorter than the run ends it too.
	now := time.Now().UTC()
	f.svc.SetClock(func() time.Time { return now })
	rec, err := f.svc.Request(context.Background(), lease.Request{
		OwnerID: ownerID, ComputerID: computer, Desk: "app:" + readerPkg, RunID: "run-b2", AgentID: "a",
		Scope: lease.Scope{Kinds: []string{string(clipKind)}}, Purpose: "short", Duration: time.Minute, Fields: fields(0.8, 0.3),
	})
	if err != nil || rec.ExpiresAt.IsZero() {
		t.Fatalf("lease = %+v, %v", rec, err)
	}
	if err := f.read("run-b2", clipKind); err != nil {
		t.Fatal(err)
	}
	f.svc.SetClock(func() time.Time { return now.Add(2 * time.Minute) })
	if err := f.read("run-b2", clipKind); !errors.Is(err, objectgraph.ErrWrongReader) {
		t.Fatalf("a lease read past its expiry: %v", err)
	}
}

// (c) A lease a crash left active is ended at boot, from the tape, and is not
// trusted in between (a fresh service that has loaded nothing admits nothing).
func TestALeaseTheCrashLeftOpenIsEndedAtBoot(t *testing.T) {
	f := newFixture(t)
	if _, err := f.request("run-c", []string{string(clipKind)}, fields(0.8, 0.3)); err != nil {
		t.Fatal(err)
	}
	// The process dies: the new process's service has an empty index.
	booted := lease.New(f.st.Leases())
	defer booted.Close()
	f.svc = booted
	if err := f.read("run-c", clipKind); !errors.Is(err, objectgraph.ErrWrongReader) {
		t.Fatalf("a fresh process trusted a lease it had not loaded: %v", err)
	}
	if err := booted.Load(context.Background(), ownerID, computer); err != nil {
		t.Fatal(err)
	}
	if n := booted.EndAll(context.Background(), lease.ReasonRestart); n != 1 {
		t.Fatalf("boot ended %d leases, want 1", n)
	}
	if err := f.read("run-c", clipKind); !errors.Is(err, objectgraph.ErrWrongReader) {
		t.Fatalf("a lease survived the boot: %v", err)
	}
	recs, _ := f.st.Leases().ListLeases(context.Background(), ownerID, computer)
	if len(recs) != 1 || recs[0].State != lease.StateEnded || recs[0].EndReason != lease.ReasonRestart {
		t.Fatalf("tape after boot = %+v", recs)
	}
}

// (d) The owner revokes, and the next read fails; the owner sees the lease as
// a sentence first.
func TestTheOwnerRevokesAndTheNextReadFails(t *testing.T) {
	f := newFixture(t)
	rec, err := f.request("run-d", []string{string(clipKind)}, fields(0.8, 0.3))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.read("run-d", clipKind); err != nil {
		t.Fatal(err)
	}
	live := f.svc.Live(ownerID)
	if len(live) != 1 || live[0].Sentence() == "" {
		t.Fatalf("owner view = %+v", live)
	}
	if err := f.svc.Revoke(context.Background(), rec.LeaseID, lease.ReasonOwner); err != nil {
		t.Fatal(err)
	}
	if err := f.read("run-d", clipKind); !errors.Is(err, objectgraph.ErrWrongReader) {
		t.Fatalf("a revoked lease still read: %v", err)
	}
	if err := f.svc.Revoke(context.Background(), rec.LeaseID, lease.ReasonOwner); err == nil {
		t.Fatal("a second revoke of the same lease succeeded")
	}
	all, _ := f.svc.All(context.Background(), ownerID, computer)
	if len(all) != 1 || all[0].State != lease.StateRevoked {
		t.Fatalf("owner list = %+v", all)
	}
}

// (e) A lease request is a scored precommitment record: it carries p_success,
// weight and the frozen materiality procedure like a Cast; a request without
// them is refused, and a refused request leaves its record.
func TestALeaseRequestIsAScoredPrecommitmentRecord(t *testing.T) {
	f := newFixture(t)
	rec, err := f.request("run-e", []string{string(clipKind)}, fields(0.8, 0.3))
	if err != nil {
		t.Fatal(err)
	}
	got, err := f.st.GetCommitmentRecord(context.Background(), ownerID, computer, rec.RequestRef)
	if err != nil || got == nil {
		t.Fatalf("request record: %v", err)
	}
	if got.PSuccess == nil || *got.PSuccess != 0.8 || got.Weight == nil || *got.Weight != 0.3 ||
		got.MaterialityProcedure == nil || !got.MaterialityProcedure.Verify() ||
		got.Kind != types.CommitmentKindPrecommit || got.Precommit == nil || got.Directive != nil ||
		got.MaterialityProcedure.Kind != types.RequestLease || got.IsDirectiveRecord() ||
		got.Prediction.Questions[0].Probabilities["yes"] != 0.8 ||
		got.Provenance.Desk != "app:"+readerPkg || got.Provenance.RunID != "run-e" || got.Provenance.Origin != types.OriginDeskRun {
		t.Fatalf("request record is not a stamped precommitment: %+v", got)
	}
	if rec.RequestRef != got.RecordID {
		t.Fatalf("the lease does not point at its request: %q vs %q", rec.RequestRef, got.RecordID)
	}

	for name, fl := range map[string]*types.RequestFields{
		"no fields":      nil,
		"no p_success":   {Weight: ptr(0.3), Settlement: types.SettlementRecipient},
		"no weight":      {PSuccess: ptr(0.8), Settlement: types.SettlementRecipient},
		"weight 0 at I1": {PSuccess: ptr(0.8), Weight: ptr(0), Settlement: types.SettlementRecipient},
	} {
		if _, err := f.request("run-e2", []string{string(clipKind)}, fl); !errors.Is(err, lease.ErrRefused) {
			t.Errorf("%s: err = %v, want a refusal", name, err)
		}
	}
	if err := f.read("run-e2", clipKind); !errors.Is(err, objectgraph.ErrWrongReader) {
		t.Fatalf("a refused request leased something: %v", err)
	}

	// A well-formed request outside the ceiling is refused, and its record
	// stays: the ask is evidence for the requester's record.
	before, _ := f.st.ListCommitmentRecords(context.Background(), ownerID, computer, "system:lease", 50)
	if _, err := f.request("run-e3", []string{string(secret)}, fields(0.9, 0.5)); !errors.Is(err, lease.ErrRefused) {
		t.Fatalf("an out-of-ceiling request: %v", err)
	}
	after, _ := f.st.ListCommitmentRecords(context.Background(), ownerID, computer, "system:lease", 50)
	if len(after) != len(before)+1 {
		t.Fatalf("a refused request left %d new records, want 1", len(after)-len(before))
	}
}

func ptr(v float64) *float64 { return &v }

// (f) A hostile fixture cannot read a kind outside its ceiling, even holding a
// lease for another kind, and even with a lease record forged on the tape.
func TestAHostileFixtureCannotReadOutsideTheCeiling(t *testing.T) {
	f := newFixture(t)
	if _, err := f.request("run-f", []string{string(clipKind)}, fields(0.8, 0.3)); err != nil {
		t.Fatal(err)
	}
	if err := f.read("run-f", secret); !errors.Is(err, objectgraph.ErrWrongReader) {
		t.Fatalf("a lease for one kind opened another: %v", err)
	}
	if _, err := f.request("run-f", []string{string(secret)}, fields(0.8, 0.3)); !errors.Is(err, lease.ErrRefused) {
		t.Fatalf("a lease outside the ceiling was issued: %v", err)
	}
	if _, err := f.request("run-f", []string{string(clipKind), string(secret)}, fields(0.8, 0.3)); !errors.Is(err, lease.ErrRefused) {
		t.Fatalf("a lease half outside the ceiling was issued: %v", err)
	}
	if _, err := f.svc.Request(context.Background(), lease.Request{
		OwnerID: ownerID, ComputerID: computer, Desk: "app:" + readerPkg, RunID: "run-f", AgentID: "a",
		Scope: lease.Scope{CaptureIDs: []string{"c1"}}, Purpose: "x", Fields: fields(0.8, 0.3),
	}); err != nil {
		// readerPkg's ceiling declares captures (LeaseCaptures true): allowed.
		t.Fatalf("a capture lease inside the ceiling was refused: %v", err)
	}

	// A forged record: written to the tape past the service, naming the
	// secret kind. A fresh service loads it as active; the ceiling is
	// re-checked at use, so it admits nothing.
	forged := lease.Record{
		LeaseID: "lease-forged", OwnerID: ownerID, ComputerID: computer, Holder: "app:" + readerPkg,
		HolderWriter: string(f.reader), HolderRunID: "run-forged", Scope: lease.Scope{Kinds: []string{string(secret)}},
		Purpose: "forged", NotBefore: time.Now().Add(-time.Minute), IssuedAt: time.Now().Add(-time.Minute), State: lease.StateActive,
	}
	if err := f.st.Leases().PutLease(context.Background(), forged); err != nil {
		t.Fatal(err)
	}
	loaded := lease.New(f.st.Leases())
	defer loaded.Close()
	if err := loaded.Load(context.Background(), ownerID, computer); err != nil {
		t.Fatal(err)
	}
	f.svc = loaded
	if err := f.read("run-forged", secret); !errors.Is(err, objectgraph.ErrWrongReader) {
		t.Fatalf("a forged lease read past the ceiling: %v", err)
	}
}

// An uninstall, a yank the computer applied or a retirement of the holder
// revokes its leases.
func TestRetiringTheHolderRevokesItsLeases(t *testing.T) {
	f := newFixture(t)
	rec, err := f.request("run-r", []string{string(clipKind)}, fields(0.8, 0.3))
	if err != nil {
		t.Fatal(err)
	}
	if err := deskmanifest.RetireApp("app:" + readerPkg); err != nil {
		t.Fatal(err)
	}
	if len(f.svc.Live(ownerID)) != 0 {
		t.Fatal("a retired desk kept a live lease")
	}
	recs, _ := f.st.Leases().ListLeases(context.Background(), ownerID, computer)
	if len(recs) != 1 || recs[0].LeaseID != rec.LeaseID || recs[0].State != lease.StateRevoked || recs[0].EndReason != lease.ReasonRetired {
		t.Fatalf("tape = %+v", recs)
	}
	// Re-register for the cleanup's RetireApp.
	registerApp(t, readerPkg, 62802, []string{string(clipKind)})
}

// X6: the capsule's store copy is a lease; the default names every built-in
// kind but Texture's bodies, mail and the lease kind, and the copy holds
// those kinds only.
func TestTheCapsuleStoreCopyIsALease(t *testing.T) {
	kinds := lease.CapsuleStoreKinds()
	has := map[string]bool{}
	for _, k := range kinds {
		has[k] = true
	}
	for _, withheld := range []string{"choir.texture_document", "choir.texture_revision", "choir.lease"} {
		if has[withheld] {
			t.Errorf("the default capsule lease names %s", withheld)
		}
	}
	if !has["choir.run"] || !has["choir.commitment_record"] {
		t.Errorf("the default capsule lease lacks the kernel's kinds: %v", kinds)
	}
	f := newFixture(t)
	rec, err := f.svc.IssueCapsule(context.Background(), ownerID, computer, "asg-1", "run:asg-1", "engineering:asg-1")
	if err != nil {
		t.Fatal(err)
	}
	if rec.Assignment != "asg-1" || len(rec.Scope.Kinds) != len(kinds) {
		t.Fatalf("capsule lease = %+v", rec)
	}
	// It is not a read lease: the engineering desk has no ceiling.
	if f.svc.AdmitsKind(objectgraph.WriterEngineering, "run:asg-1", "choir.texture_document") {
		t.Fatal("the capsule lease admitted a read of a withheld kind")
	}
}

// MA3.7c: texture.search is a first-party grant lease. Only the research desk
// run it was issued for holds it; it opens no kind to the reader gate and ends
// with the run.
func TestTextureSearchGrantIsHeldByTheIssuedRunOnly(t *testing.T) {
	f := newFixture(t)
	rec, err := f.svc.IssueTextureSearch(context.Background(), ownerID, computer, "run-g", "research:doc")
	if err != nil {
		t.Fatal(err)
	}
	if rec.Holder != deskmanifest.Research || len(rec.Scope.Grants) != 1 || len(rec.Custodians) != 1 || rec.Custodians[0] != "desk:texture" {
		t.Fatalf("unexpected grant lease: %+v", rec)
	}
	research := objectgraph.WriterResearch
	if _, ok := f.svc.AdmitsGrant(research, "run-g", lease.GrantTextureSearch); !ok {
		t.Error("the issued run does not hold the grant")
	}
	if _, ok := f.svc.AdmitsGrant(research, "run-other", lease.GrantTextureSearch); ok {
		t.Error("another run of the desk holds the grant")
	}
	if _, ok := f.svc.AdmitsGrant(objectgraph.WriterManagement, "run-g", lease.GrantTextureSearch); ok {
		t.Error("another desk holds the grant")
	}
	if f.svc.AdmitsKind(research, "run-g", "choir.texture_document") {
		t.Error("a grant lease opened a kind to the reader gate")
	}
	f.svc.EndRun(context.Background(), "run-g", lease.ReasonRunEnded)
	if _, ok := f.svc.AdmitsGrant(research, "run-g", lease.GrantTextureSearch); ok {
		t.Error("the grant outlived its run")
	}
	// A request through the public path cannot name a grant (no ceiling row).
	_, err = f.svc.Request(context.Background(), lease.Request{
		OwnerID: ownerID, ComputerID: computer, Desk: deskmanifest.Research, RunID: "run-h", AgentID: "research:doc",
		Scope: lease.Scope{Grants: []string{lease.GrantTextureSearch}}, Purpose: "self-grant", Fields: fields(0.9, 0.2),
	})
	if err == nil {
		t.Error("a desk's own request obtained the grant")
	}
}

// texture.brief: the Texture custodian's disclosure grant to research for one
// Texture run. Ways it could fail (written before the code):
//
//	B1. a desk other than Texture mints it, for itself or for another desk
//	    (research self-mints; management mints for research);
//	B2. a desk asks for it through the public request path;
//	B3. it is minted for a desk the grant table does not name;
//	B4. it admits messages from another run (another trajectory's documents);
//	B5. it admits a label outside the custodian's data (a capture, mail);
//	B6. it outlives its run, or its expiry;
//	B7. it opens a kind to the reader gate, or the texture.search service;
//	B8. a second message of the same run mints a second lease.
func TestTextureBriefGrantIsMintedByTheCustodianOnly(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	research := objectgraph.WriterResearch
	doc := types.OwnerPrivateLabel("document")
	for _, sender := range []string{deskmanifest.Research, deskmanifest.Management, "app:" + readerPkg, ""} {
		if _, err := f.svc.IssueTextureBrief(ctx, ownerID, computer, sender, "run-t", "x:1", deskmanifest.Research); !errors.Is(err, lease.ErrRefused) {
			t.Errorf("B1: sender %q minted texture.brief: %v", sender, err)
		}
	}
	if _, err := f.svc.IssueTextureBrief(ctx, ownerID, computer, deskmanifest.Texture, "run-t", "texture:1", deskmanifest.Management); !errors.Is(err, lease.ErrRefused) {
		t.Errorf("B3: texture.brief minted for management: %v", err)
	}
	if _, err := f.svc.IssueTextureBrief(ctx, ownerID, computer, deskmanifest.Texture, "run-t", "texture:1", "app:"+readerPkg); !errors.Is(err, lease.ErrRefused) {
		t.Errorf("B3: texture.brief minted for a package desk: %v", err)
	}
	_, err := f.svc.Request(ctx, lease.Request{
		OwnerID: ownerID, ComputerID: computer, Desk: deskmanifest.Research, RunID: "run-t", AgentID: "research:1",
		Scope: lease.Scope{Grants: []string{lease.GrantTextureBrief}}, Purpose: "self-grant", Fields: fields(0.9, 0.2),
	})
	if !errors.Is(err, lease.ErrRefused) {
		t.Errorf("B2: a desk's own request obtained texture.brief: %v", err)
	}
	if f.svc.AdmitsMessageLabel(research, "run-t", doc) {
		t.Fatal("a refused mint admitted a message")
	}

	start := time.Now().UTC()
	now := start
	f.svc.SetClock(func() time.Time { return now })
	rec, err := f.svc.IssueTextureBrief(ctx, ownerID, computer, deskmanifest.Texture, "run-t", "texture:1", deskmanifest.Research)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Holder != deskmanifest.Research || rec.HolderRunID != "run-t" || len(rec.Custodians) != 1 || rec.Custodians[0] != "desk:texture" ||
		rec.ExpiresAt.IsZero() || rec.ExpiresAt.Sub(rec.IssuedAt) > lease.MaxDuration {
		t.Fatalf("unexpected texture.brief lease: %+v", rec)
	}
	// On the tape.
	all, err := f.svc.All(ctx, ownerID, computer)
	if err != nil || len(all) != 1 || all[0].LeaseID != rec.LeaseID {
		t.Fatalf("the lease is not on the tape: %v %+v", err, all)
	}
	again, err := f.svc.IssueTextureBrief(ctx, ownerID, computer, deskmanifest.Texture, "run-t", "texture:1", deskmanifest.Research)
	if err != nil || again.LeaseID != rec.LeaseID || len(f.svc.Live(ownerID)) != 1 {
		t.Errorf("B8: a second message minted a second lease: %v %s %s", err, again.LeaseID, rec.LeaseID)
	}
	if !f.svc.AdmitsMessageLabel(research, "run-t", doc) || !f.svc.AdmitsMessageLabel(research, "run-t", types.OwnerPrivateLabel("texture")) {
		t.Error("the custodian's labels are not admitted for its run")
	}
	if f.svc.AdmitsMessageLabel(research, "run-other", doc) {
		t.Error("B4: another run's message is admitted")
	}
	if f.svc.AdmitsMessageLabel(objectgraph.WriterManagement, "run-t", doc) {
		t.Error("B4: another desk is admitted")
	}
	for _, l := range []string{types.OwnerPrivateLabel("capture"), types.OwnerPrivateLabel("mail"), types.OwnerPrivateLabel("evidence")} {
		if f.svc.AdmitsMessageLabel(research, "run-t", l) {
			t.Errorf("B5: %s is admitted", l)
		}
	}
	if f.svc.AdmitsKind(research, "run-t", "choir.texture_document") || f.svc.AdmitsKind(research, "run-t", "choir.texture_revision") {
		t.Error("B7: the grant opened a kind to the reader gate")
	}
	if _, ok := f.svc.AdmitsGrant(research, "run-t", lease.GrantTextureSearch); ok {
		t.Error("B7: the grant opened texture.search")
	}
	// The texture.search grant admits no message.
	if _, err := f.svc.IssueTextureSearch(ctx, ownerID, computer, "run-s", "research:2"); err != nil {
		t.Fatal(err)
	}
	if f.svc.AdmitsMessageLabel(research, "run-s", doc) {
		t.Error("texture.search admitted a message")
	}
	// Expiry.
	now = start.Add(lease.MaxDuration)
	if f.svc.AdmitsMessageLabel(research, "run-t", doc) {
		t.Error("B6: the grant admitted past its expiry")
	}
	// The run's end.
	now = start
	if _, err := f.svc.IssueTextureBrief(ctx, ownerID, computer, deskmanifest.Texture, "run-u", "texture:1", deskmanifest.Research); err != nil {
		t.Fatal(err)
	}
	f.svc.EndRun(ctx, "run-u", lease.ReasonRunEnded)
	if f.svc.AdmitsMessageLabel(research, "run-u", doc) {
		t.Error("B6: the grant outlived its run")
	}
}
