package store

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const (
	TextureSourceEntityObjectKind objectgraph.ObjectKind = "choir.source_entity"
	TextureSourceRefObjectKind    objectgraph.ObjectKind = "choir.source_ref"

	TextureSourceRefDisplayNumbered = "numbered_ref"
	TextureSourceRefDisplayExpanded = "expanded_ref"
)

type rowScanner interface {
	Scan(dest ...any) error
}

type TextureSourceGraphWriteSet struct {
	SourceEntities []TextureSourceEntityGraphRecord
	SourceRefs     []TextureSourceRefGraphRecord
}

type TextureSourceEntityGraphRecord struct {
	CanonicalID          string          `json:"canonical_id"`
	OwnerID              string          `json:"owner_id"`
	ComputerID           string          `json:"computer_id,omitempty"`
	VersionID            string          `json:"version_id"`
	ContentHash          string          `json:"content_hash"`
	Body                 []byte          `json:"body,omitempty"`
	Metadata             json.RawMessage `json:"metadata"`
	LegacySourceEntityID string          `json:"legacy_source_entity_id,omitempty"`
	CreatedAt            time.Time       `json:"created_at"`
}

type TextureSourceRefGraphRecord struct {
	CanonicalID             string          `json:"canonical_id"`
	OwnerID                 string          `json:"owner_id"`
	ComputerID              string          `json:"computer_id,omitempty"`
	VersionID               string          `json:"version_id"`
	ContentHash             string          `json:"content_hash"`
	DocID                   string          `json:"doc_id"`
	TextureRevisionID       string          `json:"texture_revision_id"`
	BodyNodeID              string          `json:"body_node_id,omitempty"`
	BodyNodePathHash        string          `json:"body_node_path_hash,omitempty"`
	LegacySourceEntityID    string          `json:"legacy_source_entity_id,omitempty"`
	SourceEntityCanonicalID string          `json:"source_entity_canonical_id"`
	SourceEntityVersionID   string          `json:"source_entity_version_id"`
	DisplayMode             string          `json:"display_mode"`
	CitationState           string          `json:"citation_state"`
	Metadata                json.RawMessage `json:"metadata"`
	CreatedAt               time.Time       `json:"created_at"`
}

func BuildTextureSourceEntityCanonicalID(ownerID, ownerScope, sourceKind, targetIdentity string) (string, error) {
	if strings.TrimSpace(ownerScope) == "" {
		ownerScope = ownerID
	}
	key, err := textureSourceEntityIdentityKey(ownerScope, sourceKind, targetIdentity)
	if err != nil {
		return "", err
	}
	return objectgraph.BuildCanonicalID(TextureSourceEntityObjectKind, ownerID, objectgraph.StableSuffixFromKey(key))
}

func BuildTextureSourceRefCanonicalID(ownerID, revisionID, occurrenceKey string) (string, error) {
	revisionID = strings.TrimSpace(revisionID)
	occurrenceKey = strings.TrimSpace(occurrenceKey)
	if revisionID == "" {
		return "", fmt.Errorf("revision_id is required")
	}
	if occurrenceKey == "" {
		return "", fmt.Errorf("occurrence key is required")
	}
	return objectgraph.BuildCanonicalID(TextureSourceRefObjectKind, ownerID, objectgraph.StableSuffixFromKey(revisionID+"\x00"+occurrenceKey))
}

func TextureSourceGraphVersionID(kind objectgraph.ObjectKind, body []byte, metadata json.RawMessage) (string, string, json.RawMessage, error) {
	if kind != TextureSourceEntityObjectKind && kind != TextureSourceRefObjectKind {
		return "", "", nil, fmt.Errorf("unsupported source graph kind %s", kind)
	}
	normalized, err := objectgraph.NormalizeMetadata(metadata)
	if err != nil {
		return "", "", nil, err
	}
	contentHash := objectgraph.ContentHash(kind, body, normalized)
	return "ver-" + strings.TrimPrefix(contentHash, "sha256:"), contentHash, normalized, nil
}

