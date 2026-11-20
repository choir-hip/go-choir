package yaegikernel

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// The research desk's one-cell helpers (docs/evidence/token-efficiency-
// 2026-11-09/followup/research-one-cell.md, options (a) and (b)). Ways a
// one-cell round fails today, written before the code:
//
//  1. Read returns JSON bytes, so a cell decodes it by hand and the first
//     guess ("r.CaptureID", `c["capture_id"]`) fails the whole cell. The
//     accessors on JSON answer those guesses.
//  2. Reads run one after another in a cell (three reads took 11.6 s); one
//     slow page holds every other. ReadAll reads in parallel, each with its
//     own guard, and a failure comes back as that page's Err, never a panic.
//  3. A cell slices a string past its end and panics; the panic replaces the
//     session and drops what the cell staged. Cut clamps.
//  4. SearchStart, SearchWait and Get are three calls for one search whose
//     one provider answers in under a second. Search is the three in one.

// Block is one block of a captured page.
type Block struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	Page int    `json:"page,omitempty"`
	Text string `json:"text"`
}

// Page is one page ReadAll read: its capture, or why it has none.
type Page struct {
	CaptureID  string  `json:"capture_id"`
	URL        string  `json:"url"`
	Title      string  `json:"title"`
	BlockCount int     `json:"block_count"`
	Blocks     []Block `json:"blocks"`
	NextBlock  int     `json:"next_block"`
	// Err is empty when the page was read.
	Err string `json:"err,omitempty"`
}

// OK reports whether the page was read into a capture.
func (p Page) OK() bool { return p.Err == "" && p.CaptureID != "" }

// String is one line a cell can print for every page, failures included.
func (p Page) String() string {
	if !p.OK() {
		return fmt.Sprintf("FAILED %s: %s", p.URL, p.Err)
	}
	return fmt.Sprintf("ok %s capture_id=%s %q blocks=%d shown=%d", p.URL, p.CaptureID, p.Title, p.BlockCount, len(p.Blocks))
}

// page decodes a Read or ReadCapture result. It never fails: a result that is
// not a read result decodes to an empty Page.
func (j JSON) page() Page {
	var p Page
	_ = json.Unmarshal(j, &p)
	return p
}

// Page decodes a Read or ReadCapture result into a Page.
func (j JSON) Page() Page { return j.page() }

// CaptureID is the result's capture id ("" when it has none).
func (j JSON) CaptureID() string { return j.page().CaptureID }

// Title is the result's page title.
func (j JSON) Title() string { return j.page().Title }

// URL is the result's page URL.
func (j JSON) URL() string { return j.page().URL }

// BlockCount is how many blocks the whole capture has.
func (j JSON) BlockCount() int { return j.page().BlockCount }

// NextBlock is the first block the result did not show (0 when it showed all).
func (j JSON) NextBlock() int { return j.page().NextBlock }

// Blocks are the blocks the result shows, each with its id and text.
func (j JSON) Blocks() []Block { return j.page().Blocks }

const (
	// readAllMax bounds the URLs of one ReadAll call.
	readAllMax = 24
	// readAllParallel is the number of reads in flight at once; it is the
	// host's per-cell concurrency.
	readAllParallel = 8
)

// readAllGuard is how long ReadAll waits for one page before it reports the
// page as failed.
var readAllGuard = 20 * time.Second

