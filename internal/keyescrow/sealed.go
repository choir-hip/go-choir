package keyescrow

// Sealed escrow private key (station FP9, NET-06; docs/definitions/
// choir-launch-fp9-keys-recovery-2026-10-21.md, "The escrow key"). The host
// escrow private key may rest as a JSON record sealed with
// XChaCha20-Poly1305 under a 32-byte key encryption key (KEK) that lives
// outside the artifacts volume and outside the backup. The purpose and the
// public-key fingerprint are associated data, so a record for another
// purpose or with a swapped fingerprint does not open.
//
// CHOIR_KEY_ESCROW_SEAL selects the load rule: off (default, exactly the
// LoadOrGeneratePrivateKey behavior), permit (sealed or plaintext, the
// caller logs which) or require (plaintext refused). Under permit and
// require a missing key file is an error, never a new key. Errors never
// carry key material.

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
)

const (
	// SealedKeyVersion is the schema version of a sealed key file.
	SealedKeyVersion = 1
	// SealPurposeEscrowPrivateKey names the escrow private key in a sealed record.
	SealPurposeEscrowPrivateKey = "choir/keyescrow/v1/escrow-private-key"

	// EnvSealMode selects off, permit or require.
	EnvSealMode = "CHOIR_KEY_ESCROW_SEAL"
	// EnvKEKPath is the absolute path of the KEK file (a systemd credential
	// under the units).
	EnvKEKPath = "CHOIR_KEY_ESCROW_KEK_PATH"

	// KEKNamePrefix is the name every KEK file must start with: the daily
	// backup (scripts/off-host-backup.sh) excludes "key-escrow-kek*" by name,
	// so a KEK under any other name would ride in the backup beside the
	// sealed key (finding B3). A systemd credential named key-escrow-kek
	// satisfies it as %d/key-escrow-kek.
	KEKNamePrefix = "key-escrow-kek"

	kekSize = chacha20poly1305.KeySize
	// maxKeyFileSize bounds a read of a key or KEK file.
	maxKeyFileSize = 4096
)

// ErrSealedKey marks a sealed key record that is malformed, for another
// purpose, or does not open under the KEK given.
var ErrSealedKey = errors.New("keyescrow: sealed key does not open")

// SealMode is the load rule for the escrow private key file.
type SealMode string

const (
	SealOff     SealMode = "off"
	SealPermit  SealMode = "permit"
	SealRequire SealMode = "require"
)

// ParseSealMode reads a CHOIR_KEY_ESCROW_SEAL value. Empty is off; an
// unknown value is an error, never off.
func ParseSealMode(raw string) (SealMode, error) {
	switch strings.TrimSpace(raw) {
	case "", string(SealOff):
		return SealOff, nil
	case string(SealPermit):
		return SealPermit, nil
	case string(SealRequire):
		return SealRequire, nil
	}
	return "", fmt.Errorf("keyescrow: %s must be off, permit or require", EnvSealMode)
}

// SealConfig is the load rule and the KEK location.
type SealConfig struct {
	Mode    SealMode
	KEKPath string
}

// SealConfigFromEnv reads CHOIR_KEY_ESCROW_SEAL and CHOIR_KEY_ESCROW_KEK_PATH.
// permit and require need an absolute KEK path.
func SealConfigFromEnv(getenv func(string) string) (SealConfig, error) {
	mode, err := ParseSealMode(getenv(EnvSealMode))
	if err != nil {
		return SealConfig{}, err
	}
	cfg := SealConfig{Mode: mode, KEKPath: strings.TrimSpace(getenv(EnvKEKPath))}
	if mode == SealOff {
		return cfg, nil
	}
	if cfg.KEKPath == "" || !filepath.IsAbs(cfg.KEKPath) {
		return SealConfig{}, fmt.Errorf("keyescrow: %s=%s needs an absolute %s", EnvSealMode, mode, EnvKEKPath)
	}
	return cfg, nil
}

// KeyFileState is what a load or an inspection found at the key path.
type KeyFileState string

const (
	KeyFileMissing   KeyFileState = "missing"
	KeyFilePlaintext KeyFileState = "plaintext"
	KeyFileSealed    KeyFileState = "sealed"
	// KeyFileGenerated is reported only under off, when the file was absent
	// and a new key was written (today's behavior).
	KeyFileGenerated KeyFileState = "generated"
)

