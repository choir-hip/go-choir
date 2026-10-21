---
definition_version: 4

# Station FP10 of the launch first pass
# (choir-launch-first-pass-metamission-2026-10-21.md): the second half of
# batch B8, NET-01's host fetch service. Closing general guest egress
# (NET-04) is not this station.
readiness: drafted

review:
  reviewer: none yet; FP0 sends the red commits to the batch panel on the frozen diff
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-21T23:30:00Z
  source:
    canonical_ref: main 53a691be6 (FP6 batch 1 and the guest network inventory on the branch base)
    deploy_identity: unknown to this lane; FP0 reads staging identity
  worktrees:
    - path: /Users/wiz/Dev/lanes/fp10-net01-host-fetch
      status: clean
      class: goal_candidate
      owner: FP10
      touch: goal_owned
      recovery: branch lane/fp10-net01-host-fetch

finish:
  deliver: >-
    A granted computer's research Read, content import, podcast and YouTube
    fetches and source polls run their network hop on the host, behind one
    gateway verb with the address guard, budgets and a log line per fetch;
    any public URL stays readable; ungranted computers behave as today.
  artifact: >-
    Commits on lane/fp10-net01-host-fetch: this file and the problem record
    (green); the host verb POST /provider/v1/fetch and its executor (red);
    the guest transport and the per-computer grant written by vmctl (red);
    podcast search through the guarded client (orange); FP0 proof steps.
  acceptance:
    - action: go test ./internal/hostfetch, ./internal/sourcefetch, ./internal/gateway -run Fetch, ./internal/vmmanager -run HostFetch
      proves: the guard refuses private, loopback, metadata and host-own answers on the host; one hop per call; caps and budgets hold; the grant reaches only named computers
      evidence_class: local test
    - action: FP0 proof steps in the body (grant one test computer, cold boot, Read, import, poll, podcast search; gateway log lines; ungranted computer unchanged)
      proves: capabilities 1 and 3 pass through the host path on staging
      evidence_class: deployed proof
  rollback: >-
    Empty CHOIR_HOST_FETCH_COMPUTERS and cold-boot the computer (the guest
    fetches itself again); git revert of the commits removes the verb.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    The address guard and the fetch record live on the host, where a
    compromised guest cannot skip them, with no capability lost.
  goodharting_would_be: >-
    An allowlist; a host route that follows redirects without re-checking;
    a verb that a granted guest silently bypasses by falling back to direct
    egress; a budget so tight the paper's polls fail.

homotopy:
  realism_axis: >-
    Unit tests with an injected resolver and dialer, then one granted test
    computer on staging, then the paper's computer under its real poll load.

boundaries:
  mutation_class: green (docs); orange (podcast search client); red (gateway route, guest transport, vmctl credential-disk grant)
  authority_sources:
    - SDC NET-01 and "Networking must enable, not starve" (owner, 2026-10-21)
    - docs/reports/guest-network-inventory-2026-10-21.md
    - FP0's station brief (selectable per computer, off by default)
  must_preserve:
    - No allowlist; arbitrary public URLs readable.
    - Off by default; an ungranted computer's fetch path is unchanged.
    - A granted computer never falls back to direct egress (fails closed).
    - No guest image change; no firewall change; no store schema change.
  excluded:
    - NET-04 (closing the guest NIC), the DNS and time routes, the A9 console decision, MTProto.
  protected_surfaces: [gateway, vmctl credential disk, guest egress]

now:
  status: working
  slice: problem record and station file
  source_ref: 53a691be6
  deploy_identity: unknown to this lane
  next_action: tests for the host executor and guest transport, then code
---

# FP10: NET-01's host fetch service

Problem: [guest-fetches-public-internet-itself-2026-10-21.md](../problems/guest-fetches-public-internet-itself-2026-10-21.md).
