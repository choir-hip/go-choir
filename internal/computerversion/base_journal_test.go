package computerversion

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/base/journal"
)

func TestBaseJournalExtractorRejectsNilJournal(t *testing.T) {
	_, err := (BaseJournalExtractor{}).Extract(context.Background(), ExtractRequest{Name: "nil", Version: baseSliceComputerVersion()})
	if err == nil {
		t.Fatal("expected nil journal to be rejected")
	}
}

func TestBaseJournalEntryExtractorVerifiesChainAndExtracts(t *testing.T) {
	version := baseSliceComputerVersion()
	entries := baseJournalEntries(t, "a", "b")
	shuffled := []journal.Entry{entries[1], entries[0]}

	leftSet, err := (BaseJournalEntryExtractor{Entries: entries}).Extract(context.Background(), ExtractRequest{Name: "entries", Version: version})
	if err != nil {
		t.Fatalf("left extract: %v", err)
	}
	rightSet, err := (BaseJournalEntryExtractor{Entries: shuffled}).Extract(context.Background(), ExtractRequest{Name: "shuffled", Version: version})
	if err != nil {
		t.Fatalf("right extract: %v", err)
	}
	manifest := CapabilityManifest{
		Materializer: "base-journal-entry-extractor",
		Substrate:    "base-journal-entry-tape",
		Supported:    []ObservationKind{ObservationFileManifest},
	}

	result := EquivalenceChecker{}.CheckRealizations(
		Realization{ID: "entries", Version: version, Capabilities: manifest, Observations: leftSet},
		Realization{ID: "shuffled", Version: version, Capabilities: manifest, Observations: rightSet},
	)
	if !result.Equivalent() {
		t.Fatalf("expected verified journal entries to extract equivalently, got %#v", result)
	}
}

func TestBaseJournalEntryExtractorRejectsTamperedHash(t *testing.T) {
	entries := baseJournalEntries(t, "a", "b")
	entries[1].Event = baseUpdateEvent(2, "c")

	_, err := BaseJournalEntriesObservationSet("tampered", baseSliceComputerVersion(), entries)
	if err == nil {
		t.Fatal("expected tampered entry payload to be rejected")
	}
}

func TestBaseJournalEntryExtractorRejectsBrokenParent(t *testing.T) {
	entries := baseJournalEntries(t, "a", "b")
	entries[1].Event.ParentEventID = "base_evt_not_parent"

	_, err := BaseJournalEntriesObservationSet("broken-parent", baseSliceComputerVersion(), entries)
	if err == nil {
		t.Fatal("expected broken parent link to be rejected")
	}
}

func baseJournalEntries(t *testing.T, createHashSuffix, updateHashSuffix string) []journal.Entry {
	t.Helper()
	j := journal.NewMemJournal()
	if _, err := j.Append(baseCreateEvent(1, createHashSuffix)); err != nil {
		t.Fatalf("append create: %v", err)
	}
	if _, err := j.Append(baseUpdateEvent(2, updateHashSuffix)); err != nil {
		t.Fatalf("append update: %v", err)
	}
	entries := j.Entries()
	if len(entries) != 2 {
		t.Fatalf("expected two entries, got %d", len(entries))
	}
	return entries
}
