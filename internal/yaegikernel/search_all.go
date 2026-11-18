package yaegikernel

import (
	"fmt"
	"strings"

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
	// searchAllEmptyRun is how many wordings in a row may return nothing.
	searchAllEmptyRun = 2
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
		if w.Sent {
			fmt.Fprintf(&b, "%d %q: sent %s (%d new in first %d, %d of %d pages new)\n", i+1, w.Wording, w.SearchID, w.NewInFirst, min(w.Returned, searchAllLook), w.NewPages, w.Returned)
		} else {
			fmt.Fprintf(&b, "%d %q: not sent (%s)\n", i+1, w.Wording, w.Why)
		}
	}
	writeHostGroups(&b, groupByHost(r.Results), func(n int) string {
		return fmt.Sprintf("... %d more hosts; read the Results of this value for them\n", n)
	})
	return b.String()
}

// SearchAll sends the wordings of one question to the web one at a time and
// returns the merged pages. It drops a wording whose terms an earlier wording
// covers (searchquery.Covers), answers one it already holds from the held
// search, and stops sending when a wording's first ten pages are at least 80%
// held or two wordings in a row return nothing. Each search waits at most
// 15 seconds, as choir.Search does.
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
	seen := map[string]bool{}
	empties, stoppedAfter := 0, 0
	for i, raw := range wordings {
		w := SearchAllWording{Wording: strings.TrimSpace(raw)}
		note := func(why, stat string) {
			w.Why = why
			if stat != "" {
				s.noteNotSent(stat)
			}
			res.Wordings = append(res.Wordings, w)
		}
		switch {
		case w.Wording == "":
			note("empty", "")
			continue
		case stoppedAfter > 0:
			note(fmt.Sprintf("the question had stopped yielding after wording %d", stoppedAfter), "stopped_yielding")
			continue
		case !searchquery.HasRealWord(w.Wording):
			note("no word of two or more letters or digits", "refused")
			continue
		}
		if by := coveredBy(wordings[:i], w.Wording); by > 0 {
			note(fmt.Sprintf("covered by wording %d", by), "covered")
			continue
		}
		stub, err := s.SearchStart(w.Wording)
		if err != nil {
			note("refused: "+err.Error(), "")
			continue
		}
		if stub.Reused != "" {
			note(strings.Replace(stub.Reused, " (same terms)", "", 1), "")
			continue
		}
		if _, err := s.SearchWait(stub.ID, searchWaitMs); err != nil {
			return res, err
		}
		got, err := s.searches.Get(stub.ID)
		if err != nil {
			return res, err
		}
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
		for _, h := range got.Results {
			if key := searchHitKey(h.URL); !seen[key] {
				seen[key] = true
				res.Results = append(res.Results, h)
			}
		}
		if len(got.Results) == 0 {
			empties++
		} else {
			empties = 0
		}
		if (len(first) > 0 && held*100 >= searchAllHeldPercent*len(first)) || empties >= searchAllEmptyRun {
			stoppedAfter = i + 1
		}
	}
	return res, nil
}

// coveredBy returns the 1-based index of the first earlier wording that
// covers later, or 0.
func coveredBy(earlier []string, later string) int {
	for j, e := range earlier {
		if searchquery.HasRealWord(e) && searchquery.Covers(e, later) {
			return j + 1
		}
	}
	return 0
}
