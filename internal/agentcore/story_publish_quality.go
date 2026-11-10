package agentcore

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/sources"
	"github.com/yusefmosiah/go-choir/internal/texturedoc"
)

// Publish quality holds (lane Q, docs/problems/autopaper-article-quality-
// 2026-11-09.md). Each is a held condition of AutoPublishRevision: the
// revision stays committed, its version is a gap, and the reason reaches the
// writer in the turn's result as publish_held. A hold is never a drop: the
// next revision that passes publishes as its own number. A hold costs no
// model call; the only loop is the writer revising, which its turn bounds
// already limit.

// MinPublishWords is the fewest words a story's body may have to publish,
// counted as the server counts them (texturedoc.CountWords). The owner's
// standard is a full page in general; this is the floor under which a
// revision is a stub, not the target. From 2026-10-27 to 2026-11-09, 24% of
// published stories were under it.
const MinPublishWords = 100

// qualityHold returns the reason a story revision's text is held from
// publishing, or "" when it passes. blocks are the body's text blocks.
func qualityHold(blocks []texturedoc.TextBlock, version int) string {
	var all []string
	words := 0
	for _, b := range blocks {
		all = append(all, b.Text)
		words += texturedoc.CountWords(b.Text)
	}
	if line := processTextLine(blocks); line != "" {
		return fmt.Sprintf("it carries process text (%q): a story reads as news for a reader who never sees the work, with no Status, Revision, Update or In-this-revision lines, so v%d stays a gap; the next revision without them publishes", line, version)
	}
	if words < MinPublishWords {
		return fmt.Sprintf("its body has %d words, under the %d-word floor for a story, so v%d stays a gap; expand it from the sources it already cites (what happened, who said it, what it follows from, what is still open) and the next revision that clears the floor publishes", words, MinPublishWords, version)
	}
	if sources.LooksNotEnglish(strings.Join(all, "\n")) {
		return fmt.Sprintf("it is not written in English; the paper publishes in English whatever language its sources are in (quote foreign words only inside English prose), so v%d stays a gap; the next revision in English publishes", version)
	}
	return ""
}

// processTextPattern matches a block that opens with process text the writer
// prompt forbids in a story (texture.yaml: no "Status", "Revision N",
// "Update:" or "In this revision" lines).
var processTextPattern = regexp.MustCompile(`(?i)^\W*(status\s*[:.\-]|revision\s+\d+\b|update\s*:|in this revision\b)`)

func processTextLine(blocks []texturedoc.TextBlock) string {
	for _, b := range blocks {
		if b.Type == "heading" {
			continue
		}
		if processTextPattern.MatchString(b.Text) {
			line := b.Text
			if r := []rune(line); len(r) > 60 {
				line = string(r[:60]) + "..."
			}
			return line
		}
	}
	return ""
}
