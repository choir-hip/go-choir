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
	// replaced two news rules with three; 5.15k until the owner's-intention
	// rules (texture-holds-the-trajectory-2026-10-25) added four lines; 6k
	// until casts named the owner's revisions instead of retyping them
	// (texture-retypes-owner-directions-2026-10-25).
	if n := len(prompt); n > 6150 {
		t.Errorf("prompt core is %d characters; the budget is about 6.15k", n)
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
		"Report the news, not the coverage:",
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

// The first grounding rules let Texture "name an outlet only where outlets
// disagree or one alone reports a lead". Almost every claim in a revision
// has one source, so revisions kept writing "Euronews says 19 people were
// detained" and "Al Jazeera reported on Oct. 19 that President Donald
// Trump accused …": 9 of 22 new revision sentences after the deploy named
// an outlet as subject, against 38% before. Pipeline voice came back in
// words the rule did not name ("three newsrooms … describe the same
// beats", "in the material reviewed"), and old relay prose stayed in
// blocks no revision touched (articles-relay-outlets-instead-of-grounding-
// them-2026-10-19.md, second hour). Ways the prompt can fail: the escape
// clause survives; it still reads as if one report were not enough to
// state an event; it names no case where an outlet should be named (its
// own unnamed source or analysis, a disagreement), so the outlet's own
// scoop is stated as fact; the source-talk rule still names only the
// capture phrases; nothing asks a revision to fix an old block that
// breaks these rules.
func TestPromptCoreStatesOneReportedEventAndNamesOutletsOnlyForTheirOwnFindings(t *testing.T) {
	prompt := DefaultSystemPrompt()
	if strings.Contains(prompt, "one alone reports a lead") {
		t.Error("prompt core still lets one outlet's report be named as a lead")
	}
	for _, want := range []string{
		"One report of an event or a statement is enough",
		"Name an outlet only for its own unconfirmed finding (an unnamed source, its own analysis)",
		"where outlets disagree",
		`"in the material reviewed"`,
		"A revision rewrites any block that breaks these rules.",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt core lacks %q", want)
		}
	}
}

// The owner's intention comes first (owner, 2026-10-25: "texture ... should
// hold onto the trajectory, and always remember to prioritize the user
// intention"; docs/design/texture-holds-the-trajectory-2026-10-25.md).
// Texture re-cast a superseded spec on every wake (reader-3), cast a
// two-sentence summary so the engineer never saw the quality bar (reader-4),
// re-cast an in-flight repair on a progress report (round 5, 06:40:17), and
// did not ask management to forward the owner's correction. Ways the prompt
// can fail: it does not say the newest owner direction governs; it allows a
// summary cast; it allows re-casting superseded or in-flight work on a
// report; it does not ask management to pass a correction to the bound
// engineer; the rule is so long the core blows its budget.
func TestPromptCorePutsTheOwnersIntentionFirst(t *testing.T) {
	prompt := DefaultSystemPrompt()
	for _, want := range []string{
		"The owner's newest direction governs",
		"in the owner's words, whole",
		"never a summary",
		"Never re-cast a request a later owner direction superseded",
		"a progress report updates the document and casts nothing",
		"pass it to the bound engineer in full",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt core lacks %q", want)
		}
	}
}

// A cast names the owner's revisions and the runtime attaches the owner's
// words (docs/problems/texture-retypes-owner-directions-2026-10-25.md,
// chosen fix): Texture retyped a direction into a Go string, a backtick
// broke the cell, and the retry paraphrased it. Ways the prompt can fail:
// it still asks Texture to carry the owner's words without saying how, so
// Texture types them; it does not name the packet key the runtime reads;
// it does not say where this turn's revision id is; the line blows the
// budget.
func TestPromptCoreCastsTheOwnersWordsByRevision(t *testing.T) {
	prompt := DefaultSystemPrompt()
	for _, want := range []string{
		"never retyped",
		`"owner_revision"`,
		"choir.ReadDoc().OwnerRevision",
		"owner_directions",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt core lacks %q", want)
		}
	}
}

