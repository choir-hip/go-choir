package runtimeprompts

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// shellScriptCalls matches the overlay's worked `sh -c` recipes in either call
// shape, capturing the verb (Exec or Start) and the script.
var shellScriptCalls = regexp.MustCompile(`choir\.(Exec|Start)\((?:\[\]string\{"sh", "-c"|"sh", \[\]string\{"-c"), "([^"]*)"`)

func TestRLMEngineeringOverlayGatesFreezeMandate(t *testing.T) {
	const mandate = "Freeze the capsule diff with choir.Freeze."
	if overlay := RLMEngineeringOverlay(RLMEngineeringOverlayOptions{}); strings.Contains(overlay, mandate) {
		t.Fatalf("document-cast overlay mandates Freeze: %q", overlay)
	}
	if overlay := RLMEngineeringOverlay(RLMEngineeringOverlayOptions{HasSelfDevelopmentOperation: true}); !strings.Contains(overlay, mandate) {
		t.Fatalf("self-development overlay omits Freeze mandate: %q", overlay)
	}
}

// Gate 2 track D: with no pre-created operation, an armed computer still lets
// an ordinary assignment propose its change by freezing it; an unarmed one
// does not mention it.
func TestRLMEngineeringOverlayOffersProposalWhenArmed(t *testing.T) {
	const offer = "To propose a change meant to land on this computer, freeze it with choir.Freeze"
	if overlay := RLMEngineeringOverlay(RLMEngineeringOverlayOptions{}); strings.Contains(overlay, offer) {
		t.Fatalf("unarmed overlay offers proposal: %q", overlay)
	}
	if overlay := RLMEngineeringOverlay(RLMEngineeringOverlayOptions{CanProposeChange: true}); !strings.Contains(overlay, offer) {
		t.Fatalf("armed overlay omits the proposal offer: %q", overlay)
	}
}

// Support judgement left management
// (docs/reports/support-judgement-to-jev-design-2026-10-14.md): Jev judges a
// Texture commit's citation pairs off management's turn, so the overlay
// must not teach Judge, support_pairs records or judging pairs by hand.
func TestRLMManagementOverlayCarriesNoSupportJudgement(t *testing.T) {
	for _, labels := range []bool{false, true} {
		overlay := RLMManagementOverlay(RLMManagementOverlayOptions{IntakeLabels: labels})
		for _, absent := range []string{"choir.Judge", "support_pairs", "judge yourself", "Support judgement"} {
			if strings.Contains(overlay, absent) {
				t.Errorf("management overlay (labels %v) still says %q", labels, absent)
			}
		}
	}
}

// The overlay must teach the release root the freeze actually reads. The C1e
// B1 drill (docs/problems/c1e-b1-engineering-assignments-cancelled-on-arrival-2026-10-14.md)
// staged the tree under /workspace/platform because the overlay said so, and
// Freeze refused with "capsule release contains no frozen runtime artifacts".
//
// Failure modes: the overlay names a root the executor does not read; the
// overlay places the release tree inside the workspace; the executor's
// prefix moves and the overlay keeps the old one.
func TestRLMEngineeringOverlayNamesTheReleaseRootFreezeReads(t *testing.T) {
	root := "/" + capsule.ReleaseTreePrefix
	overlay := RLMEngineeringOverlay(RLMEngineeringOverlayOptions{HasSelfDevelopmentOperation: true})
	if !strings.Contains(overlay, "at the capsule root, "+root) {
		t.Fatalf("overlay does not name the capsule-root release tree %s: %q", root, overlay)
	}
	if !strings.Contains(overlay, `"go", "build", "-o", "`+root+`bin/autoputer"`) {
		t.Fatalf("overlay does not build bin/autoputer into %s", root)
	}
	if strings.Contains(overlay, capsule.ReleaseTreePrefix+" inside /workspace/platform") {
		t.Fatalf("overlay places the release tree inside the workspace: %q", overlay)
	}
}

