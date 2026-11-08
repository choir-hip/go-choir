package agentcore

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/installreview"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
	"github.com/yusefmosiah/go-choir/internal/pkgruntime"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The install review as a desk act (docs/design/desk-install-review-2026-11-08.md
// section 4). Failure modes, decided before the code, and the test that holds
// each (each fails without the code it guards):
//
//	G1  a widened-ceiling release installs with no verdict signer:
//	    TestInstallReviewGatesAWidenedCeiling
//	G2  a widened release installs without management's verdict, or an
//	    unanswered review leaves a verdict on the ledger:
//	    TestInstallReviewGatesAWidenedCeiling
//	G3  a forged verdict (no seal, another key's seal) admits it:
//	    TestInstallReviewGatesAWidenedCeiling
//	G4  a sealed verdict for another tree admits this one (FM2):
//	    TestInstallReviewGatesAWidenedCeiling
//	G5  a rejected release installs on a later try:
//	    TestInstallReviewRejectStands
//	G6  a verdict in the install request body is read:
//	    TestInstallReviewGatesAWidenedCeiling
//	G7  a refused review stores the module:
//	    TestInstallReviewGatesAWidenedCeiling
//	FM1 management never answers: the continuation closes the review at 45
//	    minutes, seals nothing, and a later attempt opens a fresh review; the
//	    notices are capped: TestInstallReviewTimeoutSealsNothingAndANewAttemptOpens
//	FM3 a desk other than management calls the verbs:
//	    TestInstallVerbsRefuseOtherDesks
//	FM4 management judges its own package operation's install:
//	    TestPackageOperationPendsInAcceptedAndInstallsOnAnAcceptingVerdict
//	FM5 the source exceeds 120 KiB: TestInstallPacketTruncatesTheSource
//	FM6 a double verdict: TestInstallVerdictReplayAndSecondVerdict
//	FM7 a verdict lands after a newer install: TestInstallReviewSupersededByANewerInstall,
//	    TestPendingRerunWithAStaleBaselineInstallsNothing
//	FM8 a crash between the sealed verdict and the re-run:
//	    TestInstallReviewCrashBootClosesThePendingInstall
//	FM9 a verdict sealed between the gate's read and the owed commit:
//	    TestInstallReviewOwedCommitRereadsTheGate
//	FM10 the boot migration opens a review: TestInstallReviewDeferredNeverOpensAReview
//	FM11 the pending table grows: store.TestPackageInstallPendingClosesAndPrunes
//
// And the contracts around them: the owed record mints its own clock
// (store.TestInstallReviewOwedRecordMintsItsOwnClock), a repeat request
// answers the same 202 without building and heals a row with no record
// (TestInstallReviewDedupeAnswersTheSame202AndHeals), a failed mechanical
// check seals a rejection at once (TestInstallReviewMechanicalFailureSealsAtOnce),
// and a blocking finding makes the verdict a rejection
// (TestInstallVerdictBlockingFindingRejects).

func extendedManifest(caps string) string {
	return strings.Replace(installManifest("Demo"), `"capabilities": []`, `"capabilities": [`+caps+`]`, 1)
}

// manifestBuilder is the fake builder that returns the tree's real manifest,
// as pkgruntime.Builder does.
type manifestBuilder struct {
	fakeModuleBuilder
	manifest string
	// source, when set, is the module's code.
	source []byte
}

func (b *manifestBuilder) Build(ctx context.Context, rec pkgrelease.Record, tree []byte) (pkgruntime.Module, error) {
	m, err := b.fakeModuleBuilder.Build(ctx, rec, tree)
	if err == nil {
		m.Manifest = json.RawMessage(b.manifest)
		if len(b.source) > 0 {
			m.Bytes, m.Size = b.source, len(b.source)
			m.SHA256 = computerevent.DigestBytes(b.source)
		}
	}
	return m, err
}

type reviewSigner struct{ key computerevent.SigningKey }

func newReviewSigner(t *testing.T) *reviewSigner {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return &reviewSigner{key: computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "guest-core", KeyID: "gk"}, PrivateKey: priv}}
}

func (s *reviewSigner) PublicKey(context.Context) (computerevent.SignerRef, ed25519.PublicKey, error) {
	return s.key.SignerRef, s.key.PrivateKey.Public().(ed25519.PublicKey), nil
}

func (s *reviewSigner) SignReceipt(_ context.Context, kind, issuer string, fields map[string]any, at time.Time) (computerevent.Receipt, error) {
	return computerevent.NewSignedReceipt(kind, issuer, fields, []computerevent.SigningKey{s.key}, at)
}

func reviewRecords(t *testing.T, e *installEnv, releaseID string) []types.CommitmentRecord {
	t.Helper()
	recs, err := e.rt.store.ListCommitmentRecords(context.Background(), e.owner, e.computer, installreview.AddresseePrefix+releaseID, 100)
	if err != nil {
		t.Fatal(err)
	}
	return recs
}

// reviewFixture is a widened-ceiling release (an extended grant at I1 from a
// thin publisher: management's verdict is owed), its signer wired.
type reviewFixture struct {
	e        *installEnv
	signer   *reviewSigner
	manifest string
	rec      pkgrelease.Record
	id       string
	raw      json.RawMessage
	tree     []byte
	clock    *time.Time
}

