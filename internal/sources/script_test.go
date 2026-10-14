package sources

import "testing"

// Per-item script guess (L2a A5, defect 5 in
// docs/problems/autopaper-intake-defects-2026-10-14.md). Ways it can fail,
// written before the code:
//   - Japanese text is called han because kanji outnumber kana in a
//     headline, so it is grouped with Chinese;
//   - a Latin headline with one CJK or kana name in it is called by that
//     name's script instead of by its majority;
//   - digits, punctuation and spaces count as letters, so a short headline
//     with numbers is called latin or other by its punctuation;
//   - an empty string, or text only in another script (Greek, Hebrew,
//     Devanagari), returns an empty or unlisted value instead of "other";
//   - the guess is not deterministic, so a replayed arrival differs.
func TestGuessScript(t *testing.T) {
	cases := []struct {
		name, text, want string
	}{
		{"chinese", "中国经济第三季度增长放缓", "han"},
		{"chinese traditional", "台灣總統今日發表談話", "han"},
		{"japanese kanji and kana", "東京で震度5弱の地震", "kana"},
		{"japanese katakana only", "ニュース", "kana"},
		{"korean", "서울에서 대규모 집회 열려", "hangul"},
		{"arabic", "الحكومة تعلن عن خطة جديدة", "arabic"},
		{"russian", "Правительство объявило новый план", "cyrillic"},
		{"ukrainian", "Київ під атакою дронів", "cyrillic"},
		{"english", "Government announces a new plan", "latin"},
		{"french accents", "Le président s'exprime à l'Élysée", "latin"},
		{"latin with a kana name", "Nintendo sales of ポケモン games rise sharply this quarter", "latin"},
		{"latin with a han place", "Shares fall in Shenzhen 深圳 after the announcement today", "latin"},
		{"han majority with latin acronym", "中国GDP增长百分之五", "han"},
		{"digits and punctuation only", "2026-10-14 12:00 !!!", "other"},
		{"empty", "", "other"},
		{"greek", "Η κυβέρνηση ανακοίνωσε νέο σχέδιο", "other"},
		{"hebrew", "הממשלה הודיעה על תוכנית חדשה", "other"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := GuessScript(tc.text); got != tc.want {
				t.Fatalf("GuessScript(%q) = %q, want %q", tc.text, got, tc.want)
			}
			if again := GuessScript(tc.text); again != tc.want {
				t.Fatalf("GuessScript(%q) is not deterministic: %q then %q", tc.text, tc.want, again)
			}
		})
	}
}
