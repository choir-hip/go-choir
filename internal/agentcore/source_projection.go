package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/sourceproj"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// U1 (docs/design/computer-update-architecture-rnd-2026-10-24.md, section 4.6
// and its "U1 build notes"): the guest source repository is a projection of
// the effective release. This file gathers what the projection needs from the
// tape and the updater, runs sourceproj, and keeps the facts. It runs on every
// computer (capsule overlay S4,
// docs/design/capsule-overlay-computer-state-2026-11-03.md section 5): no
// switch, and what a reset would lose is saved and put on the tape first.

// sourceProjectionFactsFile is the guest-local log of projection facts. They
// are observation only: reconcile recomputes from the tape every time.
const sourceProjectionFactsFile = "source-projection/facts.jsonl"

// bootedBaseCommit is the booted guest image's build commit: the commit the
// host builder requires a patch to be based on. Empty when the manifest is
// unreadable or names none (a dev build).
func (rt *Runtime) bootedBaseCommit() string {
	path := rt.guestImageManifestPath()
	if path == "" {
		return ""
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if key, value, ok := strings.Cut(strings.TrimSpace(line), "="); ok && key == "build_commit" {
			value = strings.ToLower(strings.TrimSpace(value))
			if looksLikeGitCommit(value) {
				return value
			}
			return ""
		}
	}
	return ""
}

