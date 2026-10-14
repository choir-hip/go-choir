package main

// `ograph-bench platform` measures Store B retrieval through the shipped
// platform code (internal/platform) before and after the retrieval
// migration, on a LOCAL dolt sql-server: origin/main's query text on
// origin/main's indexes first, then the migration (with a concurrent
// capture writer, to see whether it blocks writes), then the new shapes.
// Problem record: docs/problems/corpus-retrieval-windowed-unindexed-untimed-2026-10-14.md.
//
//	go run ./scripts/ograph-bench platform -dsn 'root@tcp(127.0.0.1:3399)/platform_bench' -fresh -captures 200000

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/platform"
)

type benchRow struct {
	Phase string  `json:"phase"`
	Name  string  `json:"name"`
	P50   float64 `json:"p50_ms"`
	P95   float64 `json:"p95_ms"`
	Rows  int     `json:"rows"`
	Note  string  `json:"note,omitempty"`
}

func platformBench(args []string) {
	fs := flag.NewFlagSet("platform", flag.ExitOnError)
	dsn := fs.String("dsn", "", "LOCAL dolt sql-server DSN with a database name containing \"bench\" (loopback only)")
	fresh := fs.Bool("fresh", false, "drop and recreate the bench database first")
	captures := fs.Int("captures", 200000, "captures to seed")
	computers := fs.Int("computers", 3, "computers of the one owner the captures spread over")
	days := fs.Int("days", 120, "days of history the captures spread over")
	iters := fs.Int("iters", 15, "timed runs per query")
	seed := fs.Int64("seed", 20261014, "generator seed")
	out := fs.String("out", "", "write the result rows as JSON here")
	must(fs.Parse(args))

	cfg, err := mysql.ParseDSN(*dsn)
	must(err)
	host := cfg.Addr
	if h, _, ok := strings.Cut(cfg.Addr, ":"); ok {
		host = h
	}
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		must(fmt.Errorf("refusing non-loopback DSN host %q: this harness runs against a local dolt only", host))
	}
	if !strings.Contains(cfg.DBName, "bench") {
		must(fmt.Errorf("refusing database %q: the bench database name must contain \"bench\"", cfg.DBName))
	}
	if *fresh {
		root := cfg.Clone()
		root.DBName = ""
		rdb, err := sql.Open("mysql", root.FormatDSN())
		must(err)
		_, err = rdb.Exec("DROP DATABASE IF EXISTS `" + cfg.DBName + "`")
		must(err)
		_, err = rdb.Exec("CREATE DATABASE `" + cfg.DBName + "`")
		must(err)
		_ = rdb.Close()
	}
	store, err := platform.OpenStore(*dsn, "")
	must(err)
	defer store.Close()
	og := platform.NewObjectGraphStore(store)
	ctx := context.Background()
	rng := rand.New(rand.NewSource(*seed))
	var rows []benchRow
	report := func(r benchRow) {
		rows = append(rows, r)
		fmt.Printf("%-8s %-58s p50 %9.2f  p95 %9.2f  rows %6d  %s\n", r.Phase, r.Name, r.P50, r.P95, r.Rows, r.Note)
	}

	// ---- seed (no retrieval indexes, no projection: origin/main's shape)
	owner := "bench-owner"
	now := time.Now().UTC().Truncate(time.Second)
	span := time.Duration(*days) * 24 * time.Hour
	vocab := benchVocab(rng)
	var urls []string
	start := time.Now()
	batch := make([]objectgraph.Object, 0, 1000)
	seedDB, err := sql.Open("mysql", *dsn)
	must(err)
	defer seedDB.Close()
	// Seeding is not what is measured: multi-row inserts straight into
	// og_objects (the rows PutBatch would write; no projection or key rows
	// exist yet, as on a Store B before the migration).
	flush := func() {
		if len(batch) == 0 {
			return
		}
		args := make([]any, 0, 9*len(batch))
		for _, o := range batch {
			args = append(args, o.CanonicalID, string(o.ObjectKind), o.OwnerID, o.ComputerID, o.ContentHash, string(o.Metadata), o.CreatedAt, o.UpdatedAt, o.Tombstone)
		}
		values := strings.TrimSuffix(strings.Repeat("(?, ?, ?, ?, '', ?, NULL, '', 0, ?, ?, ?, ?, ''), ", len(batch)), ", ")
		_, err := seedDB.ExecContext(ctx, `INSERT INTO og_objects (canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, body_ref, body_size, metadata, created_at, updated_at, tombstone, superseded_by) VALUES `+values, args...)
		must(err)
		batch = batch[:0]
	}
	for i := 0; i < *captures; i++ {
		at := now.Add(-time.Duration(rng.Int63n(int64(span))))
		obj, u := benchCapture(rng, vocab, owner, fmt.Sprintf("computer-%d", i%*computers), i, at)
		if i%997 == 0 {
			urls = append(urls, u)
		}
		batch = append(batch, obj)
		if len(batch) == cap(batch) {
			flush()
		}
	}
	flush()
	pubs := *captures / 100
	for i := 0; i < pubs; i++ {
		at := now.Add(-time.Duration(rng.Int63n(int64(span))))
		batch = append(batch,
			benchObject(owner, "choir.publication", fmt.Sprintf("pub-%d", i), at, map[string]any{"publication_id": fmt.Sprintf("pub-%d", i), "state": "published"}),
			benchObject(owner, "choir.public_route", fmt.Sprintf("route-%d", i), at, map[string]any{"route_path": fmt.Sprintf("/pub/texture/story-%d", i), "publication_id": fmt.Sprintf("pub-%d", i)}))
		for c := 0; c < 5; c++ {
			batch = append(batch, benchObject(owner, "choir.publication_check", fmt.Sprintf("check-%d-%d", i, c), at, map[string]any{"publication_version_id": fmt.Sprintf("pub-%d-v1", i), "check": c}))
		}
		if len(batch) >= 1000 {
			flush()
		}
	}
	flush()
	fmt.Printf("seeded %d captures, %d publications (+routes, 5 checks each) in %s\n", *captures, pubs, time.Since(start).Round(time.Second))

	db, err := sql.Open("mysql", *dsn)
	must(err)
	db.SetMaxOpenConns(2)
	defer db.Close()
	words := benchQueries()
	computer := "computer-1"
	routeKey := fmt.Sprintf("/pub/texture/story-%d", pubs/2)
	checkKey := fmt.Sprintf("pub-%d-v1", pubs/3)

	// ---- before: origin/main's query text on origin/main's indexes
	const oldWindow = `SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, body_ref, body_size, metadata, created_at, updated_at, tombstone, superseded_by FROM og_objects WHERE 1=1 AND object_kind = ? AND owner_id = ? AND computer_id = ? ORDER BY updated_at DESC LIMIT ?`
	const oldJSON = `SELECT canonical_id FROM og_objects WHERE object_kind = ? AND JSON_UNQUOTE(JSON_EXTRACT(CAST(metadata AS JSON), ?)) = ? ORDER BY updated_at DESC LIMIT ?`
	windowSearch := func(terms []string) (int, error) {
		r, err := db.QueryContext(ctx, oldWindow, string(objectgraph.WebCaptureObjectKind), owner, computer, 1000)
		if err != nil {
			return 0, err
		}
		defer r.Close()
		n := 0
		for r.Next() {
			var id, kind, own, comp, ver, hash, ref, meta string
			var body []byte
			var size int64
			var created, updated time.Time
			var tomb bool
			var sup string
			if err := r.Scan(&id, &kind, &own, &comp, &ver, &hash, &body, &ref, &size, &meta, &created, &updated, &tomb, &sup); err != nil {
				return n, err
			}
			var m objectgraph.WebCaptureMetadata
			_ = json.Unmarshal([]byte(meta), &m)
			lower := strings.ToLower(m.Title)
			for _, t := range terms {
				if strings.Contains(lower, t) {
					n++
					break
				}
			}
		}
		return n, r.Err()
	}
	report(timeIt("before", "corpus window: newest 1000 of a computer (+ Go match)", *iters, func() (int, error) { return windowSearch(words[0]) }))
	report(timeIt("before", "URL citation lookup: newest 1000 + Go equality", *iters, func() (int, error) {
		_, err := windowSearch([]string{urls[len(urls)/2]})
		return 0, err
	}))
	report(timeIt("before", "public_route by route_path (JSON scan)", *iters, func() (int, error) {
		return countRows(db.QueryContext(ctx, oldJSON, "choir.public_route", "$.route_path", routeKey, 1))
	}))
	report(timeIt("before", "publication_check by version (JSON scan)", *iters, func() (int, error) {
		return countRows(db.QueryContext(ctx, oldJSON, "choir.publication_check", "$.publication_version_id", checkKey, 50))
	}))
	writeCost := func(phase string, n int) {
		var lat []float64
		for i := 0; i < n; i++ {
			obj, _ := benchCapture(rng, vocab, owner, computer, *captures+1000000*len(phase)+i+len(rows)*1000, time.Now().UTC())
			t0 := time.Now()
			_, err := og.InsertObjectIfAbsent(ctx, obj)
			must(err)
			lat = append(lat, float64(time.Since(t0).Microseconds())/1000)
		}
		p50, p95 := pcts(lat)
		report(benchRow{Phase: phase, Name: "capture write (InsertObjectIfAbsent)", P50: p50, P95: p95, Rows: n})
	}
	writeCost("before", 100)

	// ---- migration, with a concurrent capture writer
	var wrote, writeErrs atomic.Int64
	var writeLat []float64
	var latMu sync.Mutex
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		wrng := rand.New(rand.NewSource(*seed + 1))
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			case <-time.After(20 * time.Millisecond):
			}
			obj, _ := benchCapture(wrng, vocab, owner, computer, 5000000+i, time.Now().UTC())
			t0 := time.Now()
			if _, err := og.InsertObjectIfAbsent(ctx, obj); err != nil {
				writeErrs.Add(1)
				fmt.Printf("  concurrent write error: %v\n", err)
			} else {
				wrote.Add(1)
			}
			latMu.Lock()
			writeLat = append(writeLat, float64(time.Since(t0).Microseconds())/1000)
			latMu.Unlock()
		}
	}()
	t0 := time.Now()
	migrateErr := store.MigrateRetrieval(ctx)
	migrateTook := time.Since(t0)
	close(stop)
	wg.Wait()
	p50, p95 := pcts(writeLat)
	report(benchRow{Phase: "migrate", Name: "MigrateRetrieval (all steps, incl. capture backfill)", P50: float64(migrateTook.Milliseconds()), P95: float64(migrateTook.Milliseconds()), Rows: *captures,
		Note: fmt.Sprintf("err=%v live=%v", migrateErr, store.RetrievalLive())})
	report(benchRow{Phase: "migrate", Name: "concurrent capture writes during the migration", P50: p50, P95: p95, Rows: int(wrote.Load()),
		Note: fmt.Sprintf("errors=%d max=%.0fms", writeErrs.Load(), maxOf(writeLat))})
	must(migrateErr)

	// ---- after: the shipped shapes
	report(timeIt("after", "corpus window: ListObjects newest 1000 (time ladder)", *iters, func() (int, error) {
		objs, err := og.ListObjects(ctx, objectgraph.ListFilter{Kind: objectgraph.WebCaptureObjectKind, OwnerID: owner, ComputerID: computer, Limit: 1000})
		return len(objs), err
	}))
	report(timeIt("after", "origin/main window text on the new composite index", *iters, func() (int, error) { return windowSearch(words[0]) }))
	for _, q := range words {
		q := q
		report(timeIt("after", fmt.Sprintf("SearchCaptures %v (all history)", q), *iters, func() (int, error) {
			resp, err := store.SearchCaptures(ctx, platform.CaptureSearchRequest{OwnerID: owner, ComputerID: computer, Words: q, Need: need(len(q)), Candidates: 500})
			return len(resp.Objects), err
		}))
	}
	report(timeIt("after", "CaptureByURL (all history)", *iters, func() (int, error) {
		res, err := store.CaptureByURL(ctx, owner, computer, urls[len(urls)/2], true)
		if res.Found {
			return 1, err
		}
		return 0, err
	}))
	report(timeIt("after", "public_route by route_path (lookup key)", *iters, func() (int, error) {
		_, err := og.GetObjectByMetadata(ctx, "choir.public_route", "$.route_path", routeKey)
		if err != nil {
			return 0, err
		}
		return 1, nil
	}))
	report(timeIt("after", "publication_check by version (lookup key)", *iters, func() (int, error) {
		objs, err := og.ListObjectsByMetadata(ctx, "choir.publication_check", "$.publication_version_id", checkKey, 50)
		return len(objs), err
	}))
	writeCost("after", 100)

	if *out != "" {
		raw, _ := json.MarshalIndent(map[string]any{"captures": *captures, "computers": *computers, "days": *days, "iters": *iters, "rows": rows}, "", "  ")
		must(os.WriteFile(*out, raw, 0o644))
	}
}

