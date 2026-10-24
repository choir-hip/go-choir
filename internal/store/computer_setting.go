package store

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// ogKindComputerSetting is one owner-set switch on one computer (L2a A6 /
// R2-7, docs/reports/l2a-synthesis-intake-design-2026-10-14.md). A setting
// is the owner's act through the product API; no desk sets one.
const ogKindComputerSetting = objectgraph.ObjectKind("choir.computer_setting")

// SettingIntakeLabels turns on Jev's labels for arriving captures and the
// prompt text that tells management about them. Off by default; turning it
// off is the rollback.
const SettingIntakeLabels = "intake.labels"

// SettingManagementSerialTasks holds management tasks to one at a time
// (K = 1, S2a's behaviour) on the next start, without a restart: the live
// rollback of management delegation S2b on an always-busy computer. Off by
// default (K from the runtime config); running tasks finish, none is
// cancelled.
const SettingManagementSerialTasks = "management.serial_tasks"

// SettingCalibrationFeedback turns on the in-context calibration digest
// of the ICRL experiment (docs/reports/icrl-calibration-experiment-2026-10-14.md):
// desks in the digest arm read their own settled calibration as a user
// turn and in choir.Pack(). Off by default; off computes and shows nothing,
// and turning it off is the rollback.
const SettingCalibrationFeedback = "calibration.feedback"

// SettingTelemetryBatch turns on run-telemetry batching for the computer: the
// ten high-volume trace event kinds are sealed in small windows instead of one
// acknowledged tape write each, and a crash may lose the open window
// (docs/reports/telemetry-batching-design-2026-10-20.md). Off by default; off
// drains the queue and is the rollback. The env flag CHOIR_TELEMETRY_BATCH=on
// turns it on regardless.
const SettingTelemetryBatch = "telemetry.batch"

// SettingResearchSystem1 lets the research desk ask System 1
// (choir.System1) with the default caps, when the runtime config sets none
// (owner notes §29: System 1 does the related-stories reading). Off by
// default; off is the rollback.
const SettingResearchSystem1 = "research.system1"

// SettingDebouncedCheckpoint moves the guest's Dolt history commit off the
// write path: a background committer commits on a time debounce and an ops
// bound instead of finalize committing inline every hundred events
// (docs/problems/guest-checkpoint-on-the-write-path-2026-10-21.md). Off by
// default; off is the rollback. The env flag CHOIR_GUEST_CHECKPOINT=debounced
// turns it on regardless.
const SettingDebouncedCheckpoint = "store.debounced_checkpoint"

// SettingEventTable moves the ten batched telemetry kinds to their own table,
// choir_events (4 prolly trees per insert instead of 13): the writer stamps
// their projection ops for it (docs/design/choir-event-table-f-c-2026-10-22.md
// 4.2). It takes effect only on a computer the host also names
// (CHOIR_EVENT_TABLE_COMPUTERS, a grant on the credential disk), because once
// an op is stamped deploy 1 is that computer's rollback floor. Off by
// default; off is the live rollback: new events go back to og_objects, and
// the events already in choir_events stay where they are and stay readable
// (the readers read both tables).
const SettingEventTable = "store.event_table"

// SettingSourceProjection turns on the guest source projection (U1, and the
// booted base's objects U1b fetches): the guest source repository follows the
// effective release and the cast gate checks it
// (docs/design/computer-update-architecture-rnd-2026-10-24.md, section 4.6).
// An owner act at runtime, per computer, no deploy. Off by default; off is the
// rollback. The env switch CHOIR_SOURCE_PROJECTION=1 turns it on regardless.
const SettingSourceProjection = "source.projection"

// SettingTraceLog will move the nine trace kinds off the tape into the
// guest-local trace segment log (docs/design/trace-segment-log-2026-10-22.md
// section 8). Registered by slice S2 and not acted on: the writer is S4's, and
// until then nothing reads the value. Off by default; off will be the live
// rollback. No env override ships for production computers.
const SettingTraceLog = "store.trace_log"

