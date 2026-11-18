package yaegikernel

import (
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/searchquery"
)

// choir.SearchAll: all the wordings of ONE question in one call (search-reuse
// commit 3, docs/problems/search-volume-outruns-keenable-credits-2026-11-18.md).
// Fired at once, many wordings cannot inform each other: one story's burst ran
// 214 searches and its first three returned 118 new pages. SearchAll sends the
// wordings one at a time, in order, and counts the new pages among each
// wording's first ten before it sends the next.
//
// A default with its reasons shown, not a cap: every wording not sent is named
// with why, and the desk can still send any wording alone with SearchStart or
// Search.

const (
	// searchAllHeld is the share of a wording's first ten pages that, once
	// held, ends the question: the wording found what earlier ones found.
	searchAllHeldPercent = 80
	// searchAllLook is how many of a wording's pages are judged.
	searchAllLook = 10
	// searchAllEmptyRun is how many wordings in a row may return nothing
	// (a provider answered, with no page); searchAllFailRun how many may
	// fail (no provider answered).
	searchAllEmptyRun = 2
	searchAllFailRun  = 2
)

// SearchAllWording is what became of one wording.
type SearchAllWording struct {
	Wording  string `json:"wording"`
	SearchID string `json:"search_id,omitempty"`
	Sent     bool   `json:"sent"`
	// Why is empty when sent; otherwise the reason it was not.
	Why string `json:"why,omitempty"`
	// Returned and NewPages count the whole search; NewInFirst the new pages
	// among its first ten.
	Returned   int `json:"returned,omitempty"`
	NewPages   int `json:"new_pages,omitempty"`
	NewInFirst int `json:"new_in_first_ten,omitempty"`
}

// SearchAllResult is the merged answer to one question.
type SearchAllResult struct {
	Question string             `json:"question"`
	Wordings []SearchAllWording `json:"wordings"`
	// Next says what to do when wordings were left out because the question
	// already holds unread pages.
	Next string `json:"next,omitempty"`
	// Results are the pages of every search sent, each once, in the order
	// they were found; Held marks a page the desk held before this call.
	Results []SearchHit `json:"results"`
}

// String prints a line per wording and the merged pages deduped by host.
func (r SearchAllResult) String() string {
	var b strings.Builder
	sent, fresh := 0, 0
	for _, w := range r.Wordings {
		if w.Sent {
			sent++
		}
	}
	for _, h := range r.Results {
		if !h.Held {
			fresh++
		}
	}
	fmt.Fprintf(&b, "SearchAll %q: %d of %d wordings sent, %d pages, %d new\n", r.Question, sent, len(r.Wordings), len(r.Results), fresh)
	for i, w := range r.Wordings {
		if !w.Sent && strings.HasPrefix(w.Why, "FAILED") {
			fmt.Fprintf(&b, "%d %q: %s\n", i+1, w.Wording, w.Why)
		} else if w.Sent {
			fmt.Fprintf(&b, "%d %q: sent %s (%d new in first %d, %d of %d pages new)\n", i+1, w.Wording, w.SearchID, w.NewInFirst, min(w.Returned, searchAllLook), w.NewPages, w.Returned)
		} else {
			fmt.Fprintf(&b, "%d %q: not sent (%s)\n", i+1, w.Wording, w.Why)
		}
	}
	if r.Next != "" {
		fmt.Fprintf(&b, "%s\n", r.Next)
	}
	writeHostGroups(&b, groupByHost(r.Results), func(n int) string {
		return fmt.Sprintf("... %d more hosts; read the Results of this value for them\n", n)
	})
	return b.String()
}

// searchAllDeadline is the total time one SearchAll call may spend before it
// names the wordings it has not reached as not sent. Each search already waits
// up to 15 seconds and a research cell has no host deadline, so without a
// bound a call with many wordings and a slow provider holds the cell for
// minutes; 120 seconds is eight full waits. A var so a test can shorten it.
var searchAllDeadline = 120 * time.Second

