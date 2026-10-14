package modelcatalog

import "testing"

// LongCat 2.5 Preview left the default pool on 2026-10-14: on the paper
// computer its calls took 21.8 s at the median, 115 s at p90 and up to 932 s,
// against 2.5 s and 10 s for gpt-6-luna, and one 38-minute management turn on
// it stopped the paper (docs/evidence/model-latency-paper-2026-10-14.md).
// gpt-6-luna stays last so a thread that fails over from it wraps to deepseek.
func TestDefaultPoolMembers(t *testing.T) {
	pool := Pools()[DefaultPool]
	want := []PoolMember{
		{Provider: "opencode-go", Model: "deepseek-v4.1-flash"},
		{Provider: "opencode-go", Model: "cat-jam-free"},
		{Provider: "chatgpt", Model: "gpt-6-luna"},
	}
	if len(pool) != len(want) {
		t.Fatalf("default pool = %v, want %v", pool, want)
	}
	for i := range want {
		if pool[i] != want[i] {
			t.Fatalf("default pool[%d] = %v, want %v", i, pool[i], want[i])
		}
	}
	if _, ok := Lookup("opencode-go", "longcat-2.5-preview-free"); !ok {
		t.Fatal("longcat-2.5-preview-free should stay in the catalog, out of the pool only")
	}
}