// Public derives the X25519 public key.
func (p PrivateKey) Public() (PublicKey, error) {
	pub, err := curve25519.X25519(p[:], curve25519.Basepoint)
	if err != nil {
		return PublicKey{}, fmt.Errorf("keyescrow: derive public key")
	}
	var out PublicKey
	copy(out[:], pub)
	return out, nil
}

// Fingerprint is the hex SHA-256 of a public key. It is public.
func Fingerprint(pub PublicKey) string {
	sum := sha256.Sum256(pub[:])
	return hex.EncodeToString(sum[:])
}

type sealedKeyRecord struct {
	Version         int    `json:"version"`
	Purpose         string `json:"purpose"`
	PublicKeySHA256 string `json:"public_key_sha256"`
	Nonce           string `json:"nonce"`
	Ciphertext      string `json:"ciphertext"`
}

func sealedAD(purpose, fingerprint string) []byte {
	return []byte(fmt.Sprintf("choir/keyescrow/sealed/v%d\x00%s\x00%s", SealedKeyVersion, purpose, fingerprint))
}

// SealPrivateKey returns the sealed JSON record of priv under kek.
func SealPrivateKey(priv PrivateKey, kek []byte) ([]byte, error) {
	return sealPrivateKeyForPurpose(priv, kek, SealPurposeEscrowPrivateKey)
}

func sealPrivateKeyForPurpose(priv PrivateKey, kek []byte, purpose string) ([]byte, error) {
	if len(kek) != kekSize {
		return nil, fmt.Errorf("keyescrow: KEK must be %d bytes", kekSize)
	}
	pub, err := priv.Public()
	if err != nil {
		return nil, err
	}
	fp := Fingerprint(pub)
	aead, err := chacha20poly1305.NewX(kek)
	if err != nil {
		return nil, fmt.Errorf("keyescrow: KEK aead")
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("keyescrow: read nonce: %w", err)
	}
	ct := aead.Seal(nil, nonce, priv[:], sealedAD(purpose, fp))
	out, err := json.Marshal(sealedKeyRecord{
		Version:         SealedKeyVersion,
		Purpose:         purpose,
		PublicKeySHA256: fp,
		Nonce:           base64.RawStdEncoding.EncodeToString(nonce),
		Ciphertext:      base64.RawStdEncoding.EncodeToString(ct),
	})
	if err != nil {
		return nil, fmt.Errorf("keyescrow: encode sealed key")
	}
	return append(out, '\n'), nil
}

func parseSealedRecord(data []byte) (sealedKeyRecord, error) {
	var rec sealedKeyRecord
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&rec); err != nil {
		return sealedKeyRecord{}, fmt.Errorf("%w: malformed record", ErrSealedKey)
	}
	if rec.Version != SealedKeyVersion {
		return sealedKeyRecord{}, fmt.Errorf("%w: unsupported version", ErrSealedKey)
	}
	if rec.Purpose != SealPurposeEscrowPrivateKey {
		return sealedKeyRecord{}, fmt.Errorf("%w: record is not an escrow private key", ErrSealedKey)
	}
	if len(rec.PublicKeySHA256) != 64 {
		return sealedKeyRecord{}, fmt.Errorf("%w: bad fingerprint", ErrSealedKey)
	}
	return rec, nil
}

// OpenSealedPrivateKey opens a sealed record with kek and checks that the
// key matches the recorded fingerprint.
func OpenSealedPrivateKey(data, kek []byte) (PrivateKey, error) {
	if len(kek) != kekSize {
		return PrivateKey{}, fmt.Errorf("%w: KEK must be %d bytes", ErrSealedKey, kekSize)
	}
	rec, err := parseSealedRecord(data)
	if err != nil {
		return PrivateKey{}, err
	}
	nonce, err := base64.RawStdEncoding.DecodeString(rec.Nonce)
	if err != nil || len(nonce) != chacha20poly1305.NonceSizeX {
		return PrivateKey{}, fmt.Errorf("%w: bad nonce", ErrSealedKey)
	}
	ct, err := base64.RawStdEncoding.DecodeString(rec.Ciphertext)
	if err != nil {
		return PrivateKey{}, fmt.Errorf("%w: bad ciphertext", ErrSealedKey)
	}
	aead, err := chacha20poly1305.NewX(kek)
	if err != nil {
		return PrivateKey{}, fmt.Errorf("%w: KEK aead", ErrSealedKey)
	}
	plain, err := aead.Open(nil, nonce, ct, sealedAD(rec.Purpose, rec.PublicKeySHA256))
	if err != nil || len(plain) != 32 {
		clear(plain)
		return PrivateKey{}, fmt.Errorf("%w: authentication failed (wrong KEK or edited file)", ErrSealedKey)
	}
	var priv PrivateKey
	copy(priv[:], plain)
	clear(plain)
	pub, err := priv.Public()
	if err != nil || Fingerprint(pub) != rec.PublicKeySHA256 {
		clear(priv[:])
		return PrivateKey{}, fmt.Errorf("%w: fingerprint mismatch", ErrSealedKey)
	}
	return priv, nil
}

