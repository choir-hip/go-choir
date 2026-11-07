package agentcore

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
)

// The request-side checks of a plan's shots (the numbered shots the owner's
// words name, the recorded sizes). The platform's own vision review is gone:
// management looks at the images with its own model and records
// choir.ShotVerdict (desk_shots.go, docs/design/desk-shots-and-review-2026-11-07.md).
// What stays here is what needs no model: it is code, not prompt text.

// shotReviewMarker opens the reason of a verdict a request-side check failed.
const shotReviewMarker = "shot review"

func truncateRunesShot(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}

// shotSizeOK is question 5: desktop 1100x750, 1280x800 or 1440x900 (the
// platform shot driver's desktop viewport, internal/shot) (at device pixel
// ratio 1, 2 or 3), phone 390x844 at any DPR.
func shotSizeOK(width, height int) bool {
	if width <= 0 || height <= 0 {
		return false
	}
	for _, size := range [][2]int{{1100, 750}, {1280, 800}, {1440, 900}} {
		for dpr := 1; dpr <= 3; dpr++ {
			if width == size[0]*dpr && height == size[1]*dpr {
				return true
			}
		}
	}
	// Any DPR, never below 1 (a 195x422 shot is not a phone screen).
	return width >= 390 && width*844 == height*390
}

// requestedShot is one numbered shot the request names ("05 frutiger-aero
// release detail ...", "11 390 aero-chrome-heart the list on the phone").
type requestedShot struct {
	Number string
	Text   string
	Phone  bool
}

var (
	// shotNumberRe finds a two-digit number that starts a word and is
	// followed by a space: "; 05 frutiger-aero", ": 01 aero", "\n12 ...".
	shotNumberRe = regexp.MustCompile(`(?:^|[\s;:,(])(\d{2})[.)]?\s`)
	phoneWordRe  = regexp.MustCompile(`(?i)\bphone\b|\b390\b|390x844|\bmobile\b`)
)

// requestedShots parses the request's numbered shot list: the longest run
// 01, 02, ... in text order (a later 01 wins a tie, so "Take 01 alone"
// before the list does not start it). Each shot's text runs to the next
// number, ';' or line end. A run shorter than three is not a shot list.
func requestedShots(text string) []requestedShot {
	matches := shotNumberRe.FindAllStringSubmatchIndex(text, -1)
	type hit struct {
		n          int
		start, end int // the number's digits; end is where its text starts
	}
	var hits []hit
	for _, m := range matches {
		n := int(text[m[2]]-'0')*10 + int(text[m[2]+1]-'0')
		hits = append(hits, hit{n: n, start: m[2], end: m[3]})
	}
	var best []int
	for i, h := range hits {
		if h.n != 1 {
			continue
		}
		chain := []int{i}
		for j := i + 1; j < len(hits); j++ {
			if hits[j].n == hits[chain[len(chain)-1]].n+1 {
				chain = append(chain, j)
			}
		}
		if len(chain) >= len(best) {
			best = chain
		}
	}
	if len(best) < 3 {
		return nil
	}
	out := make([]requestedShot, 0, len(best))
	for _, i := range best {
		end := len(text)
		if i+1 < len(hits) {
			end = hits[i+1].start
		}
		item := text[hits[i].end:end]
		if cut := strings.IndexAny(item, ";\n"); cut >= 0 {
			item = item[:cut]
		}
		item = strings.TrimSpace(strings.TrimLeft(item, ".) "))
		out = append(out, requestedShot{Number: fmt.Sprintf("%02d", hits[i].n), Text: item, Phone: phoneWordRe.MatchString(item)})
	}
	return out
}

// shotNumberOf is the two-digit number a screen's name starts with, or "".
func shotNumberOf(name string) string {
	if len(name) >= 3 && name[0] >= '0' && name[0] <= '9' && name[1] >= '0' && name[1] <= '9' && (name[2] < '0' || name[2] > '9') {
		return name[:2]
	}
	return ""
}

// planShotsMissing lists the numbered shots the request names that the
// plan has no view for. A plan view's number is its 1-based position (the
// NN of NN-before-<view>.png). No model call; nil when the request names no
// numbered list.
func planShotsMissing(plan *capsule.ShotPlan, wanted []requestedShot) []string {
	if plan == nil {
		return nil
	}
	var missing []string
	for _, w := range wanted {
		n := 0
		fmt.Sscanf(w.Number, "%d", &n)
		if n < 1 || n > len(plan.Views) {
			missing = append(missing, w.Number)
		}
	}
	return missing
}

// planVerifyVerdict is what Verify checks of a plan bundle's shots, with no
// model call: the request's numbered shots against the plan's views. reason is "" when
// nothing is missing.
func planVerifyVerdict(plan *capsule.ShotPlan, request string) (missing []string, reason string) {
	missing = planShotsMissing(plan, requestedShots(request))
	if len(missing) > 0 {
		reason = shotReviewMarker + " rejected missing shots the request names: " + strings.Join(missing, ", ") + "; the shot plan has no such view"
	}
	return missing, reason
}

// verifyShotRefusal is why Verify will not pass bundle on its shots, or "": a
// platform bundle that carries screens and no plan (platform shots are taken
// after the host build, and judged by management's own ShotVerdict) and a plan
// with more views than management can be shown (a request carries at most 20
// images and they persist in run memory). A package bundle with screens is
// judged by management's Verify, with per-screen findings.
func verifyShotRefusal(operation selfdev.Operation, bundle transaction.CapsuleEffectBundle) string {
	if len(bundle.Screens) > 0 && !operation.IsPackage() {
		return "legacy screens bundle; shots are taken after the host build"
	}
	if bundle.ShotPlan != nil && len(bundle.ShotPlan.Views) > shotMaxViews {
		return fmt.Sprintf("the shot plan has %d views; management judges at most %d (20 images), so cut the plan", len(bundle.ShotPlan.Views), shotMaxViews)
	}
	return ""
}
