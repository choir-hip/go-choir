package agentcore

import (
	"context"
	"sync"
	"time"
)

// Automatic checkpoints (docs/design/apply-checkpoint-without-replay-2026-10-26.md,
// 4.1, 4.2 and slice 3).

const (
	defaultCheckpointEveryEvents  uint64 = 2500
	defaultCheckpointEvery               = 60 * time.Minute
	checkpointCadenceAttemptLimit        = 2
)

type checkpointCadenceConfig struct {
	EveryEvents uint64
	Every       time.Duration
}

func checkpointCadenceConfigFromEnv(getenv func(string) string) checkpointCadenceConfig {
	return checkpointCadenceConfig{}
}

// checkpointMark is the newest published checkpoint: its head sequence and
// when it was published.
type checkpointMark struct {
	Sequence uint64
	At       time.Time
}

func (c checkpointCadenceConfig) due(headSequence uint64, last checkpointMark, now time.Time) (bool, string) {
	return false, ""
}

type cadenceOutcome string

const (
	cadenceIdle    cadenceOutcome = "idle"
	cadenceBound   cadenceOutcome = "bound"
	cadenceFailed  cadenceOutcome = "failed"
	cadenceOff     cadenceOutcome = "off"
	cadenceSkipped cadenceOutcome = "skipped"
	cadenceBackoff cadenceOutcome = "backoff"
)

// CheckpointCadenceStatus is what the listing shows of the cadence.
type CheckpointCadenceStatus struct {
	State             string `json:"state"`
	Cause             string `json:"cause,omitempty"`
	EveryEvents       uint64 `json:"every_events"`
	Every             string `json:"every"`
	LastBoundAt       string `json:"last_bound_at,omitempty"`
	LastBoundSequence uint64 `json:"last_bound_sequence,omitempty"`
	LastBindMillis    int64  `json:"last_bind_millis,omitempty"`
	NotBefore         string `json:"not_before,omitempty"`
}

type checkpointCadence struct {
	mu                sync.Mutex
	cfg               *checkpointCadenceConfig
	initialDelay      time.Duration
	tickEvery         time.Duration
	last              func(ctx context.Context) (checkpointMark, bool, error)
	bind              func(ctx context.Context) (uint64, error)
	mark              checkpointMark
	markKnown         bool
	notBefore         time.Time
	failures          int
	lastBindDuration  time.Duration
	lastBoundAt       time.Time
	lastBoundSequence uint64
	off               string
	stop              chan struct{}
	// measure replaces the clock a bind is timed with (tests).
	measure func() time.Duration
}

func (rt *Runtime) checkpointCadenceTick(ctx context.Context, now time.Time) (cadenceOutcome, error) {
	return cadenceSkipped, nil
}

func (rt *Runtime) startCheckpointCadence(ctx context.Context) {}

func (rt *Runtime) stopCheckpointCadence() {}

func (rt *Runtime) resumeCheckpointCadence() error { return nil }

func (rt *Runtime) CheckpointCadenceStatus() CheckpointCadenceStatus {
	return CheckpointCadenceStatus{}
}