// bootedBaseForJoin is the booted base for the apply-time base join, which
// must tell "nothing to join against" from "cannot be proven":
//   - ("", nil): no image manifest is wired, or it names a dev build
//     (build_commit=local, or "<commit>-dirty"); the join is not claimed;
//   - (commit, nil): the booted build commit;
//   - ("", err): a manifest is wired but cannot be read, or carries no valid
//     build commit; the join cannot be proven and the caller fails closed.
func (rt *Runtime) bootedBaseForJoin() (string, error) {
	path := rt.guestImageManifestPath()
	if path == "" {
		return "", nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("the booted base image manifest is unreadable: %w", err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok || key != "build_commit" {
			continue
		}
		value = strings.ToLower(strings.TrimSpace(value))
		switch {
		case looksLikeGitCommit(value):
			return value, nil
		case value == "local" || strings.HasSuffix(value, "-dirty"):
			return "", nil
		}
		return "", fmt.Errorf("the booted base image manifest carries no valid build_commit (%q)", value)
	}
	return "", errors.New("the booted base image manifest carries no build_commit")
}

// sourceProjectionInput reads the effective release's operation: the one
// whose release is `current` (the authority the effective identity already
// uses), its bundle's patch and base, and whether an apply is in flight.
func (rt *Runtime) sourceProjectionInput(ctx context.Context) sourceproj.Input {
	in := sourceproj.Input{Repo: rt.releaseRepo(), BootedBase: rt.bootedBaseCommit(), OnSave: rt.recordSourceProjectionSaved}
	root := strings.TrimSpace(rt.selfdevUpdaterRoot)
	if root == "" {
		return in // no updater: nothing was ever applied here
	}
	// Only a missing `current` link means "no release". Every other failure to
	// read what is effective (a nil operation store, a dangling link, a digest
	// mismatch, a manifest this build cannot decode) is unknown, and unknown
	// must not read as an empty stack: reconcile would reset the repository to
	// the base and drop the applied source (fail closed, before anything moves).
	if _, err := os.Lstat(filepath.Join(root, "current")); errors.Is(err, fs.ErrNotExist) {
		return in // no current release: a base computer
	} else if err != nil {
		in.TopUnavailable = "the current release link is unreadable: " + err.Error()
		return in
	}
	if rt.selfdevOperations == nil {
		in.TopUnavailable = "the self-development operations store is unavailable, so the effective release cannot be read"
		return in
	}
	manifest, err := readCurrentManifestIdentity(root)
	if err != nil || manifest.ContentDigest == "" {
		reason := "the current release manifest is unreadable"
		if err != nil {
			reason += ": " + err.Error()
		}
		in.TopUnavailable = reason
		return in
	}
	operation, ok := rt.appliedOperationForRelease(ctx, manifest.ContentDigest)
	if !ok {
		if strings.HasPrefix(manifest.Marker, "selfdev-") {
			// The apply is in flight: the updater probes /health before the
			// operation records applied. Not "no operation" (P2).
			in.Pending = true
			in.PendingDetail = rt.pendingApplyDetail(ctx)
		}
		return in
	}
	if strings.HasPrefix(operation.OperationID, "genesis-") {
		return in // the genesis baseline carries no source change
	}
	base, patch, err := rt.boundSourcePatch(operation)
	switch {
	case errors.Is(err, errNoSourcePatch):
		return in // the operation changed no source
	case err != nil:
		in.Top = &sourceproj.Op{OperationID: operation.OperationID}
		in.TopUnavailable = err.Error()
		return in
	}
	in.Top = &sourceproj.Op{
		OperationID: operation.OperationID, BundleDigest: operation.BundleDigest, ReleaseDigest: operation.ReleaseDigest,
		DecisionEvent: operation.DecisionEvent, AppliedEventDigest: rt.appliedEventDigest(ctx, operation),
		CreatedAt: parseOperationTime(operation.CreatedAt), Base: base, Patch: patch,
	}
	return in
}

// pendingApplyDetail names the operation that holds the apply open and the
// owner act that clears it, for the cast refusal.
func (rt *Runtime) pendingApplyDetail(ctx context.Context) string {
	const clear = "it settles when the operation records applied or failed; if it stays there (apply attempts spent, or degraded), the owner clears it with a self-development rollback to the prior applied head"
	if rt.selfdevOperations == nil {
		return "the effective release is mid-apply and its operation is not recorded; " + clear
	}
	operation, found, err := rt.selfdevOperations.OpenOperation(ctx, rt.cfg.ComputerID)
	if err != nil || !found {
		return "the effective release is mid-apply and no open operation is recorded for it; " + clear
	}
	return fmt.Sprintf("operation %s is %s; %s", operation.OperationID, operation.State, clear)
}

// appliedEventDigest is the digest of the event that applied the operation,
// found by the idempotency key the materializer wrote it under. The ref the
// projection leaves is named by it; an unrecorded event gives no ref.
func (rt *Runtime) appliedEventDigest(ctx context.Context, operation selfdev.Operation) string {
	if rt.store == nil || operation.DecisionEvent == "" {
		return ""
	}
	event, found, err := rt.store.EventByIdempotency(ctx, operation.ComputerID, "selfdev-"+string(computerevent.EventMaterializationApplied)+"-"+operation.DecisionEvent)
	if err != nil || !found {
		return ""
	}
	digest, err := event.Digest()
	if err != nil {
		return ""
	}
	return digest
}

func parseOperationTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

// ReconcileSourceProjection makes the guest source repository the projection
// of the effective release, records what changed, and returns what it found.
// reason names the trigger (boot, apply, cast). Safe to call from several
// triggers: sourceproj serializes, and each call recomputes from the tape.
func (rt *Runtime) ReconcileSourceProjection(ctx context.Context, reason string) (sourceproj.Result, error) {
	if rt == nil {
		return sourceproj.Result{State: sourceproj.StateRepoUnavailable, Reason: "no runtime"}, nil
	}
	// Gather and reconcile under one lock: an input read before a newer
	// reconcile must never be applied after it (a boot pass that read an empty
	// stack must not reset the repository once an apply has projected).
	rt.sourceProjectionRunMu.Lock()
	result, err := sourceproj.Reconcile(ctx, rt.sourceProjectionInput(ctx))
	rt.sourceProjectionRunMu.Unlock()
	if err != nil {
		return result, err
	}
	result.Reason = rt.withBaseFetchReason(result)
	rt.sourceProjectionMu.Lock()
	previous := rt.sourceProjectionLast
	copied := result
	rt.sourceProjectionLast = &copied
	rt.sourceProjectionMu.Unlock()
	if previous == nil || previous.State != result.State || result.Changed {
		log.Printf("source projection (%s): %s base=%s head=%s booted=%s op=%s changed=%t repaired=%t diverged=%t %s",
			reason, result.State, shortDigest(result.Base), shortDigest(result.Head), shortDigest(result.BootedBase), result.OperationID, result.Changed, result.Repaired, result.Diverged, result.Reason)
	}
	if result.Changed || len(result.LocksCleared) > 0 {
		rt.appendSourceProjectionFact(reason, result)
	}
	// U4: a stack on a moved base arms the rebase notice timer. Arming only
	// sets an in-memory timer (due no earlier than bootedAt + the stay-up
	// interval), so a boot reconcile mints nothing. The timer's own reconcile
	// never arms.
	if reason != rebaseReconcileReason {
		rt.armRebaseNotice(ctx, result)
	}
	return result, nil
}

// reconcileSourceProjectionLogged is the trigger form: a failure is logged and
// never blocks boot or an apply (the cast gate reconciles again and refuses).
func (rt *Runtime) reconcileSourceProjectionLogged(ctx context.Context, reason string) {
	if rt == nil {
		return
	}
	if _, err := rt.ReconcileSourceProjection(ctx, reason); err != nil {
		log.Printf("source projection (%s): %v", reason, err)
	}
}

// sourceProjectionCastGate refuses to cast an engineering capsule on a source
// that is not the effective release's: an apply in flight, a stack on a base
// the computer no longer booted, or a projection that cannot be rebuilt. The
// reason is stable (its first word is the code) and names the bases.
func (rt *Runtime) sourceProjectionCastGate(ctx context.Context) error {
	_, _, err := rt.sourceProjectionCastGateFor(ctx, rebaseSpec{}, castLedgerRef{})
	return err
}

// rebaseSpec is what management's engineering Cast spec says about a rebase
// (U4): the notice it answers, the operation to rebase and the base to rebase
// onto. The zero value is an ordinary Cast.
type rebaseSpec struct {
	Notice   string
	RebaseOf string
	Base     string
}

func (s rebaseSpec) set() bool { return strings.TrimSpace(s.Notice) != "" }

// castLedgerRef names the Cast a gate answers, for the ledger warning an
// admitted Cast on a rematerialized stack carries. The zero value records none.
type castLedgerRef struct {
	OwnerID, ComputerID, CastRecordID, ManagementAgentID, TrajectoryID string
}

// rebaseNeedsRoot is the text a stack_base_moved refusal gains when no root
// Texture RLM is live to Cast the rebase (capsule overlay S4).
const rebaseNeedsRoot = "rebase_needs_root: no live root desk to Cast the rebase"

// sourceProjectionCastGateFor is the gate with U4's exception
// (docs/design/u4-stack-rebase-build-2026-10-26.md, failure mode 6). An
// ordinary Cast gets the gate: refused at stack_base_moved (with
// rebase_needs_root when no root desk is live), admitted at
// stack_rematerialized with a ledger warning. A Cast whose spec names a rebase
// notice is admitted only when every live fact agrees:
//   - the stack sits on a moved base (stack_base_moved), its operation is the
//     spec's rebase_of, the booted base is the spec's base, and the booted
//     base's objects are in the guest repository;
//   - no assignment has claimed the pair.
//
// It returns the booted base commit the capsule must start from (empty for an
// ordinary Cast), and, at stack_rematerialized, the ledger warning the caller
// records once the Cast's assignment has opened (never before: a Cast refused
// later leaves no warning behind). The store's records admission checks the
// tape facts (the notice, the root's Cast, the latest attempt) inside the
// open's transaction.
func (rt *Runtime) sourceProjectionCastGateFor(ctx context.Context, spec rebaseSpec, cast castLedgerRef) (rebaseBase string, warning *types.CommitmentRecord, err error) {
	if !spec.set() {
		rt.waitSourceBaseFetch(ctx)
		result, err := rt.ReconcileSourceProjection(ctx, "cast")
		if err != nil {
			return "", nil, fmt.Errorf("source_projection_failed: the capsule source could not be reconciled with the applied stack: %w", err)
		}
		if refusal := result.Refusal(); refusal != "" {
			if result.State == sourceproj.StateBaseMoved && !rt.rebaseRootLive(ctx) {
				refusal += "; " + rebaseNeedsRoot
			}
			return "", nil, errors.New(refusal)
		}
		if result.State == sourceproj.StateRematerialized && strings.TrimSpace(cast.CastRecordID) != "" {
			rec := rematerializedCastWarning(cast, result, time.Now())
			return "", &rec, nil
		}
		return "", nil, nil
	}
	ownerID, computerID := rt.rebaseScope()
	if strings.TrimSpace(spec.RebaseOf) == "" || strings.TrimSpace(spec.Base) == "" {
		return "", nil, errors.New("rebase_spec_incomplete: a rebase Cast's spec names rebase_notice, rebase_of (the operation) and base (the booted base), all three")
	}
	rt.waitSourceBaseFetch(ctx)
	result, err := rt.ReconcileSourceProjection(ctx, "cast")
	if err != nil {
		return "", nil, fmt.Errorf("source_projection_failed: the capsule source could not be reconciled with the applied stack: %w", err)
	}
	if result.State != sourceproj.StateBaseMoved {
		if refusal := result.Refusal(); refusal != "" {
			return "", nil, errors.New(refusal)
		}
		return "", nil, fmt.Errorf("rebase_not_needed: the applied stack is %s, not on a moved base; there is nothing to rebase", result.State)
	}
	if strings.TrimSpace(spec.RebaseOf) != result.OperationID {
		return "", nil, fmt.Errorf("rebase_of_mismatch: the spec rebases %q, the applied stack's operation is %q", spec.RebaseOf, result.OperationID)
	}
	if strings.TrimSpace(spec.Base) != result.BootedBase {
		return "", nil, fmt.Errorf("rebase_base_mismatch: the spec rebases onto %q, this computer booted %q", spec.Base, result.BootedBase)
	}
	if !sourceproj.HasCommit(ctx, rt.releaseRepo(), result.BootedBase) {
		return "", nil, fmt.Errorf("rebase_base_missing: the booted base %s is not in the guest source repository yet; the platform fetches it and the next notice follows", result.BootedBase)
	}
	noticeAttempt := 1
	if _, _, n, ok := types.ParseStackRebaseNoticeID(spec.Notice); ok {
		noticeAttempt = n
	}
	if claim, err := rt.store.GetRebaseNoticeClaim(ctx, ownerID, computerID, types.StackRebaseClaimKey(types.StackRebasePairKey(result.OperationID, result.BootedBase), noticeAttempt)); err == nil {
		return "", nil, fmt.Errorf("rebase_pair_claimed: assignment %s already serves the rebase of %s onto %s", claim.AssignmentID, result.OperationID, result.BootedBase)
	} else if !errors.Is(err, store.ErrNotFound) {
		return "", nil, fmt.Errorf("source_projection_failed: read the rebase claim: %w", err)
	}
	return result.BootedBase, nil, nil
}

// rebaseRootLive reports whether a root Texture RLM is live to Cast a rebase.
// A read failure counts as not live: the refusal then says so, which is the
// truth the owner can act on.
func (rt *Runtime) rebaseRootLive(ctx context.Context) bool {
	if rt == nil || rt.store == nil {
		return false
	}
	ownerID, computerID := rt.rebaseScope()
	if ownerID == "" || computerID == "" {
		return false
	}
	_, _, live, err := rt.rebaseRoot(ctx, ownerID, computerID)
	return err == nil && live
}

// rematerializedCastObserverAgentID commits the Cast warning: a
// system-committed observation, like the admission hold record.
const rematerializedCastObserverAgentID = "source-projection"

// rematerializedCastWarningID is the warning's deterministic id: one per
// Cast and grafted stack, so a replayed Cast records nothing new.
func rematerializedCastWarningID(castRecordID, operationID, bootedBase string) string {
	return fmt.Sprintf("source-projection:stack_rematerialized:%s:%s:%s", operationID, bootedBase, castRecordID)
}

// rematerializedCastWarning is the ledger record an admitted Cast on a
// rematerialized stack carries (design section 5): the served build is still
// the stack on its old base; the capsule builds on the graft; approving its
// candidate also approves the graft.
func rematerializedCastWarning(cast castLedgerRef, result sourceproj.Result, now time.Time) types.CommitmentRecord {
	stamp := now.UTC().Format(time.RFC3339Nano)
	return types.CommitmentRecord{
		SchemaID:    types.CommitmentRecordSchemaV1,
		RecordID:    rematerializedCastWarningID(cast.CastRecordID, result.OperationID, result.BootedBase),
		Discrepancy: types.DiscrepancyUnresolved,
		Prediction: types.CommitmentPrediction{
			Hypothesis: fmt.Sprintf("warning: Cast %s builds on the applied change %s grafted onto the booted base %s (stack_rematerialized, commit %s). The served build is still that change on %s. The booted base changed no path the change touches, so the graft is byte-identical at those paths; approving this Cast's candidate also approves the change on %s.",
				cast.CastRecordID, result.OperationID, result.BootedBase, result.Head, result.ServedBase, result.BootedBase),
			CommittedAt: stamp,
		},
		Observation: types.CommitmentObservation{
			Excerpt:    fmt.Sprintf("source projection stack_rematerialized: operation %s, served base %s, booted base %s, capsule source %s", result.OperationID, result.ServedBase, result.BootedBase, result.Head),
			SourceRef:  result.OperationID,
			ObservedAt: stamp,
		},
		Provenance: types.CommitmentProvenance{
			AgentID:     rematerializedCastObserverAgentID,
			ContextRef:  cast.TrajectoryID,
			CommittedAt: stamp,
		},
		Addressee:    strings.TrimSpace(cast.ManagementAgentID),
		ParentID:     cast.CastRecordID,
		EvidenceRefs: []string{result.OperationID, cast.CastRecordID},
	}
}

// recordRematerializedCastWarning writes the warning once the Cast's
// assignment has opened. It is observational: a ledger failure is logged and
// the Cast goes on (the state is on /health). The id is the Cast's, so a
// replayed open records nothing new.
func (rt *Runtime) recordRematerializedCastWarning(ctx context.Context, ownerID, computerID string, rec *types.CommitmentRecord) {
	if rt == nil || rt.store == nil || rec == nil {
		return
	}
	if _, err := rt.store.AppendCommitmentRecord(ctx, ownerID, computerID, *rec); err != nil {
		log.Printf("source projection: the stack_rematerialized warning %s: %v", rec.RecordID, err)
	}
}

type sourceProjectionFact struct {
	At          string `json:"at"`
	Kind        string `json:"kind"`
	Trigger     string `json:"trigger"`
	State       string `json:"state"`
	Reason      string `json:"reason,omitempty"`
	OperationID string `json:"operation_id,omitempty"`
	Head        string `json:"head"`
	From        string `json:"from,omitempty"`
	Base        string `json:"base,omitempty"`
	BootedBase  string `json:"booted_base,omitempty"`
	// LocksCleared names the stale git locks reconcile removed.
	LocksCleared []string `json:"locks_cleared,omitempty"`
	// RebaseOf and ReceiptSHA256 are set on a source_projection_rebased fact
	// (U4): the applied change a candidate was verified to contain, and the
	// lineage receipt's digest.
	RebaseOf      string `json:"rebase_of,omitempty"`
	ReceiptSHA256 string `json:"receipt_sha256,omitempty"`
	// Saved is set on a source.projection_saved fact (S4).
	Saved []sourceproj.Saved `json:"saved,omitempty"`
}

func (rt *Runtime) appendSourceProjectionFact(trigger string, result sourceproj.Result) {
	root := strings.TrimSpace(rt.selfdevUpdaterRoot)
	if root == "" {
		return
	}
	kind := sourceproj.FactBuilt
	switch {
	case result.Repaired:
		kind = sourceproj.FactRepaired
	case result.State == sourceproj.StateUpstreamed && result.Changed:
		kind = sourceproj.FactUpstreamed
	case !result.Changed && len(result.LocksCleared) > 0:
		kind = sourceproj.FactLockCleared
	}
	rt.appendSourceProjectionFactRecord(sourceProjectionFact{
		At: time.Now().UTC().Format(time.RFC3339Nano), Kind: kind, Trigger: trigger, State: string(result.State), Reason: result.Reason,
		OperationID: result.OperationID, Head: result.Head, From: result.From, Base: result.Base, BootedBase: result.BootedBase, LocksCleared: result.LocksCleared,
	})
}

func (rt *Runtime) appendSourceProjectionFactRecord(fact sourceProjectionFact) {
	root := strings.TrimSpace(rt.selfdevUpdaterRoot)
	if root == "" {
		return
	}
	raw, err := json.Marshal(fact)
	if err != nil {
		return
	}
	path := filepath.Join(root, filepath.FromSlash(sourceProjectionFactsFile))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		log.Printf("source projection: fact log: %v", err)
		return
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		log.Printf("source projection: fact log: %v", err)
		return
	}
	defer file.Close()
	if _, err := file.Write(append(raw, '\n')); err != nil {
		log.Printf("source projection: fact log: %v", err)
	}
}

