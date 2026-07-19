package computerevent

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"golang.org/x/crypto/chacha20poly1305"
)

const (
	privateArtifactVersionV1 = 1
	// PrivateArtifactMediaType identifies the canonical XChaCha20 envelope.
	PrivateArtifactMediaType = "application/vnd.choir.private-artifact+json"
)

type PrivateArtifactMetadata struct {
	Version          int            `json:"version"`
	ComputerID       string         `json:"computer_id"`
	EventID          string         `json:"event_id"`
	MediaType        string         `json:"media_type"`
	PlaintextLength  int            `json:"plaintext_length"`
	PrivacyClass     string         `json:"privacy_class"`
	KeyVersionDigest string         `json:"key_version_digest"`
	Nonce            string         `json:"nonce"`
	SecretHandles    []SecretHandle `json:"secret_handles"`
}

type privateArtifactEnvelope struct {
	Metadata   PrivateArtifactMetadata `json:"metadata"`
	Ciphertext string                  `json:"ciphertext"`
}

type privateKeyMaterial struct {
	digest string
	key    [chacha20poly1305.KeySize]byte
}

type privateKeyring interface {
	current(context.Context, string) (privateKeyMaterial, error)
	resolve(context.Context, string, string) (privateKeyMaterial, error)
}

type ExternalPrivacyKeyring struct {
	computerID string
	material   privateKeyMaterial
}

func NewExternalPrivacyKeyring(computerID, encodedKey string) (*ExternalPrivacyKeyring, error) {
	raw, err := base64.RawStdEncoding.DecodeString(strings.TrimSpace(encodedKey))
	if err != nil || len(raw) != chacha20poly1305.KeySize || strings.TrimSpace(computerID) == "" {
		return nil, fmt.Errorf("privacy keyring: invalid external key")
	}
	var key [chacha20poly1305.KeySize]byte
	copy(key[:], raw)
	return &ExternalPrivacyKeyring{
		computerID: computerID,
		material:   privateKeyMaterial{digest: DigestBytes(raw), key: key},
	}, nil
}

func (k *ExternalPrivacyKeyring) current(_ context.Context, computerID string) (privateKeyMaterial, error) {
	if k == nil || computerID != k.computerID {
		return privateKeyMaterial{}, fmt.Errorf("privacy keyring: computer binding mismatch")
	}
	return k.material, nil
}

func (k *ExternalPrivacyKeyring) resolve(_ context.Context, computerID, digest string) (privateKeyMaterial, error) {
	if k == nil || computerID != k.computerID || digest != k.material.digest {
		return privateKeyMaterial{}, fmt.Errorf("privacy keyring: key version unavailable")
	}
	return k.material, nil
}

func NewPrivateArtifactCipherFromExternalKey(computerID, encodedKey string) (*PrivateArtifactCipher, error) {
	keyring, err := NewExternalPrivacyKeyring(computerID, encodedKey)
	if err != nil {
		return nil, err
	}
	return &PrivateArtifactCipher{keys: keyring}, nil
}

type PrivateArtifactCipher struct {
	keys privateKeyring
}

func (c *PrivateArtifactCipher) Encrypt(ctx context.Context, computerID, eventID, mediaType, privacyClass string, plaintext []byte) ([]byte, PrivateArtifactMetadata, error) {
	if c == nil || c.keys == nil || computerID == "" || eventID == "" || mediaType == "" || privacyClass != "private" {
		return nil, PrivateArtifactMetadata{}, fmt.Errorf("private artifact cipher: complete private metadata is required")
	}
	material, err := c.keys.current(ctx, computerID)
	if err != nil {
		return nil, PrivateArtifactMetadata{}, err
	}
	redacted, handles, err := redactPrivatePayload(material.key[:], plaintext)
	if err != nil {
		return nil, PrivateArtifactMetadata{}, err
	}
	aead, err := chacha20poly1305.NewX(material.key[:])
	if err != nil {
		return nil, PrivateArtifactMetadata{}, err
	}
	nonce := make([]byte, chacha20poly1305.NonceSizeX)
	if _, err := rand.Read(nonce); err != nil {
		return nil, PrivateArtifactMetadata{}, fmt.Errorf("private artifact cipher: nonce: %w", err)
	}
	metadata := PrivateArtifactMetadata{
		Version: privateArtifactVersionV1, ComputerID: computerID, EventID: eventID,
		MediaType: mediaType, PlaintextLength: len(redacted), PrivacyClass: privacyClass,
		KeyVersionDigest: material.digest, Nonce: base64.RawStdEncoding.EncodeToString(nonce), SecretHandles: handles,
	}
	aad, err := CanonicalJSON(metadata)
	if err != nil {
		return nil, PrivateArtifactMetadata{}, err
	}
	ciphertext := aead.Seal(nil, nonce, redacted, aad)
	envelope, err := CanonicalJSON(privateArtifactEnvelope{Metadata: metadata, Ciphertext: base64.RawStdEncoding.EncodeToString(ciphertext)})
	if err != nil {
		return nil, PrivateArtifactMetadata{}, err
	}
	return envelope, metadata, nil
}

