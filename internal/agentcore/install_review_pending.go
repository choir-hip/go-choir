package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/installreview"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The owner and update path of the install review
// (docs/design/desk-install-review-2026-11-08.md 2.1, 7.2): a widened-ceiling
// install that needs management's verdict waits in a durable
// package_install_pending row holding the whole request. The owed record
// mints its own re-entry wakes in its commit, so a row never waits without a
// clock. An accepting verdict re-runs the stored request detached from
// management's cell; a rejection, a timeout, a crash boot or a newer install
// of the package closes the row.

const (
	installReviewRestartCause = "interrupted by a restart"
	installReviewSuperseded   = "a newer install of the package was recorded"
)

// errInstallReviewNotYet makes the actor handler defer an install_review_due
// wake that arrived before its time.
var errInstallReviewNotYet = errors.New("install review: not due yet")

// pendingInstallChange is installChange as the pending row stores it.
type pendingInstallChange struct {
	DeferReview    bool                    `json:"defer_review,omitempty"`
	Trigger        string                  `json:"trigger,omitempty"`
	Forced         bool                    `json:"forced,omitempty"`
	PlatformReason string                  `json:"platform_reason,omitempty"`
	Listing        pkgrelease.TrackRelease `json:"listing"`
}

func encodePendingChange(c installChange) []byte {
	raw, _ := json.Marshal(pendingInstallChange{DeferReview: c.deferReview, Trigger: c.trigger, Forced: c.forced, PlatformReason: c.platformReason, Listing: c.listing})
	return raw
}

func decodePendingChange(raw []byte) installChange {
	var p pendingInstallChange
	_ = json.Unmarshal(raw, &p)
	return installChange{deferReview: p.DeferReview, trigger: p.Trigger, forced: p.Forced, platformReason: p.PlatformReason, listing: p.Listing}
}

// pendingBaseline is the install of the package in force when its row was
// written; a re-run whose package now differs installs nothing.
type pendingBaseline struct {
	release string
	at      time.Time
}

func (b pendingBaseline) matches(current *store.PackageInstall) bool {
	if current == nil {
		return b.release == ""
	}
	return current.ReleaseID == b.release && current.RequestedAt.Equal(b.at)
}

// requestSubjectDigest is the review subject's digest computed from the
// request's bytes, before any build.
func requestSubjectDigest(releaseID string, rec pkgrelease.Record, treePatch []byte) string {
	return installreview.SubjectDigest(releaseID, computerevent.DigestBytes(treePatch), rec.Manifest.SHA256, rec.Manifest.BehaviourSurface)
}

func installReviewOwed202(recordID, detail string) *installRefusal {
	r := refuseInstall(http.StatusAccepted, reasonInstallReviewOwed, "%s", detail)
	r.owed = recordID
	return r
}

const installReviewOwedDetail = "management is reviewing this release; it installs when management accepts"

// pendingInstallDedupe answers a request whose subject already waits on its
// review with the same 202, before any build, healing a row whose owed record
// is missing (a failed commit). It runs under packageInstallMu.
func (rt *Runtime) pendingInstallDedupe(ctx context.Context, ownerID, computerID, subjectDigest string) *installRefusal {
	row, err := rt.store.LatestPackageInstallPending(ctx, ownerID, computerID, subjectDigest)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return refuseInstall(http.StatusInternalServerError, "install_store", "%v", err)
	}
	if !row.Open() {
		return nil
	}
	if refusal := rt.healPendingInstall(ctx, ownerID, computerID, row); refusal != nil {
		return refusal
	}
	return installReviewOwed202(row.OwedRecordID, installReviewOwedDetail)
}

