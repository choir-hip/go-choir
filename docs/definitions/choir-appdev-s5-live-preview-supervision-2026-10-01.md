---
definition_version: 4
definition_id: choir-appdev-s5-live-preview-supervision-2026-10-01
execution_mode: mission_orchestrator
readiness: drafted
member_of: choir-supervised-app-development-metamission-2026-10-01

review:
  reviewer: none
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: '2026-10-01T00:00:00Z'
  source:
    canonical_ref: main@8aa1dce9
    deploy_identity: 'staging https://choir.news deployed_commit=a3cfaa00;
      owner guest computer-03335285269bdba4f94377e56879f9e6 on a3cfaa00'
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: goal_candidate
      owner: this session
      touch: goal_owned
      recovery: git
  observed_baseline:
    - >-
      Self-dev control plane is live: M11 rendered evidence in the Texture
      document, while the effect plane reaches only the served release
      (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:94-109).
    - >-
      Capsules are air-gapped today; S4 supplies recorded capsule egress and
      a capsule-private Nix store before this station may start
      (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:112-114,
      560-573).
    - >-
      Texture transclusion and rich formatting are not integrated; the shared
      fallback is plain ledger entries with the gap recorded
      (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:357-362).
    - >-
      The existing capsule client reaches its broker through a Unix-domain
      socket from outside the capsule namespace
      (internal/capsule/broker_client.go:16-18).
    - >-
      Texture already projects durable lifecycle events by document,
      trajectory and cursor (internal/textureowner/texture_observation.go:38-72,
      101-142).
    - >-
      Existing engineering reports distinguish an observed subject digest from
      a changed candidate subject digest
      (internal/types/engineering_assignment.go:404-425, 506-525).

finish:
  deliver: >-
    An owner watching an active capsule development session sees its commands,
    fetches and tests in Texture and sees the capsule app update live in a
    safe desktop preview; approval is possible only for the candidate the
    owner actually saw rendered.
  artifact: >-
    A staging desktop preview path fed by a capability-scoped broker bridge to
    a capsule dev-server Unix socket, with Texture work entries and a rendered
    candidate-digest approval binding.
  acceptance:
    - action: >-
        On staging, drive an API-key-started capsule session through a source
        edit, fetch and test. In the logged-in owner desktop, observe their
        Texture entries and the preview change without reload; Playwright
        records action-to-render latency with p95 at or below 2 seconds.
      proves: >-
        The owner has live, causally attributable work supervision and a
        functioning capsule preview on the deployed product path.
      evidence_class: deployed proof
    - action: >-
        In the same deployed browser proof, run hostile JavaScript from the
        preview origin that attempts to read Texture state, desktop cookies,
        or send an approval-shaped message; each attempt is denied while the
        narrow documented preview-to-parent message still functions.
      proves: >-
        The preview is an isolated hostile-web boundary, not a privileged
        desktop surface.
      evidence_class: deployed negative proof
    - action: >-
        Render candidate A, change the capsule subject to candidate B before
        approval, then approve from the still-A preview. The deployed commit
        gate refuses the request and records the rendered and candidate
        digests; re-rendering B is required before a successful approval.
      proves: >-
        Owner approval binds the rendered preview digest to the committed
        candidate digest and fails closed on stale preview.
      evidence_class: deployed negative proof
  rollback: >-
    Git revert and redeploy remove the bridge and preview route. Disable the
    preview capability and refuse pending approvals whose rendered digest is
    absent or mismatched; restore a computer through its pinned-head path if a
    previously accepted change must be withdrawn.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the gap between capsule work and owner-observable, correctly bound
    preview state while preserving the desktop's authority and session boundary.
  goodharting_would_be: >-
    A screenshot or reload-only preview, a work log detached from capsule
    actions, or a digest label that approval does not verify against the
    committed candidate.

homotopy:
  realism_axis: >-
    One capsule-scoped dev-server stream at increasing resolution: a static
    response, then hot asset update, then commands/fetches/tests in Texture,
    then an owner approval proven against a changed candidate digest.

