# Memo: Autopaper and the World Wire as Generalization Codesign

**Date:** 2026-08-09  
**Status:** architecture projection; not promoted doctrine or an Autopaper reactivation  
**Mutation class:** green (documentation only)  
**Scope:** project persistent RLM actors, multisupervision, capsules, and evals
into a continuously updated, many-source public reporting application

## Authority Note

Autopaper remains tabled and has no active Definition. This memo does not
authorize source, publication, World Wire, provider, VM, or runtime work. It
projects a future architecture so that, if the owner reopens Autopaper after the
automatic computer is accepted, the application tests the general substrate
rather than recreating a bespoke news pipeline.

It inherits the ordering in [The Automatic Computer — Choir Vision](choir-vision.md):
supervised self-development first, the World Wire downstream.

## Thesis

Autopaper is valuable before activation because it is a **generalization
codesign partner** for the automatic computer.

Self-development asks whether Choir can supervise a persistent computer acting
on itself. Autopaper asks whether the same harness can supervise an entirely
different cognitive organization:

- input arrives continuously from more sources than any model can read at once;
- observations are time-sensitive, duplicated, contradictory, adversarial, and
  frequently revised;
- attention must be allocated before synthesis can begin;
- many investigations remain live simultaneously;
- publication creates public and sometimes effectively irreversible exposure;
- correction matters more than one-shot completion;
- truth is not directly available—the Wire represents the world *as reported*.

If supporting this workload requires a second scheduler, news-specific agent
loop, publication authority, transcript bus, or state ontology, the harness has
not generalized. If it requires only new typed source/publication modules,
artifact programs, capability profiles, supervision policy, and renderers on
the same durable actor/RLM/Texture substrate, that is evidence that the
automatic computer is becoming general.

```text
automatic computer substrate
  + source observation modules
  + publication artifact program
  + editorial supervision protocol
  + public projection transaction
  = Autopaper

many Autopaper perspectives
  + shared provenance and claim topology
  + contested public projections
  = World Wire
```

The codesign is bidirectional. Autoputer authority prevents Autopaper from
shortcutting the durable computer. Autopaper's scale and temporal pressure
force the autoputer to become a real general-purpose cognitive environment
rather than a coding-agent harness with broader names.

## The Real Object

The shallow object is an agent that periodically writes articles. The real
object is a continuously maintained public epistemic program.

The primary durable semantic object should not be the article. It should be a
time-indexed graph of:

- source observations and exact captured versions;
- attributed claims—what a particular source reported, at a particular time;
- corroboration, derivation, contradiction, correction, and retraction edges;
- entities, events, places, and continuing story threads;
- standing questions and uncertainty;
- investigations, evidence gaps, and editorial obligations;
- published claims and the public versions that carried them;
- later evidence that confirms, narrows, disputes, or supersedes them.

An article is a supervised Texture projection over a bounded subgraph. An
edition is an immutable allocation-of-attention snapshot. An Autopaper is the
durable artifact program that decides what to observe, investigate, synthesize,
supervise, publish, revisit, and render. The World Wire is the public,
provenance-bearing network of those contested projections—not a god's-eye fact
database.

This recovers one useful insight from the superseded Texture/transclusion
vision without restoring its product ontology: editorial style is not merely a
renderer. An editorial constitution changes which sources are sought, whose
experience is centered, which claims receive skepticism, what remains a live
question, and how much evidence is required. Style is therefore an attention
and supervision policy as well as a prose policy.

## Generalization Map

| Automatic-computer primitive | Self-development instance | Autopaper / World Wire instance |
| --- | --- | --- |
| Persistent computer | User computer improving its working state | Platform or user computer maintaining a publication program |
| Durable trajectory | A conjecture about the computer | A continuing event, investigation, or editorial question |
| Observation | Code, tests, Trace, deployment behavior | Captured source version, API observation, public record, later update |
| Standing state | Beliefs, blockers, candidate evidence | Claims, contradictions, unknowns, coverage and correction obligations |
| Texture | Owner-readable semantic control state | Article, story dossier, editorial constitution, or edition state |
| RLM activation | Plan, delegate, inspect artifacts, propose change | Allocate attention, research, reconcile updates, draft and verify |
| Capsule | Isolated source/test effects | Isolated research, extraction, synthesis, rendering, and replay work |
| Multisupervision | Review a high-consequence computer change | Editorial, source, safety, and public-risk consensus |
| Typed effect proposal | Frozen self-development bundle | Frozen publication or correction transaction |
| Acceptance/commit | Computer event and accepted effective head | Publication reducer accepts an exact Texture/provenance head |
| Roll-forward correction | Superseding computer event | New public version with visible correction/supersession lineage |
| Eval case | Frozen activation and computer state | Frozen report-time world, source corpus, question state, and publication policy |

