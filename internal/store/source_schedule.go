package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// ogKindSourceSchedule is one Autopaper source's poll schedule on the
// paper's computer (W1c, docs/archive/definitions/choir-autopaper-w1-content-in-the-graph-2026-10-11.md).
// It is this computer's working state; the captures it produces are shared
// content in Store B.
const ogKindSourceSchedule = objectgraph.ObjectKind("choir.source_schedule")

// Source schedule states. A schedule is a standing owner intent: a restart,
// crash or planned, leaves an active schedule active (owner ruling 6,
// 2026-10-14). A crash closes only the poll in flight; a schedule whose
// poll was in flight at two crash boots in a row is closed as
// interrupted_by_restart, and nothing resumes it but a new start.
const (
	SourceScheduleActive               = "active"
	SourceScheduleStopped              = "stopped"
	SourceScheduleInterruptedByRestart = "interrupted_by_restart"
)

// SourcePollInterruptedByRestart is the status of a poll a crash cut short:
// the occurrence's visible fate, recorded in the schedule's usage.
const SourcePollInterruptedByRestart = "interrupted_by_restart"

// SourcePollUsageLimit bounds the usage history a schedule keeps.
const SourcePollUsageLimit = 24

// SourcePollUsage is one poll's resource use.
type SourcePollUsage struct {
	At          time.Time `json:"at"`
	Status      string    `json:"status"`
	StatusCode  int       `json:"status_code,omitempty"`
	Fetched     int       `json:"fetched"`
	NotModified bool      `json:"not_modified,omitempty"`
	ItemBytes   int       `json:"item_bytes"`
	New         int       `json:"new"`
	Repeated    int       `json:"repeated"`
	Skipped     int       `json:"skipped,omitempty"`
	// Deferred counts items the poll left for the next one once it reached
	// its limit (L2a A5, defect 1).
	Deferred int `json:"deferred,omitempty"`
	// Labelled and Unlabelled count the new captures Jev labelled and those
	// left unlabelled (failure, timeout, rate limit) when intake.labels is
	// on (L2a A1): the poll's label coverage.
	Labelled   int    `json:"labelled,omitempty"`
	Unlabelled int    `json:"unlabelled,omitempty"`
	PollMS     int64  `json:"poll_ms"`
	TotalMS    int64  `json:"total_ms"`
	Error      string `json:"error,omitempty"`
}

// SourcePollTotals are a schedule's running totals since it was created.
type SourcePollTotals struct {
	Polls     int   `json:"polls"`
	Errors    int   `json:"errors"`
	Fetched   int   `json:"fetched"`
	New       int   `json:"new"`
	ItemBytes int64 `json:"item_bytes"`
	PollMS    int64 `json:"poll_ms"`
}

// SourceSchedule is one source's schedule. Generation changes on every start
// and stop; a wake carrying another generation is stale and does nothing.
type SourceSchedule struct {
	OwnerID         string `json:"owner_id"`
	ComputerID      string `json:"computer_id"`
	SourceID        string `json:"source_id"`
	IntervalSeconds int64  `json:"interval_seconds"`
	MaxItems        int    `json:"max_items"`
	// FixtureRun marks a schedule on a test_fixture source, started by a
	// request marked as a fixture run (L2a R2-7).
	FixtureRun          bool              `json:"fixture_run,omitempty"`
	State               string            `json:"state"`
	Generation          int64             `json:"generation"`
	NextDue             time.Time         `json:"next_due,omitempty"`
	ConsecutiveFailures int               `json:"consecutive_failures"`
	StateReason         string            `json:"state_reason,omitempty"`
	Totals              SourcePollTotals  `json:"totals"`
	Recent              []SourcePollUsage `json:"recent,omitempty"`
	UpdatedAt           time.Time         `json:"updated_at"`
	// PendingArrivals are new captures whose arrival record failed to
	// append, oldest first (L2a A5, defect 2). They are data, not an
	// obligation: the next poll's arrival names them first. A crash boot
	// closes them; ArrivalsClosed counts captures closed unannounced (by a
	// crash boot or the pending bound) and ArrivalsClosedReason says why
	// the last were.
	PendingArrivals      []types.CaptureArrival `json:"pending_arrivals,omitempty"`
	ArrivalsClosed       int                    `json:"arrivals_closed,omitempty"`
	ArrivalsClosedReason string                 `json:"arrivals_closed_reason,omitempty"`
	// PollInFlightSince is set durably when a poll starts and cleared when
	// it records its result. A crash boot that finds it set closes that
	// poll as interrupted_by_restart and counts the crash against the
	// schedule. CrashesWithPollInFlight counts crash boots in a row that
	// found a poll in flight; a finished poll, a crash boot with no poll in
	// flight, and a new start reset it. At two the schedule is closed: its
	// tick crashes the computer (the crash-loop bound).
	PollInFlightSince       time.Time `json:"poll_in_flight_since,omitempty"`
	CrashesWithPollInFlight int       `json:"crashes_with_poll_in_flight,omitempty"`
}

