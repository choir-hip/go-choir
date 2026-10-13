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
