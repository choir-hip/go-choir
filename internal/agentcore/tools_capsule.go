package agentcore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// CapsuleToolCtx is injected by guest core. Opaque handles are bound to the
// current run; signed capabilities, raw capsule IDs, paths, sockets, and keys
// never enter model-visible arguments or results.
type CapsuleToolCtx struct {
	Executor           *capsule.Executor
	AgentRunID         string
	ComputerID         string
	Role               capsule.AgentRole
	UpdaterRoot        string
	CapsuleHandle      string
	EventAppender      *computerevent.ComputerEventAppender
	TransactionBuilder *transaction.TransactionBuilder
	OperationStore     *selfdev.Store
	// OpenFreezeCandidate opens the promotion candidate when an assignment
	// freezes with none (Gate 2 track D); nil keeps the pre-created-only rule.
	OpenFreezeCandidate func(ctx context.Context, trajectoryID string) (selfdev.Operation, error)
	EventProjection     interface {
		Head(context.Context, string) (*computerevent.Head, error)
		EventByIdempotency(context.Context, string, string) (computerevent.Event, bool, error)
	}
	// ValidateCurrentObligation rejoins durable run/work/trajectory/assignment
	// fate immediately before every readable or writable capsule execution.
	ValidateCurrentObligation func(context.Context) error
	// OwnerID + Ledger wire the R4 learning-claims gate: a verifier's refs
	// resolve to commitment records only through the owner-scoped store.
	OwnerID string
	Ledger  interface {
		GetCommitmentRecord(context.Context, string, string, string) (*types.CommitmentRecord, error)
	}
	// RunningBinary identifies this computer's running executable for the
	// verification of a reused release binary; nil means
	// capsule.CurrentRunningBinary.
	RunningBinary func() (capsule.RunningBinary, error)
	// Images is the computer's asset store: Freeze files each visual
	// capture in it so the owner's browser can read it by digest
	// (visual_evidence_slice V22). Nil leaves captures unresolvable.
	Images visualAssetStore
	// frozenTree is the executor read a second Freeze on a frozen operation
	// takes to compare the capsule's tree with the candidate (R9-1); nil
	// uses Executor. A test substitutes it to drive the replay check
	// without a guest kernel.
	frozenTree frozenTreeReader
}

// frozenTreeReader is the executor surface of the replay check: the
// receipts the frozen bundle cites and a read of the live tree that changes
// no capsule state.
type frozenTreeReader interface {
	ResolveExecutionReceipts([]string) ([]capsule.ExecutionReceipt, error)
	InspectGrantedWorktree(context.Context, string, string) (capsule.GrantedWorktree, error)
	ResolveGrantedCapsuleID(agentRunID, handle string) (string, error)
	RunningJobInCapsule(capsuleID string) (yaegikernel.JobView, bool)
}

// releaseBinaryStillRunning refuses a verification pass on a bundle whose
// bin/autoputer was reused from the running build when that build is no
// longer the one running (a deploy restarted the computer after Freeze), or
// when the running identity cannot be read
// (docs/problems/selfdev-drill-capsule-cannot-build-autoputer-2026-10-14.md, A5).
func releaseBinaryStillRunning(toolCtx *CapsuleToolCtx, bundle transaction.CapsuleEffectBundle) error {
	if bundle.ReleaseBinary == nil || bundle.ReleaseBinary.Origin != capsule.ReleaseBinaryReusedRunning {
		return nil
	}
	resolve := capsule.CurrentRunningBinary
	if toolCtx != nil && toolCtx.RunningBinary != nil {
		resolve = toolCtx.RunningBinary
	}
	running, err := resolve()
	if err != nil {
		return fmt.Errorf("verify reused release binary: running build identity unavailable: %w", err)
	}
	return capsule.CheckReusedBinaryIsRunning(bundle.ReleaseBinary, running)
}

type capsuleCtxKey struct{}

func WithCapsuleCtx(ctx context.Context, value *CapsuleToolCtx) context.Context {
	return context.WithValue(ctx, capsuleCtxKey{}, value)
}

func capsuleCtxFromCtx(ctx context.Context) *CapsuleToolCtx {
	value, _ := ctx.Value(capsuleCtxKey{}).(*CapsuleToolCtx)
	return value
}

// RegisterCapsuleLocalTools installs the sole JSON escape hatch scoped to an
// already-bound capsule. All engineering effects are in-cell affordances
// staged through choir functions and authored by the reducer.
func RegisterCapsuleLocalTools(registry *toolregistry.ToolRegistry, rt *Runtime) error {
	for _, tool := range []toolregistry.Tool{
		newCapsuleGoEvalTool(rt),
	} {
		if err := registry.Register(tool); err != nil {
			return err
		}
	}
	return nil
}

func requireCapsuleRole(ctx context.Context, role capsule.AgentRole) (*CapsuleToolCtx, error) {
	value := capsuleCtxFromCtx(ctx)
	if value == nil || value.Executor == nil || value.AgentRunID == "" {
		return nil, fmt.Errorf("capsule authority unavailable")
	}
	if value.Role != role {
		return nil, fmt.Errorf("capsule operation refused for role %q", value.Role)
	}
	return value, nil
}

func requireCurrentAssignedCapsule(ctx context.Context) (*CapsuleToolCtx, error) {
	value, err := requireCapsuleRole(ctx, capsule.RoleEngineering)
	if err != nil {
		return nil, err
	}
	if value.ValidateCurrentObligation == nil {
		return nil, fmt.Errorf("assigned capsule obligation validator unavailable")
	}
	if err := value.ValidateCurrentObligation(ctx); err != nil {
		return nil, fmt.Errorf("assigned capsule obligation is no longer executable: %w", err)
	}
	return value, nil
}
func requireCapsuleMutationRole(ctx context.Context) (*CapsuleToolCtx, error) {
	toolCtx, err := requireCurrentAssignedCapsule(ctx)
	if err != nil {
		return nil, err
	}
	execution := toolregistry.ExecutionContextFrom(ctx)
	kind := ""
	if execution.RunRecord != nil {
		kind = metadataStringValue(execution.RunRecord.Metadata, "assignment_kind")
	}
	if execution.RunRecord == nil || metadataStringValue(execution.RunRecord.Metadata, "assignment_id") == "" ||
		metadataIntValue(execution.RunRecord.Metadata, "assignment_attempt") <= 0 || toolCtx.CapsuleHandle == "" ||
		(kind != string(types.EngineeringAssignmentImplementation) && kind != string(types.EngineeringAssignmentVerification)) {
		return nil, fmt.Errorf("capsule mutation requires an exact bound writable Engineering assignment")
	}
	return toolCtx, nil
}

