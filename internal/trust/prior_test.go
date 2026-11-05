package trust

// Failure modes (MA7.3), written first:
//  P1 a new version is a blank slate, or inherits its package's whole record
//     after a rewrite: the discount must be computed from the surface parts.
//  P2 a new publisher is trusted by default: it must start from the platform
//     base shrunk to n0 pseudo-counts, and a good publisher lends more.
//  P3 a version's prior double counts its own evidence: its own record starts
//     at install; earlier evidence reaches it only through the discounted prior.
//  P4 the discount grows without bound with many small changes: capped at 0.9.

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/apppackage"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func parts(over map[string]string) map[string]string {
	m := map[string]string{}
	for _, k := range []string{apppackage.PartPrompt, apppackage.PartCode, apppackage.PartSchedules, apppackage.PartAccepts, apppackage.PartKinds, apppackage.PartGrants, apppackage.PartApp, apppackage.PartAssets} {
		m[k] = "h-" + k
	}
	for k, v := range over {
		m[k] = v
	}
	return m
}

func TestChangeDiscountIsComputedFromTheSurfaceParts(t *testing.T) {
	base := parts(nil)
	cases := []struct {
		name string
		cur  map[string]string
		want float64
	}{
		{"unchanged", parts(nil), 0},
		{"assets", parts(map[string]string{"assets": "x"}), 0.1},
		{"window", parts(map[string]string{"app": "x"}), 0.3},
		{"schedule", parts(map[string]string{"schedules": "x"}), 0.5},
		{"kinds", parts(map[string]string{"kinds": "x"}), 0.7},
		{"prompt", parts(map[string]string{"prompt": "x"}), 0.9},
		{"code", parts(map[string]string{"code": "x"}), 0.9},
		{"grants", parts(map[string]string{"grants": "x"}), 0.9},
		{"many small changes are capped", parts(map[string]string{"assets": "x", "app": "x", "schedules": "x", "kinds": "x", "accepts": "x"}), 0.9},
	}
	for _, tc := range cases {
		got := ChangeDiscount(base, tc.cur)
		if math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("%s: d = %v, want %v", tc.name, got, tc.want)
		}
	}
	// a part added where there was none counts as a change
	cur := parts(nil)
	delete(base, "kinds")
	if d := ChangeDiscount(base, cur); d != 0.7 {
		t.Errorf("added part: %v", d)
	}
	// two small changes together exceed either alone (noisy-or) but not 0.9
	two := ChangeDiscount(parts(nil), parts(map[string]string{"assets": "x", "app": "x"}))
	if !(two > 0.3 && two < 0.9) {
		t.Errorf("assets+app = %v", two)
	}
}

func TestCarryKeepsTheMeanAndScalesTheCount(t *testing.T) {
	a, b := CarryPrior(18, 2, 0.5) // mean .9, n 20 -> n 10
	if math.Abs(a/(a+b)-0.9) > 1e-9 || math.Abs(a+b-10) > 1e-9 {
		t.Fatalf("%v %v", a, b)
	}
	a, b = CarryPrior(18, 2, 0.9) // n 2 -> floored at the minimum prior count
	if a+b < MinPriorCount-1e-9 {
		t.Fatalf("floor: %v", a+b)
	}
}

// publishers: good has three clean packages; bad fails; stranger is new.
func hierarchyFixture(extra func(*Inputs)) (Inputs, Hierarchy) {
	var recs []types.CommitmentRecord
	add := func(desk, outcome string, i int) {
		id := fmt.Sprintf("%s-%d", desk, i)
		at := time.Duration(i) * time.Hour
		recs = append(recs, subCast(id, desk, "research_child", "m", at, types.ImpactI2, 0.5, yesNo(0.9)), judge(id, at+time.Minute, outcome))
	}
	for _, d := range []string{"good1", "good2", "good3"} {
		for i := 0; i < 20; i++ {
			add(d, "yes", i)
		}
	}
	for i := 0; i < 20; i++ {
		o := "yes"
		if i%2 == 0 {
			o = "no"
		}
		add("bad1", o, i)
	}
	add("newgood", "yes", 30)
	add("newstranger", "yes", 30)
	pub := map[string]string{"good1": "good-pub", "good2": "good-pub", "good3": "good-pub", "newgood": "good-pub", "bad1": "bad-pub", "newstranger": "stranger-pub"}
	pkg := map[string]string{"good1": "g1", "good2": "g2", "good3": "g3", "newgood": "g-new", "bad1": "b1", "newstranger": "s-new"}
	in := Inputs{Records: recs, Resolver: func(d string) SubjectInfo { return SubjectInfo{Package: pkg[d], Publisher: pub[d]} }}
	if extra != nil {
		extra(&in)
	}
	return in, DefaultHierarchy()
}

