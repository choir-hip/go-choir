package textureowner

// Ways the wake prompt for an owner revision can misdescribe the document
// (problem note story-expansion-request-no-reader-door-and-wrong-wake-
// framing-2026-10-20, P2), written before the fix:
//
//  1. A story's owner /revise carries the whole story forward as an
//     AuthorUser revision stamped input_origin=user_prompt, with a parent.
//     The wake still says "the owner's request itself, not prose to keep:
//     write the first useful document that answers it", so Texture is told
//     to discard a finished story. The sentence must not appear when the
//     owner revision has a parent.
//  2. The fix over-corrects and drops the sentence for a seed request, the
//     first revision of a document the owner opened with a prompt (no
//     parent). That path must keep it: the request is all there is.
//  3. The fix loses the owner's direction: the direction block must still
//     appear for a carry-forward revise (it is what the owner asked for).
//  4. A human edit in the editor whose metadata carried input_origin forward
//     from an earlier prompt is also AuthorUser with a parent. It must read
//     as an edit, never as a seed request.

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

const seedRequestSentence = "not prose to keep"

func ownerRevisionMetadata(t *testing.T, ownerPrompt string) map[string]any {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"input_origin": textureInputOriginUserPrompt,
		"owner_prompt": ownerPrompt,
	})
	if err != nil {
		t.Fatal(err)
	}
	return decodeRevisionMetadata(raw)
}

func TestWakePromptDoesNotCallACarriedForwardStoryTheOwnersRequest(t *testing.T) {
	story := "# Harbor ferries resume\n\nThe ferries resumed on Monday [1][2].\n"
	parent := types.Revision{RevisionID: "rev-texture-3", AuthorKind: types.AuthorAppAgent, Content: story}
	carried := types.Revision{RevisionID: "rev-owner-4", AuthorKind: types.AuthorUser, Content: story, ParentRevisionID: parent.RevisionID}
	got := buildAgentRevisionRequest(carried, &parent, ownerRevisionMetadata(t, "[expansion rx-1] more research"), textureAgentRevisionRequest{}, "", nil)
	if strings.Contains(got, seedRequestSentence) {
		t.Fatalf("a carried-forward story was called the owner's request itself:\n%s", got)
	}
	if !strings.Contains(got, "The owner's direction for this revision:\n[expansion rx-1] more research") {
		t.Fatalf("the owner's direction is missing from the wake:\n%s", got)
	}
}

func TestWakePromptStillCallsASeedOwnerRequestTheRequest(t *testing.T) {
	seed := types.Revision{RevisionID: "rev-owner-1", AuthorKind: types.AuthorUser, Content: "Write about harbor ferries."}
	got := buildAgentRevisionRequest(seed, nil, ownerRevisionMetadata(t, ""), textureAgentRevisionRequest{}, "", nil)
	if !strings.Contains(got, seedRequestSentence) {
		t.Fatalf("a seed owner request lost its framing:\n%s", got)
	}
}

func TestWakePromptReadsAnEditWithInheritedOriginAsAnEdit(t *testing.T) {
	parent := types.Revision{RevisionID: "rev-texture-3", AuthorKind: types.AuthorAppAgent, Content: "one"}
	edit := types.Revision{RevisionID: "rev-owner-4", AuthorKind: types.AuthorUser, Content: "one two", ParentRevisionID: parent.RevisionID}
	got := buildAgentRevisionRequest(edit, &parent, ownerRevisionMetadata(t, ""), textureAgentRevisionRequest{}, "+ two", nil)
	if strings.Contains(got, seedRequestSentence) || !strings.Contains(got, "The owner edited the document.") {
		t.Fatalf("an owner edit did not read as an edit:\n%s", got)
	}
}
