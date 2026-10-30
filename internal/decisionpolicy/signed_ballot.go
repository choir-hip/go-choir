package decisionpolicy

// Signed ballots and the consensus decision receipt (P4 / S2:
// docs/archive/problems/p4-standing-consensus-autonomy-2026-10-15.md §2-3;
// docs/archive/reports/principal-identity-and-signed-acts-2026-10-15.md §3.2, §6 S2).
//
// A ConsensusBallot is a computerevent.Receipt: RFC 8785 canonical JSON under
// the "choir-receipt-v1\0" preimage prefix, so its kind, issuer and signer
// domain are signed. Each seat of reversible-selfdev-v1 may be signed by
// exactly one signer domain (SeatSigner), and the capsule-verifier seat's
// domain is corpusd's platform-control key, which casts that seat on the P5
// host build of the candidate: a guest can sign at most one of the two
// verification seats the policy's quorum needs.
//
// The hash BallotAttestation stays as the reducer's internal join format.
// AttestationFromSignedBallot builds one only from a ballot that already
// verified, so a ballot nobody signed never reaches Reduce.

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

const (
	// BallotReceiptKind is the receipt_kind of every signed ballot.
	BallotReceiptKind = "ConsensusBallot"
	// DecisionReceiptKind is the receipt_kind of the decision corpusd mints
	// from verified ballots.
	DecisionReceiptKind = "ConsensusDecisionReceipt"
	// BallotAudience names the verifier a ballot is cast for.
	BallotAudience = "corpusd"

	SeatAuthor              = "cosuper-author"
	SeatCapsuleVerifier     = "capsule-verifier"
	SeatIndependentReviewer = "independent-reviewer"

	// SignerDomainHostBuild is the capsule-verifier seat's key: corpusd's
	// platform-control key, voting on its own host build of the candidate
	// (P5). No guest holds it.
	SignerDomainHostBuild = "platform-control"
	IssuerHostBuild       = "choir-host-build"
	// SignerDomainGuestCore is the computer's guest-core realization key.
	SignerDomainGuestCore = "guest-core"
	IssuerGuestManagement = "choir-management"

	// DeskManagement is the desk whose Verify verdict is the reviewer seat.
	DeskManagement = "management"
	// MaxBallotTTL bounds a ballot's validity (the policy's window).
	MaxBallotTTL = 30 * time.Minute
)

var (
	// ErrBallotSigner refuses a ballot signed by a domain its seat does not
	// admit, or by a key the verifier does not pin.
	ErrBallotSigner = errors.New("decision policy: ballot signer refused")
	// ErrBallotBinding refuses a ballot for another subject, policy,
	// computer, operation or audience, or outside its window.
	ErrBallotBinding = errors.New("decision policy: ballot binding refused")
)

// SeatSigner is the one signer domain and issuer a seat's ballot may carry.
func SeatSigner(seat string) (domain, issuer string, ok bool) {
	switch seat {
	case SeatCapsuleVerifier:
		return SignerDomainHostBuild, IssuerHostBuild, true
	case SeatIndependentReviewer:
		return SignerDomainGuestCore, IssuerGuestManagement, true
	default:
		return "", "", false
	}
}

// Ballot is a seat's signed vote on one subject under one policy.
type Ballot struct {
	PolicyDigest       string   `json:"policy_digest"`
	SubjectDigest      string   `json:"subject_digest"`
	Seat               string   `json:"seat"`
	IndependenceDomain string   `json:"independence_domain"`
	Vote               string   `json:"vote"`
	EvidenceRefs       []string `json:"evidence_refs"`
	ComputerID         string   `json:"computer_id"`
	OperationID        string   `json:"operation_id"`
	Desk               string   `json:"desk"`
	ModelID            string   `json:"model_id"`
	Audience           string   `json:"audience"`
	Nonce              string   `json:"nonce"`
	NotAfter           string   `json:"not_after"`
}

// NewBallot fills the audience, a fresh nonce and not_after = now + ttl.
func NewBallot(seat, vote, policyDigest, subjectDigest, computerID, operationID, desk, modelID string, evidence []string, now time.Time, ttl time.Duration) (Ballot, error) {
	if ttl <= 0 || ttl > MaxBallotTTL {
		ttl = MaxBallotTTL
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return Ballot{}, err
	}
	domain := "verification"
	if seat == SeatAuthor {
		domain = "authoring"
	}
	refs := make([]string, 0, len(evidence))
	for _, ref := range evidence {
		if ref = strings.TrimSpace(ref); ref != "" {
			refs = append(refs, ref)
		}
	}
	return Ballot{
		PolicyDigest: policyDigest, SubjectDigest: subjectDigest, Seat: seat, IndependenceDomain: domain,
		Vote: vote, EvidenceRefs: refs, ComputerID: computerID, OperationID: operationID,
		Desk: desk, ModelID: modelID, Audience: BallotAudience, Nonce: hex.EncodeToString(nonce[:]),
		NotAfter: now.UTC().Truncate(time.Microsecond).Add(ttl).Format(time.RFC3339Nano),
	}, nil
}

