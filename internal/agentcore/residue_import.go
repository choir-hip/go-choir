package agentcore

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/store"
)

// ResidueImportReport is the guest response for an owner-scoped snapshot
// import. It is not an eligibility receipt and does not reclassify
// EmptyUntilSupported.
type ResidueImportReport struct {
	ComputerID string `json:"computer_id"`
	Desktops   int    `json:"desktops"`
	Sessions   int    `json:"sessions"`
	Objects    int    `json:"objects"`
	Edges      int    `json:"edges"`
	Appended   bool   `json:"appended"`
}

func (h *APIHandler) importResidueSnapshot(w http.ResponseWriter, r *http.Request, computerID string) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if h == nil || h.rt == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "residue import authority unavailable"})
		return
	}
	if r.Body != nil {
		defer r.Body.Close()
	}
	report, err := h.rt.ImportResidueSnapshot(r.Context(), computerID)
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, store.ErrResidueImportUnbound), errors.Is(err, ErrReplayCompletenessUnavailable):
			status = http.StatusServiceUnavailable
		case errors.Is(err, store.ErrResidueImportSplit):
			status = http.StatusConflict
		}
		writeAPIJSON(w, status, apiError{Error: err.Error()})
		return
	}
	writeAPIJSON(w, http.StatusOK, report)
}

func (rt *Runtime) ImportResidueSnapshot(ctx context.Context, computerID string) (ResidueImportReport, error) {
	var report ResidueImportReport
	if rt == nil || rt.store == nil {
		return report, fmt.Errorf("%w: residue import authority is not configured", ErrReplayCompletenessUnavailable)
	}
	computerID = strings.TrimSpace(computerID)
	if computerID == "" || computerID != strings.TrimSpace(rt.cfg.ComputerID) {
		return report, fmt.Errorf("%w: computer binding does not match runtime", ErrReplayCompletenessUnavailable)
	}
	result, err := rt.store.ImportResidueSnapshot(ctx)
	report = ResidueImportReport{
		ComputerID: computerID,
		Desktops:   result.Desktops,
		Sessions:   result.Sessions,
		Objects:    result.Objects,
		Edges:      result.Edges,
		Appended:   result.Appended,
	}
	return report, err
}