// freezeCapsuleEffectBundle classifies and freezes the capsule diff as a
// complete verifier-ready effect bundle draft. It is the shared body of the
// commit_transaction JSON tool and the reducer's staged Freeze intent; rec is
// the bound run record (trajectory binding), never model input.
func freezeCapsuleEffectBundle(ctx context.Context, toolCtx *CapsuleToolCtx, rec *types.RunRecord, handle, buildRecipeRef string, testReceipts, dependencyToolchainRefs []string) (map[string]any, error) {
	handle, buildRecipeRef = strings.TrimSpace(handle), strings.TrimSpace(buildRecipeRef)
	testReceipts, dependencyToolchainRefs = trimNonEmptyStrings(testReceipts), trimNonEmptyStrings(dependencyToolchainRefs)
	if handle == "" || buildRecipeRef == "" || len(testReceipts) == 0 || len(dependencyToolchainRefs) == 0 {
		return nil, fmt.Errorf("complete build recipe, test receipts, and dependency/toolchain refs are required")
	}
	if toolCtx.TransactionBuilder == nil || toolCtx.EventAppender == nil || toolCtx.ComputerID == "" {
		return nil, fmt.Errorf("capsule event authority unavailable")
	}
	// A running job writes the tree and the release Freeze would capture,
	// and Freeze's quiesce would pause it (design F10).
	if toolCtx.Executor != nil {
		if job, _, running := toolCtx.Executor.RunningJob(toolCtx.AgentRunID); running {
			return nil, fmt.Errorf("job %s is still running; Freeze waits until it ends (or choir.Stop it): it writes the tree and release Freeze would capture", job.ID)
		}
	}
	trajectoryID := trajectoryIDForRun(rec)
	if toolCtx.OperationStore == nil || toolCtx.EventProjection == nil || strings.TrimSpace(toolCtx.UpdaterRoot) == "" || trajectoryID == "" {
		return nil, fmt.Errorf("self-development freeze authority unavailable")
	}
	// The operation is the assignment's own (C1e D2): resolved by its freeze
	// key, never by the trajectory. Self-development is not a special
	// operation: an assignment whose change would land on this computer
	// opens its own candidate at its first freeze.
	assignmentID := metadataStringValue(rec.Metadata, "assignment_id")
	if assignmentID == "" {
		return nil, fmt.Errorf("self-development freeze requires an assignment")
	}
	operation, found, err := toolCtx.OperationStore.GetByIdempotency(ctx, toolCtx.ComputerID, store.EngineeringFreezeIdempotencyKey(assignmentID))
	if err == nil && !found {
		if toolCtx.OpenFreezeCandidate == nil {
			return nil, fmt.Errorf("resolve self-development operation: assignment %s has none and cannot open one", assignmentID)
		}
		operation, err = toolCtx.OpenFreezeCandidate(ctx, trajectoryID)
	}
	if err != nil {
		return nil, fmt.Errorf("resolve self-development operation: %w", err)
	}
	if operation.State != selfdev.StateExecuting {
		if operation.BundleDigest != "" && (operation.State == selfdev.StateFrozen || operation.State == selfdev.StateVerified || operation.State == selfdev.StateAwaitingApproval) {
			// A replay answers only a Freeze of the tree that froze: an
			// edit after the freeze is not in the candidate, and a replay
			// told the desk it was (R9-1).
			if err := frozenCandidateTreeChanged(ctx, toolCtx, handle, operation); err != nil {
				return nil, err
			}
			return map[string]any{
				"handle": handle, "bundle_digest": operation.BundleDigest,
				"operation_id": operation.OperationID, "state": operation.State,
			}, nil
		}
		return nil, fmt.Errorf("self-development operation is %s, expected %s", operation.State, selfdev.StateExecuting)
	}
	headBefore, err := toolCtx.EventProjection.Head(ctx, toolCtx.ComputerID)
	// The pin is a state surface, not a head digest: trajectory_started and
	// projection_batch_recorded advance canonical_event_head continuously
	// without moving desired/effective state (reducer pass-through kinds).
	// A competing transition is what staleness must catch — effect_accepted,
	// materialization_*, rollback_*, or researcher_update — and every one of
	// those moves desired/effective/pending. The operation row persists the
	// pin's surface at Start (selfdev.operations.go); compare against that.
	if err != nil || headBefore == nil || headBefore.PendingTransitionRef != "" ||
		operation.DesiredHead == "" || operation.EffectiveHead == "" ||
		headBefore.DesiredEventHead != operation.DesiredHead ||
		headBefore.EffectiveEventHead != operation.EffectiveHead {
		return nil, fmt.Errorf("self-development base head unavailable, stale, or pending")
	}
	capsuleID, err := toolCtx.Executor.ResolveGrantedCapsuleID(toolCtx.AgentRunID, handle)
	if err != nil {
		return nil, err
	}
	changes, err := toolCtx.Executor.ExtractGranted(ctx, toolCtx.AgentRunID, handle)
	if err != nil {
		return nil, err
	}
	// ExtractGranted quiesced the capsule (StateFrozen) to take the diff.
	// Every Frozen-dependent read (diff, release stage, source snapshot,
	// freeze bindings, execution receipts) happens below; the deferred thaw
	// restores the capsule so the desk can finish — Complete and any freeze
	// retry both require StateActive. The terminal fate path (fate.go)
	// re-quiesces when the capsule disposition carries a freeze intent;
	// leaving the capsule active here only permits the desk's next cell.
	defer func() {
		thawCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		if thawErr := toolCtx.Executor.ThawGranted(thawCtx, toolCtx.AgentRunID, handle); thawErr != nil {
			log.Printf("agentcore: thaw capsule after freeze intent %s failed (run %s): %v", handle, toolCtx.AgentRunID, thawErr)
		}
		cancel()
	}()
	evidenceRefs := append([]string{buildRecipeRef}, testReceipts...)
	evidenceRefs = append(evidenceRefs, dependencyToolchainRefs...)
	executionReceipts, err := toolCtx.Executor.ResolveGrantedExecutionReceipts(ctx, toolCtx.AgentRunID, handle, evidenceRefs)
	if err != nil {
		return nil, err
	}
	if len(executionReceipts) < 3 {
		// Every cited ref resolved and bound (the resolver refuses any that
		// does not), so fewer than three means the slots share receipts.
		if slotsErr := freezeReceiptSlotsError(buildRecipeRef, testReceipts, dependencyToolchainRefs); slotsErr != nil {
			return nil, slotsErr
		}
		return nil, fmt.Errorf("%s", freezeDistinctReceiptsRequired)
	}
	// A receipt of a failed, timed-out or stopped execution is not evidence
	// that its slot's work succeeded (design F11, F12).
	if err := freezeReceiptOutcomesError(buildRecipeRef, testReceipts, dependencyToolchainRefs, executionReceipts); err != nil {
		return nil, err
	}
	record, err := toolCtx.TransactionBuilder.BuildBundleFromDiff(capsuleID, changes)
	if err != nil {
		return nil, err
	}
	if record.Rejected {
		return map[string]any{"handle": handle, "rejected": true, "reject_reason": record.RejectReason}, nil
	}
	staged, err := toolCtx.Executor.StageGrantedRelease(ctx, toolCtx.AgentRunID, handle, filepath.Join(toolCtx.UpdaterRoot, "incoming"))
	if err != nil {
		return nil, err
	}
	files := staged.Files
	defer os.RemoveAll(staged.TemporaryRoot)
	if err := freezeSourceChangeError(staged); err != nil {
		return nil, err
	}
	// U1: the bundle's patch is the cumulative one, from the stack base.
	if err := composeFreezePatch(ctx, toolCtx.Executor.SourceDir(), staged); err != nil {
		return nil, err
	}
	sourceTreeDigest, err := toolCtx.Executor.ResolveGrantedSourceSnapshotDigest(toolCtx.AgentRunID, handle)
	if err != nil {
		return nil, err
	}
	capabilityPolicyDigest, resourceReceipt, err := toolCtx.Executor.ResolveGrantedFreezeBindings(toolCtx.AgentRunID, handle)
	if err != nil {
		return nil, err
	}
	runtimeIntent, err := computerevent.CanonicalJSON(files)
	if err != nil {
		return nil, err
	}
	runtimeDigest := computerevent.DigestBytes(runtimeIntent)
	generatedRefs := make([]string, len(files))
	for index, file := range files {
		generatedRefs[index] = "artifact:sha256:" + file.SHA256
	}
	record.ComputerID = toolCtx.ComputerID
	record.BaseEventHead = operation.BaseHead
	record.TrajectoryRef = trajectoryID
	record.CapabilityPolicyDigest = capabilityPolicyDigest
	record.SourceTreeRef = "source-tree:sha256:" + sourceTreeDigest
	record.GeneratedArtifactRefs = generatedRefs
	record.BuildRecipeRef = buildRecipeRef
	record.RuntimeArtifactRef = "runtime-artifact:sha256:" + runtimeDigest
	record.TestReceipts = testReceipts
	record.VerifierReceipts = []string{}
	record.DependencyToolchainRefs = dependencyToolchainRefs
	record.ResourceReceipts = []string{resourceReceipt}
	record.SourcePatchSHA256 = staged.SourcePatchSHA
	record.SourcePatchBaseCommit = staged.SourceBaseCommit
	record.RuntimeFiles = files
	record.ReleaseBinary = staged.Binary
	record.VisualEvidence = staged.Visual
	record.ContentDigest, err = record.ComputeContentDigest()
	if err != nil || record.Validate(false) != nil {
		return nil, fmt.Errorf("complete capsule effect bundle draft unavailable")
	}
	draft, err := computerevent.CanonicalJSON(record)
	if err != nil {
		return nil, fmt.Errorf("canonical capsule effect bundle draft: %w", err)
	}
	frozenRoot := filepath.Join(toolCtx.UpdaterRoot, "incoming", record.ContentDigest)
	if err := publishFrozenDraft(staged.TemporaryRoot, frozenRoot, draft); err != nil {
		return nil, err
	}
	operation, err = toolCtx.OperationStore.Transition(ctx, toolCtx.ComputerID, operation.OperationID, selfdev.StateExecuting, selfdev.StateFrozen, func(next *selfdev.Operation) error {
		next.CapsuleID = capsuleID
		next.BundleDigest = record.ContentDigest
		next.DesiredHead = headBefore.DesiredEventHead
		next.EffectiveHead = headBefore.EffectiveEventHead
		return nil
	})
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		"handle": handle, "change_count": len(changes), "classifier_version": record.ClassifierV,
		"classifier_digest": record.ClassifierDigest, "groups": record.Groups,
		"content_digest": record.ContentDigest, "bundle_digest": record.ContentDigest,
		"operation_id": operation.OperationID, "state": operation.State,
		// Internal: the changed source files, for rule precommit settlement;
		// commitFreezeIntent removes the key before the cell sees the result.
		freezeSourcePathsKey: capsuleSourcePaths(changes),
	}
	if record.ReleaseBinary != nil {
		out["release_binary_origin"] = record.ReleaseBinary.Origin
	}
	// Visual evidence: file the captures in the asset store, tell the cell
	// what resolved, and leave the runtime's observation for the rule pass.
	recordFreezeVisual(toolCtx.Images, frozenRoot, staged.Visual, out)
	return out, nil
}

