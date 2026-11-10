# Why unsupported citations publish, why duplicates open, why stories are revised so often (2026-11-10)

Mutation class: green (docs). Read-only investigation at `b316350d1`.
It answers the three open questions lane Q could not settle from counts
(`autopaper-article-quality-2026-11-09.md`, audit
`docs/evidence/autopaper-source-audit-2026-11-10/`).

## Method and what could not be read

- Public pages only: `GET /api/platform/publications/resolve?route=<permalink>`
  for the latest version and for each older version the page still serves
  (`/v<n>`; older versions are served, and a missing number is a gap, a
  revision that committed and did not publish. v1, the seed, is always one).
  At most 3 requests a second. The audit's story cache supplied the 24 h
  window (749 stories). No article text is kept here; stories are named by the
  paper's own slug suffix and a short title.
- The paper's observability read was refused for the d@e key (HTTP 403,
  "api key computer ownership required"). No other credential was tried. So
  **no run ledger, no turn-commit reason, no label or Jev judgment was read.**
  Anything about why a writer did or did not act is a hypothesis below.
- Two `journalctl` reads on node-b (corpusd). They carry no per-story or
  per-verdict line, only "story entities" lines. They settled nothing.
- Each published version carries `model_provenance` (calls per role, so the
  count of writer and Jev calls up to that version) and each check carries
  `checked_at`. These two public fields are what made the call counts and the
  judge-versus-publish timing readable.
- Analysis scripts are in the scratchpad only (not committed). Numbers below
  are from the 24 h window unless stated.

## Q1. Why unsupported citations still publish

### Five cases

1. **Nvidia shareholders sue (pub76dfcebfa).** v3 at 01:33:01 is 110 words,
   3 cites, 0 failed. Research ran (64 research calls). v4 at 01:40:03 is 303
   words, 16 cites, 10 failed. The failed checks are stamped 01:39:38, 25
   seconds before the version published. 68 minutes later the story is still
   v4 with 10 failed. The opening sentence ends with two cites side by side.
   The first quote says outright that Nvidia and its CEO were sued on that
   day in a proposed class action; the check on that pair failed at
   p(supported)=0.13. The sentence also names the court, which the second
   quote carries. Each quote is judged against the whole sentence alone.
2. **Sydney attack teenager (pub815fb44f3).** v2 at 01:48:36: 17 cites, 14
   failed. v3 six minutes later: 20 cites, 9 failed. Still v3, 9 failed, 54
   minutes after. The wake produced a successor within 6 minutes (so it was
   delivered), and the successor still failed on 9.
