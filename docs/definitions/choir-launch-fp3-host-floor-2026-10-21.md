---
definition_version: 4

# FP3 of the launch first pass (choir-launch-first-pass-metamission-2026-10-21.md):
# the host floor. Group 1 is the two vmctl defects behind the 2026-10-21
# Autopaper outage; groups 2 and 3 are batches B4 and B5 of the launch
# checklist's batch plan. Each group is its own commit set, deploy and
# rollback unit.
readiness: drafted

review:
  reviewer: none yet; FP0 sends each red group to a panel on its frozen diff
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-21T18:10:00Z
  source:
    canonical_ref: origin/main f399a2e8c (lane base)
    deploy_identity: >-
      staging serves f2c4a3ec3 or later; the Autopaper computer (vm-1669020a,
      critical_protected) was down 16:20 to 17:10 UTC on 2026-10-21 and was
      resumed by hand through /internal/vmctl/resolve.
  worktrees:
    - path: scratchpad/lanes/host-vmctl-b4-b5
      status: clean at start
      class: goal_candidate
      owner: FP3 worker
      touch: goal_owned
      recovery: branch lane/host-vmctl-b4-b5
    - path: /Users/wiz/go-choir
      status: dirty (untracked sourcecycled, ograph-bench, cmd/desktop/frontend/)
      class: other_agent_wip
      touch: forbidden
      recovery: leave in place

finish:
  deliver: >-
    A deploy always reaches an always-on computer that a bad release broke,
    and never leaves it down for a day (group 1); guests cannot reach other
    tenants, private ranges or cloud metadata, and the host's CPU and KSM
    posture is set for strangers (group 2); host services refuse unauthorized
    callers and passwordless database access while owner acts still work
    (group 3).
  artifact: >-
    Three commit groups on lane/host-vmctl-b4-b5, each: a problem record, a
    failing test first, the fix, and exact deployed-proof steps for FP0. Group
    1 lands first and alone.
  acceptance:
    - action: group 1, go test ./internal/vmctl focused on the new tests plus the whole package through scratchpad/bin/heavy
      proves: a deploy of a new target re-arms exactly one retry per computer, the sweep alone is still bounded to one per 24 hours, and a rescue never boots over a live guest
      evidence_class: unit
    - action: group 1 deployed, FP0 follows the proof steps in the lane report (stop a test computer by wedge, deploy a new target, watch it boot with no owner act)
      proves: the rollback lands on a failure-stopped protected computer
      evidence_class: deployed proof
    - action: group 2, tests in internal/vmmanager plus a nix evaluation of the host module, then FP0's cross-tenant, private-range and metadata probes from a staging guest and the SDC networking capability matrix
      proves: the wrong destinations are refused and research, search, ingestion, engineering and Mail still work
      evidence_class: deployed proof
    - action: group 3, tests in internal/proxy and the hostauth gates, then FP0's unauthorized-caller and passwordless-Dolt probes and an owner act
      proves: unauthorized callers and passwordless access are refused and owner acts still succeed
      evidence_class: deployed proof
  rollback: >-
    Each group is one git revert of its commits, landed by the Landing Loop.
    Group 1 needs no data repair (the ownership field is additive). Group 2's
    firewall rules are rebuilt idempotently at vmctl start. Group 3's nix
    enforce flags have an observe setting named in its commit.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the time an always-on computer stays down after a bad release,
    and the number of host services and network paths a compromised guest can
    use, while preserving the crash-loop bound (a bounded number of spawns per
    external event), the restart rules, and the networking capabilities the
    owner listed.
  goodharting_would_be: >-
    Removing the retry bound so the paper "always comes back"; a deploy that
    kills a working guest; firewall rules that pass a unit test and are never
    installed (a rule that fails to install must fail the start); a database
    password set but the passwordless root user still accepted.

homotopy:
  realism_axis: >-
    Unit test with fakes, then the fake iptables/registry harnesses, then a
    frozen reviewed candidate, then a staging deploy with a probe from a real
    guest, then a day of the paper's load.

