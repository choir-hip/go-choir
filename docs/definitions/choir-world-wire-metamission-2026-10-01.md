---
definition_version: 4
definition_id: choir-world-wire-metamission-2026-10-01
readiness: drafted
execution_mode: mission_orchestrator
metamission: true

review:
  reviewer: 'agentic-consensus panel 2026-10-01 — convergent verdict across 7
    panelists (claude, codex, omp-gemini38, omp-glm53-flash, omp-gpt6-sol,
    omp-gpt6-luna, devin). Synthesis below reconciles the granularity split;
    the load-bearing calls were unanimous.'
  evidence_ref: .agentic-consensus/manifest.tsv

start:
  captured_at: '2026-10-01T04:30:00Z'
  source:
    canonical_ref: main@c4ec6ee0
    deploy_identity: 'staging https://choir.news build.commit=4a718af4'
  predecessor:
    mission: choir-platform-dolt-capacity-stabilization-2026-10-01
    disposition: 'sibling, not predecessor-in-series — the substrate mission
      bounds store memory; this metamission is GATED on it but runs its
      green-class design stations in parallel.'
    evidence_ref: docs/problems/platform-dolt-oom-realization-cluster-2026-10-01.md
  observed_artifact:
    - claim: 'processor and reconciler are already non-desk ordinary role
        registries (research+evidence+memory+coagent tools); only
        management/texture/research carry desk-carrier profiles
        (internal/agentcore/tool_profiles.go:350-595).'
      evidence: 'scout dossier + source'
    - claim: 'the legacy sourcecycled reconciler-request queue is vestigial:
        BuildIngestionHandoff emits zero ReconcilerRequests; storage_test.go:418
        asserts 0. Reconciliation is driven by publish-debounce only.'
      evidence: 'internal/cycle/ingestion_handoff.go:40-66; cmd/sourcecycled/main.go:1057'
    - claim: 'the wire_captures Qdrant semantic dedup
        (internal/agentcore/qdrant_dedup.go:38) exists but has zero production
        callers; codex warns it is unsafe to wire unchanged (per-item search,
        upserts after, misses within-batch duplicates, pass-through on
        failure).'
      evidence: 'scout + panel'
    - claim: 'June attempt-12 failed on mechanical/capacity causes — readiness
        timeout, connection-pool starvation, provider 429, admission freeze,
        duplicate-rewarm cancellation, and a self-amplifying
        busy-store→failed-probe→kill-recover loop that is literally today''s
        OOM→connection-refused→restart cycle.'
      evidence: 'git history d10ec47b — autopaper-activation-attempt-report-2026-07-11'