func need(n int) int {
	if n <= 2 {
		return n
	}
	return (n + 1) / 2
}

func countRows(r *sql.Rows, err error) (int, error) {
	if err != nil {
		return 0, err
	}
	defer r.Close()
	n := 0
	for r.Next() {
		n++
	}
	return n, r.Err()
}

func timeIt(phase, name string, iters int, f func() (int, error)) benchRow {
	var lat []float64
	n := 0
	// One warm-up run, as the research note's numbers are warm.
	if _, err := f(); err != nil {
		return benchRow{Phase: phase, Name: name, Note: "error: " + err.Error()}
	}
	for i := 0; i < iters; i++ {
		t0 := time.Now()
		got, err := f()
		if err != nil {
			return benchRow{Phase: phase, Name: name, Note: "error: " + err.Error()}
		}
		lat = append(lat, float64(time.Since(t0).Microseconds())/1000)
		n = got
	}
	p50, p95 := pcts(lat)
	return benchRow{Phase: phase, Name: name, P50: p50, P95: p95, Rows: n}
}

func pcts(lat []float64) (float64, float64) {
	if len(lat) == 0 {
		return 0, 0
	}
	s := append([]float64(nil), lat...)
	sort.Float64s(s)
	at := func(q float64) float64 {
		i := int(q*float64(len(s)-1) + 0.5)
		return s[i]
	}
	return at(0.5), at(0.95)
}