// frozenTreeChangeListMax bounds the paths a changed-tree refusal lists.
const frozenTreeChangeListMax = 20

// frozenCandidateTreeChanged decides whether a Freeze on an already frozen
// operation may replay it: nil when the capsule's tree is the one that froze,
// otherwise an error the desk can read (R9-1,
// docs/problems/freeze-replays-a-changed-tree-2026-10-26.md). The frozen
// tree's digest is the worktree digest of the latest receipt the bundle
// cites, which the first Freeze's certified binding required to be the frozen
// tree, so nothing new is stored. The live read changes no capsule state.
// Both the /workspace/platform digest and the classified effect set (Ignored
// and Unknown left out: /tmp and toolchain caches never enter a bundle) must
// match. A tree that cannot be compared is not an unchanged one: that refuses
// too. A refusal changes nothing; the desk can still Complete.
func frozenCandidateTreeChanged(ctx context.Context, toolCtx *CapsuleToolCtx, handle string, operation selfdev.Operation) error {
	frozen := fmt.Sprintf("this assignment's candidate is already frozen as bundle %s (operation %s, %s)", operation.BundleDigest, operation.OperationID, operation.State)
	unchecked := func(reason string) error {
		return fmt.Errorf("Freeze refused: %s, and Freeze could not check that this capsule's tree is the one that froze (%s), so it does not answer for it. "+
			"The frozen candidate is what will be reviewed; Complete to end this round. A change made after the freeze needs a new round", frozen, reason)
	}
	var reader frozenTreeReader = toolCtx.frozenTree
	if reader == nil {
		if toolCtx.Executor == nil {
			return unchecked("capsule executor unavailable")
		}
		reader = toolCtx.Executor
	}
	bundle, _, err := readBoundSelfDevelopmentBundle(filepath.Join(toolCtx.UpdaterRoot, "incoming", operation.BundleDigest), operation)
	if err != nil {
		return unchecked(err.Error())
	}
	// The read is of the operation's capsule, whatever the handle names,
	// and no job of any run may be writing it.
	capsuleID, err := reader.ResolveGrantedCapsuleID(toolCtx.AgentRunID, handle)
	if err != nil {
		return unchecked(err.Error())
	}
	if capsuleID == "" || capsuleID != operation.CapsuleID {
		return unchecked("the handle names another capsule than the one this candidate froze")
	}
	if job, running := reader.RunningJobInCapsule(capsuleID); running {
		return unchecked(fmt.Sprintf("job %s is still running in this capsule and writes its tree", job.ID))
	}
	refs := append([]string{bundle.BuildRecipeRef}, bundle.TestReceipts...)
	refs = append(refs, bundle.DependencyToolchainRefs...)
	receipts, err := reader.ResolveExecutionReceipts(trimNonEmptyStrings(refs))
	if err != nil {
		return unchecked(fmt.Sprintf("the frozen bundle's receipts are unavailable: %v", err))
	}
	if len(receipts) == 0 {
		return unchecked("the frozen bundle's receipts are unavailable: none resolved")
	}
	// The latest receipt by occurrence, as the binder picks it.
	latestAt, frozenDigest := "", ""
	for _, receipt := range receipts {
		if receipt.OccurredAt > latestAt {
			latestAt, frozenDigest = receipt.OccurredAt, receipt.WorktreeDigest
		}
	}
	live, err := reader.InspectGrantedWorktree(ctx, toolCtx.AgentRunID, handle)
	if err != nil {
		return unchecked(err.Error())
	}
	current, err := toolCtx.TransactionBuilder.BuildBundleFromDiff(capsuleID, live.Changes)
	if err != nil {
		return unchecked(err.Error())
	}
	changed := append(frozenTreeChanges(bundle.Groups, current.Groups), frozenReleaseChanges(bundle, live.ReleaseFiles)...)
	if frozenDigest == "" || live.Digest == "" {
		return unchecked("a tree digest is unavailable")
	}
	if strings.TrimPrefix(frozenDigest, "sha256:") == strings.TrimPrefix(live.Digest, "sha256:") && len(changed) == 0 {
		return nil
	}
	what := "the files already in the candidate changed"
	if len(changed) > 0 {
		listed := changed
		if len(listed) > frozenTreeChangeListMax {
			listed = listed[:frozenTreeChangeListMax]
		}
		what = strings.Join(listed, ", ")
		if more := len(changed) - len(listed); more > 0 {
			what += fmt.Sprintf(" and %d more", more)
		}
	}
	digests := ""
	if strings.TrimPrefix(frozenDigest, "sha256:") != strings.TrimPrefix(live.Digest, "sha256:") {
		digests = fmt.Sprintf(" (/workspace/platform was %s at the freeze, is %s now)", frozenDigest, live.Digest)
	}
	return fmt.Errorf("Freeze refused: %s, and this capsule's tree changed since: %s%s. "+
		"The change is not in the candidate: the frozen candidate is what will be reviewed. Complete to end this round (a completed Complete cites a receipt that observed the current tree); "+
		"to put this change in a candidate, the round must end and a new round start", frozen, what, digests)
}

