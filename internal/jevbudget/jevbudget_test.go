package jevbudget

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// Ways this can fail, written before the code:
//
//	J1. Non-Latin text is counted like Latin: a Cyrillic, CJK or emoji state
//	    tokenizes about 3x to 7x denser than English, so a byte count lets a
//	    request through that the vendor refuses (64k tokens a request, 32k a
//	    question).
//	J2. The estimate falls below the vendor's own fit for Latin prose, so the
//	    margin is spent before it starts.
//	J3. The estimate falls below the densest ratio we have observed for an
//	    intake-shaped request (about 0.58 tokens a byte: 17,481 tokens at
//	    30,046 bytes), which is the shape most of the paper's calls have.
//	J4. The per-question limit (state plus ONE question) is ignored because
//	    only the request total is checked, or the request total is ignored
//	    because only one question is checked.
//	J5. A request that fits is refused, or one that cannot is let through,
//	    at the budget line.

func raw(s string) json.RawMessage { b, _ := json.Marshal(s); return b }

func TestJ1NonLatinCountsDenser(t *testing.T) {
	cyr := strings.Repeat("быстрая коричневая лиса ", 100)
	han := strings.Repeat("敏捷的棕色狐狸跳过了懒狗", 100)
	emoji := strings.Repeat("🦊🐕", 500)
	for name, s := range map[string]string{"cyrillic": cyr, "han": han, "emoji": emoji} {
		got := Tokens([]byte(s))
		letters := 0
		for _, r := range s {
			if r != ' ' {
				letters++
			}
		}
		if perLetter := float64(got) / float64(letters); perLetter < 0.9 {
			t.Errorf("J1: %s counts %.2f tokens a character; the vendor measures 0.92 to 1.71", name, perLetter)
		}
	}
	if got := Tokens([]byte(strings.Repeat("🦊", 1000))); got < 1710 {
		t.Errorf("J1: 1000 emoji = %d tokens, want at least 1710", got)
	}
}

func TestJ2LatinProseIsAtLeastTheVendorFit(t *testing.T) {
	prose := strings.Repeat("Officials said on Tuesday that the talks would resume next week, ", 200)
	if got, floor := Tokens([]byte(prose)), int(0.224*float64(len(prose))); got < floor {
		t.Errorf("J2: %d tokens for %d characters, under the fit's %d", got, len(prose), floor)
	}
}

func TestJ3IntakeShapeIsAtLeastTheDensestObservedRatio(t *testing.T) {
	// 300 story handles listed as criteria keys, as every intake question does.
	var b strings.Builder
	b.WriteByte('{')
	for i := 1; i <= 300; i++ {
		if i > 1 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, "%q:null", fmt.Sprintf("c%d", i))
	}
	b.WriteByte('}')
	if got, floor := Tokens([]byte(b.String())), int(0.55*float64(b.Len())); got < floor {
		t.Errorf("J3: handle criteria %d bytes estimated %d tokens, under 0.55 a byte (%d); observed intake requests reached 0.58", b.Len(), got, floor)
	}
}

func TestJ4BothVendorLimitsBind(t *testing.T) {
	// Many questions over a small state: only the request total binds.
	many := map[string]json.RawMessage{}
	q := raw(strings.Repeat("x ", 3000)) // about 1,500 tokens
	for i := 0; i < 40; i++ {
		many[fmt.Sprintf("q%d", i)] = q
	}
	est := Measure(raw("s"), many)
	if est.WorstQuestion > QuestionBudgetTokens || Fits(raw("s"), many) {
		t.Errorf("J4: 40 questions (%d tokens in all) must fail on the request total alone (worst question %d)", est.Request, est.WorstQuestion)
	}
	// A big state and one tiny question: only the per-question limit binds,
	// and the request total (one question) is the same size, so add a second
	// question the request total sees but the per-question limit does not.
	big := raw(strings.Repeat("word ", 18000)) // about 27k tokens: over the 24k question budget
	if Fits(big, map[string]json.RawMessage{"q": raw("which")}) {
		t.Errorf("J4: a state over the per-question budget fits")
	}
	two := Measure(raw(strings.Repeat("word ", 4000)), map[string]json.RawMessage{"a": raw("one"), "b": raw(strings.Repeat("longer question ", 400))})
	if two.Request <= two.WorstQuestion {
		t.Errorf("J4: request %d must exceed the worst question %d when there are two questions", two.Request, two.WorstQuestion)
	}
}

func TestJ5BudgetLine(t *testing.T) {
	if RequestBudgetTokens != VendorRequestTokens*3/4 || QuestionBudgetTokens != VendorQuestionTokens*3/4 {
		t.Fatalf("J5: the margin is a quarter of each vendor limit")
	}
	// 30,000 bytes of Latin prose (the old line) fits easily.
	if !Fits(raw(strings.Repeat("a b c ", 5000)), map[string]json.RawMessage{"q": raw("which")}) {
		t.Errorf("J5: 30 KB of prose is refused")
	}
	// Exactly at the line: a state sized to the question budget fits, one
	// more chunk of it does not.
	var fit, over string
	for n := 1000; ; n += 100 {
		s := strings.Repeat("word ", n)
		if !Fits(raw(s), map[string]json.RawMessage{"q": raw("which")}) {
			over = s
			break
		}
		fit = s
	}
	if fit == "" || over == "" || !Fits(raw(fit), map[string]json.RawMessage{"q": raw("which")}) {
		t.Errorf("J5: no fit/over boundary found")
	}
}
