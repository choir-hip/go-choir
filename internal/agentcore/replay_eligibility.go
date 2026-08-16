package agentcore

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
)

// ErrReplayIneligible means a replay probe completed but cannot authorize a
// checkpoint, restore, or effects transition. It is deliberately distinct from
// a probe execution failure: an ineligible computer remains operationally
// observable but cannot make a reversibility claim.
var ErrReplayIneligible = errors.New("replay is ineligible")

const replayEligibilityManifestVersion = 1

const emptyDoltTableHash = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

// ReplayObservationClass is the authority classification for one VM-local
// schema object. Unknown objects are never eligible by default.
type ReplayObservationClass string

const (
	ReplayEventProjection     ReplayObservationClass = "event_projection"
	ReplayEmptyUntilSupported ReplayObservationClass = "empty_until_supported"
	ReplayRetiredAbsent       ReplayObservationClass = "retired_absent"
)

// ReplayAirworthinessManifest is versioned evidence of the schema-object
// boundary used by the replay eligibility gate. It declares expected object
// names and replay classes; it is not a DDL fingerprint.
//
// The map is copied when exposed so callers cannot mutate the process-wide
// policy.
type ReplayAirworthinessManifest struct {
	Version int                               `json:"version"`
	Entries map[string]ReplayObservationClass `json:"entries"`
}

var replayAirworthinessEntries = map[string]ReplayObservationClass{
	"computer_event_index":            ReplayEventProjection,
	"computer_event_projection_heads": ReplayEventProjection,
	"computer_effective_state":        ReplayEventProjection,
	"agents":                          ReplayEmptyUntilSupported,
	"runs":                            ReplayEmptyUntilSupported,
	"events":                          ReplayEmptyUntilSupported,
	"channel_messages":                ReplayEmptyUntilSupported,
	"inbox_deliveries":                ReplayEmptyUntilSupported,
	"run_memory_entries":              ReplayEmptyUntilSupported,
	"run_acceptances":                 ReplayEmptyUntilSupported,
	"run_continuations":               ReplayEmptyUntilSupported,
	"trajectories":                    ReplayEmptyUntilSupported,
	"work_items":                      ReplayEmptyUntilSupported,
	"browser_sessions":                ReplayEmptyUntilSupported,
	"worker_updates":                  ReplayEmptyUntilSupported,
	"coagent_mailboxes":               ReplayEmptyUntilSupported,
	"co_super_slots":                  ReplayEmptyUntilSupported,
	"media_progress":                  ReplayEmptyUntilSupported,
	"media_recents":                   ReplayEmptyUntilSupported,
	"user_preferences":                ReplayEmptyUntilSupported,
	"desktop_state":                   ReplayEmptyUntilSupported,
	"desktop_workspaces":              ReplayEventProjection,
	"desktop_sessions":                ReplayEventProjection,
	"desktop_app_instances":           ReplayEventProjection,
	"desktop_window_placements":       ReplayEventProjection,
	"self_development_start_intents":  ReplayEmptyUntilSupported,
	"self_development_operations":     ReplayEmptyUntilSupported,
	"texture_documents":               ReplayEmptyUntilSupported,
	"texture_revisions":               ReplayEmptyUntilSupported,
	"texture_source_entities":         ReplayEmptyUntilSupported,
	"texture_source_refs":             ReplayEmptyUntilSupported,
	"texture_document_aliases":        ReplayEmptyUntilSupported,
	"texture_agent_mutations":         ReplayEmptyUntilSupported,
	"texture_controller_checkpoints":  ReplayEmptyUntilSupported,
	"texture_decisions":               ReplayEmptyUntilSupported,
	"agent_evidence":                  ReplayEmptyUntilSupported,
	"content_items":                   ReplayEmptyUntilSupported,
	"podcast_subscriptions":           ReplayEmptyUntilSupported,
	"og_edges":                        ReplayEventProjection,
	"og_objects":                      ReplayEventProjection,
	"app_adoptions":                   ReplayRetiredAbsent,
	"app_change_packages":             ReplayRetiredAbsent,
	"candidate_package_intakes":       ReplayRetiredAbsent,
	"computer_source_lineages":        ReplayRetiredAbsent,
	"computer_supervision_commands":   ReplayRetiredAbsent,
}

