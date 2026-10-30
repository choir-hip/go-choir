# The event layer at ingestion: captures accumulate on events, production waits for corroboration

Status 2026-10-30: Store B and the Dolt-era capture wording below are stale; captures are objects in the PostgreSQL graph database behind corpusd. Design source for the new data system: [the data system frame](../design/choir-data-system-frame-2026-10-30.md).

Draft 2026-10-16, about 00:15 UTC, for panel review (Devin and Gemini,
plus the orchestrator, per owner §24).
- Problem: [capture and production are coupled](../problems/capture-and-production-are-coupled-2026-10-16.md), F2–F4.
- Rulings: [owner notes](owner-notes-2026-10-14.md) §24 and §25.
- This amends, and does not replace,
  [the claims, events and entities ontology](claims-events-entities-ontology-2026-10-14.md)
  (abbreviated CEE below).

## 1. What the owner asked for

1. **Capture is separate from article production.** One source at time T
   publishes nothing. Sources arriving at T′ and T″ accumulate, and the
   article is produced after that (§25).
2. **Ingestion scales without more model calls.** More sources means
   more captures per call, not more calls (§24).
3. **Jev is allowed at ingestion.** "Between embeddings and a model call
   is Jev" (§25). No desk or agent run happens at ingestion.
4. **A story needs at least two outlets, ideally more and more diverse**
   (§24). The publish gate for this landed in d54242fc9.
5. **Outlets and claim sources are separate objects with statistics,
   linked** (§24).
6. **The world model is retrieved from first,** ahead of articles. It is
   the token-efficient view of the world past a model's cutoff (§25).

## 2. What exists (survey at 6c0368219)

- **Capture involves no model.** It runs poll → Store B → `CaptureArrival`
  (one record per poll batch).
- **Jev labels** (`agentcore/intake_labels.go`, on for the paper) make
  one call per capture, with two questions (same event, same topic). The
  candidates are the computer's **stories** (72h: the 30 newest plus 10
  by title overlap) and the **same poll's** sibling captures. The label
  is a computer-side `choir.capture_label`.
- **The gap.** A capture labelled "new" at T is never a candidate for
  another outlet's capture at T′. A later poll compares only against
  stories, and a story exists only after a desk opened one from the
  first capture. So corroboration can accumulate only *after*
  production has started, which is the reverse of what is wanted.
- **Every arrival wakes management.** Each record gets a management turn
  and one intake task. Desk runs scale with polls.
- **CEE plans the rest:**
  - events (`choir.world_event`) as merge keys (§9.1);
  - Jev resolution with a shortlist (§12.1 step 5);
  - judgments in Store B (§5.4);
  - routing by event (E3);
  - dossier retrieval (`EventDossier`, §9.4);
  - outlet objects (§4.2).

  CEE runs generative claim extraction per capture *before* event
  resolution, with E0 as an offline study that can stop the layer.

## 3. The amendment: events before extraction, Jev only at ingestion

CEE's pipeline order is extract → link entities → resolve events. This
amendment reorders it:

```text
ingestion (no desk, no generative model)
  poll → captures (Store B) → resolve each capture to an event with Jev
       → event projection: captures, outlets, first and last seen
production (desks, per event, not per poll)
  an event reaches 2 distinct outlets  → management decides open or decline
  an event covered by a story grows    → the story is fed (no turn when Jev is sure)
  research and Texture retrieve the event dossier first
claims (generative, deferred)
  extraction runs for events that reached production, not for every capture
```