boundaries:
  mutation_class: red (vmctl; host firewall; host service authority)
  authority_sources:
    - owner 2026-10-21 (the metamission quotes), CLAUDE.md "Restarts End Work (Crash) Or Resume It (Planned Update)"
    - docs/restart-subsystem-design-2026-10-13.md §1 (bounded spawns per external event) and §9
    - docs/security-density-architecture-checklist-2026-10-21.md items SEC-03, SEC-09, SEC-10, NET-07, REC-02, SEC-11, SEC-12
  must_preserve:
    - A crash restart never resumes work; a deploy-initiated boot of a failure-stopped computer is a crash boot with no planned-restart marker.
    - At most one automatic boot per (computer, deploy target) after a failure stop, and the 24-hour backstop.
    - A live guest that accepts connections is never stopped by a deploy; only the wedge detector's evidence ends a live process.
    - Public internet egress stays open (the SDC "Networking must enable, not starve" rules).
    - No ssh, no staging or production calls, no secrets read, no push, no touching other worktrees (lane rules).
  excluded:
    - SEC-04 (the jailer), NET-01/04 (host fetch, removing the guest NIC), SEC-05..SEC-08, anything in B6 to B9.
    - Persisting the wedge clock (restart ledger L1b).
  protected_surfaces: [vmctl, host firewall, host service authority, deployment routing (ci.yml deploy refresh)]

now:
  status: working
  slice: group 1 built and tested; frozen for FP0 review, then groups 2 and 3
  source_ref: f399a2e8c
  deploy_identity: unchanged
  candidate:
    id: none
    state: none
    ref: none
    base: f399a2e8c
    digest: none
    scope: []
  conjecture:
    id: C-FP3-event-grant
    claim: >-
      Re-arming the single automatic retry once per distinct deploy target
      (and refunding it on a refusal that spawned nothing) revives a
      failure-stopped always-on computer after a rollback without opening a
      crash loop.
    test: >-
      Unit: N posts of one target spawn once; K distinct targets spawn at most
      K times, each after a fresh failure stop; the sweep alone spawns once per
      24 hours. Deployed: FP0 wedge-stops a test computer, deploys a new target
      and sees one boot.
    edge: missing_oracle (the loop bound is argued from counters; no fault-injection matrix on a minted computer yet)
    delta_o: restart design §5 fault-injection matrix on a test computer
    scope_if_supported: always-on (premium_always_on primary, critical_protected) computers on Node B
    status: active
    evidence_refs: []
  decision:
    what: >-
      The deploy event is the target string of POST /internal/vmctl/deploy-refresh;
      the wedge detector stays the only thing that ends a live guest; the 24 h
      backstop is kept.
    kind: operational
    status: settled
    evidence_ref: docs/problems/vmctl-failed-protected-computer-waits-a-day-and-deploy-cannot-reach-it-2026-10-21.md
    owner_ratification_ref: not_applicable
  belief:
    believed_state: >-
      The code paths in the problem record are the cause; the trace of the
      outage (vmctl journal) was read by the orchestrator, not by this lane.
    main_uncertainty: >-
      Why the paper was outside the registry at 16:53 (route guard refusal, the
      StoppedBy shape, or a failed ReattachVM); the fix does not depend on which.
    next_observation: group 1's unit tests, then FP0's deployed proof.
  blocker_or_risk: >-
    Group 2 edits the host firewall, which fails a VM start if a rule does not
    install; the fake-iptables tests must prove the failure path.
  next_action: report group 1 to FP0; groups 2 and 3 after FP0 sends the lane back.

receipts: []
---

# FP3: the host floor

Three groups, landed one per deploy, in this order. The bodies of the groups'
designs live in their problem records; this file is the authority for the
finish line and the current action.

1. **vmctl** (red): `docs/problems/vmctl-failed-protected-computer-waits-a-day-and-deploy-cannot-reach-it-2026-10-21.md`.
2. **B4, the host network floor** (red): SDC SEC-03, SEC-09, SEC-10, NET-07.
3. **B5, host-service authority** (red): SDC REC-02, SEC-11, SEC-12.
