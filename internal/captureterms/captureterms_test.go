package captureterms

import (
	"reflect"
	"strings"
	"testing"
)

func TestTokenSpansAgreeWithTokensAndPointIntoTheText(t *testing.T) {
	for _, text := range []string{
		"The harbor reopened, and 5G towers: AI-driven ledgers.",
		"港口今天重新开放，货轮陆续进入。",
		"Порт сегодня снова открыт; Café Münster",
		"الميناء مفتوح اليوم",
		"mixed Latin 港口 and العربية together",
		"",
		"   ... --- !!!",
	} {
		spans := TokenSpans(text)
		var terms []string
		for _, sp := range spans {
			terms = append(terms, sp.Term)
			if sp.Start < 0 || sp.End > len(text) || sp.End <= sp.Start {
				t.Fatalf("%q: span %+v out of range", text, sp)
			}
			if !strings.EqualFold(strings.ToLower(text[sp.Start:sp.End]), sp.Term) && !strings.Contains(strings.ToLower(text[sp.Start:sp.End]), sp.Term) {
				t.Fatalf("%q: span %+v covers %q", text, sp, text[sp.Start:sp.End])
			}
		}
		if want := Tokens(text); !reflect.DeepEqual(terms, want) && !(len(terms) == 0 && len(want) == 0) {
			t.Fatalf("%q: spans give %v, Tokens gives %v", text, terms, want)
		}
	}
	// Invalid UTF-8 is a separator, not a panic.
	if got := TokenSpans("ab\xffcd"); len(got) != 2 || got[0].Term != "ab" || got[1].Term != "cd" {
		t.Fatalf("invalid byte: %+v", got)
	}
}

func TestPluralFold(t *testing.T) {
	for in, want := range map[string]string{
		"harbors": "harbor", "stories": "story", "boxes": "box", "churches": "church", "glasses": "glass",
		"bus": "bus", "analysis": "analysis", "ai": "ai", "5gs": "5gs", "café": "café", "cats": "cat",
	} {
		if got := PluralFold(in); got != want {
			t.Errorf("PluralFold(%q) = %q, want %q", in, got, want)
		}
	}
}
