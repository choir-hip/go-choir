// Package filecas defines encrypted, content-addressed file manifests.
package filecas

import (
	"bytes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/hkdf"
)

const (
	manifestVersion = 1
	chunkKeyInfo    = "choir/filecas/v1/chunk"
	chunkNonceInfo  = "choir/filecas/v1/chunk-nonce"
	fileDigestInfo  = "choir/filecas/v1/file-digest"

	// compressedAADSuffix marks a zstd-compressed chunk. It is part of the
	// AEAD's associated data, so the marker is authenticated; nothing sniffs a
	// plaintext prefix.
	compressedAADSuffix = "\x00zstd"
	// maxChunkPlaintext caps decompressed output so a bad frame cannot balloon
	// memory.
	maxChunkPlaintext = 64 << 20
	// minCompressionSavingPercent: a chunk is stored compressed only when the
	// compressed form is at least this much smaller than the raw chunk.
	minCompressionSavingPercent = 5
)

// Failure modes (tests in filecas_test.go): an old raw-sealed chunk must still
// open; an incompressible chunk must seal byte-identically to the old format;
// a compressible chunk must round-trip and shrink; sealing must be
// deterministic (same input twice, same digest); a wrong key or computer is
// refused in both forms; a tampered compressed chunk is refused; a
// decompression bomb is capped, not expanded.
var (
	zstdEncoder = mustZstdEncoder()
	zstdDecoder = mustZstdDecoder(maxChunkPlaintext)
)

func mustZstdEncoder() *zstd.Encoder {
	enc, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedDefault), zstd.WithEncoderConcurrency(1))
	if err != nil {
		panic(err)
	}
	return enc
}

func mustZstdDecoder(maxBytes uint64) *zstd.Decoder {
	dec, err := zstd.NewReader(nil, zstd.WithDecoderMaxMemory(maxBytes), zstd.WithDecoderConcurrency(1))
	if err != nil {
		panic(err)
	}
	return dec
}

func compressedAAD(computerID string) []byte { return []byte(computerID + compressedAADSuffix) }

// FileEntry describes one file in an encrypted file manifest. Chunks address
// the encrypted bytes stored in the CAS, not their plaintext.
type FileEntry struct {
	Path   string   `json:"path"`
	Mode   uint32   `json:"mode"`
	Size   int64    `json:"size"`
	Chunks []string `json:"chunks"`
}

// Manifest is the content-addressed description of a computer's file root.
type Manifest struct {
	Version    int         `json:"version"`
	ComputerID string      `json:"computer_id"`
	Root       string      `json:"root"`
	CreatedAt  string      `json:"created_at"`
	Files      []FileEntry `json:"files"`
}

// ChunkBytes splits data into chunks. Empty data intentionally has no chunks.
func ChunkBytes(data []byte, chunkSize int) [][]byte {
	if chunkSize <= 0 {
		return nil
	}
	chunks := make([][]byte, 0, (len(data)+chunkSize-1)/chunkSize)
	for len(data) > 0 {
		n := chunkSize
		if n > len(data) {
			n = len(data)
		}
		chunks = append(chunks, data[:n])
		data = data[n:]
	}
	return chunks
}

// BuildManifest makes a canonical manifest whose root commits to every field
// other than Root itself.
func BuildManifest(computerID string, entries []FileEntry, now time.Time) (*Manifest, error) {
	if strings.TrimSpace(computerID) == "" {
		return nil, fmt.Errorf("filecas: computer ID is required")
	}
	files := cloneEntries(entries)
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	manifest := &Manifest{
		Version: manifestVersion, ComputerID: computerID, CreatedAt: now.UTC().Format(time.RFC3339Nano), Files: files,
	}
	root, err := manifest.rootDigest()
	if err != nil {
		return nil, err
	}
	manifest.Root = root
	return manifest, nil
}

// ParseManifest decodes a manifest and verifies its content root.
func ParseManifest(data []byte) (*Manifest, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return nil, fmt.Errorf("filecas: parse manifest: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("filecas: parse manifest: trailing JSON")
	}
	if err := manifest.VerifyRoot(); err != nil {
		return nil, err
	}
	return &manifest, nil
}

