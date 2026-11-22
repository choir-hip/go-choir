package agentcore

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The links of a task's arrivals to the stories that may already hold their
// main point (docs/design/semantic-layer-2026-11-20.md section 3). Two reads,
// each one request for the whole task, neither a model call:
//
//   - vectors: one CaptureNeighbours request for every arrival; each nearest
//     capture is mapped to the story its disposition sent it to (an open or a
//     feed names the story's inbox), so a story repeats once for each of the
//     arrival's neighbours it holds;
//   - search leads: one request naming the stories (conversations) whose
//     research searches returned each arrival's page.
//
// Entities are not read: captures carry no entity links today (the capture
// side of mentions_entity holds no rows), so an entity candidate would be
// title words again. A failed or missing read is named in the returned note and
// the page alone is shown; it never refuses an open.
func (rt *Runtime) candidateLinks(ctx context.Context, ownerID, computerID string, arrivals []types.CaptureArrival) (map[string]arrivalLinks, string) {
	links := map[string]arrivalLinks{}
	var ids []string
	isArrival := map[string]bool{}
	for _, a := range arrivals {
		if id := strings.TrimSpace(a.CaptureID); id != "" && !isArrival[id] {
			isArrival[id] = true
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return links, ""
	}
	var notes []string
	var captures any
	if rt.researchDeps != nil {
		captures = rt.researchDeps.Captures
	}
	if nb, ok := captures.(researchtools.CaptureNeighbourer); ok {
		callCtx, cancel := context.WithTimeout(ctx, candidateNeighboursTimeout)
		ans, err := nb.CaptureNeighbours(callCtx, researchtools.CaptureNeighboursQuery{OwnerID: ownerID, ComputerID: computerID, CaptureIDs: ids,
			Since: time.Now().UTC().Add(-candidateHorizon).Format("2006-01-02"), Limit: candidateNeighbourLimit})
		cancel()
		switch {
		case err != nil:
			notes = append(notes, "the neighbours request failed, so no story is offered by nearest captures")
			log.Printf("management candidates: neighbours failed: %v", err)
		case ans.Unavailable != "":
			notes = append(notes, "nearest captures are unavailable ("+ans.Unavailable+"), so no story is offered by them")
		default:
			storyOf := map[string]string{}
			storyOfCapture := func(id string) string {
				if doc, ok := storyOf[id]; ok {
					return doc
				}
				doc := ""
				if d, found, err := rt.store.CaptureDispositionOf(objectgraph.WithCustodianReader(ctx, "choir.capture_disposition"), ownerID, computerID, id); err == nil && found &&
					(d.Kind == types.DispositionOpen || d.Kind == types.DispositionFeed) && deskmanifest.IsDocumentInbox(d.Inbox) {
					doc = docOfStoryRef(d.Inbox)
				}
				storyOf[id] = doc
				return doc
			}
			for id, list := range ans.Neighbours {
				l := links[id]
				for _, n := range list {
					if n.CaptureID == id || isArrival[n.CaptureID] {
						continue
					}
					if doc := storyOfCapture(n.CaptureID); doc != "" {
						l.Near = append(l.Near, doc)
					}
				}
				links[id] = l
			}
		}
	} else {
		notes = append(notes, "no capture store answers neighbours, so no story is offered by nearest captures")
	}
	if lr, ok := captures.(researchtools.SearchLeadStoryReader); ok {
		callCtx, cancel := context.WithTimeout(ctx, candidateNeighboursTimeout)
		got, err := lr.SearchLeadConversations(callCtx, ownerID, computerID, ids)
		cancel()
		if err != nil {
			notes = append(notes, "the search-lead read failed, so no story is offered by search leads")
			log.Printf("management candidates: search leads failed: %v", err)
		}
		for id, convs := range got {
			l := links[id]
			l.Leads = append(l.Leads, convs...)
			links[id] = l
		}
	}
	return links, strings.Join(notes, "; ")
}
