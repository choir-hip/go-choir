package agentcore

import (
	"fmt"
	"log"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/types"
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

func (rt *Runtime) forgetStoriesShown(runID string) { rt.storiesShown.Delete(strings.TrimSpace(runID)) }

func managementStoryChoiceLine(runID, choice, storyID string, shown int) string {
	if storyID == "" {
		storyID = "-"
	}
	return fmt.Sprintf("management story choice run=%s choice=%s story=%s candidates_shown=%d", runID, choice, storyID, shown)
}

// logManagementStoryChoice logs one committed record of a management run that
// went to a story: fed (an existing story's inbox) or opened (texture:new).
func (rt *Runtime) logManagementStoryChoice(runID, profile string, rec types.InboxRecordReceipt) {
	if !deskmanifest.AdmitsAssignments(profile) {
		return
	}
	switch {
	case rec.OpenedStory:
		log.Print(managementStoryChoiceLine(runID, "open", "", rt.storiesShownCount(runID)))
	case strings.HasPrefix(rec.Inbox, deskmanifest.DocumentCustodianName()+":"):
		log.Print(managementStoryChoiceLine(runID, "feed", strings.TrimPrefix(rec.Inbox, deskmanifest.DocumentCustodianName()+":"), rt.storiesShownCount(runID)))
	}
}
