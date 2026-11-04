package capsule

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

func TestScreenExemptPathTable(t *testing.T) {
	for _, tc := range []struct {
		rel    string
		exempt bool
	}{
		// exempt: docs, Markdown, tests, build output, tool state
		{"docs/design/x.html", true},
		{"docs/problems/a.md", true},
		{"README.md", true},
		{"frontend/README.md", true},
		{"frontend/tests/shell.spec.js", true},
		{"frontend/src/lib/foo.test.js", true},
		{"frontend/src/lib/foo.spec.ts", true},
		{"frontend/dist/assets/index.js", true},
		{"frontend/node_modules/x/index.js", true},
		{"frontend/playwright.config.js", true},
		{"internal/agentcore/runtime_test.go", true},
		{"internal/x/testdata/page.html", true},
		{".claude/settings.json", true},
		{"coverage/lcov.info", true},
		{"", true},
		// not exempt: UI, packages, and backend (continuity)
		{"frontend/src/App.svelte", false},
		{"frontend/src/packages/library-next/Shelf.svelte", false},
		{"frontend/src/app.css", false},
		{"frontend/index.html", false},
		{"frontend/package.json", false},
		{"frontend/package-lock.json", false},
		{"frontend/scripts/build.mjs", false},
		{"styles/tokens.css", false},
		{"internal/agentcore/runtime.go", false},
		{"cmd/autoputer/main.go", false},
		{"scripts/choir-shot.mjs", false},
		{"go.mod", false},
		{"nix/module.nix", false},
	} {
		if got := ScreenExemptPath(tc.rel); got != tc.exempt {
			t.Errorf("ScreenExemptPath(%q) = %v, want %v", tc.rel, got, tc.exempt)
		}
	}
}

func TestScreenRoleTable(t *testing.T) {
	for _, tc := range []struct {
		name string
		role ScreenRoleKind
		view string
	}{
		{"01-before-home.png", ScreenBefore, "home"},
		{"01-after-home.png", ScreenAfter, "home"},
		{"02_Before_Calendar-Month.png", ScreenBefore, "calendar-month"},
		{"before-home.png", ScreenBefore, "home"},
		{"after-home.png", ScreenAfter, "home"},
		{"home.png", ScreenAfter, ""},
		{"01-home.png", ScreenAfter, ""},
		{"beforehand.png", ScreenAfter, ""},
	} {
		if role, view := ScreenRole(tc.name); role != tc.role || view != tc.view {
			t.Errorf("ScreenRole(%q) = %s,%q want %s,%q", tc.name, role, view, tc.role, tc.view)
		}
	}
	pair := []ScreenFile{{Name: "01-before-home.png"}, {Name: "01-after-home.png"}}
	if !HasBeforeAfterPair(pair) {
		t.Error("a before/after pair of one view was not a pair")
	}
	for name, set := range map[string][]ScreenFile{
		"after only":      {{Name: "01-after-home.png"}},
		"plain":           {{Name: "home.png"}},
		"different views": {{Name: "01-before-home.png"}, {Name: "02-after-list.png"}},
		"none":            nil,
	} {
		if HasBeforeAfterPair(set) {
			t.Errorf("%s counted as a pair", name)
		}
	}
}

func diffOf(kind ChangeKind, paths ...string) []FileChange {
	var out []FileChange
	for _, p := range paths {
		out = append(out, FileChange{Path: "/workspace/platform/" + p, Kind: kind})
	}
	return out
}

