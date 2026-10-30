package store

import (
	"bytes"
	"fmt"
	"log"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// docs/archive/problems/stored-column-add-blocks-paper-boot-2026-10-21.md: the paper's
// boot hung for 50 minutes and the log said nothing about which step. Every
// store step between the process start and the listener now logs its start,
// and its end with a duration. Ways this can fail: a step has no start line
// (a hang is unnamed); a start has no end (a hang is named, as intended, but
// a clean boot must close every one); a step logs an end with no duration.

func TestOpenLogsEveryStoreStepWithItsDuration(t *testing.T) {
	var buf bytes.Buffer
	origWriter, origFlags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	restore := func() { log.SetOutput(origWriter); log.SetFlags(origFlags) }
	defer restore()

	path := testStorePath(t)
	cleanupTestStorePath(path)
	s, err := Open(path)
	restore()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = s.Close()
		cleanupTestStorePath(path)
	})

	line := regexp.MustCompile(`^store: open phase=(\S.*?) status=(starting|complete|failed)(?: duration=(\S+))?`)
	open := map[string]int{}
	var order []string
	for _, l := range strings.Split(buf.String(), "\n") {
		m := line.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		name, status, duration := m[1], m[2], m[3]
		switch status {
		case "starting":
			open[name]++
			order = append(order, name)
		case "complete":
			if duration == "" {
				t.Errorf("step %q ended without a duration: %s", name, l)
			}
			if open[name] == 0 {
				t.Errorf("step %q ended without a start line", name)
			}
			open[name]--
		case "failed":
			t.Errorf("step %q failed on a clean open: %s", name, l)
		}
	}
	for name, n := range open {
		if n != 0 {
			t.Errorf("step %q started and never ended on a clean open", name)
		}
	}
	for _, want := range []string{
		"prepare-path", "workspace-open",
		"runtime-schema", "runtime-schema/apply-ddl",
		"objectgraph-schema", "objectgraph-schema/schema-ddl",
		"objectgraph-schema/derived-keys-table",
		"store-schema-epoch",
	} {
		found := false
		for _, name := range order {
			if name == want {
				found = true
			}
		}
		if !found {
			t.Errorf("no start line for boot step %q; logged: %v", want, order)
		}
	}
}

// The boot must not scan or rewrite og_objects (the 2026-10-21 outage: an
// ALTER that rewrote every row held the paper's boot for 50 minutes). A store
// that already has many rows is closed and opened again, as a restart does:
// og_objects must be byte-identical after Open (Dolt's table hash), the side
// table must be empty and every family unvalidated (the check is online,
// after the listener), and Open must take about as long as it does empty.
func TestReopenWithManyRowsDoesNotTouchOgObjects(t *testing.T) {
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	const rows = 24000
	tx, err := s.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	for start := 0; start < rows; start += 500 {
		var sb strings.Builder
		var args []any
		sb.WriteString(`INSERT INTO og_objects (canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by) VALUES `)
		for i := start; i < start+500; i++ {
			if i > start {
				sb.WriteString(",")
			}
			sb.WriteString("(?,?,?,?,?,?,?,?,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,0,'')")
			kind := []string{"choir.run", "choir.commitment_record", "choir.source_ref", "choir.agent"}[i%4]
			args = append(args, fmt.Sprintf("obj:%s:o1:key-%06d", kind, i), kind, "o1", "c1", "v", fmt.Sprintf("h-%d", i),
				fmt.Sprintf(`{"addressee":"a%d"}`, i%50), fmt.Sprintf(`{"agent_id":"a%d","doc_id":"d%d"}`, i%50, i%90))
		}
		if _, err := tx.Exec(sb.String(), args...); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	hashOf := func(st *Store) string {
		return sqliteTableDigest(t, st, "og_objects")
	}
	before := hashOf(s)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	reopened, err := Open(path)
	took := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	if after := hashOf(reopened); after != before {
		t.Fatal("Open rewrote og_objects")
	}
	var side int
	if err := reopened.db.QueryRow(`SELECT COUNT(*) FROM og_key_index`).Scan(&side); err != nil || side != 0 {
		t.Fatalf("Open filled the side table: %d rows, %v", side, err)
	}
	d := reopened.derivedKeys()
	if allFamiliesValidated(d) || d.Validated(objectgraph.KeyRunAgentID) || d.Validated(objectgraph.KeyCommitmentAddressee) || d.Validated(objectgraph.KeySourceRefDocID) {
		t.Fatal("a family is validated straight after Open; the check is online, after the listener")
	}
	t.Logf("Open over %d rows: %s", rows, took.Round(time.Millisecond))
	if took > 20*time.Second {
		t.Fatalf("Open over %d rows took %s", rows, took)
	}
}
