package store

// TestPageChurnMeasure is landing 0 of
// docs/design/restore-bases-store-once-2026-11-15.md: a first reading of p, the
// share of fixed page units whose content changed in an interval, on the
// synthetic churn store, for two starting conditions (a compacted file, and a
// file that grew in place).
//
// For each condition it checkpoints (TRUNCATE) a quiet file, hashes every
// page (8 bytes each) through sqlite_dbpage in one read transaction (the read
// the snapshot producer does, internal/snapshot/snapshot.go), runs 1, 6 and 24
// hours of the paper's hourly write volume (churnHourTx, as TestCompactMeasure
// does), and hashes again after each of those hours. It prints p at units of
// 1, 32, 128 and 1,024 pages, the index/table split of the changed pages from
// dbstat, the top ten trees by changed pages, the packed size of the changed
// pages and their zstd size, and the time and bytes of the page-hash read.
//
//	CHOIR_CHURN_BENCH_OG_ROWS=600000 CHOIR_CHURN_BENCH_DIR=/dir CHOIR_CHURN_BENCH_LAYOUTS=current \
//	  go test ./internal/store -run TestChurnBench -v -timeout 3h
//	CHOIR_PAGE_CHURN_DB=/dir/churn-current.db go test ./internal/store -run TestPageChurnMeasure -v -timeout 6h
//
// CHOIR_PAGE_CHURN_CONDITIONS (default "compacted,grown") picks the starting
// conditions. The source file is left untouched; copies sit beside it and are
// removed.

import (
	"context"
	"database/sql"
	"fmt"
	"hash/maphash"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
)

const pageChurnPageSize = 4096

var pageChurnUnits = []int{1, 32, 128, 1024}

var pageChurnSeed = maphash.MakeSeed()

// pageHashes reads every page through sqlite_dbpage in one read transaction
// and returns the 8-byte hash of each (index 0 is page 1), the elapsed time
// and the bytes read.
func pageHashes(t *testing.T, db *sql.DB) (hashes []uint64, took time.Duration, bytes int64) {
	t.Helper()
	started := time.Now()
	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	rows, err := tx.Query(`SELECT pgno, data FROM sqlite_dbpage`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var pgno int
		var data []byte
		if err := rows.Scan(&pgno, &data); err != nil {
			t.Fatal(err)
		}
		if pgno != len(hashes)+1 {
			t.Fatalf("sqlite_dbpage returned page %d after %d pages", pgno, len(hashes))
		}
		hashes = append(hashes, maphash.Bytes(pageChurnSeed, data))
		bytes += int64(len(data))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return hashes, time.Since(started), bytes
}

// pageOwners maps each page to "kind name" ("index idx_x", "table og_objects")
// from dbstat and sqlite_schema; pages dbstat does not list are "(unowned)".
func pageOwners(t *testing.T, db *sql.DB, pages int) []string {
	t.Helper()
	kind := map[string]string{}
	rows, err := db.Query(`SELECT name, type FROM sqlite_schema WHERE type IN ('table','index')`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var n, ty string
		if err := rows.Scan(&n, &ty); err != nil {
			t.Fatal(err)
		}
		kind[n] = ty
	}
	rows.Close()
	owners := make([]string, pages)
	for i := range owners {
		owners[i] = "(unowned: freelist, ptrmap or lock page)"
	}
	rows, err = db.Query(`SELECT pageno, name FROM dbstat`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var p int
		var n string
		if err := rows.Scan(&p, &n); err != nil {
			t.Fatal(err)
		}
		if p >= 1 && p <= pages {
			ty := kind[n]
			if ty == "" {
				ty = "other"
			}
			owners[p-1] = ty + " " + n
		}
	}
	return owners
}

func zstdPackedSize(t *testing.T, db *sql.DB, changed []int) (raw, packed int64) {
	t.Helper()
	enc, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedDefault), zstd.WithEncoderConcurrency(1))
	if err != nil {
		t.Fatal(err)
	}
	defer enc.Close()
	const chunk = 4 << 20 // the snapshot chunk size; one zstd frame each
	buf := make([]byte, 0, chunk)
	flush := func() {
		if len(buf) > 0 {
			packed += int64(len(enc.EncodeAll(buf, nil)))
			buf = buf[:0]
		}
	}
	stmt, err := db.Prepare(`SELECT data FROM sqlite_dbpage WHERE pgno = ?`)
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	for _, p := range changed { // sorted by page number: a dense pack
		var data []byte
		if err := stmt.QueryRow(p).Scan(&data); err != nil {
			t.Fatal(err)
		}
		if len(buf)+4+len(data) > chunk {
			flush()
		}
		buf = append(buf, byte(p>>24), byte(p>>16), byte(p>>8), byte(p))
		buf = append(buf, data...)
		raw += int64(4 + len(data))
	}
	flush()
	return raw, packed
}

