# Corpus-first retrieval and research for Autopaper (design proposal, 2026-11-18)

**Status: a proposal, not ratified.** Written on 2026-11-18 by a design agent
(Opus, read-only) at the lead's request, from the owner's words of that day
and the measurements already recorded. Nothing here is built unless
[current-architecture.md](../current-architecture.md) says so. The owner's
decisions are listed near the end, each with a default. The problem it
answers: [research does not start from the corpus](../problems/research-does-not-start-from-the-corpus-2026-11-18.md)
and [search volume outruns the Keenable credits](../problems/search-volume-outruns-keenable-credits-2026-11-18.md).

The lead checked three of its as-built claims against main and they hold:
only source polls queue captures for the event layer
(`internal/agentcore/source_schedule.go`, `enqueueEventResolution`); the
related walk has environment switches (`CORPUSD_RELATED`,
`CORPUSD_RELATED_STORY`, `CORPUSD_RELATED_ENTITIES`,
`internal/platform/publication_related.go`); and the gateway still limits Jev
calls (`internal/gateway/jev_limiter.go`), against rule 4 of
[SYSTEM1.md](../reference/SYSTEM1.md), which already records that
contradiction. The other claims are the design agent's reading and carry its
file references.

## Summary

Make the event the unit, and treat a story as one telling of an event. Then three things have to be true. Every page the paper holds must be findable by its words and by its meaning. Every story must carry the ids of the events it tells. Research must read what is held before it searches the web. Build it in this order:

1. Change what research does first. The corpus brief and the "held" mark cost no embeddings, and they cut web searches soonest.
2. Add an embedder on Node B. It embeds stories and the lead passage of each page, and the vectors sit beside the BM25 passages that are about to land.
3. Send every captured page through the existing event layer and give each story its event ids at publish.
4. Make the new-or-revise choice after research, at a new story's first publish, from evidence: a shared event, shared citations and story-to-story similarity. The existing story's desk takes or refuses the fold.

One lookup, "neighbours of this text or event", then serves management, the event layer's intake, research, the front page, related stories and Autoradio.

Evidence tags used below: **M** measured (in the docs named), **C** read from code (file and line), **A** assumed.

## Three frame changes, and what each changed

- **What the unit is.** If the story is the unit, "revise or new" is a guess about two texts. If the event is the unit, it is a lookup: does this event already have a telling? Changed: story event ids come before any overlap check. The same ids then serve the front page, related stories and the radio. It also exposed a gap. The event layer only resolves captures from source polls; the one place that queues captures for it is `agentcore/source_schedule.go:294`. Pages research fetches with `choir.Read` never get an event. So event ids for stories need research's reads in the event layer first.
- **The physical bounds.**
  - Web searches run 5,900 to 10,600 a day (M), against a Keenable budget of about 900 to 1,000.
  - Corpus searches run about 4,600 a day (M: 384 in two hours) and cost under 100 ms of one core each.
  - Embedding steady state: 4,500 pages a day at 1,300 tokens a page (A) is about 68 tokens a second, half of one x86 thread (M: 127 to 164 a thread). At 3,600 tokens a page it is 1.5 threads.
  - Changed: the cheapest cut in web searches is changing research's first move, not vectors, so that goes first. The real cost is the first pass over the backlog, not steady state. The brute-force ceiling is about 4 million passages, not a million pages.
- **One mechanism for several jobs.** Three separate shortlists today pick "what might be the same story" by recency and title words:
  - intake labels (`intakeOverlapPicks`, `agentcore/intake_labels.go:207`);
  - the event layer's overlap slots (`agentcore/event_layer.go:45`);
  - the related walk's title-word filler.
  
  Changed: one corpusd read (vector neighbours plus an event-id join) replaces all three, and the same read feeds the research brief and the fold check.

## The retrieval substrate

**Passages are the key.** `lane/capture-fulltext-2` holds one SQLite FTS5 file per computer, owned by corpusd. Passages are about 300 tokens, and their ids are never reused (C, `CAPTURE-FULLTEXT.md` §2, §5). A vector is stored against (scope, passage id). Liveness always comes from the `passages` table, so a vector of a deleted page is never returned.

