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
		`{{cite r2.c1 "`, "{{ref", "names its line",
		"choir.Done(why)", "choir.Wait(why)", "choir.Reject(record, why)",
		`choir.Cast("research", objective, packet)`, `choir.Cast("management", objective, packet)`, "execution_request",
		"choir.ReadDoc()", "frame", "r1..rM", "b1..bN",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt core does not teach %q", want)
		}
	}
	// Write refuses {{embed}}: a story never reprints a source's text
	// (owner, 2026-10-16).
	if strings.Contains(prompt, "{{embed") {
		t.Error("prompt core still teaches {{embed}}")
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
	// 4000 until the story headline rule (2026-10-14) added one line; 4.3k
	// until the citation self-score (icrl-calibration-experiment-2026-10-14)
	// added one sentence; 4.4k until Texture could read its frame's
	// captures (texture-opens-stories-blind-to-their-capture-2026-10-16)
	// and one sentence said so; 4.56k until the grounding rules
	// (articles-relay-outlets-instead-of-grounding-them-2026-10-19)
	// replaced two news rules with three.
	if n := len(prompt); n > 5150 {
		t.Errorf("prompt core is %d characters; the budget is about 5.15k", n)
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

// Stories relayed outlets instead of reporting the news: 38% of front-page
// sentences named an outlet as their subject ("the BBC reports", "Al
// Jazeera says"), 4 of 113 sources were primary, and the pipeline's voice
// leaked into prose ("its captured text says ... but gives no further case
// details"); one headline described its sources ("surfaces without its
// text") (docs/problems/articles-relay-outlets-instead-of-grounding-them-
// 2026-10-19.md; owner, 2026-10-19). Ways the prompt can fail: it still
// asks for each claim "in its source's own terms", so Texture keeps
// attributing; it gives no example of a corroborated fact stated plainly
// with two cites; it lets Texture credit the relaying outlet rather than
// whoever made the claim; it does not ask for a cite after each part of a
// sentence that draws on two sources; it names no pipeline phrase to avoid;
// the headline rule does not forbid describing the sources.
func TestPromptCoreReportsTheNewsNotTheCoverage(t *testing.T) {
	prompt := DefaultSystemPrompt()
	for _, want := range []string{
		"Report the news, not the coverage.",
		`not "DW reports … Al Jazeera says …"`,
		"never to the outlet that relayed it",
		"cites each source right after its part",
		`"its captured text"`,
		"The headline states the news, never the state of the sources.",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt core lacks %q", want)
		}
	}
	if strings.Contains(prompt, "in its source's own terms") {
		t.Error("prompt core still asks for each claim in its source's own terms")
	}
}