// ReadAll reads every URL in parallel (eight at a time, each with its own
// 20 second guard) and returns one Page per URL in the order given. A page
// that failed has Err set; the others are still returned. Each read is
// recorded in the read ledger as choir.Read records it.
func (s *ChoirScope) ReadAll(urls []string) []Page {
	pages := make([]Page, len(urls))
	sem := make(chan struct{}, readAllParallel)
	done := make(chan struct{}, len(urls))
	started := 0
	for i, u := range urls {
		u = strings.TrimSpace(u)
		pages[i].URL = u
		switch {
		case u == "":
			pages[i].Err = "empty url"
			continue
		case i >= readAllMax:
			pages[i].Err = fmt.Sprintf("not read: ReadAll takes at most %d urls a call; call it again for the rest", readAllMax)
			continue
		}
		started++
		go func(i int, u string) {
			sem <- struct{}{}
			defer func() { <-sem; done <- struct{}{} }()
			type outcome struct {
				out JSON
				err error
			}
			ch := make(chan outcome, 1)
			go func() {
				var out JSON
				err := s.call(ActionRead, ReadPayload{URL: u}, &out)
				ch <- outcome{out, err}
			}()
			select {
			case o := <-ch:
				s.recordRead(u, o.out, o.err)
				if o.err != nil {
					pages[i].Err = o.err.Error()
					return
				}
				p := o.out.page()
				p.URL = u
				if p.CaptureID == "" {
					p.Err = "the read returned no capture"
				}
				pages[i] = p
			case <-time.After(readAllGuard):
				err := fmt.Errorf("choir: read timed out after %s", readAllGuard)
				s.recordRead(u, nil, err)
				pages[i].Err = err.Error()
			}
		}(i, u)
	}
	for ; started > 0; started-- {
		<-done
	}
	return pages
}

// searchWaitMs bounds the wait of one choir.Search.
const searchWaitMs = 15000

// Search starts one search, waits for it (at most 15 seconds) and returns it:
// SearchStart, SearchWait and Searches.Get in one call. A search of this run
// still running is not waited on: Search returns at once with Refused set, Done
// false and that search's id in ID. Results are every
// arrival's hits, each URL once. Done false means a provider was still
// pending; read choir.Searches.Get(id) later for more.
func (s *ChoirScope) Search(query string, fresh ...bool) (Search, error) {
	stub, err := s.SearchStart(query, fresh...)
	if err != nil {
		return Search{}, err
	}
	if stub.Refused != "" {
		if stub.Running {
			// Not sent: another search of the run is running. ID is that search's;
			// Done is false so this does not read as a search that found nothing.
			return Search{ID: stub.ID, Query: stub.Query, Providers: []string{}, Results: []SearchHit{}, Done: false, Refused: stub.Refused, Running: true}, nil
		}
		// Not sent: the Search holds the pages the run has not read, and why.
		return Search{Query: stub.Query, Providers: []string{}, Results: stub.Unread, Done: true, Refused: stub.Refused}, nil
	}
	if _, err := s.SearchWait(stub.ID, searchWaitMs); err != nil {
		return Search{}, err
	}
	got, err := s.searches.Get(stub.ID)
	if err == nil && stub.Reused != "" {
		// A near repeat: the value is the earlier search, and says so.
		got.Reused, got.ReusedFrom = stub.Reused, stub.ID
	}
	return got, err
}

// Cut returns str[a:b] clamped to the string: a below 0 is 0, b past the end
// is the end, a past b is "". It never panics and never splits a UTF-8
// character.
func (s *ChoirScope) Cut(str string, a, b int) string {
	if a < 0 {
		a = 0
	}
	if b > len(str) {
		b = len(str)
	}
	if a >= b {
		return ""
	}
	for a < len(str) && !utf8.RuneStart(str[a]) {
		a++
	}
	for b < len(str) && !utf8.RuneStart(str[b]) {
		b--
	}
	if a >= b {
		return ""
	}
	return str[a:b]
}

