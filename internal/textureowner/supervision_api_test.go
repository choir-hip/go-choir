package textureowner

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTextureSupervisionImportRouteRequiresOwnerAndTrajectory(t *testing.T) {
	_, handler := testAPISetup(t)

	unauthorized := httptest.NewRequest(http.MethodPost, "/api/texture/supervision/import", nil)
	unauthorizedResult := httptest.NewRecorder()
	handler.HandleTextureRouter(unauthorizedResult, unauthorized)
	if unauthorizedResult.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized import status = %d body=%s", unauthorizedResult.Code, unauthorizedResult.Body.String())
	}

	missingTrajectory := textureRequest(t, http.MethodPost, "/api/texture/supervision/import", textureSupervisionImportRequest{})
	missingTrajectoryResult := httptest.NewRecorder()
	handler.HandleTextureRouter(missingTrajectoryResult, missingTrajectory)
	if missingTrajectoryResult.Code != http.StatusBadRequest {
		t.Fatalf("missing trajectory import status = %d body=%s", missingTrajectoryResult.Code, missingTrajectoryResult.Body.String())
	}
}

func TestTextureSupervisionImportDerivesStableCommandID(t *testing.T) {
	first := textureSupervisionImportCommandID("owner-a", "trajectory-a")
	if first == "" || first != textureSupervisionImportCommandID("owner-a", "trajectory-a") {
		t.Fatalf("derived import command must be stable: %q", first)
	}
	if first == textureSupervisionImportCommandID("owner-b", "trajectory-a") {
		t.Fatal("derived import command must remain owner scoped")
	}
}

func TestTextureSupervisionRebuildRouteRequiresOwner(t *testing.T) {
	_, handler := testAPISetup(t)
	request := httptest.NewRequest(http.MethodPost, "/api/texture/supervision/rebuild", nil)
	response := httptest.NewRecorder()
	handler.HandleTextureRouter(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized rebuild status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestTextureSupervisionRebuildRejectsOrdinaryAuthenticatedUser(t *testing.T) {
	_, handler := testAPISetup(t)
	called := 0
	handler.rebuildSupervisionProjection = func(context.Context) error {
		called++
		return nil
	}
	request := authenticatedRequest(http.MethodPost, "/api/texture/supervision/rebuild", "", "user-ordinary")
	response := httptest.NewRecorder()
	handler.HandleTextureRouter(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("ordinary rebuild status = %d body=%s", response.Code, response.Body.String())
	}
	if called != 0 {
		t.Fatalf("ordinary rebuild invoked authority %d times, want 0", called)
	}
}

func TestTextureSupervisionRebuildAllowsTrustedInternalCaller(t *testing.T) {
	_, handler := testAPISetup(t)
	called := 0
	handler.rebuildSupervisionProjection = func(context.Context) error {
		called++
		return nil
	}
	request := authenticatedRequest(http.MethodPost, "/api/texture/supervision/rebuild", "", "user-operator")
	request.Header.Set("X-Internal-Caller", "true")
	response := httptest.NewRecorder()
	handler.HandleTextureRouter(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("trusted rebuild status = %d body=%s", response.Code, response.Body.String())
	}
	if called != 1 {
		t.Fatalf("trusted rebuild invoked authority %d times, want 1", called)
	}
}
