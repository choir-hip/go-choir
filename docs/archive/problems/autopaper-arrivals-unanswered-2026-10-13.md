# Autopaper: capture arrivals that management never answered (2026-10-13)

Status: fixed at cff54bf5; the rerun passed on a minted test computer
(docs/archive/evidence/c2-close-receipts-2026-10-13.json, E3_rerun). Cause confirmed
from the run trace (H2). Found by the C2 close receipts (E3) requested by the close review panel
(docs/archive/reports/c2-close-review-adjudication-2026-10-13.md).

## Contract

`autopaper_live` criterion (2) and management's prompt
(`internal/promptstore/defaults/management.yaml` lines 29-34): every captures
record from `source:<id>` gets an answer in management's next committed turn
— a story (`Cast("texture:new", …)`), a feed (`Note`/`Cast` to
`texture:<doc>` naming the capture ids) or a decline (`Note(r.From, reason)`
to the source, naming the capture ids).

## Evidence (read-only, paper computer, pulled 17:58 UTC)

Derived from the paper's management inbox, every story inbox, every source
inbox, revisions and runs, through the product API with the paper's key
(analysis kept outside the repo; the counts are recorded in
docs/archive/evidence/c2-close-receipts-2026-10-13.json).

- 34 arrival records carried 120 capture items from 15:22 to 17:52. The two
  newest records (17:52:51) were still in flight and are excluded: 118 items.
- 68 items were answered by an act naming the capture id or URL: 46 opened a
  story, 12 fed one (Note or Cast), 10 were declined to their source. Answer
  latency: median 101 s, p90 195 s, max 564 s.
- **46 items got no act that names them.** Management's cursor passed all of
  them (cursor = head at every sample).
  - About 9 belong to events that have a story opened from another source's
    capture of the same event (Gemini 4 Argon, Apple TV 4K/HomeView, CFTC,
    the Denmark-Germany tunnel, Switch to Safari, Consus), but no feed or
    decline names the HN or second-outlet capture.
  - About 37 have no story and no decline: HN items (Rust borrow-checker
    bugs, Intel D270, Codex CLI daemon), TechCrunch Disrupt items, OpenAI
    customer stories, gift guides, a Technology Review newsletter.
- 40 of the 46 come from the tier 1 burst: eight records (seq 9-16, hn_best,
  hn_frontpage, openai_news, techcrunch, technologyreview, theregister,
  theverge, wired) committed 15:29:53-55, inside management run 7bc9230b
  (15:29:44-15:30:03), with the next run ced3f93a at 15:30:22. Five more are
  one hn_frontpage record (seq 141, 17:22:53) and one of seq 103.

## Hypotheses (not yet confirmed from traces)

H1. **Model under-answering of a large batch.** The turn that saw seq 9-16
(40 items in one `choir.Records()` view) clustered the newsworthy items and
skipped the explicit decline for the rest. The prompt asks for one; nothing
checks it.

H2. **Records arriving during a running turn.** Seq 9-16 committed while run
7bc9230b was in flight. If that run's commit advanced the cursor to the
head at commit time rather than to the last seq its cells actually read,
records it never saw were consumed unanswered (a B13 relative). The
paper's build (761d94c5) contains the B13 fix (215d8f6f), which fixed the
*view*; the *cursor commit* bound is unverified.

The trace decides: read the `desk.cell` events of runs 7bc9230b, ced3f93a
and 09720566 for the record seqs their `choir.Records()` returned, and
compare with the cursor each committed.

## What is not the problem

- Not lost records: every arrival record is in management's inbox.
- Not a delivery failure: management ran 58 turns in the window, and the
  cursor reached the head each time.

## Consequences

C2 acceptance stays open on E3. Tier 2 and 3 expansion is blocked behind
this, because a 14-source burst already overflows one turn's answers.

## Trace (confirmed 18:05 UTC)

Run 7bc9230b (management, 15:29:44-15:30:03):

- 15:29:47 cell 1 `choir.Records()` returned seq 8 only (the burst had not
  landed).
- 15:29:50 cell 2 listed documents.
- 15:30:00 cell 3 cast five stories from seq 8's captures, using ids held
  from cell 1. It did not call `choir.Records()`.
- Seq 9-16 committed 15:29:53-55, between cells 2 and 3.

Run ced3f93a (next turn, 15:30:22) first `choir.Records()` returned seq 17.
No turn was ever shown seq 9-16.

Cause, in code: `loadCellRecords` (`internal/agentcore/inbox_records.go`)
loads every cell's view as (cursor, head] with `head` read when the cell
starts, and the cell's commit advances the cursor to that head through
`cellRecords.cursorAdvance()`, whether or not the cell's code called
`choir.Records()`. Cell 3 started at 15:30:00 with head 16, so its commit
marked seq 9-16 read. "Read" means "was in the cell's environment", not
"was shown to the desk". H1 (model under-answering) is not needed to
explain the burst; it may still explain the 9 covered-but-unnamed items.

Fix direction: advance the cursor only through the highest seq a
`choir.Records()` call in this run actually returned (the yaegi scope
records it), so records arriving mid-run wake the next turn instead of
being consumed. Texture's frame path (`texture_frame.go`) shows records in
its frame, so its read is real; the fix is scoped to cell reads.
