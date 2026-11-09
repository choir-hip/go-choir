//go:build !linux

package capsule

import (
	"context"
	"time"
)

// Pause and resume (pause_linux.go): capsules need the Linux guest kernel.

// PausedSeed mirrors the Linux type.
type PausedSeed struct {
	Record   PauseRecord
	UpperDir string
	Dir      string
}

func (e *Executor) PauseCapsule(context.Context, PauseRequest) (PauseRecord, error) {
	return PauseRecord{}, stubErr("pause")
}
func (e *Executor) UnpauseCapsule(context.Context, string) error { return stubErr("unpause") }
func (e *Executor) AckPause(string, time.Time) error             { return stubErr("pause ack") }
func (e *Executor) IsPaused(string) bool                         { return false }
func (e *Executor) LookupPause(string) PauseLookup               { return PauseLookup{} }
func (e *Executor) SparedPausedIDs() []string                    { return nil }
func (e *Executor) PrepareResume(context.Context, PauseRecord) (PausedSeed, error) {
	return PausedSeed{}, stubErr("resume")
}
func (e *Executor) CompleteResume(PausedSeed) error            { return stubErr("resume") }
func (e *Executor) DiscardPause(context.Context, string) error { return stubErr("discard pause") }
func (e *Executor) DeleteSparedPauses(map[string]bool) error   { return nil }
