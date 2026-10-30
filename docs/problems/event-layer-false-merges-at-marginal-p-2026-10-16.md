# Event layer V1 fails its false-merge bar: joins at p 0.85–0.92 are topic merges and fake corroboration (2026-10-16)

Status: documented. Fix to follow.

## Acceptance result (V1, design §9; the paper 01:02–07:02 UTC)

| Criterion | Bar | Result |
|---|---|---|
| Captures resolved | ≥ 95% | **211 of 211** arrived captures have a membership (100%), 0 unresolved |
| Calls per flush | ≈ 1 | 69 calls over 66 flushes |
| Cost | reported | $0.0154 for 211 captures |
| False merges, blind sample | ≤ 5% | **7.0%** with both judges agreeing (3 of 43); 9.3% by the orchestrator's reading (4 of 43). **Fails** |

The sample:
- 43 joined memberships, one per (event, member URL): 16 from the paper,
  27 from the V3a test computer `v3-events-0146`, which runs the same
  resolver on six world feeds.
- The paper has only 16 distinct-URL joins. Its other 126 joins are
  repeat captures of one URL.
- The judges were `openai/gpt-6.1-sol` and `google/gemini-3.8-flash`
  through OpenRouter. They saw only the two titles and URLs, plus the
  resolver's own definition of the same event, never Jev's p (design
  §19.2).

## The false merges

| p | Event | Joined member | Judges | Corroboration rests on it |
|---|---|---|---|---|
| 0.90 | Iran war live: Saudi blasts Houthi "war crimes" | Middle East updates: Houthis target Yemen government palace | both different | yes (2 outlets) |
| 0.89 | Are student protests being manipulated ahead of France's p… | French police accused over war-like injuries to teenagers | both different | yes (2 outlets) |
| 0.86 | Ukraine says its Starshield access is back | Wes Streeting urges US to back Ukraine on Washington visit | one different | yes, with the Starshield event (2 outlets) |
| 0.98 | Will Israeli voters punish Benjamin Netanyahu? | In Israel, Netanyahu's ultimate legacy is the opposition… | one different, one unsure | no (1 outlet) |

The first three are on the test computer. The paper's one membership
both judges called different is tesla.com/roadster, titled only "Tesla
Roadster", joined to the Roadster reveal at p 0.98. The orchestrator
reads that as a correct member: it is the reveal's official page, which
the title alone does not show.

## What the evidence shows

- Across both computers the band **0.85 ≤ p < 0.92** holds exactly 3
  joined memberships. All 3 are false merges: topic merges of live
  blogs, protest coverage, and one country's news.
- All 3 create corroboration the event would not have without them: 3
  of the test computer's 14 corroborated events.
- Every membership both judges agreed was the same event has p ≥ 0.92.
  The minimum is 0.92 on the test computer and 0.98 on the paper.
- The paper has no joined membership in the band. Its 4 corroborations
  are unaffected.
- The remaining failure above the band is opinion pieces (Netanyahu,
  0.98). It is not corroborated.

## Classification

This is substrate: the event layer's join threshold (`eventJoinP`,
`internal/agentcore/event_layer.go`). The threshold was set at 0.85 by
the design (§5) before any staging data. V3 wakes management on
corroboration, so a false corroboration becomes a false wake and a story
opened on one outlet's report. The design states the bias: "the right
bias for a gate whose failure mode is false corroboration" (§12).

## Decision

**V3 event wakes stay off on the paper.** V1 has not passed.

## Fix direction

Raise `eventJoinP` to 0.92. A match from 0.5 up to 0.92 becomes a
tentative member: visible, not counted, and never minting a rival event.
The same bound governs two new captures forming one event.

Ways it could fail:
- **Missed merges:** true same-event matches between 0.85 and 0.92 stop
  counting. There were none in this data, but n is small.
- **Circular acceptance:** the threshold was chosen on this sample. V1
  acceptance must be rerun on a fresh window after the fix.
- **Existing memberships:** tentative is stored at write time, so the
  test computer's three band members stay counted. The paper has none.

## Fresh-window rerun after the fix (13:20 UTC): fails again, and not at marginal p

78ae91887 raised the join threshold to 0.92 at 07:31. The rerun sampled
only memberships resolved after 07:31: every distinct-URL join on both
computers, 21 on the paper and 21 on the test computer. Same two judges,
same blind protocol.