**Where vectors live: a flat file per scope, beside the FTS file.**
- Each record is the passage id, a day number, 256 int8 values and a scale, about 272 bytes.
- The file is append-only and memory-mapped, so it sits in the page cache, not corpusd's heap.
- Search is a brute-force scan. A time window is a range of the file: passage ids rise with indexing time; for pages indexed by the catch-up, the day field filters.
- Size: 0.85 M passages today is about 230 MB; at 250,000 pages about 280 MB; at a million pages (4.1 M passages) about 1.1 GB. The FTS files are 1.55 and 6.2 GiB at the same sizes (est, `CAPTURE-FULLTEXT.md` §3).
- Speed (A: about 1 G int8 multiply-adds a second in Go on one core): a full scan at 250,000 pages takes about 0.25 s; a 7-day window about 30 ms.
- Dimension: 256 int8 only. It costs one point of recall@10 (0.918 against 0.927, M). If the model changes, everything is re-embedded; the model's hash is in the file header.

**Not pgvector now.** It would put a derived index into the 4 GiB-capped Postgres, its WAL and the restore set. It has no int8 type (A: halfvec and bit only; check). Window filtering with HNSW is post-filtering. Revisit when either holds: unwindowed p95 above 400 ms, or more than 4 M passages. Either is a measured trigger, not a date.

**The embedder is a host service.**
- One `llama-server --embedding` process with the official Q8_0 GGUF, as a systemd unit on Node B. It is stateless, with 4 threads and a low CPU weight, so guests win under contention.
- That thread count is a size for the service, like a VM's vCPUs. It refuses no request and counts none.
- Only corpusd calls it. Guests never do.

**Backlog without a limit.**
- The work list is not a queue in memory. It is a fact derived from the index: passages that have no vector.
- corpusd's embed worker copies the catch-up's shape: walk, skip what is held, rest between batches, yield while searches are in flight. It always takes, in order:
  1. new captures' lead passages, newest first;
  2. published story versions;
  3. the backfill of leads, newest first;
  4. remaining passages.
- The backlog is a number in a log line (`remaining=`) and is never dropped. When it grows, the fix is more machines (Node A, the Mac) or a cheaper caller, never a refusal.
- Restart: the gap is re-derived at boot. This is a derived index, rebuilt the way the BM25 catch-up rebuilds (`CAPTURE-FULLTEXT.md` §4). No run resumes.

**First pass over 207,000 pages** (lead passage plus title, about 330 tokens a page; 68 M tokens):

| Where | Speed | Leads | Whole pages at 1,300 tokens (A) | Whole pages at 3,600 tokens (A) |
|---|---|---|---|---|
| Node B, 4 threads | 349 tokens/s (M) | about 54 h | about 9 days | about 25 days |
| Node B and Node A together | about 700 tokens/s | about 27 h | about 4.5 days | about 12 days |
| Mac M1, Metal | about 3,000 tokens/s (M) | about 6 h | about 25 h | about 69 h |

Vectors made on x86 and on the Mac agree (cosine 0.9999, M), so a file made on the Mac can be imported. The import checks it: sampled passages are re-embedded on Node B, and every sample must agree at 0.999 or above. Order of the pass: all published stories first (small, and they serve overlap and the radio), then leads, then whole pages.

**Pages not in English.** Of the non-English pages the 200 stories cited, BM25 put 33% in the top ten and vectors 72% (M). The vector half is what makes those pages usable. The BM25 tokenizer stays as built.

**Privacy and labels.** Owner-private captures are never embedded, just as they are never indexed by BM25 (`CAPTURE-FULLTEXT.md` §2, write path). A result that carries a passage carries `owner_private:capture`, unchanged from the BM25 branch.

## Fusion and windows

- **What to ship:** vector order, with BM25's top three pages guaranteed a place. It ties vectors alone (0.926 against 0.927 recall@10, M). It protects the two cases where BM25 wins: an empty title and a rare exact term. Reciprocal-rank fusion is worse (0.856, M), so it goes.
  - The guest's RRF merge (`researchtools/corpus_search.go:384`, branch) is replaced.
  - `CAPTURE-FULLTEXT.md` §5, which says "fuses by reciprocal rank", must be corrected. The doc and the measurement disagree.
- **Measure before changing it:**
  - recall@10 on the fixed 200-story set, rebuilt with real page text (corpusd can read it; the public routes could not);
  - lead-only against whole-page vectors on the same set;
  - a known-item set from logs once `lane/search-reuse` lands: a query, then the capture the same run later cited;
  - a split by language.
