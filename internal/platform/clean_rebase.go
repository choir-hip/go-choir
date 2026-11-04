package platform

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/computertiers"
	"github.com/yusefmosiah/go-choir/internal/hostbuild"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

// Clean rebase admission (owner ruling 2026-11-04: "clean rebases without
// approval"; contract in internal/selfdevprotocol/clean_rebase.go). An
// effect_accepted whose authority is platform-clean-rebase:<receipt digest>
// is admitted only when the receipt, the prior decision, the two bundles on
// the tape and the host build all agree. The branch always enforces: observe
// would admit exactly the guest-forged acceptance it exists to stop.
//
// No consumption row is written: the invariant is content equality, so a
// replay of the same receipt admits the same content.

// TapeObjectReader reads one object of a computer's tape by digest.
type TapeObjectReader func(computerID string, ns tapestore.NS, digest string) ([]byte, error)

// cleanRebaseMaxChain bounds how many clean rebases a chain may stack before
// it must bottom out in an owner or consensus decision.
const cleanRebaseMaxChain = 32

// IsCleanRebaseAuthority reports an effect_accepted authority that names a
// clean rebase receipt.
func IsCleanRebaseAuthority(authorityRef string) bool {
	return strings.HasPrefix(strings.TrimSpace(authorityRef), selfdevprotocol.CleanRebaseAuthorityPrefix)
}

// ConfigureCleanRebase gives the gate the tape and the host build jobs the
// clean rebase branch reads. Without either, every clean rebase is refused.
func (a *OwnerActAuthority) ConfigureCleanRebase(tape TapeObjectReader, jobs HostBuildJobs) {
	if a == nil {
		return
	}
	a.cleanRebaseTape = tape
	a.cleanRebaseJobs = jobs
}

// SetCleanRebaseTier replaces the computer tier the clean rebase branch reads
// (the embedded tier file); a fixture computer is in no tier file.
func (a *OwnerActAuthority) SetCleanRebaseTier(tier func(computerID string) (test, platform bool, err error)) {
	if a != nil {
		a.cleanRebaseTier = tier
	}
}

// ReadTapeObject reads one tape object through the segment-first reader.
func (s *Service) ReadTapeObject(computerID string, ns tapestore.NS, digest string) ([]byte, error) {
	body, _, err := s.tapeReader().Read(computerID, ns, digest)
	return body, err
}

// computerTierForCleanRebase reads the embedded tier file. An unreadable
// file is an error, which refuses (fail closed).
func computerTierForCleanRebase(computerID string) (test, platform bool, err error) {
	platformTiersOnce.Do(func() { platformTiers, platformTiersErr = computertiers.Load() })
	if platformTiersErr != nil {
		return false, true, platformTiersErr
	}
	return platformTiers.TestComputer(computerID), platformTiers.PlatformComputer(computerID), nil
}