// healPendingInstall commits the owed record (with its wakes) of an open row
// that has none.
func (rt *Runtime) healPendingInstall(ctx context.Context, ownerID, computerID string, row store.PackageInstallPending) *installRefusal {
	existing, err := rt.store.GetCommitmentRecord(ctx, ownerID, computerID, row.OwedRecordID)
	if err != nil {
		return refuseInstall(http.StatusInternalServerError, "install_store", "%v", err)
	}
	if existing != nil {
		// A record that belongs to another path (a package operation's review)
		// is never this row's clock: refuse instead of waiting on a review no
		// wake will ever time out (build review r1, item 1).
		if existing.SubjectRefs["path"] != types.InstallReviewPathInstall {
			return refuseInstall(http.StatusConflict, "install_store", "the owed record %s belongs to another review path", row.OwedRecordID)
		}
		return nil
	}
	rec, err := pkgrelease.Decode(row.Record)
	if err != nil {
		return refuseInstall(http.StatusInternalServerError, "install_store", "the pending install's record does not decode: %v", err)
	}
	facts := types.InstallReviewOwedFacts{
		ReleaseID: row.ReleaseID, SubjectDigest: row.SubjectDigest, Package: row.Package, Path: types.InstallReviewPathInstall, Attempt: row.Attempt,
		PatchSHA256: computerevent.DigestBytes(row.TreePatch), ManifestSHA256: rec.Manifest.SHA256, Surface: rec.Manifest.BehaviourSurface,
	}
	owed := types.NewInstallReviewOwedRecord(persistentManagementAgentID(ownerID), computerID, facts, installReviewOwedLine(row.ReleaseID, row.Package))
	if err := rt.commitInstallReviewRecord(ctx, ownerID, computerID, owed); err != nil {
		return refuseInstall(http.StatusInternalServerError, "install_store", "%v", err)
	}
	return nil
}

// openPendingInstall writes the pending row, then the owed record with its
// clock, and answers 202. A request that finds an open row for its subject
// answers the same 202.
func (rt *Runtime) openPendingInstall(ctx context.Context, ownerID, computerID string, request packageInstallRequest, facts types.InstallReviewOwedFacts, plan installreview.Plan, current *store.PackageInstall) *installRefusal {
	attempt := 1
	latest, err := rt.store.LatestPackageInstallPending(ctx, ownerID, computerID, facts.SubjectDigest)
	switch {
	case err == nil && latest.Open():
		if refusal := rt.healPendingInstall(ctx, ownerID, computerID, latest); refusal != nil {
			return refusal
		}
		return installReviewOwed202(latest.OwedRecordID, installReviewOwedDetail)
	case err == nil:
		attempt = latest.Attempt + 1
	case !errors.Is(err, store.ErrNotFound):
		return refuseInstall(http.StatusInternalServerError, "install_store", "%v", err)
	}
	facts.Path, facts.Attempt = types.InstallReviewPathInstall, attempt
	owed := types.NewInstallReviewOwedRecord(persistentManagementAgentID(ownerID), computerID, facts, installReviewOwedLine(facts.ReleaseID, facts.Package)+" "+planSentence(plan))
	row := store.PackageInstallPending{
		OwnerID: ownerID, ComputerID: computerID, SubjectDigest: facts.SubjectDigest, Attempt: attempt,
		ReleaseID: facts.ReleaseID, Package: facts.Package, Action: packageActionInstall,
		Record: request.Record, TreePatch: request.TreePatch, Change: encodePendingChange(request.change),
		RequestedAt: time.Now().UTC(), OwedRecordID: owed.RecordID, Inbox: owed.Inbox, State: store.PackageInstallPendingOpen,
	}
	if current != nil {
		row.BaselineRelease, row.BaselineAt = current.ReleaseID, current.RequestedAt
	}
	if _, err := rt.store.PrunePackageInstallPending(ctx, time.Now().Add(-store.PackageInstallPendingRetention)); err != nil {
		log.Printf("install review: prune pending installs: %v", err)
	}
	if err := rt.store.PutPackageInstallPending(ctx, row); err != nil {
		return refuseInstall(http.StatusInternalServerError, "install_store", "%v", err)
	}
	if err := rt.commitInstallReviewRecord(ctx, ownerID, computerID, owed); err != nil {
		// The row stays; the next request for this subject heals the record.
		return refuseInstall(http.StatusInternalServerError, "install_store", "%v", err)
	}
	return installReviewOwed202(owed.RecordID, installReviewOwedDetail)
}

func planSentence(plan installreview.Plan) string {
	return "Risk " + string(plan.Risk) + "; added: " + strings.Join(plan.Added, ", ") + "."
}