3. **Border wall construction (pub294b525b9).** v6 at 01:54:47 and v7 at
   02:00:14 have the **identical content hash**. Failed checks: 2 in v6, 2 in
   v7. The writer was woken (checks on 01:54:44), took a turn (one more
   writer call in v7's provenance), committed, and published the same text.
   So here the wake arrived and the correction did not happen.
4. **Pope Leo in Buenos Aires (pub6c61fdfc5), a success.** v3 5 failed, v4 1
   failed 5 minutes later, v5 0 failed 3 minutes after that. Send-backs work
   when the failures are few and single-cite.
5. **Claim of a conviction goes unverified (pubbe6cd0b97).** v3 only, 137
   words, 4 cites, 3 failed, no successor after 63 minutes. The story reports
   that no source confirms a claim; its cites are to sources about the
   absence. A "supports the sentence" judge is likely to fail such sentences
   by design. This is a different class from cases 1 and 2.

### What the 24 h data say (public, all 749 stories)

| Candidate cause | Result | Evidence |
|---|---|---|
| The audit counts checks of superseded versions | **Rejected** | The audit reads each story's latest version; the overlay is per version. |
| The judge lags publication | **Rejected** | 297 of 305 failed checks were judged **before** the version published (seconds to minutes). Only 8 after. Pending checks in the 24 h window: 30. |
| The wake is dropped | **Mostly rejected** | Of 55 stories with a failing version (since 11-09 14:00), 48 failing-to-next-version steps had a median 5 minutes to the next version. The wake arrives. |
| The writer's correction fails again | **Supported** | Of those 48 steps: 3 cleared every failure, 20 reduced them, 25 kept or increased them. Case 3 republished identical text. |
| No turn budget / settled story | **Unknown** | Stories with 9 or more failures (cases 1, 2) had no successor for an hour. Needs the run ledger. |
| The judge itself fails correct citations | **Supported (see below)** | Failure rate by cites per sentence. |

### The main cause: side-by-side cites are each judged against the whole sentence

`internal/texturewrite/checks.go:114-122` (`citingSpans`) gives every
citation the words since the previous citation, and cites side by side "close
the same span, so a sentence two sources both support is judged against each
quote alone". `Checks` (`checks.go:139-168`) makes one pair per cite with that
span as `Text`; `askJevForSupport` (`internal/agentcore/support_judge.go:258-300`)
asks "does state.pairs.pN.cited_passage support what ... sentence says", and
p below 0.5 is "unsupported" (`support_judge.go:320-324`).
A sentence that two outlets together support (one gives the date and the
action, one gives the court) fails against each quote alone.

Measured from the stored cite anchors of all 749 stories (a sentence group =
the run of cite anchors with no words of their own between them):

| Cites in the sentence group | Checks | Failed | Failure rate |
|---|---:|---:|---:|
| 1 | 7,622 | 122 | 1.6% |
| 2 | 936 | 135 | 14.4% |
| 3 | 165 | 40 | 24.2% |
| 4+ | 28 | 8 | 28.6% |

Multi-cite groups are 13% of checks and 60% of failures (183 of 305). 59 of
the 142 failing stories fail only on multi-cite groups; 59 fail only on
single cites; 24 on both. The same mechanism makes "[1] [1]" runs (audit
finding 5) cost two judge questions each.
The 2026-10-19 problem doc named this for whole paragraphs and the span
change fixed it for separated sentences, not for side-by-side cites.
Confirmed: the code and the correlation. Not shown: that the judge is wrong
on a given pair (that needs the pair text and a second judge). Hypothesis:
the writer cannot clear these by editing, because splitting the sentence is
the only fix and the prompt does not say so. That explains the 25 of 48
send-backs that did not improve.

### Why a publish-time hold looked impossible, and why it is possible

The brief says the verdict is written after the commit. True, but the publish
is also after the commit and queued: `AutoPublishRevision`
(`internal/agentcore/story_publish.go:187-275`) enqueues the publish, and
`JudgeRevisionSupport` (`support_judge.go:88`) starts the judge job, both
off the commit. The judge wins the race by 4 to 25 seconds in the cases read
(Nvidia 25 s; a Tokyo story 8 s). The verdict is on the computer before the
version reaches the platform. A hold needs only to make the publish job wait
for the revision's own judge job.

### Fix at the substrate (two parts; do part A first)

**A. Judge a sentence's side-by-side cites together.** One question per
span: the sentence and **all** its quotes (`cited_passages`). A "yes" marks
every cite of the span supported. A "no" is one failure with the span, listing
the quotes. Pair keys stay per cite for the memo; add the span's group key.
Code: `texturewrite/checks.go` (group), `support_judge.go` (one question per
group, same single Jev call), `store/support.go:150-200` (one failure per
group), the platform overlay (fan the group verdict out to each anchor).
- Moves: failed checks 305 to about 120 (the single-cite floor of 122);
  stories with a failed check 19% to about 11% (142 to about 83).
- Cost: **zero extra calls.** One Jev call per commit as now; about 550
  questions instead of about 1,100 for multi-cite checks.

**B. Hold a version whose own verdicts failed.** `AutoPublishRevision`'s
enqueued job waits up to 45 s for the revision's judge job; if any verdict is
unsupported it does not publish. The older public version stays up and the
send-back (already minted by `store/support.go:178-205`) produces the next
version. No new record, no resumer, no switch.
- Moves: "latest published version carries a failed check" toward 0 for
  stories with a clean earlier version.
- Cost: zero calls. Reader-visible updates are delayed by the send-back time
  (median 5 min).
- Failure list:
  - The judge times out or has no slot (`supportJudgeQueueWait`, 2 min): the
    pairs stay pending; publish as today after the 45 s wait.
  - The writer never clears: the story's first version never publishes (no
    story, which is better than an unsupported story); a later version
    stays behind its older clean one. After two held rounds, close the
    send-back with a visible fate and let the next capture restart it.
  - Memoized verdicts (an unchanged pair keeps an older verdict): count them,
    they are real failures of this text.
  - A crash between commit and publish leaves a gap: no change from today.
  - Hold hides a story that publishes in the paper's slow hour: accept; the
    audit's number is what readers are shown.
  - Story about absence (case 5): its sentences fail by design. Part A does
    not fix it. Tell the writer to cite an absence by what the source says
    (a prompt line, cost zero), and let the hold stop the rest.

