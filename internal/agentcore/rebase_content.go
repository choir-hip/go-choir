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
	// rebaseVerdictSuperseded is the receipt verdict (and fact state) of a
	// rebase that drops the applied change because the new base supersedes it
	// (docs/problems/noop-rebase-never-clears-base-moved-2026-11-04.md).
	rebaseVerdictSuperseded = "superseded"
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
	// Upstreamed are the hunks of the applied change the candidate lacks
	// because the new base already carries them (problem 14).
	Upstreamed []sourceproj.UpstreamedHunk `json:"upstreamed,omitempty"`
	// Superseded names, on a superseded verdict, the paths of the applied
	// change where the new base's own version replaces it.
	Superseded []string `json:"superseded,omitempty"`
	Verdict    string   `json:"verdict"`
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
	// An empty candidate is the round's claim that the new base supersedes the
	// applied change; it is checked as such, never as "does not apply".
	verdict, err := sourceproj.VerifyRebase(ctx, rt.releaseRepo(), sourceproj.RebaseCheck{
		StackBase: stackBase, BootedBase: lineage.BootedBase, Patch: patch, CandidateBase: candidateBase, Candidate: candidate,
		Superseded: len(candidate) == 0,
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
	assignmentID, ok := store.EngineeringFreezeAssignmentOf(operation.IdempotencyKey)
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
		// A superseded candidate: no source change, so its base is the
		// commit its reused binary was bound to, which is the capsule's
		// pinned source (B' for a rebase round). Anything else has no
		// provable base and is refused base_mismatch.
		candidateBase, candidate = rt.supersededCandidateBase(operation), nil
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
		Clean: nonNil(verdict.Clean), Resolved: nonNil(verdict.Resolved), Extra: nonNil(verdict.Extra), Upstreamed: verdict.Upstreamed, Verdict: "pass",
	}
	if verdict.Superseded {
		receipt.Verdict, receipt.Superseded = rebaseVerdictSuperseded, nonNil(verdict.Unresolved)
	}
	raw, err := json.Marshal(receipt)
	if err != nil {
		return true, err
	}
	if err := writeRebaseReceipt(rt.selfdevUpdaterRoot, operation.OperationID, raw); err != nil {
		return true, fmt.Errorf("write the lineage receipt: %w", err)
	}
	rt.appendSourceProjectionFactRecord(sourceProjectionFact{
		At: time.Now().UTC().Format(time.RFC3339Nano), Kind: factRebased, Trigger: "apply", State: receipt.Verdict,
		OperationID: operation.OperationID, Base: stackBase, BootedBase: lineage.BootedBase, RebaseOf: lineage.OperationID, ReceiptSHA256: rebaseReceiptSHA256(raw),
		Upstreamed: verdict.Upstreamed,
	})
	return true, nil
}

// supersededCandidateBase is the base a candidate with no source patch was
// built on: the source commit its reused running binary is bound to (Freeze
// records SourceArtifactCommit of the capsule's pinned source there, inside
// the bundle digest). "" when the bundle is unreadable or its binary is not
// a reused one.
func (rt *Runtime) supersededCandidateBase(operation selfdev.Operation) string {
	if rt == nil || strings.TrimSpace(rt.selfdevUpdaterRoot) == "" {
		return ""
	}
	bundle, _, err := readBoundSelfDevelopmentBundle(filepath.Join(rt.selfdevUpdaterRoot, "incoming", operation.BundleDigest), operation)
	if err != nil {
		return ""
	}
	return supersededBaseOf(bundle.ReleaseBinary)
}

func supersededBaseOf(binary *capsule.ReleaseBinaryProvenance) string {
	if binary == nil || binary.Origin != capsule.ReleaseBinaryReusedRunning {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(binary.SourceBaseCommit))
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
	if receipt.Verdict == rebaseVerdictSuperseded {
		fmt.Fprintf(&b, " This candidate drops applied change %s: rebased onto base %s, it changes no source, because the new base supersedes the change; its lineage receipt (sha256 %s) records that the new base replaced every path of it with its own version", receipt.RebaseOf, carriedBaseLabel(receipt.Base), sha)
		if text := boundedRecordPaths(receipt.Superseded); text != "" {
			b.WriteString(": " + text)
		}
		fmt.Fprintf(&b, ". Approving it gives up %s; a rollback to %s restores it on its old base.", receipt.RebaseOf, receipt.RebaseOf)
		return b.String()
	}
	fmt.Fprintf(&b, " This candidate is a rebase of applied change %s onto base %s; its lineage receipt (sha256 %s) records that it still contains that change", receipt.RebaseOf, carriedBaseLabel(receipt.Base), sha)
	if n := len(receipt.Clean); n > 0 {
		noun := "paths"
		if n == 1 {
			noun = "path"
		}
		fmt.Fprintf(&b, ": %d %s applied unchanged", n, noun)
	}
	if len(receipt.Resolved) > 0 {
		fmt.Fprintf(&b, "; resolved conflicts: %s", boundedRecordPaths(receipt.Resolved))
	}
	if text := upstreamedText(receipt.Upstreamed); text != "" {
		b.WriteString("; dropped as upstreamed (lines the new base added carry their tokens): " + text)
	}
	b.WriteString(".")
	return b.String()
}

