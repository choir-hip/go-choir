package capsule

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
)

// Screenshots are a requirement for self-development (owner ruling,
// 2026-11-04: "screenshots are a requirement for self-development, period.
// For anything that is changing the UI, the look and feel, it needs a
// screenshot."), widened the same day: "even back-end changes which touch on
// the front-end should have a screenshot just to prove that there's still
// continuity. No regressions... For anything which is modifying anything
// that already exists ... you should take a screenshot before and after."
//
// The rules, enforced in Freeze beside the screen staging (the one producer
// of a bundle, so every route a candidate takes - the source stack, a
// package release draft, runtime install - starts from a bound bundle that
// already passed them; the engineer still holds its capsule, so it can
// shoot and Freeze again in the same round):
//
//   - screens_required: every candidate carries at least one screen, unless
//     all it changes is exempt (ScreenExemptPath: docs, Markdown, tests,
//     build output, tool state). Backend changes need one too: it proves
//     continuity.
//   - before_after_required: a candidate that modifies or deletes a
//     non-exempt file that exists on the base carries a before/after pair of
//     one view. A candidate that only adds files (a brand-new app) needs
//     after screens only. The before shot is the base build: the engineer
//     shoots the view before its first edit (refuseLateBeforeShots checks
//     the file time against the first source edit).
//
// Naming: [NN-]before-<view>.png and [NN-]after-<view>.png pair on <view>
// (ScreenRole). A screen with neither word is an after screen.
// Video is a later requirement, not this one (owner, 2026-11-04).

const (
	// ScreensRequiredReason opens the refusal for a candidate with no screen.
	ScreensRequiredReason = "screens_required"
	// BeforeAfterRequiredReason opens the refusal for a modifying candidate
	// without a before/after pair.
	BeforeAfterRequiredReason = "before_after_required"
)

// ScreenRoleKind says whether a screen is a base-build shot or a candidate shot.
type ScreenRoleKind string

const (
	ScreenBefore ScreenRoleKind = "before"
	ScreenAfter  ScreenRoleKind = "after"
)

var screenRoleRe = regexp.MustCompile(`(?i)^(?:\d+[-_])?(before|after)[-_](.+?)\.png$`)

// ScreenRole classifies a screen name: ("before"|"after", view) when it
// follows the pairing convention, (after, "") for any other name.
func ScreenRole(name string) (ScreenRoleKind, string) {
	m := screenRoleRe.FindStringSubmatch(name)
	if m == nil {
		return ScreenAfter, ""
	}
	return ScreenRoleKind(strings.ToLower(m[1])), strings.ToLower(m[2])
}

// HasBeforeAfterPair says some view has both a before and an after screen.
func HasBeforeAfterPair(screens []ScreenFile) bool {
	before, after := map[string]bool{}, map[string]bool{}
	for _, s := range screens {
		role, view := ScreenRole(s.Name)
		switch {
		case view == "":
		case role == ScreenBefore:
			before[view] = true
		default:
			after[view] = true
		}
	}
	for view := range before {
		if after[view] {
			return true
		}
	}
	return false
}

// ScreenExemptPath says a change to rel (relative to the platform source
// root, "/"-separated) cannot be shown by a screen and needs none: docs,
// Markdown, tests and test config, build output and tool state. The list is
// deliberately small; every other path, backend included, needs a screen.
func ScreenExemptPath(rel string) bool {
	rel = strings.TrimPrefix(path.Clean(strings.TrimPrefix(rel, "/")), "./")
	if rel == "" || rel == "." {
		return true
	}
	if notScreenSource(rel) || carriedExcludedPrefix(rel) != "" {
		return true
	}
	for _, seg := range []string{"tests", "test", "__tests__", "e2e", "testdata", "node_modules"} {
		if hasPathSegment(rel, seg) {
			return true
		}
	}
	base := path.Base(rel)
	for _, suffix := range []string{"_test.go", ".spec.js", ".spec.ts", ".spec.mjs", ".test.js", ".test.ts", ".test.mjs"} {
		if strings.HasSuffix(base, suffix) {
			return true
		}
	}
	return base == "playwright.config.js"
}

// screenRelevantChanges are the diff's non-exempt platform source changes.
func screenRelevantChanges(changes []FileChange) []FileChange {
	var out []FileChange
	for _, change := range changes {
		p := change.Path
		if !strings.HasPrefix(p, "/") {
			p = "/" + p
		}
		rel, ok := strings.CutPrefix(p, "/workspace/platform/")
		if !ok || change.Mode.IsDir() || ScreenExemptPath(rel) {
			continue
		}
		change.Path = rel
		out = append(out, change)
	}
	return out
}

func screenChangePaths(changes []FileChange) []string {
	var out []string
	for _, c := range changes {
		out = append(out, c.Path)
	}
	sort.Strings(out)
	return out
}

func summarizePaths(paths []string) string {
	shown := paths
	if len(shown) > 3 {
		shown = shown[:3]
	}
	more := ""
	if len(paths) > len(shown) {
		more = fmt.Sprintf(", +%d more", len(paths)-len(shown))
	}
	return strings.Join(shown, ", ") + more
}

// requireScreens is the Freeze rule over the diff and the staged screens.
func requireScreens(changes []FileChange, screens []ScreenFile, baseHasDir func(rel string) bool) error {
	relevant := screenRelevantChanges(changes)
	if len(relevant) == 0 {
		return nil
	}
	if len(screens) == 0 {
		return fmt.Errorf("capsule freeze: %s: this change touches %s and every candidate needs a screenshot (UI changes to show the look, backend changes to prove continuity); take at least one with node /workspace/platform/scripts/choir-shot.mjs (it writes PNGs to %s) after your last source write, then Freeze again", ScreensRequiredReason, summarizePaths(screenChangePaths(relevant)), ScreensDir)
	}
	var existing []FileChange
	for _, c := range relevant {
		if c.Kind == ChangeModified || c.Kind == ChangeDeleted {
			existing = append(existing, c)
		} else if root := packageRootOf(c.Path); root != "" && baseHasDir != nil && baseHasDir(root) {
			// Adding to a package that exists on the base updates an app:
			// the publish rule needs the pair, so Freeze asks for it too.
			existing = append(existing, c)
		}
	}
	if len(existing) > 0 && !HasBeforeAfterPair(screens) {
		return fmt.Errorf("capsule freeze: %s: this change modifies existing files (%s); shoot the same view on the base build (before) and the candidate (after), named NN-before-<view>.png and NN-after-<view>.png in %s; shoot the before view before your first edit", BeforeAfterRequiredReason, summarizePaths(screenChangePaths(existing)), ScreensDir)
	}
	return nil
}

// packageRootOf is the app package directory (frontend/src/packages/<name>)
// a source path lies in, or "".
func packageRootOf(rel string) string {
	const root = "frontend/src/packages/"
	rest, ok := strings.CutPrefix(rel, root)
	if !ok {
		return ""
	}
	name, _, hasFile := strings.Cut(rest, "/")
	if name == "" || !hasFile {
		return ""
	}
	return root + name
}
