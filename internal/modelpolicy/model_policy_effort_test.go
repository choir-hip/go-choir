package modelpolicy

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
)

const effortPolicyHead = "[defaults]\nmodel = \"pool:synthetic-luna\"\nreasoning = \"low\"\n\n"

// Problem 15: an inline comment after a value became part of the value.
func TestPolicyInlineCommentIsNotTheValue(t *testing.T) {
	policy, err := parsePolicy(effortPolicyHead+"[roles.engineering] # engineering desk\nmodel = \"pool:synthetic-luna\" # pool\nreasoning = \"high\"  # owner-edited: raised (2026-11-03)\n", "test")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got := policy.Resolve(agentprofile.Engineering)
	if got.ReasoningEffort != "high" || got.Model != "pool:synthetic-luna" || len(policy.Diagnostics) != 0 {
		t.Fatalf("selection = %+v diagnostics = %v", got, policy.Diagnostics)
	}
}

func TestPolicyHashInsideQuotedStringIsKept(t *testing.T) {
	for in, want := range map[string]string{
		`a = "x#y" # c`:       "x#y",
		`a = 'x#y' # c`:       "x#y",
		`a = "q\"#z" # c`:     `q\"#z`,
		`a = "plain"#nospace`: "plain",
	} {
		_, v, ok := parseAssignment(strings.TrimSpace(stripTOMLComment(in)))
		if !ok || v != want {
			t.Fatalf("%s: value %q ok=%v, want %q", in, v, ok, want)
		}
	}
}

func TestPolicyUnknownEffortFallsBackToDefaultWithDiagnostic(t *testing.T) {
	policy, err := parsePolicy(effortPolicyHead+"[roles.engineering]\nmodel = \"pool:synthetic-luna\"\nreasoning = \"hig\"\n", "test")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := policy.Resolve(agentprofile.Engineering).ReasoningEffort; got != "low" {
		t.Fatalf("engineering effort = %q, want the default low", got)
	}
	if len(policy.Diagnostics) != 1 || !strings.Contains(policy.Diagnostics[0], `"hig"`) {
		t.Fatalf("diagnostics = %v", policy.Diagnostics)
	}
}

func TestPolicyEmptyEffortIsUnset(t *testing.T) {
	policy, err := parsePolicy(effortPolicyHead+"[roles.engineering]\nmodel = \"pool:synthetic-luna\"\nreasoning = \"\" # none yet\n", "test")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := policy.Resolve(agentprofile.Engineering).ReasoningEffort; got != "low" || len(policy.Diagnostics) != 0 {
		t.Fatalf("effort = %q diagnostics = %v", got, policy.Diagnostics)
	}
}

func TestPolicyAcceptsTheProviderEffortSet(t *testing.T) {
	for _, e := range []string{"none", "minimal", "low", "medium", "high", "xhigh", "max"} {
		policy, err := parsePolicy(effortPolicyHead+"[roles.engineering]\nmodel = \"pool:synthetic-luna\"\nreasoning = \""+e+"\"\n", "test")
		if err != nil || policy.Resolve(agentprofile.Engineering).ReasoningEffort != e || len(policy.Diagnostics) != 0 {
			t.Fatalf("%s: %+v %v", e, policy, err)
		}
	}
}

func TestEnrichMetadataReportsUnknownEffort(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model-policy.toml")
	text := "# owner-edited\n" + effortPolicyHead + "[roles.engineering]\nmodel = \"pool:synthetic-luna\"\nreasoning = \"turbo\"\n"
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	m := NewManager(ManagerConfig{PolicyPath: path})
	md := m.EnrichMetadata(context.Background(), "owner", agentprofile.Engineering, nil)
	if md[MetadataReasoningEffort] != "low" {
		t.Fatalf("effort = %v", md[MetadataReasoningEffort])
	}
	if s, _ := md[MetadataPolicyError].(string); !strings.Contains(s, "turbo") {
		t.Fatalf("policy error = %v", md[MetadataPolicyError])
	}
}
