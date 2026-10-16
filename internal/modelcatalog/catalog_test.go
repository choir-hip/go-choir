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
// pool was the owner's dual pool, deepseek-v4.1-flash and gpt-6-luna,
// weighted 1:3. Owner 2026-10-16: OpenCode's weekly limit is close, so every
// thread runs on gpt-6-luna; deepseek-v4.1-flash stays only as the failover,
// assigned no thread.
func TestDefaultPoolMembers(t *testing.T) {
	pool := Pools()[DefaultPool]
	want := []PoolMember{
		{Provider: "chatgpt", Model: "gpt-6-luna"},
		{Provider: "opencode-go", Model: "deepseek-v4.1-flash", FallbackOnly: true},
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

// A fallback-only member takes no share of threads; the others keep theirs.
func TestFallbackOnlyMemberHasNoSlotWeight(t *testing.T) {
	if w := (PoolMember{Weight: 3, FallbackOnly: true}).SlotWeight(); w != 0 {
		t.Fatalf("fallback-only slot weight = %d, want 0", w)
	}
	if w := (PoolMember{}).SlotWeight(); w != 1 {
		t.Fatalf("default slot weight = %d, want 1", w)
	}
}