// checkKEKPath requires an absolute path whose file name starts with
// KEKNamePrefix.
func checkKEKPath(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("keyescrow: KEK path must be absolute")
	}
	if !strings.HasPrefix(filepath.Base(path), KEKNamePrefix) {
		return fmt.Errorf("keyescrow: KEK file name must start with %q (the backup excludes that name)", KEKNamePrefix)
	}
	return nil
}

// readBounded reads a regular file without following a final symlink and
// without blocking on a FIFO or device: open, fstat, refuse non-regular.
func readBounded(path string) ([]byte, os.FileInfo, error) {
	// O_NONBLOCK: a FIFO or device at the path is opened at once and refused
	// by the type check below instead of hanging the caller (bunch 31, low).
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("not a regular file")
	}
	data, err := io.ReadAll(io.LimitReader(f, maxKeyFileSize+1))
	if err != nil {
		return nil, nil, err
	}
	if len(data) > maxKeyFileSize {
		clear(data)
		return nil, nil, fmt.Errorf("file too large")
	}
	return data, info, nil
}

// LoadKEK reads a 32-byte KEK (raw, or base64 text) from an owner-only
// regular file that is not a symlink.
func LoadKEK(path string) ([]byte, error) {
	if err := checkKEKPath(path); err != nil {
		return nil, err
	}
	data, info, err := readBounded(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("keyescrow: KEK file absent")
		}
		return nil, fmt.Errorf("keyescrow: read KEK: %v", sanitizeFileErr(err))
	}
	defer clear(data)
	if info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("keyescrow: KEK file mode %#o allows group or other access (want 0400 or 0600)", info.Mode().Perm())
	}
	if len(data) == kekSize {
		return bytes.Clone(data), nil
	}
	text := strings.TrimSpace(string(data))
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding} {
		if decoded, err := enc.DecodeString(text); err == nil {
			if len(decoded) == kekSize {
				return decoded, nil
			}
			clear(decoded)
		}
	}
	return nil, fmt.Errorf("keyescrow: KEK file must hold %d bytes, raw or base64", kekSize)
}

// GenerateKEKFile writes a new random KEK as base64 text, mode 0400, and
// refuses to replace an existing file.
func GenerateKEKFile(path string) error {
	if err := checkKEKPath(path); err != nil {
		return err
	}
	kek := make([]byte, kekSize)
	defer clear(kek)
	if _, err := rand.Read(kek); err != nil {
		return fmt.Errorf("keyescrow: read KEK: %w", err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0o400)
	if err != nil {
		return fmt.Errorf("keyescrow: create KEK file: %v", sanitizeFileErr(err))
	}
	_, err = f.Write([]byte(base64.StdEncoding.EncodeToString(kek) + "\n"))
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("keyescrow: write KEK file: %v", sanitizeFileErr(err))
	}
	return syncDir(filepath.Dir(path))
}

func isSealedText(data []byte) bool {
	return bytes.HasPrefix(bytes.TrimSpace(data), []byte("{"))
}

func decodePlainKey(data []byte) (PrivateKey, error) {
	decoded, err := base64.RawStdEncoding.DecodeString(strings.TrimSpace(string(data)))
	if err != nil || len(decoded) != 32 {
		clear(decoded)
		return PrivateKey{}, fmt.Errorf("keyescrow: invalid escrow private key file")
	}
	var priv PrivateKey
	copy(priv[:], decoded)
	clear(decoded)
	return priv, nil
}

