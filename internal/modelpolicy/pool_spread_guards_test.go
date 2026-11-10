package modelpolicy

// Guards (Devin review of the spread commit). Ways this could fail: spread on
// a weighted member lets the spread hash put a fallback-only role in the lead
// slot; a same-name group split by another member is silently inert; a plain
// list shadowing the default pool silently drops the paid-tail spread; the
// renderer of `choir policy pin` drops a field.

import (
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

func TestPlainListCannotFlattenASpreadPool(t *testing.T) {
	_, err := parsePolicy("[pools.default]\nmembers = [\""+memDevin+"\", \""+memHaiku+"\"]\n\n"+rolesAndDefaults(), "test")
	if err == nil || !strings.Contains(err.Error(), "spread") {
		t.Fatalf("plain list over the default pool accepted or unclear: %v", err)
	}
}
