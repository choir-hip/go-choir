package platform

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"strconv"
)

var credentialEnvelopeDomain = []byte("choir-computer-credential-envelope-v1\x00")

const maximumCredentialEnvelopeTTL = 5 * time.Minute

type ComputerCredentialEnvelope struct {
	Version                int    `json:"version"`
	ComputerID             string `json:"computer_id"`
	RealizationID          string `json:"realization_id"`
	Bearer                 string `json:"bearer"`
	IssuedAt               string `json:"issued_at"`
	ExpiresAt              string `json:"expires_at"`
	RevocationEpoch        uint64 `json:"revocation_epoch"`
	Nonce                  string `json:"nonce"`
	IssuanceIdempotencyKey string `json:"issuance_idempotency_key"`
	RequestCommitment      string `json:"request_commitment"`
	SigningKeyID           string `json:"signing_key_id"`
	SigningPublicKey       string `json:"signing_public_key"`
	Signature              string `json:"signature,omitempty"`
}

type CredentialExchangeResult struct {
	Capability string                `json:"capability"`
	Receipt    computerevent.Receipt `json:"lifecycle_receipt"`
	ExpiresAt  string                `json:"expires_at"`

	PrivacyKey string `json:"privacy_key,omitempty"`
}

func (e ComputerCredentialEnvelope) VerifyBootstrap(computerID, realizationID string, now time.Time) (ed25519.PublicKey, error) {
	publicKey, err := base64.RawStdEncoding.DecodeString(e.SigningPublicKey)
	if err != nil || len(publicKey) != ed25519.PublicKeySize || e.Version != 1 || e.ComputerID != computerID || e.RealizationID != realizationID || e.Bearer == "" || e.Nonce == "" || e.RequestCommitment == "" || e.SigningKeyID == "" {
		return nil, fmt.Errorf("credential envelope: invalid bootstrap binding")
	}
	issuedAt, issuedErr := time.Parse(time.RFC3339Nano, e.IssuedAt)
	expiresAt, expiresErr := time.Parse(time.RFC3339Nano, e.ExpiresAt)
	if issuedErr != nil || expiresErr != nil || issuedAt.Location() != time.UTC || expiresAt.Location() != time.UTC || !now.UTC().Before(expiresAt) || expiresAt.Sub(issuedAt) > maximumCredentialEnvelopeTTL {
		return nil, fmt.Errorf("credential envelope: invalid bootstrap lifetime")
	}
	signature, err := base64.RawStdEncoding.DecodeString(e.Signature)
	if err != nil {
		return nil, fmt.Errorf("credential envelope: invalid bootstrap signature")
	}
	payload, err := credentialEnvelopePayload(e)
	if err != nil || !ed25519.Verify(ed25519.PublicKey(publicKey), credentialEnvelopePreimage(payload), signature) {
		return nil, fmt.Errorf("credential envelope: invalid bootstrap signature")
	}
	return ed25519.PublicKey(publicKey), nil
}

