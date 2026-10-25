// Package textureprompts holds Texture's prompt core. The per-run overlays
// (run_system, revision_policy, revision_execution_findings,
// revision_source_entities_intro) are deleted with ApplyTexture (Cut 2 2a):
// the frame carries the document and records each turn, and the runtime
// appends the generated verb list.
package textureprompts

import (
	_ "embed"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/promptspec"
)

//go:embed texture.yaml
var defaultSystemPromptYAML []byte

//go:embed texture_root.txt
var rootSection string

//go:embed texture_root_context.txt
var rootContextLabel string

// DefaultSystemPrompt returns Texture's prompt core.
func DefaultSystemPrompt() string {
	doc, err := promptspec.Parse(defaultSystemPromptYAML)
	if err != nil {
		panic("texture default prompt yaml: " + err.Error())
	}
	return doc.BodyText()
}

// RootSection is the Texture root's instructions (PD3 S1,
// docs/definitions/choir-pd3-texture-root-desk-2026-10-11.md). It is not part
// of the prompt core: the run of the document the texture_root marker names
// carries it in its request turn, with the ledger.
func RootSection() string {
	return strings.TrimSpace(rootSection)
}

// RootContextLabel heads the master document's excerpt in the request turn
// of every other document's Texture run (PD3 S2). Like the root section it
// is not part of the prompt core.
func RootContextLabel() string {
	return strings.TrimSpace(rootContextLabel)
}
