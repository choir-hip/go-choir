package decisionpolicy

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

var (
	ErrNoPolicy                            = errors.New("decision policy: no policy")
	ErrExpiredPolicy                       = errors.New("decision policy: expired")
	ErrRevokedPolicy                       = errors.New("decision policy: revoked")
	ErrMissingSelection                    = errors.New("decision policy: missing PolicySelectionReceipt")
	ErrBallotNotJoined                     = errors.New("decision policy: ballot not joined to PolicySelectionReceipt")
	ErrMissingRequiredSeat                 = errors.New("decision policy: missing required seat")
	ErrBelowQuorum                         = errors.New("decision policy: below quorum")
	ErrUnresolvedDissent                   = errors.New("decision policy: unresolved policy-blocking dissent")
	ErrSeatDropped                         = errors.New("decision policy: seat silently dropped after freeze")
	ErrIndependenceFabricated              = errors.New("decision policy: independence fabricated")
	ErrReversiblePolicyIrreversibleSubject = errors.New("decision policy: reversible policy refuses irreversible subject")
	ErrHumanSeatAbsent                     = errors.New("decision policy: required human seat absent")
	ErrSubjectMismatch                     = errors.New("decision policy: consensus receipt subject mismatch")
	ErrOwnerRecovery                       = errors.New("decision policy: OwnerRecovery is not admissible promotion evidence")
	ErrBallotAttestation                   = errors.New("decision policy: ballot attestation invalid")
	ErrWindow                              = errors.New("decision policy: decision window invalid")
	ErrBounds                              = errors.New("decision policy: capabilities, scope, budget, privacy, or blast radius violated")
	ErrInvalidReceipt                      = errors.New("decision policy: QualifiedConsensusReceipt invalid")
	ErrSelectionAfterOutputs               = errors.New("decision policy: policy selected after outputs exist")
	ErrUnknownReducer                      = errors.New("decision policy: unsupported reducer version")
)

func CanonicalDigest(v any) (string, []byte, error) {
	body, err := computerevent.CanonicalJSON(v)
	if err != nil {
		return "", nil, err
	}
	return computerevent.DigestBytes(body), body, nil
}

func (p Policy) Digest() (string, error) {
	digest, _, err := CanonicalDigest(p)
	return digest, err
}

func (m SeatManifest) Digest() (string, error) {
	digest, _, err := CanonicalDigest(m)
	return digest, err
}

func (s EffectSubject) Digest() (string, error) {
	digest, _, err := CanonicalDigest(s)
	return digest, err
}

func (r PolicySelectionReceipt) Digest() (string, error) {
	cp := r
	cp.SelectionDigest = ""
	digest, _, err := CanonicalDigest(cp)
	return digest, err
}

func (b BallotAttestation) attestationPreimage() ([]byte, error) {
	cp := b
	cp.Attestation = ""
	cp.BallotDigest = ""
	return computerevent.CanonicalJSON(cp)
}

func (b BallotAttestation) ComputeAttestation() (string, error) {
	pre, err := b.attestationPreimage()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(append([]byte(AttestationPrefix), pre...))
	return fmt.Sprintf("%x", sum[:]), nil
}

func (b BallotAttestation) Digest() (string, error) {
	cp := b
	cp.BallotDigest = ""
	digest, _, err := CanonicalDigest(cp)
	return digest, err
}

func (b *BallotAttestation) Sign() error {
	attestation, err := b.ComputeAttestation()
	if err != nil {
		return err
	}
	b.Attestation = attestation
	digest, err := b.Digest()
	if err != nil {
		return err
	}
	b.BallotDigest = digest
	return nil
}