// LoadPrivateKey loads the escrow private key under cfg. Under off it is
// LoadOrGeneratePrivateKey except that a sealed file is refused by name.
func LoadPrivateKey(path string, cfg SealConfig) (PrivateKey, KeyFileState, error) {
	if !filepath.IsAbs(path) {
		return PrivateKey{}, "", fmt.Errorf("keyescrow: private key path must be absolute")
	}
	switch cfg.Mode {
	case SealOff, "":
		raw, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			priv, err := LoadOrGeneratePrivateKey(path)
			if err != nil {
				return PrivateKey{}, "", err
			}
			return priv, KeyFileGenerated, nil
		}
		if err != nil {
			return PrivateKey{}, "", fmt.Errorf("keyescrow: read escrow private key: %v", sanitizeFileErr(err))
		}
		defer clear(raw)
		if isSealedText(raw) {
			return PrivateKey{}, "", fmt.Errorf("keyescrow: escrow private key file is sealed; set %s=permit or require with %s", EnvSealMode, EnvKEKPath)
		}
		priv, err := decodePlainKey(raw)
		if err != nil {
			return PrivateKey{}, "", err
		}
		return priv, KeyFilePlaintext, nil
	case SealPermit, SealRequire:
	default:
		return PrivateKey{}, "", fmt.Errorf("keyescrow: unknown seal mode")
	}
	// The KEK is loaded first in both modes, so a broken KEK is found at the
	// first start under permit, not at the first seal.
	kek, err := LoadKEK(cfg.KEKPath)
	if err != nil {
		return PrivateKey{}, "", err
	}
	defer clear(kek)
	raw, _, err := readBounded(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return PrivateKey{}, "", fmt.Errorf("keyescrow: escrow private key file absent; %s=%s never generates a key", EnvSealMode, cfg.Mode)
		}
		return PrivateKey{}, "", fmt.Errorf("keyescrow: read escrow private key: %v", sanitizeFileErr(err))
	}
	defer clear(raw)
	if isSealedText(raw) {
		priv, err := OpenSealedPrivateKey(raw, kek)
		if err != nil {
			return PrivateKey{}, "", err
		}
		return priv, KeyFileSealed, nil
	}
	if cfg.Mode == SealRequire {
		return PrivateKey{}, "", fmt.Errorf("keyescrow: escrow private key file is plaintext and %s=require", EnvSealMode)
	}
	priv, err := decodePlainKey(raw)
	if err != nil {
		return PrivateKey{}, "", err
	}
	return priv, KeyFilePlaintext, nil
}

// LoadConfiguredPrivateKey reads the seal configuration from getenv and
// loads the key; the service entry points call it.
func LoadConfiguredPrivateKey(path string, getenv func(string) string) (PrivateKey, KeyFileState, SealMode, error) {
	cfg, err := SealConfigFromEnv(getenv)
	if err != nil {
		return PrivateKey{}, "", "", err
	}
	priv, state, err := LoadPrivateKey(path, cfg)
	return priv, state, cfg.Mode, err
}

// Inspection is what `escrowkey check` reports. Nothing in it is secret.
type Inspection struct {
	State           KeyFileState
	PublicKeySHA256 string
	// Opens is true when the key was decoded (plaintext) or opened under
	// the KEK given (sealed).
	Opens bool
	// Reason names why a present file did not open.
	Reason string
}

// InspectKeyFile reports the state of the key file. kek may be nil; a
// sealed file then reports its recorded fingerprint and Opens=false.
func InspectKeyFile(path string, kek []byte) (Inspection, error) {
	if !filepath.IsAbs(path) {
		return Inspection{}, fmt.Errorf("keyescrow: private key path must be absolute")
	}
	raw, _, err := readBounded(path)
	if errors.Is(err, os.ErrNotExist) {
		return Inspection{State: KeyFileMissing}, nil
	}
	if err != nil {
		return Inspection{}, fmt.Errorf("keyescrow: read escrow private key: %v", sanitizeFileErr(err))
	}
	defer clear(raw)
	if isSealedText(raw) {
		rec, err := parseSealedRecord(raw)
		if err != nil {
			return Inspection{State: KeyFileSealed, Reason: err.Error()}, nil
		}
		insp := Inspection{State: KeyFileSealed, PublicKeySHA256: rec.PublicKeySHA256}
		if kek == nil {
			insp.Reason = "no KEK given"
			return insp, nil
		}
		priv, err := OpenSealedPrivateKey(raw, kek)
		if err != nil {
			insp.Reason = err.Error()
			return insp, nil
		}
		clear(priv[:])
		insp.Opens = true
		return insp, nil
	}
	priv, err := decodePlainKey(raw)
	if err != nil {
		return Inspection{State: KeyFilePlaintext, Reason: err.Error()}, nil
	}
	defer clear(priv[:])
	pub, err := priv.Public()
	if err != nil {
		return Inspection{State: KeyFilePlaintext, Reason: err.Error()}, nil
	}
	return Inspection{State: KeyFilePlaintext, PublicKeySHA256: Fingerprint(pub), Opens: true}, nil
}

