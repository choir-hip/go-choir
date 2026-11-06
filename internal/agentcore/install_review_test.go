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

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/installreview"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
	"github.com/yusefmosiah/go-choir/internal/pkgruntime"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The install review gate on the install route (MA7.5), failure modes
// written before the code:
//
//	G1  a widened-ceiling release installs with no verdict signer;
//	G2  a widened-ceiling release installs when a seat it needs cannot run,
//	    or a verdict is written for a review whose seat never answered;
//	G3  a forged verdict on the ledger (the pipeline's agent, an accept, this
//	    release's digests, no seal or another key's seal) admits it;
//	G4  a sealed verdict for another release or tree admits this one;
//	G5  a rejected release installs on a later try;
//	G6  a verdict in the install request body is read;
//	G7  a refused review stores the module.

func extendedManifest(caps string) string {
	return strings.Replace(installManifest("Demo"), `"capabilities": []`, `"capabilities": [`+caps+`]`, 1)
}

// manifestBuilder is the fake builder that returns the tree's real manifest,
// as pkgruntime.Builder does.
type manifestBuilder struct {
	fakeModuleBuilder
	manifest string
}

func (b *manifestBuilder) Build(ctx context.Context, rec pkgrelease.Record, tree []byte) (pkgruntime.Module, error) {
	m, err := b.fakeModuleBuilder.Build(ctx, rec, tree)
	if err == nil {
		m.Manifest = json.RawMessage(b.manifest)
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

type stubSeat struct {
	name   string
	accept bool
	calls  int
}

func (s *stubSeat) Name() string  { return s.name }
func (s *stubSeat) Model() string { return "stub-" + s.name }
func (s *stubSeat) Review(context.Context, installreview.Packet) (installreview.SeatVerdict, error) {
	s.calls++
	v := installreview.SeatVerdict{Accept: s.accept}
	if !s.accept {
		v.Blocking, v.Findings = true, []string{"reads notes and posts them"}
	}
	for _, q := range installreview.StandardQuestions {
		v.Predictions = append(v.Predictions, installreview.Prediction{Question: q, PYes: 0.9})
	}
	return v, nil
}

func reviewRecords(t *testing.T, e *installEnv, releaseID string) []types.CommitmentRecord {
	t.Helper()
	recs, err := e.rt.store.ListCommitmentRecords(context.Background(), e.owner, e.computer, installreview.AddresseePrefix+releaseID, 100)
	if err != nil {
		t.Fatal(err)
	}
	return recs
}

func TestInstallReviewGatesAWidenedCeiling(t *testing.T) {
	e := newInstallEnv(t)
	manifest := extendedManifest(`{"grant":"ui.read.kind","args":["notes"]}`)
	builder := &manifestBuilder{manifest: manifest}
	e.rt.packageBuilder = builder
	rec, id, raw, tree := publishedPackage(t, e.repo, e.base, "publisher-x", map[string]string{
		"choir-package.json": manifest, "App.svelte": "<p>demo</p>\n",
	})
	if rec.Manifest.Level != "extended" {
		t.Fatalf("fixture level %s", rec.Manifest.Level)
	}

	// G1: no signer.
	if code, _, reason := e.post(t, installBody(t, id, raw, tree, "")); code != http.StatusServiceUnavailable || reason != "install_review_unavailable" {
		t.Fatalf("G1 = %d %s", code, reason)
	}
	signer := newReviewSigner(t)
	e.rt.installReviewSigner = signer

	// G2: the roster's Devin seat has no provider: refused, and no verdict.
	if code, _, reason := e.post(t, installBody(t, id, raw, tree, "")); code != http.StatusServiceUnavailable || reason != "install_review_seat_unavailable" {
		t.Fatalf("G2 = %d %s", code, reason)
	}
	if recs := reviewRecords(t, e, id); len(recs) != 0 {
		t.Fatalf("G2: an unanswered review wrote %d records", len(recs))
	}

	// G3: forged verdicts on the ledger do not admit it.
	subject := installreview.Subject{ReleaseID: id, PatchSHA256: computerevent.DigestBytes(tree), ManifestSHA256: rec.Manifest.SHA256,
		BehaviourSurface: rec.Manifest.BehaviourSurface, Package: installName, Publisher: "publisher-x"}
	plan := installreview.Plan{Widened: true, Risk: types.ImpactI1, Seats: []string{installreview.SeatDevin}}
	forged := installreview.VerdictRecord(subject, plan, nil, nil, true, e.computer, time.Now().UTC())
	if _, err := e.rt.store.AppendCommitmentRecord(context.Background(), e.owner, e.computer, forged); err != nil {
		t.Fatal(err)
	}
	otherKey, err := installreview.Seal(context.Background(), newReviewSigner(t), installreview.VerdictRecord(subject, plan, nil, nil, true, e.computer, time.Now().UTC().Add(time.Second)), e.computer, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.rt.store.AppendCommitmentRecord(context.Background(), e.owner, e.computer, otherKey); err != nil {
		t.Fatal(err)
	}
	if code, _, reason := e.post(t, installBody(t, id, raw, tree, "")); code != http.StatusServiceUnavailable || reason != "install_review_seat_unavailable" {
		t.Fatalf("G3 = %d %s", code, reason)
	}

	// G4: a sealed accept for another tree of the same release id.
	elsewhere := subject
	elsewhere.PatchSHA256 = strings.Repeat("f", 64)
	other, err := installreview.Seal(context.Background(), signer, installreview.VerdictRecord(elsewhere, plan, nil, nil, true, e.computer, time.Now().UTC().Add(2*time.Second)), e.computer, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.rt.store.AppendCommitmentRecord(context.Background(), e.owner, e.computer, other); err != nil {
		t.Fatal(err)
	}
	if code, _, reason := e.post(t, installBody(t, id, raw, tree, "")); code != http.StatusServiceUnavailable || reason != "install_review_seat_unavailable" {
		t.Fatalf("G4 = %d %s", code, reason)
	}

	// G6: a verdict in the request body is not read.
	body := map[string]any{"release_id": id, "record": raw, "tree_patch": tree, "verdict": other}
	smuggled, _ := json.Marshal(body)
	if code, _, reason := e.post(t, smuggled); code != http.StatusBadRequest || reason != "request_invalid" {
		t.Fatalf("G6 = %d %s", code, reason)
	}
	if entries, _ := os.ReadDir(filepath.Join(e.rt.packageModules.Dir, "modules")); e.records(t) != 0 || len(entries) != 0 {
		t.Fatalf("G7: refusals recorded %d installs and stored modules %v", e.records(t), entries)
	}

	// The seat answers: an accept is sealed and the install records.
	devin := &stubSeat{name: installreview.SeatDevin, accept: true}
	e.rt.installReviewSeats = map[string]installreview.Seat{installreview.SeatDevin: devin}
	code, view, reason := e.post(t, installBody(t, id, raw, tree, ""))
	if code != http.StatusCreated || view.State != installStateInstalled || devin.calls != 1 {
		t.Fatalf("accepted = %d %s %+v calls=%d", code, reason, view, devin.calls)
	}
	var seats, verdicts int
	for _, r := range reviewRecords(t, e, id) {
		switch r.SubjectRefs[installreview.RefRole] {
		case installreview.RoleSeat:
			seats++
			if r.Kind != types.CommitmentKindPrecommit || r.Precommit == nil || r.Provenance.ModelID != "stub-devin" {
				t.Fatalf("seat record %+v", r)
			}
		case installreview.RoleVerdict:
			if installreview.VerifySeal(r, e.computer, signer.key.PrivateKey.Public().(ed25519.PublicKey)) == nil {
				verdicts++
			}
		}
	}
	if seats != 1 || verdicts != 2 { // the accept, and G4's sealed verdict for the other tree
		t.Fatalf("records: %d seats, %d sealed verdicts", seats, verdicts)
	}
}

func TestInstallReviewRejectStands(t *testing.T) {
	e := newInstallEnv(t)
	manifest := extendedManifest(`{"grant":"ui.read.kind","args":["notes"]},{"grant":"ui.net.egress","args":["api.example.com"]}`)
	builder := &manifestBuilder{manifest: manifest}
	e.rt.packageBuilder = builder
	e.rt.installReviewSigner = newReviewSigner(t)
	_, id, raw, tree := publishedPackage(t, e.repo, e.base, "publisher-x", map[string]string{
		"choir-package.json": manifest, "App.svelte": "<p>demo</p>\n",
	})
	devin := &stubSeat{name: installreview.SeatDevin, accept: true}
	deepseek := &stubSeat{name: installreview.SeatDeepSeek, accept: false}
	e.rt.installReviewSeats = map[string]installreview.Seat{installreview.SeatDevin: devin, installreview.SeatDeepSeek: deepseek}
	// I3 from a thin publisher: two seats; one rejects.
	if code, _, reason := e.post(t, installBody(t, id, raw, tree, "")); code != http.StatusForbidden || reason != installreview.ReasonRejected {
		t.Fatalf("reject = %d %s", code, reason)
	}
	if devin.calls != 1 || deepseek.calls != 1 {
		t.Fatalf("seats called devin=%d deepseek=%d", devin.calls, deepseek.calls)
	}
	// G5: a later try does not re-ask until an accept: the reject stands.
	deepseek.accept = true
	if code, _, reason := e.post(t, installBody(t, id, raw, tree, "")); code != http.StatusForbidden || reason != installreview.ReasonRejected {
		t.Fatalf("G5 = %d %s", code, reason)
	}
	if devin.calls != 1 || e.records(t) != 0 {
		t.Fatalf("G5: re-reviewed (devin=%d) or recorded %d", devin.calls, e.records(t))
	}
}

// R8: a default-level release installs with the mechanical path only, with
// no signer and no seats (the existing install tests run this path too).
func TestInstallReviewLeavesADefaultInstallAlone(t *testing.T) {
	e := newInstallEnv(t)
	_, id, raw, tree := publishedPackage(t, e.repo, e.base, e.owner, map[string]string{
		"choir-package.json": installManifest("Demo"), "App.svelte": "<p>demo</p>\n",
	})
	if code, _, reason := e.post(t, installBody(t, id, raw, tree, "")); code != http.StatusCreated {
		t.Fatalf("default install = %d %s", code, reason)
	}
}
