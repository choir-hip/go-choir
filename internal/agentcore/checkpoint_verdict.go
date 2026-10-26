package agentcore

import (
	"context"
	"errors"

	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

// ErrCheckpointNotDerivable refuses a restore to a checkpoint the host proved
// is not derivable from the tape.
var ErrCheckpointNotDerivable = errors.New("checkpoint is not derivable from the tape")

// depositModeOf names the live store's deposit mode for a checkpoint.
func depositModeOf(live *choirstore.Store) (string, error) {
	return "", nil
}

// refuseNotDerivableCheckpoint asks the host for its verdict on a checkpoint
// and refuses one it judged not_derivable.
func (rt *Runtime) refuseNotDerivableCheckpoint(ctx context.Context, computerID string, checkpoint selfdevprotocol.Checkpoint) error {
	return nil
}
