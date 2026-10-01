---
definition_version: 4
definition_id: choir-appdev-s5-live-preview-supervision-2026-10-01
execution_mode: mission_orchestrator
readiness: reviewed
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
    safe desktop preview; the trusted serving path attests which candidate
    digest the owner has rendered for S6's later approval decision.
  artifact: >-
    A staging desktop preview path fed by a capability-scoped broker bridge to
    a capsule dev-server Unix socket, with Texture work entries (or explicit
    plain-ledger fallback) and a trusted rendered-candidate-digest attestation.
  acceptance:
    - action: >-
        On staging, drive an API-key-started capsule session through a source
        edit, fetch and test. In the logged-in owner desktop, observe their
        Texture entries and the preview change without reload; Playwright
        records the action-to-render latency.
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
        Render candidate A, change the capsule subject to candidate B, then
        attempt to use the still-A preview binding. The trusted preview
        broker/serving path refuses the stale binding and records the rendered
        and current candidate digests; re-rendering B is required for a new
        binding attestation.
      proves: >-
        Preview digest binding fails closed before any S6 approval or commit
        decision and is never accepted from hostile preview JavaScript.
      evidence_class: deployed negative proof
    - action: >-
        If Texture transclusion remains unavailable at execution, repeat the
        deployed session and observe ordered plain ledger entries for its
        commands, fetches and tests, including a recorded transclusion gap.
      proves: >-
        Supervision remains honest and usable without falsely claiming that
        the unavailable rich-transclusion dependency was delivered.
      evidence_class: deployed proof
  rollback: >-
    Git revert and redeploy remove the bridge and preview route. Disable the
    preview capability and invalidate a pending preview binding whose rendered
    digest is absent or mismatched; restore a computer through its pinned-head
    path if a previously accepted change must be withdrawn.
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
    then a trusted rendered-candidate-digest attestation for S6 to evaluate.

weak_measures:
  - name: preview-update-latency
    kind: weak_signal
    baseline: unknown until the first deployed action-to-render trace
    desired: station-proposed p95 at or below 2 seconds without reload
    decision_use: >-
      Informs whether the preview transport is adequate for live supervision
      or needs a bounded performance investigation.
    cannot_prove: >-
      Preview isolation, Texture attribution, digest binding, owner approval,
      or station completion.

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
    - deployment routing for the isolated preview origin
    - approval binding

heresy_delta:
  discovered:
    - >-
      The requested capsule preview is a hostile-web authority boundary:
      existing Unix-socket and lifecycle-projection seams do not establish
      origin isolation or a trusted rendered-digest attestation.
  introduced: []
  repaired:
    - none; this station remains checkpoint_incomplete pending S4.

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
      Deployed Playwright proves no-reload updates, malicious preview
      JavaScript cannot read Texture, desktop cookies, or invoke approval, and
      the trusted serving path refuses a stale rendered digest before S6.
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
      with strict CSP, no desktop cookies, a narrow postMessage bridge, and a
      rendered-digest attestation issued only by the trusted broker/serving
      path for S6 to evaluate.
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
      candidate subject-digest concepts provide seams, but no deployed capsule
      preview supervision path is evidenced at this station's start.
    main_uncertainty: >-
      Whether one S4-compliant broker capability can carry dev-server updates
      with the required latency while preserving origin and approval isolation.
    next_observation: >-
      S4 completion evidence, followed by a disposable capsule rehearsal that
      measures action-to-render correlation and exercises the hostile-preview
      negative path.
  blocker_or_risk: >-
    S4 is incomplete. The critical risk is treating an embedded preview as
    trusted desktop content, or accepting a preview-provided digest or message
    as authority over the broker/serving-path attestation.
  next_action: >-
    Promotes to working when S4-capsule-open-world completes; then freeze a
    disposable bridge candidate and rehearse the deployed proof, explicit
    plain-ledger fallback, and hostile-preview negative probes.

receipts: []
---

# S5 — Live Preview and Supervision

## Mechanism

The desktop owns the preview capability and terminates the broker bridge; the
capsule exposes only its development server through that capability. The bridge
is scoped to one capsule, one owner desktop and one candidate lifetime. It does
not become general capsule networking or a desktop reverse proxy. Its separate
preview origin is a deployment-routing change, not a cosmetic iframe setting.

The preview is served from an isolated origin, with no desktop cookies, strict
CSP and a deliberately small, typed `postMessage` contract. The parent accepts
only origin-checked preview lifecycle/render notifications; it never accepts
approval commands, credentials, arbitrary URLs, HTML, executable payloads or a
candidate digest from preview JavaScript. The preview receives no Texture
document, desktop session material or approval capability.

Capsule command, fetch and test receipts enter the existing Texture lifecycle
projection. If Texture transclusion is available, the work document embeds the
live preview/work source; otherwise the delivered fallback is ordered plain
ledger entries plus a recorded transclusion gap, not a simulated rich view.

## Binding and challenge

Each rendered state is attested with the candidate digest by the trusted broker
and serving path, not by preview JavaScript. When the capsule subject changes,
that path invalidates the prior attestation; absence or inequality is a
refusal, never a warning or a best-effort refresh. This station produces no
approval authorization: S6 alone consumes the attestation at its commit gate.

The red-surface candidate must be independently challenged with negative probes
for cross-origin cookie access, Texture reads, forged or replayed preview
messages, approval invocation and update ordering. Its receipts must identify
the isolated origin, CSP, accepted message schema, rendered digest, candidate
digest and measured action-to-render bound. Rollback disables the capability
before any retry, so a bad preview path cannot remain a privileged desktop route.
