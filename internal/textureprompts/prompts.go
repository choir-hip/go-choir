// Package textureprompts holds Texture's prompt core. The per-run overlays
// (run_system, revision_policy, revision_execution_findings,
// revision_source_entities_intro) are deleted with ApplyTexture (Cut 2 2a):
// the frame carries the document and records each turn, and the runtime
// appends the generated verb list.
package textureprompts

import (
	_ "embed"

	"github.com/yusefmosiah/go-choir/internal/promptspec"
)

//go:embed texture.yaml
var defaultSystemPromptYAML []byte

// DefaultSystemPrompt returns Texture's prompt core.
func DefaultSystemPrompt() string {
	doc, err := promptspec.Parse(defaultSystemPromptYAML)
	if err != nil {
		panic("texture default prompt yaml: " + err.Error())
	}
	return doc.BodyText()
}
