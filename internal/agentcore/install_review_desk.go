package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/installreview"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
	"github.com/yusefmosiah/go-choir/internal/pkgruntime"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Management's side of the install review
// (docs/design/desk-install-review-2026-11-08.md 2.2, 2.3): the verbs
// choir.InstallPacket and choir.InstallVerdict, the package operation path's
// pend and clock, and the completion guard line. The grant is sole-holder and
// privileged; the verbs check Holds(callingDesk), no code here names the desk.

// holdsInstallReviewGrant: the desk the registry says holds the install review.
func holdsInstallReviewGrant(desk string) bool {
	return deskmanifest.Holds(canonicalDeskName(desk), deskmanifest.GrantInstallReview)
}

const (
	// installReviewOperationGrace is how long the operation path waits after
	// the notice cap is spent and the last notice went unanswered.
	installReviewOperationGrace = 10 * time.Minute
	installPacketSourceNote     = "\n// [truncated]"
)

// installReviewJudgeDesk is the desk the registry says holds the install
// review, the name on the seat record.
func installReviewJudgeDesk() string {
	d, _ := deskmanifest.GrantHolder(deskmanifest.GrantInstallReview)
	return d.Name
}

// ---- finding an owed review -------------------------------------------------

// installReviewOwed is one review management owes (or owed): the owed record,
// its facts, and what holds the request.
type installReviewOwed struct {
	Record    types.CommitmentRecord
	Facts     types.InstallReviewOwedFacts
	Row       *store.PackageInstallPending
	Operation *selfdev.Operation
	// Open says the review still waits on a verdict.
	Open bool
}

func factsOfOwedRecord(rec types.CommitmentRecord) types.InstallReviewOwedFacts {
	f := types.InstallReviewOwedFacts{
		ReleaseID: rec.SubjectRefs["release_id"], SubjectDigest: rec.SubjectRefs["subject_digest"], Package: rec.SubjectRefs["package"],
		Path: rec.SubjectRefs["path"], OperationID: rec.SubjectRefs["operation_id"],
		PatchSHA256: rec.SubjectRefs["patch_sha256"], ManifestSHA256: rec.SubjectRefs["manifest_sha256"], Surface: rec.SubjectRefs["behaviour_surface"],
	}
	_, _ = fmt.Sscan(rec.SubjectRefs["attempt"], &f.Attempt)
	return f
}

// installReviewOwedOn finds the newest review owed on a release, through the
// owed records about it (the sender inbox listing of the shots-owed pattern,
// by subject ref). found is false when no review was ever owed on it.
func (rt *Runtime) installReviewOwedOn(ctx context.Context, ownerID, computerID, releaseID string) (installReviewOwed, bool, error) {
	records, err := rt.store.ListCommitmentRecordsBySubjectRef(ctx, ownerID, computerID, "release_id", releaseID, 200)
	if err != nil {
		return installReviewOwed{}, false, err
	}
	var owed []types.CommitmentRecord
	for _, rec := range records {
		if types.IsInstallReviewOwedRecord(rec) && rec.SubjectRefs["notice"] == "" {
			owed = append(owed, rec)
		}
	}
	if len(owed) == 0 {
		return installReviewOwed{}, false, nil
	}
	sort.Slice(owed, func(i, j int) bool {
		fi, fj := factsOfOwedRecord(owed[i]), factsOfOwedRecord(owed[j])
		if fi.Attempt != fj.Attempt {
			return fi.Attempt > fj.Attempt
		}
		return owed[i].RecordID > owed[j].RecordID
	})
	rec := owed[0]
	out := installReviewOwed{Record: rec, Facts: factsOfOwedRecord(rec)}
	switch out.Facts.Path {
	case types.InstallReviewPathInstall:
		row, err := rt.store.GetPackageInstallPending(ctx, ownerID, computerID, out.Facts.SubjectDigest, out.Facts.Attempt)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			return installReviewOwed{}, false, err
		}
		if err == nil {
			out.Row, out.Open = &row, row.Open()
		}
	case types.InstallReviewPathOperation:
		if rt.selfdevOperations == nil {
			break
		}
		op, err := rt.selfdevOperations.Get(ctx, computerID, out.Facts.OperationID)
		if err == nil {
			out.Operation, out.Open = &op, op.State == selfdev.StateAccepted
		}
	}
	return out, true, nil
}

