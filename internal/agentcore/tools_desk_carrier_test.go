package agentcore

import (
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/runtimeprompts"
)

// R3c — management is live on the host desk-cell carrier. The promotion is
// per-profile and unconditional: management's registry seals to a sole
// desk_go_eval, and the prompt overlay switches to the in-cell authority.
// Texture and research are likewise promoted at their own stations.

func TestManagementPromotedToDeskCellCarrier(t *testing.T) {
	if !deskCarrierLive(agentprofile.Management) {
		t.Fatal("management must be live on the desk-cell carrier under R3c")
	}
}

func TestManagementCellRegistryIsSealed(t *testing.T) {
	rt := &Runtime{capsuleExecutor: capsule.NewExecutor(t.TempDir(), t.TempDir(), t.TempDir(), 0)}
	reg, err := buildDeskCellRegistry(rt, agentprofile.Management, researchtools.Dependencies{})
	if err != nil {
		t.Fatalf("build desk cell registry: %v", err)
	}
	// The cell doorway must be present.
	if _, ok := reg.Lookup("desk_go_eval"); !ok {
		t.Fatal("sealed management registry must expose desk_go_eval")
	}
	// R3c keeps the typed lifecycle control surface on the cell registry:
	// report_to_texture (producer report) and cancel_co_super_assignment are
	// the durable control path — orthogonal to the cell-eval seal.
	for _, kept := range []string{"report_to_texture", "cancel_co_super_assignment"} {
		if _, ok := reg.Lookup(kept); !ok {
			t.Fatalf("management cell registry must keep lifecycle control %q", kept)
		}
	}
	// The seal forbids the generic host tools a tool-loop desk would have:
	// no file/exec/research host doorway alongside desk_go_eval.
	for _, banned := range []string{"read_file", "write_file", "exec", "glob", "grep"} {
		if _, ok := reg.Lookup(banned); ok {
			t.Fatalf("carrier registry must not expose host tool %q", banned)
		}
	}
}

func TestResearchCellRegistryIsSealed(t *testing.T) {
	rt := &Runtime{capsuleExecutor: capsule.NewExecutor(t.TempDir(), t.TempDir(), t.TempDir(), 0)}
	reg, err := buildDeskCellRegistry(rt, agentprofile.Research, researchtools.Dependencies{})
	if err != nil {
		t.Fatalf("build research desk cell registry: %v", err)
	}
	if _, ok := reg.Lookup("desk_go_eval"); !ok {
		t.Fatal("sealed research registry must expose desk_go_eval")
	}
	// SR (2026-10-05): the research desk is a full RLM — exactly one tool,
	// desk_go_eval. Every former typed tool is gone; its capability is a
	// choir.* egress verb inside the cell.
	for _, gone := range []string{
		"web_search", "fetch_url", "source_search", "import_url_content",
		"import_document_content", "search_wire_corpus", "read_content_item",
		"list_content_item_selectors", "read_content_item_selector",
		"save_evidence", "read_evidence", "list_evidence", "get_run_memory_entry",
	} {
		if _, ok := reg.Lookup(gone); ok {
			t.Fatalf("sealed research registry must not expose %q — it is a cell verb, not a tool", gone)
		}
	}
	// Generic host tools a tool-loop research desk had must be gone.
	for _, banned := range []string{"read_file", "write_file", "exec", "glob", "grep", "verify_model_capability", "spawn_agent", "cancel_agent", "update_coagent"} {
		if _, ok := reg.Lookup(banned); ok {
			t.Fatalf("carrier registry must not expose host tool %q", banned)
		}
	}
}

func TestPromotedDesksUseCarrier(t *testing.T) {
	for _, profile := range []string{agentprofile.Management, agentprofile.Texture, agentprofile.Research} {
		if !deskCarrierLive(profile) {
			t.Fatalf("%s must be on the carrier", profile)
		}
	}
}

func TestManagementOverlaySwitchesToCellCarrier(t *testing.T) {
	if !deskCarrierLive(agentprofile.Management) {
		t.Skip("management not on carrier")
	}
	// A carrier-live management desk's overlay must name the in-cell
	// authority (desk_go_eval + staged choir.Report), not the retired host
	// report tool. An empty render is the failure mode to catch.
	if overlay := runtimeprompts.RLMManagementOverlay(); overlay == "" {
		t.Fatal("expected the RLM management overlay for the carrier-live desk")
	}
}

func TestResearchOverlaySwitchesToCellCarrier(t *testing.T) {
	if !deskCarrierLive(agentprofile.Research) {
		t.Skip("research not on carrier")
	}
	// A carrier-live research desk's overlay must name the cell doorway and
	// the budgeted typed surface — desk_go_eval plus the typed research
	// tools, never the generic host catalog.
	overlay := runtimeprompts.RLMResearchOverlay()
	if overlay == "" {
		t.Fatal("expected the RLM research overlay for the carrier-live desk")
	}
	for _, want := range []string{"desk_go_eval", "egress", "choir.Report"} {
		if !strings.Contains(overlay, want) {
			t.Fatalf("RLM research overlay missing %q: %q", want, overlay)
		}
	}
}