// boundedRecordPaths lists paths for a record. The paths are text a guest
// influences: escaped (a path can hold a newline) and bounded in bytes, not
// only in count (review 328).
func boundedRecordPaths(paths []string) string {
	const limit, perPath, total = 10, 160, 1200
	var shown []string
	size := 0
	for _, path := range paths {
		if len(shown) == limit {
			break
		}
		path = escapeRecordText(path, perPath)
		if size+len(path) > total {
			break
		}
		shown = append(shown, path)
		size += len(path) + 2
	}
	if len(shown) == 0 {
		return ""
	}
	more := ""
	if n := len(paths) - len(shown); n > 0 {
		more = fmt.Sprintf(", and %d more", n)
	}
	return strings.Join(shown, ", ") + more
}

// upstreamedText names upstreamed hunks for a record: path and hunk header,
// escaped (the path is guest-influenced) and bounded like resolved paths.
func upstreamedText(hunks []sourceproj.UpstreamedHunk) string {
	const limit, perHunk, total = 10, 200, 1200
	var shown []string
	size := 0
	for _, h := range hunks {
		if len(shown) == limit {
			break
		}
		item := escapeRecordText(h.Path+" "+h.Hunk, perHunk)
		if size+len(item) > total {
			break
		}
		shown = append(shown, item)
		size += len(item) + 2
	}
	if len(shown) == 0 {
		return ""
	}
	more := ""
	if n := len(hunks) - len(shown); n > 0 {
		more = fmt.Sprintf(", and %d more", n)
	}
	return strings.Join(shown, ", ") + more
}

// rebaseFreezeVerdict is what Freeze learns about a rebase round's
// candidate. Rebase is false for an assignment no pair claim names.
type rebaseFreezeVerdict struct {
	Rebase bool
	// Upstreamed are the hunks the candidate dropped as already in the new
	// base (problem 14).
	Upstreamed []sourceproj.UpstreamedHunk
	// Superseded names, for an empty candidate that passed, the paths where
	// the new base's version replaces the applied change.
	Superseded []string
}

// verifyRebaseAtFreeze is Freeze's call: for a run whose assignment a claim
// names, the staged cumulative patch must already contain the applied change,
// or, when it is empty, the new base must supersede the change on every path.
// It writes nothing.
func (rt *Runtime) verifyRebaseAtFreeze(ctx context.Context, rec *types.RunRecord, staged *capsule.StagedRelease, patch []byte) (rebaseFreezeVerdict, error) {
	if rt == nil || rt.store == nil || rec == nil || staged == nil {
		return rebaseFreezeVerdict{}, nil
	}
	assignmentID := metadataStringValue(rec.Metadata, "assignment_id")
	if assignmentID == "" {
		return rebaseFreezeVerdict{}, nil
	}
	lineage, _, isRebase, err := rt.rebaseLineageOfAssignment(ctx, rec.OwnerID, rec.ComputerID, assignmentID)
	if err != nil {
		return rebaseFreezeVerdict{}, fmt.Errorf("check whether this assignment is a rebase: %w", err)
	}
	if !isRebase {
		return rebaseFreezeVerdict{}, nil
	}
	candidateBase := staged.SourceBaseCommit
	if len(patch) == 0 {
		candidateBase = supersededBaseOf(staged.Binary)
	}
	verdict, _, _, err := rt.checkRebaseCandidate(ctx, lineage, candidateBase, patch)
	if err != nil {
		return rebaseFreezeVerdict{Rebase: true}, fmt.Errorf("Freeze refused: the rebase could not be checked: %w", err)
	}
	if !verdict.Pass {
		mismatch := &rebaseContentMismatchError{Verdict: verdict}
		if len(patch) == 0 {
			return rebaseFreezeVerdict{Rebase: true}, fmt.Errorf("Freeze refused: this rebase capsule changed no /workspace/platform source, which drops applied change %s; that is admitted only when the new base supersedes every path of the change, and it does not: %w", lineage.OperationID, mismatch)
		}
		return rebaseFreezeVerdict{Rebase: true}, mismatch
	}
	out := rebaseFreezeVerdict{Rebase: true, Upstreamed: verdict.Upstreamed}
	if verdict.Superseded {
		out.Superseded = nonNil(verdict.Unresolved)
	}
	return out, nil
}

// escapeRecordText makes guest-influenced text safe for a one-line record:
// control characters become visible escapes and the text is cut to max bytes
// on a rune boundary.
func escapeRecordText(text string, max int) string {
	var b strings.Builder
	for _, r := range text {
		var piece string
		switch {
		case r == '\n':
			piece = `\n`
		case r == '\r':
			piece = `\r`
		case r == '\t':
			piece = `\t`
		case r < 0x20 || (r >= 0x7f && r <= 0x9f):
			piece = fmt.Sprintf(`\x%02x`, r)
		default:
			piece = string(r)
		}
		if b.Len()+len(piece) > max-3 {
			b.WriteString("...")
			break
		}
		b.WriteString(piece)
	}
	return b.String()
}
