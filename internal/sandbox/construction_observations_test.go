package sandbox

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerversion"
)

func TestConstructionObservationHandlerRequiresInternalCallerAndReadsManifest(t *testing.T) {
	deviceRoot := t.TempDir()
	filesRoot := filepath.Join(deviceRoot, "files")
	if err := os.Mkdir(filesRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filesRoot, "proof.txt"), []byte("constructed"), 0o644); err != nil {
		t.Fatal(err)
	}
	version := computerversion.ComputerVersion{CodeRef: "code:test", ArtifactProgramRef: "program:test"}
	expected, err := computerversion.FilesystemProjectionObservationSet(context.Background(), "expected", version, filesRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := computerversion.WriteConstructionStateManifest(deviceRoot, version, expected); err != nil {
		t.Fatal(err)
	}
	handler := ConstructionObservationHandler{FilesRoot: filesRoot}
	url := "/internal/computer-version/observations?code_ref=code:test&artifact_program_ref=program:test"

	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, url, nil))
	if unauthorized.Code != http.StatusForbidden {
		t.Fatalf("unauthorized status = %d, want 403", unauthorized.Code)
	}

	request := httptest.NewRequest(http.MethodGet, url, nil)
	request.Header.Set("X-Internal-Caller", "true")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("readback status = %d body=%s", response.Code, response.Body.String())
	}
	if response.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("content type = %q", response.Header().Get("Content-Type"))
	}
	var live computerversion.LiveConstructionObservation
	if err := json.NewDecoder(response.Body).Decode(&live); err != nil {
		t.Fatal(err)
	}
	if live.State.Version != version || live.Geometry.FilesystemBytes == 0 || live.Geometry.FilesystemBlockSize == 0 || live.Geometry.AvailableBytes > live.Geometry.FilesystemBytes {
		t.Fatalf("invalid live construction observation: %+v", live)
	}
}
