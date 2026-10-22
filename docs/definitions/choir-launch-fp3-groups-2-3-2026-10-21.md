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
  slice: rebased onto main 5efe46672 as lane/host-b4-b5-on-main; waiting on FP0 (DeepSeek review, land, deployed proof)
  candidate:
    ref: lane/host-b4-b5-on-main (base origin/main 5efe46672; the cherry-pick replay of lane/host-b4-b5 2ccd33847)
    reviews: flash (accept with edits) and Devin (accept) at 1d1dc2c00; DeepSeek not yet
    sha_map (first rebase 1d1dc2c00-era cited -> 2ccd33847 -> this replay): >-
      8d41e22ea -> 0f311a7fb -> fb571bba5; 8b54ed2ec -> f7fdde610 -> 039e74655;
      afd85d677 -> 6bd661ef8 -> e52a29345; e8137ef5b -> febd9aa05 -> f916265e3;
      477910ddb -> 285010bc3 -> 239a23c02; 920160ea0 -> cbfe73568 -> 655bd9eae;
      c1ec4e1da -> f9ba3b3a2 -> c5d08d8a0; 71a914b12 -> add73f263 -> ec28ac0db;
      297d89ef5 -> ae6580d25 -> 93205a30d; 70ddd1966 -> 85844db13 -> 6365f329b;
      fed6bc21e -> da1fbc9e1 -> de193cf8d; 2ccd33847 -> d8b8139fc
    cuts (land in this order, one deploy each):
      cut_1_group_2: fb571bba5 (docs) .. f916265e3 = fb571bba5 039e74655 e52a29345 f916265e3
      cut_2_group_3: 239a23c02 .. 93205a30d = 239a23c02 655bd9eae c5d08d8a0 ec28ac0db 93205a30d
      cut_3_dolt_locked: 6365f329b .. 6365f329b
      cut_4_owner_act_enforce: de193cf8d .. de193cf8d
      docs_and_probe: d8b8139fc and the 8089 commit 87597d1a3 ride with cut 1 (scripts and tests only); this card's commit rides with any cut
    scope: [internal/vmmanager, internal/hostauth, internal/proxy, internal/platform (test + one gate), internal/vmctl/ownership.go (one line, 3777a3b47), nix/node-b.nix, nix/autoputer-vm.nix (comments), flake.nix (comments), scripts]
  replay_check: >-
    Eight files changed on both sides since the lane base 7c61ce078
    (flake.nix, vmctl/ownership.go, vmmanager config.go, confinement_test.go,
    manager.go, manager_test.go, snapshot_lifecycle.go, nix/node-b.nix).
    Each replayed hunk keeps its meaning: main's FP8 changes (vmmFor /
    recordLaunchedVMM, jail .shared copies under vm-state/.jail, per-computer
    selectors) do not touch prepareVMNetwork, whose new error return the
    lane threads through every caller (boot jailed and direct, both
    snapshot-restore branches); the jail .shared copy reads guest images
    and writes under vm-state, so vmctl's read-only /var/lib/go-choir/guest
    (239a23c02) still holds; main's ownership.go edits (RES-09 warmness
    order) are in other functions from the one MarkIssuer line; flake.nix
    lane hunks are comments only. NET-02's pkgmirror unit holds no secret
    and needs no hidden-path change.
  port_8089: >-
    Main's NET-02 put 8089 (go-choir-pkgmirror, 127.0.0.1:8089) on
    tapReachableHostServicePorts, the one list the floor's egress RETURN,
    per-tap INPUT ACCEPT and DNAT all read, so the floor's runtime already
    admits a guest to its own tap's :8089 and nothing else; the nix side has
    no floor port list (the public firewall does not open 8089; the
    listener is loopback). The gap was the deployed proof: host_floor_probe.py
    did not expect 8089 and verify-host-hardening.py did not check the
    egress service ports. Fixed probe/test only (yellow): the probe expects
    own pkgmirror :8089 open (owner NET-02), verify-host-hardening.py
    carries TAP_SERVICE_PORTS and checks the egress chain's service RETURN
    against it, and TestHostFloorScriptsExpectEveryTapServicePort pins 8089
    (INPUT verdict, DNAT), the 15-port multiport limit, and both scripts
    against the Go list. No other port opened on main since the lane base
    (node-b.nix firewall and -listen lines: only 127.0.0.1:8089).
  local_evidence: >-
    On the replay (87597d1a3), GOMAXPROCS=2 GOFLAGS=-p=2 go test -race:
    vmmanager -run 'Floor|Egress|Tap|HostNet|HostFirewall|Firewall|Confine|Jail|Snapshot|Reattach|Prepare|GuestDNS|Config|NAT|Sweep'
    ok (88 tests); hostauth ok; proxy -run 'ObjectGraph|GuestAuthority|Internal|Loopback|Config'
    ok; platform ok (370 s); vmctl -run 'Credential|Issue|Ownership|Envelope'
    ok; go build ./... and go vet on the five packages clean;
    python3 -m py_compile on both probe scripts and bash -n on
    vmmanager-netfilter-test.sh clean. The real-netfilter script was not
    re-run (needs root netns).
  conjecture: >-
    If the floor is enforcing and fails closed, and the guest's services,
    DNS, goproxy, the package mirror and public TCP still work, then SEC-03
    holds without starving any capability. Local evidence: fake verdict
    tests and a real kernel netfilter run
    (docs/evidence/host-floor-real-netfilter-2026-10-21.txt, taken before
    8089 joined the list).
  belief: >-
    Group 2 is right on packets (real netfilter) but its nix half is
    unevaluated here (no nix binary on the worker; CI evaluates it).
  next_action: FP0 sends the replay to DeepSeek, then lands the cuts in order and runs the steps below, cut 1 first.
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