func (a *OwnerActAuthority) cleanRebaseRefusal(ctx context.Context, tx *sql.Tx, event computerevent.Event, _ string) (string, func() error, error) {
	receiptDigest, ok := selfdevprotocol.CleanRebaseReceiptDigestFromAuthority(event.AuthorityRef)
	if !ok {
		return "clean rebase authority names no receipt digest", nil, nil
	}
	cited := false
	if len(event.InputArtifactRefs) > 1 {
		for _, raw := range event.InputArtifactRefs[1:] {
			if ref, err := computerevent.ParseArtifactRef(raw); err == nil && ref.Digest().String() == receiptDigest {
				cited = true
				break
			}
		}
	}
	if !cited {
		return "the clean rebase receipt is not an input artifact of the decision", nil, nil
	}
	tier := a.cleanRebaseTier
	if tier == nil {
		tier = computerTierForCleanRebase
	}
	test, platform, err := tier(event.ComputerID)
	if err != nil {
		return "", nil, fmt.Errorf("computer tiers: %w", err)
	}
	if platform {
		return "a platform computer never applies a clean rebase without the owner", nil, nil
	}
	if !test {
		return "clean rebases apply without approval only on test computers (ring R1)", nil, nil
	}
	if a.cleanRebaseTape == nil {
		return "no tape reader", nil, nil
	}
	if a.cleanRebaseJobs == nil {
		return "no host build service", nil, nil
	}
	receipt, reason, err := a.cleanRebaseLink(ctx, tx, event, receiptDigest, 0)
	if err != nil || reason != "" {
		return reason, nil, err
	}
	job, found := a.cleanRebaseJobs.Get(event.ComputerID, receipt.HostBuildJobID)
	if !found {
		return "the receipt's host build job is not known to the host build service", nil, nil
	}
	switch {
	case job.State != hostbuild.StateDone || job.Result == nil:
		return fmt.Sprintf("the host build is %s, not done", job.State), nil, nil
	case job.OperationID != event.ParentEventID || job.BundleDigest != event.ProposedEffectRef:
		return "the host build is for another operation or bundle", nil, nil
	case job.BaseCommit != receipt.BootedBase || job.SourcePatchSHA256 != receipt.SourcePatchSHA256:
		return "the host build is of another base or patch", nil, nil
	}
	resultDigest, err := job.Result.Digest()
	if err != nil {
		return "", nil, err
	}
	if resultDigest != receipt.HostBuildResultDigest {
		return "the host build result is not the one the receipt names", nil, nil
	}
	return "", nil, nil
}

