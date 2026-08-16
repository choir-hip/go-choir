package selfdevprotocol

import (
	"errors"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// Recovery domain for the retained computer after the unified-tape design.
// New genesis on an existing computer is refused (Reduce rejects duplicate
// genesis; choir computer create is forbidden; rematerialize is forbidden).
// Heads before CompleteFromHead reconstruct only what the tape recorded
// (today: event heads plus empty EmptyUntilSupported tables). That preserves
// tape-recovery restore of eligible checkpoints. A full projected computer
// (desktop/OG payloads) is restorable only at or after CompleteFromHead.
const (
	TapeCompletenessIncomplete       = ""
	TapeCompletenessCompleteFromHead = "complete_from_head"
	RecoveryDomainCompleteFromHead   = "complete_from_head"
	RecoveryDomainNewEpoch           = "new_epoch"
)

var (
	ErrNewEpochRefused          = errors.New("restore: new_epoch recovery domain is refused on an existing computer")
	ErrIncompleteTapeRestore    = errors.New("restore: target head is before the complete-tape boundary")
	ErrCompleteFromHeadRequired = errors.New("restore: complete-tape checkpoint omitted complete_from_head")
)

// ValidateTapeCompleteness admits incomplete-tape (tape-recovery) checkpoints
// and complete_from_head checkpoints with an explicit boundary head. new_epoch
// is refused.
func ValidateTapeCompleteness(completeness, completeFromHead string) error {
	completeness = strings.TrimSpace(completeness)
	completeFromHead = strings.TrimSpace(completeFromHead)
	switch completeness {
	case TapeCompletenessIncomplete, "incomplete":
		if completeFromHead != "" {
			return fmt.Errorf("restore: incomplete tape must not declare complete_from_head")
		}
		return nil
	case TapeCompletenessCompleteFromHead:
		if !computerevent.IsSHA256(completeFromHead) {
			return ErrCompleteFromHeadRequired
		}
		return nil
	case RecoveryDomainNewEpoch:
		return ErrNewEpochRefused
	default:
		return fmt.Errorf("restore: unknown tape completeness %q", completeness)
	}
}

// AdmitRestoreSequence refuses a full-computer restore of a head that precedes
// the declared completeness boundary. When completeness is undeclared, restore
// is the incomplete projection and the sequence is admitted (tape-recovery).
func AdmitRestoreSequence(targetSequence, completeFromSequence uint64, completenessDeclared bool) error {
	if !completenessDeclared {
		return nil
	}
	if targetSequence < completeFromSequence {
		return ErrIncompleteTapeRestore
	}
	return nil
}