// ---- completion -------------------------------------------------------------

// launchPendingInstall re-runs an accepted row's install detached from the
// cell that sealed the verdict (the verb returns when the verdict is sealed
// and the re-run is queued).
func (rt *Runtime) launchPendingInstall(row store.PackageInstallPending) {
	rt.installReviewRerun.Add(1)
	go func() {
		defer rt.installReviewRerun.Done()
		ctx, cancel := context.WithTimeout(context.WithoutCancel(context.Background()), 10*time.Minute)
		defer cancel()
		rt.completePendingInstall(ctx, row)
	}()
}

// completePendingInstall re-runs the stored request. Its authority is the
// sealed accepting verdict plus the owner's original request recorded on the
// row; the route's owner check is not repeated. It takes packageInstallMu
// itself, through installPackage.
func (rt *Runtime) completePendingInstall(ctx context.Context, row store.PackageInstallPending) {
	// The re-run has two executors (the detached goroutine and the durable
	// install_review_rerun wake); only a row still accepted is run.
	if cur, err := rt.store.GetPackageInstallPending(ctx, row.OwnerID, row.ComputerID, row.SubjectDigest, row.Attempt); err != nil || cur.State != store.PackageInstallPendingAccepted {
		return
	}
	request := packageInstallRequest{
		ReleaseID: row.ReleaseID, Record: row.Record, TreePatch: row.TreePatch, Action: row.Action,
		change: decodePendingChange(row.Change),
	}
	request.baseline = &pendingBaseline{release: row.BaselineRelease, at: row.BaselineAt}
	_, _, refusal := rt.installPackage(ctx, row.OwnerID, row.ComputerID, request)
	to, reason := store.PackageInstallPendingInstalled, ""
	if refusal != nil {
		to, reason = store.PackageInstallPendingFailed, refusal.Error()
		if refusal.reason == "install_superseded" {
			to, reason = store.PackageInstallPendingSuperseded, installReviewSuperseded
		}
	}
	if ok, err := rt.store.ClosePackageInstallPending(ctx, row.OwnerID, row.ComputerID, row.SubjectDigest, row.Attempt, store.PackageInstallPendingAccepted, to, reason); err != nil {
		log.Printf("install review: close pending install %s: %v", row.ReleaseID, err)
	} else if !ok {
		log.Printf("install review: pending install %s was closed by another executor", row.ReleaseID)
	}
	if refusal != nil {
		_ = rt.store.SetPackageInstallPendingError(ctx, row.OwnerID, row.ComputerID, row.SubjectDigest, row.Attempt, refusal.Error())
	}
}

// supersedePendingInstalls closes the open rows of a package whose newer
// install was just recorded (the subject of that install excepted).
func (rt *Runtime) supersedePendingInstalls(ctx context.Context, ownerID, computerID, pkg, exceptSubject string) {
	rows, err := rt.store.ListPackageInstallPending(ctx, ownerID, computerID, true)
	if err != nil {
		log.Printf("install review: list pending installs: %v", err)
		return
	}
	for _, row := range rows {
		if row.Package != pkg || row.SubjectDigest == exceptSubject {
			continue
		}
		if _, err := rt.store.ClosePackageInstallPending(ctx, ownerID, computerID, row.SubjectDigest, row.Attempt, store.PackageInstallPendingOpen, store.PackageInstallPendingSuperseded, installReviewSuperseded); err != nil {
			log.Printf("install review: supersede %s: %v", row.ReleaseID, err)
		}
	}
}

// ---- the clock --------------------------------------------------------------

