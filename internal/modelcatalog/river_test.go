package modelcatalog

import "testing"

// The river pool (owner 2026-11-09). Ways it could fail: River DeepSeek is
// not the only weighted member; the fallbacks are not fallback-only or are
// out of order; the model is missing from SupportedModels; DefaultPool moves.
func TestRiverPool(t *testing.T) {
	const id = "deepseek-ai/DeepSeek-V4.1-Flash-1017"
	pool := Pools()[RiverPool]
	want := []PoolMember{
		{Provider: "river", Model: id, Weight: 1},
		{Provider: "openrouter", Model: "stealth/crouton-alpha", FallbackOnly: true},
		{Provider: "chatgpt", Model: "gpt-6-luna", FallbackOnly: true},
	}
	if len(pool) != len(want) {
		t.Fatalf("river pool = %v", pool)
	}
	for i := range want {
		if pool[i] != want[i] {
			t.Fatalf("river pool[%d] = %v, want %v", i, pool[i], want[i])
		}
	}
	if _, ok := Lookup("river", id); !ok {
		t.Fatal("river model missing from SupportedModels")
	}
	if NoFailover(RiverPool) {
		t.Fatal("river pool must fail over")
	}
	if d := Pools()[DefaultPool]; d[0].Provider != "openrouter" {
		t.Fatalf("default pool changed: %v", d[0])
	}
}
