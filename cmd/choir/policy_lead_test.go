package main

// `choir policy lead | pin` and the pool provenance in `policy get`, against
// the real files handler and the real policy manager behind an httptest
// server (docs/design/programmatic-model-switching-2026-11-09.md, section 9,
// rows 24 to 35). The model ids are ones the catalog lists.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	leadDevin  = "devin/swe-2-flash"
	leadIceman = "opencode-go/iceman-contributor-free"
	leadHaiku  = "bedrock/us.anthropic.claude-haiku-5-5"
	leadSynth  = "synthetic/hf:deepseek-ai/DeepSeek-V4.1-Flash-1017"
)

func readServerPolicy(t *testing.T, root string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "System", "model-policy.toml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func getPools(t *testing.T) map[string]policyPool {
	t.Helper()
	code, out, errOut := runPolicy("get")
	if code != 0 {
		t.Fatalf("get: %s", errOut)
	}
	var res policyGetResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatal(err)
	}
	pools := map[string]policyPool{}
	for _, p := range res.Pools {
		pools[p.Name] = p
	}
	return pools
}

func memberModels(p policyPool) string {
	var names []string
	for _, m := range p.Members {
		names = append(names, m.Model)
	}
	return strings.Join(names, ",")
}

// Row 24, 32, 33: get shows each pool's source; a rollback to a file without
// pool sections goes back to the release's pools.
func TestPolicyLeadThenRollbackRestoresRelease(t *testing.T) {
	root, _ := policyServer(t)
	seedServerPolicy(t, root, cliPolicy)
	before := getPools(t)
	if before["default"].Source != "release" || before["selfdev"].Source != "release" {
		t.Fatalf("sources before = %+v", before)
	}
	code, out, errOut := runPolicy("lead", leadIceman)
	if code != 0 {
		t.Fatalf("lead: %d %s", code, errOut)
	}
	var res policyLeadResult
	if err := json.Unmarshal([]byte(out), &res); err != nil || res.Status != "saved" || res.Lead != leadIceman || len(res.Notes) == 0 {
		t.Fatalf("result = %+v (%v)\n%s", res, err, out)
	}
	after := getPools(t)
	if after["default"].Source != "file" || after["selfdev"].Source != "release" {
		t.Fatalf("sources after = %+v", after)
	}
	d := after["default"]
	if d.Members[0].Model != leadIceman || d.Members[0].Weight != 1 || d.Members[0].FallbackOnly {
		t.Fatalf("lead = %+v", d.Members[0])
	}
	for _, m := range d.Members[1:] {
		if !m.FallbackOnly || m.Weight != 0 {
			t.Fatalf("member %+v is not a fallback", m)
		}
	}
	if len(d.Members) != len(before["default"].Members) {
		t.Fatalf("members %d -> %d", len(before["default"].Members), len(d.Members))
	}
	if code, _, errOut := runPolicy("rollback"); code != 0 {
		t.Fatalf("rollback: %s", errOut)
	}
	if got := readServerPolicy(t, root); got != cliPolicy {
		t.Fatalf("after rollback = %q", got)
	}
	if rolled := getPools(t); rolled["default"].Source != "release" || memberModels(rolled["default"]) != memberModels(before["default"]) {
		t.Fatalf("rolled back = %+v", rolled["default"])
	}
}

// Row 28: the lead is already a member; the rest keep their order.
func TestPolicyLeadReordersAndAdds(t *testing.T) {
	root, _ := policyServer(t)
	seedServerPolicy(t, root, cliPolicy)
	before := getPools(t)["default"]
	if code, _, errOut := runPolicy("lead", "chatgpt/gpt-6-luna", "--allow-paid"); code != 0 {
		t.Fatalf("lead: %s", errOut)
	}
	got := getPools(t)["default"]
	want := []string{"chatgpt/gpt-6-luna"}
	for _, m := range before.Members {
		if m.Model != "chatgpt/gpt-6-luna" {
			want = append(want, m.Model)
		}
	}
	if memberModels(got) != strings.Join(want, ",") {
		t.Fatalf("members = %s, want %s", memberModels(got), strings.Join(want, ","))
	}
	// A catalogued model that is not in the pool is added as the lead.
	if code, _, errOut := runPolicy("lead", "zai/glm-5.2", "--allow-paid"); code != 0 {
		t.Fatalf("lead (new member): %s", errOut)
	}
	got = getPools(t)["default"]
	if len(got.Members) != len(before.Members)+1 || got.Members[0].Model != "zai/glm-5.2" {
		t.Fatalf("members = %s", memberModels(got))
	}
}

