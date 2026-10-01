---
definition_version: 4
definition_id: choir-platform-dolt-capacity-stabilization-2026-10-01
readiness: drafted
execution_mode: mission_orchestrator

review:
  reviewer: 'agentic-consensus panel 2026-10-01 — verdict B (8/12 convergent):
    promote OOM/serial-drain substrate to a separate active top-priority
    mission; keep the parked-run repair goal open with deployed_acceptance
    pending; do not fold substrate into the landed handler seam.'
  evidence_ref: .agentic-consensus/manifest.tsv

start:
  captured_at: '2026-10-01T03:15:00Z'
  source:
    canonical_ref: main@3896a10c
    deploy_identity: 'staging https://choir.news build.commit=4a718af4'
  predecessor:
    mission: choir-parked-run-reactivation-2026-09-30
    disposition: 'blocked on this substrate — the repair is landed+deployed but
      its deployed_acceptance cannot be observed while the guest OOM-cycles.'
    evidence_ref: docs/problems/platform-dolt-oom-realization-cluster-2026-10-01.md

finish:
  deliver: 'The staging computer sustains an uptime window long enough to drain
    its re-armed wake outbox and complete a `run start` — so that (a) the
    parked-run repair''s deployed_acceptance discharges (run 362febb2
    passivated->running + packet consumed) and (b) owner task submission stops
    returning 502 during normal operation.'
  artifact: 'platform-dolt memory bounded (host memory cap and/or dolt
    buffer/cache bound) AND a measured drain-latency bound, so a single boot''s
    re-armed wake queue clears inside one uptime window.'
  acceptance:
    - action: 'on staging, observe guest uptime exceeding the prior ~20-40min
        OOM cycle over N consecutive windows with no kernel OOM-kill of
        go-choir-platform-dolt or the retained-computer firecracker'
      proves: 'the platform-dolt memory bound holds under load'
      evidence_class: deployed proof
    - action: 'submit a `choir run start` research prompt during a stable
        window and observe it reach `state != 502` + spawn a live task
        trajectory'
      proves: 'the routing-plane 502 flap is cleared by the substrate fix (or
        confirmed as an independent routing defect)'
      evidence_class: deployed proof
    - action: 'observe run 362febb2 transition passivated->running and the
        bound control 4158e48b consumed'
      proves: 'the wake drain reached the bound run — discharges the parent
        repair''s deployed_acceptance as a side effect'
      evidence_class: deployed proof
  rollback: 'the memory cap is an ops/deploy-shape change — revert the cap or
    host-capacity setting; no repo state migration. Any proxy retry/backoff is
    a separate orange change with its own rollback.'
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: 'the computer stays alive long enough to do the work it was
    given — a durable substrate bound on host memory, not a serial of reboots
    that re-arm work faster than it can drain.'
  goodharting_would_be: 'guest reports "up" while the OOM cadence continues, or
    a 502 that goes away only because submissions are retried rather than
    routed to a stably-realized computer.'

homotopy:
  realism_axis: 'capacity — from unbounded platform-dolt RSS (~28GB on a 31GB
    host, re-minting the VM every ~20-40min) to a bounded store that survives
    inside a single uptime window. Same guest, same wakes; only the floor
    stops dropping out.'

boundaries:
  mutation_class: black
  authority_sources:
    - AGENTS.md red/black ceremony + No Blocking Asks (memory-cap = permitted
      owner/ops ask)
    - docs/problems/platform-dolt-oom-realization-cluster-2026-10-01.md
    - docs/definitions/choir-jev-supervision-metamission-2026-09-29.md (OOM gate)
    - docs/problems/api-key-computer-resolution-flap-2026-10-01.md
  must_preserve:
    - 'the parked-run repair seam is frozen — do not touch handler.go for this
      mission'
    - 'do not widen drain concurrency under memory pressure without evidence'
    - 'no proxy retry/backoff that merely hides the flap'
  excluded:
    - 'the deliverable-strand handler defect (repaired, separate goal)'
    - 'the independent routing-defect possibility for the 502 (confirm first)'
  protected_surfaces:
    - 'vmctl + guest lifecycle (realization/recreate)'
    - 'platform-dolt memory/config'
    - 'proxy computer-resolution path'
    - 'wake outbox / serial drain'

conjecture:
  id: platform-dolt-memory-bound-restores-uptime
  claim: 'If platform-dolt memory is bounded (host cgroup MemoryMax and/or dolt
    buffer cap), the guest stops being OOM-killed mid-drain, uptime windows
    lengthen past the serial drain, and both the parked-run acceptance and
    owner submissions succeed.'
  test: 'deployed: N consecutive uptime windows with no OOM kill; a successful
    run start during a window; 362febb2 passivated->running observed.'
  edge: 'wrong_substrate — if dolt memory is not the OOM driver, or the 502 has
    an independent routing cause, the cap won''t clear the flap; the first act
    is the confirmation read (resolved target State/ComputerURL during a 502 +
    RSS trend), not the fix.'
  scope_if_supported: 'all staging availability blocked on the same substrate —
    parked-run acceptance, overnight research QA, self-dev proposals.'

now:
  status: blocked_on_owner
  slice: 'drafted on the panel''s B verdict. The memory cap is an ops/
    deploy-shape decision — the one place a blocking owner ask is permitted.'
  decision:
    what: 'cap platform-dolt memory so the guest survives an uptime window;
      confirm the 502''s cause before any proxy patch; sequence capacity
      changes so they don''t confound the 362febb2 observation.'
    kind: architecture
    status: proposal
    owner_ratification_ref: pending — the memory-cap/host-capacity ask
  blocker_or_risk: 'the durable fix is a deploy-shape/host-capacity tradeoff
    (cgroup MemoryMax, dolt buffer bound, or host capacity) — needs owner
    direction on which lever. Everything else proceeds without waiting.'
  next_action: 'owner decides the memory-cap lever; meanwhile watchers continue
    the 362febb2 + QA submissions opportunistically and the substrate mission
    stands ready to execute on the decision.'

receipts: []
---

## Context

The overnight QA drive surfaced that the staging guest's OOM cycle is no longer
a tolerable background condition — it is now the single blocker on two live
work streams. The metamission already names the memory cap as "the durable fix
— owner/ops, not code"; this mission operationalizes it.

This mission is deliberately separate from the landed `362febb2` handler
repair. That repair's `deployed_acceptance` stays pending on its own goal file
and discharges here as a side effect once the guest holds an uptime window.
