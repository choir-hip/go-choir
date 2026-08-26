// Package keyescrow implements the Track K custodian escrow wrap: a
// per-computer DEK (32-byte XChaCha20 key, internal/computerevent) wrapped
// under a host-held X25519 escrow keypair. The guest seals; only the host
// escrow private key can open, and only under the application-level
// two-approval gate (platform store). Crypto is deliberately boring:
// X25519 ephemeral ECDH -> HKDF-SHA256 -> XChaCha20-Poly1305, with the
// computer identity bound as associated data so a wrap is useless for any
// other computer.
//
// Design authority: docs/designs/choir-durable-substrate-2026-08-23.md §3.2
// (custodian escrow protector; HSM/KMS deferred; passkey PRF is Track K 1b
// and wraps the owner ROOT key, not DEKs directly).
package keyescrow

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
)

const (
	// WrapVersion is the schema version of WrappedKey records.
	WrapVersion = 1
	// ProtectorCustodian is the host-held custodian escrow protector.
	ProtectorCustodian = "custodian"
	// ProtectorPasskeyPRF is the owner passkey PRF protector (Track K 1b).
	ProtectorPasskeyPRF = "passkey_prf"
	// ProtectorOfflineCapsule is the owner offline capsule protector.
	ProtectorOfflineCapsule = "offline_capsule"

	hkdfInfo = "choir/keyescrow/v1/custodian/xchacha"
)

// ErrInvalidWrap marks malformed or unauthentic wrap records.
var ErrInvalidWrap = errors.New("keyescrow: invalid wrap record")

// PrivateKey is a host-held X25519 escrow private key.
type PrivateKey [32]byte

// PublicKey is the matching X25519 escrow public key.
type PublicKey [32]byte

// GenerateKeyPair creates a fresh escrow keypair.
func GenerateKeyPair() (PrivateKey, PublicKey, error) {
	var priv PrivateKey
	if _, err := rand.Read(priv[:]); err != nil {
		return PrivateKey{}, PublicKey{}, fmt.Errorf("keyescrow: read private key: %w", err)
	}
	pub, err := curve25519.X25519(priv[:], curve25519.Basepoint)
	if err != nil {
		return PrivateKey{}, PublicKey{}, fmt.Errorf("keyescrow: derive public key: %w", err)
	}
	var out PublicKey
	copy(out[:], pub)
	return priv, out, nil
}

// LoadOrGeneratePrivateKey loads the host escrow private key from path,
// creating and persisting it (mode 0600, fsync'd) on first use. The file
// format is the base64 raw private key with optional trailing newline.
func LoadOrGeneratePrivateKey(path string) (PrivateKey, error) {
	if !strings.HasPrefix(path, "/") {
		return PrivateKey{}, fmt.Errorf("keyescrow: private key path must be absolute")
	}
	raw, err := os.ReadFile(path)
	if err == nil {
		trimmed := strings.TrimSpace(string(raw))
		decoded, decErr := base64.RawStdEncoding.DecodeString(trimmed)
		if decErr != nil || len(decoded) != 32 {
			return PrivateKey{}, fmt.Errorf("keyescrow: invalid escrow private key file")
		}
		var priv PrivateKey
		copy(priv[:], decoded)
		return priv, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return PrivateKey{}, fmt.Errorf("keyescrow: read escrow private key: %w", err)
	}
	priv, _, err := GenerateKeyPair()
	if err != nil {
		return PrivateKey{}, err
	}
	if err := os.MkdirAll(dirOf(path), 0o700); err != nil {
		return PrivateKey{}, fmt.Errorf("keyescrow: create key dir: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return PrivateKey{}, fmt.Errorf("keyescrow: create escrow private key: %w", err)
	}
	if _, err = file.Write([]byte(base64.RawStdEncoding.EncodeToString(priv[:]) + "\n")); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(path)
		return PrivateKey{}, fmt.Errorf("keyescrow: write escrow private key: %w", err)
	}
	return priv, nil
}

// WrappedKey is the persisted custodian wrap record. JSON is the canonical
// storage form (platform store persists the serialized record).
type WrappedKey struct {
	Version      int    `json:"version"`
	Protector    string `json:"protector"`
	ComputerID   string `json:"computer_id"`
	EphemeralPub string `json:"ephemeral_pub"` // base64 raw X25519 public key
	Nonce        string `json:"nonce"`         // base64 XChaCha20-Poly1305 nonce
	Ciphertext   string `json:"ciphertext"`    // base64 sealed DEK
	// KeyDigest is SHA-256 over the raw DEK, hex. It lets the host verify a
	// recovered key without revealing it and lets auditors match wraps to
	// recoveries.
	KeyDigest string `json:"key_digest"`
}

