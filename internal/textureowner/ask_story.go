package textureowner

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Ask about a story no one is covering (Lane F, handoff section 4.F, done
// item 7). The ask is the existing prompt-bar submit, unrouted: it starts a
// new Texture document on the asker's own computer whose first revision is the
// asker's words, and wakes the Texture desk, which researches (the researcher
// cast) and commits the document's next revisions as findings land. What is
// new is only the record that this was a story ask (run metadata ask_intent and
// the revision's asked=story) and a first objective that tells the desk to
// investigate. No second orchestration path.

const (
	// askStoryMaxBytes caps the question.
	askStoryMaxBytes = 8 << 10
	// askIntentStory is the run metadata value of a story ask.
	askIntentStory = "story"
	// askIntentMetadataKey is the run metadata key that carries it.
	askIntentMetadataKey = "ask_intent"
)

type askStoryRequest struct {
	Text      string `json:"text"`
	CommandID string `json:"command_id,omitempty"`
}

type askStoryContextKey struct{}

func withStoryAsk(ctx context.Context) context.Context {
	return context.WithValue(ctx, askStoryContextKey{}, true)
}

func isStoryAsk(ctx context.Context) bool {
	v, _ := ctx.Value(askStoryContextKey{}).(bool)
	return v
}

// storyAskObjective is the Texture desk's first objective for a story ask. The
// question is the owner's, quoted; the rest says what to do with it.
func storyAskObjective(question string) string {
	return "The owner asked about a story the paper may not cover. Investigate it: " +
		"open the researcher on the world's evidence, then write the document from what is found. " +
		"Commit a first version as soon as you have something sourced, and revise it as more evidence lands; " +
		"say plainly what is established, what is not, and what you could not find. " +
		"The owner's question, verbatim:\n\n" + question
}

// HandleAskStory serves POST /api/autopaper/ask: the owner's request, once the
// prompt bar has classed it as a request to ask for a story.
func (h *Handler) HandleAskStory(w http.ResponseWriter, r *http.Request) {
	r = asOwnerRoute(r)
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	var req askStoryRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, askStoryMaxBytes+4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeAPIJSON(w, http.StatusRequestEntityTooLarge, apiError{Error: "the question is too long"})
			return
		}
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid ask request"})
		return
	}
	text := strings.TrimSpace(req.Text)
	if text == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "text is required"})
		return
	}
	if len(text) > askStoryMaxBytes {
		writeAPIJSON(w, http.StatusRequestEntityTooLarge, apiError{Error: "the question is too long"})
		return
	}
	commandID := strings.TrimSpace(req.CommandID)
	if commandID == "" {
		commandID = uuid.NewString()
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), promptBarCommitTimeout)
	defer cancel()
	ctx = withStoryAsk(ownerTextContext(ctx, r))
	rec, handoff, err := h.submitPromptBar(ctx, ownerID, authenticatedUserEmail(r), text, commandID, false, false)
	if err != nil {
		status, message := promptBarSubmitFailure(err)
		if status != http.StatusConflict && status != http.StatusServiceUnavailable {
			log.Printf("texture ask story: submit: %v", err)
		}
		writeAPIJSON(w, status, apiError{Error: message})
		return
	}
	if handoff.Conductor.CommandID == "" {
		handoff.Conductor.CommandID = commandID
	}
	writeAPIJSON(w, http.StatusAccepted, promptBarSubmitResponse{
		Schema: types.DurableWorkSchemaV1, SubmissionID: rec.RunID, State: rec.State,
		CreatedAt: rec.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
		StatusURL: "/api/prompt-bar/submissions/" + rec.RunID,
		CommandID: handoff.Conductor.CommandID, TrajectoryID: handoff.Conductor.TrajectoryID,
		StartRequestDigest: handoff.Conductor.StartRequestDigest,
		DocID:              handoff.Conductor.DocID, RevisionID: handoff.Conductor.UserRevisionID,
		SubjectID: handoff.Conductor.SubjectID, ObligationIDs: handoff.Conductor.ObligationIDs,
		ReducerSeq: handoff.Conductor.ReducerSeq, SnapshotCursor: handoff.Conductor.SnapshotCursor,
	})
}
