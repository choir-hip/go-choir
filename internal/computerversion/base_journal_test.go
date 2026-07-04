package computerversion

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/base/journal"
)

func TestBaseJournalExtractorReadsMemJournal(t *testing.T) {
	version := baseSliceComputerVersion()
	extractor := BaseJournalExtractor{Journal: baseMemJournal(t, "a", "b")}

	observationSet, err := extractor.Extract(context.Background(), ExtractRequest{Name: "mem-journal", Version: version})
	if err != nil {
		t.Fatalf("extract mem journal: %v", err)
	}
	realization, err := (ProjectionMaterializer{ID: "mem-journal-projection", Observations: observationSet}).Materialize(context.Background(), version, CapabilityManifest{
		Materializer: "mem-journal-projection",
		Substrate:    "base-mem-journal",
		Supported:    []ObservationKind{ObservationFileManifest},
	})
	if err != nil {
		t.Fatalf("materialize mem journal observation: %v", err)
	}
	if realization.Observations.Name != "mem-journal" {
		t.Fatalf("expected observation label to survive extraction, got %q", realization.Observations.Name)
	}
}

func TestBaseJournalExtractorRejectsNilJournal(t *testing.T) {
	_, err := (BaseJournalExtractor{}).Extract(context.Background(), ExtractRequest{Name: "nil", Version: baseSliceComputerVersion()})
	if err == nil {
		t.Fatal("expected nil journal to be rejected")
	}
}

func TestBaseJournalExtractorReadsSQLiteJournal(t *testing.T) {
	version := baseSliceComputerVersion()
	path := t.TempDir() + "/base-journal.sqlite"
	j, err := journal.NewSQLiteJournal(path)
	if err != nil {
		t.Fatalf("open sqlite journal: %v", err)
	}
	defer func() {
		if err := j.Close(); err != nil {
			t.Fatalf("close sqlite journal: %v", err)
		}
	}()
	if _, err := j.Append(baseCreateEvent(1, "a")); err != nil {
		t.Fatalf("append create: %v", err)
	}
	if _, err := j.Append(baseUpdateEvent(2, "b")); err != nil {
		t.Fatalf("append update: %v", err)
	}

	observationSet, err := (BaseJournalExtractor{Journal: j}).Extract(context.Background(), ExtractRequest{Name: "sqlite-journal", Version: version})
	if err != nil {
		t.Fatalf("extract sqlite journal: %v", err)
	}
	realization, err := (ProjectionMaterializer{ID: "sqlite-journal-projection", Observations: observationSet}).Materialize(context.Background(), version, CapabilityManifest{
		Materializer: "sqlite-journal-projection",
		Substrate:    "base-sqlite-journal",
		Supported:    []ObservationKind{ObservationFileManifest},
	})
	if err != nil {
		t.Fatalf("materialize sqlite journal observation: %v", err)
	}
	if len(realization.Observations.Observations) != 1 {
		t.Fatalf("expected one live file observation, got %d", len(realization.Observations.Observations))
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

func baseMemJournal(t *testing.T, createHashSuffix, updateHashSuffix string) *journal.MemJournal {
	t.Helper()
	j := journal.NewMemJournal()
	if _, err := j.Append(baseCreateEvent(1, createHashSuffix)); err != nil {
		t.Fatalf("append create: %v", err)
	}
	if _, err := j.Append(baseUpdateEvent(2, updateHashSuffix)); err != nil {
		t.Fatalf("append update: %v", err)
	}
	return j
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
