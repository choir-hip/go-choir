# L2a Design Note: Synthesis Intake

Date: 2026-10-14. Station:
[L2a](../definitions/choir-ledger-l2a-synthesis-intake-2026-10-14.md).
Inputs:
- the owner's architecture (station header);
- the Jev pair probe (`docs/archive/evidence/l2a-jev-pair-probe-2026-10-14.md`,
  26/26);
- the intake defects (`docs/problems/autopaper-intake-defects-2026-10-14.md`);
- a code survey of the poll path, Jev, object-graph edges, research
  addressing, corpus search and the library view (cited inline).

Status: accepted in panel round 2 with the amendments in "Round 2
adjudication" below, which supersede the text above them where they
differ. Mutation class of the work: red (a model call on the poll path, records
routes). This note is green.

## Shape: two slices

The owner's architecture has two halves. They land separately, so that
synthesis at intake is proven before research changes.

- **S1a, intake.** Jev labels each arriving capture against the story
  library. Management sees the labels and the whole library. The intake
  bugs are fixed. This alone should make multilingual coverage of one
  event become one story. It is the fixture's test.
- **S1b, research.** Standing topic researchers work the corpus, with
  durable topic memory, and management routes captures to them. Designed
  here; built after S1a is proven.

## S1a decisions

**A1. Jev labels each new capture on arrival, as its own object.**
- **Where.** In `HandleSourcePollDue`, after the captures are written
  and before the arrival record is minted (`source_schedule.go:161-174`).
  That span is outside `sourceScheduleMu` and on the source's own
  mailbox, so a slow call delays only that source.
- **Call.** Runtime code calls `rt.candidateJudge.Judge` directly, as
  `support_judge.go:152` does; no cell is involved. The gateway enforces
  a per-computer rate limit (`gateway/jev.go:258-270`). Jev is enabled
  on staging (`GATEWAY_JEV_JUDGMENTS_ENABLED=1`).
- **State.**
  - the capture: title, RSS summary, outlet, source language, URL;
  - the shortlist: up to 40 candidate stories, each with headline, lede
    (the first paragraph of the head revision) and last update.
- **One choice question.** The choices are each candidate's id plus
  `new`. The criteria say a capture belongs to a story when it reports
  the same real-world event or its direct development, in any language,
  and that the same topic but a different event is `new`.
- **Second question, only when the answer is `new`.** Which candidate,
  if any, shares the topic (choices: candidate ids plus `none`). This is
  the topic edge of A3.
- **What is recorded.** The answer, its probability and the model go in
  a `choir.capture_label` object in the computer's store, keyed by
  (capture id, label version). It is not written into the arrival record:
  the record's command digest covers the whole record, so a relabel on
  retry would refuse its replay (`capture_arrival.go:53-58`,
  `store/inbox.go:622`). The label also carries Jev's reported cost.
- **Failure.** If Jev fails or times out (5 s bound per capture), the
  capture is announced unlabelled. Labels never gate arrival.
- **Shortlist without embeddings.** The candidates are the stories
  updated in the last 72 hours, newest first, up to 40. At the paper's
  rate (about 8 stories an hour) that covers about 5 hours. When more
  stories than that are live, the candidates are the 20 newest plus the
  20 whose titles share the most tokens with the capture title. Token
  overlap without translation is the known weak spot across languages;
  the fixture measures it.
  - The probe showed Jev reads across languages itself, so the shortlist
    only needs recall, not precision.
  - A 40-candidate state is about 2,500 tokens, roughly $0.0001 per
    capture at the probe's measured price.

**A2. Management sees labels and the whole library.**
- The arrival record view (`inboxRecordView`) joins each capture item
  with its label: "Jev: same event as <doc> '<headline>' (p 0.97)", or
  "Jev: new; same topic as <doc>", or "unlabelled". Labels are advice;
  management decides.
- The library is no longer the 50-capped document list. A new store
  query lists stories (trajectory kind `story`) with headline, lede,
  update time, version and the languages and outlets of their cited
  captures. It pages, newest first. It is exposed to management as
  `choir.Stories(limit, before)`. The ProductAPI list's cap stays for
  the UI.
- Management's "Living stories" prompt changes three ways:
  - feed a capture into the story Jev names unless management reads
    otherwise;
  - one story per event across languages;
  - every override of a label is stated in the Note or decline, so it
    becomes a recorded correction (A3).

**A3. Boundaries are learned from recorded judgments.**
- **Story.** A story is the set of captures fed into it. Its boundary
  is what management and Jev put there, and it is visible as the
  captures its versions cite.