// CurrentReplayAirworthinessManifest returns the immutable policy snapshot for
// the current schema boundary.
func CurrentReplayAirworthinessManifest() ReplayAirworthinessManifest {
	entries := make(map[string]ReplayObservationClass, len(replayAirworthinessEntries))
	for table, class := range replayAirworthinessEntries {
		entries[table] = class
	}
	return ReplayAirworthinessManifest{Version: replayEligibilityManifestVersion, Entries: entries}
}

// Validate checks the manifest's version and exhaustive entry classes before
// it is used to classify an observation set.
func (m ReplayAirworthinessManifest) Validate() error {
	if m.Version != replayEligibilityManifestVersion {
		return fmt.Errorf("replay manifest version %d is unsupported", m.Version)
	}
	if len(m.Entries) == 0 {
		return errors.New("replay manifest has no entries")
	}
	for table, class := range m.Entries {
		if strings.TrimSpace(table) == "" {
			return errors.New("replay manifest contains an empty table name")
		}
		switch class {
		case ReplayEventProjection, ReplayEmptyUntilSupported, ReplayRetiredAbsent:
		default:
			return fmt.Errorf("replay manifest table %q has unknown class %q", table, class)
		}
	}
	return nil
}

// ReplayEligibility is the strict, default-deny decision layered over the
// diagnostic equivalence report. A matching empty projection is not enough:
// both event heads must be non-nil, declared objects must be present,
// retired residue must be absent, and every observed object must be
// explainable by the declared schema boundary.
type ReplayEligibility struct {
	ManifestVersion     int      `json:"manifest_version"`
	Eligible            bool     `json:"eligible"`
	Reason              string   `json:"reason,omitempty"`
	UnsupportedTables   []string `json:"unsupported_tables,omitempty"`
	RetiredTables       []string `json:"retired_tables,omitempty"`
	UnknownTables       []string `json:"unknown_tables,omitempty"`
	MissingTables       []string `json:"missing_tables,omitempty"`
	SchemaDrift         []string `json:"schema_drift,omitempty"`
	RequiresNonNilHeads bool     `json:"requires_non_nil_heads"`
}