// Row 25.
func TestPolicyLeadRefusesWeightedAndNoFailover(t *testing.T) {
	root, seen := policyServer(t)
	seedServerPolicy(t, root, cliPolicy)
	code, _, errOut := runPolicy("lead", leadIceman, "--pool", "synthetic-luna")
	if code == 0 || !strings.Contains(errOut, "--single-lead") {
		t.Fatalf("code %d, stderr %q", code, errOut)
	}
	for _, r := range *seen {
		if strings.HasPrefix(r, "PUT ") {
			t.Fatalf("a refused lead reached the server: %v", *seen)
		}
	}
	if code, _, errOut := runPolicy("lead", leadIceman, "--pool", "synthetic-luna", "--single-lead"); code != 0 {
		t.Fatalf("--single-lead: %s", errOut)
	}
	p := getPools(t)["synthetic-luna"]
	if p.Source != "file" || p.Failover || p.Members[0].Model != leadIceman || p.Members[0].Weight != 1 {
		t.Fatalf("pool = %+v", p)
	}
	for _, m := range p.Members[1:] {
		if !m.FallbackOnly {
			t.Fatalf("member %+v still takes threads", m)
		}
	}
}

// Row 26.
func TestPolicyLeadMeteredNeedsAllowPaid(t *testing.T) {
	root, _ := policyServer(t)
	seedServerPolicy(t, root, cliPolicy)
	code, _, errOut := runPolicy("lead", leadHaiku)
	if code == 0 || !strings.Contains(errOut, "--allow-paid") || !strings.Contains(errOut, "$0.1 per million input") {
		t.Fatalf("code %d, stderr %q", code, errOut)
	}
	if readServerPolicy(t, root) != cliPolicy {
		t.Fatal("the refused lead changed the file")
	}
	if code, _, errOut := runPolicy("lead", leadHaiku, "--allow-paid"); code != 0 {
		t.Fatalf("--allow-paid: %s", errOut)
	}
	// A free lead needs no flag.
	if code, _, errOut := runPolicy("lead", leadDevin); code != 0 {
		t.Fatalf("free lead: %s", errOut)
	}
}

// Row 11, 27: the server refuses a member the catalog lacks; nothing changes.
func TestPolicyLeadUncataloguedRefused(t *testing.T) {
	root, _ := policyServer(t)
	seedServerPolicy(t, root, cliPolicy)
	code, _, errOut := runPolicy("lead", "opencode-go/no-such-model")
	if code == 0 || !strings.Contains(errOut, "no-such-model") {
		t.Fatalf("code %d, stderr %q", code, errOut)
	}
	if readServerPolicy(t, root) != cliPolicy {
		t.Fatal("the refused lead changed the file")
	}
}

// Row 11: the validated write refuses a stale member written by hand too.
func TestPolicyPutRefusesUncataloguedMember(t *testing.T) {
	root, _ := policyServer(t)
	seedServerPolicy(t, root, cliPolicy)
	bad := "# owner-edited\n[pools.default]\nmembers = [\"opencode-go/no-such-model\", \"" + leadIceman + "\"]\n\n" + strings.TrimPrefix(cliPolicy, "# owner-edited\n")
	code, _, errOut := runPolicy("set", "--local", writeLocal(t, bad))
	if code == 0 || !strings.Contains(errOut, "no-such-model") || readServerPolicy(t, root) != cliPolicy {
		t.Fatalf("code %d, stderr %q", code, errOut)
	}
}