// Fields is the ballot as receipt kind fields (what a signer signs).
func (b Ballot) Fields() (map[string]any, error) {
	raw, err := json.Marshal(b)
	if err != nil {
		return nil, err
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	if b.EvidenceRefs == nil {
		fields["evidence_refs"] = []any{}
	}
	return fields, nil
}

// SignBallot signs a ballot with a key held in-process (corpusd's host
// build seat).
func SignBallot(b Ballot, key computerevent.SigningKey, issuer string, now time.Time) (computerevent.Receipt, error) {
	fields, err := b.Fields()
	if err != nil {
		return computerevent.Receipt{}, err
	}
	return computerevent.NewSignedReceipt(BallotReceiptKind, issuer, fields, []computerevent.SigningKey{key}, now.UTC().Truncate(time.Microsecond))
}

// BallotFromReceipt reads the kind fields of a ballot receipt (unverified).
func BallotFromReceipt(receipt computerevent.Receipt) (Ballot, error) {
	raw, err := json.Marshal(receipt.KindFields)
	if err != nil {
		return Ballot{}, err
	}
	var ballot Ballot
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&ballot); err != nil {
		return Ballot{}, fmt.Errorf("%w: ballot fields: %v", ErrBallotBinding, err)
	}
	return ballot, nil
}

// BallotExpectation is what a verifier requires of one ballot.
type BallotExpectation struct {
	Seat          string
	PolicyDigest  string
	SubjectDigest string
	ComputerID    string
	OperationID   string
	Now           time.Time
}

// VerifiedBallot is a ballot whose signature, signer and binding verified.
type VerifiedBallot struct {
	Ballot  Ballot
	Receipt computerevent.Receipt
	Digest  string
	Signer  computerevent.SignerRef
}

// VerifyBallot checks one signed ballot: kind, the seat's one signer domain
// and issuer, exactly one signature that the resolver's pinned key verifies,
// and the binding to this seat, policy, subject, computer, operation,
// audience and window. The resolver is the caller's key pin: a key it does
// not know is refused.
func VerifyBallot(receipt computerevent.Receipt, resolver computerevent.KeyResolver, want BallotExpectation) (VerifiedBallot, error) {
	if receipt.ReceiptKind != BallotReceiptKind {
		return VerifiedBallot{}, fmt.Errorf("%w: not a ConsensusBallot", ErrBallotSigner)
	}
	domain, issuer, ok := SeatSigner(want.Seat)
	if !ok {
		return VerifiedBallot{}, fmt.Errorf("%w: seat %q is not signable", ErrBallotSigner, want.Seat)
	}
	if receipt.Issuer != issuer || len(receipt.RequiredSigners) != 1 || receipt.RequiredSigners[0].SignerDomain != domain {
		return VerifiedBallot{}, fmt.Errorf("%w: seat %s admits only %s/%s", ErrBallotSigner, want.Seat, domain, issuer)
	}
	if err := receipt.Verify(resolver); err != nil {
		return VerifiedBallot{}, fmt.Errorf("%w: %v", ErrBallotSigner, err)
	}
	ballot, err := BallotFromReceipt(receipt)
	if err != nil {
		return VerifiedBallot{}, err
	}
	switch {
	case ballot.Seat != want.Seat:
		return VerifiedBallot{}, fmt.Errorf("%w: ballot is for seat %q", ErrBallotBinding, ballot.Seat)
	case ballot.PolicyDigest != want.PolicyDigest:
		return VerifiedBallot{}, fmt.Errorf("%w: policy digest", ErrBallotBinding)
	case ballot.SubjectDigest != want.SubjectDigest:
		return VerifiedBallot{}, fmt.Errorf("%w: subject digest", ErrBallotBinding)
	case ballot.ComputerID == "" || ballot.ComputerID != want.ComputerID:
		return VerifiedBallot{}, fmt.Errorf("%w: computer", ErrBallotBinding)
	case ballot.OperationID == "" || ballot.OperationID != want.OperationID:
		return VerifiedBallot{}, fmt.Errorf("%w: operation", ErrBallotBinding)
	case ballot.Audience != BallotAudience:
		return VerifiedBallot{}, fmt.Errorf("%w: audience", ErrBallotBinding)
	case ballot.IndependenceDomain != "verification":
		return VerifiedBallot{}, fmt.Errorf("%w: independence domain", ErrBallotBinding)
	}
	if want.Seat == SeatIndependentReviewer && ballot.Desk != DeskManagement {
		return VerifiedBallot{}, fmt.Errorf("%w: reviewer ballot must name desk %s", ErrBallotBinding, DeskManagement)
	}
	switch ballot.Vote {
	case VoteAccept, VoteReject, VoteAbstain:
	default:
		return VerifiedBallot{}, fmt.Errorf("%w: vote %q", ErrBallotBinding, ballot.Vote)
	}
	if nonce, err := hex.DecodeString(ballot.Nonce); err != nil || len(nonce) != 16 {
		return VerifiedBallot{}, fmt.Errorf("%w: nonce", ErrBallotBinding)
	}
	issuedAt, err := time.Parse(time.RFC3339Nano, receipt.IssuedAt)
	if err != nil {
		return VerifiedBallot{}, fmt.Errorf("%w: issued_at", ErrBallotBinding)
	}
	notAfter, err := time.Parse(time.RFC3339Nano, ballot.NotAfter)
	if err != nil || notAfter.Location() != time.UTC || notAfter.Format(time.RFC3339Nano) != ballot.NotAfter ||
		!notAfter.After(issuedAt) || notAfter.Sub(issuedAt) > MaxBallotTTL {
		return VerifiedBallot{}, fmt.Errorf("%w: not_after", ErrBallotBinding)
	}
	now := want.Now.UTC()
	if issuedAt.After(now.Add(time.Minute)) || !now.Before(notAfter) {
		return VerifiedBallot{}, fmt.Errorf("%w: outside the ballot window", ErrBallotBinding)
	}
	raw, err := receipt.CanonicalBytes()
	if err != nil {
		return VerifiedBallot{}, err
	}
	return VerifiedBallot{Ballot: ballot, Receipt: receipt, Digest: computerevent.DigestBytes(raw), Signer: receipt.RequiredSigners[0]}, nil
}

