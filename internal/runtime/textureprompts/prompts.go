package textureprompts

import (
	"embed"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/runtime/promptspec"
)

//go:embed texture.yaml
var defaultSystemPromptYAML []byte

//go:embed overlays/*.yaml
var overlayFS embed.FS

// DefaultSystemPrompt returns the seeded Texture role system prompt.
func DefaultSystemPrompt() string {
	doc, err := promptspec.Parse(defaultSystemPromptYAML)
	if err != nil {
		panic("texture default prompt yaml: " + err.Error())
	}
	return doc.BodyText()
}

// RunOverlay returns the per-run Texture system overlay appended after the role prompt.
func RunOverlay() string {
	return mustRenderOverlay("run_system", nil)
}

// RevisionWorkerFindingsOptions selects worker-findings overlay text for revision requests.
type RevisionWorkerFindingsOptions struct {
	IntegrateWorkerFindings bool
	NeedsSuperExecution     bool
	HasSuperDelivery        bool
	ActiveWorkerDelegation  bool
}

// RevisionWorkerFindingsOverlay returns worker-message policy appended to revision requests.
func RevisionWorkerFindingsOverlay(opts RevisionWorkerFindingsOptions) string {
	return mustRenderOverlay("revision_worker_findings", opts)
}

// RevisionMediaSourceResearchRequired returns policy when new media sources need research.
func RevisionMediaSourceResearchRequired() string {
	return mustRenderOverlay("revision_media_source_research_required", nil)
}

// RevisionSourceEntitiesIntro returns the static intro for detected source entities.
func RevisionSourceEntitiesIntro() string {
	return mustRenderOverlay("revision_source_entities_intro", nil)
}

// RevisionPolicyOptions selects revision-request policy overlay text.
type RevisionPolicyOptions struct {
	OwnerPromptRequestRevision bool
	UserAuthoredRevision       bool
	ExplicitResearcherRequest  bool
	HasGroundedHistory         bool
	DocID                      string
	RevisionID                 string
}

// RevisionPolicyOverlay returns the policy tail for a Texture revision request.
func RevisionPolicyOverlay(opts RevisionPolicyOptions) string {
	return mustRenderOverlay("revision_policy", opts)
}

func mustRenderOverlay(name string, data any) string {
	raw, err := overlayFS.ReadFile("overlays/" + name + ".yaml")
	if err != nil {
		panic(fmt.Sprintf("texture overlay %s: %v", name, err))
	}
	out, err := promptspec.ParseAndRender(raw, data)
	if err != nil {
		panic(fmt.Sprintf("texture overlay %s: %v", name, err))
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return ""
	}
	if name == "run_system" {
		return "\n\n" + out
	}
	return "\n" + out
}
