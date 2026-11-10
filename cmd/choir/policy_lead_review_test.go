package main

// Devin's review of fc4ee0a1f (findings 1 and 3).

import (
	"encoding/json"
	"strings"
	"testing"
)

// Finding 1: free only when positively known free; everything else, priced or
// unpriced, is metered.
func TestModelCostTable(t *testing.T) {
	metered := []string{
		"bedrock/us.anthropic.claude-haiku-4-5-20251001-v1:0",
		"bedrock/us.anthropic.claude-sonnet-4-6",
		"bedrock/us.anthropic.claude-opus-4-6-v1",
		"bedrock/us.anthropic.claude-haiku-5-5",
		"zai/glm-5.2", "zai/glm-5.1", "zai/glm-5-turbo",
		"opencode-go/glm-5.3-flash", "opencode-go/muse-spark-1.3-contributor", "opencode-go/hy3", "opencode-go/qwen3.7-max",
		"chatgpt/gpt-6-luna", leadSynth, "river/deepseek-ai/DeepSeek-V4.1-Flash-1017",
		"unknown/never-heard-of-it",
	}
	for _, m := range metered {
		if free, _ := modelCost(m); free {
			t.Errorf("%s is treated as free", m)
		}
	}
	free := []string{
		leadDevin, "devin/swe-2",
		"openrouter/recursive/flint-v1.0:free", "openrouter/nvidia/nemotron-3.5-super-vl:free",
		leadIceman, "opencode-go/beam-free", "opencode-go/muse-spark-1.3-contributor-free",
	}
	for _, m := range free {
		if isFree, text := modelCost(m); !isFree || text != "" {
			t.Errorf("%s is not free: %v %q", m, isFree, text)
		}
	}
	if _, text := modelCost("bedrock/us.anthropic.claude-opus-4-6-v1"); text != "price unknown" {
		t.Errorf("unpriced text = %q", text)
	}
	if _, text := modelCost("chatgpt/gpt-6-luna"); !strings.Contains(text, "$0.1 per million input") {
		t.Errorf("priced text = %q", text)
	}
}

// An unpriced catalogued model needs the flag end to end.
func TestPolicyLeadUnpricedNeedsAllowPaid(t *testing.T) {
	root, _ := policyServer(t)
	seedServerPolicy(t, root, cliPolicy)
	code, _, errOut := runPolicy("lead", "bedrock/us.anthropic.claude-opus-4-6-v1")
	if code == 0 || !strings.Contains(errOut, "--allow-paid") || !strings.Contains(errOut, "price unknown") || readServerPolicy(t, root) != cliPolicy {
		t.Fatalf("code %d, stderr %q", code, errOut)
	}
	if code, _, errOut := runPolicy("lead", "bedrock/us.anthropic.claude-opus-4-6-v1", "--allow-paid"); code != 0 {
		t.Fatalf("--allow-paid: %s", errOut)
	}
}

// Finding 3: pin does not render an empty members list; it drops the pool
// with no usable member and says so.
func TestPolicyPinDropsEmptyPoolWithNote(t *testing.T) {
	root, _ := policyServer(t)
	seedServerPolicy(t, root, cliPolicy+"\n[pools.ghost]\nmembers = [\"stealth/crouton-alpha\"]\n")
	code, out, errOut := runPolicy("pin")
	if code != 0 {
		t.Fatalf("pin: %s", errOut)
	}
	var res policyPinResult
	if err := json.Unmarshal([]byte(out), &res); err != nil || len(res.Notes) == 0 || !strings.Contains(res.Notes[0], "ghost") {
		t.Fatalf("result = %+v (%v)", res, err)
	}
	if got := readServerPolicy(t, root); strings.Contains(got, "ghost") || strings.Contains(got, "members = [\n]") {
		t.Fatalf("file:\n%s", got)
	}
}