The instances are deliberately unlike each other. Passing both is stronger
evidence than making one coding workflow increasingly elaborate.

## Proposed Durable Objects

Names here are design placeholders, not registry authority.

### Publication program

An Autopaper should be a versioned artifact program owned by a computer:

```go
type PublicationProgram struct {
    ProgramID             PublicationProgramID
    OwnerComputerID       ComputerID
    ProgramVersion        ArtifactRef
    EditorialConstitution ArtifactRef
    SourcePolicy          PolicyRef
    AttentionPolicy       PolicyRef
    ModelPolicy           PolicyRef
    SupervisionPolicy     PolicyRef
    PublicationPolicy     PolicyRef
    SchedulePolicy        PolicyRef
    RendererRef           ArtifactRef
    EffectiveComputerHead EventHead
}
```

The schedule is a wake policy, not the semantic loop. Source changes,
contradictions, unanswered questions, corrections, deadlines, and owner
instructions can also wake the program. The program never becomes a new daemon
or bypasses the persistent computer.

### Reported claim

A claim record must preserve the distinction between report and reality:

```go
type ReportedClaim struct {
    ClaimID           ClaimID
    PropositionRef    ArtifactRef
    AssertingSource   SourceVersionRef
    ObservedAt        time.Time
    ValidTime         TimeRange
    Attribution       Attribution
    SupportRefs       []EvidenceRef
    RelationRefs      []ClaimRelationRef
    PublicationRefs   []PublicationVersionRef
    Status            ClaimStatus
}
```

`ClaimStatus` describes Choir's handling—unreviewed, attributed, corroborated,
contested, corrected, retracted—not metaphysical truth. Model output cannot
promote itself into source evidence.

### Editorial decision case

Before a consequential synthesis or publication round, the host freezes:

- the claim/evidence subgraph and observation cutoff;
- actor wake and memory cursors;
- active questions, correction obligations, and previous coverage;
- editorial constitution and publication-risk class;
- source, model, capability, and privacy policy;
- eligible supervisory seats and independence requirements;
- the exact Texture and provenance heads under consideration.

This object is simultaneously the input to production multisupervision and the
spine for later retrospective evals.

## Continuous Operation Across Four Timescales

The old cycle model encouraged a serial pipeline to re-earn end-to-end success
on every poll. The RLM computer should separate timescales.

### Observation time: seconds to minutes

Host source connectors capture immutable source versions, record retrieval
provenance, normalize safe metadata, and append graph relationships. Cheap,
deterministic reducers deduplicate exact items, identify source lineage, and
route updates. They do not decide what is true, write articles, or activate one
model per item.

### Attention time: minutes to hours

Updates change standing questions and work obligations. Clustering and triage
may link an observation to an existing story, open a new investigation, flag a
correction, or suppress a duplicate. RLM supervisors decide what deserves
scarce cognition. Backpressure is explicit: some observations wait, aggregate,
expire, or remain searchable without becoming work.

### Editorial time: hours to days

Research and CoSuper actors operate over durable artifact collections much
larger than their context windows. They use Go orchestration to map extraction,
compare sources, track disagreement, pursue missing evidence, and construct
bounded views. Texture maintains the evolving article or dossier while work is
still open.

### Publication and correction time: durable history

A publication transaction binds one exact Texture version, source/transclusion
manifest, supervisory decision receipt, risk class, renderer, route, and
idempotency identity. Publication creates a public projection in the World Wire
store; it does not transfer authorship to corpusd or mutate the computer's
private canonical document.

Later evidence creates a new version or correction edge. Past public bytes and
the decision that exposed them remain inspectable. A correction is an ordinary
forward write, not silent replacement.