### Settling observations for the unknowns

- Why large failure sets get no successor in an hour: the Texture run
  ledger for case 1 and 2 after 01:39 (wake delivered? turn started? Write
  committed? stop reason).
- Whether case 3's identical-text revision was a refused or no-op Write:
  the turn-commit reason for the run that produced v7.

## Q2. Why duplicates open

### Five pairs (all six read, one extra)

In every pair the second story was opened from **different captures** of the
same event (0 to 2 shared cited URLs, other outlets), so capture dedup could
not see it.

| Pair (pub suffixes) | First story first served | Second story first served | Gap | Other stories updated between (lower bound) | Shared cited URLs at first version |
|---|---|---|---|---|---:|
| Border wall: a498a8085 / 294b525b9 | 11-09 22:41 | 11-10 01:38 | 3.0 h | 98 | 0 |
| Saudi pipeline: a0717c88c / 718731b6b | 11-09 22:59 | 11-09 23:39 | 40 min | 35 | 1 |
| Canada tariff relief: f30174606 / c8075af36 | 11-09 22:31 | 11-09 23:58 | 1.4 h | 57 | 2 |
| Anthropic model launch: 50989f615 / 720b27264 | 11-09 21:13 | 11-10 00:34 | 3.3 h | 122 | 2 |
| Massie impeachment vote: 7a15b1cad / 46d0cfc15 | 11-09 19:11 | 11-10 01:40 | 6.5 h | 38 | 0 |
| Russell Brand sentence: 166ce9b3c / 882feeb18 (a third, 20ca9e81e, is earlier) | 11-09 14:19 | 11-09 17:50 | 3.5 h | 34 | 1 |

The Massie pair was not in the audit's title-word groups (different words);
the audit's 8 groups are a lower bound. Two of the 8 groups are false
positives by title words (different Kerberos CVEs; two different Fiji
earthquakes). About 6 of 8 are real.

The paper updates about 33 stories an hour on 11-09 (hourly counts 14 to 46;
749 stories in 24 h). "Other stories updated between" counts stories whose
latest update lies between the first story's last update and the second's
opening, so it is a lower bound on how far down the recency list the first
story had fallen.

### Causes, ranked

1. **Windows are counts, and the paper's rate makes them short (arithmetic
   confirmed, mechanism per pair a hypothesis).** The intake label
   shortlist is the 30 newest stories by update plus 10 title-word picks
   (`internal/agentcore/intake_labels.go:50-52`, `:87-100`). At 33 updates an
   hour, 30 is about 55 minutes. Management reads `choir.Stories(50)`
   (`management_tasks.go:68`), about 90 minutes. In 5 of the 6 pairs, 38 or
   more other stories had moved since; the first story was outside the
   30. Saudi (35) is at the edge. The title-word slots (10 of about 2,000 in
   72 h) rank by shared words; a second outlet's headline wording differs
   ("attacked" vs "targeted", "stays" vs "lets").
2. **The event layer cannot rejoin a story-bearing event (code confirmed;
   whether it drives openings depends on a setting I could not read).**
   `eventShortlist` (`event_layer.go:598-650`) takes the 30 newest events
   inside 72 h. Its overlap slots, the only route to older events, skip
   corroborated events: `if picked[v] || v.corroborated() { continue }` at
   `event_layer.go:629`. A story needs two outlets, so every story's event is
   corroborated. After about 30 events pass, that event cannot be matched by
   any later report, which mints a rival event (join also needs p of 0.92,
   `:63`, else the capture is tentative and mints nothing). The file's header
   still calls V1 a shadow, but `event_wakes.go` builds management tasks from
   events when `intake.event_wakes` is on. Whether it is on for the paper is
   unread.
