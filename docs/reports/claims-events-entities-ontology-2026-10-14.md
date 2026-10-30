# Claims, Events and Entities: The Ledger Between Sources and Stories

Date: 2026-10-14. Status: **design only.** Revised after a convergent
panel (§15): three members accepted with repairs and one sent it back
but approved E0 after revision; the repairs are folded in. No code
changes here. Mutation class of this note: **green**. The slices it
proposes are orange and red (§12).

## The owner's brief

At about 14:40 UTC on 2026-10-14:

> I think we may be missing a level of the architecture. On one level
> there's articles, at another level there is the search results or
> sources we're ingesting. But I think there should be an intermediate
> representation in the object graph that is more structured data of
> claims and events and entities. So source X said Y, source Z said Y
> prime, Y and Y prime overlap. [...] I think it becomes probably one of
> the most important layers of the data architecture that we're going to
> have. That's really the ontology.

About 14:50 UTC, widened:

> This needs to be so deeply integrated with precommitment records. X said
> Y at time T — that's the precommitment. And then we have to score that
> at time T′. The whole system is valuable because we have the system. So
> we're going from sources to the intermediate representation and then to
> the statistical analytics on top of it, which we can present as like a
> box score. [...] The precommitment records, the scoring, the
> prospective self-alignment is the essence of it. [...] Individuals,
> organizations and institutions: are they doing what they say? One could
> be normatively aligned, doing good things and even saying good things,
> but if the good things they're doing are different from the good things
> they're saying, they're missing a key dimension of alignment.

About 14:55 UTC, on uncertainty, risk and the thesis (owner's words,
lightly trimmed):

> If someone says something in a podcast, it's less certain, more
> ambiguous, more subject to interpretation. There could be many
> different ways to parse that statement, or context that's needed. We
> need to extract an estimated level of certainty.

> I am not worried about the defamation risk at all, because we're doing
> a serious best effort aiming for excellence at making this an automated
> system that is attempting to be neutral and multi-perspectival. It's
> essential that if you want to act in the public sphere, there's
> accountability — not cancel culture, but: do intelligent observers take
> you at your word? [...] maybe take on a little bit of legal risk to
> really provide the service for the world.

On the thesis: reputation, meaning "can I take you at your word?", is
the deep compression of intelligence. It applies equally to humans,
organizations, APIs and AI agents. Ask an agent "can you build this app
with X, Y and Z?":
- it says yes and delivers: intelligent;
- it says yes and fails: not;
- it says no but could have: poor;
- it says no because it knows it can't: excellent.

Today's AI is overconfident because nothing holds its reputation to
account. When a confident wrong answer about a parking ticket costs
anyone, it costs the user, not the model. So calibrated say–do track
records are a central eval: for Choir's own desks and agents first, on
their precommitments about their own work, and then for public actors.

**Ruling recorded (owner, 2026-10-14 about 14:55 UTC).** Legal risk from
scoring public actors is accepted by the owner. The quality standards
stay, as the excellence bar rather than as risk avoidance:
- attribution evidence;
- several readings of a statement;
- neutrality;
- a right of reply and correction;
- visible uncertainty.

Other directions from the same day that this layer serves:
- Merge stories, so sources grow much faster than articles. "The ratio
  of sources to articles is a crude proxy of quality."
- Keep the search results and pages we read in the object graph, and
  reuse them instead of searching again. Local embeddings and a large
  retrieval index are on the horizon.
- Use precommitment records and scoring to build a detailed reputation
  from observed track records: individuals, agents, teams, organizations,
  institutions and public figures.
- Contributions, op-eds and suggested edits
  (`docs/archive/reports/autopaper-ranking-and-contributions-intent-2026-10-14.md`).

## 0. Summary

**Three layers.**
1. **Sources.** Captures, blocks and the pages research reads (these
   exist).
2. **The ledger.** The new middle layer. A **claim** is one attributed
   assertion: speaker S said Y at time T, anchored to a verified quote span
   in one capture. A claim is a precommitment that Choir records on the
   speaker's behalf. Claims assert **propositions**, the content Y with no
   speaker attached; several claims across outlets and languages assert one
   proposition. They report **events**, real-world occurrences with a time,
   a place and participants, and they mention **entities**: people,
   organizations, places and products, carrying Wikidata ids where they
   exist. Propositions relate to each other: Y′ refines, updates or
   contradicts Y.
3. **Settlement and analytics.** At a later time T′, evidence, action
   events or a deadline settle claims. Scorers write attributed,
   revisable settlement records. Scores, track records and **box scores**
   per entity, outlet and agent are views computed over those records,
   never stored as fields.

**The claim is the precommitment.** A claim is recorded with a time that
can be proven (when Choir captured it), before the evidence that settles
it exists. That is the prequential condition the precommitment memo
requires. Three times are kept apart: when the speaker said it, when the
outlet says it published, and when Choir captured it. Only claims from
scheduled, timely captures enter a track record. Claims that research
dug up after the fact are recorded but never scored, so nobody can
cherry-pick (§4.1). The speaker never wrote a record, and does not need to. Choir
records it on the speaker's behalf. This amends the wording of the ledger
metamission's rule ("sources never precommit"), but not its intent: a
source's track record stays a view over settled claims, with denominators,
never one number (§4.1).

**Claims are typed by how they settle** (§6):
- factual assertions, by later evidence;
- forecasts, at a date, with proper scoring rules;
- commitments and promises, by observed actions;
- stated values and intents, by whether later actions are consistent with
  them;
- Choir's own agents' precommitments, by the trace, which Choir owns
  completely.

**Say–do alignment** is the share of an actor's checkable commitments
that its observed actions kept, with its sample size and uncertainty. It
is a separate dimension from whether the actions are good (§7).

**Five uncertainties are recorded on every claim** (§6a) and carried into
every score:
- attribution: did this speaker say it?
- interpretation: weighted readings of the statement, plus the context
  each needs;
- the speaker's own expressed confidence;
- extraction confidence;
- settlement confidence.

Ambiguous or weakly attributed claims are discounted. Statements that
cannot be scored are shown as **unscorable**, never as passes or
fails.

**The thesis** (§7.0): a track record answers "can I take you at your
word?", the deep compression of intelligence. Choir applies it first to
its own agents, as an eval of their say–do on their own work, and then to
public actors. Goodhart is countered by:
- proper scoring rules that reward calibration, not abstention;
- the system, not the speaker, selecting which claims count;
- difficulty weighting;
- coverage shown beside accuracy.

**Storage.** Each fact has one home (§5):
- Claims and mentions live inside one content-addressed **extraction**
  blob per capture, addressed like blocks.
- Canonical entities, events and propositions are small, versioned rows.
- Every link between them is a **judgment** with a judge, a probability
  and its inputs. Merges and splits are judgments, so they can be
  reversed.
- The current graph is a projection of the judgments under a governing
  policy, and it can be rebuilt.
- Shared evidence lives in Store B behind corpusd; private overlays stay
  on computers.

**Order of the slices (§12).** Each step waits for the one before it,
except that the say–do eval of Choir's own desks runs beside the first
two:
1. an offline study on a frozen, blind evaluation set, which measures
   routing and whether settlement is feasible, and can stop the layer;
   beside it, the say–do eval of Choir's own desks on their own
   precommitments;
2. claims and entities extracted for new captures on the paper, in shadow;
3. events resolved in shadow against L2a's labels, measured by merge
   recall;
4. routing by event, measured by sources per story;
5. Texture cites claims and writes what changed;
6. research retrieves from the corpus first;
7. outlet box scores, internal, settled by counting, with independence
   counted by origin, not by carrier;
8. public commitments and say–do, held to the excellence bar (§7.3);
9. publication of the public part into the Autopaper object graph.

## 1. Why now: the gap, with evidence

**Stories are sets of captures.** L2a's design defines a story as "the
set of captures fed into it" and a topic as "the set of stories joined by
Jev's 'new, same topic as' answers"
(`docs/archive/reports/l2a-synthesis-intake-design-2026-10-14.md` A3). Jev's label
points a capture at a *story* (`internal/store/capture_label.go:22-68`).
Nothing in the graph says what the story is about.

**Most routing errors confuse topic with event.** The after-window
re-read (`docs/archive/problems/l2a-merge-recall-on-the-paper-2026-10-14.md`,
now on main) found:
- 10 wrong merges, every one of them a feed by topic: the Guardian
  Europe live blog on the grid outage fed into the Zelenskyy-in-London
  story, and different Astra customer case studies folded into the
  launch story;
- cause C, a second story opened for an event that already had one;
- cause D, a capture fed into a story on the same topic but a different
  event.

**About a third of captures are re-captures.** 146 of 425 window captures
re-capture a URL already captured, because capture identity is URL plus
document hash and Hacker News text churns (`sourcegraph/captures.go:125-134`).

**Claims exist only as text.** `types.ReportClaim{Text, Sources}`
(`internal/types/commitment.go:363`), `types.CoagentPacketClaim{ClaimID,
Text, SourceIDs, Stance}` (`types/evidence.go:75-81`) and
`texturewrite.FrameClaim` are prose that desks write. None of them is a
graph object or carries an attribution or an anchor.

**Search results evaporate.** Asynchronous search results live in the
desk's REPL session (`yaegikernel/search_store.go`). Only pages passed to
`choir.Read` persist, as captures with `SourceID "web:read"`
(`researchtools/read.go:157-159`). Corpus search is a keyword match over
capture metadata, not over block text (`researchtools/corpus_search.go`).
The Mauritius stub trace shows research reading two pages and reporting
one (`docs/archive/reports/autopaper-deepen-important-stories-2026-10-14.md`).

**Reputation has been designed, but there is nothing to settle.** Protocol
§10 and the scoring design say a source's record is "a view over settled
claims with denominators"
(`docs/precommitment-records.md` §10;
`docs/archive/definitions/choir-ledger-metamission-2026-10-13.md` must_preserve).
No claim object exists to settle. The app design's R5 warns that "most
news sentences are attributions, not predictions"
(`docs/autopaper-sw-design-2026-10-09.md` §6).

**This has been tried twice:**

- **sourcecycled's processor and reconciler** extracted claims as a host
  daemon over queue tables. It failed on its substrate, not its idea
  (`docs/archive/world-wire-rearchitecture-2026-10-05.md`):
  - it ran on the wrong plane, as a host daemon with no ComputerID or
    tape;
  - the store was entangled: 8.5M og objects shared a server with control
    tables;
  - no supervision surface existed;
  - its lifecycle had five authorities;
  - result: 2,582 `dispatch_failed`, and 127 of about 211,000 requests
    completed.
- **The W0 schema** proposed five kinds (`source_version`, `claim`,
  `entity`, `thread`, `article`) and seven edges (asserts, about,
  corroborates, contradicts, supersedes). It was superseded before it was
  built
  (`docs/archive/definitions/choir-wire-kb-graph-schema-w0-2026-10-01.md`). Two of
  its rules carry forward: a cap on claims per item, and nothing is ever
  deleted, only superseded.

**The tension this design must resolve.** The Autopaper design said
"existing types only" and left "claim schemas" to the agents, following
the bitter lesson (`docs/autopaper-sw-design-2026-10-09.md` §1-2). This
note asks for a small fixed schema. §11 answers why. Agents cannot score
each other's private schemas across computers, and a track record needs
one schema (scoring design §6). The schema fixes **identity, attribution,
anchoring and settlement**. Content stays free text: there is no closed
predicate vocabulary and no event taxonomy.

## 2. What exists to build on (code map)

