package claudedelegate

import "testing"

// The delegate is Sonnet at medium effort (owner 2026-11-09).
func TestDelegateRunsSonnetAtMediumEffort(t *testing.T) {
	if Model != "claude-sonnet-5-5" || Effort != "medium" {
		t.Fatalf("model=%s effort=%s", Model, Effort)
	}
	env, err := Env([]string{"PATH=/bin"}, "/tmp/choir-delegate/d1/home", fakeToken)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, kv := range env {
		if kv == "CLAUDE_CODE_EFFORT_LEVEL=medium" {
			found = true
		}
	}
	if !found {
		t.Fatal("CLAUDE_CODE_EFFORT_LEVEL=medium missing from the delegate env")
	}
}
