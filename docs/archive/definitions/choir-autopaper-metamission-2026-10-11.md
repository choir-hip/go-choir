---
definition_version: 4

# Autopaper (Gate 3): the computer pulls content into its object graph, keeps
# living Textures revised by evidence, and publishes them automatically.
# Design: docs/autopaper-sw-design-2026-10-09.md (station SW, owner brief
# and decisions 2026-10-09). Owner 2026-10-09: "Gate 1 today, Gate 2
# tomorrow, Gate 3 in October"; "viable meaning no shortcuts". Owner
# 2026-10-11: "keep going back to the meta mission and keep making progress
# into gate three". Supersedes the 2026-10-01 World Wire metamission
# (already marked superseded in the mission graph).
readiness: executable

review:
  reviewer: none
  frozen_ref: none
  verdict: none
  evidence_ref: none

metamission:
  stations:
    - id: W1
      path: docs/archive/definitions/choir-autopaper-w1-content-in-the-graph-2026-10-11.md
      readiness: executable
      status: complete
      depends_on: []
    - id: W2
      path: docs/archive/definitions/choir-autopaper-w2-living-textures-2026-10-11.md
      readiness: drafted
      status: pending (carried by re-engineering Cut 2 on Cut 1; owner 2026-10-12)
      depends_on: [W1]
    - id: W3a
      path: docs/archive/definitions/choir-autopaper-w3a-reader-2026-10-11.md
      readiness: drafted
      status: pending (revised 2026-10-12 to a linked permalink per version; independent of the re-engineering stations)
      depends_on: [W1]
    - id: W3
      path: none (auto-publish, W3b; to be authored when W2 lands)
      readiness: intent
      status: pending
      depends_on: [W2, W3a]

start:
  captured_at: 2026-10-11T09:00:00Z
  source:
    canonical_ref: main 78c9173e
    deploy_identity: staging choir.news 78c9173e
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: goal_candidate
      owner: this session
      touch: goal_owned
      recovery: git
  observed:
    - >-
      Corrected 2026-10-11 (owner): Autopaper's shared content lives in the
      global store (Store B behind corpusd), not in the paper computer's
      embedded Dolt; computers keep overlays
      (problems/autopaper-captures-built-per-computer-against-store-b-2026-10-11.md).
      The July cause C1 (one embedded connection) is therefore off the
      ingest path; W1a still upgraded Dolt past the race behind it.
    - >-
      choir.web_capture exists in the object graph
      (internal/objectgraph/web_capture.go); polling code exists in
      internal/sources (RSS, Telegram web preview, GDELT, Polymarket).
    - >-
      The host store behind corpusd was reset to empty on 2026-10-09;
      Autopaper starts from zero content.
    - >-
      Research is one persistent desk per document with sessions that
      outlive runs (PD1, PD2 on staging); Texture is one desk per document.

finish:
  deliver: >-
    An Autopaper computer that polls the MVP sources into the object graph,
    keeps Textures revised as evidence arrives, and publishes them at
    permalinks that logged-out readers can open, with every piece the real
    mechanism (design §1.8).
  artifact: W1, W2, W3 landed on staging with deployed acceptance.
  acceptance:
    - action: each station's acceptance on staging
      proves: the station's claim on the product path
      evidence_class: deployed proof
  rollback: per station, in reverse order
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Maximize grounded, current Textures readable at permalinks per unit of
    inference, while preserving Texture as sole writer, research read-only,
    one authority per fact, and reads that never touch the paper's computer.
  goodharting_would_be: >-
    Counting captures or revisions that nobody can read; a pipeline of fixed
    stages instead of desks; a workaround for C1 that a later station has
    to tear out.

homotopy:
  realism_axis: >-
    captures land in the graph without starving Texture -> Textures revise
    from delivered evidence -> automatic publishing and public permalinks.

