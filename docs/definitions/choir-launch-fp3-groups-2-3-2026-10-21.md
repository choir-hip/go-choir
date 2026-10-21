---
definition_version: 4

# FP3 groups 2 and 3: the host floor. A station section of the launch first
# pass (docs/definitions/choir-launch-first-pass-metamission-2026-10-21.md,
# station FP3), built on lane/host-b4-b5 while another worker repairs
# group 1 on lane/host-vmctl-g1-r3. Authority: the metamission's
# now.slice and FP0's assignment of 2026-10-21 (the items below, from
# docs/security-density-architecture-checklist-2026-10-21.md).
readiness: executable

review:
  reviewer: none (FP0 sends the frozen candidates to the bunch panel)
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  source_ref: b489ed48f (main)
  deploy_identity: c893a956a serving (per the metamission's now card)
  worktree: lanes/host-b4-b5, branch lane/host-b4-b5, clean at start
  facts:
    - The host egress commit 61f4da20f is on origin/land/desk-cell-confinement, unlanded.
    - origin/security/quick-containment holds 9f337a3ac and 3777a3b47, unlanded (REC-02).
    - Tap rule failures only log (manager.go setupHostNetworking); see docs/problems/host-egress-floor-fails-open-2026-10-21.md.
    - The proxy object-graph route trusts loopback plus X-Internal-Caller (loopback-is-a-credential residual, SEC-11).
    - Dolt auth is at stage "users"; owner-act is in observe (nix/node-b.nix).

finish:
  artifact: >-
    Two landable commit groups on lane/host-b4-b5. Group 2 (B4): SEC-03
    host egress floor, fail-closed install; SEC-09 KSM off and comments;
    SEC-10 host resolver for guests (host side); NET-07 /dev/kvm 0660
    (SMT recorded, not changed). Group 3 (B5): REC-02 cherry-picks;
    SEC-11 proxy route behind hostauth with no production TEST-NET-1;
    SEC-12 Dolt stages 2 and 3 and owner-act enforce, one commit each.
  proof: >-
    Local: focused go tests (fake iptables proves the failure path).
    Deployed (FP0): the probe steps in "FP0 deployed proof" below.
  rollback: >-
    git revert of the group's commits; per-commit host steps are in each
    commit message. The floor has no mode (revert it). Mode rollbacks:
    the proxy route's mode back to observe;
    doltAuthStage back one stage; ownerActMode = "observe".
  landing: FP0 alone; group 2 and group 3 are separate deploys; Dolt "locked" is a third deploy.

value:
  better_means: a guest that is compromised reaches only its own host services and the public internet, and host internal authority needs a credential, with every capability in the checklist's "Networking must enable" list still working.
  goodharting_would_be: rules that pass fake tests but drop DNS, Mail, goproxy or the gateway on staging; or flips landed without the clean-window evidence.

homotopy:
  realism_axis: fake iptables -> FP0 deploy on staging -> probes from a staging guest -> a day of real traffic with no would-refuse lines.

boundaries:
  mutation_class: red (vmctl host firewall, gateway-adjacent auth, Dolt auth)
  must_preserve:
    - The metamission's must_preserve list.
    - Not edited here: internal/vmctl/ownership.go (except the one line 3777a3b47 adds), deploy_refresh*.go, wedge.go, the FP3 station file, the guest side of nix/autoputer-vm.nix (FP2; this lane changes comments there only).
    - Reattach of a running guest never stops it for a rule failure.
  protected_surfaces: [vmctl, auth and session, deployment routing]

now:
  status: working
  slice: group 2 (B4)
  candidate: none yet
  conjecture: >-
    If the floor is enforcing and fails closed, and the guest's services,
    DNS, goproxy and public TCP still work, then SEC-03 holds without
    starving any capability.
  next_action: build group 2 commits.
---

# FP3 groups 2 and 3

Problem records: [host-egress-floor-fails-open-2026-10-21.md](../problems/host-egress-floor-fails-open-2026-10-21.md)
(group 2); the SEC-11 residual in
[loopback-is-a-credential-2026-10-14.md](../problems/loopback-is-a-credential-2026-10-14.md)
and [hostauth-enforce-flip-2026-10-20.md](../problems/hostauth-enforce-flip-2026-10-20.md)
(group 3).

## FP0 deployed proof

Filled in as each group is built.
