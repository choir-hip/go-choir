package snapshot

// S4: a 409 whose body is not the host's JSON reads cleanly, with no
// dangling colon.

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPChunksRegisterSnapshotRootNonJSONRefusalHasNoDanglingColon(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte("<html>conflict</html>"))
	}))
	defer srv.Close()
	h := HTTPChunks{BaseURL: srv.URL, ComputerID: testComputer}
	err := h.RegisterSnapshotRoot(context.Background(), strings.Repeat("cd", 32), 7, []string{strings.Repeat("ab", 32)})
	if !errors.Is(err, ErrSnapshotChunkMissing) {
		t.Fatalf("err = %v", err)
	}
	if strings.HasSuffix(strings.TrimSpace(err.Error()), ":") {
		t.Fatalf("dangling colon: %q", err.Error())
	}
}