Why this order:
- The owner's rulings put generative calls out of ingestion.
- Event resolution on title, summary and lead blocks is already CEE's
  fallback path (§12.1, "event resolution for a capture with no
  extraction runs on its title and summary").
- Extraction for corroborated events only cuts CEE's estimated per-capture
  cost by the share of captures that never corroborate.

## 4. Objects

| Object | Home | Writer | Notes |
|---|---|---|---|
| **Outlet key** | derived | `internal/outlet` (d54242fc9) | Registrable host, or platform/account. Deterministic, so it is never a judgment. CEE's Outlet entity (ownership groups, `member_of`) comes later and keys on this |
| **Event** (`choir.world_event`) | see Q1 | minted by the first membership judgment that answers "new" | headline (the minting capture's title, later Texture's), first_seen, scale `event` |
| **Membership** (`reports`, capture → event) | see Q1 | `system:resolve` (a Jev act, harness-run) | capture id, event id, p, alternatives shown, model served, cost. Immutable; a correction supersedes it |
| **Event projection** | rebuildable table | reducer | captures, distinct outlets, first/last seen, languages, covered_by story. **Never authority** |
| `choir.capture_label` | computer | existing | Kept for the topic answer and the cost. Its story pointer is retired when routing moves to events (CEE's dated deletion clock) |

**Q1 (the panel decides): where events and memberships live first.** The
two options are below. **Draft lean: (a), the paper's computer store, as a
shadow, with Store B in the routing slice.** Store B is still in its Dolt
2.4.2 soak until about 2026-10-17 15:40, and a new write kind would
confound that soak.
- **(a) The paper's computer store,** like `capture_label`. Fast to land,
  on the tape, restorable. But it is not shared, so a second computer
  cannot adopt it until it is published.
- **(b) Store B through a typed corpusd route** (CEE §5.4: a platform-tier
  computer's judgments are public by nature). It is shared at once and is
  the eventual Autopaper object graph home. But it adds a red corpusd
  write route and Store B growth during the soak.

## 5. Ingestion: resolution with Jev, batched across sources

- **Shortlist (deterministic).** It covers:
  - events with any member capture first seen in the last 72h: the 30
    newest by last_seen, plus 10 by title-token overlap, as today's
    library does;
  - every story's covered event;
  - the poll's siblings.

  After the soak it gains Store B title postings (land/store-b-retrieval).
- **One Jev call per coalescing window, across sources.** Superseded
  "per poll" after the offline study (§12): most polls bring one item.
  - New captures from every source queue in memory. A flush runs at 12
    queued captures, or 60s after the first.
  - A flush resolves in chunks of up to 12, **in order**, so events
    minted in one chunk are candidates in the next.
  - The state holds the candidate events (headline plus the last 2 member
    titles, clipped) and the chunk's captures (title, summary clipped to
    400 runes, outlet). It asks one choice question per capture, whose
    choices are the events, the *other* captures, or new.
  - A chunk whose state exceeds 24 KB splits in half, keeping it under
    Jev's ~32 KB default context.
  - Budgets: 30s per call and 2 min per flush, on the flush's own
    timer, off the poll path.
- **Thresholds and the formation fold.**
  - **p ≥ 0.92 for a known event:** a counted member (0.85 until 2026-10-16; see docs/problems/event-layer-false-merges-at-marginal-p-2026-10-16.md).
  - **0.5 ≤ p < 0.92 for a known event:** a **tentative** member. It is
    visible on the event, never counts toward its outlets, and never
    mints a rival event (panel B1).
  - **p ≥ 0.92 for another capture of the chunk:** a sibling union
    (union-find, lowest index as root). A sibling group joins its
    members' best counted match by p. Otherwise it mints one event,
    opened by its earliest capture.
  - **Anything else** mints a new event.

  A false merge is worse than a late one, because it counts the wrong
  outlets.
- **Restart rule (CLAUDE.md).** Resolution is in-memory, with no durable
  obligation. On a crash, captures in flight stay unresolved, which is a
  visible state, and there is no boot sweep. An unresolved capture is
  resolved when a live act needs it, or the next poll's call carries it
  as a sibling.
- **Order.** Resolution runs after the Store B write and before the
  arrival record, where labels run today. It is bounded per poll, and a
  timeout leaves captures unresolved. It never blocks capture.

## 6. Production: management wakes per event, not per poll

- **Arrival records keep being minted** (frozen declaration, no change).
  What changes is **what wakes management.** Today every arrival does.
  After this change:
  - **Event became corroborated:** the event's distinct outlets went from
    1 to 2. One new record kind, `EventCorroborated` (event id, outlets,
    member capture ids), wakes management to open or decline. Its record
    id is deterministic in the event id and the outlet pair, so a retry
    or a re-resolution cannot wake management twice.
  - **Covered event grew:** a capture joined an event that a story covers.
    With p ≥ 0.95 it is fed without a turn (CEE's O3 rule). Below that,
    it is batched into the next intake task for that story.
  - **Uncorroborated captures** wake nothing. They wait on their event.
    Management's `Stories` view gains a counted line, "N single-outlet
    events waiting (oldest Xh)", so nothing is invisible.
- **Intake tasks per event, batched.** `Delegate("intake", …)` takes
  event-grouped work, one task per corroborated event or several events
  per task, instead of one task per arrival record. Desk runs then scale
  with events crossing the threshold, which is sublinear in sources.
- **Single-outlet primary records.** A government statement or a court
  filing is one outlet but may matter. Leaning: they wait like any
  other. The publish gate needs two outlets anyway, and research can be
  asked for a second source when management chooses to (a costed act,
  not an automatic one). **Q2 (panel):** is there a deterministic class
  of single-outlet events that should wake management (official
  registry sources, or scale signals)?

## 7. Retrieval: the dossier first

New read-only cell verbs:
- `choir.Events(query, since)` returns headline, outlets (count and
  keys), first and last seen, captures (title, outlet, URL, id) and
  covered_by. It is title-token search over events, and over Store B
  postings once landed.
- `choir.EventDossier(eventID)` returns the same for one event, plus
  member summaries clipped.

Who gets them:
- **Research:** `SearchWireCorpus` stays, and the overlay puts `Events`
  first.
- **Management:** `Stories` shows each story's event outlet count.
- **Texture:** gets `EventDossier` for the event its story covers, which
  shows what other outlets said before it writes.

Token efficiency comes from the dossier: one event of 8 captures in
about 1.5k tokens, against 8 capture reads.

## 8. Outlets and claim sources (§24)

- **Outlets are available now** through the deterministic key. Per-outlet
  statistics read from the projection: captures, events first reported,
  events corroborated, and lead time (how often an outlet is first on an
  event later corroborated). These are counts, not truth scores.
- **Claim sources** (the speaker inside an article) need claim
  extraction (CEE §4.1). They come in the deferred slice, for
  corroborated events only. `speaker` and `carried_by` link them to
  outlets.

## 9. Slices

| # | Slice | Class | Acceptance |
|---|---|---|---|
| V1 | **Events in shadow.** Batched Jev resolution writes events and memberships. Nothing reads them. Per-computer enable `intake.events` | red (gateway calls; Q1 store) | On the paper over 6h: ≥ 95% of crash-free captures resolved within 2 min. Calls per hour ≈ polls per hour, not captures. A blind sample of 40 memberships judged by the registered judge has false-merge ≤ 5% and missed-merge reported. Cost per capture is reported. No publication regression |
| V2 | **Retrieval verbs** (`Events`, `EventDossier`), read-only, plus outlet statistics | orange | Research uses `Events` in ≥ 50% of runs. Dossier size is bounded |
| V3 | **Production by event.** `EventCorroborated` wakes management. Uncorroborated captures wake nothing. Intake tasks are grouped by event. The capture_label story pointer starts its deletion clock | red (inbox declarations, management wake path) | Management turns per hour fall as sources rise: with 2× active sources, desk runs per hour stay ≤ 1.2× baseline. Captures per new story rise from the baseline of 2.35. No corroborated event goes unwoken |
| V4 | **Scale ingestion.** Activate more registry sources, Telegram excluded (§25; MTProto with the owner) | orange (schedules) | Steady paper health. Model calls per hour within V3's bound |
| V5 | **Claims for corroborated events** (CEE E1 onward, scoped) | red | per CEE |

V1 and V2 can land overnight (V1 on Q1's answer). V3 is the substrate
change and is the next station. V4 waits on V3, because activating
sources before V3 multiplies desk runs, which contradicts §24.

## 10. Ways it could fail (tests are written against these)

- **E1:** a low-confidence join counts a different event's outlet as
  corroboration, so a one-outlet story publishes in substance.
- **E2:** resolution runs per capture again (a regression to per-capture
  calls), so calls scale with captures.
- **E3:** a crash mid-resolution resumes at boot (the forbidden resumer),
  or a retried resolution mints a second event for the same capture.
- **E4:** `EventCorroborated` fires twice for one event, or never fires
  when the second outlet arrives in the same poll as the first.
- **E5:** the shortlist omits single-outlet events older than the
  horizon, so a slow second report (T″ hours later) mints a new event
  instead of corroborating.
- **E6:** two pages of one outlet count as corroboration (the outlet key's
  O1 rule, already tested).
- **E7:** state above 32 KB silently drops into long-context mode, and
  accuracy falls unmeasured.
- **E8:** a membership written for a capture the poll never stored
  (Store B write failed) points at nothing.
- **E9:** syndicated wire copy (AP text on many outlets) counts as
  independent corroboration. This is a known residual: CEE's `copy_of`
  and `originates_from` handle it later. Until then, an exact-text
  duplicate across outlets (equal doc_hash of the body) counts once.

## 11. Ceremony (V1 and V3)

- **Conjecture delta.** Resolving events from title and summary with
  Jev, before extraction, is precise enough (false-merge ≤ 5%) to gate
  production. It is falsified if V1's sample exceeds that, in which
  case extraction-first (CEE order) returns.
- **Protected surfaces.** Gateway calls (Jev), the poll path, the inbox
  declarations (V3), and the management wake path (V3).
- **Admissible evidence.** Deployed on staging. A minted test computer
  first, then the paper.
- **Rollback.** Turn off `intake.events`, then revert. V3's rollback
  restores arrival-wakes-management.
- **Heresy delta.**
  - Discovered: per-poll desk runs (the substrate), and stories as the
    only accumulation point.
  - Introduced: the transitional dual answer (capture_label's story
    pointer and event memberships), with CEE's dated deletion clock.
  - Repaired: none until V3.

## 12. Offline study (2026-10-16, about 00:20 UTC)

**Setup.**
- The paper's 500 newest captures (`GET /api/captures`, 20:19–00:18 UTC,
  184 outlets) were resolved in arrival order against an evolving event
  set.
- Resolution used `~typesafe/jev-latest` (served jev-1.14), one choice
  question per capture.
- The shortlist followed §5: the 30 most recent events plus 10 by title
  overlap.
- Join threshold p ≥ 0.85.
- The script and outputs are in the session scratchpad, outside the repo.

**Results.**

| Measure | Value |
|---|---|
| Calls | 432 for 500 captures. Per-poll batching barely batches, because most polls bring one new item |
| Cost | $0.064 in total, about **$0.00013 per capture** |
| Largest request | 23 KB, under the 32 KB long-context threshold |
| Events | 273, of which **36 reached ≥ 2 outlets** |
| Joins / explicit new / low-confidence new | 227 / 182 / 89 |

**Quality of the 36 corroborated events (orchestrator read, not blind).**
About 33 are one real-world occurrence. They include cross-language
joins: the Brazil runoff poll in Portuguese and English, and the
Eurotunnel accident in French and English. Topic-but-not-event pairs
stayed apart: the Plex ticketing deal and the Plex data-sharing story
became separate events.

Errors:
- **Four were an article plus its Hacker News thread.** These were fixed
  in the outlet key (afd160449, O5).
- **One cluster was research's own reads of one subject** (FEC record
  pages): research's web reads are captures too.
- **One** (a trade-show news item plus the show's exhibitor page) is a
  topic link.

**Design consequences, adopted.**
- **Batch across sources on a coalescing window,** not per poll. Pending
  captures from every source are resolved together every 60s (or at 12
  captures), so calls scale with time and not with sources.
- **The 89 low-confidence mints are the splintering risk** the panel
  named. Uncertain matches become tentative members; see §13.
- **Corroboration counts feed (registry) captures and research reads
  alike.** But research's repeated reads of one subject are one actor's
  search, not independent reporting. A research-origin capture can
  corroborate only an event a feed capture opened.

## 13. Panel round 1 adjudication (Devin and Gemini, 2026-10-16)

Both returned **ready with changes**. Raw output is in
`.agentic-consensus/event-layer-r1/` (gitignored).

| Point | Raised by | Decision |
|---|---|---|
| Q1: events first in the paper's store (a) | both | **Adopted.** Memberships carry p, model and cost now, so a later Store B export is a route change, not a re-mint (Devin) |
| Tentative members (splintering at p < 0.85) | both | **Adopted** in §5 and in code: visible, uncounted, no rival event. **Residual R-T:** re-asking tentative members when their event grows is not built. V1 measures how many there are |
| Wire copy needs a wire-tag origin, not doc_hash | Gemini | **Adopted.** A wire tag in the lead (`(Reuters)`, `AFP -`, …) sets an origin, and one origin counts once. Residual: untagged syndication |
| Overlap horizon of 14 days for uncorroborated events | Gemini | **Adopted** (recency slots stay at 72h) |
| `EventCorroborated` keyed by event only, fired on the ≥2 transition, 0→2 included | both | **Adopted** in V1 as the projection's transition (logged). V3 mints the record on it |
| Batch across sources on a coalescing window | Devin, the study | **Adopted** (§5) |
| Sibling formation fold | Devin | **Adopted:** union-find, then best counted match, else one mint by the earliest capture. Devin's "no blind transitivity" is not adopted for V1; V1 measures false merges instead |
| Rebuild the pending set at boot from stored captures lacking a membership | Devin | **Rejected.** That is a boot-time resumer (CLAUDE.md, "Restarts end work"). A crash leaves captures visibly unresolved. A later live act (the next flush's sibling, or V3's consumer) may resolve them as new work |
| Measure shortlist overflow, and a `merged_into` producer for post-horizon misses | Devin | **V1 measures it** with the event view (singleton events with high title overlap). The merge question is V1.5 |
| Single-outlet official sources: all wait (Gemini), or an aged, bounded wake for `sources.Source.Official` events (Devin) | split | **Decided for V3: Devin's aged wake.** An official-source event still single-outlet after one poll interval wakes management once, so it can ask research for a second source. The publish gate is unchanged. Gemini's concern (noise) is met by the age bound and one wake per event |
| `Stories`, `Delegate("intake")` and Feed/Decline are gated on `intake.labels` | Devin | **V3 must decide the enable matrix.** Lean: fold the topic question into the batched call, migrate the gates to `intake.events`, and retire per-capture label calls, so calls stop scaling with captures |
| Batched multi-question accuracy is unmeasured | Devin | **V1 acceptance:** replay the 500-capture study coalesced, against per-capture resolution, and report agreement before V3 depends on it |

## 14. Coalesced versus per-capture accuracy (Devin's V1 measure, 2026-10-16 about 00:40 UTC)

**Setup.** The same 500 captures were replayed with the V1 algorithm in
both modes:
- chunks of up to 12 per call, with sibling choices and the tentative
  band;
- one capture per call.

**Results.**

| | Coalesced | Per capture |
|---|---|---|
| Calls | **137** | 499 |
| Cost | **$0.040** | $0.062 |
| Events | 202 | 160 |
| Tentative members | 61 | 56 |
| Same-event pairs | 1,159 | 1,504 |
| Share of its pairs the other mode also made | **0.93** | 0.72 |

**Reading the disagreements (orchestrator, not blind).**
- **Pairs only the per-capture mode made lean toward topic merges.**
  Examples: a box-office report with a film review, a mortgage-rate data
  page with a rates article, and a president's statement with the strike
  it reacts to (a development, arguably the same event).
- **Pairs only the coalesced mode made are mostly correct same-event
  joins**, among them the Romania tanker reports.

**Conclusion.** Coalesced resolution is more conservative, with fewer
merges and fewer topic merges, at about a quarter of the calls. That is
the right bias for a gate whose failure mode is false corroboration.

**Remaining measures.**
- **Missed merges.** The coalesced run split the Romania tanker event
  into more than one cluster.
- **The V1.5 merge question,** which is the repair for missed merges.

The replay includes research-read captures (FEC pages, review
aggregators), which V1 does not resolve (it takes only source-poll
captures), so production precision should be higher.

## 15. V3 mechanism (drafted 2026-10-16 about 01:00 UTC, for panel round 2)

A survey at 9eb2c037f of what V3 must change. File and line references
are from that commit.

### 15.1 What wakes management today

- `store.RecordWakesReader` (`store/management_tasks.go:63`) is the one
  wake predicate. A system record wakes when it is a capture arrival with
  items (line 73). It is a pure function of the record, so it cannot read
  a setting.
- Each poll's arrival is one record to `management:<owner>`
  (`agentcore/capture_arrival.go:53`). Its id is deterministic in the
  source and the capture ids, and its digest covers its content.
- Management's turn reads every unread captures record through
  `choir.Records()`, with Jev's labels joined in when `intake.labels` is
  on. The paper has `intake.labels` on (since 2026-10-14 06:50), so each
  capture also costs one label call.
- `Delegate("intake", …, "per_record")` makes one task per captures record.
  `Decline` routes each capture to `source:<id>` using the record's
  `SourceID`. `Feed` needs a story. The disposition index
  (`choir.capture_disposition`) is per capture, so any captures record
  that names a capture can lease it.

### 15.2 The change: arrivals are held, and events wake

1. **Held arrivals.** `CommitmentCaptures` gains `Held bool`
   (`json:"held,omitempty"`). When `intake.events` and the new
   `intake.event_wakes` are both on at mint time, the poll mints its
   arrival with `Held: true`. `RecordWakesReader` returns false for a held
   captures record. The record is still minted, so every capture is still
   announced on the ledger (the frozen declaration holds), and the pending
   list and the retry and conflict rules are unchanged. A flag flip
   between a failed append and its retry changes the digest. The retry
   then gets `ErrLifecycleCommandConflict` and is treated as announced,
   which is the existing rule.
2. **Held arrivals leave the desk's view.** `choir.Records()` drops a held
   captures record from the view; the cursor still passes it. In its
   place the view carries one counted line: "N captures held on M
   single-outlet events (oldest Xh); U unresolved". Nothing is invisible.
3. **Event records wake.** When the event index sees an event cross to
   `EventMinOrigins` counted origins (the transition V1 already logs), the
   flush mints one system captures record to `management:<owner>` with:
   - `SourceID: "event:<event_id>"`;
   - `Items` = the event's members (counted and tentative), each with a
     new per-item `SourceID`;
   - a new `Event` body: headline, counted origins, outlets, first seen.

   Its record id is `event-corroborated:<event_id>`, so it fires once per
   event (E4), whatever the retries. It wakes because it is not held.
4. **Growth.** A capture that joins an event that already woke is not
   lost. At the end of each flush, one growth record per event with new
   members is minted, with id `event-grew:<event_id>:<hash of new member
   ids>`. Wakes per flush are bounded by events that grew, and the
   management cursor batches them.
5. **Decline routes by item.** `Decline` uses an item's `SourceID` when
   set, otherwise the record's. A capture declined from an event record
   still lands on its real `source:<id>`.
6. **Fallback: the layer down must not stop production.** A flush that
   fails (Jev errors or timeouts past the retry) mints a plain, unheld
   captures record for the failed captures, with id
   `event-fallback:<hash of ids>`. That is today's behaviour for exactly
   those captures (E10). `stopEventLayer` at shutdown mints nothing. A
   crash drops the in-memory queue: those captures stay held and
   unresolved, counted in the line from 2. The next live flush may take
   up to 50 of the newest held, unresolved captures (from the last 6h) as
   siblings. That is new work by a live act, not a boot resumer.
7. **Aged official events** (Devin, round 1). In each flush, an event
   still below `EventMinOrigins` whose opener's source is
   `sources.Source.Official`, and which was first seen more than 45 min
   ago, mints one `event-aged:<event_id>` record. It is the same shape as
   3, with `Event.Reason: "official single-outlet"`, so management can ask
   research for a second outlet. The publish gate is unchanged.
8. **The intake.labels gates.** The Stories, Delegate, Feed/Decline and
   override surfaces are on when `intake.labels || intake.event_wakes`. The
   per-capture label calls stay as they are in V3a. Folding the topic
   question into the batched event call, and retiring per-capture labels,
   is V3b, which must land before V4.

### 15.3 Why this shape

- **No new record kind and no new wake class.** An event record is a
  captures record, so `Delegate`, `Feed`, `Decline`, the disposition
  index, task leases, residual re-delegation and the crash budget all
  apply unchanged. The only predicate change is that held records do not
  wake.
- **The ledger still names every capture at arrival.** That is the
  announcement contract. The held flag records why it did not wake.
- **Rollback is one setting.** With `intake.event_wakes` off, arrivals mint
  unheld and wake as before. Records already held stay held and stay
  counted. Turning the flag off does not wake them; turning it on again,
  or the sibling sweep, picks them up.

### 15.4 Ways V3 could fail (tests are written against these)

- **E10:** a Jev outage silently stops production. The fallback record in
  6 covers it. Test: the judge fails, and an unheld captures record wakes
  management naming exactly those captures.
- **E11:** an event wakes twice for one transition, or a growth record
  repeats members the corroboration record named.
- **E12:** a held record wakes management, or appears in its view as
  work.
- **E13:** Decline of an event-record capture lands on `source:event:…`.
- **E14:** a capture named by both its held arrival and its event record
  is disposed twice. The index refuses the second. The test asserts that
  the held record's presence does not make the capture a residual.
- **E15:** the aged official wake fires for a non-official source, or
  fires twice.
- **E16:** the counted line hides a stuck queue. The test asserts the
  unresolved count rises when the layer is stopped.

### 15.5 Acceptance (V3a, on the paper)

The baseline is taken during V1's shadow window from the same paper:
management turns per hour, intake tasks per hour, captures per new story,
and stories published per hour. V3a acceptance over 6h:
- management turns per hour ≤ the baseline;
- intake tasks per hour ≤ 0.5× the baseline;
- every corroborated event is woken (count of transitions = count of
  `event-corroborated` records);
- stories published per hour ≥ 0.7× the baseline;
- zero events woken twice.

A fall in published stories is expected and is the owner's two-source
rule working. A collapse below 0.7× means the threshold or the
resolution recall needs work before V4.

## 16. Panel round 2 adjudication (Devin and Gemini, 2026-10-16 about 01:05 UTC)

Both said "ready with changes". Neither asked for a redesign. Both
endorsed the held flag on the existing record kind over a new kind.

| Finding | From | Decision |
|---|---|---|
| Wakes must come from the ledger, not from transitions seen in memory: a crash between a membership write and its record loses the wake silently | Devin B1 | **Adopted.** Each flush checks events whose members changed against their conversation's records (`store.ConversationRecords`, indexed): no `event-corroborated:` record means mint one, and members no record named go on a growth record. A checked event is settled until it gains a member. Test: a crash before the record, then the next flush fills the gap |
| The settings matrix and the held backlog on rollback | Devin B2 | **Adopted.** Events and wakes both on is V3. Events on with wakes off is V1 shadow: arrivals are unheld and no event records are minted. The sweep (below) drains: with wakes off, every unnamed held capture is released (`released`). Rollback order: turn wakes off, wait one sweep (5 min plus the 10 min grace), then events off if wanted |
| The sibling sweep re-puts poison captures to Jev, so a crash loop; newest-first starves | Gemini B1 | **Adopted, replacing §15.2 #6.** No capture is put to Jev twice. A live poll (never a boot), at most every 5 min, scans the newest 1000 management records and releases held captures older than 10 min that no event record named and that have no membership. They go oldest first, at most 48 a sweep, as `event-unresolved:` records that wake. Captures older than 24h are not released. That is a named residual, visible in the sweep's log counts |
| Growth wakes management every flush; declined events keep waking | Gemini B2 | **Adopted, changed.** Growth records are held: they wake no one and are read on management's next turn, because a held record that carries an `Event` stays in the view. So wakes come only from corroboration, aged official events and unresolved captures. Gating growth on declined events, and §6's O3 auto-feed, wait for V3b, where they belong with the label fold |
| Tentative members read as corroboration | Gemini B3 | **Adopted.** Items carry `Tentative`. The overlay says never to count one as a second source. The publish gate counts outlets of cited captures, so a cited tentative capture from another outlet would count. That is a named residual for V3b: the gate should consult membership |
| Aged wake idempotence; aged then corroborated | Gemini B4 | **Adopted.** `agedWoken` in memory, plus the deterministic id; after a restart the retry is a conflict that counts as minted. Aged then corroborated is two wakes, which is legitimate. Acceptance counts wakes per kind |
| Per-item `SourceID` | both | **Adopted.** Every event and unresolved item carries its own source, and `Decline` routes by it (E13 test) |
| Disposed captures in event items; second-disposal churn | Devin | **Partly.** Growth names only members no event record named. A capture released as unresolved never gets a membership, so it is never named again. The flag-flip window can name a capture on both an unheld arrival and an event record. The index refuses the second disposal; this is a named residual |
| Acceptance: model calls per hour, gap = 0, fallback and sweep counts | Devin | **Adopted** into V3a acceptance |

**Baseline for V3a** (the paper, 22:00–00:00 UTC on 2026-10-15/16, from
`/api/runs`), runs per hour:

| management | management tasks | research | Texture |
|---|---|---|---|
| ~33 | ~14 | ~21 | ~100 |

## 17. V3a code review adjudication (Devin and Gemini, 2026-10-16 about 01:20 UTC)

Both said "ship with fixes". All blocking findings are fixed and tested.

| Finding | From | Fix |
|---|---|---|
| `settled` and `agedWoken` were set when a record was built, not when it committed, so a transient commit failure lost the wake until a restart | both (B1) | The flags are set after the commit, or on a conflict that counts as committed. Test: a failed commit, then the next flush mints the wake |
| A failed flush sent every input as unresolved, including captures a chunk had already resolved | Devin B2 | Only inputs without a membership are left. Test |
| An event with more members than one record holds left the rest unnamed for good | Gemini B2 | Members past the bound go on held growth pages in the same flush. Test |
| A poll with no new captures but pending ones minted an unheld arrival | Gemini B3 | Held whenever both settings are on. Captures never queued are released by the sweep |
| The counted held line was missing | both | `choir.Stories()` first page carries `held_captures` (waiting, released, swept_at) from the last sweep. Test |
| `enabledAt` moved on any rewrite of the setting | both | Setting a value it already has writes nothing (`/api/computer-settings`). Test |
| The labels text was shown when only event wakes were on | Devin | The overlay splits: labels on `intake.labels`; the Delegate, Feed, Decline and Stories text on either setting; event records on `intake.event_wakes`. Test |
| A failed sweep read suppressed sweeps for 5 min | Devin | The sweep is stamped only after its scan reads |
| Aged wake across a restart | test gap | Test: one record after a restart |

**Named residuals** (accepted for V3a, watched on the paper):
- The sweep sees only the newest 1000 management records. On a very busy
  inbox, a held arrival can fall out of the window before it is released.
- A capture whose event never corroborates waits until it ages out of the
  24h window. That is the owner's two-source rule. It is counted as
  waiting on Stories while in the window.
- The aged official check runs only inside a flush. A computer with no
  new captures never ages an event. The paper is always busy.
- Event items carry no PublishedAt, Language or Script (memberships do
  not store them).
- The flag-flip window can present one capture on an unheld arrival and
  on an event record. The disposition index refuses the second disposal.
- Ledger reads run under the index lock, so corpus search waits on a
  flush's ledger check.
- The rollback drains at most 48 captures per sweep, one sweep per 5 min:
  a backlog of N held captures drains in about N/48 × 5 min.

## 18. V3a staging proof, and V3b: event records name their stories; labels retire under event wakes

### 18.1 V3a on a minted test computer (2026-10-16, 01:45–02:00 UTC)

`v3-events-0146` was minted on build 4b66f9ef2 with `intake.labels`,
`intake.events` and `intake.event_wakes` on. It ran six schedules at
15 min: BBC World, Al Jazeera, Guardian World, DW, NPR World and the
Fed's press feed. Within 4 min of the first polls:
- 6 held arrivals were minted. None woke anyone.
- 24 events formed, and 4 were corroborated: BBC+NPR (Russia/Ukraine
  refinery), BBC+DW (Channel Tunnel collision), BBC+DW (Maduro plea), and
  AJ+DW (Houthi/Saudi).
- Exactly 4 `event-corroborated:` records were minted, one per event.
- Management delegated them to 4 intake tasks. Each opened one story
  ("2 of 2 captures disposed"), and the 4 story headlines match the 4
  events.
- One story had published v1 by 02:00. The other three Textures asked
  management for capture passages first: the usual Texture–management
  ask, not a V3 path.

On the paper (V1 shadow, events only), the first two corroborated events
were the Tesla Roadster (TechCrunch, The Verge, Wired, tesla.com) and the
Oregon school shooting (The Hill, Guardian, Al Jazeera). Both are
correct. Repeat captures of one URL (one HN item through several feeds)
join one event and add no outlet.

### 18.2 The problem V3b fixes

Under event wakes, management sees captures only on event records. But
the per-capture Jev label call (`intake.labels`: "same event as story X",
"same topic as story Y") still runs on every capture at poll time.
Captures that never corroborate never reach a desk, so most of those
calls are spent on nothing, and the calls still scale with captures
(§24). An event record also does not say whether a story already holds
the event's earlier reports. Management must infer it from the Stories
list, which is the duplicate-story risk the labels existed to cut.

### 18.3 The change

1. **Covered and Declined, from the ledger, with no model call.** When an
   event record is built, the disposition index (`CaptureDispositionOf`)
   of each of the event's other members gives the story inboxes that
   opened or were fed one of them. These go in `Event.Covered`. If every
   disposed member was declined, `Event.Declined` is set. The overlay
   says: feed a covered event's captures to the story named; a declined
   event's new reports may be declined unless they change the event.
2. **Labels retire under event wakes.** With `intake.event_wakes` on, the
   poll does not run per-capture labels. Model calls at ingestion are
   then one batched event call per flush (§12: about 137 calls per 500
   captures, against 500 or more before).
3. Nothing changes with `intake.event_wakes` off.

### 18.4 Ways it could fail

- **V3b-1:** Covered names a story that only a tentative member reached.
  Tentative members are skipped in the Covered lookup.
- **V3b-2:** a lookup error blocks the record. Covered is best effort; an
  error is logged and the record still mints.
- **V3b-3:** labels are skipped while event wakes are off. The label gate
  is unchanged when event wakes are off.

The class is orange (prompt text, a record annotation and a skipped
optional call). The rollback is a revert, or turning event wakes off.

## 19. Overnight evidence, 2026-10-16 03:00–03:15 UTC

### 19.1 V2 on the paper

Since V1 was turned on (01:02), 29 of 38 of the paper's research runs
(76%) carried world event ids in their run events. The bar in §9 is
≥ 50%. Event wakes are off on the paper, so event ids reach research
only through corpus search. The count is a proxy, by run, not by call.

`EventDossier` for Texture and outlet statistics are not built. They
wait until V3 is measured on the paper. A change to Texture's frame now
would confound the §16 baseline, which counts Texture runs per hour.

### 19.2 The V1 blind sample, rehearsed

The judge script sees only the opener's and the member's title and URL,
never Jev's p or model. It gives the judge the resolver's own
definition: the same occurrence or its direct development (a later
announcement, reaction or outcome). A false merge counts only when both
judges say different. The judges are `openai/gpt-6.1-sol` and
`google/gemini-3.8-flash`, both non-Claude, through OpenRouter.

- **Paper (03:10):** 36 joined memberships. Only 14 have distinct URLs;
  the rest are repeat captures of one URL. The single judge in the
  rehearsal disagreed on the two tesla.com pages titled only "Tesla
  Roadster". Those are the official page of the reveal, which a title
  alone does not show.
- **Test computer (6 world feeds):** 21 joined memberships, all with
  distinct URLs. Both judges said different for 1 of 21 (4.8%): two
  Middle East live blogs on different strikes.
- **Judge noise:** one judge flipped on the Argentina protest pair
  between two runs. A single judge is not enough.

The paper alone will not reach 40 distinct-URL joins by 07:00. The
acceptance run pools the paper and the test computer. Both run the same
resolver; the test computer's six world feeds make more cross-outlet
joins. Each computer is reported separately.

### 19.3 The event is one incident (owner ruling 2026-10-20); the blind rerun

The definition changed (owner notes §27 item 8; problem doc "Owner ruling
and plan"). An event is one incident, in one place, at one time. A later
development, another incident of the same story, a live blog or roundup
spanning incidents, and opinion or analysis about a topic are each a
different event. A capture Jev reads as a later development mints its
own event with `follows` set to the earlier one (offered per capture as up
to three `after_` choices over the events sharing the most title words).
The link is evidence and never counts toward corroboration. The sample
judges in §19.2 were given the old wording ("or its direct
development"); the judge text now carries the new definition, so the rerun
is not directly comparable with the 7.0% and 9.5% runs. It measures the
new definition against its own text.

**Rerun procedure** (after the change is deployed; nothing here runs on
staging until then). The script is `scripts/event-blind-sample.py`; its
definition is held equal to the resolver's by
`TestBlindSampleScriptCarriesTheResolverDefinition`.

1. **Deploy and identity.** Confirm `choir.news` health reports the
   change's SHA. The window opens when each computer is *running* the new
   code, not at the push: for the paper, the time of the planned refresh
   that applied it; for the test computer, its first boot on the new
   build. Call the later of the two `T`, in UTC.
2. **Settings and wake.** `intake.events` on for both computers; leave
   `intake.event_wakes` off. The test computer `v3-events-0146` hibernates
   within the hour (free tier): wake it with a lifecycle start at each
   check, or its feeds do not poll.
3. **Accumulate a fresh window.** Only joins resolved at or after `T`
   count; do not pad with older joins. The previous fresh window gave 21
   distinct-URL joins per computer in about 6 hours, so expect 6 to 12
   hours to reach 40 pooled. A short sample is reported as short.
4. **Sample.** From a shell with the test-computer credentials in
   `~/.config/choir-qa/test-computers/` and `OPENROUTER_API_KEY` in the
   environment or the repo `.env`:
   `scripts/event-blind-sample.py --since <T as RFC3339 UTC> --n 40 --out <scratch dir> autopaper-paper v3-events-0146`.
   It lists the counted, non-opening, distinct-URL joins from
   `GET /api/autopaper/events`, samples 40 pooled (fixed seed), and asks
   `openai/gpt-6.1-sol` and `google/gemini-3.8-flash` through OpenRouter,
   which see only the two titles and URLs. It prints per-judge tallies,
   per-computer counts, and a JSON file of every row.
5. **The bar.** False merge counts only when both judges say DIFFERENT.
   The bar is **at most 5%**: with 40 sampled, at most 2 rows. 3 of 40
   (7.5%) fails. Then the orchestrator reads every DIFFERENT row itself
   and reports both numbers, as in the earlier runs (a title-only member
   such as tesla.com/roadster, the reveal's official page, is a correct
   member the judges cannot see).
6. **Also report, not gated.** (a) Corroborations per window against the
   previous fresh window (5 corroborations over 172 captures, all correct
   by reading): a large drop is the signal that true duplicates stopped
   merging (F2). (b) Read 10 events with `follows` set and say whether
   each link names a real earlier event (link precision). (c) Calls per
   flush and cost from the log lines; the extra choices must not push the
   calls per flush above about 1.
7. **Record.** Append the result to the problem doc and set
   `intake.event_wakes` only if the bar passes; otherwise the next step is
   a structural assessment, since this is the third failure and CLAUDE.md
   says to stop patching after non-convergence.