// SearchAll sends the wordings of one question to the web one at a time and
// returns the merged pages. A wording whose terms equal an earlier search's
// is answered from that search (its pages join the list, marked held). The
// call stops sending when a wording's first ten pages are at least 80% held or
// two wordings in a row return nothing (a provider answered, with no page),
// and when two wordings in a row fail (no provider answered, or the host
// refused): a failure is named with its reason and is never an empty result.
// Each search waits at most 15 seconds, as choir.Search does, and the whole
// call at most searchAllDeadline.
func (s *ChoirScope) SearchAll(question string, wordings []string) (SearchAllResult, error) {
	if s == nil || s.searches == nil {
		return SearchAllResult{}, fmt.Errorf("choir: SearchAll: no searches in this session")
	}
	if len(wordings) == 0 {
		return SearchAllResult{}, fmt.Errorf("choir: SearchAll needs the wordings of the question")
	}
	// SearchAll names a reused wording in its own result, not in a note.
	s.statsMu.Lock()
	s.reuseQuiet = true
	s.statsMu.Unlock()
	defer func() {
		s.statsMu.Lock()
		s.reuseQuiet = false
		s.statsMu.Unlock()
	}()
	res := SearchAllResult{Question: strings.TrimSpace(question), Results: []SearchHit{}}
	qk := questionKey(question)
	s.statsMu.Lock()
	s.allStat.calls++
	s.allStat.offered += len(wordings)
	s.statsMu.Unlock()
	seen := map[string]bool{}
	addPages := func(hits []SearchHit, held bool) {
		for _, h := range hits {
			if key := searchHitKey(h.URL); !seen[key] {
				seen[key] = true
				if held {
					h.Held = true
				}
				res.Results = append(res.Results, h)
			}
		}
	}
	start := time.Now()
	empties, failures, stoppedAfter := 0, 0, 0
	failReason := ""
	stopWhy, stopStat := "", ""
	for i, raw := range wordings {
		w := SearchAllWording{Wording: strings.TrimSpace(raw)}
		notSent := func(why, stat string) {
			w.Why = why
			if stat != "" {
				s.noteNotSent(stat)
			}
			res.Wordings = append(res.Wordings, w)
		}
		switch {
		case w.Wording == "":
			notSent("empty", "")
			continue
		case stopWhy != "":
			notSent(stopWhy, stopStat)
			continue
		case time.Since(start) > searchAllDeadline:
			stopWhy, stopStat = fmt.Sprintf("the call's %d-second deadline passed; send the rest alone or in another call", int(searchAllDeadline.Seconds())), "deadline"
			notSent(stopWhy, stopStat)
			continue
		case !searchquery.HasRealWord(w.Wording):
			notSent("no word of two or more letters or digits", "refused")
			continue
		}
		// Alternative wordings are fallbacks, not multipliers: a question that
		// already holds enough unread pages is read before it is searched again.
		if n := s.searches.unreadNew(qk); qk != "" && n >= searchEnoughUnread {
			stopWhy, stopStat = fmt.Sprintf("this question already holds %d pages new to this run that you have not read", n), "holds_unread"
			res.Next = "Read the ones that matter (the Results of this value; choir.Searches.Summary(id)), then search again for what is still missing, naming it. A page that will not load counts once you have tried it. A wording left out can be sent now with fresh: choir.Search(wording, true)."
			notSent(stopWhy, stopStat)
			continue
		}
		stub, err := s.SearchStart(w.Wording)
		if err != nil {
			failures, empties = failures+1, 0
			failReason = err.Error()
			w.Why = "FAILED (" + failReason + ")"
			res.Wordings = append(res.Wordings, w)
			if failures >= searchAllFailRun {
				stopWhy, stopStat = "search is failing: "+failReason, "failing"
			}
			continue
		}
		if stub.Reused == "" {
			tagged := s.searches.tagQuestion(stub.ID, qk)
			s.statsMu.Lock()
			if !tagged && qk != "" {
				// The gate cannot see this search: counted so a deploy shows it.
				s.allStat.untagged++
			}
			s.allStat.sent++
			s.statsMu.Unlock()
		}
		if stub.Reused != "" {
			// Held: its pages join the list, marked held, with their true count.
			if got, err := s.searches.Get(stub.ID); err == nil {
				addPages(got.Results, true)
				w.Returned = len(got.Results)
			}
			w.SearchID = stub.ID
			notSent(fmt.Sprintf("%s; its %d pages are in the list, marked held; send it alone with fresh",
				strings.TrimSuffix(strings.Replace(stub.Reused, " (same terms); send it anyway with fresh", "", 1), ""), w.Returned), "")
			continue
		}
		if _, err := s.SearchWait(stub.ID, searchWaitMs); err != nil {
			why := "the search wait failed: " + err.Error()
			w.Why = "FAILED (" + why + ")"
			res.Wordings = append(res.Wordings, w)
			for _, rest := range wordings[i+1:] {
				res.Wordings = append(res.Wordings, SearchAllWording{Wording: strings.TrimSpace(rest), Why: why})
				s.noteNotSent("failing")
			}
			return res, err
		}
		got, err := s.searches.Get(stub.ID)
		if err != nil {
			return res, err
		}
		if got.Answered == 0 {
			// No provider answered: a failure, whatever the page count.
			failures, empties = failures+1, 0
			failReason = "no provider answered: " + searchFailureText(got)
			w.SearchID = stub.ID
			w.Why = "FAILED (" + failReason + ")"
			res.Wordings = append(res.Wordings, w)
			if failures >= searchAllFailRun {
				stopWhy, stopStat = "search is failing: "+failReason, "failing"
			}
			continue
		}
		failures = 0
		w.Sent, w.SearchID = true, stub.ID
		w.Returned, w.NewPages = len(got.Results), got.NewPages
		first := got.Results[:min(len(got.Results), searchAllLook)]
		held := 0
		for _, h := range first {
			if h.Held {
				held++
			}
		}
		w.NewInFirst = len(first) - held
		res.Wordings = append(res.Wordings, w)
		addPages(got.Results, false)
		if len(got.Results) == 0 {
			empties++
		} else {
			empties = 0
		}
		if (len(first) > 0 && held*100 >= searchAllHeldPercent*len(first)) || empties >= searchAllEmptyRun {
			stoppedAfter = i + 1
			stopWhy, stopStat = fmt.Sprintf("the question had stopped yielding after wording %d", stoppedAfter), "stopped_yielding"
		}
	}
	return res, nil
}

// searchFailureText is why a search no provider answered says it failed: the
// first provider error, the search's own error, or that none arrived.
func searchFailureText(s Search) string {
	for _, a := range s.Arrivals {
		if a.Error != "" {
			return a.Error
		}
	}
	if s.Error != "" {
		return s.Error
	}
	return "no provider arrived within the wait"
}