// installReviewRequestOf is the release record and tree patch an owed review
// is of: the pending row's request, or the operation's persisted draft.
func (rt *Runtime) installReviewRequestOf(owed installReviewOwed) (json.RawMessage, []byte, error) {
	switch {
	case owed.Row != nil:
		return owed.Row.Record, owed.Row.TreePatch, nil
	case owed.Operation != nil:
		draft, found, err := rt.loadPackageDraft(*owed.Operation)
		if err != nil {
			return nil, nil, err
		}
		if !found || draft.ReleaseID != owed.Facts.ReleaseID {
			return nil, nil, errors.New("the operation's draft does not name this release")
		}
		return draft.Record, draft.TreePatch, nil
	}
	return nil, nil, errors.New("the review's request is unavailable")
}

// installReviewSubjectInfo is a review's subject rebuilt from its stored
// request: the module the platform built, the checks and the plan.
type installReviewSubjectInfo struct {
	Rec     pkgrelease.Record
	Module  pkgruntime.Module
	Subject installreview.Subject
	Checks  []installreview.Check
	Plan    installreview.Plan
}

// installReviewSubjectOf rebuilds the subject of an owed review from its
// stored request, and refuses when it is not the subject the owed record
// bound (FM2).
func (rt *Runtime) installReviewSubjectOf(ctx context.Context, ownerID, computerID string, owed installReviewOwed) (installReviewSubjectInfo, error) {
	raw, patch, err := rt.installReviewRequestOf(owed)
	if err != nil {
		return installReviewSubjectInfo{}, err
	}
	rec, err := pkgrelease.Decode(raw)
	if err != nil {
		return installReviewSubjectInfo{}, fmt.Errorf("the stored release record does not decode: %w", err)
	}
	if err := pkgrelease.VerifyID(rec, owed.Facts.ReleaseID); err != nil {
		return installReviewSubjectInfo{}, err
	}
	if rt.packageBuilder == nil {
		return installReviewSubjectInfo{}, errors.New("this computer has no package builder")
	}
	module, err := rt.packageBuilder.Build(ctx, rec, patch)
	if err != nil {
		return installReviewSubjectInfo{}, fmt.Errorf("the release does not build: %w", err)
	}
	prior, err := rt.packageInstalls(ctx, ownerID, computerID)
	if err != nil {
		return installReviewSubjectInfo{}, err
	}
	subject := installReviewSubject(owed.Facts.ReleaseID, rec, patch, module, currentPackageInstalls(prior)[rec.Package.Name])
	if subject.Digest() != owed.Facts.SubjectDigest {
		return installReviewSubjectInfo{}, errors.New("the stored request is not the subject the review was opened on")
	}
	subject, checks, plan := rt.installReviewPlanFor(ctx, ownerID, computerID, rec, subject, module)
	return installReviewSubjectInfo{Rec: rec, Module: module, Subject: subject, Checks: checks, Plan: plan}, nil
}

// ---- InstallPacket ----------------------------------------------------------

func installCheckLines(checks []installreview.Check) []string {
	out := make([]string, 0, len(checks))
	for _, c := range checks {
		state := "ok"
		if !c.OK {
			state = "FAILED"
		}
		line := c.Name + ": " + state
		if c.Detail != "" {
			line += " (" + c.Detail + ")"
		}
		out = append(out, line)
	}
	return out
}

