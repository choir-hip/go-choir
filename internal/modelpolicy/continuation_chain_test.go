package modelpolicy

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/modelcatalog"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
)

// The continuation chain (quota means switch, never pause). Failure modes:
// the chain names a model the computer's policy and the default pool do not
// allow (it widens who may call what); it names a model the catalog does not
// serve (a retired id, the Claude delegate); it repeats a model; a pinned
// role's model is missing from it; an unreadable policy leaves the chain
// empty so the pinned role has nowhere to go.

func chainIDs(chain []provideriface.LLMSelection) []string {
	var out []string
	for _, s := range chain {
		out = append(out, s.Provider+"/"+s.Model)
	}
	return out
}

func TestContinuationChainIsTheComputersPolicyThenTheDefaultPool(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model-policy.toml")
	if err := os.WriteFile(path, []byte(`# owner-edited
[defaults]
model = "glm-5.2"
provider = "zai"

[roles.engineering]
provider = "chatgpt"
model = "gpt-6-luna"

[roles.research]
provider = "claude-delegate"
model = "claude-opus"

[roles.verifier]
provider = "chatgpt"
model = "retired-model-9"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	m := NewManager(ManagerConfig{PolicyPath: path})
	ids := chainIDs(m.ContinuationChain(context.Background(), "owner"))
	want := []string{"zai/glm-5.2", "chatgpt/gpt-6-luna", "openrouter/stealth/crouton-alpha", "opencode-go/iceman-contributor-free", "opencode-go/beam-free", "openrouter/recursive/flint-v1.0:free", "openrouter/nvidia/nemotron-3.5-super-vl:free"}
	if strings.Join(ids, ",") != strings.Join(want, ",") {
		t.Fatalf("chain = %v, want %v: the policy's catalogued models, then the default pool, no repeats, no uncatalogued model", ids, want)
	}
}

func TestContinuationChainOfADefaultComputerIsTheDefaultPool(t *testing.T) {
	m := NewManager(ManagerConfig{PolicyPath: filepath.Join(t.TempDir(), "model-policy.toml")})
	var want []string
	for _, member := range modelcatalog.Pools()[modelcatalog.DefaultPool] {
		want = append(want, member.Provider+"/"+member.Model)
	}
	ids := chainIDs(m.ContinuationChain(context.Background(), "owner"))
	if len(ids) != len(want) {
		t.Fatalf("chain = %v, want the default pool %v", ids, want)
	}
	for _, id := range ids {
		// OpenRouter's crouton-alpha joined the default pool (owner
		// 2026-11-05); the Claude delegate is never the computer's to use.
		if strings.Contains(id, "claude") || strings.Contains(id, "synthetic") {
			t.Fatalf("chain %v holds a model that is not the computer's to use", ids)
		}
	}
}

func TestContinuationChainSurvivesAnUnreadablePolicy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model-policy.toml")
	if err := os.WriteFile(path, []byte("# owner-edited\n[roles.engineering\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if ids := chainIDs(NewManager(ManagerConfig{PolicyPath: path}).ContinuationChain(context.Background(), "o")); len(ids) == 0 {
		t.Fatal("an unparseable policy left the chain empty")
	}
}

// The chain is read on the quota path and must never write the policy file.
func TestContinuationChainDoesNotWriteThePolicyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model-policy.toml")
	NewManager(ManagerConfig{PolicyPath: path}).ContinuationChain(context.Background(), "o")
	if _, err := os.Stat(path); err == nil {
		t.Fatal("the continuation chain wrote the policy file")
	}
}
