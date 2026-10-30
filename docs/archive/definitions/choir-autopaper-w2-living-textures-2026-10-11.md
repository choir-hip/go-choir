---
definition_version: 4

# W2 — living Textures. Station of
# definitions/choir-autopaper-metamission-2026-10-11.md; design
# docs/autopaper-sw-design-2026-10-09.md §2 (Textures: one writer, woken by
# evidence; desks talk through precommitment records), §4 (spend follows
# story size) and §6 W2.
readiness: drafted

review:
  reviewer: agentic-consensus file review (7 panelists)
  frozen_ref: b36d58b3
  verdict: send_back
  evidence_ref: receipt W2-review1

start:
  captured_at: 2026-10-11T14:40:00Z
  source:
    canonical_ref: main 56ec63e0 (W1 candidate; W1 not yet settled)
    deploy_identity: staging choir.news 726aa392
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: goal_candidate
      owner: this session
      touch: goal_owned
      recovery: git
  observed:
    - >-
      W1 (pending its completion panel): the paper computer polls sources on
      generation-fenced durable wakes into choir.web_capture objects in
      Store B, bodies on host disk; any bound computer reads a capture with
      choir.ReadCapture. Polling makes no model call and notifies no desk.
    - >-
      Management reaches product routes with choir.ProductAPI (allowlist in
      internal/agentcore/tools_product_api.go: /api/prompt-bar,
      /api/texture/, /api/autopaper/ and others). /api/prompt-bar opens a
      document whose first revision is an owner prompt, which wakes that
      document's Texture desk.
    - >-
      Texture is the sole writer of a document's revisions. Research
      reports to Texture with choir.Report; Texture opens research and
      management on its own document's trajectory. There is no observed
      path yet for management to deliver evidence to a Texture other than
      its own trajectory's.
    - >-
      Commitment records exist (choir.Precommit, choir.Resolve, the
      commitment pack); no Autopaper desk uses them yet.
    - >-
      Captures are whatever the writing guest says they are (W1 panel,
      second round): a reader of shared captures must filter by a trusted
      owner or computer.

finish:
  deliver: >-
    On the paper computer, management notices stories in the incoming
    captures and opens or feeds Textures; each Texture revises when its
    evidence arrives, citing the captures; and desks record what they
    expect and resolve it, with resource use tracked per story.
  artifact: >-
    A bounded, debounced delivery of new captures to management; a read of
    recent shared captures filtered to trusted writers; management's path to
    open a story Texture and to deliver evidence to an existing one; Texture
    revisions that cite captures; precommitment records for story size;
    usage per story and per Texture.
  acceptance:
    - action: >-
        With the W1 schedules running on the paper's test computer for two
        hours, management receives capture deliveries no more often than the
        debounce allows, opens at least one story Texture, and later feeds
        it at least one capture that arrived after it opened. That Texture
        has at least two revisions after its first, each citing captures
        by URL.
      proves: Textures live on delivered evidence, not on a prompt
      evidence_class: deployed proof (test computer)
    - action: >-
        Management records a precommitment of each story's expected size
        before spending on it; at least one resolves by count with no model
        call.
      proves: desks talk through records, and resolution by counting is free
      evidence_class: deployed proof
    - action: >-
        Measure Texture turn latency and polling cost on the paper computer
        under the schedule, and model tokens per story.
      proves: Texture stays responsive with polling on the same computer; spend is visible per story
      evidence_class: deployed measurement
    - action: restart the paper's computer while a delivery is pending
      proves: a crash restart closes the delivery with a visible fate and nothing resumes on its own
      evidence_class: deployed proof
  rollback: >-
    revert each slice; stopping the schedules stops all deliveries; no
    data migration
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    More of the world's reporting reaches a living Texture that cites it,
    per unit of model spend, with each Texture revised only when evidence
    arrives.
  goodharting_would_be: >-
    A new Texture per capture batch instead of revising existing ones;
    revisions that restate without new evidence; a Texture written by
    management or research instead of the Texture desk; a delivery on every
    poll regardless of spend; counting opened Textures instead of cited
    captures.

homotopy:
  realism_axis: >-
    one hand-sent delivery wakes management -> debounced deliveries from the
    schedule -> management opens one Texture -> a later capture feeds it ->
    records and per-story usage -> two hours unattended.

boundaries:
  mutation_class: >-
    red (actor wakes to management; a delivery path into Texture; product
    API surface for management)
  authority_sources:
    - owner design decisions 2026-10-09 (design doc §1, §2, §4, §6)
    - owner 2026-10-11 — shared content in the global store; computers hold overlays
    - CLAUDE.md desk ontology (Texture sole writer; management owns coherence)
  must_preserve:
    - only the Texture desk writes a Texture revision
    - polling makes no model call; a delivery is debounced and bounded
    - a crash restart never resumes work
    - shared captures are read filtered to a trusted writer
  excluded:
    - the per-computer research lift and the Texture root (PD3; owner review) unless W2 cannot land without them
    - publishing and readers (W3); email
    - new feeds and beats
  protected_surfaces:
    - internal/agentcore (source schedule, management wakes, product API allowlist)
    - internal/textureowner (any new delivery into a Texture)
    - internal/actorruntime (wake kinds, restartWorkKinds)