### Impact propagation: update only what changed

The essential transition is not `source cycle -> regenerate paper`. It is:

```text
new immutable source version
  -> source-lineage and claim-relation reducers
  -> impacted claim/question/publication index
  -> open or revise exact obligations
  -> wake affected trajectories, Textures, or supervision rounds
```

An update may alter one attribution, invalidate a paragraph, reopen a dormant
investigation, change an edition candidate, or require no model work at all.
Materiality policy and dependency edges determine the radius. This primitive
generalizes beyond reporting: dependency releases, security advisories, API
changes, test failures, and owner corrections are all new observations that
should wake the standing beliefs they bear on rather than restart the computer.

## The Autopaper Actor Organization

Autopaper should not restore fixed `processor` and `reconciler` harness loops as
the fundamental ontology. It uses uniform persistent actors whose assignments,
modules, and protocol seats vary.

```text
source observations
  -> attention/work reducer
  -> story or investigation trajectory
       -> Research capability actors
       -> orchestration-focused CoSupers
       -> canonical Texture writer
       -> Super protocol round
  -> typed publication/correction proposal
  -> trusted publication reducer
  -> World Wire projection
```

Useful capability compositions include:

- source lineage and extraction;
- event/claim linking and contradiction search;
- domain research and public-record retrieval;
- adversarial verification and missing-source search;
- synthesis and citation coverage;
- privacy, defamation, safety, and public-harm review;
- edition-level attention, balance, and continuity;
- rendering and accessibility.

These may be local model calls inside one RLM activation, durable actors, or
Super protocol seats depending on whether they need identity, memory,
communication, restart continuity, or a vote. They should not automatically
become new global role classes.

## Multisupervision as the Newsroom

The Super protocol becomes a programmable newsroom constitution. Multiple
persistent actors occupy attributable seats; no singleton `Super` becomes the
editorial sovereign.

Routine rounds can use diverse models to inspect sourcing, synthesis, and
uncertainty. Higher-risk publication classes require stronger policy. Public
exposure may be effectively irreversible even when a correction is possible:
people can read, copy, trade on, or be harmed by the first version. Accusations,
personal information, safety-critical reporting, election calls, and emergency
claims should therefore require all designated independent seats to approve,
no unresolved safety dissent, and any separately required human approval.

Editorial independence has two dimensions:

1. **Cognitive independence:** different model families, provider paths,
   prompts, context selectors, and supervisor actors.
2. **Evidentiary independence:** different primary sources, ownership chains,
   witnesses, collection mechanisms, jurisdictions, and incentives.

The second is often more important. Ten outlets repeating one wire report are
one evidence lineage. Three model families reading the same poisoned summary
are one compromised observation path. The host should calculate and expose
independence claims rather than equating vote count with plurality.

A supervision round emits proposals, objections, abstentions, missing-evidence
requests, and a decision receipt. Trusted Go validates membership, quorum,
independence, provenance, risk policy, and the exact publication head before a
publication API can accept the receipt. Consensus is evidence and a required
gate; it is not publication authority.

## RLM Context Virtualization at World Scale

This workload is a strong test of the RLM thesis because the complete input is
necessarily outside every foreground model context.

Model-authored Go should orchestrate typed modules such as:

```go
sources.Query(ctx, question)
sources.Open(ctx, sourceVersion, selector)
claims.Related(ctx, entityOrEvent)
claims.Contradictions(ctx, claimID)
artifacts.Map(ctx, refs, extract)
models.Parallel(ctx, requests...)
coagents.Assign(ctx, work)
supervision.Propose(ctx, decisionCase, proposal)
publication.Propose(ctx, textureHead, manifest)
```

The activation sees bounded views and refs, not the whole corpus. Full source
captures, extraction outputs, claim relations, and disagreements remain in
content-addressed artifacts. Model calls can run concurrently within one
activation; durable investigations communicate through typed work and message
state. No capsule receives raw credentials, unrestricted network, corpusd
write access, or public publication authority.

This gives Autopaper a concrete generalization test: input scale should increase
artifact graph size and orchestration breadth without increasing the root
model's context in proportion.

## World Wire Authority and Placement

The current two-store boundary remains:

