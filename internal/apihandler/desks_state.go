package apihandler

import (
	"net/http"

	"github.com/yusefmosiah/go-choir/internal/appdesk"
	"github.com/yusefmosiah/go-choir/internal/apppackage"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// desksState is the derived desk set: what the boot reconcile registered, at
// which epoch, what the image requires and what the registry still retains for
// runs in flight (plan MA4.2 and MA4.5).
type desksState struct {
	// Desks are every registered desk, sorted; AppDesks the package desks as
	// name@epoch, candidates included.
	Desks    []string `json:"desks"`
	AppDesks []string `json:"app_desks"`
	// Required are the image's required packages as package@version.
	Required []string `json:"required"`
	// Retained counts the rows kept beside the live ones and the holds on
	// them; both are 0 on a quiet computer.
	Retained int `json:"retained_rows"`
	Holders  int `json:"epoch_holders"`
	// DeskSandbox is whether this computer admits a desk from another
	// publisher (R24): the iframe sandbox is shipped and the worker sandbox is
	// gVisor in required mode with a passed boot selftest. Read-only; the
	// reason says what is missing. Same reading as the isolation selftest.
	DeskSandbox desksSandboxState `json:"desk_sandbox"`
}

// desksSandboxState is the R24 capability reading. Admits is the one answer
// the validator, the release record and the package builder read
// (apppackage.CurrentPlatform().DeskSandbox).
type desksSandboxState struct {
	Admits        bool                          `json:"admits_nonfirstparty_desks"`
	IframeSandbox bool                          `json:"iframe_sandbox"`
	Worker        yaegikernel.DeskSandboxStatus `json:"worker_sandbox"`
}

// HandleDesksState handles GET /api/desks/state for the authenticated owner:
// the derived desk set, readable without SSH. Read-only.
func (h *Handler) HandleDesksState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if _, err := authenticateUser(r); err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	out := desksState{Desks: deskmanifest.Names(), AppDesks: deskmanifest.AppEpochs(), Required: []string{}}
	required, err := appdesk.ImageRequired()
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "the image's required desk set is malformed"})
		return
	}
	for _, p := range required {
		out.Required = append(out.Required, p.Package+"@"+p.Version)
	}
	state := deskmanifest.EpochStateNow()
	out.Retained, out.Holders = state.Retained, state.Holders
	out.DeskSandbox = desksSandboxState{
		Admits:        apppackage.CurrentPlatform().DeskSandbox,
		IframeSandbox: apppackage.IframeSandboxShipped,
		Worker:        yaegikernel.DeskSandboxStatusNow(),
	}
	writeAPIJSON(w, http.StatusOK, out)
}