finish:
  deliver: 'A compounding, self-citing knowledge base on the object graph that
    turns general-news source ingestion into published World Wire editions —
    first general news, then verticals sequenced one-by-one (AI first, then
    Taiwan/geopolitics/semis/internal-democracy, then the region ladder), all
    on RLM machinery at moderate-now-scale-later.'
  artifact: 'the ordered W-station set landed on staging; see `metamission.stations`.'
  metamission_stations:
    - id: W0
      name: wire-kb-graph-schema
      class: green
      deliver: 'Typed world-wire object/edge kinds on the objectgraph registry —
        source_version, claim, entity, thread, article + edges
        cites/transcludes/asserts/about/corroborates/contradicts/supersedes —
        plus a typed graph read/write API for RLM activations. A vertical is a
        graph query over tagged sources, not new feed config.'
      gated_on: 'nothing — pure schema; runs parallel to the substrate mission.'
      store_type_decision: 'FIRST ACT — which StoreType the wire kinds register
        into (platform-dolt vs corpus-dolt vs separate). Sits at the
        intersection of the compounding KB, the OOM bounds, and Dolt
        transactional integrity.'
      acceptance: 'a scripted ingest→claim→cite sequence yields a deterministic
        graph head provable as a computer-version ObservationObjectGraphHead.'
    - id: W1
      name: wire-ingest-tiered-dedup
      class: orange
      deliver: 'Durable capture + three-tier dedup at ingest: (0) canonical
        URL+content-hash, (1) lexical near-dup, (2) semantic Qdrant AS
        CANDIDATE-ROUTING not evidence-deletion — every source/version
        observation is preserved; similarity groups investigation candidates
        with inspectable scores and a safe unavailable-index path.'
      gated_on: 'capacity-stabilization acceptance + Qdrant reachable on staging.'
      acceptance: 'measured duplicate-drop rate on general news with Dolt
        memory staying in budget; a restart leaves an inspectable backlog, not
        lost items.'
      fixes_june: 'backpressure + durable idempotency keys replace the
        admission-freeze failure mode — a transient provider failure defers
        with visible lag instead of freezing admission.'
    - id: W2
      name: wire-processor-batch
      class: red
      deliver: 'The per-item→per-set seam. One processor RLM activation per
        intake window (e.g. 15min / ≤200 dedup-survivors), reasoning over the
        SET + the subgraph it pulls in: extract claims, link entities, decide
        attach-to-existing-thread vs new-thread, cite into the KB. Texture
        handoff only on materiality threshold.'
      gated_on: 'W0+W1; one end-to-end processor→Texture delegated-cast handoff
        verified.'
      acceptance: 'one general-news batch → typed graph objects → one Texture
        draft, end-to-end on staging. record_wire_processor_decision reworked
        per-batch.'
      note: 'keeps processor as a non-desk work policy on the common actor
        lifecycle; replaces per-request update_coagent continuity with
        collection-manifest obligation. June''s per-item admission gate is
        removed.'
    - id: W3
      name: wire-reconciler-corpus
      class: orange
      deliver: 'Reconciler as impacted-index-driven corpus-consistency over the
        compounding KB: entity merge (reversible via supersede, never delete),
        citation integrity (no dangling/superseded cites), duplicate-thread
        merge, contradiction records, corrections as forward Texture writes.
        O(impacted entities), not per-publish reread.'
      gated_on: 'W2.'
      acceptance: 'a merge or correction lands as a graph transaction visible
        on staging; DELETE the vestigial ReconcilerRequest queue machinery
        (storage.go Save/List/Supersede).'
      note: 'a changed source-version can challenge a standing claim before
        the next publication — reconcile from changed evidence, not only
        publish-debounce.'
    - id: W4
      name: wire-edition-assembly
      class: red
      deliver: 'A multi-story general-news edition = an immutable
        allocation-of-attention snapshot over a bounded subgraph: selected
        stories, exact article revision heads, citations, editorial receipts,
        and a public projection readable when the producing guest is down.
        Later articles transclude/cite earlier Choir claims — the compounding
        metric is the share of claims with ≥1 internal citation.'
      gated_on: 'W2+W3.'
      acceptance: 'a reader sees the edition on staging; a later correction
        changes it by forward revision, never silent replacement. This — not a
        single article — is "general news working."'
    - id: W5
      name: wire-vertical-ai
      class: orange
      deliver: 'First vertical: AI. Sources.json already carries
        verticals/regions tags — a vertical = source cohort + attention policy
        + standing-question set over the KB. Config+policy, not new code.'
      gated_on: 'W4.'
      acceptance: 'a multi-source AI story, a justified exclusion, and a
        correction all visible in the AI edition on staging.'
      dissent_note: 'owner directive names AI first; pitch-bones names Taiwan
        the first reporting SERIES — reconcile at W5: AI is the first vertical,
        Taiwan is the first series; update the pitch doc to remove the dual
        authority.'
    - id: W6
      name: wire-vertical-taiwan-cluster
      class: orange
      deliver: 'Taiwan, geopolitics, semiconductors, internal-democracy —
        one-by-one, each a config+policy station. Taiwan is the first
        reporting series; needs Chinese-language sources.'
      gated_on: 'W5 stable.'
      acceptance: 'per-vertical edition + a contested/changed claim with its
        correction path on staging.'
    - id: W7
      name: wire-region-ladder
      class: orange
      deliver: 'Region ladder one-at-a-time until intake budget holds, then
        parallel: broader East Asia → Southeast+South Asia → West Asia → East
        Africa → rest of Africa → Latin America → Europe+US. Biotech+science
        run a parallel EVIDENCE-ONLY lane (research-desk authority, no
        publication route) from after W5 until their slot.'
      gated_on: 'W6 cadence holding.'
      acceptance: 'each step is a source pack + language + entity seed; no new
        architecture. Real local sourcing, not an English-feed relabel.'
    - id: W8
      name: wire-scale-hardening
      class: orange
      deliver: 'The thousands/min ceiling: tier concurrency, batch-sizing
        curves, backpressure, measured cost/tokens-per-1000-inputs model.
        Deliberately deferred until moderate scale has run — honest
        no-painted-corner, not premature provisioning.'
      gated_on: 'moderate-scale soak (weeks).'
      acceptance: 'a cost model + measured ceiling; scale by adding batcher
        workers (parallel RLM activations), not redesigning tier interfaces.'

value:
  better_means: 'turn source ingestion into a compounding, self-citing
    knowledge base — minimize per-item reasoning cost while preserving the
    invariant that every published claim is graph-cited back to a source
    version and the KB grows by transclusion.'
  goodharting_would_be: 'a flat article feed that "publishes" without writing
    typed claims/entities/threads to the graph, or a dedup that silently drops
    corroborating sources to lower the queue count. The KB compounding metric
    (claims with ≥1 internal citation) cannot be faked by article volume.'

homotopy:
  realism_axis: 'capture fidelity + graph-native claim granularity. Low
    resolution = whole-article objects with title-level dedup; full = typed
    claim/entity/thread objects with cited provenance and corpus-level
    consistency. Every station preserves topology — same store, same
    trajectory/desk machinery, same Texture-write authority — only resolution
    rises.'

