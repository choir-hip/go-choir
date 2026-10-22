package agentcore

import (
	"errors"
	"slices"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
)

func TestReplayEligibilityDefaultDeniesNilHeads(t *testing.T) {
	result := computerversion.EquivalenceResult{Status: computerversion.EquivalenceEquivalent}
	eligibility := replayEligibility(nil, nil, computerversion.ObservationSet{}, computerversion.ObservationSet{}, result)
	if eligibility.Eligible {
		t.Fatal("nil-head replay was eligible")
	}
	if !errors.Is(eligibility.Error(), ErrReplayIneligible) {
		t.Fatalf("eligibility error=%v, want ErrReplayIneligible", eligibility.Error())
	}
}

func TestReplayEligibilityAcceptsMatchingNonNilHeadsAndProjection(t *testing.T) {
	head := &computerevent.Head{ComputerID: "computer", Sequence: 1, CanonicalEventHead: "head"}
	observations := replayManifestObservationSet()
	for i := range observations.Observations {
		if observations.Observations[i].Key == "dolt:texture:table:computer_event_index" {
			observations.Observations[i].Value = "content"
		}
	}
	result := computerversion.EquivalenceResult{Status: computerversion.EquivalenceEquivalent}
	eligibility := replayEligibility(head, head, observations, observations, result)
	if !eligibility.Eligible || eligibility.Error() != nil {
		t.Fatalf("matching replay was rejected: %#v (%v)", eligibility, eligibility.Error())
	}
}

func replayManifestObservationSet() computerversion.ObservationSet {
	manifest := CurrentReplayAirworthinessManifest()
	observations := make([]computerversion.Observation, 0, len(manifest.Entries)*2)
	for table, class := range manifest.Entries {
		if class == ReplayRetiredAbsent {
			continue
		}
		observations = append(observations,
			computerversion.Observation{
				Kind: computerversion.ObservationDoltHead,
				Key:  "dolt:texture:schema:" + table, Value: "schema:" + table,
			},
			computerversion.Observation{
				Kind: computerversion.ObservationDoltHead,
				Key:  "dolt:texture:table:" + table, Value: emptyDoltTableHash,
			},
		)
	}
	return computerversion.ObservationSet{Observations: observations}
}

func TestReplayEligibilityRejectsMissingDeclaredTables(t *testing.T) {
	head := &computerevent.Head{ComputerID: "computer", Sequence: 1, CanonicalEventHead: "head"}
	observations := replayManifestObservationSet()
	observations.Observations = observations.Observations[:2]
	result := computerversion.EquivalenceResult{Status: computerversion.EquivalenceEquivalent}
	eligibility := replayEligibility(head, head, observations, observations, result)
	if eligibility.Eligible || len(eligibility.MissingTables) == 0 {
		t.Fatalf("missing declared tables were accepted: %#v", eligibility)
	}
}

func TestReplayEligibilityRejectsRetiredAndUnknownTables(t *testing.T) {
	head := &computerevent.Head{ComputerID: "computer", Sequence: 1, CanonicalEventHead: "head"}
	retired := computerversion.Observation{Kind: computerversion.ObservationDoltHead, Key: "dolt:texture:table:app_adoptions", Value: "content"}
	unknown := computerversion.Observation{Kind: computerversion.ObservationDoltHead, Key: "dolt:texture:table:legacy_state", Value: "content"}
	live := computerversion.ObservationSet{Observations: []computerversion.Observation{retired, unknown}}
	replay := computerversion.ObservationSet{Observations: []computerversion.Observation{retired}}
	result := computerversion.EquivalenceResult{Status: computerversion.EquivalenceEquivalent}
	eligibility := replayEligibility(head, head, live, replay, result)
	if eligibility.Eligible {
		t.Fatal("retired/unknown table replay was eligible")
	}
	if len(eligibility.RetiredTables) != 1 || eligibility.RetiredTables[0] != "app_adoptions" {
		t.Fatalf("retired tables=%v", eligibility.RetiredTables)
	}
	if len(eligibility.UnknownTables) != 1 || eligibility.UnknownTables[0] != "legacy_state" {
		t.Fatalf("unknown tables=%v", eligibility.UnknownTables)
	}
}

