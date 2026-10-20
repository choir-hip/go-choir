package agentcore

// Failure 15 (docs/problems/engineering-completion-path-cluster-2026-10-20.md):
// a re-cast ran in a fresh capsule, the earlier round's change was gone, and
// a capsule that changed no source froze, passed verify and approval, and was
// applied. Ways the Freeze source check could fail (written before the code):
//
//  1. A staged release with no source patch freezes: refused.
//  2. A missing staged release (nil) freezes: refused, never a panic.
//  3. A real change (any /workspace/platform file, frontend source included,
//     so a non-empty patch) is refused: never.
//  4. The refusal does not tell the desk why or what to do: it says the
//     capsule changed no source, that a re-cast's capsule starts from the
//     base tree, and what to do next.

import (
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/capsule"
)

func TestFreezeSourceChangeError(t *testing.T) {
	for _, tc := range []struct {
		name   string
		staged *capsule.StagedRelease
		refuse bool
	}{
		{"no source patch", &capsule.StagedRelease{Files: []capsule.FrozenReleaseFile{{Path: "frontend/index.html"}}}, true},
		{"blank source patch digest", &capsule.StagedRelease{SourcePatchSHA: "  "}, true},
		{"nil staged release", nil, true},
		{"a source change", &capsule.StagedRelease{SourcePatchSHA: strings.Repeat("a", 64), SourceBaseCommit: "ac4737862"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := freezeSourceChangeError(tc.staged)
			if (err != nil) != tc.refuse {
				t.Fatalf("freezeSourceChangeError = %v, want refuse=%v", err, tc.refuse)
			}
			if err == nil {
				return
			}
			for _, want := range []string{"changed no /workspace/platform source", "fresh capsule", "base tree", "Complete"} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("refusal %q does not say %q", err, want)
				}
			}
		})
	}
}
