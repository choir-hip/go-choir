package agentcore

import (
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/texturedoc"
)

// Quality holds, unit level (lane Q). Ways each can fail, written before the
// code:
//   - a legitimate opener ("Status-quo", "Status: the bridge reopens ...",
//     "In this revision of the treaty") is held as process text;
//   - the bare forms the audit found ("Status.", "Revision 4", "In this
//     revision, I ...") publish;
//   - the headline's words count toward the floor, so a 12-word headline
//     lets an 88-word body through;
//   - the length hold applies on every revision (a thin-sourced story is held
//     for ever) or the once-flag also lets a French or process-text story
//     through.
func blk(kind, text string) texturedoc.TextBlock { return texturedoc.TextBlock{Type: kind, Text: text} }

func words(n int) string { return strings.TrimSpace(strings.Repeat("harbor ", n)) }

func TestProcessTextMatchesOnlyTheBareFormsTheAuditFound(t *testing.T) {
	held := []string{"Status.", "Status:", "Update:", "Revision 4", "Status. Revision 4.", "In this revision, I have added the court's ruling."}
	for _, text := range held {
		if got := processTextLine([]texturedoc.TextBlock{blk("paragraph", text)}); got == "" {
			t.Errorf("%q passed, want held as process text", text)
		}
	}
	passed := []string{
		"Status-quo bias held the vote back.",
		"Status: the bridge reopened on Monday after repairs.",
		"Update: the council has now confirmed the date.",
		"In this revision of the treaty, the border article was dropped.",
		"Revision 4 of the treaty was signed on Tuesday.",
	}
	for _, text := range passed {
		if got := processTextLine([]texturedoc.TextBlock{blk("paragraph", text)}); got != "" {
			t.Errorf("%q held as process text (%q), want it to pass", text, got)
		}
	}
}

func TestLengthHoldCountsTheBodyAndAppliesOnce(t *testing.T) {
	headline := blk("heading", words(30))
	short := []texturedoc.TextBlock{headline, blk("paragraph", words(MinPublishWords-1))}
	full := []texturedoc.TextBlock{headline, blk("paragraph", words(MinPublishWords))}

	if got := qualityHold(short, 2, false); !strings.Contains(got, "body (the text after the headline) has 99 words") || !strings.Contains(got, "applies once") {
		t.Errorf("first short revision: %q, want a length hold naming the body count and that it applies once", got)
	}
	if got := qualityHold(short, 3, true); got != "" {
		t.Errorf("second short revision: %q, want it to publish", got)
	}
	if got := qualityHold(full, 2, false); got != "" {
		t.Errorf("revision at the floor: %q, want it to publish", got)
	}

	// Not one-shot: the other holds apply however many times the story was held.
	french := strings.Repeat("Le conseil municipal a annoncé que le pont sera fermé pendant les travaux, et les habitants sont invités à utiliser la nouvelle route pour se rendre au centre. ", 5)
	if got := qualityHold([]texturedoc.TextBlock{headline, blk("paragraph", french)}, 4, true); !strings.Contains(got, "not written in English") {
		t.Errorf("French after a length hold: %q, want the language hold", got)
	}
	process := []texturedoc.TextBlock{headline, blk("paragraph", "Status. Revision 4."), blk("paragraph", words(200))}
	if got := qualityHold(process, 4, true); !strings.Contains(got, "process text") {
		t.Errorf("process text after a length hold: %q, want the process-text hold", got)
	}
}