func (b BallotAttestation) VerifyAttestation() error {
	want, err := b.ComputeAttestation()
	if err != nil {
		return err
	}
	if b.Attestation != want {
		return fmt.Errorf("%w: attestation mismatch", ErrBallotAttestation)
	}
	digest, err := b.Digest()
	if err != nil {
		return err
	}
	if b.BallotDigest != digest {
		return fmt.Errorf("%w: ballot digest mismatch", ErrBallotAttestation)
	}
	if !computerevent.IsSHA256(b.EligibilityProofDigest) || strings.TrimSpace(b.SignerProvenance) == "" ||
		strings.TrimSpace(b.WindowID) == "" || strings.TrimSpace(b.BallotID) == "" {
		return fmt.Errorf("%w: missing window, eligibility proof, or signer", ErrBallotAttestation)
	}
	switch b.Vote {
	case VoteAccept, VoteReject, VoteAbstain:
	default:
		return fmt.Errorf("%w: vote", ErrBallotAttestation)
	}
	return nil
}

func (r QualifiedConsensusReceipt) Digest() (string, error) {
	cp := r
	cp.ReceiptDigest = ""
	digest, _, err := CanonicalDigest(cp)
	return digest, err
}

func (r *QualifiedConsensusReceipt) Seal() error {
	digest, err := r.Digest()
	if err != nil {
		return err
	}
	r.ReceiptDigest = digest
	return nil
}

func (r QualifiedConsensusReceipt) VerifyDigest() error {
	if r.ReceiptKind != ReceiptKindQualifiedConsensus {
		return fmt.Errorf("%w: receipt kind", ErrInvalidReceipt)
	}
	if r.ReducerVersion != ReducerVersionV1 {
		return ErrUnknownReducer
	}
	digest, err := r.Digest()
	if err != nil {
		return err
	}
	if r.ReceiptDigest != digest {
		return fmt.Errorf("%w: receipt digest mismatch", ErrInvalidReceipt)
	}
	return nil
}

func ParsePolicy(raw []byte) (Policy, error) {
	var policy Policy
	if err := json.Unmarshal(raw, &policy); err != nil {
		return Policy{}, fmt.Errorf("decision policy: parse: %w", err)
	}
	if strings.TrimSpace(policy.PolicyID) == "" || strings.TrimSpace(policy.EffectClass) == "" {
		return Policy{}, fmt.Errorf("%w: policy_id and effect_class are required", ErrNoPolicy)
	}
	return policy, nil
}

type Store struct {
	mu       sync.RWMutex
	policies map[string][]byte
	revoked  map[string]bool
}

func NewStore() *Store {
	return &Store{policies: map[string][]byte{}, revoked: map[string]bool{}}
}

func (s *Store) Put(raw []byte) (string, error) {
	raw = []byte(strings.TrimSpace(string(raw)))
	policy, err := ParsePolicy(raw)
	if err != nil {
		return "", err
	}
	fileDigest := computerevent.DigestBytes(raw)
	structDigest, err := policy.Digest()
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.policies[fileDigest] = append([]byte(nil), raw...)
	if structDigest != fileDigest {
		s.policies[structDigest] = append([]byte(nil), raw...)
	}
	return fileDigest, nil
}

func (s *Store) Get(digest string) (Policy, []byte, error) {
	s.mu.RLock()
	raw, ok := s.policies[digest]
	revoked := s.revoked[digest]
	s.mu.RUnlock()
	if !ok {
		return Policy{}, nil, ErrNoPolicy
	}
	if revoked {
		return Policy{}, nil, ErrRevokedPolicy
	}
	policy, err := ParsePolicy(raw)
	if err != nil {
		return Policy{}, nil, err
	}
	return policy, append([]byte(nil), raw...), nil
}

func (s *Store) Revoke(digest string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.revoked[digest] = true
}

func (s *Store) Known(digest string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.policies[digest]
	return ok
}

func MustReversibleSelfDevV1Store() *Store {
	s := NewStore()
	digest, err := s.Put(reversibleSelfDevV1JSON)
	if err != nil {
		panic(err)
	}
	if digest != PolicyDigestReversibleSelfDevV1 {
		panic("reversible-selfdev-v1 digest mismatch")
	}
	return s
}
