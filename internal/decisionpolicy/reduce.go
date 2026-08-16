package decisionpolicy

import (
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

func (s EffectSubject) Irreversible() bool {
	return s.EffectClass == EffectClassIrreversible ||
		strings.TrimSpace(s.Recipient) != "" ||
		strings.TrimSpace(s.PayloadDigest) != "" ||
		s.Actuator == ActuatorTrustedOutbox ||
		strings.TrimSpace(s.AcceptanceInbox) != "" ||
		s.ExternalSends > 0
}

func parseDecisionWindow(spec string) (time.Duration, error) {
	spec = strings.TrimSpace(spec)
	if spec == "PT30M" {
		return 30 * time.Minute, nil
	}
	if strings.HasPrefix(spec, "PT") && strings.HasSuffix(spec, "M") {
		var minutes int
		if _, err := fmt.Sscanf(spec, "PT%dM", &minutes); err == nil && minutes > 0 {
			return time.Duration(minutes) * time.Minute, nil
		}
	}
	return 0, fmt.Errorf("%w: decision_window %q", ErrWindow, spec)
}

func Reduce(store *Store, input ConsensusInput) (QualifiedConsensusReceipt, error) {
	if store == nil {
		return QualifiedConsensusReceipt{}, ErrNoPolicy
	}
	policyDigest := strings.TrimSpace(input.Selection.PolicyDigest)
	if policyDigest == "" {
		var err error
		policyDigest, err = input.Policy.Digest()
		if err != nil {
			return QualifiedConsensusReceipt{}, err
		}
	}
	policy, _, err := store.Get(policyDigest)
	if err != nil {
		return QualifiedConsensusReceipt{}, err
	}
	if policy.PolicyID == "" || (input.Policy.PolicyID != "" && input.Policy.PolicyID != policy.PolicyID) {
		return QualifiedConsensusReceipt{}, ErrNoPolicy
	}
	if input.Subject.OwnerRecovery {
		return QualifiedConsensusReceipt{}, ErrOwnerRecovery
	}
	if policy.EffectClass == EffectClassReversible && input.Subject.Irreversible() {
		return QualifiedConsensusReceipt{}, ErrReversiblePolicyIrreversibleSubject
	}
	if err := refuseBounds(policy, input.Subject); err != nil {
		return QualifiedConsensusReceipt{}, err
	}

	subjectDigest, err := input.Subject.Digest()
	if err != nil {
		return QualifiedConsensusReceipt{}, err
	}
	manifestDigest, err := input.Manifest.Digest()
	if err != nil {
		return QualifiedConsensusReceipt{}, err
	}
	if err := validateManifest(policy, input.Manifest); err != nil {
		return QualifiedConsensusReceipt{}, err
	}

	selection := input.Selection
	if selection.ReceiptKind != ReceiptKindPolicySelection {
		return QualifiedConsensusReceipt{}, ErrMissingSelection
	}
	if selection.PolicyDigest != policyDigest || selection.SeatManifestDigest != manifestDigest ||
		selection.SubjectDigest != subjectDigest {
		return QualifiedConsensusReceipt{}, ErrSubjectMismatch
	}
	if !computerevent.IsSHA256(selection.SelectedAtHead) || selection.SelectedSequence == 0 {
		return QualifiedConsensusReceipt{}, ErrSelectionAfterOutputs
	}
	selectionDigest, err := selection.Digest()
	if err != nil {
		return QualifiedConsensusReceipt{}, err
	}
	if selection.SelectionDigest != "" && selection.SelectionDigest != selectionDigest {
		return QualifiedConsensusReceipt{}, fmt.Errorf("%w: selection digest", ErrMissingSelection)
	}

	now, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(input.Now))
	if err != nil || now.Location() != time.UTC {
		return QualifiedConsensusReceipt{}, fmt.Errorf("%w: now must be canonical UTC", ErrWindow)
	}
	window, err := parseDecisionWindow(policy.Timeout.DecisionWindow)
	if err != nil {
		return QualifiedConsensusReceipt{}, err
	}
	startedAt := now.UTC().Truncate(time.Microsecond)
	expiresAt := startedAt.Add(window)
	windowID := selectionDigest

	seatsByID := map[string]Seat{}
	policySeatByID := map[string]PolicySeat{}
	for _, seat := range input.Manifest.Seats {
		seatsByID[seat.SeatID] = seat
	}
	for _, seat := range policy.EligibleSeats {
		policySeatByID[seat.SeatID] = seat
	}

	authorSigners := map[string]bool{}
	for _, ballot := range input.Ballots {
		policySeat, ok := policySeatByID[ballot.SeatID]
		if ok && policySeat.IndependenceDomain == "authoring" {
			authorSigners[ballot.SignerProvenance] = true
		}
	}

	seenBallot := map[string]bool{}
	signerDomain := map[string]string{}
	accepts := 0
	domainAccepts := map[string]int{}
	blockingDissent := false
	humanPresent := false
	requiredPresent := map[string]bool{}

	for _, ballot := range input.Ballots {
		if seenBallot[ballot.BallotID] {
			return QualifiedConsensusReceipt{}, fmt.Errorf("%w: duplicate ballot", ErrBallotAttestation)
		}
		seenBallot[ballot.BallotID] = true
		if err := ballot.VerifyAttestation(); err != nil {
			return QualifiedConsensusReceipt{}, err
		}
		if ballot.PolicyDigest != policyDigest || ballot.SeatManifestDigest != manifestDigest ||
			ballot.SubjectDigest != subjectDigest || ballot.PolicySelectionDigest != selectionDigest {
			return QualifiedConsensusReceipt{}, ErrBallotNotJoined
		}
		if ballot.WindowID != windowID {
			return QualifiedConsensusReceipt{}, fmt.Errorf("%w: ballot window", ErrWindow)
		}
		seat, ok := seatsByID[ballot.SeatID]
		if !ok {
			return QualifiedConsensusReceipt{}, ErrMissingRequiredSeat
		}
		if seat.Recused {
			return QualifiedConsensusReceipt{}, ErrSeatDropped
		}
		policySeat, ok := policySeatByID[ballot.SeatID]
		if !ok {
			return QualifiedConsensusReceipt{}, ErrMissingRequiredSeat
		}
		if ballot.IndependenceDomain != seat.IndependenceDomain || ballot.IndependenceDomain != policySeat.IndependenceDomain {
			return QualifiedConsensusReceipt{}, ErrIndependenceFabricated
		}
		prevDomain, seenSigner := signerDomain[ballot.SignerProvenance]
		if seenSigner && prevDomain != ballot.IndependenceDomain {
			return QualifiedConsensusReceipt{}, ErrIndependenceFabricated
		}
		signerDomain[ballot.SignerProvenance] = ballot.IndependenceDomain
		if containsString(policySeat.RecusedFrom, ballot.IndependenceDomain) {
			return QualifiedConsensusReceipt{}, ErrIndependenceFabricated
		}
		if authorSigners[ballot.SignerProvenance] {
			if author, ok := policySeatByID["cosuper-author"]; ok && containsString(author.RecusedFrom, ballot.IndependenceDomain) {
				return QualifiedConsensusReceipt{}, ErrIndependenceFabricated
			}
		}
		if seat.Kind == "owner_human" || policySeat.Kind == "owner_human" {
			humanPresent = true
		}
		if policySeat.CountsTowardQuorum {
			requiredPresent[ballot.SeatID] = true
		}
		switch ballot.Vote {
		case VoteAccept:
			if policySeat.CountsTowardQuorum {
				accepts++
				domainAccepts[ballot.IndependenceDomain]++
			}
		case VoteReject:
			if policySeat.CountsTowardQuorum && policy.Dissent.PolicyBlockingUnresolved == "refuse" {
				blockingDissent = true
			}
		}
	}

	if blockingDissent {
		return QualifiedConsensusReceipt{}, ErrUnresolvedDissent
	}
	for _, policySeat := range policy.EligibleSeats {
		domainRule := policy.Quorum.PerDomain[policySeat.IndependenceDomain]
		if domainRule.RequiredPresent && policySeat.CountsTowardQuorum && !requiredPresent[policySeat.SeatID] {
			return QualifiedConsensusReceipt{}, ErrMissingRequiredSeat
		}
	}
	if accepts < policy.Quorum.GlobalAcceptMinimum {
		return QualifiedConsensusReceipt{}, ErrBelowQuorum
	}
	for domain, rule := range policy.Quorum.PerDomain {
		if domainAccepts[domain] < rule.AcceptMinimum {
			return QualifiedConsensusReceipt{}, ErrBelowQuorum
		}
	}

	humanState := HumanSeatNotRequired
	switch policy.HumanSeat {
	case HumanSeatRequired:
		if !humanPresent {
			return QualifiedConsensusReceipt{}, ErrHumanSeatAbsent
		}
		humanState = HumanSeatPresent
	case HumanSeatAbsent:
		if humanPresent {
			return QualifiedConsensusReceipt{}, fmt.Errorf("%w: human seat forbidden", ErrBounds)
		}
		humanState = HumanSeatAbsent
	case HumanSeatOptional:
		if humanPresent {
			humanState = HumanSeatPresent
		} else {
			humanState = HumanSeatAbsent
		}
	}

	receipt := QualifiedConsensusReceipt{
		ReceiptKind:         ReceiptKindQualifiedConsensus,
		PolicyID:            policy.PolicyID,
		PolicyDigest:        policyDigest,
		SubjectDigest:       subjectDigest,
		EligibleSeatsDigest: manifestDigest,
		SelectionDigest:     selectionDigest,
		Ballots:             append([]BallotAttestation(nil), input.Ballots...),
		QuorumEvaluation: QuorumEvaluation{
			GlobalAccepts: accepts,
			DomainAccepts: domainAccepts,
			Met:           true,
		},
		DissentDisposition: "resolved_none_blocking",
		HumanSeatState:     humanState,
		Window: DecisionWindow{
			StartedAt: startedAt.Format(time.RFC3339Nano),
			ExpiresAt: expiresAt.Format(time.RFC3339Nano),
		},
		ReducerVersion: ReducerVersionV1,
	}
	if err := receipt.Seal(); err != nil {
		return QualifiedConsensusReceipt{}, err
	}
	return receipt, nil
}

