package styleguide

import (
	"strings"
	"testing"
)

func wrap(lines string) string {
	return "# Style Guide\n\nprose\n\n## Design values\n\n" + lines + "\n\n## Other\n- serif: modern\n"
}

func TestDefaultIsTodaysPage(t *testing.T) {
	d := Default()
	if d.Serif != "classic" || d.HeadlineWeight != "700" || d.DekStyle != "normal" || d.NameplateRule != "double" {
		t.Fatalf("default = %+v", d)
	}
}

func TestParseAppliesOnlyInsideTheDesignSection(t *testing.T) {
	d, ignored := Parse(wrap("- headline-weight: 800\n- dek-style: italic\n- nameplate-rule: single"))
	if d.HeadlineWeight != "800" || d.DekStyle != "italic" || d.NameplateRule != "single" {
		t.Fatalf("design = %+v", d)
	}
	if d.Serif != "classic" || len(ignored) != 0 {
		t.Fatalf("serif under ## Other must not count: %+v ignored=%v", d, ignored)
	}
	out, _ := Parse("## Other\n- headline-weight: 800\n")
	if out != Default() {
		t.Fatalf("outside the section: %+v", out)
	}
}

func TestParseRefusesInjectionAndUnknowns(t *testing.T) {
	d, ignored := Parse(wrap("- headline-weight: 700; } body { display:none\n- serif: red; x\n- nameplate-rule: url(//evil)\n- colour: blue\n- dek-style: wobbly"))
	if d != Default() || len(ignored) != 5 {
		t.Fatalf("design=%+v ignored=%v", d, ignored)
	}
}

func TestParseBadInputIsTheDefault(t *testing.T) {
	for _, bad := range []string{"", "\x00\x01", strings.Repeat("x", MaxGuideBytes+1), "no headings at all"} {
		if d, _ := Parse(bad); d != Default() {
			t.Fatalf("%.20q gave %+v", bad, d)
		}
	}
}

func TestFirstOccurrenceWinsAndKeysNormalise(t *testing.T) {
	d, ignored := Parse(wrap("- headline-weight: 600\n- headline-weight: 800\n*  Dek-Style :  ITALIC "))
	if d.HeadlineWeight != "600" || d.DekStyle != "italic" || len(ignored) != 1 {
		t.Fatalf("design=%+v ignored=%v", d, ignored)
	}
}

func TestCodeFencedLinesAreNotApplied(t *testing.T) {
	d, _ := Parse(wrap("```\n- headline-weight: 800\n```"))
	if d.HeadlineWeight != "700" {
		t.Fatalf("design=%+v", d)
	}
}

func TestHouseGuideTextParsesCleanly(t *testing.T) {
	d, ignored := Parse(HouseGuideText)
	if d != Default() || len(ignored) != 0 {
		t.Fatalf("design=%+v ignored=%v", d, ignored)
	}
}

func TestIsGuideTitle(t *testing.T) {
	for _, ok := range []string{"Style Guide", "  style  guide ", "STYLE GUIDE"} {
		if !IsGuideTitle(ok) {
			t.Fatalf("%q should match", ok)
		}
	}
	for _, no := range []string{"", "Style", "Style Guide 2", "My Style Guide"} {
		if IsGuideTitle(no) {
			t.Fatalf("%q should not match", no)
		}
	}
}
