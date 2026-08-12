package agentcore

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
)

// ErrWorkspaceReplaceUnavailable means the runtime cannot replace the
// VM-local workspace through the product path.
var ErrWorkspaceReplaceUnavailable = errors.New("workspace replace unavailable")

// WorkspaceReplaceReport is the product receipt for a VM-local workspace
// replacement. It is evidence, not a checkpoint or restore authority.
type WorkspaceReplaceReport struct {
	ComputerID          string                        `json:"computer_id"`
	Receipt             store.WorkspaceReplaceReceipt `json:"receipt"`
	AppendedEvent       bool                          `json:"appended_event"`
	PublishedCheckpoint bool                          `json:"published_checkpoint"`
	StoreClosed         bool                          `json:"store_closed"`
}

// ReplaceWorkspace quarantines the live VM-local Dolt workspace and opens a
// current-DDL workspace at the same path. It appends no event and publishes
// no checkpoint. The returned process has closed the store so a subsequent
// owner-scoped restart can reopen current schema exclusively.
func (rt *Runtime) ReplaceWorkspace(ctx context.Context, computerID string) (WorkspaceReplaceReport, error) {
	var report WorkspaceReplaceReport
	if rt == nil || rt.store == nil {
		return report, fmt.Errorf("%w: runtime store is not configured", ErrWorkspaceReplaceUnavailable)
	}
	computerID = strings.TrimSpace(computerID)
	if computerID == "" || computerID != strings.TrimSpace(rt.cfg.ComputerID) {
		return report, fmt.Errorf("%w: computer binding does not match runtime", ErrWorkspaceReplaceUnavailable)
	}
	if err := ctx.Err(); err != nil {
		return report, err
	}

	rt.workspaceReplaceMu.Lock()
	defer rt.workspaceReplaceMu.Unlock()
	if rt.store == nil {
		return report, fmt.Errorf("%w: runtime store is not configured", ErrWorkspaceReplaceUnavailable)
	}

	originalPath := strings.TrimSpace(rt.store.Path())
	if originalPath == "" {
		return report, fmt.Errorf("%w: store path is required", ErrWorkspaceReplaceUnavailable)
	}
	quarantineDir := filepath.Join(filepath.Dir(originalPath), "workspace-replaced-"+time.Now().UTC().Format("20060102T150405.000000000Z"))
	fresh, receipt, err := rt.store.ReplaceWorkspace(quarantineDir)
	rt.store = nil
	report.ComputerID = computerID
	report.Receipt = receipt
	report.AppendedEvent = false
	report.PublishedCheckpoint = false
	if err != nil {
		if fresh != nil {
			_ = fresh.Close()
		}
		return report, err
	}
	if err := fresh.Close(); err != nil {
		return report, fmt.Errorf("workspace replace: close current schema: %w", err)
	}
	report.StoreClosed = true
	return report, nil
}

func (h *APIHandler) replaceComputerWorkspace(w http.ResponseWriter, r *http.Request, computerID string) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if h == nil || h.rt == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "workspace replace authority unavailable"})
		return
	}
	report, err := h.rt.ReplaceWorkspace(r.Context(), computerID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrWorkspaceReplaceUnavailable) {
			status = http.StatusServiceUnavailable
		}
		writeAPIJSON(w, status, apiError{Error: err.Error()})
		return
	}
	writeAPIJSON(w, http.StatusOK, report)
}