func Verify(store *Store, input ConsensusInput, receipt QualifiedConsensusReceipt) error {
	got, err := Reduce(store, input)
	if err != nil {
		return err
	}
	if err := receipt.VerifyDigest(); err != nil {
		return err
	}
	if got.ReceiptDigest != receipt.ReceiptDigest {
		return fmt.Errorf("%w: reducer output mismatch", ErrInvalidReceipt)
	}
	subjectDigest, err := input.Subject.Digest()
	if err != nil {
		return err
	}
	if receipt.SubjectDigest != subjectDigest {
		return ErrSubjectMismatch
	}
	return nil
}

func validateManifest(policy Policy, manifest SeatManifest) error {
	if len(manifest.Seats) != len(policy.EligibleSeats) {
		return ErrMissingRequiredSeat
	}
	seen := map[string]bool{}
	policyIDs := map[string]PolicySeat{}
	for _, seat := range policy.EligibleSeats {
		policyIDs[seat.SeatID] = seat
	}
	for _, seat := range manifest.Seats {
		if seen[seat.SeatID] {
			return ErrIndependenceFabricated
		}
		seen[seat.SeatID] = true
		if seat.Recused {
			return ErrSeatDropped
		}
		policySeat, ok := policyIDs[seat.SeatID]
		if !ok {
			return ErrMissingRequiredSeat
		}
		if seat.IndependenceDomain != policySeat.IndependenceDomain || seat.Kind != policySeat.Kind {
			return ErrIndependenceFabricated
		}
		if strings.TrimSpace(seat.EligibilityProof) == "" {
			return ErrBallotAttestation
		}
	}
	for _, policySeat := range policy.EligibleSeats {
		if !seen[policySeat.SeatID] {
			return ErrMissingRequiredSeat
		}
	}
	return nil
}

