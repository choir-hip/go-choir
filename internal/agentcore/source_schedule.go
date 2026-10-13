package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/sources"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// The Autopaper source schedule (W1c,
// docs/definitions/choir-autopaper-w1-content-in-the-graph-2026-10-11.md):
// each scheduled source is a chain of durable source_poll_due wakes. A wake
// polls the source with no model call, records the poll's usage on the
// schedule, and arms the next wake. A crash boot closes every active
// schedule; a planned boot leaves the pending wake to fire.
const (
	sourcePollDueKind            = "source_poll_due"
	defaultSourcePollInterval    = 15 * time.Minute
	minSourcePollInterval        = time.Minute
	maxSourcePollBackoff         = 24 * time.Hour
	sourceScheduleInterruptedWhy = "interrupted by a restart"
)

// sourceScheduleMu serializes schedule read-modify-write in this process.
var sourceScheduleMu sync.Mutex

type sourcePollDue struct {
	SourceID   string `json:"source_id"`
	Generation int64  `json:"generation"`
	Due        int64  `json:"due_unix"`
}

// sourcePollMailbox gives each source its own mailbox, so one source's polls
// run one at a time and a slow feed never holds a desk's mailbox.
func sourcePollMailbox(sourceID string) string { return "source:" + sourceID }

func (rt *Runtime) armSourcePoll(ctx context.Context, sched store.SourceSchedule) error {
	if rt.scheduleActor == nil {
		return fmt.Errorf("source schedule: durable wakes are unavailable")
	}
	content, err := json.Marshal(sourcePollDue{SourceID: sched.SourceID, Generation: sched.Generation, Due: sched.NextDue.Unix()})
	if err != nil {
		return err
	}
	return rt.scheduleActor(ctx, sched.OwnerID, sched.ComputerID, sourcePollMailbox(sched.SourceID),
		sourcePollDueKind, string(content), "", "", sched.NextDue)
}

// StartSourceSchedule starts (or restarts) polling sourceID every interval.
// The first poll is due now. Any earlier chain for the source goes stale.
func (rt *Runtime) StartSourceSchedule(ctx context.Context, ownerID, computerID, sourceID string, interval time.Duration, maxItems int) (store.SourceSchedule, error) {
	if rt == nil || rt.store == nil {
		return store.SourceSchedule{}, fmt.Errorf("source schedule: store unavailable")
	}
	sourceID = strings.TrimSpace(sourceID)
	var err error
	var reg sources.Registry
	if rt.researchDeps != nil && rt.researchDeps.Sources != nil {
		reg = *rt.researchDeps.Sources
	} else if reg, err = sources.DefaultRegistry(); err != nil {
		return store.SourceSchedule{}, err
	}
	if _, ok := reg.Lookup(sourceID); !ok {
		return store.SourceSchedule{}, fmt.Errorf("unknown source %q", sourceID)
	}
	if interval <= 0 {
		interval = defaultSourcePollInterval
	}
	if interval < minSourcePollInterval {
		interval = minSourcePollInterval
	}
	sourceScheduleMu.Lock()
	defer sourceScheduleMu.Unlock()
	sched, err := rt.store.GetSourceSchedule(ctx, ownerID, sourceID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return store.SourceSchedule{}, err
	}
	now := time.Now().UTC()
	sched.OwnerID, sched.ComputerID, sched.SourceID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID), sourceID
	sched.IntervalSeconds, sched.MaxItems = int64(interval/time.Second), maxItems
	sched.State, sched.StateReason = store.SourceScheduleActive, ""
	sched.Generation++
	sched.NextDue, sched.ConsecutiveFailures, sched.UpdatedAt = now, 0, now
	if err := rt.store.PutSourceSchedule(ctx, sched); err != nil {
		return store.SourceSchedule{}, err
	}
	if err := rt.armSourcePoll(ctx, sched); err != nil {
		sched.State, sched.StateReason = store.SourceScheduleStopped, "could not arm the first poll: "+err.Error()
		_ = rt.store.PutSourceSchedule(ctx, sched)
		return sched, err
	}
	return sched, nil
}

