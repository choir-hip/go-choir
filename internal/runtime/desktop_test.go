//go:build comprehensive

package runtime

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// bytesReader is a convenience wrapper for creating an io.Reader from a byte slice.
func bytesReader(b []byte) *bytes.Reader {
	return bytes.NewReader(b)
}

func TestDesktopStateGetUnauthenticated(t *testing.T) {
	t.Parallel()
	_, h := testAPISetup(t)

	req := httptest.NewRequest(http.MethodGet, "/api/desktop/state", nil)
	// No X-Authenticated-User header — should be denied.
	w := httptest.NewRecorder()
	h.HandleDesktopStateGet(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestDesktopStateGetEmpty(t *testing.T) {
	t.Parallel()
	_, h := testAPISetup(t)

	req := httptest.NewRequest(http.MethodGet, "/api/desktop/state", nil)
	req.Header.Set("X-Authenticated-User", "user-1")
	w := httptest.NewRecorder()
	h.HandleDesktopStateGet(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp desktopStateGetResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.OwnerID != "user-1" {
		t.Errorf("OwnerID = %q, want %q", resp.OwnerID, "user-1")
	}
	if len(resp.Windows) != 0 {
		t.Errorf("Windows count = %d, want 0", len(resp.Windows))
	}
}

func TestDesktopStateSaveAndGet(t *testing.T) {
	t.Parallel()
	_, h := testAPISetup(t)

	// Save desktop state.
	saveReq := desktopStateSaveRequest{
		Windows: []types.WindowState{
			{
				WindowID: "win-1",
				AppID:    "vtext",
				Title:    "E-Text Editor",
				Geometry: types.WindowGeometry{X: 100, Y: 100, Width: 600, Height: 400},
				Mode:     types.WindowNormal,
				ZIndex:   1,
			},
		},
		ActiveWindowID: "win-1",
	}

	body, _ := json.Marshal(saveReq)
	req := httptest.NewRequest(http.MethodPut, "/api/desktop/state", bytesReader(body))
	req.Header.Set("X-Authenticated-User", "user-1")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.HandleDesktopStateSave(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("save status = %d, want %d", w.Code, http.StatusOK)
	}

	// Get the saved state.
	getReq := httptest.NewRequest(http.MethodGet, "/api/desktop/state", nil)
	getReq.Header.Set("X-Authenticated-User", "user-1")
	getW := httptest.NewRecorder()
	h.HandleDesktopStateGet(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d", getW.Code, http.StatusOK)
	}

	var resp desktopStateGetResponse
	if err := json.NewDecoder(getW.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.OwnerID != "user-1" {
		t.Errorf("OwnerID = %q, want %q", resp.OwnerID, "user-1")
	}
	if len(resp.Windows) != 1 {
		t.Fatalf("Windows count = %d, want 1", len(resp.Windows))
	}
	if resp.Windows[0].WindowID != "win-1" {
		t.Errorf("Window[0].WindowID = %q, want %q", resp.Windows[0].WindowID, "win-1")
	}
	if resp.Windows[0].AppID != "vtext" {
		t.Errorf("Window[0].AppID = %q, want %q", resp.Windows[0].AppID, "vtext")
	}
	if resp.ActiveWindowID != "win-1" {
		t.Errorf("ActiveWindowID = %q, want %q", resp.ActiveWindowID, "win-1")
	}
}

func TestDesktopStateSaveSanitizesInvalidWindowRecords(t *testing.T) {
	t.Parallel()
	_, h := testAPISetup(t)

	saveReq := desktopStateSaveRequest{
		Windows: []types.WindowState{
			{
				WindowID: "",
				AppID:    "vtext",
				Title:    "Missing ID",
				Geometry: types.WindowGeometry{Width: 0, Height: -1},
				Mode:     types.WindowMode("floating"),
			},
			{
				WindowID: "dup",
				AppID:    "terminal",
				Title:    "Duplicate 1",
				Geometry: types.WindowGeometry{Width: 300, Height: 200},
				Mode:     types.WindowMinimized,
			},
			{
				WindowID: "dup",
				AppID:    "files",
				Title:    "Duplicate 2",
				Geometry: types.WindowGeometry{Width: 320, Height: 240},
				Mode:     types.WindowNormal,
			},
		},
		ActiveWindowID: "missing-active-window",
	}

	body, _ := json.Marshal(saveReq)
	req := httptest.NewRequest(http.MethodPut, "/api/desktop/state", bytesReader(body))
	req.Header.Set("X-Authenticated-User", "user-1")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.HandleDesktopStateSave(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("save status = %d, want %d", w.Code, http.StatusOK)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/desktop/state", nil)
	getReq.Header.Set("X-Authenticated-User", "user-1")
	getW := httptest.NewRecorder()
	h.HandleDesktopStateGet(getW, getReq)
	if getW.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d", getW.Code, http.StatusOK)
	}

	var resp desktopStateGetResponse
	if err := json.NewDecoder(getW.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Windows) != 3 {
		t.Fatalf("Windows count = %d, want 3", len(resp.Windows))
	}
	seen := map[string]struct{}{}
	for _, win := range resp.Windows {
		if win.WindowID == "" {
			t.Fatalf("found empty WindowID in %+v", resp.Windows)
		}
		if _, ok := seen[win.WindowID]; ok {
			t.Fatalf("found duplicate WindowID %q in %+v", win.WindowID, resp.Windows)
		}
		seen[win.WindowID] = struct{}{}
		if !win.Mode.Valid() {
			t.Fatalf("found invalid mode %q in %+v", win.Mode, resp.Windows)
		}
		if win.Geometry.Width <= 0 || win.Geometry.Height <= 0 {
			t.Fatalf("found invalid geometry %+v in %+v", win.Geometry, resp.Windows)
		}
	}
	if _, ok := seen["dup-2"]; !ok {
		t.Fatalf("sanitized duplicate ID missing dup-2 in %+v", resp.Windows)
	}
	if resp.Windows[0].Mode != types.WindowNormal {
		t.Fatalf("first window mode = %q, want %q", resp.Windows[0].Mode, types.WindowNormal)
	}
	if resp.ActiveWindowID != "dup-2" {
		t.Fatalf("ActiveWindowID = %q, want top visible sanitized window dup-2", resp.ActiveWindowID)
	}
}

func TestDesktopStateActiveWindowFollowsTopVisibleZOrder(t *testing.T) {
	t.Parallel()
	_, h := testAPISetup(t)

	saveReq := desktopStateSaveRequest{
		Windows: []types.WindowState{
			{
				WindowID: "win-email",
				AppID:    "email",
				Title:    "Email",
				Geometry: types.WindowGeometry{X: 10, Y: 20, Width: 600, Height: 400},
				Mode:     types.WindowNormal,
				ZIndex:   2,
			},
			{
				WindowID: "win-trace",
				AppID:    "trace",
				Title:    "Trace",
				Geometry: types.WindowGeometry{X: 30, Y: 40, Width: 700, Height: 500},
				Mode:     types.WindowNormal,
				ZIndex:   8,
			},
		},
		ActiveWindowID: "win-email",
		Driver:         true,
	}
	body, _ := json.Marshal(saveReq)
	req := httptest.NewRequest(http.MethodPut, "/api/desktop/state", bytesReader(body))
	req.Header.Set("X-Authenticated-User", "user-1")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Choir-Session", "session-a")
	w := httptest.NewRecorder()
	h.HandleDesktopStateSave(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("save status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	get := httptest.NewRequest(http.MethodGet, "/api/desktop/state", nil)
	get.Header.Set("X-Authenticated-User", "user-1")
	get.Header.Set("X-Choir-Session", "session-a")
	getW := httptest.NewRecorder()
	h.HandleDesktopStateGet(getW, get)
	if getW.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d; body: %s", getW.Code, http.StatusOK, getW.Body.String())
	}
	var resp desktopStateGetResponse
	if err := json.NewDecoder(getW.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ActiveWindowID != "win-trace" {
		t.Fatalf("ActiveWindowID = %q, want top visible win-trace", resp.ActiveWindowID)
	}
}

func TestDesktopStateSaveUnauthenticated(t *testing.T) {
	t.Parallel()
	_, h := testAPISetup(t)

	saveReq := desktopStateSaveRequest{
		Windows:        []types.WindowState{},
		ActiveWindowID: "",
	}

	body, _ := json.Marshal(saveReq)
	req := httptest.NewRequest(http.MethodPut, "/api/desktop/state", bytesReader(body))
	// No X-Authenticated-User header — should be denied.
	w := httptest.NewRecorder()
	h.HandleDesktopStateSave(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestDesktopStateUserIsolation(t *testing.T) {
	t.Parallel()
	_, h := testAPISetup(t)

	// Save state for user-1.
	saveReq1 := desktopStateSaveRequest{
		Windows: []types.WindowState{
			{WindowID: "win-a", AppID: "vtext", Title: "User 1 Doc", Geometry: types.WindowGeometry{X: 10, Y: 10, Width: 400, Height: 300}, Mode: types.WindowNormal, ZIndex: 1},
		},
		ActiveWindowID: "win-a",
	}
	body1, _ := json.Marshal(saveReq1)
	req1 := httptest.NewRequest(http.MethodPut, "/api/desktop/state", bytesReader(body1))
	req1.Header.Set("X-Authenticated-User", "user-1")
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	h.HandleDesktopStateSave(w1, req1)

	if w1.Code != http.StatusOK {
		t.Fatalf("save user-1 status = %d, want %d", w1.Code, http.StatusOK)
	}

	// Save state for user-2.
	saveReq2 := desktopStateSaveRequest{
		Windows: []types.WindowState{
			{WindowID: "win-b", AppID: "terminal", Title: "User 2 Terminal", Geometry: types.WindowGeometry{X: 20, Y: 20, Width: 500, Height: 400}, Mode: types.WindowNormal, ZIndex: 1},
		},
		ActiveWindowID: "win-b",
	}
	body2, _ := json.Marshal(saveReq2)
	req2 := httptest.NewRequest(http.MethodPut, "/api/desktop/state", bytesReader(body2))
	req2.Header.Set("X-Authenticated-User", "user-2")
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	h.HandleDesktopStateSave(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("save user-2 status = %d, want %d", w2.Code, http.StatusOK)
	}

	// Verify user-1's state is independent.
	getReq1 := httptest.NewRequest(http.MethodGet, "/api/desktop/state", nil)
	getReq1.Header.Set("X-Authenticated-User", "user-1")
	getW1 := httptest.NewRecorder()
	h.HandleDesktopStateGet(getW1, getReq1)

	var resp1 desktopStateGetResponse
	if err := json.NewDecoder(getW1.Body).Decode(&resp1); err != nil {
		t.Fatalf("decode user-1 response: %v", err)
	}
	if len(resp1.Windows) != 1 || resp1.Windows[0].AppID != "vtext" {
		t.Errorf("user-1 desktop state was affected by user-2 save")
	}

	// Verify user-2's state is independent.
	getReq2 := httptest.NewRequest(http.MethodGet, "/api/desktop/state", nil)
	getReq2.Header.Set("X-Authenticated-User", "user-2")
	getW2 := httptest.NewRecorder()
	h.HandleDesktopStateGet(getW2, getReq2)

	var resp2 desktopStateGetResponse
	if err := json.NewDecoder(getW2.Body).Decode(&resp2); err != nil {
		t.Fatalf("decode user-2 response: %v", err)
	}
	if len(resp2.Windows) != 1 || resp2.Windows[0].AppID != "terminal" {
		t.Errorf("user-2 desktop state incorrect")
	}
}

func TestDesktopStateRouterMethodDispatch(t *testing.T) {
	t.Parallel()
	_, h := testAPISetup(t)

	// POST should be method not allowed.
	req := httptest.NewRequest(http.MethodPost, "/api/desktop/state", nil)
	req.Header.Set("X-Authenticated-User", "user-1")
	w := httptest.NewRecorder()
	h.HandleDesktopState(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}

	// DELETE should be method not allowed.
	req2 := httptest.NewRequest(http.MethodDelete, "/api/desktop/state", nil)
	req2.Header.Set("X-Authenticated-User", "user-1")
	w2 := httptest.NewRecorder()
	h.HandleDesktopState(w2, req2)

	if w2.Code != http.StatusMethodNotAllowed {
		t.Errorf("DELETE status = %d, want %d", w2.Code, http.StatusMethodNotAllowed)
	}

	// GET should work.
	req3 := httptest.NewRequest(http.MethodGet, "/api/desktop/state", nil)
	req3.Header.Set("X-Authenticated-User", "user-1")
	w3 := httptest.NewRecorder()
	h.HandleDesktopState(w3, req3)

	if w3.Code != http.StatusOK {
		t.Errorf("GET status = %d, want %d", w3.Code, http.StatusOK)
	}
}

func TestDesktopStateSaveAndGetByDesktopSelector(t *testing.T) {
	t.Parallel()
	_, h := testAPISetup(t)

	saveReq := desktopStateSaveRequest{
		Windows: []types.WindowState{
			{
				WindowID: "win-branch",
				AppID:    "vtext",
				Title:    "Branch desktop",
				Geometry: types.WindowGeometry{X: 50, Y: 60, Width: 700, Height: 500},
				Mode:     types.WindowNormal,
				ZIndex:   1,
			},
		},
		ActiveWindowID: "win-branch",
	}

	body, _ := json.Marshal(saveReq)
	save := httptest.NewRequest(http.MethodPut, "/api/desktop/state?desktop_id=branch-a", bytesReader(body))
	save.Header.Set("X-Authenticated-User", "user-1")
	save.Header.Set("Content-Type", "application/json")
	saveW := httptest.NewRecorder()
	h.HandleDesktopStateSave(saveW, save)
	if saveW.Code != http.StatusOK {
		t.Fatalf("save status = %d, want %d", saveW.Code, http.StatusOK)
	}

	getBranch := httptest.NewRequest(http.MethodGet, "/api/desktop/state?desktop_id=branch-a", nil)
	getBranch.Header.Set("X-Authenticated-User", "user-1")
	getBranchW := httptest.NewRecorder()
	h.HandleDesktopStateGet(getBranchW, getBranch)
	if getBranchW.Code != http.StatusOK {
		t.Fatalf("branch get status = %d, want %d", getBranchW.Code, http.StatusOK)
	}
	var branchResp desktopStateGetResponse
	if err := json.NewDecoder(getBranchW.Body).Decode(&branchResp); err != nil {
		t.Fatalf("decode branch response: %v", err)
	}
	if branchResp.DesktopID != "branch-a" {
		t.Errorf("branch DesktopID = %q, want %q", branchResp.DesktopID, "branch-a")
	}
	if len(branchResp.Windows) != 1 || branchResp.Windows[0].WindowID != "win-branch" {
		t.Fatalf("branch desktop windows mismatch: %+v", branchResp.Windows)
	}

	getPrimary := httptest.NewRequest(http.MethodGet, "/api/desktop/state", nil)
	getPrimary.Header.Set("X-Authenticated-User", "user-1")
	getPrimaryW := httptest.NewRecorder()
	h.HandleDesktopStateGet(getPrimaryW, getPrimary)
	if getPrimaryW.Code != http.StatusOK {
		t.Fatalf("primary get status = %d, want %d", getPrimaryW.Code, http.StatusOK)
	}
	var primaryResp desktopStateGetResponse
	if err := json.NewDecoder(getPrimaryW.Body).Decode(&primaryResp); err != nil {
		t.Fatalf("decode primary response: %v", err)
	}
	if primaryResp.DesktopID != types.PrimaryDesktopID {
		t.Errorf("primary DesktopID = %q, want %q", primaryResp.DesktopID, types.PrimaryDesktopID)
	}
	if len(primaryResp.Windows) != 0 {
		t.Fatalf("expected empty primary desktop state, got %+v", primaryResp.Windows)
	}
}

func TestDesktopStatePassiveSessionCannotReplaceSharedState(t *testing.T) {
	t.Parallel()
	_, h := testAPISetup(t)

	driverReq := desktopStateSaveRequest{
		Windows: []types.WindowState{
			{
				WindowID: "win-a",
				AppID:    "vtext",
				Title:    "Driver document",
				Geometry: types.WindowGeometry{X: 10, Y: 20, Width: 600, Height: 400},
				Mode:     types.WindowNormal,
				ZIndex:   2,
			},
		},
		ActiveWindowID: "win-a",
		Driver:         true,
	}
	body, _ := json.Marshal(driverReq)
	saveA := httptest.NewRequest(http.MethodPut, "/api/desktop/state", bytesReader(body))
	saveA.Header.Set("X-Authenticated-User", "user-1")
	saveA.Header.Set("Content-Type", "application/json")
	saveA.Header.Set("X-Choir-Session", "session-a")
	saveA.Header.Set("X-Choir-Device", "device-a")
	wA := httptest.NewRecorder()
	h.HandleDesktopStateSave(wA, saveA)
	if wA.Code != http.StatusOK {
		t.Fatalf("driver save status = %d, want %d", wA.Code, http.StatusOK)
	}

	passiveReq := desktopStateSaveRequest{
		Windows: []types.WindowState{
			{
				WindowID: "win-b",
				AppID:    "terminal",
				Title:    "Stale passive terminal",
				Geometry: types.WindowGeometry{X: 300, Y: 320, Width: 500, Height: 300},
				Mode:     types.WindowNormal,
				ZIndex:   99,
			},
		},
		ActiveWindowID: "win-b",
		Driver:         false,
	}
	passiveBody, _ := json.Marshal(passiveReq)
	saveB := httptest.NewRequest(http.MethodPut, "/api/desktop/state", bytesReader(passiveBody))
	saveB.Header.Set("X-Authenticated-User", "user-1")
	saveB.Header.Set("Content-Type", "application/json")
	saveB.Header.Set("X-Choir-Session", "session-b")
	saveB.Header.Set("X-Choir-Device", "device-b")
	wB := httptest.NewRecorder()
	h.HandleDesktopStateSave(wB, saveB)
	if wB.Code != http.StatusOK {
		t.Fatalf("passive save status = %d, want %d", wB.Code, http.StatusOK)
	}

	get := httptest.NewRequest(http.MethodGet, "/api/desktop/state", nil)
	get.Header.Set("X-Authenticated-User", "user-1")
	get.Header.Set("X-Choir-Session", "session-a")
	getW := httptest.NewRecorder()
	h.HandleDesktopStateGet(getW, get)
	if getW.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d", getW.Code, http.StatusOK)
	}
	var resp desktopStateGetResponse
	if err := json.NewDecoder(getW.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Windows) != 1 || resp.Windows[0].AppID != "vtext" || resp.Windows[0].WindowID != "win-a" {
		t.Fatalf("passive session replaced shared state: %+v", resp.Windows)
	}
}

func TestDesktopStateSessionsConvergeOnLatestDriverPlacement(t *testing.T) {
	t.Parallel()
	_, h := testAPISetup(t)

	saveForSession := func(sessionID string, x int) {
		t.Helper()
		saveReq := desktopStateSaveRequest{
			Windows: []types.WindowState{
				{
					WindowID: "win-shared",
					AppID:    "vtext",
					Title:    "Shared document",
					Geometry: types.WindowGeometry{X: x, Y: 40, Width: 600, Height: 400},
					Mode:     types.WindowNormal,
					ZIndex:   4,
				},
			},
			ActiveWindowID: "win-shared",
			Driver:         true,
		}
		body, _ := json.Marshal(saveReq)
		req := httptest.NewRequest(http.MethodPut, "/api/desktop/state", bytesReader(body))
		req.Header.Set("X-Authenticated-User", "user-1")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Choir-Session", sessionID)
		w := httptest.NewRecorder()
		h.HandleDesktopStateSave(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s save status = %d, want %d", sessionID, w.Code, http.StatusOK)
		}
	}

	saveForSession("desktop-session", 20)
	saveForSession("mobile-session", 360)

	getX := func(sessionID string) int {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/desktop/state", nil)
		req.Header.Set("X-Authenticated-User", "user-1")
		req.Header.Set("X-Choir-Session", sessionID)
		w := httptest.NewRecorder()
		h.HandleDesktopStateGet(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s get status = %d, want %d", sessionID, w.Code, http.StatusOK)
		}
		var resp desktopStateGetResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decode %s response: %v", sessionID, err)
		}
		if len(resp.Windows) != 1 {
			t.Fatalf("%s windows = %+v, want one", sessionID, resp.Windows)
		}
		return resp.Windows[0].Geometry.X
	}

	if got := getX("desktop-session"); got != 360 {
		t.Fatalf("desktop session x = %d, want latest synced placement 360", got)
	}
	if got := getX("mobile-session"); got != 360 {
		t.Fatalf("mobile session x = %d, want latest synced placement 360", got)
	}
}