- **Pages, not passages:** a page's score is its best passage, with at most three passages a page among the candidates. This is the BM25 rule (C), applied to both halves.
- **Windows:** `since` and `until` apply to both halves before ranking, on the published time if known, else the fetched time. `order: recent` keeps the built rule: by time, among pages at least half as relevant as the best.
- **Floor:** the phone's cosine floor of 0.67 was set on 56 synthetic questions (M). Re-measure it on the corpus before a "nothing held" answer relies on it.
- **What the model is told:** pages searched by words and by meaning out of the total; the window; dropped words; and when the meaning half is still backfilling, the date before which it does not reach.

## The graph

**Events for every captured page.** The event layer is the one place pages join events (Jev at p 0.92 or more, `event_layer.go`), so it should take every capture, research reads included.
- Its shortlist adds vector neighbours of each capture's lead among events' member leads. This replaces the title-overlap slot.
- Jev still judges, in one call per flush holding every queued capture. No new call site, and nothing split.
- In the sample window, 63 of 74 captures minted a new event (M). A recency-and-title shortlist plausibly misses reworded reports.
- Production by event (`intake.event_wakes`) stays held off. This work does not touch it.

**A story's event ids, at each publish.**
- An event belongs to a story when two of its cited captures, or 30% of them, are counted members of it (tentative members do not count).
- The primary event is the one with the most counted members. If two events tie, the story carries both, since a roundup tells several events; that is also the signal for a false merge.
- No Jev call is needed: memberships are immutable (C, `store/world_event.go:15-21`), so the derivation is deterministic.
- The ids are exported to Store B as `reports_event` edges, copying how entities are exported (`publication_story_entities.go`). The tape is the authority and the export can be re-run.
- Events stay with the event layer on the computer, which is their custodian. corpusd holds a public projection.

**Entity merges.**
- What exists (C): the audit says there is no alias table; the code has a partial one. The learning loop records aliases on entity rows (`publication_entity_learning.go:160-210`) and detects "sprawl" (`publication_entity_candidates.go:123`). It refuses any form that is already another entity's name, so two ids already minted for one thing ("U.S." and "United States") can never join.
- Add a `choir.entity_merge` record (from, into, the Jev receipt). A daily sweep sends every pair the sprawl detector or vector similarity of the names proposes to Jev in one call.
- Reads resolve merges when they read; edges are never rewritten. A wrong merge is undone by an owner act that tombstones the record, as stopwords are revoked.
- corpusd stays custodian of entity rows.

**The neighbourhood read.** One corpusd read takes a topic, a draft's text or a story, plus a window. It returns:
- stories on the same events;
- stories whose headline-and-standfirst vectors score 0.86 or more (a proposal only);
- related stories by merged central entities and shared cited pages;
- the held captures on those events;
- the top hybrid passages.

Wire the existing walk (`RelatedStories`, `publication_related.go:593`; `basis.events` is always 0 today, `:188`) and extend it. Do not write a second walk.

**What to delete:**
- the setting `research.related_story`;
- the environment switches `CORPUSD_RELATED`, `CORPUSD_RELATED_STORY` and `CORPUSD_RELATED_ENTITIES`;
- the guest's substring search over events (`researchtools/corpus_events.go:54`);
- `intakeOverlapPicks`;
- the related walk's title-word filler;
- after the backfill, the capture term table's search role and the title half.

The setting is off for a real reason (C, `store/computer_setting.go`): its finish rule holds research on owner computers for stories that never publish. Fix the rule (it should apply only to desks whose story is published to the paper) in the same station that deletes the setting.

## The research flow: corpus first, web for the diff

