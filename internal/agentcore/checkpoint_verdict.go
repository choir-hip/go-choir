package agentcore

import (
	"errors"
	"fmt"

	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

// ErrCheckpointNotDerivable refuses a restore to a checkpoint whose witness
// the reconstruction at its head does not reproduce, when both were built in
// the same deposit mode (selfdevprotocol.JudgeCheckpointWitness). The host no
// longer rebuilds checkpoints (the projection bases are gone), so restore's own
// reconstruction gate is the only judge.
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
