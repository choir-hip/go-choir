package agentcore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The rebase offer (U4, docs/design/u4-stack-rebase-build-2026-10-26.md,
// question 6 and slice 4). A rebase round is an ordinary implementation
// assignment whose capsule starts from the booted base B'. Its engineer is
// offered the applied stack top's patch P (built on the stack base B) as an
// inert, digest-bound patch, the way a rejected candidate's patch is offered
// to a retry. An ordinary assignment still never carries an applied
// operation (roundCarriable): this branch is chosen only for an assignment
// whose pair claim names it, and its source is the applied operation's bound
// bundle, not the trajectory's history.
//
// A rebase offer mounts P even when it does not apply to B', and names the
// conflicting and the clean paths: the engineer's job is exactly to resolve
// them. Rebase offers are written under <root>/carried/rebase-offers/ with
// schema choir.rebase_patch_offer.v1, a path and schema old code never reads,
// so a full revert can never present P as an ordinary carry.

const rebaseOfferSchema = "choir.rebase_patch_offer.v1"

// rebaseLineage is what a rebase assignment rebases: the notice it answers
// and the pair (operation, booted base) the claim names.
type rebaseLineage struct {
	NoticeID    string
	OperationID string
	BootedBase  string
	Attempt     int
}

func rebaseOfferPath(root, assignmentID string, attempt uint64) string {
	return filepath.Join(root, "carried", "rebase-offers", url.PathEscape(assignmentID)+fmt.Sprintf("-%d.json", attempt))
}

// rebaseLineage derives an assignment's rebase lineage from the tape: the
// opening Cast's spec names the notice, the operation and the base; the notice
// id agrees with them; and the pair's claim names this assignment. Anything
// else is no lineage. Admission has already proved the spec agrees with the
// notice; the claim is what makes it this assignment's.
func (rt *Runtime) rebaseLineage(ctx context.Context, assignment types.EngineeringAssignment) (rebaseLineage, bool) {
	binding := assignment.Binding
	if rt == nil || rt.store == nil {
		return rebaseLineage{}, false
	}
	cast, err := rt.store.GetCommitmentRecord(ctx, binding.OwnerID, binding.ComputerID, binding.ParentControlID)
	if err != nil || cast == nil || cast.Directive == nil {
		return rebaseLineage{}, false
	}
	var spec engineeringCastSpec
	if json.Unmarshal([]byte(cast.Directive.Body), &spec) != nil {
		return rebaseLineage{}, false
	}
	operationID, base, attempt, ok := types.ParseStackRebaseNoticeID(spec.RebaseNotice)
	if !ok || strings.TrimSpace(spec.RebaseOf) != operationID || strings.TrimSpace(spec.Base) != base {
		return rebaseLineage{}, false
	}
	claim, err := rt.store.GetRebaseNoticeClaim(ctx, binding.OwnerID, binding.ComputerID, types.StackRebaseClaimKey(types.StackRebasePairKey(operationID, base), attempt))
	if err != nil || claim.AssignmentID != assignment.AssignmentID || claim.NoticeID != spec.RebaseNotice {
		return rebaseLineage{}, false
	}
	return rebaseLineage{NoticeID: spec.RebaseNotice, OperationID: operationID, BootedBase: base, Attempt: attempt}, true
}

type rebaseOfferDeps struct {
	// Deriver checks P against the rebase capsule's base.
	Deriver carriedPatchDeriver
	// Root is the updater root.
	Root string
	// SourcePatch reads the applied operation's bound source patch: its base
	// commit (B) and the bytes.
	SourcePatch func(ctx context.Context, operationID string) (base string, patch []byte, err error)
}

type rebaseOfferInput struct {
	OwnerID, ComputerID, AssignmentID string
	Attempt                           uint64
	// RetryBaseRef is the rebase capsule's own pinned source (B').
	RetryBaseRef string
	Lineage      rebaseLineage
}

func (rt *Runtime) rebaseOfferDeps() rebaseOfferDeps {
	return rebaseOfferDeps{
		Deriver: rt.capsuleExecutor, Root: rt.selfdevUpdaterRoot,
		SourcePatch: func(ctx context.Context, operationID string) (string, []byte, error) {
			if rt.selfdevOperations == nil {
				return "", nil, errors.New("the self-development operations store is unavailable")
			}
			operation, err := rt.selfdevOperations.Get(ctx, rt.cfg.ComputerID, operationID)
			if err != nil {
				return "", nil, fmt.Errorf("operation %s: %w", operationID, err)
			}
			if operation.State != selfdev.StateApplied {
				return "", nil, fmt.Errorf("operation %s is %s, not applied", operationID, operation.State)
			}
			return rt.boundSourcePatch(operation)
		},
	}
}

// offerRebasePatch decides, once, what the rebase round's engineer is told
// about the applied change, and stages P when it is offered. It never fails
// the round: a problem carrying P is a refusal the engineer reads, and the
// capsule starts clean from B'. A transient problem is refused and not
// recorded, so a replay decides again.
func offerRebasePatch(ctx context.Context, deps rebaseOfferDeps, in rebaseOfferInput) carriedPatchOffer {
	if existing := readRebaseOffer(deps.Root, in.AssignmentID, in.Attempt); existing.Status != "" {
		return existing
	}
	offer := carriedPatchOffer{
		Schema: rebaseOfferSchema, AssignmentID: in.AssignmentID, Attempt: in.Attempt, RetryBaseRef: in.RetryBaseRef, BaseMoved: true,
		RebaseOf: in.Lineage.OperationID, RebaseNotice: in.Lineage.NoticeID,
	}
	refuse := func(persist bool, format string, args ...any) carriedPatchOffer {
		offer.Status, offer.Reason = carriedRefused, fmt.Sprintf(format, args...)
		if persist {
			writeOfferFile(rebaseOfferPath(deps.Root, offer.AssignmentID, offer.Attempt), offer)
		}
		return offer
	}
	if deps.SourcePatch == nil || deps.Deriver == nil || strings.TrimSpace(deps.Root) == "" {
		return refuse(false, "the applied change's patch cannot be read on this computer")
	}
	base, patch, err := deps.SourcePatch(ctx, in.Lineage.OperationID)
	if err != nil {
		log.Printf("agentcore: rebase offer: read the patch of %s: %v", in.Lineage.OperationID, err)
		return refuse(false, "the applied change's patch could not be read (%v)", err)
	}
	offer.StackBase, offer.BaseRef = base, base
	sum := sha256.Sum256(patch)
	digest := hex.EncodeToString(sum[:])
	switch {
	case len(patch) == 0:
		return refuse(true, "the applied change carries no source patch")
	case len(patch) > capsule.CarriedPatchMaxBytes:
		return refuse(true, "the applied change's patch is too large to carry (%d bytes, bound %d)", len(patch), capsule.CarriedPatchMaxBytes)
	}
	files, err := capsule.CarriedPatchPaths(patch)
	if err != nil {
		return refuse(true, "the applied change's patch names a path outside the source tree (%v)", err)
	}
	carried := capsule.CarriedPatch{Patch: patch, SHA256: digest, Files: files}
	if err := deps.Deriver.CheckCarriedPatchApplies(ctx, carried, in.RetryBaseRef); err != nil {
		var conflict *capsule.CarriedPatchConflictError
		if !errors.As(err, &conflict) {
			log.Printf("agentcore: rebase offer: check %s against the new base: %v", in.Lineage.OperationID, err)
			return refuse(false, "the applied change's patch could not be checked against the new base (%v)", err)
		}
		// Mounted anyway: resolving these paths is the round's work.
		offer.ConflictPaths, offer.CleanPaths = conflict.Paths, conflict.Clean
	}
	facts := carriedPatchFacts{
		Schema: carriedFactsSchema, PatchSHA256: digest, Size: len(patch), BaseRef: base,
		CandidateID: "rebase:" + in.Lineage.OperationID, SourceAssignmentID: in.Lineage.OperationID, SourceResult: "applied", Files: files,
	}
	dir, err := stageCarriedPatch(deps.Root, patch, facts)
	if err != nil {
		log.Printf("agentcore: rebase offer: stage %s: %v", digest, err)
		return refuse(false, "the applied change's patch could not be staged (%v)", err)
	}
	offer.Status, offer.PatchSHA256, offer.Size, offer.Files, offer.Dir = carriedOffered, digest, len(patch), files, dir
	writeOfferFile(rebaseOfferPath(deps.Root, offer.AssignmentID, offer.Attempt), offer)
	return offer
}

// readRebaseOffer returns the recorded decision for a rebase assignment, with
// an offered patch re-verified: bytes that no longer match are a refusal,
// never a mount. Nothing is returned for an assignment that has none.
func readRebaseOffer(root, assignmentID string, attempt uint64) carriedPatchOffer {
	if strings.TrimSpace(root) == "" || strings.TrimSpace(assignmentID) == "" {
		return carriedPatchOffer{}
	}
	raw, err := os.ReadFile(rebaseOfferPath(root, assignmentID, attempt))
	if err != nil {
		return carriedPatchOffer{}
	}
	var offer carriedPatchOffer
	if json.Unmarshal(raw, &offer) != nil || offer.Schema != rebaseOfferSchema || offer.AssignmentID != assignmentID || offer.Attempt != attempt {
		return carriedPatchOffer{}
	}
	if offer.Status != carriedOffered {
		offer.Dir = ""
		return offer
	}
	dir := filepath.Join(root, "carried", offer.PatchSHA256)
	if err := verifyStagedCarriedPatch(dir, offer.PatchSHA256); err != nil {
		offer.Status = carriedRefused
		offer.Reason = fmt.Sprintf("the staged patch (sha256 %s) no longer matches its digest and was not mounted (%v)", offer.PatchSHA256, err)
		offer.Dir = ""
		return offer
	}
	offer.Dir = dir
	return offer
}

// rebasePatchNotice is what the rebase round's engineer reads in its run
// context. The patch is described as untrusted; nothing here says it passed
// anything.
func rebasePatchNotice(offer carriedPatchOffer) string {
	var b strings.Builder
	switch offer.Status {
	case carriedOffered:
		fmt.Fprintf(&b, "Rebase request (notice %s): this computer's applied self-development change %s is a source patch, sha256 %s, %d bytes, changing: %s. It was built on platform base %s; the computer now runs a newer base, and this capsule's /workspace/platform is that newer base (%s) with nothing applied.",
			offer.RebaseNotice, offer.RebaseOf, offer.PatchSHA256, offer.Size, carriedFileList(offer.Files), carriedBaseLabel(offer.StackBase), carriedBaseLabel(offer.RetryBaseRef))
		b.WriteString(" Your task is to rebase that change, as it is, onto this base: reproduce what the patch does on this tree. Add no new work.")
		b.WriteString(" The patch is mounted read only at /selfdev/carried/source.patch (its binding is /selfdev/carried/carried.json). It is not trusted and carries no test results.")
		if len(offer.ConflictPaths) > 0 {
			b.WriteString(" The files that conflict with the new base: " + carriedFileList(offer.ConflictPaths) + ".")
			if len(offer.CleanPaths) > 0 {
				b.WriteString(" The rest still applies: " + carriedFileList(offer.CleanPaths) + ".")
			}
			b.WriteString(" Apply what applies from /workspace/platform with `git apply --reject /selfdev/carried/source.patch`, then resolve each rejected hunk by hand so the change's intent stands on top of the new base's code; a conflict path you leave as the new base has it counts as dropped.")
		} else {
			b.WriteString(" It applies to this base: apply it from /workspace/platform with `git apply /selfdev/carried/source.patch`.")
		}
		b.WriteString(" Re-run the build, tests and checks in this capsule after your last write; Freeze judges only this capsule's own diff, and when the candidate is applied the platform checks that it still contains this change.")
	default:
		fmt.Fprintf(&b, "Rebase request (notice %s): the applied change %s is not mounted in this capsule: %s. /workspace/platform is the newer platform base (%s) with nothing applied; reproduce the change from the objective.",
			offer.RebaseNotice, offer.RebaseOf, offer.Reason, carriedBaseLabel(offer.RetryBaseRef))
	}
	return b.String()
}
