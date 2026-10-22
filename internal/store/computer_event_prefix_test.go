package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// LatestComputerEventByIdempotencyPrefix held the paper's engine for minutes
// after every boot: it walked the computer's whole finalized index
// newest-first and read event_json per row to test the LIKE
// (docs/problems/paper-unreachable-minutes-after-every-boot-2026-10-22.md).
// Ways the repair could fail, each a test below:
//  1. It returns another member of the family than the newest finalized one.
//  2. It reports an error, or a row, for an absent family.
//  3. A prefix with LIKE metacharacters (_ %) or a backslash matches as a
//     pattern rather than a literal.
//  4. A newer prepared (not finalized) member of the family wins.
//  5. A neighbouring key that sorts inside a naive range (prefix plus a
//     higher byte) or a key of another computer is returned.
//  6. It still reads rows outside the family: the plan does not range the
//     unique (computer_id, idempotency_key) key, or it sorts.

// insertEventIndexRow writes one computer_event_index row directly: these
// tests pin the read's semantics over the table, not the append protocol
// (computer_events_test.go covers Prepare and Finalize).
func insertEventIndexRow(t *testing.T, s *Store, computerID string, sequence uint64, key, status string) string {
	t.Helper()
	return insertEventIndexRowWith(t, s.db, computerID, sequence, key, status)
}

func insertEventIndexRowWith(t *testing.T, db interface {
	Exec(string, ...any) (sql.Result, error)
}, computerID string, sequence uint64, key, status string) string {
	t.Helper()
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s/%d", computerID, sequence)))
	digest := hex.EncodeToString(sum[:])
	raw, err := json.Marshal(computerevent.Event{ComputerID: computerID, Sequence: sequence, IdempotencyKey: key, EventKind: computerevent.EventGenesisImported})
	if err != nil {
		t.Fatal(err)
	}
	zero := computerevent.ZeroHead
	_, err = db.Exec(`INSERT INTO computer_event_index (event_digest, computer_id, sequence, previous_head, event_kind, event_json, event_artifact_digest, event_pin_receipt_digest, payload_pin_receipt_digests_json, request_commitment, idempotency_key, status, next_desired_event_head, next_effective_event_head, next_desired_state_commitment, next_effective_state_commitment, next_reducer_version, next_credential_revocation_epoch, prepared_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, '[]', ?, ?, ?, ?, ?, ?, ?, 1, 0, ?)`,
		digest, computerID, sequence, zero, string(computerevent.EventGenesisImported), string(raw), digest, digest, zero, key, status, zero, zero, zero, zero, time.Now().UTC())
	if err != nil {
		t.Fatalf("insert event index row %d: %v", sequence, err)
	}
	return digest
}

func latestByPrefix(t *testing.T, s *Store, computerID, prefix string) (computerevent.Event, bool) {
	t.Helper()
	event, found, err := s.LatestComputerEventByIdempotencyPrefix(context.Background(), computerID, prefix)
	if err != nil {
		t.Fatalf("latest by prefix %q: %v", prefix, err)
	}
	return event, found
}

func TestLatestEventByPrefixReturnsTheNewestFinalizedMember(t *testing.T) {
	s := openTestStore(t)
	insertEventIndexRow(t, s, "computer-test", 1, "platform-update-applied-b", "finalized")
	insertEventIndexRow(t, s, "computer-test", 2, "platform-update-applied-c", "finalized")
	insertEventIndexRow(t, s, "computer-test", 3, "platform-update-applied-a", "finalized") // newest by sequence, lowest by key
	insertEventIndexRow(t, s, "computer-test", 4, "platform-update-failed-a", "finalized")
	insertEventIndexRow(t, s, "computer-test", 5, "platform-update-applied-d", "prepared") // newer, not finalized
	event, found := latestByPrefix(t, s, "computer-test", "platform-update-applied-")
	if !found || event.Sequence != 3 || event.IdempotencyKey != "platform-update-applied-a" {
		t.Fatalf("latest = %+v found=%v, want sequence 3 platform-update-applied-a", event, found)
	}
}

func TestLatestEventByPrefixAbsentFamilyIsNotAnError(t *testing.T) {
	s := openTestStore(t)
	if _, found := latestByPrefix(t, s, "computer-test", "platform-update-applied-"); found {
		t.Fatal("empty store reported a member")
	}
	insertEventIndexRow(t, s, "computer-test", 1, "platform-update-failed-a", "finalized")
	insertEventIndexRow(t, s, "computer-test", 2, "platform-update-applied-x", "prepared")
	insertEventIndexRow(t, s, "computer-other", 3, "platform-update-applied-y", "finalized")
	if event, found := latestByPrefix(t, s, "computer-test", "platform-update-applied-"); found {
		t.Fatalf("absent family (only prepared, another family, another computer) returned %+v", event)
	}
}

