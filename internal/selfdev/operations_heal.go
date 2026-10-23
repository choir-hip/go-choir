package selfdev

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
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

// StampRefusal is a live row the heal will not touch because it differs from
// the replayed row in some column other than last_intent_error and
// updated_at. A snapshot of such a row would pass the projector's
// ExpectedState check against the live table and fail it against the replayed
// table, and an event the replay cannot apply on the canonical tape is a boot
// loop with no way back. The operator reads the replay's values and repairs
// the divergence by another act.
type StampRefusal struct {
	OperationID string `json:"operation_id"`
	// Reason is "state_diverged" when state is among the differing fields and
	// "row_diverged" otherwise.
	Reason          string            `json:"reason"`
	LiveState       string            `json:"live_state"`
	ReplayState     string            `json:"replay_state"`
	DifferentFields []string          `json:"different_fields"`
	ReplayValues    map[string]string `json:"replay_values"`
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
	// Refused lists rows that differ from the replay in a column other than
	// the two stamp columns. They are reported with the replay's values and
	// never healed.
	Refused []StampRefusal
}

var stampColumns = map[string]bool{"last_intent_error": true, "updated_at": true}

// operationFields renders every field of an Operation as a string keyed by its
// json name (or its Go name when it has none), so two rows compare
// column by column without listing the columns here. The names are JSON names,
// not column names: terminal_error is "error" and verifier_refs_json is
// "verifier_refs" in a refusal's different_fields and replay_values.
func operationFields(operation Operation) map[string]string {
	value := reflect.ValueOf(operation)
	kind := value.Type()
	fields := make(map[string]string, kind.NumField())
	for index := 0; index < kind.NumField(); index++ {
		field := kind.Field(index)
		name := strings.ToLower(field.Name)
		if tag := strings.Split(field.Tag.Get("json"), ",")[0]; tag != "" && tag != "-" {
			name = tag
		}
		item := value.Field(index)
		// A string renders as itself; every other field renders as its JSON,
		// so ["a b","c"] and ["a","b c"] differ (%v would print both as
		// [a b c]) and a nil pointer is "null".
		if item.Kind() == reflect.String {
			fields[name] = item.String()
			continue
		}
		encoded, err := json.Marshal(item.Interface())
		if err != nil {
			encoded = []byte(fmt.Sprintf("<unencodable %T: %v>", item.Interface(), err))
		}
		fields[name] = string(encoded)
	}
	return fields
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
	plan := StampHealPlan{ComputerID: strings.TrimSpace(computerID), RowsExamined: len(live), Rows: []StampDivergence{}, MissingInReplay: []string{}, Refused: []StampRefusal{}}
	for _, operation := range live {
		other, ok := byID[operation.OperationID]
		if !ok {
			plan.MissingInReplay = append(plan.MissingInReplay, operation.OperationID)
			continue
		}
		// Every column but the two stamp columns must already agree: the heal
		// event is validated at replay against the replayed row, not the live
		// one.
		liveFields, replayFields := operationFields(operation), operationFields(other)
		var divergent []string
		for name, value := range liveFields {
			if !stampColumns[name] && value != replayFields[name] {
				divergent = append(divergent, name)
			}
		}
		if len(divergent) > 0 {
			sort.Strings(divergent)
			refusal := StampRefusal{
				OperationID: operation.OperationID, Reason: "row_diverged",
				LiveState: operation.State, ReplayState: other.State,
				DifferentFields: divergent, ReplayValues: map[string]string{},
			}
			for _, name := range divergent {
				refusal.ReplayValues[name] = replayFields[name]
				if name == "state" {
					refusal.Reason = "state_diverged"
				}
			}
			plan.Refused = append(plan.Refused, refusal)
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
		// Both stamp columns: a reason that went A, B, A since the plan has
		// the plan's text and a later updated_at. The writeMu held here is
		// what makes this check sufficient; the projector cannot do it (see
		// the record, "Round 2", for why an expected_updated_at is not carried
		// on the op).
		// A state move also moves updated_at; it is left to the projector's
		// ExpectedState check (the authoritative refusal, the same one replay
		// applies), so only a same-state change is refused here.
		if current.State == row.live.State && (current.LastIntentError != row.live.LastIntentError || current.UpdatedAt != row.live.UpdatedAt) {
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
