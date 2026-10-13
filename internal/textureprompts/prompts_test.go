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
	if n := len(prompt); n > 4000 {
		t.Errorf("prompt core is %d characters; the budget is about 3.5k", n)
	}
}
