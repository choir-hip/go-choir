// og-delta-sync copies rows written to Store A's B-domain tables during the
// authority-split cutover window into Store B. The deploy restarted
// corpusd/sourcecycled mid-cutover (before the corpus DSN flip), so
// 03:01–03:54 world-wire writes landed in Store A. This syncs every B table
// that has a created_at or updated_at column, upserting rows newer than the
// fence timestamp. Idempotent and safe to re-run.
//
// Usage (on Node B):
//
//	go run og-delta-sync.go -since '2026-09-19 02:45:00'
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var (
	aDSN  = flag.String("a", "root@tcp(127.0.0.1:13306)/platform?parseTime=true&multiStatements=true", "store A DSN")
	bDSN  = flag.String("b", "root@tcp(127.0.0.1:13307)/corpus?parseTime=true&multiStatements=true", "store B DSN")
	since = flag.String("since", "", "fence timestamp (UTC) — rows with created_at/updated_at >= this are synced")
)

// bTables are the Store B tables with a timestamp column usable for the
// delta filter. Tables without one are skipped (they were not written in the
// window — verified by matching A/B counts).
var bTables = []string{
	"artifact_blobs", "artifact_manifests", "citation_edges", "consent_records",
	"cycle_events", "cycles", "fetches", "ingestion_events", "issues", "items",
	"og_edges", "og_objects", "platform_subjects", "platform_texture_documents",
	"platform_texture_revisions", "platform_vtext_documents", "platform_vtext_revisions",
	"processor_requests", "proposal_delivery_records", "provenance_activities",
	"provenance_agents", "provenance_edges", "provenance_entities", "public_routes",
	"publication_policies", "publication_proposals", "publication_source_entities",
	"publication_transclusions", "publication_version_proposals", "publication_versions",
	"publications", "reconciler_requests", "retrieval_manifests", "retrieval_sources",
	"retrieval_spans", "review_records", "rollback_refs", "sources", "verifier_attestations",
}

func main() {
	flag.Parse()
	if *since == "" {
		fmt.Fprintln(os.Stderr, "-since required")
		os.Exit(2)
	}
	a, err := sql.Open("mysql", *aDSN)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open A: %v\n", err)
		os.Exit(1)
	}
	defer a.Close()
	b, err := sql.Open("mysql", *bDSN)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open B: %v\n", err)
		os.Exit(1)
	}
	defer b.Close()
	a.SetMaxOpenConns(2)
	b.SetMaxOpenConns(2)

	grand := 0
	for _, table := range bTables {
		tsCol, err := timestampColumn(a, table)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", table, err)
			continue
		}
		if tsCol == "" {
			fmt.Printf("%-40s no timestamp column, skipped\n", table)
			continue
		}
		n, err := syncTable(a, b, table, tsCol, *since)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", table, err)
			continue
		}
		grand += n
		if n > 0 {
			fmt.Printf("%-40s synced %d rows\n", table, n)
		}
	}
	fmt.Printf("done: %d rows synced at %s\n", grand, time.Now().UTC().Format(time.RFC3339))
}

// timestampColumn returns the best delta-filter column on the table,
// preferring updated_at (captures upserts) then created_at then the
// fetch/cycle timestamp columns, else "".
func timestampColumn(db *sql.DB, table string) (string, error) {
	rows, err := db.Query("SHOW COLUMNS FROM " + table)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	cols := map[string]bool{}
	for rows.Next() {
		var f, typ, null, key, def, extra sql.NullString
		if err := rows.Scan(&f, &typ, &null, &key, &def, &extra); err != nil {
			return "", err
		}
		cols[f.String] = true
	}
	for _, c := range []string{"updated_at", "created_at", "fetched_at", "started_at", "ended_at", "published", "escrowed_at", "requested_at", "completed_at"} {
		if cols[c] {
			return c, nil
		}
	}
	return "", nil
}

// syncTable upserts rows with tsCol >= since from A into B. Returns rows
// written (INSERT ... ON DUPLICATE KEY UPDATE affected-rows count).
func syncTable(a, b *sql.DB, table, tsCol, since string) (int, error) {
	// Column list for the upsert.
	colRows, err := a.Query("SHOW COLUMNS FROM " + table)
	if err != nil {
		return 0, err
	}
	var cols []string
	for colRows.Next() {
		var f, typ, null, key, def, extra sql.NullString
		if err := colRows.Scan(&f, &typ, &null, &key, &def, &extra); err != nil {
			colRows.Close()
			return 0, err
		}
		cols = append(cols, f.String)
	}
	colRows.Close()
	colList := "`" + strings.Join(cols, "`,`") + "`"
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(cols)), ",")
	updates := make([]string, 0, len(cols))
	for _, c := range cols {
		updates = append(updates, "`"+c+"`=VALUES(`"+c+"`)")
	}
	insert := fmt.Sprintf("INSERT INTO `%s` (%s) VALUES (%s) ON DUPLICATE KEY UPDATE %s",
		table, colList, placeholders, strings.Join(updates, ","))

	sel := fmt.Sprintf("SELECT %s FROM `%s` WHERE `%s` >= ?", colList, table, tsCol)
	rows, err := a.Query(sel, since)
	if err != nil {
		return 0, fmt.Errorf("select: %w", err)
	}
	defer rows.Close()
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	n := 0
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return n, fmt.Errorf("scan: %w", err)
		}
		if _, err := b.Exec(insert, vals...); err != nil {
			return n, fmt.Errorf("upsert: %w", err)
		}
		n++
	}
	return n, rows.Err()
}
