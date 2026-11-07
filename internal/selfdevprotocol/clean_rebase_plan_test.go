package selfdevprotocol

import (
	"strings"
	"testing"
)

// A clean rebase carries the shot plan unchanged (source-only freeze, design
// 3.1): shot_plan is not a changeable field, so a rebase that edits, adds or
// drops it is refused, and a rebase that carries it byte for byte passes.
// Failure modes: the plan is silently replaced on a rebase (the new base is
// shot with different views than the owner approved); a source-only bundle's
// empty runtime_files is read as a difference.
func TestCleanRebaseCarriesTheShotPlanUnchanged(t *testing.T) {
	patch := strings.Repeat("7", 64)
	bundle := func(base, plan string) []byte {
		out := `{"source_patch_sha256":"` + patch + `","source_patch_base_commit":"` + base + `","runtime_files":[],"release_binary":{"origin":"host_build","sha256":""}`
		if plan != "" {
			out += `,"shot_plan":` + plan
		}
		return []byte(out + `}`)
	}
	const (
		a    = "0123456789abcdef0123456789abcdef01234567"
		b    = "89abcdef0123456789abcdef0123456789abcdef"
		plan = `{"views":[{"name":"home","path":"/","viewport":"desktop"}]}`
	)
	if _, err := CompareCleanRebaseBundles(bundle(a, plan), bundle(b, plan)); err != nil {
		t.Fatalf("a rebase that carries the plan unchanged: %v", err)
	}
	if _, err := CompareCleanRebaseBundles(bundle(a, plan), bundle(b, `{"views":[{"name":"other","path":"/","viewport":"desktop"}]}`)); err == nil {
		t.Fatal("a rebase that changed the plan was admitted")
	}
	if _, err := CompareCleanRebaseBundles(bundle(a, plan), bundle(b, "")); err == nil {
		t.Fatal("a rebase that dropped the plan was admitted")
	}
	if _, err := CompareCleanRebaseBundles(bundle(a, ""), bundle(b, plan)); err == nil {
		t.Fatal("a rebase that added a plan was admitted")
	}
	for _, field := range CleanRebaseBundleFields {
		if field == "shot_plan" {
			t.Fatal("shot_plan must not be a field a clean rebase may change")
		}
	}
}
