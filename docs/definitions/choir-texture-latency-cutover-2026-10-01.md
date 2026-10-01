---
definition_version: 4
definition_id: choir-texture-latency-cutover-2026-10-01
readiness: executable
execution_mode: mission_orchestrator

# Owner-directed mission (owner 2026-10-01): "we should do a general latency
# improvement research... do the highest leverage moves". Per the
# continuous-authority rule, execution authority here is the owner statement;
# the file itself has not yet passed per-file panel review.

review:
  reviewer: none
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: '2026-10-01T14:05:00Z'
  source:
    canonical_ref: main@db7f1063
    deploy_identity: 'staging https://choir.news build.commit=aea62d05;
      owner guest computer-03335285269bdba4f94377e56879f9e6 commit f563200e'
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: goal_owned
      owner: this session
      touch: read_write
      recovery: git
  predecessor:
    mission: none — emerges from the texture-read repair (243665b4, aea62d05)
      and its divergent-consensus residual review
    evidence_ref: docs/problems/texture-latency-layered-evidence-2026-10-01.md
  observed_artifact:
    - claim: 'post-fix latency floor is ~1.4s quiet / 10s+ under desk drain;
        proxy api.resolve avg 1392ms (two uncached vmctl RPCs per request);
        guest /health direct 7ms idle, 1-3.7s under drain; per-event
        getLifecycleSnapshot reads every og_objects row for the whole
        computer under engineMu'
      ref: docs/problems/texture-latency-layered-evidence-2026-10-01.md

finish:
  deliver: 'Texture editor reads (document list, revision list, revision get,
    lifecycle snapshot) return in under ~500ms p50 / ~1.5s p95 on the owner
    staging computer, including while a desk mutation drain is running.'
  artifact: 'three landed cuts: (1) lifecycle snapshot reads bounded to the
    trajectory (version-keyed memoization or a scoped index — not a
    whole-computer scan per call); (2) proxy computer-route resolution cached
    with a short TTL or event invalidation instead of two vmctl RPCs per
    request; (3) read engineMu decoupled from write holds if measurement shows
    writes still starving reads after (1)+(2).'
  acceptance:
    - action: 'curl-timed GET /api/texture/documents, /api/texture/documents/
        {doc}/revisions, /api/texture/revisions/{rev}, and
        /api/trajectories/{traj} with the owner API key against choir.news,
        5 samples each, during a live desk drain (desk_pending_mutations > 10)'
      proves: 'p95 <= ~1.5s and p50 <= ~500ms end-to-end through the proxy'
      evidence_class: deployed proof
    - action: 'repeat the same probes while the desk is idle'
      proves: 'quiet-path latency is transport-bound, not compute-bound
        (<= ~400ms p50 through the proxy including auth+resolve+guest)'
      evidence_class: deployed proof
    - action: 'proxy health stage counters (api.resolve avg/max) after deploy'
      proves: 'resolve stage dropped from ~1.4s toward cache-hit latency'
      evidence_class: deployed proof
  rollback: 'git revert per cut; the route cache falls back to direct vmctl
    resolution, the snapshot cut falls back to whole-computer read; both are
    additive, no state migration.'
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: 'minimize end-to-end editor request latency while preserving
    authority semantics — every read still reflects the durable head, every
    route still authoritative, no cache may serve a torn view'
  goodharting_would_be: 'serving stale routes or stale snapshots to hit the
    latency number, or measuring only idle-window requests while the desk
    drain still starves reads'

homotopy:
  realism_axis: 'latency resolution — from today (per-request uncached resolve
    + whole-computer snapshot + single-mutex serialization) continuously
    deformable into the target (cached resolve, trajectory-scoped or memoized
    snapshot, read/write decoupled engine). Same objects, same authority, same
    freshness contract — only the wait shrinks.'

boundaries:
  mutation_class: orange
  authority_sources:
    - 'owner statement 2026-10-01: "do the highest leverage moves"'
    - docs/problems/texture-latency-layered-evidence-2026-10-01.md
    - divergent panel .agentic-consensus/agentic-consensus-20261001-074717
  must_preserve:
    - 'snapshot freshness: a response may never reflect a state older than the
      last committed lifecycle version the client already saw'
    - 'route authority: vmctl remains the route of record; cache TTL only,
      never a second authority'
    - 'objectgraph tombstone/visibility semantics on any scoped read'
    - 'the parked-run repair seam (actorruntime handler.go) is frozen'
  excluded:
    - 'frontend bundle work beyond measurement (already landed)'
    - 'proxy SSE/transport changes (unverified family E)'
    - 'multi-read-replica / split-store redesign — evaluate need after cuts 1+2'
    - 'guest VM shape changes (capacity mission owns that)'
  protected_surfaces:
    - 'proxy computer-resolution path (internal/proxy)'
    - 'objectgraph read/write serialization (engineMu)'
    - 'lifecycle snapshot contract consumed by SSE + editor'

conjecture:
  id: latency-bound-by-uncached-resolve-and-whole-computer-snapshot
  claim: 'If route resolution is cached and the lifecycle snapshot stops
    scanning the whole computer per call, p50 editor latency drops from ~1.4s
    to under ~500ms and drain-loaded p95 from 10s+ to ~1.5s without weakening
    freshness or authority.'
  test: 'deployed before/after timing matrix on the owner guest, both idle
    and during a desk drain; plus engineMu wait instrumentation if the
    residual is unexplained'
  edge: 'missing_oracle — if reads starve on engineMu behind provider-turn
    writes rather than on query cost, cuts 1+2 move the floor but not the
    loaded tail; cut 3 (decoupled read path) is the escalation'
  delta_o: 'engineMu wait/hold timing on the guest under drain — one log line
    per op distinguishes queueing from execution cost'
  scope_if_supported: 'all texture/lifecycle read paths on every computer'
  status: active
  evidence_refs:
    - docs/problems/texture-latency-layered-evidence-2026-10-01.md

