package agentcore

import (
	"strings"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// What Texture's revise call needs of the runtime to show a published
// version to the desk as data (docs/design/autopaper-step0-contracts-2026-11-10.md,
// contract 1, merge path): the corpusd endpoint, the data frame, and the run
// taint that a read of outside content carries.

// CorpusdURL is the platform endpoint this computer's guest reaches corpusd by.
func (rt *Runtime) CorpusdURL() string {
	if rt == nil {
		return ""
	}
	return strings.TrimSpace(rt.cfg.CorpusdURL)
}

var worldUntrustedLabels = types.NewLabelSet(types.LabelWorldUntrusted)

// FrameWorldUntrusted frames text from outside the computer as data, with the
// frame's closing text defused inside it so the text cannot end the frame early.
func FrameWorldUntrusted(text string) string {
	return frameLabelled(worldUntrustedLabels, strings.ReplaceAll(text, untrustedEnd, "[End of data​]"))
}

// TaintRunWorldUntrusted marks a run as having been shown world_untrusted text.
func (rt *Runtime) TaintRunWorldUntrusted(runID string) {
	rt.taintRun(runID, worldUntrustedLabels)
}
