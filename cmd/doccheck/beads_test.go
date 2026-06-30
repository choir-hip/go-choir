package main

import (
	"os"
	"path/filepath"
	"testing"
)

// fixture covers: an epic, closed and open children parented via the explicit
// `parent` field, a child parented via a parent-child dependency edge only
// (mirroring the real .beads JSONL export which omits the field), a blank line,
// and an unrelated issue.
const beadsFixture = `{"_type":"issue","id":"epic-1","title":"Epic One","status":"open","issue_type":"epic","labels":["mission"]}
{"_type":"issue","id":"epic-1.1","title":"Closed child","status":"closed","parent":"epic-1"}

{"_type":"issue","id":"epic-1.2","title":"Open child","status":"in_progress","parent":"epic-1"}
{"_type":"issue","id":"epic-1.3","title":"Edge-only child","status":"open","dependencies":[{"issue_id":"epic-1.3","depends_on_id":"epic-1","type":"parent-child"}]}
{"_type":"issue","id":"other","title":"Unrelated","status":"open"}
`

func TestBeadsReader(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "issues.jsonl")
	if err := os.WriteFile(path, []byte(beadsFixture), 0o644); err != nil {
		t.Fatal(err)
	}

	issues, err := readBeadsJSONL(path)
	if err != nil {
		t.Fatalf("readBeadsJSONL: %v", err)
	}

	t.Run("issue count ignores blank lines", func(t *testing.T) {
		if got, want := len(issues), 5; got != want {
			t.Fatalf("issue count = %d, want %d", got, want)
		}
	})

	t.Run("parent/child grouping", func(t *testing.T) {
		children := beadsEpicChildren(issues, "epic-1")
		var ids []string
		for _, c := range children {
			ids = append(ids, c.ID)
		}
		want := []string{"epic-1.1", "epic-1.2", "epic-1.3"}
		if len(ids) != len(want) {
			t.Fatalf("children = %v, want %v", ids, want)
		}
		for i := range want {
			if ids[i] != want[i] {
				t.Fatalf("children = %v, want %v", ids, want)
			}
		}
	})

	t.Run("variant counts only non-closed children", func(t *testing.T) {
		// epic-1.1 is closed; epic-1.2 and epic-1.3 are open => V=2.
		if got, want := beadsVariant(issues, "epic-1"), 2; got != want {
			t.Fatalf("beadsVariant = %d, want %d", got, want)
		}
		if got := beadsVariant(issues, "no-such-epic"); got != 0 {
			t.Fatalf("beadsVariant(missing) = %d, want 0", got)
		}
	})
}

func TestBeadsReaderMalformedLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "issues.jsonl")
	bad := `{"_type":"issue","id":"ok","status":"open"}
this is not json
`
	if err := os.WriteFile(path, []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readBeadsJSONL(path); err == nil {
		t.Fatal("expected error on malformed line, got nil")
	} else if want := "line 2"; !contains(err.Error(), want) {
		t.Fatalf("error %q does not mention %q", err.Error(), want)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
