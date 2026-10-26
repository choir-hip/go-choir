package agentcore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/sourceproj"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Rebase content verification (U4, docs/design/u4-stack-rebase-build-2026-10-26.md,
// question 7 and slice 6). A rebase round is an ordinary implementation
// assignment, so nothing in the bundle says it is one. The pair claim does:
// when an operation's assignment (freeze-open:<assignment>) is the one a claim
// names, its candidate is checked, before apply, to contain the applied change
// P on the booted base B'. It never runs for an ordinary operation, nor for a
// journalOnly crash-boot settlement. A refusal settles the operation failed
// (the computer keeps its release); a pass writes a deterministic lineage
// receipt and a fact, and the owner's apply record names the receipt's digest,
// so it reaches the master document.
//
// The same check runs once at Freeze, so a bad candidate fails before the
// owner sees it; the materializer's run is the authority and the only writer
// of the receipt. No bundle field is added (the materializer decodes with
// DisallowUnknownFields).

const (
	// rebaseContentMismatch is the first word of the refusal.
	rebaseContentMismatch = "rebase_content_mismatch"
	rebaseLineageSchema   = "choir.rebase_lineage.v1"
	// rebaseBootedBaseMoved: the computer booted a newer base after the claim,
	// so a candidate on B' is stale. This holds with or without the base join
	// (lane/base-join), which is the general check for every operation.
	rebaseBootedBaseMoved = "booted_base_moved"
	// FactRebased is the facts-log kind a passing rebase writes.
	factRebased = "source_projection_rebased"
)

// rebaseContentMismatchError is a refused rebase candidate.
type rebaseContentMismatchError struct {
	Verdict sourceproj.RebaseVerdict
}

func (e *rebaseContentMismatchError) Error() string {
	v := e.Verdict
	var b strings.Builder
	b.WriteString(rebaseContentMismatch + ": " + v.Reason)
	switch v.Reason {
	case sourceproj.RebaseCleanPathDiffers:
		b.WriteString(": paths of the applied change that apply cleanly to the new base are not in the candidate as they are: " + strings.Join(v.Differing, ", "))
	case sourceproj.RebaseHunkMissing:
		b.WriteString(": paths of the applied change that conflict with the new base lack a hunk of that change, or were deleted or edited elsewhere in the path: " + strings.Join(v.Dropped, ", "))
	case sourceproj.RebaseConflictUnresolved:
		b.WriteString(": paths of the applied change that conflict with the new base were left as the new base has them, which counts as dropped: " + strings.Join(v.Unresolved, ", "))
	case sourceproj.RebaseBaseMismatch:
		b.WriteString(": the candidate is not built on the booted base the rebase round was cast for")
	case sourceproj.RebaseCandidateDoesNotApply:
		b.WriteString(": the candidate's patch does not apply to the booted base")
	case rebaseBootedBaseMoved:
		b.WriteString(": the computer booted a newer base after this rebase was claimed; the next base gets its own notice")
	}
	return b.String()
}

// rebaseLineageReceipt is <root>/rebase/lineage/<operation id>.json. It holds
// no time, so a repeat verification writes the same bytes.
type rebaseLineageReceipt struct {
	Schema               string   `json:"schema"`
	RebaseOf             string   `json:"rebase_of"`
	NoticeID             string   `json:"notice_id"`
	Attempt              int      `json:"attempt"`
	StackBase            string   `json:"stack_base"`
	Base                 string   `json:"base"`
	PatchDigest          string   `json:"patch_sha256"`
	CandidatePatchDigest string   `json:"candidate_patch_sha256"`
	Clean                []string `json:"clean"`
	Resolved             []string `json:"resolved"`
	Extra                []string `json:"extra"`
	Verdict              string   `json:"verdict"`
}

