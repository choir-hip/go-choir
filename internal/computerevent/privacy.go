package computerevent

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/crypto/chacha20poly1305"
	"syscall"
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

type PrivateArtifactCipher struct {
	keys privateKeyring
}

func NewPrivateArtifactCipher(keys *FilePrivacyKeyring) (*PrivateArtifactCipher, error) {
	if keys == nil {
		return nil, fmt.Errorf("private artifact cipher: keyring is required")
	}
	return &PrivateArtifactCipher{keys: keys}, nil
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

type FilePrivacyKeyring struct {
	root string
	mu   sync.Mutex
}

func NewFilePrivacyKeyring(root string) (*FilePrivacyKeyring, error) {
	root = filepath.Clean(root)
	if root == "" || root == "." || !filepath.IsAbs(root) {
		return nil, fmt.Errorf("privacy keyring: absolute root is required")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("privacy keyring: create root: %w", err)
	}
	if err := requirePrivateDirectory(root); err != nil {
		return nil, err
	}
	return &FilePrivacyKeyring{root: root}, nil
}

func (k *FilePrivacyKeyring) current(_ context.Context, computerID string) (privateKeyMaterial, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	directory, err := k.computerDirectory(computerID)
	if err != nil {
		return privateKeyMaterial{}, err
	}
	currentPath := filepath.Join(directory, "current")
	digestBytes, err := readPrivateRegularFile(currentPath)
	if err == nil {
		return k.readKey(directory, strings.TrimSpace(string(digestBytes)))
	}
	if !errors.Is(err, os.ErrNotExist) {
		return privateKeyMaterial{}, fmt.Errorf("privacy keyring: read current key: %w", err)
	}
	var key [chacha20poly1305.KeySize]byte
	if _, err := rand.Read(key[:]); err != nil {
		return privateKeyMaterial{}, fmt.Errorf("privacy keyring: generate key: %w", err)
	}
	digest := sha256.Sum256(key[:])
	digestText := hex.EncodeToString(digest[:])
	if err := writePrivateFile(filepath.Join(directory, digestText+".key"), key[:]); err != nil {
		return privateKeyMaterial{}, err
	}
	if err := writePrivateFile(currentPath, []byte(digestText+"\n")); err != nil {
		return privateKeyMaterial{}, err
	}
	return privateKeyMaterial{digest: digestText, key: key}, nil
}

func (k *FilePrivacyKeyring) resolve(_ context.Context, computerID, digest string) (privateKeyMaterial, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if !IsSHA256(digest) {
		return privateKeyMaterial{}, fmt.Errorf("privacy keyring: invalid key digest")
	}
	directory, err := k.computerDirectory(computerID)
	if err != nil {
		return privateKeyMaterial{}, err
	}
	return k.readKey(directory, digest)
}

func (k *FilePrivacyKeyring) computerDirectory(computerID string) (string, error) {
	if computerID == "" || strings.ContainsAny(computerID, `/\\`) || computerID == "." || computerID == ".." {
		return "", fmt.Errorf("privacy keyring: invalid computer ID")
	}
	directory := filepath.Join(k.root, computerID)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", fmt.Errorf("privacy keyring: create computer directory: %w", err)
	}
	if err := requirePrivateDirectory(directory); err != nil {
		return "", err
	}
	return directory, nil
}

func (k *FilePrivacyKeyring) readKey(directory, digest string) (privateKeyMaterial, error) {
	if !IsSHA256(digest) {
		return privateKeyMaterial{}, fmt.Errorf("privacy keyring: invalid current key digest")
	}
	path := filepath.Join(directory, digest+".key")
	raw, err := readPrivateRegularFile(path)
	if err != nil || len(raw) != chacha20poly1305.KeySize {
		return privateKeyMaterial{}, fmt.Errorf("privacy keyring: read key")
	}
	actual := sha256.Sum256(raw)
	if hex.EncodeToString(actual[:]) != digest {
		return privateKeyMaterial{}, fmt.Errorf("privacy keyring: key digest mismatch")
	}
	var key [chacha20poly1305.KeySize]byte
	copy(key[:], raw)
	return privateKeyMaterial{digest: digest, key: key}, nil
}

func requirePrivateDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("privacy keyring: inspect directory: %w", err)
	}
	if !info.IsDir() || info.Mode().Perm()&0o077 != 0 || !ownedByEffectiveUser(info) {
		return fmt.Errorf("privacy keyring: directory permissions are not private")
	}
	return nil
}

func readPrivateRegularFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 || !ownedByEffectiveUser(info) {
		return nil, fmt.Errorf("privacy keyring: file permissions are not private")
	}
	return os.ReadFile(path)
}

func ownedByEffectiveUser(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && int(stat.Uid) == os.Geteuid()
}

func writePrivateFile(path string, content []byte) error {
	if _, err := os.Lstat(path); err == nil {
		existing, err := readPrivateRegularFile(path)
		if err != nil || !bytes.Equal(existing, content) {
			return fmt.Errorf("privacy keyring: immutable file differs")
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".private-*")
	if err != nil {
		return fmt.Errorf("privacy keyring: create temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o400); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("privacy keyring: protect temporary file: %w", err)
	}
	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("privacy keyring: write temporary file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("privacy keyring: sync temporary file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("privacy keyring: close temporary file: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("privacy keyring: install file: %w", err)
	}
	return nil
}