- **Topic.** A topic is the set of stories joined by Jev's "new, same
  topic as" answers. In S1a it is recorded on the label only, and S1b
  makes it a researcher's scope.
- **Corrections.** When management overrides a label by feeding
  elsewhere, opening, or splitting a story by casting a new one from its
  captures, the label object gains an `override` with management's
  reason. The latest 12 overrides become worked examples in Jev's
  state. Boundaries improve from the ledger, with no training.
- **Time is part of identity.** The 72-hour window is the default
  horizon. The fixture and live runs measure how long stories keep
  drawing feeds; the number comes from data.
- **Audit.** A sampled independent judge (the fixture's registered
  judge) scores merges each live window.

**A4. Texture writes from convergence and conflict.**
`internal/textureprompts/texture.yaml` gains a synthesis contract.
- When a story's captures or reports come from more than one outlet or
  language, Texture writes what they agree on as established and states
  where they differ, with each side cited.
- Each new version says in one line what changed and which new evidence
  changed it.
- Independent means a different publisher. Wire copies and one
  broadcaster's language services count once.

Texture's existing rule ("A claim backed by a primary record or two
independent reports is confirmed") stays, and now has a definition.

**A5. Intake bugs (from the defects record).**
- **Lossless per-poll truncation (defect 1).** Items beyond the limit
  are counted as `deferred` in the poll report. The source's validators
  are not stored while items were deferred, so the next poll reads the
  feed again and the repeat check picks up where it left off.
- **Lost arrivals (defect 2).** If the arrival append fails, the
  captures' ids go to a durable pending list for that source. The next
  poll's arrival names them first.
- **Library cap (defect 3).** Answered by A2's `choir.Stories`.
- **Per-item language (defect 5).** The arrival item and the label carry
  the source language and a script-based guess (Han, Kana, Hangul,
  Arabic, Cyrillic, Latin). Jev reads the text itself anyway.
- Defect 4 (dead corpus search) goes to S1b, B2 below.

**A6. Per-computer enable.** A computer setting, `intake.labels`
(off by default), is set by an owner act through the product API. When
it is off, A1 does not run and A2's prompt text is not shown. The paper
computer is enabled only after the fixture passes, and disabling it is
the rollback. Bug fixes A5 are not gated: they are corrections.

## S1b decisions (designed now, built after S1a's proof)

**B1. Topic researchers.**
- Research becomes per topic instead of per document. A story belongs
  to one topic, and a new event with no topic makes a topic of one
  story.
- The research desk's inbox is `research:topic:<id>`, and its agent is
  minted on the first record to it, as today's per-document research is
  minted in a Texture turn (`store/texture_turn.go:625-656`), but by
  either Texture or management.
- Texture's `choir.Cast("research", …)` resolves to its story's topic
  researcher, and the researcher's reports go to the requesting story's
  `texture:<doc>`.
- Management may Note a topic researcher with new captures for its
  topic. The researcher decides which of its stories they change and
  reports to that story's Texture.
- `recordsRoute` gains these routes: management → `research:topic:*`,
  Texture → its topic's researcher, research → `texture:<doc>` for docs
  in its topic.

**B2. A real corpus search.**
- `choir.SearchWireCorpus` drops the dead "published" test and searches
  the computer's stories.
- A new `choir.SearchCaptures(query, since)` searches the computer's
  captures in Store B by title and block text, through corpusd.
- Research's prompt says: corpus first, then the web.

**B3. Durable topic memory and dormancy.**
- REPL state dies after 30 idle minutes, and at most 12 idle sessions
  are kept per computer (`tools_desk.go:40-52`). So a topic researcher's
  memory is a durable topic brief, not REPL variables.
- The brief is the researcher's latest report per story plus its open
  questions, readable at wake through `choir.TopicBrief()`.
- A dormant researcher costs no process: the session is reclaimed as
  today, and the next record wakes a fresh run that reads the brief.

## Ways S1a could fail (written before the code)

1. Jev merges two different events that share actors (the Kyiv strike
   and the drones off Bulgaria). The fixture's same-topic singletons
   test this. Management's override is the correction path.
2. The shortlist misses the right story (an old story, or title tokens
   in another language). Measured on the fixture as label recall.
   Fallback: a capture labelled `new` that management recognizes is fed
   anyway, and the override is recorded.
3. Jev is slow or down. Arrival is never gated (5 s bound, unlabelled
   fallback). Tested with a failing judge.
4. A label differs on retry. Labels are their own objects, never part of
   the arrival digest. Tested by replaying a poll with a different label.