| Need | Exists | Where |
|---|---|---|
| Object and edge model, content hash, external keys | yes | `internal/objectgraph/object.go:23-46,66-175`; `registry.go:23-28` |
| Shared store for captures, with bodies by hash | yes (Store B, corpusd) | `platform/store.go:131-161`; `platform/captures.go:80-144`; typed write route `POST /internal/platform/captures` (`objectgraph_handlers.go:138-149`) |
| Blocks with content ids and a document hash | yes | `sourcefetch/blocks.go:50-118`; block blob `choir.capture_blocks.v1` |
| Anchor into one block | yes | `types.Ref{Doc, Version, CID, Hash, Quote}` (`types/ref.go:17-23`); byte offsets computed but dropped |
| Quote verification | yes | `quotematch.Match` (`internal/quotematch/quotematch.go:80-137`); exact, then whitespace and case tolerant, then punctuation tolerant; one block |
| Selector vocabulary | partly used | `sourcecontract/selector.go:5-19`; `text_quote` and `whole_resource` are used; `text_position`, `timestamp_range`, `transcript_segment`, `table_cell`, `data_vintage` and others are only in the validator |
| Typed cheap judgments | yes | Jev `typesafe/jev-1.13`, question shapes noul, choice and score (`gateway/jev.go:165-171`); $0.042 per million input tokens (`llmcost/pricing.go:85`); pair probe 26/26 |
| Arrival labels | yes (on the VM) | `agentcore/intake_labels.go`; `choir.capture_label`, `choir.capture_label_override` |
| Citation support verdicts | yes (binary) | `types/support.go:20-65`; `agentcore/support_judge.go`; `choir.citation_check` and `choir.support_verdict` memos; published as `choir.support_judgement` |
| Score records, separate from claims | designed | `CommitmentScoreRecord{TargetID, Scorer{ComputerID, Desk, AgentID, ModelID, Method}, Answers, SawPrediction}` (scoring design §4.1) |
| Emitter kinds agent, owner, edition, source, system | yes | `types/commitment.go:401-415` |
| Hypotheses with H-ids (open, settled, dropped) | yes | Cast hypotheses (`store/inbox.go`); L3 settles them |
| Embeddings | unwired | `internal/qdrant` with `qwen3-embedding:0.6b`; `EmbeddingModel` fields on captures unused |
| PROV-style provenance kinds | yes | `provenance_entity/activity/agent`, `was_derived_from`, `attests_to` (`registry.go`) |
| Real-world entity, event or claim kinds | **no** | `choir.event` is the computer's runtime event (`registry.go:63`), so the new kind cannot use that name |

Heresies discovered during the survey, named here (not repaired):
- **H-OG1.** The registry is enforced only on the `Service` path. At least
  22 kinds are written through `PutBatch` without registration, including
  `citation_check`, `support_verdict`, `support_judgement` and
  `commitment_record`.
- **H-OG2.** `choir.universal_wire_story_cluster` is registered and never
  written. It is a dead kind and a naming residue.
- **H-OG3.** `types.Ref` drops the matched byte span, so a citation cannot
  highlight its exact words without matching again.

## 3. The three layers

```text
 Layer 1  SOURCES            outlets ─ captures ─ blocks        (exists; Store B)
            │ extraction: quote-verified spans, one blob per capture
 Layer 2  THE LEDGER         claim ──asserts──▶ proposition ──refines/updates/contradicts──▶ proposition
                               │  speaker ▶ entity                 │
                               │  reports ▶ event ─participant─▶ entity
                               │  at T (capture time, provable)    └─part_of─▶ event (topic)
            │ settlement at T′: evidence, action events, deadlines, traces
 Layer 3  SETTLEMENT &       settlement records (attributed, plural, revisable)
          ANALYTICS          → views: track records, calibration, say–do, box scores
            │
 Stories   Texture documents cover events, cite claims, and publish box-score snapshots
```

Stories sit beside Layer 3, not above it. A story is a view over an event
cluster that Texture writes (§9.1). A box score is a view over Layer 3
that a story can pin.

## 4. Layer 2: the ontology

### 4.1 Claim is the precommitment

A claim has the precommitment memo's required fields
(`docs/precommitment-records.md`, "Record schema"):

| Memo field | Claim |
|---|---|
| Prediction (frozen) | the asserted proposition, the stance (affirms, denies, hedges), the modality (§6), any stated probability or estimative word, any deadline, and the speaker |
| Commit time | **T_commit = the capture's `fetched_at`**, which Choir can prove (store time, hashed). The outlet's `published_at` is recorded as *asserted* time and can be backdated or edited |
| Observation (later) | the evidence, action events or traces linked at settlement (Layer 3) |
| Score | settlement records, one per scorer (Layer 3) |
| Discrepancy | the memo's classes: confirmed, qualified, contradicted, unresolved, plus the classes §6 adds per type |
| Provenance | the speaker (entity), the carrier (the outlet and capture), the recorder (the computer, the extractor version and model), and the anchor (`Ref` plus the span) |

**Three times, never one** (panel repair P-8):
- `uttered_at`: when the speaker said it. This comes from the
  transcript timestamp, the article's dateline or the quote's context,
  and it is often only approximate.
- `published_at`: what the outlet asserts. It can be backdated or
  edited.
- `fetched_at`: when Choir captured it. This is provable, because it is
  store time and hashed.

**When a claim counts as a precommitment.** A claim is a precommitment
*for scoring purposes* only when Choir recorded it before the evidence
that settles it existed. In practice that requires all three:
- `fetched_at − published_at` is under a bound (default 48 hours);
- the claim was extracted from a **scheduled** capture: a source poll,
  or a search result recorded in the normal course of work;
- no settling evidence was already in the graph at `fetched_at`.

Claims from **targeted** captures (research digging into an actor's past
statements) and stale captures are recorded and attributed, but they
never enter a track record. A record that admitted them would let anyone
cherry-pick a speaker's old misses after the fact. Every claim carries a
`selection: scheduled | targeted` tag.

**Recorder is not speaker.** Choir records the claim, and the speaker
made it. This is how "X said Y at T is the precommitment" fits with the
must_preserve "sources never precommit":
- **Rule kept.** Sources never *emit records*. They never write into
  Choir's commitment ledger, and their track record is a view over
  settled claims, never one number.
- **Rule amended.** Their utterances *are* recorded as attributed
  precommitments.
- Wording delta proposed for the metamission's must_preserve: "sources
  never emit records; Choir records their claims as attributed
  precommitments; a source's record is a view over settled claims with
  denominators, never one reputation number."
- This needs owner ratification (D1).

**Attribution has two levels, each an explicit claim** (panel repair
P-4).
- "Reuters reports the ministry said 600,000 homes lost power" yields
  **two** claims from one span:
  - the **embedded claim**: speaker = the ministry, proposition "about
    600,000 lost power";
  - the **reporting claim**: speaker = Reuters, proposition "the ministry
    said P", modality `attribution`.
- They are linked by `relays`. Attribution fidelity, meaning did the
  ministry say it, is scored on the reporting claim, against a
  first-party capture of the ministry's statement or against independent
  relays. Speaker accuracy, meaning was it true, is scored on the
  embedded claim.
- When an outlet asserts something in its own voice, speaker and carrier
  are the same entity.
- Folding the two together is the classic error of reputation systems
  (§10): an outlet is not wrong because the minister it quoted was wrong.

### 4.2 Kinds (logical ontology)

Logical kinds are what the ontology means. §5 maps them to storage: some
are rows, and some are sub-objects inside a blob, the way blocks are.

| Kind | Meaning | Identity | Mutable? |
|---|---|---|---|
| **Outlet** | a publisher (registered domain, ownership) | an Entity of type organization with `publishes_at` domains; the registry `sources.Source` links to it | versioned |
| **Capture** | one fetched document version | exists: `autopaper:capture:<canonical_url>#<doc_hash>` | immutable |
| **Block** | a paragraph or similar unit of a capture | exists: content id inside the blocks blob | immutable |
| **Mention** | a span in a block that refers to something | `<extraction>#m<n>` | immutable |
| **Claim** | an attributed assertion: speaker, stance, modality and T_commit, anchored to a verified span | `<extraction>#c<n>` | immutable; a corrected article is a new capture, so a new claim, linked by `revises` |
| **Proposition** | the content asserted, with no speaker: normalized English gloss, original-language text, optional quantity and optional deadline | minted on its first resolution (`prop:<ulid>`); scoped to an event | versioned (gloss and label only) |
| **Event** (`choir.world_event`) | a real-world occurrence: a short free-text type, a time interval with a precision, places, participants with roles | minted on first resolution (`ev:<ulid>`); a Wikidata id when one exists | versioned |
| **Entity** | a person, organization, place, product, work or law, with `actor_scope` ∈ {organization, outlet, institution, public_official, public_figure, agent, private}; the default is `private` until positively set | `wd:Q<n>` when linked (QIDs are external identity *evidence*; a Wikidata redirect is followed by a `merged_into` judgment), otherwise minted (`ent:<ulid>`); multilingual aliases live in a side table, not in the versioned row, so new surface forms do not create versions | versioned |
| **Topic** | a standing subject that is not one occurrence ("the war in Ukraine", "AI chip export controls"), anchored to entities | minted (`topic:<ulid>`); a `choir.world_event` row with `scale: topic` and no required interval; `part_of` chains are capped at depth 3 | versioned |
| **Quantity** | a measurement inside a proposition: value, unit, measure, as_of, approximate | a field of Proposition, not a kind | — |
| **Judgment** | one judge's assertion of a link (Mention→Entity, Claim→Proposition, Capture→Event, a relation between propositions or events, a merge or split) with p | content-addressed record | immutable; superseded by newer judgments |
| **Settlement** | one scorer's answer for a claim at T′ (§6) | the scoring design's score record, targeting a claim | immutable |

Events are named `choir.world_event` because `choir.event` is taken by
the computer's runtime events.

**Prediction is not a separate kind.** It is a claim whose modality is
`forecast`, `commitment` or `intent` (§6). Choir's own desks' predictions
remain `choir.commitment_record` (P3 and P4). A record's subject refs may
point at propositions and events, and that link is how L3's hypotheses
"bear on" the world.

### 4.3 Edges (logical)

