package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// ogKindSourceSchedule is one Autopaper source's poll schedule on the
// paper's computer (W1c, docs/definitions/choir-autopaper-w1-content-in-the-graph-2026-10-11.md).
// It is this computer's working state; the captures it produces are shared
// content in Store B.
const ogKindSourceSchedule = objectgraph.ObjectKind("choir.source_schedule")

// Source schedule states. A crash boot closes an active schedule as
// interrupted_by_restart; nothing resumes it but a new start.
const (
	SourceScheduleActive               = "active"
	SourceScheduleStopped              = "stopped"
	SourceScheduleInterruptedByRestart = "interrupted_by_restart"
)

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
	PollMS      int64     `json:"poll_ms"`
	TotalMS     int64     `json:"total_ms"`
	Error       string    `json:"error,omitempty"`
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
	OwnerID             string            `json:"owner_id"`
	ComputerID          string            `json:"computer_id"`
	SourceID            string            `json:"source_id"`
	IntervalSeconds     int64             `json:"interval_seconds"`
	MaxItems            int               `json:"max_items"`
	State               string            `json:"state"`
	Generation          int64             `json:"generation"`
	NextDue             time.Time         `json:"next_due,omitempty"`
	ConsecutiveFailures int               `json:"consecutive_failures"`
	StateReason         string            `json:"state_reason,omitempty"`
	Totals              SourcePollTotals  `json:"totals"`
	Recent              []SourcePollUsage `json:"recent,omitempty"`
	UpdatedAt           time.Time         `json:"updated_at"`
}

// RecordPoll appends one poll's usage, bounded to SourcePollUsageLimit.
func (s *SourceSchedule) RecordPoll(u SourcePollUsage) {
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