func newReviewFixture(t *testing.T, caps string) *reviewFixture {
	t.Helper()
	e := newInstallEnv(t)
	manifest := extendedManifest(caps)
	e.rt.packageBuilder = &manifestBuilder{manifest: manifest}
	signer := newReviewSigner(t)
	e.rt.installReviewSigner = signer
	rec, id, raw, tree := publishedPackage(t, e.repo, e.base, "publisher-x", map[string]string{
		"choir-package.json": manifest, "App.svelte": "<p>demo</p>\n",
	})
	f := &reviewFixture{e: e, signer: signer, manifest: manifest, rec: rec, id: id, raw: raw, tree: tree}
	now := time.Now()
	f.clock = &now
	e.rt.installReviewNow = func() time.Time { return *f.clock }
	return f
}

func newNotesReviewFixture(t *testing.T) *reviewFixture {
	return newReviewFixture(t, `{"grant":"ui.read.kind","args":["notes"]}`)
}

func (f *reviewFixture) post(t *testing.T) (int, PackageInstallView, string) {
	t.Helper()
	return f.e.post(t, installBody(t, f.id, f.raw, f.tree, ""))
}

func (f *reviewFixture) advance(d time.Duration) { *f.clock = f.clock.Add(d) }

func (f *reviewFixture) subject() installreview.Subject {
	return installreview.Subject{ReleaseID: f.id, PatchSHA256: computerevent.DigestBytes(f.tree), ManifestSHA256: f.rec.Manifest.SHA256,
		BehaviourSurface: f.rec.Manifest.BehaviourSurface, Package: installName, Publisher: "publisher-x"}
}

func (f *reviewFixture) subjectDigest() string { return f.subject().Digest() }

// reduction is a management cell's reduction over the fixture's computer.
func (f *reviewFixture) reduction(profile string) *rlmCallReduction {
	run := types.RunRecord{RunID: "run-mgmt-" + profile, OwnerID: f.e.owner, ComputerID: f.e.computer, AgentID: persistentManagementAgentID(f.e.owner), AgentProfile: profile}
	return &rlmCallReduction{active: true, mb: f.e.rt, st: f.e.rt.store, ledger: f.e.rt.store, rec: &run, scope: ReductionScope{
		RunID: run.RunID, OwnerID: run.OwnerID, ComputerID: run.ComputerID, DeskAgentID: run.AgentID,
	}}
}

// packetRef is the owed record of the review the desk would read now.
func (f *reviewFixture) packetRef() string {
	owed, found, err := f.e.rt.installReviewOwedOn(context.Background(), f.e.owner, f.e.computer, f.id)
	if err != nil || !found {
		return "no-review"
	}
	return owed.Record.RecordID
}

func (f *reviewFixture) verdict(accept bool, p float64, findings []yaegikernel.InstallFinding, reason string) (map[string]any, error) {
	return f.verdictRef(f.packetRef(), accept, p, findings, reason)
}

func (f *reviewFixture) verdictRef(ref string, accept bool, p float64, findings []yaegikernel.InstallFinding, reason string) (map[string]any, error) {
	decision := "accept"
	if !accept {
		decision = "reject"
	}
	return f.reduction(agentprofile.Management).commitInstallVerdictIntent(context.Background(), yaegikernel.StagedIntent{
		LocalID: "v1", Kind: yaegikernel.IntentInstallVerdict, TargetRef: f.id, PacketRef: ref, Decision: decision, PWithinCeiling: p, InstallFindings: findings, Reason: reason,
	})
}

func (f *reviewFixture) judge(t *testing.T, accept bool) map[string]any {
	t.Helper()
	out, err := f.verdict(accept, 0.9, nil, map[bool]string{true: "", false: "it posts notes to a sink"}[accept])
	if err != nil {
		t.Fatalf("verdict: %v", err)
	}
	f.e.rt.installReviewRerun.Wait()
	return out
}

func (f *reviewFixture) pendingRow(t *testing.T) store.PackageInstallPending {
	t.Helper()
	row, err := f.e.rt.store.LatestPackageInstallPending(context.Background(), f.e.owner, f.e.computer, f.subjectDigest())
	if err != nil {
		t.Fatalf("pending row: %v", err)
	}
	return row
}

func (f *reviewFixture) dueWakes(t *testing.T) []types.InstallReviewDueWake {
	t.Helper()
	wakes, err := f.e.rt.store.ListUnprojectedActorWakes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var out []types.InstallReviewDueWake
	for _, w := range wakes {
		if w.Kind != types.InstallReviewDueWakeKind {
			continue
		}
		var due types.InstallReviewDueWake
		if err := json.Unmarshal([]byte(w.Content), &due); err != nil {
			t.Fatal(err)
		}
		out = append(out, due)
	}
	return out
}

func (f *reviewFixture) due(t *testing.T, stage string, notice int) error {
	t.Helper()
	content, _ := json.Marshal(types.InstallReviewDueWake{ReleaseID: f.id, SubjectDigest: f.subjectDigest(), Attempt: f.pendingRow(t).Attempt, Stage: stage, Notice: notice})
	return f.e.rt.HandleInstallReviewDue(context.Background(), f.e.owner, f.e.computer, persistentManagementAgentID(f.e.owner), string(content))
}