| | Paper | Test computer | Both |
|---|---|---|---|
| Both judges DIFFERENT | 2 of 21 | 2 of 21 | **4 of 42 (9.5%)** |
| gpt-6.1-sol DIFFERENT | 2 | 2 | 4 |
| gemini-3.8-flash DIFFERENT | 5 | 7 | 12 |

The four both-judge disagreements:

| Join p | Event | Member | Orchestrator's reading |
|---|---|---|---|
| 0.98–1 | Tesla reveals SpaceX-powered Roadster (TechCrunch) | tesla.com/roadster, "Tesla Roadster" | correct: the reveal's official page (as before) |
| 0.93 | "What is your personal recession indicator?" (Guardian) | "Bad news for Trump: it's still the economy" (Guardian) | false: one topic |
| 0.98 | Ship attack reports near Hormuz at a record high (BBC) | Two container ships sink near Hormuz (Guardian) | a developing story, not one occurrence |
| 1.0 | Indonesia fights fire with dry ice as haze spreads (Guardian) | Wildfire kills 4 firefighters at Mount Bromo (DW) | a fire season, not one fire |

The 0.93 join formed between two captures in one batch: the opener chose
its sibling at 0.93. The member's stored p (0.74) is its own answer's
probability, not the join's, which makes `member_list.p` misleading for
sibling-formed events.

**What this shows:** the remaining false merges are not marginal. Jev
merges the developments of one ongoing story at p 0.98–1.0. The judges,
given the same event definition, read "event" as one occurrence. No
threshold separates them. The disagreement is the definition of an event
(occurrence or developing story), so the next step is a design decision,
not a parameter.

Options, for the owner and a panel (not tuned here):
1. **Tighten Jev's event definition** in its prompt (same incident, place
   and time; a later development of the same story is a new event linked
   to it). Then rerun V1 on a fresh window.
2. **Adopt story-level events** and require corroboration to name the
   same occurrence by a second check, for example a counted origin needs
   p ≥ 0.97 and a same-incident verification call.
3. **Count corroboration only from captures resolved in the same flush
   window**, since developments arrive later than the first reports.

**V3 event wakes stay off on the paper.** V1 has failed twice; turning
wakes on would wake management on story-level merges.

## Owner ruling and plan: option 1, an event is one incident (2026-10-20)

Ruling (docs/reports/owner-notes-2026-10-14.md §27 item 8): "One event is
one incident, one place, one time; later developments are new linked
events. Rerun the blind sample on a fresh window." Option 1 above is
adopted. Options 2 and 3 are not pursued; they would have kept story-level
events and tried to filter corroboration afterwards.

Root cause, from the code. Jev's event definition lives in one place,
`eventQuestion` (`internal/agentcore/event_layer.go`): "the same
occurrence or its direct development (a later announcement, reaction or
outcome of it)". The blind judge script (`v1_blind.py`, which lives in a
session scratchpad and is not in the repo) carried the same sentence. So
the 0.98-1.0 merges were not Jev misreading its definition: the definition
itself says a later development is the same event. A developing story
(Hormuz ship attacks, the Indonesian fire season, a live blog) is by that
text one event. No threshold can fix a definition.

Plan (all in the shadow event layer, `intake.events`, off by default):
1. **Definition.** One shared definition text for the Jev task, its
   question and the blind judge: one incident, in one place, at one time.
   Updated detail about the same incident is the same event. A later
   development (reaction, response, investigation, outcome), another
   incident of the same story, place or actors, a live blog or roundup
   spanning incidents, and opinion or analysis about a topic are new events.
2. **Link.** `store.WorldEvent` gains an optional `follows` event id
   (additive JSON field on an immutable object; no migration, older readers
   ignore it). A capture that Jev answers as a later development of a
   known event mints a new event with `follows` set to it. The link is
   evidence for the reader and never counts toward corroboration. Jev is a
   choice judge, so the question offers, per capture, up to three extra
   choices `after_eN` for the candidate events that share the most title
   words with that capture; this bounds the request growth that matters
   for the "calls per flush" bar.
3. **Rerun.** `scripts/event-blind-sample.py` (new, in the repo; it
   replaces the scratchpad `v1_blind.py`) with the new definition, on a
   fresh window, per the procedure in the design doc (§19.3).

