package platform

import (
	"encoding/json"
	"testing"
)

func TestBuildVersionHistoryManifestDeterministic(t *testing.T) {
	history := []PublishTextureRevision{
		{
			RevisionID:    "rev-1",
			VersionNumber: 1,
			AuthorKind:    "appagent",
			Content:       "first body",
			Provenance:    json.RawMessage(`{"schema_version":"v0"}`),
			RevisionHash:  "hash-1",
			CreatedAt:     "2026-01-01T00:00:00.000Z",
		},
		{
			RevisionID:       "rev-2",
			ParentRevisionID: "rev-1",
			VersionNumber:    2,
			AuthorKind:       "appagent",
			Content:          "second body",
			Provenance:       json.RawMessage(`{"schema_version":"v0"}`),
			RevisionHash:     "hash-2",
			CreatedAt:        "2026-01-02T00:00:00.000Z",
		},
	}

	manifest, raw, hash := buildVersionHistoryManifest(history)
	if hash == "" {
		t.Fatal("expected non-empty manifest hash")
	}
	if manifest.RevisionCount != 2 {
		t.Fatalf("revision count = %d, want 2", manifest.RevisionCount)
	}
	if manifest.ChainHeadHash != "hash-2" {
		t.Fatalf("chain head hash = %q, want hash-2", manifest.ChainHeadHash)
	}
	if manifest.Schema != versionHistorySchema {
		t.Fatalf("schema = %q, want %q", manifest.Schema, versionHistorySchema)
	}
	if got := manifest.Revisions[0].ContentHash; got != sha256Hex([]byte("first body")) {
		t.Fatalf("entry content hash = %q, want %q", got, sha256Hex([]byte("first body")))
	}

	manifest2, raw2, hash2 := buildVersionHistoryManifest(history)
	if hash2 != hash {
		t.Fatalf("hash not deterministic: %q vs %q", hash, hash2)
	}
	if string(raw2) != string(raw) {
		t.Fatalf("manifest bytes not deterministic")
	}
	if manifest2.ManifestHash != hash {
		t.Fatalf("manifest hash field = %q, want %q", manifest2.ManifestHash, hash)
	}
}

func TestBuildVersionHistoryManifestEmpty(t *testing.T) {
	manifest, raw, hash := buildVersionHistoryManifest(nil)
	if hash != "" {
		t.Fatalf("empty history should yield empty hash, got %q", hash)
	}
	if raw != nil {
		t.Fatalf("empty history should yield nil raw, got %q", string(raw))
	}
	if manifest.RevisionCount != 0 || len(manifest.Revisions) != 0 {
		t.Fatalf("empty history should yield empty manifest")
	}
}

func TestBuildVersionHistoryManifestRoundTrip(t *testing.T) {
	history := []PublishTextureRevision{{
		RevisionID:   "rev-1",
		Content:      "body",
		RevisionHash: "hash-1",
	}}
	_, raw, _ := buildVersionHistoryManifest(history)
	var decoded PublicationVersionHistory
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("round-trip unmarshal: %v", err)
	}
	if decoded.RevisionCount != 1 || decoded.Revisions[0].RevisionID != "rev-1" {
		t.Fatalf("round-trip mismatch: %+v", decoded)
	}
}
