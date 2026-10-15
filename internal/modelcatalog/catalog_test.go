package modelcatalog

import "testing"

// LongCat 2.5 Preview left the default pool on 2026-10-14: on the paper
// computer its calls took 21.8 s at the median, 115 s at p90 and up to 932 s,
// against 2.5 s and 10 s for gpt-6-luna, and one 38-minute management turn on
// it stopped the paper (docs/evidence/model-latency-paper-2026-10-14.md).
// gpt-6-luna stays last so a thread that fails over from it wraps to deepseek.
// cat-jam-free left the default pool on 2026-10-15 (owner): an OpenCode key
// is parked for every model after any 429, so its free-tier 429s parked the
// keys deepseek uses and pushed about 55% of paper calls to failover. The
// pool is the owner's dual pool, deepseek-v4.1-flash and gpt-6-luna,
// weighted 1:3 so gpt-6-luna takes about three threads in four.
func TestDefaultPoolMembers(t *testing.T) {
	pool := Pools()[DefaultPool]
	want := []PoolMember{
		{Provider: "opencode-go", Model: "deepseek-v4.1-flash", Weight: 1},
		{Provider: "chatgpt", Model: "gpt-6-luna", Weight: 3},
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
	if _, ok := Lookup("opencode-go", "cat-jam-free"); !ok {
		t.Fatal("cat-jam-free should stay in the catalog, out of the pool only")
	}
}
