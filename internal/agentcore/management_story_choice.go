package agentcore

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The feed-or-open choice, logged (search-reuse commit 4, from
// docs/problems/research-does-not-start-from-the-corpus-2026-11-18.md,
// "Remaining error"): when a management commit sends a record to a story,
// one line names the run, whether it fed an existing story or opened a new
// one, the existing story's id when fed, and how many candidate stories the
// run was shown through choir.Stories. Ids and counts only; no behaviour.

// noteStoriesShown adds the stories one choir.Stories page showed to the run.
func (rt *Runtime) noteStoriesShown(runID string, n int) {
	runID = strings.TrimSpace(runID)
	if rt == nil || runID == "" || n <= 0 {
		return
	}
	for {
		v, loaded := rt.storiesShown.LoadOrStore(runID, n)
		if !loaded {
			return
		}
		if old, _ := v.(int); rt.storiesShown.CompareAndSwap(runID, v, old+n) {
			return
		}
	}
}

func (rt *Runtime) storiesShownCount(runID string) int {
	if v, ok := rt.storiesShown.Load(strings.TrimSpace(runID)); ok {
		n, _ := v.(int)
		return n
	}
	return 0
}

// noteOpeningOffer records what the opening a task was handed carried: the
// page's size and the held stories offered. Building the opening marks
// nothing; the commit calls this.
func (rt *Runtime) noteOpeningOffer(runID string, offer *openingOffer) {
	runID = strings.TrimSpace(runID)
	if rt == nil || runID == "" || offer == nil {
		return
	}
	rt.openingOffers.Store(runID, offer)
}

func (rt *Runtime) openingShownCount(runID string) int {
	if o := rt.openingOfferOf(runID); o != nil {
		return o.Shown
	}
	return 0
}

func (rt *Runtime) forgetStoriesShown(runID string) {
	runID = strings.TrimSpace(runID)
	rt.storiesShown.Delete(runID)
	rt.openingOffers.Delete(runID)
}

// managementStoryChoiceLine: candidates_shown stays the sum of the two, for
// readers of the line from before the split. The page's window, its stories in
// that window and how many it left out, the candidate stories offered for the
// decision's captures and the answers given (none: an offered story with no
// answer, which the host's check refuses for a task opening; -: not asked, as
// for a feed or a decline) follow, then what the main-point judgment adds:
// captures (how many the act disposes of), sources (candidates offered, by
// how they were found), main_point (characters; an open's only) and succeeds
// (the document a successor continues, - for none). Ids and counts only.
func managementStoryChoiceLine(runID, choice, storyID string, opening, verb int, d choiceDetail) string {
	if storyID == "" {
		storyID = "-"
	}
	succeeds := d.Succeeds
	if succeeds == "" {
		succeeds = "-"
	}
	return fmt.Sprintf("management story choice run=%s choice=%s story=%s opening_shown=%d verb_shown=%d candidates_shown=%d window=%s page_in_window=%d page_left_out=%d offered=%s answers=%s captures=%d sources=near:%d,lead:%d main_point=%d succeeds=%s",
		runID, choice, storyID, opening, verb, opening+verb, shortDuration(d.Window), d.InWindow, d.LeftOut, d.offeredText(), d.answersText(),
		d.Captures, d.Sources.Near, d.Sources.Lead, d.MainPoint, succeeds)
}

// logManagementStoryChoice logs one committed record of a management run that
// went to a story: fed (an existing story's inbox), opened (texture:new) or a
// decline (the captures a task leaves out of every story). in is the staged
// act that made the record.
func (rt *Runtime) logManagementStoryChoice(runID, profile string, rec types.InboxRecordReceipt, in yaegikernel.StagedIntent) {
	if !deskmanifest.AdmitsAssignments(profile) {
		return
	}
	switch {
	case rec.OpenedStory:
		d := rt.detailOfDecision(runID, stagedDecision{packet: in.Statement})
		log.Print(managementStoryChoiceLine(runID, "open", strings.TrimPrefix(rec.Inbox, deskmanifest.DocumentCustodianName()+":"), rt.openingShownCount(runID), rt.storiesShownCount(runID), d))
	case in.Kind == yaegikernel.IntentDecline:
		d := rt.detailOfDecision(runID, stagedDecision{captures: in.Captures})
		log.Print(managementStoryChoiceLine(runID, "decline", "", rt.openingShownCount(runID), rt.storiesShownCount(runID), d))
	case strings.HasPrefix(rec.Inbox, deskmanifest.DocumentCustodianName()+":"):
		d := rt.detailOfDecision(runID, stagedDecision{captures: in.Captures})
		log.Print(managementStoryChoiceLine(runID, "feed", strings.TrimPrefix(rec.Inbox, deskmanifest.DocumentCustodianName()+":"), rt.openingShownCount(runID), rt.storiesShownCount(runID), d))
	}
}

// storyResearchLine is the one line each Cast a story's Texture sends to its
// research desk leaves: the story, the round, whether the Cast opened the round
// (each Cast wakes the research reader; an opened round is the start of a
// run) and its cause, decided here at the source: "open" (the story's first
// round), "successor" (its first round, the story naming a succeeds ref),
// "feed" (a later round, which follows a feed or an owner's turn) or "continue"
// (a Cast inside a round already open). The status script counts these.
func storyResearchLine(runID, story string, round int, opened bool, cause string) string {
	return fmt.Sprintf("story research cast run=%s story=%s round=%d opened=%t cause=%s", runID, story, round, opened, cause)
}

// logStoryResearchCast logs a Texture's Cast to its research desk.
func (rt *Runtime) logStoryResearchCast(ctx context.Context, ownerID, computerID, runID, profile string, rec types.InboxRecordReceipt) {
	if strings.TrimSpace(profile) != deskmanifest.DocumentCustodianName() || !deskmanifest.IsResearchInbox(rec.Inbox) {
		return
	}
	doc := strings.TrimPrefix(rec.Inbox, deskmanifest.ResearcherName()+":")
	cause := "continue"
	switch {
	case rec.OpenedRound && rec.Round <= 1:
		cause = "open"
		if rt != nil && rt.store != nil {
			if _, succeeds := rt.store.StoryPointOf(ctx, ownerID, computerID, doc); succeeds != "" {
				cause = "successor"
			}
		}
	case rec.OpenedRound:
		cause = "feed"
	}
	log.Print(storyResearchLine(runID, doc, rec.Round, rec.OpenedRound, cause))
}