// A change with no Go input ships the running binary (docs/problems/
// selfdev-drill-capsule-cannot-build-autoputer-2026-10-14.md, option A): the
// overlay must stop telling the desk to build bin/autoputer for a
// frontend-only change, must name the Go inputs the executor classifies,
// and must keep the build instruction for a Go change. Failure modes: the
// overlay still demands a build for every change (the drill spends its
// window linking); the overlay tells the desk to copy a binary itself; the
// overlay's Go-input list drifts from capsule.BinaryInputPath.
func TestRLMEngineeringOverlayTeachesBinaryReuseForNonGoChanges(t *testing.T) {
	overlay := RLMEngineeringOverlay(RLMEngineeringOverlayOptions{HasSelfDevelopmentOperation: true})
	if strings.Contains(overlay, "a frontend-only change still ships the binary") {
		t.Fatalf("overlay still demands a build for a frontend-only change")
	}
	for _, want := range []string{"touches no Go input", "Freeze stages this computer's running binary", "Never stage a stub"} {
		if !strings.Contains(overlay, want) {
			t.Errorf("overlay does not say %q", want)
		}
	}
	for _, rel := range []string{"cmd/", "internal/", "vendor/", "nix/", "go.mod", "go.sum", "flake.nix", "flake.lock"} {
		if !strings.Contains(overlay, rel) {
			t.Errorf("overlay does not name Go input %q", rel)
		}
		probe := strings.TrimSuffix(rel, "/")
		if strings.HasSuffix(rel, "/") {
			probe = rel + "x.go"
		}
		if !capsule.BinaryInputPath(probe) {
			t.Errorf("overlay names %q as a Go input but the executor does not classify %q as one", rel, probe)
		}
	}
	if !capsule.BinaryInputPath("frontend/tools/gen.go") || capsule.BinaryInputPath("internal/x/y_test.go") {
		t.Fatalf("executor classification drifted from the overlay's .go and _test.go rules")
	}
}

// Drill selfdev-ab-2239 (docs/problems/selfdev-freeze-receipts-refused-2026-10-15.md):
// for a frontend-only change the desk built Go seven times under the 30 s
// exec bound, never built the frontend, cited one receipt as both build and
// test, read Freeze's refusal as "build Go", and finished blocked without
// retrying. Failure modes the overlay must close:
//
//   - F1: the Go-input decision comes after "build bin/autoputer", so the
//     desk builds Go for a non-Go change; and the no-Go path has no build
//     recipe (the frontend build).
//   - F2: the three Freeze slots are not said to be three different cells.
//   - F5: the build cell leaves node_modules/dist in the workspace, or the
//     cited cells run before the last write.
//   - F6: commands assume /workspace/platform while choir.Exec runs at /;
//     the worked example's relative `go test` fails there.
//   - F7: no retry rule for a refused Freeze.
//   - F8: the background-build form waits on choir.Exec's pipes and dies
//     with the 30 s kill.
//   - F9: the overlay's exec bound drifts from the broker's.
func TestRLMEngineeringOverlayTeachesFreezeReceiptsAndExecLimits(t *testing.T) {
	overlay := RLMEngineeringOverlay(RLMEngineeringOverlayOptions{HasSelfDevelopmentOperation: true})

	decide, goBuild := strings.Index(overlay, "touches no Go input"), strings.Index(overlay, `"go", "build", "-o"`)
	if decide < 0 || goBuild < 0 || decide > goBuild {
		t.Errorf("F1: the Go-input decision does not come before the Go build instruction")
	}
	for _, want := range []string{
		"do not run go build",
		"npm ci", "npm run build", "cp -rf --no-preserve=mode,ownership,timestamps dist/. /" + capsule.ReleaseTreePrefix + "frontend/", "rm -rf node_modules dist",
	} {
		if !strings.Contains(overlay, want) {
			t.Errorf("F1/F5: the no-Go build recipe does not say %q", want)
		}
	}

	for _, want := range []string{"three different cells", "buildRecipeRef", "testReceipts", "dependencyToolchainRefs",
		"distinct build, test, and dependency/toolchain execution receipts are required"} {
		if !strings.Contains(overlay, want) {
			t.Errorf("F2: the Freeze evidence rule does not say %q", want)
		}
	}
	if !strings.Contains(overlay, "after your last write") {
		t.Errorf("F5: the overlay does not order the cited cells after the last write")
	}

	if !strings.Contains(overlay, "choir.Exec runs at the capsule root, /,") {
		t.Errorf("F6: the overlay does not say where choir.Exec runs")
	}
	if strings.Contains(overlay, `choir.Exec("go", []string{"test", "./`) {
		t.Errorf("F6: the worked example runs a relative go test from /")
	}
	if !strings.Contains(overlay, `choir.Exec("go", []string{"-C", "/workspace/platform"`) {
		t.Errorf("F6: the worked example does not name the module directory")
	}

	if !strings.Contains(overlay, "A refused Freeze is not a blocker") || !strings.Contains(overlay, "call choir.Freeze again") {
		t.Errorf("F7: the overlay has no retry rule for a refused Freeze")
	}

	// F8 is closed by capsule jobs (TestRLMEngineeringOverlayTeachesCapsuleJobs):
	// a long build is a job, never a backgrounded shell.
	if strings.Contains(overlay, "</dev/null >/dev/null 2>&1 &") {
		t.Errorf("F8: the overlay still teaches the detached shell build")
	}

	bound := fmt.Sprintf("killed after %d seconds", int(yaegikernel.DefaultExecTimeout/time.Second))
	if !strings.Contains(overlay, bound) || !strings.Contains(overlay, "exit code -1") {
		t.Errorf("F9: the overlay does not state the broker's exec bound (%q) and its exit code", bound)
	}
}

