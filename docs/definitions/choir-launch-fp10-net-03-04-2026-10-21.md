---
definition_version: 4

# Station FP10 / NET-03 and NET-04 of the launch first pass
# (choir-launch-first-pass-metamission-2026-10-21.md): a vsock control plane
# with identity bound to the VM's vsock endpoint, and the order in which the
# guest NIC can be removed. P2, red when built. Authority: the security and
# density checklist (NET-03, NET-04, and "Networking must enable, not
# starve"), and FP0's brief: write the design; vmmanager, vmctl and the VM
# image are FP8's (lane/fp8-image-jailer), so design only there.
readiness: drafted

review:
  reviewer: none yet; design only, no red diff
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-22T00:30:00Z
  source:
    canonical_ref: main d1c60ad87
    deploy_identity: unknown to this lane; FP0 reads staging identity
  worktrees:
    - path: lanes/fp10-net03-vsock
      status: clean
      class: goal_candidate
      owner: FP10 NET-03/04
      touch: goal_owned
      recovery: branch lane/fp10-net03-vsock
    - path: lane/fp8-image-jailer, lane/fp10-net01-host-fetch, lane/fp10-net02-mirrors, lane/fp10-net10-app-network (branches only, read with git show)
      status: unknown
      class: other_agent_wip
      owner: FP8, FP10 neighbours
      touch: forbidden
      recovery: leave in place

finish:
  deliver: >-
    A design for NET-03 and NET-04 that fits main and FP8: the inventory of
    TCP paths between guest and host, the vsock device and relay, how a
    host service learns which computer is calling without trusting an
    address, the vsock CVE review, snapshot behaviour, and the ordered,
    revertible steps from today's tap to a guest with no NIC, each naming
    the capabilities it touches.
  artifact: >-
    docs/problems/guest-host-identity-rides-tap-addresses-2026-10-22.md and
    this file (green); docs/design/vsock-control-plane-and-nic-removal-2026-10-22.md
    (green).
  acceptance:
    - action: FP0 reads the design against main, FP8's diff and the NET-01/02/10 designs
      proves: the inventory and the step order match the code and the neighbours
      evidence_class: review receipt
  rollback: git revert of the docs commits
  landing:
    required: false
    environment: none (docs only; the docs truth checker runs on push)
    required_receipts: [pushed_commit]

value:
  better_means: >-
    A host service knows which computer is calling from a fact the guest
    cannot forge and no firewall rule has to keep true, and the NIC can be
    removed without starving any capability.
  goodharting_would_be: >-
    Moving the bytes to vsock while identity still comes from a peer string
    a relay writes; removing the NIC before every capability has a host
    route; ticking NET-03 on a design.

homotopy:
  realism_axis: >-
    design on paper -> vsock device and relay in observe mode on one test
    computer -> one service enforced over vsock -> all control traffic on
    vsock with the tap DNAT removed -> NIC forward-dropped -> NIC removed
    on test computers -> everywhere at cold boot.

boundaries:
  mutation_class: green for this station (docs); every build step in the design is red (vmmanager, vmctl, gateway auth, guest image)
  authority_sources:
    - docs/security-density-architecture-checklist-2026-10-21.md (NET-03, NET-04, SEC-05)
    - docs/archive/reports/computer-isolation-security-review-2026-10-14.md section 4.6
    - docs/learn/03-gateway-and-providers.md section 2.4
    - docs/reports/guest-network-inventory-2026-10-21.md
    - docs/archive/reports/boundary-strength-ladder-2026-10-15.md (Vghost, VsockDrop)
  must_preserve:
    - no edit to vmmanager, vmctl, the VM image or nix while FP8 owns them
    - public internet egress stays open until NET-01 gives each path a host route
    - moving a path to the host moves where the check runs, not what is reachable
    - no boot-time resumer without a planned-restart marker
    - a hibernated computer's wake is never turned into a crash boot by a device change
  excluded:
    - building any step (each is its own red change after FP8 lands)
    - NET-01's fetch verb, NET-02's mirrors, NET-05 grants, NET-10 app policy (neighbours; this design only orders them)
  protected_surfaces: [vmctl, VM lifecycle, gateway auth, guest authority predicates, snapshot validity]

now:
  status: checkpoint_incomplete
  slice: >-
    Design written (docs/design/vsock-control-plane-and-nic-removal-2026-10-22.md);
    nothing built, by brief. Hybrid Firecracker vsock per VM; identity is
    the VM's endpoint, carried by a separate host relay (choir-vsockd) as a
    PROXY v2 record from a vmctl-written binding outside the jail; host
    authority predicates refuse any vsock caller; the gateway binds the
    bearer's computer to the endpoint's (SEC-05). Order V0-V6 (vsock) then
    N1-N7 (NIC), each per computer, off by default, cold boot only.
  source_ref: d1c60ad87 (+ FP8 ee2dcf0a3, NET-01, NET-02, NET-10 lanes, read only)
  deploy_identity: unknown to this lane
  decision: >-
    Conservative choices taken without the owner (No Blocking Asks): a
    relay unit of its own (not in vmctl: fate-sharing) over services
    listening on per-VM sockets; PROXY v2 over an HMAC header; no TCP
    upstream from the relay ever (loopback is a credential); the device is
    added at cold boot only, never on wake; host to guest moves before
    guest to host because the guest's host predicates need the NIC's
    default route.
  blocker_or_risk: >-
    V1 onward waits for FP8 to land (vmmanager, jailer layout, image).
    N4 (the console's zot and shell, inventory A9) needs an owner ruling.
    Unverified at the Firecracker 1.16.1 pin: vsock reset and uds_path
    re-bind on restore, the busy-spin fixes, ptp_kvm for time.
  next_action: >-
    FP0 reads the design. After FP8 lands, a lane takes V0 (yellow/red,
    tests first, no caller) and V1 (red, off by default) from section 9.

receipts:
  - 40b246a85 station and problem record (green)
---

# FP10 NET-03 and NET-04: vsock control plane, then no guest NIC

Problem: [guest-host-identity-rides-tap-addresses-2026-10-22.md](../problems/guest-host-identity-rides-tap-addresses-2026-10-22.md).
Design: [vsock-control-plane-and-nic-removal-2026-10-22.md](../design/vsock-control-plane-and-nic-removal-2026-10-22.md).
