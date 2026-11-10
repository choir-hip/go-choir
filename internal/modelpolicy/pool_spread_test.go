package modelpolicy

// The default pool's paid tail is a weighted spread group (owner 2026-11-10:
// "use river and synthetic just a bit in autopaper. Like 10% each" and "I
// don't want DeepSeek before haiku but spread the load during Devin's
// downtime"): Haiku 8, Synthetic DeepSeek 1, River DeepSeek 1, by thread hash.
//
// Ways this could fail, written down before the code:
//   - the overflow still goes only to Haiku (no spread), or DeepSeek goes
//     ahead of Haiku for every thread (Haiku is not about 80%);
//   - a thread flips its first tail member between turns (prompt cache);
//   - spread applies to a pool whose members did not opt in;
//   - a spread thread loses the other tail members as its fallbacks, or the
//     free members and luna move out of their place ahead of the tail;
//   - an image turn lands on a text-only member (the vision chain still has
//     nemotron ahead of the tail and Haiku in it);
//   - the self-dev pool or another compiled pool changes;
//   - the file format drops or accepts a bad spread or spread_weight key, or
//     the pool view shows a different group from Assign.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/modelcatalog"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
)

const (
	memSynthetic = "synthetic/hf:deepseek-ai/DeepSeek-V4.1-Flash-1017"
	memRiver     = "river/deepseek-ai/DeepSeek-V4.1-Flash-1017"
)

func chain(first provideriface.LLMSelection, rest []provideriface.LLMSelection) []string {
	out := []string{first.Provider + "/" + first.Model}
	for _, f := range rest {
		out = append(out, f.Provider+"/"+f.Model)
	}
	return out
}

func TestDefaultPoolTailSpreadsHaikuSyntheticRiver(t *testing.T) {
	spec := provideriface.LLMSelection{Model: "pool:default"}
	const n = 4000
	firstOfTail := map[string]int{}
	for i := 0; i < n; i++ {
		first, rest, _ := Assign(spec, fmt.Sprintf("trajectory-%d", i))
		c := chain(first, rest)
		if len(c) != 9 {
			t.Fatalf("chain = %v", c)
		}
		// Devin leads; the four free members and luna keep their place; the
		// last three are the tail in some rotation, each reachable.
		want := []string{memDevin, memIceman, "opencode-go/beam-free", "openrouter/recursive/flint-v1.0:free", "openrouter/nvidia/nemotron-3.5-super-vl:free", memLuna}
		if strings.Join(c[:6], ",") != strings.Join(want, ",") {
			t.Fatalf("head of chain moved: %v", c)
		}
		tail := map[string]bool{c[6]: true, c[7]: true, c[8]: true}
		if !tail[memHaiku] || !tail[memSynthetic] || !tail[memRiver] {
			t.Fatalf("tail lost a member: %v", c)
		}
		firstOfTail[c[6]]++
	}
	for member, want := range map[string]int{memHaiku: n * 8 / 10, memSynthetic: n / 10, memRiver: n / 10} {
		if got := firstOfTail[member]; got < want*8/10 || got > want*12/10 {
			t.Fatalf("tail share of %s = %d of %d, want about %d (%v)", member, got, n, want, firstOfTail)
		}
	}
}

func TestSpreadIsStablePerThread(t *testing.T) {
	spec := provideriface.LLMSelection{Model: "pool:default"}
	for i := 0; i < 200; i++ {
		thread := fmt.Sprintf("texture:doc-%d", i)
		f0, r0, _ := Assign(spec, thread)
		want := strings.Join(chain(f0, r0), ",")
		for j := 0; j < 3; j++ {
			f, r, _ := Assign(spec, thread)
			if got := strings.Join(chain(f, r), ","); got != want {
				t.Fatalf("thread %s flipped: %s then %s", thread, want, got)
			}
		}
	}
}