func TestRequireScreens(t *testing.T) {
	shot := []ScreenFile{{Name: "home.png"}}
	pair := []ScreenFile{{Name: "01-before-home.png"}, {Name: "01-after-home.png"}}

	// A UI change with no screens is refused, naming the path and the way to shoot.
	err := requireScreens(diffOf(ChangeAdded, "frontend/src/NewApp.svelte", "docs/a.md"), nil, nil)
	if err == nil {
		t.Fatal("a UI change with no screens passed")
	}
	for _, want := range []string{ScreensRequiredReason, "frontend/src/NewApp.svelte", "choir-shot", ScreensDir} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q lacks %q", err, want)
		}
	}
	if strings.Contains(err.Error(), "docs/a.md") {
		t.Errorf("refusal names an exempt path: %q", err)
	}
	// A backend-only change with no screens is refused too (continuity).
	if err := requireScreens(diffOf(ChangeAdded, "internal/agentcore/x.go"), nil, nil); err == nil || !strings.Contains(err.Error(), ScreensRequiredReason) {
		t.Errorf("a backend change with no screens passed: %v", err)
	}
	// A package path is a source path like any other.
	if err := requireScreens(diffOf(ChangeAdded, "frontend/src/packages/reader/Reader.svelte"), nil, nil); err == nil {
		t.Error("a package-path change with no screens passed")
	}
	// Exempt-only changes need no screen.
	if err := requireScreens(diffOf(ChangeModified, "docs/a.md", "README.md", "internal/agentcore/x_test.go", "frontend/tests/a.spec.js", "frontend/dist/a.js"), nil, nil); err != nil {
		t.Errorf("an exempt-only change was refused: %v", err)
	}
	if err := requireScreens(nil, nil, nil); err != nil {
		t.Errorf("an empty diff was refused: %v", err)
	}
	// Adding only new files (a brand-new app): after screens suffice.
	if err := requireScreens(diffOf(ChangeAdded, "frontend/src/packages/new/App.svelte"), shot, nil); err != nil {
		t.Errorf("a new-files change with an after screen was refused: %v", err)
	}
	// Modifying an existing file needs a pair.
	err = requireScreens(diffOf(ChangeModified, "frontend/src/App.svelte"), shot, nil)
	if err == nil || !strings.Contains(err.Error(), BeforeAfterRequiredReason) || !strings.Contains(err.Error(), "-before-") {
		t.Errorf("a modifying change with no pair: %v", err)
	}
	if err := requireScreens(diffOf(ChangeModified, "internal/agentcore/x.go"), shot, nil); err == nil {
		t.Error("a modifying backend change with no pair passed")
	}
	if err := requireScreens(diffOf(ChangeDeleted, "frontend/src/Old.svelte"), shot, nil); err == nil {
		t.Error("deleting an existing file with no pair passed")
	}
	if err := requireScreens(diffOf(ChangeModified, "frontend/src/App.svelte"), pair, nil); err != nil {
		t.Errorf("a modifying change with a pair was refused: %v", err)
	}
	// Modifying only an exempt file does not demand a pair.
	mixed := append(diffOf(ChangeModified, "docs/a.md"), diffOf(ChangeAdded, "frontend/src/N.svelte")...)
	if err := requireScreens(mixed, shot, nil); err != nil {
		t.Errorf("an exempt modification demanded a pair: %v", err)
	}
	// Adding files to a package that exists on the base updates an app and
	// needs the pair, as the publish rule does; a package new to the base does not.
	has := func(rel string) bool { return rel == "frontend/src/packages/reader" }
	add := diffOf(ChangeAdded, "frontend/src/packages/reader/Extra.svelte")
	if err := requireScreens(add, shot, has); err == nil || !strings.Contains(err.Error(), BeforeAfterRequiredReason) {
		t.Errorf("an add into an existing package with no pair: %v", err)
	}
	if err := requireScreens(add, pair, has); err != nil {
		t.Errorf("an add into an existing package with a pair: %v", err)
	}
	if err := requireScreens(diffOf(ChangeAdded, "frontend/src/packages/brandnew/App.svelte"), shot, has); err != nil {
		t.Errorf("a new package demanded a pair: %v", err)
	}
	// The screens directory itself is not source.
	if err := requireScreens([]FileChange{{Path: ScreensPrefix + "a.png", Kind: ChangeAdded}}, nil, nil); err != nil {
		t.Errorf("the screens directory counted as a change: %v", err)
	}
	many := diffOf(ChangeAdded, "a.go", "b.go", "c.go", "d.go")
	if err := requireScreens(many, nil, nil); err == nil || !strings.Contains(err.Error(), "+1 more") {
		t.Errorf("refusal does not summarize: %v", err)
	}
}

func TestRefuseLateBeforeShots(t *testing.T) {
	edit := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	mods := map[string]time.Time{
		"01-before-home.png": edit.Add(-time.Minute),
		"02-before-late.png": edit.Add(time.Minute),
		"01-after-home.png":  edit.Add(2 * time.Minute),
	}
	mod := func(name string) (time.Time, error) { return mods[name], nil }
	if err := refuseLateBeforeShots([]string{"01-before-home.png", "01-after-home.png"}, edit, mod); err != nil {
		t.Errorf("a before shot older than the first edit was refused: %v", err)
	}
	err := refuseLateBeforeShots([]string{"01-before-home.png", "02-before-late.png"}, edit, mod)
	if err == nil || !strings.Contains(err.Error(), "02-before-late.png") || !strings.Contains(err.Error(), BeforeAfterRequiredReason) {
		t.Errorf("a before shot taken after the first edit: %v", err)
	}
	if err := refuseLateBeforeShots([]string{"02-before-late.png"}, time.Time{}, mod); err != nil {
		t.Errorf("no source edit should skip the check: %v", err)
	}
}

func TestStageScreenFilesLetsABeforeShotPredateTheEdit(t *testing.T) {
	at := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	png := testPNG(t, 32, 32, false)
	open := func(name string) (screenInput, error) {
		mod := at.Add(time.Minute)
		if strings.Contains(name, "before") {
			mod = at.Add(-time.Minute)
		}
		return memInput{Reader: bytes.NewReader(png), mod: mod}, nil
	}
	if _, err := stageScreenFiles(context.Background(), []string{"01-before-home.png", "01-after-home.png"}, open, t.TempDir(), at, "App.svelte"); err != nil {
		t.Fatalf("a before shot older than the newest edit was called stale: %v", err)
	}
}

func TestSourceChangeTimesIgnoreExemptEdits(t *testing.T) {
	t0 := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	mods := map[string]time.Time{
		"/workspace/platform/frontend/tests/a.spec.js": t0.Add(-time.Hour), // tests written first
		"/workspace/platform/frontend/src/App.svelte":  t0,
		"/workspace/platform/docs/a.md":                t0.Add(time.Hour),
	}
	var changes []FileChange
	for p := range mods {
		changes = append(changes, FileChange{Path: p, Kind: ChangeModified})
	}
	oldest, newest, _, err := sourceChangeTimes(changes, func(p string) (time.Time, error) { return mods[p], nil })
	if err != nil || !oldest.Equal(t0) || !newest.Equal(t0) {
		t.Fatalf("oldest %v newest %v err %v, want both %v", oldest, newest, err, t0)
	}
}
