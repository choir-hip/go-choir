package runtime

import (
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestVTextPromptCreativeDraftFastPath(t *testing.T) {
	prompt := "tell me a story about computers"
	if !vtextPromptAllowsUngroundedCreativeDraft(prompt) {
		t.Fatalf("expected %q to allow an ungrounded creative draft", prompt)
	}
	if vtextRevisionRequiresWorkerGrounding(false, types.AuthorAppAgent, true) {
		t.Fatal("creative conductor seed should not require worker grounding")
	}

	current := types.Revision{
		DocID:      "doc-story",
		RevisionID: "rev-story",
		Content:    "# tell me a story\n\ntell me a story about computers",
		AuthorKind: types.AuthorAppAgent,
	}
	request := buildAgentRevisionRequest(current, nil, nil, vtextAgentRevisionRequest{
		Intent: "initial_conductor_workflow",
		Prompt: prompt,
	}, "", false, true, nil, nil)

	for _, want := range []string{
		"You may call edit_vtext to produce the requested creative document without worker grounding",
		"Do not spawn researcher or request super for this creative draft",
	} {
		if !strings.Contains(request, want) {
			t.Fatalf("creative vtext prompt missing %q:\n%s", want, request)
		}
	}
}

func TestVTextPromptCurrentEventsRequiresResearcher(t *testing.T) {
	prompt := "what's going on with iran deal now"
	if vtextPromptAllowsUngroundedCreativeDraft(prompt) {
		t.Fatalf("%q should require grounded research", prompt)
	}

	current := types.Revision{
		DocID:      "doc-current-events",
		RevisionID: "rev-current-events",
		Content:    prompt,
		AuthorKind: types.AuthorAppAgent,
	}
	request := buildAgentRevisionRequest(current, nil, nil, vtextAgentRevisionRequest{
		Intent: "initial_conductor_workflow",
		Prompt: prompt,
	}, "", false, false, nil, nil)

	for _, want := range []string{
		"For factual/current claims, call spawn_agent with role=\"researcher\"",
		"Ordinary factual, current-events, web, or \"what is going on now\" questions are research work, not super work",
		"Do not route them to request_super_execution",
	} {
		if !strings.Contains(request, want) {
			t.Fatalf("current-events vtext prompt missing %q:\n%s", want, request)
		}
	}
}
