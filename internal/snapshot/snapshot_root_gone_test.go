package snapshot

// A destroyed computer (410) must not be told to retake its snapshot.

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPChunksRegisterSnapshotRootGoneIsNotAMissingChunk(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusGone)
		_, _ = w.Write([]byte(`{"error":"snapshot root refused: computer tape deleted"}`))
	}))
	defer srv.Close()
	h := HTTPChunks{BaseURL: srv.URL, ComputerID: testComputer}
	err := h.RegisterSnapshotRoot(context.Background(), strings.Repeat("cd", 32), 7, []string{strings.Repeat("ab", 32)})
	if err == nil || errors.Is(err, ErrSnapshotChunkMissing) {
		t.Fatalf("err = %v, a destroyed computer must not read as a missing chunk", err)
	}
	if !errors.Is(err, ErrSnapshotComputerDeleted) {
		t.Fatalf("err = %v, want ErrSnapshotComputerDeleted", err)
	}
}
