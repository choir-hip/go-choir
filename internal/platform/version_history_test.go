package platform

import (
	"crypto/ed25519"
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

	manifest, raw, hash := buildVersionHistoryManifest(history, nil)
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

	manifest2, raw2, hash2 := buildVersionHistoryManifest(history, nil)
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
	manifest, raw, hash := buildVersionHistoryManifest(nil, nil)
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
	_, raw, _ := buildVersionHistoryManifest(history, nil)
	var decoded PublicationVersionHistory
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("round-trip unmarshal: %v", err)
	}
	if decoded.RevisionCount != 1 || decoded.Revisions[0].RevisionID != "rev-1" {
		t.Fatalf("round-trip mismatch: %+v", decoded)
	}
}

func TestBuildVersionHistoryManifestSignsEveryRevision(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	signer := newSigningKey(priv)

	history := []PublishTextureRevision{
		{RevisionID: "r0", Content: "prompt", RevisionHash: "hash-0"},
		{RevisionID: "r1", Content: "v1 body", RevisionHash: "hash-1"},
		{RevisionID: "r2", Content: "v2 body", RevisionHash: "hash-2"},
	}
	manifest, raw, hash := buildVersionHistoryManifest(history, signer)
	if hash == "" {
		t.Fatal("expected non-empty signed manifest hash")
	}
	if manifest.SigningPublicKey == "" || manifest.SigningKeyID == "" || manifest.SigningSchema == "" {
		t.Fatalf("manifest missing signing envelope: %+v", manifest)
	}

	var decoded PublicationVersionHistory
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal signed manifest: %v", err)
	}

	// Every published entry must carry a signature that verifies against the
	// manifest's public key, over its own revision hash.
	for i, entry := range decoded.Revisions {
		if entry.Signature == "" || entry.SigningKeyID != signer.KeyID {
			t.Fatalf("entry %d missing signature: %+v", i, entry)
		}
		if !VerifyRevisionSignature(decoded.SigningPublicKey, entry.RevisionHash, entry.Signature) {
			t.Fatalf("entry %d signature failed to verify", i)
		}
		// Tamper-evidence: the signature must NOT verify against a different hash.
		if VerifyRevisionSignature(decoded.SigningPublicKey, entry.RevisionHash+"x", entry.Signature) {
			t.Fatalf("entry %d signature verified against a tampered hash", i)
		}
	}

	// A different signer must produce different signatures but the same chain
	// structure (the chain is signer-independent; only attestations change).
	_, priv2, _ := ed25519.GenerateKey(nil)
	manifestB, _, _ := buildVersionHistoryManifest(history, newSigningKey(priv2))
	if manifestB.Revisions[0].Signature == manifest.Revisions[0].Signature {
		t.Fatal("different signers must produce different signatures")
	}
}

func TestBuildVersionHistoryManifestUnsignedWhenNoSigner(t *testing.T) {
	history := []PublishTextureRevision{{
		RevisionID: "r0", Content: "body", RevisionHash: "hash-0",
	}}
	manifest, _, _ := buildVersionHistoryManifest(history, nil)
	if manifest.SigningPublicKey != "" || manifest.SigningSchema != "" {
		t.Fatalf("nil signer must not populate signing envelope: %+v", manifest)
	}
	if manifest.Revisions[0].Signature != "" {
		t.Fatal("nil signer must not produce per-revision signatures")
	}
}