func rebaseReceiptSHA256(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func rebaseReceiptPath(root, operationID string) string {
	return filepath.Join(root, "rebase", "lineage", operationID+".json")
}

// rebaseLineageOfAssignment is the lineage of assignmentID when the pair claim
// names it, or false (also when the assignment does not exist).
func (rt *Runtime) rebaseLineageOfAssignment(ctx context.Context, ownerID, computerID, assignmentID string) (rebaseLineage, types.EngineeringAssignment, bool, error) {
	assignment, err := rt.store.GetEngineeringAssignment(ctx, ownerID, computerID, assignmentID, 1)
	if errors.Is(err, store.ErrNotFound) {
		return rebaseLineage{}, types.EngineeringAssignment{}, false, nil
	}
	if err != nil {
		return rebaseLineage{}, types.EngineeringAssignment{}, false, err
	}
	lineage, ok := rt.rebaseLineage(ctx, assignment)
	return lineage, assignment, ok, nil
}

// checkRebaseCandidate runs the content check of a candidate patch built on
// candidateBase against the applied change lineage names.
func (rt *Runtime) checkRebaseCandidate(ctx context.Context, lineage rebaseLineage, candidateBase string, candidate []byte) (sourceproj.RebaseVerdict, string, []byte, error) {
	if booted := rt.bootedBaseCommit(); booted == "" || booted != lineage.BootedBase {
		return sourceproj.RebaseVerdict{Reason: rebaseBootedBaseMoved}, "", nil, nil
	}
	stackBase, patch, err := rt.rebaseOfferDeps().SourcePatch(ctx, lineage.OperationID)
	if err != nil {
		return sourceproj.RebaseVerdict{}, "", nil, fmt.Errorf("read the applied change's patch: %w", err)
	}
	verdict, err := sourceproj.VerifyRebase(ctx, rt.releaseRepo(), sourceproj.RebaseCheck{
		StackBase: stackBase, BootedBase: lineage.BootedBase, Patch: patch, CandidateBase: candidateBase, Candidate: candidate,
	})
	return verdict, stackBase, patch, err
}

// rebaseContentGate is the materializer's call: nothing for a journalOnly
// settlement, else the check of an operation a claim names. A refusal is a
// *rebaseContentMismatchError; any other error is transient.
func (rt *Runtime) rebaseContentGate(ctx context.Context, operation selfdev.Operation, journalOnly bool) error {
	if journalOnly {
		return nil
	}
	_, err := rt.verifyRebaseContentForOperation(ctx, operation)
	return err
}

// verifyRebaseContentForOperation checks the operation's bound bundle when the
// operation is a claimed rebase round's. checked is false for any other.
func (rt *Runtime) verifyRebaseContentForOperation(ctx context.Context, operation selfdev.Operation) (checked bool, err error) {
	assignmentID, ok := strings.CutPrefix(operation.IdempotencyKey, "freeze-open:")
	if !ok || strings.TrimSpace(assignmentID) == "" || rt == nil || rt.store == nil {
		return false, nil
	}
	ownerID, computerID := rt.rebaseScope()
	if ownerID == "" || computerID == "" {
		return false, nil
	}
	lineage, _, isRebase, err := rt.rebaseLineageOfAssignment(ctx, ownerID, computerID, assignmentID)
	if err != nil || !isRebase {
		return false, err
	}
	candidateBase, candidate, err := rt.boundSourcePatch(operation)
	switch {
	case errors.Is(err, errNoSourcePatch):
		return true, &rebaseContentMismatchError{Verdict: sourceproj.RebaseVerdict{Reason: sourceproj.RebaseCandidateDoesNotApply}}
	case err != nil:
		return true, fmt.Errorf("read the candidate's patch: %w", err)
	}
	verdict, stackBase, patch, err := rt.checkRebaseCandidate(ctx, lineage, candidateBase, candidate)
	if err != nil {
		return true, err
	}
	if !verdict.Pass {
		return true, &rebaseContentMismatchError{Verdict: verdict}
	}
	receipt := rebaseLineageReceipt{
		Schema: rebaseLineageSchema, RebaseOf: lineage.OperationID, NoticeID: lineage.NoticeID, Attempt: lineage.Attempt,
		StackBase: stackBase, Base: lineage.BootedBase, PatchDigest: sourceproj.SHA256(patch), CandidatePatchDigest: sourceproj.SHA256(candidate),
		Clean: nonNil(verdict.Clean), Resolved: nonNil(verdict.Resolved), Extra: nonNil(verdict.Extra), Verdict: "pass",
	}
	raw, err := json.Marshal(receipt)
	if err != nil {
		return true, err
	}
	if err := writeRebaseReceipt(rt.selfdevUpdaterRoot, operation.OperationID, raw); err != nil {
		return true, fmt.Errorf("write the lineage receipt: %w", err)
	}
	rt.appendSourceProjectionFactRecord(sourceProjectionFact{
		At: time.Now().UTC().Format(time.RFC3339Nano), Kind: factRebased, Trigger: "apply", State: "pass",
		OperationID: operation.OperationID, Base: stackBase, BootedBase: lineage.BootedBase, RebaseOf: lineage.OperationID, ReceiptSHA256: rebaseReceiptSHA256(raw),
	})
	return true, nil
}

func nonNil(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}

func writeRebaseReceipt(root, operationID string, raw []byte) error {
	if strings.TrimSpace(root) == "" {
		return errors.New("no updater root")
	}
	path := rebaseReceiptPath(root, operationID)
	if existing, err := os.ReadFile(path); err == nil && string(existing) == string(raw) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".receipt-")
	if err != nil {
		return err
	}
	_, writeErr := tmp.Write(raw)
	closeErr := tmp.Close()
	if writeErr != nil || closeErr != nil || os.Chmod(tmp.Name(), 0o444) != nil || os.Rename(tmp.Name(), path) != nil {
		_ = os.Remove(tmp.Name())
		return errors.New("the lineage receipt could not be written")
	}
	return nil
}