// sourcePollErrorLimit bounds a poll's saved error text: the schedule keeps
// its last polls and is rewritten on every poll, so an upstream error page
// must not ride along in full.
const sourcePollErrorLimit = 512

// RecordPoll appends one poll's usage, bounded to SourcePollUsageLimit.
func (s *SourceSchedule) RecordPoll(u SourcePollUsage) {
	if len(u.Error) > sourcePollErrorLimit {
		u.Error = strings.ToValidUTF8(u.Error[:sourcePollErrorLimit], "") + "…"
	}
	s.Totals.Polls++
	if u.Error != "" {
		s.Totals.Errors++
	}
	s.Totals.Fetched += u.Fetched
	s.Totals.New += u.New
	s.Totals.ItemBytes += int64(u.ItemBytes)
	s.Totals.PollMS += u.PollMS
	s.Recent = append(s.Recent, u)
	if len(s.Recent) > SourcePollUsageLimit {
		s.Recent = append([]SourcePollUsage(nil), s.Recent[len(s.Recent)-SourcePollUsageLimit:]...)
	}
}

// PutSourceSchedule writes the schedule, replacing the previous state.
func (s *Store) PutSourceSchedule(ctx context.Context, sched SourceSchedule) error {
	sched.OwnerID = strings.TrimSpace(sched.OwnerID)
	sched.SourceID = strings.TrimSpace(sched.SourceID)
	if sched.OwnerID == "" || sched.SourceID == "" {
		return fmt.Errorf("source schedule: owner_id and source_id are required")
	}
	if sched.UpdatedAt.IsZero() {
		sched.UpdatedAt = time.Now().UTC()
	}
	metadata := map[string]any{
		"source_id":  sched.SourceID,
		"state":      sched.State,
		"generation": sched.Generation,
		"updated_at": sched.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	_, err := s.ogPut(ctx, ogKindSourceSchedule, sched.OwnerID, sched.SourceID, sched, metadata, sched.UpdatedAt)
	return err
}

// GetSourceSchedule returns the owner's schedule for sourceID, or ErrNotFound.
func (s *Store) GetSourceSchedule(ctx context.Context, ownerID, sourceID string) (SourceSchedule, error) {
	if s.og == nil {
		return SourceSchedule{}, fmt.Errorf("store: object graph not initialized")
	}
	id, err := objectgraph.BuildCanonicalID(ogKindSourceSchedule, strings.TrimSpace(ownerID), objectgraph.StableSuffixFromKey(strings.TrimSpace(sourceID)))
	if err != nil {
		return SourceSchedule{}, err
	}
	obj, err := s.og.GetObject(ctx, id)
	if errors.Is(err, objectgraph.ErrNotFound) {
		return SourceSchedule{}, ErrNotFound
	}
	if err != nil {
		return SourceSchedule{}, err
	}
	var sched SourceSchedule
	if err := ogDecode(obj, &sched); err != nil {
		return SourceSchedule{}, err
	}
	return sched, nil
}

// ListSourceSchedules lists schedules, for one owner or (ownerID "") all.
func (s *Store) ListSourceSchedules(ctx context.Context, ownerID string) ([]SourceSchedule, error) {
	if s.og == nil {
		return nil, fmt.Errorf("store: object graph not initialized")
	}
	objs, err := s.og.ListObjects(ctx, objectgraph.ListFilter{Kind: ogKindSourceSchedule, OwnerID: strings.TrimSpace(ownerID), Limit: 1000})
	if err != nil {
		return nil, err
	}
	out := make([]SourceSchedule, 0, len(objs))
	for _, obj := range objs {
		var sched SourceSchedule
		if err := ogDecode(obj, &sched); err != nil {
			return nil, err
		}
		out = append(out, sched)
	}
	return out, nil
}
