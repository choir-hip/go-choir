package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

// Owner, 2026-11-03: "nothing is off by default. thats the old wrong pattern
// of all these hidden switches." A feature setting (product behaviour behind
// a flag) reads on when nothing is stored; correctness refusals, operator
// bounds and the features held off for a named, written reason keep their
// defaults (docs/problems/default-off-switches-audit-2026-11-03.md).

// defaultOnFeatures are the feature settings that read on when no value is
// stored on a computer.
var defaultOnFeatures = []string{
	SettingTextureEvidenceEmbeds,
	SettingTextureOwnerDirections,
	SettingTextureHoldTrajectory,
	SettingEngineeringProgressToTexture,
	SettingEngineeringCarryUnfinished,
	SettingEngineeringSlotsGraph,
	SettingFilesObject,
	SettingPackagesPublish,
	SettingIntakeEvents,
	SettingIntakeLabels,
	SettingTelemetryBatch,
	SettingTelemetryRetire,
}

// heldDefaults are the settings that keep an off default, each for the
// reason the audit names: an operator bound or rollback lever, or a feature
// whose default-on would break something today.
var heldDefaults = []string{
	SettingManagementSerialTasks, // rollback lever (on = K=1)
	SettingCompactForce,          // proof lever for d@e
	SettingDispatchWideSlots,     // capacity bound (8 -> 12 slots)
	SettingTextureRootDesk,       // F4/F5 re-proof on d@e; the paper's hourly batch not built
	SettingResearchRelatedStory,  // its finish rule would bind every owner research desk
	SettingEvidenceGraph,         // owner: measure the import on a paper copy first
	SettingTelemetryPartitions,   // GC starvation; floor 5 on every computer
	SettingTapeGroupCommit,       // owner ruled delete (crash-lossy window)
	SettingEventTable,            // owner ruled delete; needs a host grant too
	SettingIntakeEventWakes,      // failed its false-merge bar three times
	// Experiment arms, not features: none held is the shipped behaviour and
	// E1's comparator (docs/design/calibration-experiment-e1-2026-11-04.md).
	SettingCalibrationE1Restate,
	SettingCalibrationE1Pull,
	SettingCalibrationE1Stub,
	SettingCalibrationE1TwoStage,
	SettingCalibrationE1Baseline,
}

func TestFeatureSettingsDefaultOn(t *testing.T) {
	for _, key := range defaultOnFeatures {
		def, known := KnownComputerSettings[key]
		if !known || !def {
			t.Errorf("%s: known=%v default=%v; want a known setting that defaults on", key, known, def)
		}
	}
}

func TestHeldSettingsKeepTheirOffDefault(t *testing.T) {
	for _, key := range heldDefaults {
		def, known := KnownComputerSettings[key]
		if !known || def {
			t.Errorf("%s: known=%v default=%v; want a known setting that defaults off", key, known, def)
		}
	}
	if got, want := len(defaultOnFeatures)+len(heldDefaults), len(KnownComputerSettings); got != want {
		t.Fatalf("the audit classifies %d settings, KnownComputerSettings has %d: classify the new one", got, want)
	}
}

// The retired keys, which nothing read, are gone: a stored row stays
// invisible and a new one is refused.
func TestRetiredStoreSettingsAreRemoved(t *testing.T) {
	for _, key := range []string{"store.debounced_checkpoint", "store.trace_log"} {
		if _, known := KnownComputerSettings[key]; known {
			t.Errorf("%s is still known", key)
		}
	}
}

// With no stored value the Files API writer applies, as the default says;
// an explicit off still refuses.
func TestFileObjectWriterHonoursTheDefault(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	const owner, computer = "owner-files-default", "computer-files-default"
	now := time.Now().UTC()
	change := FileChange{Op: FileOpWrite, Record: fileRecord("notes/a.md", "sha256:aa", 1), RecordedBy: "files_api"}
	if _, err := s.ApplyFileChanges(ctx, owner, computer, []FileChange{change}, now); err != nil {
		t.Fatalf("ApplyFileChanges with nothing stored: %v; want the default (on) to apply", err)
	}
	if err := s.PutComputerSetting(ctx, ComputerSetting{OwnerID: owner, ComputerID: computer, Key: SettingFilesObject, Value: false}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApplyFileChanges(ctx, owner, computer, []FileChange{change}, now); !errors.Is(err, ErrFileObjectsOff) {
		t.Fatalf("ApplyFileChanges with a stored off = %v; want ErrFileObjectsOff", err)
	}
}
