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
// R2-7, docs/archive/reports/l2a-synthesis-intake-design-2026-10-14.md). A setting
// is the owner's act through the product API; no desk sets one.
const ogKindComputerSetting = objectgraph.ObjectKind("choir.computer_setting")

// SettingIntakeLabels turns on Jev's labels for arriving captures and the
// prompt text that tells management about them. On by default (owner,
// 2026-11-03: "nothing is off by default"); a stored off is the rollback.
const SettingIntakeLabels = "intake.labels"

// SettingManagementSerialTasks holds management tasks to one at a time
// (K = 1, S2a's behaviour) on the next start, without a restart: the live
// rollback of management delegation S2b on an always-busy computer. Off by
// default (K from the runtime config); running tasks finish, none is
// cancelled.
const SettingManagementSerialTasks = "management.serial_tasks"

// SettingTelemetryBatch turns on run-telemetry batching for the computer: the
// ten high-volume trace event kinds are sealed in small windows instead of one
// acknowledged tape write each, and a crash may lose the open window
// (docs/reports/telemetry-batching-design-2026-10-20.md). It is how telemetry
// rides the one tape (docs/design/one-tape-durability-2026-10-31.md). On by
// default for every computer (2026-10-31: d@e ran it from 10-30 across three
// boots with no sealer error, while the paper's unbatched trace lines were
// 82% of its host receipts); off drains the queue and is the rollback. The env flag
// CHOIR_TELEMETRY_BATCH=on turns it on regardless.
const SettingTelemetryBatch = "telemetry.batch"

// SettingResearchSystem1 lets the research desk ask System 1
// (choir.System1) with the default caps, when the runtime config sets none
// (owner notes §29: System 1 does the related-stories reading). On by
// default (owner, 2026-11-03); a stored off is the rollback, unless the
// runtime config sets both caps, which wins as before.
const SettingResearchSystem1 = "research.system1"

// SettingResearchRelatedStory lets the research desk record judged pair
// relations (choir.RelateStories) and export them to Store B as related_story
// edges (docs/problems/related-stories-thin-no-subject-nodes-2026-10-20.md,
// "Build: the related_story edge"). The edges show only when corpusd's
// CORPUSD_RELATED_STORY read switch is also on. Off by default, held: its
// finish rule treats every research:<doc> desk as a story desk, so on an
// owner computer it would hold research rounds to record entities for a
// story that was never published
// (docs/problems/default-off-switches-audit-2026-11-03.md); off is the
// rollback: the verb refuses and nothing is recorded or exported.
const SettingResearchRelatedStory = "research.related_story"

// SettingEventTable moves the ten batched telemetry kinds to their own table,
// choir_events (4 prolly trees per insert instead of 13): the writer stamps
// their projection ops for it (docs/archive/design/choir-event-table-f-c-2026-10-22.md
// 4.2). It takes effect only on a computer the host also names
// (CHOIR_EVENT_TABLE_COMPUTERS, a grant on the credential disk), because once
// an op is stamped deploy 1 is that computer's rollback floor. Off by
// default; off is the live rollback: new events go back to og_objects, and
// the events already in choir_events stay where they are and stay readable
// (the readers read both tables).
const SettingEventTable = "store.event_table"

// SettingEngineeringProgressToTexture makes each engineering progress Report
// (a choir.Report from an assignment's engineering desk, which goes to
// management) also mint one platform-authored progress record into the
// inbox of the Texture whose request the assignment serves, in the same
// transaction, so the owner's document hears of the work without waiting
// for a management turn to relay it
// (docs/problems/engineering-reports-never-reach-texture-2026-10-24.md).
// Texture stays the only writer of the document and management stays the
// addressee. On by default (owner, 2026-11-03); a stored off mints nothing
// and is the rollback.
const SettingEngineeringProgressToTexture = "engineering.progress_to_texture"

// SettingEvidenceGraph moves research evidence (SaveEvidence) from the
// agent_evidence table to the choir.agent_evidence graph kind, on the tape
// (docs/problems/data-has-more-than-one-home-2026-10-24.md, step 3). Read live
// at each write. Off by default, held: the owner asked for the first-write
// import to be measured on a copy of the paper's store first (switch audit
// 2026-11-03, group 3); off is the rollback: new evidence returns to
// the table, and evidence already in the graph stays readable. Turning it on
// moves the table's rows into the graph.
const SettingEvidenceGraph = "evidence.graph"

