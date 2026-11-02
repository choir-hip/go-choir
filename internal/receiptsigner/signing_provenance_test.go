package receiptsigner

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A signer key that is generated is never silent
// (docs/problems/verifier-key-regenerated-locks-out-checkpoints-2026-11-02.md):
// the generation is recorded beside the key, and a key generated over a
// recorded prior key names that key, so the guest can report the change
// before an apply spends an attempt on it. Failure modes covered:
//   - a fresh disk (genesis, first boot) must still generate a key;
//   - a restart must load the same key and report no generation;
//   - a lost key file with its record present must name the prior key;
//   - a key from before the record existed must load and be backfilled;
//   - a corrupt record must never stop the signer (the autoputer requires
//     the verifier signer unit, so a refusal would brick the computer);
//   - the public-key route must carry the provenance.

func TestSigningKeyGenerationIsRecordedAndReportedOnce(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "verifier", "key.ed25519")
	first, provenance, err := LoadOrCreateSigningKeyWithProvenance(path, ModeVerifier)
	if err != nil {
		t.Fatal(err)
	}
	if !provenance.Generated || provenance.GeneratedAt.IsZero() || provenance.PriorKeyID != "" || provenance.KeyID != first.KeyID {
		t.Fatalf("fresh disk provenance = %+v", provenance)
	}
	again, provenance, err := LoadOrCreateSigningKeyWithProvenance(path, ModeVerifier)
	if err != nil {
		t.Fatal(err)
	}
	if again.KeyID != first.KeyID || provenance.Generated || provenance.PriorKeyID != "" {
		t.Fatalf("restart provenance = %+v (key %s, first %s)", provenance, again.KeyID, first.KeyID)
	}
	if provenance.GeneratedAt.IsZero() {
		t.Fatalf("restart lost the generation time: %+v", provenance)
	}
}

func TestSigningKeyLostFileNamesThePriorKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "key.ed25519")
	first, _, err := LoadOrCreateSigningKeyWithProvenance(path, ModeVerifier)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	second, provenance, err := LoadOrCreateSigningKeyWithProvenance(path, ModeVerifier)
	if err != nil {
		t.Fatalf("a lost key file must not stop the signer: %v", err)
	}
	if second.KeyID == first.KeyID || !provenance.Generated || provenance.PriorKeyID != first.KeyID {
		t.Fatalf("regeneration over a recorded key = %+v (first %s, second %s)", provenance, first.KeyID, second.KeyID)
	}
	// The record names the prior key until a later generation replaces it.
	_, provenance, err = LoadOrCreateSigningKeyWithProvenance(path, ModeVerifier)
	if err != nil || provenance.Generated || provenance.PriorKeyID != first.KeyID {
		t.Fatalf("restart after regeneration = %+v, %v", provenance, err)
	}
}

func TestSigningKeyFromBeforeTheRecordIsBackfilled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "key.ed25519")
	key, err := LoadOrCreateSigningKey(path, ModeVerifier)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(provenancePath(path)); err != nil {
		t.Fatal(err)
	}
	loaded, provenance, err := LoadOrCreateSigningKeyWithProvenance(path, ModeVerifier)
	if err != nil || loaded.KeyID != key.KeyID || provenance.Generated || provenance.KeyID != key.KeyID || !provenance.GeneratedAt.IsZero() {
		t.Fatalf("pre-record key = %+v, %v", provenance, err)
	}
	if _, statErr := os.Stat(provenancePath(path)); statErr != nil {
		t.Fatalf("record not backfilled: %v", statErr)
	}
}

func TestSigningKeyCorruptRecordNeverStopsTheSigner(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "key.ed25519")
	key, _, err := LoadOrCreateSigningKeyWithProvenance(path, ModeVerifier)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(provenancePath(path), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, provenance, err := LoadOrCreateSigningKeyWithProvenance(path, ModeVerifier)
	if err != nil || loaded.KeyID != key.KeyID || provenance.Generated {
		t.Fatalf("corrupt record = %+v, %v", provenance, err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(provenancePath(path), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, provenance, err = LoadOrCreateSigningKeyWithProvenance(path, ModeVerifier); err != nil || !provenance.Generated || provenance.PriorKeyID != "unknown" {
		t.Fatalf("lost key with corrupt record = %+v, %v", provenance, err)
	}
}

func TestPublicKeyRouteCarriesProvenance(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "key.ed25519")
	if _, _, err := LoadOrCreateSigningKeyWithProvenance(path, ModeVerifier); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	key, provenance, err := LoadOrCreateSigningKeyWithProvenance(path, ModeVerifier)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := NewHandler(ModeVerifier, "computer-1", filepath.Join(dir, "receipts"), key)
	if err != nil {
		t.Fatal(err)
	}
	handler.SetProvenance(provenance)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/public-key", nil))
	var body map[string]any
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &body) != nil {
		t.Fatalf("public key: %d %s", response.Code, response.Body.String())
	}
	if body["key_id"] != key.KeyID || body["prior_key_id"] != provenance.PriorKeyID || body["generated_this_boot"] != true {
		t.Fatalf("public key body = %v", body)
	}
	if at, _ := body["generated_at"].(string); at == "" {
		t.Fatalf("public key body has no generated_at: %v", body)
	} else if _, parseErr := time.Parse(time.RFC3339Nano, at); parseErr != nil {
		t.Fatalf("generated_at %q: %v", at, parseErr)
	}
}