| Edge | From → to | Set by | Notes |
|---|---|---|---|
| `grounded_in` | claim, mention → block | extractor (deterministic) | `Ref` plus a block-relative `text_position`; the quote must pass `quotematch` |
| `speaker` | claim → mention | extractor | the mention resolves to an entity |
| `carried_by` | capture → outlet | registry (deterministic) | |
| `refers_to` | mention → entity | judgment | entity linking |
| `asserts` | claim → proposition | judgment | stance affirms, denies or hedges; fidelity exact, paraphrase or partial |
| `reports` | claim or capture → event | judgment | role: occurrence, background or reaction. This replaces the L2a event label's target (§9.1) |
| `participant` | event → entity | judgment | role: agent, patient, place or other |
| `part_of` | event → event | judgment | sub-event within a larger event or a topic. **This edge is the line between topic and event** |
| `develops` | event → event | judgment | a later development of the same story line |
| `refines` | proposition → proposition | judgment | Y′ is more specific than Y and consistent with it |
| `updates` | proposition → proposition | judgment | Y′ supersedes Y as time passes (the toll rose from 10 to 25); both stay true at their as_of |
| `contradicts` | proposition → proposition | judgment | cannot both hold at the same as_of |
| `copy_of` | capture → capture | judgment or deterministic | wire syndication; independence counts the original once |
| `translation_of` | capture → capture | judgment | |
| `relays` | reporting claim → embedded claim | extractor, then judgment | the carrier's attribution claim relays the speaker's claim (§4.1); also outlet B citing outlet A |
| `originates_from` | proposition support → entity or capture | judgment | the **origin** of a proposition: the first speaker or primary record it traces to. Independence for corroboration is counted by origin, not by carrier (§6) |
| `found_by` | capture or claim → search result | deterministic | a search hit led to the page or the claim (§9.4) |
| `revises` | claim → claim | deterministic | the same outlet and URL changed the span (a correction or a stealth edit) |
| `member_of`, `owned_by` | entity → entity | judgment | ownership defines independence groups (one broadcaster's language services count once) |
| `merged_into` | entity, event or proposition → same kind | judgment | redirect; reversible |
| `covers` | story (Texture document) → event | management or Texture | §9.1 |
| `cites_claim` | revision → claim | Texture write | alongside the existing source_ref (§9.2) |
| `fulfills`, `violates`, `partially_fulfills` | event (action) → claim (commitment or intent) | judgment | the say–do link (§7) |
| `settles` | settlement → claim | scorer | Layer 3 |

### 4.4 Attribution versus truth

- The ledger records **who claimed what, when**. It never stores "Y is
  true".
- Truth enters only as settlement records (Layer 3). Each one names its
  scorer, method and evidence. Several can disagree, and the
  disagreement is kept.
- The governing answer is a view parameter, not a field (scoring design
  §2).
- This keeps the Autopaper principle "show what the world indicates; do
  not adjudicate" (SW design §1.1). A proposition shows its stance
  structure: who affirms it, who denies it, the primary record if there is
  one, and the updates.
- A settlement is a scorer's attributed reading of the evidence, never a
  ruling.

### 4.5 Worked example (illustrative, simplified)

Three captures about Russian strikes on Ukraine's grid. The figures other
than the after-window's 600,000 are invented for illustration.

```text
capture A  Guardian (en) live blog   claim A1: speaker=Ukraine energy ministry
           "about 600,000 consumers were without power"   modality=assertion
capture B  Ukrainska Pravda (uk)     claim B1: speaker=Ukrenergo
           "понад 800 тисяч споживачів" (over 800,000)    as_of later than A1
capture C  TASS (ru)                 claim C1: speaker=Russian MoD
           strikes hit "energy facilities supporting the military-industrial complex"
           claim C2: speaker=Russian MoD (forecast): strikes "will continue"

proposition P1  "~600k consumers lost power" (quantity 600000, as_of T_A)
proposition P2  "800k+ consumers lost power" (quantity 800000, as_of T_B)   P2 updates P1
proposition P3  "the strikes targeted military-industrial energy facilities"
proposition P4  "the strikes targeted civilian energy infrastructure" (Ukrainian claims)
                P3 contradicts P4 (a framing dispute, kept, not adjudicated)
event E1  Russian strikes on Ukraine's grid, 13-14 Oct (part_of E0: the war; topic)
event E2  Zelenskyy's London visit (part_of E0)   ← the after-window's wrong merge was E1 into E2's story
```

The story covering E1 writes from this structure:
- P1 updated by P2, which gives the "what changed" line;
- the P3 versus P4 tension, with each side cited;
- C2 as a forecast to settle at its horizon.

The after-window's mistake would have been visible as structure: E1 and
E2 are siblings under E0, not the same event.

## 5. Identity, merging and storage

### 5.1 Observations are immutable and content-addressed

One **extraction** per capture and extractor version:
- `choir.capture_extraction`, keyed by `(capture doc_hash, extractor
  version)`;
- its body is a blob, `choir.capture_extraction.v1`, holding the mentions
  and claims, each with a `Ref`, a block-relative span and the original
  text of the quote;
- the row holds only indexed fields: capture id, counts, languages,
  speaker ids, cost and model.

Mentions and claims are addressed as `<extraction id>#m<n>` and
`#c<n>`, exactly as blocks are addressed inside a capture's blocks blob.
Two computers that extract the same capture with the same extractor get
the same id, so extraction is idempotent.

**Re-captures reuse earlier claims through a manifest** (panel repair
P-12).
- Block content ids are content hashes. A re-capture's extraction
  carries a **manifest**: for each unchanged block, the id of the prior
  extraction's claims anchored there (`carried`), with no new claim ids;
  for changed blocks, new claims.
- Extraction runs only on changed blocks, with the unchanged neighbours
  passed as context, so a speaker named one paragraph earlier still
  resolves.
- The 34% HN churn then costs almost nothing, and a repeated claim is not
  a new trial (§6, the scoring unit).

Changed blocks of the same URL produce `revises` edges. This is also how
stealth edits by outlets become visible: a later capture of the same URL
whose span changed with no correction notice.

**Volume guard.**
- At most 12 claims per capture, ranked by salience at extraction. This
  carries forward W0's guard.
- One extraction row per capture: about 1,700 a day at the after-window's
  rate.
- A capture beyond the cap is still entirely there: the cap limits
  extraction, never the capture.

**Volume budget for Store B** (panel repair P-11). These are estimates
at today's rate, to be measured in E1, with a tripwire at 2× budget:

| Object | Rows a day | Bytes a day (row + blob) | Churn |
|---|---|---|---|
| `capture_extraction` | ~1,700 | ~1.5 KB row + ~6 KB blob → ~13 MB | append-only |
| `graph_judgment` (one batch per capture per judge, plus management exports) | ~2,500 | ~1 KB row + ~3 KB blob → ~10 MB | append-only; supersession is a new row |
| `world_event` and topics | ~300 new | small | versioned on label change only |
| `proposition` | ~3,000 new | small | versioned on gloss change only |
| `entity` | ~200 new | small | aliases in a side table, not versions |
| `search_result` pointers (§9.4) | ~4,000 | ~0.5 KB row + ~3 KB hit-pointer blob → ~14 MB | append-only |

- That is under 50 MB a day of new Dolt rows and blobs. Blobs live in
  content storage beside capture bodies, not in Dolt. Captures already
  write about 1.4 GB a day.
- Store B's snapshot committer is already debounced; per-mutation Dolt
  commits were removed after the 218 GB history incident
  (`internal/platform/store.go:405-410`).
- The design adds no update-in-place rows: everything is append-only
  except the small canonical rows. The projection tables are rebuildable
  and stay out of Dolt history (in `dolt_ignore`d tables, or a SQLite
  file beside corpusd).
- Tripwire: Store B growth over 2× this budget for a day turns the
  extractor off on the paper (the per-computer enable), and the problem
  is recorded.

### 5.2 Canonical nodes are small and versioned

- `choir.entity`, `choir.world_event` and `choir.proposition` rows use
  identity mode external key and are versioned. A canonical row holds
  identity and display (label, aliases, gloss, time interval). It holds
  **no membership**: membership lives in judgments.
- **Entity identity.**
  - Use the Wikidata QID when the entity is linked. Two computers then
    converge without coordinating.
  - Otherwise the id is minted by the judgment that first created the
    entity.
  - Aliases are multilingual, taken from Wikidata labels and from
    observed surface forms.
- **Event identity.**
  - Events have no global key. Wikidata covers only large events, and
    GDELT event ids are per article.
  - An event's id is minted by its creating judgment and namespaced by
    the judging computer.
  - Two computers' events about one occurrence are joined by a
    cross-computer `merged_into` judgment.
  - **Time is part of identity**, as L2a's A3 says: an event has an
    interval, and a judgment that two captures report one event sees both
    intervals.
- **Proposition scope depends on modality** (panel repair P-3).
  - **Assertions** resolve within the event they report. This bounds
    the n² problem: the comparison set is a single event's propositions,
    typically under 30, never the whole corpus.
  - **Commitments, forecasts and intents** resolve by **speaker and
    subject entity, across events**. The shortlist is the speaker's open
    commitments and forecasts on the same subject, matched by quantity
    and deadline. "We will release open weights by the end of October",
    repeated in coverage of three different events, is one proposition
    with three claims. Otherwise say–do would double-count and miss
    reversals.
  - Cross-event relations between assertions are rare and are judged
    explicitly (`part_of`, `develops`).
  - E0 and E2 measure a **duplicate-proposition rate**, with a bar.

### 5.3 Judgments carry every link, and the projection is the current graph

A judgment is one record:

```text
judgment { id (content hash), judge {computer_id, desk|system, agent_id, model_id, method:
           extractor|jev|desk|panel|owner|rule}, assertions: [ {kind: refers_to|asserts|
           reports|part_of|refines|updates|contradicts|copy_of|merged_into|split|...,
           from, to, attrs, p} ], inputs {candidate ids shown, label/version ids, prompt
           version}, supersedes: [judgment ids], created_at }
```

Rules:
- **Every link carries confidence and provenance.** p, the judge, and
  what the judge saw are on every assertion. Edges made by `PutEdge`
  with a hashed metadata blob are not enough on their own.
- **Batch per capture, but every assertion is separately addressable**
  (panel repair P-6).
  - One resolution judgment per capture per judge pass holds all of that
    capture's assertions, so rows grow with captures, not with claims.
  - Each assertion has a stable id `<judgment>#a<n>`. `supersedes` names
    assertion ids, not whole batches, so reversing one merge never
    retracts the unrelated links in the same batch.
- **Every judgment records the judging computer's event head**
  (panel repair P-7). Store B is outside the restore set. When a
  computer is restored to an earlier head, its projection may ignore its
  own judgments made after that head, which the policy decides, and the
  judgments themselves remain as history.
- **The projection is the current graph.** A deterministic reducer
  folds the judgments under a **versioned governing policy**, with
  precedence **by relation type**, never one global ranking (panel
  repair P-6):

  | Relation class | Precedence (highest first) |
  |---|---|
  | routing (`covers`, the event a capture is fed to) | owner > management > Jev at p ≥ θ > extractor |
  | world evidence (`asserts`, `refers_to`, `refines`, `updates`, `contradicts`, `copy_of`, `originates_from`, `fulfills`) | owner > panel > count rule > Jev at p ≥ θ > research > management > extractor |
  | identity (`merged_into`, split) | owner > panel > management (events) or Jev at p ≥ θ (entities and propositions) > extractor |

  Within one rank, the later judgment wins. A merge cycle is resolved
  union-find style: the earliest-created canonical id survives.
  Management governs where stories go; it does not govern what the world
  says.
- The policy is a view parameter, as in scoring design §4.3. The paper
  reads its own policy, and a reader's computer may choose another.
- The projection's tables (memberships, counts, independence groups) are
  **rebuildable indexes, not authority**. They stay out of Dolt history,
  or are committed on a bounded cadence (SW design §2, storage).
- **Merges and splits are reversible.**
  - A merge is a `merged_into` judgment. The projection redirects, and
    nothing is deleted.
  - An unmerge is a later judgment that supersedes it. The projection
    recomputes, and every citation still resolves, because ids never die.
  - A split mints a new node and moves membership assertions to it.
- **Disagreement is data.** If Jev says same-event and management fed the
  capture elsewhere, both judgments stay. The override is the training
  example (L2a A3), and the disagreement rate is a monitored number.

### 5.4 Where each fact lives (single state authority)

| Fact | Home | Writer | Notes |
|---|---|---|---|
| capture, blocks | Store B | corpusd capture route | exists |
| extraction (claims, mentions) | Store B row plus blob | `system:extract` on the computer that ran it, through a typed corpusd route like the capture route | evidence is shared; content-addressed, so idempotent |
| entity, event, proposition rows | Store B | created by the judgment that mints them | identity only |
| judgments | Store B for a platform-tier computer (the paper), whose judgments are public by nature; a private computer's judgments stay in its embedded store as an overlay | the judge, through corpusd, authenticated as its computer | "evidence is shared, judgment is not" (protocol §10): a judgment in Store B is *that computer's* judgment, not a global truth |
| current memberships and counts | rebuildable projection tables | the reducer | never authority |
| settlements of world claims | Store B (same schema as `CommitmentScoreRecord`, home chosen by the target: a world claim lands here) | scorers | §6 |
| desks' own records and scores | the computer's tape (exists) | desks and the harness | unchanged |
| story covers event | the computer (story working state); published with the version | management or Texture | §9.1 |
| management dispositions (feed, decline, open) | **the computer's records** (`CommitInboxRecords`, `choir.capture_disposition`): this is the authority | management | Store B receives a derived **export** judgment carrying the source record id. It is never edited independently, and a mismatch is a bug the projection reports |
| citation support verdicts | the computer's `choir.support_verdict` memo (authority); published as `choir.support_judgement` (export) | `system:support` | used as settlement *evidence* by reference to the memo id, never copied as a second verdict |
| search-result pointers | Store B (§9.4) | the computer that searched | |

**One schema, homes partitioned by a stated function** (panel repair
P-7). Settlement and score records share one Go type. The home is
decided by `home(speaker class, target)`:
- the target is a desk's own commitment record → **the tape**;
- the target is a world claim (speaker is an outside entity, or the
  edition speaking about the world, including research's Report claims
  once extracted as claims) → **Store B**;
- any record about a world claim that originates on the tape (a desk's
  answerer settlement) is written once, to Store B, with the desk's
  record id as provenance. It is not also kept on the tape.

Each record therefore has exactly one home. This is a partition, not a
second path for the same fact. Standing question 5 is answered per fact.

**Restore.** Store B is outside a computer's restore set
(`docs/computer-ontology.md`, restore-set boundary). Judgments are
therefore forward-only: a correction is a new judgment, never a rewind.

**L2a's labels, avoiding a dual path (I5).**
- In shadow, the L2a `capture_label` (on the VM) and event judgments (in
  Store B) coexist, and both answer which story or event a capture
  belongs to.
- This is labelled transitional, with a **deletion clock**: when slice
  E3 lands, Jev's arrival question targets events, `capture_label` keeps
  only the "same article" answer and the cost, and its story pointer is
  retired.

### 5.5 Text, rights and what is public

- **Capture bodies stay in content storage.** The extraction blob
  stores, per claim:
  - the quote: the shortest span that grounds it, normally one sentence,
    capped at 300 characters;
  - the selector;
  - the English gloss.
  It does not store the article.
- **Public claims carry very short extracts.** A claim published into
  the Autopaper object graph carries the quote only within the rights
  rule: "very short extracts", about one sentence, with a link, for EU
  press under two years old
  (`docs/research/autopaper-media-and-rights-2026-10-14/README.md`
  §1.2). Otherwise it carries the gloss, the link and the selector.
- **Paywalled or rights-reserved captures** (TDM opt-out, `store_body_policy:
  bounded_metadata`) yield claims from metadata only, or no claims at
  all. The extraction records why.

## 6. Layer 3: settlement, typed by how a claim settles

The extractor sets a claim's **modality**, and a judgment can correct it.
The modality decides what can settle the claim, when, and how it is
scored.

| Modality | Example | Settles by | When | Outcome classes | Scored as |
|---|---|---|---|---|---|
| **assertion** (fact about past or present) | "600,000 lost power" | later evidence: a primary record, independent corroboration, a contradiction, an update | rolling; a horizon of 30 days by default, then unresolved | confirmed, qualified, contradicted, unresolved | accuracy with denominators; earliness; attribution fidelity for relays |
| **forecast** | "strikes will continue"; "likely to pass" | the world at the deadline or horizon | at the deadline | yes, no, unresolvable | Brier and log score of the stated or mapped probability; calibration curves; skill against a base rate and a reference forecast |
| **commitment** (a promise by the speaker about its own action) | "open weights by end of October"; "we will build 10 reactors by 2030" | the speaker's observed **action events** | the deadline, or rolling when no date is given | kept, partially kept, broken, pending, overtaken, not observable | say–do alignment (§7) |
| **intent or stated value** | "we put safety first"; "we oppose tariffs" | consistency of later actions | rolling, never closed | per action: consistent, inconsistent, irrelevant | consistency evidence; a number only above a specificity and sample bar |
| **Choir agent precommitment** | a desk's Cast expectation; a front-page importance prediction; Texture's own sentences | the trace (owned completely), counts, support verdicts | the record's deadline | the existing P3 and P4 outcomes; supported or unsupported | P4 calibration; procedural fidelity; support rate |
| **opinion, evaluation, question** | "a disastrous policy" | does not settle | — | — | recorded and attributed, never scored |

**Operationalization is Choir's act, and it is itself scored.**
- Turning "strikes will continue" into a checkable question is a choice:
  which strikes count, by when, and what counts as continuing.
- Choir writes resolution criteria when it records a forecast or a
  commitment: a criterion, a deadline (the stated one, or a horizon by
  modality), and a specificity check ("could two careful readers disagree
  about whether this was met?", scoring design §4.5).
- A claim that fails the specificity check is recorded as **not
  checkable**. That counts in the speaker's box score as a share
  (§8), not as a settlement.
- The criteria carry their author (extractor, research, Jev), so a bad
  operationalization is attributable to Choir, not to the speaker.

**Estimative words become probabilities by a published, versioned
mapping.**
- The default anchors are the IPCC likelihood scale and Sherman Kent's
  words of estimative probability ("likely" maps to about 0.66 to 0.9,
  and so on). An explicit probability is used as given.
- The mapping version is on the settlement, so scores can be recomputed
  under another mapping.
- **Per-speaker verbal calibration** is itself a box-score item: "when
  this ministry says 'likely', it happens 41% of the time (n = 17)". This
  avoids assuming everyone means the same thing by "likely".

**Who settles, cheapest and most independent first.** This is scoring
design §4.2 applied to world claims:

1. **count**, a deterministic rule over the graph, with no model call:
   - **Confirmation needs an independent origin** (panel repair P-2).
     An assertion settles `confirmed_primary` when a primary record of
     the fact is captured: the official dataset, filing, transcript, or
     the actor's own act for a statement about itself. It settles
     `corroborated` when at least k sources with **different origins**
     affirm it. Origins come from `originates_from`, not from carriers,
     so ten relays of the ministry's figure are one origin. A claim whose
     only support traces back to its own speaker stays **unresolved**:
     relays confirm attribution, never truth. The box score shows
     `confirmed_primary` and `corroborated` separately, because
     consensus is not truth.
   - **Count predicates exist only for a small closed vocabulary of
     settlement action classes** (panel repair P-9): release or launch,
     ship or deliver, sign or enact, appoint or resign, publish a figure,
     hold an event, start or stop an operation. Tens of classes, not
     hundreds, and never constraining claim content. A forecast or
     commitment whose criterion maps to a class resolves by a counted
     predicate ("an event of class release with participant E as agent
     before the deadline"). Everything else goes to Jev or the panel,
     with its own agreement bar.
   - For Choir agents, the settlement is the trace. **count governs where
     it applies.**
2. **jev**, a typed choice: "does evidence E confirm, qualify or
   contradict P?" and "does action event A fulfil commitment C?". It is
   blind to who asserted the claim, so an outlet's reputation cannot sway
   it. At about $0.00002 to $0.0001 per question (input tokens only, at
   $0.042 per million), it governs the judged cases, as P4
   decides.
3. **research**, when the evidence must be found first. Research reports
   with typed answers, written as `answerer` settlements; it is
   read-only toward the world, and recording a settlement is evidence,
   not a world effect.
4. **panel** (agentic consensus, offline), only on disagreements and on
   a sample; this supplies gold labels.
5. **human review** (the owner or an editor), on a random sample and on
   cases the panel flags. It is an audit of quality, not a gate on
   publishing. The owner accepted the legal risk on 2026-10-14. The bar
   for publishing a settlement about a named actor is the excellence bar
   in §7.3: verified attribution, readings shown, uncertainty shown, and
   a right of reply.

**Confidence and revision.**
- Every settlement carries its scorer, method, p, evidence refs and
  `saw_speaker` (whether the scorer knew who said it).
- New evidence produces a new settlement that supersedes, never an
  edit.
- A view picks the governing answer per policy, so a confirmed claim can
  later read contradicted, and the box score shows both the change and
  when it happened.
- Scorers are scored against count ground truth and against the panel
  (scoring design §4.4). That is the defence against a captured or lazy
  scorer.

**The unit of scoring is (speaker, proposition, reading), at its first
scheduled utterance** (panel repair P-3).
- A speaker repeating a figure, wire syndication and re-captures produce
  more *claims*, but not more *trials*. Repeats count as evidence that
  the speaker persisted in the claim, and a reversal is its own claim.
- Without this rule n is inflated, the outcomes are correlated, and every
  interval is too narrow.

**Scoring differs by modality** (panel repair P-10). One formula for
everything would be wrong:

| Modality | Scored as | Probability used |
|---|---|---|
| assertion | accuracy rate by **stance class** (unhedged, hedged, reportedly), with denominators | none. An assertion is not a forecast, and no Brier score is computed |
| forecast | Brier and log score; skill against a base rate; a Murphy decomposition (reliability, resolution, uncertainty) | the explicit probability; otherwise the versioned estimative-word mapping, fixed before any outcome; an unhedged "will" maps to a versioned default (0.85) that is refined per speaker by verbal calibration |
| commitment | say–do (§7) | none, or the speaker's own stated confidence when given |
| intent or value | consistency evidence | none |
| Choir agent | P4 unchanged: skill, calibration, and **"unresolvable counts as wrong"** for questions the agent wrote itself (scoring design §4.5) | the agent's own p |

**Unresolvable for agents differs from unscorable for public actors.**
- Choir's agents write their own questions, so vagueness is their
  choice, and P4 charges it as a miss.
- For public actors Choir writes the operationalization, so vagueness is
  shown in the unscorable bucket (§6a) rather than charged. Charging it
  would score Choir's extraction, not the speaker.

## 6a. Uncertainty: five kinds, recorded and carried into scoring

A statement in a podcast is less certain than a sentence in a ministry
press release. It can be heard wrong, parsed several ways, hedged, or
need context. Choir records five separate uncertainties on every claim
and never folds them into one number before scoring.

| # | Uncertainty | Question | Recorded as | Set by |
|---|---|---|---|---|
| U1 | **attribution** | did this speaker say this? | `p_attr` and its basis: verbatim span in a first-party capture (high); a relay by an outlet; a transcript (with the ASR model and the word-confidence of the span); a translation (with the model); an anonymous source; "reportedly" | extractor, corrected by judgment; relays and transcripts lower it, and a first-party primary record raises it |
| U2 | **interpretation** | what does it mean? | 1 to 4 **readings** `{gloss, weight, context_needed}` with weights summing to 1. Each reading may map to its own proposition | extractor (the readings), then Jev or a panel (the weights) when the readings matter for settlement |
| U3 | **expressed confidence** | how sure did the speaker say they were? | an explicit probability, or an estimative word or hedge ("I think", "likely", "may") with the mapping version; `asserted` when unhedged | extractor. **This is the probability calibration scores** |
| U4 | **extraction** | did Choir pull this claim out correctly? | `p_extract`, the extractor's own confidence that the span, speaker and modality are right, plus the quote check's tier | extractor; audited by a judge on a sample |
| U5 | **settlement** | how sure is the verdict? | the settlement's `p` (§6), its method, and scorer disagreement | scorer |

**Readings settle separately.**
- "We'll have it ready next quarter" can mean the speaker's fiscal
  quarter or the calendar quarter, and the release or general
  availability.
- Each reading has its own criterion, so a claim can be kept under one
  reading and broken under another.
- The settlement records an outcome per reading. The claim's outcome is
  the weighted mix, and the box score shows when the readings disagree.
- `context_needed` names the missing context ("which product", "before
  or after the merger"). Research may fill it, and its later judgment
  re-weights the readings.

**How the uncertainties enter a score.**

```text
weight(c)    = p_attr(c) · p_extract(c) · w_materiality(c)
               (interpretation is NOT in the weight; it enters once, through the readings below)
forecasts:   E[Brier](c) = Σ_r w_r · [ p_{s,r}·(q−1)² + (1−p_{s,r})·q² ]
               q = the speaker's probability (U3); p_{s,r} = the settlement's P(yes) under reading r (U5)
assertions,  outcome(c)  = Σ_r w_r · outcome_r        (outcome_r ∈ [0,1]; expected under U5)
commitments: also report the charitable bound: max_r outcome_r ("kept under reading A")
aggregates:  weighted mean m; n_eff = (Σw)² / Σw²; interval from Beta(1 + n_eff·m, 1 + n_eff·(1−m)),
             or a bootstrap over scoring units when outcomes are fractional or correlated
```

This corrects the draft's two mistakes (panel repair P-5):
- the draft multiplied a determinacy factor into the weight *and* mixed
  the outcomes across readings, which counted interpretation twice;
- the draft put a Beta(1,1) interval over weighted, fractional outcomes,
  where the n_eff pseudo-count or a bootstrap is the defensible form.

**Reading weights are frozen at T_commit by a judge that cannot see the
outcome.** The extractor proposes the readings, and Jev weights them at
extraction time, before any settling evidence exists. A re-weighting
after evidence exists is recorded but used only for display, never for
calibration.

**Unscorable is a result, not a pass or a fail.** A claim goes to the
**unscorable** bucket, counted and shown but never averaged in, when any
of these holds:
- `p_attr` < 0.6;
- no reading has weight ≥ 0.25, which means it is too ambiguous to settle
  under any reading. Several readings that each settle are fine;
- the specificity check fails;
- the evidence leaves it unresolvable;
- the outcome is not observable (§7.2);
- the claim is `targeted` or stale (§4.1), so it is recorded but not
  scored.

This bucket applies to public actors only. Choir's agents follow P4's
"unresolvable counts as wrong".

The thresholds are defaults (D8), on the settlement so they can be
recomputed. An actor whose claims are mostly unscorable shows exactly
that: "70% of recorded commitments unscorable (vague 45%, ambiguous 20%,
unattributed 5%)". The share is itself information about the actor.

**Where each uncertainty comes from in the pipeline (§12.1).**
- The extractor emits U1 (from basis and relay depth), U2 (readings),
  U3 (hedges) and U4 (self-confidence) in the same call.
- Transcripts and translations lower U1 by their own recorded
  confidence.
- Jev re-weights U2 only when settlement depends on the reading.
- Scorers set U5.

None of these is invented later from the outcome. They are all frozen
with the claim at T_commit, except the U2 weights, which a judgment may
revise with a recorded reason. A revision made after settlement evidence
exists is flagged and excluded from calibration, to prevent fitting the
readings to the outcome.

## 7. Say–do alignment

### 7.0 The thesis: can I take you at your word?

A track record answers one question for any actor: **can I take you at
your word?** That question compresses a great deal of intelligence into
a single checkable relation between what an actor says and what then
happens.
- It applies equally to people, organizations, institutions, APIs and AI
  agents.
- For a capability question ("can you build this app with X, Y and Z?")
  the four outcomes rank: yes and delivers > no, because it knows it
  can't > no, though it could have > yes and fails.
- A calibrated "no" is worth more than an overconfident "yes". **Refusals
  and declines are claims too**: a self-forecast about capability,
  scored like any forecast.

Today's models are overconfident because a wrong confident answer costs
the user, not the model. Choir's desks make precommitments about their
own work all day: P3 expectations, Cast hypotheses, and "I will cite
three sources". So **Choir's own agents are the first population** for
this eval. The action log is complete for them, the scorer is
independent, and the record changes which model and desk get trusted
with what (context packs, routing). Public actors come second, with the
same ledger and the same views.

**Goodhart, benchmaxing and "the tails come apart"**, with concrete
countermeasures:

| Gaming move | Countermeasure |
|---|---|
| **abstain**: hedge everything, so nothing is ever wrong | Corrected after the panel (P-5): hedging is **not** worthless to a poor forecaster. Zero skill beats negative skill, so a hedge is a safe harbour. The countermeasure is therefore display, not arithmetic. The **hedge share** and the **unscorable share** sit in the headline row, next to accuracy. Forecast cells show the Murphy decomposition, so **resolution and sharpness** (how far forecasts move from the base rate, and whether they discriminate) are visible beside reliability. A record of hedges reads as "calibrated, uninformative", which is exactly what an intelligent observer should conclude. For Choir's agents, P4's "unresolvable counts as wrong" closes the vagueness route |
| **decline hard tasks** (a "no" can never be shown wrong) | Panel repair P-6b. A decline is a capability self-forecast, but it settles only if someone tries. A **random sample of declined tasks** (default 10%) is run anyway, on another desk or model, inside the existing engineering admission, to settle the counterfactual "could it have been done". Unsampled declines are counted and their rate is shown; their accuracy is never imputed |
| **cherry-pick easy claims** | The **system selects** which claims count: every extracted claim from scheduled captures, plus every desk precommitment. The speaker never chooses. Skill is measured against a base rate and a reference forecast (a market price where one exists), so easy claims earn about zero |
| **say little** | Volume and coverage are shown beside accuracy, in claims made, checkable and settled per period. A silent actor reads "n too small", never "accurate" |
| **make many trivial promises** | Materiality weight; specificity check; difficulty weighting by the base rate |
| **optimize the scorer** (desks) | Scores stay off the acting desk's context (I26). Scorers are blind to the speaker and are themselves scored against count. A random audit sample is held out |
| **one number gets optimized and decouples from the goal** (the tails come apart) | There is never one number. Accuracy, calibration, say–do, coverage and correction behaviour are shown separately, each with n and an interval. The panel watches for divergence between them (high accuracy with falling coverage, say) as a Goodhart signal |
| **influence the outcome** (self-fulfilling forecasts by an actor about its own actions) | That is the commitment modality by design: doing what you said is the point. For forecasts about the world, the speaker's control is flagged, as `self_controlled` in P3, and scored separately |

### 7.1 Definition

For an actor A over a window W, take:
- C_A, A's **checkable** commitment claims: commitments, plus intents
  specific enough to be checked;
- whose deadline has passed, or which have linked action evidence;
- and which fall in a domain where Choir's sources **observe A's actions**
  (the coverage condition, §7.3).

Then:

```text
say–do(A, W) = Σ_c w(c) · s(c) / Σ_c w(c)      over c ∈ C_A settled in W
   s(c) = 1 kept, 0.5 partially kept, 0 broken          (overtaken / not observable excluded, counted)
   w(c) = materiality weight (default 1; the PSA weight w = P × impact × relevance is a later refinement)
reported with n (scoring units, §6), n_eff, the excluded counts, and an interval
   (Beta with n_eff pseudo-counts, or a bootstrap over units, §6a) — never as a bare number
```

Two companion measures:
- **consistency** of actions with stated values: per action, consistent,
  inconsistent or irrelevant, judged; shown as evidence until n is
  enough;
- **silent reversals**: action events that contradict A's own stated
  intent with no prior statement of a change.

**Separate from normative goodness.** Say–do is not whether the
commitments were good. A kept promise to do harm is aligned on this
dimension, and a broken promise to do good is not. That is the owner's
point: an actor can do good and say good and still be misaligned, if
what it does differs from what it says. The box score never folds say–do
into a "goodness" or trust number. It sits beside whatever normative
judgment the reader brings. This is the procedural surface of the five
(`docs/precommitment-records.md` §4), kept separate from
the normative one.

### 7.2 How the graph computes it

```text
commitment claim c (speaker A, proposition P: "A releases open weights", deadline D)
  ← criterion + specificity check (Choir-authored, attributed)
event e (participant A role=agent, type "release", time t)  — from later captures
judgment  e ─fulfills(p=0.93)─▶ c      (jev, blind; or count when the predicate is typed)
at D with no fulfilling event: outcome "broken" only if coverage(A, domain) ≥ bar,
  else "not observable"
settlement(c) = {kept | partially | broken | pending | overtaken | not observable}, scorer, evidence
```

**For AI agents the loop is complete.**
- Choir owns the action log, so for its own desks say–do is procedural
  fidelity measured from authenticated traces.
- The comparisons: declared envelope versus capabilities used; Cast
  expectations versus outcomes; "I will cite three sources" versus the
  revision.
- This is the strongest form, and it is where the claim that alignment
  can be measured gets tested first (PSA §4.1).
- **Asymmetry, stated plainly:** for people and institutions, actions are
  observed only through reports, so their say–do is bounded by coverage.

### 7.3 Failure modes, written first

| Failure | What goes wrong | Guard |
|---|---|---|
| vague claims | "we will do more on safety" settles to anything | the specificity check; unspecific claims are counted as **not checkable** and shown as a share, never scored |
| unfalsifiable claims | values with no observable action class | modality intent; consistency evidence only; no number below the bar |
| selection bias in recording | we see only what our sources report and what the extractor ranks salient; quoted people are not a random sample | box scores show source coverage and the extraction cap; claims count only when extracted from captures polled by schedule, not from targeted digging, so one actor cannot be cherry-picked (a hypothesis to test) |
| selection bias in settlement | broken promises are newsier than kept ones (or the reverse) | "not observable" is separate from "broken"; settled share reported; panel audit of a random sample of open commitments |
| base rates | "the sun will rise" scores well | skill against a base rate and a reference forecast (a market price where Polymarket covers the question); trivial claims carry low weight |
| gaming by actors | many easy promises, hedging everything | specificity weighting; the hedge share shown; materiality weight |
| gaming by Choir's agents | desks learn the scorer | scores stay off the acting desk's context (I26); blind scorers; Jev scored against count |
| misattribution | the wrong speaker; a quote out of context | the quote must pass `quotematch`; speaker resolution is a judgment with p; low p is never published |
| translation | a gloss distorts the original | the original span is stored; translation errors are disputable; cross-language settlements are sampled by the panel |
| outlet versus speaker | an outlet blamed for its source's error | relay versus assertion (§4.1) |
| harm to a named actor from a wrong settlement | "X broke a promise" when X did not, or under a reading X did not mean | The owner accepted the legal risk (2026-10-14). The **excellence bar** for any published settlement about a named actor: a verified verbatim span (U1 ≥ bar); every plausible reading shown with its weight (U2); the dated evidence linked; uncertainty visible; a **right of reply** (a reply or dispute from the subject is a claim of theirs, shown beside the settlement, and it re-opens settlement); corrections published as prominently as the settlement. Scope: actors who act in the public sphere. Private individuals are not scored (the metamission excludes "individual-level tracking of people in public-discourse sources"; D6) |
| Goodhart on the box score | optimizing the displayed number | several measures, never one; denominators; no ranking of people by a single score |
| media selection of actions (panel) | outlets report failures and conflict more than routine kept promises, so say–do inherits newsroom selection | "not observable" is the strict default, unless the actor's **domain coverage** passes a measured bar: primary channels polled (official sites, gazettes, filings, release notes) and not only news. Coverage is shown **per actor**, never as a global figure |
| data protection (panel) | scoring natural persons is profiling under GDPR as well as a defamation question; the owner's ruling covers defamation risk | named separately in D6: rights of access and objection, and how far the journalism exemption reaches. The reply path doubles as the objection channel |
| false "silent edit" signals | page reflow, A/B variants or feed churn read as stealth edits by `revises` | the "silent span changes" cell is not shown until its false-positive rate is measured on a sample |
| reference-forecast skew | market prices exist only for listed, money-attractive questions | skill against a market reference is reported only where one exists, with its coverage stated; otherwise the base rate is used |

## 8. The box score

A box score is a **view**: a query over settlements with a scope (entity,
outlet, agent, team or institution), a window, topics derived from
subject refs, and a governing policy. It is never a stored number. A
published story pins a **snapshot** of it, with the query, the policy and
the as-of time hashed, so a reader's numbers never change under them. A
"newer record available" indicator works like transclusion pins.

```text
Ukrenergo (wd:Q4122614 · organization · ua)            Jun 1 – Oct 14 2026 · scorer policy: paper default
Claims recorded 214 (scoring units 131 scheduled · 23 targeted, not scored)   checkable 61%   hedged 18%   by type: assertion 188 · forecast 9 · commitment 14 · intent 3
Unscorable      84 (vague 51 · ambiguous 19 · weak attribution 9 · not observable 5)   n_eff 118
Assertions      settled 97 / 131 checkable · confirmed by primary record 41 · corroborated (≥2 origins) 48
                · qualified 5 · contradicted 3   [92% ± 5]   self-sourced only, unresolved: 22
                updates own figures: 22 (median +4h)   first to report: 31 of confirmed
Forecasts       resolved 6 (n too small for calibration) · Brier 0.19 vs base 0.24
Commitments     kept 7 · partial 2 · broken 1 · pending 4 · not observable 0   say–do 0.80 [0.55–0.93]
Corrections     2 announced · 0 silent span changes seen
Scorers         count 71% · jev 26% · panel 3% · human 0%   scorer disagreement 4%
```

The numbers are invented to show the shape.

**For an outlet**, add:
- attribution fidelity on relays;
- earliness, as median lead time on confirmed propositions;
- copy share: how much of its output is a `copy_of` another outlet;
- correction behaviour: announced corrections versus silent span
  changes, from `revises`.

**For a Choir agent or model**, the row draws on P4:
- calibration on count-resolved questions;
- procedural fidelity;
- support rate of the sentences it cited;
- the self-score gap.

**What it never shows:** a single overall trust number, a rank of
people, or a number below its sample bar. Below the bar the cell reads
"n too small".

**In Autopaper.**
- **Claim chips.** An attributed claim in a story ("Ukrenergo said…")
  gets a compact line in its citation popover: "Ukrenergo: 89 of 97
  checked figures confirmed since June". This uses the popover that
  already groups scores by scorer (app design §4.5).
- **Pages.** Entity and outlet pages show the full box score, with every
  cell drillable to its claims and evidence.
- **Choir's own record.** The paper's own box score is public: its
  sentences' support rate, corrections, and the forecasts it made in
  stories. Choir is scored like anyone else, and that is the credibility
  argument.

**In Autoradio.** The script carries the same structured snapshot, and a
host line reads it as a record, never as a verdict: "Ukrenergo's outage
figures have held up in 89 of 97 checks since June". Rules:
- spoken only when n meets the bar;
- the scorer is named on the episode page;
- never a characterization of a person ("unreliable", "liar").

## 9. How each consumer uses the ledger

### 9.1 Story merging: events are the merge keys

- A capture's claims `report` events. A story `covers` one primary event
  and may cover its `develops` successors.
- A new capture whose event is covered by story S updates S. A capture
  whose event is new but `part_of` the same parent as S's event is **the
  same topic but a different event**: it opens a new story, or the
  topic's researcher decides. This is exactly causes C and D and the 10
  wrong merges.
- Several Textures may cover one event (SW design §1.4). The front page
  groups them by event.
- **Jev's arrival question changes target from story to event.** The
  shortlist is candidate events (recent events, events whose participant
  entities overlap the capture's, and later, embedding neighbours of the
  capture's claim glosses), with the same choice shape and
  the same O3 rule: a high-confidence same-event label settles the feed
  without a model turn.
- **The owner's ratio becomes measurable.** Captures per event and
  independent outlets per event grow with sources. Stories grow only with
  events, which is sublinear. The measures: captures per new story,
  independent outlets cited per published story, and new stories per
  hour against captures per hour.

### 9.2 Texture writing and citation

- **Event dossier.** Texture's research packet gains an event dossier: a
  read view `choir.EventDossier(event, since_revision)` that lists:
  - propositions with their affirming and denying claims, grouped by
    independence group;
  - updates and contradictions;
  - **what is new since the story's last revision**.

  This is the A4 synthesis contract made structural: agreement, conflict,
  and the differential update of ruling 11.
- **Texture cites claims.** `{{cite h "quote"}}` already verifies a quote
  in one block. A claim id resolves to the same `Ref`, so a citation can
  name the claim. The revision records `cites_claim`, and the reader shows
  the attribution ("Ukrenergo, via Reuters"). I14 holds: the citation is
  still a source entity and a transclusion, now with a claim attached.
- **The quote checker runs twice.** It runs at extraction (no claim
  without a verified span) and at Texture's write (unchanged). The
  support judge's `CitationPair` gains the claim id, so a support verdict
  becomes evidence for the claim's settlement as well as for the
  sentence.
- **Choir's own sentences become claims.** The paper is the speaker and
  the edition is the emitter. They are settled by support verdicts and by
  later contradiction or correction. That is the paper's own box score.

### 9.3 Front-page importance and depth

Signals per event:
- independent outlets and languages;
- velocity, as claims per hour;
- **tension**, as the density of `contradicts` edges and open disputes;
- Jev's importance score (LF0).

**Depth** is the share of the event's well-supported propositions that
the current version cites. It replaces the `source_count < 3` heuristic
in the deepen tick. A thin story becomes "important event, low
coverage", and deepening has a concrete target: the uncited propositions.

### 9.4 Search reuse and retrieval

- **Claims, propositions and entities are the retrieval units.** They are
  short, normalized, multilingual through glosses, and attributed.
- Research gets `choir.SearchClaims(query, entities, since)` and
  `choir.EntityTimeline(entity)` before any web search, which makes the
  "corpus first" rule concrete (L2a B2).
- **Search results feed the same extraction as captures** (owner, about
  15:05 UTC: "I really want to save search results as content, but we
  should be extracting into our object graph — that's the transformative
  work, so that we're not just copying the search result and reusing
  that"). This aligns with `choir.search_result` in
  `docs/archive/reports/search-efficiency-and-reuse-2026-10-14.md` §2 and narrows
  what it keeps.
  - **Kept verbatim, as a pointer object** (`choir.search_result`, one
    per search): query, normalized query key, run, desk, computer,
    providers answered and skipped, time, and per hit: URL, canonical
    URL, title, `published_at`, provider and rank. Edges:
    `search_result —returned→ url-key`, and `found_by` from any later
    capture of that URL.
  - **Not kept: snippet text.** It is used *transiently*. The snippets of
    a search's hits go to the extractor in one batched call while the
    run holds them, and are then dropped. The blob holds pointers, not
    snippets. This narrows the companion report's "merged hits with
    snippet" blob.
  - **Kept, transformed:** the claims, entities and events extracted
    from the snippets, each attributed to **the source page's outlet**
    (carrier) and speaker. Each gets `found_by` to the search result and
    an anchor holding only the short quote span that grounds it, capped
    like any claim quote (§5.5). The basis is `snippet`, so attribution
    confidence (U1) is lowered, because a snippet is a provider's excerpt
    and may be truncated or out of context. Snippet claims are
    `targeted` unless the search was part of the normal course of work,
    and they never settle anything on their own.
  - **Upgrade on read.** When the page is later read and captured, the
    claim is re-anchored to the capture's block, and `quotematch` must
    pass. U1 rises, or the mismatch is recorded as a correction to the
    snippet claim.
  - **Reuse instead of re-search.** Before a web search, research's
    `choir.SearchClaims` and the normalized query key return earlier
    searches' extracted claims and pointers on the same subject, with
    their age. Research re-searches only if they are stale (a freshness
    bound by subject: hours for breaking news, days for documentation) or
    insufficient. The companion report's refused-read fallback ("return
    the stored hit text") becomes "return the extracted claims with their
    snippet basis".
  - **Provider terms (D7).** Titles and URLs are safe to keep under any
    reading. Transient processing of snippets falls within ordinary
    caching use. Extracted facts are Choir's own transformative work, and
    facts are not copyrightable. But some provider terms restrict
    building **derived datasets** from results, so each provider's terms
    are an owner check before extraction from snippets is turned on.
    Pointers can ship first.
  - **Cost.** About 4,000 searches a day. Extraction is batched per
    search and skips hits whose URL already has a capture or an earlier
    extraction, at about $0.001 to $0.002 per search. Measured in E5.
- This needs more research first (owner: "we shouldn't expect to ship
  this in the next hour"). It is slice E5, after the capture path is
  proven.
- **Embeddings are a derived index, never authority.** The unwired qdrant
  path with `qwen3-embedding:0.6b` embeds propositions' glosses, claim
  glosses and entity labels: short texts, cheap, and cross-lingual. It
  produces shortlists for resolution and for retrieval. A large index
  over blocks comes later. Similarity never decides membership; a
  judgment does (`docs/source-external-data-publication.md` §7).

### 9.5 Contributions, op-eds and suggested edits

- **An op-ed** cites stories at pinned versions and cites claims and
  propositions directly. The contributor is the speaker of their own
  claims, which enter their box score like anyone's.
- **A suggested edit** is a `publication_proposal` (it exists as
  `reader_derivative_proposal_v0`). It carries the contributor's sources,
  which become captures with provenance and so yield claims.
- **A dispute** of a story's sentence is a contributor claim plus a
  proposed `contradicts` judgment. Texture or management accepts it; Jev
  checks the citations; Texture stays the only writer.
- **A dispute from a scored subject** (a person or an organization
  contesting its settlement) is the same object, shown beside the
  settlement.

### 9.6 Precommitment records and reputation

Covered by §§6-8. Three links tie the ledger to the existing commitment
ledger:
1. a desk record's subject refs may name propositions and events (the
   L3 hypotheses "bear on" them);
2. an edition Precommit about the world (app design §4.5) targets a
   proposition, so the paper's forecasts and a speaker's forecasts on the
   same question are directly comparable;
3. LF's front-page importance predictions resolve by counted growth of
   the event: outlets and captures after T.

### 9.7 Publication into the Autopaper object graph

| Public (Store B, readable logged out) | Private (stays on the computer, or in Store B with private scope) |
|---|---|
| entities, events and propositions; claims with gloss, selector, link and a very short extract within the rights rule; the paper's judgments with judge identity; settlements and box-score snapshots pinned by published versions | full capture text; claims from private feeds, private documents or contributors' private sources; a private computer's judgments (its overlay); reader attention; settlements about named actors that do not yet meet the §7.3 excellence bar (span verified, readings shown, reply path) |

Another computer **adopts** public ledger objects onto its own tape by
reference (id and content hash). It may judge them differently, and its
judgments stay its own until it publishes them. This is the
cross-computer data subscription that the 10-05 re-architecture named as
missing. Here it is just reading Store B plus writing local judgments.

## 10. Positioning: neighbours, what each gets right, and what none captures

| System | Gets right | Misses |
|---|---|---|
| **Nate Silver (Silver Bulletin, formerly FiveThirtyEight)** | sources rated by historical error (pollster ratings, house effects); weights inputs by track record; probabilistic outputs; transparent method | narrow domains (elections, sports); rates instruments (polls), not attributed claims; no say–do; the model is the product, not an open ledger |
| **Tetlock and Good Judgment** | explicit probabilities, precise resolution criteria, Brier scores, calibration; evidence that track records beat credentials | forecasters must opt in to authored questions; it cannot score the public's natural, vague claims, the "vague verbiage" Tetlock himself named; no promises |
| **Prediction markets (Polymarket, Kalshi; Metaculus as a forecasting platform)** | prices aggregate dispersed belief; resolution criteria are written first; real stakes; Metaculus keeps track records | a price is an anonymous aggregate, not who believed what; only listed questions; money-weighted and manipulable; resolution disputes (oracles). **Use:** a reference forecast and base rate (Polymarket is already a source type) |
| **Palantir (Foundry Ontology)** | ontology as the core asset: objects, links, actions; entity resolution; an operational graph that drives action | stores facts as given, or source-tagged, not as contested attributed claims; no scoring of sources over time; closed and operator-centric; truth is whoever writes the row |
| **Fact-checkers and ClaimReview** | claim-level, attributed, with appearance and date; a verdict scale; a public schema | hand-picked viral claims (heavy selection bias); one organization's verdict collapses truth to one rating; slow; no forecasts or calibration. **Use:** emit ClaimReview-compatible exports of settlements later |
| **Promise trackers (PolitiFact's Obameter and Trump-O-Meter; Polimeter)** | the say–do dimension, with kept, compromise, broken, stalled and in-the-works ratings | a few hundred hand-curated promises, one actor at a time; no probabilities; not comparable across actors; never applied to organizations or AI |
| **GDELT** | events at global, multilingual scale; actor and event coding (CAMEO); metadata that may be reused | per-article event records with heavy duplication; a closed taxonomy; no who-said-what settlement; no scoring |
| **Wikidata** | entity identity (QIDs); statements with qualifiers and references; multilingual labels | stores a consensus statement with ranks, not a time-ordered record of who claimed what when; no scoring. **Use:** entity identity |
| **X Community Notes** (added after the panel) | plural rating with bridging: a note shows only when raters who usually disagree agree, the closest neighbour to "neutral, multi-perspectival" | rates single posts, not speakers over time; no forecasts, no say–do; volunteer selection |
| **Automated claim checking (ClaimBuster, FEVER-style verification)** and **AI forecasting evals (ForecastBench)** | claim detection and verification at scale; scored forecasts by AI systems | one-shot benchmarks, not a standing ledger; no attribution to real speakers; no say–do for the agent's own work |
| **Outlet raters (NewsGuard, Ad Fontes, Ground News)** | outlet-level judgments; coverage-gap views | rubric judgments by raters, not track records of claims; one number per outlet |

**What none of them captures, and what Choir is:**
- **Prospective.** One ledger records natural public utterances by
  people, organizations, outlets and AI agents, Choir's own included, at
  the time they are made, as frozen, attributed and checkable
  commitments.
- **Settled.** A shared ontology links those utterances to the later
  evidence and the actors' own actions.
- **Plural.** Several attributed scorers settle them, and the scorers are
  scored in turn.
- **Analysed.** Conditional track records come out of it: accuracy,
  calibration, and say–do alignment.
- **Open, revisable and reflexive.** The same ledger that scores a
  ministry scores the paper and its agents.
- **Each neighbour has one or two of these pieces.** Silver has track
  records, Tetlock calibration, markets resolution criteria, Palantir the
  ontology, promise trackers say–do, GDELT scale, and Wikidata identity.
  None joins prospective attribution to settlement to say–do across all
  actors including itself.
- **The join is what prospective self-alignment means**, applied to the
  world as much as to the AI.

## 11. Alternatives considered

1. **No middle layer; embeddings only.** Cheap, schema-free and "bitter
   lesson". Rejected as the authority:
   - there is no attribution, stance or time, so nothing to settle or
     score;
   - it is not auditable on the tape;
   - vectors are model-version-bound and must be re-indexed.
   Kept as a derived index (§9.4).
2. **RDF / Wikidata-style triples with a closed property vocabulary.**
   Standard, and expressive for identity. Rejected for content:
   - news assertions do not fit a closed property list;
   - attribution needs reification, which is clumsy in triples.
   Adopted: QIDs, and the statement-plus-qualifiers-plus-references idea
   (claim plus attributes plus a span).
3. **Event-centric, GDELT and CAMEO style.** Excellent for counting.
   Rejected:
   - the closed taxonomy;
   - an event record per article (duplication);
   - no claims.
   Adopted: events as clusters with actor and place roles, and GDELT as a
   possible free input of entity and place hints.
4. **Claim-centric, ClaimReview style.** It conflates the claim with the
   verdict and is sparse by design. Rejected as the core and kept as an
   export format.
5. **Status quo: captures labelled to stories (L2a).** It works for
   routing, but:
   - it cannot tell topic from event except by a model's say-so on one
     label;
   - it cannot count support per assertion;
   - it cannot feed reputation.
   Kept as the shadow baseline until E3.
6. **Let agents write whatever objects they like** (SW design §2).
   Unqueryable across computers, and no track record survives a schema
   per agent. Adopted instead: a fixed identity, anchoring and settlement
   schema with free-text content.
7. **Claims as `commitment_record` rows with emitter kind `source`** (app
   design §4.1). Rejected for volume and for authority:
   - thousands of claims a day would flood management's supervision
     ledger;
   - commitment records are delivery-shaped.
   Kept: the record schema, through the shared settlement type, and
   edition Precommits for Choir's own world forecasts.
8. **Run extraction as research desk work.** Rejected as the default:
   - it is serial, model-turn priced, and selective about what it reads;
   - the owner moved support judgment *off* desk turns for the same
     reason (`docs/archive/reports/support-judgement-to-jev-design-2026-10-14.md`).
   Research still contributes claims from its Reports and owns relation
   judgments that need evidence found first.
9. **Pairwise claim–claim edges without propositions.** This is
   O(n²) per event. Clustering to propositions gives "how many
   independent outlets affirm Y" directly. Rejected.

## 12. Extraction pipeline, authority, slices and acceptance

### 12.1 Pipeline

```text
poll → captures (Store B) → arrival → [labels, L2a] → extract job (system:extract)
  1. diff blocks against the URL's previous capture; changed blocks only
  2. one call to a cheap generative model (default deepseek-v4.1-flash or gpt-6-luna):
     ≤12 claims {speaker mention, quote, stance, modality, readings[1..4]{gloss, weight,
     context_needed}, quantity?, deadline?, expressed confidence (U3), attribution basis +
     p_attr (U1), p_extract (U4)} + mentions {surface, type}
  3. quotematch every quote against its block; drop non-matches (counted)
  4. entity linking: alias index (Wikidata labels, cached) → Jev choice among ≤8 candidates or new
  5. event resolution: shortlist (recent, shared entities, later embeddings) → Jev choice or new
  6. proposition resolution within the event: Jev choice among the event's propositions or new;
     relation question (refines/updates/contradicts) only against the chosen neighbour
  7. write extraction + one judgment batch through corpusd; record cost on /api/costs
```

**Plane and authority.**
- The job runs **inside the computer** (the paper is platform tier), as
  a runtime system act `system:extract`, like `system:support`.
- It is not a host daemon, which is the sourcecycled lesson, and not a
  desk turn, which is the support-judge lesson.

**Restart rule.**
- The job is in-memory, bounded by a semaphore like `supportJudgeConcurrency`,
  and holds no durable obligation.
- On a crash, captures in flight stay **unextracted**, which is a visible
  state.
- There is no boot sweep, per CLAUDE.md's crash rule.
- A later live act that needs the capture (research cites it, or a story
  is fed it) extracts it on demand, because that is new work. A backfill
  is an explicit owner act.

**Labels never filter, and arrival never waits on extraction** (panel
repair P-13).
- An unextracted or claim-capped capture still reaches management with
  its arrival and label.
- After E3, event resolution for a capture with no extraction runs on
  its title and summary, the same shortlist with fewer features, so a
  crash never orphans a capture from routing.
- Management's disposition of an unextracted capture is a live act on
  it, so it triggers extraction then. That is new work, not a resumed
  job.
- E1's two-minute target counts only captures that arrived while no
  crash intervened. Crash-interrupted captures are counted separately,
  with their fate.

**Cost per capture**, from `llmcost/pricing.go`:
- Extraction: about 2,500 input and 700 to 1,000 output tokens (the
  readings and confidences add output).
  - deepseek-v4.1-flash: about $0.0016 to $0.002.
  - gpt-6-luna: about $0.0006 to $0.00075.
- Jev resolution: about 15 questions of about 400 tokens each, about
  $0.0003.
- Total: about $0.001 to $0.0025 per capture, or **about $2 to $4 a day**
  at 1,700 captures a day, before re-capture reuse.
- These are estimates, measured in E1.
- The cost is tracked, never capped (ruling 1).

**Multilingual.**
- Quotes stay in the original language, and every claim gets an English
  gloss.
- Entity labels come from Wikidata in every language.
- Jev reads across languages (26/26 probe).
- Script and language travel on the extraction.
- RSS summary-only captures yield few claims. The extraction records
  `body_kind`, and the deepen loop fetches full text for important
  events.

### 12.2 Who writes what

| Writer | Writes | Never |
|---|---|---|
| `system:extract` (runtime, on the computer) | extractions | judgments other than the deterministic ones (`grounded_in`, `speaker`, `revises`) |
| `system:resolve` (Jev, harness-run) | resolution judgments; high-confidence ones settle routing (O3) | stories |
| management | event-level dispositions (feed, decline, open) as judgments; merges and splits; `covers` | document text |
| research | claims from Reports (speaker = research, an agent); relation judgments needing evidence; `answerer` settlements | world effects |
| Texture | revisions that cite claims; Choir's own sentences | ledger judgments: Texture writes documents, not the graph |
| scorers (count, Jev, panel, human) | settlements | edits to claims |
| owner | any judgment or settlement, which governs | — |

Texture stays the sole writer of canonical document revisions. The
ledger is the "artifact/provenance graph" row of the ontology's ledger
split ("claims, citations, source anchors, verifier results: evidence
projection; cannot append, accept, or materialize an event",
`docs/computer-ontology.md`). It is evidence, never canonical computer
events.

### 12.3 Slices, smallest first

Each slice has a problem-first doc, a per-computer enable (off by
default), a tripwire, and landing on staging.

| # | Slice | Class | Acceptance (measured, bar fixed before the run) |
|---|---|---|---|
| E0 | **Offline study**, outside the runtime, on a **frozen, blind evaluation set**: the L2a fixture (27 captures, 8 events) as a diagnostic, plus a held-out labelled sample of the after-window's 425 captures with re-captures removed. Event, sub-event, topic, duplicate and false-merge labels are made by the registered judge (gemini-3.8-flash, two shuffled calls) before any prompt tuning, and a tuning split is kept apart | green/yellow | **Routing:** event-cluster precision and recall against the event labels (not against L2a's story labels, which target a different object); L2a's labels are scored on the same set as the control, using the label rerun with the Jev peer pin fixed; the 10 topic-feed wrong merges and causes C and D separated by `part_of`; duplicate-proposition rate. **Extraction validity**, on a fixed sample by the judge: speaker correct ≥ 0.9, modality correct ≥ 0.85, readings non-vacuous; quote matches reported as **attempted versus retained**, with retained ≥ 95%; entity-link precision ≥ 0.9. **Settlement feasibility:** share of claims passing the specificity check; share of assertions that a count rule could settle (primary record, or two origins); commitments checkable per 100 captures. Cost per capture. **Exit:** if routing beats the control, take the merging-first order; if routing fails but settlement feasibility passes, take the settlement-first order (E6a, E6, then E1 for settlement); if both fail, stop, and the layer is recorded as falsified for now (CE-1) |
| E6a | **Choir's own desks' say–do eval** (moved up by the panel: it needs no extraction, no entities and no Store B; it is the tape, P3, P4 and score records). P3 expectations, Cast hypotheses and capability self-forecasts are settled from the trace; per-desk and per-model box scores are kept off the acting context; 10% of declines are counterfactually sampled | orange | calibration (Murphy decomposition) and say–do per desk and model, with n and intervals; hedge and decline rates shown; scorers blind; runs **alongside E0 and E1** |
| E1 | **Claims and entities on the paper, shadow.** `intake.extract`; extraction plus entity judgments written; no consumer reads them. **Entry conditions:** the Jev peer-pin fix (2cc8ab8fa on main) proven on the paper (label coverage near 100%); the new kinds registered and written only through typed corpusd routes with validation (H-OG1 repaired *for these kinds*); the volume budget and tripwire in place (§5.1) | red (gateway calls, new corpusd write route, poll path neighbour) | ≥ 95% of new captures that arrived with no crash in between extracted within 2 min; retained quote matches ≥ 95%; claim cap respected; re-captures carry ≥ 80% of unchanged blocks; Store B growth within budget; cost reported; no publication regression (L2a tripwire) |
| E2 | **Events in shadow.** Event and proposition judgments beside L2a labels (the transitional dual answer, with a **dated** deletion clock: if E3 has not landed 21 days after E2 starts, the shadow writer turns off) | red | on two 6-h windows with the fixed scorer: event-keyed merge recall ≥ label-keyed, false merges lower, topic-versus-event errors named; duplicate-proposition rate within the E0 bar |
| E3 | **Route by event.** Jev's arrival question targets events; `covers`; the capture_label story pointer retired | red | merge recall ≥ 0.85, false merges ≤ 1 per window; captures per new story up from the baseline of 2.35 (f4ef4436a); new stories per hour sublinear in captures per hour |
| E4 | **Texture writes from the dossier and cites claims** | red (Texture write surface) | ≥ 80% of new citations name a claim; the "what changed" line names a new proposition in ≥ 80% of revisions after a feed; the support pass rate holds or rises |
| E5 | **Corpus first, and search results into extraction.** `SearchClaims`, `EntityTimeline`; search-result pointers first; snippet extraction after D7 | orange (red for the gateway calls) | the corpus share of research evidence rises; web searches per story fall; time to first report holds; no snippet text retained |
| E6 | **Outlet box scores, internal, settled by count** (assertions; origin-based independence) | orange | per-outlet settled counts with intervals, `confirmed_primary` and `corroborated` shown separately; Jev shadow-scored against count on 25%; a **back-test** once Store B's earliest captures (from about 2026-10-11) are 30 days old |
| E7 | **Public commitments and say–do**: organizations first (vertical zero, tech: dated vendor promises), then people acting in public roles. **Gate:** D1 and D6 ratified as amendments in the metamission | red where published | ≥ 30 checkable commitments settled; panel agreement with Jev ≥ 0.8; per-actor coverage shown; every published named-actor settlement meets the §7.3 excellence bar (span verified, readings shown, reply path) |
| E8 | **Publish the public part** into the Autopaper object graph; the front page uses independent outlets per event and depth | red (publication) | public reads never touch the paper; 100% of published quotes within the rights rule |

**E0 decides the order, and can stop the layer.** E0 measures routing
and settlement feasibility together, so it can fail the layer (panel
repair P-1). E6a runs beside it from the start, because the owner made
Choir's own agents the first population and their loop is already
complete.

### 12.4 Ceremony for the red slices (carried into each slice's goal file)

- **Conjecture delta.** CE-1: a small fixed schema for identity,
  attribution, anchoring and settlement, with free-text content, beats
  both labels-only and agent-defined objects on merge recall and makes
  track records possible. Falsified if E0 or E2 shows no merge gain *and*
  E6 shows settlement by count covers under 20% of assertions.
- **Protected surfaces.** gateway and provider calls (extraction and
  Jev); a new corpusd write route; the poll path's neighbourhood; the
  Texture citation grammar (E4); publication (E8).
- **Admissible evidence.** Deployed proof on a minted test computer, then
  on the paper, measured by scripts against labels outside the repo.
- **Rollback.** Turn off the per-computer enable. Ledger objects are
  inert evidence, so leaving them costs nothing. Then revert.
- **Heresy delta.**
  - Discovered: H-OG1, H-OG2, H-OG3.
  - Introduced, to be named per slice: the transitional dual answer of
    capture_label and event judgments, with a dated deletion clock (21
    days after E2 starts, or when E3 lands).
  - Repaired: none.

## 13. Open decisions for the owner

> **Ratified (owner, 2026-10-20; owner notes §27 item 15, the answer to
> the decisions digest, item fifteen):** D1–D5 and D7–D13 are accepted as
> recommended, text unchanged below. D6 was already ruled (owner,
> 2026-10-14 evening) and stays as ruled. Open sub-items inside the
> accepted decisions keep their stated form: D7's per-provider terms check
> before snippet extraction in E5, D9's later named editor role, and D11's
> model choice by E0's measured quality per dollar.
>
> Follow-ons recorded on 2026-10-20 (the digest asked for "accept the rest
> as written"; the two items below were not separately worded in §27, so
> they are recorded as the inferred recommendations, now accepted, and are
> reversible):
>
> - **E0 finding F1** (scheduled captures are 95% feed summaries; full text
>   arrives through research's targeted reads, which §4.1 never scores;
>   [claims-e0-e6a-results-2026-10-14.md](../archive/reports/claims-e0-e6a-results-2026-10-14.md)
>   §6 and §7 name two options and say "either needs a ruling alongside
>   D1", without a stated recommendation). Status: D1 is ratified; the
>   option is taken as **count a research read of a scheduled item's own
>   URL as scheduled** (no new fetch cost), with the deepen fetch of
>   scheduled items as the fallback if E1 shows the coverage too thin.
>   Carried as a named residual until E1 measures it.
> - **GDPR note on D6** (scoring natural persons is profiling under the
>   GDPR, separate from defamation; rights of access and objection and the
>   reach of the journalism exemption). Status: D6's scope is settled; the
>   GDPR sub-question is recorded as an **accepted risk with the reply path
>   serving as the objection channel**, as D6's "To confirm" recommended, and
>   a lawyer's read remains owed later (the same posture as the AI-written
>   disclosure, §27 item 14). It gates nothing before E7.
> - The gate in §12 stands: D1 and D6 still have to be written into the
>   metamission as amendments before E7. Ratification is given; the
>   amendment text is not yet written.

1. **D1. The claim is the speaker's precommitment, recorded by Choir.**
   Amend the metamission's "sources never precommit" to "sources never
   emit records; Choir records their claims as attributed
   precommitments; only scheduled, timely claims enter a track record".
   Recommended: yes. Ratifying it in the metamission gates E7.
2. **D2. Events are the merge key, and a story is a view over an event
   cluster.** This replaces "a story is the set of captures fed into it".
   Recommended: yes, after E0 and E2 confirm it.
3. **D3. A fixed schema despite "existing types only".** The new kinds:
   - `choir.capture_extraction`;
   - `choir.entity`;
   - `choir.world_event`, with `scale: topic` for topics;
   - `choir.proposition`;
   - `choir.graph_judgment`;
   - `choir.search_result` (with D7).

   Settlements reuse the score record. Recommended: yes. Each kind exists
   because something consumes it.
4. **D4. Shared or per computer.** The paper's ledger and judgments go in
   Store B as public; private computers keep overlays. Recommended: yes.
5. **D5. Wikidata as the identity source for entities.** Use the API
   with a cache to start (rate-limited, with a User-Agent); a local dump
   comes later if volume needs it. Wikidata is CC0. Recommended: the API
   first.
6. **D6. Whose track records are computed and shown.** The owner's
   ruling covers anyone who "act[s] in the public sphere", and the legal
   risk is accepted. Recommended:
   - outlets, organizations, institutions, APIs, Choir's own agents and
     models, and people acting in a public role;
   - not private individuals (for example a witness quoted once, or a
     social-media user in a discourse sample).

   - `actor_scope` on every entity defaults to `private` and must be
     set positively before any box score exists.

   To confirm:
   - that this narrows the metamission's exclusion of "individual-level
     tracking of people in public-discourse sources" rather than lifting
     it. Ratifying it gates E7;
   - separately from defamation, which the owner's ruling covers, scoring
     natural persons is profiling under the GDPR (rights of access and
     objection; the reach of the journalism exemption). Is that also an
     accepted risk, with the reply path serving as the objection
     channel?
7. **D7. Search results.** Pointers (URL, title, query, run, provider,
   rank, time) are kept. Snippets are used transiently for extraction and
   not retained. Extracted claims are kept with attribution to the source
   page (owner, about 15:05 UTC). Open: each provider's terms on building
   derived data from results, checked before snippet extraction is turned
   on in E5. Pointers can ship first.
8. **D8. Estimative-word mapping and settlement horizons.** Recommended:
   - IPCC and Kent anchors, versioned;
   - a 30-day horizon for assertions;
   - the stated deadline, or 1 year, for commitments without a date.
9. **D9. Human review.** This is a quality audit, not a gate (owner
   ruling). Recommended:
   - the agentic panel audits disagreements and a random sample;
   - the owner samples when he wants to;
   - replies from scored actors re-open settlement automatically.

   Open: whether a named editor role exists later.
10. **D10. Choir's own public box score.** Publish the paper's own record
    (support rate, corrections, forecasts) from E6 on. Recommended: yes,
    as the credibility argument.
11. **D11. Extraction model.** deepseek-v4.1-flash or gpt-6-luna as
    default, chosen by E0's measured quality per dollar. Spend is
    tracked, not capped.
12. **D12. The settlement action-class vocabulary.** A short closed list
    (release, ship, sign or enact, appoint or resign, publish a figure,
    hold an event, start or stop an operation) used only for counting
    settlements, never to constrain what claims say. Recommended: yes.
    This is the one closed vocabulary in the design.
13. **D13. The scoring defaults:**
    - the 48-hour freshness bound for a claim to count as a
      precommitment;
    - an unhedged "will" maps to 0.85;
    - the unscorable thresholds (attribution 0.6, no reading at or above
      0.25);
    - 10% counterfactual sampling of declines.

    All are versioned, and scores recompute under new values.

## 14. Conjecture and heresy deltas (this note)

- Conjecture delta (proposed): CE-1 (§12.4). CE-2: say–do alignment can
  be measured from the ledger for AI agents completely and for
  institutions up to coverage. It is falsified if the specificity filter
  leaves fewer than 1 in 5 commitments checkable on vertical zero.
- Heresy delta: discovered H-OG1, H-OG2 and H-OG3. Introduced none,
  since this is a note. Repaired none. Also discovered, by the panel: the
  draft's own count rule would have scored self-sourced figures as
  confirmed (repaired in this note, P-2).
- Mutation class: green (docs only).

## 15. Panel adjudication (convergent, round 1)

**Panel and subject.**
- Runner: `agentic-consensus-runner.sh --mode convergent --keep-going`,
  with members claude (opus), omp-gemini38, omp-gpt6-luna,
  omp-deepseek-v41-flash and omp-glm53-flash. OMP members had no tools;
  devin and codex were excluded.
- Subject: draft d033b48a5, embedded in the prompt (prompt sha256
  prefix 001fb4b0c2ec2cf0).
- The prompt carried the owner's framing up to 14:55 UTC. The 15:05
  search-results direction arrived after the run, and §9.4 takes it in
  directly.
- Raw outputs are session diagnostics outside the repo (scratchpad
  `panel-cee/out`).

**Verdicts.**

| Member | Verdict | Notes |
|---|---|---|
| claude (opus) | accept with repairs | 10 blockers; E0 may start once its gate is fixed |
| gemini-3.8-flash | accept with repairs | 4 blockers |
| glm-5.3-flash | accept with repairs | 4 blockers |
| gpt-6-luna | send back | approve E0 alone after revision |
| deepseek-v4.1-flash | **no valid output** | it emitted a raw tool-call block with tools disabled, and no verdict. Recorded as a failed seat, not as a vote |

**Consensus, raised by two or more members, adopted:**
- **P-1. E0 could not fail the layer** (claude, glm, luna). E0 now has a
  frozen blind evaluation set, labels for event, sub-event, topic,
  duplicate and false merge, the control scored on the same set,
  extraction-validity bars (speaker, modality, readings), quote matches
  reported as attempted versus retained, settlement-feasibility bars,
  and a three-way exit that can stop the layer (§12.3).
- **P-2. Count settlement was circular on relays and syndication**
  (claude, glm, gemini, luna). Independence is now counted by origin
  (`originates_from`). Self-sourced support stays unresolved, and
  `confirmed_primary` is split from `corroborated` (§6).
- **P-3. Propositions scoped to one event break commitments and
  forecasts** (claude, glm, gemini). Commitments, forecasts and intents
  now resolve by speaker and subject across events. The scoring unit is
  (speaker, proposition, reading) at its first scheduled utterance, and
  a duplicate-proposition bar is set (§5.2, §6).
- **P-5. Scoring math** (gemini, claude, luna):
  - interpretation was counted twice, and is now removed from the
    weight;
  - the per-reading expected Brier is written out;
  - the interval uses n_eff pseudo-counts or a bootstrap;
  - reading weights are frozen at T_commit by a blind judge;
  - "unscorable" applies to public actors only, while Choir's agents keep
    P4's "unresolvable counts as wrong". Verified against
    `docs/precommitment-records.md`
    §4.5;
  - the hedging argument is corrected: a hedge is a safe harbour, so it
    is countered by showing the hedge share, resolution and sharpness
    (§6a, §7.0).
- **P-6. Projection semantics** (luna, claude). Assertions inside a
  batch judgment are individually addressable and supersedable.
  Precedence is by relation type, and management governs routing, not
  world evidence. Merge cycles are resolved union-find style (§5.3).
- **P-8. Commit time and selection** (claude, luna). There are three
  times. Claims are tagged `scheduled` or `targeted`, and only scheduled,
  timely claims enter a track record (§4.1).
- **P-11. Store B volume** (gemini, claude, luna, glm). There is now a
  budget table, append-only rows, aliases in a side table, projection
  tables out of history, and a tripwire (§5.1).
  - Verified: Store B's committer is already debounced
    (`internal/platform/store.go:405-410`, after the 218 GB history
    incident). Gemini's "thousands of Dolt commits a day" is therefore
    partly wrong. Its point about growth stands and is adopted.
- **E6a moved up** (claude, luna). It runs beside E0 and E1, because it
  needs only the tape, P3, P4 and score records.
- **D1 and D6 must be ratified as metamission amendments before E7**
  (gemini, luna). Adopted as a gate. `actor_scope` defaults to `private`
  (gemini).

**Single-member findings, verified and adopted:**
- **P-4. Relay fidelity had no claim to score** (luna). The reporting
  claim is now explicit, as an `attribution` modality (§4.1).
- **P-7. Single state authority** (claude).
  - Verified: dispositions are written on the computer
    (`CommitInboxRecords`, `choir.capture_disposition`,
    `internal/agentcore/intake_overrides.go`).
  - Now the computer's records are the authority and the Store B
    judgment is a derived export.
  - Support verdicts are used by reference.
  - The settlement home is a stated function of speaker class and
    target.
  - Judgments carry the computer's event head, for restore (§5.3, §5.4).
- **P-6b. Declines can't be settled** (claude). 10% of declines are
  counterfactually sampled; the rest are counted and never imputed
  (§7.0).
- **P-9. Free-text event types cannot be counted** (glm). A small closed
  settlement action-class vocabulary is added, used only for counting
  (§6, D12).
- **P-10. Unhedged assertions had no probability, and modalities were
  mixed** (glm, luna). Scoring is now per modality: assertions are scored
  by accuracy per stance class, with no Brier score, and forecasts carry
  a versioned default for an unhedged "will" (§6, D13).
- **P-12. Block-level reuse conflicted with per-capture claim ids**
  (luna). An extraction now carries a manifest of carried claims (§5.1).
- **P-13. A crash could orphan routing after E3** (gemini). Arrival
  never waits on extraction, event resolution falls back to title and
  summary, and a disposition triggers extraction as new work (§12.1).
- **E1 entry conditions** (claude): the Jev peer-pin fix proven on the
  paper, and H-OG1 repaired for the new kinds (§12.3).
- **Topic is not an event** (luna, glm). A topic node is a
  `choir.world_event` with `scale: topic` and a depth cap (§4.2).
- **Neighbours added** (claude): Community Notes, ClaimBuster and
  FEVER-style automated claim checking, and ForecastBench (§10).
- **Risks added to §7.3:** media selection of actions with per-actor
  coverage (gemini); GDPR profiling as a separate question (claude); the
  false-positive rate of "silent edit" signals (glm); market-reference
  skew (glm).
- **The dated deletion clock** for the shadow dual answer (claude):
  21 days after E2 starts (§12.3).

**Not adopted, with reasons:**
- *Freeze the full schema before E0* (implied by luna's "do not build the
  full object schema"). Agreed in the opposite direction: E0 writes no
  Store B state and builds no kinds. Nothing to change.
- *A blocking human-review gate for named actors.* The owner ruled at
  about 14:55 UTC that legal risk is accepted. Review is a quality audit
  (D9), and the excellence bar (§7.3) is the publication condition.
- *Gemini's "thousands of Dolt commits a day"*, as a factual claim.
  Contradicted by the code (debounced committer), as noted above.
- *A back-test on weeks-old captures for E0* (claude). Store B restarted
  from zero on 2026-10-09, and the captures since about 2026-10-11 are
  days old. The back-test moves to E6, once those captures are 30 days
  old. The June dump is a possible earlier source, unverified.

**Residual dissent.**
- Luna's send-back rested on blockers that this revision addresses.
  Whether they are sufficient is for a round-2 review of this revision,
  which is recommended before E1 (not before E0).
- No member disputed the three-layer ontology, attribution versus
  truth, judgments with a projection, or the thesis.

**Heresy delta from the panel:** discovered: the draft's own count rule
would have confirmed self-sourced figures (repaired here, P-2).