func TestNewPublisherStartsFromPlatformBaseAndGoodPublisherLendsMore(t *testing.T) {
	in, h := hierarchyFixture(nil)
	p := baseParams()
	v := DeriveHierarchical(in, p, h)
	cls := ActionClass{Grant: "desk.request", Impact: types.ImpactI2}
	good, ok1 := row(v, SubjectAgent, "newgood", cls)
	stranger, ok2 := row(v, SubjectAgent, "newstranger", cls)
	if !ok1 || !ok2 {
		t.Fatalf("rows: %+v", v.Rows)
	}
	gm, sm := good.Alpha0/(good.Alpha0+good.Beta0), stranger.Alpha0/(stranger.Alpha0+stranger.Beta0)
	if !(gm > sm) {
		t.Fatalf("a publisher with a clean record must lend a stronger prior: good %v stranger %v", gm, sm)
	}
	// both are shrunk: no more pseudo-counts than n0 of the version level
	for _, r := range []Row{good, stranger} {
		if n := r.Alpha0 + r.Beta0; math.Abs(n-h.N0Version) > 1e-9 {
			t.Errorf("%s prior count %v, want n0 %v", r.Subject.ID, n, h.N0Version)
		}
	}
	// the stranger's publisher record is the platform's, not trusted by default
	sp, ok := row(v, SubjectPublisher, "stranger-pub", cls)
	if !ok || math.Abs(sp.Alpha0+sp.Beta0-h.N0Publisher) > 1e-9 {
		t.Fatalf("stranger publisher prior: %+v", sp)
	}
	// a flat (non-hierarchical) view gives both the same blank prior
	flat := Derive(in, p)
	fg, _ := row(flat, SubjectAgent, "newgood", cls)
	fs, _ := row(flat, SubjectAgent, "newstranger", cls)
	if fg.Alpha0 != fs.Alpha0 || fg.Beta0 != fs.Beta0 {
		t.Fatal("the flat view is the contrast")
	}
	// I4 for a stranger stays low: the platform base for I4 is deliberately low
	a4, b4 := PlatformBase(ActionClass{Grant: "desk.capsule.mutate", Impact: types.ImpactI4})
	if a4/(a4+b4) >= 0.5 {
		t.Fatalf("I4 base %v/%v", a4, b4)
	}
}

func TestVersionCarriesItsPackageRecordDiscountedByChange(t *testing.T) {
	cls := ActionClass{Grant: "desk.request", Impact: types.ImpactI2}
	since := t0.Add(24 * time.Hour) // the new version installs after day one
	carried := func(cur map[string]string) Row {
		in, h := hierarchyFixture(func(in *Inputs) {
			// good1 shipped a new version at `since`; two more settled acts after it
			for i := 0; i < 2; i++ {
				id := fmt.Sprintf("good1-new-%d", i)
				at := 30*time.Hour + time.Duration(i)*time.Hour
				in.Records = append(in.Records, subCast(id, "good1", "research_child", "m", at, types.ImpactI2, 0.5, yesNo(0.9)), judge(id, at+time.Minute, "yes"))
			}
		})
		h.Lineage = func(desk string) (Lineage, bool) {
			if desk != "good1" {
				return Lineage{}, false
			}
			return Lineage{Since: since, Previous: parts(nil), Current: cur}, true
		}
		v := DeriveHierarchical(in, baseParams(), h)
		r, ok := row(v, SubjectAgent, "good1", cls)
		if !ok {
			t.Fatalf("no row: %+v", v.Rows)
		}
		return r
	}
	assets := carried(parts(map[string]string{"assets": "x"}))
	prompt := carried(parts(map[string]string{"prompt": "x"}))
	if !(assets.Alpha0+assets.Beta0 > prompt.Alpha0+prompt.Beta0) {
		t.Fatalf("an asset change keeps more of the record than a prompt rewrite: %v vs %v", assets.Alpha0+assets.Beta0, prompt.Alpha0+prompt.Beta0)
	}
	// the mean is kept, the count is scaled
	if math.Abs(assets.Alpha0/(assets.Alpha0+assets.Beta0)-prompt.Alpha0/(prompt.Alpha0+prompt.Beta0)) > 1e-9 {
		t.Fatal("carry keeps the mean")
	}
	// the version's own record starts at install: only the two new acts count
	if assets.N != 2 {
		t.Fatalf("own evidence since install: n = %d", assets.N)
	}
	// unchanged surface carries everything the package had
	same := carried(parts(nil))
	if !(same.Alpha0+same.Beta0 > assets.Alpha0+assets.Beta0) {
		t.Fatalf("an unchanged surface carries the most: %v vs %v", same.Alpha0+same.Beta0, assets.Alpha0+assets.Beta0)
	}
}
