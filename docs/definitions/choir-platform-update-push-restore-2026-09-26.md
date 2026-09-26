---
definition_version: 3
definition_id: choir-platform-update-push-restore-2026-09-26
execution_mode: mission_orchestrator

start:
  captured_at: '2026-09-26T13:00:00Z'
  source:
    canonical_ref: main@87259816
    deploy_identity: staging https://choir.news build.commit=722b49bf
    worktrees:
      - path: /Users/wiz/go-choir
        status: clean
        class: source
        owner: this session
        touch: read_write
        recovery: git
  predecessor:
    mission: choir-selfdev-derivable-continuations-2026-09-26
    disposition: >-
      M7 settled 2026-09-26 (deployed build 722b49bf, ci 36241357497).
      Selfdev ops advance on derivable continuations — canonical
      decision append -> post-commit observer -> drain -> reconciler
      drives Accepted -> Materializing -> Applied with checkpoint +
      route promotion, zero API calls after the owner decision; crash
      mid-materialize recovers on the same reconciler. The owner
      decision remains the only authority gate. M9a is unblocked: the
      pipeline produces a signable materialized change.
    evidence_ref: docs/definitions/choir-selfdev-derivable-continuations-2026-09-26.md
  spine_meta_goal: docs/definitions/choir-rectification-spine-2026-09-25.md
  station: M9a
  owner_directive: >-
    Plan §11 M9a (on spine, after M7): platform-signed update push +
    restore to a pinned head — a tracking computer fast-forwards
    non-divergent components on receipt of a platform release under a
    declared follow policy, without the self-dev ceremony. Proof: the
    staging computer applies a pushed update and restores to a pinned
    commit. On M11's restore edge.