3. **Jev says "new" or management declines to feed (unread).** Settled by the
   label and judgment record: lane A now records every Jev judgment. Observation:
   for each second story's first capture, the candidate list shown to Jev
   (was the first story in it?) and the answer.
4. **Race (unread).** Two tasks read the library before either opens
   (`checkStoryLibrary`, `store/management_tasks.go:638`, refuses only when
   the library moved since the read). Possible for Saudi (40 min).
   Observation: the two opening commits' library head.

### Where the fix belongs

The event is the unit that knows an event across outlets and time; the story
list only knows titles. Fix there, with the platform as a backstop that alone
knows the published set.

**D1. Event layer: let corroborated events into the overlap slots.** Remove
`v.corroborated()` from the skip at `event_layer.go:629`, give corroborated
events their own overlap budget (say 10) over a 72 h horizon, scored by title
words plus numbers and capitalised names. Carry the event's story link so a
capture that joins a story-bearing event is fed to that story (the arrival
record already names the event).
- Moves: duplicate groups 8 to 3 or fewer per 24 h (the pairs here are all
  later reports of corroborated events).
- Cost: zero extra calls; about 10 more candidates in a Jev state capped at
  24 KB (`event_layer.go:62`).
- Failure list: a topic merge at the marginal join (the history of
  `event-layer-false-merges-at-marginal-p-2026-10-16.md`; keep 0.92); an
  event with later developments (design: later developments are new linked
  events, so a follow-up story must still open: a strong "development" signal
  is needed); the Jev state grows and slows (cap the extra by bytes); a
  corroborated event whose story was declined (no story to feed); restart
  loses the index (rebuilt from the store on first use, as now).

**D2. Windows by time, not count.** Make the intake shortlist and the
management page the stories of the last 6 hours (cap 150 titles, one line
each), not the newest 30 or 50. Cost: about 100 more title lines of context
per capture (Jev state under 24 KB at 80 characters a line); no extra calls.
Failure list: Jev recall falls with a longer list (73% now; measure on the
lane A records before and after); timeouts rise (the 5 s per-call budget,
4.9% unlabelled today).

**D3. Platform publish-time check.** When the first version of a new story
arrives, the platform compares it with the published set of the last 48 h:
a shared cited URL, or title-word similarity of 0.5 or more with a shared
named entity or number. On a hit it refuses that first version with the
older story's slug; the refusal returns to the computer as a record and
management feeds or declines (the computer already handles a platform
refusal as a held version, `story_publish.go:440-452`).
- Moves: what still gets through D1 and D2; the audit's duplicate groups.
- Cost: zero calls to check; one management turn per hit (about 8 a day).
- Failure list: false hits (Kerberos CVEs and the Fiji quakes would hit by
  title words: require a shared named entity or a shared URL and a number
  match); a correct hit when the later story is a real development (the
  management turn decides, and can force with a visible reason);
  concurrent firsts (two firsts in the same second: lock per 48 h title
  bucket); the refusal itself is a loop if the writer re-opens (one
  refusal per story, then publish).

## Q3. Why stories are revised so often

### Five cases (versions read from the public history)

Class per step: *send-back* = the previous version carried a failed check;
*new source* = more source entities; *identical* = same content hash.

1. **Massie impeachment vote, pub7a15b1cad: v14.** v1 and v2 were gaps, v8
   was a gap. 11 served versions. Steps: 4 send-backs, 5 new-source, 1 other.
   Writer calls 7 at v3 to 36 at v14: about 2.6 per revision. Sources 3 to
   10, words 208 to 452. Similarity of each step to the previous is 0.93 to
   0.99. v11 to v12: failed 1, then +14 words with the same sources, clean.
2. **Pope Leo in Buenos Aires, pub6c61fdfc5: v13.** v3 106 words to v13
   297. Failed 5, 1, 0 in the first three versions; later steps added
   sources (2 to 10). About 1.6 writer calls per revision. Reader-visible:
   yes, the story grew.
3. **Senate votes on the permitting bill, pub0a8734cc0: v11.** Steps after
   v4: 5 send-backs, 2 other. Words went 348, 381, 361, 337, 312, 322,
   319, 324: three steps in a row trimmed text after a version with 4 failed
   (v5 and v6 each carried 4).
   17 writer calls from v4 to v11.