func maxOf(lat []float64) float64 {
	m := 0.0
	for _, v := range lat {
		if v > m {
			m = v
		}
	}
	return m
}

// benchVocab is a Zipf-ish title vocabulary with the shapes real queries
// carry: two-letter tokens, proper names, numbers.
func benchVocab(rng *rand.Rand) []string {
	base := []string{"ai", "eu", "us", "uk", "5g", "model", "release", "harbor", "storm", "election", "ukraine", "russia", "china", "market",
		"bank", "rates", "talks", "ceasefire", "climate", "energy", "court", "ruling", "minister", "president", "vote", "budget", "chips",
		"nvidia", "openai", "regulators", "act", "trade", "tariffs", "oil", "gas", "war", "peace", "health", "covid", "vaccine"}
	letters := "abcdefghijklmnopqrstuvwxyz"
	for len(base) < 20000 {
		n := 4 + rng.Intn(6)
		var b strings.Builder
		for i := 0; i < n; i++ {
			b.WriteByte(letters[rng.Intn(len(letters))])
		}
		base = append(base, b.String())
	}
	return base
}

var benchCJK = []string{"中国发布新的人工智能模型", "日本の選挙結果", "人工智能监管新规", "台风登陆广东"}
var benchArabic = []string{"والمصريون يحتفلون بالعيد", "الحكومة تعلن الميزانية"}