// AttestationFromSignedBallot is the reducer's join record for a verified
// ballot: the signer's domain and key id are its provenance, and the signed
// ballot's digest is its eligibility proof.
func AttestationFromSignedBallot(verified VerifiedBallot, manifestDigest, selectionDigest string) (BallotAttestation, error) {
	attestation := BallotAttestation{
		BallotID: verified.Receipt.ReceiptID, SeatID: verified.Ballot.Seat,
		EligibilityProofDigest: verified.Digest, IndependenceDomain: verified.Ballot.IndependenceDomain,
		PolicyDigest: verified.Ballot.PolicyDigest, SeatManifestDigest: manifestDigest,
		SubjectDigest: verified.Ballot.SubjectDigest, PolicySelectionDigest: selectionDigest,
		Vote: verified.Ballot.Vote, WindowID: selectionDigest,
		SignerProvenance: verified.Signer.SignerDomain + ":" + verified.Signer.KeyID,
	}
	if err := attestation.Sign(); err != nil {
		return BallotAttestation{}, err
	}
	return attestation, nil
}

// DecisionAuthorityRef is the AuthorityRef an effect_accepted carries for a
// decision corpusd minted: the decision receipt's canonical-bytes digest.
func DecisionAuthorityRef(decisionDigest string) string {
	return AuthorityPrefix + strings.TrimSpace(decisionDigest)
}

// StandingPolicyAllowed reports whether a policy digest may be armed as a
// standing (autonomous) mode: reversible, with no human seat.
func StandingPolicyAllowed(policyDigest string) bool {
	return strings.TrimSpace(policyDigest) == PolicyDigestReversibleSelfDevV1
}

// SeatManifestForPolicy is the seat manifest the decision poster builds
// from a policy: every eligible seat, in policy order, none recused (the
// policy's own recusal rules apply in Reduce), each with its eligibility.
func SeatManifestForPolicy(policy Policy, eligibility map[string]string) SeatManifest {
	seats := make([]Seat, 0, len(policy.EligibleSeats))
	for _, seat := range policy.EligibleSeats {
		proof := strings.TrimSpace(eligibility[seat.SeatID])
		if proof == "" {
			proof = seat.Eligibility
		}
		seats = append(seats, Seat{
			SeatID: seat.SeatID, IndependenceDomain: seat.IndependenceDomain, Kind: seat.Kind, EligibilityProof: proof,
		})
	}
	return SeatManifest{Seats: seats}
}