Mutation class: **red** by ceremony. The change is orange on its face
(shadow event layer, additive schema field) but it changes the content of
every Jev gateway call, which CLAUDE.md lists as a protected surface.

- Conjecture delta: the false merges are definitional, so a one-incident
  definition lowers the story-level merge rate below 5% without a
  threshold change. Refuted if the fresh blind sample is above 5% again
  with the new judge text.
- Protected surfaces: gateway/provider calls (Jev request content). The
  store object body gains an optional field. No Texture, tape, auth or vmctl
  path.
- Admissible evidence: unit tests prove plumbing only (what Jev is asked,
  what a linked answer writes, that true duplicates still merge and
  corroborate). They cannot prove that Jev reads the new text correctly.
  Only the staging blind sample can; it is the acceptance evidence.
- Rollback: set `intake.events` false, or revert the commit. Events written
  with `follows` stay valid for older code.
- Heresy delta: discovered, the blind judge definition and the resolver
  definition were duplicated by hand across a scratchpad script and Go
  (introduced a shared text plus a sync test; repaired). Introduced: none
  known. Repaired: the "direct development" clause.

Ways it could fail (written before the tests and the code):
- **F1 Story-level merge survives:** the Jev text still tells it to join
  developments. Test: the request carries the one-incident definition and
  no "direct development" wording, in the task and in every question.
- **F2 True duplicates stop merging:** the stricter text, or the new
  choices, make two outlets' reports of one incident split. Test: the
  Eurotunnel-style pair from the fixtures still joins at p >= 0.92 and
  corroborates; so do updated-detail reports of one incident.
- **F3 A development merges as a counted member** (Hormuz ships, the
  Indonesia fire season, the Tesla recession pair, the Houthi live blog)
  and fakes corroboration. Test, fixtures from the tables above: when Jev
  answers `after_eN` the capture mints its own event, with `follows` set,
  and the earlier event's outlets and corroboration are unchanged.
- **F4 The link counts:** a linked event lends its outlets to its
  predecessor or the reverse. Test: neither event corroborates from the
  other's members.
- **F5 The link is unreliable or crashes the resolver:** `after_eN` with
  low p, an alias that does not exist, a sibling component with
  conflicting links, a link to an event of a retried mint. Test: below
  `eventTentativeP` no link is set; the strongest link in a sibling
  component wins; a retry writes the same event (identity is the opener).
- **F6 Request growth:** extra choices push calls past the 24 KB bound and
  split flushes (calls per flush >> 1). Test: at most three `after_`
  choices per capture, and a full 12-capture chunk with 40 candidates
  stays within one split of today's call count.
- **F7 Judge/resolver definitions drift** again. Test: the blind script's
  judge prompt contains the Go definition text verbatim.
- **F8 Circular acceptance:** the sample window overlaps one the text was
  chosen on. Mitigation: the rerun samples only memberships resolved after
  the deploy time of this change (`SINCE`).

Implemented (2026-10-20): `eventDefinition` and `eventLinkChoices` in
`internal/agentcore/event_layer.go`; `WorldEvent.Follows` in
`internal/store/world_event.go`; tests in
`internal/agentcore/event_layer_incident_test.go` (fixtures: the seven
story-level pairs from the tables above and four true-duplicate pairs).
Not yet deployed and not yet measured; the acceptance evidence is the
blind rerun. Known limit of the link: a pair with no shared title word
(France protests, the recession pair, the Indonesia fires) is offered no
`after_` choice, so it is a new unlinked event. That is correct for the
bar and loses only the link.

## Preliminary rerun under the one-incident definition (2026-10-20 09:10 UTC), short

Procedure: design §19.3, window from the paper's planned refresh onto
31a5c2bc4 (`--since 2026-10-20T03:05:00Z`), judges `openai/gpt-6.1-sol` and
`google/gemini-3.8-flash`, output in the session scratchpad (not in the repo).

- **Short sample.** The paper had 12 distinct-URL joins in about 6 hours
  (the previous window gave 21 per computer). The test computer
  `v3-events-0146` (computer-3191b29f…) returned 403 from
  `GET /api/autopaper/events` and is not in the vmctl list; it was not
  among the computers the 2026-10-20 tape sweep removed (no archive). So
  this read is the paper alone, n = 12, and the bar cannot be decided on
  it: one merge is 8.3%.
