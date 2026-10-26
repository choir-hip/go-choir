package agentcore

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/projectionbase"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

// ErrCheckpointNotDerivable refuses a restore to a checkpoint the host proved
// is not derivable from the tape: its tape-only rebuild at the checkpoint's
// head differs from the checkpoint's witness, and both were built in the same
// deposit mode (selfdevprotocol.JudgeCheckpointWitness).
var ErrCheckpointNotDerivable = errors.New("checkpoint is not derivable from the tape")

// depositModeOf names the live store's deposit mode for a checkpoint: upcast
// when its deposits carry an upcast ledger, as every host rebuild's do,
// retained when it kept the bytes its writers recorded. The host needs it to
// tell residual R1 (a mismatch that is only the mode) from rows the tape
// cannot rebuild.
func depositModeOf(live *choirstore.Store) (string, error) {
	upcast, err := live.DepositsUpcast()
	if err != nil {
		return "", fmt.Errorf("read live deposit mode: %w", err)
	}
	if upcast {
		return selfdevprotocol.DepositModeUpcast, nil
	}
	return selfdevprotocol.DepositModeRetained, nil
}

// checkpointVerdict asks the host for its verdict on a checkpoint. found is
// false when the source cannot give verdicts, has none for this checkpoint
// (one published before the host judged them), or cannot be reached: the
// verdict is the host's early word, never a precondition, so every one of
// those leaves restore to its own reconstruction gate.
func (rt *Runtime) checkpointVerdict(ctx context.Context, computerID string, checkpoint selfdevprotocol.Checkpoint) (selfdevprotocol.CheckpointVerdict, bool) {
	src, err := rt.resolveRestoreBaseSource()
	if err != nil {
		return selfdevprotocol.CheckpointVerdict{}, false
	}
	verdicts, ok := src.(projectionbase.CheckpointVerdictSource)
	if !ok {
		return selfdevprotocol.CheckpointVerdict{}, false
	}
	verdict, found, err := verdicts.CheckpointVerdict(ctx, computerID, checkpoint.Digest)
	if err != nil {
		log.Printf("checkpoint verdict: read %s: %v", checkpoint.Digest, err)
		return selfdevprotocol.CheckpointVerdict{}, false
	}
	return verdict, found
}

// refuseNotDerivableCheckpoint refuses a checkpoint the host judged
// not_derivable, before restore installs a base or replays an event. Pending,
// verified and unverified (residual R1 included) checkpoints go on: restore
// reconstructs to the checkpoint's head in the live deposit mode and refuses a
// witness mismatch itself.
func (rt *Runtime) refuseNotDerivableCheckpoint(ctx context.Context, computerID string, checkpoint selfdevprotocol.Checkpoint) error {
	verdict, found := rt.checkpointVerdict(ctx, computerID, checkpoint)
	if !found || verdict.Status != selfdevprotocol.CheckpointVerdictNotDerivable {
		return nil
	}
	return fmt.Errorf("%w: the host rebuilt the tape at the checkpoint's head and %s (tables: %s)",
		ErrCheckpointNotDerivable, verdict.Reason, strings.Join(verdict.DifferingTables, ", "))
}
