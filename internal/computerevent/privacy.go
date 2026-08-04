package computerevent

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

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

type guestPrivacyKeyring struct {
	computerID string
	currentKey privateKeyMaterial
	keys       map[string]privateKeyMaterial
}

type guestPrivacyKeyFile struct {
	Version        int      `json:"version"`
	ComputerID     string   `json:"computer_id"`
	Key            string   `json:"key"`
	HistoricalKeys []string `json:"historical_keys,omitempty"`
}

func newPrivateArtifactCipher(computerID, encodedKey string) (*PrivateArtifactCipher, error) {
	return newPrivateArtifactCipherWithHistoricalKeys(computerID, encodedKey, nil)
}

func newPrivateArtifactCipherWithHistoricalKeys(computerID, encodedKey string, historicalKeys []string) (*PrivateArtifactCipher, error) {
	raw, err := base64.RawStdEncoding.DecodeString(strings.TrimSpace(encodedKey))
	if err != nil || len(raw) != chacha20poly1305.KeySize || strings.TrimSpace(computerID) == "" {
		return nil, fmt.Errorf("privacy keyring: invalid guest key")
	}
	var current [chacha20poly1305.KeySize]byte
	copy(current[:], raw)
	currentMaterial := privateKeyMaterial{digest: DigestBytes(raw), key: current}
	materials := map[string]privateKeyMaterial{currentMaterial.digest: currentMaterial}
	for _, encodedHistorical := range historicalKeys {
		historicalRaw, err := base64.RawStdEncoding.DecodeString(strings.TrimSpace(encodedHistorical))
		if err != nil || len(historicalRaw) != chacha20poly1305.KeySize {
			return nil, fmt.Errorf("privacy keyring: invalid historical guest key")
		}
		var historical [chacha20poly1305.KeySize]byte
		copy(historical[:], historicalRaw)
		material := privateKeyMaterial{digest: DigestBytes(historicalRaw), key: historical}
		if _, duplicate := materials[material.digest]; duplicate {
			return nil, fmt.Errorf("privacy keyring: duplicate historical guest key")
		}
		materials[material.digest] = material
	}
	return &PrivateArtifactCipher{keys: &guestPrivacyKeyring{
		computerID: computerID, currentKey: currentMaterial, keys: materials,
	}}, nil
}

// LoadGuestPrivateArtifactCipher loads the root guest-owned per-computer key.
// A key may be created only before the canonical event chain exists.
func LoadGuestPrivateArtifactCipher(path, computerID string, allowCreate bool) (*PrivateArtifactCipher, error) {
	path = filepath.Clean(path)
	computerID = strings.TrimSpace(computerID)
	if !filepath.IsAbs(path) || computerID == "" {
		return nil, fmt.Errorf("privacy keyring: absolute path and computer identity are required")
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) && allowCreate {
		raw, err = createGuestPrivacyKey(path, computerID)
	}
	if err != nil {
		return nil, fmt.Errorf("privacy keyring: load guest key: %w", err)
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o400 {
		return nil, fmt.Errorf("privacy keyring: guest key must be a mode-0400 regular file")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Geteuid()) {
		return nil, fmt.Errorf("privacy keyring: guest key owner mismatch")
	}
	var keyFile guestPrivacyKeyFile
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&keyFile); err != nil {
		return nil, fmt.Errorf("privacy keyring: invalid guest key")
	}
	canonical, err := CanonicalJSON(keyFile)
	if err != nil || !bytes.Equal(canonical, raw) || keyFile.Version != 1 || keyFile.ComputerID != computerID {
		return nil, fmt.Errorf("privacy keyring: guest key binding mismatch")
	}
	return newPrivateArtifactCipherWithHistoricalKeys(computerID, keyFile.Key, keyFile.HistoricalKeys)
}