// Capsule jobs (docs/design/capsule-long-exec-2026-10-15.md): Go builds and
// tests outlast choir.Exec's 30 s, so the overlay teaches them as jobs.
// Failure modes the overlay must close:
//
//   - J1: a Go build or go test still runs through choir.Exec (killed at
//     30 s) or a backgrounded shell.
//   - J2: the desk polls a job in a loop instead of ending its turn.
//   - J3: the desk cites a failed or timed-out job's receipt, or does not
//     know a job's receipt is citable in Freeze's slots.
//   - J4: the desk starts two jobs at once, or Freezes while one runs.
//   - J5: the no-Go path changes (the frontend build stays one cell).
//   - J6: the overlay names a job option or verb the kernel does not have.
func TestRLMEngineeringOverlayTeachesCapsuleJobs(t *testing.T) {
	overlay := RLMEngineeringOverlay(RLMEngineeringOverlayOptions{HasSelfDevelopmentOperation: true})
	for _, want := range []string{
		`choir.Start([]string{"go", "build", "-o", "/var/lib/artifact/release/bin/autoputer"`,
		`choir.Start([]string{"go", "test"`,
		"never a choir.Exec and never a backgrounded shell",
	} {
		if !strings.Contains(overlay, want) {
			t.Errorf("J1: the overlay does not say %q", want)
		}
	}
	for _, want := range []string{"end your turn", "you are woken", "choir.Poll shows the job still running ends your turn"} {
		if !strings.Contains(overlay, want) {
			t.Errorf("J2: the overlay does not say %q", want)
		}
	}
	for _, want := range []string{"a job is one, with its own receipt_ref", "the bin/autoputer build job for a Go change",
		"a job whose state is failed, timeout or stopped", "during which the workspace changed"} {
		if !strings.Contains(overlay, want) {
			t.Errorf("J3: the overlay does not say %q", want)
		}
	}
	for _, want := range []string{"One job runs at a time", "Freeze is refused while a job runs"} {
		if !strings.Contains(overlay, want) {
			t.Errorf("J4: the overlay does not say %q", want)
		}
	}
	// J5 (docs/problems/engineering-completion-path-cluster-2026-10-20.md,
	// failure 4): the no-Go frontend build is a job. Its recipe ran
	// `npm ci && npm run build` inside one choir.Exec, which the overlay's
	// own text says is killed at 30 s; the visual drill followed the recipe
	// and hit the broker timeout three times.
	noGo := overlay[strings.Index(overlay, "touches no Go input"):strings.Index(overlay, "A change that touches a Go input")]
	if strings.Contains(noGo, "It fits within one choir.Exec") || !strings.Contains(noGo, "it is a job") ||
		!strings.Contains(noGo, "never run any part of it in a choir.Exec") {
		t.Errorf("J5: the no-Go build is not taught as a job: %q", noGo)
	}
	startedBuild := false
	for _, m := range shellScriptCalls.FindAllStringSubmatch(overlay, -1) {
		verb, script := m[1], m[2]
		if !strings.Contains(script, "npm ci") && !strings.Contains(script, "npm run build") {
			continue
		}
		if verb != "Start" {
			t.Errorf("J5: a recipe runs the npm build in choir.%s, not choir.Start: %q", verb, script)
			continue
		}
		startedBuild = true
		if !strings.Contains(overlay, `"minutes": 15`) {
			t.Errorf("J5: the npm build job names no minutes")
		}
		if !strings.Contains(script, "rm -rf node_modules dist") {
			t.Errorf("J5: the npm build job does not clean up after itself: %q", script)
		}
		// Failure 24: a job that failed before its cleanup left node_modules
		// and dist behind, so the next job changed the workspace while it ran
		// and Freeze refused its receipt. The cleanup runs on failure too.
		if !strings.Contains(script, "s=$?; rm -rf node_modules dist; exit $s") {
			t.Errorf("failure 24: the npm build job cleans up only when it succeeds: %q", script)
		}
	}
	if !startedBuild {
		t.Errorf("J5: no choir.Start recipe runs the npm build")
	}
	for _, want := range []string{"the job's own receipt_ref is the frontend build receipt you cite as buildRecipeRef",
		"the frontend build job for a change with no Go input", "a second job (one job at a time)"} {
		if !strings.Contains(overlay, want) {
			t.Errorf("J5: the overlay does not say %q", want)
		}
	}
	// Failures 6 and 7 of the cluster doc (S6-0 drill): a failed build was
	// reported "completed", and a running job was reported as a blocker.
	for _, want := range []string{
		"never report the build done, to management or in a Report, unless its job's State is done with exit code 0",
		"Never run the steps of this command as separate cells or jobs",
		"A running job is never a blocker: end your turn and wait to be woken",
		`do not Complete "blocked" because a build or test is still running`,
		"and neither is a job that is still running",
	} {
		if !strings.Contains(overlay, want) {
			t.Errorf("J7: the overlay does not say %q", want)
		}
	}
	for _, verb := range []string{"Start", "Poll", "Stop"} {
		if !strings.Contains(yaegikernel.DeskSurface("engineering", "implementation", false), "choir."+verb+"(") {
			t.Errorf("J6: the kernel documents no choir.%s", verb)
		}
	}
	for _, opt := range []string{`"cwd"`, `"minutes"`} {
		if !strings.Contains(overlay, opt) {
			t.Errorf("J6: the overlay does not name the option %s", opt)
		}
	}
	// Problem 3 of docs/evidence/selfdev-trace-reviews-2026-10-25.md: a job
	// run under `timeout 200` ran 271 s. The job's own wall clock is the
	// bound (capsulejob: SIGTERM to the job's group, then SIGKILL); the
	// overlay says so, and that a fraction of a minute is allowed.
	for _, want := range []string{
		"The timeout command does not work in the capsule",
		"0.5 is 30 seconds",
		"the job's whole process group is sent SIGTERM and then SIGKILL",
	} {
		if !strings.Contains(overlay, want) {
			t.Errorf("problem 3: the overlay does not say %q", want)
		}
	}
}