// HandleInstallReviewDue is the install_review_due continuation, minted by the
// owed record's own commit: a tick sends the next notice (at most
// types.InstallReviewOwedMaxNotices) when management is idle, and the deadline
// closes the obligation at 45 minutes. A timeout seals nothing. The runtime
// re-reads the row, so a stale or replayed wake does nothing.
func (rt *Runtime) HandleInstallReviewDue(ctx context.Context, ownerID, computerID, agentID, content string) error {
	if rt == nil || rt.store == nil {
		return fmt.Errorf("install review due: store unavailable")
	}
	var wake types.InstallReviewDueWake
	if err := json.Unmarshal([]byte(content), &wake); err != nil || wake.SubjectDigest == "" || wake.Attempt < 1 {
		log.Printf("runtime: install review due: unreadable continuation %q", content)
		return nil
	}
	if strings.TrimSpace(agentID) != persistentManagementAgentID(ownerID) {
		return nil
	}
	row, err := rt.store.GetPackageInstallPending(ctx, ownerID, computerID, wake.SubjectDigest, wake.Attempt)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if !row.Open() {
		return nil
	}
	owed, err := rt.store.GetCommitmentRecord(ctx, ownerID, computerID, row.OwedRecordID)
	if err != nil {
		return err
	}
	now := rt.reviewNow()
	if owed == nil {
		// No record, no clock: close the row instead of deferring a deadline
		// wake forever (build review r1, item 7).
		if _, err := rt.store.ClosePackageInstallPending(ctx, ownerID, computerID, row.SubjectDigest, row.Attempt, store.PackageInstallPendingOpen, store.PackageInstallPendingInterrupted, "its owed record is missing"); err != nil {
			return err
		}
		return nil
	}
	committed := now
	if at, perr := time.Parse(time.RFC3339Nano, owed.Provenance.CommittedAt); perr == nil {
		committed = at
	}
	if now.Sub(committed) >= types.InstallReviewOwedDeadline {
		if ok, err := rt.store.ClosePackageInstallPending(ctx, ownerID, computerID, row.SubjectDigest, row.Attempt, store.PackageInstallPendingOpen, store.PackageInstallPendingTimedOut, installReviewNotJudged); err != nil {
			return err
		} else if ok {
			log.Printf("runtime: install review of %s not judged in time; closed (nothing sealed)", row.ReleaseID)
		}
		return nil
	}
	if wake.Stage == types.InstallReviewStageDeadline {
		return errInstallReviewNotYet
	}
	n, last := rt.installReviewNotices(ctx, ownerID, computerID, row.ReleaseID, row.SubjectDigest, "", row.Attempt)
	if wake.Notice != n || n >= types.InstallReviewOwedMaxNotices {
		return nil // a later notice has its own tick, or the cap is spent and the deadline closes it
	}
	since := committed
	if !last.IsZero() {
		since = last
	}
	if now.Sub(since) < types.InstallReviewOwedNoticeAfter {
		return errInstallReviewNotYet
	}
	// Only when management is idle: a working run's guard and its end ask for
	// the verdict. A busy management defers this tick, so a later one fires
	// and the notice chain is not ended by a busy moment (build review r1,
	// item 5).
	active, found, err := rt.activeRunByAgent(ctx, ownerID, agentID)
	if err != nil {
		return err
	}
	if found && !rt.runHasNoLiveLoop(active, rt.assignmentWaitNow()) {
		return errInstallReviewNotYet
	}
	facts := types.InstallReviewOwedFacts{ReleaseID: row.ReleaseID, SubjectDigest: row.SubjectDigest, Package: row.Package, Path: types.InstallReviewPathInstall,
		Attempt: row.Attempt, Notice: n + 1}
	if rec, derr := pkgrelease.Decode(row.Record); derr == nil {
		facts.PatchSHA256, facts.ManifestSHA256, facts.Surface = computerevent.DigestBytes(row.TreePatch), rec.Manifest.SHA256, rec.Manifest.BehaviourSurface
	}
	body := fmt.Sprintf("%s Notice %d of %d.", installReviewOwedLine(row.ReleaseID, row.Package), n+1, types.InstallReviewOwedMaxNotices)
	notice := types.NewInstallReviewOwedNotice(agentID, computerID, facts, body)
	return rt.commitInstallReviewRecord(ctx, ownerID, computerID, notice)
}