func TestInstallReviewGatesAWidenedCeiling(t *testing.T) {
	// G1: no signer.
	g1 := newReviewFixture(t, `{"grant":"ui.read.kind","args":["notes"]}`)
	g1.e.rt.installReviewSigner = nil
	if code, _, reason := g1.post(t); code != http.StatusServiceUnavailable || reason != "install_review_unavailable" {
		t.Fatalf("G1 = %d %s", code, reason)
	}

	// G2: management owes the verdict: the install waits (202), and nothing
	// is sealed, recorded or stored.
	g2 := newReviewFixture(t, `{"grant":"ui.read.kind","args":["notes"]}`)
	if g2.rec.Manifest.Level != "extended" {
		t.Fatalf("fixture level %s", g2.rec.Manifest.Level)
	}
	if code, _, reason := g2.post(t); code != http.StatusAccepted || reason != reasonInstallReviewOwed {
		t.Fatalf("G2 = %d %s", code, reason)
	}
	if recs := reviewRecords(t, g2.e, g2.id); len(recs) != 0 {
		t.Fatalf("G2: an unanswered review wrote %d records", len(recs))
	}
	// G6: a verdict in the request body is not read.
	smuggled, _ := json.Marshal(map[string]any{"release_id": g2.id, "record": g2.raw, "tree_patch": g2.tree, "verdict": map[string]any{"verdict": "accept"}})
	if code, _, reason := g2.e.post(t, smuggled); code != http.StatusBadRequest || reason != "request_invalid" {
		t.Fatalf("G6 = %d %s", code, reason)
	}
	// G7: no refusal or wait recorded an install or stored a module.
	if entries, _ := os.ReadDir(filepath.Join(g2.e.rt.packageModules.Dir, "modules")); g2.e.records(t) != 0 || len(entries) != 0 {
		t.Fatalf("G7: refusals recorded %d installs and stored modules %v", g2.e.records(t), entries)
	}

	// G3: forged verdicts on the ledger (no seal; another key's seal) do not
	// admit it. Each case runs on a fresh computer, before any request, so the
	// gate itself reads the forgery.
	g3 := newReviewFixture(t, `{"grant":"ui.read.kind","args":["notes"]}`)
	plan := installreview.Plan{Widened: true, Risk: types.ImpactI1, Seats: []string{installreview.SeatDesk}}
	forged := installreview.VerdictRecord(g3.subject(), plan, nil, nil, true, g3.e.computer, time.Now().UTC())
	otherKey, err := installreview.Seal(context.Background(), newReviewSigner(t), installreview.VerdictRecord(g3.subject(), plan, nil, nil, true, g3.e.computer, time.Now().UTC().Add(time.Second)), g3.e.computer, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range []types.CommitmentRecord{forged, otherKey} {
		if _, err := g3.e.rt.store.AppendCommitmentRecord(context.Background(), g3.e.owner, g3.e.computer, r); err != nil {
			t.Fatal(err)
		}
	}
	if code, _, reason := g3.post(t); code != http.StatusAccepted || reason != reasonInstallReviewOwed {
		t.Fatalf("G3 = %d %s", code, reason)
	}

	// G4 (FM2): a sealed accept for another tree of the same release id.
	g4 := newReviewFixture(t, `{"grant":"ui.read.kind","args":["notes"]}`)
	elsewhere := g4.subject()
	elsewhere.PatchSHA256 = strings.Repeat("f", 64)
	other, err := installreview.Seal(context.Background(), g4.signer, installreview.VerdictRecord(elsewhere, plan, nil, nil, true, g4.e.computer, time.Now().UTC().Add(2*time.Second)), g4.e.computer, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g4.e.rt.store.AppendCommitmentRecord(context.Background(), g4.e.owner, g4.e.computer, other); err != nil {
		t.Fatal(err)
	}
	if code, _, reason := g4.post(t); code != http.StatusAccepted || reason != reasonInstallReviewOwed {
		t.Fatalf("G4 = %d %s", code, reason)
	}
	if g4.e.records(t) != 0 {
		t.Fatal("G4: a verdict for another tree admitted this one")
	}
}

func TestInstallReviewOwedPendsAndAnAcceptingVerdictInstallsWithoutASecondOwnerAct(t *testing.T) {
	f := newNotesReviewFixture(t)
	e := f.e
	code, _, reason := f.post(t)
	if code != http.StatusAccepted || reason != reasonInstallReviewOwed {
		t.Fatalf("first request = %d %s, want 202 install_review_owed", code, reason)
	}
	// The pending row holds the whole request; the owed record is addressed to
	// management and binds the subject.
	row := f.pendingRow(t)
	if !row.Open() || row.Attempt != 1 || string(row.Record) != string(f.raw) || computerevent.DigestBytes(row.TreePatch) != computerevent.DigestBytes(f.tree) || row.ReleaseID != f.id {
		t.Fatalf("row = %+v", row)
	}
	owed, err := e.rt.store.GetCommitmentRecord(context.Background(), e.owner, e.computer, types.InstallReviewOwedRecordID(f.id, f.subjectDigest(), "", 1))
	if err != nil || owed == nil || owed.Inbox != persistentManagementAgentID(e.owner) || owed.SubjectRefs["subject_digest"] != f.subjectDigest() || owed.SubjectRefs["path"] != types.InstallReviewPathInstall {
		t.Fatalf("owed record = %+v, %v", owed, err)
	}
	// The record minted its own clock in its commit: a tick and the deadline.
	var stages []string
	for _, d := range f.dueWakes(t) {
		stages = append(stages, d.Stage)
	}
	if len(stages) != 2 {
		t.Fatalf("re-entry wakes = %v, want a tick and the deadline", stages)
	}
	if e.records(t) != 0 {
		t.Fatal("the install recorded before the verdict")
	}
	// The installs API lists the pending row.
	var pendingViews int
	for _, v := range e.list(t) {
		if v.State == installStatePendingReview && v.ReleaseID == f.id {
			pendingViews++
		}
	}
	if pendingViews != 1 {
		t.Fatalf("installs API pending views = %d", pendingViews)
	}

	// Management reads the packet, then judges; the runtime seals the verdict
	// and re-runs the stored request off the cell.
	out := f.judge(t, true)
	if out["verdict"] != installreview.VerdictAccept || out["installing"] != true {
		t.Fatalf("verdict result = %v", out)
	}
	if e.records(t) != 1 {
		t.Fatalf("installs after the accepting verdict = %d, want 1 with no second owner act", e.records(t))
	}
	if row := f.pendingRow(t); row.State != store.PackageInstallPendingInstalled {
		t.Fatalf("row state = %s", row.State)
	}
	// The seat record is management's own prediction at the standard question.
	var seats, verdicts int
	for _, r := range reviewRecords(t, e, f.id) {
		switch r.SubjectRefs[installreview.RefRole] {
		case installreview.RoleSeat:
			seats++
			if r.Kind != types.CommitmentKindPrecommit || r.Precommit == nil || r.SubjectRefs[installreview.RefSeat] != installReviewJudgeDesk() ||
				r.Precommit.YesProbability() != 0.9 || r.Precommit.Question != installreview.StandardQuestions[1] {
				t.Fatalf("seat record %+v", r)
			}
		case installreview.RoleVerdict:
			if installreview.VerifySeal(r, e.computer, f.signer.key.PrivateKey.Public().(ed25519.PublicKey)) == nil {
				verdicts++
			}
		}
	}
	if seats != 1 || verdicts != 1 {
		t.Fatalf("records: %d seats, %d sealed verdicts", seats, verdicts)
	}
	// The same request now installs at once (the verdict stays).
	if code, _, _ := f.post(t); code != http.StatusOK {
		t.Fatalf("repeat after install = %d", code)
	}
}

func TestInstallReviewRejectStands(t *testing.T) {
	f := newReviewFixture(t, `{"grant":"ui.read.kind","args":["notes"]},{"grant":"ui.net.egress","args":["api.example.com"]}`)
	e := f.e
	if code, _, reason := f.post(t); code != http.StatusAccepted || reason != reasonInstallReviewOwed {
		t.Fatalf("pend = %d %s", code, reason)
	}
	out := f.judge(t, false)
	if out["verdict"] != installreview.VerdictReject {
		t.Fatalf("verdict result = %v", out)
	}
	row := f.pendingRow(t)
	if row.State != store.PackageInstallPendingRejected || !strings.Contains(row.Reason, "posts notes") {
		t.Fatalf("row = %+v", row)
	}
	// G5: a later try meets the sealed rejection with management's reason.
	code, _, reason := f.post(t)
	if code != http.StatusForbidden || reason != installreview.ReasonRejected {
		t.Fatalf("G5 = %d %s", code, reason)
	}
	if e.records(t) != 0 {
		t.Fatalf("G5: recorded %d installs", e.records(t))
	}
	// The installs API says review_rejected with the reason.
	found := false
	for _, v := range e.list(t) {
		if v.ReleaseID == f.id && v.State == store.PackageInstallPendingRejected && strings.Contains(v.Reason, "posts notes") {
			found = true
		}
	}
	if !found {
		t.Fatalf("installs API = %+v", e.list(t))
	}
}

func TestInstallReviewDedupeAnswersTheSame202AndHeals(t *testing.T) {
	f := newNotesReviewFixture(t)
	e := f.e
	if code, _, _ := f.post(t); code != http.StatusAccepted {
		t.Fatal("first request")
	}
	builds := e.rt.packageBuilder.(*manifestBuilder).builds
	// A second request for the same subject answers the same 202, with no build.
	code, _, reason := f.post(t)
	if code != http.StatusAccepted || reason != reasonInstallReviewOwed {
		t.Fatalf("second request = %d %s", code, reason)
	}
	if got := e.rt.packageBuilder.(*manifestBuilder).builds; got != builds {
		t.Fatalf("the dedupe built the release again (%d -> %d)", builds, got)
	}
	if n := len(f.dueWakes(t)); n != 2 {
		t.Fatalf("re-entry wakes after a repeat = %d, want 2 (no second review)", n)
	}

	// A row with no owed record (a failed commit) is healed by the next
	// request: the record is committed with its clock before the 202.
	g := newReviewFixture(t, `{"grant":"ui.read.kind","args":["mail"]}`)
	row := store.PackageInstallPending{
		OwnerID: g.e.owner, ComputerID: g.e.computer, SubjectDigest: g.subjectDigest(), Attempt: 1, ReleaseID: g.id, Package: installName, Action: packageActionInstall,
		Record: g.raw, TreePatch: g.tree, Change: encodePendingChange(installChange{}), OwedRecordID: types.InstallReviewOwedRecordID(g.id, g.subjectDigest(), "", 1),
	}
	if err := g.e.rt.store.PutPackageInstallPending(context.Background(), row); err != nil {
		t.Fatal(err)
	}
	if rec, _ := g.e.rt.store.GetCommitmentRecord(context.Background(), g.e.owner, g.e.computer, row.OwedRecordID); rec != nil {
		t.Fatal("the fixture already has an owed record")
	}
	if code, _, reason := g.post(t); code != http.StatusAccepted || reason != reasonInstallReviewOwed {
		t.Fatalf("healing request = %d %s", code, reason)
	}
	if rec, _ := g.e.rt.store.GetCommitmentRecord(context.Background(), g.e.owner, g.e.computer, row.OwedRecordID); rec == nil {
		t.Fatal("the dedupe hit did not heal the missing owed record")
	}
	if len(g.dueWakes(t)) < 2 {
		t.Fatalf("the healed record has no clock: %d wakes", len(g.dueWakes(t)))
	}
}

func TestInstallReviewTimeoutSealsNothingAndANewAttemptOpens(t *testing.T) {
	f := newNotesReviewFixture(t)
	e := f.e
	if code, _, _ := f.post(t); code != http.StatusAccepted {
		t.Fatal("first request")
	}
	// A tick before its time is deferred, not spent.
	if err := f.due(t, types.InstallReviewStageTick, 0); err == nil {
		t.Fatal("a tick before the notice interval was served")
	}
	// Notices are capped at two more after the record.
	for want := 1; want <= types.InstallReviewOwedMaxNotices+1; want++ {
		f.advance(types.InstallReviewOwedNoticeAfter + time.Second)
		n := want - 1
		if n > types.InstallReviewOwedMaxNotices {
			n = types.InstallReviewOwedMaxNotices
		}
		if err := f.due(t, types.InstallReviewStageTick, n); err != nil {
			t.Fatalf("tick %d: %v", want, err)
		}
	}
	notices, _ := e.rt.installReviewNotices(context.Background(), e.owner, e.computer, f.id, f.subjectDigest(), "", 1)
	if notices != types.InstallReviewOwedMaxNotices {
		t.Fatalf("notices = %d, want exactly the cap %d", notices, types.InstallReviewOwedMaxNotices)
	}
	if extra, _ := e.rt.store.GetCommitmentRecord(context.Background(), e.owner, e.computer, types.InstallReviewOwedNoticeID(f.id, f.subjectDigest(), "", 1, types.InstallReviewOwedMaxNotices+1)); extra != nil {
		t.Fatal("a notice past the cap was committed")
	}
	// Each notice minted the next tick; the cap-th minted none.
	// The deadline: a deadline wake before 45 minutes is deferred, after it
	// closes the review with nothing sealed.
	if err := f.due(t, types.InstallReviewStageDeadline, 0); err == nil {
		t.Fatal("the deadline closed the review early")
	}
	f.advance(types.InstallReviewOwedDeadline)
	if err := f.due(t, types.InstallReviewStageDeadline, 0); err != nil {
		t.Fatal(err)
	}
	row := f.pendingRow(t)
	if row.State != store.PackageInstallPendingTimedOut || !strings.Contains(row.Reason, installReviewNotJudged) {
		t.Fatalf("row = %+v", row)
	}
	if recs := reviewRecords(t, e, f.id); len(recs) != 0 {
		t.Fatalf("a timeout sealed %d record(s)", len(recs))
	}
	// A late verdict is refused: the review is closed.
	if _, err := f.verdict(true, 0.9, nil, ""); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("late verdict = %v", err)
	}
	// A later attempt of the same subject opens a fresh review.
	if code, _, reason := f.post(t); code != http.StatusAccepted || reason != reasonInstallReviewOwed {
		t.Fatalf("new attempt = %d %s", code, reason)
	}
	if row := f.pendingRow(t); row.Attempt != 2 || !row.Open() {
		t.Fatalf("second row = %+v", row)
	}
	if rec, _ := e.rt.store.GetCommitmentRecord(context.Background(), e.owner, e.computer, types.InstallReviewOwedRecordID(f.id, f.subjectDigest(), "", 2)); rec == nil {
		t.Fatal("the second attempt has no owed record of its own")
	}
	// And it can be judged: the verdict lands on the new attempt.
	f.judge(t, true)
	if e.records(t) != 1 {
		t.Fatalf("installs after the second attempt's verdict = %d", e.records(t))
	}
}

func TestInstallVerbsRefuseOtherDesks(t *testing.T) {
	f := newNotesReviewFixture(t)
	if code, _, _ := f.post(t); code != http.StatusAccepted {
		t.Fatal("first request")
	}
	for _, profile := range []string{agentprofile.Research, agentprofile.Texture, agentprofile.Engineering} {
		_, err := f.reduction(profile).commitInstallVerdictIntent(context.Background(), yaegikernel.StagedIntent{
			LocalID: "v", Kind: yaegikernel.IntentInstallVerdict, TargetRef: f.id, PacketRef: f.packetRef(), Decision: "accept", PWithinCeiling: 0.9,
		})
		if err == nil || !strings.Contains(err.Error(), "management's") {
			t.Fatalf("%s verdict = %v", profile, err)
		}
		payload, _ := json.Marshal(yaegikernel.InstallPacketPayload{ReleaseID: f.id})
		if _, err := f.e.rt.installPacketEgress(context.Background(), profile, toolregistry.ExecutionContext{OwnerID: f.e.owner, ComputerID: f.e.computer}, payload); err == nil || !strings.Contains(err.Error(), "management's") {
			t.Fatalf("%s packet = %v", profile, err)
		}
	}
	if f.e.records(t) != 0 || len(reviewRecords(t, f.e, f.id)) != 0 {
		t.Fatal("another desk sealed a verdict")
	}
}

func TestInstallPacketTruncatesTheSource(t *testing.T) {
	f := newReviewFixture(t, `{"grant":"ui.read.kind","args":["notes"]},{"grant":"ui.net.egress","args":["api.example.com"]}`)
	e := f.e
	e.rt.packageBuilder = &manifestBuilder{manifest: f.manifest, source: []byte("export const abi='choir.app-frontend/module/v1'; // " + strings.Repeat("x", installReviewSourceBytes+10))}
	exec := toolregistry.ExecutionContext{OwnerID: e.owner, ComputerID: e.computer}
	payload, _ := json.Marshal(yaegikernel.InstallPacketPayload{ReleaseID: f.id})
	// Refused when no review is owed.
	if _, err := e.rt.installPacketEgress(context.Background(), agentprofile.Management, exec, payload); err == nil || !strings.Contains(err.Error(), "no review is owed") {
		t.Fatalf("packet with nothing owed = %v", err)
	}
	if code, _, _ := f.post(t); code != http.StatusAccepted {
		t.Fatal("first request")
	}
	raw, err := e.rt.installPacketEgress(context.Background(), agentprofile.Management, exec, payload)
	if err != nil {
		t.Fatal(err)
	}
	var packet yaegikernel.InstallPacketResult
	if err := json.Unmarshal(raw, &packet); err != nil {
		t.Fatal(err)
	}
	if !packet.Truncated || len(packet.Source) > installReviewSourceBytes+len(installPacketSourceNote) || !strings.HasSuffix(packet.Source, "[truncated]") {
		t.Fatalf("FM5: truncated=%v source=%d bytes", packet.Truncated, len(packet.Source))
	}
	if packet.ReleaseID != f.id || packet.Path != types.InstallReviewPathInstall || len(packet.Added) != 2 || len(packet.Flow) == 0 || packet.ManifestJSON == "" || packet.Risk != string(types.ImpactI3) {
		t.Fatalf("packet = %+v", packet)
	}
	if !strings.Contains(strings.Join(packet.Flow, " "), "api.example.com") {
		t.Fatalf("flow sentences = %v", packet.Flow)
	}
}

func TestInstallVerdictReplayAndSecondVerdict(t *testing.T) {
	f := newNotesReviewFixture(t)
	if code, _, _ := f.post(t); code != http.StatusAccepted {
		t.Fatal("first request")
	}
	findings := []yaegikernel.InstallFinding{{Concern: "reads notes", Evidence: "App.svelte"}}
	if _, err := f.verdict(true, 0.9, findings, ""); err != nil {
		t.Fatal(err)
	}
	f.e.rt.installReviewRerun.Wait()
	before := len(reviewRecords(t, f.e, f.id))
	// FM6: an identical replay answers the sealed record and writes nothing.
	out, err := f.verdict(true, 0.5, findings, "")
	if err != nil || out["replay"] != true {
		t.Fatalf("replay = %v, %v", out, err)
	}
	if after := len(reviewRecords(t, f.e, f.id)); after != before {
		t.Fatalf("a replay wrote records (%d -> %d)", before, after)
	}
	// A different second verdict is refused.
	if _, err := f.verdict(false, 0.1, findings, "changed my mind"); err == nil || !strings.Contains(err.Error(), "different sealed verdict") {
		t.Fatalf("different second verdict = %v", err)
	}
	if after := len(reviewRecords(t, f.e, f.id)); after != before {
		t.Fatalf("a refused verdict wrote records (%d -> %d)", before, after)
	}
}

func TestInstallVerdictBlockingFindingRejects(t *testing.T) {
	f := newNotesReviewFixture(t)
	if code, _, _ := f.post(t); code != http.StatusAccepted {
		t.Fatal("first request")
	}
	out, err := f.verdict(true, 0.9, []yaegikernel.InstallFinding{{Concern: "posts notes to a sink", Blocking: true}}, "")
	if err != nil || out["verdict"] != installreview.VerdictReject {
		t.Fatalf("verdict = %v, %v", out, err)
	}
	f.e.rt.installReviewRerun.Wait()
	if f.e.records(t) != 0 || f.pendingRow(t).State != store.PackageInstallPendingRejected {
		t.Fatalf("a blocking finding installed (%d) or left the row %s", f.e.records(t), f.pendingRow(t).State)
	}
	if code, _, reason := f.post(t); code != http.StatusForbidden || reason != installreview.ReasonRejected {
		t.Fatalf("retry = %d %s", code, reason)
	}
}

func TestInstallReviewMechanicalFailureSealsAtOnce(t *testing.T) {
	f := newNotesReviewFixture(t)
	// The built tree's manifest is not the record's: the check fails and a
	// rejection is sealed without troubling management.
	f.e.rt.packageBuilder = &manifestBuilder{manifest: strings.Replace(f.manifest, `"1.0.0"`, `"1.0.1"`, 1)}
	code, _, reason := f.post(t)
	if code != http.StatusForbidden || reason != installreview.ReasonRejected {
		t.Fatalf("mechanical failure = %d %s", code, reason)
	}
	if rec, _ := f.e.rt.store.GetCommitmentRecord(context.Background(), f.e.owner, f.e.computer, types.InstallReviewOwedRecordID(f.id, f.subjectDigest(), "", 1)); rec != nil {
		t.Fatal("management was asked to review a release the checks refused")
	}
	if rows, _ := f.e.rt.store.ListPackageInstallPending(context.Background(), f.e.owner, f.e.computer, false); len(rows) != 0 {
		t.Fatalf("pending rows = %d", len(rows))
	}
}

func TestInstallReviewSupersededByANewerInstall(t *testing.T) {
	f := newNotesReviewFixture(t)
	e := f.e
	if code, _, _ := f.post(t); code != http.StatusAccepted {
		t.Fatal("first request")
	}
	// A newer release of the package (default level: no review) installs.
	e.rt.packageBuilder = &fakeModuleBuilder{}
	_, id2, raw2, tree2 := publishedPackage(t, e.repo, e.base, "publisher-x", map[string]string{
		"choir-package.json": strings.Replace(installManifest("Demo"), `"version": "1.0.0"`, `"version": "1.1.0"`, 1), "App.svelte": "<p>newer</p>\n",
	})
	if code, _, reason := e.post(t, installBody(t, id2, raw2, tree2, "")); code != http.StatusCreated {
		t.Fatalf("newer install = %d %s", code, reason)
	}
	if row := f.pendingRow(t); row.State != store.PackageInstallPendingSuperseded {
		t.Fatalf("FM7: the row is %s after a newer install", row.State)
	}
	// The verdict that lands afterwards installs nothing.
	e.rt.packageBuilder = &manifestBuilder{manifest: f.manifest}
	if _, err := f.verdict(true, 0.9, nil, ""); err == nil {
		t.Fatal("a verdict on a superseded review was accepted")
	}
	e.rt.installReviewRerun.Wait()
	current := currentPackageInstalls(mustInstalls(t, e))[installName]
	if current == nil || current.ReleaseID != id2 {
		t.Fatalf("current install = %+v, want the newer release %s", current, id2)
	}
}

func mustInstalls(t *testing.T, e *installEnv) []store.PackageInstall {
	t.Helper()
	list, err := e.rt.packageInstalls(context.Background(), e.owner, e.computer)
	if err != nil {
		t.Fatal(err)
	}
	return list
}

func TestPendingRerunWithAStaleBaselineInstallsNothing(t *testing.T) {
	f := newNotesReviewFixture(t)
	e := f.e
	if code, _, _ := f.post(t); code != http.StatusAccepted {
		t.Fatal("first request")
	}
	// An accepting verdict is sealed, but the package changes before the
	// re-run: install a newer default release directly, then run the stored
	// request. The re-run closes the row superseded and installs nothing.
	row := f.pendingRow(t)
	owed, found, err := e.rt.installReviewOwedOn(context.Background(), e.owner, e.computer, f.id)
	if err != nil || !found {
		t.Fatal(err)
	}
	info, err := e.rt.installReviewSubjectOf(context.Background(), e.owner, e.computer, owed)
	if err != nil {
		t.Fatal(err)
	}
	if refusal := e.rt.sealInstallVerdict(context.Background(), e.owner, e.computer, info.Subject, info.Plan, info.Checks, nil, true, "", time.Now()); refusal != nil {
		t.Fatal(refusal)
	}
	if ok, err := e.rt.store.ClosePackageInstallPending(context.Background(), e.owner, e.computer, row.SubjectDigest, row.Attempt, store.PackageInstallPendingOpen, store.PackageInstallPendingAccepted, ""); err != nil || !ok {
		t.Fatal(err)
	}
	e.rt.packageBuilder = &fakeModuleBuilder{}
	_, id2, raw2, tree2 := publishedPackage(t, e.repo, e.base, "publisher-x", map[string]string{
		"choir-package.json": strings.Replace(installManifest("Demo"), `"version": "1.0.0"`, `"version": "1.1.0"`, 1), "App.svelte": "<p>newer</p>\n",
	})
	if code, _, reason := e.post(t, installBody(t, id2, raw2, tree2, "")); code != http.StatusCreated {
		t.Fatalf("newer install = %d %s", code, reason)
	}
	e.rt.packageBuilder = &manifestBuilder{manifest: f.manifest}
	accepted := f.pendingRow(t)
	accepted.State = store.PackageInstallPendingAccepted
	e.rt.completePendingInstall(context.Background(), accepted)
	if got := f.pendingRow(t); got.State != store.PackageInstallPendingSuperseded {
		t.Fatalf("FM7: the re-run left the row %s", got.State)
	}
	if current := currentPackageInstalls(mustInstalls(t, e))[installName]; current == nil || current.ReleaseID != id2 {
		t.Fatalf("the stale re-run replaced the newer install: %+v", current)
	}
}

func TestInstallReviewCrashBootClosesThePendingInstall(t *testing.T) {
	f := newNotesReviewFixture(t)
	e := f.e
	if code, _, _ := f.post(t); code != http.StatusAccepted {
		t.Fatal("first request")
	}
	// FM8: the verdict is sealed and the row accepted, but the process dies
	// before the re-run finished.
	owed, _, err := e.rt.installReviewOwedOn(context.Background(), e.owner, e.computer, f.id)
	if err != nil {
		t.Fatal(err)
	}
	info, err := e.rt.installReviewSubjectOf(context.Background(), e.owner, e.computer, owed)
	if err != nil {
		t.Fatal(err)
	}
	if refusal := e.rt.sealInstallVerdict(context.Background(), e.owner, e.computer, info.Subject, info.Plan, info.Checks, nil, true, "", time.Now()); refusal != nil {
		t.Fatal(refusal)
	}
	row := f.pendingRow(t)
	if ok, err := e.rt.store.ClosePackageInstallPending(context.Background(), e.owner, e.computer, row.SubjectDigest, row.Attempt, store.PackageInstallPendingOpen, store.PackageInstallPendingAccepted, ""); err != nil || !ok {
		t.Fatal(err)
	}
	e.rt.closeOpenObligationsAtBoot(context.Background())
	got := f.pendingRow(t)
	if got.State != store.PackageInstallPendingInterrupted || got.Reason != installReviewRestartCause {
		t.Fatalf("row after a crash boot = %+v", got)
	}
	if e.records(t) != 0 {
		t.Fatal("a crash boot resumed the install")
	}
	// The sealed verdict stays: the owner's next install of the same bytes
	// passes the gate at once.
	if code, _, reason := f.post(t); code != http.StatusCreated {
		t.Fatalf("install after the crash boot = %d %s", code, reason)
	}
}

func TestInstallReviewOwedCommitRereadsTheGate(t *testing.T) {
	f := newNotesReviewFixture(t)
	e := f.e
	// FM9: a verdict is sealed after this request's first read of the ledger
	// and before its owed commit.
	e.rt.installReviewBeforeOwed = func() {
		e.rt.installReviewBeforeOwed = nil
		rec, _ := pkgrelease.Decode(f.raw)
		module := pkgruntime.Module{Manifest: json.RawMessage(f.manifest), ABI: pkgruntime.ModuleABI, Bytes: []byte("x")}
		subject, checks, plan := e.rt.installReviewPlanFor(context.Background(), e.owner, e.computer, rec, installReviewSubject(f.id, rec, f.tree, module, nil), module)
		if refusal := e.rt.sealInstallVerdict(context.Background(), e.owner, e.computer, subject, plan, checks, nil, true, "", time.Now()); refusal != nil {
			t.Error(refusal)
		}
	}
	code, _, reason := f.post(t)
	if code != http.StatusCreated {
		t.Fatalf("a verdict that landed before the owed commit still opened a review: %d %s", code, reason)
	}
	if rows, _ := e.rt.store.ListPackageInstallPending(context.Background(), e.owner, e.computer, false); len(rows) != 0 {
		t.Fatalf("pending rows = %d, want none", len(rows))
	}
	if rec, _ := e.rt.store.GetCommitmentRecord(context.Background(), e.owner, e.computer, types.InstallReviewOwedRecordID(f.id, f.subjectDigest(), "", 1)); rec != nil {
		t.Fatal("an owed record was committed for a review a verdict already answers")
	}
}

func TestInstallReviewDeferredNeverOpensAReview(t *testing.T) {
	f := newNotesReviewFixture(t)
	e := f.e
	// FM10: the boot migration defers; it opens no review, row or record.
	_, _, refusal := e.rt.installPackage(context.Background(), e.owner, e.computer, packageInstallRequest{
		ReleaseID: f.id, Record: f.raw, TreePatch: f.tree, change: installChange{deferReview: true},
	})
	if refusal == nil || refusal.reason != installReviewDeferred {
		t.Fatalf("deferred = %+v", refusal)
	}
	if rows, _ := e.rt.store.ListPackageInstallPending(context.Background(), e.owner, e.computer, false); len(rows) != 0 {
		t.Fatalf("the migration opened %d pending row(s)", len(rows))
	}
	if rec, _ := e.rt.store.GetCommitmentRecord(context.Background(), e.owner, e.computer, types.InstallReviewOwedRecordID(f.id, f.subjectDigest(), "", 1)); rec != nil {
		t.Fatal("the migration committed an owed record")
	}
}

func TestInstallReviewGuardAsksOverTheVerdict(t *testing.T) {
	f := newNotesReviewFixture(t)
	if code, _, _ := f.post(t); code != http.StatusAccepted {
		t.Fatal("first request")
	}
	rec := &types.RunRecord{RunID: "run-g", OwnerID: f.e.owner, ComputerID: f.e.computer, AgentID: persistentManagementAgentID(f.e.owner), AgentProfile: agentprofile.Management}
	res, err := f.e.rt.managementVerifyOwedGuard(rec)(context.Background(), completionStateForTest(0))
	if err != nil || !res.Continue || !strings.Contains(res.Instruction, "Install review owed on "+f.id) {
		t.Fatalf("guard = %+v, %v", res, err)
	}
	f.judge(t, true)
	res, err = f.e.rt.managementVerifyOwedGuard(rec)(context.Background(), completionStateForTest(0))
	if err != nil || res.Continue {
		t.Fatalf("guard after the verdict = %+v, %v", res, err)
	}
}

func TestUpdateRefusalWaitingOnAReviewIsPendingNotAnError(t *testing.T) {
	var view PackageUpdateView
	(&Runtime{}).noteUpdateRefusal(&view, installReviewOwed202("rec", installReviewOwedDetail))
	if view.State != pkgrelease.UpdateStatePendingReview || view.Error != "" {
		t.Fatalf("view = %+v", view)
	}
	view = PackageUpdateView{}
	(&Runtime{}).noteUpdateRefusal(&view, refuseInstall(http.StatusForbidden, installreview.ReasonRejected, "no"))
	if view.State == pkgrelease.UpdateStatePendingReview || view.Error == "" {
		t.Fatalf("view = %+v", view)
	}
}

// R8: a default-level release installs with the mechanical path only, with
// no signer and no review (the existing install tests run this path too).
func TestInstallReviewLeavesADefaultInstallAlone(t *testing.T) {
	e := newInstallEnv(t)
	_, id, raw, tree := publishedPackage(t, e.repo, e.base, e.owner, map[string]string{
		"choir-package.json": installManifest("Demo"), "App.svelte": "<p>demo</p>\n",
	})
	if code, _, reason := e.post(t, installBody(t, id, raw, tree, "")); code != http.StatusCreated {
		t.Fatalf("default install = %d %s", code, reason)
	}
}
