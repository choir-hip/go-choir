package capsule

import (
	"errors"
	"fmt"
)

// grantedExecutionBindReason names the first field that prevents freeze/grant
// from certifying an execution receipt against the exact frozen assignment.
// Empty means the receipt binds. Only authenticity binds per receipt: the
// run, capability handle, capsule, and frozen source snapshot. The exit code
// is the recorded observation of that command, and an intermediate receipt's
// WorktreeDigest is its own post-evaluation subject, not the final frozen
// one — a multi-cell assignment's earlier cells necessarily recorded earlier
// tree states. The final subject is certified once, by the chronologically
// latest receipt, at the resolver.
func grantedExecutionBindReason(receipt ExecutionReceipt, agentRunID, handleDigest, capsuleID, sourceDigest string) string {
	switch {
	case receipt.AgentRunID != agentRunID:
		return "run"
	case receipt.CapabilityHandleDigest != handleDigest:
		return "handle"
	case receipt.CapsuleID != capsuleID:
		return "capsule"
	case receipt.SourceTreeDigest != sourceDigest:
		return "frozen source"
	default:
		return ""
	}
}

// SubjectBinding says what a set of cited execution receipts must prove about
// the frozen final subject before the executor grants them
// (docs/problems/engineering-completion-path-cluster-2026-10-20.md).
type SubjectBinding string

const (
	// SubjectCertified is the default and the only binding that supports a
	// claim about the subject (a completed fate, choir.Freeze): the
	// chronologically latest cited receipt must observe the frozen digest.
	SubjectCertified SubjectBinding = ""
	// SubjectObserved is for a fate that claims nothing about the subject (a
	// failed or blocked report): every receipt still binds the run, handle,
	// capsule and frozen source, and the frozen digest and the latest
	// receipt's worktree digest are recorded side by side, not compared.
	SubjectObserved SubjectBinding = "observed"
)

// ErrFinalSubjectNotCertified is the certified binding's refusal: the
// chronologically latest cited receipt did not observe the tree it is bound
// against. Against a frozen tree it is deterministic (the tree cannot change
// and receipts are content-addressed), so a caller can tell it from a
// transient failure: the terminal saga ends the assignment on it rather
// than re-driving a proposition that can never bind.
var ErrFinalSubjectNotCertified = errors.New("capsule execution evidence does not bind the final frozen subject")

// boundGrantedReceipts is what bindGrantedReceipts established: the receipts
// in citation order and the worktree digest the chronologically latest of
// them observed.
type boundGrantedReceipts struct {
	Receipts             []ExecutionReceipt
	LatestWorktreeDigest string
}

// bindGrantedReceipts opens and binds every cited receipt (run, handle,
// capsule, frozen source) and applies the final-subject rule of binding. It
// mints and persists nothing, so a refusal leaves no artifact behind.
// Duplicate refs collapse. The latest receipt is the first of the greatest
// OccurredAt, as the resolver always chose it.
func bindGrantedReceipts(open func(ref string) (ExecutionReceipt, error), refs []string, binding SubjectBinding, agentRunID, handleDigest, capsuleID, sourceDigest, frozenDigest string) (boundGrantedReceipts, error) {
	if binding != SubjectCertified && binding != SubjectObserved {
		return boundGrantedReceipts{}, fmt.Errorf("capsule execution evidence: unknown subject binding %q", string(binding))
	}
	out := boundGrantedReceipts{Receipts: make([]ExecutionReceipt, 0, len(refs))}
	latestOccurrence := ""
	seen := make(map[string]struct{}, len(refs))
	for _, ref := range refs {
		if _, duplicate := seen[ref]; duplicate {
			continue
		}
		receipt, err := open(ref)
		if err != nil {
			return boundGrantedReceipts{}, fmt.Errorf("capsule execution evidence does not bind the exact run, handle, capsule, and frozen source: %s: %w", ref, err)
		}
		if reason := grantedExecutionBindReason(receipt, agentRunID, handleDigest, capsuleID, sourceDigest); reason != "" {
			return boundGrantedReceipts{}, fmt.Errorf("capsule execution evidence does not bind the exact run, handle, capsule, and frozen source: %s", reason)
		}
		if receipt.OccurredAt > latestOccurrence {
			latestOccurrence, out.LatestWorktreeDigest = receipt.OccurredAt, receipt.WorktreeDigest
		}
		seen[ref] = struct{}{}
		out.Receipts = append(out.Receipts, receipt)
	}
	if binding == SubjectObserved {
		return out, nil
	}
	if len(out.Receipts) == 0 {
		return boundGrantedReceipts{}, fmt.Errorf("capsule execution evidence is required")
	}
	// The frozen final subject is certified once, by the chronologically
	// latest cited receipt: earlier receipts legitimately recorded earlier
	// post-evaluation tree states.
	if out.LatestWorktreeDigest != frozenDigest {
		return boundGrantedReceipts{}, fmt.Errorf("%w: latest receipt observed %s, frozen subject is %s", ErrFinalSubjectNotCertified, out.LatestWorktreeDigest, frozenDigest)
	}
	return out, nil
}
