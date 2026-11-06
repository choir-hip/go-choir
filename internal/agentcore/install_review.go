package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/apppackage"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/installreview"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
	"github.com/yusefmosiah/go-choir/internal/pkgruntime"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/trust"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The install review gate (MA7.5, trust draft section 7). An install whose
// ceiling is widened (an extended grant, a desk, or an update that adds a
// ceiling entry) does not record until a sealed verdict for exactly its
// release, tree, manifest and behaviour surface is on this computer's ledger.
// With none, the pipeline runs here, in the install request: mechanical
// checks, then the seats the risk table sizes (Devin, DeepSeek), each answer
// a precommitment record; the verdict is sealed by the guest-core signer and
// written before the gate reads it back. A seat that cannot run (no provider
// wired) leaves no verdict and the install refused; nothing stands in for it.
// The seats run under packageInstallMu, so one review at a time per computer.

const (
	// installReviewSeatTimeout bounds one seat's model call.
	installReviewSeatTimeout = 5 * time.Minute
	// installReviewSourceBytes bounds the code a seat reads.
	installReviewSourceBytes = 120 << 10
	// installReviewRecordWindow bounds the review records the gate reads for
	// one release.
	installReviewRecordWindow = 500
)

// WithInstallReviewSigner wires the guest-core signer that seals install
// review verdicts (receiptsigner.Client).
func WithInstallReviewSigner(signer installreview.Signer) RuntimeOption {
	return func(rt *Runtime) { rt.installReviewSigner = signer }
}

func (rt *Runtime) reviewSeats() map[string]installreview.Seat {
	if rt.installReviewSeats != nil {
		return rt.installReviewSeats
	}
	// Only a provider that serves the requested model natively may sit a
	// seat: the tool-loop adapter ignores req.Provider and req.Model, so a
	// seat's record would name a model that never answered (review r1).
	var caller installreview.Caller
	if native, ok := rt.provider.(provideriface.ToolLoopProvider); ok {
		caller = native
	}
	return installreview.Roster(caller)
}

// lenientManifest reads the ceiling and level fields of a stored or built
// manifest without validating it (a pre-v2 install record still names its
// ceiling, if any).
func lenientManifest(raw json.RawMessage) apppackage.Manifest {
	var m apppackage.Manifest
	_ = json.Unmarshal(raw, &m)
	return m
}

// installReviewGate refuses an install whose widened ceiling has no sealed
// accepting verdict, running the review when none exists.
// installReviewDeferred refuses a release that needs a review when the caller
// asked not to run one now (the boot migration).
const installReviewDeferred = "install_review_deferred"

func (rt *Runtime) installReviewGate(ctx context.Context, ownerID, computerID, releaseID string, rec pkgrelease.Record, treePatch []byte, module pkgruntime.Module, current *store.PackageInstall, deferReview bool) *installRefusal {
	subject := installreview.Subject{
		ReleaseID: releaseID, PatchSHA256: computerevent.DigestBytes(treePatch), ManifestSHA256: rec.Manifest.SHA256,
		BehaviourSurface: rec.Manifest.BehaviourSurface, Package: rec.Package.Name, Publisher: rec.Track,
		Manifest: lenientManifest(module.Manifest),
	}
	if current != nil && current.Package == rec.Package.Name {
		subject.HasPrior, subject.PriorCeiling = true, installreview.Ceiling(lenientManifest(current.Manifest))
	}
	widened := subject.Widened() || rec.Manifest.Level != string(apppackage.LevelDefault) || rec.Manifest.DesksCount > 0
	if !widened {
		return nil
	}
	if deferReview && rt.installReviewSigner == nil {
		return refuseInstall(http.StatusConflict, installReviewDeferred, "this release widens the computer's ceiling and needs an install review; this computer has no verdict signer now")
	}
	if rt.installReviewSigner == nil {
		return refuseInstall(http.StatusServiceUnavailable, "install_review_unavailable", "this release widens the computer's ceiling and needs a review verdict; this computer has no verdict signer")
	}
	_, pub, err := rt.installReviewSigner.PublicKey(ctx)
	if err != nil {
		return refuseInstall(http.StatusServiceUnavailable, "install_review_unavailable", "the verdict signer: %v", err)
	}
	read := func() (installreview.Decision, *installRefusal) {
		records, err := rt.store.ListCommitmentRecords(ctx, ownerID, computerID, installreview.AddresseePrefix+releaseID, installReviewRecordWindow)
		if err != nil {
			return installreview.Decision{}, refuseInstall(http.StatusInternalServerError, "install_store", "%v", err)
		}
		d := installreview.Gate(records, subject, computerID, pub)
		for _, why := range d.Ignored {
			log.Printf("install review: release %s: ignored %s", releaseID, why)
		}
		return d, nil
	}
	decision, refusal := read()
	if refusal != nil {
		return refusal
	}
	if decision.Reason == installreview.ReasonRequired && deferReview {
		return refuseInstall(http.StatusConflict, installReviewDeferred, "this release widens the computer's ceiling and needs an install review; it was not run now")
	}
	if decision.Reason == installreview.ReasonRequired {
		if refusal := rt.runInstallReview(ctx, ownerID, computerID, rec, subject, module); refusal != nil {
			return refusal
		}
		if decision, refusal = read(); refusal != nil {
			return refusal
		}
	}
	switch {
	case decision.Allowed:
		return nil
	case decision.Reason == installreview.ReasonRejected:
		return refuseInstall(http.StatusForbidden, installreview.ReasonRejected, "the install review rejected this release (%s): %s", decision.VerdictID, decision.Detail)
	}
	return refuseInstall(http.StatusForbidden, installreview.ReasonRequired, "%s", decision.Detail)
}

