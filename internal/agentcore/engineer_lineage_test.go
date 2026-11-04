package agentcore

import (
	"context"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Engineer reuse, phase B (docs/design/engineer-reuse-2026-11-04.md). The
// ways it can fail:
//  1. A change is credited to the wrong app, or a non-app path to an app.
//  2. A request about an app does not find its lineage, or a request about
//     something else is handed a stranger's brief.
//  3. The brief replaces the code as the source of truth, or grows without
//     bound into the run context.
//  4. The brief changes between the run's turns (the prompt cache breaks).
//  5. An assignment is handed its own lineage.

func TestEngineerLineageAppOfPath(t *testing.T) {
	for path, want := range map[string]string{
		"workspace/platform/frontend/src/packages/calendar/src/App.svelte": "calendar",
		"frontend/src/packages/image/index.ts":                             "image",
		"workspace/platform/frontend/src/lib/CalendarApp.svelte":           "calendar",
		"workspace/platform/frontend/src/lib/MailApp.svelte":               "mail",
		"workspace/platform/frontend/src/lib/apps/registry.ts":             "",
		"workspace/platform/frontend/src/lib/Desktop.svelte":               "",
		"workspace/platform/internal/agentcore/runtime.go":                 "",
		"workspace/platform/frontend/src/packages/README.md":               "",
		"workspace/platform/frontend/src/packages/../../etc/x":             "",
	} {
		if got := lineageAppOfPath(path); got != want {
			t.Errorf("lineageAppOfPath(%q) = %q, want %q", path, got, want)
		}
	}
	apps := lineageAppsOfPaths([]string{
		"workspace/platform/frontend/src/packages/calendar/b.ts",
		"workspace/platform/frontend/src/packages/calendar/a.ts",
		"workspace/platform/frontend/src/lib/CalendarApp.svelte",
		"workspace/platform/docs/x.md",
	})
	if len(apps) != 1 || strings.Join(apps["calendar"], ",") != "frontend/src/lib/CalendarApp.svelte,frontend/src/packages/calendar/a.ts,frontend/src/packages/calendar/b.ts" {
		t.Fatalf("lineageAppsOfPaths = %v", apps)
	}
}

func TestEngineerLineageAppsNamed(t *testing.T) {
	apps := []string{"calendar", "image", "mail"}
	for text, want := range map[string]string{
		"add a feature to the Calendar app built last week": "calendar",
		"CalendarApp should show weeks":                     "calendar",
		"fix the image and mail packages, then calendar":    "calendar,image",
		"make the desktop faster":                           "",
		"calendaring is hard":                               "",
		"the calendar-app grid":                             "calendar",
	} {
		if got := strings.Join(lineageAppsNamed(text, apps), ","); got != want {
			t.Errorf("lineageAppsNamed(%q) = %q, want %q", text, got, want)
		}
	}
}

func TestEngineerLineageNoticeFromTheStore(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	const owner, computer = "owner-lineage", "computer-lineage"
	summary := strings.Repeat("built the week view. ", 10) + "Handoff: the grid lives in WeekGrid.svelte; drag to move is open."
	if err := s.PutEngineeringLineage(ctx, store.EngineeringLineage{
		OwnerID: owner, ComputerID: computer, App: "Calendar", AssignmentID: "assignment-old", Attempt: 1, RunID: "run-old",
		OperationID: "selfdev-old", Result: "completed", Files: []string{"frontend/src/packages/calendar/WeekGrid.svelte"}, Summary: summary,
	}); err != nil {
		t.Fatal(err)
	}
	if apps, err := s.ListEngineeringLineageApps(ctx, owner, computer); err != nil || strings.Join(apps, ",") != "calendar" {
		t.Fatalf("apps = %v, %v", apps, err)
	}
	if apps, _ := s.ListEngineeringLineageApps(ctx, "owner-other", computer); len(apps) != 0 {
		t.Fatalf("another owner sees the lineage: %v", apps)
	}
	rec := &types.RunRecord{RunID: "run-new", OwnerID: owner, ComputerID: computer, Prompt: "Add drag to move to the Calendar app",
		Metadata: map[string]any{"assignment_kind": string(types.EngineeringAssignmentImplementation), "assignment_id": "assignment-new"}}
	notice := rt.lineageNoticeForRun(rec)
	for _, want := range []string{"Prior work on the calendar app", "assignment-old", "run run-old", "WeekGrid.svelte", "Handoff: the grid lives", "trust them over it"} {
		if !strings.Contains(notice, want) {
			t.Fatalf("notice lacks %q:\n%s", want, notice)
		}
	}
	// The same run gets the same brief, even after the lineage moves on.
	if err := s.PutEngineeringLineage(ctx, store.EngineeringLineage{OwnerID: owner, ComputerID: computer, App: "calendar", AssignmentID: "assignment-newer", Summary: "x"}); err != nil {
		t.Fatal(err)
	}
	if again := rt.lineageNoticeForRun(rec); again != notice {
		t.Fatal("the brief changed between the run's turns")
	}
	// A run about something else gets none; a run is never handed its own.
	other := &types.RunRecord{RunID: "run-other", OwnerID: owner, ComputerID: computer, Prompt: "make the desktop faster",
		Metadata: map[string]any{"assignment_kind": string(types.EngineeringAssignmentImplementation), "assignment_id": "assignment-other"}}
	if got := rt.lineageNoticeForRun(other); got != "" {
		t.Fatalf("an unrelated request got a brief: %s", got)
	}
	own := &types.RunRecord{RunID: "run-own", OwnerID: owner, ComputerID: computer, Prompt: "the calendar again",
		Metadata: map[string]any{"assignment_kind": string(types.EngineeringAssignmentImplementation), "assignment_id": "assignment-newer"}}
	if got := rt.lineageNoticeForRun(own); got != "" {
		t.Fatalf("an assignment was handed its own lineage: %s", got)
	}
	verification := &types.RunRecord{RunID: "run-verify", OwnerID: owner, ComputerID: computer, Prompt: "the calendar",
		Metadata: map[string]any{"assignment_kind": string(types.EngineeringAssignmentVerification)}}
	if got := rt.lineageNoticeForRun(verification); got != "" {
		t.Fatal("a verification run got a brief")
	}
}