func (s *Store) ListTextureSourceEntities(ctx context.Context, ownerID string) ([]TextureSourceEntityGraphRecord, error) {
	rows, err := s.textureHandle().QueryContext(ctx,
		`SELECT canonical_id, version_id, owner_id, computer_id, content_hash, body, metadata_json, legacy_source_entity_id, created_at
		   FROM texture_source_entities
		  WHERE owner_id = ?
		  ORDER BY created_at, canonical_id, version_id`,
		ownerID,
	)
	if err != nil {
		return nil, fmt.Errorf("query texture source entities: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []TextureSourceEntityGraphRecord
	for rows.Next() {
		rec, err := scanTextureSourceEntity(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate texture source entities: %w", err)
	}
	return out, nil
}

func (s *Store) ListTextureSourceRefsForRevision(ctx context.Context, ownerID, docID, revisionID string) ([]TextureSourceRefGraphRecord, error) {
	rows, err := s.textureHandle().QueryContext(ctx,
		`SELECT canonical_id, version_id, owner_id, computer_id, content_hash, doc_id, texture_revision_id, body_node_id, body_node_path_hash, legacy_source_entity_id, source_entity_canonical_id, source_entity_version_id, display_mode, citation_state, metadata_json, created_at
		   FROM texture_source_refs
		  WHERE owner_id = ? AND doc_id = ? AND texture_revision_id = ?
		  ORDER BY created_at, canonical_id, version_id`,
		ownerID,
		docID,
		revisionID,
	)
	if err != nil {
		return nil, fmt.Errorf("query texture source refs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []TextureSourceRefGraphRecord
	for rows.Next() {
		rec, err := scanTextureSourceRef(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate texture source refs: %w", err)
	}
	return out, nil
}

func (s *Store) writeTextureSourceGraph(ctx context.Context, tx *sql.Tx, rev types.Revision, graph TextureSourceGraphWriteSet) error {
	if len(graph.SourceEntities) == 0 && len(graph.SourceRefs) == 0 {
		return nil
	}
	knownEntities := make(map[string]bool, len(graph.SourceEntities))
	for _, rec := range graph.SourceEntities {
		normalized, err := normalizeTextureSourceEntityGraphRecord(rec, rev.OwnerID, rev.CreatedAt)
		if err != nil {
			return fmt.Errorf("texture source entity graph record: %w", err)
		}
		if err := s.putTextureSourceEntityGraphRecord(ctx, tx, normalized); err != nil {
			return err
		}
		knownEntities[entityVersionKey(normalized.CanonicalID, normalized.VersionID)] = true
	}
	for _, rec := range graph.SourceRefs {
		normalized, err := normalizeTextureSourceRefGraphRecord(rec, rev, rev.CreatedAt)
		if err != nil {
			return fmt.Errorf("texture source ref graph record: %w", err)
		}
		key := entityVersionKey(normalized.SourceEntityCanonicalID, normalized.SourceEntityVersionID)
		if !knownEntities[key] {
			exists, err := textureSourceEntityVersionExists(ctx, tx, normalized.SourceEntityCanonicalID, normalized.SourceEntityVersionID)
			if err != nil {
				return err
			}
			if !exists {
				return fmt.Errorf("texture source ref points at missing source entity version %s/%s", normalized.SourceEntityCanonicalID, normalized.SourceEntityVersionID)
			}
		}
		if err := s.putTextureSourceRefGraphRecord(ctx, tx, normalized); err != nil {
			return err
		}
	}
	return nil
}

func normalizeTextureSourceEntityGraphRecord(rec TextureSourceEntityGraphRecord, ownerID string, createdAt time.Time) (TextureSourceEntityGraphRecord, error) {
	rec.OwnerID = strings.TrimSpace(rec.OwnerID)
	if rec.OwnerID == "" {
		rec.OwnerID = ownerID
	}
	if rec.OwnerID != ownerID {
		return TextureSourceEntityGraphRecord{}, fmt.Errorf("owner_id %q does not match revision owner %q", rec.OwnerID, ownerID)
	}
	kind, parsedOwner, _, err := objectgraph.ParseCanonicalID(rec.CanonicalID)
	if err != nil {
		return TextureSourceEntityGraphRecord{}, err
	}
	if kind != TextureSourceEntityObjectKind {
		return TextureSourceEntityGraphRecord{}, fmt.Errorf("canonical_id kind %s is not %s", kind, TextureSourceEntityObjectKind)
	}
	if parsedOwner != rec.OwnerID {
		return TextureSourceEntityGraphRecord{}, fmt.Errorf("canonical_id owner %q does not match owner_id %q", parsedOwner, rec.OwnerID)
	}
	versionID, contentHash, metadata, err := TextureSourceGraphVersionID(TextureSourceEntityObjectKind, rec.Body, rec.Metadata)
	if err != nil {
		return TextureSourceEntityGraphRecord{}, err
	}
	if strings.TrimSpace(rec.VersionID) == "" {
		rec.VersionID = versionID
	}
	if rec.VersionID != versionID {
		return TextureSourceEntityGraphRecord{}, fmt.Errorf("version_id %q does not match content version %q", rec.VersionID, versionID)
	}
	if strings.TrimSpace(rec.ContentHash) == "" {
		rec.ContentHash = contentHash
	}
	if rec.ContentHash != contentHash {
		return TextureSourceEntityGraphRecord{}, fmt.Errorf("content_hash %q does not match computed hash %q", rec.ContentHash, contentHash)
	}
	rec.Metadata = metadata
	rec.CreatedAt = defaultTextureGraphTime(rec.CreatedAt, createdAt)
	return rec, nil
}

func normalizeTextureSourceRefGraphRecord(rec TextureSourceRefGraphRecord, rev types.Revision, createdAt time.Time) (TextureSourceRefGraphRecord, error) {
	rec.OwnerID = strings.TrimSpace(rec.OwnerID)
	if rec.OwnerID == "" {
		rec.OwnerID = rev.OwnerID
	}
	if rec.OwnerID != rev.OwnerID {
		return TextureSourceRefGraphRecord{}, fmt.Errorf("owner_id %q does not match revision owner %q", rec.OwnerID, rev.OwnerID)
	}
	if strings.TrimSpace(rec.DocID) == "" {
		rec.DocID = rev.DocID
	}
	if rec.DocID != rev.DocID {
		return TextureSourceRefGraphRecord{}, fmt.Errorf("doc_id %q does not match revision doc %q", rec.DocID, rev.DocID)
	}
	if strings.TrimSpace(rec.TextureRevisionID) == "" {
		rec.TextureRevisionID = rev.RevisionID
	}
	if rec.TextureRevisionID != rev.RevisionID {
		return TextureSourceRefGraphRecord{}, fmt.Errorf("texture_revision_id %q does not match revision_id %q", rec.TextureRevisionID, rev.RevisionID)
	}
	kind, parsedOwner, _, err := objectgraph.ParseCanonicalID(rec.CanonicalID)
	if err != nil {
		return TextureSourceRefGraphRecord{}, err
	}
	if kind != TextureSourceRefObjectKind {
		return TextureSourceRefGraphRecord{}, fmt.Errorf("canonical_id kind %s is not %s", kind, TextureSourceRefObjectKind)
	}
	if parsedOwner != rec.OwnerID {
		return TextureSourceRefGraphRecord{}, fmt.Errorf("canonical_id owner %q does not match owner_id %q", parsedOwner, rec.OwnerID)
	}
	if strings.TrimSpace(rec.SourceEntityCanonicalID) == "" || strings.TrimSpace(rec.SourceEntityVersionID) == "" {
		return TextureSourceRefGraphRecord{}, fmt.Errorf("source entity canonical_id and version_id are required")
	}
	if err := validateTextureSourceEntityCanonicalID(rec.SourceEntityCanonicalID, rec.OwnerID); err != nil {
		return TextureSourceRefGraphRecord{}, err
	}
	if rec.DisplayMode == "" {
		rec.DisplayMode = "numbered_ref"
	}
	if rec.DisplayMode != "numbered_ref" && rec.DisplayMode != "expanded_ref" {
		return TextureSourceRefGraphRecord{}, fmt.Errorf("display_mode %q is not supported", rec.DisplayMode)
	}
	if rec.CitationState == "" {
		rec.CitationState = "cited"
	}
	if rec.CitationState != "cited" {
		return TextureSourceRefGraphRecord{}, fmt.Errorf("citation_state %q is not supported for source_ref records", rec.CitationState)
	}
	versionID, contentHash, metadata, err := TextureSourceGraphVersionID(TextureSourceRefObjectKind, sourceRefVersionBody(rec), rec.Metadata)
	if err != nil {
		return TextureSourceRefGraphRecord{}, err
	}
	if strings.TrimSpace(rec.VersionID) == "" {
		rec.VersionID = versionID
	}
	if rec.VersionID != versionID {
		return TextureSourceRefGraphRecord{}, fmt.Errorf("version_id %q does not match content version %q", rec.VersionID, versionID)
	}
	if strings.TrimSpace(rec.ContentHash) == "" {
		rec.ContentHash = contentHash
	}
	if rec.ContentHash != contentHash {
		return TextureSourceRefGraphRecord{}, fmt.Errorf("content_hash %q does not match computed hash %q", rec.ContentHash, contentHash)
	}
	rec.Metadata = metadata
	rec.CreatedAt = defaultTextureGraphTime(rec.CreatedAt, createdAt)
	return rec, nil
}

func (s *Store) putTextureSourceEntityGraphRecord(ctx context.Context, tx *sql.Tx, rec TextureSourceEntityGraphRecord) error {
	existing, err := getTextureSourceEntityGraphRecord(ctx, tx, rec.CanonicalID, rec.VersionID)
	if err == nil {
		if existing.OwnerID != rec.OwnerID || existing.ContentHash != rec.ContentHash || !bytes.Equal(existing.Body, rec.Body) || string(existing.Metadata) != string(rec.Metadata) {
			return fmt.Errorf("texture source entity version conflict for %s/%s", rec.CanonicalID, rec.VersionID)
		}
		return nil
	}
	if !errors.Is(err, ErrNotFound) {
		return err
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO texture_source_entities (canonical_id, version_id, owner_id, computer_id, content_hash, body, metadata_json, legacy_source_entity_id, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.CanonicalID,
		rec.VersionID,
		rec.OwnerID,
		rec.ComputerID,
		rec.ContentHash,
		string(rec.Body),
		string(rec.Metadata),
		rec.LegacySourceEntityID,
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("insert texture source entity: %w", err)
	}
	return nil
}

func (s *Store) putTextureSourceRefGraphRecord(ctx context.Context, tx *sql.Tx, rec TextureSourceRefGraphRecord) error {
	existing, err := getTextureSourceRefGraphRecord(ctx, tx, rec.CanonicalID, rec.VersionID)
	if err == nil {
		if existing.OwnerID != rec.OwnerID ||
			existing.ContentHash != rec.ContentHash ||
			existing.DocID != rec.DocID ||
			existing.TextureRevisionID != rec.TextureRevisionID ||
			existing.SourceEntityCanonicalID != rec.SourceEntityCanonicalID ||
			existing.SourceEntityVersionID != rec.SourceEntityVersionID ||
			existing.DisplayMode != rec.DisplayMode ||
			existing.CitationState != rec.CitationState ||
			string(existing.Metadata) != string(rec.Metadata) {
			return fmt.Errorf("texture source ref version conflict for %s/%s", rec.CanonicalID, rec.VersionID)
		}
		return nil
	}
	if !errors.Is(err, ErrNotFound) {
		return err
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO texture_source_refs (canonical_id, version_id, owner_id, computer_id, content_hash, doc_id, texture_revision_id, body_node_id, body_node_path_hash, legacy_source_entity_id, source_entity_canonical_id, source_entity_version_id, display_mode, citation_state, metadata_json, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.CanonicalID,
		rec.VersionID,
		rec.OwnerID,
		rec.ComputerID,
		rec.ContentHash,
		rec.DocID,
		rec.TextureRevisionID,
		rec.BodyNodeID,
		rec.BodyNodePathHash,
		rec.LegacySourceEntityID,
		rec.SourceEntityCanonicalID,
		rec.SourceEntityVersionID,
		rec.DisplayMode,
		rec.CitationState,
		string(rec.Metadata),
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("insert texture source ref: %w", err)
	}
	return nil
}

func getTextureSourceEntityGraphRecord(ctx context.Context, tx *sql.Tx, canonicalID, versionID string) (TextureSourceEntityGraphRecord, error) {
	row := tx.QueryRowContext(ctx,
		`SELECT canonical_id, version_id, owner_id, computer_id, content_hash, body, metadata_json, legacy_source_entity_id, created_at
		   FROM texture_source_entities
		  WHERE canonical_id = ? AND version_id = ?`,
		canonicalID,
		versionID,
	)
	return scanTextureSourceEntity(row)
}

func getTextureSourceRefGraphRecord(ctx context.Context, tx *sql.Tx, canonicalID, versionID string) (TextureSourceRefGraphRecord, error) {
	row := tx.QueryRowContext(ctx,
		`SELECT canonical_id, version_id, owner_id, computer_id, content_hash, doc_id, texture_revision_id, body_node_id, body_node_path_hash, legacy_source_entity_id, source_entity_canonical_id, source_entity_version_id, display_mode, citation_state, metadata_json, created_at
		   FROM texture_source_refs
		  WHERE canonical_id = ? AND version_id = ?`,
		canonicalID,
		versionID,
	)
	return scanTextureSourceRef(row)
}

func textureSourceEntityVersionExists(ctx context.Context, tx *sql.Tx, canonicalID, versionID string) (bool, error) {
	var count int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM texture_source_entities WHERE canonical_id = ? AND version_id = ?`,
		canonicalID,
		versionID,
	).Scan(&count); err != nil {
		return false, fmt.Errorf("query texture source entity version: %w", err)
	}
	return count > 0, nil
}

func scanTextureSourceEntity(row rowScanner) (TextureSourceEntityGraphRecord, error) {
	var rec TextureSourceEntityGraphRecord
	var body, metadata, createdAt string
	if err := row.Scan(&rec.CanonicalID, &rec.VersionID, &rec.OwnerID, &rec.ComputerID, &rec.ContentHash, &body, &metadata, &rec.LegacySourceEntityID, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return TextureSourceEntityGraphRecord{}, ErrNotFound
		}
		return TextureSourceEntityGraphRecord{}, fmt.Errorf("scan texture source entity: %w", err)
	}
	rec.Body = []byte(body)
	rec.Metadata = json.RawMessage(metadata)
	rec.CreatedAt = parseTextureSourceGraphTime(createdAt)
	return rec, nil
}

func scanTextureSourceRef(row rowScanner) (TextureSourceRefGraphRecord, error) {
	var rec TextureSourceRefGraphRecord
	var metadata, createdAt string
	if err := row.Scan(&rec.CanonicalID, &rec.VersionID, &rec.OwnerID, &rec.ComputerID, &rec.ContentHash, &rec.DocID, &rec.TextureRevisionID, &rec.BodyNodeID, &rec.BodyNodePathHash, &rec.LegacySourceEntityID, &rec.SourceEntityCanonicalID, &rec.SourceEntityVersionID, &rec.DisplayMode, &rec.CitationState, &metadata, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return TextureSourceRefGraphRecord{}, ErrNotFound
		}
		return TextureSourceRefGraphRecord{}, fmt.Errorf("scan texture source ref: %w", err)
	}
	rec.Metadata = json.RawMessage(metadata)
	rec.CreatedAt = parseTextureSourceGraphTime(createdAt)
	return rec, nil
}

func textureSourceEntityIdentityKey(ownerScope, sourceKind, targetIdentity string) (string, error) {
	ownerScope = strings.TrimSpace(ownerScope)
	sourceKind = strings.ToLower(strings.TrimSpace(sourceKind))
	targetIdentity = strings.TrimSpace(targetIdentity)
	if ownerScope == "" {
		return "", fmt.Errorf("owner scope is required")
	}
	if sourceKind == "" {
		return "", fmt.Errorf("source kind is required")
	}
	if targetIdentity == "" {
		return "", fmt.Errorf("target identity is required")
	}
	return ownerScope + "\x00" + sourceKind + "\x00" + targetIdentity, nil
}

func validateTextureSourceEntityCanonicalID(canonicalID, ownerID string) error {
	kind, parsedOwner, _, err := objectgraph.ParseCanonicalID(canonicalID)
	if err != nil {
		return err
	}
	if kind != TextureSourceEntityObjectKind {
		return fmt.Errorf("source_entity_canonical_id kind %s is not %s", kind, TextureSourceEntityObjectKind)
	}
	if parsedOwner != ownerID {
		return fmt.Errorf("source_entity_canonical_id owner %q does not match owner_id %q", parsedOwner, ownerID)
	}
	return nil
}

func sourceRefVersionBody(rec TextureSourceRefGraphRecord) []byte {
	payload, _ := json.Marshal(struct {
		DocID                   string `json:"doc_id"`
		TextureRevisionID       string `json:"texture_revision_id"`
		BodyNodeID              string `json:"body_node_id,omitempty"`
		BodyNodePathHash        string `json:"body_node_path_hash,omitempty"`
		SourceEntityCanonicalID string `json:"source_entity_canonical_id"`
		SourceEntityVersionID   string `json:"source_entity_version_id"`
		DisplayMode             string `json:"display_mode"`
		CitationState           string `json:"citation_state"`
	}{
		DocID:                   rec.DocID,
		TextureRevisionID:       rec.TextureRevisionID,
		BodyNodeID:              rec.BodyNodeID,
		BodyNodePathHash:        rec.BodyNodePathHash,
		SourceEntityCanonicalID: rec.SourceEntityCanonicalID,
		SourceEntityVersionID:   rec.SourceEntityVersionID,
		DisplayMode:             rec.DisplayMode,
		CitationState:           rec.CitationState,
	})
	return payload
}

func entityVersionKey(canonicalID, versionID string) string {
	return canonicalID + "\x00" + versionID
}

func parseTextureSourceGraphTime(value string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return t
}

func defaultTextureGraphTime(candidate, fallback time.Time) time.Time {
	if !candidate.IsZero() {
		return candidate.UTC()
	}
	if !fallback.IsZero() {
		return fallback.UTC()
	}
	return time.Now().UTC()
}