- 039e74655, the host egress commit 61f4da20f cherry-picked (mangle
  CHOIR-GUEST-EGRESS).
- e52a29345, SEC-03: every install step fails the VM start (boot and
  restore), tap and rules torn down; reattach only logs. Replies-only
  forwarding towards guests (CHOIR-GUEST-RET), own-tap host services only
  (`--limit-iface-in`, INPUT bound to the loopback DNAT), no host 80/443,
  guarded rebuild, IPv6 drop required, DNS DNAT to a host resolver when
  `VMCTL_GUEST_DNS_PORT` is set. Tests: `host_floor_test.go`,
  `guest_egress_test.go`, `hostfirewall_test.go`; real kernel:
  `scripts/vmmanager-netfilter-test.sh`.
- f916265e3, SEC-09 (KSM 2 at boot, comments), SEC-10 host side (Unbound
  on 127.0.0.1:8053, `VMCTL_GUEST_DNS_PORT=8053`), NET-07 (/dev/kvm 0660;
  SMT left on by ruling), and `verify-host-hardening.py
  --expect-guest-floor`.

Group 3 (B5), one deploy: 239a23c02 and 655bd9eae (REC-02, the two
quick-containment commits, cherry-picked with -x; one conflict in
`nix/node-b.nix`, adjacent let-bindings, both kept); c5d08d8a0 (problem
addendum); ec28ac0db (SEC-11, route mode `observe`); 93205a30d (Dolt stage
"clients"). Later, one deploy each: 6365f329b (Dolt "locked"), de193cf8d
(owner-act enforce).

## FP0 deployed proof

Cut 1 (group 2: fb571bba5 .. f916265e3, with d8b8139fc and the 8089 probe commit 87597d1a3), after the deploy (Node B, root):

1. `python3 - --expect-internal enforce --expect-dolt users --expect-guest-floor < scripts/verify-host-hardening.py`:
   the guest-floor section passes (KSM run 0 or 2, /dev/kvm 660, egress
   chain without 80,443 and with --limit-iface-in, egress service ports
   exactly 8082,8083,8084,8086,8087,8088,8089, jump first in mangle
   PREROUTING, IPv6 drop, CHOIR-GUEST-RET jump, no per-tap FORWARD ACCEPT,
   unbound active on 127.0.0.1:8053).
2. `journalctl -u go-choir-vmctl --since <deploy>`: no "host networking
   for ... failed"; "installed guest chain CHOIR-GUEST-RET" once.
3. Start (cold boot) a test computer, then
   `python3 - --guest-ip <its guest IP> --other-guest-ip <another live guest> --json-out /tmp/host-floor-probe.json < scripts/host_floor_probe.py`:
   every line PASS (own services 8082-8084, 8086-8088, the package
   mirror 8089 (NET-02; it connects with every ecosystem off) and the
   resolver on :53 open; host 22/80/443/13306/13307, the public address, another tap's
   host address, another guest, metadata, private, CGNAT and SMTP closed;
   public 443 open). Keep the JSON as the artifact.
4. Capabilities on that computer: a research Read of an arbitrary public
   URL, a web search, a source poll (RSS) and a Mail draft save all work;
   the paper keeps publishing (checklist "Networking must enable" 1-3, 5, 6).
5. A computer that was running across the deploy (reattached) is still
   ready and reachable; its replies pass (CHOIR-GUEST-RET).
Rollback: revert the three code commits and redeploy, then the host lines in
e52a29345's message (vmctl restarts reinstall the older shapes).

Cut 2 (group 3: 239a23c02 .. 93205a30d), after the deploy:

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
5. Cut 3, 6365f329b (Dolt locked), own deploy: gate `--expect-dolt clients`
   healthy; proof `--expect-dolt locked`, the next backup run succeeds.
6. Cut 4, de193cf8d (owner-act enforce), own deploy: gate is a journal window
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
