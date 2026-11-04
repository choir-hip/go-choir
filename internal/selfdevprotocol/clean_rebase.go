package selfdevprotocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// Clean rebase (owner ruling 2026-11-04, recorded in
// docs/design/u4-stack-rebase-build-2026-10-26.md, "Owner ruling"): "I want
// all this, and clean rebases without approval." When the applied
// self-development stack sits on a base the computer no longer booted, and
// the applied change's source patch applies, as the same bytes, to the
// booted base, the runtime re-applies it without an owner act: a new
// operation whose bundle is the approved bundle with only its base fields
// changed, host built (the frontend build with the package check, and the Go
// build) before the decision. The decision is an effect_accepted whose
// authority names a CleanRebaseReceipt; corpusd admits it only when the
// receipt, the two bundles on the tape, the prior decision and the host
// build all agree. A conflict, or a build that fails, goes to engineering and
// the owner as before (the rebase notice).

const (
	// CleanRebaseReceiptSchema names the receipt's shape.
	CleanRebaseReceiptSchema = "choir.clean_rebase_receipt.v1"
	// CleanRebaseReceiptMediaType is the receipt payload's media type on the
	// decision event (input artifact 1; input artifact 0 is the mode receipt).
	CleanRebaseReceiptMediaType = "application/vnd.choir.clean-rebase-receipt+json"
	// CleanRebaseAuthorityPrefix + the receipt digest is the decision event's
	// AuthorityRef.
	CleanRebaseAuthorityPrefix = "platform-clean-rebase:"
	// CleanRebaseRuling is the ruling the receipt cites.
	CleanRebaseRuling = "owner 2026-11-04: clean rebases apply without approval"
)

// CleanRebaseBundleFields are the top-level bundle fields a clean
// re-application may change: the identity of the new operation (its base
// event head, trajectory, capsule), the patch's base commit, the verifier
// receipt the new operation's verification adds, the recomputed content
// digest, and the provenance of a capsule binary that is not a release input
// on the host-built path. Every other field, the patch digest among them, is
// the approved bundle's, byte for byte.
var CleanRebaseBundleFields = []string{
	"base_event_head", "trajectory_ref", "capsule_identity",
	"source_patch_base_commit", "verifier_receipts", "content_digest", "release_binary",
}

// CleanRebaseReceipt is the record of one clean re-application. It holds no
// time, so the same attempt always has the same digest.
type CleanRebaseReceipt struct {
	Schema     string `json:"schema"`
	ComputerID string `json:"computer_id"`
	// OperationID and BundleDigest are the new operation and its finalized
	// bundle (the decision event's ParentEventID and ProposedEffectRef).
	OperationID  string `json:"operation_id"`
	BundleDigest string `json:"bundle_digest"`
	// RebaseOf, PriorBundleDigest and PriorDecisionEvent are the applied
	// operation, its finalized bundle, and the digest of the effect_accepted
	// event that approved it.
	RebaseOf           string `json:"rebase_of"`
	PriorBundleDigest  string `json:"prior_bundle_digest"`
	PriorDecisionEvent string `json:"prior_decision_event"`
	// SourcePatchSHA256 is the patch both bundles carry; StackBase the base
	// it was approved on; BootedBase the base it is re-applied to.
	SourcePatchSHA256 string `json:"source_patch_sha256"`
	StackBase         string `json:"stack_base"`
	BootedBase        string `json:"booted_base"`
	// HostBuildJobID and HostBuildResultDigest name the host build of the
	// new bundle that finished before the decision.
	HostBuildJobID        string `json:"host_build_job_id"`
	HostBuildResultDigest string `json:"host_build_result_digest"`
	Ruling                string `json:"ruling"`
	Statement             string `json:"statement"`
}

// CleanRebaseStatement is the receipt's one-line statement.
func CleanRebaseStatement(rebaseOf, bootedBase string) string {
	return fmt.Sprintf("clean re-application of %s onto %s, applied without an owner act under the owner's 2026-11-04 ruling", rebaseOf, bootedBase)
}

// Validate checks the receipt is complete.
func (r CleanRebaseReceipt) Validate() error {
	switch {
	case r.Schema != CleanRebaseReceiptSchema:
		return fmt.Errorf("clean rebase receipt: schema %q", r.Schema)
	case strings.TrimSpace(r.ComputerID) == "" || strings.TrimSpace(r.OperationID) == "" || strings.TrimSpace(r.RebaseOf) == "":
		return errors.New("clean rebase receipt: computer, operation and rebase_of are required")
	case r.OperationID == r.RebaseOf:
		return errors.New("clean rebase receipt: an operation cannot re-apply itself")
	case !computerevent.IsSHA256(r.BundleDigest) || !computerevent.IsSHA256(r.PriorBundleDigest) || !computerevent.IsSHA256(r.PriorDecisionEvent):
		return errors.New("clean rebase receipt: bundle and decision digests are required")
	case !computerevent.IsSHA256(r.SourcePatchSHA256):
		return errors.New("clean rebase receipt: the source patch digest is required")
	case !isGitCommit(r.StackBase) || !isGitCommit(r.BootedBase) || r.StackBase == r.BootedBase:
		return errors.New("clean rebase receipt: two different base commits are required")
	case !computerevent.IsSHA256(r.HostBuildJobID) || !computerevent.IsSHA256(r.HostBuildResultDigest):
		return errors.New("clean rebase receipt: the host build job and result are required")
	case r.Ruling != CleanRebaseRuling || r.Statement != CleanRebaseStatement(r.RebaseOf, r.BootedBase):
		return errors.New("clean rebase receipt: the ruling and statement are fixed")
	}
	return nil
}