// installPacketEgress serves choir.InstallPacket: the packet of the review
// management owes on a release. Refused for any other desk and for a release
// no review is owed on.
func (rt *Runtime) installPacketEgress(ctx context.Context, deskRole string, exec toolregistry.ExecutionContext, raw json.RawMessage) (json.RawMessage, error) {
	if !holdsInstallReviewGrant(deskRole) {
		return nil, fmt.Errorf("install review: the install review verbs are management's, not %q", deskRole)
	}
	var req yaegikernel.InstallPacketPayload
	if err := json.Unmarshal(raw, &req); err != nil || strings.TrimSpace(req.ReleaseID) == "" {
		return nil, errors.New("install review: InstallPacket needs a release id")
	}
	ownerID := strings.TrimSpace(exec.OwnerID)
	if ownerID == "" {
		ownerID = strings.TrimSpace(rt.selfdevRouteOwnerID)
	}
	computerID := rt.shotExecComputer(exec)
	releaseID := strings.ToLower(strings.TrimSpace(req.ReleaseID))
	owed, found, err := rt.installReviewOwedOn(ctx, ownerID, computerID, releaseID)
	if err != nil {
		return nil, err
	}
	if !found || !owed.Open {
		return nil, fmt.Errorf("install review: no review is owed on release %s", releaseID)
	}
	info, err := rt.installReviewSubjectOf(ctx, ownerID, computerID, owed)
	if err != nil {
		return nil, fmt.Errorf("install review: %w", err)
	}
	source, truncated := string(info.Module.Bytes), false
	if len(source) > installReviewSourceBytes {
		source, truncated = source[:installReviewSourceBytes]+installPacketSourceNote, true
	}
	return json.Marshal(yaegikernel.InstallPacketResult{
		ReleaseID: releaseID, Package: info.Subject.Package, Publisher: info.Subject.Publisher, Path: owed.Facts.Path,
		PatchSHA256: info.Subject.PatchSHA256, ManifestSHA: info.Subject.ManifestSHA256, Surface: info.Subject.BehaviourSurface,
		Risk: string(info.Plan.Risk), Strong: info.Plan.Strong, Added: info.Plan.Added, Flow: installreview.Flow(info.Subject.Manifest),
		Checks: installCheckLines(info.Checks), ManifestJSON: string(info.Module.Manifest), Source: source, Truncated: truncated,
	})
}

// ---- InstallVerdict ---------------------------------------------------------

// commitInstallVerdictIntent reduces management's staged InstallVerdict. A
// refusal is stamped where the owner or management sees it: the pending row's
// last_error (owner path) or the operation (RecordIntentError).
func (r *rlmCallReduction) commitInstallVerdictIntent(ctx context.Context, in yaegikernel.StagedIntent) (map[string]any, error) {
	out, err := r.reduceInstallVerdict(ctx, in)
	if err != nil {
		if rt := r.rt(); rt != nil && rt.store != nil && strings.TrimSpace(in.TargetRef) != "" {
			writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
			defer cancel()
			reason := err.Error()
			if len(reason) > 500 {
				reason = reason[:500]
			}
			release := strings.ToLower(strings.TrimSpace(in.TargetRef))
			if owed, found, ferr := rt.installReviewOwedOn(writeCtx, r.scope.OwnerID, r.scope.ComputerID, release); ferr == nil && found {
				switch {
				case owed.Row != nil:
					_ = rt.store.SetPackageInstallPendingError(writeCtx, owed.Row.OwnerID, owed.Row.ComputerID, owed.Row.SubjectDigest, owed.Row.Attempt, reason)
				case owed.Operation != nil && rt.selfdevOperations != nil:
					if rerr := rt.selfdevOperations.RecordIntentError(writeCtx, r.scope.ComputerID, owed.Operation.OperationID, reason); rerr != nil {
						log.Printf("agentcore: record install verdict intent error on %s failed (run %s): %v", owed.Operation.OperationID, r.scope.RunID, rerr)
					}
				}
			}
		}
	}
	return out, err
}

func installFindingLines(findings []yaegikernel.InstallFinding) (lines []string, blocking bool) {
	for _, f := range findings {
		line := strings.TrimSpace(f.Concern)
		if e := strings.TrimSpace(f.Evidence); e != "" {
			line += " (" + e + ")"
		}
		if f.Blocking {
			line += " [blocking]"
			blocking = true
		}
		lines = append(lines, line)
	}
	return lines, blocking
}

// sealedInstallVerdictFor returns the sealed verdict (and management's seat
// record under it) that decides a release's subject, if any.
func (rt *Runtime) sealedInstallVerdictFor(ctx context.Context, ownerID, computerID string, facts types.InstallReviewOwedFacts) (verdict types.CommitmentRecord, seat *types.CommitmentRecord, found bool, err error) {
	if rt.installReviewSigner == nil {
		return verdict, nil, false, nil
	}
	_, pub, err := rt.installReviewSigner.PublicKey(ctx)
	if err != nil {
		return verdict, nil, false, err
	}
	records, err := rt.store.ListCommitmentRecords(ctx, ownerID, computerID, installreview.AddresseePrefix+facts.ReleaseID, installReviewRecordWindow)
	if err != nil {
		return verdict, nil, false, err
	}
	d := installreview.Gate(records, installreview.Subject{ReleaseID: facts.ReleaseID, PatchSHA256: facts.PatchSHA256, ManifestSHA256: facts.ManifestSHA256, BehaviourSurface: facts.Surface}, computerID, pub)
	if d.VerdictID == "" {
		return verdict, nil, false, nil
	}
	byID := map[string]types.CommitmentRecord{}
	for _, rec := range records {
		byID[rec.RecordID] = rec
	}
	verdict = byID[d.VerdictID]
	for _, child := range verdict.ChildIDs {
		if c, ok := byID[child]; ok && c.SubjectRefs[installreview.RefRole] == installreview.RoleSeat {
			cc := c
			seat = &cc
		}
	}
	return verdict, seat, true, nil
}

