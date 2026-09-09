package projectionbase

import (
	"errors"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// ErrBaseRefused is the typed visible refusal for every required-base
// failure: missing, foreign, corrupt, non-ancestor, or incompatible. Recovery
// paths map it to a refusal that leaves the active route and accepted
// realization unchanged; it must never fall through to genesis replay.
var ErrBaseRefused = errors.New("projection base refused")

// VerifyForRecovery authenticates a required base as a verified accelerator
// for this computer and chain position before installation. It binds computer,
// watermark W with its canonical head, blob identity, reducer/schema/
// vocabulary compatibility, and witness — but it cannot prove ancestry alone:
// a well-formed foreign head at the same sequence passes here and is caught
// by VerifyTailHead against immutable tape.
func (d Descriptor) VerifyForRecovery(computerID, targetHead string, targetSequence uint64) error {
	if err := d.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrBaseRefused, err)
	}
	if strings.TrimSpace(computerID) == "" || d.ComputerID != strings.TrimSpace(computerID) {
		return fmt.Errorf("%w: base computer %q is not this computer", ErrBaseRefused, d.ComputerID)
	}
	if !computerevent.IsSHA256(targetHead) {
		return fmt.Errorf("%w: recovery target head must be lowercase SHA-256", ErrBaseRefused)
	}
	if targetSequence == 0 {
		return fmt.Errorf("%w: recovery target sequence must be positive", ErrBaseRefused)
	}
	if d.Sequence > targetSequence {
		return fmt.Errorf("%w: base watermark %d is after recovery target %d", ErrBaseRefused, d.Sequence, targetSequence)
	}
	if d.Sequence == targetSequence && d.CanonicalHead != strings.ToLower(strings.TrimSpace(targetHead)) {
		return fmt.Errorf("%w: base head does not match recovery target", ErrBaseRefused)
	}
	if d.ReducerVersion != computerevent.ReducerVersionV1 {
		return fmt.Errorf("%w: base reducer version %d is incompatible with live %d", ErrBaseRefused, d.ReducerVersion, computerevent.ReducerVersionV1)
	}
	if d.SchemaVersion != computerevent.SchemaVersionV1 {
		return fmt.Errorf("%w: base schema version %d is incompatible with live %d", ErrBaseRefused, d.SchemaVersion, computerevent.SchemaVersionV1)
	}
	return nil
}

// VerifyTailHead proves base ancestry from immutable tape: the first event of
// the tail (W,H] must chain exactly from the base head. A non-ancestor W —
// right computer, right sequence, foreign head — refuses here, never in
// replay mechanics. The full tail still replays event-by-event afterwards;
// this gate only admits the tail start.
func (d Descriptor) VerifyTailHead(first computerevent.Event) error {
	if first.ComputerID != d.ComputerID {
		return fmt.Errorf("%w: tail computer %q is not base computer %q", ErrBaseRefused, first.ComputerID, d.ComputerID)
	}
	if first.Sequence != d.Sequence+1 {
		return fmt.Errorf("%w: tail starts at sequence %d, want %d", ErrBaseRefused, first.Sequence, d.Sequence+1)
	}
	if first.PreviousHead != d.CanonicalHead {
		return fmt.Errorf("%w: tail does not chain from base head", ErrBaseRefused)
	}
	return nil
}
