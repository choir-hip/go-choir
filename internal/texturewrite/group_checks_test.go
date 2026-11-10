package texturewrite

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/quotematch"
)

// Side-by-side cites are one group (docs/problems/unsupported-cites-
// duplicates-and-churn-2026-11-10.md, fix A): a sentence carrying several
// cites is judged once with all its quotes as evidence. Ways it can fail:
// G1 a group of 2, 3 or 4+ cites is not listed as one group on every member;
// G2 a lone cite gets a Group (it must stay span-keyed and memoized as
// before); G3 cites separated by words wrongly join one group; G4 the same
// quote cited twice side by side counts as two pieces of evidence or two
// pairs; G5 a group changed by one cite keeps its old keys, so a verdict for
// the old group answers the new one; G6 an unchanged group is re-keyed by an
// edit to another sentence; G7 a group larger than maxGroupCites is carried
// whole into one question.

func groupFixture(n int) (*fakeCaptures, string) {
	caps := &fakeCaptures{captures: map[string]Capture{}}
	var cites []string
	for i := 1; i <= n; i++ {
		url := fmt.Sprintf("https://example.org/s%d", i)
		caps.captures[url] = Capture{
			Doc: fmt.Sprintf("cap-s%d", i), Version: "v1", URL: url, Title: fmt.Sprintf("Outlet %d", i),
			Blocks: []quotematch.Block{{ID: "b1", Text: fmt.Sprintf("Outlet %d reports the harbour board approved the plan.", i)}},
		}
		cites = append(cites, fmt.Sprintf("{{cite %s \"Outlet %d reports the harbour board approved the plan\"}}", url, i))
	}
	return caps, strings.Join(cites, " ")
}

func groupPairs(t *testing.T, caps *fakeCaptures, first, cites, second string) []string {
	t.Helper()
	res, err := Apply(context.Background(), Input{
		OwnerID: "owner", DocID: "doc", Seed: "rev-0",
		Base:     baseDoc(heading("h1", 1, "Harbour"), paragraph("p1", "Opening line.")),
		Script:   "@replace b2\n" + first + " " + cites + ". " + second,
		Resolver: caps,
	})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	_, pairs := Checks(res.Doc, "doc", "rev-1")
	var out []string
	for _, p := range pairs {
		out = append(out, fmt.Sprintf("%s|%d|%s", p.Key, len(p.Group), p.Text))
	}
	return out
}

func TestChecksGroupSideBySideCites(t *testing.T) {
	for _, n := range []int{2, 3, 4, 5} {
		caps, cites := groupFixture(n)
		res, err := Apply(context.Background(), Input{
			OwnerID: "owner", DocID: "doc", Seed: "rev-0",
			Base:     baseDoc(heading("h1", 1, "Harbour"), paragraph("p1", "Opening line.")),
			Script:   "@replace b2\nThe harbour board approved the plan " + cites + ".",
			Resolver: caps,
		})
		if err != nil {
			t.Fatalf("n=%d write: %v", n, err)
		}
		_, pairs := Checks(res.Doc, "doc", "rev-1")
		if len(pairs) != n {
			t.Fatalf("G1: n=%d pairs = %d; want one per cite", n, len(pairs))
		}
		keys := map[string]bool{}
		for _, p := range pairs {
			if len(p.Group) != n {
				t.Errorf("G1: n=%d pair group = %d quotes; want %d", n, len(p.Group), n)
			}
			if p.Text != "The harbour board approved the plan." {
				t.Errorf("G1: span = %q", p.Text)
			}
			keys[p.Key] = true
		}
		if len(keys) != n {
			t.Errorf("G1: n=%d keys are not distinct per cite", n)
		}
	}
}

