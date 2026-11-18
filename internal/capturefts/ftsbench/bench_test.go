// Package ftsbench is the measurement behind docs/reference/CAPTURE-FULLTEXT.md
// section 1: four ways to make captured page text searchable, on one
// synthetic corpus shaped like the real one (Zipf vocabulary, a mix of
// short feed items and long articles). It holds no product code. It runs
// only when CHOIR_FTS_BENCH_DOCS is set (the number of pages), for example
//
//	CHOIR_FTS_BENCH_DOCS=20000 go test ./internal/capturefts/ftsbench -run TestBench -v -timeout 3h
//
// The corpus is made of invented words; no third-party text is used.
package ftsbench

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/yusefmosiah/go-choir/internal/capturefts"
	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/dbx/dbxtest"
)

func TestMain(m *testing.M) { os.Exit(dbxtest.Main(m)) }

const (
	vocab       = 150000
	passageToks = 300
	k1, bb      = 1.2, 0.75
)

var cons = []byte("bdfgklmnprstvz")
var vow = []byte("aeiou")

// wordOf is the invented word of a Zipf rank: alternating consonants and
// vowels, so common words are short.
func wordOf(r int) string {
	var b []byte
	n := r
	for {
		b = append(b, cons[n%len(cons)])
		n /= len(cons)
		b = append(b, vow[n%len(vow)])
		n /= len(vow)
		if n == 0 {
			break
		}
	}
	return string(b)
}

type doc struct {
	id    int
	title []string
	body  []string
}

func bytesOf(d doc) int {
	n := 0
	for _, w := range d.body {
		n += len(w) + 1
	}
	return n
}

// corpus draws n pages: 30% short feed items (median 80 words), 69% articles
// (median 1,250), 1% long documents (median 15,000), capped at 60,000 words.
func corpus(n int, seed int64) []doc {
	rng := rand.New(rand.NewSource(seed))
	zipf := rand.NewZipf(rng, 1.07, 1, vocab-1)
	words := make([]string, vocab)
	for i := range words {
		words[i] = wordOf(i)
	}
	docs := make([]doc, n)
	for i := range docs {
		var median, sigma float64
		switch p := rng.Float64(); {
		case p < 0.30:
			median, sigma = 80, 0.6
		case p < 0.99:
			median, sigma = 1250, 0.7
		default:
			median, sigma = 15000, 0.5
		}
		length := int(math.Exp(math.Log(median) + sigma*rng.NormFloat64()))
		length = max(10, min(length, 60000))
		body := make([]string, length)
		for j := range body {
			body[j] = words[zipf.Uint64()]
		}
		title := make([]string, 6+rng.Intn(6))
		for j := range title {
			title[j] = words[(50+int(zipf.Uint64()))%vocab]
		}
		docs[i] = doc{id: i + 1, title: title, body: body}
	}
	return docs
}

type query struct{ terms []string }

// queries draws count queries of size words from random pages: the words
// are taken from the page body, so every query has at least one hit.
func queries(docs []doc, count, size int, seed int64, dropCommon bool) []query {
	rng := rand.New(rand.NewSource(seed))
	common := map[string]bool{}
	if dropCommon {
		// The 50 most frequent words stand for the stop words a query drops.
		for r := 0; r < 50; r++ {
			common[wordOf(r)] = true
		}
	}
	var out []query
	for len(out) < count {
		d := docs[rng.Intn(len(docs))]
		if len(d.body) < size*3 {
			continue
		}
		seen := map[string]bool{}
		var terms []string
		for len(terms) < size {
			w := d.body[rng.Intn(len(d.body))]
			if !seen[w] && !common[w] {
				seen[w] = true
				terms = append(terms, w)
			}
		}
		out = append(out, query{terms})
	}
	return out
}

// q12raw are 12-word queries that keep the most common words.
var q12raw []query

type lat []time.Duration

