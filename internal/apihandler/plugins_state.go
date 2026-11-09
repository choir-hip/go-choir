package apihandler

import (
	"context"
	"net/http"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/kernel"
)

// SetSlotSource gives the plugins state the shell slots the owner's installed
// packages provide (agentcore.APIHandler.InstalledSlots).
func (h *Handler) SetSlotSource(fn func(ctx context.Context, ownerID string) []agentcore.InstalledSlot) {
	h.slots = fn
}

// pluginsState is the kernel snapshot plus the installed shell providers.
type pluginsState struct {
	kernel.Snapshot
	Slots []agentcore.InstalledSlot `json:"slots"`
}

// HandlePluginsState handles GET /api/plugins/state: the Go kernel's snapshot
// (open scopes, pending effects, services, unit states and causes) for the
// authenticated owner. It is read-only and needs no SSH to inspect.
func (h *Handler) HandlePluginsState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	out := pluginsState{Snapshot: h.kernel.Snapshot(), Slots: []agentcore.InstalledSlot{}}
	if h.slots != nil {
		out.Slots = h.slots(r.Context(), ownerID)
	}
	writeAPIJSON(w, http.StatusOK, out)
}