1. **The brief.** Before the first model turn, the host delivers it, the way a management task's opening is delivered (`management_tasks.go`, `managementTaskOpening`). It is the neighbourhood read for the objective and for each hypothesis (one corpusd call; the query embeddings go in one embedder batch). That puts the existing stories on the event, with their claims and citations, the held captures and the newest held time on the desk's first turn. It costs no model turns.
2. **The gap list, in the first cell.** The desk writes, in an interim packet, each thread as one of: held (with capture ids), stale (the event has members newer than the existing story's last revision), or missing. The packet is the precommitment the run is settled against.
3. **Web searches only for gaps.** One `SearchAll` per gap (`lane/search-reuse`), which stops when wordings stop finding new pages.
   - A result whose URL the computer already holds is marked "held" with its capture id (a URL lookup in `proj_capture_index`), and is read with `ReadCapture`, never fetched again.
   - The stop is information, never a count: "this gap's last wording found 0 new pages."
   - Delete the prompt's "several searches per thread in one cell, with different wording" (`research.yaml:52-54`).
4. **What it fetches becomes corpus.** A fetched page is indexed inline (BM25 4 to 10 ms, est), embedded within minutes, and resolved onto events in the next flush. The next run holds it.
5. **Measures,** from the `lane/search-reuse` log lines (`run=` on the corpus-search line, `held=` on the "corpus read" line, C, commit 38f1b3e37):
   - corpus reads against web searches per run;
   - web searches per published revision;
   - the held share: of a story's cited captures, those held before its run began.
   
   Today 0 of 58 sampled stories cite an earlier story (M). The held share has no baseline yet.

## New story or revision

- **Where:** at a new story's first publish, after research.
- **Evidence the host computes:**
  - a primary event shared with a published story;
  - shared cited pages;
  - headline-and-standfirst cosine of 0.86 or more (M: at or above 0.86 the hand-read pairs were the same event or an update, all but one of the pairs read);
  - when the existing story was last revised.
- **Who judges:** where any signal proposes a candidate, Jev judges "same event, update, different". One call holds every story then waiting for its first publish.
- **Default rule:** fold when the primary event matches a story revised within 72 hours, or when Jev says "same" or "update" at p 0.8 or more. Otherwise publish as new and link the earlier story (`Follows`).
- **What happens to the work on a fold:**
  - The new story's Texture desk Casts its findings (captures and claims as a packet) to `texture:<existing>`. Casting with captures is the path management tasks already use (C, the task overlay, line 27).
  - The draft closes unpublished, marked `folded_into`. Nothing is lost: the captures are corpus, and the packet is the evidence.
  - The existing desk is the custodian. It may refuse with `Reject`, which publishes the draft as new. That refusal is the visible undo and settles the forecast "false".
- **A miss that published:** a daily sweep finds same-event pairs among published stories. The newer is marked "continued in" and leaves the front. Merging published text is not built.
- **Overlap measure:** the audit's clustering (116 of 1,584 stories beyond the first of their cluster, 7.3%, a lower bound, M), re-run daily, plus same-event and cosine-0.86 clusters.

## Other readers: one mechanism

- **Management's feed-or-open:** for each capture, the stories on its event and its vector neighbours, beside the 8-hour headlines (`intake_labels.go:1378`). Intake labels use the same shortlist.
- **Front page:** at most one story per primary event in a window. The rest appear as its related stories.
- **Related stories:** `basis.events` becomes real.
- **Autoradio:** a candidate whose event ids meet a heard story's ids is told as a follow-up. `lane/heard-land` already has `tellingFollowup` but matches by story.

## Stations

| Id | What | Depends on | Class | Claim and test | Rollback | Deletes |
|---|---|---|---|---|---|---|
| S0 | Land `search-reuse` and `capture-fulltext-2`; a `status-corpus-reuse` script computes the per-run ratios | none | orange; yellow | C129, C137 as written; a baseline week of ratios is printed | revert | none |
| S1 | Research prompt: corpus first, gap list, no rewording bursts | S0 | green (prompt) | Web searches per published revision fall 30% in 7 days against the S0 week, with the held share not lower | revert | `research.yaml:52-54` |
| S2 | Web results marked "held" by URL across the computer, read with `ReadCapture` | S0 | orange | Of web results, the share held is logged; new Reads of held URLs go to zero | revert | none |
| S3 | Brief delivered to the first turn (BM25 halves). Built in lane `corpus-brief` 2026-11-18: captures and published stories per thread, no events, no story dates (so no "stale" judgment yet) | S0 | orange | 95% of runs make no web search before their gap packet; web searches a day at or below 50% of the S0 baseline | revert | none |
| S4 | Embedder unit, corpusd embed worker, vector file; stories then leads | S0 | red (nix, a host model service) | Vectors' recall@10 at least 0.90 on the 200 set with real text; steady-state backlog drains within the day on staging | stop the unit, delete the file | none |
| S5 | First pass: on the Mac if allowed, else Node B and Node A | S4 | yellow (ops) | `remaining=0` for leads; import samples agree at cosine 0.999 or more | delete the file | none |
| S6 | Fusion in corpusd: vectors, BM25's top three kept | S4 | orange | The 200 set holds 0.92 or more; non-English pages in the top ten 65% or more | revert | the guest's RRF merge |
| S7 | Event layer takes research reads; vector shortlist | S4 | orange | Captures minting a new event fall from 85% to at most 60%; of 50 joins hand-read, false merges no higher than the event-wakes bar | revert | the title-overlap slot |
| S8 | Story event ids at publish, exported; `basis.events` wired; the setting and environment switches removed; finish rule fixed | S7 | red (export, tape) | 80% or more of new stories carry an event; of the audit's 84 clusters, 70% or more share an id | revert | `research.related_story`, `CORPUSD_RELATED*` |
| S9 | The neighbourhood read used by the brief, management tasks and intake labels | S6, S8 | orange | The "management story choice" line shows 50% fewer opens on an event that already has a story | revert | `intakeOverlapPicks`, the event substring search |
| S10 | Fold check at first publish | S8, S9 | red (crosses Texture custody) | Second-or-later stories in a same-event cluster fall from 7.3% to at most 3% in 14 days; the owner judges 20 folds | revert | none |
| S11 | Entity merge records and the daily sweep | S8 | orange | Groups split by alias fall, on 30 hand-picked pairs | tombstone the merges | none |
| S12 | Front dedupe by event; radio follow-up by event | S8 | orange | Front rows sharing a primary event fall to zero; the radio proof shows a follow-up telling | revert | none |
| S13 | Remove the title half and the term table's search role | S5, a week of S6 | orange | `by_labels_only` stays near zero for a week | revert | `proj_capture_terms` search |

**Not built yet:**
- pgvector and HNSW, until the latency trigger above;
- event wakes;
- a cross-encoder re-ranker;
- measuring fact overlap by model;
- merging the text of published duplicates;
- renting AWS (the Mac covers the first pass);
- vectors inside a listener's computer.

## Owner's decisions, with defaults

| Decision | Default |
|---|---|
| Captured text to the Mac for the first pass | Yes, for public captures only (owner-private captures are excluded at export). Vectors return; the text copy is deleted after import. If no: Node B and Node A, about 27 h for leads. |
| pgvector or files | Files now, with the trigger above. |
| Gemma licence | Apache 2.0 plus the Prohibited Use Policy. Our use (retrieval over news) is within it. Read the linked licence text before S4 lands, since the link names a `gemma_4_license` page. |
| Revise-or-new threshold | Same primary event and revised within 72 h, or Jev "same/update" at p 0.8 or more. |
| Money | Nothing in this plan spends any. Buying more search credits is a separate decision. |

## Risks: the three assumptions the design leans on hardest

1. **That the corpus holds what research needs.** Refute cheaply: from the S0 logs, the held share of the citations of 50 stories. Below 20% means the cut must come from `SearchAll`, not from the brief.
2. **That event ids and cosine 0.86 separate overlap from distinct stories.** The evidence is 30 hand-read pairs. Refute: read 100 pairs from the audit's clusters and 100 near misses. Check precision at 0.86 and the event-id agreement.
3. **That text runs about 1,300 tokens a page.** Refute: the read-only size query in `CAPTURE-FULLTEXT.md` §1.1. At 3,600 tokens, whole-page vectors take 1.5 threads all day and leads-only is the steady state.

## What I could not establish, and where docs and code disagree

**Facts not established:**
- the total number of stories in the archive;
- the share of cited captures that came from source polls against research reads;
- why a fifth of stories have no entities;
- whether a Texture desk's Cast may target another story's inbox (it holds `act.cast`, C `deskmanifest/manifest.go:374`, but the rules on targets are unread);
- whether the nixpkgs `llama-cpp` build has the EmbeddingGemma 2 architecture;
- how much capacity Node A has free.

**Disagreements:**
- The gateway still has a rate and in-flight limiter on Jev (`gateway/jev_limiter.go:24-45`, wired at `jev.go:72`). `CLAUDE.md` and `SYSTEM1.md` rule 4 forbid one.
- `intake_labels.go:43` names a "bucket".
- `CAPTURE-FULLTEXT.md` §5 prescribes reciprocal-rank fusion against the measurement.