func (s *Service) mintComputerCredentialEnvelope(ctx context.Context, computerID, realizationID, idempotencyKey string, expiresAt time.Time) (ComputerCredentialEnvelope, computerevent.Receipt, error) {
	if s == nil || s.store == nil || s.signingKey == nil || computerID == "" || realizationID == "" || idempotencyKey == "" {
		return ComputerCredentialEnvelope{}, computerevent.Receipt{}, fmt.Errorf("credential envelope: complete issuance input is required")
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	expiresAt = expiresAt.UTC().Truncate(time.Microsecond)
	if !expiresAt.After(now) || expiresAt.Sub(now) > maximumCredentialEnvelopeTTL {
		return ComputerCredentialEnvelope{}, computerevent.Receipt{}, fmt.Errorf("credential envelope: expiry exceeds the five-minute issuance window")
	}
	head, err := readComputerEventHead(ctx, s.store.db, computerID, false)
	if err != nil {
		return ComputerCredentialEnvelope{}, computerevent.Receipt{}, err
	}
	var epoch uint64
	if head != nil {
		epoch = head.CredentialRevocationEpoch
	}
	intent := map[string]any{
		"version": 1, "computer_id": computerID, "realization_id": realizationID,
		"expires_at": expiresAt.Format(time.RFC3339Nano), "revocation_epoch": epoch,
		"issuance_idempotency_key": idempotencyKey,
	}
	intentJSON, err := computerevent.CanonicalJSON(intent)
	if err != nil {
		return ComputerCredentialEnvelope{}, computerevent.Receipt{}, err
	}
	requestCommitment := computerevent.DigestBytes(intentJSON)

	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	issuedAt := now
	if existing, completedAt, found, err := s.credentialLifecycleReceipt(ctx, computerID, idempotencyKey, requestCommitment, "credential_envelope_issued"); err != nil {
		return ComputerCredentialEnvelope{}, computerevent.Receipt{}, err
	} else if found {
		envelope, err := s.buildCredentialEnvelope(computerID, realizationID, idempotencyKey, requestCommitment, epoch, completedAt, expiresAt)
		return envelope, existing, err
	}
	envelope, err := s.buildCredentialEnvelope(computerID, realizationID, idempotencyKey, requestCommitment, epoch, issuedAt, expiresAt)
	if err != nil {
		return ComputerCredentialEnvelope{}, computerevent.Receipt{}, err
	}
	receipt, err := computerevent.NewSignedReceipt("LifecycleReceipt", "corpusd", map[string]any{
		"computer_id": computerID, "action": "credential_envelope_issued",
		"prior_lifecycle_state": "absent", "resulting_lifecycle_state": "issued",
		"generation": epoch, "idempotency_key": idempotencyKey,
		"request_commitment": requestCommitment, "completed_at": issuedAt.Format(time.RFC3339Nano),
	}, []computerevent.SigningKey{s.computerEventSigningKey()}, issuedAt)
	if err != nil {
		return ComputerCredentialEnvelope{}, computerevent.Receipt{}, err
	}
	if err := s.insertCredentialLifecycleReceipt(ctx, computerID, idempotencyKey, requestCommitment, "credential_envelope_issued", "absent", "issued", epoch, issuedAt, receipt); err != nil {
		return ComputerCredentialEnvelope{}, computerevent.Receipt{}, err
	}
	return envelope, receipt, nil
}

// MintComputerCredentialEnvelope issues the short-lived signed bootstrap
// envelope consumed once by the exact guest realization.
func (s *Service) MintComputerCredentialEnvelope(ctx context.Context, computerID, realizationID, idempotencyKey string, expiresAt time.Time) (ComputerCredentialEnvelope, computerevent.Receipt, error) {
	return s.mintComputerCredentialEnvelope(ctx, computerID, realizationID, idempotencyKey, expiresAt)
}

func (s *Service) exchangeComputerCredentialEnvelope(ctx context.Context, encoded []byte) (CredentialExchangeResult, error) {
	if s == nil || s.store == nil || s.signingKey == nil {
		return CredentialExchangeResult{}, fmt.Errorf("credential envelope: service unavailable")
	}
	envelope, err := s.verifyCredentialEnvelope(encoded)
	if err != nil {
		return CredentialExchangeResult{}, err
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	expiresAt, _ := time.Parse(time.RFC3339Nano, envelope.ExpiresAt)
	if !now.Before(expiresAt) {
		return CredentialExchangeResult{}, fmt.Errorf("credential envelope: expired")
	}
	head, err := readComputerEventHead(ctx, s.store.db, envelope.ComputerID, false)
	if err != nil {
		return CredentialExchangeResult{}, err
	}
	if (head == nil && envelope.RevocationEpoch != 0) || (head != nil && head.CredentialRevocationEpoch != envelope.RevocationEpoch) {
		return CredentialExchangeResult{}, fmt.Errorf("credential envelope: revoked")
	}
	if _, _, found, err := s.credentialLifecycleReceipt(ctx, envelope.ComputerID, envelope.IssuanceIdempotencyKey, envelope.RequestCommitment, "credential_envelope_issued"); err != nil {
		return CredentialExchangeResult{}, err
	} else if !found {
		return CredentialExchangeResult{}, fmt.Errorf("credential envelope: issuance record absent")
	}
	consumeKey := "credential-envelope-consume:" + envelope.Nonce

	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	capability := ComputerCapability{
		Version: 1, ComputerID: envelope.ComputerID,
		Scopes: []string{"event:read", "event:pin", "event:append"}, ExpiresAt: envelope.ExpiresAt,
		RevocationEpoch: envelope.RevocationEpoch, Nonce: envelope.Nonce,
	}
	token, err := MintComputerCapability(capability, s.signingKey.Private)
	if err != nil {
		return CredentialExchangeResult{}, err
	}
	if _, _, found, err := s.credentialLifecycleReceipt(ctx, envelope.ComputerID, consumeKey, envelope.RequestCommitment, "credential_envelope_consumed"); err != nil {
		return CredentialExchangeResult{}, err
	} else if found {
		return CredentialExchangeResult{}, fmt.Errorf("credential envelope: already consumed")
	}
	receipt, err := computerevent.NewSignedReceipt("LifecycleReceipt", "corpusd", map[string]any{
		"computer_id": envelope.ComputerID, "action": "credential_envelope_consumed",
		"prior_lifecycle_state": "issued", "resulting_lifecycle_state": "consumed",
		"generation": envelope.RevocationEpoch, "idempotency_key": consumeKey,
		"request_commitment": envelope.RequestCommitment, "completed_at": now.Format(time.RFC3339Nano),
	}, []computerevent.SigningKey{s.computerEventSigningKey()}, now)
	if err != nil {
		return CredentialExchangeResult{}, err
	}
	if err := s.insertCredentialLifecycleReceipt(ctx, envelope.ComputerID, consumeKey, envelope.RequestCommitment, "credential_envelope_consumed", "issued", "consumed", envelope.RevocationEpoch, now, receipt); err != nil {
		return CredentialExchangeResult{}, err
	}
	privacyKey, err := s.computerPrivacyKey(ctx, envelope.ComputerID)
	if err != nil {
		return CredentialExchangeResult{}, err
	}
	return CredentialExchangeResult{Capability: token, Receipt: receipt, ExpiresAt: envelope.ExpiresAt, PrivacyKey: privacyKey}, nil
}
func (s *Service) RenewComputerCapability(ctx context.Context, computerID string) (CredentialExchangeResult, error) {
	if s == nil || s.store == nil || s.signingKey == nil || computerID == "" {
		return CredentialExchangeResult{}, fmt.Errorf("computer capability renewal unavailable")
	}
	head, err := readComputerEventHead(ctx, s.store.db, computerID, false)
	if err != nil {
		return CredentialExchangeResult{}, err
	}
	var epoch uint64
	if head != nil {
		epoch = head.CredentialRevocationEpoch
	}
	var nonceBytes [32]byte
	if _, err := rand.Read(nonceBytes[:]); err != nil {
		return CredentialExchangeResult{}, err
	}
	expiresAt := time.Now().UTC().Truncate(time.Microsecond).Add(defaultComputerCapabilityTTL)
	token, err := MintComputerCapability(ComputerCapability{
		Version: 1, ComputerID: computerID,
		Scopes:    []string{"event:read", "event:pin", "event:append"},
		ExpiresAt: expiresAt.Format(time.RFC3339Nano), RevocationEpoch: epoch,
		Nonce: base64.RawURLEncoding.EncodeToString(nonceBytes[:]),
	}, s.signingKey.Private)
	if err != nil {
		return CredentialExchangeResult{}, err
	}
	return CredentialExchangeResult{Capability: token, ExpiresAt: expiresAt.Format(time.RFC3339Nano)}, nil
}

func (s *Service) computerPrivacyKey(ctx context.Context, computerID string) (string, error) {
	var encoded, digest string
	read := func() error {
		return s.store.db.QueryRowContext(ctx, `SELECT key_material, key_version_digest FROM computer_privacy_keys WHERE computer_id=?`, computerID).Scan(&encoded, &digest)
	}
	if err := read(); err == nil {
		raw, decodeErr := base64.RawStdEncoding.DecodeString(encoded)
		if decodeErr != nil || len(raw) != 32 || computerevent.DigestBytes(raw) != digest {
			return "", fmt.Errorf("computer privacy key: stored key is invalid")
		}
		return encoded, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	encoded = base64.RawStdEncoding.EncodeToString(raw)
	digest = computerevent.DigestBytes(raw)
	_, err := s.store.db.ExecContext(ctx, `INSERT INTO computer_privacy_keys (computer_id, key_version_digest, key_material, created_at) VALUES (?, ?, ?, ?)`, computerID, digest, encoded, time.Now().UTC().Truncate(time.Microsecond))
	if err != nil {
		if readErr := read(); readErr == nil {
			return encoded, nil
		}
		return "", err
	}
	return encoded, nil
}

func (s *Service) buildCredentialEnvelope(computerID, realizationID, idempotencyKey, requestCommitment string, epoch uint64, issuedAt, expiresAt time.Time) (ComputerCredentialEnvelope, error) {
	seed := s.signingKey.Private.Seed()
	bearer := credentialPRF(seed, "bearer", requestCommitment)
	nonce := credentialPRF(seed, "nonce", requestCommitment)
	envelope := ComputerCredentialEnvelope{
		Version: 1, ComputerID: computerID, RealizationID: realizationID,
		Bearer: bearer, IssuedAt: issuedAt.UTC().Format(time.RFC3339Nano), ExpiresAt: expiresAt.UTC().Format(time.RFC3339Nano),
		RevocationEpoch: epoch, Nonce: nonce, IssuanceIdempotencyKey: idempotencyKey,
		RequestCommitment: requestCommitment, SigningKeyID: s.signingKey.KeyID,
		SigningPublicKey: base64.RawStdEncoding.EncodeToString(s.signingKey.Public),
	}
	payload, err := credentialEnvelopePayload(envelope)
	if err != nil {
		return ComputerCredentialEnvelope{}, err
	}
	envelope.Signature = base64.RawStdEncoding.EncodeToString(ed25519.Sign(s.signingKey.Private, credentialEnvelopePreimage(payload)))
	return envelope, nil
}

func (s *Service) verifyCredentialEnvelope(encoded []byte) (ComputerCredentialEnvelope, error) {
	var envelope ComputerCredentialEnvelope
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil {
		return ComputerCredentialEnvelope{}, fmt.Errorf("credential envelope: decode: %w", err)
	}
	canonical, err := computerevent.CanonicalJSON(envelope)
	if err != nil || !bytes.Equal(canonical, encoded) {
		return ComputerCredentialEnvelope{}, fmt.Errorf("credential envelope: non-canonical encoding")
	}
	issuedAt, issuedErr := time.Parse(time.RFC3339Nano, envelope.IssuedAt)
	expiresAt, expiresErr := time.Parse(time.RFC3339Nano, envelope.ExpiresAt)
	if envelope.Version != 1 || envelope.ComputerID == "" || envelope.RealizationID == "" || envelope.IssuanceIdempotencyKey == "" || envelope.RequestCommitment == "" || envelope.Nonce == "" || envelope.Bearer == "" || envelope.SigningKeyID != s.signingKey.KeyID || envelope.SigningPublicKey != base64.RawStdEncoding.EncodeToString(s.signingKey.Public) || issuedErr != nil || expiresErr != nil || issuedAt.Location() != time.UTC || expiresAt.Location() != time.UTC || issuedAt.Format(time.RFC3339Nano) != envelope.IssuedAt || expiresAt.Format(time.RFC3339Nano) != envelope.ExpiresAt || !expiresAt.After(issuedAt) || expiresAt.Sub(issuedAt) > maximumCredentialEnvelopeTTL {
		return ComputerCredentialEnvelope{}, fmt.Errorf("credential envelope: invalid fields")
	}
	payload, err := credentialEnvelopePayload(envelope)
	if err != nil {
		return ComputerCredentialEnvelope{}, err
	}
	signature, err := base64.RawStdEncoding.DecodeString(envelope.Signature)
	if err != nil || !ed25519.Verify(s.signingKey.Public, credentialEnvelopePreimage(payload), signature) {
		return ComputerCredentialEnvelope{}, fmt.Errorf("credential envelope: invalid signature")
	}
	if envelope.Bearer != credentialPRF(s.signingKey.Private.Seed(), "bearer", envelope.RequestCommitment) || envelope.Nonce != credentialPRF(s.signingKey.Private.Seed(), "nonce", envelope.RequestCommitment) {
		return ComputerCredentialEnvelope{}, fmt.Errorf("credential envelope: invalid credential material")
	}
	return envelope, nil
}

func credentialEnvelopePayload(envelope ComputerCredentialEnvelope) ([]byte, error) {
	envelope.Signature = ""
	return computerevent.CanonicalJSON(envelope)
}

func credentialEnvelopePreimage(payload []byte) []byte {
	preimage := make([]byte, 0, len(credentialEnvelopeDomain)+len(payload))
	preimage = append(preimage, credentialEnvelopeDomain...)
	return append(preimage, payload...)
}

func credentialPRF(seed []byte, purpose, commitment string) string {
	mac := hmac.New(sha256.New, seed)
	_, _ = mac.Write([]byte("choir-computer-credential-v1\x00"))
	_, _ = mac.Write([]byte(purpose))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(commitment))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s *Service) credentialLifecycleReceipt(ctx context.Context, computerID, idempotencyKey, requestCommitment, action string) (computerevent.Receipt, time.Time, bool, error) {
	var storedCommitment, storedAction, storedPrior, storedResult, rawReceipt, receiptDigest string
	var storedGeneration uint64
	var completedAt time.Time
	err := s.store.db.QueryRowContext(ctx, `SELECT request_commitment, action, prior_lifecycle_state, resulting_lifecycle_state, generation, receipt_json, receipt_digest, completed_at FROM computer_lifecycle_receipts WHERE computer_id=? AND idempotency_key=?`, computerID, idempotencyKey).Scan(&storedCommitment, &storedAction, &storedPrior, &storedResult, &storedGeneration, &rawReceipt, &receiptDigest, &completedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return computerevent.Receipt{}, time.Time{}, false, nil
	}
	if err != nil {
		return computerevent.Receipt{}, time.Time{}, false, err
	}
	if storedCommitment != requestCommitment || storedAction != action {
		return computerevent.Receipt{}, time.Time{}, true, fmt.Errorf("credential envelope: idempotency conflict")
	}
	var receipt computerevent.Receipt
	if err := json.Unmarshal([]byte(rawReceipt), &receipt); err != nil {
		return computerevent.Receipt{}, time.Time{}, true, err
	}
	canonicalReceipt, err := receipt.CanonicalBytes()
	if err != nil || computerevent.DigestBytes(canonicalReceipt) != receiptDigest {
		return computerevent.Receipt{}, time.Time{}, true, fmt.Errorf("credential envelope: lifecycle receipt digest mismatch")
	}
	if receipt.ReceiptKind != "LifecycleReceipt" || receipt.Issuer != "corpusd" {
		return computerevent.Receipt{}, time.Time{}, true, fmt.Errorf("credential envelope: invalid lifecycle receipt kind")
	}
	if err := receipt.RequireKindFields("computer_id", "action", "prior_lifecycle_state", "resulting_lifecycle_state", "generation", "idempotency_key", "request_commitment", "completed_at"); err != nil {
		return computerevent.Receipt{}, time.Time{}, true, err
	}
	if receipt.KindFields["computer_id"] != computerID || receipt.KindFields["action"] != action || receipt.KindFields["prior_lifecycle_state"] != storedPrior || receipt.KindFields["resulting_lifecycle_state"] != storedResult || receipt.KindFields["idempotency_key"] != idempotencyKey || receipt.KindFields["request_commitment"] != requestCommitment || fmt.Sprint(receipt.KindFields["generation"]) != strconv.FormatUint(storedGeneration, 10) || receipt.KindFields["completed_at"] != completedAt.UTC().Format(time.RFC3339Nano) {
		return computerevent.Receipt{}, time.Time{}, true, fmt.Errorf("credential envelope: lifecycle receipt binding mismatch")
	}
	resolver := bootstrapControlKeyResolver{store: s.store, domain: "platform-control", keyID: s.signingKey.KeyID, publicKey: s.signingKey.Public}
	if err := receipt.Verify(resolver); err != nil {
		return computerevent.Receipt{}, time.Time{}, true, err
	}
	return receipt, completedAt.UTC(), true, nil
}

func (s *Service) insertCredentialLifecycleReceipt(ctx context.Context, computerID, idempotencyKey, requestCommitment, action, priorState, resultingState string, generation uint64, completedAt time.Time, receipt computerevent.Receipt) error {
	receiptJSON, err := receipt.CanonicalBytes()
	if err != nil {
		return err
	}
	_, err = s.store.db.ExecContext(ctx, `INSERT INTO computer_lifecycle_receipts (computer_id, idempotency_key, receipt_id, request_commitment, action, prior_lifecycle_state, resulting_lifecycle_state, generation, receipt_json, receipt_digest, completed_at, joined_event_digest) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL)`, computerID, idempotencyKey, receipt.ReceiptID, requestCommitment, action, priorState, resultingState, generation, string(receiptJSON), computerevent.DigestBytes(receiptJSON), completedAt.UTC())
	if err != nil {
		return fmt.Errorf("credential envelope: persist lifecycle receipt: %w", err)
	}
	return s.store.commitDolt(ctx, action+" for "+computerID)
}