// Research searches the computer's own corpus before the web (symptom 3 of
// docs/problems/prompt-bar-research-slow-unsynthesized-mangled-2026-10-14.md):
// the overlay's evidence doctrine names choir.SearchWireCorpus before it
// teaches web search, and says a capture it returns is citable.
func TestRLMResearchOverlaySearchesTheCorpusFirst(t *testing.T) {
	overlay := RLMResearchOverlay()
	at := strings.Index(overlay, "Evidence Doctrine")
	if at < 0 {
		t.Fatalf("overlay has no evidence doctrine: %q", overlay)
	}
	doctrine := overlay[at:]
	corpus, web := strings.Index(doctrine, "choir.SearchWireCorpus("), strings.Index(doctrine, "choir.SearchStart")
	if corpus < 0 || web < 0 || corpus > web {
		t.Fatalf("evidence doctrine does not search the corpus before the web: %q", doctrine)
	}
	if !strings.Contains(doctrine, "capture_id") {
		t.Fatalf("evidence doctrine does not say a corpus capture is citable by its capture_id: %q", doctrine)
	}
}

// docs/design/claude-delegate-slice1-2026-10-14.md F18: the Claude delegate
// section, and the carve-out to "never start another agent", appear only in
// a claude-mode capsule's overlay; everywhere else the overlay keeps the
// ecosystem network text and the unqualified rule.
func TestRLMEngineeringOverlayShowsTheClaudeDelegateOnlyInClaudeMode(t *testing.T) {
	const rule = "Never start another agent or create an alternate candidate world."
	const carveOut = "Never start another agent besides the Claude delegate"
	plain := RLMEngineeringOverlay(RLMEngineeringOverlayOptions{HasSelfDevelopmentOperation: true})
	if strings.Contains(plain, "choir-delegate") || strings.Contains(plain, carveOut) || !strings.Contains(plain, rule) {
		t.Fatalf("plain overlay mentions the delegate or lost the rule: %q", plain)
	}
	if !strings.Contains(plain, "PyPI, npm, Go, crates") {
		t.Fatal("plain overlay lost the L2 network text")
	}
	claude := RLMEngineeringOverlay(RLMEngineeringOverlayOptions{HasSelfDevelopmentOperation: true, ClaudeDelegate: true})
	for _, want := range []string{
		carveOut, `choir.Exec("choir-delegate", []string{"start", "--spec-json"`, `"wait", "--id"`,
		"re-run the test in your own cell", "resume_session", "only the model API", "Precommit in your first cell", `"-newer", dir + "/spec.json"`,
		// F15: the delegate's cwd is the tree, the cells' is /.
		"written_files", "means /docs/x.md to you", "absolute /workspace/platform/",
	} {
		if !strings.Contains(claude, want) {
			t.Errorf("claude overlay lacks %q", want)
		}
	}
	if strings.Contains(claude, rule) || strings.Contains(claude, "PyPI, npm, Go, crates") {
		t.Fatal("claude overlay keeps the unqualified rule or the L2 network text")
	}
	if strings.Contains(claude, "{{") {
		t.Fatal("unrendered template in the claude overlay")
	}
}

