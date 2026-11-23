package store

import (
	"context"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// attachStoryToResearchCasts gives each Cast the Texture's turn sends to its document's
// research desk the story's own account (types.CastStory): its main point, the text of
// the revision this turn committed (resultRevision) or, when the turn wrote none, the
// head it left unchanged, the document's labels and its predecessor. doc is the result
// document, so a turn that retitles the story sends the new title. The Cast carries the
// committed head whether or not it is published: it says nothing of publication (an
// unpublished head's text travels in the Cast to a research run, and words of it can
// reach a web query: the disclosure table lets owner_private data out through egress,
// internal/disclosure/rules.go, and no owner ruling names this case; the change that added
// this says so in its Security-Effect and in the design note, section 4.2). It is the document
// custodian's act under Texture's authority, atomic with the revision and visible on the
// tape: research never reads the document (objectgraph reader gate X1); it is handed the
// text inside the Cast it answers. A `story` a cell typed into its packet is overwritten,
// never trusted. Other records are returned as they were.
func (s *Store) attachStoryToResearchCasts(ctx context.Context, records []types.CommitmentRecord, doc types.Document, head types.Revision, resultRevision *types.Revision, ownerID, computerID string) []types.CommitmentRecord {
	research := deskmanifest.ResearchInbox(doc.DocID)
	var out []types.CommitmentRecord
	for i, rec := range records {
		if rec.Cast == nil || strings.TrimSpace(rec.Inbox) != research {
			continue
		}
		rev := head
		if resultRevision != nil {
			rev = *resultRevision
		}
		story := &types.CastStory{Title: strings.TrimSpace(doc.Title), RevisionID: strings.TrimSpace(rev.RevisionID), Labels: []string{types.OwnerPrivateLabel("document")}}
		story.Text, story.TextCut = clipStoryText(rev.Content, types.CastStoryTextRunes)
		story.MainPoint, story.Succeeds = s.storyPointOf(ctx, ownerID, computerID, doc.TrajectoryID)
		if succeeds := strings.TrimPrefix(strings.TrimSpace(story.Succeeds), deskmanifest.DocumentCustodianName()+":"); succeeds != "" {
			story.SucceedsMainPoint, _ = s.StoryPointOf(ctx, ownerID, computerID, succeeds)
		}
		if out == nil {
			out = append([]types.CommitmentRecord(nil), records...)
		}
		cast := *rec.Cast
		cast.Story = story
		out[i].Cast = &cast
	}
	if out == nil {
		return records
	}
	return out
}

func clipStoryText(text string, max int) (string, bool) {
	text = strings.TrimSpace(text)
	if r := []rune(text); len(r) > max {
		return string(r[:max]), true
	}
	return text, false
}