func (r *rlmCallReduction) reduceInstallVerdict(ctx context.Context, in yaegikernel.StagedIntent) (map[string]any, error) {
	rt := r.rt()
	if r.rec == nil || rt == nil || rt.store == nil {
		return nil, errors.New("reduce: install verdict without authority")
	}
	if !holdsInstallReviewGrant(r.rec.AgentProfile) {
		return nil, fmt.Errorf("reduce: install verdict %s: the install verdict is management's", in.LocalID)
	}
	if in.Decision != "accept" && in.Decision != "reject" {
		return nil, fmt.Errorf("reduce: install verdict %s decision %q invalid", in.LocalID, in.Decision)
	}
	if !(in.PWithinCeiling >= 0 && in.PWithinCeiling <= 1) {
		return nil, fmt.Errorf("reduce: install verdict %s: pWithinCeiling %v is not a probability", in.LocalID, in.PWithinCeiling)
	}
	ownerID, computerID := r.scope.OwnerID, r.scope.ComputerID
	release := strings.ToLower(strings.TrimSpace(in.TargetRef))
	reason := yaegikernel.NormalizeVerifyReason(in.Reason)
	requestedAccept := in.Decision == "accept"
	if !requestedAccept && reason == "" {
		return nil, fmt.Errorf("reduce: install verdict %s: a reject needs its reason", in.LocalID)
	}
	lines, blocking := installFindingLines(in.InstallFindings)
	owed, found, err := rt.installReviewOwedOn(ctx, ownerID, computerID, release)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("reduce: install verdict %s: no install review is owed on release %s", in.LocalID, release)
	}
	if !owed.Open {
		// A replay of the verdict already sealed is success; a different one
		// is refused, and a closed review (timed out, interrupted, a failed
		// operation) takes no verdict.
		if verdict, seat, sealed, serr := rt.sealedInstallVerdictFor(ctx, ownerID, computerID, owed.Facts); serr != nil {
			return nil, serr
		} else if sealed && seat != nil {
			want := installreview.SeatHypothesis(installReviewJudgeDesk(), installreview.SeatVerdict{Accept: requestedAccept, Findings: lines, Reason: reason})
			if seat.Prediction.Hypothesis == want {
				return map[string]any{"release_id": release, "verdict": verdict.SubjectRefs[installreview.RefVerdict], "verdict_id": verdict.RecordID, "replay": true}, nil
			}
			return nil, fmt.Errorf("reduce: install verdict %s: release %s already has a different sealed verdict", in.LocalID, release)
		}
		return nil, fmt.Errorf("reduce: install verdict %s: the review of release %s is closed", in.LocalID, release)
	}
	info, err := rt.installReviewSubjectOf(ctx, ownerID, computerID, owed)
	if err != nil {
		return nil, fmt.Errorf("reduce: install verdict %s: %w", in.LocalID, err)
	}
	accept := requestedAccept
	switch {
	case requestedAccept && !installreview.Passed(info.Checks):
		accept, reason = false, "a mechanical check failed: "+strings.Join(failedCheckLines(info.Checks), "; ")
	case requestedAccept && blocking:
		accept, reason = false, "a blocking finding: "+strings.Join(lines, "; ")
	}
	now := rt.reviewNow().UTC()
	seat := installreview.SeatRecord(info.Subject, info.Plan, installReviewJudgeDesk(), recordModelID(ctx, r.rec), installreview.SeatVerdict{
		Reason: reason, Accept: accept, Blocking: blocking, Findings: lines,
		Predictions: []installreview.Prediction{{Question: installreview.StandardQuestions[1], PYes: in.PWithinCeiling}},
	}, computerID, now)
	// Seal and close under the install mutex: a request's owed commit re-reads
	// the gate under it, so a verdict and a pend never miss each other.
	rt.packageInstallMu.Lock()
	defer rt.packageInstallMu.Unlock()
	if owed.Row != nil {
		row, err := rt.store.GetPackageInstallPending(ctx, owed.Row.OwnerID, owed.Row.ComputerID, owed.Row.SubjectDigest, owed.Row.Attempt)
		if err != nil || !row.Open() {
			return nil, fmt.Errorf("reduce: install verdict %s: the review of release %s closed before the verdict landed", in.LocalID, release)
		}
	}
	if refusal := rt.sealInstallVerdict(ctx, ownerID, computerID, info.Subject, info.Plan, info.Checks, &seat, accept, reason, now); refusal != nil {
		return nil, fmt.Errorf("reduce: install verdict %s: %s", in.LocalID, refusal.Error())
	}
	verdictWord := installreview.VerdictReject
	if accept {
		verdictWord = installreview.VerdictAccept
	}
	out := map[string]any{"release_id": release, "verdict": verdictWord}
	switch {
	case owed.Row != nil && accept:
		ok, err := rt.store.ClosePackageInstallPending(ctx, owed.Row.OwnerID, owed.Row.ComputerID, owed.Row.SubjectDigest, owed.Row.Attempt, store.PackageInstallPendingOpen, store.PackageInstallPendingAccepted, "")
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("reduce: install verdict %s: the review of release %s closed as the verdict landed; the verdict is sealed, so installing the release again passes the gate", in.LocalID, release)
		}
		row := *owed.Row
		row.State = store.PackageInstallPendingAccepted
		rt.launchPendingInstall(row)
		out["installing"] = true
	case owed.Row != nil:
		why := reason
		if why == "" {
			why = "management rejected the release"
		}
		if _, err := rt.store.ClosePackageInstallPending(ctx, owed.Row.OwnerID, owed.Row.ComputerID, owed.Row.SubjectDigest, owed.Row.Attempt, store.PackageInstallPendingOpen, store.PackageInstallPendingRejected, why); err != nil {
			return nil, err
		}
	case owed.Operation != nil:
		rt.TriggerSelfDevelopmentReconcile()
	}
	return out, nil
}

