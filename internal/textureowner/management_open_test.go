package textureowner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// TestHandleManagementOpenMintsRealControlAndWake proves the deterministic
// owner-side mint surface: a POST issues a real execution_request control
// through the canonical IssueLifecycleControl reducer (same validators as a
// texture-desk ApplyTexture) and wakes the persistent management agent — no
// texture-desk agency required.
func TestHandleManagementOpenMintsRealControlAndWake(t *testing.T) {
	core, handler := testAPISetup(t)
	_ = core // runtime needed for lifecycle wiring; store assertions go through handler.Store
	ctx := context.Background()
	ownerID, computerID := "user-alice", "autoputer-test"
	mgmtAgentID := agentprofile.Management + ":" + ownerID
	now := time.Now().UTC()
	if err := handler.Store.UpsertAgent(ctx, types.AgentRecord{
		AgentID: mgmtAgentID, OwnerID: ownerID, ComputerID: computerID,
		Profile: agentprofile.Management, Role: agentprofile.Management,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("seed management agent: %v", err)
	}

	body := `{"objective":"run the three SMG verbs","actions":[{"type":"run_command","objective":"exercise cancel","safety":{"mutation_class":"green","network":"forbidden","file_mutation":"forbidden"}},{"type":"run_command","objective":"exercise bound report","safety":{"mutation_class":"green","network":"forbidden","file_mutation":"forbidden"}}]}`
	w := runtimeHandlerRequest(t, handler.HandleManagementOpen, http.MethodPost, "/api/texture/management-open", body, ownerID)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusAccepted, w.Body.String())
	}
	var resp managementOpenResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.WorkItemID == "" || resp.UpdateID == "" || resp.TextureRunID == "" || resp.TrajectoryID == "" {
		t.Fatalf("response missing durable identities: %+v", resp)
	}

	// The caller run must be a real texture activation bound to the minted
	// document — the IssueLifecycleControl validator rejects anything else.
	caller, err := handler.Store.GetLifecycleRun(ctx, ownerID, computerID, resp.TextureRunID)
	if err != nil {
		t.Fatalf("load texture caller run: %v", err)
	}
	if caller.AgentProfile != agentprofile.Texture || caller.AgentID != agentprofile.Texture+":"+resp.DocID {
		t.Fatalf("caller = %+v, want active texture run for doc %s", caller, resp.DocID)
	}

	// The control packet must be durable and bound to the management work item.
	work, err := handler.Store.GetLifecycleWorkItem(ctx, ownerID, computerID, resp.WorkItemID)
	if err != nil {
		t.Fatalf("load minted work item: %v", err)
	}
	if work.AssignedAgentID != mgmtAgentID || work.AuthorityProfile != agentprofile.Management {
		t.Fatalf("work item = %+v, want assigned to %s with management authority", work, mgmtAgentID)
	}
}

// TestHandleManagementOpenRegistersAgentOnFirstUse proves the endpoint is
// self-bootstrapping: a computer with no prior persistent-management agent
// record (fresh disposable, first open) gets one minted rather than failing
// the IssueLifecycleControl validator. Regression for the
// "persistent management agent not registered" 500 observed on the first
// SMG disposable probe.
func TestHandleManagementOpenRegistersAgentOnFirstUse(t *testing.T) {
	_, handler := testAPISetup(t)
	ctx := context.Background()
	ownerID, computerID := "user-alice", "autoputer-test"
	mgmtAgentID := agentprofile.Management + ":" + ownerID

	body := `{"objective":"first open registers management","actions":[{"type":"inspect_file","objective":"noop","safety":{"mutation_class":"green","network":"forbidden","file_mutation":"forbidden"}}]}`
	w := runtimeHandlerRequest(t, handler.HandleManagementOpen, http.MethodPost, "/api/texture/management-open", body, ownerID)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusAccepted, w.Body.String())
	}
	agent, err := handler.Store.GetAgentByScope(ctx, ownerID, computerID, mgmtAgentID)
	if err != nil {
		t.Fatalf("management agent must be registered by the endpoint: %v", err)
	}
	if agent.Profile != agentprofile.Management || agent.Role != agentprofile.Management {
		t.Fatalf("agent = %+v, want persistent-management shape", agent)
	}
}

// TestHandleManagementOpenRejectsNoActions proves the persistent-management
// control shape requirement is enforced: execution_request actions are
// mandatory (same validator the desk path uses).
func TestHandleManagementOpenRejectsNoActions(t *testing.T) {
	_, handler := testAPISetup(t)
	w := runtimeHandlerRequest(t, handler.HandleManagementOpen, http.MethodPost, "/api/texture/management-open", `{"objective":"noop"}`, "user-alice")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "action") {
		t.Fatalf("error should name the missing actions: %s", w.Body.String())
	}
}

// TestHandleManagementOpenRequiresAuth proves the endpoint is owner-scoped.
func TestHandleManagementOpenRequiresAuth(t *testing.T) {
	_, handler := testAPISetup(t)
	req := httptest.NewRequest(http.MethodPost, "/api/texture/management-open", strings.NewReader(`{"objective":"x","actions":[{"type":"run_command","objective":"y"}]}`))
	w := httptest.NewRecorder()
	handler.HandleManagementOpen(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}
