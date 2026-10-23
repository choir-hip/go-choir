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
// thread ran on gpt-6-luna with deepseek-v4.1-flash only as the failover.
// Owner 2026-10-21: "We have Opencode quota now"; the dual pool is back at
// the owner's 1:3, so the same tasks run on both models and the errors they
// share can be fixed in the tool surface, not tuned to one model.
// Owner 2026-10-23 (UTC): "Just ChatGPT luna and synthetic DeepSeek 1017
// tonight. I'd rather them fail than fallback." and "Don't use Opencode as
// fallback tonight." The default pool is gpt-6-luna alone, so nothing falls
// back to OpenCode; the deepseek-v4.1-flash catalog entry stays.
func TestDefaultPoolMembers(t *testing.T) {
	pool := Pools()[DefaultPool]
	want := []PoolMember{
		{Provider: "chatgpt", Model: "gpt-6-luna", Weight: 1},
	}
	if len(pool) != len(want) {
		t.Fatalf("default pool = %v, want %v", pool, want)
	}
	for i := range want {
		if pool[i] != want[i] {
			t.Fatalf("default pool[%d] = %v, want %v", i, pool[i], want[i])
		}
	}
	if _, ok := Lookup("opencode-go", "deepseek-v4.1-flash"); !ok {
		t.Fatal("deepseek-v4.1-flash should stay in the catalog, out of the pool only")
	}
	for name, members := range Pools() {
		for _, m := range members {
			if m.Provider == "opencode-go" || m.Provider == "opencode-zen" {
				t.Fatalf("pool %s carries OpenCode member %v", name, m)
			}
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

// synthetic-luna is Synthetic's DeepSeek V4.1 Flash 1017 and gpt-6-luna at
// 1:1, the paper's overnight split (owner 2026-10-22), and it is the one pool
// that never fails over. The catalog says the window Synthetic serves.
func TestSyntheticLunaPool(t *testing.T) {
	const id = "hf:deepseek-ai/DeepSeek-V4.1-Flash-1017"
	want := []PoolMember{
		{Provider: "synthetic", Model: id, Weight: 1},
		{Provider: "chatgpt", Model: "gpt-6-luna", Weight: 1},
	}
	pool := Pools()[SyntheticLunaPool]
	if len(pool) != len(want) || pool[0] != want[0] || pool[1] != want[1] {
		t.Fatalf("synthetic-luna = %v, want %v", pool, want)
	}
	for _, m := range pool {
		if _, ok := Lookup(m.Provider, m.Model); !ok {
			t.Fatalf("pool member %v is not in the catalog", m)
		}
	}
	if !NoFailover(SyntheticLunaPool) || NoFailover(DefaultPool) {
		t.Fatal("only synthetic-luna is a no-failover pool")
	}
	info, _ := Lookup("synthetic", id)
	if info.ContextWindowTokens != 524288 || info.MaxOutputTokens != 65536 {
		t.Fatalf("synthetic window = %d / %d, want 524288 / 65536 (probed 2026-10-22)", info.ContextWindowTokens, info.MaxOutputTokens)
	}
	if !SupportsImage(id) {
		t.Fatal("the 1017 model took an image in the probe; it must not be opted out")
	}
}

// luna-synthetic is the owner's 2026-10-23 pool: "synthetic is using much
// faster... we should go to chatgpt luna 100% with synthetic only as fallback
// for now". Ways it could fail: Synthetic takes a share of threads; luna is
// not first; the pool is NoFailover (so luna's failure fails the thread
// instead of moving it); it changes the default or synthetic-luna pool.
func TestLunaSyntheticPool(t *testing.T) {
	const id = "hf:deepseek-ai/DeepSeek-V4.1-Flash-1017"
	pool := Pools()[LunaSyntheticPool]
	want := []PoolMember{
		{Provider: "chatgpt", Model: "gpt-6-luna", Weight: 1},
		{Provider: "synthetic", Model: id, FallbackOnly: true},
	}
	if len(pool) != len(want) || pool[0] != want[0] || pool[1] != want[1] {
		t.Fatalf("luna-synthetic = %v, want %v", pool, want)
	}
	if pool[0].SlotWeight() != 1 || pool[1].SlotWeight() != 0 {
		t.Fatalf("slot weights = %d, %d, want 1, 0", pool[0].SlotWeight(), pool[1].SlotWeight())
	}
	for _, m := range pool {
		if _, ok := Lookup(m.Provider, m.Model); !ok {
			t.Fatalf("pool member %v is not in the catalog", m)
		}
	}
	if NoFailover(LunaSyntheticPool) {
		t.Fatal("luna-synthetic must fail over (Synthetic is its fallback)")
	}
	if !NoFailover(SyntheticLunaPool) || NoFailover(DefaultPool) {
		t.Fatal("only synthetic-luna is a no-failover pool")
	}
	// The other pools are unchanged.
	if d := Pools()[DefaultPool]; len(d) != 1 || d[0] != (PoolMember{Provider: "chatgpt", Model: "gpt-6-luna", Weight: 1}) {
		t.Fatalf("default pool = %v", d)
	}
	if s := Pools()[SyntheticLunaPool]; len(s) != 2 || s[0].SlotWeight() != 1 || s[1].SlotWeight() != 1 || s[0].Provider != "synthetic" {
		t.Fatalf("synthetic-luna pool = %v", s)
	}
}