boundaries:
  mutation_class: red
  authority_sources:
    - owner-ratified metamission spine (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:373-379)
    - S5 station intent (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:574-577)
    - repository mutation and landing contract (AGENTS.md:109-131, 193-202)
  must_preserve:
    - Capsules own no semantic state; only an accepted event changes desired code.
    - Owner approval (or a stronger declared consensus policy) gates every commit.
    - Every committed change is restorable through the pinned-head path.
    - Provider credentials never enter a capsule.
  excluded:
    - S4 capsule egress recording and capsule-private Nix-store construction.
    - S6 commit-gate release materialization and its wider canonical event-commit work.
    - S7 dynamic app packages and S8 source publication.
    - Texture transclusion implementation; consume it when available, otherwise use the shared plain-ledger fallback.
  protected_surfaces:
    - desktop preview route
    - approval binding

now:
  status: checkpoint_incomplete
  slice: >-
    S5 live preview supervision is pending S4-capsule-open-world; author the
    bridge only after S4 establishes the capsule boundary it depends on.
  source_ref: main@8aa1dce9
  deploy_identity: 'staging https://choir.news deployed_commit=a3cfaa00'
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: isolated-preview-is-sufficient-supervision
    claim: >-
      A capability-scoped broker bridge and digest-bound preview can give the
      owner live supervision without making hostile preview code a desktop or
      approval principal.
    test: >-
      Deployed Playwright proves sub-2-second no-reload updates and proves
      malicious preview JavaScript cannot read Texture, desktop cookies, or
      invoke approval; a stale rendered digest is refused by the commit gate.
    edge: missing_oracle
    delta_o: >-
      Instrument a deployed action-to-render correlation and record both the
      rendered digest and candidate digest at approval evaluation.
    scope_if_supported: >-
      A single staging desktop and one S4-compliant capsule dev-server session;
      it does not assert safety for arbitrary desktop embedding or publication.
    status: proposed
    evidence_refs: []
  decision:
    what: >-
      Use a broker-bridged Unix socket to a separately originated preview,
      with strict CSP, no desktop cookies, a narrow postMessage bridge, and
      approval refusal unless rendered and candidate digests match.
    kind: architecture
    status: proposal
    evidence_ref: >-
      S5 intent plus owner-ratified metamission authorization
      (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:373-379, 574-577)
    owner_ratification_ref: >-
      owner ratified the metamission spine on 2026-10-01
      (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:375-379);
      this bridge protocol remains a station proposal.
  belief:
    believed_state: >-
      Capsule broker Unix-socket transport, Texture lifecycle projection and
      candidate subject-digest concepts provide seams, but no preview
      supervision path is evidenced yet.
    main_uncertainty: >-
      Whether one S4-compliant broker capability can carry dev-server updates
      with the required latency while preserving origin and approval isolation.
    next_observation: >-
      S4 completion evidence, followed by a disposable capsule rehearsal that
      measures action-to-render correlation and exercises the hostile-preview
      negative path.
  blocker_or_risk: >-
    S4 is incomplete. The critical risk is treating an embedded preview as
    trusted desktop content, which would expose Texture or approval authority.
  next_action: >-
    Promotes to working when S4-capsule-open-world completes; then freeze a
    disposable bridge candidate and rehearse the deployed proof and hostile
    preview negative proof.

receipts: []
---

# S5 — Live Preview and Supervision

## Mechanism

The desktop owns the preview capability and terminates the broker bridge; the
capsule exposes only its development server through that capability. The bridge
is scoped to one capsule, one owner desktop and one candidate lifetime. It does
not become general capsule networking or a desktop reverse proxy.

The preview is served from an isolated origin, with no desktop cookies, strict
CSP and a deliberately small, typed `postMessage` contract. The parent accepts
only origin-checked preview lifecycle/render notifications; it never accepts
approval commands, credentials, arbitrary URLs, HTML or executable payloads
from preview JavaScript. The preview receives no Texture document, desktop
session material or approval capability.

Capsule command, fetch and test receipts enter the existing Texture lifecycle
projection. If Texture transclusion is available, the work document embeds the
live preview/work source; otherwise it renders ordered plain ledger entries and
records the transclusion gap rather than simulating a rich view.

## Binding and challenge

Each rendered state carries the candidate digest that produced it. The approval
evaluation compares that rendered digest with the frozen candidate digest at the
same decision boundary; absence or inequality is a refusal, never a warning or
a best-effort refresh. A subsequent capsule mutation invalidates the prior
preview binding.

The red-surface candidate must be independently challenged against cross-origin
cookie access, Texture reads, forged or replayed preview messages, approval
invocation and update ordering. Its receipts must identify the isolated origin,
CSP, accepted message schema, rendered digest, candidate digest and the
measured action-to-render bound. Rollback disables the capability before any
retry, so a bad preview path cannot remain a privileged desktop route.
