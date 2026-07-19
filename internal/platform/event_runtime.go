package platform

import (
	"context"
	"crypto/ed25519"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type bootstrapControlKeyResolver struct {
	store     *Store
	domain    string
	keyID     string
	publicKey ed25519.PublicKey
}

func (r bootstrapControlKeyResolver) ResolveReceiptKey(domain, computerID, keyID string, sequence uint64, issuedAt time.Time) (ed25519.PublicKey, error) {
	key, err := (ControlKeyResolver{Store: r.store}).ResolveReceiptKey(domain, computerID, keyID, sequence, issuedAt)
	if err == nil {
		return key, nil
	}
	if !errors.Is(err, sql.ErrNoRows) || domain != r.domain || keyID != r.keyID {
		return nil, err
	}
	head, headErr := readComputerEventHead(context.Background(), r.store.db, computerID, false)
	if headErr != nil {
		return nil, headErr
	}
	if head != nil {
		return nil, fmt.Errorf("control key resolver: bootstrap key absent after genesis")
	}
	return append(ed25519.PublicKey(nil), r.publicKey...), nil
}

func (s *Service) ComputerEventRuntime() (*ComputerEventCAS, *EventArtifactService, SignedCapabilityVerifier, error) {
	if s == nil || s.store == nil || s.signingKey == nil {
		return nil, nil, SignedCapabilityVerifier{}, fmt.Errorf("computer event runtime: platform signer unavailable")
	}
	resolver := bootstrapControlKeyResolver{store: s.store, domain: "platform-control", keyID: s.signingKey.KeyID, publicKey: s.signingKey.Public}
	artifacts, err := NewEventArtifactService(s, resolver)
	if err != nil {
		return nil, nil, SignedCapabilityVerifier{}, err
	}
	cas, err := NewComputerEventCAS(s.store, "corpusd", s.computerEventSigningKey(), artifacts)
	if err != nil {
		return nil, nil, SignedCapabilityVerifier{}, err
	}
	auth := SignedCapabilityVerifier{Store: s.store, PublicKey: s.signingKey.Public}
	return cas, artifacts, auth, nil
}

func (s *Service) SelfDevelopmentModeRuntime() (*SelfDevelopmentModeCAS, error) {
	if s == nil || s.store == nil || s.signingKey == nil {
		return nil, fmt.Errorf("self-development mode runtime: platform signer unavailable")
	}
	return NewSelfDevelopmentModeCAS(s.store, s.computerEventSigningKey())
}
