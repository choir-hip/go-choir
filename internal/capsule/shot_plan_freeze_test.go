package capsule

import (
	"strings"
	"testing"
)

// The shot plan is data the engineer writes and the platform trusts to
// drive a browser: every bound is checked, nothing is clamped (FM5, FM6).
// Failure modes: an out-of-range plan is admitted; two views collide on one
// screen name; a view name cannot pair as NN-before-/NN-after-; the digest
// changes with key order or does not change with content; a plan satisfies
// the Freeze rule for a package candidate.

func soOkPlan() ShotPlan {
	return ShotPlan{Views: []ShotView{
		{Name: "home", Path: "/", Viewport: "desktop", Theme: "dark", Steps: []ShotStep{{Op: "click", Selector: "#a"}, {Op: "wait_ms", Ms: 200}}},
		{Name: "phone-home", Path: "/files", Viewport: "phone"},
	}, Video: &ShotVideo{View: "home", Seconds: 10}}
}

func TestShotPlanValidateBoundsUnion(t *testing.T) {
	if err := soOkPlan().Validate(); err != nil {
		t.Fatalf("a good plan: %v", err)
	}
	mutate := map[string]func(p *ShotPlan){
		"no views":          func(p *ShotPlan) { p.Views = nil },
		"too many views":    func(p *ShotPlan) { p.Views = make([]ShotView, MaxScreens+1) },
		"duplicate name":    func(p *ShotPlan) { p.Views[1].Name = "Home" },
		"bad name":          func(p *ShotPlan) { p.Views[0].Name = "../x" },
		"empty name":        func(p *ShotPlan) { p.Views[0].Name = "" },
		"relative path":     func(p *ShotPlan) { p.Views[0].Path = "files" },
		"scheme-relative":   func(p *ShotPlan) { p.Views[0].Path = "//evil.example/" },
		"bad viewport":      func(p *ShotPlan) { p.Views[0].Viewport = "tablet" },
		"bad theme":         func(p *ShotPlan) { p.Views[0].Theme = "sepia" },
		"unknown op":        func(p *ShotPlan) { p.Views[0].Steps[0].Op = "eval" },
		"click no selector": func(p *ShotPlan) { p.Views[0].Steps[0].Selector = "" },
		"wait too long":     func(p *ShotPlan) { p.Views[0].Steps[1].Ms = MaxShotStepMs + 1 },
		"negative ms":       func(p *ShotPlan) { p.Views[0].Steps[1].Ms = -1 },
		"too many steps":    func(p *ShotPlan) { p.Views[0].Steps = make([]ShotStep, MaxShotSteps+1) },
		"video unknown":     func(p *ShotPlan) { p.Video.View = "nope" },
		"video too long":    func(p *ShotPlan) { p.Video.Seconds = MaxShotVideoSeconds + 1 },
		"video zero":        func(p *ShotPlan) { p.Video.Seconds = 0 },
	}
	for name, fn := range mutate {
		p := soOkPlan()
		fn(&p)
		if err := p.Validate(); err == nil {
			t.Errorf("%s: plan validated", name)
		}
	}
}

func TestShotPlanDigestIsContentBoundUnion(t *testing.T) {
	a, err := soOkPlan().Digest()
	if err != nil || len(a) != 64 {
		t.Fatalf("digest %q %v", a, err)
	}
	p := soOkPlan()
	p.Views[0].Path = "/other"
	if b, _ := p.Digest(); b == a {
		t.Fatal("the digest ignores the path")
	}
}

func TestDecodeShotPlanIsStrict(t *testing.T) {
	good := `{"views":[{"name":"home","path":"/","viewport":"phone"}]}`
	if _, err := DecodeShotPlan([]byte(good)); err != nil {
		t.Fatal(err)
	}
	for name, raw := range map[string]string{
		"unknown field": `{"views":[{"name":"home","path":"/","viewport":"phone","js":"x"}]}`,
		"trailing":      good + `{}`,
		"not json":      `views`,
		"invalid plan":  `{"views":[]}`,
	} {
		if _, err := DecodeShotPlan([]byte(raw)); err == nil {
			t.Errorf("%s: decoded", name)
		}
	}
	if _, err := DecodeShotPlan([]byte(strings.Repeat(" ", MaxShotPlanBytes+1))); err == nil {
		t.Error("an oversize plan decoded")
	}
}

// A valid plan satisfies screens_required and before_after_required for a
// platform change (design 3.3, Freeze rule); without it the refusals stand.
func TestRequireScreensOrPlan(t *testing.T) {
	goChange := []FileChange{{Path: "/workspace/platform/internal/agentcore/x.go", Kind: ChangeModified}}
	if err := requireScreensOrPlan(goChange, nil, nil, nil); err == nil || !strings.Contains(err.Error(), ScreensRequiredReason) {
		t.Fatalf("a Go change with neither screens nor a plan: %v", err)
	}
	plan := soOkPlan()
	if err := requireScreensOrPlan(goChange, nil, nil, &plan); err != nil {
		t.Fatalf("a valid plan must satisfy screens_required and before_after_required: %v", err)
	}
	bad := ShotPlan{}
	if err := requireScreensOrPlan(goChange, nil, nil, &bad); err == nil {
		t.Fatal("an invalid plan satisfied the rule")
	}
	// Exempt-only changes need neither, as before.
	docs := []FileChange{{Path: "/workspace/platform/docs/a.md", Kind: ChangeModified}}
	if err := requireScreensOrPlan(docs, nil, nil, nil); err != nil {
		t.Fatalf("an all-exempt change: %v", err)
	}
}

func TestPlatformSourceTouchedAndScreenRelevantPaths(t *testing.T) {
	if PlatformSourceTouched(nil) {
		t.Fatal("no changes touch the platform")
	}
	if PlatformSourceTouched([]FileChange{{Path: "/var/lib/artifact/shot-plan.json"}, {Path: "/tmp/x"}}) {
		t.Fatal("artifacts and /tmp are not platform source")
	}
	if PlatformSourceTouched([]FileChange{{Path: "/workspace/platform/frontend/node_modules/x/y.js"}, {Path: "/workspace/platform/.git/index"}}) {
		t.Fatal("node_modules and .git are not carried source (FM10)")
	}
	if !PlatformSourceTouched([]FileChange{{Path: "/workspace/platform/internal/x/x.go"}}) {
		t.Fatal("a Go file under the platform is source")
	}
	got := ScreenRelevantPaths([]FileChange{{Path: "workspace/platform/internal/x/x.go"}, {Path: "workspace/platform/docs/a.md"}, {Path: "workspace/platform/internal/x/x_test.go"}})
	if len(got) != 1 || got[0] != "internal/x/x.go" {
		t.Fatalf("relevant paths = %v", got)
	}
}