func (l lat) pct(p float64) time.Duration {
	s := append(lat(nil), l...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	return s[int(float64(len(s)-1)*p)]
}

func measure(qs []query, run func(q query) error) (lat, error) {
	var out lat
	for _, q := range qs {
		t := time.Now()
		if err := run(q); err != nil {
			return nil, err
		}
		out = append(out, time.Since(t))
	}
	return out, nil
}

func report(t *testing.T, name string, build time.Duration, size int64, extra string, l4, l12 lat, ins time.Duration) {
	t.Logf("RESULT %-34s build=%-9s size=%-8s q4 p50=%-8s p95=%-8s q12 p50=%-8s p95=%-8s insert/doc=%-8s %s",
		name, build.Round(time.Millisecond), mib(size), l4.pct(.5).Round(10*time.Microsecond), l4.pct(.95).Round(10*time.Microsecond),
		l12.pct(.5).Round(10*time.Microsecond), l12.pct(.95).Round(10*time.Microsecond), ins.Round(10*time.Microsecond), extra)
}

func rawp(run func(q query) error) string {
	l, err := measure(q12raw, run)
	if err != nil {
		return "raw12 error: " + err.Error()
	}
	return fmt.Sprintf("q12raw(with common words) p50=%s p95=%s", l.pct(.5).Round(10*time.Microsecond), l.pct(.95).Round(10*time.Microsecond))
}

func mib(n int64) string { return fmt.Sprintf("%.0fMiB", float64(n)/(1<<20)) }

func fileSize(path string) int64 {
	var n int64
	for _, suffix := range []string{"", "-wal"} {
		if st, err := os.Stat(path + suffix); err == nil {
			n += st.Size()
		}
	}
	return n
}

func rss() string {
	b, _ := os.ReadFile("/proc/self/status")
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "VmHWM") || strings.HasPrefix(line, "VmRSS") {
			return strings.Join(strings.Fields(line), " ")
		}
	}
	return ""
}

func TestBench(t *testing.T) {
	n, _ := strconv.Atoi(os.Getenv("CHOIR_FTS_BENCH_DOCS"))
	if n == 0 {
		t.Skip("CHOIR_FTS_BENCH_DOCS not set")
	}
	docs := corpus(n, 1)
	var textBytes, toks int
	for _, d := range docs {
		textBytes += bytesOf(d)
		toks += len(d.body)
	}
	t.Logf("CORPUS pages=%d text=%s mean=%dB tokens=%d (%.0f per page)", n, mib(int64(textBytes)), textBytes/n, toks, float64(toks)/float64(n))
	q4, q12 := queries(docs, 200, 4, 7, true), queries(docs, 200, 12, 8, true)
	q12raw = queries(docs, 100, 12, 9, false)
	extraDocs := corpus(100, 99)
	for i := range extraDocs {
		extraDocs[i].id = n + 1 + i
	}
	dir := t.TempDir()

	// detail=column and detail=none on a contentless table make bm25() return 0
	// (seen, SQLite 3.5x in modernc v1.49.0), so only detail=full is measured.
	t.Run("sqlite_fts5_passages", func(t *testing.T) { benchFTS5(t, dir, docs, extraDocs, q4, q12, true, "full") })
	t.Run("sqlite_fts5_pages", func(t *testing.T) { benchFTS5(t, dir, docs, extraDocs, q4, q12, false, "full") })
	t.Run("memory_inverted_pages", func(t *testing.T) { benchMemory(t, docs, extraDocs, q4, q12) })
	t.Run("postgres_tsvector_gin_pages", func(t *testing.T) { benchTSVector(t, docs, extraDocs, q4, q12) })
	t.Run("postgres_term_table_pages", func(t *testing.T) { benchTermTable(t, docs, extraDocs, q4, q12) })
}

// ---- (a) SQLite FTS5 ----

type unit struct {
	id        int
	doc       int
	title     string
	body      string
	ordinal   int
	passageOf int
}

