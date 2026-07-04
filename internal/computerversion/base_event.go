package computerversion

import (
	"context"
	"fmt"

	"github.com/yusefmosiah/go-choir/internal/base/model"
	basetree "github.com/yusefmosiah/go-choir/internal/base/tree"
)

// BaseEventExtractor extracts observations from a typed Choir Base event tape.
// It is pure: callers provide the events, and extraction derives a tree snapshot
// without filesystem, database, network, clock, random, or hypervisor access.
type BaseEventExtractor struct {
	Events []model.Event
}

var _ Extractor = BaseEventExtractor{}

// Extract derives a Base tree from the event tape and converts it into a
// file-manifest ObservationSet for request.Version.
func (e BaseEventExtractor) Extract(ctx context.Context, request ExtractRequest) (ObservationSet, error) {
	if err := ctx.Err(); err != nil {
		return ObservationSet{}, err
	}
	return BaseEventJournalObservationSet(request.Name, request.Version, e.Events)
}

// BaseEventJournalObservationSet derives a Base tree from committed Base events
// and converts that tree into an ObservationSet. CursorSeq must be positive and
// unique so the artifact-program ref can name a committed tape cursor rather
// than an unordered fixture bag.
func BaseEventJournalObservationSet(name string, version ComputerVersion, events []model.Event) (ObservationSet, error) {
	if err := validateBaseEventTape(events); err != nil {
		return ObservationSet{}, err
	}
	return BaseTreeObservationSet(name, version, basetree.Derive(events))
}

func validateBaseEventTape(events []model.Event) error {
	seenEvents := make(map[model.EventID]struct{}, len(events))
	seenCursors := make(map[int64]struct{}, len(events))
	for _, event := range events {
		if !event.Valid() {
			return fmt.Errorf("base event extraction: invalid event %q", event.EventID)
		}
		if event.CursorSeq <= 0 {
			return fmt.Errorf("base event extraction: event %q has non-committed cursor %d", event.EventID, event.CursorSeq)
		}
		if _, ok := seenEvents[event.EventID]; ok {
			return fmt.Errorf("base event extraction: duplicate event id %q", event.EventID)
		}
		seenEvents[event.EventID] = struct{}{}
		if _, ok := seenCursors[event.CursorSeq]; ok {
			return fmt.Errorf("base event extraction: duplicate cursor seq %d", event.CursorSeq)
		}
		seenCursors[event.CursorSeq] = struct{}{}
	}
	return nil
}
