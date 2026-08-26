package autoputer

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/keyescrow"
)

func TestEnsureCustodianEscrowUploadsWrapOnce(t *testing.T) {
	priv, pub, err := keyescrow.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	pubB64 := mustEncodePub(t, pub)
	var mu sync.Mutex
	uploaded := ""
	dek := make([]byte, 32)
	for i := range dek {
		dek[i] = byte(7 * i)
	}
	const computerID = "computer-escrow-test"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/internal/computers/keys/escrow/status":
			status := map[string]any{"escrows": []any{}}
			if uploaded != "" {
				status["escrows"] = []any{map[string]string{"protector": "custodian", "key_digest": "x"}}
			}
			writeJSON(t, w, status)
		case r.Method == http.MethodGet && r.URL.Path == "/internal/computers/keys/escrow-public-key":
			writeJSON(t, w, map[string]string{"public_key": pubB64})
		case r.Method == http.MethodPut && r.URL.Path == "/internal/computers/keys/escrow":
			var req keyEscrowPutRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("decode put: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			uploaded = req.WrappedKey
			writeJSON(t, w, map[string]string{"status": "escrowed"})
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := newKeyEscrowClient(server.URL)
	present, err := client.EnsureCustodianEscrow(context.Background(), computerID, dek)
	if err != nil {
		t.Fatal(err)
	}
	if !present {
		t.Fatal("expected escrow present after upload")
	}
	// Second call must be a no-op (status already escrowed).
	present, err = client.EnsureCustodianEscrow(context.Background(), computerID, dek)
	if err != nil {
		t.Fatal(err)
	}
	if !present {
		t.Fatal("expected escrow present on second call")
	}
	// The uploaded wrap must open to the same DEK under the host private key.
	var record keyescrow.WrappedKey
	if err := json.Unmarshal([]byte(uploaded), &record); err != nil {
		t.Fatal(err)
	}
	opened, err := keyescrow.OpenDEK(priv, &record, computerID)
	if err != nil {
		t.Fatal(err)
	}
	for i := range dek {
		if opened[i] != dek[i] {
			t.Fatal("recovered DEK mismatch")
		}
	}
}

func mustEncodePub(t *testing.T, pub keyescrow.PublicKey) string {
	t.Helper()
	raw := make([]byte, len(pub))
	copy(raw, pub[:])
	data, err := json.Marshal(map[string]string{"public_key": encodeB64(raw)})
	if err != nil {
		t.Fatal(err)
	}
	// Return just the b64 payload the handler would send.
	var parsed struct {
		PublicKey string `json:"public_key"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	return parsed.PublicKey
}
func encodeB64(raw []byte) string {
	return base64.RawStdEncoding.EncodeToString(raw)
}

func writeJSON(t *testing.T, w http.ResponseWriter, payload any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		t.Errorf("encode response: %v", err)
	}
}
