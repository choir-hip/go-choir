package snapshot

// Ways a refused snapshot root could still leave the guest believing it has
// a recovery base, written before the code
// (docs/problems/snapshot-root-recorded-without-chunk-check-2026-11-12.md):
//
//  R1. the host answers 409 (the root names a chunk it does not hold) and the
//      guest's error says only "status 409", so the failure's cause is lost;
//  R2. the guest appends snapshot_recorded after a refused registration;
//  R3. the refusal is not a typed error the producer's failure path (and a
//      reader of its logged cause) can recognise.

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPChunksRegisterSnapshotRootReportsTheHostsRefusal(t *testing.T) {
	missing := strings.Repeat("ab", 32)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"snapshot root names chunk ` + missing + `, which the host does not hold; take the snapshot again"}`))
	}))
	defer srv.Close()
	h := HTTPChunks{BaseURL: srv.URL, ComputerID: testComputer}
	err := h.RegisterSnapshotRoot(context.Background(), strings.Repeat("cd", 32), 7, []string{missing})
	if !errors.Is(err, ErrSnapshotChunksMissing) {
		t.Fatalf("err = %v, want ErrSnapshotChunksMissing", err)
	}
	if !strings.Contains(err.Error(), missing) {
		t.Fatalf("the host's reason is lost: %v", err)
	}
}

type refusingChunks struct{ *memChunks }

func (refusingChunks) RegisterSnapshotRoot(context.Context, string, uint64, []string) error {
	return ErrSnapshotChunksMissing
}

func TestARefusedSnapshotRootAppendsNoSnapshotRecorded(t *testing.T) {
	f := newFixture(t, ChunkSize)
	f.cfg.Chunks = refusingChunks{f.chunks}
	_, err := TakeSnapshot(context.Background(), f.cfg)
	if !errors.Is(err, ErrSnapshotChunksMissing) {
		t.Fatalf("err = %v, want ErrSnapshotChunksMissing", err)
	}
	if len(f.tape.events) != 0 {
		t.Fatalf("a refused root appended %d events", len(f.tape.events))
	}
	assertWALReleased(t, f)
}