boundaries:
  mutation_class: red (embedded store concurrency, Texture wakes, publishing)
  authority_sources:
    - owner brief and decisions 2026-10-09 (design doc)
    - owner 2026-10-11 overnight goal
  must_preserve:
    - Texture is the only writer of a Texture
    - research has read-only authority toward the world
    - one authority per fact (one route, one revision chain, one publication state)
    - published reads never touch the paper's computer
    - a crash restart never resumes work
  excluded:
    - new feeds and beats (design §6)
    - the per-computer research lift and the Texture root (PD3; owner review) unless W2 needs them
  protected_surfaces: [internal/store embedded Dolt, object graph, Texture commit path, corpusd host store]

now:
  status: working
  slice: >-
    W1 complete. The protocol's P1 and P2 landed (staging b172895b); W2
    (living Textures) waits on P2d2, which is designed through three review
    rounds and held for the owner's review (one open question: how a
    Texture run takes directive work from outside the owner's request).
    The Autopaper app is designed
    (docs/autopaper-sw-design-2026-10-09.md): a news front page whose
    stories open as Textures with quoted, captured sources, built on the
    protocol. It proposes a reader slice (one permalink per story, a public
    front page, citations that quote captures) that does not depend on
    P2d2. Owner direction 2026-10-11 evening: design before building
  source_ref: 5b93ba54 (pushed)
  deploy_identity: staging b172895b
  candidate: {id: W1, state: landing, ref: main, base: 78c9173e, digest: 56ec63e0, scope: [internal/platform, internal/proxy, internal/agentcore, internal/actorruntime, internal/store, internal/researchtools, internal/sources, internal/yaegikernel]}
  conjecture:
    id: AP-1
    claim: >-
      With captures in Store B (bodies on host disk) and polling as
      generation-fenced durable wakes with no model call, continuous polling
      shares the paper computer with Texture and stays inside Store B's
      memory budget, and a crash never resumes it.
    test: W1 acceptance on staging (hour-long scheduled run; Store B rows and body files; corpus-dolt memory; crash restart)
    edge: missing_oracle
    delta_o: Texture turn latency under polling load is not yet measured; W2 puts Texture and polling on one computer
    scope_if_supported: one Autopaper computer, a handful of sources
    status: supported
    evidence_refs: [docs/archive/definitions/choir-autopaper-w1-content-in-the-graph-2026-10-11.md receipts W1c, W1-hour, W1-read, W1-fix, W1-fix2, W1-panel3]
  decision: {what: "shared content (captures, published Textures) in Store B behind corpusd, bodies in host content storage; computers hold overlays", kind: architecture, status: settled (owner 2026-10-11), evidence_ref: problems/autopaper-captures-built-per-computer-against-store-b-2026-10-11.md, owner_ratification_ref: "owner 2026-10-11: yes, move the write path to host store"}
  belief:
    believed_state: >-
      Scheduled polls land in Store B with no body bytes in rows, dedupe on
      re-poll, record usage, back off on failure, and close on a crash
      restart. RDF feeds never produced items before the parser fix. Two
      search providers are out of quota (owner billing).
    main_uncertainty: whether Texture stays responsive with polling on the same computer (W2's first observation)
    next_observation: W2's file review, then Texture turn latency with polling on the paper computer
  blocker_or_risk: the MVP source list is still open (design §8 open item 3); W1 used six stand-in sources
  next_action: >-
    owner reviews the master report
    (docs/archive/reports/master-report-protocol-research-autopaper-2026-10-11.md):
    the Autopaper app design and its reader slice, P2d2's open question, and
    the research job (Autopaper's quality depends on research). Then author
    the reader slice as a station and re-author W2 on P2d

receipts: []
---

# Autopaper metamission (Gate 3)

Autopaper is the computer reading the world and keeping living documents
about it. The design fixes only what the substrate must guarantee and
leaves the rest to the four desks. It is built in three stations:

1. **W1, content in the graph.** Captures land as `choir.web_capture`
   objects without queueing behind Texture on one connection (the July C1
   cause). Polling runs on the source schedule, and resource use is
   tracked from the first poll.
2. **W2, living Textures.** Management notices stories and opens or feeds
   Textures; Texture revises on delivered evidence; records flow and
   resolve.
3. **W3, readers.** Automatic publishing, the Autopaper app, and public
   permalinks served from the host store.