// sourceProjectionStatus is what /health serves: the last reconcile's result,
// without running git on the caller's path. Nil when no reconcile has run.
func (rt *Runtime) sourceProjectionStatus() *sourceproj.Result {
	if rt == nil {
		return nil
	}
	rt.sourceProjectionMu.Lock()
	defer rt.sourceProjectionMu.Unlock()
	if rt.sourceProjectionLast == nil {
		return nil
	}
	copied := *rt.sourceProjectionLast
	return &copied
}

// composeFreezePatch makes the patch Freeze records the cumulative one: from
// the stack base, not from the commit the capsule was pinned to. Nothing
// changes when the capsule was not cast from a projection commit, or the
// capsule made no source patch. The bundle then names a patch
// corpusd and the host builder already accept ("U1 build notes").
func composeFreezePatch(ctx context.Context, repo string, staged *capsule.StagedRelease) error {
	if staged == nil || strings.TrimSpace(staged.SourcePatchSHA) == "" || strings.TrimSpace(staged.SourceBaseCommit) == "" {
		return nil
	}
	path := filepath.Join(staged.TemporaryRoot, "source.patch")
	patch, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("compose the cumulative source patch: %w", err)
	}
	if sourceproj.SHA256(patch) != staged.SourcePatchSHA {
		return fmt.Errorf("compose the cumulative source patch: the staged patch does not match its digest")
	}
	cumulative, base, err := sourceproj.Cumulative(ctx, repo, staged.SourceBaseCommit, patch)
	if err != nil {
		return fmt.Errorf("Freeze refused: %w", err)
	}
	if base == staged.SourceBaseCommit && string(cumulative) == string(patch) {
		return nil
	}
	tmp := path + ".cumulative"
	if err := os.WriteFile(tmp, cumulative, 0o444); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	staged.SourcePatchSHA, staged.SourceBaseCommit = sourceproj.SHA256(cumulative), base
	return nil
}
