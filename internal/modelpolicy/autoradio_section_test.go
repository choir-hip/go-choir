package modelpolicy

import (
	"context"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
)

// A section for the role overrides only what it names (review round 2, 3): it
// must not inherit the Defaults' effort or max_tokens, which would turn
// reasoning on and add a token budget silently.
func TestAutoradioSectionDoesNotInheritTheDefaultsEffortOrBudget(t *testing.T) {
	head := "# owner-edited\n[defaults]\nmodel = \"pool:default\"\nreasoning = \"low\"\nmax_tokens = 4096\n\n[roles.autoradio]\n"
	for name, c := range map[string]struct {
		section string
		effort  string
		max     int
	}{
		"no reasoning line":      {"model = \"pool:autoradio\"\n", ReasoningOff, 0},
		"none means off":         {"model = \"pool:autoradio\"\nreasoning = \"none\"\n", ReasoningOff, 0},
		"off":                    {"model = \"pool:autoradio\"\nreasoning = \"off\"\n", ReasoningOff, 0},
		"explicit low is theirs": {"model = \"pool:autoradio\"\nreasoning = \"low\"\n", "low", 0},
		"explicit budget":        {"model = \"pool:autoradio\"\nmax_tokens = 300\n", ReasoningOff, 300},
	} {
		m := autoradioManager(t, head+c.section)
		pol, _ := m.Load(context.Background(), "owner")
		got := pol.Resolve(AutoradioRole)
		if got.ReasoningEffort != c.effort || got.MaxTokens != c.max {
			t.Errorf("%s: effort %q max_tokens %d, want %q %d", name, got.ReasoningEffort, got.MaxTokens, c.effort, c.max)
		}
		for _, s := range m.PoolChain(context.Background(), "owner", AutoradioRole) {
			if s.ReasoningEffort != c.effort || s.MaxTokens != c.max {
				t.Errorf("%s: chain member %s/%s effort %q max_tokens %d", name, s.Provider, s.Model, s.ReasoningEffort, s.MaxTokens)
			}
		}
	}
	// With no section at all, and the resolve endpoint's path (EnrichMetadata),
	// the real route shows.
	m := autoradioManager(t, defaultPolicyText(provideriface.Config{}))
	meta := m.EnrichMetadata(context.Background(), "owner", AutoradioRole, map[string]any{})
	spec, _ := meta[MetadataSpec].(string)
	if !strings.Contains(spec, "pool:autoradio") || meta[MetadataReasoningEffort] != ReasoningOff {
		t.Fatalf("resolve shows %v", meta)
	}
}
