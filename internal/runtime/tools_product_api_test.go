package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
)

func TestProductAPIRequestToolUsesRunOwnerForAllowedProductRoute(t *testing.T) {
	rt, _ := testAPISetup(t)
	tool := newProductAPIRequestTool(rt)
	run := &types.RunRecord{
		RunID:        "run-product-api",
		AgentID:      "agent-super-product-api",
		OwnerID:      "user-product-api",
		AgentProfile: agentprofile.Super,
		AgentRole:    agentprofile.Super,
	}

	raw, err := tool.Func(toolregistry.WithExecutionContext(context.Background(), toolExecutionContextForRun(run)), json.RawMessage(`{
		"method":"POST",
		"path":"/api/texture/documents",
		"body":{"title":"Product API tool owner proof"}
	}`))
	if err != nil {
		t.Fatalf("product_api_request: %v", err)
	}
	var resp struct {
		StatusCode int    `json:"status_code"`
		Path       string `json:"path"`
		Body       string `json:"body"`
		AllowedBy  string `json:"allowed_by"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("decode product_api_request response: %v\n%s", err, raw)
	}
	if resp.StatusCode != http.StatusCreated || resp.Path != "/api/texture/documents" {
		t.Fatalf("unexpected product API response: %+v", resp)
	}
	if resp.AllowedBy != "product_api_request_allowlist" {
		t.Fatalf("allowed_by = %q", resp.AllowedBy)
	}
	if !strings.Contains(resp.Body, `"owner_id":"user-product-api"`) {
		t.Fatalf("response body did not use run owner identity: %s", resp.Body)
	}
}

func TestProductAPIRequestToolRefusesInternalAndNonSuperCalls(t *testing.T) {
	rt, _ := testAPISetup(t)
	tool := newProductAPIRequestTool(rt)
	superRun := &types.RunRecord{
		RunID:        "run-product-api-refuse",
		AgentID:      "agent-super-product-api-refuse",
		OwnerID:      "user-product-api",
		AgentProfile: agentprofile.Super,
		AgentRole:    agentprofile.Super,
	}
	if _, err := tool.Func(toolregistry.WithExecutionContext(context.Background(), toolExecutionContextForRun(superRun)), json.RawMessage(`{
		"method":"GET",
		"path":"/internal/runtime/runs/run-1"
	}`)); err == nil || !strings.Contains(err.Error(), "refuses non-product route") {
		t.Fatalf("internal route error = %v, want refusal", err)
	}

	workerRun := &types.RunRecord{
		RunID:        "run-product-api-worker",
		AgentID:      "agent-worker-product-api",
		OwnerID:      "user-product-api",
		AgentProfile: agentprofile.CoSuper,
		AgentRole:    agentprofile.CoSuper,
	}
	if _, err := tool.Func(toolregistry.WithExecutionContext(context.Background(), toolExecutionContextForRun(workerRun)), json.RawMessage(`{
		"method":"GET",
		"path":"/api/universal-wire/stories"
	}`)); err == nil || !strings.Contains(err.Error(), "only available to foreground super") {
		t.Fatalf("non-super error = %v, want profile refusal", err)
	}
}
