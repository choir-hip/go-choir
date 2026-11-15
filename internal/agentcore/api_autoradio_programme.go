package agentcore

import (
	"context"
	"net/http"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// STUB, replaced by the code commit of station L1: the station's tests fail
// against this, as the method requires.

type autoradioLedger interface {
	AppendCommitmentRecord(ctx context.Context, ownerID, computerID string, rec types.CommitmentRecord) (string, error)
	GetCommitmentRecord(ctx context.Context, ownerID, computerID, recordID string) (*types.CommitmentRecord, error)
	ListCommitmentRecordsBySubjectRef(ctx context.Context, ownerID, computerID, key, value string, limit int) ([]types.CommitmentRecord, error)
}

func (h *APIHandler) programmeLedger() autoradioLedger { return nil }

func (h *APIHandler) HandleAutoradioProgramme(w http.ResponseWriter, r *http.Request) {
	writeAPIJSON(w, http.StatusNotImplemented, apiError{Error: "not built"})
}