// StopSourceSchedule stops sourceID's schedule; its pending wake goes stale.
func (rt *Runtime) StopSourceSchedule(ctx context.Context, ownerID, sourceID, reason string) (store.SourceSchedule, error) {
	if rt == nil || rt.store == nil {
		return store.SourceSchedule{}, fmt.Errorf("source schedule: store unavailable")
	}
	sourceScheduleMu.Lock()
	defer sourceScheduleMu.Unlock()
	sched, err := rt.store.GetSourceSchedule(ctx, ownerID, strings.TrimSpace(sourceID))
	if err != nil {
		return store.SourceSchedule{}, err
	}
	sched.State, sched.StateReason = store.SourceScheduleStopped, strings.TrimSpace(reason)
	sched.Generation++
	sched.NextDue, sched.UpdatedAt = time.Time{}, time.Now().UTC()
	return sched, rt.store.PutSourceSchedule(ctx, sched)
}

// HandleSourcePollDue runs one scheduled poll. A wake for a schedule that is
// no longer active, or from an older generation, does nothing.
func (rt *Runtime) HandleSourcePollDue(ctx context.Context, ownerID, computerID, content string) error {
	if rt == nil || rt.store == nil {
		return fmt.Errorf("source schedule: store unavailable")
	}
	var due sourcePollDue
	if err := json.Unmarshal([]byte(content), &due); err != nil {
		return fmt.Errorf("source schedule: decode wake: %w", err)
	}
	current := func() (store.SourceSchedule, bool, error) {
		sched, err := rt.store.GetSourceSchedule(ctx, ownerID, due.SourceID)
		if errors.Is(err, store.ErrNotFound) {
			return sched, false, nil
		}
		if err != nil {
			return sched, false, err
		}
		return sched, sched.State == store.SourceScheduleActive && sched.Generation == due.Generation, nil
	}
	sourceScheduleMu.Lock()
	sched, live, err := current()
	sourceScheduleMu.Unlock()
	if err != nil || !live {
		return err
	}
	if sched.NextDue.Unix() != due.Due {
		// A wake of the current generation whose slot already ran: a replay,
		// or a retry after the schedule was saved but the actor commit that
		// carried its successor failed. It does not poll again; it re-arms the
		// saved next poll, which dedupes if the successor already exists
		// (problems/w1-panel-capture-boundary-and-lost-items-2026-10-11.md).
		return rt.armSourcePoll(ctx, sched)
	}

	started := time.Now().UTC()
	usage := store.SourcePollUsage{At: started}
	if rt.researchDeps == nil {
		usage.Status, usage.Error = "error", "research dependencies are not configured"
	} else {
		report, pollErr := researchtools.PollSourceOnce(ctx, *rt.researchDeps, ownerID, computerID, sched.SourceID, sched.MaxItems)
		usage = store.SourcePollUsage{
			At: started, Status: report.Status, StatusCode: report.StatusCode, Fetched: report.Fetched,
			NotModified: report.NotModified, ItemBytes: report.ItemBytes, New: report.New,
			Repeated: report.Repeated, Skipped: report.Skipped, PollMS: report.PollMS,
			TotalMS: report.TotalMS, Error: report.Error,
		}
		if pollErr != nil {
			usage.Status, usage.Error = "error", pollErr.Error()
		}
	}

	sourceScheduleMu.Lock()
	defer sourceScheduleMu.Unlock()
	sched, live, err = current()
	if err != nil {
		return err
	}
	live = live && sched.NextDue.Unix() == due.Due
	sched.RecordPoll(usage)
	now := time.Now().UTC()
	interval := time.Duration(sched.IntervalSeconds) * time.Second
	if interval < minSourcePollInterval {
		interval = minSourcePollInterval
	}
	if usage.Error != "" {
		sched.ConsecutiveFailures++
		for i := 0; i < sched.ConsecutiveFailures && interval < maxSourcePollBackoff; i++ {
			interval *= 2
		}
		if interval > maxSourcePollBackoff {
			interval = maxSourcePollBackoff
		}
		log.Printf("source schedule: %s poll failed (%d in a row): %s", sched.SourceID, sched.ConsecutiveFailures, usage.Error)
	} else {
		sched.ConsecutiveFailures = 0
	}
	sched.UpdatedAt = now
	if !live {
		// Stopped or restarted while this poll ran: keep the usage, arm nothing.
		return rt.store.PutSourceSchedule(ctx, sched)
	}
	sched.NextDue = now.Add(interval)
	if err := rt.store.PutSourceSchedule(ctx, sched); err != nil {
		return err
	}
	return rt.armSourcePoll(ctx, sched)
}

