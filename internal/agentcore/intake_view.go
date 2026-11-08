package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Management sees labels and the whole library (L2a A2 / R2-6,
// docs/archive/reports/l2a-synthesis-intake-design-2026-10-14.md). Labels join a
// captures record's view where the store is (loadCellRecords), naming the
// version shown; inboxRecordView stays a pure function of the record.

const (
	storiesDefaultLimit = 50
	storiesMaxLimit     = 200
)

// captureLabelViews reads the newest label of each item.
func (rt *Runtime) captureLabelViews(ctx context.Context, ownerID, computerID string, items []types.CaptureArrival) map[string]yaegikernel.CaptureLabelView {
	out := make(map[string]yaegikernel.CaptureLabelView, len(items))
	for _, item := range items {
		label, found, err := rt.store.LatestCaptureLabel(ctx, ownerID, computerID, item.CaptureID)
		if err != nil || !found {
			out[item.CaptureID] = yaegikernel.CaptureLabelView{Says: "unlabelled"}
			continue
		}
		out[item.CaptureID] = captureLabelView(label)
	}
	return out
}

// captureLabelView renders one label for management.
func captureLabelView(l store.CaptureLabel) yaegikernel.CaptureLabelView {
	v := yaegikernel.CaptureLabelView{Version: l.Version, Event: l.Event, Headline: l.EventHeadline, SameEventAs: l.EventCaptureID, P: l.EventP, TopicHeadline: l.TopicHeadline}
	if l.EventDocID != "" {
		v.Story = deskmanifest.DocumentInbox(l.EventDocID)
	}
	if l.TopicDocID != "" {
		v.TopicStory = deskmanifest.DocumentInbox(l.TopicDocID)
	}
	switch {
	case l.Model == intakeNoCandidatesModel:
		v.Says = "new: there was no story or other new capture to compare it with"
	case l.Event == store.CaptureLabelEventStory:
		v.Says = fmt.Sprintf("Jev: same event as %s '%s' (p %.2f)", v.Story, l.EventHeadline, l.EventP)
	case l.Event == store.CaptureLabelEventSibling:
		v.Says = fmt.Sprintf("Jev: same event as new capture %s (p %.2f)", l.EventCaptureID, l.EventP)
	default:
		v.Says = fmt.Sprintf("Jev: new (p %.2f)", l.EventP)
		if v.TopicStory != "" {
			v.Says = fmt.Sprintf("Jev: new; same topic as %s '%s' (p %.2f)", v.TopicStory, l.TopicHeadline, l.EventP)
		}
	}
	return v
}

// storiesEgress serves ActionStories for a cell whose desk holds the
// intake.read grant (management and its task today; MA4.9: the grant, not the
// desk's name, admits it). The grant is the session's desk's, the worker the
// frame came from, never the role of the run whose context the cell carries
// (MA6.3: the run's role is not the frame's authority).
func (rt *Runtime) storiesEgress(ctx context.Context, deskRole string, exec toolregistry.ExecutionContext, payload json.RawMessage) (json.RawMessage, error) {
	role := strings.TrimSpace(deskRole)
	if !deskmanifest.Holds(role, "intake.read") {
		return nil, fmt.Errorf("the story library is read under the intake.read grant, which %q does not hold", role)
	}
	if rt == nil || rt.store == nil {
		return nil, fmt.Errorf("stories host unavailable")
	}
	owner, computer := strings.TrimSpace(exec.OwnerID), strings.TrimSpace(exec.ComputerID)
	if !rt.intakeSurfacesEnabled(ctx, owner, computer) {
		return nil, fmt.Errorf("choir.Stories is off on this computer (the owner's intake.labels and intake.event_wakes settings); list documents with choir.ProductAPI(\"GET\", \"/api/texture/documents\", \"\")")
	}
	var req yaegikernel.StoriesPayload
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &req); err != nil {
			return nil, fmt.Errorf("decode stories request: %w", err)
		}
	}
	limit := req.Limit
	if limit <= 0 {
		limit = storiesDefaultLimit
	}
	if limit > storiesMaxLimit {
		limit = storiesMaxLimit
	}
	// The cursor is a page's Next: the last story's update time and doc
	// id, so stories updated at the same instant are neither repeated nor
	// skipped. A bare time is accepted too.
	var before time.Time
	var beforeDoc string
	if b := strings.TrimSpace(req.Before); b != "" {
		at, doc, _ := strings.Cut(b, "|")
		var err error
		if before, err = time.Parse(time.RFC3339Nano, at); err != nil {
			return nil, fmt.Errorf("before %q is not a page's Next; pass the Next of the page before", b)
		}
		beforeDoc = doc
	}
	// Management reads the story index Texture publishes, never Texture's
	// documents or revisions (X8): the custodian reconciles it first.
	if err := rt.store.PublishStoryIndex(ctx, owner, computer); err != nil {
		return nil, fmt.Errorf("publish the story index: %w", err)
	}
	if before.IsZero() {
		// The newest page: the run has now seen the library's head, which
		// its texture:new commits carry (management delegation R2-5).
		rt.noteStoryLibraryShown(ctx, exec.RunID, owner, computer)
	}
	sums, err := rt.store.ListStoryIndex(ctx, owner, computer, before, beforeDoc, limit+1)
	if err != nil {
		return nil, err
	}
	page := yaegikernel.StoriesPage{Stories: []yaegikernel.StoryEntry{}}
	more := len(sums) > limit
	if more {
		sums = sums[:limit]
	}
	for _, sum := range sums {
		page.Stories = append(page.Stories, yaegikernel.StoryEntry{DocID: sum.DocID, Inbox: deskmanifest.DocumentInbox(sum.DocID),
			Headline: sum.Headline, Lede: sum.Lede, UpdatedAt: sum.UpdatedAt.UTC().Format(time.RFC3339Nano), Version: sum.Version})
	}
	if more && len(sums) > 0 {
		last := sums[len(sums)-1]
		page.Next = last.UpdatedAt.UTC().Format(time.RFC3339Nano) + "|" + last.DocID
	}
	if before.IsZero() {
		page.Held = rt.heldCapturesTally(owner, computer)
	}
	return json.Marshal(page)
}