finish:
  outcome: >-
    The platform can ship a signed update to a computer and the computer
    applies it through its own updater under the declared
    platform-follow policy — update content, authority signature, and
    canonical head binding all verified — and can then restore to a
    prior pinned head through the existing checkpoint/tape path. A
    tracking computer needs no owner click, no LLM pass, and no
    self-dev operation to advance its platform-tracked surface.
  artifact: >-
    An end-to-end platform-update path: a platform-control-signed update
    offer carrying the release manifest + content, transported into the
    guest over the existing vmctl->autoputer proxy; a guest endpoint
    that verifies the signature/policy/head binding, stages the payload
    into the updater's trusted incoming store, drives updater.Apply,
    and commits the canonical update events; route promotion under a
    platform-follow evidence class (not the owner self-dev projection
    path); and the existing pinned-head restore exercised after the
    push. On staging: the update lands on a live computer and the
    restore returns it to the pinned prior head.
  acceptance:
    - action: >-
        Boundary probe: name the exact seams — signed update envelope
        object, guest apply endpoint, canonical event binding,
        route-ledger evidence class — and which parts are new vs reused.
      proves: R2-boundary discipline — acceptance names only reachable
        evidence.
      evidence_class: boundary probe
    - action: >-
        A platform-signed update offer presented to the guest apply
        endpoint mutates the served release: updater.Apply receipts
        verify, canonical update events commit with head binding, and
        the route slot promotes under the platform-follow evidence
        class — no owner decision, no selfdev operation.
      proves: the push path is real, not a self-dev relabel.
      evidence_class: local test
    - action: >-
        After the pushed update, restore to the prior pinned head
        returns the prior release: the checkpoint/tape path resolves
        the pre-push head and RestagePinnedRelease repoints current —
        the update is a forward transaction on the computer's event
        chain, reversible.
      proves: >-
        M9a's restore edge holds — platform updates are
        recoverable, not destructive.
      evidence_class: local test
    - action: >-
        Signature and policy refusals: an update offer with a bad
        signature, wrong computer binding, expired window, or a
        divergent/pinned lineage is refused before any mutation.
      proves: the guest verifies authority before apply — a forged or
        misbound push cannot write.
      evidence_class: local test
    - action: >-
        On staging: a pushed platform update lands on a live tracking
        computer (served release advances, route slot promotes) and a
        subsequent pinned restore returns the prior head — observed via
        deployed endpoints, not local fixtures.
      proves: the M11 restore edge works on the deployed product path.
      evidence_class: deployed proof
  rollback: >-
    git revert + redeploy. The push path is additive: a new offer type,
    endpoint, and evidence class; selfdev apply/rollback semantics are
    unchanged underneath. A computer that took a bad update restores
    via the same pinned-head path being proven.
    Protected surfaces: canonical event commit path (new event kinds +
    platform-authorized desired-state append), updater trust boundary
    (root-owned incoming + manifest verification unchanged), checkpoint/
    route projection (new platform-follow evidence class beside
    selfdev's), platform-control signing domain.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the gap "a tracking computer can only advance its platform
    surface through the heavyweight self-dev ceremony or a manual
    break-glass restage" while preserving every authority invariant —
    platform updates are signed, head-bound, journaled, reversible, and
    policy-gated.
  goodharting_would_be: >-
    An unsigned/owner-impersonating push, an apply that skips canonical
    events so restore can't find the head, or a "pushed update" that is
    really a self-dev operation wearing platform clothes — the letter
    satisfied, the platform-follow authority boundary missed.

homotopy:
  realism_axis: >-
    Tracking-computer fast-forward only: non-divergent components
    (follow policy auto/proposal), frontend/static payload change first.
    Divergent-component 3-way merge, CoSuper rebase capsules, and the
    1-click update desk UI are Phase-3 scope — out. Low resolution: a
    release carrying only an SPA file change. High resolution: the same
    envelope carries multi-component payloads; topology identical.

boundaries:
  mutation_class: red
  authority_sources:
    - docs/desk-rlm-rectification-plan-2026-09-23.md §11 (M9a)
    - docs/reports/choir-platform-update-system-consensus-2026-09-08.md (platform-follow-v1)
    - docs/agent-product-doctrine.md
    - docs/computer-ontology.md
  must_preserve:
    - owner decision remains the only authority for self-dev ops — a
      platform update is not an owner decision and must not mint one
    - updater manifest/trust verification and receipt verification
      unchanged; incoming store stays root-owned
    - route CAS/generation mechanics unchanged; platform-follow is a
      new evidence class, not a relaxed selfdev path
    - pinned-head restore requires the checkpoint/tape path — no
      direct pointer swap endpoint
    - divergent/pinned lineage is never auto-advanced
  excluded:
    - divergent-component merge / CoSuper rebase (Phase 3)
    - desktop 1-click update UI (Phase 3)
    - computer->computer publish (M9b), choir->microVMs (M10)
    - global deploy restaging semantics (DEPLOY_ACTIVE_VM_REFRESH
      fence unchanged — push is the sanctioned per-computer path)
    - R5a vocab decoders (separate station, gates M11)
  protected_surfaces:
    - canonical event/commit path
    - checkpoint + route projection authority
    - updater root-owned incoming store + apply journal
    - platform-control / guest-core / verifier signing domains
    - vmctl proxy authorization (internal caller gating)

now:
  status: working
  slice: >-
    station M9a WORKING 2026-09-26 — boundary probe complete.
    Gap confirmed: no platform->computer update channel exists;
    PlatformRelease/ComputerLineage are unconsumed schema; the only
    updater.Apply callers are selfdev materialize/rollback. Chosen
    seams: (1) signed update offer minted by platform-control —
    computer/realization/idempotency/release/content digests +
    lineage policy + expiry; (2) guest apply endpoint reached via the
    existing vmctl autoputer-proxy transport, verifying signature +
    computer binding + epoch + policy before staging payload into the
    updater incoming store; (3) canonical binding — a platform-authorized
    desired-state event commits before Apply so AcceptedEventHead is a
    real head, then applied event + checkpoint + route promotion under
    a new platform-follow evidence class (TransitionPromote mechanics,
    distinct evidence payloads — not the owner self-dev endpoint);
    (4) restore edge = existing checkpoint/tape rematerialization
    (AcceptedEventHead pinning + RestagePinnedRelease). Rejected:
    guest poller (push is the mandate; poll adds a second authority to
    tune), self-dev route reuse (wrong evidence contract — would forge
    owner-decision evidence), unversioned apply (breaks restore
    binding).
  source_ref: main@87259816
  deploy_identity: staging https://choir.news build.commit=722b49bf
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: platform-follow-push
    claim: >-
      A platform-control-signed update offer, applied by the guest
      updater under canonical head binding and promoted through the
      route ledger under a platform-follow evidence class, gives M11
      the restore edge it was ratified to require: updates are forward
      transactions on the event chain, recoverable via pinned-head
      restore.
    test: >-
      Local: signed offer -> guest apply -> receipts + canonical events
      + route promotion; restore to pre-push pinned head returns the
      prior release; forged/expired/misbound offers refused.
      Deployed: staging computer takes the push and restores.
    edge: >-
      missing_oracle — staging proof needs a live computer whose
      updater surface the push can reach; if no reachable tracking
      computer exists on choir.news, the deployed leg degrades to
      local+identity evidence and the edge is named, not hidden.
    delta_o: >-
      The vmctl autoputer-proxy transport already reaches a live
      guest; the observer upgrade is a staging computer with a
      tracking lineage. If staging proves unreachable, the named
      residual is exercised on M11's own restore leg.
    scope_if_supported: >-
      platform-tracked computer surfaces advance without per-update
      ceremony; M11's restore edge has a proven forward transaction
      to restore from
    status: active
    evidence_refs:
      - docs/desk-rlm-rectification-plan-2026-09-23.md §11 (M9a)
      - docs/reports/choir-platform-update-system-consensus-2026-09-08.md
      - internal/vmctl/handlers.go:1228-1297 (proxy transport)
      - internal/updater/updater.go:115-228 (apply state machine)
      - internal/agentcore/rematerialize.go (pinned-head restore)
  decision:
    what: >-
      Platform-follow push over the existing vmctl->autoputer proxy;
      canonical desired-state event binds the head before Apply;
      platform-follow evidence class for route promotion; restore via
      the existing checkpoint/tape edge.
    kind: operational
    status: settled
    evidence_ref: >-
      boundary probe 2026-09-26 (scout findings, this file's
      now.slice + receipts)
    owner_ratification_ref: >-
      docs/desk-rlm-rectification-plan-2026-09-23.md §11 ratified
      2026-09-25; docs/reports/choir-platform-update-system-consensus-2026-09-08.md
      Phase-2 platform-follow-v1
  belief:
    believed_state: >-
      CHARTERED. M7 landed — derivable continuations + receipts live.
      The platform has a release artifact (PlatformRelease) and a
      signing domain (platform-control) but no update channel; the
      guest has the updater, credentials, and restore path. The work
      is the signed offer + guest apply endpoint + canonical/route
      binding — not new primitives.
    main_uncertainty: >-
      Whether staging exposes a reachable tracking computer for the
      deployed proof leg; api-key computers have been qualified
      unreachable before. If so, the deployed leg narrows to the
      protocol/handshake surface and the named edge moves to M11.
    next_observation: >-
      First construct: the signed offer type + guest verification leg.
      What would most change the picture: whether updater.Apply's
      AcceptedEventHead join forces a canonical event first (believed
      yes — sha256 requirement + accepted-head binding).
  blocker_or_risk: >-
    Authority risk: the update must never masquerade as an owner
    decision — platform-follow evidence is a distinct class with
    platform-control provenance, and a pinned/divergent computer must
    be refused, not auto-advanced. Route-projection receipt idempotency
    windows (authorization window truncates to the minute) must not
    collide under retry.
  next_action: >-
    Construct the signed offer type + guest apply endpoint
    (verification-first: signature, computer/realization binding,
    lineage policy, expiry), wire the canonical desired-state append +
    apply + promotion sequence, then local proofs, then landing loop.

receipts:
  - "boundary probe 2026-09-26: no platform->computer update channel
    exists — PlatformRelease/ComputerLineage are unconsumed schema
    (internal/platformrelease), updater.Apply has only selfdev callers
    (self_development_materializer.go:229,280), and no platform-update
    verb exists on vmctl/autoputer/agentcore route tables. Transport
    exists: /internal/vmctl/autoputer-proxy/{owner}/{path} reverse-
    proxies to the live guest (internal-caller gated). Canonical
    requirement: ApplyRequest.AcceptedEventHead must be a real head —
    a platform-authorized desired-state event precedes Apply (guest
    capability already carries event:append). Route: TransitionPromote
    mechanics + new platform-follow evidence payloads; the owner
    self-dev projection endpoint is the wrong contract and is not
    reused. Restore edge exists and is reused unchanged: checkpoint/
    AcceptedEventHead tape rematerialization + RestagePinnedRelease.
    Verifier domain name is verifier-control."
---

# M9a — Platform-Signed Update Push + Restore (station on the rectification spine)

Live station under [`choir-rectification-spine-2026-09-25.md`](choir-rectification-spine-2026-09-25.md).
Scope per [`desk-rlm-rectification-plan-2026-09-23.md`](../desk-rlm-rectification-plan-2026-09-23.md) §11
(M9a, on spine, unblocked by M7): a tracking computer applies a platform-signed
update push and restores to a pinned head — the forward transaction and the
restore edge M11 needs. Architecture ratified:
[`reports/choir-platform-update-system-consensus-2026-09-08.md`](../reports/choir-platform-update-system-consensus-2026-09-08.md)
(platform-follow-v1, Phase 2).

## The one-line mission

A platform-control-signed update offer -> guest verifies + stages + applies
through its own updater -> canonical events + checkpoint + route promotion
under platform-follow evidence -> restore to the prior pinned head works. The
update is a forward transaction on the computer's event chain, not a pointer
swap — that is what makes it restorable.