// Row 29.
func TestPolicyLeadIdempotent(t *testing.T) {
	root, _ := policyServer(t)
	seedServerPolicy(t, root, cliPolicy)
	if code, _, errOut := runPolicy("lead", leadIceman); code != 0 {
		t.Fatal(errOut)
	}
	first := readServerPolicy(t, root)
	if code, _, errOut := runPolicy("lead", leadIceman); code != 0 {
		t.Fatal(errOut)
	}
	if second := readServerPolicy(t, root); second != first || strings.Count(second, "[pools.default]") != 1 {
		t.Fatalf("second run changed the file:\n%s", second)
	}
}

// Row 30.
func TestPolicyLeadKeepsRolesAndMark(t *testing.T) {
	root, _ := policyServer(t)
	unmarked := "# my notes\n" + strings.TrimPrefix(cliPolicy, "# owner-edited\n")
	seedServerPolicy(t, root, unmarked)
	if code, _, errOut := runPolicy("lead", leadIceman); code != 0 {
		t.Fatal(errOut)
	}
	got := readServerPolicy(t, root)
	if !hasOwnerEditedLine([]byte(got)) || !strings.Contains(got, "# my notes") || !strings.Contains(got, "[roles.texture]") || !strings.Contains(got, `reasoning = "medium"`) {
		t.Fatalf("file lost content:\n%s", got)
	}
}

// Row 31.
func TestPolicyPinRoundTrip(t *testing.T) {
	root, _ := policyServer(t)
	seedServerPolicy(t, root, cliPolicy)
	before := getPools(t)
	code, out, errOut := runPolicy("pin")
	if code != 0 {
		t.Fatalf("pin: %s", errOut)
	}
	var res policyPinResult
	if err := json.Unmarshal([]byte(out), &res); err != nil || len(res.Pinned) != len(before) {
		t.Fatalf("result = %+v (%v)", res, err)
	}
	after := getPools(t)
	for name, p := range before {
		a := after[name]
		if a.Source != "file" || memberModels(a) != memberModels(p) || a.Failover != p.Failover {
			t.Fatalf("pool %s: before %+v after %+v", name, p, a)
		}
		for i := range p.Members {
			if p.Members[i] != a.Members[i] {
				t.Fatalf("pool %s member %d: %+v != %+v", name, i, p.Members[i], a.Members[i])
			}
		}
	}
	// Pinning again changes nothing.
	first := readServerPolicy(t, root)
	if code, _, errOut := runPolicy("pin"); code != 0 {
		t.Fatal(errOut)
	}
	if readServerPolicy(t, root) != first {
		t.Fatal("pin is not idempotent")
	}
}

// Row 35.
func TestPolicyLeadRefusesBadNames(t *testing.T) {
	root, seen := policyServer(t)
	seedServerPolicy(t, root, cliPolicy)
	for _, args := range [][]string{
		{"lead", `opencode-go/x"y`},
		{"lead", "opencode-go/x\ny"},
		{"lead", "no-slash"},
		{"lead", leadIceman, "--pool", `a"b`},
		{"lead", leadIceman, "--pool", "../x"},
	} {
		if code, _, errOut := runPolicy(args...); code == 0 {
			t.Errorf("%q accepted", args)
		} else if errOut == "" {
			t.Errorf("%q: no message", args)
		}
	}
	for _, r := range *seen {
		if strings.HasPrefix(r, "PUT ") {
			t.Fatalf("a refused lead reached the server: %v", *seen)
		}
	}
}

// Rendering: the section the CLI writes reads back as the pool it was given.
func TestRenderedPoolParsesToSameMembers(t *testing.T) {
	root, _ := policyServer(t)
	seedServerPolicy(t, root, cliPolicy)
	text := policyWithPools(cliPolicy, []policyPool{{Name: "mine", Failover: false, Members: []policyPoolMember{{Model: leadIceman, Weight: 2}, {Model: leadDevin, FallbackOnly: true}}}})
	if code, _, errOut := runPolicy("set", "--local", writeLocal(t, text)); code != 0 {
		t.Fatalf("set: %s\n%s", errOut, text)
	}
	p := getPools(t)["mine"]
	if p.Source != "file" || p.Failover || len(p.Members) != 2 || p.Members[0].Weight != 2 || !p.Members[1].FallbackOnly {
		t.Fatalf("pool = %+v", p)
	}
}