// SettingEngineeringSlotsGraph moves engineering slot claims from the
// co_super_slots table to the choir.co_super_slot graph kind, on the tape.
// Read live at each claim. On by default (owner, 2026-11-03); a stored off
// is the rollback, as for
// evidence.graph.
const SettingEngineeringSlotsGraph = "engineering.slots_graph"

// SettingEngineeringCarryUnfinished keeps the source changes of an
// implementation round stopped before it froze (its activation budget, its
// deadline, a run that failed without Complete, a cancel) as an unfinished,
// unverified patch taken from the capsule just before it is destroyed, and
// offers it, labeled so, to the next round on the same trajectory
// (docs/evidence/selfdev-trace-reviews-2026-10-25.md, problem 1). The patch
// is never applied by the platform and is never a candidate. Read live at
// each capsule revoke and each offer. On by default (owner, 2026-11-03); a
// stored off takes no snapshot
// and offers none of the kept ones, and is the rollback.
const SettingEngineeringCarryUnfinished = "engineering.carry_unfinished"

// SettingTextureOwnerDirections gives every Texture turn the owner's later
// directions (the owner_prompt of each earlier owner revision on the head's
// parent chain, oldest first), ranked above the original request
// (docs/design/texture-spec-follows-owner-2026-10-25.md). Without it the turn
// reads owner intent from the head revision only, so one Texture revision
// after an owner revise drops the direction and the original seed stands
// alone. Read at each Texture run start. On by default (owner, 2026-11-03);
// a stored off is the
// rollback: the turn is byte-identical to before.
const SettingTextureOwnerDirections = "texture.owner_directions"

// SettingTextureHoldTrajectory makes Texture hold its trajectory across
// wakes (docs/design/texture-holds-the-trajectory-2026-10-25.md): a new
// Texture run continues its earlier run's conversation verbatim (checkpoint
// and messages, bounded by the run's own compaction) instead of the actor
// rewarm digest, and when no revision carries a seed_prompt the request turn
// shows the document's first owner revision as the owner's request. Read at
// each Texture run start. On by default (owner, 2026-11-03); a stored off is
// the rollback: the run is
// seeded with the digest and the turn is byte-identical to before.
const SettingTextureHoldTrajectory = "texture.hold_trajectory"

// SettingTextureEvidenceEmbeds makes a fresh Freeze that observed visual
// captures embed them in the operation's Texture document, written by
// Texture's own package from the frozen bundle (station TE,
// docs/design/texture-evidence-embeds-te-2026-10-25.md). On by default
// (owner, 2026-11-03); a stored off writes nothing and is the rollback.
const SettingTextureEvidenceEmbeds = "texture.evidence_embeds"

// SettingTextureRootDesk turns on the Texture root desk (PD3 S1,
// docs/archive/definitions/choir-pd3-texture-root-desk-2026-10-11.md): commits on
// other documents mint a debounced texture_root_digest, whose dispatch
// ensures one master document (named by the texture_root marker) and wakes
// its own Texture desk, texture:<master>, to render the computer's open
// work. Off by default, held: the owner turns it on once its excerpt order
// (F4/F5) is re-proved on d@e and the paper's story documents feed the
// master as one batched entry an hour (switch audit 2026-11-03, group 1);
// off mints no digest and creates nothing, and is the
// rollback: the master stays an ordinary document.
const SettingTextureRootDesk = "texture.root_desk"

// SettingPackagesPublish lets the owner publish this computer's applied
// package changes to the Choir Library, where a published release is public
// to every signed-in owner (owner, 2026-11-03; Library S2,
// docs/design/library-package-propagation-2026-10-26.md 3.2): the guest
// drafts a package release only when it is on. On by default (owner,
// 2026-11-03); a stored off is the
// rollback, and releases already listed stay readable and inert.
const SettingPackagesPublish = "packages.publish"

// SettingDispatchWideSlots raises the dispatcher's shared slots from 8 to
// 12 at once, without a restart (the owner for the paper, 2026-10-27: "I
// have the tokens for it"; docs/problems/related-stories-not-showing-on-the-paper-2026-10-27.md
// saw slots 8/8 with 94 to 114 agents due). Off by default; off is the
// rollback: running activations finish and new ones wait for one of 8.
const SettingDispatchWideSlots = "dispatch.wide_slots"

// SettingTelemetryRetire runs the daily telemetry retirement job
// (telemetry_retire.go): aged choir.event rows of the unread and trace kinds
// leave the working store by a tape op; they stay on the sealed tape. On by
// default for every computer (2026-10-30, lean system L1; d@e ran it first).
// Off is the rollback: no further retire ops are appended.
const SettingTelemetryRetire = "store.telemetry_retire"

