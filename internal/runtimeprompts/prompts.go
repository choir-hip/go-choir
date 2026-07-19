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

func SuperRuntimeOverlay() string {
	return mustRenderOverlay("super_runtime", nil)
}

func CoSuperRuntimeOverlay() string {
	return mustRenderOverlay("co_super_runtime", nil)
}

func ResearcherRuntimeOverlay() string {
	return mustRenderOverlay("researcher_runtime", nil)
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
