package store

import (
	"context"
	"encoding/json"
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
	return BuildTextureSourceRefCanonicalIDByScope(ownerID, "", revisionID, occurrenceKey)
}

func BuildTextureSourceRefCanonicalIDByScope(ownerID, computerID, revisionID, occurrenceKey string) (string, error) {
	revisionID = strings.TrimSpace(revisionID)
	occurrenceKey = strings.TrimSpace(occurrenceKey)
	if revisionID == "" {
		return "", fmt.Errorf("revision_id is required")
	}
	if occurrenceKey == "" {
		return "", fmt.Errorf("occurrence key is required")
	}
	identity := revisionID + "\x00" + occurrenceKey
	if computerID = strings.TrimSpace(computerID); computerID != "" {
		identity = computerID + "\x00" + identity
	}
	return objectgraph.BuildCanonicalID(TextureSourceRefObjectKind, ownerID, objectgraph.StableSuffixFromKey(identity))
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
	return s.ListTextureSourceEntitiesByScopeOG(ctx, ownerID, "", 100000)
}

func (s *Store) ListTextureSourceEntitiesByScope(ctx context.Context, ownerID, computerID string) ([]TextureSourceEntityGraphRecord, error) {
	return s.ListTextureSourceEntitiesByScopeOG(ctx, ownerID, computerID, 100000)
}

func (s *Store) ListTextureSourceRefsForRevision(ctx context.Context, ownerID, docID, revisionID string) ([]TextureSourceRefGraphRecord, error) {
	return s.ListTextureSourceRefsByRevisionAndScopeOG(ctx, ownerID, "", docID, revisionID, 100000)
}

func (s *Store) ListTextureSourceRefsForRevisionByScope(ctx context.Context, ownerID, computerID, docID, revisionID string) ([]TextureSourceRefGraphRecord, error) {
	return s.ListTextureSourceRefsByRevisionAndScopeOG(ctx, ownerID, computerID, docID, revisionID, 100000)
}

func (s *Store) ListTextureSourceEntitiesForRevision(ctx context.Context, ownerID, docID, revisionID string) ([]TextureSourceEntityGraphRecord, error) {
	return s.listTextureSourceEntitiesForRevision(ctx, ownerID, "", docID, revisionID)
}

func (s *Store) ListTextureSourceEntitiesForRevisionByScope(ctx context.Context, ownerID, computerID, docID, revisionID string) ([]TextureSourceEntityGraphRecord, error) {
	return s.listTextureSourceEntitiesForRevision(ctx, ownerID, computerID, docID, revisionID)
}

func (s *Store) listTextureSourceEntitiesForRevision(ctx context.Context, ownerID, computerID, docID, revisionID string) ([]TextureSourceEntityGraphRecord, error) {
	refs, err := s.ListTextureSourceRefsForRevisionByScope(ctx, ownerID, computerID, docID, revisionID)
	if err != nil {
		return nil, err
	}
	pinned := make(map[string]bool, len(refs))
	for _, ref := range refs {
		pinned[entityVersionKey(ref.SourceEntityCanonicalID, ref.SourceEntityVersionID)] = true
	}

	entities, err := s.ListTextureSourceEntitiesByScope(ctx, ownerID, computerID)
	if err != nil {
		return nil, err
	}
	out := make([]TextureSourceEntityGraphRecord, 0, len(entities))
	seen := make(map[string]bool, len(entities))
	for _, rec := range entities {
		key := entityVersionKey(rec.CanonicalID, rec.VersionID)
		if seen[key] {
			continue
		}
		if pinned[key] || textureSourceEntityRecordMatchesRevision(rec, docID, revisionID) {
			out = append(out, rec)
			seen[key] = true
		}
	}
	return out, nil
}

func (s *Store) ListTextureSourceGraphForRevisions(ctx context.Context, ownerID, docID string, revisionIDs []string) (map[string]TextureSourceGraphWriteSet, error) {
	return s.ListTextureSourceGraphForRevisionsByScope(ctx, ownerID, "", docID, revisionIDs)
}

func (s *Store) ListTextureSourceGraphForRevisionsByScope(ctx context.Context, ownerID, computerID, docID string, revisionIDs []string) (map[string]TextureSourceGraphWriteSet, error) {
	ids, wanted := normalizeTextureSourceGraphRevisionIDs(revisionIDs)
	out := make(map[string]TextureSourceGraphWriteSet, len(ids))
	for _, revisionID := range ids {
		out[revisionID] = TextureSourceGraphWriteSet{}
	}
	if len(ids) == 0 {
		return out, nil
	}

	refsByRevision, err := s.listTextureSourceRefsForRevisions(ctx, ownerID, computerID, docID, ids)
	if err != nil {
		return nil, err
	}
	pinnedRevisionsByEntity := map[string][]string{}
	for revisionID, refs := range refsByRevision {
		set := out[revisionID]
		set.SourceRefs = refs
		out[revisionID] = set
		for _, ref := range refs {
			key := entityVersionKey(ref.SourceEntityCanonicalID, ref.SourceEntityVersionID)
			pinnedRevisionsByEntity[key] = append(pinnedRevisionsByEntity[key], revisionID)
		}
	}

	entities, err := s.ListTextureSourceEntitiesByScope(ctx, ownerID, computerID)
	if err != nil {
		return nil, err
	}
	seenEntityByRevision := make(map[string]map[string]bool, len(ids))
	appendEntity := func(revisionID string, rec TextureSourceEntityGraphRecord) {
		if !wanted[revisionID] {
			return
		}
		key := entityVersionKey(rec.CanonicalID, rec.VersionID)
		if seenEntityByRevision[revisionID] == nil {
			seenEntityByRevision[revisionID] = map[string]bool{}
		}
		if seenEntityByRevision[revisionID][key] {
			return
		}
		set := out[revisionID]
		set.SourceEntities = append(set.SourceEntities, rec)
		out[revisionID] = set
		seenEntityByRevision[revisionID][key] = true
	}

	for _, rec := range entities {
		key := entityVersionKey(rec.CanonicalID, rec.VersionID)
		for _, revisionID := range pinnedRevisionsByEntity[key] {
			appendEntity(revisionID, rec)
		}
		if revisionID, ok := textureSourceEntityRecordRevisionID(rec, docID); ok {
			appendEntity(revisionID, rec)
		}
	}
	return out, nil
}

