package capsule

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// Failure modes, decided before the code
// (docs/design/source-only-freeze-and-candidate-shots-2026-11-06.md, 3.2, 4):
//
//   - A plan is data from a model or an owner, so it is validated before a
//     browser starts: bounded views, steps, waits and video seconds, unique
//     valid file names, known ops. Nothing in a plan is code or a URL; Path
//     is joined to the BaseURL and must stay on that origin.
//   - The browser never shoots the network: the driver only dials BaseURL
//     (the caller's loopback origin); the sandbox that enforces it is the
//     caller's (FM3).
//   - A failed step, a timeout, a missing browser or a missing ffmpeg is an
//     error naming the view and step. Run never returns a partial set as
//     success, and it removes the files it wrote when it fails.
//   - A hung browser: every view, the whole plan, the screencast encode and
//     the browser process have a deadline; cancelling the context kills the
//     process through chromedp's allocator.
//   - Before and after runs must differ only in the code under test (FM13):
//     one launcher with fixed flags, fixed viewports, DPR and fonts
//     (FONTCONFIG_FILE passes through) serves both, and the plan digest is
//     the one sha both runs record (FM5).
//   - Video is bounded: at most MaxShotVideoSeconds, frames go to a scratch
//     directory (not encoded during capture), the WebM must pass the
//     capsule video checks (size cap, EBML header) or Run fails.
//   - A phone view at DPR 3 makes big PNGs; each PNG is checked against
//     MaxScreenBytes and the pixel bound before it counts.

const (
	// MaxShotSteps bounds one view's steps.
	MaxShotSteps = 20
	// MaxShotStepMs bounds one wait_ms (and a wait_for) step.
	MaxShotStepMs = 5000
	// MaxShotPathBytes bounds a view's path.
	MaxShotPathBytes = 2048
	// MaxShotVideoSeconds bounds the recorded video.
	MaxShotVideoSeconds = 20

	ShotViewportDesktop = "desktop"
	ShotViewportPhone   = "phone"

	ShotThemeLight = "light"
	ShotThemeDark  = "dark"
)

// Step ops.
const (
	ShotOpClick   = "click"
	ShotOpFill    = "fill"
	ShotOpPress   = "press"
	ShotOpWaitFor = "wait_for"
	ShotOpWaitMs  = "wait_ms"
	ShotOpScroll  = "scroll"
)

// ShotStep is one scripted action in a view.
type ShotStep struct {
	Op       string `json:"op"`
	Selector string `json:"selector,omitempty"`
	Text     string `json:"text,omitempty"`
	Ms       int    `json:"ms,omitempty"`
}

// ShotView is one screenshot: a page, a viewport, a theme and the steps before the capture.
type ShotView struct {
	// Name is the file stem; the PNG is <Name>.png (ValidScreenName).
	Name     string     `json:"name"`
	Path     string     `json:"path"`
	Viewport string     `json:"viewport"`
	Theme    string     `json:"theme,omitempty"`
	Steps    []ShotStep `json:"steps,omitempty"`
}

// ShotVideo records one view: from page load through its steps, and at least Seconds.
type ShotVideo struct {
	View    string `json:"view"`
	Seconds int    `json:"seconds"`
}

// ShotPlan is the whole plan.
type ShotPlan struct {
	Views []ShotView `json:"views"`
	Video *ShotVideo `json:"video,omitempty"`
}