// choir.System1 is the research desk's closed-question verb (owner ruling
// 2026-10-20: research gets it, management does not in this change; the verb
// is System1, not the name of the judge behind it). Research's overlay lists
// it with its contract: a distribution comes back, the desk decides, a flat
// one means read closely, evidence is by reference. No other desk's overlay
// teaches it, and none names the backing judge as a verb.
func TestRLMResearchOverlayListsSystem1WithItsContract(t *testing.T) {
	overlay := RLMResearchOverlay()
	at := strings.Index(overlay, "Research Cell Surface")
	if at < 0 {
		t.Fatalf("overlay has no cell surface: %q", overlay)
	}
	surface := overlay[at:]
	for _, want := range []string{"choir.System1(", "distribution", "you decide", "flat", "read the sources closely", "context_refs", "story:<revision_id>", "may be off on this computer"} {
		if !strings.Contains(surface, want) {
			t.Errorf("research overlay does not say %q about choir.System1", want)
		}
	}
	if strings.Contains(overlay, "choir.Jev") {
		t.Error("research overlay names a choir.Jev verb; the verb is choir.System1")
	}
}

// choir.RelateStories records a judged pair relation (the related_story edge).
// Research's overlay says what it needs: a System 1 receipt, a reason when it
// overrides, that story text is data and the verb may be off on this computer.
func TestRLMResearchOverlayListsRelateStoriesWithItsContract(t *testing.T) {
	overlay := RLMResearchOverlay()
	at := strings.Index(overlay, "Research Cell Surface")
	if at < 0 {
		t.Fatalf("overlay has no cell surface: %q", overlay)
	}
	surface := overlay[at:]
	for _, want := range []string{"choir.RelateStories(", "judgment_ref", "receipt_ref", "reason", "may be off on this computer", "publication_id"} {
		if !strings.Contains(surface, want) {
			t.Errorf("research overlay does not say %q about choir.RelateStories", want)
		}
	}
}

func TestOtherDesksOverlaysDoNotTeachSystem1(t *testing.T) {
	overlays := map[string]string{
		"management":      RLMManagementOverlay(RLMManagementOverlayOptions{IntakeLabels: true, EventWakes: true}),
		"management-off":  RLMManagementOverlay(),
		"management task": RLMManagementTaskOverlay(RLMManagementTaskOverlayOptions{Deadline: "10m"}),
		"engineering":     RLMEngineeringOverlay(RLMEngineeringOverlayOptions{HasSelfDevelopmentOperation: true}),
	}
	for desk, overlay := range overlays {
		for _, banned := range []string{"choir.System1", "choir.Jev", "choir.RelateStories"} {
			if strings.Contains(overlay, banned) {
				t.Errorf("%s overlay mentions %s, a verb it does not have", desk, banned)
			}
		}
	}
}
