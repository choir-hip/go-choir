package computerevent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrPayloadResolverRequired = errors.New("computer event payload: artifact reader is required before projection")
	ErrPayloadDigestMismatch   = errors.New("computer event payload: artifact digest mismatch")
	ErrPayloadPrivacyMismatch  = errors.New("computer event payload: privacy class mismatch")
	ErrPayloadSQLBeforeResolve = errors.New("computer event payload: SQL projection opened before payloads were resolved")
)

// ArtifactReader fetches a pinned payload by digest. Implementations must not
// open a VM-local SQL transaction. HTTP fetch and decrypt happen here, then
// Project receives only verified bytes.
type ArtifactReader interface {
	FetchPayload(ctx context.Context, computerID, artifactDigest string) ([]byte, error)
}

var _ ArtifactReader = (*HTTPClient)(nil)
var _ ArtifactReader = (*MemoryArtifactReader)(nil)

// PayloadRef names one payload artifact bound to an event. PrivacyClass
// "private" requires Decrypt against (computerID, eventID) before Project.
type PayloadRef struct {
	ArtifactDigest string `json:"artifact_digest"`
	MediaType      string `json:"media_type"`
	PrivacyClass   string `json:"privacy_class"`
	Role           string `json:"role"`
	SchemaVersion  int    `json:"schema_version"`
}

// ResolvedPayload is the authenticated plaintext (or public bytes) ready for
// a SQL-only Project. Network and decrypt must already have succeeded.
type ResolvedPayload struct {
	Ref            PayloadRef
	Plaintext      []byte
	EnvelopeDigest string
}

// ResolvePayloads fetches, hash-verifies, and decrypts every payload *before*
// BeginTx. A nil reader is refused when refs are present. cipher may be nil
// only when every ref is non-private.
func ResolvePayloads(ctx context.Context, reader ArtifactReader, cipher *PrivateArtifactCipher, computerID, eventID string, refs []PayloadRef) ([]ResolvedPayload, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	computerID = strings.TrimSpace(computerID)
	eventID = strings.TrimSpace(eventID)
	if computerID == "" || eventID == "" {
		return nil, fmt.Errorf("computer event payload: computer and event identity are required")
	}
	if len(refs) == 0 {
		return []ResolvedPayload{}, nil
	}
	if reader == nil {
		return nil, ErrPayloadResolverRequired
	}
	out := make([]ResolvedPayload, 0, len(refs))
	for _, ref := range refs {
		ref.ArtifactDigest = strings.TrimSpace(ref.ArtifactDigest)
		ref.MediaType = strings.TrimSpace(ref.MediaType)
		ref.PrivacyClass = strings.TrimSpace(ref.PrivacyClass)
		ref.Role = strings.TrimSpace(ref.Role)
		if !IsSHA256(ref.ArtifactDigest) || ref.MediaType == "" || ref.PrivacyClass == "" || ref.Role == "" || ref.SchemaVersion < 1 {
			return nil, fmt.Errorf("computer event payload: complete payload ref is required")
		}
		raw, err := reader.FetchPayload(ctx, computerID, ref.ArtifactDigest)
		if err != nil {
			return nil, fmt.Errorf("computer event payload: fetch %s: %w", ref.ArtifactDigest, err)
		}
		if DigestBytes(raw) != ref.ArtifactDigest {
			return nil, fmt.Errorf("%w: %s", ErrPayloadDigestMismatch, ref.ArtifactDigest)
		}
		resolved := ResolvedPayload{Ref: ref, EnvelopeDigest: ref.ArtifactDigest, Plaintext: raw}
		switch ref.PrivacyClass {
		case "private":
			if cipher == nil {
				return nil, fmt.Errorf("%w: private payload requires guest cipher", ErrPayloadPrivacyMismatch)
			}
			plaintext, meta, err := cipher.Decrypt(ctx, raw, computerID, eventID)
			if err != nil {
				return nil, fmt.Errorf("computer event payload: decrypt: %w", err)
			}
			if meta.PrivacyClass != "private" {
				return nil, fmt.Errorf("%w: envelope metadata", ErrPayloadPrivacyMismatch)
			}
			resolved.Plaintext = plaintext
		default:
			if ref.PrivacyClass == "private" {
				return nil, fmt.Errorf("%w: unreachable", ErrPayloadPrivacyMismatch)
			}
		}
		out = append(out, resolved)
	}
	return out, nil
}

// MemoryArtifactReader is a test/double store. It is not a second durable log.
type MemoryArtifactReader struct {
	payloads map[string][]byte
}

func NewMemoryArtifactReader() *MemoryArtifactReader {
	return &MemoryArtifactReader{payloads: map[string][]byte{}}
}

func (m *MemoryArtifactReader) Put(payload []byte) string {
	digest := DigestBytes(payload)
	if m.payloads == nil {
		m.payloads = map[string][]byte{}
	}
	m.payloads[digest] = bytes.Clone(payload)
	return digest
}

func (m *MemoryArtifactReader) FetchPayload(_ context.Context, _, artifactDigest string) ([]byte, error) {
	if m == nil {
		return nil, ErrPayloadResolverRequired
	}
	payload, ok := m.payloads[artifactDigest]
	if !ok {
		return nil, fmt.Errorf("computer event payload: artifact %s not found", artifactDigest)
	}
	return bytes.Clone(payload), nil
}