now:
  status: blocked
  slice: >-
    W2 waits on the inter-agent protocol design (owner 2026-10-11: all
    inter-agent communication through precommitment records); the file is
    re-based on it before W2a code
  source_ref: 56ec63e0 (local; pushes after the S1h demo)
  deploy_identity: staging 726aa392
  candidate: none
  conjecture:
    id: W2-1
    claim: >-
      A debounced delivery naming the new captures since a watermark is
      enough for management to notice stories and choose to open or feed a
      Texture, and an evidence delivery addressed to a document wakes that
      document's Texture desk to revise, without a new Texture substrate.
    test: deployed — a hand-sent delivery wakes management, which opens a Texture through an existing route and later feeds it
    edge: missing_oracle
    delta_o: management's turn trace (Updates read, controls emitted) and the Texture's revisions and sources after a delivery
    scope_if_supported: one paper computer, a handful of sources, one owner
    status: proposed
    evidence_refs: []
  decision:
    what: >-
      open — how management delivers evidence to a Texture other than its
      own trajectory's (an existing report or channel path addressed to the
      document, or a new delivery kind); and whether a Texture management
      opens is authored as an owner prompt or carries management's
      provenance
    kind: architecture
    status: open; settle in W2a from the code, conservative default is an
      existing path with management's provenance recorded
    evidence_ref: none
  belief:
    believed_state: >-
      Polling, captures and reads work (W1). Nothing yet tells management
      that captures arrived, and management has no observed way to feed a
      Texture outside its own trajectory.
    main_uncertainty: whether an existing delivery path reaches an arbitrary document's Texture desk
    next_observation: trace one management turn woken by a hand-sent delivery on staging
  blocker_or_risk: >-
    W1 must settle first (its second completion panel). Each scheduled
    poll adds one chain event; W2's deliveries add management activations,
    so the debounce and the per-story spend record land with the first
    delivery, not after.
  next_action: >-
    owner reviews the precommitment-records protocol design; then re-author
    this file on it (deliveries as records addressed to management and to
    texture:<doc>; story size as a precommit resolved by count) and re-review

receipts:
  - id: W2-review1
    at: 2026-10-11T15:30Z
    kind: agentic-consensus file review (session scratchpad w2/review)
    result: >-
      Sent back 5 to 2 (Claude, Codex, GLM, GPT-6 Luna and GPT-6.1 Sol send
      back; DeepSeek and Gemini accept). No general path exists for
      management to deliver evidence to a Texture outside its own
      trajectory: channel mail refuses texture targets, and management's
      report path binds to one exact delivered control, trajectory and work
      item. ProductAPI lets management write a Texture as the owner
      (/revisions, /revise, /restore), so "management never writes a
      Texture" is not enforced and acceptance could pass on owner-authored
      revisions. A precommit cannot resolve by count (no runtime resolver,
      no deadline, no story key). The watermark delivery can skip captures
      (newest-first list with no cursor) and loses a notification if arming
      fails. The prompt-bar open path cannot carry management's provenance.
---

# W2 — living Textures

W1 put the world's reporting into the shared store. W2 makes it matter:
management notices stories, Textures form around them, and each Texture
revises when new evidence arrives. The paper mostly revises; it rarely
starts over.

## W2a — captures reach management

The schedule handler already knows when a poll wrote new captures. After
such a poll it arms one delivery to the paper computer's management desk,
unless one is already pending. The delivery is a durable wake, not a
model call. It carries a watermark (the time of the newest capture it
covers) and a count, not the items. Management reads the captures since its
last watermark through a product route on the `/api/autopaper/` allowlist,
filtered to the paper's own owner, and records the watermark it has read.

Debounce: at most one pending delivery, and none sooner than a minimum gap
after the last one (default 30 minutes, set on the schedule API). Many polls
between deliveries collapse into one.

Restart: the delivery kind joins `restartWorkKinds`; a crash boot closes a
pending delivery with the visible fate "interrupted by a restart", and the
next poll with new captures arms a fresh one.

Ways it can fail, written before the code:

- a delivery per poll (no debounce), which spends a management activation
  every few minutes;
- a delivery that carries every item, growing without bound;
- management reads captures written by another owner's guest and treats
  them as the paper's;
- a watermark that skips captures written out of order (use the capture's
  creation time in Store B, and read with overlap; ids dedupe);
- a pending delivery that survives a crash and fires after it;
- management's mailbox held by a slow capture read.

## W2b — management opens and feeds Textures

Management decides what is a story and how much to spend on it. To open a
story it opens a document whose first revision states the story and cites
the captures that started it; the document's Texture desk writes from
there. To feed a story, management delivers the new captures to that
document's Texture desk, which decides whether to revise. Management never
writes a Texture.

The open decision in the now card is settled here from the code, before
any new route is added.

## W2c — records and spend

Before spending on a story, management records a precommitment: how big
the story will get (captures, sources, revisions) by when. The count of
captures and revisions resolves it with no model call. Usage (tokens,
model, time) is recorded per story and per Texture, as W1 records it per
poll. No budget yet: the design says track first.
