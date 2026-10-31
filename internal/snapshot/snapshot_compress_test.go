package snapshot

import (
	"context"
	"fmt"
	"testing"
)

// Failure mode: a snapshot of a compressible store stores chunks compressed
// but restore does not reassemble the exact pages (the size/geometry checks
// compare plaintext lengths), or the stored bytes do not shrink.
func TestCompressibleStoreSnapshotShrinksAndRestores(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, 0)
	for i := 0; i < 20000; i++ {
		note := fmt.Sprintf(`{"event":"turn.commit","seq":%d,"run":"run-%d","payload":{"text":"agent said something about item %d and moved on","ok":true}}`, i, i%97, i*7)
		if _, err := f.db.Write.Exec(`INSERT INTO blobs (id, body, note) VALUES (?, ?, ?)`, i+1000000, []byte(note), note); err != nil {
			t.Fatal(err)
		}
	}
	ref, err := TakeSnapshot(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	var stored int64
	for _, sealed := range f.chunks.chunks {
		stored += int64(len(sealed))
	}
	plain := ref.Record.PlaintextSize
	t.Logf("SQLite store: plaintext %d bytes, stored %d bytes, ratio %.2fx", plain, stored, float64(plain)/float64(stored))
	if stored >= plain/2 {
		t.Fatalf("stored %d of %d plaintext bytes, want under half", stored, plain)
	}
	restoreAndOpen(t, f)
}