// cleanRebaseLink checks one clean rebase decision against its receipt, its
// prior decision and its two bundles. A prior decision that is itself a clean
// rebase is checked the same way (without its host build, which the service
// may have retired), so every chain bottoms out in a decision corpusd
// admitted under owner or consensus authority.
func (a *OwnerActAuthority) cleanRebaseLink(ctx context.Context, tx *sql.Tx, event computerevent.Event, receiptDigest string, depth int) (selfdevprotocol.CleanRebaseReceipt, string, error) {
	var none selfdevprotocol.CleanRebaseReceipt
	raw, err := a.cleanRebaseTape(event.ComputerID, tapestore.NSPayload, receiptDigest)
	if err != nil {
		return none, "", fmt.Errorf("clean rebase receipt %s: %w", receiptDigest, err)
	}
	if computerevent.DigestBytes(raw) != receiptDigest {
		return none, "the clean rebase receipt bytes do not hash to the authority's digest", nil
	}
	receipt, err := selfdevprotocol.DecodeCleanRebaseReceipt(raw)
	if err != nil {
		return none, "the clean rebase receipt does not decode: " + err.Error(), nil
	}
	switch {
	case receipt.ComputerID != event.ComputerID:
		return none, "the clean rebase receipt is for another computer", nil
	case receipt.OperationID != event.ParentEventID:
		return none, "the clean rebase receipt names another operation", nil
	case receipt.BundleDigest != event.ProposedEffectRef:
		return none, "the clean rebase receipt names another bundle", nil
	}

	// The prior decision.
	var kind string
	err = tx.QueryRowContext(ctx, `SELECT event_kind FROM computer_event_append_receipts WHERE computer_id=? AND event_digest=?`, event.ComputerID, receipt.PriorDecisionEvent).Scan(&kind)
	if errors.Is(err, sql.ErrNoRows) {
		return none, "the prior decision is not on this computer's tape", nil
	}
	if err != nil {
		return none, "", err
	}
	if kind != string(computerevent.EventEffectAccepted) {
		return none, "the prior decision is a " + kind + ", not an effect_accepted", nil
	}
	rawPrior, err := a.cleanRebaseTape(event.ComputerID, tapestore.NSEvent, receipt.PriorDecisionEvent)
	if err != nil {
		return none, "", fmt.Errorf("prior decision %s: %w", receipt.PriorDecisionEvent, err)
	}
	if computerevent.DigestBytes(rawPrior) != receipt.PriorDecisionEvent {
		return none, "the prior decision bytes do not hash to its digest", nil
	}
	prior, err := computerevent.DecodeHistoricEvent(rawPrior)
	if err != nil {
		return none, "the prior decision does not decode", nil
	}
	switch {
	case prior.EventKind != computerevent.EventEffectAccepted || prior.ComputerID != event.ComputerID:
		return none, "the prior decision is not an effect_accepted of this computer", nil
	case prior.ParentEventID != receipt.RebaseOf:
		return none, "the prior decision accepted another operation than the one rebased", nil
	case prior.ProposedEffectRef != receipt.PriorBundleDigest:
		return none, "the prior decision accepted another bundle than the receipt's prior bundle", nil
	}
	// The prior authority must be one corpusd checked and marked when it
	// admitted it (an observe-mode refusal is admitted but never marked).
	priorAuthority := strings.TrimSpace(prior.AuthorityRef)
	switch {
	case strings.HasPrefix(priorAuthority, AuthorityExternalOwner) && strings.TrimSpace(strings.TrimPrefix(priorAuthority, AuthorityExternalOwner)) != "":
		marked, err := admittedMark(ctx, tx, `self_development_mode_receipts`, event.ComputerID, receipt.PriorDecisionEvent)
		if err != nil {
			return none, "", err
		}
		if !marked {
			return none, "the prior owner decision was never admitted against an owner grant", nil
		}
	case IsConsensusAuthority(priorAuthority):
		marked, err := admittedMark(ctx, tx, `consensus_decisions`, event.ComputerID, receipt.PriorDecisionEvent)
		if err != nil {
			return none, "", err
		}
		if !marked {
			return none, "the prior consensus decision was never admitted against a minted decision", nil
		}
	case IsCleanRebaseAuthority(priorAuthority):
		if depth+1 >= cleanRebaseMaxChain {
			return none, "the clean rebase chain is too long", nil
		}
		priorReceiptDigest, ok := selfdevprotocol.CleanRebaseReceiptDigestFromAuthority(priorAuthority)
		if !ok {
			return none, "the prior clean rebase names no receipt digest", nil
		}
		if _, reason, err := a.cleanRebaseLink(ctx, tx, prior, priorReceiptDigest, depth+1); err != nil || reason != "" {
			if reason != "" {
				reason = "the prior clean rebase does not hold: " + reason
			}
			return none, reason, err
		}
	default:
		return none, fmt.Sprintf("the prior decision's authority %q is not an owner, consensus or clean rebase decision", priorAuthority), nil
	}

	// The two bundles.
	readBundle := func(name, digest string) ([]byte, string, error) {
		body, err := a.cleanRebaseTape(event.ComputerID, tapestore.NSPayload, digest)
		if err != nil {
			return nil, "", fmt.Errorf("%s bundle %s: %w", name, digest, err)
		}
		if computerevent.DigestBytes(body) != digest {
			return nil, "the " + name + " bundle bytes do not hash to its digest", nil
		}
		return body, "", nil
	}
	priorBundle, reason, err := readBundle("prior", receipt.PriorBundleDigest)
	if err != nil || reason != "" {
		return none, reason, err
	}
	nextBundle, reason, err := readBundle("next", receipt.BundleDigest)
	if err != nil || reason != "" {
		return none, reason, err
	}
	facts, err := selfdevprotocol.CompareCleanRebaseBundles(priorBundle, nextBundle)
	if err != nil {
		return none, err.Error(), nil
	}
	switch {
	case facts.SourcePatchSHA256 != receipt.SourcePatchSHA256:
		return none, "the bundles carry another patch than the receipt names", nil
	case facts.PriorBase != receipt.StackBase:
		return none, "the prior bundle's base is not the receipt's stack base", nil
	case facts.NextBase != receipt.BootedBase:
		return none, "the next bundle's base is not the receipt's booted base", nil
	}
	return receipt, "", nil
}

// admittedMark reports whether an authority log row recorded this event as
// the one it admitted.
func admittedMark(ctx context.Context, tx *sql.Tx, table, computerID, eventDigest string) (bool, error) {
	var count int
	err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table+` WHERE computer_id=? AND admitted_event_digest=?`, computerID, eventDigest).Scan(&count)
	return count > 0, err
}
