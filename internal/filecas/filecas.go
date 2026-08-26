// Package filecas defines encrypted, content-addressed file manifests.
package filecas

import (
	"bytes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/hkdf"
)

const (
	manifestVersion = 1
	chunkKeyInfo    = "choir/filecas/v1/chunk"
)

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
func SealChunk(dek []byte, computerID string, chunk []byte) ([]byte, string, error) {
	aead, err := chunkCipher(dek, computerID)
	if err != nil {
		return nil, "", err
	}
	nonce := make([]byte, chacha20poly1305.NonceSizeX)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, "", fmt.Errorf("filecas: random nonce: %w", err)
	}
	sealed := make([]byte, 0, len(nonce)+len(chunk)+aead.Overhead())
	sealed = append(sealed, nonce...)
	sealed = aead.Seal(sealed, nonce, chunk, []byte(computerID))
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
	return aead.Open(nil, sealed[:chacha20poly1305.NonceSizeX], sealed[chacha20poly1305.NonceSizeX:], []byte(computerID))
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
