package proxy

import (
	"context"
	"sync"
	"time"
)

const (
	computeRecoveryDetachedTimeout = 4 * time.Minute
	computeRecoveryTerminalTTL     = 15 * time.Minute
)

type computeRecoveryStatus struct {
	Active     bool   `json:"active"`
	Status     string `json:"status"`
	Action     string `json:"action,omitempty"`
	StartedAt  string `json:"started_at,omitempty"`
	UpdatedAt  string `json:"updated_at,omitempty"`
	FinishedAt string `json:"finished_at,omitempty"`
	Message    string `json:"message,omitempty"`
}

type computeRecoveryRunResult struct {
	Current computeComputer
	Runtime *computeRuntimeStatus
	Err     error
}

type computeRecoveryOperation struct {
	action   string
	done     chan struct{}
	status   string
	started  time.Time
	updated  time.Time
	finished time.Time
	current  computeComputer
	runtime  *computeRuntimeStatus
	err      error
}

type computeRecoveryTracker struct {
	mu  sync.Mutex
	ops map[string]*computeRecoveryOperation
}

func newComputeRecoveryTracker() *computeRecoveryTracker {
	return &computeRecoveryTracker{ops: make(map[string]*computeRecoveryOperation)}
}

func computeRecoveryKey(userID, desktopID string) string {
	return userID + "\x00" + desktopID
}

func (t *computeRecoveryTracker) startOrJoin(userID, desktopID, action string, run func(context.Context) computeRecoveryRunResult) *computeRecoveryOperation {
	if t == nil {
		return nil
	}
	key := computeRecoveryKey(userID, desktopID)
	now := time.Now().UTC()

	t.mu.Lock()
	t.cleanupLocked(now)
	if existing := t.ops[key]; existing != nil && existing.status == "refreshing" {
		t.mu.Unlock()
		return existing
	}
	op := &computeRecoveryOperation{
		action:  action,
		done:    make(chan struct{}),
		status:  "refreshing",
		started: now,
		updated: now,
	}
	t.ops[key] = op
	t.mu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), computeRecoveryDetachedTimeout)
		defer cancel()
		result := run(ctx)
		t.finish(op, result)
	}()

	return op
}

func (t *computeRecoveryTracker) cleanupLocked(now time.Time) {
	for key, op := range t.ops {
		if op == nil || op.status == "refreshing" || op.finished.IsZero() {
			continue
		}
		if now.Sub(op.finished) > computeRecoveryTerminalTTL {
			delete(t.ops, key)
		}
	}
}

func (t *computeRecoveryTracker) finish(op *computeRecoveryOperation, result computeRecoveryRunResult) {
	if t == nil || op == nil {
		return
	}
	now := time.Now().UTC()
	t.mu.Lock()
	op.current = result.Current
	op.runtime = result.Runtime
	op.err = result.Err
	op.updated = now
	op.finished = now
	if result.Err != nil {
		op.status = "failed"
	} else {
		op.status = "ready"
	}
	close(op.done)
	t.mu.Unlock()
}

func (t *computeRecoveryTracker) snapshot(userID, desktopID string) (*computeRecoveryStatus, computeComputer, *computeRuntimeStatus, error, bool) {
	if t == nil {
		return nil, computeComputer{}, nil, nil, false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	op := t.ops[computeRecoveryKey(userID, desktopID)]
	if op == nil {
		return nil, computeComputer{}, nil, nil, false
	}
	status := computeRecoveryStatus{
		Active:    op.status == "refreshing",
		Status:    op.status,
		Action:    op.action,
		StartedAt: op.started.Format(time.RFC3339),
		UpdatedAt: op.updated.Format(time.RFC3339),
	}
	if !op.finished.IsZero() {
		status.FinishedAt = op.finished.Format(time.RFC3339)
	}
	if op.err != nil {
		status.Message = "current computer recovery failed"
	}
	return &status, op.current, op.runtime, op.err, true
}
