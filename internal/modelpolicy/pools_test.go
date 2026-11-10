package modelpolicy

// Pool sections in the policy file and the one pool resolver
// (docs/design/programmatic-model-switching-2026-11-09.md, section 9, the M1
// failure list; each test names the row it covers).

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/modelcatalog"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
)

const (
	memDevin  = "devin/swe-2-flash"
	memIceman = "opencode-go/iceman-contributor-free"
	memHaiku  = "bedrock/us.anthropic.claude-haiku-5-5"
	memLuna   = "chatgpt/gpt-6-luna"
)

func rolesAndDefaults() string {
	return "[defaults]\nmodel = \"pool:default\"\nreasoning = \"low\"\n\n[roles.texture]\nmodel = \"pool:default\"\n"
}

func parseOK(t *testing.T, text string) Policy {
	t.Helper()
	policy, err := parsePolicy(text, "test")
	if err != nil {
		t.Fatalf("parse: %v\n%s", err, text)
	}
	return policy
}

func memberNames(members []modelcatalog.PoolMember) []string {
	out := make([]string, 0, len(members))
	for _, m := range members {
		out = append(out, m.Provider+"/"+m.Model)
	}
	return out
}

func writePolicyFile(t *testing.T, text string) *Manager {
	t.Helper()
	path := filepath.Join(t.TempDir(), "System", "model-policy.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return NewManager(ManagerConfig{PolicyPath: path})
}

// Row 1.
func TestPoolShorthandFlattens(t *testing.T) {
	policy := parseOK(t, "[pools.mine]\nmembers = [\""+memLuna+"\", \""+memIceman+"\", \""+memHaiku+"\"]\n\n"+rolesAndDefaults())
	pool, ok := policy.pool("mine")
	if !ok || pool.Source != PoolSourceFile || !pool.Failover {
		t.Fatalf("pool = %+v ok=%v", pool, ok)
	}
	if got := strings.Join(memberNames(pool.Members), ","); got != memLuna+","+memIceman+","+memHaiku {
		t.Fatalf("order = %s", got)
	}
	if pool.Members[0].SlotWeight() != 1 || pool.Members[1].SlotWeight() != 0 || pool.Members[2].SlotWeight() != 0 {
		t.Fatalf("weights = %+v", pool.Members)
	}
}

// Row 2.
func TestShorthandShadowingWeightedOrNoFailoverPoolRefused(t *testing.T) {
	for _, name := range []string{modelcatalog.SyntheticLunaPool, modelcatalog.LunaSyntheticPool} {
		text := "[pools." + name + "]\nmembers = [\"" + memLuna + "\", \"" + memIceman + "\"]\n\n" + rolesAndDefaults()
		_, err := parsePolicy(text, "test")
		if name == modelcatalog.SyntheticLunaPool {
			if err == nil || !strings.Contains(err.Error(), "flatten") {
				t.Fatalf("%s: shorthand over a weighted no-failover pool: %v", name, err)
			}
			continue
		}
		// luna-synthetic has one weighted member and failover: accepted.
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	// The same pool written out is accepted.
	text := "[pools.synthetic-luna]\nmembers = [{ model = \"" + memLuna + "\", weight = 1 }, { model = \"" + memIceman + "\", weight = 1 }]\nfailover = false\n\n" + rolesAndDefaults()
	policy := parseOK(t, text)
	if pool, _ := policy.pool("synthetic-luna"); pool.Failover || pool.Source != PoolSourceFile {
		t.Fatalf("pool = %+v", pool)
	}
}

// Row 3.
func TestShorthandShadowingSingleLeadPoolAccepted(t *testing.T) {
	for _, name := range []string{modelcatalog.SelfDevPool} { // the default pool carries a spread group: a plain list is refused (pool_spread_guards_test.go)
		parseOK(t, "[pools."+name+"]\nmembers = [{ model = \""+memLuna+"\" }, { model = \""+memIceman+"\", fallback_only = true }]\n\n"+rolesAndDefaults())
	}
}

// Row 4.
func TestPoolInlineTables(t *testing.T) {
	text := "[pools.mine]\nmembers = [{ model = \"" + memDevin + "\", weight = 3 }, { model = \"" + memIceman + "\" }, { model = \"" + memLuna + "\", fallback_only = true }]\nfailover = false\n\n" + rolesAndDefaults()
	pool, _ := parseOK(t, text).pool("mine")
	if pool.Failover {
		t.Fatal("failover = false was read as true")
	}
	if w := []int{pool.Members[0].SlotWeight(), pool.Members[1].SlotWeight(), pool.Members[2].SlotWeight()}; w[0] != 3 || w[1] != 1 || w[2] != 0 {
		t.Fatalf("weights = %v", w)
	}
	// failover omitted: true.
	pool, _ = parseOK(t, "[pools.mine]\nmembers = [{ model = \""+memDevin+"\" }]\n\n"+rolesAndDefaults()).pool("mine")
	if !pool.Failover {
		t.Fatal("omitted failover is not true")
	}
}

// Row 4 (refusals).
func TestPoolParseRefusals(t *testing.T) {
	cases := map[string]string{
		"weight zero":              `[pools.p]` + "\n" + `members = [{ model = "` + memDevin + `", weight = 0 }]`,
		"weight negative":          `[pools.p]` + "\n" + `members = [{ model = "` + memDevin + `", weight = -1 }]`,
		"weight and fallback":      `[pools.p]` + "\n" + `members = [{ model = "` + memDevin + `", weight = 2, fallback_only = true }]`,
		"unknown member key":       `[pools.p]` + "\n" + `members = [{ model = "` + memDevin + `", color = "red" }]`,
		"mixed forms":              `[pools.p]` + "\n" + `members = ["` + memDevin + `", { model = "` + memLuna + `" }]`,
		"table without model":      `[pools.p]` + "\n" + `members = [{ weight = 1 }]`,
		"unqualified model":        `[pools.p]` + "\n" + `members = ["swe-2-flash"]`,
		"empty members":            `[pools.p]` + "\n" + `members = []`,
		"no members":               `[pools.p]` + "\n" + `failover = true`,
		"duplicate section":        `[pools.p]` + "\n" + `members = ["` + memDevin + `"]` + "\n" + `[pools.p]` + "\n" + `members = ["` + memLuna + `"]`,
		"unknown pool key":         `[pools.p]` + "\n" + `members = ["` + memDevin + `"]` + "\n" + `color = "red"`,
		"bad failover":             `[pools.p]` + "\n" + `members = ["` + memDevin + `"]` + "\n" + `failover = maybe`,
		"unclosed list":            `[pools.p]` + "\n" + `members = [` + "\n" + `"` + memDevin + `",`,
		"bad pool name":            `[pools.p"q]` + "\n" + `members = ["` + memDevin + `"]`,
		"members given twice":      `[pools.p]` + "\n" + `members = ["` + memDevin + `"]` + "\n" + `members = ["` + memLuna + `"]`,
		"members not a list":       `[pools.p]` + "\n" + `members = "` + memDevin + `"`,
		"junk after list":          `[pools.p]` + "\n" + `members = ["` + memDevin + `"] extra`,
		"model not a string":       `[pools.p]` + "\n" + `members = [{ model = 3 }]`,
		"fallback_only not a bool": `[pools.p]` + "\n" + `members = [{ model = "` + memDevin + `", fallback_only = 1 }]`,
	}
	for name, section := range cases {
		if _, err := parsePolicy(section+"\n\n"+rolesAndDefaults(), "test"); err == nil {
			t.Errorf("%s: accepted:\n%s", name, section)
		}
	}
}

// Row 5.
func TestPoolMultilineArrayAndComments(t *testing.T) {
	text := `# owner-edited
[pools.default]   # the lead
members = [
  # first is the lead
  { model = "` + memDevin + `", weight = 1 },   # trailing comment
  { model = "` + memIceman + `", fallback_only = true },
  "x/y",
]
failover = true
`
	// The last entry is a plain string next to tables: refused, naming the mix.
	if _, err := parsePolicy(text+rolesAndDefaults(), "test"); err == nil || !strings.Contains(err.Error(), "mixes") {
		t.Fatalf("mixed list: %v", err)
	}
	text = strings.Replace(text, "  \"x/y\",\n", "", 1)
	policy := parseOK(t, text+"\n"+rolesAndDefaults())
	pool, _ := policy.pool("default")
	if got := strings.Join(memberNames(pool.Members), ","); got != memDevin+","+memIceman || pool.Source != PoolSourceFile {
		t.Fatalf("pool = %+v", pool)
	}
	// Brackets, braces, a hash and an escaped quote inside a string do not end the list.
	if got := arrayComplete(`[ "a]b{c#d\"e" ,`); got {
		t.Fatal("arrayComplete closed on characters inside a string")
	}
	if !arrayComplete(`[ "a]b{c#d\"e" ]`) {
		t.Fatal("arrayComplete did not close")
	}
}

// Row 6 and 11: write-time validation.
func TestValidateTextAcceptsPoolSections(t *testing.T) {
	text := "# owner-edited\n[pools.default]\nmembers = [\n  { model = \"" + memDevin + "\", weight = 1 },\n  { model = \"" + memIceman + "\", fallback_only = true },\n]\nfailover = true\n\n" + rolesAndDefaults()
	if err := ValidateText(text); err != nil {
		t.Fatalf("ValidateText refused a good pool file: %v", err)
	}
}

func TestValidateTextRefusesUncataloguedMember(t *testing.T) {
	text := "# owner-edited\n[pools.default]\nmembers = [{ model = \"opencode-go/no-such-model\" }, { model = \"" + memIceman + "\", fallback_only = true }]\n\n" + rolesAndDefaults()
	err := ValidateText(text)
	if err == nil || !strings.Contains(err.Error(), "no-such-model") {
		t.Fatalf("ValidateText: %v", err)
	}
	// Run start does not refuse it; it drops the member (row 9).
	policy := parseOK(t, text)
	pool, _ := policy.pool("default")
	if len(pool.Members) != 1 || pool.Members[0].Model != "iceman-contributor-free" {
		t.Fatalf("pool = %+v", pool.Members)
	}
}

// Row 9, 10.
func TestStaleMemberDroppedWithDiagnostic(t *testing.T) {
	text := "[pools.mine]\nmembers = [\"" + memLuna + "\", \"stealth/crouton-alpha\", \"" + memIceman + "\"]\n\n" + rolesAndDefaults()
	policy := parseOK(t, text)
	pool, _ := policy.pool("mine")
	if got := strings.Join(memberNames(pool.Members), ","); got != memLuna+","+memIceman {
		t.Fatalf("members = %s", got)
	}
	if len(policy.Diagnostics) != 1 || !strings.Contains(policy.Diagnostics[0], "crouton-alpha") {
		t.Fatalf("diagnostics = %v", policy.Diagnostics)
	}
}

func TestStaleMemberDiagnosticReachesMetadata(t *testing.T) {
	manager := writePolicyFile(t, "# owner-edited\n[pools.default]\nmembers = [{ model = \"stealth/crouton-alpha\" }, { model = \""+memIceman+"\", fallback_only = true }]\n\n"+rolesAndDefaults())
	metadata := manager.EnrichMetadata(context.Background(), "owner", "texture", map[string]any{"trajectory_id": "t1"})
	if got := metadataString(metadata, MetadataPolicyError); !strings.Contains(got, "crouton-alpha") {
		t.Fatalf("llm_policy_error = %q", got)
	}
	if metadataString(metadata, MetadataModel) != "iceman-contributor-free" {
		t.Fatalf("model = %v", metadata[MetadataModel])
	}
}

// Row 12.
func TestAllMembersStaleFallsBackToCompiled(t *testing.T) {
	policy := parseOK(t, "[pools.default]\nmembers = [{ model = \"stealth/crouton-alpha\" }]\n\n"+rolesAndDefaults())
	pool, ok := policy.pool("default")
	if !ok || pool.Source != PoolSourceRelease || len(pool.Members) == 0 {
		t.Fatalf("pool = %+v", pool)
	}
	if len(policy.Diagnostics) != 2 || !strings.Contains(policy.Diagnostics[1], "release") {
		t.Fatalf("diagnostics = %v", policy.Diagnostics)
	}
	chosen, _, assignment := policy.Assign(provideriface.LLMSelection{Model: "pool:default"}, "t")
	if assignment != "hash" || chosen.Model == "" {
		t.Fatalf("assign = %q %+v", assignment, chosen)
	}
}

// Rows 13, 14, 15.
func TestFilePoolWithoutCompiledNamesAndNoMembers(t *testing.T) {
	text := "[pools.ghost]\nmembers = [{ model = \"stealth/crouton-alpha\" }]\n\n[defaults]\nmodel = \"pool:default\"\n\n[roles.texture]\nmodel = \"pool:ghost\"\n"
	policy := parseOK(t, text) // the file is not invalidated
	if len(policy.Diagnostics) == 0 {
		t.Fatal("no diagnostic")
	}
	chosen, fallbacks, assignment := policy.Assign(policy.Resolve("texture"), "t")
	if assignment != AssignmentUnknownPool || chosen.Provider != "" || len(fallbacks) != 0 {
		t.Fatalf("assign = %q %+v %+v", assignment, chosen, fallbacks)
	}
}

func TestFileDefinedPoolNamedByRole(t *testing.T) {
	text := "[pools.mine]\nmembers = [\"" + memLuna + "\", \"" + memIceman + "\"]\n\n[defaults]\nmodel = \"pool:default\"\n\n[roles.texture]\nmodel = \"pool:mine\"\n"
	policy := parseOK(t, text)
	chosen, fallbacks, assignment := policy.Assign(policy.Resolve("texture"), "t")
	if assignment != "hash" || chosen.Model != "gpt-6-luna" || len(fallbacks) != 1 || fallbacks[0].Model != "iceman-contributor-free" {
		t.Fatalf("assign = %q %+v %+v", assignment, chosen, fallbacks)
	}
	// Without the section the name is unknown and the file is invalid.
	if _, err := parsePolicy(strings.SplitN(text, "\n\n", 2)[1], "test"); err == nil || !strings.Contains(err.Error(), "mine") {
		t.Fatalf("pool without section: %v", err)
	}
}

// Row 16.
func TestOverlayNamingFilePool(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "System", "model-policy.toml")
	if err := os.MkdirAll(filepath.Join(dir, "System", "model-policy-overlays"), 0o755); err != nil {
		t.Fatal(err)
	}
	base := "# owner-edited\n[pools.mine]\nmembers = [{ model = \"" + memLuna + "\" }]\n\n" + rolesAndDefaults()
	if err := os.WriteFile(path, []byte(base), 0o644); err != nil {
		t.Fatal(err)
	}
	overlay := "[overlay]\nexpires_at = \"2099-01-01T00:00:00Z\"\n\n[roles.texture]\nmodel = \"pool:mine\"\n"
	if err := os.WriteFile(filepath.Join(dir, "System", "model-policy-overlays", "o1.toml"), []byte(overlay), 0o644); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(ManagerConfig{PolicyPath: path})
	metadata := manager.EnrichMetadata(context.Background(), "owner", "texture", map[string]any{MetadataPolicyOverlayID: "o1", "trajectory_id": "t1"})
	if metadataString(metadata, MetadataPolicyError) != "" || metadataString(metadata, MetadataModel) != "gpt-6-luna" {
		t.Fatalf("metadata = %+v", metadata)
	}
}

// Row 17.
func TestFilePoolAssignWeightsAndFailover(t *testing.T) {
	text := "[pools.mine]\nmembers = [{ model = \"" + memLuna + "\", weight = 3 }, { model = \"" + memIceman + "\", weight = 1 }]\nfailover = false\n\n" + rolesAndDefaults()
	policy := parseOK(t, text)
	counts := map[string]int{}
	for i := 0; i < 4000; i++ {
		chosen, fallbacks, _ := policy.Assign(provideriface.LLMSelection{Model: "pool:mine"}, fmt.Sprintf("thread-%d", i))
		if len(fallbacks) != 0 {
			t.Fatalf("failover = false returned fallbacks %+v", fallbacks)
		}
		counts[chosen.Model]++
	}
	if counts["gpt-6-luna"] < 2700 || counts["gpt-6-luna"] > 3300 || counts["iceman-contributor-free"] == 0 {
		t.Fatalf("share = %v, want about 3:1", counts)
	}
}

// Row 20.
func TestRetiredRemapUsesFileDefaultPool(t *testing.T) {
	text := "[pools.default]\nmembers = [{ model = \"" + memLuna + "\" }]\n\n" + rolesAndDefaults()
	policy := parseOK(t, text)
	chosen, _, assignment := policy.Assign(provideriface.LLMSelection{Provider: "chatgpt", Model: "gpt-5.6-luna"}, "t")
	if assignment != "retired_remapped" || chosen.Model != "gpt-6-luna" {
		t.Fatalf("assign = %q %+v", assignment, chosen)
	}
}

// Row 18.
func TestContinuationChainReadsFilePool(t *testing.T) {
	manager := writePolicyFile(t, "# owner-edited\n[pools.default]\nmembers = [{ model = \""+memLuna+"\" }]\n\n"+rolesAndDefaults())
	var names []string
	for _, s := range manager.ContinuationChain(context.Background(), "owner") {
		names = append(names, s.Provider+"/"+s.Model)
	}
	if strings.Join(names, ",") != memLuna {
		t.Fatalf("chain = %v, want only the file's default pool", names)
	}
}

// Row 19.
func TestRoleChainReadsFilePool(t *testing.T) {
	manager := writePolicyFile(t, "# owner-edited\n[pools.mine]\nmembers = [\""+memIceman+"\", \""+memHaiku+"\"]\n\n[defaults]\nmodel = \"pool:default\"\n\n[roles.verifier_multimodal]\nmodel = \"pool:mine\"\n")
	var names []string
	for _, s := range manager.RoleChain(context.Background(), "owner", "verifier_multimodal") {
		names = append(names, s.Provider+"/"+s.Model)
	}
	if len(names) < 2 || names[0] != memIceman || names[1] != memHaiku {
		t.Fatalf("role chain = %v", names)
	}
}

// Row 7: every reader answers from the file.
func TestEveryReaderUsesTheResolver(t *testing.T) {
	text := "[pools.default]\nmembers = [{ model = \"" + memLuna + "\" }]\n\n[defaults]\nmodel = \"pool:default\"\n\n[roles.texture]\nmodel = \"pool:default\"\n"
	policy := parseOK(t, text)
	if sel := policy.Selections(); len(sel) != 1 || sel[0].Model != "gpt-6-luna" {
		t.Fatalf("Selections = %+v", sel)
	}
	if views := policy.PoolViews(); len(views) == 0 {
		t.Fatal("no pool views")
	}
	if !policy.HasPool("default") || policy.HasPool("nope") {
		t.Fatal("HasPool")
	}
}

// Row 8: no file but pools.go reads the compiled table.
func TestNoDirectCompiledPoolReaders(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	reader := regexp.MustCompile(`catalogPools|modelcatalog\.Pools\(|modelcatalog\.NoFailover\(`)
	for _, file := range files {
		if file == "pools.go" || strings.HasSuffix(file, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(raw), "\n") {
			code, _, _ := strings.Cut(line, "//")
			if reader.MatchString(code) {
				t.Errorf("%s:%d reads the compiled pools outside the resolver: %s", file, i+1, strings.TrimSpace(line))
			}
		}
	}
}

// Row 21.
func TestEnsureDefaultPolicyFileKeepsPoolFile(t *testing.T) {
	text := "[pools.default]\nmembers = [{ model = \"" + memLuna + "\" }]\n\n" + rolesAndDefaults() // no version line, no mark
	manager := writePolicyFile(t, text)
	if _, err := manager.Load(context.Background(), "owner"); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(manager.config.PolicyPath)
	if string(raw) != text {
		t.Fatalf("file rewritten:\n%s", raw)
	}
}

// Row 23: run with -race.
func TestConcurrentPoolReads(t *testing.T) {
	manager := writePolicyFile(t, "# owner-edited\n[pools.default]\nmembers = [{ model = \""+memLuna+"\" }, { model = \""+memIceman+"\", fallback_only = true }]\n\n"+rolesAndDefaults())
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				md := manager.EnrichMetadata(context.Background(), "owner", "texture", map[string]any{"trajectory_id": fmt.Sprintf("t%d-%d", i, j)})
				if metadataString(md, MetadataModel) == "" {
					t.Error("no model")
				}
				manager.PoolViews(context.Background(), "owner")
				manager.ContinuationChain(context.Background(), "owner")
			}
		}(i)
	}
	wg.Wait()
}

// Row 32: provenance.
func TestResolvedPoolsSource(t *testing.T) {
	policy := parseOK(t, "[pools.default]\nmembers = [{ model = \""+memLuna+"\" }]\n\n"+rolesAndDefaults())
	sources := map[string]string{}
	for _, v := range policy.PoolViews() {
		sources[v.Name] = v.Source
	}
	if sources["default"] != PoolSourceFile || sources["selfdev"] != PoolSourceRelease {
		t.Fatalf("sources = %v", sources)
	}
	for _, v := range policy.PoolViews() {
		if v.Name == "default" && (len(v.Members) != 1 || v.Members[0].Model != memLuna || v.Members[0].Weight != 1 || !v.Failover) {
			t.Fatalf("default view = %+v", v)
		}
	}
}
