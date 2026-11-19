package yaegikernel

import (
	"fmt"

	"github.com/yusefmosiah/go-choir/internal/searchquery"
)

// Search for what is lacking, not for what is new (docs/problems/
// search-volume-outruns-keenable-credits-2026-11-18.md, "Why a run searches
// again"). After search reuse deployed, 83% of the pages a run's searches
// returned were new to it, so "stop when wordings stop finding new pages" almost
// never stopped, and the runs read under 5% of the pages they were handed. What
// a question lacks is known only to a desk that has read what it holds.

// searchEnoughUnread is how many pages new to the run, returned for one
// question and not yet read, mean the question already holds more than the run
// will read: the runs of 2026-11-18 read about 19 pages a run in all, of every
// 40 returned 1 to 2, and a search returns up to 40. 15 is nearly a whole run's
// reading, spent on one question. It is the line at which a further wording of
// that question is reported to the desk instead of sent, not a cap: the desk
// reads, and then it can search again; fresh sends a wording any time.
const searchEnoughUnread = 15

// allCounts are a cell's SearchAll counts (SearchStats.AllCalls and the rest).
type allCounts struct {
	calls, offered, sent, unreadNotes, untagged int
}

// tagQuestion records that search id was sent for the question with this key.
// It reports whether it did: false when the store does not hold the search
// (its start frame had not arrived), and then the gate cannot see that search.
func (st *SearchStore) tagQuestion(id, qk string) bool {
	if st == nil || qk == "" {
		return false
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	s := st.searches[id]
	if s == nil {
		return false
	}
	s.question = qk
	return true
}

// unreadNew counts the pages of this run's searches that were new to the run
// when they came and that the run has not yet tried to read, each once. A page
// that will not load (a dead link, a 403, a paywall) counts once it was tried. With a question
// key only the searches sent for that question count; with none, every search
// of the run does.
func (st *SearchStore) unreadNew(qk string) int {
	if st == nil {
		return 0
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	var tried map[string]bool
	if st.triedKeys != nil {
		tried = st.triedKeys()
	}
	read := st.readSet()
	seen := map[string]bool{}
	for i := st.runStart; i < len(st.order); i++ {
		if qk != "" && st.searches[st.order[i]].question != qk {
			continue
		}
		for _, h := range st.markOneLocked(i, read).Results {
			if key := searchHitKey(h.URL); !h.Held && !h.Read && !tried[key] && !seen[key] {
				seen[key] = true
			}
		}
	}
	return len(seen)
}

// questionKey is the key of a SearchAll question: the terms of its words, as
// every in-run key is (searchquery.Key). Empty when the question has none.
func questionKey(question string) string {
	return searchquery.Key(question)
}

// noteUnreadPages tells the desk once a cell that its run holds many unread
// pages when it starts a plain search. A note only: the search is sent.
func (s *ChoirScope) noteUnreadPages() {
	if s == nil || s.searches == nil {
		return
	}
	n := s.searches.unreadNew("")
	if n < searchEnoughUnread {
		return
	}
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	if s.reuseQuiet || s.unreadNoted {
		return
	}
	s.unreadNoted = true
	s.allStat.unreadNotes++
	s.notes = append(s.notes, fmt.Sprintf("this run holds %d pages new to this run that you have not tried to read, returned by its earlier searches; the search is sent. Read the ones that matter (choir.Searches.Summary(id) lists them), then search again for what is still missing, naming it. A page that will not load counts once you have tried it; fresh sends a search the run holds results for", n))
}
