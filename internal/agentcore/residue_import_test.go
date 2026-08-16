package agentcore

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

func TestImportResidueSnapshotRefusesUnboundTape(t *testing.T) {
	computerID := "computer-residue-unbound"
	productStore, err := choirstore.Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer productStore.Close()
	handler := &APIHandler{rt: &Runtime{cfg: provideriface.Config{ComputerID: computerID}, store: productStore}}
	request := httptest.NewRequest(http.MethodPost, "/api/computers/"+computerID+"/lifecycle/import-residue-snapshot", nil)
	request.Header.Set("X-Authenticated-User", "owner")
	request.Header.Set("X-Authenticated-Computer", computerID)
	response := httptest.NewRecorder()
	handler.HandleComputersRouter(response, request)
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), "bound projection tape") {
		t.Fatalf("unbound import status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestImportResidueSnapshotRefusesWrongComputerBinding(t *testing.T) {
	handler := &APIHandler{rt: &Runtime{cfg: provideriface.Config{ComputerID: "computer-a"}}}
	request := httptest.NewRequest(http.MethodPost, "/api/computers/computer-b/lifecycle/import-residue-snapshot", nil)
	request.Header.Set("X-Authenticated-User", "owner")
	request.Header.Set("X-Authenticated-Computer", "computer-b")
	response := httptest.NewRecorder()
	handler.HandleComputersRouter(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("wrong computer status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestImportResidueSnapshotRejectsGET(t *testing.T) {
	handler := &APIHandler{rt: &Runtime{cfg: provideriface.Config{ComputerID: "computer-a"}}}
	request := httptest.NewRequest(http.MethodGet, "/api/computers/computer-a/lifecycle/import-residue-snapshot", nil)
	request.Header.Set("X-Authenticated-User", "owner")
	request.Header.Set("X-Authenticated-Computer", "computer-a")
	response := httptest.NewRecorder()
	handler.HandleComputersRouter(response, request)
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET import status=%d body=%s", response.Code, response.Body.String())
	}
}
