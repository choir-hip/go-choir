---
definition_version: 3
definition_id: choir-platform-computer-park-2026-09-25
execution_mode: mission_orchestrator

start:
  captured_at: '2026-09-25T05:55:00Z'
  source:
    canonical_ref: main@daa7a216
    deploy_identity: staging https://choir.news build.commit=daa7a216 (deployed
      2026-09-25; boot crash fixed)
  worktrees:
    - path: /Users/wiz/go-choir
      status: dirty
      class: goal_candidate
      owner: this session
      touch: goal_owned
      recovery: git
      note: 'internal/vmctl/platform_computer.go modified — IsHeld guard added
        to WarmUniversalWirePlatformComputer (uncommitted).'
  predecessor:
    mission: K ontology kernel (choir-ontology-kernel-2026-09-24) — in flight
    disposition: >-
      This is the 1.4 hygiene interleave between K and R2, alongside the
      authored (not-yet-promoted) 1.5 test-signal-purge. It does not gate K
      or R2; it removes a noise/incident source from staging first.
    evidence_ref: docs/desk-rlm-rectification-plan-2026-09-23.md
  observed_artifact:
    - claim: >-
        During the boot-incident triage, vmctl's journal on node-b showed
        candidate-fleet-d03dacaa7404b1e4412b2e6f (the universal-wire-platform
        computer, computer-4c20ff4a21a021c4306d8c783be0037d, epoch 12649,
        ~30 days) spamming 'runtime api: submit internal run: computer is
        pre-genesis: run admission refused' every ~3s.
    - claim: >-
        The VM is NOT a stale orphan: ownership class is primary/active and
        WarmUniversalWirePlatformComputer in the idle sweeper resurrects it
        whenever it is stopped or hibernated (always-on platform computer).
        Hibernate alone does not hold it down.
    - claim: >-
        WarmUniversalWirePlatformComputer does not consult IsHeld(), unlike
        WarmAlwaysOnDesktops which skips held computers. So a maintenance hold
        cannot actually park the platform computer — a real gap fixed in this
        mission's candidate.

finish:
  deliver: >-
    The universal-wire-platform computer is parked (held + hibernated) so it
    stops spamming pre-genesis refusals and burning CPU, while remaining
    resumable. Its durable state and ownership are preserved for later
    unhold + resume.
  artifact: >-
    (a) WarmUniversalWirePlatformComputer honors IsHeld (hold guard shipped);
    (b) the platform ownership carries a maintenance hold and its firecracker
    process is down; (c) the vmctl journal shows no further pre-genesis spam
    from computer-4c20ff4a.
  acceptance:
    - action: 'go build ./internal/vmctl && go test ./internal/vmctl'
      proves: the hold guard compiles and the vmctl suite passes
      evidence_class: local test
    - action: >-
        POST /internal/vmctl/hold for universal-wire-platform, then
        /internal/vmctl/hibernate; observe ownership state stays
        hibernated/stopped (not booting) for >60s and no new firecracker
        process for d03dacaa exists
      proves: the platform computer is held down — the resurrection loop
        respects the hold (previously it did not)
      evidence_class: deployed proof (node-b vmctl + journal)
    - action: 'journalctl -u go-choir-vmctl | grep pre-genesis (after park)'
      proves: the pre-genesis internal-run refusals stopped
      evidence_class: deployed proof
  rollback: >-
    POST /internal/vmctl/unhold + /internal/vmctl/resume for
    universal-wire-platform re-launch the platform computer on its preserved
    data volume. The hold-guard code change is a one-line guard; git revert
    removes it but does not affect the parked state.
  landing:
    required: true
    environment: staging (node-b vmctl)
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the divergence between "operator wants the wedged platform
    computer parked" and "the always-on sweeper keeps resurrecting it",
    while preserving the durable state and the hold-authority contract that
    already protects every other always-on desktop.
  goodharting_would_be: >-
    Silencing the log line (filtering 'pre-genesis' out of the journal) or
    killing the firecracker process without a hold — the sweeper re-launches
    it within seconds, so the spam resumes. The metric looks like quiet but
    the wedged computer is still burning a VM.

homotopy:
  realism_axis: >-
    Parked-ness: log-silence (no behavior change) -> hibernate without hold
    (resurrected) -> hold + hibernate with the sweeper honoring IsHeld (the
    real parked state) -> a platform-computer that can be told to stay down
    and resumed on demand. The mission sits at the third rung and must not
    regress to the first two.

