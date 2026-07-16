package sandbox

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/diskinstantiation"
)

type ConstructionObservationHandler struct {
	FilesRoot string
}

func (h ConstructionObservationHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Internal-Caller")), "true") {
		http.Error(w, "internal caller required", http.StatusForbidden)
		return
	}
	filesRoot, err := filepath.Abs(strings.TrimSpace(h.FilesRoot))
	if err != nil || strings.TrimSpace(h.FilesRoot) == "" {
		http.Error(w, "construction files root unavailable", http.StatusServiceUnavailable)
		return
	}
	version := computerversion.ComputerVersion{
		CodeRef:            computerversion.CodeRef(strings.TrimSpace(r.URL.Query().Get("code_ref"))),
		ArtifactProgramRef: computerversion.ArtifactProgramRef(strings.TrimSpace(r.URL.Query().Get("artifact_program_ref"))),
	}
	observations, err := computerversion.ObserveConstructionState(r.Context(), filepath.Dir(filesRoot), filesRoot, version)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	var stat syscall.Statfs_t
	if err := syscall.Statfs(filesRoot, &stat); err != nil || stat.Bsize <= 0 {
		http.Error(w, "construction filesystem geometry unavailable", http.StatusConflict)
		return
	}
	geometry := diskinstantiation.RuntimeGeometryReceipt{
		FilesystemBytes:     uint64(stat.Blocks) * uint64(stat.Bsize),
		FilesystemBlockSize: uint64(stat.Bsize),
		AvailableBytes:      uint64(stat.Bavail) * uint64(stat.Bsize),
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(computerversion.LiveConstructionObservation{State: observations, Geometry: geometry})
}