// SettingEngineeringProgressToTexture makes each engineering progress Report
// (a choir.Report from an assignment's engineering desk, which goes to
// management) also mint one platform-authored progress record into the
// inbox of the Texture whose request the assignment serves, in the same
// transaction, so the owner's document hears of the work without waiting
// for a management turn to relay it
// (docs/problems/engineering-reports-never-reach-texture-2026-10-24.md).
// Texture stays the only writer of the document and management stays the
// addressee. Off by default; off mints nothing and is the rollback.
const SettingEngineeringProgressToTexture = "engineering.progress_to_texture"

// KnownComputerSettings are the settings a computer has, with their
// defaults. A key outside this list is refused, so a typo never stores a
// setting nobody reads.
var KnownComputerSettings = map[string]bool{
	SettingIntakeLabels:                 false,
	SettingManagementSerialTasks:        false,
	SettingCalibrationFeedback:          false,
	SettingTelemetryBatch:               false,
	SettingResearchSystem1:              false,
	SettingDebouncedCheckpoint:          false,
	SettingEventTable:                   false,
	SettingSourceProjection:             false,
	SettingTraceLog:                     false,
	SettingEngineeringProgressToTexture: false,
}

// ComputerSetting is one switch's value on one computer.
type ComputerSetting struct {
	OwnerID    string    `json:"owner_id"`
	ComputerID string    `json:"computer_id"`
	Key        string    `json:"key"`
	Value      bool      `json:"value"`
	UpdatedAt  time.Time `json:"updated_at,omitempty"`
}

func computerSettingKey(computerID, key string) string {
	return strings.TrimSpace(computerID) + "\x00" + strings.TrimSpace(key)
}

// PutComputerSetting stores a known setting for one computer.
func (s *Store) PutComputerSetting(ctx context.Context, setting ComputerSetting) error {
	setting.OwnerID, setting.ComputerID, setting.Key = strings.TrimSpace(setting.OwnerID), strings.TrimSpace(setting.ComputerID), strings.TrimSpace(setting.Key)
	if setting.OwnerID == "" || setting.ComputerID == "" {
		return fmt.Errorf("computer setting: owner_id and computer_id are required")
	}
	if _, known := KnownComputerSettings[setting.Key]; !known {
		return fmt.Errorf("computer setting: unknown key %q", setting.Key)
	}
	if setting.UpdatedAt.IsZero() {
		setting.UpdatedAt = time.Now().UTC()
	}
	metadata := map[string]any{
		"computer_id": setting.ComputerID,
		"key":         setting.Key,
		"value":       setting.Value,
		"updated_at":  setting.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	_, err := s.ogPut(ctx, ogKindComputerSetting, setting.OwnerID, computerSettingKey(setting.ComputerID, setting.Key), setting, metadata, setting.UpdatedAt)
	return err
}

// GetComputerSetting returns one stored setting, or ErrNotFound.
func (s *Store) GetComputerSetting(ctx context.Context, ownerID, computerID, key string) (ComputerSetting, error) {
	if s.og == nil {
		return ComputerSetting{}, fmt.Errorf("store: object graph not initialized")
	}
	id, err := objectgraph.BuildCanonicalID(ogKindComputerSetting, strings.TrimSpace(ownerID), objectgraph.StableSuffixFromKey(computerSettingKey(computerID, key)))
	if err != nil {
		return ComputerSetting{}, err
	}
	obj, err := s.og.GetObject(ctx, id)
	if errors.Is(err, objectgraph.ErrNotFound) {
		return ComputerSetting{}, ErrNotFound
	}
	if err != nil {
		return ComputerSetting{}, err
	}
	var setting ComputerSetting
	if err := ogDecode(obj, &setting); err != nil {
		return ComputerSetting{}, err
	}
	return setting, nil
}

// ListComputerSettings returns every known setting of one computer, stored
// or default, sorted by key.
func (s *Store) ListComputerSettings(ctx context.Context, ownerID, computerID string) ([]ComputerSetting, error) {
	out := make([]ComputerSetting, 0, len(KnownComputerSettings))
	for key, def := range KnownComputerSettings {
		setting, err := s.GetComputerSetting(ctx, ownerID, computerID, key)
		if errors.Is(err, ErrNotFound) {
			setting = ComputerSetting{OwnerID: strings.TrimSpace(ownerID), ComputerID: strings.TrimSpace(computerID), Key: key, Value: def}
		} else if err != nil {
			return nil, err
		}
		out = append(out, setting)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}
