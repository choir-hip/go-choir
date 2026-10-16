# The event layer at ingestion: captures accumulate on events, production waits for corroboration

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
  - **p ≥ 0.85 for a known event:** a counted member.
  - **0.5 ≤ p < 0.85 for a known event:** a **tentative** member. It is
    visible on the event, never counts toward its outlets, and never
    mints a rival event (panel B1).
  - **p ≥ 0.85 for another capture of the chunk:** a sibling union
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
