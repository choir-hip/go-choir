package store

import (
	"bytes"
	"log"
	"regexp"
	"strings"
	"testing"
)

// docs/problems/stored-column-add-blocks-paper-boot-2026-10-21.md: the paper's
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
		"runtime-schema", "runtime-schema/apply-ddl", "runtime-schema/ensure-columns", "runtime-schema/events-indexes",
		"runtime-schema/desktop-primary-keys", "runtime-schema/backfill-events-stream-seq",
		"objectgraph-schema", "objectgraph-schema/schema-ddl", "objectgraph-schema/og-objects-indexes",
		"objectgraph-schema/derived-keys-table", "objectgraph-schema/metadata-columns",
		"texture-schema", "texture-schema/apply-ddl", "texture-schema/backfill-revision-version-numbers",
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