// frozenReleaseChanges lists, sorted by path, how the capsule's live release
// tree moved from the bundle's RuntimeFiles: "added release", "removed
// release" or "modified release" (content or mode), per path. The workspace
// digest and the classified groups do not cover the release tree, so a
// same-path rebuild of bin/autoputer shows only here. A bin/autoputer the
// freeze reused from the running build was staged beside the capsule's
// release, not read from it, so its absence from the live tree is not a
// removal.
func frozenReleaseChanges(bundle transaction.CapsuleEffectBundle, live []capsule.FrozenReleaseFile) []string {
	reused := bundle.ReleaseBinary != nil && bundle.ReleaseBinary.Origin == capsule.ReleaseBinaryReusedRunning
	frozen := map[string]capsule.FrozenReleaseFile{}
	for _, file := range bundle.RuntimeFiles {
		if reused && file.Path == capsule.ReleaseBinaryPath {
			continue
		}
		frozen[file.Path] = file
	}
	paths := map[string]string{}
	for _, file := range live {
		before, in := frozen[file.Path]
		switch {
		case !in:
			paths[file.Path] = "added"
		case before.SHA256 != file.SHA256 || before.Mode != file.Mode:
			paths[file.Path] = "modified"
		}
		delete(frozen, file.Path)
	}
	for path := range frozen {
		paths[path] = "removed"
	}
	sorted := make([]string, 0, len(paths))
	for path := range paths {
		sorted = append(sorted, path)
	}
	sort.Strings(sorted)
	out := make([]string, len(sorted))
	for i, path := range sorted {
		out[i] = paths[path] + " release " + path
	}
	return out
}

// frozenTreeChanges lists, sorted by path, how the classified effect set
// moved from the frozen bundle's to the live tree's: "added", "removed" or
// "modified" since the freeze, per path. A path the candidate added and the
// tree no longer changes was removed; one the candidate deleted and the tree
// no longer deletes was added back.
func frozenTreeChanges(frozenGroups, liveGroups map[string][]transaction.ChangeRecord) []string {
	flatten := func(groups map[string][]transaction.ChangeRecord) map[string]transaction.ChangeRecord {
		out := map[string]transaction.ChangeRecord{}
		for _, records := range groups {
			for _, record := range records {
				out[record.Path] = record
			}
		}
		return out
	}
	frozen, live := flatten(frozenGroups), flatten(liveGroups)
	paths := map[string]string{}
	for path, before := range frozen {
		after, still := live[path]
		switch {
		case !still && before.Kind == "added":
			paths[path] = "removed"
		case !still && before.Kind == "deleted":
			paths[path] = "added"
		case !still:
			paths[path] = "modified"
		case after.Kind == before.Kind && after.Mode == before.Mode:
		case after.Kind == "deleted":
			paths[path] = "removed"
		case before.Kind == "deleted":
			paths[path] = "added"
		default:
			paths[path] = "modified"
		}
	}
	for path, after := range live {
		if _, inFrozen := frozen[path]; inFrozen {
			continue
		}
		switch after.Kind {
		case "added":
			paths[path] = "added"
		case "deleted":
			paths[path] = "removed"
		default:
			paths[path] = "modified"
		}
	}
	sorted := make([]string, 0, len(paths))
	for path := range paths {
		sorted = append(sorted, path)
	}
	sort.Strings(sorted)
	out := make([]string, len(sorted))
	for i, path := range sorted {
		out[i] = paths[path] + " " + path
	}
	return out
}