boundaries:
  authority: 'Texture remains the sole canonical document writer.
    Processor/reconciler stay NON-DESK work policies on the common actor
    lifecycle — never new canonical writers, never root desks, never a
    parallel state authority.'
  mutation_class: 'red for graph-write and Texture-handoff surfaces; orange for
    dedup/edition/reconciler routing; green for W0 schema + verticals config.'
  invariants:
    - 'RLM architecture is the invariant — reasoning stays on
      desk/channel/trajectory machinery over the object graph as working
      memory; no bolt-on pipeline producing flat stories.'
    - 'Qdrant semantic similarity is candidate ROUTING, never disposition
      authority — it must not erase corroboration or consequential updates.'
    - 'every internal citation traces back to an external source_version — no
      self-citation loops that never ground out.'
    - 'bad merges are undone by SUPERSEDE, never delete — a self-citing KB
      cannot tolerate dangling history.'
  exclusions:
    - 'no premature scale provisioning — moderate-now, scale-later (owner
      amendment 2).'
    - 'no new global role classes (the 2026-08-09 memo constraint).'
    - 'the vestigial reconciler-request queue is DELETED at W3, never revived
      as a second authority.'

now:
  status: working
  slice: 'metamission authored from panel consensus + June receipt; green-class
    design (W0 ontology, station files, D4 amendment) proceeds now; live
    ingestion is gated on capacity-stabilization acceptance.'
  conjecture:
    bridge: 'If the compounding-KB graph substrate (W0-W4) lands on RLM
      machinery under a bounded store, then World Wire advances from a
      feed-capture to a self-citing knowledge base — and the June
      admission-freeze failure class cannot recur.'
    verdict: 'proposed — the seam (per-item→per-set) and the KB-first ordering
      are unanimous; the store-type decision and batch-attach policy are the
      open edges.'
  decision:
    what: 'the seam is the work unit: replace one-ProcessorRequest-per-cycle
      dispatch with bounded collection-manifest obligations on the durable
      channel. Processor = batched-set intake; reconciler = impacted-index
      corpus-consistency. Both stay non-desk.'
    kind: architecture
    status: settled
    owner_ratification_ref: 'owner directive 2026-10-01: "RLM based ... radical
      redesign ... compounding knowledge base ... recursively transcluding and
      citing ourselves"'
  belief:
    believed_state: 'substrate is being bounded (corpus-dolt cap holding,
      MemoryCurrent ~11.6GiB, OOMKills=0); guests alive and draining; the
      metamission is designed but not yet executable.'
    main_uncertainty: 'whether corpus-dolt is the right store for the wire
      kinds (W0 first act) and whether the 502 routing flap is fully cleared
      by the cap or is an independent routing lag.'
    next_observation: 'a clean multi-hour uptime window + a successful
      run start + the 362febb2 reactivation discharging.'
  blocker_or_risk: 'live W-stations are hard-gated on capacity-stabilization
    deployed acceptance; design/authoring is not.'
  next_action: 'author the W0 station file (KB-graph schema) green-class in
    parallel; persist the corpus-dolt cap drop-in; watch the uptime window for
    the 362febb2 discharge.'

receipts:
  - kind: owner_decision
    ref: '2026-10-01: "lets do the repairs and improvements ... world wire
      territory ... RLM native ... radical redesign ... compounding knowledge
      base ... verticals one by one"'
  - kind: consensus
    ref: .agentic-consensus/manifest.tsv (2026-10-01 run — 7/13 converged)
  - kind: june_receipt
    ref: 'git d10ec47b — autopaper-activation-attempt-report-2026-07-11:
      attempt-13 fails the same way; the per-item admission gate and the
      busy-store→kill-recover loop are the failure modes the redesign removes'
  - kind: substrate_fix
    ref: 'corpus-dolt cgroup cap applied live 2026-10-01: MemoryHigh=12G
      MemoryMax=14G, MemoryCurrent 16.6→11.6GiB, OOMKills=0'
---

## The design rationale (one page)

World Wire is not a feed — it is a compounding knowledge base on the object
graph where the system cites and transcludes *itself* alongside sources. That
reframes processor and reconciler from per-article gates into graph-write and
graph-consistency over a self-citing KB.

The single load-bearing seam is the **unit of reasoning**: today's
one-ProcessorRequest-per-source-route-per-cycle actor loop cannot scale and —
per the June receipt — its per-item admission gate is the failure mode that
froze attempt-12 under a transient 429. The redesign replaces it with a
**bounded collection-manifest obligation**: durable capture + tiered dedup →
one batched RLM activation per intake window reasoning over the *set* plus its
subgraph → attach-to-thread vs new-thread + claim extraction + cite-into-KB →
Texture handoff only on materiality.

Reconciler shifts symmetrically from per-publish review to impacted-index
corpus-consistency — entity merge (reversible via supersede), citation
integrity, contradiction records — O(impacted entities), not O(corpus).

Processor and reconciler keep their non-desk profile names as *policy
identities* but lose their per-request loop semantics; the legitimate fallback
the panel named (fold both into uniform batched actors) stays open at W2 if
the named-role semantics fit a generic actor better.