type pageChurnRow struct {
	condition string
	hours     int
	pages0    int
	pages1    int
	changed   int
	newPages  int
	p         map[int]float64
	unitBytes map[int]int64
	idxPages  int
	tblPages  int
	other     int
	top       []string
	rawPack   int64
	zstdPack  int64
	readTime  time.Duration
	readBytes int64
}

func (r pageChurnRow) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%-9s %3dh pages %d -> %d (%d new), changed pages %d (%.2f%%)\n", r.condition, r.hours, r.pages0, r.pages1, r.newPages, r.changed, 100*float64(r.changed)/float64(r.pages1))
	fmt.Fprintf(&b, "    p_1=%.4f p_32=%.4f p_128=%.4f p_1024=%.4f\n", r.p[1], r.p[32], r.p[128], r.p[1024])
	fmt.Fprintf(&b, "    bytes shipped if units: 1p %d MB, 32p %d MB, 128p %d MB, 1024p %d MB (whole file %d MB)\n",
		r.unitBytes[1]>>20, r.unitBytes[32]>>20, r.unitBytes[128]>>20, r.unitBytes[1024]>>20, int64(r.pages1)*pageChurnPageSize>>20)
	fmt.Fprintf(&b, "    changed pages by tree kind: index %d (%.1f%%), table %d (%.1f%%), unowned/other %d\n",
		r.idxPages, 100*float64(r.idxPages)/float64(max(r.changed, 1)), r.tblPages, 100*float64(r.tblPages)/float64(max(r.changed, 1)), r.other)
	fmt.Fprintf(&b, "    packed changed pages: %d bytes raw (4-byte pgno + page), zstd %d bytes (ratio %.2f)\n", r.rawPack, r.zstdPack, float64(r.rawPack)/float64(max(r.zstdPack, 1)))
	fmt.Fprintf(&b, "    page-hash read: %s, %d bytes (%.0f MB/s)\n", r.readTime.Round(time.Millisecond), r.readBytes, float64(r.readBytes)/1e6/r.readTime.Seconds())
	b.WriteString("    top ten trees by changed pages:\n")
	for _, l := range r.top {
		b.WriteString("      " + l + "\n")
	}
	return b.String()
}

