// dolt-dump-split routes a `dolt dump` SQL stream into Store A (canonical
// event/control) and Store B (world-wire/corpus) files for the platform-dolt
// authority split (docs/designs/platform-dolt-storage-normalization-2026-09-18.md).
//
// Usage:
//
//	dolt dump -fn - | go run scripts/dolt-dump-split.go store-a.sql store-b.sql
//
// Statement model: dolt dump emits one SQL statement per line, each line
// ending in ';' (string literals escape newlines/semicolons; verified against
// Dolt 2.1.9). The splitter routes each statement by its target table.
// CREATE DATABASE/USE are dropped (the importer selects the database).
// SET lines go to both outputs. Any statement whose table is not in the
// known A or B set aborts with a nonzero exit — a missed table is a split
// bug, not a routing decision.
package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// storeBTables is the world-wire/corpus authority: og_* objectgraph,
// storeBTables is the world-wire/corpus authority: og_* objectgraph,
// platform texture mirrors, the publication/provenance/artifact domain, and
// the sourcecycled ingestion tables.
var storeBTables = map[string]bool{
	"artifact_blobs":                true,
	"artifact_manifests":            true,
	"citation_edges":                true,
	"consent_records":               true,
	"cycle_events":                  true,
	"cycles":                        true,
	"fetches":                       true,
	"ingestion_events":              true,
	"issues":                        true,
	"items":                         true,
	"og_edges":                      true,
	"og_objects":                    true,
	"platform_subjects":             true,
	"platform_texture_documents":    true,
	"platform_texture_revisions":    true,
	"platform_vtext_documents":      true,
	"platform_vtext_revisions":      true,
	"processor_requests":            true,
	"proposal_delivery_records":     true,
	"provenance_activities":         true,
	"provenance_agents":             true,
	"provenance_edges":              true,
	"provenance_entities":           true,
	"public_routes":                 true,
	"publication_policies":          true,
	"publication_proposals":         true,
	"publication_source_entities":   true,
	"publication_transclusions":     true,
	"publication_version_proposals": true,
	"publication_versions":          true,
	"publications":                  true,
	"reconciler_requests":           true,
	"retrieval_manifests":           true,
	"retrieval_sources":             true,
	"retrieval_spans":               true,
	"review_records":                true,
	"rollback_refs":                 true,
	"sources":                       true,
	"verifier_attestations":         true,
}

// storeATables is the canonical event/control authority: computer-scoped
// event log, checkpoints, lifecycle, key escrow, route projection, and the
// vmctl computer_version route ledger. Enumerated explicitly so a table
// missing from both sets aborts the split instead of silently misrouting.
var storeATables = map[string]bool{
	"computer_checkpoints":                          true,
	"computer_event_append_receipts":                true,
	"computer_event_heads":                          true,
	"computer_file_roots":                           true,
	"computer_key_escrow_transparency":              true,
	"computer_key_escrows":                          true,
	"computer_key_unwrap_approvals":                 true,
	"computer_key_unwrap_requests":                  true,
	"computer_lifecycle_operations":                 true,
	"computer_lifecycle_receipts":                   true,
	"computer_replay_watermarks":                    true,
	"computer_route_projection_certificates":        true,
	"computer_self_development_modes":               true,
	"computer_version_artifact_programs":            true,
	"computer_version_code_closures":                true,
	"computer_version_route_authority_config":       true,
	"computer_version_route_authority_modes":        true,
	"computer_version_route_authorization_evidence": true,
	"computer_version_route_slots":                  true,
	"computer_version_route_transition_receipts":    true,
	"control_key_history":                           true,
}

var (
	tableRe = regexp.MustCompile("^(?:DROP TABLE IF EXISTS|CREATE TABLE|INSERT INTO|ALTER TABLE|TRUNCATE TABLE|LOCK TABLES|UNLOCK TABLES|CREATE INDEX [a-zA-Z0-9_]+ ON|CREATE UNIQUE INDEX [a-zA-Z0-9_]+ ON)\\s+`?([a-zA-Z0-9_]+)`?")
	skipRe  = regexp.MustCompile(`^(CREATE DATABASE|USE\s|DROP DATABASE)`)
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "usage: dolt-dump-split <store-a.sql> <store-b.sql> < dump.sql\n")
		os.Exit(2)
	}
	aFile, err := os.Create(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "open %s: %v\n", os.Args[1], err)
		os.Exit(1)
	}
	defer aFile.Close()
	bFile, err := os.Create(os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "open %s: %v\n", os.Args[2], err)
		os.Exit(1)
	}
	defer bFile.Close()
	a := bufio.NewWriterSize(aFile, 1<<20)
	b := bufio.NewWriterSize(bFile, 1<<20)
	defer a.Flush()
	defer b.Flush()

	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 1<<20), 64<<20) // batched INSERT lines can be large

	var counts [2]int // [0]=A, [1]=B
	var skipped, passthrough int
	lineNo := 0
	var stmt strings.Builder
	flush := func() error {
		trimmed := strings.TrimSpace(stmt.String())
		stmt.Reset()
		if trimmed == "" {
			return nil
		}
		if strings.HasPrefix(trimmed, "--") || strings.HasPrefix(trimmed, "/*") {
			return nil
		}
		if skipRe.MatchString(trimmed) {
			skipped++
			return nil
		}
		if strings.HasPrefix(trimmed, "SET ") {
			fmt.Fprintln(a, trimmed)
			fmt.Fprintln(b, trimmed)
			passthrough++
			return nil
		}
		m := tableRe.FindStringSubmatch(trimmed)
		if m == nil {
			return fmt.Errorf("unrecognized statement: %.160s", trimmed)
		}
		table := m[1]
		if storeBTables[table] {
			fmt.Fprintln(b, trimmed)
			counts[1]++
		} else if storeATables[table] {
			fmt.Fprintln(a, trimmed)
			counts[0]++
		} else {
			return fmt.Errorf("table %q in neither store set: %.120s", table, trimmed)
		}
		return nil
	}
	for sc.Scan() {
		lineNo++
		line := sc.Text()
		// Split on ';' outside single-quoted strings. dolt dump escapes
		// quotes as \' and '' inside literals; CREATE DATABASE/USE share
		// one line, so each statement is classified independently.
		inStr := false
		start := 0
		for i := 0; i < len(line); i++ {
			c := line[i]
			if inStr {
				if c == '\\' {
					i++ // skip escaped char
					continue
				}
				if c == '\'' {
					if i+1 < len(line) && line[i+1] == '\'' {
						i++ // '' escape
						continue
					}
					inStr = false
				}
				continue
			}
			if c == '\'' {
				inStr = true
				continue
			}
			if c == ';' {
				stmt.WriteString(line[start : i+1])
				start = i + 1
				if err := flush(); err != nil {
					fmt.Fprintf(os.Stderr, "line %d: %v\n", lineNo, err)
					os.Exit(1)
				}
			}
		}
		if start < len(line) {
			stmt.WriteString(line[start:])
		}
		stmt.WriteString("\n")
	}
	if err := sc.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "read: %v\n", err)
		os.Exit(1)
	}
	if err := flush(); err != nil {
		fmt.Fprintf(os.Stderr, "trailing: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "store-a statements: %d\nstore-b statements: %d\nset passthrough: %d\nskipped (create database/use): %d\n", counts[0], counts[1], passthrough, skipped)
}