// runInstallReview runs the pipeline once and writes its records: the seat
// precommitments, then the sealed verdict. It writes nothing when a seat
// cannot run or the plan refuses.
func (rt *Runtime) runInstallReview(ctx context.Context, ownerID, computerID string, rec pkgrelease.Record, subject installreview.Subject, module pkgruntime.Module) *installRefusal {
	built := len(module.Bytes) > 0 && module.ABI == pkgruntime.ModuleABI || len(module.Bytes) == 0 && module.ABI == ""
	checks, parsed := installreview.Mechanical(installreview.MechanicalInput{
		ManifestRaw: module.Manifest, RecordSHA256: rec.Manifest.SHA256, RecordLevel: rec.Manifest.Level,
		RecordDesks: rec.Manifest.DesksCount, ModuleBuilt: built,
	})
	if installreview.Passed(checks) {
		subject.Manifest = parsed
	}
	plan := installreview.PlanFor(subject, rt.publisherStrong(ctx, ownerID, computerID, subject.Publisher, subject.Manifest))
	if plan.Refusal != "" {
		return refuseInstall(http.StatusForbidden, "install_review_refused", "%s", plan.Refusal)
	}
	now := time.Now().UTC()
	var seatRecords []types.CommitmentRecord
	accept := installreview.Passed(checks)
	if accept {
		source := string(module.Bytes)
		if len(source) > installReviewSourceBytes {
			source = source[:installReviewSourceBytes] + "\n// [truncated]"
		}
		packet := installreview.Packet{Subject: subject, Risk: plan.Risk, Added: plan.Added, Flow: installreview.Flow(subject.Manifest),
			Checks: checks, ManifestJSON: string(module.Manifest), Source: source}
		seats := rt.reviewSeats()
		for _, name := range plan.Seats {
			seat := seats[name]
			if seat == nil {
				return refuseInstall(http.StatusServiceUnavailable, "install_review_seat_unavailable", "review seat %s is not in the roster", name)
			}
			seatCtx, cancel := context.WithTimeout(ctx, installReviewSeatTimeout)
			v, err := seat.Review(seatCtx, packet)
			cancel()
			if errors.Is(err, installreview.ErrSeatUnavailable) {
				return refuseInstall(http.StatusServiceUnavailable, "install_review_seat_unavailable", "this release needs the %s review seat (%s risk, %s publisher record) and it cannot run: %v", name, plan.Risk, recordWord(plan.Strong), err)
			}
			if err != nil {
				return refuseInstall(http.StatusBadGateway, "install_review_seat_failed", "review seat %s: %v", name, err)
			}
			seatRecords = append(seatRecords, installreview.SeatRecord(subject, plan, name, seat.Model(), v, computerID, now))
			if !v.Accept || v.Blocking {
				accept = false
			}
		}
	}
	verdict := installreview.VerdictRecord(subject, plan, checks, seatRecords, accept, computerID, now)
	sealed, err := installreview.Seal(ctx, rt.installReviewSigner, verdict, computerID, now)
	if err != nil {
		return refuseInstall(http.StatusServiceUnavailable, "install_review_unavailable", "%v", err)
	}
	for _, r := range append(seatRecords, sealed) {
		if _, err := rt.store.AppendCommitmentRecord(ctx, ownerID, computerID, r); err != nil {
			return refuseInstall(http.StatusInternalServerError, "install_store", "install review record: %v", err)
		}
	}
	return nil
}

func recordWord(strong bool) string {
	if strong {
		return "strong"
	}
	return "thin"
}

// publisherStrong reports whether the publisher's derived record is strong in
// every action class the ceiling requests: a row at or above its class's tau
// with no open breaker. No row is thin.
func (rt *Runtime) publisherStrong(ctx context.Context, ownerID, computerID, publisher string, m apppackage.Manifest) bool {
	if strings.TrimSpace(publisher) == "" || len(m.Capabilities) == 0 {
		return false
	}
	in, err := rt.trustInputs(ctx, ownerID, computerID)
	if err != nil {
		return false
	}
	in.Resolver = rt.installedPackageResolver(ctx, ownerID, computerID)
	rows := map[trust.ActionClass]trust.Row{}
	for _, r := range trust.Derive(in, trust.DefaultParams()).Rows {
		if r.Subject.Kind == trust.SubjectPublisher && r.Subject.ID == publisher {
			rows[r.Class] = r
		}
	}
	for _, c := range m.Capabilities {
		impact, ok := trust.GrantImpact(c.Grant)
		if !ok {
			return false
		}
		r, ok := rows[trust.ActionClass{Grant: c.Grant, Impact: impact}]
		if !ok || !r.AboveTau || r.Breaker {
			return false
		}
	}
	return true
}

// installedPackageResolver places an installed package's desk (app:<name>)
// under its package and its publisher (the release's track).
func (rt *Runtime) installedPackageResolver(ctx context.Context, ownerID, computerID string) trust.SubjectResolver {
	records, err := rt.packageInstalls(ctx, ownerID, computerID)
	if err != nil {
		return nil
	}
	current := currentPackageInstalls(records)
	byDesk := map[string]trust.SubjectInfo{}
	for name, item := range current {
		byDesk["app:"+name] = trust.SubjectInfo{Package: name, Publisher: item.PublisherTrack}
	}
	return func(desk string) trust.SubjectInfo { return byDesk[desk] }
}