func TestReplayEligibilityRejectsNonEmptyUnsupportedDirectWrites(t *testing.T) {
	head := &computerevent.Head{ComputerID: "computer", Sequence: 1, CanonicalEventHead: "head"}
	live := computerversion.ObservationSet{Observations: []computerversion.Observation{{
		Kind: computerversion.ObservationDoltHead, Key: "dolt:texture:table:user_preferences", Value: "non-empty",
	}}}
	result := computerversion.EquivalenceResult{Status: computerversion.EquivalenceEquivalent}
	eligibility := replayEligibility(head, head, live, live, result)
	if eligibility.Eligible {
		t.Fatal("non-empty direct-write table replay was eligible")
	}
	if len(eligibility.UnsupportedTables) != 1 || eligibility.UnsupportedTables[0] != "user_preferences" {
		t.Fatalf("unsupported tables=%v", eligibility.UnsupportedTables)
	}
}

func TestReplayEligibilityRejectsNonEmptyWirePublishDebounceState(t *testing.T) {
	head := &computerevent.Head{ComputerID: "computer", Sequence: 1, CanonicalEventHead: "head"}
	observations := computerversion.ObservationSet{Observations: []computerversion.Observation{
		{Kind: computerversion.ObservationDoltHead, Key: "dolt:texture:table:wire_publish_debounce_entries", Value: "pending-batch"},
		{Kind: computerversion.ObservationDoltHead, Key: "dolt:texture:table:wire_publish_debounce_state", Value: "dispatch-index"},
	}}
	result := computerversion.EquivalenceResult{Status: computerversion.EquivalenceEquivalent}
	eligibility := replayEligibility(head, head, observations, observations, result)
	if eligibility.Eligible {
		t.Fatal("non-empty durable debounce state was replay-eligible")
	}
	if got, want := eligibility.UnsupportedTables, []string{"wire_publish_debounce_entries", "wire_publish_debounce_state"}; !slices.Equal(got, want) {
		t.Fatalf("unsupported tables=%v, want %v", got, want)
	}
}

func TestReplayEligibilityRejectsSchemaDrift(t *testing.T) {
	head := &computerevent.Head{ComputerID: "computer", Sequence: 1, CanonicalEventHead: "head"}
	live := computerversion.ObservationSet{Observations: []computerversion.Observation{{
		Kind: computerversion.ObservationDoltHead, Key: "dolt:texture:schema:events", Value: "live-schema",
	}}}
	replay := computerversion.ObservationSet{Observations: []computerversion.Observation{{
		Kind: computerversion.ObservationDoltHead, Key: "dolt:texture:schema:events", Value: "replay-schema",
	}}}
	result := computerversion.EquivalenceResult{Status: computerversion.EquivalenceEquivalent}
	eligibility := replayEligibility(head, head, live, replay, result)
	if eligibility.Eligible {
		t.Fatal("schema-drift replay was eligible")
	}
	if len(eligibility.SchemaDrift) != 1 || eligibility.SchemaDrift[0] != "events" {
		t.Fatalf("schema drift=%v", eligibility.SchemaDrift)
	}
}

func TestReplayEligibilityRejectsUnknownTableEvenWhenBothProjectionsContainIt(t *testing.T) {
	head := &computerevent.Head{ComputerID: "computer", Sequence: 1, CanonicalEventHead: "head"}
	observations := computerversion.ObservationSet{Observations: []computerversion.Observation{{
		Kind: computerversion.ObservationDoltHead, Key: "dolt:texture:table:unclassified", Value: emptyDoltTableHash,
	}}}
	result := computerversion.EquivalenceResult{Status: computerversion.EquivalenceEquivalent}
	eligibility := replayEligibility(head, head, observations, observations, result)
	if eligibility.Eligible || len(eligibility.UnknownTables) != 1 || eligibility.UnknownTables[0] != "unclassified" {
		t.Fatalf("unknown table was not rejected: %#v", eligibility)
	}
}