func init() {
	// The surface docs of these verbs live beside them, so a change to the
	// shared doc table does not collide with this file.
	choirFunctionDocs["ReadAll"] = FunctionDoc{Args: []string{"urls"}, Summary: "Reads every URL in parallel (eight at a time, each with its own 20 second guard) and returns one Page per URL, in order. A Page has CaptureID, URL, Title, BlockCount, Blocks (each with ID, Kind, Text), NextBlock and Err; Err is set when the page failed, and the other pages are still returned. Each read is recorded as choir.Read records it, so a failure shows in the cell result. Take at most 24 urls a call.", Example: "pages := choir.ReadAll([]string{\"https://example.com/a\", \"https://example.org/b\"})\nfor _, p := range pages { fmt.Println(p) }"}
	choirFunctionDocs["Search"] = FunctionDoc{Args: []string{"query", "fresh"}, Summary: "SearchStart, SearchWait (at most 15 seconds) and Searches.Get in one call: returns the Search with Results, each URL once (Title, URL, Snippet), and Done. Read the results before you search again; when the run holds 15 or more pages new to it that it has not tried to read, the search is not sent (no error): the returned Search has Refused set (the reason), Done true and, as Results, those unread pages. Read them, then search again for what is still missing, naming it; fresh sends the search anyway. The same comes back at once, with no pages, when another search of this run is still running: wait for it with choir.SearchWait. A query that asks the same question as an earlier search of this run is not sent: the returned Search IS that earlier search (its ID and Results), with Reused set (the reason) and ReusedFrom (its id), so check Reused before you take the Query and Results as the answer to a new wording; pass true as the second argument (fresh) to search anyway. When your latest search for a question returned mostly pages you already held (8 or more pages, under a quarter new) and you hold pages for it that you have not tried to read, a further wording of it is not sent: Refused says the web has shown what it has, Results are those pages; read them or conclude not found, the hold ends once you have tried every page you hold for it (a page that will not load counts once tried: if all fail, the next wording is sent), or pass fresh; a wording with site: to an outlet you hold no page from is never held. A search refused because another is still running has Running true and that search's ID (Done false: it found nothing yet, not nothing).", Example: "s, err := choir.Search(\"ministry cabinet changes\")\nfor _, h := range s.Results { fmt.Println(h.URL) }"}
	choirFunctionDocs["SearchAll"] = FunctionDoc{Args: []string{"question", "wordings"}, Summary: "Give it the wordings of ONE question you lack pages for, and it searches the web for them one at a time, in order, instead of all at once, so each result can inform the next. A wording that asks the same question as a search you already made in this run (the same terms, or only word order, today, a weekday, or today's or yesterday's date changed; a different site:, a quoted phrase or another date is a different question) is answered from that search (its pages join the list, marked held). When your latest search for the question returned mostly pages you already held and you hold pages for it you have not tried to read, the wordings left are not sent (never one restricted with site: to an outlet you hold no page from): read what you hold or conclude not found; the hold ends once you have tried every page; fresh sends one anyway. Wordings are fallbacks, not multipliers: before each wording it checks the question's pages, and when the question already holds 15 or more pages new to this run that you have not tried to read (a page that will not load counts once you have tried it), it sends no more and names the count; read the ones that matter, then search again for what is still missing, naming it. It also stops sending when a wording's first ten pages are at least 80% pages you already hold (Held), or two wordings in a row return nothing (a provider answered with no page), or two wordings in a row fail (no provider answered, or the host refused: named FAILED with the reason, then the rest are not sent because search is failing), or after 120 seconds. It returns one merged list, each page once, with Held (you held it before this call) and Read marks, and one line per wording: sent (N new in first 10), FAILED (reason), or not sent with the reason. A wording left out can still be sent alone, any time: choir.Search(wording, true) or choir.SearchStart(wording, true) (fresh sends it although a held search has the same terms). A wording is not sent while another search of this run is still running (its line says so); wait for that one with choir.SearchWait. Different questions are different calls. Each search waits at most 15 seconds, as Search does.", Example: "r, err := choir.SearchAll(\"harbor strike\", []string{\"harbor strike mauritius\", \"mauritius port workers walkout\", \"site:govmu.org harbor strike\"})\nfmt.Println(r)\nfor _, h := range r.Results { if !h.Held { fmt.Println(h.URL) } }"}
	choirFunctionDocs["Cut"] = FunctionDoc{Args: []string{"str", "a", "b"}, Summary: "Returns str[a:b] clamped to the string: a below 0 is 0, b past the end is the end, and it never panics (a slice past the end of a string ends the cell and replaces the session).", Example: "head := choir.Cut(out, 0, 2000)"}
}