func units(docs []doc, passages bool, base int) []unit {
	var out []unit
	id := base
	for _, d := range docs {
		title := strings.Join(d.title, " ")
		if !passages {
			id++
			out = append(out, unit{id: id, doc: d.id, title: title, body: strings.Join(d.body, " ")})
			continue
		}
		for o, start := 0, 0; start < len(d.body) || o == 0; o, start = o+1, start+passageToks {
			end := min(start+passageToks, len(d.body))
			id++
			out = append(out, unit{id: id, doc: d.id, title: title, body: strings.Join(d.body[start:end], " "), ordinal: o})
			if end == len(d.body) {
				break
			}
		}
	}
	return out
}

func benchFTS5(t *testing.T, dir string, docs, extra []doc, q4, q12 []query, passages bool, detail string) {
	ctx := context.Background()
	path := filepath.Join(dir, strings.ReplaceAll(t.Name(), "/", "_")+".db")
	open := func() *sql.DB {
		db, err := sql.Open("sqlite", "file:"+path+"?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=cache_size(-65536)&_pragma=busy_timeout(5000)")
		if err != nil {
			t.Fatal(err)
		}
		db.SetMaxOpenConns(1)
		return db
	}
	db := open()
	if _, err := db.Exec(`CREATE VIRTUAL TABLE f USING fts5(title, body, content='', contentless_delete=1, tokenize='unicode61 remove_diacritics 2', detail=` + detail + `)`); err != nil {
		t.Fatalf("fts5 create: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO f(f, rank) VALUES('rank', 'bm25(3.0, 1.0)')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE p (id INTEGER PRIMARY KEY, doc INTEGER NOT NULL, ordinal INTEGER NOT NULL, fetched INTEGER NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	us := units(docs, passages, 0)
	start := time.Now()
	insert := func(tx *sql.Tx, us []unit) error {
		for _, u := range us {
			if _, err := tx.Exec(`INSERT INTO f(rowid, title, body) VALUES (?, ?, ?)`, u.id, u.title, u.body); err != nil {
				return err
			}
			if _, err := tx.Exec(`INSERT INTO p(id, doc, ordinal, fetched) VALUES (?, ?, ?, ?)`, u.id, u.doc, u.ordinal, u.doc); err != nil {
				return err
			}
		}
		return nil
	}
	for i := 0; i < len(us); i += 2000 {
		tx, _ := db.Begin()
		if err := insert(tx, us[i:min(i+2000, len(us))]); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	build := time.Since(start)
	if _, err := db.Exec(`INSERT INTO f(f) VALUES('optimize')`); err != nil {
		t.Fatal(err)
	}
	_, _ = db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
	if _, err := db.Exec(`CREATE INDEX p_fetched ON p(fetched)`); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	size := fileSize(path)

	// A restart: reopen and time the first query (cold page cache for this
	// handle; the OS cache is warm).
	reopen := time.Now()
	db = open()
	defer db.Close()
	run := func(q query) error {
		quoted := make([]string, len(q.terms))
		for i, w := range q.terms {
			quoted[i] = `"` + w + `"`
		}
		rows, err := db.QueryContext(ctx, `SELECT f.rowid, p.doc FROM f JOIN p ON p.id = f.rowid WHERE f MATCH ? ORDER BY bm25(f, 3.0, 1.0) LIMIT 50`, strings.Join(quoted, " OR "))
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var a, b int
			if err := rows.Scan(&a, &b); err != nil {
				return err
			}
		}
		return rows.Err()
	}
	if err := run(q4[0]); err != nil {
		t.Fatal(err)
	}
	firstQuery := time.Since(reopen)
	l4, err := measure(q4, run)
	if err != nil {
		t.Fatal(err)
	}
	l12, err := measure(q12, run)
	if err != nil {
		t.Fatal(err)
	}
	// The core ranking alone (no joins), and with the query's common terms
	// pruned: terms found in more than 5% of the rows are dropped when at
	// least three terms remain (the three rarest otherwise).
	var rowsTotal int
	_ = db.QueryRow(`SELECT count(*) FROM p`).Scan(&rowsTotal)
	dfCache := map[string]int{}
	df := func(term string) int {
		if v, ok := dfCache[term]; ok {
			return v
		}
		var n int
		_ = db.QueryRow(`SELECT count(*) FROM f WHERE f MATCH ?`, `"`+term+`"`).Scan(&n)
		dfCache[term] = n
		return n
	}
	core := func(terms []string) error {
		quoted := make([]string, len(terms))
		for i, w := range terms {
			quoted[i] = `"` + w + `"`
		}
		rows, err := db.QueryContext(ctx, `SELECT rowid FROM f WHERE f MATCH ? ORDER BY bm25(f, 3.0, 1.0) LIMIT 300`, strings.Join(quoted, " OR "))
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
		}
		return rows.Err()
	}
	prune := func(terms []string) []string {
		type td struct {
			t string
			n int
		}
		var all []td
		for _, w := range terms {
			all = append(all, td{w, df(w)})
		}
		sort.Slice(all, func(i, j int) bool { return all[i].n < all[j].n })
		var keep []string
		for i, x := range all {
			if i < 3 || x.n*20 <= rowsTotal {
				keep = append(keep, x.t)
			}
		}
		return keep
	}
	for _, qs := range [][]query{q12, q12raw} {
		for _, q := range qs {
			prune(q.terms) // warm the document-frequency cache; its cost is reported below
		}
	}
	coreL, _ := measure(q12, func(q query) error { return core(q.terms) })
	coreRaw, _ := measure(q12raw, func(q query) error { return core(q.terms) })
	prunedL, _ := measure(q12, func(q query) error { return core(prune(q.terms)) })
	prunedRaw, _ := measure(q12raw, func(q query) error { return core(prune(q.terms)) })
	t.Logf("RESULT %s core(no joins) q12 p50=%s p95=%s raw p50=%s p95=%s | pruned q12 p50=%s p95=%s raw p50=%s p95=%s (df cache holds %d terms)", t.Name(),
		coreL.pct(.5), coreL.pct(.95), coreRaw.pct(.5), coreRaw.pct(.95), prunedL.pct(.5), prunedL.pct(.95), prunedRaw.pct(.5), prunedRaw.pct(.95), len(dfCache))
	// Windowed: the same 12-word query restricted to the newest tenth.
	winRun := func(q query) error {
		quoted := make([]string, len(q.terms))
		for i, w := range q.terms {
			quoted[i] = `"` + w + `"`
		}
		rows, err := db.QueryContext(ctx, `SELECT f.rowid FROM f JOIN p ON p.id = f.rowid WHERE f MATCH ? AND p.fetched >= ? ORDER BY bm25(f, 3.0, 1.0) LIMIT 50`, strings.Join(quoted, " OR "), len(docs)*9/10)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
		}
		return rows.Err()
	}
	lw, err := measure(q12, winRun)
	if err != nil {
		t.Fatal(err)
	}
	// Incremental insert: 100 new pages, one transaction each.
	next := len(us)
	insStart := time.Now()
	for _, d := range extra {
		tx, _ := db.Begin()
		one := units([]doc{d}, passages, next)
		next += len(one)
		if err := insert(tx, one); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	ins := time.Since(insStart) / time.Duration(len(extra))
	var rowsN int
	_ = db.QueryRow(`SELECT count(*) FROM p`).Scan(&rowsN)
	report(t, t.Name(), build, size, fmt.Sprintf("rows=%d first_query_after_open=%s q12_window10pct p50=%s p95=%s %s %s", rowsN, firstQuery.Round(time.Millisecond), lw.pct(.5).Round(10*time.Microsecond), lw.pct(.95).Round(10*time.Microsecond), rawp(run), rss()), l4, l12, ins)
}

// ---- (d) in-memory inverted index, as publication_pieces_index.go ----

type posting struct {
	doc int32
	tf  uint16
}

type memIndex struct {
	postings map[string][]posting
	length   []int32
	total    int64
}

func (ix *memIndex) add(d doc) {
	slot := int32(len(ix.length))
	counts := map[string]uint16{}
	for _, w := range d.body {
		counts[w]++
	}
	for _, w := range d.title {
		counts[w] += 3
	}
	for w, c := range counts {
		ix.postings[w] = append(ix.postings[w], posting{slot, c})
	}
	l := int32(len(d.body) + 3*len(d.title))
	ix.length = append(ix.length, l)
	ix.total += int64(l)
}

func (ix *memIndex) search(terms []string, k int) []int32 {
	avg := float64(ix.total) / float64(len(ix.length))
	scores := map[int32]float64{}
	N := float64(len(ix.length))
	for _, w := range terms {
		ps := ix.postings[w]
		df := float64(len(ps))
		idf := math.Log(1 + (N-df+.5)/(df+.5))
		for _, p := range ps {
			tf := float64(p.tf)
			scores[p.doc] += idf * tf * (k1 + 1) / (tf + k1*(1-bb+bb*float64(ix.length[p.doc])/avg))
		}
	}
	type kv struct {
		d int32
		s float64
	}
	all := make([]kv, 0, len(scores))
	for d, s := range scores {
		all = append(all, kv{d, s})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].s > all[j].s })
	var out []int32
	for i := 0; i < len(all) && i < k; i++ {
		out = append(out, all[i].d)
	}
	return out
}

