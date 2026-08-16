package agentcore

import (
	"errors"
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

func TestCurrentReplayAirworthinessManifestReturnsCopy(t *testing.T) {
	manifest := CurrentReplayAirworthinessManifest()
	if manifest.Version != replayEligibilityManifestVersion || len(manifest.Entries) == 0 {
		t.Fatalf("invalid manifest: %#v", manifest)
	}
	if err := manifest.Validate(); err != nil {
		t.Fatalf("manifest validation failed: %v", err)
	}
	manifest.Entries["computer_event_index"] = ReplayRetiredAbsent
	if CurrentReplayAirworthinessManifest().Entries["computer_event_index"] != ReplayEventProjection {
		t.Fatal("manifest policy was mutable through returned entries")
	}
}

func TestDesktopAndOGAreEventProjectionAfterLiveResidueImport(t *testing.T) {
	manifest := CurrentReplayAirworthinessManifest()
	for _, table := range []string{
		"desktop_workspaces", "desktop_sessions", "desktop_app_instances",
		"desktop_window_placements", "og_objects", "og_edges",
	} {
		if got := manifest.Entries[table]; got != ReplayEventProjection {
			t.Fatalf("%s class=%q, want event_projection after live residue import", table, got)
		}
	}
	if got := manifest.Entries["desktop_state"]; got != ReplayEmptyUntilSupported {
		t.Fatalf("desktop_state class=%q, want empty_until_supported leftover", got)
	}
	invalid := manifest
	invalid.Entries = map[string]ReplayObservationClass{"desktop_sessions": ReplayObservationClass("presence_volatile")}
	if err := invalid.Validate(); err == nil {
		t.Fatal("presence_volatile class was admitted — that would weaken the nonempty gate")
	}
}
