package modelpolicy

// Guards (Devin review of the spread commit). Ways this could fail: spread on
// a weighted member lets the spread hash put a fallback-only role in the lead
// slot; a same-name group split by another member is silently inert; a plain
// list shadowing the default pool silently drops the paid-tail spread; the
// renderer of `choir policy pin` drops a field.

import (
	"context"
	"strings"
	"testing"
)

func TestSpreadRequiresFallbackOnly(t *testing.T) {
	text := "[pools.mine]\nmembers = [{ model = \"" + memDevin + "\", spread = \"g\" }, { model = \"" + memHaiku + "\", fallback_only = true, spread = \"g\" }]\n\n" + rolesAndDefaults()
	_, err := parsePolicy(text, "test")
	if err == nil || !strings.Contains(err.Error(), "fallback_only") {
		t.Fatalf("weighted member with spread accepted or unclear: %v", err)
	}
}

func TestSpreadGroupMustBeAdjacent(t *testing.T) {
	text := "[pools.mine]\nmembers = [{ model = \"" + memDevin + "\" }, { model = \"" + memHaiku + "\", fallback_only = true, spread = \"g\" }, { model = \"" + memLuna + "\", fallback_only = true }, { model = \"" + memRiver + "\", fallback_only = true, spread = \"g\" }]\n\n" + rolesAndDefaults()
	_, err := parsePolicy(text, "test")
	if err == nil || !strings.Contains(err.Error(), "adjacent") || !strings.Contains(err.Error(), `"g"`) {
		t.Fatalf("split spread group accepted or unclear: %v", err)
	}
}

// An existing file with a plain-list default pool (written before the release
// carried a spread group) still loads: its roles survive, the pool runs in
// strict order, a diagnostic says so, and every write path refuses it.
func TestPlainListOverSpreadPoolReadsTolerantlyAndWritesRefuse(t *testing.T) {
	text := "[pools.default]\nmembers = [\"" + memDevin + "\", \"" + memHaiku + "\"]\n\n[defaults]\nmodel = \"pool:default\"\nreasoning = \"high\"\n\n[roles.texture]\nprovider = \"chatgpt\"\nmodel = \"gpt-6-luna\"\n"
	policy, err := parsePolicy(text, "test")
	if err != nil {
		t.Fatalf("existing file rejected on read: %v", err)
	}
	if got := policy.Resolve("texture"); got.Model != "gpt-6-luna" {
		t.Fatalf("role assignment lost: %+v", got)
	}
	if got := policy.Resolve("management"); got.ReasoningEffort != "high" {
		t.Fatalf("defaults lost: %+v", got)
	}
	pool, ok := policy.pool("default")
	if !ok || len(pool.Members) != 2 || pool.Source != PoolSourceFile {
		t.Fatalf("pool = %+v", pool)
	}
	if len(policy.Diagnostics) != 1 || !strings.Contains(policy.Diagnostics[0], "spread") {
		t.Fatalf("diagnostics = %v", policy.Diagnostics)
	}
	if err := ValidateText(text); err == nil || !strings.Contains(err.Error(), "spread") {
		t.Fatalf("write path accepted the plain list: %v", err)
	}
	// A manager booting on that file uses it, not the platform fallback.
	m := writePolicyFile(t, text)
	loaded, lerr := m.Load(context.Background(), "owner")
	if lerr != nil || loaded.Resolve("texture").Model != "gpt-6-luna" {
		t.Fatalf("manager load: %v %+v", lerr, loaded.Resolve("texture"))
	}
}
