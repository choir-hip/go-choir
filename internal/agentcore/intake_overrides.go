package agentcore

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Corrections (L2a A3 / R2-5,
// docs/archive/reports/l2a-synthesis-intake-design-2026-10-14.md). The runtime
// compares each of management's dispositions with the capture's newest
// label. A mismatch is recorded as a choir.capture_label_override object
// linked to the label version it corrects, with management's reason; label
// versions are never edited. The latest corrections become worked examples
// in Jev's state.

// captureDispositionOpen marks a capture management opened a new story
// from (a Cast to texture:new naming it).
const captureDispositionOpen = "open"

// overrideDisposition says in words what management did instead.
func overrideDisposition(o store.CaptureLabelOverride) string {
	switch o.Disposition {
	case types.DispositionFeed:
		return "fed it into the story '" + o.Headline + "'"
	case captureDispositionOpen:
		return "opened a new story '" + o.Headline + "'"
	case types.DispositionDecline:
		return "declined it"
	}
	return o.Disposition
}

// labelAgrees reports whether a disposition matches the label: feeding
// into the story the label named (or with the sibling it named), opening a
// story for a capture labelled new or sibling, or declining a capture the
// label did not place in a story.
func labelAgrees(label store.CaptureLabel, disposition, docID string, together []string) bool {
	switch disposition {
	case types.DispositionFeed:
		switch label.Event {
		case store.CaptureLabelEventStory:
			return label.EventDocID == docID
		case store.CaptureLabelEventSibling:
			for _, id := range together {
				if id == label.EventCaptureID {
					return true
				}
			}
		}
		return false
	case captureDispositionOpen, types.DispositionDecline:
		return label.Event != store.CaptureLabelEventStory
	}
	return true
}

// recordLabelOverrides records an override for every committed
// disposition that disagrees with its capture's label. receipts align with
// records by record id.
func (rt *Runtime) recordLabelOverrides(ctx context.Context, ownerID, computerID string, records []types.CommitmentRecord, receipts []types.InboxRecordReceipt) {
	if rt == nil || rt.store == nil || len(records) == 0 {
		return
	}
	inboxOf := map[string]string{}
	for _, r := range receipts {
		inboxOf[r.RecordID] = r.Inbox
	}
	enabled, checked := false, false
	for _, rec := range records {
		var disposition, docID, headline, reason string
		var captures []string
		switch {
		case rec.Disposition != nil:
			disposition, captures, reason = rec.Disposition.Kind, rec.Disposition.Captures, rec.Disposition.Reason
			if disposition == types.DispositionFeed {
				docID = strings.TrimPrefix(firstNonEmpty(inboxOf[rec.RecordID], rec.Inbox), agentprofile.Texture+":")
			}
		case rec.IsCast() && rec.Cast != nil && len(rec.Cast.Captures) > 0 && strings.TrimSpace(rec.Inbox) == types.StoryOpenInbox:
			disposition, captures = captureDispositionOpen, rec.Cast.Captures
			docID = strings.TrimPrefix(inboxOf[rec.RecordID], agentprofile.Texture+":")
			// An opening carries no reason of its own; its headline says
			// what management judged the event to be.
			if rec.Directive != nil {
				headline = rec.Directive.Objective
			}
		default:
			continue
		}
		if !checked {
			enabled, checked = rt.intakeLabelsEnabled(ctx, ownerID, computerID), true
		}
		if !enabled {
			return
		}
		if disposition == types.DispositionFeed && docID != "" {
			if doc, err := rt.store.GetLifecycleDocument(ctx, ownerID, computerID, docID); err == nil {
				headline = doc.Title
			}
		}
		for _, captureID := range captures {
			label, found, err := rt.store.LatestCaptureLabel(ctx, ownerID, computerID, captureID)
			if err != nil {
				log.Printf("label overrides: read the label of %s: %v", captureID, err)
				continue
			}
			if !found || labelAgrees(label, disposition, docID, captures) {
				continue
			}
			o := store.CaptureLabelOverride{
				OwnerID: ownerID, ComputerID: computerID, CaptureID: captureID, LabelVersion: label.Version,
				CaptureTitle: label.Title, JevSaid: captureLabelView(label).Says, Disposition: disposition,
				DocID: docID, Headline: headline, RecordID: rec.RecordID, Reason: strings.TrimSpace(reason), CreatedAt: time.Now().UTC(),
			}
			if err := rt.store.PutCaptureLabelOverride(ctx, o); err != nil {
				log.Printf("label overrides: record the override of %s (label v%d) by %s: %v", captureID, label.Version, rec.RecordID, err)
			}
		}
	}
}
