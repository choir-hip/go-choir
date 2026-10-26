package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The backfill (docs/problems/related-stories-no-entity-layer-2026-10-26.md,
// slice d): an owner act asks the research desks of stories with no
// story.entities record to record what their story is about, newest first,
// through the reader's request path (an owner Cast from texture:<doc> to
// research:<doc>). New stories are covered at the source by the finish
// rule; this reaches the older ones. A batch is an occurrence: it runs in
// the act, and a crash ends it; only the owner's act starts another.

// StoryEntitiesBackfillMax bounds one act. The design allowed 2,000; one
// act asks at most 200 stories (one research run, one or two System 1
// calls and one Texture turn each), inside a day's 3,000 System 1 calls.
const StoryEntitiesBackfillMax = 200

var (
	// ErrStoryEntitiesBackfillOff refuses the act while the computer's
	// research.related_story setting is off.
	ErrStoryEntitiesBackfillOff = errors.New("related review: " + store.SettingResearchRelatedStory + " is off on this computer")
	// ErrStoryEntitiesBackfillBadRequest refuses a batch with no id or a
	// limit past the bound.
	ErrStoryEntitiesBackfillBadRequest = errors.New("related review: bad request")
)

// StoryEntitiesBackfillResult is what one act did.
type StoryEntitiesBackfillResult struct {
	BatchID string `json:"batch_id"`
	// Requested: the stories asked by this batch, replays included.
	Requested []string `json:"requested"`
	// Replayed: asks this batch had already made.
	Replayed int `json:"replayed"`
	// Recorded: stories skipped because their entities are recorded.
	Recorded int `json:"recorded"`
	// Busy: stories skipped because their research desk is working.
	Busy int `json:"busy"`
	// Scanned: stories read.
	Scanned int `json:"scanned"`
}

// StoryEntitiesBackfill runs one batch of at most limit asks.
func (rt *Runtime) StoryEntitiesBackfill(ctx context.Context, owner, computer, batchID string, limit int) (StoryEntitiesBackfillResult, error) {
	owner, computer, batchID = strings.TrimSpace(owner), strings.TrimSpace(computer), strings.TrimSpace(batchID)
	out := StoryEntitiesBackfillResult{BatchID: batchID, Requested: []string{}}
	if rt == nil || rt.store == nil {
		return out, fmt.Errorf("related review: host unavailable")
	}
	if owner == "" || computer == "" {
		return out, fmt.Errorf("%w: owner and computer are required", ErrStoryEntitiesBackfillBadRequest)
	}
	if batchID == "" || limit < 1 || limit > StoryEntitiesBackfillMax {
		return out, fmt.Errorf("%w: a batch_id and a limit from 1 to %d are required", ErrStoryEntitiesBackfillBadRequest, StoryEntitiesBackfillMax)
	}
	if !rt.computerSettingOn(ctx, owner, computer, store.SettingResearchRelatedStory) {
		return out, ErrStoryEntitiesBackfillOff
	}
	tape, err := rt.system1ChannelTape(ctx, owner, computer)
	if err != nil {
		return out, fmt.Errorf("related review: read the tape: %w", err)
	}
	recorded := map[string]bool{}
	for _, ev := range tape {
		if ev.Kind != types.EventStoryEntities || ev.Phase != storyEntitiesPhaseRecorded {
			continue
		}
		var rec storyEntitiesRecord
		if json.Unmarshal(ev.Payload, &rec) == nil && (rec.Computer == "" || rec.Computer == computer) {
			recorded[rec.Story] = true
		}
	}
	docs, err := rt.store.ListStoryDocuments(ctx, owner, computer, time.Time{}, "", 0)
	if err != nil {
		return out, fmt.Errorf("related review: list the stories: %w", err)
	}
	for _, doc := range docs {
		if len(out.Requested) >= limit {
			break
		}
		out.Scanned++
		clientID := batchID + ":" + doc.DocID
		asked, err := rt.store.GetCommitmentRecord(ctx, owner, computer, store.StoryEntitiesRecordID(clientID))
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			return out, fmt.Errorf("related review: read the ask of %s: %w", doc.DocID, err)
		}
		if asked == nil {
			if recorded[types.StoryPublicationID(owner, doc.DocID)] {
				out.Recorded++
				continue
			}
			// As the reader's route does: a research run still working
			// is busy even after its round closed.
			if _, active, err := rt.TextureActiveRunByAgent(ctx, owner, computer, agentprofile.Research+":"+doc.DocID); err != nil || active {
				out.Busy++
				continue
			}
		}
		receipt, err := rt.store.CommitStoryResearchRequest(ctx, store.StoryResearchRequest{
			OwnerID: owner, ComputerID: computer, DocumentID: doc.DocID, ClientRequestID: clientID,
			ExpectedHeadRevisionID: doc.CurrentRevisionID, Kind: store.StoryResearchKindEntities,
		})
		switch {
		case errors.Is(err, store.ErrStoryResearchBusy), errors.Is(err, store.ErrStoryResearchChanged):
			out.Busy++
			continue
		case errors.Is(err, store.ErrNotFound):
			continue
		case err != nil:
			return out, fmt.Errorf("related review: ask %s: %w", doc.DocID, err)
		}
		out.Requested = append(out.Requested, doc.DocID)
		if receipt.Replay {
			out.Replayed++
		}
	}
	return out, nil
}
