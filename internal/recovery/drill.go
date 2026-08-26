package recovery

import (
	"context"
	"fmt"
	"time"
)

// DrillReceipt records the measured performance and outcome of an automated restore drill.
type DrillReceipt struct {
	DrillID       string    `json:"drill_id"`
	ComputerID    string    `json:"computer_id"`
	CapsuleDigest string    `json:"capsule_digest"`
	DurationMs    int64     `json:"duration_ms"`
	RTOSeconds    float64   `json:"rto_seconds"`
	RestoredFiles int       `json:"restored_files"`
	EventSequence uint64    `json:"event_sequence"`
	Success       bool      `json:"success"`
	Error         string    `json:"error,omitempty"`
	ExecutedAt    time.Time `json:"executed_at"`
}

// RestoreExecutor defines the function signature for executing a cold recovery.
type RestoreExecutor func(ctx context.Context, capsule *RecoveryCapsule) (restoredFiles int, err error)

// DrillRunner executes automated restore drills and measures real SLO numbers.
type DrillRunner struct {
	executor RestoreExecutor
}

// NewDrillRunner creates a new drill runner with the provided restore executor.
func NewDrillRunner(executor RestoreExecutor) *DrillRunner {
	return &DrillRunner{executor: executor}
}

// RunDrill executes a single recovery drill against a capsule and records its receipt.
func (r *DrillRunner) RunDrill(ctx context.Context, drillID string, capsule *RecoveryCapsule) (*DrillReceipt, error) {
	if capsule == nil {
		return nil, fmt.Errorf("drill: nil capsule")
	}
	if err := capsule.Verify(); err != nil {
		return nil, fmt.Errorf("drill: invalid capsule: %w", err)
	}

	start := time.Now()
	restored, err := r.executor(ctx, capsule)
	elapsed := time.Since(start)

	receipt := &DrillReceipt{
		DrillID:       drillID,
		ComputerID:    capsule.ComputerID,
		CapsuleDigest: capsule.CapsuleDigest,
		DurationMs:    elapsed.Milliseconds(),
		RTOSeconds:    elapsed.Seconds(),
		RestoredFiles: restored,
		EventSequence: capsule.EventSequence,
		Success:       err == nil,
		ExecutedAt:    start.UTC(),
	}
	if err != nil {
		receipt.Error = err.Error()
	}

	return receipt, nil
}
