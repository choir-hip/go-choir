package sectionlist

import (
	"math"
	"strings"
	"testing"
)

func TestHouseListIsValidAndFitsOneCall(t *testing.T) {
	l := House()
	if err := l.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(l.Sections) < 8 || len(l.Sections) > MaxSections {
		t.Fatalf("house list has %d sections", len(l.Sections))
	}
	if !l.Published().Valid() {
		t.Fatalf("published house list is not valid")
	}
}

func TestValidateRefuses(t *testing.T) {
	good := Section{ID: "a", Label: "A", Question: "Is it a?", Threshold: 0.5}
	cases := map[string]List{
		"no sections":     {Version: 1},
		"bad id":          {Version: 1, Sections: []Section{{ID: "A B", Label: "A", Question: "q", Threshold: 0.5}}},
		"reserved id":     {Version: 1, Sections: []Section{{ID: "front", Label: "A", Question: "q", Threshold: 0.5}}},
		"zero threshold":  {Version: 1, Sections: []Section{{ID: "a", Label: "A", Question: "q"}}},
		"empty question":  {Version: 1, Sections: []Section{{ID: "a", Label: "A", Question: "  ", Threshold: 0.5}}},
		"same question":   {Version: 1, Sections: []Section{good, {ID: "b", Label: "B", Question: "Is  it a?", Threshold: 0.5}}},
		"same id":         {Version: 1, Sections: []Section{good, {ID: "a", Label: "B", Question: "other", Threshold: 0.5}}},
		"version zero":    {Sections: []Section{good}},
		"too many (11)":   {Version: 1, Sections: manySections(11)},
		"threshold above": {Version: 1, Sections: []Section{{ID: "a", Label: "A", Question: "q", Threshold: 1.5}}},
	}
	for name, l := range cases {
		if err := l.Validate(); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func manySections(n int) []Section {
	var out []Section
	for i := 0; i < n; i++ {
		out = append(out, Section{ID: "s" + strings.Repeat("x", i), Label: "S", Question: "q" + strings.Repeat("x", i), Threshold: 0.5})
	}
	return out
}

func TestHashNormalizesWhitespaceOnly(t *testing.T) {
	if Hash("Is it  a?\n") != Hash(" Is it a? ") {
		t.Fatalf("whitespace changed the hash")
	}
	if Hash("Is it a?") == Hash("Is it b?") {
		t.Fatalf("different questions share a hash")
	}
}

func TestPlace(t *testing.T) {
	l := House()
	p := l.Published()
	h := func(id string) string {
		for _, e := range p.Entries {
			if e.ID == id {
				return e.Hash
			}
		}
		t.Fatalf("no section %s", id)
		return ""
	}
	// Multi-membership: both above threshold, primary is the higher.
	got := p.Place(map[string]float64{h("world"): 0.7, h("politics"): 0.9, h("sports"): 0.1})
	if strings.Join(got.Sections, ",") != "world,politics" || got.Primary != "politics" || got.Fallback {
		t.Fatalf("multi = %+v", got)
	}
	// None above threshold: the top one, marked.
	got = p.Place(map[string]float64{h("world"): 0.2, h("science"): 0.4, h("sports"): 0.1})
	if strings.Join(got.Sections, ",") != "science" || got.Primary != "science" || !got.Fallback {
		t.Fatalf("fallback = %+v", got)
	}
	// A tie goes to the earlier section.
	got = p.Place(map[string]float64{h("world"): 0.8, h("us"): 0.8})
	if got.Primary != "world" {
		t.Fatalf("tie = %+v", got)
	}
	// Nothing stored: unplaced, not a panic.
	if got = p.Place(nil); len(got.Sections) != 0 || got.Primary != "" {
		t.Fatalf("empty = %+v", got)
	}
	// NaN is not a member and not the top.
	got = p.Place(map[string]float64{h("world"): math.NaN(), h("us"): 0.3})
	if got.Primary != "us" || !got.Fallback {
		t.Fatalf("nan = %+v", got)
	}
}

func TestThresholdChangeReplaces(t *testing.T) {
	l := House()
	probs := map[string]float64{Hash(l.Sections[0].Question): 0.6}
	if got := l.Published().Place(probs); got.Fallback {
		t.Fatalf("0.6 at threshold 0.5 should be a member: %+v", got)
	}
	l.Sections[0].Threshold = 0.8
	if got := l.Published().Place(probs); !got.Fallback {
		t.Fatalf("0.6 at threshold 0.8 should fall back: %+v", got)
	}
}
