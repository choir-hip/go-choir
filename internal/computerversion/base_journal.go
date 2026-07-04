package computerversion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/yusefmosiah/go-choir/internal/base/journal"
	"github.com/yusefmosiah/go-choir/internal/base/model"
)

// BaseJournalExtractor extracts observations from a live Journal interface
// without mutating it. It uses the journal's own chain verifier, then rechecks
// the returned entries before deriving observations.
type BaseJournalExtractor struct {
	Journal journal.Journal
}

var _ Extractor = BaseJournalExtractor{}

// Extract verifies the journal, reads its entries, and extracts an observation
// set from the committed tape.
func (e BaseJournalExtractor) Extract(ctx context.Context, request ExtractRequest) (ObservationSet, error) {
	if err := ctx.Err(); err != nil {
		return ObservationSet{}, err
	}
	if e.Journal == nil {
		return ObservationSet{}, fmt.Errorf("base journal extraction: nil journal")
	}
	if err := e.Journal.VerifyChain(); err != nil {
		return ObservationSet{}, fmt.Errorf("base journal extraction: verify journal: %w", err)
	}
	return BaseJournalEntriesObservationSet(request.Name, request.Version, e.Journal.Entries())
}

// BaseJournalEntryExtractor extracts observations from verified Base journal
// entries. It is still non-runtime and in-memory, but it validates the same
// tamper-evident entry chain that the Base journal stores.
type BaseJournalEntryExtractor struct {
	Entries []journal.Entry
}

var _ Extractor = BaseJournalEntryExtractor{}

// Extract verifies the entry chain, derives committed Base events, and returns
// a file-manifest ObservationSet.
func (e BaseJournalEntryExtractor) Extract(ctx context.Context, request ExtractRequest) (ObservationSet, error) {
	if err := ctx.Err(); err != nil {
		return ObservationSet{}, err
	}
	return BaseJournalEntriesObservationSet(request.Name, request.Version, e.Entries)
}

// BaseJournalEntriesObservationSet verifies tamper-evident Base journal entries
// before deriving their ObservationSet.
func BaseJournalEntriesObservationSet(name string, version ComputerVersion, entries []journal.Entry) (ObservationSet, error) {
	ordered, err := verifyBaseJournalEntries(entries)
	if err != nil {
		return ObservationSet{}, err
	}
	return BaseEventJournalObservationSet(name, version, journal.Events(ordered))
}

func verifyBaseJournalEntries(entries []journal.Entry) ([]journal.Entry, error) {
	ordered := make([]journal.Entry, len(entries))
	copy(ordered, entries)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Event.CursorSeq < ordered[j].Event.CursorSeq })

	byID := make(map[model.EventID]journal.Entry, len(ordered))
	lastByItem := make(map[model.ItemID]model.EventID)
	seenEvents := make(map[model.EventID]struct{}, len(ordered))
	var lastSeq int64
	for i, entry := range ordered {
		event := entry.Event
		if !event.Valid() {
			return nil, fmt.Errorf("base journal extraction: invalid event %q", event.EventID)
		}
		if event.CursorSeq <= 0 {
			return nil, fmt.Errorf("base journal extraction: event %q has non-committed cursor %d", event.EventID, event.CursorSeq)
		}
		if i > 0 && event.CursorSeq <= lastSeq {
			return nil, fmt.Errorf("base journal extraction: cursor seq %d is not greater than previous %d", event.CursorSeq, lastSeq)
		}
		lastSeq = event.CursorSeq
		if _, ok := seenEvents[event.EventID]; ok {
			return nil, fmt.Errorf("base journal extraction: duplicate event id %q", event.EventID)
		}
		seenEvents[event.EventID] = struct{}{}

		expectedParent := lastByItem[event.ItemID]
		if event.ParentEventID != expectedParent {
			return nil, fmt.Errorf("base journal extraction: event %q parent %q does not match expected %q", event.EventID, event.ParentEventID, expectedParent)
		}

		parentHash := ""
		if expectedParent != "" {
			parent, ok := byID[expectedParent]
			if !ok {
				return nil, fmt.Errorf("base journal extraction: parent event %q not found", expectedParent)
			}
			parentHash = parent.Hash
		}
		want := baseJournalEntryHash(event, parentHash)
		if entry.Hash != want {
			return nil, fmt.Errorf("base journal extraction: hash mismatch for event %q", event.EventID)
		}

		byID[event.EventID] = entry
		lastByItem[event.ItemID] = event.EventID
	}
	return ordered, nil
}

func baseJournalEntryHash(event model.Event, parentHash string) string {
	encoded, err := json.Marshal(event)
	if err != nil {
		encoded = []byte(fmt.Sprintf("%v", event))
	}
	digest := sha256.New()
	_, _ = digest.Write([]byte(parentHash))
	_, _ = digest.Write(encoded)
	return hex.EncodeToString(digest.Sum(nil))
}