func TestChecksLoneCiteStaysSpanKeyedAndSeparatedCitesAreNotGrouped(t *testing.T) {
	caps, _ := groupFixture(2)
	res, err := Apply(context.Background(), Input{
		OwnerID: "owner", DocID: "doc", Seed: "rev-0",
		Base: baseDoc(heading("h1", 1, "Harbour"), paragraph("p1", "Opening line.")),
		Script: "@replace b2\nThe board approved the plan {{cite https://example.org/s1 \"Outlet 1 reports the harbour board approved the plan\"}}. " +
			"Dredging starts in May {{cite https://example.org/s2 \"Outlet 2 reports the harbour board approved the plan\"}}.",
		Resolver: caps,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, pairs := Checks(res.Doc, "doc", "rev-1")
	if len(pairs) != 2 {
		t.Fatalf("pairs = %d", len(pairs))
	}
	for i, p := range pairs {
		if len(p.Group) != 0 {
			t.Errorf("G2/G3: pair %d has a group of %d; separated sentences are judged alone", i, len(p.Group))
		}
		if p.Key != PairKey(p.BlockHash, p.Cited.Hash, p.Cited.Quote) {
			t.Errorf("G2: lone pair %d is not keyed by its span, so its old memo is lost", i)
		}
	}
}

func TestChecksTheSameQuoteTwiceSideBySideIsOnePair(t *testing.T) {
	caps, _ := groupFixture(1)
	one := "{{cite https://example.org/s1 \"Outlet 1 reports the harbour board approved the plan\"}}"
	res, err := Apply(context.Background(), Input{
		OwnerID: "owner", DocID: "doc", Seed: "rev-0",
		Base:     baseDoc(heading("h1", 1, "Harbour"), paragraph("p1", "Opening line.")),
		Script:   "@replace b2\nThe board approved the plan " + one + " " + one + ".",
		Resolver: caps,
	})
	if err != nil {
		t.Fatal(err)
	}
	checks, pairs := Checks(res.Doc, "doc", "rev-1")
	if len(pairs) != 1 || len(pairs[0].Group) != 0 {
		t.Fatalf("G4: pairs = %+v; want one lone pair for a repeated cite", pairs)
	}
	if len(checks) != 2 || checks[0].Pair != checks[1].Pair {
		t.Errorf("G4: both anchors should point at the one pair: %+v", checks)
	}
}

func TestChecksGroupKeyChangesWithTheGroupNotWithOtherSentences(t *testing.T) {
	caps, three := groupFixture(3)
	two := strings.Join(strings.Split(three, " {{")[:2], " {{")
	a := groupPairs(t, caps, "The board approved the plan", two, "Closing line.")
	b := groupPairs(t, caps, "The board approved the plan", three, "Closing line.")
	c := groupPairs(t, caps, "The board approved the plan", two, "A different closing line.")
	if len(a) != 2 || len(b) != 3 {
		t.Fatalf("pairs a=%d b=%d", len(a), len(b))
	}
	keyOf := func(s string) string { return strings.SplitN(s, "|", 2)[0] }
	for _, ka := range a {
		for _, kb := range b {
			if keyOf(ka) == keyOf(kb) {
				t.Error("G5: a group with another cite kept an old key")
			}
		}
	}
	if keyOf(a[0]) != keyOf(c[0]) || keyOf(a[1]) != keyOf(c[1]) {
		t.Error("G6: an edit to another sentence re-keyed the group")
	}
}

func TestChecksLargeGroupsAreChunked(t *testing.T) {
	caps, cites := groupFixture(maxGroupCites + 2)
	got := groupPairs(t, caps, "The board approved the plan", cites, "Closing line.")
	if len(got) != maxGroupCites+2 {
		t.Fatalf("pairs = %d", len(got))
	}
	sizes := map[string]int{}
	for _, g := range got {
		sizes[strings.Split(g, "|")[1]]++
	}
	if sizes[fmt.Sprint(maxGroupCites)] != maxGroupCites || sizes["2"] != 2 {
		t.Errorf("G7: group sizes = %v; want %d in a chunk of %d and 2 in a chunk of 2", sizes, maxGroupCites, maxGroupCites)
	}
}
