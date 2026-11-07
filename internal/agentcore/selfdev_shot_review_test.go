package agentcore

import (
	"testing"
)

// The request-side checks of a plan's shots (selfdev_shot_review.go).

func TestShotSizes(t *testing.T) {
	for _, c := range []struct {
		w, h int
		ok   bool
	}{{1100, 750, true}, {1280, 800, true}, {1440, 900, true}, {2880, 1800, true}, {3510, 2532, false}, {2560, 1600, true}, {390, 844, true}, {1170, 2532, true}, {1024, 1536, false}, {1280, 2400, false}, {1100, 751, false}, {195, 422, false}, {0, 0, false}} {
		if got := shotSizeOK(c.w, c.h); got != c.ok {
			t.Fatalf("%dx%d = %v, want %v", c.w, c.h, got, c.ok)
		}
	}
}

// FM1, FM2: a failed call, a non-JSON answer, or a missed image is an
// error, never a pass.
func TestRequestedShotsParsesTheNumberedList(t *testing.T) {
	shots := requestedShots(libraryShotLine)
	if len(shots) != 12 {
		t.Fatalf("parsed %d shots: %+v", len(shots), shots)
	}
	if shots[0].Number != "01" || shots[0].Text != "aero-chrome-heart the package list" || shots[11].Number != "12" {
		t.Fatalf("ends = %+v, %+v", shots[0], shots[11])
	}
	for _, s := range shots {
		if want := s.Number == "11" || s.Number == "12"; s.Phone != want {
			t.Fatalf("shot %s phone = %v (%q)", s.Number, s.Phone, s.Text)
		}
	}
	for _, text := range []string{"", "Fix the 10 issues from 04:10Z", "01 only one; 02 two"} {
		if got := requestedShots(text); got != nil {
			t.Fatalf("%q parsed as a shot list: %+v", text, got)
		}
	}
}

const libraryShotLine = "Screenshots: scripts/choir-shot.mjs alone, `--size 1100x750` (11 and 12 at 390x844), `--contrast <selector>` for contrast. Take 01 alone first and look at it. Twelve PNGs in /var/lib/artifact/screens/: 01 aero-chrome-heart the package list; 02 london-salmon a search with results; 03 futuristic-noir an empty search; 04 carbon-fiber-kintsugi package detail with its release history; 05 frutiger-aero release detail (publisher, time, track and seq, changed paths, precommitments) with the gallery's larger view open on one screenshot; 06 vector-heart the \"Install <package> <release>?\" confirm; 07 liquid-metal the install outcome; 08 aero-chrome-heart the installed list with releases marked \"Installed\"; 09 london-salmon a refused install shown with its reason; 10 futuristic-noir the error state (catalog unreachable) with Try again; 11 390 aero-chrome-heart the list on the phone; 12 390 futuristic-noir release detail on the phone, for a release with no screenshots, showing \"No screenshots yet\".\nThe owner wrote this at 04:10Z: 10 more later."