func (c *PrivateArtifactCipher) Decrypt(ctx context.Context, envelopeJSON []byte, expectedComputerID, expectedEventID string) ([]byte, PrivateArtifactMetadata, error) {
	if c == nil || c.keys == nil {
		return nil, PrivateArtifactMetadata{}, fmt.Errorf("private artifact cipher: unavailable")
	}
	envelope, nonce, ciphertext, err := decodePrivateArtifactEnvelope(envelopeJSON)
	if err != nil {
		return nil, PrivateArtifactMetadata{}, err
	}
	metadata := envelope.Metadata
	if metadata.ComputerID != expectedComputerID || metadata.EventID != expectedEventID {
		return nil, PrivateArtifactMetadata{}, fmt.Errorf("private artifact cipher: metadata identity mismatch")
	}
	material, err := c.keys.resolve(ctx, metadata.ComputerID, metadata.KeyVersionDigest)
	if err != nil {
		return nil, PrivateArtifactMetadata{}, err
	}
	aead, err := chacha20poly1305.NewX(material.key[:])
	if err != nil {
		return nil, PrivateArtifactMetadata{}, err
	}
	aad, err := CanonicalJSON(metadata)
	if err != nil {
		return nil, PrivateArtifactMetadata{}, err
	}
	plaintext, err := aead.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, PrivateArtifactMetadata{}, fmt.Errorf("private artifact cipher: authentication failed")
	}
	if len(plaintext) != metadata.PlaintextLength {
		return nil, PrivateArtifactMetadata{}, fmt.Errorf("private artifact cipher: plaintext length mismatch")
	}
	return plaintext, metadata, nil
}

func InspectPrivateArtifactEnvelope(envelopeJSON []byte) (PrivateArtifactMetadata, error) {
	envelope, _, _, err := decodePrivateArtifactEnvelope(envelopeJSON)
	if err != nil {
		return PrivateArtifactMetadata{}, err
	}
	return envelope.Metadata, nil
}

func decodePrivateArtifactEnvelope(envelopeJSON []byte) (privateArtifactEnvelope, []byte, []byte, error) {
	var envelope privateArtifactEnvelope
	decoder := json.NewDecoder(bytes.NewReader(envelopeJSON))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil {
		return privateArtifactEnvelope{}, nil, nil, fmt.Errorf("private artifact cipher: decode envelope: %w", err)
	}
	canonical, err := CanonicalJSON(envelope)
	if err != nil || !bytes.Equal(canonical, envelopeJSON) {
		return privateArtifactEnvelope{}, nil, nil, fmt.Errorf("private artifact cipher: envelope is not canonical")
	}
	metadata := envelope.Metadata
	if metadata.Version != privateArtifactVersionV1 || metadata.ComputerID == "" || metadata.EventID == "" || metadata.PrivacyClass != "private" || metadata.MediaType == "" || metadata.PlaintextLength < 0 || !IsSHA256(metadata.KeyVersionDigest) {
		return privateArtifactEnvelope{}, nil, nil, fmt.Errorf("private artifact cipher: metadata mismatch")
	}
	seenHandles := make(map[string]struct{}, len(metadata.SecretHandles))
	for _, handle := range metadata.SecretHandles {
		if handle.Kind == "" || !strings.HasPrefix(handle.Handle, "secret-handle:v1:"+handle.Kind+":") {
			return privateArtifactEnvelope{}, nil, nil, fmt.Errorf("private artifact cipher: invalid secret handle")
		}
		if _, duplicate := seenHandles[handle.Handle]; duplicate {
			return privateArtifactEnvelope{}, nil, nil, fmt.Errorf("private artifact cipher: duplicate secret handle")
		}
		seenHandles[handle.Handle] = struct{}{}
	}
	nonce, err := base64.RawStdEncoding.DecodeString(metadata.Nonce)
	if err != nil || len(nonce) != chacha20poly1305.NonceSizeX {
		return privateArtifactEnvelope{}, nil, nil, fmt.Errorf("private artifact cipher: invalid nonce")
	}
	ciphertext, err := base64.RawStdEncoding.DecodeString(envelope.Ciphertext)
	if err != nil || len(ciphertext) < chacha20poly1305.Overhead {
		return privateArtifactEnvelope{}, nil, nil, fmt.Errorf("private artifact cipher: invalid ciphertext")
	}
	return envelope, nonce, ciphertext, nil
}
