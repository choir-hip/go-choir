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
  slice: DeepSeek round 2 edits made; waiting on FP0 (land cuts 1-3, cut 4 entry gate, deployed proof)
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
      docs_and_probe: d8b8139fc, 87597d1a3, d43422458 and b7eef4039 ride with cut 1 (scripts and tests only); the card commits ride with any cut
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
    re-run then (it was re-run in round 2: see round_2). Round 2 at
    b7eef4039, same filters: vmmanager, hostauth, proxy, vmctl, platform
    (347 s) all ok with -race -count=1; vet, py_compile, bash -n clean.
  round_2 (DeepSeek bunch 41; cuts 1-3 accept with edits, cut 4 accept on a gate): >-
    M1 kvm group: NixOS declares it for every system
    (nixos/modules/config/users-groups.nix at the flake's locked nixpkgs
    fe1324228: users.groups.kvm.gid = ids.gids.kvm, 302 in
    nixos/modules/misc/ids.nix, "default udev rules from systemd requires
    these"), matching Node B's getent kvm:x:302; no declaration added.
    0660 root:kvm cannot break a boot: direct VMMs are started by vmctl
    (root) with no uid drop (no SysProcAttr credential in vmmanager), and
    jailed VMMs open the jail's own node, which Firecracker's jailer mknods
    in the chroot (env.rs mknod_and_own_dev(DEV_KVM), 0600, chowned to the
    jail uid); nothing in vmmanager binds or opens the host /dev/kvm as
    another uid. Only local Nix builds with the kvm feature (nixbld) lose
    it; those go to node-a (residual below). M2: re-run on this worker
    (unprivileged user+net+mount namespace; no sudo needed) at d43422458,
    which adds 8089 dials to the real test:
    docs/evidence/host-floor-real-netfilter-2026-10-22.txt, both tests
    PASS, the chain carries seven ports, own tap :8089 reachable, another
    tap's :8089 not. M3 and M4: FP0 proof section. L1 and L2: b7eef4039.
    L3: noted, no change (AuthorizeAtMost(r, ModeOff) has no caller that
    passes it). L4: reworded here and in the problem record.
  conjecture: >-
    If the floor is enforcing and fails closed, and the guest's services,
    DNS, goproxy, the package mirror and public TCP still work, then SEC-03
    holds without starving any capability. Local evidence: fake verdict
    tests and a real kernel netfilter run
    (docs/evidence/host-floor-real-netfilter-2026-10-22.txt, seven ports;
    the 10-21 file is the six-port run).
  belief: >-
    Group 2 is right on packets (real netfilter) but its nix half is
    unevaluated here (no nix binary on the worker; CI evaluates it).
  next_action: FP0 lands cuts 1-3 in order with the mandatory --expect-guest-floor check after each; cut 4 only after its entry gate (step 6) holds.
  containment_gaps (2026-10-22, lane fp3-containment-gaps from 273d139c9): >-
    Cut 2 is live (deploy T). Its proof found 28 containment failures,
    recorded in docs/problems/host-containment-gaps-after-fp3-cut-2-2026-10-22.md.
    The candidate is 49b2f1790 (record addendum), 545d573fd (tests),
    844e0026b (fix, red) and the docs commit. It moves the four
    EnvironmentFile-only env files into /var/lib/go-choir/unit-env, hidden
    as a directory; it deletes the deploy's deploy.env scrub; it gives
    caddy, qdrant, unbound, searx, searx-init, goproxy and pkgmirror the
    host set. Cut 2 runbook step 2 is corrected (the marker with the
    credential). Local: nix test 12/12, verifier tests 12/12, contract and
    CI script tests pass (the gate test needs Linux flock), drvPath
    evaluates. Next: FP0 lands it as its own deploy and runs "Containment
    gaps after cut 2" steps 1-7. Open: problem 3, the gateway's renames of
    codex-auth.json and gateway-identities.json, waits on its probe.
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
- e52a29345, SEC-03: every install step fails the VM start. A cold boot
  tears the tap and its rules down; a restore does not start the VMM and
  falls back to a cold boot that reuses the tap and fails the same way if
  the floor still does not install (snapshot_lifecycle.go restoreFailed);
  reattach only logs. Replies-only
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

After every deploy of any cut (and every later deploy while the lane's
code is live), mandatory, Node B as root:
`python3 - --expect-guest-floor [the cut's --expect-internal/--expect-dolt flags] < scripts/verify-host-hardening.py`,
and the guest-floor section must be all PASS. Why: the floor fails closed
only at a VM start. Reattaching a running guest (every vmctl restart, so
every deploy) repairs the egress chain and the tap rules but only logs a
failed repair (guest_egress.go, hostfirewall.go reconcile), so a running
computer, the paper included, could keep unfenced egress until its next
start while the deploy otherwise looks green. This check is the only
thing that catches that; a FAIL there is a deploy failure: run
`systemctl restart go-choir-vmctl` once and re-check, then roll back.

Cut 1 (group 2: fb571bba5 .. f916265e3, with d8b8139fc and the probe/test commits 87597d1a3, d43422458, b7eef4039), after the deploy (Node B, root):

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
   "hostauth: observe: would refuse ... /internal/platform/objects".
   The same request with the marker and the credential,
   `curl -s -o /dev/null -w '%{http_code}' -H 'X-Internal-Caller: true' -H @/var/lib/go-choir/secrets/internal-service-token.header 'http://127.0.0.1:8082/internal/platform/objects?owner=x&computer=y'`,
   answers 200 and logs nothing. The credential is checked in addition to
   the marker, not instead of it: the credential alone answers 403
   "internal caller required" (corrected 2026-10-22 after deploy T's
   proof). Guest paths unchanged: a computer's story publish and capture
   writes still work (they arrive from tap addresses).
3. A cold boot of a test computer logs no "issuer: would refuse" (the
   issuer stays observe; its enforce flip is a later one-line change).
4. After a clean day: flip `PROXY_OBJECT_GRAPH_INTERNAL_AUTH` to enforce
   (one line; then the curl without the credential answers 403).
5. Cut 3, 6365f329b (Dolt locked), own deploy: gate `--expect-dolt clients`
   healthy; proof `--expect-dolt locked`, the next backup run succeeds.
6. Cut 4, de193cf8d (owner-act enforce), own deploy. Entry gate (DeepSeek
   M4, bunch 41): do not land until a positive observe window exists on
   the currently deployed head (ownerActMode = "observe"). Window W runs
   from T0 (`T0="$(date -u '+%F %T')"` before step a) to T1 (the same, after step d; UTC, the format both journalctl and the DATETIME(6) columns take), on a
   test computer (`class:test`; never the paper or the delegate computer):
   a. The proxy holds the signing key:
      `journalctl -u go-choir-proxy -b --no-pager | grep -E 'proxy: owner-act key host-owner-act-[^ ]+ loaded from '`
      prints one line, and
      `journalctl -u go-choir-proxy -b --no-pager | grep -cE 'owner-act key unusable|no owner-act key at'`
      prints 0. corpusd agrees:
      `journalctl -u go-choir-corpusd -b --no-pager | grep 'corpusd: owner acts: mode=observe'`
      names the same key id.
   b. One real owner mode write through the proxy: as the test computer's
      owner in the browser on https://choir.news, arm self-development
      accept_once for one operation (the UI's approve; it is a PUT to
      `/api/computers/<computer>/self-development/mode`, which the proxy
      signs). It answers 2xx.
   c. One accepted effect: let that operation apply and its effect be
      accepted (an `effect_accepted` event on the computer's tape whose
      authority is the mode receipt from b).
   d. One Texture owner edit through the proxy on the same computer (edit
      and save a document as the owner); it saves. (This is a control:
      owner-act does not gate Texture saves, so it must still work and log
      nothing.)
   e. Zero refusals in W:
      `journalctl -u go-choir-corpusd --since "$T0 UTC" --until "$T1 UTC" --no-pager | grep -c 'owner-act: observe: would refuse'`
      prints 0, and
      `journalctl -u go-choir-corpusd --since "$T0 UTC" --until "$T1 UTC" --no-pager | grep -cE 'owner-act: observe: (effect_accepted authority lookup failed|record admission failed)'`
      prints 0.
   f. Positive rows (the platform Dolt corpusd writes, 127.0.0.1:13306 database platform, read-only, as the app user):
      `SELECT receipt_id, act_kind, created_at FROM owner_act_receipts WHERE computer_id='<computer>' AND created_at >= '<T0>';`
      returns at least one row (b's receipt, act_kind selfdev.approve), and
      `SELECT receipt_digest, owner_act_receipt_digest, admitted_event_digest FROM self_development_mode_receipts WHERE computer_id='<computer>' AND created_at >= '<T0>';`
      returns a row with owner_act_receipt_digest and admitted_event_digest
      both set (c admitted against b).
   Land only when a-f all hold; keep the journal excerpts and query output
   as the gate artifact. Proof after the deploy: corpusd logs
   `owner acts: mode=enforce` with the key loaded; repeat b-d on the test
   computer (all succeed, no `owner-act: enforce: refused` line); a direct
   corpusd owner-mode write with the internal credential but no act is
   refused. Rollback: `ownerActMode = "observe"` and redeploy.

## Containment gaps after cut 2 (lane fp3-containment-gaps)

Problem record:
[host-containment-gaps-after-fp3-cut-2-2026-10-22.md](../problems/host-containment-gaps-after-fp3-cut-2-2026-10-22.md).
Deploy T's proof left 28 containment failures. Twelve came from the
deploy's rename of deploy.env after the units restarted. Sixteen came from
caddy, goproxy, qdrant and searx hiding nothing. The fix (844e0026b, red;
tests 545d573fd) does three things:

- moves the four EnvironmentFile-only env files into
  `/var/lib/go-choir/unit-env`, which every long-running host unit (and
  every oneshot that declares hidden paths) hides as a directory;
- deletes the deploy's scrub of deploy.env;
- gives caddy, qdrant, unbound, searx, searx-init, go-choir-goproxy and
  go-choir-pkgmirror the host set.

Local evidence: `python3 -I scripts/test_host_containment_nix.py` (the
evaluated host configuration and the activation snippet on a temp tree,
12/12), `python3 -I scripts/test_verify_host_hardening_containment.py`
(12/12), `.github/scripts/deploy-workflow-contract-test`, and the toplevel
drvPath evaluates.

FP0 deployed proof (Node B, root). Deploy D1 lands 844e0026b. Use the
verifier's flags for the stage then current; at T they were
`--expect-internal enforce --expect-dolt clients --expect-guest-floor --allow-reboot-pending`.

1. The layout, names and modes only:
   `ls -la /var/lib/go-choir/unit-env; ls -l /var/lib/go-choir/{deploy,corpus-dsn,corpusd-retrieval,sandbox-gateway-token}.env`.
   Expect:
   - unit-env is 0700 root, holding deploy.env, corpus-dsn.env and
     sandbox-gateway-token.env (each 0600) and `.imported`;
   - each old path is a link `-> unit-env/<name>`;
   - the switch output in the deploy log shows "go-choir: moved ... into
     /var/lib/go-choir/unit-env" once per file.
2. `python3 - <flags> < scripts/verify-host-hardening.py`. Expect no
   `[containment]` FAIL (T had 28). That covers:
   - "cannot reach /var/lib/go-choir/unit-env" and its four files for
     every running unit, go-choir-pkgmirror and unbound included;
   - both sockets for every unit except vmctl and the Dolt probe;
   - the four "is a link into /var/lib/go-choir/unit-env or absent" lines.

   `[modes]` passes for unit-env and its files.
3. The environments still load (key names only, never values):
   ```
   pid=$(systemctl show -p MainPID --value go-choir-proxy)
   comm -23 <(grep -v '^#' /var/lib/go-choir/unit-env/deploy.env | cut -d= -f1 | sort -u) \
            <(tr '\0' '\n' < /proc/$pid/environ | cut -d= -f1 | sort -u)
   pid=$(systemctl show -p MainPID --value go-choir-corpusd)
   comm -23 <(grep -v '^#' /var/lib/go-choir/unit-env/corpus-dsn.env | cut -d= -f1 | sort -u) \
            <(tr '\0' '\n' < /proc/$pid/environ | cut -d= -f1 | sort -u)
   ss -tnp state established '( dport = :13307 )' | grep -c corpusd
   ```
   Expect both `comm` outputs empty (every key reached the unit) and
   corpusd connected to Store B (count above 0).
4. Units restarted at the switch still serve:
   - `curl -s -o /dev/null -w '%{http_code}' https://choir.news/health` is 200 (caddy);
   - `curl -s 'http://127.0.0.1:8888/search?q=choir&format=json' | jq '.results | length'` is above 0 (searx);
   - `dig @127.0.0.1 -p 8053 example.com +short` answers (unbound);
   - `curl -s http://127.0.0.1:6333/readyz` answers (qdrant);
   - goproxy's and pkgmirror's journals since the deploy show no start
     failure;
   - every unit is active: `systemctl --failed` lists no go-choir unit,
     caddy, qdrant, searx or unbound.
5. The rename case, on D1, with the same content:
   `cd /var/lib/go-choir/unit-env && stat -c %i deploy.env && cp -p deploy.env deploy.env.next && mv deploy.env.next deploy.env && stat -c %i deploy.env`.
   The inode changes. Re-run step 2: still no `[containment]` FAIL. This
   is the operation that exposed deploy.env on T.
6. A second deploy D2 (any later deploy): record
   `stat -c %i /var/lib/go-choir/unit-env/deploy.env` before and after; it
   is unchanged (the deploy no longer touches the file). Run step 2 again:
   no `[containment]` FAIL.
7. Problem 3 probe (in the problem record): read only. Its result decides
   the gateway-state fix.

Rollback: revert the fix and redeploy, and move no files. The links keep
the old paths working for the reverted units, and a later landing relinks
without importing anything. If the files must go back to their old paths
for good, run `mv -f /var/lib/go-choir/unit-env/<name> /var/lib/go-choir/<name>`
for each name present. Then also `rm /var/lib/go-choir/unit-env/.imported`,
so the next landing imports them as on its first run (corrected after
round 2, F2). Roll back at once if any unit fails to start with a
namespace error, if corpusd is not connected to 13307, or if step 3 shows
a missing key.

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