5. Rate limit. The gateway's per-computer bucket refuses, and the
   capture arrives unlabelled. The poll's label coverage is counted.
6. Deferred items starve: a feed that always has more than N new items
   per interval. The deferred count is visible per poll, and the fix is
   the interval, not silence.
7. A pending-arrival list that grows without end. It is bounded by the
   poll's own limit, and the oldest are named first.
8. Management ignores labels. Measured as the override rate and the
   fixture's stories per event.
9. Cost. Recorded per label. The fixture reports the cost per capture.
10. The paper regresses. The per-computer enable, the tripwire (no
    version in 90 minutes, or more than half the captures declined in an
    hour) and a measured baseline cover it.

## Order

1. S1a code: A5 (bug fixes, ungated), A1-A4 and A6, with tests for 1-7.
2. Push, CI and deploy (with the intake off by default).
3. Fixture run on a minted test computer with `intake.labels` on.
4. If the fixture passes, enable the paper and run the after-window.
5. S1b after that.

## Round 2 adjudication (2026-10-14)

Panel: claude and glm accept; deepseek and gemini send back on one
point, which all four name. gpt-6.1-sol was usage-limited. The blocker
is fixed below, so the note is accepted with these amendments.

**R2-1. Defect 1 is fixed in the right order (the blocker).**
- The poller truncates to `max_items` before the repeat check
  (`rss.go:91-96`, then `sourcegraph/captures.go:154-158`). Withholding
  validators alone would re-read the same first N items every poll, never
  reach the rest, and turn off 304s for every feed longer than N.
- The fix:
  - the poller returns every parsed item, up to the hard cap of 100;
  - the per-poll limit applies to new captures, after the repeat check,
    in feed order, and new items beyond it are counted as `deferred`;
  - validators are stored only when `deferred` is 0.
- Residual: an item that leaves the feed's window before it is captured
  stays unreachable without a per-source cursor. The `deferred` count
  makes that visible.
- The fixture schedule sets `max_items` to at least the fixture's item
  count, so its acceptance cannot hide this defect.

**R2-2. The pending-arrival list (defect 2).**
- The list lives on the existing `choir.source_schedule` object.
- It is written before the append is attempted and cleared when the
  append succeeds.
- Writing it is best effort: a failure is logged and counted, and never
  fails the poll.
- A crash boot clears it, and its ids are recorded with the fate
  "interrupted by a restart". A boot with a planned-restart marker keeps
  it.

**R2-3. One Jev call per capture, with short choices and siblings.**
- The event and topic questions go in one request.
- The choices are short aliases (c1, c2, …) mapped back to doc ids.
- The candidates include the poll's other new captures (n1, n2, …), so
  same-event captures in one poll are labelled as siblings.
- Labelling has its own gateway rate scope, so it cannot starve the
  support judge that publication needs.
- A poll labels its captures in parallel (at most 4 at once) within 15
  seconds in total.
- The cost comes from the response's `usage`, which the decoder must
  read.

**R2-4. The shortlist weights recency over tokens.** The 30 newest
stories plus 10 by token overlap. The live window reports label recall
separately from merge recall. Before the paper is enabled, it states how
far back cross-language matches reached.

**R2-5. Structured dispositions.**
- Management gets `choir.Feed(doc, captureIDs, reason)` and
  `choir.Decline(captureIDs, reason)`. They write the same records a Note
  and a decline write today, plus a structured list of captures.
- The runtime compares each disposition with the label and records an
  `override` on a mismatch.
- This is what the override examples, the override rate and the
  fixture's "every capture has a disposition" score are read from.

**R2-6. Labels join the view where the store is.**
- `inboxRecordView` is a pure function of the record, so the join happens
  in `loadCellRecords`.
- The view names the label version it showed, so a later override never
  rewrites what management saw.

**R2-7. The enable and the fixture guard are new, named surfaces.**
- `intake.labels` is a `choir.computer_setting` object, set by an owner
  act through the product API, like the schedules endpoint. It must be
  live on staging before the paper is enabled.
- The fixture's registry entry has `tos_class: test_fixture`.
  `StartSourceSchedule` refuses that class unless the request is marked
  as a fixture run.

**R2-8. Moved into S1a.** Deleting the dead "published" filter in
`SearchWireCorpus` (defect 4, one line), so per-document research can
already find the paper's own stories. Searching captures stays in S1b.

**R2-9. The order gains the live-window denominator.** The judge-scored
same-event pair count in `scripts/l2a_paper_window.py` is built before
the after-window. L2a does not close until S1b lands, because
`finish.deliver` names the researchers and the corpus search.
