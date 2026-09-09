package autoputer

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/projectionbase"
)

func TestIsStoreEmpty(t *testing.T) {
	dir := t.TempDir()
	if !isStoreEmpty(dir) {
		t.Fatalf("expected empty temp dir to be empty")
	}

	// Create non-empty marker like .dolt
	if err := os.Mkdir(filepath.Join(dir, ".dolt"), 0o755); err != nil {
		t.Fatal(err)
	}
	if isStoreEmpty(dir) {
		t.Fatalf("expected dir with .dolt to be non-empty")
	}
}

func bootTestServer(t *testing.T, headStatus int, headBody any, watermarkStatus int, watermarkBody any, calls *int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*calls++
		switch r.URL.Path {
		case "/internal/computers/events/head":
			w.WriteHeader(headStatus)
			if headBody != nil {
				_ = json.NewEncoder(w).Encode(headBody)
			}
		case "/internal/computers/files/watermark":
			w.WriteHeader(watermarkStatus)
			if watermarkBody != nil {
				_ = json.NewEncoder(w).Encode(watermarkBody)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestMaterializeBootstrapsNewComputerWithoutBase(t *testing.T) {
	var calls int
	server := bootTestServer(t, http.StatusNotFound, nil, http.StatusNotFound, nil, &calls)
	defer server.Close()
	capability := func(ctx context.Context) (string, error) { return "test-cap", nil }

	materialized, err := materializeProjectionBaseIfNeeded(context.Background(), t.TempDir(), "computer-new", server.URL, capability)
	if err != nil {
		t.Fatalf("bootstrap refused: %v", err)
	}
	if materialized {
		t.Fatalf("bootstrap must not claim a base was installed")
	}
}

func TestMaterializeRefusesMissingBaseForExistingChain(t *testing.T) {
	var calls int
	head := map[string]any{"computer_id": "computer-old", "sequence": 42, "canonical_event_head": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	server := bootTestServer(t, http.StatusOK, head, http.StatusNotFound, nil, &calls)
	defer server.Close()
	capability := func(ctx context.Context) (string, error) { return "test-cap", nil }

	storeDir := t.TempDir()
	_, err := materializeProjectionBaseIfNeeded(context.Background(), storeDir, "computer-old", server.URL, capability)
	if !errors.Is(err, projectionbase.ErrBaseRefused) {
		t.Fatalf("missing required base did not refuse: %v", err)
	}
	entries, readErr := os.ReadDir(storeDir)
	if readErr != nil || len(entries) != 0 {
		t.Fatalf("refused install mutated the store dir: %v %v", entries, readErr)
	}
}

func TestMaterializeShortCircuitsNonEmptyStore(t *testing.T) {
	var calls int
	server := bootTestServer(t, http.StatusOK, nil, http.StatusOK, nil, &calls)
	defer server.Close()
	capability := func(ctx context.Context) (string, error) { return "test-cap", nil }

	storeDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(storeDir, ".dolt"), 0o755); err != nil {
		t.Fatal(err)
	}
	materialized, err := materializeProjectionBaseIfNeeded(context.Background(), storeDir, "computer-live", server.URL, capability)
	if err != nil {
		t.Fatalf("non-empty store refused: %v", err)
	}
	if materialized || calls != 0 {
		t.Fatalf("non-empty store caused platform reads: materialized=%v calls=%d", materialized, calls)
	}
}