// SealDEK wraps dek (32 bytes) for computerID under the escrow public key.
func SealDEK(escrowPub PublicKey, computerID string, dek []byte) (*WrappedKey, error) {
	if len(dek) != chacha20poly1305.KeySize {
		return nil, fmt.Errorf("keyescrow: dek must be %d bytes", chacha20poly1305.KeySize)
	}
	if strings.TrimSpace(computerID) == "" {
		return nil, fmt.Errorf("keyescrow: computer id required")
	}
	ephPriv := make([]byte, 32)
	if _, err := rand.Read(ephPriv); err != nil {
		return nil, fmt.Errorf("keyescrow: read ephemeral key: %w", err)
	}
	shared, err := curve25519.X25519(ephPriv, escrowPub[:])
	if err != nil {
		return nil, fmt.Errorf("keyescrow: ephemeral exchange: %w", err)
	}
	wrapKey := deriveWrapKey(shared, computerID)
	aead, err := chacha20poly1305.NewX(wrapKey)
	if err != nil {
		return nil, fmt.Errorf("keyescrow: aead: %w", err)
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("keyescrow: read nonce: %w", err)
	}
	sealed := aead.Seal(nil, nonce, dek, []byte(computerID))
	digest := sha256.Sum256(dek)
	return &WrappedKey{
		Version:      WrapVersion,
		Protector:    ProtectorCustodian,
		ComputerID:   computerID,
		EphemeralPub: base64.RawStdEncoding.EncodeToString(ephPub(ephPriv)),
		Nonce:        base64.RawStdEncoding.EncodeToString(nonce),
		Ciphertext:   base64.RawStdEncoding.EncodeToString(sealed),
		KeyDigest:    fmt.Sprintf("%x", digest),
	}, nil
}

// OpenDEK opens a custodian wrap with the host escrow private key. The
// computerID must match the record; the record must be version- and
// protector-valid.
func OpenDEK(escrowPriv PrivateKey, record *WrappedKey, computerID string) ([]byte, error) {
	if record == nil {
		return nil, ErrInvalidWrap
	}
	if record.Version != WrapVersion || record.Protector != ProtectorCustodian {
		return nil, fmt.Errorf("%w: unsupported version/protector", ErrInvalidWrap)
	}
	if record.ComputerID != computerID {
		return nil, fmt.Errorf("%w: computer binding mismatch", ErrInvalidWrap)
	}
	ephPub, err := base64.RawStdEncoding.DecodeString(record.EphemeralPub)
	if err != nil || len(ephPub) != 32 {
		return nil, fmt.Errorf("%w: bad ephemeral key", ErrInvalidWrap)
	}
	shared, err := curve25519.X25519(escrowPriv[:], ephPub)
	if err != nil {
		return nil, fmt.Errorf("%w: exchange failed", ErrInvalidWrap)
	}
	wrapKey := deriveWrapKey(shared, computerID)
	aead, err := chacha20poly1305.NewX(wrapKey)
	if err != nil {
		return nil, fmt.Errorf("keyescrow: aead: %w", err)
	}
	nonce, err := base64.RawStdEncoding.DecodeString(record.Nonce)
	if err != nil {
		return nil, fmt.Errorf("%w: bad nonce", ErrInvalidWrap)
	}
	sealed, err := base64.RawStdEncoding.DecodeString(record.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("%w: bad ciphertext", ErrInvalidWrap)
	}
	dek, err := aead.Open(nil, nonce, sealed, []byte(computerID))
	if err != nil {
		return nil, fmt.Errorf("%w: open failed", ErrInvalidWrap)
	}
	digest := sha256.Sum256(dek)
	if fmt.Sprintf("%x", digest) != record.KeyDigest {
		return nil, fmt.Errorf("%w: key digest mismatch", ErrInvalidWrap)
	}
	return dek, nil
}

// ParseWrappedKey decodes and structurally validates a stored wrap record.
func ParseWrappedKey(data []byte) (*WrappedKey, error) {
	var record WrappedKey
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&record); err != nil {
		return nil, fmt.Errorf("%w: decode: %v", ErrInvalidWrap, err)
	}
	if record.Version != WrapVersion || strings.TrimSpace(record.ComputerID) == "" ||
		strings.TrimSpace(record.Ciphertext) == "" || strings.TrimSpace(record.KeyDigest) == "" {
		return nil, fmt.Errorf("%w: incomplete record", ErrInvalidWrap)
	}
	return &record, nil
}

func deriveWrapKey(shared []byte, computerID string) []byte {
	out := make([]byte, chacha20poly1305.KeySize)
	reader := hkdf.New(sha256.New, shared, []byte(computerID), []byte(hkdfInfo))
	if _, err := readFull(reader, out); err != nil {
		// hkdf from a 32-byte shared secret never exhausts for 32 bytes.
		panic(fmt.Sprintf("keyescrow: hkdf: %v", err))
	}
	return out
}

func readFull(reader interface{ Read([]byte) (int, error) }, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := reader.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

func ephPub(ephPriv []byte) []byte {
	pub, err := curve25519.X25519(ephPriv, curve25519.Basepoint)
	if err != nil {
		panic(fmt.Sprintf("keyescrow: derive ephemeral public: %v", err))
	}
	return pub
}

func dirOf(path string) string {
	idx := strings.LastIndex(path, "/")
	if idx <= 0 {
		return "/"
	}
	return path[:idx]
}