// publishFrozenDraft writes the draft into the staged tree and publishes the
// tree at frozenRoot. Every file and directory is fsynced before the rename
// and incoming/ after it: a refresh hard-kills the guest, and a rename that
// reaches the disk before its files leaves a short or empty bundle under the
// operation's digest (docs/problems/awaiting-candidate-unreadable-after-refresh-2026-10-26.md).
func publishFrozenDraft(temporaryRoot, frozenRoot string, draft []byte) error {
	if err := os.WriteFile(filepath.Join(temporaryRoot, "bundle.draft.json"), draft, 0o400); err != nil {
		return err
	}
	if err := syncTree(temporaryRoot); err != nil {
		return fmt.Errorf("sync frozen bundle draft: %w", err)
	}
	if err := os.Rename(temporaryRoot, frozenRoot); err != nil {
		existing, readErr := os.ReadFile(filepath.Join(frozenRoot, "bundle.draft.json"))
		if readErr != nil || !bytes.Equal(existing, draft) {
			return fmt.Errorf("freeze immutable bundle draft: %w", err)
		}
	}
	// incoming/ itself may be new since boot: sync its entry in the
	// updater root too, or the whole subtree can vanish under a kill.
	incoming := filepath.Dir(frozenRoot)
	if err := syncPath(incoming); err != nil {
		return fmt.Errorf("sync frozen bundle draft: %w", err)
	}
	if err := syncPath(filepath.Dir(incoming)); err != nil {
		return fmt.Errorf("sync frozen bundle draft: %w", err)
	}
	return nil
}

// recordSelfDevelopmentVerdict records one verdict on a frozen operation: the
// verification event, then frozen -> verified -> awaiting_approval on pass or
// frozen -> failed on fail. The legacy verifier slot and the decision-model
// judgment (judgeFrozenCandidate) both write through it; extra fields join
// the event payload but not its idempotency key.
func recordSelfDevelopmentVerdict(ctx context.Context, toolCtx *CapsuleToolCtx, trajectoryID, operationID, bundleDigest, decision string, verifierRefs []string, extra map[string]any) (map[string]any, error) {
	if toolCtx.OperationStore == nil || toolCtx.EventAppender == nil || toolCtx.EventProjection == nil {
		return nil, fmt.Errorf("self-development verification authority unavailable")
	}
	operationID, bundleDigest, decision = strings.TrimSpace(operationID), strings.TrimSpace(bundleDigest), strings.TrimSpace(decision)
	verifierRefs = trimNonEmptyStrings(verifierRefs)
	if operationID == "" || !computerevent.IsSHA256(bundleDigest) || (decision != "pass" && decision != "fail") || len(verifierRefs) == 0 {
		return nil, fmt.Errorf("complete exact verification binding is required")
	}
	operation, err := toolCtx.OperationStore.Get(ctx, toolCtx.ComputerID, operationID)
	if err != nil {
		return nil, err
	}
	if operation.TrajectoryID != strings.TrimSpace(trajectoryID) {
		return nil, fmt.Errorf("verification does not bind the frozen operation")
	}
	if operation.State == selfdev.StateAwaitingApproval && decision == "pass" {
		// Replay detection: the operation's durable digest is the finalized
		// bundle digest, so the replayed intent names it — not the draft
		// content digest inside bundle.json. The finalized bundle must still
		// validate and carry the recorded verifier ref.
		rawFinal, readErr := os.ReadFile(filepath.Join(toolCtx.UpdaterRoot, "incoming", operation.BundleDigest, "bundle.json"))
		var finalBundle transaction.CapsuleEffectBundle
		if readErr == nil && json.Unmarshal(rawFinal, &finalBundle) == nil &&
			operation.BundleDigest == bundleDigest && finalBundle.Validate(true) == nil &&
			selfDevelopmentContainsString(finalBundle.VerifierReceipts, firstString(operation.VerifierRefs)) {
			return map[string]any{"operation_id": operation.OperationID, "state": operation.State, "bundle_digest": operation.BundleDigest, "decision": decision, "verifier_ref": firstString(operation.VerifierRefs)}, nil
		}
	}
	if operation.BundleDigest != bundleDigest {
		return nil, fmt.Errorf("verification does not bind the frozen bundle content")
	}
	record := map[string]any{
		"schema_version": 1, "operation_id": operation.OperationID, "bundle_digest": operation.BundleDigest,
		"decision": decision, "verifier_refs": verifierRefs, "verifier_run_id": toolCtx.AgentRunID,
		// R4 learning-claims gate: a self-development outcome claim (this
		// verification) must cite scored commitment evidence. The gate
		// flags rather than refuses — an unbacked claim stays
		// distinguishable on the durable event instead of breaking
		// verifier flows predating the ledger.
		"claim_gate": learningClaimGate(ctx, toolCtx, verifierRefs),
	}
	for key, value := range extra {
		if _, taken := record[key]; !taken {
			record[key] = value
		}
	}
	payload, err := computerevent.CanonicalJSON(record)
	if err != nil {
		return nil, err
	}
	// claim_gate is a pure derivation of verifier_refs — it never enters
	// the idempotency input, so an event appended before this field
	// existed still dedupes and a replayed cell replays the same event.
	idempotencyRecord := map[string]any{
		"schema_version": 1, "operation_id": operation.OperationID, "bundle_digest": operation.BundleDigest,
		"decision": decision, "verifier_refs": verifierRefs, "verifier_run_id": toolCtx.AgentRunID,
	}
	idempotencyPayload, err := computerevent.CanonicalJSON(idempotencyRecord)
	if err != nil {
		return nil, err
	}
	eventIdempotency := computerevent.DigestBytes(append([]byte("selfdev-verification-v1\x00"), idempotencyPayload...))
	verificationEvent, found, lookupErr := toolCtx.EventProjection.EventByIdempotency(ctx, toolCtx.ComputerID, eventIdempotency)
	if lookupErr != nil {
		return nil, lookupErr
	}
	if !found {
		eventID, eventErr := computerevent.NewEventID()
		if eventErr != nil {
			return nil, eventErr
		}
		event := computerevent.Event{
			SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: toolCtx.ComputerID,
			EventKind: computerevent.EventVerificationRecorded, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
			IdempotencyKey: eventIdempotency, TrajectoryID: operation.TrajectoryID, CapsuleID: operation.CapsuleID,
			ActorProfile: agentprofile.Engineering, AuthorityRef: "guest-core:self-development-verifier",
			PrivacyClass: "public", ReducerVersion: computerevent.ReducerVersionV1,
		}
		if _, _, appendErr := toolCtx.EventAppender.AppendNewPayload(ctx, event, computerevent.TransitionInput{}, payload, "application/vnd.choir.self-development-verification+json", "public"); appendErr != nil {
			return nil, appendErr
		}
		verificationEvent, found, lookupErr = toolCtx.EventProjection.EventByIdempotency(ctx, toolCtx.ComputerID, eventIdempotency)
		if lookupErr != nil || !found {
			return nil, fmt.Errorf("verification event projection unavailable: %w", lookupErr)
		}
	}
	verifierRef, err := verificationEvent.Digest()
	if err != nil {
		return nil, err
	}
	if decision == "fail" {
		operation, err = toolCtx.OperationStore.Transition(ctx, toolCtx.ComputerID, operation.OperationID, selfdev.StateFrozen, selfdev.StateFailed, func(next *selfdev.Operation) error {
			next.VerifierRefs = []string{verifierRef}
			next.TerminalError = "independent judgment rejected the frozen bundle"
			return nil
		})
	} else {
		operation, err = finalizeVerifiedCapsuleBundle(ctx, toolCtx, operation, verifierRef)
		if err == nil {
			operation, err = toolCtx.OperationStore.Transition(ctx, toolCtx.ComputerID, operation.OperationID, selfdev.StateVerified, selfdev.StateAwaitingApproval, nil)
		}
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"operation_id": operation.OperationID, "state": operation.State, "bundle_digest": operation.BundleDigest, "decision": decision, "verifier_ref": verifierRef}, nil
}