// Canonical returns the receipt's canonical bytes and digest.
func (r CleanRebaseReceipt) Canonical() ([]byte, string, error) {
	if err := r.Validate(); err != nil {
		return nil, "", err
	}
	raw, err := computerevent.CanonicalJSON(r)
	if err != nil {
		return nil, "", err
	}
	return raw, computerevent.DigestBytes(raw), nil
}

// DecodeCleanRebaseReceipt decodes and validates receipt bytes strictly.
func DecodeCleanRebaseReceipt(raw []byte) (CleanRebaseReceipt, error) {
	var receipt CleanRebaseReceipt
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&receipt); err != nil {
		return CleanRebaseReceipt{}, fmt.Errorf("clean rebase receipt: %w", err)
	}
	return receipt, receipt.Validate()
}

// CleanRebaseAuthority is the decision event's AuthorityRef for a receipt
// digest.
func CleanRebaseAuthority(receiptDigest string) string {
	return CleanRebaseAuthorityPrefix + receiptDigest
}

// CleanRebaseReceiptDigestFromAuthority parses a clean rebase AuthorityRef.
func CleanRebaseReceiptDigestFromAuthority(authority string) (string, bool) {
	digest, ok := strings.CutPrefix(strings.TrimSpace(authority), CleanRebaseAuthorityPrefix)
	return digest, ok && computerevent.IsSHA256(digest)
}

// CleanRebaseBundleFacts is what CompareCleanRebaseBundles read from the two
// bundles.
type CleanRebaseBundleFacts struct {
	SourcePatchSHA256 string
	PriorBase         string
	NextBase          string
}

// CompareCleanRebaseBundles checks that next is prior with only
// CleanRebaseBundleFields changed, that both carry the same non-empty source
// patch, and that the patch's base moved. It reads the bundles as JSON
// objects, so corpusd needs no bundle type; a field either side has that the
// other lacks is a difference.
func CompareCleanRebaseBundles(prior, next []byte) (CleanRebaseBundleFacts, error) {
	decode := func(name string, raw []byte) (map[string]json.RawMessage, error) {
		var fields map[string]json.RawMessage
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		if err := decoder.Decode(&fields); err != nil || fields == nil {
			return nil, fmt.Errorf("clean rebase: the %s bundle is not a JSON object", name)
		}
		return fields, nil
	}
	priorFields, err := decode("prior", prior)
	if err != nil {
		return CleanRebaseBundleFacts{}, err
	}
	nextFields, err := decode("next", next)
	if err != nil {
		return CleanRebaseBundleFacts{}, err
	}
	str := func(fields map[string]json.RawMessage, key string) string {
		var value string
		if raw, ok := fields[key]; ok {
			_ = json.Unmarshal(raw, &value)
		}
		return strings.TrimSpace(value)
	}
	facts := CleanRebaseBundleFacts{
		SourcePatchSHA256: str(nextFields, "source_patch_sha256"),
		PriorBase:         str(priorFields, "source_patch_base_commit"),
		NextBase:          str(nextFields, "source_patch_base_commit"),
	}
	switch {
	case !computerevent.IsSHA256(facts.SourcePatchSHA256) || str(priorFields, "source_patch_sha256") != facts.SourcePatchSHA256:
		return facts, errors.New("clean rebase: the two bundles do not carry the same source patch")
	case !isGitCommit(facts.PriorBase) || !isGitCommit(facts.NextBase) || facts.PriorBase == facts.NextBase:
		return facts, errors.New("clean rebase: the patch's base did not move to another commit")
	}
	changeable := make(map[string]bool, len(CleanRebaseBundleFields))
	for _, key := range CleanRebaseBundleFields {
		changeable[key] = true
	}
	canonical := func(raw json.RawMessage) (string, error) {
		var value any
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		if err := decoder.Decode(&value); err != nil {
			return "", err
		}
		out, err := computerevent.CanonicalJSON(value)
		return string(out), err
	}
	for key, priorRaw := range priorFields {
		if changeable[key] {
			continue
		}
		nextRaw, ok := nextFields[key]
		if !ok {
			return facts, fmt.Errorf("clean rebase: the next bundle drops field %q", key)
		}
		a, errA := canonical(priorRaw)
		b, errB := canonical(nextRaw)
		if errA != nil || errB != nil || a != b {
			return facts, fmt.Errorf("clean rebase: field %q differs from the approved bundle", key)
		}
	}
	for key := range nextFields {
		if _, ok := priorFields[key]; !ok && !changeable[key] {
			return facts, fmt.Errorf("clean rebase: the next bundle adds field %q", key)
		}
	}
	return facts, nil
}

func isGitCommit(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}
