package agentcore

import (
	"strings"
	"testing"
)

// Ways this could fail, written first: a differing base shows no note; equal
// commits show a false mismatch; an unknown booted commit (nil BootedCommit)
// produces a note against ""; the fields overwrite existing result keys.
func TestFreezeResultNamesTheReleaseBaseAndBootedCommit(t *testing.T) {
	a, b := strings.Repeat("a", 40), strings.Repeat("b", 40)

	out := map[string]any{"handle": "h"}
	noteFreezeBase(out, strings.ToUpper(a), b)
	if out["release_base"] != a || out["booted_commit"] != b || out["handle"] != "h" {
		t.Fatalf("fields = %+v", out)
	}
	if note, _ := out["base_note"].(string); !strings.Contains(note, a[:12]) || !strings.Contains(note, b[:12]) || !strings.Contains(note, "differs") {
		t.Fatalf("a differing base has no note: %+v", out)
	}

	same := map[string]any{}
	noteFreezeBase(same, a, a)
	if _, has := same["base_note"]; has || same["release_base"] != a || same["booted_commit"] != a {
		t.Fatalf("equal commits: %+v", same)
	}

	unknown := map[string]any{}
	noteFreezeBase(unknown, a, "")
	if _, has := unknown["base_note"]; has || unknown["release_base"] != a {
		t.Fatalf("unknown booted commit: %+v", unknown)
	}
	if _, has := unknown["booted_commit"]; has {
		t.Fatalf("empty booted commit was reported: %+v", unknown)
	}
}
