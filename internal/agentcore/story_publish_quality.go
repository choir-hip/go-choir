package agentcore

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/sources"
	"github.com/yusefmosiah/go-choir/internal/texturedoc"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Publish quality holds (lane Q, docs/problems/autopaper-article-quality-
// 2026-11-09.md). Each is a held condition of AutoPublishRevision: the
// revision stays committed, its version is a gap, and the reason reaches the
// writer in the turn's result as publish_held. A hold is never a drop: the
// next revision that passes publishes as its own number. A hold costs no
// model call; the only loop is the writer revising, which its turn bounds
// already limit.
//
// The length hold is a guard, not a wall: it holds a story's revision once.
// If the story's next revision is still under the floor, the writer was told
// and judged its sources exhausted, so it publishes. "Already held" is not
// kept in memory: it is read from the revision chain (lengthHeldBefore), so
// a restart forgets nothing. The language and process-text holds are not
// one-shot: their cure is always possible.

// MinPublishWords is the fewest words a story's body may have to publish.
// The body is every text block after the leading headline, counted as the
// server counts words (texturedoc.CountWords); the headline is not counted.
// The owner's standard is a full page in general; this is the floor under
// which a revision is a stub, not the target. From 2026-10-27 to 2026-11-09,
// 24% of published stories were under it.
const MinPublishWords = 100

// bodyWords counts the words of the body: every text block except the
// leading headline.
func bodyWords(blocks []texturedoc.TextBlock) int {
	words := 0
	for i, b := range blocks {
		if i == 0 && b.Type == "heading" {
			continue
		}
		words += texturedoc.CountWords(b.Text)
	}
	return words
}

// qualityHold returns the reason a story revision's text is held from
// publishing, or "" when it passes. blocks are the body's text blocks.
// lengthHeldBefore is true when the story was already held for length once,
// so a body still under the floor publishes.
func qualityHold(blocks []texturedoc.TextBlock, version int, lengthHeldBefore bool) string {
	var all []string
	for _, b := range blocks {
		all = append(all, b.Text)
	}
	if line := processTextLine(blocks); line != "" {
		return fmt.Sprintf("it carries process text (%q): a story reads as news for a reader who never sees the work, with no bare Status or Revision lines and no \"In this revision,\" openers, so v%d stays a gap; the next revision without them publishes", line, version)
	}
	if words := bodyWords(blocks); words < MinPublishWords && !lengthHeldBefore {
		return fmt.Sprintf("its body (the text after the headline) has %d words, under the %d-word floor for a story, so v%d stays a gap; expand it from the sources it already cites (what happened, who said it, what it follows from, what is still open). If the sources hold no more, say in one sentence what is not yet known rather than padding: this hold applies once, and your next revision publishes whatever its length", words, MinPublishWords, version)
	}
	if sources.LooksNotEnglish(strings.Join(all, "\n")) {
		return fmt.Sprintf("it is not written in English; the paper publishes in English whatever language its sources are in (quote foreign words only inside English prose), so v%d stays a gap; the next revision in English publishes", version)
	}
	return ""
}

// lengthHeldBefore reports whether this story's previous revision was
// already held for length: it is a story revision (not the seed) that
// passed the conditions before the quality holds, had no process text, and
// had a body under the floor. That is a pure function of the revision chain,
// the durable record of the hold, so no in-memory state or restart matters.
func (rt *Runtime) lengthHeldBefore(ctx context.Context, ownerID, computerID string, v StoryVersion, rev types.Revision) bool {
	parentID := strings.TrimSpace(rev.ParentRevisionID)
	if parentID == "" {
		return false
	}
	parent, err := rt.store.GetLifecycleRevision(withKernelReader(ctx), ownerID, computerID, parentID)
	if err != nil || revisionRole(parent) == types.StorySeedRole {
		return false
	}
	pv := StoryVersion{OwnerID: ownerID, ComputerID: computerID, DocID: v.DocID, RevisionID: parent.RevisionID, Revision: types.RevisionNumber(parent.VersionNumber)}
	if rt.publishPreconditionHold(ctx, ownerID, pv, parent) != "" {
		return false
	}
	blocks := texturedoc.TextBlocks(parent.BodyDoc)
	return processTextLine(blocks) == "" && bodyWords(blocks) < MinPublishWords
}

// processTextPatterns match only the bare process forms the audit found in
// story bodies (docs/problems/autopaper-article-quality-2026-11-09.md, "Process
// text in the body": "**Status.** Revision 4."): a block that is only
// "Status", "Update" or "Revision N" (with an optional "Status." before it),
// or that opens "In this revision," in the writer's first person. "Status:"
// or "Update:" followed by prose is live-blog style and passes, and so does
// "In this revision of the treaty".
var processTextPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^\W*(status|update)\W*$`),
	regexp.MustCompile(`(?i)^\W*(status\W+)?revision\s+\d+\W*$`),
	regexp.MustCompile(`(?i)^\W*in this revision,`),
}

func processTextLine(blocks []texturedoc.TextBlock) string {
	for _, b := range blocks {
		if b.Type == "heading" {
			continue
		}
		for _, re := range processTextPatterns {
			if re.MatchString(b.Text) {
				line := b.Text
				if r := []rune(line); len(r) > 60 {
					line = string(r[:60]) + "..."
				}
				return line
			}
		}
	}
	return ""
}