// rebaseReceiptSentence is what the owner's apply record adds for a rebase
// operation: the receipt's digest, so it reaches the master document. Empty
// for any other operation.
func (rt *Runtime) rebaseReceiptSentence(operationID string) string {
	if rt == nil || strings.TrimSpace(rt.selfdevUpdaterRoot) == "" {
		return ""
	}
	raw, err := os.ReadFile(rebaseReceiptPath(rt.selfdevUpdaterRoot, operationID))
	if err != nil {
		return ""
	}
	var receipt rebaseLineageReceipt
	if json.Unmarshal(raw, &receipt) != nil || receipt.Schema != rebaseLineageSchema {
		return ""
	}
	return rebaseReceiptText(receipt, rebaseReceiptSHA256(raw))
}

// rebaseReceiptText is the sentence the apply record carries: the change
// rebased, the receipt's digest, how many paths applied unchanged and which
// conflicts were resolved (bounded).
func rebaseReceiptText(receipt rebaseLineageReceipt, sha string) string {
	var b strings.Builder
	fmt.Fprintf(&b, " This candidate is a rebase of applied change %s onto base %s; its lineage receipt (sha256 %s) records that it still contains that change", receipt.RebaseOf, carriedBaseLabel(receipt.Base), sha)
	if n := len(receipt.Clean); n > 0 {
		noun := "paths"
		if n == 1 {
			noun = "path"
		}
		fmt.Fprintf(&b, ": %d %s applied unchanged", n, noun)
	}
	if len(receipt.Resolved) > 0 {
		const limit = 10
		shown := receipt.Resolved
		more := ""
		if len(shown) > limit {
			more = fmt.Sprintf(", and %d more", len(shown)-limit)
			shown = shown[:limit]
		}
		fmt.Fprintf(&b, "; resolved conflicts: %s%s", strings.Join(shown, ", "), more)
	}
	b.WriteString(".")
	return b.String()
}

// verifyRebaseAtFreeze is Freeze's call: for a run whose assignment a claim
// names, the staged cumulative patch must already contain the applied change.
// It writes nothing.
func (rt *Runtime) verifyRebaseAtFreeze(ctx context.Context, rec *types.RunRecord, staged *capsule.StagedRelease, patch []byte) error {
	if rt == nil || rt.store == nil || rec == nil || staged == nil {
		return nil
	}
	assignmentID := metadataStringValue(rec.Metadata, "assignment_id")
	if assignmentID == "" {
		return nil
	}
	lineage, _, isRebase, err := rt.rebaseLineageOfAssignment(ctx, rec.OwnerID, rec.ComputerID, assignmentID)
	if err != nil {
		return fmt.Errorf("check whether this assignment is a rebase: %w", err)
	}
	if !isRebase {
		return nil
	}
	verdict, _, _, err := rt.checkRebaseCandidate(ctx, lineage, staged.SourceBaseCommit, patch)
	if err != nil {
		return fmt.Errorf("Freeze refused: the rebase could not be checked: %w", err)
	}
	if !verdict.Pass {
		return &rebaseContentMismatchError{Verdict: verdict}
	}
	return nil
}
