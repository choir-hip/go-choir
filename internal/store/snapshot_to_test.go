package store

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/storeschema"
)

// The capsule's store copy (S3 failure modes 1, 2, 4, 6, 7,
// docs/design/capsule-overlay-computer-state-2026-11-03.md).

func TestSnapshotToIsOneFileWithTheSidecarAndOpens(t *testing.T) {
	s, _ := compactTestStore(t, 5, 20, 200)
	dir := t.TempDir()
	receipt, err := s.SnapshotTo(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Sequence != 5 || receipt.Head != compactTestHead(5) || receipt.ComputerID != compactTestComputer {
		t.Fatalf("receipt head = %+v", receipt)
	}
	entries, _ := os.ReadDir(dir)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	if strings.Join(names, ",") != "state.db,state.texture" {
		t.Fatalf("copy dir holds %v, want state.db and state.texture only", names)
	}
	side, _ := os.ReadDir(filepath.Join(dir, "state.texture"))
	for _, e := range side {
		switch e.Name() {
		case storeschema.FloorFile, storeschema.File, storeschema.LeaseUntrustedFile, vocabReportFileName, depositUpcastLedgerFileName:
		default:
			t.Fatalf("sidecar carried %s", e.Name())
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "state.texture", storeschema.File)); err != nil {
		t.Fatalf("schema receipt not carried: %v", err)
	}
	// The copy opens as a store (the in-capsule autoputer's path).
	copyStore, err := Open(filepath.Join(dir, "state"))
	if err != nil {
		t.Fatalf("open the copy: %v", err)
	}
	var rows int
	if err := copyStore.ReadDB().QueryRow(`SELECT COUNT(*) FROM compact_probe`).Scan(&rows); err != nil || rows != 20 {
		t.Fatalf("copy rows = %d (%v), want 20", rows, err)
	}
	_ = copyStore.Close()
}

func TestSnapshotToLeavesTheLiveStoreUnchangedAndWritable(t *testing.T) {
	s, _ := compactTestStore(t, 3, 5, 0)
	if _, err := s.SnapshotTo(context.Background(), filepath.Dir(s.StatePath())); err == nil {
		t.Fatal("a snapshot into the live store's directory was accepted")
	}
	dir := t.TempDir()
	if _, err := s.SnapshotTo(context.Background(), dir); err != nil {
		t.Fatal(err)
	}
	compactTestAppend(t, s, 4)
	var seq uint64
	if err := s.ReadDB().QueryRow(`SELECT sequence FROM computer_event_projection_heads`).Scan(&seq); err != nil || seq != 4 {
		t.Fatalf("live head after copy = %d (%v), want 4", seq, err)
	}
	db, err := sql.Open("sqlite", dbx.SQLiteDSN(filepath.Join(dir, "state.db"), dbx.SQLiteOptions{}, false))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.QueryRow(`SELECT sequence FROM computer_event_projection_heads`).Scan(&seq); err != nil || seq != 3 {
		t.Fatalf("copy head = %d (%v), want 3: the copy is one instant", seq, err)
	}
}

// A secret-shaped value is blanked in the copy, and its bytes are in no
// file of the copy (secure_delete and the second VACUUM), while the live
// store keeps it.
func TestSnapshotToBlanksSecretShapedValuesAndNoBytesSurvive(t *testing.T) {
	s, _ := compactTestStore(t, 2, 3, 0)
	secret := "ghp_" + strings.Repeat("Z7q", 8)
	if _, err := s.db.Exec(`INSERT INTO compact_probe VALUES (900, ?)`, "note: "+secret+" end"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT INTO compact_probe VALUES (901, ?)`, []byte("blob "+secret)); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	receipt, err := s.SnapshotTo(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Blanked["compact_probe.payload"] != 2 || receipt.ScannedBytes == 0 {
		t.Fatalf("receipt = %+v", receipt)
	}
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.Mode().IsRegular() {
			return err
		}
		raw, _ := os.ReadFile(path)
		if bytes.Contains(raw, []byte(secret)) {
			t.Fatalf("the secret survives in %s", path)
		}
		return nil
	})
	var live string
	if err := s.ReadDB().QueryRow(`SELECT payload FROM compact_probe WHERE id = 900`).Scan(&live); err != nil || !strings.Contains(live, secret) {
		t.Fatalf("the live store was changed: %q %v", live, err)
	}
}

// Every column of the guest store whose name looks like a credential is
// ruled on here, with its reason. A new one fails until it is ruled on
// (S3 failure mode 4). The S0 inventory found none holding a credential.
func TestStoreColumnsWithCredentialShapedNamesAreRuledOn(t *testing.T) {
	s, _ := trimTestOpen(t)
	rulings := map[string]string{
		"store_meta.key":                                              "the store's own metadata key names",
		"run_memory_entries.tokens_before":                            "a count of model tokens",
		"user_preferences.preference_key":                             "a preference's name",
		"desktop_sessions.session_id":                                 "a layout key the client picks (X-Choir-Session), not an authenticator",
		"desktop_app_instances.created_by_session_id":                 "the same layout key",
		"desktop_window_placements.session_id":                        "the same layout key",
		"computer_event_projection_heads.credential_revocation_epoch": "an epoch number",
		"computer_event_index.idempotency_key":                        "a request's idempotency name",
		"computer_event_index.next_credential_revocation_epoch":       "an epoch number",
		"self_development_start_intents.idempotency_key":              "a request's idempotency name",
		"self_development_operations.idempotency_key":                 "a request's idempotency name",
		"og_key_index.key_name":                                       "an object-graph index key's name",
		"og_key_index.key_value":                                      "an object-graph index key's value (object ids and kinds)",
	}
	shape := regexp.MustCompile(`(?i)token|secret|key|password|cookie|session|nonce|credential`)
	rows, err := s.ReadDB().Query(`SELECT m.name, p.name FROM sqlite_master m, pragma_table_info(m.name) p WHERE m.type = 'table' AND m.name NOT LIKE 'sqlite_%'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var unruled []string
	matched := 0
	for rows.Next() {
		var table, col string
		if err := rows.Scan(&table, &col); err != nil {
			t.Fatal(err)
		}
		name := table + "." + col
		if shape.MatchString(col) {
			matched++
			if rulings[name] == "" {
				unruled = append(unruled, name)
			}
		}
	}
	if matched < len(rulings)/2 {
		t.Fatalf("the scan matched %d columns; the schema query is not reading the store", matched)
	}
	if len(unruled) > 0 {
		sort.Strings(unruled)
		t.Fatalf("credential-shaped columns with no ruling (say what each holds, and blank it in SnapshotTo if it is a credential): %v", unruled)
	}
}
