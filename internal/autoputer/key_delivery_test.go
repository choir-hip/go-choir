package autoputer

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/keyescrow"
)

// TestDeliverPrivacyKeyInstallsEscrowedKey: the guest requests delivery with
// its issuance proof, opens the key sealed to its ephemeral recipient key,
// checks the digest, and installs the canonical key file. Platform refusals
// are typed: no escrow or a superseded issuance is deterministic
// (errPrivacyKeyUndeliverable); 5xx is transient.
func TestDeliverPrivacyKeyInstallsEscrowedKey(t *testing.T) {
	const computerID = "computer-delivery"
	dek := bytes.Repeat([]byte{0x33}, 32)
	status := http.StatusOK
	var seen struct {
		auth       string
		commitment string
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/computers/keys/realization-delivery" {
			http.NotFound(w, r)
			return
		}
		var body struct {
			ComputerID         string `json:"computer_id"`
			RequestCommitment  string `json:"request_commitment"`
			RecipientPublicKey string `json:"recipient_public_key"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		seen.auth, seen.commitment = r.Header.Get("Authorization"), body.RequestCommitment
		if status != http.StatusOK {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"error":"refused"}`))
			return
		}
		var recipient keyescrow.PublicKey
		raw, _ := base64.RawStdEncoding.DecodeString(body.RecipientPublicKey)
		copy(recipient[:], raw)
		sealed, err := keyescrow.SealDEK(recipient, computerID, dek)
		if err != nil {
			t.Error(err)
		}
		wrapped, _ := json.Marshal(sealed)
		writeJSON(t, w, map[string]string{"wrapped_key": string(wrapped), "key_digest": sealed.KeyDigest})
	}))
	defer server.Close()

	path := filepath.Join(t.TempDir(), "creds", "privacy-key")
	if err := deliverPrivacyKey(context.Background(), server.URL, computerID, "commitment-a", "token-a", path); err != nil {
		t.Fatal(err)
	}
	if seen.auth != "Bearer token-a" || seen.commitment != "commitment-a" {
		t.Fatalf("delivery request proof = %q %q", seen.auth, seen.commitment)
	}
	cipher, err := computerevent.LoadGuestPrivateArtifactCipher(path, computerID, false)
	if err != nil {
		t.Fatalf("delivered key does not load: %v", err)
	}
	exported, _ := cipher.ExportKeyForEscrow(context.Background(), computerID)
	if !bytes.Equal(exported, dek) {
		t.Fatal("installed key differs from the escrowed key")
	}

	for code, deterministic := range map[int]bool{http.StatusNotFound: true, http.StatusConflict: true, http.StatusForbidden: true, http.StatusBadGateway: false} {
		status = code
		err := deliverPrivacyKey(context.Background(), server.URL, computerID, "commitment-a", "token-a", filepath.Join(t.TempDir(), "privacy-key"))
		if err == nil {
			t.Fatalf("status %d: expected an error", code)
		}
		if errors.Is(err, errPrivacyKeyUndeliverable) != deterministic {
			t.Fatalf("status %d: deterministic=%t, err=%v", code, !deterministic, err)
		}
	}
}
