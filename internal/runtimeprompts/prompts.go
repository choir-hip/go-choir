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

// RunContextOptions carries per-run agent coordination identifiers. They
// belong in the run-context turn, not the system prompt: the system prompt
// is the stable prefix every run of a desk shares with the provider's
// prompt cache.
type RunContextOptions struct {
	AgentID                string
	RequesterAgentID       string
	TextureDeliveryAgentID string
	ChannelID              string
}

// PeerCoordinationOptions carries the desk's static coordination default.
type PeerCoordinationOptions struct {
	// InCellCarrier is true when the run's desk lives on the in-cell carrier:
	// peer coordination is a semantic act (choir.ReportPacket, choir.Note,
	// choir.Cast, choir.Ask), not a JSON tool.
	InCellCarrier bool
}

func TemporalContext(opts TemporalContextOptions) string {
	return mustRenderOverlay("temporal_context", opts)
}

func ConductorRunOverlay(opts ConductorRunOptions) string {
	return mustRenderOverlay("conductor_run", opts)
}

// RLMManagementOverlay is the sealed-Go variant served when the management
// desk is live on the host cell carrier (R3c): desk_go_eval is the sole JSON
// doorway, choir verbs stage acts, and report_to_texture is replaced by the
// staged Report/ReportPacket intent. The legacy catalog text is replaced,
// not amended.
func RLMManagementOverlay(opts ...RLMManagementOverlayOptions) string {
	var o RLMManagementOverlayOptions
	if len(opts) > 0 {
		o = opts[0]
	}
	return mustRenderOverlay("rlm_management_runtime", o)
}

// RLMManagementOverlayOptions carries the computer's settings that change
// management's prompt.
type RLMManagementOverlayOptions struct {
	// IntakeLabels is the computer's intake.labels setting (L2a A6): only
	// when on does management read about Jev's labels, choir.Feed,
	// choir.Decline and choir.Stories.
	IntakeLabels bool
	// EventWakes is the computer's intake.event_wakes setting (event
	// layer §15): captures reach management on their events' records.
	EventWakes bool
}

// RLMManagementTaskOverlay is a management task's overlay (management
// delegation S2a): its job, its records, how it disposes of captures, and
// how it ends.
func RLMManagementTaskOverlay(opts RLMManagementTaskOverlayOptions) string {
	return mustRenderOverlay("rlm_management_task_runtime", opts)
}

// RLMManagementTaskOverlayOptions carries the task overlay's static
// settings. The task id and capture count are per run and go in the
// run-context turn.
type RLMManagementTaskOverlayOptions struct {
	Deadline string
}

// RLMEngineeringOverlay is the sealed-Go Engineering overlay:
// capsule_go_eval is the sole capsule doorway and the choir package subsumes
// JSON file and exec operations so the model sees one authority.
type RLMEngineeringOverlayOptions struct {
	HasSelfDevelopmentOperation bool
	// CanProposeChange: no operation is bound yet, but the computer's signed
	// mode authorizes proposals, so a freeze opens the candidate (Gate 2 D).
	CanProposeChange bool
	// ClaudeDelegate: this assignment's capsule is in the claude mode, so the
	// Claude delegate is available to it
	// (docs/design/claude-delegate-slice1-2026-10-14.md).
	ClaudeDelegate bool
}

func RLMEngineeringOverlay(opts RLMEngineeringOverlayOptions) string {
	return mustRenderOverlay("rlm_engineering_runtime", opts)
}

// RLMResearchOverlay is the sealed-Go surface for the research desk (SR
// 2026-10-05): desk_go_eval is the sole tool — the typed surface and the
// legacy research_runtime overlay are deleted. The egress budget names its
// own backpressure.
func RLMResearchOverlay() string {
	return mustRenderOverlay("rlm_research_runtime", nil)
}

func RunContextOverlay(opts RunContextOptions) string {
	return mustRenderOverlay("run_context", opts)
}

// PeerCoordination is the desk's peer coordination default, part of the
// stable system prompt.
func PeerCoordination(opts PeerCoordinationOptions) string {
	return mustRenderOverlay("peer_coordination", opts)
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