func failedCheckLines(checks []installreview.Check) []string {
	var out []string
	for _, c := range checks {
		if !c.OK {
			out = append(out, c.Name+" "+c.Detail)
		}
	}
	return out
}

// ---- the package operation path --------------------------------------------

// openOperationReview is the operation path's pend and clock (design 2.2, 7.1):
// the owed record is committed once, and each pass that re-enters the gate
// keeps the clock: a notice when management is idle and the record or last
// notice is old enough, a close at 45 minutes (or with the notice cap spent
// and unanswered for a grace). It returns a refusal the caller turns into
// errInstallReviewPending or a failure.
func (rt *Runtime) openOperationReview(ctx context.Context, ownerID, computerID string, operation selfdev.Operation, facts types.InstallReviewOwedFacts, plan installreview.Plan) *installRefusal {
	facts.Path, facts.OperationID, facts.Attempt = types.InstallReviewPathOperation, operation.OperationID, 1
	management := persistentManagementAgentID(ownerID)
	recordID := types.InstallReviewOwedRecordID(facts.ReleaseID, facts.SubjectDigest, 1)
	rec, err := rt.store.GetCommitmentRecord(ctx, ownerID, computerID, recordID)
	if err != nil {
		return refuseInstall(http.StatusInternalServerError, "install_store", "%v", err)
	}
	if rec == nil {
		owed := types.NewInstallReviewOwedRecord(management, computerID, facts, installReviewOwedLine(facts.ReleaseID, facts.Package)+" "+planSentence(plan))
		if err := rt.commitInstallReviewRecord(ctx, ownerID, computerID, owed); err != nil {
			return refuseInstall(http.StatusInternalServerError, "install_store", "%v", err)
		}
		return installReviewOwed202(recordID, installReviewOwedDetail)
	}
	now := rt.reviewNow()
	committed := now
	if at, perr := time.Parse(time.RFC3339Nano, rec.Provenance.CommittedAt); perr == nil {
		committed = at
	}
	n, last := rt.installReviewNotices(ctx, ownerID, computerID, facts.ReleaseID, facts.SubjectDigest, 1)
	since := committed
	if !last.IsZero() {
		since = last
	}
	if now.Sub(committed) >= types.InstallReviewOwedDeadline || (n >= types.InstallReviewOwedMaxNotices && now.Sub(since) >= installReviewOperationGrace) {
		return refuseInstall(http.StatusRequestTimeout, reasonInstallReviewTimedOut, "%s", installReviewNotJudged)
	}
	if n < types.InstallReviewOwedMaxNotices && now.Sub(since) >= types.InstallReviewOwedNoticeAfter {
		active, found, err := rt.activeRunByAgent(ctx, ownerID, management)
		if err != nil {
			return refuseInstall(http.StatusInternalServerError, "install_store", "%v", err)
		}
		if !found || rt.runHasNoLiveLoop(active, rt.assignmentWaitNow()) {
			facts.Notice = n + 1
			body := fmt.Sprintf("%s Notice %d of %d.", installReviewOwedLine(facts.ReleaseID, facts.Package), n+1, types.InstallReviewOwedMaxNotices)
			if err := rt.commitInstallReviewRecord(ctx, ownerID, computerID, types.NewInstallReviewOwedNotice(management, computerID, facts, body)); err != nil {
				return refuseInstall(http.StatusInternalServerError, "install_store", "%v", err)
			}
		}
	}
	return installReviewOwed202(recordID, installReviewOwedDetail)
}

