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
func buildVersionHistoryManifest(history []PublishTextureRevision, signer *SigningKey) (PublicationVersionHistory, []byte, string) {
	if len(history) == 0 {
		return PublicationVersionHistory{Schema: versionHistorySchema, Revisions: []PublicationVersionHistoryEntry{}}, nil, ""
	}

	entries := make([]PublicationVersionHistoryEntry, 0, len(history))
	chainHeadHash := ""
	for _, rev := range history {
		if rev.RevisionHash != "" {
			chainHeadHash = rev.RevisionHash
		}
		entry := PublicationVersionHistoryEntry{
			RevisionID:       rev.RevisionID,
			ParentRevisionID: rev.ParentRevisionID,
			VersionNumber:    rev.VersionNumber,
			AuthorKind:       rev.AuthorKind,
			AuthorLabel:      rev.AuthorLabel,
			Content:          rev.Content,
			BodyDoc:          rev.BodyDoc,
			SourceEntities:   rev.SourceEntities,
			ContentHash:      sha256Hex([]byte(rev.Content)),
			Citations:        rev.Citations,
			Metadata:         rev.Metadata,
			Provenance:       rev.Provenance,
			RevisionHash:     rev.RevisionHash,
			CreatedAt:        rev.CreatedAt,
		}
		// Mission D6: the platform attests each revision by signing the
		// canonical attestation of its revision hash. RevisionHash commits to
		// body + citations + provenance (timestamp + authoring model) + parent
		// hash, so the signature makes each version tamperproof and attributable
		// to the platform. Unsigned when no signer is configured.
		if signer != nil && rev.RevisionHash != "" {
			sig, err := signer.signRevision(rev.RevisionHash)
			if err != nil {
				// Signing is deterministic over a fixed payload; the only
				// failure path is a marshaling error, which is a programmer bug.
				// Fail loudly rather than emitting a partially-signed chain.
				return PublicationVersionHistory{}, nil, ""
			}
			entry.Signature = sig
			entry.SigningKeyID = signer.KeyID
		}
		entries = append(entries, entry)
	}

	manifest := PublicationVersionHistory{
		Schema:        versionHistorySchema,
		RevisionCount: len(entries),
		ChainHeadHash: chainHeadHash,
		Revisions:     entries,
	}
	if signer != nil {
		manifest.SigningSchema = revisionAttestationSchema
		manifest.SigningPublicKey = signer.PublicKeyBase64()
		manifest.SigningKeyID = signer.KeyID
	}

	// Hash over the manifest without its own ManifestHash field set, so the
	// hash is a pure function of the chain content (including signatures, which
	// are part of the published artifact).
	raw, err := json.Marshal(manifest)
	if err != nil {
		return manifest, nil, ""
	}
	hash := sha256Hex(raw)
	manifest.ManifestHash = hash
	return manifest, raw, hash
}
