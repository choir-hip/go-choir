// Package objectgraph provides a small storage-agnostic object graph foundation.
//
// The package is intentionally not wired into runtime routes yet. It defines
// stable object identity, content hash, edge, registry, memory, and SQLite
// persistence semantics that later News, Autoradio, Qdrant, and Base work can
// build on without inventing a parallel model.
package objectgraph

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type ObjectKind string
type EdgeKind string

type Object struct {
	CanonicalID  string          `json:"canonical_id"`
	ObjectKind   ObjectKind      `json:"object_kind"`
	OwnerID      string          `json:"owner_id"`
	ComputerID   string          `json:"computer_id,omitempty"`
	VersionID    string          `json:"version_id,omitempty"`
	ContentHash  string          `json:"content_hash"`
	Body         []byte          `json:"body,omitempty"`
	Metadata     json.RawMessage `json:"metadata"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
	Tombstone    bool            `json:"tombstone,omitempty"`
	SupersededBy string          `json:"superseded_by,omitempty"`
}

type Edge struct {
	EdgeID    string          `json:"edge_id"`
	FromID    string          `json:"from_id"`
	ToID      string          `json:"to_id"`
	Kind      EdgeKind        `json:"kind"`
	Metadata  json.RawMessage `json:"metadata"`
	CreatedAt time.Time       `json:"created_at"`
	Tombstone bool            `json:"tombstone,omitempty"`
}

type ListFilter struct {
	Kind      ObjectKind
	OwnerID   string
	Limit     int
	Tombstone *bool
}

type EdgeFilter struct {
	FromID    string
	ToID      string
	Kind      EdgeKind
	Limit     int
	Tombstone *bool
}

var ErrMalformedID = errors.New("objectgraph: malformed canonical id")

func BuildCanonicalID(kind ObjectKind, ownerID, suffix string) (string, error) {
	if err := validateKind(kind); err != nil {
		return "", err
	}
	if strings.TrimSpace(ownerID) == "" {
		return "", fmt.Errorf("owner_id is required")
	}
	if strings.TrimSpace(suffix) == "" || strings.ContainsAny(suffix, ":/\\?#[]@!$&'()*+,;=") {
		return "", fmt.Errorf("suffix must be non-empty and URL-safe")
	}
	return "obj:" + string(kind) + ":" + encodeIDPart(ownerID) + ":" + suffix, nil
}

func ParseCanonicalID(id string) (kind ObjectKind, ownerID, suffix string, err error) {
	parts := strings.Split(id, ":")
	if len(parts) != 4 || parts[0] != "obj" {
		return "", "", "", fmt.Errorf("%w: %q", ErrMalformedID, id)
	}
	owner, err := decodeIDPart(parts[2])
	if err != nil {
		return "", "", "", fmt.Errorf("%w: invalid owner component: %q", ErrMalformedID, id)
	}
	if err := validateKind(ObjectKind(parts[1])); err != nil {
		return "", "", "", fmt.Errorf("%w: %v", ErrMalformedID, err)
	}
	if parts[3] == "" {
		return "", "", "", fmt.Errorf("%w: empty suffix", ErrMalformedID)
	}
	return ObjectKind(parts[1]), owner, parts[3], nil
}

func ContentHash(kind ObjectKind, body []byte, metadata json.RawMessage) string {
	normalized, err := NormalizeMetadata(metadata)
	if err != nil {
		normalized = json.RawMessage(`{}`)
	}
	payload, _ := json.Marshal(struct {
		ObjectKind ObjectKind      `json:"object_kind"`
		Body       []byte          `json:"body,omitempty"`
		Metadata   json.RawMessage `json:"metadata"`
	}{
		ObjectKind: kind,
		Body:       body,
		Metadata:   normalized,
	})
	return SHA256(payload)
}

func SHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func NormalizeMetadata(v any) (json.RawMessage, error) {
	if v == nil {
		return json.RawMessage(`{}`), nil
	}
	var raw []byte
	switch t := v.(type) {
	case json.RawMessage:
		raw = t
	case []byte:
		raw = t
	case string:
		raw = []byte(t)
	default:
		var err error
		raw, err = json.Marshal(t)
		if err != nil {
			return nil, fmt.Errorf("marshal metadata: %w", err)
		}
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, fmt.Errorf("metadata must be valid JSON: %w", err)
	}
	out, err := json.Marshal(decoded)
	if err != nil {
		return nil, fmt.Errorf("normalize metadata: %w", err)
	}
	if len(out) == 0 || string(out) == "null" {
		return json.RawMessage(`{}`), nil
	}
	return json.RawMessage(out), nil
}

func StableSuffixFromContent(contentHash string) string {
	return "sha256-" + strings.TrimPrefix(contentHash, "sha256:")
}

func StableSuffixFromKey(key string) string {
	return "key-" + strings.TrimPrefix(SHA256([]byte(key)), "sha256:")
}

func BuildEdgeID(fromID, toID string, kind EdgeKind, metadata json.RawMessage) (string, error) {
	if err := validateEdgeKind(kind); err != nil {
		return "", err
	}
	normalized, err := NormalizeMetadata(metadata)
	if err != nil {
		return "", err
	}
	payload, _ := json.Marshal(struct {
		FromID   string          `json:"from_id"`
		ToID     string          `json:"to_id"`
		Kind     EdgeKind        `json:"kind"`
		Metadata json.RawMessage `json:"metadata"`
	}{FromID: fromID, ToID: toID, Kind: kind, Metadata: normalized})
	return "edge:" + string(kind) + ":" + strings.TrimPrefix(SHA256(payload), "sha256:"), nil
}

func encodeIDPart(value string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(value))
}

func decodeIDPart(value string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(value)
	return string(raw), err
}

func validateKind(kind ObjectKind) error {
	if strings.TrimSpace(string(kind)) == "" || strings.ContainsAny(string(kind), ":/\\?#[]@!$&'()*+,;=") {
		return fmt.Errorf("invalid object kind %q", kind)
	}
	return nil
}

func validateEdgeKind(kind EdgeKind) error {
	if strings.TrimSpace(string(kind)) == "" || strings.ContainsAny(string(kind), ":/\\?#[]@!$&'()*+,;=") {
		return fmt.Errorf("invalid edge kind %q", kind)
	}
	return nil
}
