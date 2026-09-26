package runtimeprompts

import (
	"embed"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/promptspec"
)

//go:embed overlays/*.yaml
var overlayFS embed.FS

// TemporalContextOptions carries dynamic run-time grounding for all roles.
type TemporalContextOptions struct {
	NowUTC string
}

// ConductorRunOptions carries per-run conductor routing context.
type ConductorRunOptions struct {
	RequestedApp string
	SeedPrompt   string
}

// RunContextOptions carries per-run agent coordination identifiers.
type RunContextOptions struct {
	AgentID                string
	RequesterAgentID       string
	TextureDeliveryAgentID string
	ChannelID              string
	// InCellCarrier is true when the run's desk lives on the in-cell carrier:
	// peer coordination is choir.Message, not a JSON tool.
	InCellCarrier bool
	// NoReportChannel is true when the desk has no peer-coordination tool at
	// all (tools-actuator assigned Engineering): the prompt must not name one.
	NoReportChannel bool
}

func TemporalContext(opts TemporalContextOptions) string {
	return mustRenderOverlay("temporal_context", opts)
}

func ConductorRunOverlay(opts ConductorRunOptions) string {
	return mustRenderOverlay("conductor_run", opts)
}

func ProcessorRuntimeOverlay() string {
	return mustRenderOverlay("processor_runtime", nil)
}

func ReconcilerRuntimeOverlay() string {
	return mustRenderOverlay("reconciler_runtime", nil)
}

func ManagementRuntimeOverlay() string {
	return mustRenderOverlay("management_runtime", nil)
}

// RLMManagementOverlay is the sealed-Go variant served when the management
// desk is live on the host cell carrier (R3c): desk_go_eval is the sole JSON
// doorway, choir verbs stage acts, and report_to_texture is replaced by the
// staged Report/ReportPacket intent. The legacy catalog text is replaced,
// not amended.
func RLMManagementOverlay() string {
	return mustRenderOverlay("rlm_management_runtime", nil)
}

func EngineeringRuntimeOverlay() string {
	return mustRenderOverlay("engineering_runtime", nil)
}

// RLMEngineeringOverlay is the sealed-Go variant served when actuator=rlm:
// capsule_go_eval is the sole capsule doorway and the choir package subsumes
// the JSON file/exec tools. The legacy catalog sentence is replaced, not
// amended, so the model never sees two authorities.
type RLMEngineeringOverlayOptions struct {
	HasSelfDevelopmentOperation bool
}

func RLMEngineeringOverlay(opts RLMEngineeringOverlayOptions) string {
	return mustRenderOverlay("rlm_engineering_runtime", opts)
}

func ResearchRuntimeOverlay() string {
	return mustRenderOverlay("research_runtime", nil)
}

// RLMResearchOverlay is the sealed-Go variant served when the research desk
// is live on the host cell carrier (R3r): desk_go_eval is the cell doorway,
// the typed research surface stays for host-mediated world access, and the
// egress budget names its own backpressure. The legacy catalog text is
// replaced, not amended.
func RLMResearchOverlay() string {
	return mustRenderOverlay("rlm_research_runtime", nil)
}

func RunContextOverlay(opts RunContextOptions) string {
	return mustRenderOverlay("run_context", opts)
}

func mustRenderOverlay(name string, data any) string {
	raw, err := overlayFS.ReadFile("overlays/" + name + ".yaml")
	if err != nil {
		panic(fmt.Sprintf("runtime overlay %s: %v", name, err))
	}
	out, err := promptspec.ParseAndRender(raw, data)
	if err != nil {
		panic(fmt.Sprintf("runtime overlay %s: %v", name, err))
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return ""
	}
	return "\n\n" + out
}
