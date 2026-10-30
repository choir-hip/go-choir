package researchtools

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/captureterms"
)

// The capture term table narrows; the Go matcher decides. So every
// (field text, query word) pair corpusTermIn accepts must leave the capture
// a candidate in the table: each of the word's parts is a stored term (or
// a prefix of one). Problem record failure modes 8 and 9
// (docs/archive/problems/corpus-retrieval-windowed-unindexed-untimed-2026-10-14.md):
// two-letter tokens, CJK without spaces, Arabic and Hebrew with attached
// prefixes, Thai, hyphens, apostrophes, digits, case, diacritics, marks.
// The one named deviation: a word of fewer than captureterms.MinPrefixRunes
// runes matches whole tokens only ("ai" no longer finds "aid").

var termFixtureTexts = []string{
	"New AI model release: EU and US regulators weigh in",
	"Aid convoy arrives in Gaza after talks",
	"COVID-19 cases rise; WHO's update for 2026",
	"中国发布新的人工智能模型",
	"日本のAI規制、来年施行へ",
	"والمصريون يحتفلون بالعيد",
	"הממשלה אישרה את התקציב",
	"ประเทศไทยเปิดตัวรถไฟความเร็วสูง",
	"Украина и ЕС подписали соглашение",
	"Café São Paulo élu meilleur café",
	"नमस्ते दुनिया: भारत में चुनाव",
	"iPhone17 launch, 5G and Wi-Fi 7",
	"reuters.com\nrss:reuters-world\nen",
	"x-rays reveal U.S. shipwreck",
	"Ελλάδα: νέα μέτρα για την ενέργεια",
	"한국 대통령 선거 결과 발표",
}

var termFixtureQueries = []string{
	"AI", "ai model", "EU US", "aid", "covid-19", "who's update", "2026", "人工智能", "中国", "AI規制",
	"مصر", "المصريون", "ממשלה", "ไทย", "рельсы", "Украина ЕС", "café", "sao", "são paulo", "भारत",
	"iphone", "5g", "wi-fi", "reuters", "rss:reuters", "x-ray", "u.s.", "shipwreck", "ενέργεια", "대통령",
	"中", "智", "模型", "élu", "release", "regulators weigh", "gaza", "convoy",
}

func partsHeld(stored map[string]bool, parts []captureterms.Part) bool {
	for _, p := range parts {
		if !p.Prefix {
			if !stored[p.Term] {
				return false
			}
			continue
		}
		found := false
		for term := range stored {
			if strings.HasPrefix(term, p.Term) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func TestCaptureTermsHoldEveryMatchTheGoMatcherAccepts(t *testing.T) {
	checked, deviations := 0, 0
	for _, text := range termFixtureTexts {
		stored := map[string]bool{}
		for _, term := range captureterms.Terms(text) {
			stored[term] = true
		}
		lower := strings.ToLower(text)
		for _, query := range termFixtureQueries {
			for _, word := range corpusQueryTerms(query) {
				if !corpusTermIn(lower, word) {
					continue
				}
				checked++
				parts := captureterms.QueryParts(word)
				if partsHeld(stored, parts) {
					continue
				}
				first, _ := utf8.DecodeRuneInString(word)
				if utf8.RuneCountInString(word) < captureterms.MinPrefixRunes && !captureterms.Anywhere(first) {
					deviations++
					continue
				}
				t.Errorf("text %q: the Go matcher accepts %q but the table would not return it (parts %+v, terms %v)", text, word, parts, captureterms.Terms(text))
			}
		}
	}
	if checked < 40 {
		t.Fatalf("only %d accepted (text, word) pairs checked; the fixture lost its coverage", checked)
	}
	// "ai" inside "aid" is the named deviation; it must stay rare.
	if deviations == 0 || deviations > 3 {
		t.Fatalf("deviations = %d, want the named short-prefix case only", deviations)
	}
}

func TestCaptureTermsKeepShortTokensAndSegmentCJK(t *testing.T) {
	terms := map[string]bool{}
	for _, term := range captureterms.Terms("New AI model: EU, US and 5G — 中国发布人工智能") {
		terms[term] = true
	}
	for _, want := range []string{"ai", "eu", "us", "5g", "中", "中国", "人工", "工智", "智能"} {
		if !terms[want] {
			t.Errorf("term %q missing from %v", want, terms)
		}
	}
	if parts := captureterms.QueryParts("人工智能"); len(parts) != 3 {
		t.Errorf("人工智能 parts = %+v, want its three bigrams", parts)
	}
	if parts := captureterms.QueryParts("ai"); len(parts) != 1 || parts[0].Prefix {
		t.Errorf("ai parts = %+v, want one whole-token part", parts)
	}
	if parts := captureterms.QueryParts("ukrain"); len(parts) != 1 || !parts[0].Prefix {
		t.Errorf("ukrain parts = %+v, want one prefix part", parts)
	}
}
