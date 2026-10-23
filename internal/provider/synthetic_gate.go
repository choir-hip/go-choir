package provider

import "errors"

// Skeleton for the failing tests: the names the tests use, with no behavior.
// The next commit gives them one.

// ErrConcurrencyGateWait: a call ended waiting for a Synthetic concurrency slot.
var ErrConcurrencyGateWait = errors.New("call ended while waiting for a synthetic concurrency slot")

// ErrUpstreamConcurrencyLimit: the upstream answered 429, its concurrency limit.
var ErrUpstreamConcurrencyLimit = errors.New("upstream concurrency limit")

// SyntheticGateStats is the gate's state and counters.
type SyntheticGateStats struct {
	Limit        int   `json:"limit"`
	InFlight     int   `json:"in_flight"`
	Peak         int   `json:"peak"`
	Granted      int64 `json:"granted"`
	Waited       int64 `json:"waited"`
	WaitTimeouts int64 `json:"wait_timeouts"`
}

// SyntheticGateStatus returns the gate's snapshot, false when no Synthetic provider exists.
func SyntheticGateStatus() (SyntheticGateStats, bool) { return SyntheticGateStats{}, false }