// ---- management's pack and guard -------------------------------------------

// installReviewsOwedBy lists the reviews management owes on this computer:
// the open pending rows and the accepted package operations with an owed
// record. Open rows only; the pack is context, never a gate.
func (rt *Runtime) installReviewsOwedBy(ctx context.Context, ownerID, computerID string) ([]installReviewOwed, error) {
	if rt == nil || rt.store == nil || strings.TrimSpace(computerID) == "" {
		return nil, nil
	}
	seen := map[string]bool{}
	var out []installReviewOwed
	rows, err := rt.store.ListPackageInstallPending(ctx, ownerID, computerID, true)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if seen[row.ReleaseID] {
			continue
		}
		if owed, found, err := rt.installReviewOwedOn(ctx, ownerID, computerID, row.ReleaseID); err == nil && found && owed.Open {
			seen[row.ReleaseID] = true
			out = append(out, owed)
		}
	}
	if rt.selfdevOperations != nil {
		accepted, err := rt.selfdevOperations.ListByStates(ctx, computerID, selfdev.StateAccepted)
		if err != nil {
			return out, err
		}
		for _, op := range accepted {
			if !op.IsPackage() {
				continue
			}
			records, err := rt.store.ListCommitmentRecordsBySubjectRef(ctx, ownerID, computerID, "operation_id", op.OperationID, 50)
			if err != nil {
				return out, err
			}
			for _, rec := range records {
				if !types.IsInstallReviewOwedRecord(rec) || rec.SubjectRefs["notice"] != "" || seen[rec.SubjectRefs["release_id"]] {
					continue
				}
				if owed, found, err := rt.installReviewOwedOn(ctx, ownerID, computerID, rec.SubjectRefs["release_id"]); err == nil && found && owed.Open {
					seen[rec.SubjectRefs["release_id"]] = true
					out = append(out, owed)
				}
			}
		}
	}
	return out, nil
}

// installReviewOwedLines are the harness lines of the reviews management owes,
// for its completion guard.
func (rt *Runtime) installReviewOwedLines(ctx context.Context, ownerID, computerID string) []string {
	owed, err := rt.installReviewsOwedBy(ctx, ownerID, computerID)
	if err != nil {
		log.Printf("runtime: install reviews owed: %v", err)
		return nil
	}
	var lines []string
	for _, o := range owed {
		lines = append(lines, installReviewOwedLine(o.Facts.ReleaseID, o.Facts.Package))
	}
	return lines
}

// installReviewSubject builds the subject of a release being installed.
func installReviewSubject(releaseID string, rec pkgrelease.Record, treePatch []byte, module pkgruntime.Module, current *store.PackageInstall) installreview.Subject {
	subject := installreview.Subject{
		ReleaseID: releaseID, PatchSHA256: computerevent.DigestBytes(treePatch), ManifestSHA256: rec.Manifest.SHA256,
		BehaviourSurface: rec.Manifest.BehaviourSurface, Package: rec.Package.Name, Publisher: rec.Track,
		Manifest: lenientManifest(module.Manifest),
	}
	if current != nil && current.Package == rec.Package.Name {
		subject.HasPrior, subject.PriorCeiling = true, installreview.Ceiling(lenientManifest(current.Manifest))
	}
	return subject
}
