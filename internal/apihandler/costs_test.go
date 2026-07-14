//go:build comprehensive

package apihandler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/llmcost"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestHandleCostsRequiresAuth(t *testing.T) {
	t.Parallel()
	_, handler := testCostsSetup(t)

	req := httptest.NewRequest(http.MethodGet, "/api/costs", nil)
	w := httptest.NewRecorder()
	handler.HandleCosts(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandleCostsRejectsPost(t *testing.T) {
	t.Parallel()
	_, handler := testCostsSetup(t)

	req := authenticatedCostsRequest(http.MethodPost, "/api/costs", "user-alice")
	w := httptest.NewRecorder()
	handler.HandleCosts(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleCostsReturnsEstimate(t *testing.T) {
	t.Parallel()
	s, handler := testCostsSetup(t)

	seedCostEvents(t, s, "user-alice")

	req := authenticatedCostsRequest(http.MethodGet, "/api/costs", "user-alice")
	w := httptest.NewRecorder()
	handler.HandleCosts(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp costsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.Estimate {
		t.Fatal("response should be marked as estimate")
	}
	if resp.Summary.CallCount != 2 {
		t.Fatalf("CallCount: got %d, want 2", resp.Summary.CallCount)
	}
	if resp.Summary.TotalInputTokens != 3000 {
		t.Fatalf("TotalInputTokens: got %d, want 3000", resp.Summary.TotalInputTokens)
	}
	if resp.Summary.TotalOutputTokens != 1500 {
		t.Fatalf("TotalOutputTokens: got %d, want 1500", resp.Summary.TotalOutputTokens)
	}
	// gpt-4o: $5/1M input, $15/1M output.
	// 1000/500 = 0.0125, 2000/1000 = 0.025 -> total 0.0375
	if resp.Summary.TotalCost < 0.0374 || resp.Summary.TotalCost > 0.0376 {
		t.Fatalf("TotalCost: got %.6f, want ~0.0375", resp.Summary.TotalCost)
	}
	openai, ok := resp.Summary.ByProvider["openai"]
	if !ok {
		t.Fatal("ByProvider[openai] missing")
	}
	if openai.CallCount != 2 {
		t.Fatalf("ByProvider[openai] CallCount: got %d, want 2", openai.CallCount)
	}
}

func TestHandleCostsDetailIncludesEntries(t *testing.T) {
	t.Parallel()
	s, handler := testCostsSetup(t)

	seedCostEvents(t, s, "user-alice")

	req := authenticatedCostsRequest(http.MethodGet, "/api/costs?detail=1", "user-alice")
	w := httptest.NewRecorder()
	handler.HandleCosts(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusOK)
	}

	var resp costsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Entries) != 2 {
		t.Fatalf("Entries: got %d, want 2", len(resp.Entries))
	}
}

func TestHandleCostsModelsFlagIncludesPricing(t *testing.T) {
	t.Parallel()
	_, handler := testCostsSetup(t)

	req := authenticatedCostsRequest(http.MethodGet, "/api/costs?models=1", "user-alice")
	w := httptest.NewRecorder()
	handler.HandleCosts(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusOK)
	}

	var resp costsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.KnownModels) == 0 {
		t.Fatal("KnownModels should not be empty when models=1")
	}
	found := false
	for _, m := range resp.KnownModels {
		if m.Model == "gpt-4o" {
			found = true
		}
	}
	if !found {
		t.Fatal("KnownModels should include gpt-4o")
	}
}

func TestHandleCostsOwnerScoped(t *testing.T) {
	t.Parallel()
	s, handler := testCostsSetup(t)

	// Seed events for alice.
	seedCostEvents(t, s, "user-alice")

	// Bob should see zero calls.
	req := authenticatedCostsRequest(http.MethodGet, "/api/costs", "user-bob")
	w := httptest.NewRecorder()
	handler.HandleCosts(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusOK)
	}

	var resp costsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Summary.CallCount != 0 {
		t.Fatalf("Bob CallCount: got %d, want 0 (owner-scoped)", resp.Summary.CallCount)
	}
}

func TestHandleCostsTimeWindowFilter(t *testing.T) {
	t.Parallel()
	s, handler := testCostsSetup(t)

	seedCostEvents(t, s, "user-alice")

	// Use a from bound in the future to exclude all events.
	future := time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339)
	req := authenticatedCostsRequest(http.MethodGet, "/api/costs?from="+future, "user-alice")
	w := httptest.NewRecorder()
	handler.HandleCosts(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusOK)
	}

	var resp costsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Summary.CallCount != 0 {
		t.Fatalf("future-window CallCount: got %d, want 0", resp.Summary.CallCount)
	}
}

func testCostsSetup(t *testing.T) (*store.Store, *Handler) {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "costs.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	})
	return s, NewHandler(s)
}

func authenticatedCostsRequest(method, path, user string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	if user != "" {
		req.Header.Set("X-Authenticated-User", user)
	}
	return req
}

// seedCostEvents appends two tool_loop progress events with token usage for
// the given owner so the costs API has data to aggregate.
func seedCostEvents(t *testing.T, s *store.Store, ownerID string) {
	t.Helper()
	ctx := context.Background()

	payload1, _ := json.Marshal(map[string]any{
		"phase":         "tool_loop",
		"iteration":     1,
		"stop_reason":   "end_turn",
		"model":         "gpt-4o",
		"llm_provider":  "openai",
		"llm_model":     "gpt-4o",
		"input_tokens":  1000,
		"output_tokens": 500,
	})
	payload2, _ := json.Marshal(map[string]any{
		"phase":         "tool_loop",
		"iteration":     2,
		"stop_reason":   "end_turn",
		"model":         "gpt-4o",
		"llm_provider":  "openai",
		"llm_model":     "gpt-4o",
		"input_tokens":  2000,
		"output_tokens": 1000,
	})
	events := []types.EventRecord{
		{
			EventID:      "cost-ev-1",
			RunID:        "cost-run-1",
			OwnerID:      ownerID,
			TrajectoryID: "cost-traj-1",
			AgentID:      "cost-agent-1",
			Kind:         types.EventRunProgress,
			Phase:        "tool_loop",
			Payload:      payload1,
			Timestamp:    time.Now().UTC(),
		},
		{
			EventID:      "cost-ev-2",
			RunID:        "cost-run-1",
			OwnerID:      ownerID,
			TrajectoryID: "cost-traj-1",
			AgentID:      "cost-agent-1",
			Kind:         types.EventRunProgress,
			Phase:        "tool_loop",
			Payload:      payload2,
			Timestamp:    time.Now().UTC(),
		},
	}
	for _, ev := range events {
		if err := s.AppendEvent(ctx, &ev); err != nil {
			t.Fatalf("append cost event: %v", err)
		}
	}
}

// Verify the llmcost package integrates with the canonical API handler import graph.
func TestLLMCostPackageIntegration(t *testing.T) {
	t.Parallel()
	cost := llmcost.EstimateCall("gpt-4o", 1000, 500)
	if !cost.Found {
		t.Fatal("gpt-4o estimate should be found")
	}
	if cost.Provider != "openai" {
		t.Fatalf("provider: got %q, want openai", cost.Provider)
	}
}
