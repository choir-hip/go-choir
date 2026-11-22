# Read-first makes research runs read more but not search less

Recorded 2026-11-22, 70 minutes after read-first (2543187a8, 92f068980) went
live on the paper at about 01:30Z. Ledger claim C188 is still open; these
readings point to false.

## Problem

The rule was meant to bring web searches per research run under 2.0 (3.9
before). Over the first 70 minutes: 9 runs, 39 searches, mean 4.33. Three runs
sent 7 or more and carry 72% of the searches. `choir.SearchFresh`, the escape
meant for fewer than one run in five, sent 15 of 40 searches. Pages read rose
to 17% of pages returned (about 8% before), so runs do read more.

## Evidence (`scripts/status-search-reads --since 2026-11-22T01:30Z`, the console and the gateway journal)

- Run ed73d3de: 10 searches, no SearchFresh. Each search was followed by 3 to
  17 reads and then the next search; 400 pages returned, 330 new, 51 read. The
  rule allows this for ever: trying the first five new pages earns the next
  search.
- Run 136d2827: 11 searches, 7 by SearchFresh, one of them with zero reads
  since the previous send and 15 seconds after it; later queries overlapped
  earlier ones by word set up to 1.00. Run 9772f50d (same conversation): 7
  searches, 4 by SearchFresh, the last two 8 seconds apart. Both rounds ended
  without a final Report.
- Four runs used SearchFresh as their only search, straight after a
  held-first refusal.
- Every refusal text ends by naming SearchFresh
  (`internal/yaegikernel/search_need.go:441-445`), and a SearchFresh send
  skips every check, a near-duplicate of the run's own earlier query included
  (`search_store.go:573-661`).
- The stop rule (`stopped_yielding`) exists only inside `SearchAll`
  (`search_all.go:305-310`); `mostly_seen` needs under 25% new pages and these
  searches returned 82% new.

## Belief

Two separate faults. (1) The escape is presented at the end of every refusal
and is unchecked, so the model takes it as the instruction. (2) The need test
looks only at the first five new pages of each search, so a run that holds
hundreds of untried pages it asked for is still judged to need another search.
The owner's rule is "reading much more and searching less": the stock of
untried pages a run already holds is the measure of whether it needs more.

## Remaining error

The SearchFresh reason text is on the guest's tape, which the host cannot
read, so reflex use is inferred from timing and query overlap, not from the
reasons. Whether ed73d3de's ten queries were materially different is unknown
(the gateway journal cut them). Two of the three heavy runs ended without a
Report; why is not known and is a separate question. Total volume is under
the 1,000 a day target (about 33 an hour) because the main-point change
starts fewer research runs, not because of this rule.

## Owner ruling, 2026-11-22

The lead's first plan was a tighter gate (judge need by the stock of untried
pages; restrict the escape). The owner rejected the frame: "we shouldnt need
rules. we should be able with prompts to encourage models to search the object
graph first, and then the web... we should be able to provide a lot of
compressed context from the object graph, from prior stories, more than a web
search". The fix is therefore the run's opening context and its prompt, and
the refusal gates come out. The tighter gate was not built.