// closeSourceSchedulesAfterCrash closes every active schedule at a crash
// boot (owner rule, CLAUDE.md "Restarts End Work (Crash) Or Resume It").
// The pre-boot wake is refused by the actor handler; this makes the closed
// state visible. A planned boot closes nothing and its pending wake fires.
func (rt *Runtime) closeSourceSchedulesAfterCrash(ctx context.Context) {
	if rt == nil || rt.store == nil {
		return
	}
	if _, planned := rt.BootWasPlannedRestart(); planned {
		return
	}
	sourceScheduleMu.Lock()
	defer sourceScheduleMu.Unlock()
	schedules, err := rt.store.ListSourceSchedules(ctx, "")
	if err != nil {
		log.Printf("source schedule: crash boot: list schedules: %v", err)
		return
	}
	for _, sched := range schedules {
		if sched.State != store.SourceScheduleActive {
			continue
		}
		sched.State, sched.StateReason = store.SourceScheduleInterruptedByRestart, sourceScheduleInterruptedWhy
		sched.Generation++
		sched.NextDue, sched.UpdatedAt = time.Time{}, time.Now().UTC()
		if err := rt.store.PutSourceSchedule(ctx, sched); err != nil {
			log.Printf("source schedule: crash boot: close %s: %v", sched.SourceID, err)
			continue
		}
		log.Printf("source schedule: crash boot: %s closed: %s", sched.SourceID, sourceScheduleInterruptedWhy)
	}
}

// sourceScheduleOverdueGrace is how long past its next poll an active
// schedule may go before the list calls it overdue: a wake whose re-arm kept
// failing leaves a schedule active with nothing queued, and only this flag
// shows it (problems/w1-panel-capture-boundary-and-lost-items-2026-10-11.md).
const sourceScheduleOverdueGrace = 15 * time.Minute

type sourceScheduleView struct {
	store.SourceSchedule
	Overdue bool `json:"overdue"`
}

func sourceScheduleOverdue(sched store.SourceSchedule, now time.Time) bool {
	return sched.State == store.SourceScheduleActive && !sched.NextDue.IsZero() &&
		now.After(sched.NextDue.Add(sourceScheduleOverdueGrace))
}

type sourceScheduleRequest struct {
	SourceID        string `json:"source_id"`
	IntervalMinutes int    `json:"interval_minutes,omitempty"`
	MaxItems        int    `json:"max_items,omitempty"`
	Reason          string `json:"reason,omitempty"`
}

// HandleSourceSchedules serves /api/autopaper/sources/schedules: GET lists
// the owner's schedules, POST starts one, DELETE (?source_id=) stops one.
func (h *APIHandler) HandleSourceSchedules(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	rt := h.rt
	switch r.Method {
	case http.MethodGet:
		schedules, err := rt.store.ListSourceSchedules(r.Context(), ownerID)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
			return
		}
		now := time.Now().UTC()
		views := make([]sourceScheduleView, 0, len(schedules))
		for _, sched := range schedules {
			views = append(views, sourceScheduleView{SourceSchedule: sched, Overdue: sourceScheduleOverdue(sched, now)})
		}
		writeAPIJSON(w, http.StatusOK, map[string]any{"schedules": views})
	case http.MethodPost:
		var req sourceScheduleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid JSON body"})
			return
		}
		sched, err := rt.StartSourceSchedule(r.Context(), ownerID, rt.TextureComputerID(), req.SourceID,
			time.Duration(req.IntervalMinutes)*time.Minute, req.MaxItems)
		if err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
			return
		}
		writeAPIJSON(w, http.StatusOK, map[string]any{"schedule": sched})
	case http.MethodDelete:
		sched, err := rt.StopSourceSchedule(r.Context(), ownerID, r.URL.Query().Get("source_id"), r.URL.Query().Get("reason"))
		if errors.Is(err, store.ErrNotFound) {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "no schedule for that source"})
			return
		}
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
			return
		}
		writeAPIJSON(w, http.StatusOK, map[string]any{"schedule": sched})
	default:
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}
