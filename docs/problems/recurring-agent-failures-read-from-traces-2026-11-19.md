# Recurring agent failures read from traces, and a clustering assessment (2026-11-19)

Station L1 of
[the night's goal file](../archive/definitions/choir-night-exact-vectors-learning-2026-11-19.md).
Read-only; nothing was changed. The owner, 2026-11-19: "we need to review the
traces and investigate all the problems eg the 3090 read loop. where we have
repeated problems there may be solutions. but note that if the problems are at
the model level, naively changing the harness or prompt may not be correct."

The four trace readings that belong with an existing record are there:
the read loop, the runs that start many searches and the 6% read share in
[search-volume-outruns-keenable-credits-2026-11-18.md](search-volume-outruns-keenable-credits-2026-11-18.md)
("Traces read, 2026-11-19"); `candidates_shown=0` in
[research-does-not-start-from-the-corpus-2026-11-18.md](research-does-not-start-from-the-corpus-2026-11-18.md);
the corpus search's time in
[corpus-search-takes-two-seconds-on-the-paper-2026-11-19.md](corpus-search-takes-two-seconds-on-the-paper-2026-11-19.md).
This record holds the rest.

## What was and was not read

- Read: Node B's gateway, vmctl and corpusd journals; the newspaper's guest
  console, which rotates at about 1 MB and now begins at 02:14:45Z (so the
  02:12Z run is gone from it, except its last two model calls); the code.
- **Not read: any run's own ledger** (cell programs, tool results). The
  supported path is `choir trace export` (`cmd/choir/trace.go`, product routes
  only), which needs a key bound to the newspaper computer with its run-event
  read scope. None was available, and the owner's key, the operator key and
  the tape were out of bounds. Every statement here about what a model wrote
  is an inference from counts and code and says so. Later the same night, with
  no key for the paper to be had, the cell programs were obtained by
  **reproduction on the drill computer d@e**, nine runs on the same model; see
  "Reproduction on the drill computer d@e" in the search-volume record. That is
  evidence about the model and the surface, not proof of what the paper's runs
  did. The paper's own runs (`c0351184` and the others) are still unread; the
  owner minting a read-scoped key for the newspaper, or running the export,
  settles them.
- The paper's research desk runs `devin/swe-2-flash`. A cause called
  "model behaviour" below means the logs fit the model's choices and no
  harness difference is shown; it is not a claim that another model would do
  better. Only the comparisons named settle it.

## Store stalls and unhealthy lines (item 5): the rate has not moved

`vmmanager: VM ... is unhealthy` lines in the vmctl journal per hour, the
paper's VM (794 of the 796 lines; the other 2 are one other computer):

| day | hours covered | lines | an hour |
| --- | --- | --- | --- |
| 2026-11-17 | 11Z to 23Z | 235 | 18 |
| 2026-11-18 | 00Z to 23Z | 433 | 18 (3 to 50; peaks 40 at 16Z, 50 at 20Z) |
| 2026-11-19 | 00Z to 05:22Z | 128 | 24 (30, 44, 21, 8, 12, 13) |

Inside the guest, 161 `store_stalled` lines in 3 hours (the `holders:`
summary line was written 12, 8, 10 and 6 times in the hours 02Z to 05Z; the
rest are one line per holder). Holders: `ReadObjectSnapshotFiltered` 46,
`ListObjects` 25, `GetObjectByMetadata` 13, `ListObjectsByKindOwnerWhere` 9,
`GetObject` 7, `ListObjectsByMetadataPage` 6, `RedriveFamilyStatus` 4. The
summary shows a single-connection write pool that has waited 41,054 times for
1,625 seconds in all, and `unanswered=live_capsules,effective_release` (the
health probe's answers waiting on the store). No `vacuumIntoCopy` or
`checkCompactCopy` holder in these hours, so the compact copy of
[compact-copy-grows-the-host-image-and-restarts-discard-it-2026-11-15.md](compact-copy-grows-the-host-image-and-restarts-discard-it-2026-11-15.md)
is not what holds it now; the holders are object-graph reads. Earlier
records of the same stall family:
[the archived holder list of 2026-10-23](../archive/problems/paper-remaining-store-stall-holders-2026-10-23.md)
and the storage mission
([ACTIVE.md](../ACTIVE.md), store once). Verdict: the rate is the same on
November 17, 18 and 19 (18, 18, 24 an hour); the 24 is within the day-to-day
swing (the 18th alone ran 3 to 50). Not re-investigated.

## Other recurring failures seen (counts from the 3 hours of console)

Each with a count and an example; none was chased.

1. **`choir.Stories` paging is refused by the broker's own bound.** 4 lines of
   `desk frame refused: desk "management_task" action stories ... refused:
   broker protocol: stories.before is 67 bytes, over 64` (03:39Z, 03:56Z,
   05:11Z, 05:15Z). The cursor a page returns is a nine-digit-nanosecond
   timestamp, a bar and a 36-character document id: 30 + 1 + 36 = 67
   (`intake_view.go:150-154` builds it; `broker_bounds.go:66` caps `before` at
   64 since 2026-11-05, the cursor having had a document id since
   2026-10-13). A cursor whose timestamp has fewer digits fits, so some page
   2 calls pass and some do not. Class: **harness**, certain. The management
   run that wanted the second page of candidates cannot have it. A test that
   feeds a page's `Next` back to the verb would have found it; none does.
2. **A story's publication fails and "stays a gap".** 38 lines of `story
   publish: <id> v<n> (revision <id>) failed and stays a gap: ... status 502:
   {"error":"failed to load private texture revision"}`. Class: not
   established (a store read failing under the stall above is the first
   thing to test: 38 in 3 hours against the store stalls).
3. **Citations dropped from a revision.** 98 or more of the 331 `texture cell
   authored turn` lines carry `dropped_citations` above 0 (60 with 1, 24
   with 2, 8 with 3, 6 with 5). Class: not established; the trace of one such
   turn says which citations and why (a quote not found in its capture, the
   usual cause, is model behaviour on exact quoting; a capture the revision
   could not read is harness).
4. **Publish-auto work refused without an owner ask.** 10 lines (8 on one
   `desk_message` run, 03:46Z; 2 on a publication) and 2 `story publish ...
   not published: disclosure refused`. Class: not established; a refusal
   that is correct by the disclosure rules and repeated by a run that does
   not learn from it, or a path that sends what the rules forbid.
5. **A run fails on `max_tokens` with no text.** 2 runs (iteration 32 and
   iteration 1). Class: not established; iteration 1 failing is a request
   whose output budget is smaller than one cell.
6. **The same search sent again inside one run.** 23 of 641 searches in 14 of
   85 runs were exact repeats of a query the run had already sent (one sent
   seven times in 85 seconds); the in-run reuse did not answer them
   (`from_held=0`). Class: not established (`fresh`, a failed first search or
   a reuse gap).
7. **A page read many times in one run.** The most reads of one capture by
   one run, apart from the 3,090 of run `c0351184`: 21, 18, 17, 16, 13
   (`corpus read` lines, 2,982 in 3 hours). Mostly the paging of long pages
   at 8,000 characters; not chased.
8. **Guest virtual-CPU I/O failures in the console.** 75 lines (`vcpu: IO
   write/read @ ... failed: Failed to find address range`). Noise from the
   virtual machine monitor; no run attributed.
9. **The corpus brief missed its deadline once in three.** See the update
   in the two-seconds record.

## Clustering assessment: three defects in corpus search in a week (rule 3)

The three defects:

- **A. The title-only index** (found 2026-11-18, `research-does-not-start-from-the-corpus`).
  `SearchWireCorpus` over 207,000 captured pages searched only title, host,
  source id, language and Jev's labels; "a fact that appears only in a page's
  text cannot be found". The verb's name and the prompt read as a search of
  the corpus.
- **B. The two-second search** (found 2026-11-19, `corpus-search-takes-two-seconds`).
  One call takes a median of 2.1 seconds on the paper; the brief was designed
  to make six in turn under an eight-second deadline. Seven review rounds did
  not find it "because none of them measured the search". The time is still
  a median of 1.7 seconds over 280 calls in the three hours read here.
- **C. A brief that claimed absence wrongly** (nine review rounds, 2026-11-18
  to 2026-11-19). "Nothing found" was said when part of the search had not run:
  a failed half, a deadline cut, an index still catching up, text never
  searched. Each repair added a description of coverage that the fallback
  paths could not make true; round 6 deleted it. The first live brief that hit
  its deadline (05:25Z) is that case happening for real.

Earlier members of the same family, in the ten days before: the published
half returned 403 to every guest and reported it as a note
([published-search-refuses-every-guest-2026-11-09.md](published-search-refuses-every-guest-2026-11-09.md));
"the paper cannot find its own earlier stories" because duplicate detection read
a window of the newest stories, not the archive
([published-search-recall-2026-11-10.md](published-search-recall-2026-11-10.md)).
That makes five in two weeks, which is the rule's threshold twice over.

**What the three share.** The search verb returns a list and, beside it, a
free-text note. It does not return what it covered, how long it took, or
whether it finished. So:

1. *The answer cannot say what it searched.* A caller (the model, or the
   brief builder) can only read "no results" as "nothing held", and A, C and
   the 403 all end there. The shape of the answer makes absence the default
   reading of silence.
2. *Nobody measured the real thing.* The tests prove that "stories and
   captures are found and failures reported" on small fixtures; "there is no
   recall or known-item test" and no latency measurement against 200,000
   pages. B is that gap; A would have shown in one known-item query (a fact
   from a page's body); the 403 would have shown in one guest call.
3. *Each defect was repaired where it was seen*, by a note or a description
   in the caller (C's six rounds), not in the verb, so the next caller met the
   same absence.

**What substrate fact would have prevented them.** A corpus search answers
with a typed record the verb is not allowed to omit: the layers it searched
(titles, page text, events, published stories) and for each its status (ran,
refused, partial, deadline, not indexed yet) and size (pages indexed against
pages held, the newest page held), and the time each took. A caller may call
a thread "nothing held" only when every layer ran to completion; otherwise
the record says "not searched" and says which part. With that answer in the
verb, the brief needs no description of coverage (C), a title-only layer is
visible as one layer of two on every call (A), the time is in every answer and
a gate over the real corpus (a fixed set of known-item queries, a latency
budget at the 214,000 pages we hold) fails a change that exceeds it (B), and a
403 is a layer that refused, not a note. This is the substrate change; the
patches (an index of page text, a concurrent brief) are on the symptom
side of the same fact.

A second, smaller substrate fact the same readings show: the harness reports
**counts where a choice needs a list** (search results as "12 of 40 pages
new", management's `candidates_shown` that counts calls not what the opening
delivered, the per-cell bound's "continue in the next cell"). In each case the
log or the message names a quantity that is not the thing the reader needed.
That is not corpus search, but it is the same kind of defect in the same two
weeks.

## What a controlled comparison would have to vary (for the redesign of the calibration experiment)

For every failure here and in the three records it links, the log shows what
the model was shown and what it did, and cannot show why. Nine reproduction
runs on d@e (2026-11-19, same model) now show what the programs are, which
changes what is worth varying. To tell the model's behaviour from the
harness's, the experiment has four axes, each varied alone against a fixed,
replayable set of objectives (the first message and held captures of real
runs, so the surface is the only difference):

- **The model**, the surface fixed: the same model several times (variance;
  one run each told us nothing about a rate: D1 started 89 searches, D3 18
  on objectives of one kind), and two or more other models. A behaviour that
  appears in every model is a surface effect; one in one model is the model's.
- **What the cell shows**, the model fixed: counts only against counts and
  titles; the unread-pages note as now against no note against a note that
  lists the unread titles (the reproduction shows the note's number climbing
  95, 172, 289, 369, 467 while the next cell searched again, so the arm
  "no note" is the control); a refusal that says "continue" against one that
  names the verb and the capture.
- **What the surface permits and keeps**, the model fixed: `SearchStart` and
  `Search` present or absent (1 `SearchAll` in 82 searches: the stop rule is
  on the verb the model does not use); a page-read verb that returns a whole
  page against one that must be walked; **the session's state after a panic
  or a compile error**: today a slice panic ends the session and a failed
  cell leaves its variables undefined, and the model answers by re-paging the
  whole page (C1, C2, C3: 8, 13 and 14 full passes). An arm that keeps
  the variables, or says what survived, tests whether the loop is the model's
  habit or the harness's lost state; and the per-cell bound as now against a
  lower one.
- **The objective's kind**, everything fixed: well-covered news, a local
  event with thin coverage in another language, one long page. The searches
  a run starts moved with this kind (6 to 12 on the A runs, 18 to 89 on the
  thin ones) more than with anything else seen, so a result that pools kinds
  hides it.

Each arm records, per run, from the run's own events: the cell programs
(`tool.result` `arguments.source`, so a loop is a program or a sequence), the
host calls in each cell (`desk.cell` `egress_calls`), the notes shown, whether
the session survived each cell (`reuse`), searches by verb and by subject,
URLs tried by rank in the printed list, and whether the answer was found. The
export that yields them is `choir trace export` under a key bound to the test
computer; d@e takes the arms, and **the paper and the owner's computer hold
no arm**. The research desk's model is read from
`/mnt/persistent/files/System/model-policy.toml` on the computer: an arm that
changes the model is a setting that outlives the run, so it is set by the
owner as an E1 arm on a test computer, not by the reader. What the nine runs
settle, and what they leave: the read loop is shown to have a mechanism (lost
session state, then a whole-page re-read each cell) but not its scale (the
largest cell made 188 host calls against a bound of 1,024); the many-search
shape is shown (89 searches in 18 cells of looped wordings) but only for a
thin-coverage kind and one run of it; the read share is shown to follow
what the task needs and not what searches return.
