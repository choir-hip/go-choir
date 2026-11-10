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
	existing := existingSurfaceChanges(relevant, baseHasDir)
	if len(existing) > 0 && !HasBeforeAfterPair(screens) {
		return fmt.Errorf("capsule freeze: %s: this change modifies existing files (%s); shoot the same view on the base build (before) and the candidate (after), named NN-before-<view>.png and NN-after-<view>.png in %s; shoot the before view before your first edit", BeforeAfterRequiredReason, summarizePaths(screenChangePaths(existing)), ScreensDir)
	}
	return nil
}

// existingSurfaceChanges are the relevant changes that touch a surface which
// exists on the base: a modified or deleted file, or a file added to an app
// package that exists on the base. Only they need a before and after pair.
func existingSurfaceChanges(relevant []FileChange, baseHasDir func(rel string) bool) []FileChange {
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
	return existing
}

// needsBeforeAfterPair says the diff needs a before and after pair: the one
// predicate behind both the pair requirement (requireScreens) and the
// late-before-shots check (stageScreens), so the two cannot disagree. A new
// package needs no before shot, so nothing can be late; a diff that mixes a
// new package with a changed existing surface does need the pair.
func needsBeforeAfterPair(changes []FileChange, baseHasDir func(rel string) bool) bool {
	return len(existingSurfaceChanges(screenRelevantChanges(changes), baseHasDir)) > 0
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

// requireScreensOrPlan is the Freeze rule with a shot plan: a valid plan
// satisfies screens_required and before_after_required for a platform
// candidate, because the platform shoots the host-built candidate against
// the running build before the swap (design 3.3, Freeze rule). The plan is
// validated at read; an all-exempt change needs neither, as before.
func requireScreensOrPlan(changes []FileChange, screens []ScreenFile, baseHasDir func(rel string) bool, plan *ShotPlan) error {
	if plan != nil {
		if err := plan.Validate(); err != nil {
			return fmt.Errorf("capsule freeze: %w", err)
		}
		return nil
	}
	return requireScreens(changes, screens, baseHasDir)
}

// describeCandidateLayer names what a candidate is, for a refusal.
func describeCandidateLayer(patchSHA, layer string) string {
	switch {
	case patchSHA == "":
		return "a change with no source patch"
	case layer != "":
		return "a change to package " + layer
	}
	return "a platform change"
}

// ScreenRelevantPaths are the platform-relative paths among changes that a
// screen must show: the non-exempt platform source changes (the same set
// Freeze's screens rule judges). The materializer's candidate-shot
// interlock reads a frozen bundle's effects through it.
func ScreenRelevantPaths(changes []FileChange) []string {
	return screenChangePaths(screenRelevantChanges(changes))
}

// PlatformSourceTouched says the diff changes a file under /workspace/platform
// that a source patch would carry (not node_modules, .git or .cache). The
// Complete guard reads it: such a diff is a candidate that must be frozen.
func PlatformSourceTouched(changes []FileChange) bool {
	for _, change := range changes {
		p := strings.TrimPrefix(change.Path, "/")
		rel, ok := strings.CutPrefix(p, platformWorkspacePrefix)
		if !ok || change.Mode.IsDir() || rel == "" || carriedExcludedPrefix(rel) != "" {
			continue
		}
		return true
	}
	return false
}