// VerifyRoot verifies the root and canonical ordering of file entries.
func (m *Manifest) VerifyRoot() error {
	if m == nil || m.Version != manifestVersion || strings.TrimSpace(m.ComputerID) == "" || strings.TrimSpace(m.Root) == "" || m.CreatedAt == "" {
		return fmt.Errorf("filecas: invalid manifest")
	}
	if _, err := time.Parse(time.RFC3339Nano, m.CreatedAt); err != nil {
		return fmt.Errorf("filecas: invalid manifest timestamp: %w", err)
	}
	for i, entry := range m.Files {
		if entry.Path == "" || entry.Size < 0 || (i > 0 && m.Files[i-1].Path >= entry.Path) {
			return fmt.Errorf("filecas: invalid file entry ordering")
		}
		for _, chunk := range entry.Chunks {
			if !validDigest(chunk) {
				return fmt.Errorf("filecas: invalid chunk digest")
			}
		}
	}
	root, err := m.rootDigest()
	if err != nil {
		return err
	}
	if m.Root != root {
		return fmt.Errorf("filecas: manifest root mismatch")
	}
	return nil
}

func (m *Manifest) rootDigest() (string, error) {
	unsigned := struct {
		Version    int         `json:"version"`
		ComputerID string      `json:"computer_id"`
		CreatedAt  string      `json:"created_at"`
		Files      []FileEntry `json:"files"`
	}{Version: m.Version, ComputerID: m.ComputerID, CreatedAt: m.CreatedAt, Files: m.Files}
	canonical, err := computerevent.CanonicalJSON(unsigned)
	if err != nil {
		return "", fmt.Errorf("filecas: canonical manifest: %w", err)
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}

// SealChunk encrypts one chunk and returns the nonce-prefixed ciphertext and
// its SHA-256 CAS digest.
//
// The nonce is synthetic: HMAC-SHA256 of the plaintext under a key derived
// from the DEK and computer ID (separate from the encryption key), truncated
// to the 24-byte XChaCha20 nonce. Sealing is therefore deterministic per
// (DEK, computer, plaintext): an unchanged chunk re-sealed after a guest
// reboot has the same CAS digest and the platform stores it once. A random
// nonce made every boot re-store the whole Files tree
// (docs/problems/artifact-store-grows-12gb-a-day-2026-10-14.md). Distinct
// plaintexts get distinct nonces except with negligible (2^-96 birthday)
// probability, so the AEAD never reuses a nonce across messages. The cost is
// that the CAS holder learns when two chunks of one computer are identical;
// it already stores that computer's plaintext manifests (paths, sizes, chunk
// lists). Nothing is shared across computers or keys. Chunks sealed earlier
// with random nonces still open: OpenChunk reads the nonce from the prefix.
func SealChunk(dek []byte, computerID string, chunk []byte) ([]byte, string, error) {
	nonce, err := chunkNonce(dek, computerID, chunk)
	if err != nil {
		return nil, "", err
	}
	// Compress first; keep the raw form (byte-identical to the original
	// format) unless the saving is worth it. The nonce is over the original
	// plaintext either way, so sealing stays deterministic.
	if len(chunk) > 0 && compressChunkWrites {
		comp := zstdEncoder.EncodeAll(chunk, nil)
		if len(comp) <= len(chunk)-len(chunk)*minCompressionSavingPercent/100 {
			return sealChunkWithNonce(dek, computerID, nonce, comp, compressedAAD(computerID))
		}
	}
	return sealChunkWithNonce(dek, computerID, nonce, chunk, []byte(computerID))
}

// FileDigestKey derives the HMAC-SHA-256 key for the file object's digest from
// the DEK and computer ID, separate from the chunk encryption key and the chunk
// nonce key. The digest of a file's bytes under this key identifies the bytes
// to the computer (equal bytes, equal digest) without letting a holder of the
// tape confirm a guessed plaintext.
func FileDigestKey(dek []byte, computerID string) ([]byte, error) {
	if len(dek) != chacha20poly1305.KeySize || strings.TrimSpace(computerID) == "" {
		return nil, fmt.Errorf("filecas: invalid key or computer ID")
	}
	key := make([]byte, sha256.Size)
	reader := hkdf.New(sha256.New, dek, []byte(computerID), []byte(fileDigestInfo))
	if _, err := io.ReadFull(reader, key); err != nil {
		return nil, fmt.Errorf("filecas: derive file digest key: %w", err)
	}
	return key, nil
}

// chunkNonce derives the synthetic nonce for chunk.
func chunkNonce(dek []byte, computerID string, chunk []byte) ([]byte, error) {
	if len(dek) != chacha20poly1305.KeySize || strings.TrimSpace(computerID) == "" {
		return nil, fmt.Errorf("filecas: invalid key or computer ID")
	}
	key := make([]byte, sha256.Size)
	defer clear(key)
	reader := hkdf.New(sha256.New, dek, []byte(computerID), []byte(chunkNonceInfo))
	if _, err := io.ReadFull(reader, key); err != nil {
		return nil, fmt.Errorf("filecas: derive chunk nonce key: %w", err)
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(chunk)
	return mac.Sum(nil)[:chacha20poly1305.NonceSizeX], nil
}

func sealChunkWithNonce(dek []byte, computerID string, nonce, chunk, aad []byte) ([]byte, string, error) {
	aead, err := chunkCipher(dek, computerID)
	if err != nil {
		return nil, "", err
	}
	if len(nonce) != chacha20poly1305.NonceSizeX {
		return nil, "", fmt.Errorf("filecas: invalid nonce size")
	}
	sealed := make([]byte, 0, len(nonce)+len(chunk)+aead.Overhead())
	sealed = append(sealed, nonce...)
	sealed = aead.Seal(sealed, nonce, chunk, aad)
	sum := sha256.Sum256(sealed)
	return sealed, hex.EncodeToString(sum[:]), nil
}

// OpenChunk verifies the CAS digest then decrypts a nonce-prefixed chunk.
func OpenChunk(dek []byte, computerID, digest string, sealed []byte) ([]byte, error) {
	if !validDigest(digest) {
		return nil, fmt.Errorf("filecas: invalid chunk digest")
	}
	sum := sha256.Sum256(sealed)
	if digest != hex.EncodeToString(sum[:]) {
		return nil, fmt.Errorf("filecas: chunk digest mismatch")
	}
	aead, err := chunkCipher(dek, computerID)
	if err != nil {
		return nil, err
	}
	if len(sealed) < chacha20poly1305.NonceSizeX+aead.Overhead() {
		return nil, fmt.Errorf("filecas: encrypted chunk too short")
	}
	nonce, body := sealed[:chacha20poly1305.NonceSizeX], sealed[chacha20poly1305.NonceSizeX:]
	if comp, err := aead.Open(nil, nonce, body, compressedAAD(computerID)); err == nil {
		plain, derr := zstdDecoder.DecodeAll(comp, nil)
		if derr != nil {
			return nil, fmt.Errorf("filecas: decompress chunk: %w", derr)
		}
		return plain, nil
	}
	return aead.Open(nil, nonce, body, []byte(computerID))
}

func chunkCipher(dek []byte, computerID string) (cipher.AEAD, error) {
	if len(dek) != chacha20poly1305.KeySize || strings.TrimSpace(computerID) == "" {
		return nil, fmt.Errorf("filecas: invalid key or computer ID")
	}
	key := make([]byte, chacha20poly1305.KeySize)
	reader := hkdf.New(sha256.New, dek, []byte(computerID), []byte(chunkKeyInfo))
	if _, err := io.ReadFull(reader, key); err != nil {
		return nil, fmt.Errorf("filecas: derive chunk key: %w", err)
	}
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, fmt.Errorf("filecas: chunk cipher: %w", err)
	}
	return aead, nil
}

func cloneEntries(entries []FileEntry) []FileEntry {
	files := make([]FileEntry, len(entries))
	for i, entry := range entries {
		files[i] = entry
		files[i].Chunks = append([]string(nil), entry.Chunks...)
	}
	return files
}

func validDigest(digest string) bool {
	if len(digest) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(digest)
	return err == nil
}

// compressChunkWrites holds compressed writes back for one release, so the
// release that can open a compressed chunk is deployed (and is the rollback
// target) before any chunk is sealed compressed. OpenChunk always accepts
// both forms.
var compressChunkWrites = false