func refuseBounds(policy Policy, subject EffectSubject) error {
	if policy.Budget.ExternalSends == 0 && subject.ExternalSends > 0 {
		return ErrBounds
	}
	if policy.Budget.ExternalSends > 0 && subject.ExternalSends > policy.Budget.ExternalSends {
		return ErrBounds
	}
	if containsString(policy.ForbiddenCapabilities, "outbox.send") && subject.Actuator == ActuatorTrustedOutbox {
		return ErrBounds
	}
	if containsString(policy.InadmissibleEvidence, "owner_recovery_checkpoint") && subject.OwnerRecovery {
		return ErrOwnerRecovery
	}
	if policy.EffectClass == EffectClassIrreversible {
		if strings.TrimSpace(subject.Recipient) == "" || !computerevent.IsSHA256(subject.PayloadDigest) ||
			subject.Actuator != ActuatorTrustedOutbox || strings.TrimSpace(subject.AcceptanceInbox) == "" {
			return ErrBounds
		}
	}
	return nil
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func AuthorityRef(receipt QualifiedConsensusReceipt) string {
	return AuthorityPrefix + strings.TrimSpace(receipt.ReceiptDigest)
}

func ReceiptDigestFromAuthority(authorityRef string) (string, bool) {
	digest := strings.TrimPrefix(strings.TrimSpace(authorityRef), AuthorityPrefix)
	if digest == "" || digest == strings.TrimSpace(authorityRef) || !computerevent.IsSHA256(digest) {
		return "", false
	}
	return digest, true
}

func (s *Store) ReduceBundle(bundle ConsensusBundle) (QualifiedConsensusReceipt, error) {
	if s == nil {
		return QualifiedConsensusReceipt{}, ErrNoPolicy
	}
	policy, _, err := s.Get(strings.TrimSpace(bundle.Selection.PolicyDigest))
	if err != nil {
		return QualifiedConsensusReceipt{}, err
	}
	return Reduce(s, ConsensusInput{
		Policy: policy, Manifest: bundle.Manifest, Subject: bundle.Subject,
		Selection: bundle.Selection, Ballots: bundle.Ballots, Now: bundle.Now,
	})
}