// learningClaimGate applies the R4 learning-claims gate to a verification's
// cited refs: "backed" when at least one ref names a stored commitment
// record that carries a verdict (resolved discrepancy) or a score stamp —
// the scored evidence the claim must stand on. The check tolerates scheme-
// prefixed and canonical-id refs. A nil ledger (management capsule ctx, or
// a desk without the store wired) yields "unbacked" — the gate never
// fails open by omission.
func learningClaimGate(ctx context.Context, toolCtx *CapsuleToolCtx, refs []string) string {
	if toolCtx == nil || toolCtx.Ledger == nil || toolCtx.OwnerID == "" {
		return types.LearningClaimUnbacked
	}
	ownerID, computerID := toolCtx.OwnerID, toolCtx.ComputerID
	return types.GateLearningClaim(refs, func(recordID string) bool {
		rec, err := toolCtx.Ledger.GetCommitmentRecord(ctx, ownerID, computerID, recordID)
		if err != nil || rec == nil {
			return false
		}
		return rec.Discrepancy != types.DiscrepancyUnresolved || len(rec.Scores) > 0
	})
}

func finalizeVerifiedCapsuleBundle(ctx context.Context, toolCtx *CapsuleToolCtx, operation selfdev.Operation, verifierRef string) (selfdev.Operation, error) {
	if operation.State == selfdev.StateVerified {
		return operation, nil
	}
	contentDigest := operation.BundleDigest
	draftRoot := filepath.Join(toolCtx.UpdaterRoot, "incoming", contentDigest)
	var bundle transaction.CapsuleEffectBundle
	rawDraft, draftErr := os.ReadFile(filepath.Join(draftRoot, "bundle.draft.json"))
	if draftErr == nil {
		decoder := json.NewDecoder(bytes.NewReader(rawDraft))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&bundle) != nil || bundle.ContentDigest != contentDigest || bundle.Validate(false) != nil {
			return operation, fmt.Errorf("verified bundle draft binding mismatch")
		}
		bundle.VerifierReceipts = []string{verifierRef}
		if bundle.Validate(true) != nil {
			return operation, fmt.Errorf("verified bundle is incomplete")
		}
	} else {
		mapping, err := os.ReadFile(filepath.Join(toolCtx.UpdaterRoot, "bundle-finalizations", contentDigest))
		if err != nil || !computerevent.IsSHA256(strings.TrimSpace(string(mapping))) {
			return operation, fmt.Errorf("verified bundle draft unavailable")
		}
		finalRoot := filepath.Join(toolCtx.UpdaterRoot, "incoming", strings.TrimSpace(string(mapping)))
		rawFinal, err := os.ReadFile(filepath.Join(finalRoot, "bundle.json"))
		if err != nil || json.Unmarshal(rawFinal, &bundle) != nil || bundle.ContentDigest != contentDigest ||
			!selfDevelopmentContainsString(bundle.VerifierReceipts, verifierRef) || bundle.Validate(true) != nil {
			return operation, fmt.Errorf("verified bundle recovery binding mismatch")
		}
	}
	if err := releaseBinaryStillRunning(toolCtx, bundle); err != nil {
		return operation, err
	}
	finalBytes, err := computerevent.CanonicalJSON(bundle)
	if err != nil {
		return operation, err
	}
	finalDigest := computerevent.DigestBytes(finalBytes)
	finalRoot := filepath.Join(toolCtx.UpdaterRoot, "incoming", finalDigest)
	if draftErr == nil {
		// Durable before the rename, as Freeze: the final bundle, the whole
		// tree (a draft frozen by an older binary was never synced), and the
		// mapping recovery reads once the draft root is gone.
		if err := writeFileDurable(filepath.Join(draftRoot, "bundle.json"), finalBytes, 0o400); err != nil {
			return operation, fmt.Errorf("write verified bundle: %w", err)
		}
		if err := syncTree(draftRoot); err != nil {
			return operation, fmt.Errorf("sync verified bundle: %w", err)
		}
		mappingRoot := filepath.Join(toolCtx.UpdaterRoot, "bundle-finalizations")
		if err := os.MkdirAll(mappingRoot, 0o700); err != nil {
			return operation, err
		}
		if err := syncPath(toolCtx.UpdaterRoot); err != nil {
			return operation, fmt.Errorf("sync bundle finalizations: %w", err)
		}
		if err := writeFileDurable(filepath.Join(mappingRoot, contentDigest), []byte(finalDigest), 0o400); err != nil {
			return operation, fmt.Errorf("write bundle finalization: %w", err)
		}
		if err := os.Rename(draftRoot, finalRoot); err != nil {
			existing, readErr := os.ReadFile(filepath.Join(finalRoot, "bundle.json"))
			if readErr != nil || !bytes.Equal(existing, finalBytes) {
				return operation, fmt.Errorf("publish verified bundle: %w", err)
			}
		}
		if err := syncPath(filepath.Dir(finalRoot)); err != nil {
			return operation, fmt.Errorf("sync verified bundle: %w", err)
		}
	}
	eventIdempotency := computerevent.DigestBytes([]byte("capsule-effect-final-v1\x00" + operation.OperationID + "\x00" + finalDigest))
	projected, found, err := toolCtx.EventProjection.EventByIdempotency(ctx, toolCtx.ComputerID, eventIdempotency)
	if err != nil {
		return operation, err
	}
	if !found {
		eventID, eventErr := computerevent.NewEventID()
		if eventErr != nil {
			return operation, eventErr
		}
		event := computerevent.Event{
			SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: toolCtx.ComputerID,
			EventKind: computerevent.EventEffectProposed, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
			IdempotencyKey: eventIdempotency, TrajectoryID: operation.TrajectoryID, CapsuleID: operation.CapsuleID,
			ActorProfile: agentprofile.Engineering, AuthorityRef: "guest-core:verified-bundle-finalizer",
			PrivacyClass: "public", VerifierRefs: []string{verifierRef}, ReducerVersion: computerevent.ReducerVersionV1,
		}
		if _, pinnedDigest, appendErr := toolCtx.EventAppender.AppendNewPayload(ctx, event, computerevent.TransitionInput{}, finalBytes, "application/vnd.choir.capsule-effect+json", "public"); appendErr != nil || pinnedDigest != finalDigest {
			return operation, fmt.Errorf("append verified effect proposal: %w", appendErr)
		}
		projected, found, err = toolCtx.EventProjection.EventByIdempotency(ctx, toolCtx.ComputerID, eventIdempotency)
		if err != nil || !found {
			return operation, fmt.Errorf("verified effect proposal projection unavailable")
		}
	}
	if projected.ProposedEffectRef != finalDigest || projected.TrajectoryID != operation.TrajectoryID || projected.CapsuleID != operation.CapsuleID {
		return operation, fmt.Errorf("verified effect proposal binding mismatch")
	}
	head, err := toolCtx.EventProjection.Head(ctx, toolCtx.ComputerID)
	if err != nil || head == nil {
		return operation, fmt.Errorf("verified effect head unavailable")
	}
	return toolCtx.OperationStore.Transition(ctx, toolCtx.ComputerID, operation.OperationID, selfdev.StateFrozen, selfdev.StateVerified, func(next *selfdev.Operation) error {
		next.BundleDigest = finalDigest
		next.VerifierRefs = []string{verifierRef}
		next.DesiredHead = head.DesiredEventHead
		next.EffectiveHead = head.EffectiveEventHead
		return nil
	})
}