func TestLatestEventByPrefixTreatsMetacharactersLiterally(t *testing.T) {
	s := openTestStore(t)
	insertEventIndexRow(t, s, "computer-test", 1, "a_b%c\\d-1", "finalized")
	insertEventIndexRow(t, s, "computer-test", 2, "aXbYc\\d-2", "finalized")  // matches a_b%c as a pattern only
	insertEventIndexRow(t, s, "computer-test", 3, "a_bZZc\\d-3", "finalized") // matches % as a pattern only
	event, found := latestByPrefix(t, s, "computer-test", "a_b%c\\d-")
	if !found || event.Sequence != 1 {
		t.Fatalf("literal prefix a_b%%c\\d- = %+v found=%v, want sequence 1", event, found)
	}
	if event, found := latestByPrefix(t, s, "computer-test", "a%"); found {
		t.Fatalf("prefix a%% matched %+v as a pattern", event)
	}
}

func TestLatestEventByPrefixExcludesNeighbouringKeys(t *testing.T) {
	s := openTestStore(t)
	insertEventIndexRow(t, s, "computer-test", 1, "platform-update-applied-a", "finalized")
	insertEventIndexRow(t, s, "computer-test", 2, "platform-update-applied.", "finalized") // '.' sorts right after '-'
	insertEventIndexRow(t, s, "computer-test", 3, "platform-update-applied", "finalized")  // the prefix minus its last byte
	insertEventIndexRow(t, s, "computer-test", 4, "platform-update-applied-é", "finalized")
	insertEventIndexRow(t, s, "computer-test", 5, "platform-update-applie", "finalized")
	event, found := latestByPrefix(t, s, "computer-test", "platform-update-applied-")
	if !found || event.Sequence != 4 {
		t.Fatalf("latest = %+v found=%v, want sequence 4 (the multi-byte member)", event, found)
	}
}

func TestLatestEventByPrefixAmongThousandsOfUnrelatedEvents(t *testing.T) {
	s := openTestStore(t)
	insertEventIndexRow(t, s, "computer-test", 1, "platform-update-applied-first", "finalized")
	insertEventIndexRow(t, s, "computer-test", 2, "platform-update-applied-second", "finalized")
	tx, err := s.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	for i := uint64(3); i < 5003; i++ {
		insertEventIndexRowWith(t, tx, "computer-test", i, fmt.Sprintf("run-telemetry-%06d", i), "finalized")
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	event, found := latestByPrefix(t, s, "computer-test", "platform-update-applied-")
	if !found || event.Sequence != 2 {
		t.Fatalf("latest among 5000 unrelated = %+v found=%v, want sequence 2", event, found)
	}
}

// TestLatestEventByPrefixPlanRangesTheIdempotencyKey asserts the access
// path with EXPLAIN rather than a time bound: a time bound on the embedded
// engine under -race is noise, and it would not say which rows were read. The
// plan says it directly: the family read is a range on the unique
// (computer_id, idempotency_key) key, with no sort, so its cost is the
// family's size, not the computer's event count.
func TestLatestEventByPrefixPlanRangesTheIdempotencyKey(t *testing.T) {
	s := openTestStore(t)
	for i := uint64(1); i < 200; i++ {
		insertEventIndexRow(t, s, "computer-test", i, fmt.Sprintf("run-telemetry-%06d", i), "finalized")
	}
	query, args := idempotencyFamilySQL("computer-test", "platform-update-applied-")
	rows, err := s.db.Query("EXPLAIN PLAN "+query, args...)
	if err != nil {
		t.Fatalf("explain: %v\n%s", err, query)
	}
	defer rows.Close()
	var b strings.Builder
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatal(err)
		}
		b.WriteString(line + "\n")
	}
	plan := b.String()
	if !strings.Contains(plan, "IndexedTableAccess(computer_event_index)") ||
		!strings.Contains(plan, "index: [computer_event_index.computer_id,computer_event_index.idempotency_key]") {
		t.Fatalf("family read does not range the (computer_id, idempotency_key) key:\n%s", plan)
	}
	if strings.Contains(plan, "TopN") || strings.Contains(plan, "Sort(") {
		t.Fatalf("family read sorts:\n%s", plan)
	}
	if strings.Contains(strings.ToLower(query), "event_json") {
		t.Fatalf("family read selects event_json for every member:\n%s", query)
	}
}
