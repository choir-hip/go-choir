package modelpolicy

// The autoradio_programme role (station L1, failure list item F39,
// docs/design/autoradio-l1-failure-list-2026-11-15.md). It is the role of the
// programme call: the same pool as the fast turn (River, then Bedrock Haiku
// 5.5), with reasoning ON, fixed per role (owner 2026-11-11), on its own prompt
// prefix.

import (
	"context"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
)

func TestF39_ProgrammeRoleNormalizesAsABareRole(t *testing.T) {
	for _, in := range []string{"autoradio_programme", " Autoradio_Programme "} {
		if got := NormalizeRole(in); got != AutoradioProgrammeRole {
			t.Fatalf("NormalizeRole(%q) = %q", in, got)
		}
	}
	if AutoradioProgrammeRole != "autoradio_programme" {
		t.Fatalf("role name %q", AutoradioProgrammeRole)
	}
	if got := NormalizeRole("autoradio"); got != AutoradioRole {
		t.Fatalf("the fast turn role changed: %q", got)
	}
}

func TestF39_ProgrammeRoleResolvesToTheAutoradioPoolWithReasoningOn(t *testing.T) {
	for name, text := range map[string]string{
		"a current file with no section": defaultPolicyText(provideriface.Config{}),
		// A file that predates the role (an owner-edited one names no section):
		// the programme must not fall to the default pool, whose first members
		// are slow.
		"an owner-edited file with no section": "# owner-edited\n[defaults]\nmodel = \"pool:default\"\nreasoning = \"high\"\nmax_tokens = 4096\n",
	} {
		m := autoradioManager(t, text)
		got := chainOf(m, AutoradioProgrammeRole)
		want := []string{memRiver, "bedrock/us.anthropic.claude-haiku-5-5"}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("%s: chain %v, want %v", name, got, want)
		}
		for _, s := range m.PoolChain(context.Background(), "owner", AutoradioProgrammeRole) {
			if s.ReasoningEffort != "low" || s.MaxTokens != 0 {
				t.Fatalf("%s: %s/%s effort %q max_tokens %d, want low and no budget", name, s.Provider, s.Model, s.ReasoningEffort, s.MaxTokens)
			}
		}
	}
}

func TestF39_ProgrammeRoleSectionOverridesOnlyWhatItNames(t *testing.T) {
	head := "# owner-edited\n[defaults]\nmodel = \"pool:default\"\nreasoning = \"high\"\nmax_tokens = 4096\n\n[roles.autoradio_programme]\n"
	m := autoradioManager(t, head+"model = \"pool:synthetic-luna\"\n")
	pol, _ := m.Load(context.Background(), "owner")
	if len(pol.Diagnostics) != 0 {
		t.Fatalf("the section drew diagnostics: %v", pol.Diagnostics)
	}
	got := chainOf(m, AutoradioProgrammeRole)
	if len(got) != 2 || got[0] != memSynthetic {
		t.Fatalf("override chain %v", got)
	}
	for _, s := range m.PoolChain(context.Background(), "owner", AutoradioProgrammeRole) {
		if s.ReasoningEffort != "low" || s.MaxTokens != 0 {
			t.Fatalf("a section with no reasoning line inherited the defaults: %+v", s)
		}
	}
	m = autoradioManager(t, head+"model = \"pool:autoradio\"\nreasoning = \"medium\"\n")
	if s := m.PoolChain(context.Background(), "owner", AutoradioProgrammeRole)[0]; s.ReasoningEffort != "medium" {
		t.Fatalf("an explicit reasoning line is the owner's: %+v", s)
	}
}

func TestF39_ProgrammeRoleNeverHoldsOffAndTheFastRoleStillDoes(t *testing.T) {
	m := autoradioManager(t, "# owner-edited\n[defaults]\nmodel = \"pool:default\"\nreasoning = \"low\"\n\n[roles.autoradio_programme]\nmodel = \"pool:autoradio\"\nreasoning = \"off\"\n\n[roles.autoradio]\nmodel = \"pool:autoradio\"\nreasoning = \"off\"\n")
	pol, _ := m.Load(context.Background(), "owner")
	if got := pol.Resolve(AutoradioProgrammeRole).ReasoningEffort; got == ReasoningOff {
		t.Fatalf("the programme role holds %q: reasoning off is the fast turn's alone", got)
	}
	if len(pol.Diagnostics) == 0 {
		t.Fatal("a dropped off value left no diagnostic")
	}
	if got := pol.Resolve(AutoradioRole).ReasoningEffort; got != ReasoningOff {
		t.Fatalf("the fast turn role lost its off: %q", got)
	}
	for _, s := range pol.Selections() {
		if s.ReasoningEffort == ReasoningOff {
			t.Fatalf("Selections carries off: %+v", s)
		}
	}
}

func TestF39_TheFastRoleKeepsItsPoolAndOff(t *testing.T) {
	m := autoradioManager(t, defaultPolicyText(provideriface.Config{}))
	for _, s := range m.PoolChain(context.Background(), "owner", AutoradioRole) {
		if s.ReasoningEffort != ReasoningOff {
			t.Fatalf("%s/%s effort %q", s.Provider, s.Model, s.ReasoningEffort)
		}
	}
}