// newCapsuleGoEvalTool evaluates model-authored Go source inside the assigned
// capsule through the broker. It is Engineering-only for now: the Research
// Go-only profile requires a Research capsule-context injection path in
// runtime.go that is a separate wiring slice. The tool uses
// requireCurrentAssignedCapsule, which revalidates the durable assignment,
// cancellation intent, work-item, run-state, and capsule fate immediately
// before evaluation. The package allowlist is resolved server-side by the
// broker from the verified capability role; the model never supplies
// allowed_packages.

func newCapsuleGoEvalTool(rt *Runtime) toolregistry.Tool {
	type args struct {
		Source    string `json:"source"`
		Cwd       string `json:"cwd"`
		TimeoutMS int    `json:"timeout_ms"`
	}
	return toolregistry.Tool{
		Name: "capsule_go_eval", Description: "Evaluate model-authored Go source inside the assigned isolated capsule.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"source":     map[string]any{"type": "string", "description": "Raw Go source for one REPL cell."},
			"cwd":        map[string]any{"type": "string", "description": "Optional working directory inside /workspace/platform."},
			"timeout_ms": map[string]any{"type": "integer", "description": "Evaluation timeout in milliseconds."},
		}, []string{"source"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			toolCtx, err := requireCurrentAssignedCapsule(ctx)
			if err != nil {
				return "", err
			}
			var input args
			if err := json.Unmarshal(raw, &input); err != nil {
				return "", err
			}
			if strings.TrimSpace(input.Source) == "" {
				return "", fmt.Errorf("capsule_go_eval: source is required")
			}
			evalCtx := ctx
			if input.TimeoutMS > 0 {
				var cancel context.CancelFunc
				evalCtx, cancel = context.WithTimeout(ctx, time.Duration(input.TimeoutMS)*time.Millisecond)
				defer cancel()
			}
			req := capsule.GoEvalRequest{Source: input.Source, Cwd: input.Cwd, TimeoutMS: input.TimeoutMS}
			reduction := rlmReductionForCall(ctx, rt, toolCtx)
			if reduction.active {
				req.Records = reduction.records.view
				req.Inbox = reduction.inbox
				// RLM prompt-as-variable: pending update_coagent records ride
				// the request so choir.Updates() exposes them in the cell.
				req.Updates = pendingCellUpdates(ctx, rt, toolregistry.ExecutionContextFrom(ctx))
				rt.armCellTerminalDeadline(ctx, reduction)
			}
			result, err := traceCall(rt, toolCtx.AgentRunID, "cell.go_eval", func() (capsule.GoEvalResult, error) {
				return toolCtx.Executor.GoEval(evalCtx, toolCtx.AgentRunID, toolCtx.CapsuleHandle, req)
			})
			if err != nil {
				if reduction.active {
					fate := cellFateFailure
					if evalCtx.Err() == context.DeadlineExceeded {
						fate = cellFateTimeout
					}
					if fateErr := reduction.abort(context.Background(), fate, err.Error()); fateErr != nil {
						return "", fateErr
					}
				}
				return "", err
			}
			if reduction.active && result.Error != "" {
				fate := cellFateFailure
				if evalCtx.Err() == context.DeadlineExceeded {
					fate = cellFateTimeout
				}
				if fateErr := reduction.abort(context.Background(), fate, result.Error); fateErr != nil {
					return "", fateErr
				}
			}
			if reduction.active && result.Error == "" {
				reduction.records = reduction.records.capToShown(rt.noteRecordsShown(reduction.scope.RunID, result.RecordsShown))
				if rerr := traceErr(rt, toolCtx.AgentRunID, "cell.reduce", func() error { return reduction.commit(ctx, result.Intents) }); rerr != nil {
					return "", rerr // commit's error names a freeze it committed
				}
				for _, in := range reduction.receipt.Intents {
					result.StagedIntentIDs = append(result.StagedIntentIDs, fmt.Sprintf("rlm:%s:%d", in.Kind, in.Seq))
				}
				result.FateTerminal = reduction.fateTerminal
				result.FreezeResult = reduction.freezeResult
				result.VerifyResult = reduction.verifyResult
				result.JobResults = reduction.jobResults
			}
			if reduction.active {
				// The job this turn waits on, if the cell looked at one
				// still running (capsule_jobs.go, cellJobPark).
				result.JobPark = cellJobPark(result)
			}
			return toolregistry.ResultJSON(result)
		},
	}
}

// committedFreezeMarker prefixes the note a cell error carries when its
// freeze committed before a later intent failed.
const committedFreezeMarker = "the freeze in this cell committed: freeze_result="

// withCommittedFreeze adds the committed freeze to a cell's reduce error.
// The error path returns no result, so without it a durable freeze left no
// freeze_result in the cell's output and the completion guard kept a stale
// refusal (review round 4b, DeepSeek).
func withCommittedFreeze(err error, freezeResult map[string]any) error {
	if err == nil || len(freezeResult) == 0 {
		return err
	}
	encoded, encodeErr := json.Marshal(freezeResult)
	if encodeErr != nil {
		return err
	}
	return fmt.Errorf("%w; %s%s", err, committedFreezeMarker, encoded)
}

func sortedUniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

// recordedCommandsFromReceipts projects resolved execution receipts into the
// recorded-command rows the assignment report binds. Shared by the retired
// JSON tool path and the reducer's Complete-intent fate path.
func recordedCommandsFromReceipts(receipts []capsule.ExecutionReceipt) []types.EngineeringRecordedCommand {
	commands := make([]types.EngineeringRecordedCommand, 0, len(receipts))
	for _, receipt := range receipts {
		commandDigest := objectgraph.SHA256([]byte(receipt.Command))
		commandID := "capsule-command:" + objectgraph.SHA256([]byte(receipt.ReceiptRef))
		commands = append(commands, types.EngineeringRecordedCommand{CommandID: commandID, CommandDigest: commandDigest, ExecutionRef: receipt.ReceiptRef, ExitCode: receipt.ExitCode})
	}
	return commands
}

// recordedOutputsFromReceipts projects resolved execution receipts into the
// recorded stdout/stderr output rows the assignment report binds.
func recordedOutputsFromReceipts(receipts []capsule.ExecutionReceipt) []types.EngineeringRecordedOutput {
	outputs := make([]types.EngineeringRecordedOutput, 0, len(receipts)*2)
	for _, receipt := range receipts {
		commandID := "capsule-command:" + objectgraph.SHA256([]byte(receipt.ReceiptRef))
		outputs = append(outputs,
			types.EngineeringRecordedOutput{OutputID: commandID + ":stdout", Kind: "stdout", Digest: "sha256:" + strings.TrimPrefix(receipt.StdoutDigest, "sha256:"), Ref: receipt.ReceiptRef + "#stdout"},
			types.EngineeringRecordedOutput{OutputID: commandID + ":stderr", Kind: "stderr", Digest: "sha256:" + strings.TrimPrefix(receipt.StderrDigest, "sha256:"), Ref: receipt.ReceiptRef + "#stderr"})
	}
	return outputs
}

func trimNonEmptyStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}

// completeExecutionRefs trims a Complete's execution refs and drops blanks
// and repeats, keeping first-cited order.
func completeExecutionRefs(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range trimNonEmptyStrings(values) {
		if !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

const freezeDistinctReceiptsRequired = "distinct build, test, and dependency/toolchain execution receipts are required"

// freezeReceiptSlotsError diagnoses a Freeze whose three receipt slots name
// fewer than three distinct receipts: which slots share which receipt, and
// what each slot should cite (docs/problems/selfdev-freeze-receipts-refused-2026-10-15.md).
// It decides nothing new: nil exactly when the slots hold three or more
// distinct refs, the count the executor's resolver dedupes to.
// freezeSourceChangeError refuses a candidate whose capsule changed no
// /workspace/platform source. A release is source (api_release.go), and a
// frontend change is source too (frontend/ is under /workspace/platform), so
// an empty source patch means the release is the base tree rebuilt. Failure
// 15 (docs/problems/engineering-completion-path-cluster-2026-10-20.md): a
// re-cast ran in a fresh capsule without the earlier round's change, and the
// unchanged tree passed verify, approval and apply.
func freezeSourceChangeError(staged *capsule.StagedRelease) error {
	if staged != nil && strings.TrimSpace(staged.SourcePatchSHA) != "" {
		return nil
	}
	return fmt.Errorf("Freeze refused: this capsule changed no /workspace/platform source, so its candidate would release the base tree unchanged. " +
		"Each assignment, a re-cast included, runs in a fresh capsule from the base tree: an earlier assignment's edits are not here. " +
		"Make the change in this capsule, run its tests, then Freeze; or Complete and report that nothing was changed")
}

func freezeReceiptSlotsError(buildRecipeRef string, testReceipts, dependencyToolchainRefs []string) error {
	var order []string
	slots := map[string][]string{}
	cite := func(ref, slot string) {
		if _, seen := slots[ref]; !seen {
			order = append(order, ref)
		}
		slots[ref] = append(slots[ref], slot)
	}
	cite(buildRecipeRef, "build_recipe_ref")
	for i, ref := range testReceipts {
		cite(ref, fmt.Sprintf("test_receipts[%d]", i))
	}
	for i, ref := range dependencyToolchainRefs {
		cite(ref, fmt.Sprintf("dependency_toolchain_refs[%d]", i))
	}
	if len(order) >= 3 {
		return nil
	}
	var shared []string
	for _, ref := range order {
		names := slots[ref]
		if len(names) < 2 {
			continue
		}
		joined := strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
		shared = append(shared, joined+" cite the same receipt "+ref)
	}
	return fmt.Errorf("%s: %s; cite three different cells — build_recipe_ref: the cell that built the release tree; test_receipts: the cell that ran the tests or checks, after your last write; dependency_toolchain_refs: a cell that recorded the toolchain (go version, node --version, npm --version)",
		freezeDistinctReceiptsRequired, strings.Join(shared, "; "))
}

const freezeFailedReceipts = "a cited execution receipt records an execution that did not succeed"

// freezeReceiptOutcomesError refuses Freeze evidence whose recorded outcome
// is a failure: a non-zero exit code (a failed or killed command, a cell
// that panicked), a job that did not end done, and, in the build and test
// slots, a job whose workspace changed while it ran
// (docs/design/capsule-long-exec-2026-10-15.md, F11/F12). nil when every
// cited receipt records a success.
func freezeReceiptOutcomesError(buildRecipeRef string, testReceipts, dependencyToolchainRefs []string, receipts []capsule.ExecutionReceipt) error {
	byRef := make(map[string]capsule.ExecutionReceipt, len(receipts))
	for _, receipt := range receipts {
		byRef[receipt.ReceiptRef] = receipt
	}
	var problems []string
	check := func(ref, slot string, buildOrTest bool) {
		receipt, ok := byRef[strings.TrimSpace(ref)]
		if !ok {
			return
		}
		if why := capsule.JobReceiptOutcomeRefusal(receipt, buildOrTest); why != "" {
			problems = append(problems, slot+" cites "+receipt.ReceiptRef+": "+why)
		}
	}
	check(buildRecipeRef, "build_recipe_ref", true)
	for i, ref := range testReceipts {
		check(ref, fmt.Sprintf("test_receipts[%d]", i), true)
	}
	for i, ref := range dependencyToolchainRefs {
		check(ref, fmt.Sprintf("dependency_toolchain_refs[%d]", i), false)
	}
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("%s: %s; cite work that succeeded: a cell that ran its check without panicking, or a job whose state is done", freezeFailedReceipts, strings.Join(problems, "; "))
}

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
