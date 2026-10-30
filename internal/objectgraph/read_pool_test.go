package objectgraph

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// SQLite's one writer and its read pool replace the engine mutex the Dolt
// engine needed. A write store on the write connection and a read store on
// the pool run side by side: readers never fail with a lock error, never
// see a half-written batch, and see every committed write.
func TestReadPoolRunsBesideTheWriter(t *testing.T) {
	ctx := context.Background()
	writeDB, readDB := openTestDBs(t)
	writer := NewSQLStore(writeDB)
	if err := writer.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	reader := NewSQLStore(readDB)
	reader.ShareDerivedKeys(writer)

	const batches, perBatch = 60, 5
	at := time.Date(2026, 10, 30, 3, 0, 0, 0, time.UTC)
	stop := make(chan struct{})
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for r := 0; r < 3; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				objs, err := reader.ReadObjectSnapshot(ctx, "o1", "c1")
				if err != nil {
					errs <- fmt.Errorf("snapshot: %w", err)
					return
				}
				if len(objs)%perBatch != 0 {
					errs <- fmt.Errorf("snapshot saw %d objects, a torn batch of %d", len(objs), perBatch)
					return
				}
				if _, err := reader.ListObjectsByMetadataPage(ctx, string(commitmentRecordKind), "$.inbox", "management:o1", "", 50); err != nil {
					errs <- fmt.Errorf("metadata page: %w", err)
					return
				}
			}
		}()
	}
	for b := 0; b < batches; b++ {
		var batch Batch
		for i := 0; i < perBatch; i++ {
			obj := keyedObj{id: fmt.Sprintf("rec-%03d-%d", b, i), kind: commitmentRecordKind,
				meta: map[string]any{"inbox": "management:o1"}, body: `{"addressee":"a"}`, at: at.Add(time.Duration(b) * time.Second), computer: "c1"}.object(t)
			batch.Objects = append(batch.Objects, obj)
		}
		if err := writer.PutBatch(ctx, batch); err != nil {
			t.Fatalf("batch %d: %v", b, err)
		}
	}
	close(stop)
	wg.Wait()
	select {
	case err := <-errs:
		t.Fatal(err)
	default:
	}
	got, err := reader.ListObjectsByMetadataPage(ctx, string(commitmentRecordKind), "$.inbox", "management:o1", "", batches*perBatch+1)
	if err != nil || len(got) != batches*perBatch {
		t.Fatalf("read store sees %d of %d committed objects (%v)", len(got), batches*perBatch, err)
	}
}
