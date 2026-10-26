package agentcore

import (
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// R10-6 (docs/evidence/selfdev-trace-reviews-2026-10-25.md, Round 10 section 5):
// the engineer needs one authoritative capsule browser recipe in its context,
// so it does not guess the toolchain (six statements, three false, in four
// assignments).

func TestEngineeringRunContextCarriesTheCapsuleBrowserRecipe(t *testing.T) {
	rt := &Runtime{}
	for _, kind := range []types.EngineeringAssignmentKind{types.EngineeringAssignmentImplementation, types.EngineeringAssignmentVerification} {
		rec := &types.RunRecord{
			RunID: "run:browser-recipe", AgentID: "engineering:browser-recipe", AgentProfile: agentprofile.Engineering, AgentRole: agentprofile.Engineering,
			Metadata: map[string]any{"assignment_id": "assignment-browser", "assignment_kind": string(kind)},
		}
		got := rt.runContextForRun(rec)
		for _, want := range []string{
			"Capsule browser recipe",
			"$CHOIR_FRONTEND_DEPS/node_modules",
			"Vite",
			"pinned Playwright",
			"executablePath",
			"under /tmp",
			"rm -rf node_modules",
			"networkless capsule",
			"bin/chromium",
			"npx playwright install",
			"context.route",
		} {
			if !strings.Contains(got, want) {
				t.Fatalf("%s engineering run context lacks %q:\n%s", kind, want, got)
			}
		}
	}
}

func TestCapsuleBrowserRecipeIsAbsentFromOtherDesks(t *testing.T) {
	rt := &Runtime{}
	for _, profile := range []string{agentprofile.Management, agentprofile.Research, agentprofile.Texture, agentprofile.Conductor} {
		rec := &types.RunRecord{
			RunID: "run:other-desk", AgentID: profile + ":other", AgentProfile: profile, AgentRole: profile,
			Metadata: map[string]any{"assignment_id": "assignment-browser", "assignment_kind": string(types.EngineeringAssignmentImplementation)},
		}
		if got := rt.runContextForRun(rec); strings.Contains(got, "Capsule browser recipe") || strings.Contains(got, "executablePath") {
			t.Fatalf("%s run context carries the engineering browser recipe:\n%s", profile, got)
		}
	}
}