func normalizeTextureSourceEntityGraphRecord(rec TextureSourceEntityGraphRecord, ownerID, computerID string, createdAt time.Time) (TextureSourceEntityGraphRecord, error) {
	rec.OwnerID = strings.TrimSpace(rec.OwnerID)
	if rec.OwnerID == "" {
		rec.OwnerID = ownerID
	}
	if rec.OwnerID != ownerID {
		return TextureSourceEntityGraphRecord{}, fmt.Errorf("owner_id %q does not match revision owner %q", rec.OwnerID, ownerID)
	}
	rec.ComputerID = strings.TrimSpace(rec.ComputerID)
	computerID = strings.TrimSpace(computerID)
	if rec.ComputerID == "" {
		rec.ComputerID = computerID
	}
	if rec.ComputerID != computerID {
		return TextureSourceEntityGraphRecord{}, fmt.Errorf("computer_id %q does not match revision computer %q", rec.ComputerID, computerID)
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
	rec.ComputerID = strings.TrimSpace(rec.ComputerID)
	revisionComputerID := strings.TrimSpace(rev.ComputerID)
	if rec.ComputerID == "" {
		rec.ComputerID = revisionComputerID
	}
	if rec.ComputerID != revisionComputerID {
		return TextureSourceRefGraphRecord{}, fmt.Errorf("computer_id %q does not match revision computer %q", rec.ComputerID, revisionComputerID)
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

func defaultTextureGraphTime(candidate, fallback time.Time) time.Time {
	if !candidate.IsZero() {
		return candidate.UTC()
	}
	if !fallback.IsZero() {
		return fallback.UTC()
	}
	return time.Now().UTC()
}

func (s *Store) listTextureSourceRefsForRevisions(ctx context.Context, ownerID, computerID, docID string, revisionIDs []string) (map[string][]TextureSourceRefGraphRecord, error) {
	ids, wanted := normalizeTextureSourceGraphRevisionIDs(revisionIDs)
	out := make(map[string][]TextureSourceRefGraphRecord, len(ids))
	for _, revisionID := range ids {
		out[revisionID] = nil
	}
	if len(ids) == 0 {
		return out, nil
	}
	// One metadata scan by doc_id covers every revision of the document —
	// replaces the prior per-revision scan loop (N full-kind scans per call).
	objs, err := s.ogListAllByMetadata(ctx, TextureSourceRefObjectKind, "doc_id", docID)
	if err != nil {
		return nil, fmt.Errorf("query texture source refs for revisions: %w", err)
	}
	for _, obj := range objs {
		var rec TextureSourceRefGraphRecord
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		revID := strings.TrimSpace(rec.TextureRevisionID)
		if rec.OwnerID != ownerID || strings.TrimSpace(rec.ComputerID) != computerID || rec.DocID != docID || !wanted[revID] {
			continue
		}
		out[revID] = append(out[revID], rec)
	}
	return out, nil
}

func normalizeTextureSourceGraphRevisionIDs(revisionIDs []string) ([]string, map[string]bool) {
	out := make([]string, 0, len(revisionIDs))
	seen := make(map[string]bool, len(revisionIDs))
	for _, revisionID := range revisionIDs {
		revisionID = strings.TrimSpace(revisionID)
		if revisionID == "" || seen[revisionID] {
			continue
		}
		seen[revisionID] = true
		out = append(out, revisionID)
	}
	return out, seen
}

func textureSourceEntityRecordMatchesRevision(rec TextureSourceEntityGraphRecord, docID, revisionID string) bool {
	gotRevisionID, ok := textureSourceEntityRecordRevisionID(rec, docID)
	return ok && gotRevisionID == strings.TrimSpace(revisionID)
}

func textureSourceEntityRecordRevisionID(rec TextureSourceEntityGraphRecord, docID string) (string, bool) {
	docID = strings.TrimSpace(docID)
	var meta map[string]any
	if err := json.Unmarshal(rec.Metadata, &meta); err != nil {
		return "", false
	}
	revisionID := strings.TrimSpace(fmt.Sprint(meta["texture_revision_id"]))
	if revisionID == "" {
		return "", false
	}
	if docID != "" && strings.TrimSpace(fmt.Sprint(meta["texture_doc_id"])) != docID {
		return "", false
	}
	return revisionID, true
}