func TestSpreadOnlyAppliesToPoolsThatOptIn(t *testing.T) {
	for _, name := range []string{modelcatalog.SelfDevPool, modelcatalog.LunaSyntheticPool, modelcatalog.SyntheticLunaPool} {
		for _, m := range modelcatalog.Pools()[name] {
			if m.Spread != "" || m.SpreadWeight != 0 {
				t.Fatalf("pool %s member %v opted into spread", name, m)
			}
		}
	}
	for i := 0; i < 100; i++ {
		first, rest, _ := Assign(provideriface.LLMSelection{Model: "pool:selfdev"}, fmt.Sprintf("t-%d", i))
		c := chain(first, rest)
		if c[0] != memSynthetic || (len(c) > 1 && c[1] != memRiver) {
			t.Fatalf("selfdev order changed: %v", c)
		}
	}
	policy := parseOK(t, "[pools.mine]\nmembers = [{ model = \""+memDevin+"\" }, { model = \""+memHaiku+"\", fallback_only = true }, { model = \""+memSynthetic+"\", fallback_only = true }, { model = \""+memRiver+"\", fallback_only = true }]\n\n"+rolesAndDefaults())
	for i := 0; i < 100; i++ {
		first, rest, _ := policy.Assign(provideriface.LLMSelection{Model: "pool:mine"}, fmt.Sprintf("t-%d", i))
		if c := chain(first, rest); c[1] != memHaiku || c[2] != memSynthetic || c[3] != memRiver {
			t.Fatalf("pool without spread reordered: %v", c)
		}
	}
}

func TestSpreadMemberKeysParseValidateAndShow(t *testing.T) {
	text := "[pools.mine]\nmembers = [{ model = \"" + memDevin + "\" }, { model = \"" + memHaiku + "\", fallback_only = true, spread = \"tail\", spread_weight = 8 }, { model = \"" + memSynthetic + "\", fallback_only = true, spread = \"tail\" }, { model = \"" + memRiver + "\", fallback_only = true, spread = \"tail\" }]\n\n" + rolesAndDefaults()
	policy := parseOK(t, text)
	pool, _ := policy.pool("mine")
	if m := pool.Members; m[1].Spread != "tail" || m[1].SpreadWeight != 8 || m[2].Spread != "tail" || m[2].SpreadWeight != 0 || m[0].Spread != "" {
		t.Fatalf("spread not read: %+v", m)
	}
	starts := map[string]int{}
	for i := 0; i < 2000; i++ {
		_, rest, _ := policy.Assign(provideriface.LLMSelection{Model: "pool:mine"}, fmt.Sprintf("t-%d", i))
		starts[rest[0].Provider]++
	}
	if starts["bedrock"] < 1400 || starts["synthetic"] < 120 || starts["river"] < 120 {
		t.Fatalf("file pool spread: %v", starts)
	}
	for _, bad := range []string{`spread = tail`, `spread = ""`, `spread = "a b"`, `spread_weight = 3`, `spread = "x", spread_weight = 0`, `spread = "x", spread_weight = 1001`, `spread = "x", spread = "y"`} {
		if _, err := parsePolicy("[pools.mine]\nmembers = [{ model = \""+memDevin+"\", "+bad+" }]\n\n"+rolesAndDefaults(), "test"); err == nil {
			t.Fatalf("accepted bad spread %q", bad)
		}
	}
	for _, v := range policy.PoolViews() {
		if v.Name == "mine" && (v.Members[1].Spread != "tail" || v.Members[1].SpreadWeight != 8 || v.Members[2].SpreadWeight != 1) {
			t.Fatalf("PoolViews dropped the group: %+v", v.Members)
		}
	}
}

// The vision chain of the default pool: nemotron is the first vision member,
// ahead of the whole tail, and Haiku is in it. The text-only lead and the
// text-only free members are never picked for an image.
func TestDefaultPoolImageChainStillReachesAVisionMember(t *testing.T) {
	for i := 0; i < 200; i++ {
		first, rest, _ := Assign(provideriface.LLMSelection{Model: "pool:default"}, fmt.Sprintf("t-%d", i))
		var vision []string
		for _, s := range append([]provideriface.LLMSelection{first}, rest...) {
			if modelcatalog.SupportsImage(s.Model) {
				vision = append(vision, s.Provider+"/"+s.Model)
			}
		}
		hasHaiku := false
		for _, v := range vision {
			hasHaiku = hasHaiku || v == memHaiku
		}
		if len(vision) == 0 || vision[0] != "openrouter/nvidia/nemotron-3.5-super-vl:free" || !hasHaiku {
			t.Fatalf("image chain = %v", vision)
		}
	}
}
