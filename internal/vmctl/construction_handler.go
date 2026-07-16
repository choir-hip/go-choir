package vmctl

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerversion"
)

type constructionRequest struct {
	Version  computerversion.ComputerVersion      `json:"computer_version"`
	Identity computerversion.ConstructionIdentity `json:"identity"`
}

type constructionService struct {
	template computerversion.ProductionMaterializer
	manifest computerversion.CapabilityManifest
}

func (h *Handler) SetConstructionService(template computerversion.ProductionMaterializer, manifest computerversion.CapabilityManifest) {
	h.construction = &constructionService{template: template, manifest: manifest}
}

func (h *Handler) HandleConstructComputerVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}
	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "vmctl control endpoints are not publicly accessible"})
		return
	}
	if h.construction == nil {
		writeVMCTLJSON(w, http.StatusServiceUnavailable, vmctlErrorResponse{Error: "ComputerVersion construction service is unavailable"})
		return
	}
	var request constructionRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "invalid construction request"})
		return
	}
	identity := request.Identity
	if strings.TrimSpace(identity.RealizationID) == "" || identity.ComputerKind != "candidate" || strings.TrimSpace(identity.OwnerID) == "" || strings.TrimSpace(identity.DesktopID) == "" || strings.TrimSpace(identity.CandidateID) == "" || identity.DesktopID != identity.CandidateID || !request.Version.Valid() {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "valid ComputerVersion and unpublished candidate identity are required"})
		return
	}
	materializer := h.construction.template
	materializer.Identity = request.Identity
	result, err := materializer.Construct(r.Context(), request.Version, h.construction.manifest)
	if err != nil {
		writeVMCTLJSON(w, http.StatusConflict, vmctlErrorResponse{Error: err.Error()})
		return
	}
	writeVMCTLJSON(w, http.StatusCreated, result)
}