// Validate bounds a plan before any browser starts.
func (p ShotPlan) Validate() error {
	if len(p.Views) == 0 {
		return fmt.Errorf("shot plan: no views")
	}
	if len(p.Views) > MaxScreens {
		return fmt.Errorf("shot plan: %d views; at most %d", len(p.Views), MaxScreens)
	}
	seen := map[string]bool{}
	for i, v := range p.Views {
		if !ValidScreenName(v.Name+".png") || strings.HasSuffix(strings.ToLower(v.Name), ".png") {
			return fmt.Errorf("shot plan: view %d name %q is not a plain screen file stem", i, v.Name)
		}
		if seen[strings.ToLower(v.Name)] {
			return fmt.Errorf("shot plan: view name %q is used twice", v.Name)
		}
		seen[strings.ToLower(v.Name)] = true
		if len(v.Path) > MaxShotPathBytes || !strings.HasPrefix(v.Path, "/") || strings.HasPrefix(v.Path, "//") || strings.ContainsAny(v.Path, "\\\r\n\x00") {
			return fmt.Errorf("shot plan: view %s path %q must be an absolute path on the base origin", v.Name, v.Path)
		}
		switch v.Viewport {
		case ShotViewportDesktop, ShotViewportPhone:
		default:
			return fmt.Errorf("shot plan: view %s viewport %q is not desktop or phone", v.Name, v.Viewport)
		}
		switch v.Theme {
		case "", ShotThemeLight, ShotThemeDark:
		default:
			return fmt.Errorf("shot plan: view %s theme %q is not light, dark or empty", v.Name, v.Theme)
		}
		if len(v.Steps) > MaxShotSteps {
			return fmt.Errorf("shot plan: view %s has %d steps; at most %d", v.Name, len(v.Steps), MaxShotSteps)
		}
		for j, s := range v.Steps {
			if err := s.validate(); err != nil {
				return fmt.Errorf("shot plan: view %s step %d: %w", v.Name, j, err)
			}
		}
	}
	if vd := p.Video; vd != nil {
		if !seen[strings.ToLower(vd.View)] {
			return fmt.Errorf("shot plan: video view %q is not a view", vd.View)
		}
		if vd.Seconds < 1 || vd.Seconds > MaxShotVideoSeconds {
			return fmt.Errorf("shot plan: video seconds %d out of 1..%d", vd.Seconds, MaxShotVideoSeconds)
		}
		if !ValidVideoName(vd.FileName()) {
			return fmt.Errorf("shot plan: video name %q is not a plain .webm stem", vd.FileName())
		}
	}
	return nil
}

// FileName is the video file: <View>.webm.
func (v ShotVideo) FileName() string {
	return v.View + ".webm"
}

func (s ShotStep) validate() error {
	if s.Ms < 0 || s.Ms > MaxShotStepMs {
		return fmt.Errorf("ms %d out of 0..%d", s.Ms, MaxShotStepMs)
	}
	if len(s.Selector) > 512 || len(s.Text) > 2048 {
		return fmt.Errorf("selector or text too long")
	}
	switch s.Op {
	case ShotOpClick, ShotOpWaitFor, ShotOpFill:
		if s.Selector == "" {
			return fmt.Errorf("%s needs a selector", s.Op)
		}
	case ShotOpPress:
		if _, ok := validShotKey(s.Text); !ok {
			return fmt.Errorf("press key %q is not one of %s", s.Text, shotKeyList)
		}
	case ShotOpWaitMs:
		if s.Ms <= 0 {
			return fmt.Errorf("wait_ms needs ms in 1..%d", MaxShotStepMs)
		}
	case ShotOpScroll:
		if s.Selector == "" && s.Text != "top" && s.Text != "bottom" {
			return fmt.Errorf("scroll needs a selector or text top|bottom")
		}
	default:
		return fmt.Errorf("unknown op %q", s.Op)
	}
	return nil
}

// ShotKeys is the one list of key names a press step may use; internal/shot
// maps each to a browser key and a test keeps the two in step.
var ShotKeys = []string{"ArrowDown", "ArrowLeft", "ArrowRight", "ArrowUp", "Backspace", "End", "Enter", "Escape", "Home", "PageDown", "PageUp", "Tab"}

var shotKeyList = strings.Join(ShotKeys, ", ")

func validShotKey(k string) (string, bool) {
	for _, n := range ShotKeys {
		if n == k {
			return k, true
		}
	}
	return "", false
}

// Timeout is how long a step may wait.
func (s ShotStep) Timeout() time.Duration {
	if s.Ms > 0 {
		return time.Duration(s.Ms) * time.Millisecond
	}
	return MaxShotStepMs * time.Millisecond
}

// Digest is the hex sha256 of the plan's canonical JSON: the one sha both the
// before and the after run record.
func (p ShotPlan) Digest() (string, error) {
	if p.Views == nil {
		p.Views = []ShotView{}
	}
	raw, err := computerevent.CanonicalJSON(p)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
