package textureprompts

import (
	"strings"
	"testing"
)

// Texture's prompt core (Cut 2 2a). Ways it can fail: it keeps teaching the
// retired ApplyTexture body, decide turns, work_disposition, update
// dispositions or controls; it loses the owner's revision-shape rules; it
// grows back past its budget; it teaches Texture to open engineering.
func TestPromptCoreTeachesWriteCitationsAndTurnVerbs(t *testing.T) {
	prompt := DefaultSystemPrompt()
	for _, want := range []string{
		"choir.Write(edits, why)", "@replace", "@insert", "@delete", "@move", "@section", "@all",
		`{{cite r2.c1 "`, "{{embed", "{{ref", "names its line",
		"choir.Done(why)", "choir.Wait(why)", "choir.Reject(record, why)",
		`choir.Cast("research", objective, packet)`, `choir.Cast("management", objective, packet)`, "execution_request",
		"choir.ReadDoc()", "frame", "r1..rM", "b1..bN",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt core does not teach %q", want)
		}
	}
}

func TestPromptCoreKeepsTheOwnerRevisionShape(t *testing.T) {
	prompt := DefaultSystemPrompt()
	for _, want := range []string{
		"V1, your first revision on a new request, is a fast acknowledgment",
		"dispatched in the same cell",
		"Every later revision rewrites the ideas, not a log. Never narrate actions.",
		"Texture never writes programs.",
		"state it plainly with its citation",
		"you never open engineering yourself",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt core lost the owner's rule %q", want)
		}
	}
}

// Published stories narrated Texture's process to the reader (docs/problems/
// autopaper-story-prose-markdown-and-meta-2026-10-13.md): a "**Status.**
// Revision 4." block carried and re-numbered across every version, "the
// management desk reports", "H1, management's probability 0.85", "As of
// this revision". Cut 2 dropped the overlay rule that kept coordination
// metadata out of reader prose. Ways it can fail: the core names no
// concrete anti-pattern, so the model copies frame vocabulary (desks,
// hypothesis labels, probabilities, revision numbers) into the story; the
// rule is restated so long the core blows its budget.
func TestPromptCoreKeepsProcessOutOfReaderProse(t *testing.T) {
	prompt := DefaultSystemPrompt()
	for _, want := range []string{
		"news story at its current state",
		`"Status"`, `"Revision N"`, `"Update:"`, `"In this revision"`,
		"desks", "hypothesis labels",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt core does not name the process-narration anti-pattern %q", want)
		}
	}
}

func TestPromptCoreRetiresTheOldWriteSurface(t *testing.T) {
	prompt := DefaultSystemPrompt()
	for _, retired := range []string{
		"ApplyTexture", `"op"`, "decide", "work_disposition", "update_dispositions", "decision_kind",
		"open_researcher", "open_persistent_super", "target_work_item_id", `"controls"`,
		"insert_source_ref", "mark_source_unused", "base_revision_id",
	} {
		if strings.Contains(prompt, retired) {
			t.Errorf("prompt core still teaches %q", retired)
		}
	}
	// 4000 until the story headline rule (2026-10-14) added one line.
	if n := len(prompt); n > 4300 {
		t.Errorf("prompt core is %d characters; the budget is about 4.3k", n)
	}
}

// A cite renders as a number only (docs/problems/prompt-bar-research-slow-
// unsynthesized-mangled-2026-10-14.md): Texture wrote "Anthropic says
// running it costs {{cite r1.c3 "...75% less to run."}} than Haiku 4.5" and
// "at an introductory price of {{cite ...}}", so the reader saw "costs ①
// than" with the quoted words missing; and the revision listed findings by
// category with no inference. Ways it can fail: the core still reads as if
// a cite's quote were shown; it gives no example of a sentence that reads
// whole without its cite; it does not ask for the inference before the
// facts; the rule is restated so long the core blows its budget.
func TestPromptCoreTeachesCitesAreNumbersAndInferenceFirst(t *testing.T) {
	prompt := DefaultSystemPrompt()
	for _, want := range []string{
		"A cite shows as a number, never its quote",
		"the sentence must read whole without it",
		`"costs 75% less {{cite ...}}", not "costs {{cite ...}}"`,
		"Lead with what the evidence adds up to",
		"A list of findings is not an answer.",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt core lacks %q", want)
		}
	}
}

// Stories were published under management's writing brief ("Write the
// story of ...") and with V1 plans as their deks ("This document will
// explain ...") (docs/problems/instruction-text-published-as-story-titles-
// 2026-10-14.md). The store now titles a story from the heading its
// revision leads with, and a story revision without one is held. Ways the
// prompt can fail: Texture is never told a story leads with a headline, so
// every revision is held; it is not told the headline is not the brief, so
// it copies the brief into the heading; it still writes a story's V1 as an
// acknowledgment, now under a heading, and the plan publishes.
func TestPromptCoreTeachesTheStoryHeadline(t *testing.T) {
	prompt := DefaultSystemPrompt()
	for _, want := range []string{
		"publish: auto",
		"V1 included, so its V1 is news, not a plan",
		"leads with one heading, its headline, never the brief you were sent",
		"it titles the story, and a revision without it does not publish",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt core lacks %q", want)
		}
	}
}