- the platform or user computer's embedded store owns its canonical Texture,
  trajectories, memory, publication staging state, and supervision receipts;
- the corpusd World Wire store owns public/source objects, publication
  versions, routes, citation/provenance objects, and public review/control
  records.

Platform World Wire semantic work runs under the platform computer. Personal
Autopapers, if later authorized, run under their owning user computers. A typed
publication transaction projects selected immutable content and provenance to
corpusd. corpusd validates and stores the public projection but does not decide
what to publish, rewrite private state, or become a third semantic actor.

The existing graph-native publication, publication-version, proposal, policy,
source-entity, transclusion, citation, and provenance surfaces are seeds. New
claim, question, correction, and supervision objects should extend or reuse
that ledger deliberately rather than introduce another news database.

Sourcecycled, if retained, is a narrow observation service. It may fetch,
capture, and submit typed observations. It must not own editorial activation,
article semantics, completion, or publication authority. The durable computer
decides what an observation means.

## Autopaper as a Live Eval Generator

Continuous reporting naturally produces high-value eval cases:

- the same report-time source corpus under different models;
- different context selectors or compaction policies;
- one versus several supervisory seats;
- different source-diversity and editorial-constitution policies;
- different RLM orchestration programs;
- frozen-time replay before and after a later correction becomes known;
- model or provider failure during an active investigation;
- different Autopaper perspectives over the same observation graph.

Production multisupervision yields attributable live evidence: which seat found
a contradiction, requested the decisive source, resisted a false merge,
noticed a correction, or failed with its peers. Those seats are production
inference, not eval shadows. Controlled causal comparison still uses isolated
baseline and challenger trials against a frozen editorial decision case.

Retrospective grading must respect time. A model cannot be penalized for lacking
evidence published after the case cutoff unless the grade is explicitly a
later-outcome measure. Refreshed-world reruns answer a different question and
must not masquerade as historical replay.

Useful measurements form a vector:

- source and citation coverage;
- evidence-lineage diversity rather than raw source count;
- unsupported-claim and attribution error rate;
- false event merge/split rate;
- contradiction and retraction detection;
- correction latency and public correction completeness;
- standing-question closure and appropriate uncertainty retention;
- novelty relative to source text without invented facts;
- coverage latency, cost, and context-virtualization ratio;
- supervisor dissent quality and correlated-failure rate;
- public or owner correction, complaint, and later-evidence outcomes.

Engagement is not an epistemic score. Clicks may be operational telemetry but
must not select the winning truth policy.

## Security Projection

Autopaper makes the threat model more realistic than code alone.

- **Prompt injection:** source bodies are tainted observations, never
  instructions or capabilities.
- **Source poisoning and sybil reporting:** source ownership and derivation
  lineage matter; repeated copies do not create independent corroboration.
- **Temporal attacks:** late edits, deleted pages, changed timestamps, and
  retractions require immutable captures and explicit version relationships.
- **Consensus capture:** model diversity without evidence diversity can produce
  confident coordinated error.
- **Context flooding:** high-volume actors can try to consume attention or evict
  contradictory evidence; budgets and attention policy require auditable
  backpressure.
- **Data and rights leakage:** provider, judge, execution-host, storage, and
  public-projection admission are separate decisions.
- **Public harm:** publication is an external effect. Capsules can draft and
  render, but only the trusted publication path can expose bytes publicly.
- **Correction laundering:** a later correction cannot erase who approved and
  published the earlier version.

Multisupervision supplies defense in depth only across genuinely independent
failure domains. The canonical reducer, capability boundary, and human
constitutional authority remain above it.

## Generalization Codesign Consequences

Autopaper pressure should improve the autoputer substrate in reusable ways:

- incremental wake and backpressure over unbounded observation streams;
- temporal identity and correction-aware evidence graphs;
- artifact-scale context virtualization;
- typed multi-model orchestration and durable delegation;
- multisupervision with real independence accounting;
- public-effect transactions and no-silent-rewrite history;
- live and retrospective evals over frozen contexts;
- readable standing state despite thousands of underlying events.

Autoputer constraints should simplify Autopaper:

