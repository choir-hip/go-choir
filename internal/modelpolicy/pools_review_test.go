package modelpolicy

// Devin's review of fc4ee0a1f (findings 2 and 4).

import (
	"context"
	"os"
	"strings"
	"testing"
)

// Finding 2: the parser and hasPoolSection read the same header spelling.
func TestHasPoolSectionMatchesParser(t *testing.T) {
	for _, header := range []string{"[pools.mine]", "[ pools.mine ]", "[pools.mine ]", "[ pools.mine]", "[pools.mine] # note", "  [pools.mine]"} {
		text := header + "\nmembers = [\"" + memLuna + "\"]\n\n" + rolesAndDefaults()
		policy, err := parsePolicy(text, "test")
		if err != nil || !policy.HasPool("mine") {
			t.Errorf("%q: parser does not read it as a pool: %v", header, err)
		}
		if !hasPoolSection(text) {
			t.Errorf("%q: the parser reads a pool section that hasPoolSection does not see", header)
		}
	}
	if hasPoolSection("[defaults]\nmodel = \"pool:default\"\n# [pools.x]\n") {
		t.Error("a comment counts as a pool section")
	}
}

func TestEnsureDefaultPolicyFileKeepsSpacedPoolHeader(t *testing.T) {
	text := "[ pools.default ]\nmembers = [{ model = \"" + memLuna + "\" }]\n\n" + rolesAndDefaults()
	manager := writePolicyFile(t, text)
	if _, err := manager.Load(context.Background(), "owner"); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(manager.config.PolicyPath)
	if string(raw) != text {
		t.Fatalf("file rewritten:\n%s", raw)
	}
}

// Finding 4: tabs are whitespace around = and values; a duplicate key is
// refused, not last-wins.
func TestPoolTabsAroundTokens(t *testing.T) {
	text := "[pools.mine]\nmembers = [{\tmodel\t=\t\"" + memDevin + "\",\tweight\t=\t3\t}, {model=\"" + memIceman + "\",fallback_only\t=\ttrue\t}]\n\n" + rolesAndDefaults()
	pool, ok := parseOK(t, text).pool("mine")
	if !ok || len(pool.Members) != 2 || pool.Members[0].SlotWeight() != 3 || pool.Members[1].SlotWeight() != 0 {
		t.Fatalf("pool = %+v", pool)
	}
}

func TestPoolDuplicateKeysRefused(t *testing.T) {
	for name, section := range map[string]string{
		"failover twice": "[pools.p]\nmembers = [\"" + memDevin + "\"]\nfailover = true\nfailover = false",
		"model twice":    "[pools.p]\nmembers = [{ model = \"" + memDevin + "\", model = \"" + memLuna + "\" }]",
		"weight twice":   "[pools.p]\nmembers = [{ model = \"" + memDevin + "\", weight = 1, weight = 2 }]",
		"fallback twice": "[pools.p]\nmembers = [{ model = \"" + memDevin + "\", fallback_only = true, fallback_only = false }]",
	} {
		if _, err := parsePolicy(section+"\n\n"+rolesAndDefaults(), "test"); err == nil || !strings.Contains(err.Error(), "twice") {
			t.Errorf("%s: %v", name, err)
		}
	}
}
