package objectgraph

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	embedded "github.com/dolthub/driver/v2"
)

// Evidence for docs/design/store-engine-concurrency-2026-10-21.md: what the
// embedded engine does when more than one connection reads while one writes.
// The engine mutex and the one-connection pools were put there for a race that
// the pinned Dolt (6bea93f2, 2026-07-16) fixed; these probes ask what is still
// true on the pinned version. They assert only what the design relies on:
//
//  1. a second connection sees every write the first connection committed
//     before the read started (no read-pool snapshot older than a commit);
//  2. several connections reading JSON, with HTML-escaped content, while a
//     writer upserts, return every row whole and never fail (run with -race to
//     put the shared prolly-node buffers under the detector).

// openTestDoltConnections opens one workspace and returns a writer pool of one
// connection and a reader pool of readers connections over the SAME connector,
// as store.Open does for db and readDB.
func openTestDoltConnections(t *testing.T, readers int) (writer, reader *sql.DB) {
	t.Helper()
	root := t.TempDir()
	rootCfg, err := embedded.ParseDSN(fmt.Sprintf("file://%s?commitname=Choir&commitemail=system@choir.local&multistatements=true", root))
	if err != nil {
		t.Fatal(err)
	}
	rootConnector, err := embedded.NewConnector(rootCfg)
	if err != nil {
		t.Fatal(err)
	}
	rootDB := sql.OpenDB(rootConnector)
	if _, err := rootDB.Exec("CREATE DATABASE IF NOT EXISTS testdb"); err != nil {
		t.Fatal(err)
	}
	_ = rootDB.Close()
	_ = rootConnector.Close()
	cfg, err := embedded.ParseDSN(fmt.Sprintf("file://%s?commitname=Choir&commitemail=system@choir.local&database=testdb&multistatements=true&clientfoundrows=true", root))
	if err != nil {
		t.Fatal(err)
	}
	connector, err := embedded.NewConnector(cfg)
	if err != nil {
		t.Fatal(err)
	}
	writer = sql.OpenDB(connector)
	writer.SetMaxOpenConns(1)
	writer.SetMaxIdleConns(1)
	reader = sql.OpenDB(connector)
	reader.SetMaxOpenConns(readers)
	reader.SetMaxIdleConns(readers)
	t.Cleanup(func() {
		_ = reader.Close()
		_ = writer.Close()
		_ = connector.Close()
	})
	return writer, reader
}

func TestSecondConnectionSeesEveryCommittedWrite(t *testing.T) {
	ctx := context.Background()
	writerDB, readerDB := openTestDoltConnections(t, 1)
	writeStore := NewDoltStore(writerDB)
	if err := writeStore.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	readStore := NewDoltStore(readerDB)
	readStore.ShareDerivedKeys(writeStore)
	at := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	for i := 0; i < 300; i++ {
		k := keyedObj{id: fmt.Sprintf("rec-%03d", i%25), kind: commitmentRecordKind, meta: map[string]any{"n": i}, body: mustJSON(map[string]any{"addressee": "a", "n": i}), at: at.Add(time.Duration(i) * time.Second)}
		obj := k.object(t)
		if err := writeStore.PutObject(ctx, obj); err != nil {
			t.Fatal(err)
		}
		got, err := readStore.GetObject(ctx, obj.CanonicalID)
		if err != nil {
			t.Fatalf("write %d: the reader connection does not see %s: %v", i, obj.CanonicalID, err)
		}
		if got.ContentHash != obj.ContentHash {
			t.Fatalf("write %d: the reader connection sees an older version of %s (hash %s, wrote %s)", i, obj.CanonicalID, got.ContentHash, obj.ContentHash)
		}
	}
}

func TestSeveralReadersWhileOneWriterReturnWholeRows(t *testing.T) {
	ctx := context.Background()
	const readers = 4
	writerDB, readerDB := openTestDoltConnections(t, readers)
	writeStore := NewDoltStore(writerDB)
	if err := writeStore.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	// Content that takes the engine's JSON paths with HTML-escapable text.
	const rows = 120
	for i := 0; i < rows; i++ {
		k := keyedObj{id: fmt.Sprintf("rec-%03d", i), kind: commitmentRecordKind, meta: map[string]any{"title": "<b>a & b</b>", "n": i},
			body: mustJSON(map[string]any{"addressee": fmt.Sprintf("a%d", i%5), "text": "<script>x</script> & more", "n": i}), at: at}
		if err := writeStore.PutObject(ctx, k.object(t)); err != nil {
			t.Fatal(err)
		}
	}
	var stop atomic.Bool
	var reads, failures atomic.Int64
	var wg sync.WaitGroup
	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func(r int) {
			defer wg.Done()
			for n := 0; !stop.Load(); n++ {
				var count int
				err := readerDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM og_objects
					WHERE object_kind = ? AND JSON_UNQUOTE(JSON_EXTRACT(CAST(CAST(body AS CHAR) AS JSON), '$.addressee')) = ?`,
					string(commitmentRecordKind), fmt.Sprintf("a%d", (n+r)%5)).Scan(&count)
				if err != nil || count != rows/5 {
					failures.Add(1)
					t.Errorf("reader %d: count %d, err %v (want %d)", r, count, err, rows/5)
					return
				}
				var meta string
				if err := readerDB.QueryRowContext(ctx, `SELECT JSON_UNQUOTE(JSON_EXTRACT(CAST(metadata AS JSON), '$.title')) FROM og_objects WHERE canonical_id = ?`,
					fmt.Sprintf("rec-%03d", n%rows)).Scan(&meta); err != nil || meta != "<b>a & b</b>" {
					failures.Add(1)
					t.Errorf("reader %d: title %q, err %v", r, meta, err)
					return
				}
				reads.Add(1)
			}
		}(r)
	}
	deadline := time.Now().Add(6 * time.Second)
	for n := 0; time.Now().Before(deadline) && failures.Load() == 0; n++ {
		// Rewrites with the same keys and new content: the readers' counts
		// stay constant, their rows change underneath them.
		i := n % rows
		k := keyedObj{id: fmt.Sprintf("rec-%03d", i), kind: commitmentRecordKind, meta: map[string]any{"title": "<b>a & b</b>", "n": i, "rev": n},
			body: mustJSON(map[string]any{"addressee": fmt.Sprintf("a%d", i%5), "text": fmt.Sprintf("<script>%d</script> & more", n), "n": i}), at: at.Add(time.Duration(n+1) * time.Second)}
		if err := writeStore.PutObject(ctx, k.object(t)); err != nil {
			t.Fatalf("writer: %v", err)
		}
	}
	stop.Store(true)
	wg.Wait()
	if reads.Load() == 0 {
		t.Fatal("the readers made no reads")
	}
	t.Logf("%d reader iterations on %d connections beside one writer, no failures", reads.Load(), readers)
}
