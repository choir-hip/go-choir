package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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

// storyEntitiesBackfillScanMax bounds the stories one act reads (review
// 325 F2); the result's Next is the cursor to go on from.
var storyEntitiesBackfillScanMax = 2000

// storyEntitiesBackfillWallMax bounds one act's wall time, from its start,
// so its receipt is written inside the server's default 120 s write
// timeout (internal/server; the guest sets none);
// past it the act stops after the story in hand and returns Next.
var storyEntitiesBackfillWallMax = 90 * time.Second

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
	// Empty: stories skipped because their head has no text yet.
	Empty int `json:"empty"`
	// Gone: stories that went away during the act.
	Gone int `json:"gone"`
	// Scanned: stories read.
	Scanned int `json:"scanned"`
	// Next is where the next act goes on from, when this one stopped
	// before the oldest story.
	Next *StoryEntitiesBackfillCursor `json:"next,omitempty"`
}

// StoryEntitiesBackfillCursor names where an act goes on from: stories
// updated before Before, or at Before with a doc id after BeforeDoc.
type StoryEntitiesBackfillCursor struct {
	Before    time.Time `json:"before"`
	BeforeDoc string    `json:"before_doc"`
}

// StoryEntitiesBackfill runs one batch of at most limit asks from the
// newest story.
func (rt *Runtime) StoryEntitiesBackfill(ctx context.Context, owner, computer, batchID string, limit int) (StoryEntitiesBackfillResult, error) {
	return rt.StoryEntitiesBackfillFrom(ctx, owner, computer, batchID, limit, StoryEntitiesBackfillCursor{})
}

// StoryEntitiesBackfillFrom runs one batch of at most limit asks, reading
// at most storyEntitiesBackfillScanMax stories from the cursor, for at most
// storyEntitiesBackfillWallMax.
func (rt *Runtime) StoryEntitiesBackfillFrom(ctx context.Context, owner, computer, batchID string, limit int, from StoryEntitiesBackfillCursor) (StoryEntitiesBackfillResult, error) {
	var ph storyEntitiesBackfillPhases
	start := time.Now()
	out, err := rt.storyEntitiesBackfillFrom(ctx, owner, computer, batchID, limit, from, start, &ph)
	if ph.stop == "" {
		ph.stop = "error"
	}
	log.Printf("related review: batch %q: tape %s (%d events), list %s (%d stories), loop %s, scanned %d, requested %d, stop %s, total %s, err=%v",
		out.BatchID, ph.tape.Round(time.Millisecond), ph.events, ph.list.Round(time.Millisecond), ph.stories, ph.loop.Round(time.Millisecond),
		out.Scanned, len(out.Requested), ph.stop, time.Since(start).Round(time.Millisecond), err)
	return out, err
}

// storyEntitiesBackfillPhases is where one act's wall time went, for its
// log line (docs/problems/related-review-act-outlives-its-request-2026-10-27.md).
type storyEntitiesBackfillPhases struct {
	tape, list, loop time.Duration
	events, stories  int
	// stop: limit, scan, wall, end; empty when the act failed.
	stop string
}

func (rt *Runtime) storyEntitiesBackfillFrom(ctx context.Context, owner, computer, batchID string, limit int, from StoryEntitiesBackfillCursor, start time.Time, ph *storyEntitiesBackfillPhases) (StoryEntitiesBackfillResult, error) {
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
	t0 := time.Now()
	tape, err := rt.system1ChannelTape(ctx, owner, computer)
	ph.tape, ph.events = time.Since(t0), len(tape)
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
	scanMax := max(storyEntitiesBackfillScanMax, 1)
	// One more than the act may read tells whether older stories remain.
	t0 = time.Now()
	docs, err := rt.store.ListStoryDocuments(ctx, owner, computer, from.Before, from.BeforeDoc, scanMax+1)
	ph.list, ph.stories = time.Since(t0), len(docs)
	if err != nil {
		return out, fmt.Errorf("related review: list the stories: %w", err)
	}
	t0 = time.Now()
	defer func() { ph.loop = time.Since(t0) }()
	for i, doc := range docs {
		if i > 0 {
			switch {
			case time.Since(start) >= storyEntitiesBackfillWallMax:
				ph.stop = "wall"
			case len(out.Requested) >= limit:
				ph.stop = "limit"
			case out.Scanned >= scanMax:
				ph.stop = "scan"
			}
		}
		if ph.stop != "" {
			// Stopped before the oldest: go on after the last story read.
			last := docs[i-1]
			out.Next = &StoryEntitiesBackfillCursor{Before: last.UpdatedAt, BeforeDoc: last.DocID}
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
			// A story with no text yet (its opening seed) cannot be
			// analysed; asking it would ask it again every batch.
			if empty, err := rt.storyHeadEmpty(ctx, owner, computer, doc); err != nil {
				return out, fmt.Errorf("related review: read the head of %s: %w", doc.DocID, err)
			} else if empty {
				out.Empty++
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
			out.Gone++
			continue
		case err != nil:
			return out, fmt.Errorf("related review: ask %s: %w", doc.DocID, err)
		}
		out.Requested = append(out.Requested, doc.DocID)
		if receipt.Replay {
			out.Replayed++
		}
	}
	if ph.stop == "" {
		ph.stop = "end"
	}
	return out, nil
}

// storyHeadEmpty reports whether a story's head revision has no text.
func (rt *Runtime) storyHeadEmpty(ctx context.Context, owner, computer string, doc types.Document) (bool, error) {
	if strings.TrimSpace(doc.CurrentRevisionID) == "" {
		return true, nil
	}
	rev, err := rt.store.GetLifecycleRevision(ctx, owner, computer, doc.CurrentRevisionID)
	if errors.Is(err, store.ErrNotFound) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(rev.Content) == "", nil
}