func benchCapture(rng *rand.Rand, vocab []string, owner, computer string, i int, at time.Time) (objectgraph.Object, string) {
	var title string
	switch r := rng.Intn(100); {
	case r < 3:
		title = benchCJK[rng.Intn(len(benchCJK))]
	case r < 5:
		title = benchArabic[rng.Intn(len(benchArabic))]
	default:
		n := 6 + rng.Intn(8)
		words := make([]string, n)
		for j := range words {
			// Zipf-ish: square the uniform draw toward the head.
			f := rng.Float64()
			words[j] = vocab[int(f*f*f*float64(len(vocab)-1))]
		}
		title = strings.Join(words, " ")
	}
	u := fmt.Sprintf("https://host%d.example/news/%d", rng.Intn(500), i)
	meta, _ := json.Marshal(objectgraph.WebCaptureMetadata{SchemaVersion: objectgraph.WebCaptureSchemaVersion, URL: u, CanonicalURL: u, Title: title,
		FetchedAt: at.Format(time.RFC3339Nano), ContentBlobID: "sha256:r", ExtractedTextBlobID: "sha256:t", BlocksBlobID: "sha256:b", Extractor: "x1",
		DocHash: fmt.Sprintf("h%d", i), SourceID: fmt.Sprintf("rss:source-%d", rng.Intn(50)), Language: "en", BlockCount: 8})
	id, err := objectgraph.BuildCanonicalID(objectgraph.WebCaptureObjectKind, owner, objectgraph.StableSuffixFromKey(fmt.Sprintf("bench:%d", i)))
	must(err)
	return objectgraph.Object{CanonicalID: id, ObjectKind: objectgraph.WebCaptureObjectKind, OwnerID: owner, ComputerID: computer,
		ContentHash: fmt.Sprintf("h%d", i), Metadata: meta, CreatedAt: at, UpdatedAt: at}, u
}

func benchObject(owner, kind, key string, at time.Time, metadata map[string]any) objectgraph.Object {
	raw, _ := json.Marshal(metadata)
	id, err := objectgraph.BuildCanonicalID(objectgraph.ObjectKind(kind), owner, objectgraph.StableSuffixFromKey(key))
	must(err)
	return objectgraph.Object{CanonicalID: id, ObjectKind: objectgraph.ObjectKind(kind), OwnerID: owner, ContentHash: "h-" + key, Metadata: raw, CreatedAt: at, UpdatedAt: at}
}

// benchQueries are corpus-search word lists of the real query shapes: a
// two-letter token, a common word, a rare word, a multi-word query, CJK.
func benchQueries() [][]string {
	return [][]string{{"ai"}, {"election"}, {"eu", "ai", "act"}, {"harbor", "storm"}, {"人工智能"}, {"nvidia", "chips", "tariffs", "china"}}
}
