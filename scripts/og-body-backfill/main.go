// og-body-backfill externalizes existing og_objects bodies into the
// platform-artifacts filesystem CAS (Move 3, docs/designs/
// platform-dolt-storage-normalization-2026-09-18.md). Safe to run against a
// live store: the UPDATE is guarded on body_ref=” AND content_hash=<seen>,
// so a concurrent writer that changed the row wins and the row is skipped.
//
// Usage (on Node B, after the authority split):
//
//	go run og-body-backfill.go \
//	  -dsn 'root@tcp(127.0.0.1:13307)/corpus?parseTime=true&multiStatements=true' \
//	  -root /var/lib/go-choir/platform-artifacts -batch 500
//
// Idempotent: rows already externalized (body_ref != ”) are skipped.
package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

const minBytes = 1024 // must match ogBodyExternalizeMinBytes

func main() {
	dsn := flag.String("dsn", "", "corpus DSN (required)")
	root := flag.String("root", "/var/lib/go-choir/platform-artifacts", "CAS root")
	batch := flag.Int("batch", 500, "rows per SELECT batch")
	flag.Parse()
	if *dsn == "" {
		fmt.Fprintln(os.Stderr, "-dsn required")
		os.Exit(2)
	}
	db, err := sql.Open("mysql", *dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()
	db.SetMaxOpenConns(2)

	var total, done, skipped, failed int
	lastID := ""
	for {
		rows, err := db.Query(`SELECT canonical_id, content_hash, body FROM og_objects
			WHERE body_ref = '' AND body IS NOT NULL AND LENGTH(body) >= ? AND canonical_id > ?
			ORDER BY canonical_id LIMIT ?`, minBytes, lastID, *batch)
		if err != nil {
			fmt.Fprintf(os.Stderr, "select: %v\n", err)
			os.Exit(1)
		}
		type row struct {
			id, hash string
			body     []byte
		}
		var batchRows []row
		for rows.Next() {
			var r row
			if err := rows.Scan(&r.id, &r.hash, &r.body); err != nil {
				fmt.Fprintf(os.Stderr, "scan: %v\n", err)
				os.Exit(1)
			}
			batchRows = append(batchRows, r)
		}
		rows.Close()
		if len(batchRows) == 0 {
			break
		}
		for _, r := range batchRows {
			lastID = r.id
			total++
			sum := sha256.Sum256(r.body)
			ref := filepath.Join("sha256", "og", hex.EncodeToString(sum[:])+".bin")
			path := filepath.Join(*root, ref)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
					fmt.Fprintf(os.Stderr, "mkdir %s: %v\n", filepath.Dir(path), err)
					failed++
					continue
				}
				tmp := path + ".tmp-bf"
				if err := os.WriteFile(tmp, r.body, 0o640); err != nil {
					fmt.Fprintf(os.Stderr, "write %s: %v\n", path, err)
					failed++
					continue
				}
				if err := os.Rename(tmp, path); err != nil {
					_ = os.Remove(tmp)
					fmt.Fprintf(os.Stderr, "rename %s: %v\n", path, err)
					failed++
					continue
				}
			}
			res, err := db.Exec(`UPDATE og_objects SET body = NULL, body_ref = ?, body_size = ?
				WHERE canonical_id = ? AND body_ref = '' AND content_hash = ?`,
				ref, len(r.body), r.id, r.hash)
			if err != nil {
				fmt.Fprintf(os.Stderr, "update %s: %v\n", r.id, err)
				failed++
				continue
			}
			if n, _ := res.RowsAffected(); n == 0 {
				skipped++ // row changed under us; writer wins
				continue
			}
			done++
		}
		if total%5000 < *batch {
			fmt.Printf("progress: scanned=%d externalized=%d skipped=%d failed=%d\n", total, done, skipped, failed)
		}
	}
	fmt.Printf("done: scanned=%d externalized=%d skipped=%d failed=%d at %s\n",
		total, done, skipped, failed, time.Now().UTC().Format(time.RFC3339))
	if failed > 0 {
		os.Exit(1)
	}
}
