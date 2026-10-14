package main

import (
	"database/sql"
	"flag"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/go-sql-driver/mysql"
)

// postings builds our own inverted index table (term -> rows) from a text
// column through ordinary transactional INSERTs, timing the write rate, then
// times word and phrase lookups against it. This is the "inverted index as a
// table" alternative to Dolt FULLTEXT: any token length, CJK bigrams, case
// folding in Go, and a time-ordered posting key.
//
//	ograph-bench postings -dsn 'root@tcp(127.0.0.1:3399)/db' -source blocks|titles -limit N -params P
func postings(args []string) {
	fs := flag.NewFlagSet("postings", flag.ExitOnError)
	dsn := fs.String("dsn", "", "LOCAL dolt sql-server DSN (loopback only)")
	source := fs.String("source", "blocks", "blocks (capture_blocks.text) | titles (capture_index.title)")
	limit := fs.Int("limit", 36000, "rows of source text to index")
	batch := fs.Int("batch", 2000, "posting rows per INSERT")
	paramsPath := fs.String("params", "", "params.json (for query terms)")
	must(fs.Parse(args))
	cfg, err := mysql.ParseDSN(*dsn)
	must(err)
	host, _, _ := strings.Cut(cfg.Addr, ":")
	if host != "127.0.0.1" && host != "localhost" {
		must(fmt.Errorf("refusing non-loopback DSN host %q", host))
	}
	db, err := sql.Open("mysql", *dsn)
	must(err)
	db.SetMaxOpenConns(1)
	defer db.Close()

	table := "block_postings"
	sel := `SELECT capture_id, block_ord, text FROM capture_blocks LIMIT ?`
	if *source == "titles" {
		table = "title_postings"
		sel = `SELECT canonical_id, 0, title FROM capture_index ORDER BY fetched_at DESC LIMIT ?`
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS ` + table + ` (
		term VARCHAR(64) COLLATE utf8mb4_0900_bin NOT NULL,
		doc_id VARCHAR(255) NOT NULL,
		ord INT NOT NULL,
		first_pos SMALLINT NOT NULL,
		PRIMARY KEY (term, doc_id, ord))`)
	must(err)
	rows, err := db.Query(sel, *limit)
	must(err)
	type doc struct {
		id   string
		ord  int
		text string
	}
	var docs []doc
	for rows.Next() {
		var d doc
		must(rows.Scan(&d.id, &d.ord, &d.text))
		docs = append(docs, d)
	}
	must(rows.Err())
	rows.Close()

	start := time.Now()
	var vals []string
	var vargs []any
	total := 0
	flush := func() {
		if len(vals) == 0 {
			return
		}
		_, err := db.Exec(`INSERT IGNORE INTO `+table+` (term, doc_id, ord, first_pos) VALUES `+strings.Join(vals, ","), vargs...)
		must(err)
		total += len(vals)
		vals, vargs = vals[:0], vargs[:0]
	}
	for _, d := range docs {
		seen := map[string]bool{}
		for pos, term := range tokenize(d.text) {
			if seen[term] || len(term) > 64 {
				continue
			}
			seen[term] = true
			vals = append(vals, "(?,?,?,?)")
			vargs = append(vargs, term, d.id, d.ord, min(pos, math.MaxInt16))
			if len(vals) >= *batch {
				flush()
			}
		}
	}
	flush()
	el := time.Since(start).Seconds()
	fmt.Printf("indexed %d %s rows -> %d postings in %.1fs (%.0f postings/s, %.0f source rows/s)\n", len(docs), *source, total, el, float64(total)/el, float64(len(docs))/el)

	if *paramsPath == "" {
		return
	}
	// Query terms: sample real words and three-word phrases from the indexed text.
	var words, phrases []string
	for i := 0; i < len(docs) && len(phrases) < 30; i += max(1, len(docs)/30) {
		toks := strings.Fields(docs[i].text)
		if len(toks) >= 6 {
			words = append(words, strings.ToLower(toks[3]))
			phrases = append(phrases, strings.Join(toks[2:5], " "))
		}
	}
	timeIt := func(name string, n int, f func(i int) (int, error)) {
		var d []float64
		got := 0
		for i := 0; i < n; i++ {
			t := time.Now()
			r, err := f(i)
			must(err)
			d = append(d, float64(time.Since(t).Microseconds())/1000)
			got += r
		}
		sort.Float64s(d)
		fmt.Printf("%-34s %6d %10.2f %10.2f %10.2f %8.1f\n", name, n, d[len(d)/2], d[int(math.Ceil(0.95*float64(len(d))))-1], d[len(d)-1], float64(got)/float64(n))
	}
	count := func(q string, a ...any) (int, error) {
		rs, err := db.Query(q, a...)
		if err != nil {
			return 0, err
		}
		defer rs.Close()
		n := 0
		for rs.Next() {
			n++
		}
		return n, rs.Err()
	}
	fmt.Printf("%-34s %6s %10s %10s %10s %8s\n", "query", "iters", "p50_ms", "p95_ms", "max_ms", "rows")
	timeIt(table+"_word", 30, func(i int) (int, error) {
		return count(`SELECT doc_id FROM `+table+` WHERE term = ? LIMIT 50`, words[i%len(words)])
	})
	timeIt(table+"_phrase_and_verify", 30, func(i int) (int, error) {
		ph := phrases[i%len(phrases)]
		terms := tokenize(ph)
		ph2 := strings.TrimSuffix(strings.Repeat("?,", len(terms)), ",")
		a := []any{}
		for _, t := range terms {
			a = append(a, t)
		}
		a = append(a, len(dedupe(terms)))
		textCol, base, key := "text", "capture_blocks", "b.capture_id = m.doc_id AND b.block_ord = m.ord"
		if *source == "titles" {
			textCol, base, key = "title_ci", "capture_index", "b.canonical_id = m.doc_id"
		}
		a = append(a, "%"+ph+"%")
		return count(`WITH m AS (SELECT doc_id, ord FROM `+table+` WHERE term IN (`+ph2+`) GROUP BY doc_id, ord HAVING COUNT(*) = ?) SELECT m.doc_id FROM m JOIN `+base+` b ON `+key+` WHERE b.`+textCol+` LIKE ? LIMIT 50`, a...)
	})
}

func dedupe(xs []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range xs {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

// tokenize lowercases, splits on anything that is not a letter or digit,
// keeps tokens of any length (so "ai", "eu", "14" survive), and emits
// overlapping bigrams for runs of Han/Kana/Hangul characters.
func tokenize(s string) []string {
	var out []string
	var cur []rune
	var cjk []rune
	flushWord := func() {
		if len(cur) > 0 {
			out = append(out, string(cur))
			cur = cur[:0]
		}
	}
	flushCJK := func() {
		if len(cjk) == 1 {
			out = append(out, string(cjk))
		}
		for i := 0; i+1 < len(cjk); i++ {
			out = append(out, string(cjk[i:i+2]))
		}
		cjk = cjk[:0]
	}
	for _, r := range s {
		switch {
		case unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul):
			flushWord()
			cjk = append(cjk, r)
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			flushCJK()
			cur = append(cur, unicode.ToLower(r))
		default:
			flushWord()
			flushCJK()
		}
	}
	flushWord()
	flushCJK()
	return out
}
