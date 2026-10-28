package agentcore

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/modelpolicy"
)

// Owner 2026-10-23: "we should go to chatgpt luna 100% with synthetic only as
// fallback for now". GET /api/model-policy/resolve is how the owner and
// the proof scripts read what a role will run on. Ways it can fail: it names
// Synthetic as the model; it hides the fallback, so a proof cannot tell
// luna-synthetic from a pool with no failover; it reports a fallback for
// the pinned management task or for a pool that has none.
func TestModelPolicyResolveReportsLunaWithSyntheticFallback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "System", "model-policy.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	policy := "# owner-edited\n[defaults]\nmodel = \"pool:luna-synthetic\"\nreasoning_effort = \"low\"\n\n[roles.texture]\nmodel = \"pool:luna-synthetic\"\nreasoning_effort = \"high\"\n"
	if err := os.WriteFile(path, []byte(policy), 0o644); err != nil {
		t.Fatal(err)
	}
	h := &APIHandler{rt: &Runtime{modelPolicy: modelpolicy.NewManager(modelpolicy.ManagerConfig{PolicyPath: path})}}
	resolve := func(role string) modelPolicyResolveResponse {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/model-policy/resolve?role="+role, nil)
		req.Header.Set("X-Authenticated-User", "owner-resolve")
		rec := httptest.NewRecorder()
		h.HandleModelPolicyRouter(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("resolve %s = %d %s", role, rec.Code, rec.Body)
		}
		var out modelPolicyResolveResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	for _, role := range []string{"texture", "management", "research"} {
		got := resolve(role)
		if got.Provider != "chatgpt" || got.Model != "gpt-6-luna" || got.PolicyError != "" ||
			got.Fallbacks != "synthetic/hf:deepseek-ai/DeepSeek-V4.1-Flash-1017" {
			t.Fatalf("%s resolves to %+v", role, got)
		}
	}
	if got := resolve("texture"); got.ReasoningEffort != "high" {
		t.Fatalf("texture effort = %q", got.ReasoningEffort)
	}
	// The management task is pinned, not pooled; it falls back to the
	// default pool's other member (owner 2026-10-28).
	if got := resolve("management_task"); got.Model != "gpt-6-luna" || got.Fallbacks != "synthetic/hf:deepseek-ai/DeepSeek-V4.1-Flash-1017" {
		t.Fatalf("management_task resolves to %+v", got)
	}
	// A pool with no failover reports none.
	if err := os.WriteFile(path, []byte("# owner-edited\n[defaults]\nmodel = \"pool:synthetic-luna\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := resolve("texture"); got.Fallbacks != "" {
		t.Fatalf("synthetic-luna reports fallbacks %q", got.Fallbacks)
	}
}