- no Autopaper service or separate scheduler;
- no processor/reconciler completion ontology parallel to durable work;
- no VM-as-worker or candidate-computer proliferation;
- no direct source-daemon-to-publication authority;
- no transcript as editorial state;
- no publication inferred from run completion;
- no third semantic store;
- no model or consensus vote as canonical authority.

The strongest generalization signal is deletion: news-specific loops become
ordinary modules and policy on the common RLM/actor/Texture kernel.

## A Future Proof Ladder

This is not an executable plan while Autopaper remains tabled. If later
authorized, a topology-honest proof sequence would be:

### 1. Retrospective newsroom case

Build a frozen, non-private case from existing source captures whose full text
is much larger than the root model context. Include duplicates, independent
corroboration, contradiction, a later correction, and a malicious instruction
inside source text. Run isolated RLM trials with no publication capability.

### 2. Private continuous rehearsal

Feed several timed update waves into one durable platform-computer trajectory.
Prove incremental claim/question updates, bounded model context, progressive
Texture revisions, multisupervisor dissent, restart continuity, and zero public
writes. Do not regenerate the whole publication on each wave.

### 3. Frozen publication proposal

Produce one immutable publication transaction with exact Texture head,
source/transclusion manifest, decision case, supervision receipt, renderer,
risk class, and idempotency identity. Prove every missing or stale component is
rejected and no capsule can publish it.

### 4. Controlled public projection and correction

Only after the automatic computer and publication authority are accepted,
publish one bounded non-sensitive World Wire projection, then ingest a
preplanned contradictory update and issue a visible forward correction.
Prove old and new versions, attribution, approval, route, and public bytes all
join under exact identities.

### 5. Generalization acceptance

Demonstrate that both self-development and Autopaper use the same activation
kernel, actor continuity, Texture semantics, capability broker, Super protocol,
eval case, and trusted transaction boundary. News-specific additions are typed
modules, policies, artifact schemas, and renderers—not a parallel harness.

The stopping condition is not “an article was generated.” It is that a
many-source, frequently updating world can change the machine's standing
questions, produce supervised public synthesis, correct itself visibly, and
survive restart without violating the automatic computer's authorities.

## Cognitive Transform Result

**Current uncertainty or obstacle:** how to use Autopaper as an architectural
forcing function while respecting the settled sequence that forbids premature
Autopaper activation.

**Selected transforms:**

1. **Object transform:** replaces “writing agent” with continuously maintained
   epistemic program; this makes claim and correction state primary.
2. **Scale separation:** separates observation, attention, editorial, and
   publication time; this removes the serial all-or-nothing cycle.
3. **Observer hierarchy:** treats multisupervision as an accountable production
   protocol whose independence and decisions are themselves observable.
4. **Generalization:** asks which primitives survive the move from code and
   self-development to continuous public reporting; role-specific branches are
   evidence against the harness.

**Route-changing insights:**

- design the claim/evidence/correction graph before the article loop;
- make source updates mutate standing questions, not launch a whole pipeline;
- use Super as the newsroom protocol and frozen decision cases as both
  production inputs and eval spines;
- measure context virtualization and correction quality, not article count;
- use the future proof to delete bespoke loops, not validate their revival.

**Changed action:**

- **definition:** Autopaper becomes an artifact program; World Wire becomes the
  contested public projection network.
- **implementation route:** typed modules and policies on the shared
  actor/RLM/Texture kernel, with sourcecycled narrowed to observation.
- **verifier/evidence:** controlled update waves, contradiction/correction,
  restart, multisupervisor dissent, provenance, and zero-authority proofs.
- **scope:** architecture and eval-case design now; no activation while tabled.
- **stopping condition:** same harness proves both self-development and
  continuous many-source reporting without a second authority path.

**Next high-information action:** once the RLM eval capture boundary exists,
construct one frozen historical report-time case and determine whether the
common kernel can process it with source/publication modules and inert
authority surfaces. That probe tests the generalization topology without
reopening live Autopaper.

## Decisions Still Required

- Is the claim graph a first-class object family or a projection over existing
  source, evidence, Texture, and publication objects?
- What identity rule distinguishes two reports about one event from two events
  that merely look similar?
- Which deterministic reducers may cluster and route observations before an
  RLM sees them?
