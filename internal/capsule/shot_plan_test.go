package capsule

import (
	"testing"
)

func goodPlan() ShotPlan {
	return ShotPlan{Views: []ShotView{
		{Name: "01-before-home", Path: "/", Viewport: "desktop"},
		{Name: "02-after-home", Path: "/", Viewport: "phone", Theme: "dark", Steps: []ShotStep{{Op: "click", Selector: "#b"}}},
	}}
}

func TestValidate(t *testing.T) {
	if err := goodPlan().Validate(); err != nil {
		t.Fatal(err)
	}
	mut := func(f func(p *ShotPlan)) error {
		p := goodPlan()
		f(&p)
		return p.Validate()
	}
	bad := map[string]func(p *ShotPlan){
		"no views":       func(p *ShotPlan) { p.Views = nil },
		"dup name":       func(p *ShotPlan) { p.Views[1].Name = p.Views[0].Name },
		"slash name":     func(p *ShotPlan) { p.Views[0].Name = "../x" },
		"png suffix":     func(p *ShotPlan) { p.Views[0].Name = "a.png" },
		"empty name":     func(p *ShotPlan) { p.Views[0].Name = "" },
		"relative path":  func(p *ShotPlan) { p.Views[0].Path = "x" },
		"other origin":   func(p *ShotPlan) { p.Views[0].Path = "//evil.example/" },
		"viewport":       func(p *ShotPlan) { p.Views[0].Viewport = "tablet" },
		"theme":          func(p *ShotPlan) { p.Views[0].Theme = "sepia" },
		"unknown op":     func(p *ShotPlan) { p.Views[0].Steps = []ShotStep{{Op: "eval", Text: "1"}} },
		"click no sel":   func(p *ShotPlan) { p.Views[0].Steps = []ShotStep{{Op: "click"}} },
		"wait too long":  func(p *ShotPlan) { p.Views[0].Steps = []ShotStep{{Op: "wait_ms", Ms: 5001}} },
		"wait zero":      func(p *ShotPlan) { p.Views[0].Steps = []ShotStep{{Op: "wait_ms"}} },
		"bad key":        func(p *ShotPlan) { p.Views[0].Steps = []ShotStep{{Op: "press", Text: "Ctrl+Q"}} },
		"bad scroll":     func(p *ShotPlan) { p.Views[0].Steps = []ShotStep{{Op: "scroll", Text: "middle"}} },
		"video no view":  func(p *ShotPlan) { p.Video = &ShotVideo{View: "nope", Seconds: 3} },
		"video too long": func(p *ShotPlan) { p.Video = &ShotVideo{View: "01-before-home", Seconds: 21} },
		"video zero":     func(p *ShotPlan) { p.Video = &ShotVideo{View: "01-before-home"} },
		"too many steps": func(p *ShotPlan) {
			p.Views[0].Steps = make([]ShotStep, MaxShotSteps+1)
			for i := range p.Views[0].Steps {
				p.Views[0].Steps[i] = ShotStep{Op: "wait_ms", Ms: 1}
			}
		},
		"too many views": func(p *ShotPlan) {
			p.Views = nil
			for i := 0; i <= MaxScreens; i++ {
				p.Views = append(p.Views, ShotView{Name: "v" + string(rune('a'+i)), Path: "/", Viewport: "desktop"})
			}
		},
	}
	for name, f := range bad {
		if err := mut(f); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
	if err := mut(func(p *ShotPlan) { p.Video = &ShotVideo{View: "02-after-home", Seconds: 20} }); err != nil {
		t.Errorf("max video: %v", err)
	}
}

func TestDigestStable(t *testing.T) {
	a, err := goodPlan().Digest()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := goodPlan().Digest()
	if a != b || len(a) != 64 {
		t.Fatalf("digest unstable: %q %q", a, b)
	}
	p := goodPlan()
	p.Views[1].Theme = "light"
	if c, _ := p.Digest(); c == a {
		t.Fatal("digest ignores the theme")
	}
}