- **Judges:** gpt-6.1-sol 2 of 12 DIFFERENT, gemini-3.8-flash 3 of 12; 2 of
  12 with both DIFFERENT (16.7%).
- **Orchestrator's reading of the two all-DIFFERENT rows:**
  - `ev-65b0d8ae…` OpenBSD: a bsd.network post on what the 8.0 release will
    carry, joined to `openbsd.org/80.html`, the 8.0 release page. Same
    release; the bare title "OpenBSD 8.0" hides it (the tesla.com/roadster
    case again). Counted correct.
  - `ev-f3b7b3dd…` Kansas: the event's **opener is a Guardian live blog**
    ("US politics live", `/us-news/live/2026/oct/19/…`), and a Guardian
    report on the Kansas rally joined it at p 0.98. The ruling makes a
    live blog spanning incidents its own event, so this is a false merge.
  - Reading: 1 of 12 (8.3%), above the bar, on a sample too small to judge.
- **Hypothesis, not confirmed:** live pages open events and then attract
  joins, the same failure the related-stories walk had last night
  (`docs/problems/related-stories-dead-end-2026-10-20.md`, `isLivePageURL`).
  Candidate fix, not made: a capture whose URL is a live page never opens or
  joins a counted event (it gets a singleton event, or no event). Confirm on
  a full sample first.
- **Next:** rerun on the paper alone at about 15:00 UTC (12-hour window,
  expect about 24 joins), and decide whether a second test computer is
  worth minting for the pooled 40.

## The 15:00 UTC rerun under the one-incident definition (2026-10-20), short

Window 03:05 to 15:00 UTC on the paper; `scripts/event-blind-sample.py`,
judges `openai/gpt-6.1-sol` and `google/gemini-3.8-flash`, output in the
session scratchpad (not in the repo).

- **Sample:** 20 distinct-URL joins (short of the 40 the bar assumes; not
  padded with older joins).
- **Judges:** each said DIFFERENT on 8 of 20, the same 8 rows; 40% by the
  script's count.
- **Orchestrator's reading of the 8, from titles and URLs, under the
  one-incident definition:**
  - False merges (an analysis, roundup or topic piece joined to an
    incident), 3: "DOJ charges 28 with voting crimes. And, TikTok …" (a
    newsletter roundup) with "Federal efforts over alleged noncitizen
    voting escalate"; "Seventh day of violent clashes between Albanian
    protesters …" with "'Horizontal revolt': Why Albania's young protesters
    want …" (analysis); "JB Pritzker is turning midterm money into potential
    2028 …" with "Is JB Pritzker positioning himself for a 2028 …" (two
    pieces about a topic, not an incident).
  - Arguable, 3: "Google cloud outage takes down banks" with "Millions unable
    to use Revolut app or cards …" (the cause and one of its effects); two
    Kansas pairs: "At Kansas rally, Trump focuses closing midterm message"
    with "In ruby-red Kansas, Trump contemplates …" and with "WATCH: Trump
    heads to rally in Kansas …" (a preview video before the rally).
  - Judge errors by title, 2: "Helen Haines loses a leg and surgeons fight to
    save the …" with "Helen Haines loses leg as surgeons fight to save the
    other …"; "Google says 'power event' that disrupted Cloud services" with
    "Google Cloud says a 'power event' caused today's outage". The script
    keeps only the first SAME/DIFFERENT/UNSURE word of each answer and not
    the answer text, so a misparse cannot be ruled out; that is a gap in the
    instrument (it should keep the answer).
  - Reading: 3 to 6 of 20 false merges, 15% to 30%. Far above the 5% bar
    either way.
- **Decision (No Blocking Asks, the conservative option):** phase 2 event
  links do not show as "same incident". The event layer keeps running (it
  feeds the paper's story assembly), but related stories do not read it as
  a same-incident relation.
- **What the wrong joins are:** every false or arguable merge above is a real
  *related* pair: the same running subject (Albania's protests, Pritzker
  2028, Kansas), a development of an incident, or a roundup that includes
  it. They fail "same incident" and would pass "related story". That is the
  case for phase 3's typed relation (same incident, development, same
  thread, shared central entity) judged per pair, rather than one event
  identity that has to mean both
  (`docs/reports/related-stories-graph-design-2026-10-19.md`).
- **Next for the instrument:** keep each judge's full answer in the output,
  and parse the last verdict word, not the first.
