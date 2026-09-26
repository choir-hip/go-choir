package agentcore

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/runtimeprompts"
)

// R3c — management is live on the host desk-cell carrier. The promotion is
// per-profile and unconditional: management's registry seals to a sole
// desk_go_eval, report_to_texture leaves its live surface, and the prompt
// overlay switches to the in-cell authority. Texture and research stay
// behind actuator=rlm until their own stations promote them.

func TestManagementPromotedToDeskCellCarrier(t *testing.T) {
	if !deskCarrierLive(agentprofile.Management) {
		t.Fatal("management must be live on the desk-cell carrier under R3c")
	}
}

func TestManagementCellRegistryIsSealed(t *testing.T) {
	rt := &Runtime{capsuleExecutor: capsule.NewExecutor(t.TempDir(), t.TempDir(), t.TempDir(), 0)}
	reg, err := buildDeskCellRegistry(rt, agentprofile.Management)
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

func TestUnpromotedDesksStayOffCarrier(t *testing.T) {
	// Under actuator=tools (staging default) management (R3c) and texture
	// (R3d) are unconditionally on the cell carrier; research keeps its live
	// host-tool registry until R3r promotes it.
	if capsule.HostSelectsRLM() {
		t.Skip("actuator=rlm promotes all desks; nothing to assert here")
	}
	if !deskCarrierLive(agentprofile.Management) {
		t.Fatal("management must be on the carrier (R3c)")
	}
	if !deskCarrierLive(agentprofile.Texture) {
		t.Fatal("texture must be on the carrier (R3d)")
	}
	if deskCarrierLive(agentprofile.Research) {
		t.Fatal("research must stay off the carrier until R3r promotes it")
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
