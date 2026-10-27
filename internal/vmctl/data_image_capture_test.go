package vmctl

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// The host capture route (S2 of
// docs/problems/paper-never-checkpointed-2026-10-27.md). Ways it can fail,
// written before the code: a guest or any non-internal caller pauses a
// computer; an unknown or inactive computer reaches the manager; a manager
// without the capability is called; a capture error reads as success; the
// operator cannot tell which VM was paused or for how long.

type captureVMManager struct {
	*mockVMManager
	calls []string
	err   error
}

func (m *captureVMManager) CaptureDataImage(vmID string) (DataImageCapture, error) {
	m.calls = append(m.calls, vmID)
	if m.err != nil {
		return DataImageCapture{}, m.err
	}
	return DataImageCapture{Path: "/state/" + vmID + "/data.img.capture", PausedFor: 1500 * time.Millisecond, ClonedFor: 1400 * time.Millisecond}, nil
}

func captureFixture(t *testing.T, mgr VMManager) (*Handler, VMOwnership) {
	t.Helper()
	reg := NewOwnershipRegistry("")
	reg.SetCorpusdURL(testComputerCredentialIssuerURL(t))
	reg.SetVMManager(mgr)
	own, err := reg.ResolveOrAssign("owner-capture")
	if err != nil {
		t.Fatal(err)
	}
	return NewHandler(reg), *own
}

func postCapture(t *testing.T, h *Handler, body, remote string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/internal/vmctl/capture-data-image", bytes.NewBufferString(body))
	req.Header.Set("X-Internal-Caller", "true")
	if remote != "" {
		req.RemoteAddr = remote
	}
	rec := httptest.NewRecorder()
	h.HandleCaptureDataImage(rec, req)
	var resp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	return rec.Code, resp
}

func TestCaptureDataImageRouteCapturesTheNamedComputer(t *testing.T) {
	mgr := &captureVMManager{mockVMManager: &mockVMManager{}}
	h, own := captureFixture(t, mgr)
	code, resp := postCapture(t, h, `{"computer_id":"`+own.ComputerID+`"}`, "")
	if code != http.StatusOK {
		t.Fatalf("capture: %d %v", code, resp)
	}
	if len(mgr.calls) != 1 || mgr.calls[0] != own.VMID {
		t.Fatalf("manager calls = %v, want [%s]", mgr.calls, own.VMID)
	}
	if resp["vm_id"] != own.VMID || resp["computer_id"] != own.ComputerID || resp["user_id"] != own.UserID {
		t.Fatalf("response identity = %v", resp)
	}
	if resp["capture_path"] != "/state/"+own.VMID+"/data.img.capture" || resp["paused_ms"] != float64(1500) {
		t.Fatalf("response capture = %v", resp)
	}
}

func TestCaptureDataImageRouteRefusals(t *testing.T) {
	t.Run("not internal", func(t *testing.T) {
		mgr := &captureVMManager{mockVMManager: &mockVMManager{}}
		h, own := captureFixture(t, mgr)
		if code, _ := postCapture(t, h, `{"computer_id":"`+own.ComputerID+`"}`, "10.0.0.5:4000"); code != http.StatusForbidden {
			t.Fatalf("guest caller: %d, want 403", code)
		}
		if len(mgr.calls) != 0 {
			t.Fatalf("manager called for a guest: %v", mgr.calls)
		}
	})
	t.Run("unknown computer", func(t *testing.T) {
		mgr := &captureVMManager{mockVMManager: &mockVMManager{}}
		h, _ := captureFixture(t, mgr)
		if code, _ := postCapture(t, h, `{"computer_id":"computer-nobody"}`, ""); code != http.StatusNotFound {
			t.Fatalf("unknown computer: %d, want 404", code)
		}
		if code, _ := postCapture(t, h, `{}`, ""); code != http.StatusBadRequest {
			t.Fatalf("no computer: %d, want 400", code)
		}
		if len(mgr.calls) != 0 {
			t.Fatalf("manager called: %v", mgr.calls)
		}
	})
	t.Run("unsupported manager", func(t *testing.T) {
		h, own := captureFixture(t, &mockVMManager{})
		if code, _ := postCapture(t, h, `{"computer_id":"`+own.ComputerID+`"}`, ""); code != http.StatusNotImplemented {
			t.Fatalf("no capability: %d, want 501", code)
		}
	})
	t.Run("capture fails", func(t *testing.T) {
		mgr := &captureVMManager{mockVMManager: &mockVMManager{}, err: errors.New("clone refused")}
		h, own := captureFixture(t, mgr)
		code, resp := postCapture(t, h, `{"computer_id":"`+own.ComputerID+`"}`, "")
		if code != http.StatusInternalServerError || resp["error"] == nil {
			t.Fatalf("failed capture: %d %v, want 500 with the error", code, resp)
		}
	})
	// Review 364 lows: contention is told apart from failure, and the
	// lookup of one computer does not depend on classifying every other.
	t.Run("VM busy", func(t *testing.T) {
		mgr := &captureVMManager{mockVMManager: &mockVMManager{}, err: errors.New("capture data image vm-x: VM operation in progress")}
		h, own := captureFixture(t, mgr)
		if code, _ := postCapture(t, h, `{"computer_id":"`+own.ComputerID+`"}`, ""); code != http.StatusConflict {
			t.Fatalf("busy VM: %d, want 409", code)
		}
	})
	t.Run("route authority unavailable", func(t *testing.T) {
		mgr := &captureVMManager{mockVMManager: &mockVMManager{}}
		h, own := captureFixture(t, mgr)
		h.routeAuthorityRequired = true
		if code, resp := postCapture(t, h, `{"computer_id":"`+own.ComputerID+`"}`, ""); code != http.StatusOK {
			t.Fatalf("capture with route authority down: %d %v, want 200", code, resp)
		}
	})
}