4. **Border wall, pub294b525b9: v7.** v6 to v7 identical content hash (case 3
   above); 3 send-backs in 4 steps, 12 writer calls.
5. **Hegseth impeachment (the second story), pub46d0cfc15: v8.** 5 served
   versions, 4 send-backs in 4 steps, 24 writer calls first to last, and it
   is a duplicate of case 1 (the same event revised twice over).

### The classes across 26 stories with latest revision 6 or more (95 steps)

This sample is biased to high revision; read the shares as a direction.

| Class of step | Steps | Share | Median similarity | Median word change |
|---|---:|---:|---:|---:|
| Preceded by a failed version (send-back), same sources | 44 | 46% | 0.95 | +2 |
| Send-back and a new source | 4 | 4% | 0.93 | +16 |
| New source, no failed version | 24 | 25% | 0.93 | +25 |
| Rewrite, no new source, no failure | 11 | 12% | 0.86 | +25 |
| **Identical text republished** | 8 | 8% | 1.00 | 0 |
| Tiny edit, no new source | 4 | 4% | 0.99 | +4 |

Per revision, the provenance deltas show a median of 2 and a mean of 2.4
writer calls and 0.9 Jev calls (the judge). So a send-back round costs about
3.3 calls. Research calls do not repeat per revision. 72 of 193 slots in the
sample are gaps: revisions that committed (and cost calls) and never showed.
So "latest revision 4" counts unseen revisions as well; the reader sees fewer.

### Cause and the largest avoidable class

