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
  slice: built; waiting on FP0 (bunch review, land, deployed proof)
  candidate:
    ref: lane/host-b4-b5 (base b489ed48f; rebase onto current main before freezing)
    group_2: 8b54ed2ec afd85d677 e8137ef5b (docs 8d41e22ea rides with it)
    group_3: 477910ddb 920160ea0 c1ec4e1da 71a914b12 297d89ef5 (one deploy)
    group_3_later: 70ddd1966 (Dolt locked, its own deploy), fed6bc21e (owner-act enforce, its own deploy)
    scope: [internal/vmmanager, internal/hostauth, internal/proxy, internal/platform (test + one gate), internal/vmctl/ownership.go (one line, 3777a3b47), nix/node-b.nix, nix/autoputer-vm.nix (comments), flake.nix (comments), scripts]
  conjecture: >-
    If the floor is enforcing and fails closed, and the guest's services,
    DNS, goproxy and public TCP still work, then SEC-03 holds without
    starving any capability. Local evidence: fake verdict tests and a real
    kernel netfilter run (docs/evidence/host-floor-real-netfilter-2026-10-21.txt).
  belief: >-
    Group 2 is right on packets (real netfilter) but its nix half is
    unevaluated here (no nix binary on the worker; CI evaluates it).
  next_action: FP0 runs the steps below, group 2 first.
---

# FP3 groups 2 and 3

Problem records: [host-egress-floor-fails-open-2026-10-21.md](../problems/host-egress-floor-fails-open-2026-10-21.md)
(group 2); the SEC-11 residual in
[loopback-is-a-credential-2026-10-14.md](../problems/loopback-is-a-credential-2026-10-14.md)
(with its 2026-10-21 addendum)
and [hostauth-enforce-flip-2026-10-20.md](../problems/hostauth-enforce-flip-2026-10-20.md)
(group 3).

## What was built

Group 2 (B4), one deploy:

- 8b54ed2ec, the host egress commit 61f4da20f cherry-picked (mangle
  CHOIR-GUEST-EGRESS).
- afd85d677, SEC-03: every install step fails the VM start (boot and
  restore), tap and rules torn down; reattach only logs. Replies-only
  forwarding towards guests (CHOIR-GUEST-RET), own-tap host services only
  (`--limit-iface-in`, INPUT bound to the loopback DNAT), no host 80/443,
  guarded rebuild, IPv6 drop required, DNS DNAT to a host resolver when
  `VMCTL_GUEST_DNS_PORT` is set. Tests: `host_floor_test.go`,
  `guest_egress_test.go`, `hostfirewall_test.go`; real kernel:
  `scripts/vmmanager-netfilter-test.sh`.
- e8137ef5b, SEC-09 (KSM 2 at boot, comments), SEC-10 host side (Unbound
  on 127.0.0.1:8053, `VMCTL_GUEST_DNS_PORT=8053`), NET-07 (/dev/kvm 0660;
  SMT left on by ruling), and `verify-host-hardening.py
  --expect-guest-floor`.

Group 3 (B5), one deploy: 477910ddb and 920160ea0 (REC-02, the two
quick-containment commits, cherry-picked with -x; one conflict in
`nix/node-b.nix`, adjacent let-bindings, both kept); c1ec4e1da (problem
addendum); 71a914b12 (SEC-11, route mode `observe`); 297d89ef5 (Dolt stage
"clients"). Later, one deploy each: 70ddd1966 (Dolt "locked"), fed6bc21e
(owner-act enforce).

## FP0 deployed proof

Group 2, after the deploy (Node B, root):

1. `python3 - --expect-internal enforce --expect-dolt users --expect-guest-floor < scripts/verify-host-hardening.py`:
   the guest-floor section passes (KSM run 0 or 2, /dev/kvm 660, egress
   chain without 80,443 and with --limit-iface-in, jump first in mangle
   PREROUTING, IPv6 drop, CHOIR-GUEST-RET jump, no per-tap FORWARD ACCEPT,
   unbound active on 127.0.0.1:8053).
2. `journalctl -u go-choir-vmctl --since <deploy>`: no "host networking
   for ... failed"; "installed guest chain CHOIR-GUEST-RET" once.
3. Start (cold boot) a test computer, then
   `python3 - --guest-ip <its guest IP> --other-guest-ip <another live guest> --json-out /tmp/host-floor-probe.json < scripts/host_floor_probe.py`:
   every line PASS (own services 8082-8084, 8086-8088 and the resolver on
   :53 open; host 22/80/443/13306/13307, the public address, another tap's
   host address, another guest, metadata, private, CGNAT and SMTP closed;
   public 443 open). Keep the JSON as the artifact.
4. Capabilities on that computer: a research Read of an arbitrary public
   URL, a web search, a source poll (RSS) and a Mail draft save all work;
   the paper keeps publishing (checklist "Networking must enable" 1-3, 5, 6).
5. A computer that was running across the deploy (reattached) is still
   ready and reachable; its replies pass (CHOIR-GUEST-RET).
Rollback: revert the three commits and redeploy, then the host lines in
afd85d677's message (vmctl restarts reinstall the older shapes).

Group 3, after the deploy:

1. `verify-host-hardening.py --expect-internal enforce --expect-dolt clients`:
   the Dolt section passes; corpusd, checkpointd and vmctl active, no
   dbcred or "Access denied" line; the issuer credential is held only by
   vmctl and corpusd (3777a3b47's check); host units no longer see PID 1's
   socket (9f337a3ac's check).
2. SEC-11: on Node B, `curl -s -o /dev/null -w '%{http_code}' -H 'X-Internal-Caller: true' 'http://127.0.0.1:8082/internal/platform/objects?owner=x&computer=y'`
   answers (observe allows) and the proxy journal shows
   "hostauth: observe: would refuse ... /internal/platform/objects";
   the same with `-H @/var/lib/go-choir/secrets/internal-service-token.header`
   logs nothing. Guest paths unchanged: a computer's story publish and
   capture writes still work (they arrive from tap addresses).
3. A cold boot of a test computer logs no "issuer: would refuse" (the
   issuer stays observe; its enforce flip is a later one-line change).
4. After a clean day: flip `PROXY_OBJECT_GRAPH_INTERNAL_AUTH` to enforce
   (one line; then the curl without the credential answers 403).
5. 70ddd1966 (Dolt locked), own deploy: gate `--expect-dolt clients`
   healthy; proof `--expect-dolt locked`, the next backup run succeeds.
6. fed6bc21e (owner-act enforce), own deploy: gate is a journal window
   with real owner acts and no "owner-act: observe: would refuse"; proof
   in the commit message.

## Residuals and owner questions

- NET-07: SMT and core scheduling stay on (owner ruling); owed before
  strangers share the host (DEC-06).
- SEC-10 guest side: FP2 adds `DNS=$gateway_ip` to the
  `go-choir-configure-network` unit (the gateway is the tap host address,
  already parsed there); then public UDP 53 leaves the floor (one line in
  `guestEgressRuleSpecs`).
- Guests reattached across the group 2 deploy get the DNS DNAT at their
  next start (reattach reconciles filter rules, not nat).
- Public TCP egress stays open until NET-01.
- `internal/vmctl/ownership.go` carries one added line from 3777a3b47
  (`hostauth.MarkIssuer` in `issueComputerCredentialEnvelopeOnce`); the
  group 1 lane edits that file, so expect a trivial rebase there.
- Local Nix builds needing `/dev/kvm` (the kvm system feature) fall to
  node-a; a local fallback would fail until nixbld is in `kvm`.
- No owner question blocks; owner-act enforce runs on the §27 ruling.