func TestPageChurnMeasure(t *testing.T) {
	src := os.Getenv("CHOIR_PAGE_CHURN_DB")
	if src == "" {
		t.Skip("CHOIR_PAGE_CHURN_DB unset")
	}
	conditions := []string{"compacted", "grown"}
	if c := os.Getenv("CHOIR_PAGE_CHURN_CONDITIONS"); c != "" {
		conditions = strings.Split(c, ",")
	}
	horizons := map[int]bool{1: true, 6: true, 24: true}
	var report []pageChurnRow
	for _, cond := range conditions {
		path := src + "." + cond + ".pagechurn"
		cleanup := func() {
			for _, s := range []string{"", "-wal", "-shm"} {
				_ = os.Remove(path + s)
			}
		}
		cleanup()
		defer cleanup()
		switch cond {
		case "compacted":
			if err := vacuumIntoCopy(context.Background(), src, path); err != nil {
				t.Fatal(err)
			}
		case "grown":
			copyFileForMeasure(t, src, path)
		default:
			t.Fatalf("unknown condition %q", cond)
		}

		db := openChurnDB(t, path, false)
		g := newChurnGen(777)
		layout := churnLayout{name: "current"}
		w := &churnWriter{g: g, layout: layout}
		w.prepare(t, db)
		// The same warm-up as TestCompactMeasure: 2,000 objects, then the base.
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
		base, baseTook, baseBytes := pageHashes(t, db)
		// Equivalence: after a TRUNCATE checkpoint the WAL is empty, so the
		// dbpage view and the file's bytes are the same pages. Check it once.
		fileHashes := fileGoPageHashes(t, path)
		if len(fileHashes) != len(base) {
			t.Fatalf("file has %d pages, sqlite_dbpage %d", len(fileHashes), len(base))
		}
		for i := range base {
			if fileHashes[i] != base[i] {
				t.Fatalf("page %d differs between the file and sqlite_dbpage", i+1)
			}
		}
		t.Logf("%s: start %d pages (%d MB); baseline page-hash read %s, %d bytes; file read matches sqlite_dbpage page for page",
			cond, len(base), fileSizeOrZero(path)>>20, baseTook.Round(time.Millisecond), baseBytes)

		for h := 1; h <= 24; h++ {
			for i := 0; i < 1520; i++ {
				if err := churnHourTx(w, g, db, layout, i); err != nil {
					t.Fatal(err)
				}
			}
			if !horizons[h] {
				continue
			}
			checkpointTruncate(t, db)
			now, took, nbytes := pageHashes(t, db)
			row := pageChurnRow{condition: cond, hours: h, pages0: len(base), pages1: len(now), p: map[int]float64{}, unitBytes: map[int]int64{}, readTime: took, readBytes: nbytes}
			var changed []int
			for i := range now {
				if i >= len(base) {
					row.newPages++
					changed = append(changed, i+1)
				} else if now[i] != base[i] {
					changed = append(changed, i+1)
				}
			}
			row.changed = len(changed)
			for _, u := range pageChurnUnits {
				units := (len(now) + u - 1) / u
				hit := map[int]struct{}{}
				for _, p := range changed {
					hit[(p-1)/u] = struct{}{}
				}
				row.p[u] = float64(len(hit)) / float64(units)
				row.unitBytes[u] = int64(len(hit)) * int64(u) * pageChurnPageSize
			}
			owners := pageOwners(t, db, len(now))
			byTree := map[string]int{}
			for _, p := range changed {
				o := owners[p-1]
				byTree[o]++
				switch {
				case strings.HasPrefix(o, "index "):
					row.idxPages++
				case strings.HasPrefix(o, "table "):
					row.tblPages++
				default:
					row.other++
				}
			}
			type kv struct {
				k string
				n int
			}
			var all []kv
			for k, n := range byTree {
				all = append(all, kv{k, n})
			}
			sort.Slice(all, func(i, j int) bool { return all[i].n > all[j].n || all[i].n == all[j].n && all[i].k < all[j].k })
			for i := 0; i < len(all) && i < 10; i++ {
				row.top = append(row.top, fmt.Sprintf("%-60s %8d pages (%.1f%% of changed)", all[i].k, all[i].n, 100*float64(all[i].n)/float64(max(row.changed, 1))))
			}
			row.rawPack, row.zstdPack = zstdPackedSize(t, db, changed)
			report = append(report, row)
			t.Logf("\n%s", row)
		}
		_ = db.Close()
		cleanup()
	}
	var b strings.Builder
	for _, r := range report {
		b.WriteString(r.String())
	}
	t.Logf("\nFULL TABLE\n%s", b.String())
}

// fileGoPageHashes hashes the file's pages directly, 4 KiB at a time.
func fileGoPageHashes(t *testing.T, path string) []uint64 {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	buf := make([]byte, pageChurnPageSize)
	var out []uint64
	for {
		n, err := f.Read(buf)
		if n == pageChurnPageSize {
			out = append(out, maphash.Bytes(pageChurnSeed, buf))
		} else if n > 0 {
			t.Fatalf("short page read of %d bytes", n)
		}
		if err != nil {
			break
		}
	}
	return out
}