func createGuestPrivacyKey(path, computerID string) ([]byte, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	if err := os.Chmod(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	key := make([]byte, chacha20poly1305.KeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	canonical, err := CanonicalJSON(guestPrivacyKeyFile{
		Version: 1, ComputerID: computerID, Key: base64.RawStdEncoding.EncodeToString(key),
	})
	if err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o400)
	if errors.Is(err, os.ErrExist) {
		return os.ReadFile(path)
	}
	if err != nil {
		return nil, err
	}
	if _, err = file.Write(canonical); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(path)
		return nil, err
	}
	return canonical, nil
}

func (k *guestPrivacyKeyring) current(_ context.Context, computerID string) (privateKeyMaterial, error) {
	if k == nil || computerID != k.computerID {
		return privateKeyMaterial{}, fmt.Errorf("privacy keyring: computer binding mismatch")
	}
	return k.currentKey, nil
}

func (k *guestPrivacyKeyring) resolve(_ context.Context, computerID, digest string) (privateKeyMaterial, error) {
	if k == nil || computerID != k.computerID {
		return privateKeyMaterial{}, fmt.Errorf("privacy keyring: key version unavailable")
	}
	material, ok := k.keys[digest]
	if !ok {
		return privateKeyMaterial{}, fmt.Errorf("privacy keyring: key version unavailable")
	}
	return material, nil
}

type PrivateArtifactCipher struct {
	keys privateKeyring
}

func (c *PrivateArtifactCipher) Encrypt(ctx context.Context, computerID, eventID, mediaType, privacyClass string, plaintext []byte) ([]byte, PrivateArtifactMetadata, error) {
	nonce := make([]byte, chacha20poly1305.NonceSizeX)
	if _, err := rand.Read(nonce); err != nil {
		return nil, PrivateArtifactMetadata{}, fmt.Errorf("private artifact cipher: nonce: %w", err)
	}
	return c.encryptWithNonce(ctx, computerID, eventID, mediaType, privacyClass, plaintext, nonce)
}

// EncryptSupervisionDeterministic produces one stable ciphertext for a reserved
// supervision binding. The binding must not be used before the enclosing
// command digest has been durably reserved.
func (c *PrivateArtifactCipher) EncryptSupervisionDeterministic(ctx context.Context, computerID, bindingID, mediaType string, plaintext []byte) ([]byte, PrivateArtifactMetadata, error) {
	if c == nil || c.keys == nil || computerID == "" || bindingID == "" || mediaType == "" || len(plaintext) == 0 {
		return nil, PrivateArtifactMetadata{}, fmt.Errorf("private artifact cipher: complete supervision metadata is required")
	}
	material, err := c.keys.current(ctx, computerID)
	if err != nil {
		return nil, PrivateArtifactMetadata{}, err
	}
	seed, err := CanonicalJSON(map[string]string{
		"domain":           "choir.supervision-private-nonce.v1",
		"computer_id":      computerID,
		"binding_id":       bindingID,
		"media_type":       mediaType,
		"plaintext_digest": DigestBytes(plaintext),
		"key_version":      material.digest,
	})
	if err != nil {
		return nil, PrivateArtifactMetadata{}, fmt.Errorf("private artifact cipher: canonical supervision nonce seed: %w", err)
	}
	sum := sha256.Sum256(seed)
	nonce := make([]byte, chacha20poly1305.NonceSizeX)
	copy(nonce, sum[:chacha20poly1305.NonceSizeX])
	return c.encryptWithNonce(ctx, computerID, bindingID, mediaType, "private", plaintext, nonce)
}

func (c *PrivateArtifactCipher) encryptWithNonce(ctx context.Context, computerID, eventID, mediaType, privacyClass string, plaintext, nonce []byte) ([]byte, PrivateArtifactMetadata, error) {
	if c == nil || c.keys == nil || computerID == "" || eventID == "" || mediaType == "" || privacyClass != "private" || len(nonce) != chacha20poly1305.NonceSizeX {
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