// HandleInstallReviewRerun is the durable re-run of an accepted pending
// install, minted in the transaction that closed the row open -> accepted. A
// planned restart (marker present) keeps the accepted row and delivers this
// wake, which runs the stored request; a crash boot has closed the row
// "interrupted by a restart", so a pre-boot wake finds nothing to run.
func (rt *Runtime) HandleInstallReviewRerun(ctx context.Context, ownerID, computerID, agentID, content string) error {
	if rt == nil || rt.store == nil {
		return fmt.Errorf("install review rerun: store unavailable")
	}
	var wake types.InstallReviewRerunWake
	if err := json.Unmarshal([]byte(content), &wake); err != nil || wake.SubjectDigest == "" || wake.Attempt < 1 {
		log.Printf("runtime: install review rerun: unreadable continuation %q", content)
		return nil
	}
	row, err := rt.store.GetPackageInstallPending(ctx, ownerID, computerID, wake.SubjectDigest, wake.Attempt)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if row.State != store.PackageInstallPendingAccepted {
		return nil
	}
	rt.completePendingInstall(ctx, row)
	return nil
}

// installReviewNotices counts the notices already committed for one review and
// when the last was.
func (rt *Runtime) installReviewNotices(ctx context.Context, ownerID, computerID, releaseID, subjectDigest, operationID string, attempt int) (int, time.Time) {
	var last time.Time
	n := 0
	for i := 1; i <= types.InstallReviewOwedMaxNotices; i++ {
		rec, err := rt.store.GetCommitmentRecord(ctx, ownerID, computerID, types.InstallReviewOwedNoticeID(releaseID, subjectDigest, operationID, attempt, i))
		if err != nil || rec == nil {
			break
		}
		n = i
		if at, perr := time.Parse(time.RFC3339Nano, rec.Provenance.CommittedAt); perr == nil {
			last = at
		}
	}
	return n, last
}

// closeInstallReviewPendingAfterCrash is the crash boot's closer for the
// pending installs (CLAUDE.md "Restarts End Work", the leases pattern): every
// row still waiting, or accepted with its re-run unfinished, is closed
// "interrupted by a restart" and never re-dispatched. A sealed verdict stays,
// so the owner's next install of the same bytes passes the gate at once. A
// planned restart (a marker) never reaches this pass: its wakes resume.
func (rt *Runtime) closeInstallReviewPendingAfterCrash(ctx context.Context) {
	if rt == nil || rt.store == nil {
		return
	}
	closed, err := rt.store.ClosePackageInstallPendingAtBoot(ctx, installReviewRestartCause)
	if err != nil {
		log.Printf("runtime: install review pending installs at boot: %v", err)
		return
	}
	for _, row := range closed {
		log.Printf("runtime: crash boot closed the pending install of %s (%s) as %s", row.ReleaseID, row.Package, installReviewRestartCause)
	}
}

// ---- the installs API -------------------------------------------------------

// pendingInstallViews lists the pending rows as install views: pending_review
// while waiting, and the closed state with its reason after.
func (rt *Runtime) pendingInstallViews(ctx context.Context, ownerID, computerID string) []PackageInstallView {
	rows, err := rt.store.ListPackageInstallPending(ctx, ownerID, computerID, false)
	if err != nil {
		log.Printf("install review: list pending installs: %v", err)
		return nil
	}
	var views []PackageInstallView
	for _, row := range rows {
		state := row.State
		switch row.State {
		case store.PackageInstallPendingOpen, store.PackageInstallPendingAccepted:
			state = installStatePendingReview
		case store.PackageInstallPendingInstalled:
			continue // the install record itself is the install
		case store.PackageInstallPendingFailed:
			state = installStateReviewFailed
		}
		views = append(views, PackageInstallView{
			PackageInstall: store.PackageInstall{
				InstallID: "pending-" + row.SubjectDigest[:16] + fmt.Sprintf("-a%d", row.Attempt), OwnerID: row.OwnerID, ComputerID: row.ComputerID,
				ReleaseID: row.ReleaseID, Package: row.Package, Kind: installKindPackage, Action: packageActionInstall, RequestedAt: row.RequestedAt,
				Attempt: row.Attempt, Mode: store.PackageInstallModeRuntime,
			},
			State: state, Reason: row.Reason, LastError: row.LastError,
		})
	}
	return views
}

const (
	installStatePendingReview = "pending_review"
	installStateReviewFailed  = "install_failed"
)
