---
definition_version: 4
definition_id: choir-wire-kb-graph-schema-w0-2026-10-01
readiness: drafted
execution_mode: mission_orchestrator

start:
  captured_at: '2026-10-01T04:45:00Z'
  source:
    canonical_ref: main@c4ec6ee0
    deploy_identity: 'staging https://choir.news build.commit=4a718af4'
  spine: choir-world-wire-metamission-2026-10-01
  observed_artifact:
    - claim: 'the objectgraph registry already carries choir.web_capture and
        choir.universal_wire_story_cluster (internal/objectgraph/web_capture.go:13-16)
        plus choir.source_entity/choir.source_ref
        (internal/store/texture_source_graph.go:18-19). It has NO kinds for
        claims, entities, threads, or source versions.'
      evidence: 'registry.go + grep'
    - claim: 'kind registration is schema-only (green) — it does not touch
        runtime behavior until W1+ wires writes.'
      evidence: 'objectgraph registry is kind-registration based with
        deterministic head machinery already consumed by computer-version
        snapshots'

finish:
  deliver: 'The typed world-wire graph schema exists on the objectgraph
    registry, so every later station writes claims/entities/threads into a
    compounding knowledge base instead of flat prose.'
  artifact: 'registered object kinds + edges and a typed graph read/write API
    usable by RLM activations; the store-type decision recorded.'
  acceptance:
    - action: 'register the minimal kind/edge set and run a scripted
        ingest→claim→cite sequence'
      proves: 'a deterministic graph head is produced and provable as a
        computer-version ObservationObjectGraphHead'
      evidence_class: 'staging or harness proof'
    - action: 'record the store-type decision (platform-dolt vs corpus-dolt vs
        separate) with the OOM-bounds rationale'
      proves: 'the KB lands on a store whose memory bound is understood'
      evidence_class: 'design receipt'
  rollback: 'schema registration is additive — remove the kind registrations;
    no data migration exists at W0.'
  landing:
    required: true
    environment: staging

kinds:
  - choir.source_version      # one immutable versioned source observation
  - choir.claim               # a salient assertion bearing on entities
  - choir.entity              # a named thing claims attach to
  - choir.thread              # a developing story the KB tracks
  - choir.article             # a published projection of a thread subgraph
edges:
  - cites         # claim/article -> source_version or prior claim
  - transcludes   # article -> thread/claim (recursive self-cite)
  - asserts       # source_version -> claim
  - about         # claim -> entity
  - corroborates  # claim -> claim
  - contradicts   # claim -> claim
  - supersedes    # revision/merge -> prior object (reversible, never delete)
invariants:
  - 'every internal citation traces back to an external source_version.'
  - 'claim extraction bounded at ≤5 core claims per source item (graph-
    explosion guard).'
  - 'no object is ever deleted for correctness — supersede preserves history.'

value:
  better_means: 'the smallest typed schema that lets later stations write
    cited, transcludable claims — not a speculative ontology ahead of any
    article flow.'
  goodharting_would_be: 'registering a broad speculative ontology before any
    real article flows through it.'

homotopy:
  realism_axis: 'kind granularity — start with the minimal 5-kind/7-edge set;
    kind-registration makes later additions cheap and continuous.'

now:
  status: superseded
  director_note_2026_10_05: >-
    SUPERSEDED with the 10-01 World Wire metamission. The store-type decision
    is re-posed by the rearchitecture (claim feed + per-computer og) and the
    S8 claim-feed design slice.
  slice: 'W0 is green-class and may run parallel to the capacity mission; its
    first act is the store-type decision, which sits at the intersection of
    the KB, the OOM bounds, and Dolt transactional integrity.'
  next_action: 'decide store type (platform-dolt vs corpus-dolt vs separate)
    with the OOM-bound rationale, then register the 5 kinds + 7 edges and add
    the typed read/write API.'

receipts:
  - kind: spine
    ref: docs/definitions/choir-world-wire-metamission-2026-10-01.md
  - kind: precedent
    ref: 'docs/world-wire-mission-stack-2026-09-22.md Phase 5a names
      ReportedClaim/corroboration/contradiction objects — W0 is that
      projection made executable'
---