- What opens, merges, parks, reopens, or settles a continuing story trajectory?
- Which publication classes require threshold agreement, unanimity, human
  approval, embargo, or an absolute refusal?
- How are source ownership and derivation lineages represented strongly enough
  to support independence claims?
- Does one Autopaper own article Textures and an edition Texture, or are
  editions deterministic projections over accepted publication versions?
- What correction semantics are shown publicly when only one claim in a long
  article changes?
- Which historical source bodies may be retained and replayed under privacy,
  copyright, deletion, and provider policies?
- How is attention allocation evaluated without optimizing toward engagement,
  novelty theater, or coverage volume?
- Which current processor, reconciler, ingestion-handoff, and publication paths
  are seeds, which are superseded symptoms, and which should eventually be
  deleted after a common-kernel proof?

## Invariants

1. Autopaper is a program inside a persistent computer, never a separate
   service, scheduler, computer ontology, or authority.
2. The World Wire represents the world as reported; claims retain attribution,
   time, evidence, contradiction, and correction lineage.
3. Source content is tainted observation and never grants instructions,
   capabilities, identity, or authority.
4. Source ingestion cannot decide editorial meaning, work completion, or
   publication.
5. Texture remains the canonical semantic writer for its document; articles,
   editions, renderings, and public routes do not become competing truths.
6. Production supervisor seats are attributable inference lanes, not eval
   shadows; controlled comparison uses isolated trials.
7. Required quorum and independence policy are fixed before supervisor outputs
   exist and cannot shrink around failures or dissent.
8. Consensus produces a decision receipt, never direct canonical or public
   mutation authority.
9. Publication binds one exact document and provenance head through a typed,
   idempotent trusted transaction.
10. Public correction is a forward, inspectable version transition; history is
    not silently rewritten.
11. Context grows by reference and orchestration, not by forcing the entire
    source world into a model window.
12. Model output is synthesis or a proposal, never self-authenticating source
    evidence.
13. Platform World Wire work belongs to the platform computer; personal
    Autopaper work belongs to its user computer; shared serving does not merge
    their semantic authority.
14. The two-store boundary remains intact; no news-specific third semantic
    ledger is introduced.
15. A future Autopaper proof cannot precede or substitute for acceptance of the
    automatic computer and its supervised self-development path.

## Current Non-Claims

This memo does not claim that:

- Autopaper is active, authorized, or ready for a new Definition;
- the current processor/reconciler pipeline implements this architecture;
- existing Trace or publication objects already form a sufficient claim graph;
- model plurality guarantees editorial or evidentiary independence;
- a consensus result makes public exposure safe;
- World Wire is objective truth or exhaustive world coverage;
- engagement, publication volume, or low latency proves reporting quality;
- later evidence may be leaked into a frozen historical eval;
- personal Autopapers or user publication are current product scope;
- architecture projection is staging acceptance.

## Relationship to Current and Historical Design

This memo is downstream of [Choir Doctrine](choir-doctrine.md), [Choir Agent
Product Doctrine](agent-product-doctrine.md), [Choir Computer
Ontology](computer-ontology.md), [The Automatic Computer — Choir
Vision](choir-vision.md), and [The Signal Is Sparse, Not the
Learner](signal-is-sparse-not-the-learner-2026-08-01.md).

It extends the proposals in [Persistent RLM Actors and Capsule-Isolated Go
Orchestration](memo-persistent-rlm-actors-2026-08-09.md) and [Live and
Retrospective Evals for the RLM
Computer](memo-live-retrospective-evals-2026-08-09.md).

The historical [Autopaper activation Definition](definitions/choir-autopaper-activation-2026-07-10.md)
and its [attempt report](definitions/choir-autopaper-activation-attempt-report-2026-07-11.md)
remain evidence of what not to reactivate: a long serial pipeline with unstable
substrate dependencies and duplicated lifecycle semantics. The archived
Texture/transclusion vision contributes the perspective and generative-style
insights only; its superseded product framing and topology are not restored.

If this projection is later adopted, the Super-as-protocol proposal must first
be reconciled with the currently promoted [Texture supervision
protocol](supervision-protocol.md), and all durable changes must pass the normal
architecture-promotion process. This memo does not silently revise those
authorities.