func benchMemory(t *testing.T, docs, extra []doc, q4, q12 []query) {
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	ix := &memIndex{postings: map[string][]posting{}}
	for _, d := range docs {
		ix.add(d)
	}
	build := time.Since(start)
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	heap := int64(after.HeapAlloc) - int64(before.HeapAlloc)
	run := func(q query) error { ix.search(q.terms, 50); return nil }
	l4, _ := measure(q4, run)
	l12, _ := measure(q12, run)
	insStart := time.Now()
	for _, d := range extra {
		ix.add(d)
	}
	ins := time.Since(insStart) / time.Duration(len(extra))
	report(t, t.Name(), build, 0, "heap="+mib(heap)+" (build excludes reading text from disk; restart rebuilds all) "+rawp(run), l4, l12, ins)
}

// ---- (b) PostgreSQL tsvector + GIN ----

func benchTSVector(t *testing.T, docs, extra []doc, q4, q12 []query) {
	ctx := context.Background()
	db, err := dbx.OpenPostgres(dbxtest.PostgresDSN(t), dbx.PGOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE d (id int primary key, tsv tsvector NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	add := func(d doc) error {
		_, err := db.ExecContext(ctx, `INSERT INTO d(id, tsv) VALUES ($1, setweight(to_tsvector('simple', $2), 'A') || to_tsvector('simple', $3))`,
			d.id, strings.Join(d.title, " "), strings.Join(d.body, " "))
		return err
	}
	for _, d := range docs {
		if err := add(d); err != nil {
			t.Fatalf("insert %d (%d words): %v", d.id, len(d.body), err)
		}
	}
	if _, err := db.Exec(`CREATE INDEX d_gin ON d USING gin(tsv)`); err != nil {
		t.Fatal(err)
	}
	build := time.Since(start)
	_, _ = db.Exec(`ANALYZE d`)
	var size int64
	_ = db.QueryRow(`SELECT pg_total_relation_size('d')`).Scan(&size)
	var idx int64
	_ = db.QueryRow(`SELECT pg_relation_size('d_gin')`).Scan(&idx)
	run := func(q query) error {
		rows, err := db.QueryContext(ctx, `SELECT id FROM d, to_tsquery('simple', $1) q WHERE tsv @@ q ORDER BY ts_rank(tsv, q) DESC LIMIT 50`, strings.Join(q.terms, " | "))
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
		}
		return rows.Err()
	}
	l4, err := measure(q4, run)
	if err != nil {
		t.Fatal(err)
	}
	l12, err := measure(q12, run)
	if err != nil {
		t.Fatal(err)
	}
	insStart := time.Now()
	for _, d := range extra {
		if err := add(d); err != nil {
			t.Fatal(err)
		}
	}
	ins := time.Since(insStart) / time.Duration(len(extra))
	report(t, t.Name(), build, size, fmt.Sprintf("gin=%s (test cluster has fsync off; ts_rank is not BM25) %s", mib(idx), rawp(run)), l4, l12, ins)
}

// ---- (c) the term table, extended with frequencies ----

func benchTermTable(t *testing.T, docs, extra []doc, q4, q12 []query) {
	ctx := context.Background()
	db, err := dbx.OpenPostgres(dbxtest.PostgresDSN(t), dbx.PGOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, ddl := range []string{
		`CREATE TABLE docs (id int primary key, len int not null)`,
		`CREATE TABLE terms (term text COLLATE "C" NOT NULL, doc int NOT NULL, tf smallint NOT NULL, PRIMARY KEY (term, doc))`,
	} {
		if _, err := db.Exec(ddl); err != nil {
			t.Fatal(err)
		}
	}
	var totalLen int64
	add := func(ds []doc) error {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		var args []any
		var ph []string
		flush := func() error {
			if len(ph) == 0 {
				return nil
			}
			_, err := tx.ExecContext(ctx, `INSERT INTO terms(term, doc, tf) VALUES `+strings.Join(ph, ","), args...)
			args, ph = args[:0], ph[:0]
			return err
		}
		for _, d := range ds {
			counts := map[string]int{}
			for _, w := range d.body {
				counts[w]++
			}
			for _, w := range d.title {
				counts[w] += 3
			}
			l := len(d.body) + 3*len(d.title)
			totalLen += int64(l)
			if _, err := tx.ExecContext(ctx, `INSERT INTO docs(id, len) VALUES ($1, $2)`, d.id, l); err != nil {
				return err
			}
			for w, c := range counts {
				n := len(args)
				ph = append(ph, fmt.Sprintf("($%d,$%d,$%d)", n+1, n+2, n+3))
				args = append(args, w, d.id, c)
				if len(ph) == 1000 {
					if err := flush(); err != nil {
						return err
					}
				}
			}
		}
		if err := flush(); err != nil {
			return err
		}
		return tx.Commit()
	}
	start := time.Now()
	for i := 0; i < len(docs); i += 200 {
		if err := add(docs[i:min(i+200, len(docs))]); err != nil {
			t.Fatal(err)
		}
	}
	build := time.Since(start)
	_, _ = db.Exec(`ANALYZE terms; ANALYZE docs`)
	var size int64
	_ = db.QueryRow(`SELECT pg_total_relation_size('terms') + pg_total_relation_size('docs')`).Scan(&size)
	avg := float64(totalLen) / float64(len(docs))
	N := float64(len(docs))
	run := func(q query) error {
		rows, err := db.QueryContext(ctx, `WITH df AS (SELECT term, count(*) AS n FROM terms WHERE term = ANY($1) GROUP BY term)
			SELECT t.doc, sum(ln(1 + ($2 - df.n + .5) / (df.n + .5)) * t.tf * 2.2 / (t.tf + 1.2 * (0.25 + 0.75 * d.len / $3))) AS s
			FROM terms t JOIN df USING (term) JOIN docs d ON d.id = t.doc WHERE t.term = ANY($1) GROUP BY t.doc ORDER BY s DESC LIMIT 50`,
			"{"+strings.Join(q.terms, ",")+"}", N, avg)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
		}
		return rows.Err()
	}
	l4, err := measure(q4, run)
	if err != nil {
		t.Fatal(err)
	}
	l12, err := measure(q12, run)
	if err != nil {
		t.Fatal(err)
	}
	insStart := time.Now()
	for _, d := range extra {
		if err := add([]doc{d}); err != nil {
			t.Fatal(err)
		}
	}
	ins := time.Since(insStart) / time.Duration(len(extra))
	report(t, t.Name(), build, size, "(test cluster has fsync off) "+rawp(run), l4, l12, ins)
}

// TestProductionPath measures the code that ships (internal/capturefts) on
// the same corpus: Put per page, then Search with its pruning of common
// terms (the document-frequency cache starts cold, so the first queries pay
// for it), for 4-word and 12-word queries, with and without a window.
func TestProductionPath(t *testing.T) {
	n, _ := strconv.Atoi(os.Getenv("CHOIR_FTS_BENCH_DOCS"))
	if n == 0 {
		t.Skip("CHOIR_FTS_BENCH_DOCS not set")
	}
	docs := corpus(n, 1)
	q4, q12 := queries(docs, 200, 4, 7, true), queries(docs, 200, 12, 8, true)
	q12raw = queries(docs, 100, 12, 9, false)
	ix, err := capturefts.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer ix.Close()
	// CHOIR_FTS_BENCH_BUDGET scales the scan budget to a larger index: the
	// default 100,000 postings is 8% of 1.25 million passages (250,000
	// pages), so 8% of this corpus's passages models that size.
	if b, _ := strconv.Atoi(os.Getenv("CHOIR_FTS_BENCH_BUDGET")); b > 0 {
		ix.SetScanBudget(b)
	}
	scope := capturefts.Scope{Owner: "o", Computer: "c"}
	ctx := context.Background()
	text := func(d doc) string {
		var b strings.Builder
		for i, w := range d.body {
			b.WriteString(w)
			switch {
			case i%60 == 59:
				b.WriteString(".\n\n")
			case i%12 == 11:
				b.WriteString(". ")
			default:
				b.WriteByte(' ')
			}
		}
		return b.String()
	}
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	start := time.Now()
	var textBytes int
	for _, d := range docs {
		tx := text(d)
		textBytes += len(tx)
		if _, err := ix.Put(ctx, scope, capturefts.Page{CaptureID: fmt.Sprint("c", d.id), Title: strings.Join(d.title, " "), TextHash: fmt.Sprint("h", d.id),
			Text: tx, FetchedAt: base.Add(time.Duration(d.id) * time.Minute)}); err != nil {
			t.Fatal(err)
		}
	}
	build := time.Since(start)
	st, _ := ix.Stats(ctx, scope)
	var size int64
	entries, _ := os.ReadDir(ix.Dir())
	for _, e := range entries {
		if info, err := e.Info(); err == nil {
			size += info.Size()
		}
	}
	run := func(q query) error {
		_, err := ix.Search(ctx, scope, capturefts.Query{Text: strings.Join(q.terms, " "), Limit: 25})
		return err
	}
	cold4, _ := measure(q4[:50], run) // df cache filling
	l4, err := measure(q4, run)
	if err != nil {
		t.Fatal(err)
	}
	l12, _ := measure(q12, run)
	l12raw, _ := measure(q12raw, run)
	win := func(q query) error {
		_, err := ix.Search(ctx, scope, capturefts.Query{Text: strings.Join(q.terms, " "), Limit: 25, Since: base.Add(time.Duration(len(docs)*9/10) * time.Minute)})
		return err
	}
	lw, _ := measure(q12, win)
	rec := func(q query) error {
		_, err := ix.Search(ctx, scope, capturefts.Query{Text: strings.Join(q.terms, " "), Limit: 25, Order: capturefts.ByRecency})
		return err
	}
	lr, _ := measure(q12, rec)
	t.Logf("PRODUCTION budget=%s pages=%d passages=%d text=%s put=%s/page total=%s size=%s | q4 cold50 p50=%s | q4 p50=%s p95=%s | q12 p50=%s p95=%s | q12raw p50=%s p95=%s | q12 window10%% p50=%s p95=%s | q12 recent p50=%s p95=%s | %s",
		os.Getenv("CHOIR_FTS_BENCH_BUDGET"), st.Pages, st.Passages, mib(int64(textBytes)), (build / time.Duration(n)).Round(10*time.Microsecond), build.Round(time.Second), mib(size), cold4.pct(.5).Round(100*time.Microsecond),
		l4.pct(.5).Round(100*time.Microsecond), l4.pct(.95).Round(100*time.Microsecond), l12.pct(.5).Round(100*time.Microsecond), l12.pct(.95).Round(100*time.Microsecond),
		l12raw.pct(.5).Round(100*time.Microsecond), l12raw.pct(.95).Round(100*time.Microsecond), lw.pct(.5).Round(100*time.Microsecond), lw.pct(.95).Round(100*time.Microsecond),
		lr.pct(.5).Round(100*time.Microsecond), lr.pct(.95).Round(100*time.Microsecond), rss())
}
