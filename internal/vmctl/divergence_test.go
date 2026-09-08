package vmctl

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDivergenceStatusWorkflow(t *testing.T) {
	registry := NewOwnershipRegistry("http://127.0.0.1:8085")
	handler := NewHandler(registry)

	// Assign an initial VM
	req, _ := http.NewRequest(http.MethodPost, "/internal/vmctl/resolve", bytes.NewBufferString(`{"user_id":"user-1","desktop_id":"primary"}`))
	req.Header.Set("X-Internal-Caller", "true")
	rec := httptest.NewRecorder()
	handler.HandleResolve(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("resolve status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resolveResp resolveResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resolveResp); err != nil {
		t.Fatalf("unmarshal resolve: %v", err)
	}

	// 1. Initial list: should default to "tracking" for normal interactive VM
	listReq, _ := http.NewRequest(http.MethodGet, "/internal/vmctl/list", nil)
	listReq.Header.Set("X-Internal-Caller", "true")
	listRec := httptest.NewRecorder()
	handler.HandleList(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d", listRec.Code, http.StatusOK)
	}

	var listData struct {
		Ownerships []ownershipResponse `json:"ownerships"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listData); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	if len(listData.Ownerships) == 0 {
		t.Fatalf("expected at least 1 ownership")
	}
	own := listData.Ownerships[0]
	if own.DivergenceStatus != "tracking" {
		t.Fatalf("expected default divergence_status = 'tracking', got: %s", own.DivergenceStatus)
	}

	// 2. Set divergence to "divergent" via /internal/vmctl/divergence
	setReqBody, _ := json.Marshal(setDivergenceRequest{
		ComputerID:       own.ComputerID,
		DivergenceStatus: "divergent",
		PlatformBaseRef:  "main@abc1234",
	})
	setReq, _ := http.NewRequest(http.MethodPost, "/internal/vmctl/divergence", bytes.NewReader(setReqBody))
	setReq.Header.Set("X-Internal-Caller", "true")
	setRec := httptest.NewRecorder()
	handler.HandleSetDivergence(setRec, setReq)
	if setRec.Code != http.StatusOK {
		t.Fatalf("set divergence status = %d, want %d", setRec.Code, http.StatusOK)
	}

	// 3. Verify list reflects explicit divergence status
	listRec2 := httptest.NewRecorder()
	handler.HandleList(listRec2, listReq)
	if err := json.Unmarshal(listRec2.Body.Bytes(), &listData); err != nil {
		t.Fatalf("unmarshal list 2: %v", err)
	}
	own2 := listData.Ownerships[0]
	if own2.DivergenceStatus != "divergent" {
		t.Fatalf("expected divergence_status = 'divergent', got: %s", own2.DivergenceStatus)
	}
	if own2.PlatformBaseRef != "main@abc1234" {
		t.Fatalf("expected platform_base_ref = 'main@abc1234', got: %s", own2.PlatformBaseRef)
	}
}

func TestRefreshRejectsDivergentWithoutForce(t *testing.T) {
	registry := NewOwnershipRegistry("http://127.0.0.1:8085")
	handler := NewHandler(registry)

	// 1. Resolve computer
	resolveReq, _ := http.NewRequest(http.MethodPost, "/internal/vmctl/resolve", bytes.NewBufferString(`{"user_id":"user-divergent","desktop_id":"primary"}`))
	resolveReq.Header.Set("X-Internal-Caller", "true")
	resolveRec := httptest.NewRecorder()
	handler.HandleResolve(resolveRec, resolveReq)
	if resolveRec.Code != http.StatusOK {
		t.Fatalf("resolve status = %d, want %d", resolveRec.Code, http.StatusOK)
	}

	var resolveResp resolveResponse
	if err := json.Unmarshal(resolveRec.Body.Bytes(), &resolveResp); err != nil {
		t.Fatalf("unmarshal resolve: %v", err)
	}

	// 2. Mark computer as divergent
	if err := registry.SetDivergenceStatus(resolveResp.ComputerID, "divergent", "main@xyz"); err != nil {
		t.Fatalf("SetDivergenceStatus failed: %v", err)
	}

	// 3. Attempt refresh without force: should conflict (409)
	refreshReq, _ := http.NewRequest(http.MethodPost, "/internal/vmctl/refresh", bytes.NewBufferString(`{"user_id":"user-divergent","desktop_id":"primary"}`))
	refreshReq.Header.Set("X-Internal-Caller", "true")
	refreshRec := httptest.NewRecorder()
	handler.HandleRefresh(refreshRec, refreshReq)
	if refreshRec.Code != http.StatusConflict {
		t.Fatalf("refresh without force status = %d, want %d; body = %s", refreshRec.Code, http.StatusConflict, refreshRec.Body.String())
	}

	// 4. Attempt refresh with force: true
	refreshReqForce, _ := http.NewRequest(http.MethodPost, "/internal/vmctl/refresh", bytes.NewBufferString(`{"user_id":"user-divergent","desktop_id":"primary","force":true}`))
	refreshReqForce.Header.Set("X-Internal-Caller", "true")
	refreshRecForce := httptest.NewRecorder()
	handler.HandleRefresh(refreshRecForce, refreshReqForce)
	if refreshRecForce.Code != http.StatusOK {
		t.Fatalf("refresh with force status = %d, want %d; body = %s", refreshRecForce.Code, http.StatusOK, refreshRecForce.Body.String())
	}
}
