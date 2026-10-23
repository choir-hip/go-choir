package selfdev

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// ErrStampHealUnbound is returned when the heal is asked to append on a store
// whose projection sink is not bound. An unbound store is a test seam; healing
// it would write nothing to the tape and claim otherwise.
var ErrStampHealUnbound = errors.New("self-development stamp heal: the projection tape is not bound")

// StampDivergence is one operation row whose last_intent_error or updated_at
// differs between the live table and the table replayed from the tape. The
// full live row is kept (unexported) so the apply appends the snapshot the
// plan saw, not a fresh read.
type StampDivergence struct {
	OperationID           string   `json:"operation_id"`
	State                 string   `json:"state"`
	LiveLastIntentError   string   `json:"live_last_intent_error"`
	ReplayLastIntentError string   `json:"replay_last_intent_error"`
	LiveUpdatedAt         string   `json:"live_updated_at"`
	ReplayUpdatedAt       string   `json:"replay_updated_at"`
	Differs               []string `json:"differs"`

	live Operation
}

// StampHealPlan is the dry-run result: what an apply would append.
type StampHealPlan struct {
	ComputerID   string
	RowsExamined int
	Rows         []StampDivergence
	// MissingInReplay names live rows the replay does not have. They are
	// reported and never healed: a state-preserving snapshot of a row the
	// replay lacks is refused by the projector.
	MissingInReplay []string
}

// ListAll returns every operation of a computer, oldest first.
func (s *Store) ListAll(ctx context.Context, computerID string) ([]Operation, error) {
	rows, err := s.db.QueryContext(ctx, operationSelect+` WHERE computer_id=? ORDER BY created_at, operation_id`, strings.TrimSpace(computerID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	operations := make([]Operation, 0)
	for rows.Next() {
		operation, err := scanOperation(rows)
		if err != nil {
			return nil, err
		}
		operations = append(operations, operation)
	}
	return operations, rows.Err()
}

// PlanStampHeal compares the live rows with the replayed rows and selects
// exactly those whose last_intent_error or updated_at differ. Read-only.
func (s *Store) PlanStampHeal(ctx context.Context, computerID string, replayed []Operation) (StampHealPlan, error) {
	live, err := s.ListAll(ctx, computerID)
	if err != nil {
		return StampHealPlan{}, err
	}
	byID := make(map[string]Operation, len(replayed))
	for _, operation := range replayed {
		byID[operation.OperationID] = operation
	}
	plan := StampHealPlan{ComputerID: strings.TrimSpace(computerID), RowsExamined: len(live), Rows: []StampDivergence{}, MissingInReplay: []string{}}
	for _, operation := range live {
		other, ok := byID[operation.OperationID]
		if !ok {
			plan.MissingInReplay = append(plan.MissingInReplay, operation.OperationID)
			continue
		}
		var differs []string
		if operation.LastIntentError != other.LastIntentError {
			differs = append(differs, "last_intent_error")
		}
		if operation.UpdatedAt != other.UpdatedAt {
			differs = append(differs, "updated_at")
		}
		if len(differs) == 0 {
			continue
		}
		plan.Rows = append(plan.Rows, StampDivergence{
			OperationID: operation.OperationID, State: operation.State,
			LiveLastIntentError: operation.LastIntentError, ReplayLastIntentError: other.LastIntentError,
			LiveUpdatedAt: operation.UpdatedAt, ReplayUpdatedAt: other.UpdatedAt,
			Differs: differs, live: operation,
		})
	}
	return plan, nil
}

// ApplyStampHeal appends the plan as one projection batch of state-preserving
// snapshots, under writeMu, each guarded by ExpectedState = the state the plan
// saw. Two stale-plan hazards are refused with ErrConflict and nothing is
// appended: a row whose stamp moved since the plan (an older snapshot must not
// overwrite a newer stamp), and a row whose state moved (the projector's
// ExpectedState check refuses it). It returns the number of rows appended.
func (s *Store) ApplyStampHeal(ctx context.Context, plan StampHealPlan) (int, error) {
	if s.projection == nil {
		return 0, ErrStampHealUnbound
	}
	if len(plan.Rows) == 0 {
		return 0, nil
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	ops := make([]computerevent.ProjectionOp, 0, len(plan.Rows))
	for _, row := range plan.Rows {
		current, err := s.Get(ctx, plan.ComputerID, row.OperationID)
		if errors.Is(err, sql.ErrNoRows) {
			return 0, fmt.Errorf("%w: operation %s is gone since the plan", ErrConflict, row.OperationID)
		}
		if err != nil {
			return 0, err
		}
		if current.LastIntentError != row.live.LastIntentError {
			return 0, fmt.Errorf("%w: the stamp of %s changed since the plan", ErrConflict, row.OperationID)
		}
		op, err := operationProjectionOp(row.live, row.live.State)
		if err != nil {
			return 0, err
		}
		ops = append(ops, op)
	}
	if err := s.projection.AppendProjectionOps(ctx, ops); err != nil {
		if errors.Is(err, computerevent.ErrProjectionMismatch) {
			return 0, fmt.Errorf("%w: a row moved since the plan: %w", ErrConflict, err)
		}
		return 0, fmt.Errorf("append stamp heal projection: %w", err)
	}
	return len(ops), nil
}
