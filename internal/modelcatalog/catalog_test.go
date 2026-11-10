package modelcatalog

import (
	"strings"
	"testing"
)

// LongCat 2.5 Preview left the default pool on 2026-10-14: on the paper
// computer its calls took 21.8 s at the median, 115 s at p90 and up to 932 s,
// against 2.5 s and 10 s for gpt-6-luna, and one 38-minute management turn on
// it stopped the paper (docs/archive/evidence/model-latency-paper-2026-10-14.md).
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
// Owner 2026-10-28: every desk of a new computer runs on "a pool 50-50
// between ChatGPT and synthetic DeepSeek ... falling back to one or the
// other if one is unavailable": both members weighted 1, and the default
// pool fails over (it is not NoFailover), so a thread on a member that is
// out moves to the other.
func TestDefaultPoolMembers(t *testing.T) {
	pool := Pools()[DefaultPool]
	// Owner 2026-11-09: swe-2-flash (Devin) leads and takes every thread;
	// iceman, beam, flint, nemotron, luna and Haiku follow as fallback-only
	// members in their earlier order; no Synthetic member; inkling is in no
	// pool while OpenRouter gates it.
	want := []PoolMember{
		{Provider: "devin", Model: "swe-2-flash", Weight: 1},
		{Provider: "opencode-go", Model: "iceman-contributor-free", FallbackOnly: true},
		{Provider: "opencode-go", Model: "beam-free", FallbackOnly: true},
		{Provider: "openrouter", Model: "recursive/flint-v1.0:free", FallbackOnly: true},
		{Provider: "openrouter", Model: "nvidia/nemotron-3.5-super-vl:free", FallbackOnly: true},
		{Provider: "chatgpt", Model: "gpt-6-luna", FallbackOnly: true},
		// Owner 2026-11-10: the paid tail is spread by thread, Haiku about 80%,
		// Synthetic and River DeepSeek about 10% each.
		{Provider: "bedrock", Model: "us.anthropic.claude-haiku-5-5", FallbackOnly: true, Spread: "paid-tail", SpreadWeight: 8},
		{Provider: "synthetic", Model: "hf:deepseek-ai/DeepSeek-V4.1-Flash-1017", FallbackOnly: true, Spread: "paid-tail", SpreadWeight: 1},
		{Provider: "river", Model: "deepseek-ai/DeepSeek-V4.1-Flash-1017", FallbackOnly: true, Spread: "paid-tail", SpreadWeight: 1},
	}
	if len(pool) != len(want) {
		t.Fatalf("default pool = %v, want %v", pool, want)
	}
	for i := range want {
		if pool[i] != want[i] {
			t.Fatalf("default pool[%d] = %v, want %v", i, pool[i], want[i])
		}
	}
	if NoFailover(DefaultPool) {
		t.Fatal("default pool must fail over to its other member")
	}
	if _, ok := Lookup("opencode-go", "deepseek-v4.1-flash"); !ok {
		t.Fatal("deepseek-v4.1-flash should stay in the catalog, out of the pool only")
	}
	for name, members := range Pools() {
		for _, m := range members {
			// The OpenCode members are the free iceman and beam in the
			// default pool (owner 2026-11-05); OpenCode's paid models stay out.
			if (m.Provider == "opencode-go" || m.Provider == "opencode-zen") && (name != DefaultPool || (m.Model != "iceman-contributor-free" && m.Model != "beam-free")) {
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
	if d := Pools()[DefaultPool]; len(d) != 9 || d[5] != (PoolMember{Provider: "chatgpt", Model: "gpt-6-luna", FallbackOnly: true}) {
		t.Fatalf("default pool = %v", d)
	}
	if s := Pools()[SyntheticLunaPool]; len(s) != 2 || s[0].SlotWeight() != 1 || s[1].SlotWeight() != 1 || s[0].Provider != "synthetic" {
		t.Fatalf("synthetic-luna pool = %v", s)
	}
}

// thinkingmachines/inkling-2-small:free on OpenRouter (owner 2026-11-09), from
// the public models listing: a 1048576-token window, 131072 completion
// tokens, image input. Catalogued so a policy can name it, and in no pool
// while OpenRouter answers it 403; stealth/crouton-alpha, retired the same
// day, is gone from the catalog and every pool.
func TestOpenRouterInklingCataloguedAndCroutonGone(t *testing.T) {
	info, ok := Lookup("openrouter", "thinkingmachines/inkling-2-small:free")
	if !ok {
		t.Fatal("inkling not catalogued")
	}
	if info.ContextWindowTokens != 1048576 || info.MaxOutputTokens != 131072 {
		t.Fatalf("inkling = %+v", info)
	}
	if !SupportsImage(info.ID) {
		t.Fatal(ImageRefusal(info.ID))
	}
	if _, ok := Lookup("openrouter", "stealth/crouton-alpha"); ok {
		t.Fatal("retired crouton-alpha is still catalogued")
	}
	for name, members := range Pools() {
		for _, m := range members {
			if strings.Contains(m.Model, "crouton") || strings.Contains(m.Model, "inkling") {
				t.Fatalf("pool %s carries %v", name, m)
			}
			free := m.Model == "recursive/flint-v1.0:free" || m.Model == "nvidia/nemotron-3.5-super-vl:free"
			if m.Provider == "openrouter" && !(free && name == DefaultPool) {
				t.Fatalf("pool %s carries openrouter member %v", name, m)
			}
		}
	}
	if _, ok := Pools()["crouton-synthetic-luna"]; ok {
		t.Fatal("the crouton-synthetic-luna pool was deleted with its only primary")
	}
}

// The self-development pool (owner 2026-11-09: "use deepseek via Synthetic
// and River for self-dev"). Ways it could fail: Devin or a free member leaks
// in; Synthetic is not the only weighted member; Synthetic or River is missing, out of order or
// weighted; Haiku leaks in; a member is not in the catalog.
func TestSelfDevPool(t *testing.T) {
	if SelfDevPool != "selfdev" {
		t.Fatalf("SelfDevPool = %q", SelfDevPool)
	}
	want := []PoolMember{
		{Provider: "synthetic", Model: "hf:deepseek-ai/DeepSeek-V4.1-Flash-1017", Weight: 1},
		{Provider: "river", Model: "deepseek-ai/DeepSeek-V4.1-Flash-1017", FallbackOnly: true},
	}
	pool := Pools()[SelfDevPool]
	if len(pool) != len(want) {
		t.Fatalf("selfdev pool = %v", pool)
	}
	for i := range want {
		if pool[i] != want[i] {
			t.Fatalf("selfdev pool[%d] = %v, want %v", i, pool[i], want[i])
		}
		if _, ok := Lookup(want[i].Provider, want[i].Model); !ok {
			t.Fatalf("selfdev member %v missing from SupportedModels", want[i])
		}
		if p := pool[i].Provider; p == "bedrock" || p == "devin" || p == "opencode-go" || p == "openrouter" {
			t.Fatalf("selfdev pool holds %s", p)
		}
	}
	if NoFailover(SelfDevPool) {
		t.Fatal("selfdev pool must fail over")
	}
}

func TestHaiku55Catalogued(t *testing.T) {
	if _, ok := Lookup("bedrock", "us.anthropic.claude-haiku-5-5"); !ok {
		t.Fatal("Haiku 5.5 missing from SupportedModels")
	}
}

// swe-2-flash (Devin) leads the default pool but reads text only: an image
// turn skips it (toolloop needsImage, desk_shots, verifier multimodal) and
// lands on the first vision-capable member behind it, as the text-only free
// models are skipped today.
func TestDevinFlashIsTextOnlyAndImageTurnsSkipIt(t *testing.T) {
	if SupportsImage("swe-2-flash") || ImageRefusal("swe-2-flash") == "" {
		t.Fatal("swe-2-flash must be opted out of image input")
	}
	first := ""
	for _, m := range Pools()[DefaultPool] {
		if SupportsImage(m.Model) {
			first = m.Provider + "/" + m.Model
			break
		}
	}
	if first != "openrouter/nvidia/nemotron-3.5-super-vl:free" {
		t.Fatalf("first vision-capable default member = %q", first)
	}
	for _, id := range []string{"swe-2-flash", "swe-2"} {
		info, ok := Lookup("devin", id)
		if !ok || info.ContextWindowTokens < 262000 || info.MaxOutputTokens < 128000 {
			t.Fatalf("%s = %+v ok=%v", id, info, ok)
		}
	}
	if _, ok := Lookup("devin", "swe-2-fast"); ok {
		t.Fatal("swe-2-fast must not be catalogued")
	}
}
