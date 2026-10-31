package store

import (
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestChurnAttribute names the B-tree (dbstat) of every page a batch of
// og_objects inserts dirties, on a store TestChurnBench built
// (docs/design/append-clustered-store-2026-10-31.md section 6):
//
//	CHOIR_CHURN_ATTR_DB=<path> CHOIR_CHURN_ATTR_LAYOUT=current|clustered
//
// A page dbstat does not list is a pointer-map, freelist or header page.
func TestChurnAttribute(t *testing.T) {
	path := os.Getenv("CHOIR_CHURN_ATTR_DB")
	if path == "" {
		t.Skip("CHOIR_CHURN_ATTR_DB unset")
	}
	name := os.Getenv("CHOIR_CHURN_ATTR_LAYOUT")
	layout := churnLayout{name: name, clustered: name != "current"}
	db := openChurnDB(t, path, false)
	defer db.Close()
	seed, _ := strconv.ParseUint(os.Getenv("CHOIR_CHURN_ATTR_SEED"), 10, 64)
	g := newChurnGen(99 + seed)
	w := &churnWriter{g: g, layout: layout}
	w.prepare(t, db)
	writes, _ := strconv.Atoi(os.Getenv("CHOIR_CHURN_ATTR_WRITES"))
	if writes <= 0 {
		writes = 1000
	}
	checkpointTruncate(t, db)
	for done := 0; done < writes; done += 10 {
		tx, err := db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		for j := 0; j < 10; j++ {
			if err := w.putObject(tx, g.object()); err != nil {
				t.Fatal(err)
			}
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	pages := walPageSet(t, path+"-wal")
	owner := map[uint32]string{}
	rows, err := db.Query(`SELECT name, pageno, pagetype FROM dbstat`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var obj, typ string
		var pg uint32
		if err := rows.Scan(&obj, &pg, &typ); err != nil {
			t.Fatal(err)
		}
		if _, ok := pages[pg]; ok {
			owner[pg] = obj + "/" + typ
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	// Pointer-map pages (auto_vacuum) sit at 2, then every usable/5+1
	// pages: 820 for 4 KiB pages.
	var autoVacuum int
	_ = db.QueryRow(`PRAGMA auto_vacuum`).Scan(&autoVacuum)
	count := map[string]int{}
	chunks := map[string]map[uint32]bool{}
	allChunks := map[uint32]bool{}
	for pg := range pages {
		obj, ok := owner[pg]
		if !ok {
			switch {
			case pg == 1:
				obj = "(header)"
			case autoVacuum != 0 && pg >= 2 && (pg-2)%820 == 0:
				obj = "(ptrmap)"
			default:
				obj = "(freelist/other)"
			}
		}
		count[obj]++
		class := strings.SplitN(obj, "/", 2)[0]
		if chunks[class] == nil {
			chunks[class] = map[uint32]bool{}
		}
		chunks[class][(pg-1)/1024] = true
		allChunks[(pg-1)/1024] = true
	}
	chunkCount := map[string]int{}
	for class, set := range chunks {
		chunkCount[class] = len(set)
	}
	raw, _ := json.MarshalIndent(count, "", " ")
	craw, _ := json.MarshalIndent(chunkCount, "", " ")
	t.Logf("%d distinct pages in %d distinct 4 MiB chunks for %d inserts (auto_vacuum=%d):\n%s\nchunks touched by class:\n%s", len(pages), len(allChunks), writes, autoVacuum, raw, craw)
	checkpointTruncate(t, db)
}

func walPageSet(t *testing.T, walPath string) map[uint32]struct{} {
	t.Helper()
	raw, err := os.ReadFile(walPath)
	if err != nil || len(raw) < 32 {
		t.Fatalf("wal: %v", err)
	}
	pageSize := int(binary.BigEndian.Uint32(raw[8:12]))
	s1, s2 := binary.BigEndian.Uint32(raw[16:20]), binary.BigEndian.Uint32(raw[20:24])
	out := map[uint32]struct{}{}
	for off := 32; off+24 <= len(raw); off += 24 + pageSize {
		if binary.BigEndian.Uint32(raw[off+8:]) != s1 || binary.BigEndian.Uint32(raw[off+12:]) != s2 {
			break
		}
		out[binary.BigEndian.Uint32(raw[off:])] = struct{}{}
	}
	return out
}

// TestChurnHourCompacted is the hour of writes on a compact copy of a store
// TestChurnBench built: VACUUM INTO writes every B-tree contiguously, which
// is what the compact copy (store/compact.go) and the relayout copy of S4
// install at a planned rebase. A store grown by inserts interleaves every
// index's pages through the whole file, so one random index touches every
// chunk; a copied store confines each index to its own region.
//
//	CHOIR_CHURN_HOUR_DB=<path> CHOIR_CHURN_HOUR_LAYOUT=current|clustered
func TestChurnHourCompacted(t *testing.T) {
	src := os.Getenv("CHOIR_CHURN_HOUR_DB")
	if src == "" {
		t.Skip("CHOIR_CHURN_HOUR_DB unset")
	}
	name := os.Getenv("CHOIR_CHURN_HOUR_LAYOUT")
	layout := churnLayout{name: name, clustered: name != "current"}
	path := src + ".compact"
	for _, s := range []string{"", "-wal", "-shm"} {
		_ = os.Remove(path + s)
	}
	srcDB := openChurnDB(t, src, false)
	// The copy takes the connection's auto_vacuum (as compact.go relies on):
	// INCREMENTAL keeps a pointer map, one page per 820, that every page
	// allocation and parent change rewrites; NONE has none.
	av := os.Getenv("CHOIR_CHURN_HOUR_AUTOVACUUM")
	if av == "" {
		av = "INCREMENTAL"
	}
	if _, err := srcDB.Exec(`PRAGMA auto_vacuum=` + av); err != nil {
		t.Fatal(err)
	}
	path = src + ".compact-" + strings.ToLower(av)
	for _, s := range []string{"", "-wal", "-shm"} {
		_ = os.Remove(path + s)
	}
	started := time.Now()
	if _, err := srcDB.Exec(`VACUUM INTO '` + path + `'`); err != nil {
		t.Fatal(err)
	}
	_ = srcDB.Close()
	t.Logf("%s: vacuum into in %s", name, time.Since(started).Round(time.Second))
	db := openChurnDB(t, path, false)
	defer db.Close()
	seed, _ := strconv.ParseUint(os.Getenv("CHOIR_CHURN_HOUR_SEED"), 10, 64)
	g := newChurnGen(5000 + seed)
	w := &churnWriter{g: g, layout: layout}
	w.prepare(t, db)
	// Warm the generator's live runs so updates have run objects to hit.
	tx, _ := db.Begin()
	for i := 0; i < 2000; i++ {
		if err := w.putObject(tx, g.object()); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	checkpointTruncate(t, db)
	before := chunkDigests(t, path)
	hours, _ := strconv.Atoi(os.Getenv("CHOIR_CHURN_HOUR_HOURS"))
	if hours <= 0 {
		hours = 1
	}
	for h := 1; h <= hours; h++ {
		if h > 1 {
			// Hours after the first: only the reuse against the compact
			// base, as a daily snapshot after a compaction sees it.
			for i := 0; i < 1520; i++ {
				if err := churnHourTx(w, g, db, layout, i); err != nil {
					t.Fatal(err)
				}
			}
			checkpointTruncate(t, db)
			after := chunkDigests(t, path)
			same := 0
			for i, d := range before {
				if i < len(after) && after[i] == d {
					same++
				}
			}
			t.Logf("%s after %d hours: %d of %d chunks, %d same as the compact base", name, h, len(after), len(before), same)
			continue
		}
		for i := 0; i < 1520; i++ {
			tx, err := db.Begin()
			if err != nil {
				t.Fatal(err)
			}
			n := 10
			if i%2 == 0 {
				n = 11
			}
			for j := 0; j < n; j++ {
				var err error
				switch {
				case j >= 7:
					err = w.putMemory(tx, g.memory(layout.clustered))
				case g.rng.IntN(10) < 3:
					if o, ok := g.update(); ok {
						err = w.putObject(tx, o)
						break
					}
					fallthrough
				default:
					err = w.putObject(tx, g.object())
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
		}
		dirty, frames := walDistinctPages(t, path+"-wal")
		checkpointTruncate(t, db)
		after := chunkDigests(t, path)
		same, held := 0, 0
		seen := map[string]bool{}
		for i, d := range before {
			seen[d] = true
			if i < len(after) && after[i] == d {
				same++
			}
		}
		for _, d := range after {
			if seen[d] {
				held++
			}
		}
		t.Logf("auto_vacuum=%s", av)
		t.Logf("%s compacted hour: %d chunks before, %d after, %d same position, %d held, %d distinct pages, %d frames",
			name, len(before), len(after), same, held, dirty, frames)
	}
}

// churnHourTx is one transaction of the hour: seven og writes (30% run
// updates) and three or four run-memory entries.
func churnHourTx(w *churnWriter, g *churnGen, db *sql.DB, layout churnLayout, i int) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	n := 10
	if i%2 == 0 {
		n = 11
	}
	for j := 0; j < n; j++ {
		switch {
		case j >= 7:
			err = w.putMemory(tx, g.memory(layout.clustered))
		case g.rng.IntN(10) < 3:
			if o, ok := g.update(); ok {
				err = w.putObject(tx, o)
				break
			}
			err = w.putObject(tx, g.object())
		default:
			err = w.putObject(tx, g.object())
		}
		if err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}