The largest class is the **support send-back** (50%): the failed verdict mints
a record that wakes the writer (`store/support.go:178-205`), the writer
revises, the new revision is judged, and the mechanisms in Q1 make it fail
again. Cause in code: Q1's per-cite judging (`checks.go:114-168`) plus no
hold, so each round is a published version. This is a **trigger inferred from
timing** (the next version arrives in a median 5 minutes after a failed
one); the run ledger would confirm that the wake started the turn.
Second: **identical text republished** (8%): a Write that changes nothing
still commits and publishes. Not read: where `Write` could refuse an
empty change (a candidate is `internal/texturewrite/apply.go`; the Write
refusal rule is in `texture.yaml:22`, "a refused Write names its line and
commits nothing"). New-source steps (25%) are real improvements and stay.
Management delegation and owner acts are not visible on public pages.

### Fix at the substrate

- **Remove most send-backs with Q1 A and B.** Moves: stories' mean latest
  revision from 4.4 toward 3 (the 50% class is about 1.5 steps a story in
  the sample), calls per day by about 3.3 times the send-backs removed.
  Cost to build: none extra.
- **A no-op Write is refused:** if the new body equals the base, the Write
  commits nothing and says so. Moves the 8% class to 0 and saves 1 to 5
  writer calls per instance. Failure list: a Write that changes only a cite
  (new quote, same text) is a real change: compare the text and the cites,
  not the text alone; a refused Write may loop the model (the prompt already
  tells it a refusal commits nothing; limit refusals per turn to the
  existing turn budget).
- **Do not build a materiality gate for new-source steps now.** They are
  real, they add 25 words and a source, and the rewrite class (12%) is
  smaller than the send-back class.

## Ranked: what to build first tonight

1. **Q1 part A, the group judge** (then part B, the hold). It has a confirmed
   cause with a measurement behind it (1.6% against 14 to 29% failure). It
   cuts the audit's failed-check stories from 19% toward 11% with no extra
   call, and because send-backs are half the revision steps it is also the
   biggest cut to Q3. Part B is a few lines in `AutoPublishRevision` and
   takes the number toward zero.
2. **Q3's no-op Write refusal.** Small, local, cuts the 8% class.
3. **Q2 D1** (event layer: corroborated events in the overlap slots). One
   confirmed line, zero calls, but it depends on whether `intake.event_wakes`
   is on for the paper, which I could not read; check that first.
   D2 and D3 after the lane A records say whether Jev or the window failed.
4. **Q3's rewrite class and Q2's race:** not enough evidence; wait for the
   run ledger.

Audit numbers to move, and the run that shows it: stories with a failed
check 19% (24 h); duplicate groups 8; mean latest revision 4.4 and the share
of steps preceded by a failed version (50% in the sample). Rerun
`scripts/autopaper_source_audit.py` six hours after the deploy.

## Build: decisions and failure list (2026-11-10)

Mutation classes: item 1 red (support verdicts, a store failure record),
items 2, 3 and 4 orange. Built on lane A's Jev branch (`lane-a-jev`), whose
concurrency (`internal/fanout`, `judgeCall`, the Jev record) is used and not
touched. Rollback for each: revert its commit. Fixes B and D3 are designed
below and not built.

### 1. Side-by-side cites are judged together (fix A)

Decisions.
- `texturewrite.Checks` groups the cites that close one span (the same
  `citingSpans` rule, now with a group number). A pair whose group holds
  more than one distinct quote carries `CitationPair.Group` (every quote of
  the group, its own included). Its `BlockHash` is then
  `texturewrite.GroupHash(span, group)`, so `Key` (derived from `BlockHash`
  by the store's `validSupportPair`) names the span and the whole group: a
  group with a cite added, dropped or swapped is a new question, an unchanged
  group stays memoized, and a lone cite keeps its old span key and its old
  memo.
- The same quote cited twice side by side is one piece of evidence and one
  pair (the audit's "[1] [1]" costs one question, not two).
- A group is chunked at 6 quotes (`maxGroupCites`), in order; each chunk is
  its own group.
- The support judge asks one question per group: the sentence and the
  group's passages as `cited_passages` (source, quote, passage), with the
  instruction that the passages between them must state or entail the
  sentence, that an irrelevant passage does not count against it and that no
  single passage has to carry every part. One Jev call per revision as
  before; fewer questions (about 8,150 a day instead of 8,750 at the 24 h
  window's rate). The passages of one question share 12,000 runes
  (`supportGroupEvidenceRunes`), each cut to a window around its quote when
  longer; a lone cite's passage is unbounded as before.
- Records. Every member pair of a group gets its own verdict memo, so the
  platform overlay, which reads per pair and per anchor, needs no change. The
  verdict records `GroupSize` (the number of quotes judged together; absent
  for a lone pair), so a "supported" with `group_size: 3` is
  supported-as-group, and an "unsupported" says in `why` that the N passages
  together do not support the sentence. The send-back is one failure per
  group (store `planSupportVerdicts` keeps the first failing member of a
  group), and the Texture frame renders it as the span followed by "cites
  together" and every quote of the group, with the header telling the writer
  that cites listed together are judged together.

Failure list and tests (`internal/texturewrite/group_checks_test.go`,
`internal/agentcore/support_group_test.go`).
- G1 a group of 2, 3, 4 or 5 cites is not one group on every member
  (`TestChecksGroupSideBySideCites`) or costs more than one question
  (`TestGroupedCitesAreOneQuestionAndTakeTheGroupsVerdict`: one call, one
  question).
- G2 a lone cite gets a group, or loses its old key
  (`TestChecksLoneCiteStaysSpanKeyedAndSeparatedCitesAreNotGrouped`).
- G3 cites separated by words are grouped (same test).
- G4 the same quote twice counts twice
  (`TestChecksTheSameQuoteTwiceSideBySideIsOnePair`); a group's verdict is
  not on every member or does not say it was a group's.
- G5 a changed group keeps its old keys; G6 an edit to another sentence
  re-keys an unchanged group
  (`TestChecksGroupKeyChangesWithTheGroupNotWithOtherSentences`).
- G7 more than six cites in one question
  (`TestChecksLargeGroupsAreChunked`); a lone pair beside a group is lost or
  judged as a group (`TestLonePairBesideAGroupIsStillItsOwnQuestion`).
- GJ2 one cite carries the claim and the irrelevant ones fail it (the
  "good" plus "other" case); GJ3 none supports it and the group passes (the
  "a, b" and four-cite cases).
- GJ5 a group with one unreadable or changed member is judged on partial
  evidence (`TestAGroupWithAnUnreadableMemberStaysPending`: it stays pending).
- GJ6 a failed group sends the writer one failure per cite
  (`TestAFailedGroupSendsTheWriterOneFailureNamingTheSpan`).
- GJ8 the evidence of a heavily cited span is unbounded
  (`TestAGroupsEvidenceIsBounded`, `TestBoundPassageKeepsTheQuote`).
- Not covered by a test, stated: a judge that says "yes" to a group a careful
  reader would split (a group lets a weak cite ride on a strong one; that is
  the rule, and the single-cite floor of 1.6% is the number to watch after the
  deploy); pairs listed before this change keep their keys and are judged
  alone.

### 2. A Write that changes nothing is refused (churn)

Decision. `commitTextureWrite` already refused a Write whose body bytes
equalled the head's; the audit's identical-text steps got past it because the
bytes differ (ids, offsets, spacing) while the text and cites do not. The
refusal now compares what a reader sees: the projected text with whitespace
collapsed, every citation's check (outcome and pair key, so a cite moved,
added, dropped or re-quoted is a change), and the embedded assets. It lives in
the desk-write commit function only (`writeChangesNothing`); a merge request
revision that carries the head forward with new parent links, an owner edit
and every other commit path do not pass through it. The refusal names its
reason ("the Write changes nothing (its text and citations equal the head's);
use choir.Wait or choir.Done"), commits no revision, publishes nothing and
spends no judge call. The loop bound is the turn budget, as the problem doc
said.

Failure list and test (`internal/textureowner/noop_write_test.go`).
- N1 the same text with different bytes (a repeated Write, extra spaces, a
  trailing newline) commits. N2 a Write that changes only a cite is refused
  (it commits). N3 a real text change is refused (it commits). N4 the refusal
  does not name its reason. N5 a refused Write leaves a revision behind.
- Residual: a text projection that renumbers citation markers differs from
  its base even when nothing else changed; that Write commits (the safe side).

### 3. Corroborated events may enter the overlap slots (fix D1)

Decision. `eventShortlist` keeps its 30 recent and 10 uncorroborated overlap
slots, and adds a budget of 10 overlap slots for corroborated events within
the 72 hour recency horizon (a corroborated event past 72 hours is not
offered: a later development is a new linked event by definition). Zero extra
calls; at most 10 more candidates, inside the existing byte bound, which drops
the last candidates first. The 0.92 join threshold is unchanged.

Failure list and test (`TestEventShortlistOffersCorroboratedEventsByOverlap`).
- E1 a corroborated event past the recency slots is not offered. E2
  corroborated events crowd out the uncorroborated overlap budget. E3 a
  corroborated event past the horizon is offered. E4 the extra budget is
  unbounded.
- Not built: carrying the event's story link into the feed decision (the
  arrival record already names the event). A topic merge at the marginal join
  is the history in `event-layer-false-merges-at-marginal-p-2026-10-16.md`;
  the threshold stays.

### 4. Candidate windows are time, not counts (fix D2)

Decisions.
- Intake shortlist: the stories updated in the last 8 hours (the pairs
  opened 40 minutes to 6.5 hours after their first), then up to 10 older ones
  by title overlap as before; a ceiling of 480 stories backstops the window
  (six chunks, seven with the overlap picks), and a floor of the newest 30 stories whatever their age keeps a quiet stretch from shrinking the list. The list is asked in chunks of 80 stories
  (`intakeChunkCandidates`), one call per chunk, concurrently through
  `fanout.Do`, with the whole poll's calls bounded by
  `intakeLabelConcurrency`. A chunk entry is a headline and a time (a lede on
  the newest 30 only), about 12 KB per call, under the 24 KB a Jev call is
  kept to. Aliases are numbered across chunks; the capture's new siblings ride
  the first chunk. Merging: a story answer beats a sibling beats "new"; among
  stories the highest p; "new" only if every chunk said new, at the lowest p;
  the topic is the highest-p story over chunks; usage is summed. A chunk that
  fails leaves the capture unlabelled (labelling "new" on part of the window
  would be the wrong answer).
- Management's story read: the page handed to a management task is the
  stories updated in the last 8 hours, never fewer than the newest 50, at most
  200 (`storiesEgressWithin`, `storiesWindow`); when it exceeds the section
  bound the task gets the newest 50 as before. `choir.Stories(limit, before)`
  with an explicit limit is unchanged.
- Cost. Questions per capture, event plus topic per call: before, 2 questions
  in 1 call over at most 40 candidates (30 newest, about 55 minutes at 33
  updates an hour, and 10); after, at the audit's 31 stories an hour, about
  250 stories in the window plus 10, so 4 calls and 8 questions over about 260
  candidates (7 calls, 14 questions at the ceiling). The Jev question limit
  measured in `docs/evidence/jev-measurement-2026-11-10.md` is not reached (2
  questions a call); what grows is calls and input tokens, about 4x. A poll of
  20 captures goes from 20 calls to about 80 against the 12 at once bound and
  the 15 s poll budget: a poll may leave its slowest captures unlabelled where
  it labelled them before. Unmeasured: Jev recall at 80 choices a question
  (73% at 40 today) and call latency at that size.

Failure list and tests (`TestIntakeShortlistTakesTheWindowThenTenByTitleOverlap`,
`TestIntakeLabelsAskALongWindowInChunksAndKeepEveryCandidate`,
`TestStoriesWindowPageHoldsEveryStoryOfTheWindow`).
- C1 the list is cut to one call's worth. C2 aliases collide across chunks.
  C3 chunks asked one after another, or past the concurrency bound. C4 a
  failed chunk gives "new". C5 a story in a later chunk loses to the other
  chunks' "new".
- W1 the window page is shorter than the page it replaces. W2 a busy window
  shows only 50. W3 the window is unbounded (200). W4 a limit read changes.
- Residual: a poll over the 15 s budget; recall at 80 choices. Lane A's Jev
  record measures both after the deploy.

### 5. Designed, not built

**B. Hold a version whose own verdicts failed.** Built after
`internal/agentcore/story_publish_quality.go` lands (another builder is
editing it); it touches that file and `internal/agentcore/story_publish.go`
(`AutoPublishRevision`'s enqueued job).
- Where: the publish job already runs off the commit. Before it publishes
  revision R it waits up to 45 s for R's judge job (a per-revision done
  channel kept by the judging state, closed when `judgePendingSupport`
  returns for R's pairs), then reads R's verdicts (`SupportVerdict` for each
  pair of R). The judge wins the race by 4 to 25 s in the cases read, so the
  wait is normally short.
- Rule: if any of R's own pairs has an unsupported verdict, the job does not
  publish R; the older public version stays up; the send-back already minted
  by `store/support.go` produces the next revision. Pending (no verdict by 45
  s) publishes as today. A memoized failed verdict counts (it is a real
  failure of this text).
- Bound (a guard, not a wall): a hold for failed support applies at most
  twice in a row per story. The third consecutive revision with failed
  verdicts publishes, and the exhausted hold is a visible record (a
  commitment record on the story's trajectory, "support hold exhausted after
  two holds; published with N failed checks", its refs naming the two held
  revisions), counted in the next audit. The counter is derived from the tape
  (the story's last revisions and their verdicts), not memory, so a restart
  neither loses nor resumes it; no resumer, no switch.
- Failure list: the judge job times out or has no slot (publish after 45 s);
  the writer never clears (the third round publishes); a crash between commit
  and publish (a gap, as today); a story about an absence (its sentences fail
  by design; the hold exhausts after two rounds); a held revision superseded
  before the hold ends (the newer one is judged on its own); reader-visible
  updates delayed by the send-back time (median 5 minutes).

**D3. Platform publish-time backstop.** Designed in D3 above, unchanged, and
built after D1 and D2 show their effect in the audit: a first version
compared with the 48 hour published set (shared cited URL, or title-word
similarity of 0.5 with a shared named entity or number), one refusal per
story (then publish), a lock per 48 hour title bucket for concurrent firsts.
It touches the platform publication path and the computer's handling of a
refusal (`story_publish.go:440-452`); neither is touched by this build.

### Audit numbers expected to move

- Failed-check share of checks 3.5% (24 h) toward 1.5% to 2% (the
  single-cite floor, 122 of 8,751, is 1.4%, plus group failures); stories
  with a failed check 19% toward 11%; judge questions down about 7%.
- Share of revision steps that republish identical text, 8% toward 0, and
  fewer send-back rounds (50% of steps) as fewer cites fail.
- Duplicate groups 8 per 24 h toward 3 or fewer, from D1 and D2 together; D3
  would take the rest.
- Cost: intake label calls per capture about 4x (see item 4); the support
  judge makes the same one call per revision with fewer questions.