func TestDesktopAndOGAreEventProjectionAfterLiveResidueImport(t *testing.T) {
	manifest := CurrentReplayAirworthinessManifest()
	for _, table := range []string{
		"desktop_workspaces", "desktop_sessions", "desktop_app_instances",
		"desktop_window_placements", "og_objects", "og_edges",
		"run_memory_entries", "self_development_start_intents",
		"self_development_operations", "texture_agent_mutations",
	} {
		if got := manifest.Entries[table]; got != ReplayEventProjection {
			t.Fatalf("%s class=%q, want event_projection after live residue import", table, got)
		}
	}
	for _, table := range []string{
		"agents", "runs", "channel_messages", "inbox_deliveries", "run_acceptances",
		"run_continuations", "trajectories", "work_items", "browser_sessions",
		"worker_updates", "coagent_mailboxes", "desktop_state", "computer_effective_state",
		"texture_documents", "texture_controller_checkpoints", "content_items", "podcast_subscriptions",
	} {
		if got := manifest.Entries[table]; got != ReplayRetiredAbsent {
			t.Fatalf("%s class=%q, want retired_absent", table, got)
		}
	}
	invalid := manifest
	invalid.Entries = map[string]ReplayObservationClass{"desktop_sessions": ReplayObservationClass("presence_volatile")}
	if err := invalid.Validate(); err == nil {
		t.Fatal("presence_volatile class was admitted — that would weaken the nonempty gate")
	}
}

// F12 (docs/design/choir-event-table-f-c-2026-10-22.md section 3.3): the
// optional event projection class. Absent from both sets is eligible (an
// empty table is not observed); present in both with equal hashes is
// eligible; present on one side only is schema drift.
func TestReplayEligibilityOptionalEventTable(t *testing.T) {
	head := &computerevent.Head{ComputerID: "computer", Sequence: 1, CanonicalEventHead: "head"}
	result := computerversion.EquivalenceResult{Status: computerversion.EquivalenceEquivalent}
	manifest := CurrentReplayAirworthinessManifest()
	if manifest.Version != 4 || manifest.Entries["choir_events"] != ReplayEventProjectionOptional {
		t.Fatalf("manifest version %d, choir_events %q", manifest.Version, manifest.Entries["choir_events"])
	}
	if err := manifest.Validate(); err != nil {
		t.Fatalf("Validate refuses the optional class: %v", err)
	}
	without := func() computerversion.ObservationSet {
		set := replayManifestObservationSet()
		kept := set.Observations[:0]
		for _, o := range set.Observations {
			if o.Key != "dolt:texture:schema:choir_events" && o.Key != "dolt:texture:table:choir_events" {
				kept = append(kept, o)
			}
		}
		set.Observations = kept
		return set
	}
	with := func(content string) computerversion.ObservationSet {
		set := without()
		set.Observations = append(set.Observations,
			computerversion.Observation{Kind: computerversion.ObservationDoltHead, Key: "dolt:texture:schema:choir_events", Value: "schema:choir_events"},
			computerversion.Observation{Kind: computerversion.ObservationDoltHead, Key: "dolt:texture:table:choir_events", Value: content},
		)
		return set
	}
	if e := replayEligibility(head, head, without(), without(), result); !e.Eligible {
		t.Fatalf("absent from both sets: %#v", e)
	}
	if e := replayEligibility(head, head, with("rows"), with("rows"), result); !e.Eligible {
		t.Fatalf("present in both with content: %#v", e)
	}
	if e := replayEligibility(head, head, with("rows"), without(), result); e.Eligible || !slices.Contains(e.SchemaDrift, "choir_events") {
		t.Fatalf("live only: %#v", e)
	}
	if e := replayEligibility(head, head, without(), with("rows"), result); e.Eligible || !slices.Contains(e.SchemaDrift, "choir_events") {
		t.Fatalf("replay only: %#v", e)
	}
	bad := ReplayAirworthinessManifest{Version: 4, Entries: map[string]ReplayObservationClass{"choir_events": "event_projection_maybe"}}
	if bad.Validate() == nil {
		t.Fatal("Validate accepted an unknown class")
	}
}