// SealKeyFile replaces a plaintext key file with its sealed form. The new
// file is written beside the old, read back and opened, and its key must
// have the same public key before the rename. Returns the fingerprint.
func SealKeyFile(path string, kek []byte) (string, error) {
	// Read directly: the off loader would generate a key for a missing file.
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("keyescrow: private key path must be absolute")
	}
	raw, _, err := readBounded(path)
	if err != nil {
		return "", fmt.Errorf("keyescrow: seal needs an existing plaintext key file: %v", sanitizeFileErr(err))
	}
	defer clear(raw)
	if isSealedText(raw) {
		return "", fmt.Errorf("keyescrow: key file is already sealed")
	}
	priv, err := decodePlainKey(raw)
	if err != nil {
		return "", err
	}
	defer clear(priv[:])
	pub, err := priv.Public()
	if err != nil {
		return "", err
	}
	want := Fingerprint(pub)
	sealed, err := SealPrivateKey(priv, kek)
	if err != nil {
		return "", err
	}
	verify := func(data []byte) error {
		got, err := OpenSealedPrivateKey(data, kek)
		if err != nil {
			return err
		}
		defer clear(got[:])
		if got != priv {
			return fmt.Errorf("keyescrow: sealed key differs from the original")
		}
		return nil
	}
	if err := replaceVerified(path, sealed, verify); err != nil {
		return "", err
	}
	return want, nil
}

// UnsealKeyFile replaces a sealed key file with the plaintext form (the
// rollback path). It refuses unless the file opens under kek.
func UnsealKeyFile(path string, kek []byte) (string, error) {
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("keyescrow: private key path must be absolute")
	}
	raw, _, err := readBounded(path)
	if err != nil {
		return "", fmt.Errorf("keyescrow: read escrow private key: %v", sanitizeFileErr(err))
	}
	defer clear(raw)
	if !isSealedText(raw) {
		return "", fmt.Errorf("keyescrow: unseal needs a sealed key file")
	}
	priv, err := OpenSealedPrivateKey(raw, kek)
	if err != nil {
		return "", err
	}
	defer clear(priv[:])
	pub, err := priv.Public()
	if err != nil {
		return "", err
	}
	plain := []byte(base64.RawStdEncoding.EncodeToString(priv[:]) + "\n")
	defer clear(plain)
	verify := func(data []byte) error {
		got, err := decodePlainKey(data)
		if err != nil {
			return err
		}
		defer clear(got[:])
		if got != priv {
			return fmt.Errorf("keyescrow: unsealed key differs from the original")
		}
		return nil
	}
	if err := replaceVerified(path, plain, verify); err != nil {
		return "", err
	}
	return Fingerprint(pub), nil
}

// replaceVerified writes data to a temporary file beside path (0600), syncs
// it, reads it back through verify, then renames it over path and syncs the
// directory. On any failure the original file is untouched.
func replaceVerified(path string, data []byte, verify func([]byte) error) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("keyescrow: create temporary key file: %v", sanitizeFileErr(err))
	}
	tmpPath := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("keyescrow: chmod temporary key file")
	}
	_, err = tmp.Write(data)
	if err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("keyescrow: write temporary key file: %v", sanitizeFileErr(err))
	}
	back, _, err := readBounded(tmpPath)
	if err != nil {
		return fmt.Errorf("keyescrow: read back temporary key file: %v", sanitizeFileErr(err))
	}
	verifyErr := verify(back)
	clear(back)
	if verifyErr != nil {
		return verifyErr
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("keyescrow: replace key file: %v", sanitizeFileErr(err))
	}
	ok = true
	return syncDir(dir)
}

func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("keyescrow: open key dir: %v", sanitizeFileErr(err))
	}
	defer d.Close()
	if err := d.Sync(); err != nil {
		return fmt.Errorf("keyescrow: sync key dir: %v", sanitizeFileErr(err))
	}
	return nil
}

// sanitizeFileErr keeps the operation and errno of a path error; file
// contents never reach these errors, the path is not secret.
func sanitizeFileErr(err error) error {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return fmt.Errorf("%s %s: %v", pe.Op, pe.Path, pe.Err)
	}
	return err
}
