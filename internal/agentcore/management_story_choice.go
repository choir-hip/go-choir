package agentcore

import (
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
// that window and how many it left out, the held stories offered for the
// decision's captures and the verdicts given (none: asked and not answered; -:
// not asked) follow. Ids and counts only.
func managementStoryChoiceLine(runID, choice, storyID string, opening, verb int, d choiceDetail) string {
	if storyID == "" {
		storyID = "-"
	}
	return fmt.Sprintf("management story choice run=%s choice=%s story=%s opening_shown=%d verb_shown=%d candidates_shown=%d window=%s page_in_window=%d page_left_out=%d offered=%s verdicts=%s",
		runID, choice, storyID, opening, verb, opening+verb, shortDuration(d.Window), d.InWindow, d.LeftOut, d.offeredText(), d.verdictsText())
}

// logManagementStoryChoice logs one committed record of a management run that
// went to a story: fed (an existing story's inbox) or opened (texture:new).
// in is the staged act that made the record.
func (rt *Runtime) logManagementStoryChoice(runID, profile string, rec types.InboxRecordReceipt, in yaegikernel.StagedIntent) {
	if !deskmanifest.AdmitsAssignments(profile) {
		return
	}
	switch {
	case rec.OpenedStory:
		d := rt.detailOfDecision(runID, stagedDecision{packet: in.Statement})
		log.Print(managementStoryChoiceLine(runID, "open", "", rt.openingShownCount(runID), rt.storiesShownCount(runID), d))
	case strings.HasPrefix(rec.Inbox, deskmanifest.DocumentCustodianName()+":"):
		d := rt.detailOfDecision(runID, stagedDecision{captures: in.Captures})
		log.Print(managementStoryChoiceLine(runID, "feed", strings.TrimPrefix(rec.Inbox, deskmanifest.DocumentCustodianName()+":"), rt.openingShownCount(runID), rt.storiesShownCount(runID), d))
	}
}
