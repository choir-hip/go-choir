package platform

import "encoding/json"

// versionHistorySchema identifies the canonical published version-history
// manifest format. Bump only on a breaking shape change.
const versionHistorySchema = "choir.platform.version_history.v0"

// buildVersionHistoryManifest collates the full source revision chain into a
// canonical, self-contained manifest plus its content-addressed hash.
//
// The manifest is the published, signable spine of a Texture-as-versioned-
// artifact: it carries every revision's body, citations, system-attributed
// provenance, and the per-revision hash chain, so a reader or verifier can
// replay the chain without trusting the head projection. Serialization is a
// struct marshal (deterministic field order; raw JSON fields passed through
// verbatim from the source), so the same chain always yields the same bytes
// and hash. Revisions are kept in the caller-provided order, which is the
// oldest-first chain order produced by the runtime revision store.
//
// An empty chain yields a zero manifest, nil bytes, and an empty hash so the
// head-only publish path stays unchanged when no history is supplied.
func buildVersionHistoryManifest(history []PublishTextureRevision) (PublicationVersionHistory, []byte, string) {
	if len(history) == 0 {
		return PublicationVersionHistory{Schema: versionHistorySchema, Revisions: []PublicationVersionHistoryEntry{}}, nil, ""
	}

	entries := make([]PublicationVersionHistoryEntry, 0, len(history))
	chainHeadHash := ""
	for _, rev := range history {
		if rev.RevisionHash != "" {
			chainHeadHash = rev.RevisionHash
		}
		entries = append(entries, PublicationVersionHistoryEntry{
			RevisionID:       rev.RevisionID,
			ParentRevisionID: rev.ParentRevisionID,
			VersionNumber:    rev.VersionNumber,
			AuthorKind:       rev.AuthorKind,
			AuthorLabel:      rev.AuthorLabel,
			Content:          rev.Content,
			ContentHash:      sha256Hex([]byte(rev.Content)),
			Citations:        rev.Citations,
			Metadata:         rev.Metadata,
			Provenance:       rev.Provenance,
			RevisionHash:     rev.RevisionHash,
			CreatedAt:        rev.CreatedAt,
		})
	}

	manifest := PublicationVersionHistory{
		Schema:        versionHistorySchema,
		RevisionCount: len(entries),
		ChainHeadHash: chainHeadHash,
		Revisions:     entries,
	}

	// Hash over the manifest without its own ManifestHash field set, so the
	// hash is a pure function of the chain content.
	raw, err := json.Marshal(manifest)
	if err != nil {
		return manifest, nil, ""
	}
	hash := sha256Hex(raw)
	manifest.ManifestHash = hash
	return manifest, raw, hash
}