decision:
  what: 'order of work: (1) snapshot memoization keyed on lifecycle_version
    (freshness check = one PK read of the trajectory row — the only write
    paths that touch snapshot-relevant objects are conditional appends that
    bump it); (2) short-TTL route cache in the proxy; (3) measure, then decide
    on engine decoupling'
  kind: operational
  status: settled
  evidence_ref: docs/problems/texture-latency-layered-evidence-2026-10-01.md
  owner_ratification_ref: 'owner directive 2026-10-01'

now:
  status: working
  slice: 'cuts 1+2 landed (c835c295): scoped lifecycle snapshot read + proxy
    route cache; cut 3 gated on the deployed timing matrix'
  source_ref: main@db7f1063

  deploy_identity: 'staging aea62d05; owner guest f563200e'
  candidate:
    id: none
    state: none
    ref: none
    base: main@db7f1063
    digest: none
    scope: []
  conjecture:
    id: latency-bound-by-uncached-resolve-and-whole-computer-snapshot
    claim: 'see conjecture block'
    test: 'deployed timing matrix idle + drain'
    edge: missing_oracle
    delta_o: 'engineMu wait/hold instrumentation'
    scope_if_supported: 'all texture/lifecycle read paths'
    status: 'partially_weakened — version-keyed memoization falsified by
      TestLifecycleActivationAdmissionUsesCanonicalAgentCAS: activation
      projection and title writes mutate snapshot-visible objects without
      bumping trajectory.LifecycleVersion. Cut 1 shipped as the two-phase
      filtered read instead.'
    evidence_refs:
      - docs/problems/texture-latency-layered-evidence-2026-10-01.md
  decision:
    what: 'cut 1 = two-phase filtered snapshot read (kind allowlist +
      trajectory metadata filter in one serializable tx); cut 2 = 2s route
      cache with transport-error invalidation; cut 3 = engine decoupling,
      only if the timing matrix still fails'
    kind: operational
    status: settled
    evidence_ref: docs/problems/texture-latency-layered-evidence-2026-10-01.md
    owner_ratification_ref: 'owner directive 2026-10-01'
  belief:
    believed_state: 'reads are individually fast post-243665b4; latency is
      resolve-RPC (1.4s) + whole-computer snapshot per event + mutex queueing
      under desk drain'
    main_uncertainty: 'whether writes hold engineMu across provider turns —
      if so, cut 3 becomes mandatory, not optional'
    next_observation: 'deployed timing matrix after cuts 1+2; if loaded p95 is
      still >3s, instrument engineMu wait/hold'
  blocker_or_risk: 'stale-route risk on the 2s cache is bounded by transport-
    error invalidation; a missed invalidation path serves a dead upstream for
    at most one TTL window'
  next_action: 'acceptance matrix recorded (idle, desk drained); cut 3
    decision pending a drain-loaded re-measurement — idle p95 is ~1.9s on
    trajectories, dominated by guest-side event/run scan work'

receipts:
  - id: cuts-1-2-deployed
    boundary: implement
    identity: c835c295 (+5439733b empty retrigger)
    proof_refs:
      - 'CI run 36875419303 success; staging deployed_commit=5439733b; guest
        computer-03335285269bdba4f94377e56879f9e6 rebooted onto 5439733b
        via DEPLOY_ACTIVE_VM_REFRESH'
      - 'proxy stage counters post-deploy: api.resolve n=10 avg=1ms max=4ms
        (was avg=1392ms); route cache serving every request'
      - 'deployed timing matrix idle (desk drained to 1): texture/documents
        0.28-0.72s, trajectories 0.92-1.93s through choir.news'
    rollback_ref: 'git revert c835c295 — cache falls back to per-request
      vmctl resolve; snapshot falls back to whole-computer scan'
    disposition: 'cuts 1+2 landed and measured; cut 3 gated on drain-loaded
      p95 re-measurement'
    landing:
      source_commit: 5439733b
      ci_ref: '36875419303 success'
      deploy_ref: '36875419303 success'
      environment_identity: 'staging 5439733b; guest 5439733b'
      deployed_acceptance: 'idle matrix recorded; drain-loaded pending'

weak_measures:
  - name: 'editor request p50/p95 through proxy'
    kind: weak_signal
    baseline: '~1.4s quiet / 10s+ under drain (2026-10-01 measured)'
    desired: 'p50 <= 500ms, p95 <= 1.5s under drain'
    decision_use: 'decides whether cut 3 (engine decoupling) is required'
    cannot_prove: 'that the editor feels instant — it certifies request
      latency, not perceived interactivity'
  - name: 'proxy api.resolve avg'
    kind: telemetry
    baseline: 'avg 1392ms, max 20147ms'
    desired: 'avg < 100ms on cache hits'
    decision_use: 'confirms the route cache is actually serving'
    cannot_prove: 'cache correctness — a wrong-URL cache hit would still be
      fast'

---

## Context

The divergent panel (7/13 runners) clustered on: per-event whole-computer
snapshot reads, uncached vmctl route resolution, and read/write mutex
serialization. Two of three are directly actionable; the third escalates only
if measurement demands. Full evidence map:
docs/problems/texture-latency-layered-evidence-2026-10-01.md.