boundaries:
  mutation_class: red
  authority_sources:
    - owner directive (this session): park the platform computer (Hibernate it)
    - vmctl maintenance-hold contract (VAL-VM-012, hold is host-authoritative)
  must_preserve:
    - the platform computer's durable state dir and ownership record
    - the maintenance-hold semantics (refuse automatic lifecycle while held)
    - resumability: unhold + resume must restore it
  excluded:
    - deleting the platform computer state (DestroyVMState) — owner chose park
    - fixing the underlying pre-genesis condition (why it has no canonical
      genesis) — a separate investigation, recorded as residual risk
    - any change to WarmAlwaysOnDesktops or other hold consumers
  protected_surfaces:
    - vmctl lifecycle routing (hold/hibernate/resume)
    - VM state dir /var/lib/go-choir/vm-state/candidate-fleet-d03dacaa...

now:
  status: complete
  slice: platform computer parked (held + stopped); hold-guard shipped
  source_ref: main@1566bbc7
  deploy_identity: staging https://choir.news build.commit=1566bbc7
  candidate:
    id: platform-park-1
    state: landed
    ref: main@1566bbc7
    base: main@daa7a216
    digest: none
    scope: [internal/vmctl/platform_computer.go]
  conjecture:
    id: pp-hold
    claim: >-
      The platform computer stays parked if and only if the sweeper's warm
      path honors IsHeld; with the guard shipped, hold + hibernate parks it
      durably and the pre-genesis spam stops.
    test: >-
      Hold + hibernate on node-b after the guard deploys, then watch the
      ownership state and journal for >=60s: state must not return to
      booting/active and no pre-genesis refusals may recur.
    edge: missing_oracle
    delta_o: >-
      Direct journal + process inspection on node-b is the oracle; if the
      sweeper resurrects a held VM despite the guard, another warm path
      (WarmAlwaysOnDesktops or a recovery sweep) is the unguarded entry.
    scope_if_supported: >-
      the universal-wire-platform computer specifically; the hold-guard
      pattern generalizes to any always-on ownership
    status: supported
    evidence_refs:
      - 'node-b vmctl: ownership held=true state=stopped stopped_by=recovery_failed; firecracker proc absent; journalctl -u go-choir-vmctl pre-genesis count=0 post-park'
  decision:
    what: >-
      Park (hold + hibernate) the wedged platform computer; ship the missing
      IsHeld guard so the park holds. Do not delete state.
    kind: operational
    status: settled
    evidence_ref: owner directive this session (Hibernate it)
    owner_ratification_ref: not_applicable
  belief:
    believed_state: >-
      The platform computer is wedged pre-genesis and will spam until parked.
      The always-on sweeper resurrects it because the warm path skips IsHeld.
      The hold-guard fix + hold + hibernate parks it durably.
    main_uncertainty: >-
      Whether a second resurrection path exists besides the idle sweeper's
      warm call (e.g. deploy refresh or a recovery sweep). The journal test
      after parking is the discriminator.
    next_observation: >-
      Ownership state + journal 60s post-park: hibernated-staying-down =
      conjecture supported; booting/active = an unguarded warm path.
  blocker_or_risk: >-
    The hold guard must deploy before hold+hibernate, else the sweeper
    re-launches the held VM. Deploy ordering: ship guard -> hold -> hibernate.
    POST hold -> set held=true; wait for the wedged booting VM to reach a
    stoppable state; POST stop -> state=stopped, process down; confirm no
    resurrection (IsHeld now blocks the warm path) and zero pre-genesis lines.

receipts:
  - id: platform-park-landed
    boundary: terminal
    identity: main@1566bbc7
    proof_refs:
      - 'vmctl /internal/vmctl/list: universal-wire-platform state=stopped held=true; no firecracker proc'
      - 'journalctl -u go-choir-vmctl: 0 pre-genesis refusals after park'
    rollback_ref: 'POST /internal/vmctl/unhold + /internal/vmctl/resume (universal-wire-platform)'
    disposition: parked (held + stopped); durable state preserved
    landing:
      source_commit: 1566bbc7
      ci_ref: run 36100438871 (deploy step succeeded; run cancelled post-deploy)
      deploy_ref: 'Node B deploy landed deployed_commit=1566bbc7'
      environment_identity: 'choir.news build.commit=1566bbc7'
      deployed_acceptance: 'hold+stop on node-b; platform VM parked, spam stopped'
---

## Residual risk (out of scope)

The platform computer is `pre-genesis` — it has no canonical genesis on the
tape and refuses internal runs. Parking stops the noise but does not fix why
it's wedged. Root-causing the missing genesis (or whether the platform
computer should be rebuilt under a fresh genesis) is a follow-on mission,
not part of this park.
