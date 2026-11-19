package agentcore

import (
	"encoding/json"
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

// noteOpeningStoriesShown counts the stories of the page a task's opening
// delivered, which the host puts in the first message and which no verb call
// counts. An unreadable page counts nothing.
func (rt *Runtime) noteOpeningStoriesShown(runID string, page json.RawMessage) {
	runID = strings.TrimSpace(runID)
	var p yaegikernel.StoriesPage
	if rt == nil || runID == "" || json.Unmarshal(page, &p) != nil || len(p.Stories) == 0 {
		return
	}
	rt.openingShown.Store(runID, len(p.Stories))
}

func (rt *Runtime) openingShownCount(runID string) int {
	if v, ok := rt.openingShown.Load(strings.TrimSpace(runID)); ok {
		n, _ := v.(int)
		return n
	}
	return 0
}

func (rt *Runtime) forgetStoriesShown(runID string) {
	runID = strings.TrimSpace(runID)
	rt.storiesShown.Delete(runID)
	rt.openingShown.Delete(runID)
}

// managementStoryChoiceLine: candidates_shown stays the sum of the two, for
// readers of the line from before the split.
func managementStoryChoiceLine(runID, choice, storyID string, opening, verb int) string {
	if storyID == "" {
		storyID = "-"
	}
	return fmt.Sprintf("management story choice run=%s choice=%s story=%s opening_shown=%d verb_shown=%d candidates_shown=%d", runID, choice, storyID, opening, verb, opening+verb)
}

// logManagementStoryChoice logs one committed record of a management run that
// went to a story: fed (an existing story's inbox) or opened (texture:new).
func (rt *Runtime) logManagementStoryChoice(runID, profile string, rec types.InboxRecordReceipt) {
	if !deskmanifest.AdmitsAssignments(profile) {
		return
	}
	switch {
	case rec.OpenedStory:
		log.Print(managementStoryChoiceLine(runID, "open", "", rt.openingShownCount(runID), rt.storiesShownCount(runID)))
	case strings.HasPrefix(rec.Inbox, deskmanifest.DocumentCustodianName()+":"):
		log.Print(managementStoryChoiceLine(runID, "feed", strings.TrimPrefix(rec.Inbox, deskmanifest.DocumentCustodianName()+":"), rt.openingShownCount(runID), rt.storiesShownCount(runID)))
	}
}