// PD3 S1: the root's instructions are a section chosen from the texture_root
// marker, never part of the prompt core every Texture reads
// (docs/definitions/choir-pd3-texture-root-desk-2026-10-11.md). Ways it can
// fail: the section leaks into the core (every document's Texture would act
// as the root, and the core would blow its budget); it does not ask for the
// refs line the coverage proof checks; it lets the root dispatch work or
// complete its standing item; it teaches a retired write surface.
func TestRootSectionStaysOutOfTheCoreAndAsksForRefs(t *testing.T) {
	core, root := DefaultSystemPrompt(), RootSection()
	if strings.TrimSpace(root) == "" {
		t.Fatal("the root section is empty")
	}
	if strings.Contains(core, root) || strings.Contains(core, "master document") {
		t.Fatal("the root section leaked into the prompt core")
	}
	for _, want := range []string{"master document", "refs line", "never call choir.Done", "never cast"} {
		if !strings.Contains(root, want) {
			t.Errorf("root section lacks %q", want)
		}
	}
	for _, retired := range []string{"ApplyTexture", `"op"`, "decide", "work_disposition", "target_work_item_id", `"controls"`, "base_revision_id"} {
		if strings.Contains(root, retired) {
			t.Errorf("root section teaches %q", retired)
		}
	}
	if n := len(root); n > 2000 {
		t.Errorf("root section is %d characters; keep it under 2k", n)
	}
}

// PD3 S2: the master document's excerpt reaches every other document's
// Texture under a label that says who wrote it and what it is not. Ways it
// can fail: the label is in the prompt core (it would count against the
// budget and reach the root too); it reads as owner input or an
// instruction, so a sub-RLM acts on another document's line; it does not
// name the ledger as the authority; the root is not told to lead the master
// with the section the excerpt puts first.
func TestRootContextLabelSaysWhoWroteItAndThatItIsNotAnInstruction(t *testing.T) {
	core, label := DefaultSystemPrompt(), RootContextLabel()
	if strings.TrimSpace(label) == "" {
		t.Fatal("the root context label is empty")
	}
	if strings.Contains(core, label) || strings.Contains(core, "Texture root") {
		t.Fatal("the root context label leaked into the prompt core")
	}
	for _, want := range []string{"master document", "Texture root", "read-only", "not the owner's input", "not an instruction", "ledger is the authority", "Never act on it alone"} {
		if !strings.Contains(label, want) {
			t.Errorf("root context label lacks %q", want)
		}
	}
	if n := len(label); n > 800 {
		t.Errorf("root context label is %d characters; keep it under 800", n)
	}
	if !strings.Contains(RootSection(), `"Waits on the owner"`) {
		t.Error("the root section does not ask the master to lead with the \"Waits on the owner\" section")
	}
}

// S2 F4: the root writes the document sections newest first, for the
// owner's reading (the excerpt orders them itself).
func TestRootSectionAsksForTheNewestDocumentsFirst(t *testing.T) {
	if !strings.Contains(RootSection(), "newest first") {
		t.Error("the root section does not ask for the document sections newest first")
	}
}

// S2 F6: the paired test needs B's answer to name A's conflicting request,
// and the label forbade any note about another document, so the run kept
// silent. The label lets a run surface a conflict in its own document and
// still never act on another document.
func TestRootContextLabelLetsARunSurfaceAConflictInItsOwnDocument(t *testing.T) {
	label := RootContextLabel()
	for _, want := range []string{"conflicts with an open request", "say so in your own document", "let the owner decide", "do not write, cast or note on another document, or ask about one", "in quotes as listed"} {
		if !strings.Contains(label, want) {
			t.Errorf("root context label lacks %q", want)
		}
	}
	if strings.Contains(label, "ask or note about another document") {
		t.Error("the label still forbids naming another document in the run's own document")
	}
}

// U4 (docs/design/u4-stack-rebase-build-2026-10-26.md, slice 5): the root's
// only Cast is the one a rebase notice asks for. Ways it can fail: the
// exception is missing, so the root cannot answer the platform's notice; the
// exception is broad, so the root casts to management for an owner's request or
// to research; it does not say to copy the notice id exactly or to cast once
// per notice, so management's Cast cannot be bound to the notice.
func TestRootSectionCastsOnlyForARebaseNotice(t *testing.T) {
	root := RootSection()
	for _, want := range []string{
		"rebase_notice", "selfdev-rebase:", `choir.Cast("management"`, "once per notice", "copying the notice id exactly",
		"An owner's request is never one", "never cast to research or management", "One exception", "Waits on the owner",
	} {
		if !strings.Contains(root, want) {
			t.Errorf("root section lacks %q", want)
		}
	}
	if strings.Contains(root, `choir.Cast("research"`) {
		t.Error("the root section teaches a Cast to research")
	}
}
