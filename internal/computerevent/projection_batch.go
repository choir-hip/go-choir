package computerevent

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	// ProjectorVersionV1 is the first complete-tape projector. It is distinct
	// from ReducerVersion (head algebra). Replay must bind this version so
	// restored rows do not depend on whichever binary happens to run.
	ProjectorVersionV1 = 1
	ProjectionBatchV1  = 1
)

var (
	ErrProjectionBatchInvalid = errors.New("computer event projection batch: invalid")
	ErrProjectionPoison       = errors.New("computer event projection: poison event cannot wedge the computer")
	ErrSQLDuringResolve       = errors.New("computer event projection: SQL executor used during payload resolve")
)

// ProjectionOp is one mutation inside an atomic batch. Intermediate heads
// between ops in the same batch are not restore targets; the whole batch is.
type ProjectionOp struct {
	Kind        string          `json:"kind"`
	CanonicalID string          `json:"canonical_id,omitempty"`
	Table       string          `json:"table,omitempty"`
	Body        json.RawMessage `json:"body,omitempty"`
}

// ProjectionBatch is the durable unit Project applies inside Finalize's SQL
// transaction. Payload bytes are already resolved; this value is SQL-only.
type ProjectionBatch struct {
	Version          int            `json:"version"`
	ProjectorVersion int            `json:"projector_version"`
	ComputerID       string         `json:"computer_id"`
	EventID          string         `json:"event_id"`
	EventDigest      string         `json:"event_digest"`
	Ops              []ProjectionOp `json:"ops"`
}

func (b ProjectionBatch) Validate() error {
	if b.Version != ProjectionBatchV1 || b.ProjectorVersion != ProjectorVersionV1 {
		return fmt.Errorf("%w: unsupported version", ErrProjectionBatchInvalid)
	}
	if strings.TrimSpace(b.ComputerID) == "" || strings.TrimSpace(b.EventID) == "" || !IsSHA256(b.EventDigest) {
		return fmt.Errorf("%w: computer/event identity required", ErrProjectionBatchInvalid)
	}
	if len(b.Ops) == 0 {
		return fmt.Errorf("%w: empty batch", ErrProjectionBatchInvalid)
	}
	for i, op := range b.Ops {
		if strings.TrimSpace(op.Kind) == "" {
			return fmt.Errorf("%w: op %d missing kind", ErrProjectionBatchInvalid, i)
		}
	}
	return nil
}

// SQLExecutor is the transaction-scoped writer Project uses. ResolvePayloads
// must never receive one.
type SQLExecutor interface {
	Exec(query string, args ...any) error
}

// GuardNoSQL is a resolver-phase executor that fails closed if SQL is attempted.
type GuardNoSQL struct{}

func (GuardNoSQL) Exec(string, ...any) error { return ErrSQLDuringResolve }

// ClassifyProjectionFailure distinguishes decrypt/resolve failures (must happen
// before CAS) from post-CAS Project failures that would otherwise wedge the
// computer via ErrNeedsProjectionRepair retrying the same crash.
func ClassifyProjectionFailure(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrPayloadResolverRequired) || errors.Is(err, ErrPayloadDigestMismatch) ||
		errors.Is(err, ErrPayloadPrivacyMismatch) || errors.Is(err, ErrPayloadSQLBeforeResolve) ||
		errors.Is(err, ErrSQLDuringResolve) {
		return fmt.Errorf("%w: %w", ErrPayloadSQLBeforeResolve, err)
	}
	return fmt.Errorf("%w: %w", ErrProjectionPoison, err)
}