// SettingTelemetryPartitions moves tool.result telemetry into per-day
// partition files (telemetry_partition.go,
// docs/design/one-tape-durability-2026-10-31.md section 6c): the writer
// stamps the op, so the tape decides where the row lands, and retiring a day
// is deleting a file. Rings from d@e (a stored setting), then the default.
// Turning it on writes the store floor 5 first. Off is the live rollback: new
// tool results go to og_objects, and the rows already in partitions stay
// readable and are retired as before; below epoch 5 needs the floor lifted
// by hand once no partition holds a row.
const SettingTelemetryPartitions = "store.telemetry_partitions"

// SettingTapeGroupCommit makes acknowledged projection writes of unconditional
// upserts (objects, edges, run memory) share one tape event with the writes
// that arrive while an earlier one commits, and makes pending telemetry ride
// the next such event instead of being sealed as its own (window 10 s)
// (docs/design/tape-group-commit-2026-10-31.md). Every such write is still
// answered only after the shared event is durable. It also opens the
// run-memory window: a run's message entries wait with the telemetry and a
// crash may lose a run's newest ones (owner, 2026-10-31; the crash closes
// that run); every other run-memory kind stays acknowledged. Rings from d@e (a stored setting),
// then the paper, then the default. Off is the live rollback: every write
// takes the direct path again.
const SettingTapeGroupCommit = "tape.group_commit"

// SettingCompactForce makes the daily compact copy due whatever the store
// file's size and age (compact.go), so the copy and its swap at a planned
// restart can be proved on a computer under the 1 GiB layout trigger (d@e,
// docs/design/paper-compaction-2026-10-31.md) before they run on the paper.
// A fresh staged copy is still kept. Off by default and for every computer
// but the proof ring; off is the rollback: the size and age triggers decide
// again.
const SettingCompactForce = "store.compact_force"

// The calibration experiment E1's arm assignments
// (docs/design/calibration-experiment-e1-2026-11-04.md). Each is an
// experiment arm a computer holds, not a feature switch: a computer that
// holds none runs the shipped behaviour (engineering's digest pushed to half
// its rounds, one statement of a Cast's predictions), which is E1's
// comparator. Off by default for that reason; the owner's act sets the arms
// once the precondition window has passed, and clearing them ends E1.
//
// SettingCalibrationE1Restate is E1-M: management restates each engineering
// Cast's predictions once, shown its own record for half the Casts.
// SettingCalibrationE1Pull, ...Stub, ...TwoStage and ...Baseline are E1-E's
// engineering deliveries (pull, stub, two-stage, no seed); none on is push.
const (
	SettingCalibrationE1Restate  = "calibration.e1_restate"
	SettingCalibrationE1Pull     = "calibration.e1_pull"
	SettingCalibrationE1Stub     = "calibration.e1_stub"
	SettingCalibrationE1TwoStage = "calibration.e1_two_stage"
	SettingCalibrationE1Baseline = "calibration.e1_baseline"
)

// KnownComputerSettings are the settings a computer has, with their
// defaults. A key outside this list is refused, so a typo never stores a
// setting nobody reads.
var KnownComputerSettings = map[string]bool{
	SettingIntakeLabels:                 true,
	SettingManagementSerialTasks:        false,
	SettingTelemetryBatch:               true,
	SettingResearchSystem1:              true,
	SettingResearchRelatedStory:         false,
	SettingEventTable:                   false,
	SettingEngineeringProgressToTexture: true,
	SettingEvidenceGraph:                false,
	SettingEngineeringSlotsGraph:        true,
	SettingEngineeringCarryUnfinished:   true,
	SettingTextureOwnerDirections:       true,
	SettingTextureEvidenceEmbeds:        true,
	SettingTextureHoldTrajectory:        true,
	SettingTextureRootDesk:              false,
	SettingPackagesPublish:              true,
	SettingDispatchWideSlots:            false,
	SettingTelemetryRetire:              true,
	SettingTelemetryPartitions:          false,
	SettingTapeGroupCommit:              false,
	SettingCompactForce:                 false,
	SettingCalibrationE1Restate:         false,
	SettingCalibrationE1Pull:            false,
	SettingCalibrationE1Stub:            false,
	SettingCalibrationE1TwoStage:        false,
	SettingCalibrationE1Baseline:        false,
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