func replayEligibility(liveHead, replayHead *computerevent.Head, live, replay computerversion.ObservationSet, result computerversion.EquivalenceResult) ReplayEligibility {
	eligibility := ReplayEligibility{
		ManifestVersion:     replayEligibilityManifestVersion,
		RequiresNonNilHeads: true,
		UnsupportedTables:   []string{},
		RetiredTables:       []string{},
		UnknownTables:       []string{},
		MissingTables:       []string{},
		SchemaDrift:         []string{},
	}

	manifest := CurrentReplayAirworthinessManifest()
	if err := manifest.Validate(); err != nil {
		eligibility.Reason = fmt.Sprintf("replay manifest is invalid: %v", err)
		return eligibility
	}
	liveTables, liveSchemas := replayObservationTables(live)
	replayTables, replaySchemas := replayObservationTables(replay)
	seenUnsupported := make(map[string]struct{})
	seenRetired := make(map[string]struct{})
	seenUnknown := make(map[string]struct{})
	seenMissing := make(map[string]struct{})
	record := func(dst *[]string, seen map[string]struct{}, table string) {
		if _, ok := seen[table]; ok {
			return
		}
		seen[table] = struct{}{}
		*dst = append(*dst, table)
	}
	recordMissing := func(table string) {
		record(&eligibility.MissingTables, seenMissing, table)
	}
	classify := func(table, content string) {
		class, known := manifest.Entries[table]
		if !known {
			record(&eligibility.UnknownTables, seenUnknown, table)
			return
		}
		switch class {
		case ReplayRetiredAbsent:
			record(&eligibility.RetiredTables, seenRetired, table)
		case ReplayEmptyUntilSupported:
			if content != "" && content != emptyDoltTableHash {
				record(&eligibility.UnsupportedTables, seenUnsupported, table)
			}
		case ReplayEventProjection:
			// Event projection rows are the only non-empty rows admitted by
			// this first-slice manifest.
		default:
			record(&eligibility.UnknownTables, seenUnknown, table)
		}
	}
	for table, content := range liveTables {
		classify(table, content)
	}
	for table, content := range replayTables {
		classify(table, content)
	}
	for table := range liveSchemas {
		classify(table, "")
	}
	for table := range replaySchemas {
		classify(table, "")
	}
	for table, class := range manifest.Entries {
		if class == ReplayRetiredAbsent {
			continue
		}
		if _, ok := liveSchemas[table]; !ok {
			recordMissing(table)
		}
		if _, ok := replaySchemas[table]; !ok {
			recordMissing(table)
		}
	}
	for table, schema := range liveSchemas {
		replaySchema, ok := replaySchemas[table]
		if !ok || schema != replaySchema {
			eligibility.SchemaDrift = append(eligibility.SchemaDrift, table)
		}
	}
	for table := range replaySchemas {
		if _, ok := liveSchemas[table]; !ok {
			eligibility.SchemaDrift = append(eligibility.SchemaDrift, table)
		}
	}

	sort.Strings(eligibility.UnsupportedTables)
	sort.Strings(eligibility.RetiredTables)
	sort.Strings(eligibility.UnknownTables)
	sort.Strings(eligibility.MissingTables)
	sort.Strings(eligibility.SchemaDrift)

	switch {
	case liveHead == nil || replayHead == nil:
		eligibility.Reason = "canonical and replay event heads must both be non-nil"
	case *liveHead != *replayHead:
		eligibility.Reason = "replay terminal head does not match canonical head"
	case len(eligibility.RetiredTables) > 0:
		eligibility.Reason = "retired tables are present in the observed workspace"
	case len(eligibility.UnknownTables) > 0:
		eligibility.Reason = "observed workspace contains tables outside the declared schema boundary"
	case len(eligibility.MissingTables) > 0:
		eligibility.Reason = "declared schema objects are missing from the observed workspace"
	case len(eligibility.SchemaDrift) > 0:
		eligibility.Reason = "live and replay workspace schemas differ"
	case len(eligibility.UnsupportedTables) > 0:
		eligibility.Reason = "behavior-bearing direct-write tables are non-empty without reducers"
	case !result.Equivalent():
		eligibility.Reason = "reconstructed projection is not equivalent to live state"
	default:
		eligibility.Eligible = true
		eligibility.Reason = "canonical event replay is eligible for the declared manifest"
	}
	return eligibility
}

func replayObservationTables(observations computerversion.ObservationSet) (map[string]string, map[string]string) {
	tables := make(map[string]string)
	schemas := make(map[string]string)
	const tablePrefix = "dolt:texture:table:"
	const schemaPrefix = "dolt:texture:schema:"
	for _, observation := range observations.Observations {
		switch {
		case strings.HasPrefix(observation.Key, tablePrefix):
			tables[strings.TrimPrefix(observation.Key, tablePrefix)] = observation.Value
		case strings.HasPrefix(observation.Key, schemaPrefix):
			schemas[strings.TrimPrefix(observation.Key, schemaPrefix)] = observation.Value
		}
	}
	return tables, schemas
}

func (e ReplayEligibility) Error() error {
	if e.Eligible {
		return nil
	}
	if strings.TrimSpace(e.Reason) == "" {
		return ErrReplayIneligible
	}
	return fmt.Errorf("%w: %s", ErrReplayIneligible, e.Reason)
}
