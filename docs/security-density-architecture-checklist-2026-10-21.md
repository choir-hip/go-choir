# Security, Density and Architecture Checklist

Status 2026-10-30: the Dolt-era items (line 175, SEC-12, RES-18) are retargeted by the cutover: read them against host PostgreSQL and guest SQLite. Design source for the new data system: [the data system frame](design/choir-data-system-frame-2026-10-30.md).

Date: 2026-10-21. Status: **living checklist**; tick items as they land.
Mutation class of this document: green (docs only). Each item names its own
class. Base: `origin/main` at 8f71f9c64, every origin branch compared with
`git log origin/main..origin/<branch>`.

Owner direction (2026-10-21, [owner notes §36](reports/owner-notes-2026-10-14.md)):
make the isolation, egress and memory review a comprehensive checklist,
say for each item whether work or plans already exist, update those plans,
and land the security and resource fixes as soon as possible. The goal is a
secure, lightweight, dense multi-agent system per user, with many agents
per user, app-agent desks, and RLM trees that can scale across processes,
VMs and later hosts.

## How to use it

- **Tick rule.** A runtime item is ticked only with its pushed SHA, the
  staging deploy that carried it, and an evidence path (Landing Loop,
  CLAUDE.md). A docs item is ticked with its SHA. Write them on the item:
  `Done: <sha>, deploy <run>, evidence <path>`.
- **Priority.** **P0** before launch. **P1** before strangers share a host,
  which on Node B is launch day. **P2** architecture, decided item by item.
  Owner rule (2026-10-21, owner notes §37): every security and density item
  that can land before launch does, so P0 and P1 both land before launch,
  and a P2 item is post-launch only when it is architecture, not unfinished
  hardening. This answers DEC-04.
- **Each item says:** what is true on main now, existing branch work, the
  plans that cover it, and what in those plans was stale or missing (the
  "plan update" line says whether this change already fixed it).
- **IDs are stable.** Add new items at the end of a section; never renumber.

## Order

**Superseded (2026-10-21):** the order is now the batch plan in the
[launch checklist](launch-checklist-2026-10-21.md#batch-plan), ordered by
what speeds up the next batch (owner notes §37) and reviewed by a
three-model panel. One ordered list lives there; this checklist stays the
authority for each item's facts and its tick. The earlier P0 order, kept
for reference (one red deploy at a time):

1. REC-01, then SEC-01 (recover and land desk-cell confinement).
2. SEC-02 (the daemon checks the role on every frame action).
3. SEC-03 (host egress floor).
4. RES-01 and RES-02 (in-guest cgroup tree; desk workers capped).
5. REC-02 (land `security/quick-containment`).
6. RES-05, then SEC-04 (keep store sharing under the jailer, then turn the
   jailer on).
7. SEC-05, SEC-06, SEC-07, SEC-11 (gateway bearer binding, tunnel identity,
   unguarded fetches, proxy route).
8. SEC-08, SEC-09 (delegate posture, KSM off).
9. ARCH-01 (doctrine names the app-desk tier, docs only, any time).

## Networking must enable, not starve (owner, 2026-10-21)

The owner: research must read the contents of arbitrary URLs safely and
search the web; source ingestion, engineering's network grants and future
app agents' ingress and egress must keep working. Rules for every network
item below:

- **The floor blocks the wrong destinations, not the internet.** SEC-03
  drops guest traffic to host services other than the guest's own, to other
  guests, and to private, link-local, CGNAT and cloud-metadata addresses.
  Public internet egress stays open until NET-01 gives each path a host
  route.
- **No allowlist for research reads.** Research may read any public URL.
  "Safely" means the address guard (resolve, refuse non-public answers, dial
  the checked address, re-check every redirect), size and time limits, and
  page content treated as untrusted data. Allowlists are for engineering
  toolchains, where the destinations are known.
- **Moving a path to the host moves where the check runs, not what is
  reachable.** NET-01 and NET-04 change the route, never the capability.
- **Capabilities that must keep working at every step.** An item that
  touches networking names which of these it affects and proves them on
  staging before it is ticked:
  1. research reads an arbitrary public URL (Read, fetch_url);
  2. web search through the gateway;
  3. source ingestion: RSS, Telegram (MTProto, not HTTP), GDELT, keyed
     source APIs, podcasts, YouTube, content import;
  4. Web Lens live inspection (browser control, Obscura);
  5. engineering: the L2 ecosystem proxy to package registries and source
     hosts, the Go module proxy, the delegate's model API, and open egress
     by grant when L3/L4 exist;
  6. Mail in and out through maild;
  7. app agents' declared ingress and egress (NET-10).

## 0. Recover and land existing work

- [x] **REC-01 · P0 · red — Recover `fix/desk-cell-confinement`**
  - Done 2026-10-21: recovered from the owner's iMac (`~/go-choir`) and
    pushed by a session there with the owner's approval:
    `origin/fix/desk-cell-confinement` at fedb7656c (11d257d53 problem doc,
    790d5e2c1 desk-cell confinement, 0400a37dd host guest-egress policy,
    fedb7656c repair record; 973 commits behind main) and
    `origin/land/desk-cell-confinement` at 9cee80a2d (the same three
    commits rebased onto 133de45fa as 07abf0351, 61f4da20f, 398f4ba71, plus
    432cbede0 landing record and 9cee80a2d owner-only guest isolation
    self-test and staging probe; 831 behind). Backup bundle:
    iCloud Drive "Choir Backups/desk-cell-confinement-2026-10-21.bundle"
    (verified). Landing is SEC-01 and SEC-03: start from `land/`.
  - Before: the branch was local to the owner's machine
    (`docs/archive/reports/selfdev-readiness-2026-10-15.md:569-571`); paused 10-15
    as "needs a rebase and Linux Docker tests"
    (`docs/archive/reports/handoff-2026-10-15.md:372`).
  - Contents per docs: a separate unprivileged user per desk,
    `no_new_privs`, Landlock V5, an empty network namespace, seccomp
    `DeskCellSeccompFilter` (no socket/connect/exec/fork/ptrace/mount/bpf/
    io_uring; `clone` only with `CLONE_THREAD`), and the host-side guest
    egress (mangle) policy (`docs/archive/reports/boundary-strength-ladder-2026-10-15.md:188-205`;
    `selfdev-readiness-2026-10-15.md:335-350`).
  - Next: SEC-01 and SEC-03 land it, rebased from `land/` onto current
    main.
- [ ] **REC-02 · P0 · red — Land `origin/security/quick-containment`**
  - Commits 9f337a3ac (PID 1 and D-Bus sockets and EnvironmentFile-only env
    files hidden from host units; vmctl read-only on
    `/var/lib/go-choir/{services,guest}`) and 3777a3b47 (a
    `choir-issuer-token` only vmctl and corpusd hold gates credential
    issuance, observe first). 173 commits behind main; merges cleanly.
  - Plans: [host-compromise-blast-radius-2026-10-20.md](problems/host-compromise-blast-radius-2026-10-20.md)
    (its stated residual: vmctl is still root-equivalent through PID 1's
    socket).
  - Plan update owed: no doc on main names the branch; ACTIVE omits it
    (fixed in ACTIVE by this change).

## 1. Security

- [x] **SEC-01 · P0 · red — Confine desk workers**
  - Done: FP2 batch 1, 564840fcf with CI fix 7c61ce078, deploy CI 39702960215 (2026-10-22 01:12 UTC), evidence docs/evidence/fp2-desk-boundary-20261022.json (desk cells ok in mode required; Seccomp 2, NoNewPrivs 1 and socket EPERM in every desk; cross-desk canary reads refused under the guest's root spawner). Residual: the per-desk memory budget proof is RES-02's step 4; the non-root daemon is SEC-15.
  - Target: every desk-session worker runs as a non-root identity with
    `no_new_privs`, Landlock (read-only system paths, its own scratch
    only), seccomp with `AF_UNIX` only and no exec/fork, an empty network
    namespace, a private PID namespace, read-only mounts plus a private
    tmpfs, and its own cgroup (RES-02). One sandbox profile per desk class;
    research and Mail (untrusted input) get the strictest.
  - Now: not started on main. Spawn sets only `Setpgid` and `Pdeathsig`
    (`internal/yaegikernel/desk_worker_spawn_linux.go:27-32`); the entry
    point drops nothing (`internal/autoputer/desk_session.go:18-41`); the
    guest runtime unit has no `User=` (`nix/autoputer-vm.nix:928-941`).
  - Existing work: `origin/land/desk-cell-confinement` (9cee80a2d; REC-01),
    rebased 10-15 and 831 commits behind main; its own landing record
    (432cbede0) lists what the rebase found.
  - Plans: [desk-cells-run-as-guest-root-and-egress-is-open-2026-10-14.md](problems/desk-boundary-residuals-2026-10-21.md)
    (needs inventory, failure modes 1-9); security review F4 and M4; ladder
    rung 1; ACTIVE landing order item 6; owner notes §31 (landed before
    launch).
  - Plan update: the problem doc's status said "fix in progress on branch"
    (fixed by this change). Still owed: the S1 security-floor definition
    (`definitions/choir-appdev-s1-security-floor-2026-10-01.md`) does not
    mention F4; the staging Landlock proof is owed (the Docker kernel has no
    Landlock).
- [x] **SEC-02 · P0 · red — The daemon checks the role on every frame action**
  - Done: FP2 batch 1, 564840fcf with CI fix 7c61ce078, deploy CI 39702960215 (2026-10-22 01:12 UTC), evidence docs/evidence/fp2-desk-boundary-20261022.json (frame_boundary: every desk but management refused when claiming management's product_api; wrong, missing and foreign slot tokens refused and the worker ended; emit_stream refused for management).
  - Target: the daemon binds each worker to the run record it spawned and
    serves a frame only if the desk's role table allows that action;
    unknown desks fail closed; `ActionExec` and `ActionWriteFile` leave
    every handle except engineering's.
  - Now: `serviceEgress` ignores `HandleRef` and `Epoch`; handles are
    verified only inside the worker; `ProductAPI`, `Research`, evidence
    tools and `HostEgress` take no role; Texture and management handles
    carry Exec and WriteFile.
  - Existing work: none. Plans: none before this change.
  - Plan update: new problem record
    [desk-frame-actions-not-role-checked-by-host-2026-10-21.md](problems/desk-boundary-residuals-2026-10-21.md)
    (this change). The role table should come from the desk manifest
    (ARCH-02).
- [x] **SEC-03 · P0 · red — Host egress floor for guest taps**
  - Done 2026-10-22: FP3 cut 1 deployed as 8cb7e1acb (deploy M). Proof
    (FP0, `_runs/fp3cut1`): `verify-host-hardening.py --expect-guest-floor`
    all PASS on the guest-floor section (24-rule egress chain, service ports
    exactly 8082-8084 and 8086-8089, jump first in mangle PREROUTING, IPv6
    drop, reply-only FORWARD, no per-tap ACCEPT, KSM run 2, /dev/kvm 660);
    no "host networking for … failed" in vmctl's journal;
    `host_floor_probe.py` from a test computer cold booted after the deploy
    matched 23 of 23 (own services and public 443 open; host 22/80/443,
    Dolt 13306/13307, another tap, another guest, metadata, private, CGNAT
    and SMTP closed); a reattached computer answers through the floor; a
    research run read GitHub and searched the web; the paper kept
    publishing. Item 5: maild 8087 stays open (Mail drafts are a guest
    caller). `--expect-guest-floor` is mandatory after every later deploy.
  - Target, in `internal/vmmanager/hostfirewall.go`:
    1. `-o <tap>` accepts only `ESTABLISHED,RELATED` (F8; ACTIVE item 1's
       unbuilt half; review N1).
    2. Guest to host: only that guest's service ports; drop the rest,
       including sshd and 80/443 (F6; `nix/node-b.nix:524-530` opens them on
       every interface).
    3. Guest to private, link-local, CGNAT and metadata destinations:
       drop.
    4. A rule that fails to install fails the VM start (today it only logs,
       `manager.go:3506-3512`).
    5. Drop guest access to maild 8087 if no guest caller exists.
    6. Confirm no IPv6 path reaches host services.
    7. The mangle-table guest egress policy from the desk-cells doc
       (code: 61f4da20f on `origin/land/desk-cell-confinement`).
    Public internet egress stays open (see "Networking must enable, not
    starve"); prove capabilities 1-6 still work after the deploy.
  - Now: partial. F7 landed (efbd69e27: forward default-deny over the whole
    pool, sweep at start). Egress is open (MASQUERADE to any destination).
  - Plans: desk-cells doc Problem 2 (inventory, mangle design, failure
    modes 10-16); review §3.6, §4.5; S1 definition acceptance item 2;
    s0-tap-egress-unfiltered-2026-10-01.md.
  - Plan update owed: no plan makes rule failure fail closed; the desk-cells
    doc's mention of open branch `fix/tap-deny-guest-range` is stale (it
    landed).
- [ ] **SEC-04 · P0 · red — Turn the Firecracker jailer on**
  - Now: built (9ac656298, 10e9f18e1) and off (`VMCTL_JAILER=off`,
    `nix/node-b.nix:1283-1288`) by design until staging proves it on one
    test computer; owner authorized the rollout (owner notes §27 item 3).
    Only baseline evidence exists (`evidence/vmm-confinement-baseline-2026-10-14.json`).
  - Depends on RES-05, or the rollout regresses host page-cache sharing.
  - Owed: rollout steps 2-4 of
    [firecracker-runs-as-root-without-jailer-2026-10-14.md](problems/firecracker-runs-as-root-without-jailer-2026-10-14.md)
    (`VMCTL_JAILER_ONLY`, `scripts/vmm_confinement_verify.py`, snapshot and
    wake, vmctl-restart reattach, assumed-breach probe; step 4 cold boots
    every hibernated computer once).
  - Plan update: the problem doc's status said "branch, not deployed" (fixed
    by this change). Still owed: slice 2 (per-VM network namespace).
- [ ] **SEC-05 · P0 · red — Bind the gateway bearer to its computer**
  - Now: `authenticateAutoputer` accepts the bearer from loopback or any
    Choir guest address (`internal/gateway/handlers.go:677-699, 753-794`);
    only the Jev route pins the computer's own address
    (`internal/gateway/config.go:336-399, 432-456`); hash compare is not
    constant time (`config.go:424-426`); TTL 720 h, no rotation.
  - Plans: review F9, §4.6; `docs/learn/03-gateway-and-providers.md` §2.3-2.4
    (stale: test-net allowance is now test-binary-only, b02a71fe8).
  - Plan update owed: no problem doc or station; rotation unplanned. Later
    the binding moves to the vsock CID (NET-03).
- [ ] **SEC-06 · P0 · red — Egress proxy tunnel identity (a P1 finding, promoted to P0)**
  - Now: suffix match admits subdomains and broad suffixes
    (`githubusercontent.com`, `hf.co`) (`internal/capsule/egress.go:104-143`).
  - Plans: [egress-proxy-tunnel-identity-2026-10-21.md](problems/egress-proxy-tunnel-identity-2026-10-21.md)
    (fix not chosen); the grants design's earlier argument for
    protocol-aware mirrors (`design/engineering-network-grants-2026-10-10.md`
    principle 2, threats T1/T8).
  - Plan update owed: cross-reference the mirror argument; NET-02 is the
    durable fix.
- [x] **SEC-07 · P0 · red — Address guard on browser and Obscura fetches**
  (red, corrected 2026-10-21: an egress boundary on untrusted URLs, like
  SEC-03 and NET-01)
  - Done: FP6 batch 1, 53a691be6, deploy CI 39692787991 (2026-10-21 23:36 UTC), evidence docs/definitions/choir-launch-first-pass-metamission-2026-10-21.ledger.md (23:36: GET /api/browser/capabilities and POST /api/browser/sessions answer 404; the unguarded browsercontrol and Obscura fetch routes are deleted; import-url still serves). Capability 4 is the Web Lens launcher.
  - Now: browser navigate checks only the scheme
    (`internal/browsercontrol/handler.go:79-93, 272`); the follow fetch uses
    a plain client (`:817-826, 920-930`); the alternate fetch pre-checks
    but follows redirects unguarded (`:746-757`). Guarded today:
    `sourcefetch.Client` paths (fetch_url, Read, content, sources).
  - Plans: review F5; desk-cells residual (claims a host policy that was on
    the unlanded `land/desk-cell-confinement` branch); [research-fetch-url-has-no-address-guard-2026-10-10.md](archive/problems/research-fetch-url-has-no-address-guard-2026-10-10.md)
    (no "fixed" status line; its egressTables cache leak still "not fixed").
  - Plan update owed: a problem record for browser and Obscura paths.
  - Research (2026-10-21, launch checklist L2-06): no automated capture
    path uses Obscura; research Read, content import and the source
    pollers all use `sourcefetch`. Its only caller is the Web Lens
    "Snapshot" button. Recommendation: delete Obscura and the
    `browsercontrol` package, and make Web Lens open the device's own
    browser. That closes the browser half of this item and half of
    SEC-15, rather than guarding it.
- [ ] **SEC-08 · P0 · red — Claude delegate launch posture**
  - Now: one test computer until 2026-11-03 (`nix/node-b.nix:1371-1389`).
    The owner's Claude token sits on the guest credential disk, readable by
    any guest-root process, including today's unconfined desk cells; the
    delegate calls `api.anthropic.com` directly, bypassing the gateway.
  - Plans: [design/claude-delegate-slice1-2026-10-14.md](design/claude-delegate-slice1-2026-10-14.md)
    (I19 exception, one week, now two);
    `reports/claude-agent-sdk-in-choir-2026-10-14.md` Options R/T/K.
  - Decided (owner, 2026-10-21, owner notes §36): the delegate stays on
    the one test computer until desk-cell confinement (SEC-01) lands.
    Before any wider use, Option K (key held by the gateway): SEC-01 alone
    does not unlock wider use.
- [x] **SEC-09 · P0 · green/red — KSM off, stale comments fixed**
  - Done 2026-10-22: bd4b47201 (KSM off, comments corrected) deployed in
    deploy M (8cb7e1acb); the guest-floor check reads KSM run 2 (unmerge).
  - Now: nothing in nix sets KSM; the last reading (`run=1`, nothing
    merged) predates the 10-15 host reboot. Comments still claim KSM
    dedups guests: `internal/vmmanager/manager.go:259-262`,
    `nix/autoputer-vm.nix:290-292, 303-309`, `flake.nix:525, 608`.
  - Plans: review N4, §4.7; ladder order #6; density L13 (rejected).
  - Plan update: ACTIVE framed KSM as an optimization (fixed by this
    change).
- [ ] **SEC-10 · P1 · red — Guest DNS**
  - 2026-10-22: host side done in deploy M (8cb7e1acb): unbound on
    127.0.0.1:8053, DNAT at VM start; the probe reaches the host resolver
    over TCP. Remaining: the guest network unit's line (FP2), then no public
    resolver.
  - Now: no `DNS=` in the guest network unit (`nix/autoputer-vm.nix:357-366`);
    per the desk-cells inventory the guest uses public resolvers.
  - Target: a host resolver only (then none, once NET-01 removes general
    egress). Plans: desk-cells inventory; grants design D2. No design.
- [ ] **SEC-11 · P0 · red — Proxy object-graph route trusts loopback**
  - Now: `requirePlatformObjectGraphCaller` accepts loopback plus
    `X-Internal-Caller: true` without `hostauth.Authorize` and admits
    `192.0.2.*` in production (`internal/proxy/guest_authority.go:20-51`),
    then forwards to corpusd marked internal (`internal/proxy/handlers.go:1799-1820`).
  - Plan update: added as a residual to
    [loopback-is-a-credential-2026-10-14.md](problems/loopback-is-a-credential-2026-10-14.md)
    (this change).
- [ ] **SEC-12 · P1 · red — Dolt auth stages 2-3 and owner-act enforce**
  - Now: hostauth enforces (9a42a7abb); Dolt is at stage "users" so
    passwordless root still works (`nix/node-b.nix:111`); owner-act is in
    observe (`:92`).
  - Plans: loopback doc residuals; [hostauth-enforce-flip-2026-10-20.md](problems/hostauth-enforce-flip-2026-10-20.md).
- [ ] **SEC-13 · P1 · red — Capsule seccomp: `clone3` and user namespaces**
  - Now: `clone3` allowed (`internal/capsule/seccomp.go:28`); target ENOSYS
    for `clone3` and deny `CLONE_NEWUSER`. Plans: ladder items.
- [ ] **SEC-14 · P1 · red — CI deploys as root with host-key checking off**
  - Now: `ci.yml:686-689, 800-806, 945-951`. Plans: review F13 (partial).
  - First half landed 2026-10-21 (6bfe06fcf): every CI ssh to Node B checks
    pinned host keys (`NODE_B_KNOWN_HOSTS`, `HostKeyAlias go-choir-node-b`,
    strict checking); ssh-keyscan is gone. Open: CI still deploys as root.
- [ ] **SEC-15 · P1 · red — Non-root guest runtime and guest sysctls**
  - Now: the guest daemon runs as root; pdftotext and Obscura run
    unsandboxed; no guest sysctls; `route_localnet=1` per tap. Plans:
    review F5; S1 remainder.
- [x] **SEC-16 · P2 · orange — Host-side callers that bypass the gateway**
  - Done 2026-10-22: the scorer reaches Jev through the gateway's host route
    (FP7, live since deploy K; "Jev reached by gateway" every run), and
    deploy N (4bc3aa3ec, 9174af033) removed its EnvironmentFile: the unit
    loads no provider environment and gateway-provider.env is an
    InaccessiblePath. First run after N (11:30:17): Jev reached by gateway,
    9 scored, 0 failed, scores.json rewritten, three host judgments served
    and receipts written by the gateway. The OpenRouter key is held only by
    the gateway.
  - Was: the front-importance timer called OpenRouter with the provider env
    (`docs/learn/03-gateway-and-providers.md:303-307`).

## 2. Network shape (after the floor)

- [ ] **NET-01 · P1 · red — Host fetch service; no general guest internet**
  - Target: research Read, PollSource (RSS, Telegram, GDELT), content import,
    podcast and YouTube fetches run on the host behind a verb, with the
    address guard, budgets and logging enforced there. Then the guest needs
    no general internet route.
  - 2026-10-22: the host fetch verb is on main (deploy N, 4bc3aa3ec), off:
    no computer is listed, and N logged no fetch and no grant. Next: list
    one test computer and run scripts/proofs/net01-host-fetch.sh.
  - Now: all of these fetch from the guest through open tap egress
    (`internal/researchtools/read.go:105-121`; `internal/sources/*.go`;
    `internal/content/content.go`). Search and keyed source APIs already
    run on the host gateway.
  - Plans: grants design Phase 3; desk-cells doc; review §4.5; S4
    definition addendum. No design; the desk-cells inventory misses
    YouTube, iTunes, podcast feeds and Obscura.
  - Done when: capabilities 1-4 of "Networking must enable, not starve"
    pass through the host path, with arbitrary public URLs still readable
    (no allowlist). (FP0, 2026-10-22: the MTProto clause is struck. The
    owner skipped Telegram, owner notes 2026-10-14 §"Telegram skipped",
    "Skip Telegram then", and no MTProto code exists, inventory Fact 1.
    If Telegram is reinstated, its host route is a new item.)
- [ ] **NET-02 · P1 · red — Package mirrors beyond goproxy**
  - Target: npm, PyPI, crates and git mirrors on the host, extending the
    goproxy pattern (`internal/goproxy`, tap port 8088), so capsule egress
    is mirrors only. Closes SEC-06 durably and dedups downloads across
    users.
  - Now: only the Go module proxy (3c6347e81, c841fa474).
  - Plans: grants design principle 2, §4.1, §4.5, Phases 1-3 (predates and
    was superseded by L2 CONNECT); `design/delegate-one-round-build-2026-10-21.md`
    §3 (npm through a Nix closure, first-party frontend only).
  - Plan update: the goproxy design's status said "built on a branch" (fixed
    by this change).
- [ ] **NET-03 · P2 · red — vsock control plane and identity**
  - Target: host-to-guest and guest-to-host service traffic over vsock;
    credentials bound to the VMM-assigned CID.
  - Now: `VsockPort` and `BaseVsockPort` exist and are never read
    (`manager.go:116-117, 275-276, 375`); no vsock device.
  - Plans: review §4.6 ("longer term"); learn/03 §2.4. No design.
  - Owed in the design: inventory of TCP paths (8082-8088 guest to host,
    8085 host to guest); vsock CVE review (ladder: Vghost, VsockDrop);
    snapshot restore resets vsock connections (hostlink already retries).
- [ ] **NET-04 · P2 · red — Remove the guest NIC**
  - After NET-01 to NET-03, and only once every capability in "Networking
    must enable, not starve" has a host route (Web Lens and Telegram
    included). No plan proposes it yet.
- [ ] **NET-05 · P2 · red — Grant object for open egress**
  - Target: `EgressGrant`, `request_grant`/`decide_grant`, L1/L3/L4, by
    owner act, time-limited, enforced at the host proxy; one grant model
    shared with app-package manifest grants (S7 addendum, S4).
  - Now: L2 only (284f7c1a2). Plans: grants design §4.0a, §4.2, phases.
  - Plan update: the design's header said "not settled" though L2 shipped
    (fixed by this change).
- [ ] **NET-06 · P1 · red — Data at rest (F12)**: snapshot and volume
  encryption, escrow sealing. Plans: blast-radius items 2-3.
- [ ] **NET-07 · P1 · red — CPU isolation before strangers (F11)**: SMT
  and core scheduling, `/dev/kvm` mode. Owner ruling: SMT stays on until
  outside users share the host (owner notes, around line 330).
- [ ] **NET-08 · P1 · red — Firecracker 1.17 and `--enable-pci`**; also
  unblocks `VM_DATA_DISCARD` (RES-16).
- [ ] **NET-09 · P2 · red — Keys behind policy signers; passkey-bound
  owner acts** (ladder items 7-8).
- [ ] **NET-10 · P1 · green then red — Network policy for app agents**
  - Target: each app desk's manifest (ARCH-02) declares its egress (verbs
    and grants, approved by the owner at install, re-approved when an
    upgrade widens them) and its ingress (inbound webhooks, push
    notifications, mail). Ingress always lands on a host service and
    reaches the desk as records, never as a connection into the guest.
  - Now: Mail ingress arrives through maild on the host (`internal/maild`);
    no other app agent has ingress, and no ingress model exists.
  - Plans: S7 addendum (manifest-declared grants); S4 egress capability
    model (`definitions/choir-appdev-s4-capsule-open-world-2026-10-01.md:283-370`);
    email desk design (maild as the ingress authority).
  - Plan update owed: an ingress section in S4 or S7.

## 3. Resources

- [ ] **RES-01 · P0 · red — In-guest cgroup tree**
  - Target: the daemon in its own slice with `memory.min`; a desks slice
    with an aggregate `memory.max` and `memory.high`; a capsules slice with
    an aggregate limit; cgroup delegation to the runtime unit.
  - Now: the daemon has no memory settings and `OOMPolicy=continue`
    (`nix/autoputer-vm.nix:928-941`); capsules are created under
    `/sys/fs/cgroup/capsule/<id>` with no aggregate.
  - Plans: an options row only (`docs/learn/02-desk-runtime.md:629`). No
    design.
- [ ] **RES-02 · P0 · red — A hard memory budget per desk worker**
  - Now: soft `GOMEMLIMIT` (MemTotal/16), `RLIMIT_AS` 8 GiB (meaningless in
    an 8 GiB guest), `oom_score_adj` 1000 (f93bbfd84).
  - Plans: [desk-cell-init-only-for-loop-never-ends-guest-oom-2026-10-16.md](archive/problems/desk-cell-init-only-for-loop-never-ends-guest-oom-2026-10-16.md)
    ("still open: an RSS budget per worker"). No slice owns it.
- [ ] **RES-03 · P1 · orange — Bound the capsule `exec-go-session` worker**
  - Now: no `MemoryLimitBytes`, rlimit or `GOMEMLIMIT`
    (`cmd/capsule-broker/main.go:153-162`); only the capsule cgroup bounds
    it. Plans: none.
- [ ] **RES-04 · P1 · red — Capsule memory per doctrine**
  - Target: `memory.high` = requested, `memory.max` = 2x requested,
    admission against requested, `memory.events` as feedback
    (`docs/choir-doctrine.md:406-408`; `docs/runtime-invariants.md:139-141`).
  - Now: admission reserves the full `memory.max` (2 GiB per engineering
    capsule) against 3/4 of MemTotal; no `memory.high` anywhere in
    `internal/capsule`.
  - Plan update owed: record the code-vs-doctrine divergence.
- [ ] **RES-05 · P0 · red — Keep store page-cache sharing under the jailer**
  - Now: the jailer places a reflink or full copy of the store image per VM
    (`internal/vmmanager/confinement.go:600-664`).
  - Plan update: new problem record
    [jailer-copies-store-image-per-vm-2026-10-21.md](problems/jailer-copies-store-image-per-vm-2026-10-21.md)
    (this change). Blocks SEC-04.
- [ ] **RES-06 · P1 · red — Make the guest give memory back**
  - Target: proactive reclaim of cold guest page cache (`DAMON_RECLAIM`,
    built in but unused, or periodic `memory.reclaim` on the desks and daemon
    slices), so free page reporting has free pages to return.
  - Now: only capsule teardown reclaim (5d228ecfd, 1f71940f8). The paper
    computer still holds about 7.9 GiB host-side, mostly guest page cache.
    Read again 2026-10-21 18:50 UTC: 7,970 MiB host RSS against about
    2.8 GB used in the guest (about 5.5 GB available); Node B 9.3 of
    31.1 GiB available.
  - Owed first: the after-burst measurement of free page reporting
    (`evidence/guest-memory-split-balloon-2026-10-14.md:38-46`).
  - Plans: density L5; `reports/lr-guest-memory-return-2026-10-14.md`
    steps 4-6. The LR station's now card is stale.
- [ ] **RES-07 · P1 · green — Measure what fills idle guest page cache**
  - A guest `fincore` read: nix store pages (shareable, RES-08) versus Dolt
    store pages (per computer). Owed by `reports/vm-density-limits-2026-10-14.md:308, 474-475`.
- [ ] **RES-08 · P2 · red — Share the base OS in guest RAM (pmem + DAX)**
  - Today the base OS is shared on disk and in the host page cache (one
    EROFS image), not in guest RAM: each guest keeps its own decompressed
    cache. Target: the store image as a read-only virtio-pmem device with
    an uncompressed EROFS mounted `-o dax`.
  - Plans: density L10, behind a security panel (cross-tenant shared pages,
    Flush+Reload). Both VMMs have virtio-pmem
    (`docs/research/isolation-runtimes-2026-10-14.md:299`), so this does not
    wait for Cloud Hypervisor; verify on Firecracker 1.16.1. Needs
    DEC-05.
- [ ] **RES-09 · P1 · red — Prove virtio-mem hotplug, then a small base**
  - Now: controller and leases landed (23cf3a400, ce959b2db), on for test
    computers (c9621a721); no staging proof; the probe refusal is
    undiagnosed. Base guest stays at the full shape plus a 4 GiB region.
  - Target after proof: a 1.5-2 GiB base plus leases for bursts (density
    L11, L12).
  - Plans: `reports/memory-hotplug-controller-design-2026-10-14.md` §6, §9;
    `scripts/memory_hotplug_probe.py`.
  - Plan update owed: `reports/overnight-landing-2026-10-15.md:136-138`
    wrongly says the feature is flag-off; residual J14 (`GOMEMLIMIT` fixed
    at boot).
- [ ] **RES-10 · P2 · red — Balloon stats reader; emergency inflation**
  - Now: stats off (`VM_BALLOON_STATS_POLLING_INTERVAL_S` unset), no reader.
  - Plans: playwright report §1.3; vm-backend-interface `MemoryController`.
    The LR rationale "with --no-api nobody can read stats" is stale
    (snapshot mode always has an API socket).
- [ ] **RES-11 · P1 · red — Model-call scheduling at the gateway**
  - Target: a per-computer fair-share queue at the gateway, replacing the
    fixed 60/min reject (`internal/gateway/ratelimit.go:9-16`) and the
    in-guest 1.5 s spacer (`research_children.go:30-37`). Fairness, not
    spending caps (owner: track spending, don't cap).
  - Plans: none at the gateway; management-delegation design O5;
    [research-fanout-overruns-computer-model-budget-2026-10-11.md](archive/problems/research-fanout-overruns-computer-model-budget-2026-10-11.md).
  - Prompt caching already landed (3f142fe2f and follow-ups).
- [ ] **RES-12 · P2 · orange — Batch endpoints for calls that can wait**
  (CPU lever C12, `reports/computer-cpu-efficiency-2026-10-14.md:552-560`).
- [ ] **RES-13 · P2 · red — Snapshot levers**: sparse or diff snapshots
  (L3; `fcapi.go:126` is Full), shorter idle hibernate (L4; 30 min today),
  snapshots surviving deploys (L20).
- [x] **RES-14 · P1 · red — Verify the snapshot budget counts real bytes**
  - Done 2026-10-22: the read-only probe (b2fa05c3a tests, 3ea3642f7
    `scripts/snapshot_budget_bytes_probe.py`; on main in deploy J,
    a84b39bac) read Node B at 09:00 UTC on deploy I (58e705802); reading
    358741b5d in
    [snapshot-budget-falls-back-to-logical-bytes-2026-10-14.md](problems/snapshot-budget-falls-back-to-logical-bytes-2026-10-14.md).
    The budget counts the true compressed bytes (`btrfs_extents` equals
    the tree search for all three committed snapshots, 790 to 1,503 MiB);
    G2 answered (the 4 GiB hotplug region is all holes, so `need` reserves
    the real worst case); no fix. The 48 GiB and 36 h settings stay: free
    space and the 36 h age bind first; a second reading with more
    hibernated computers is what would move either.
  - Now: FIEMAP bound landed (9001e9842); vmctl gained `CAP_SYS_ADMIN`
    (10e9f18e1), which may let the exact method succeed; no staging
    reading of which method hibernates log. Then revisit the 48 GiB and
    36 h settings (`nix/node-b.nix:1350-1355`).
  - Plan update: the problem doc's status said "no fix yet" (fixed by this
    change).
- [ ] **RES-15 · P2 · green — Record that admission does not reserve a
  running VM's growth** (`internal/vmctl/host_admission.go:58-66, 149-210`).
- [ ] **RES-16 · P1 · red — Data images that shrink**: Firecracker 1.17,
  then `VM_DATA_DISCARD` ([guest-data-image-never-trims-2026-10-20.md](problems/guest-data-image-never-trims-2026-10-20.md)).
- [ ] **RES-17 · P2 · orange — Desk-worker idle reaper and a smaller worker
  binary** (`docs/learn/02-desk-runtime.md:604-613`: reclaim runs only on
  acquire and done; a dedicated binary saves an estimated 40-60 MiB).
- [ ] **RES-18 · P1 · orange — Daemon memory**: Dolt cache knob off by
  default (`internal/gomemlimit/gomemlimit.go:63-65`);
  [guest-appender-retains-every-projection-payload-2026-10-19.md](problems/guest-appender-retains-every-projection-payload-2026-10-19.md);
  `GOMEMLIMIT` fixed at boot (J14).
  - Now (2026-10-22): measured on the paper, no lever landed
    ([daemon-memory-res18-2026-10-22.md](problems/daemon-memory-res18-2026-10-22.md)).
    The appender leak is fixed on main (b36519198). The daemon's largest
    retained term is Dolt's table-file indexes (about 0.5 GiB at a 20 GiB
    store), not the node cache (full at 256 MiB). The guest is filled by
    cold Dolt page cache, so daemon knobs do not lower the paper's host
    RSS until guest reclaim (RES-06) is on. The knob has no per-computer
    path; J14 matters only with RES-09's small base.
- [ ] **RES-19 · P1 · green — Density benchmark and LR measurements**
  - The density benchmark (`vm-density-limits` §4), the LR source sweep and
    the oversubscription probe were never run. Run before and after the P0
    resource items.
- [ ] **RES-20 · P2 · red — Cloud Hypervisor behind a backend seam**
  - Now: not on main. The seam was built 10-14 and never pushed until
    2026-10-21: `origin/recovered/vmbackend-2026-10-14` at b6f27ded4
    (99e69fe07 report, b6f27ded4 Backend interface, Firecracker and Cloud
    Hypervisor backends, conformance suite, no caller; 977 behind main).
    ACTIVE item 10's condition (items 4 and 5 landed) is met.
  - Why CH: VFIO/GPU (video app desks), copy-on-write restore and live
    migration, balloon resize, `vm.resize`; it has no jailer, so Choir
    builds the cage (review §5). Base-OS sharing (RES-08) does not need CH.
  - Plans: `reports/vm-backend-interface-2026-10-14.md`;
    `research/isolation-runtimes-2026-10-14.md`. No station; CH density
    unmeasured.
  - Plan update: the report now says nothing landed (this change). Still
    owed: `definitions/choir-supervised-app-development-metamission-2026-10-01.md:536`
    still excludes CH and hotplug.
- [x] **RES-21 · P2 · green — Reconcile guest sizes in docs**: the code
  defaults are 2 GiB interactive and 4 GiB platform
  (`internal/vmctl/ownership.go:1734, 1743`); Node B overrides interactive
  computers to 8 GiB (`nix/node-b.nix:1308`, `VM_INTERACTIVE_MEM_MIB=8192`);
  several docs say 4 GiB test computers. (Corrected 2026-10-21.)
  - Done: 57a994549 (docs).
- [ ] **RES-22 · P2 · red — Host slab and btrfs metadata** (density L19;
  [node-b-btrfs-metadata-exhausted-small-artifact-files-2026-10-16.md](problems/node-b-btrfs-metadata-exhausted-small-artifact-files-2026-10-16.md)).

## 4. App desks and scale-out

- [x] **ARCH-01 · P0 · green — Doctrine names the app-desk tier**
  - Done: e5ea2404d (doctrine I26, CLAUDE.md/AGENTS.md desk ontology,
    agent-product-doctrine, choir-vision; owner approved the wording
    2026-10-21, owner notes §36).
  - Owner: each app agent is its own RLM desk (owner notes §5); app agents
    get their own desks, and the plan is to add many (owner notes §36).
    The §23 "no new desks" ruling was about coordinator-invented desks
    ("story writer", "triage"), not app desks.
- [x] **ARCH-02 · P1 · red — A desk manifest instead of hardcoded desks**
  - Done: FP2 batch 1, 564840fcf with CI fix 7c61ce078, deploy CI 39702960215 (2026-10-22 01:12 UTC), evidence docs/evidence/fp2-desk-boundary-20261022.json (the self-test spawns one worker per manifest desk role: management, texture, research, research_child, management_task).
  - Now: desks are hardcoded across `internal/agentprofile/agentprofile.go`,
    `internal/agentcore/tool_profiles.go`, `internal/capsule/roles.go`,
    `internal/agentcore/inbox_records.go`, `internal/yaegikernel/choir.go`
    (handles, module sets) and the prompt overlays.
  - Manifest fields: owned object kinds; verbs; role table (SEC-02);
    sandbox profile and taint class; egress verbs; compute class; memory
    class; model policy; addressing and inbox routing; wake and dormancy;
    the restart contract (CLAUDE.md "Restarts End Work").
  - Plans: SC "desk capability surface" (unauthored, metamission
    `:192-196`). No design.
- [ ] **ARCH-03 · P1 · green then red — An app package carries its app desk**
  - The S7 manifest (frontend module, optional backend, recipe, data
    contract) has no desk; the email desk design never mentions packages.
    Install registers the desk from the package's desk manifest (ARCH-02);
    manifest-declared grants (S7 addendum, S4) attach to that desk.
  - Existing work: `origin/feat/s7-0-app-packages` at 4bc762102 (d9340a9a0
    S7-0a seam design, 5fe486f52 failure-mode tests, 4bc762102 wip
    implementation, paused and unreviewed; 831 behind main), recovered from
    the owner's iMac 2026-10-21. Its design predates app desks.
  - Plan update: S7 and the email desk report now cross-reference this
    (this change). Still owed in S7: apply owner ruling D1 (install is a
    tape fact, not part of the release closure; owner notes §23) and the
    edits listed in `definitions/choir-lp-s6-commit-gate-2026-10-20.md:391-397`.
- [ ] **ARCH-04 · P1 · orange — One writer per object kind**
  - Generalize Texture's sole-writer rule (`internal/store/texture_turn.go:179-180`)
    to every app desk; object-graph `Owner` is per package today
    (`internal/objectgraph/registry.go:24-44`).
  - Plans: doctrine C2; standing question 5. No general rule.
- [x] **ARCH-05 · P1 · green then red — Taint classes**
  - Desks that read untrusted content (research, Mail) never share a
    worker with privileged desks; record how their output flows to Texture
    and management. Only maild labels untrusted input today
    (`internal/maild/ingest.go:121, 176`).
  - Plans: email desk §5 (lethal trifecta, origin tagging deferred to E8);
    grants design `taint` field; RLM consensus. No per-desk definition.
  - Done: FP2 batch 1, 564840fcf with CI fix 7c61ce078, deploy CI 39702960215 (2026-10-22 01:12 UTC). The manifest's `Taint` keys the worker pool and the cell user; the data flow between classes is recorded in docs/definitions/choir-launch-fp2-desk-boundary-2026-10-21.md "Data flow between taint classes". Evidence: docs/evidence/fp2-desk-boundary-20261022.json (the deployed self-test: three distinct cell uids, every desk in its own confinement) and docs/evidence/fp2-desk-boundary-steps4-5-20261022.json (real management, research, Texture and engineering turns completing confined on computer-9648031f, 05:00 UTC).
- [ ] **ARCH-06 · P2 · red — Agents are records; worker slots are pooled**
  - Now: one REPL per agent id, 30 min retention, at most 12 idle
    (`internal/agentcore/tools_desk.go:84-305`). Target: pool workers per
    (tenant, desk class, taint class, tree).
  - Plans: PD2 (`definitions/choir-pd2-desk-sessions-outlive-runs-2026-10-11.md`);
    PD3.
- [x] **ARCH-07 · P1 · red — Versioned, authenticated frame protocol**
  (red, corrected 2026-10-21: it authenticates the worker-to-daemon
  boundary SEC-02 relies on)
  - Done: FP2 batch 1, 564840fcf with CI fix 7c61ce078, deploy CI 39702960215 (2026-10-22 01:12 UTC), evidence docs/evidence/fp2-desk-boundary-20261022.json (a protocol version 2 hello and a version-2 frame header both refused; per-slot tokens enforced).
  - Now: 8-byte header, no version, no authentication
    (`internal/yaegikernel/transport.go:14-23`). The RLM consensus asked for
    a version byte from day one (`reviews/rlm-implementation-consensus-2026-08-29.md:246-251`).
  - Target: a version and a per-slot token, so the transport can move from
    socketpair to vsock to TLS when workers leave the guest.
  - Existing branch: `origin/protocol-p2c` is the record protocol, not this.
- [ ] **ARCH-08 · P2 · red — Budgets passed down agent trees**
  - Target: a child gets a slice of its parent's tokens, wall time, egress
    bytes, child count and memory; depth cap 2-3 (settled design D7b/M6.5,
    `reviews/rlm-implementation-consensus-2026-08-29.md:304-318`).
  - Now: one level of fan-out (PD1); stale comments say children may fan out
    (`internal/yaegikernel/choir.go:359-360`; `research_children.go:137`).
  - Reconcile with "track spending, never cap" (DEC-07).
- [ ] **ARCH-09 · P2 · red — A compute capsule kind for app desks**
  - Artifact-producing capsules without a self-development effect bundle:
    video render, slides export, attachment conversion (email E5), capture.
    Now only engineering capsules exist; visual capture runs inside them
    (`origin/feat/visual-evidence-2026-10-20`, 11 commits, product work).
- [ ] **ARCH-10 · P2 · red — N capsules per assignment, apply serialized**
  - Now: one live engineering capsule per computer, new assignments
    supersede (`internal/agentcore/engineering_assignment_runtime.go:477-482, 651-701`);
    doctrine says FIFO with retryable refusal; `maxManagementActiveEngineeringSlots`
    (`runtime.go:1656-1679`) is unreachable.
  - Plan update owed: record supersede versus FIFO as a doctrine deviation.
  - Split (panel, 2026-10-21): admission correctness (FIFO with a
    retryable refusal, not cancelling the live assignment) is launch work,
    because the launch checklist's L3-07 builds several app agents at
    once; N parallel capsules per assignment stays P2.
- [ ] **ARCH-11 · P2 · red — Executor VMs**
  - Ephemeral, one per (tenant, taint class), cloned from a snapshot, no
    persistent state or authority, a lease per tree; they run untrusted
    content leaves and heavy compute and return proposed acts.
  - Plans: S9 forks and fleets; SP4 multi-host placement; PD4. "Worker VM"
    is a retired term (`docs/computer-ontology.md:406-418`), so this needs
    an ontology name. Cross-tenant snapshot templates need DEC-05.
- [ ] **ARCH-12 · P2 · red — Executors on other hosts** (SP4, after S9;
  needs ARCH-07).
- [ ] **ARCH-13 · P2 · red — Store fan-in for wide trees**
  - Now: guest commits every 100 events on one engine mutex
    (`internal/objectgraph/engine_mutex.go:10-14`); `doltbatch` is host only.
  - Plans: `reports/projection-event-volume-design-2026-10-19.md` option (c);
    SA slices 4 and 6.
- [ ] **ARCH-14 · P2 · orange — App verb registry with per-caller allow
  lists** (puter pattern, `docs/research/puter-everything-2026-10-14/README.md:53-85`).
- [ ] **ARCH-15 · P2 · green — Rename the launcher's "desk"**
  (`DESK_APPS`, `launcher.desk` in `frontend/src/lib/apps/registry.ts:20-23, 269-271`)
  so it does not collide with agent desks.

## 5. Plans updated

Done in this change (2026-10-21):

- [x] Problem record: [desk-frame-actions-not-role-checked-by-host-2026-10-21.md](problems/desk-boundary-residuals-2026-10-21.md) (SEC-02).
- [x] Problem record: [jailer-copies-store-image-per-vm-2026-10-21.md](problems/jailer-copies-store-image-per-vm-2026-10-21.md) (RES-05).
- [x] Status lines: desk-cells (REC-01), jailer (SEC-04), loopback (plus the
  proxy-route residual, SEC-11), engineering network grants (NET-05),
  goproxy design (NET-02), snapshot budget (RES-14), pressure reclaim.
- [x] `reports/vm-backend-interface-2026-10-14.md`: nothing landed (RES-20).
- [x] `reports/vm-density-limits-2026-10-14.md`: lever status as of
  2026-10-21.
- [x] S7 app packages and the email desk report: app packages carry app
  desks (ARCH-03).
- [x] ACTIVE: this checklist, landing-order status, KSM framing.
- [x] Owner notes §36: today's direction.

Still owed:

- [ ] S7: apply ruling D1 and the LP S6 list of S7 edits (ARCH-03).
- [ ] S1 security-floor definition: F4, the confinement branch, SEC-02.
- [x] Doctrine and CLAUDE.md desk ontology (ARCH-01): e5ea2404d.
- [ ] `definitions/choir-appdev-sa-agent-density-2026-10-05.md:61` lists
  zswap, rejected as L15.
- [ ] Supervised metamission `:536` excludes CH and hotplug (RES-20).
- [ ] `docs/learn/03` §2.3 and `docs/learn/04` §5.2 (admission) are stale.
- [ ] `reports/overnight-landing-2026-10-15.md:136-138` (hotplug flag).
- [ ] `problems/prompt-cache-breaks-across-turns-2026-10-14.md:151` (repair
  landed).
- [ ] Firecracker version cites (1.15.1, now 1.16.1) in the density,
  ladder and review reports.
- [ ] Problem record for browser and Obscura fetches (SEC-07); problem
  record or station for the gateway bearer (SEC-05).
- [ ] Mission graph: no node exists for any item here.

## 6. Owner decisions

- **DEC-01** Push `fix/desk-cell-confinement` from your machine, or confirm
  it is lost (REC-01). Done 2026-10-21: an iMac session found all three
  and pushed them with the owner's approval (REC-01, ARCH-03, RES-20).
- **DEC-02** Decided 2026-10-21: the owner approved the app-desk wording;
  applied in e5ea2404d (ARCH-01).
- **DEC-03** Decided 2026-10-21: the Claude delegate stays on the one test
  computer until SEC-01 lands (SEC-08).
- **DEC-04** Answered 2026-10-21 (owner notes §37): every security and
  density item that can land before launch does; P1 is launch work.
- **DEC-05** Cross-tenant shared pages (pmem DAX, shared snapshot templates):
  a side channel of the same class as KSM; run the security panel
  (RES-08, ARCH-11).
- **DEC-06** SMT and core scheduling before strangers (NET-07; existing
  open decision).
- **DEC-07** Budgets for agent trees as scheduling limits under "track
  spending, never cap" (ARCH-08, RES-11).

## 7. Branches checked

| Branch | Unmerged | Relevance |
|---|---|---|
| `fix/desk-cell-confinement` | 4 (pushed 2026-10-21 from the iMac) | REC-01 |
| `land/desk-cell-confinement` | 5 (rebased 10-15; pushed 2026-10-21) | REC-01, SEC-01, SEC-03 |
| `recovered/vmbackend-2026-10-14` | 2 (pushed 2026-10-21) | RES-20 |
| `security/quick-containment` | 2 | REC-02 |
| `feat/visual-evidence-2026-10-20` | 11 | ARCH-09 (product); its teardown reclaim already on main |
| `protocol-p2c` | 3 (WIP) | record protocol, not ARCH-07 |
| `feat/research-system1` | 4 | System 1 verb; not in scope |
| `feat/s6-0-release-witness` | 3 | LP S6; not in scope |
| `feat/cast-request-to-engineering` | 2 | not in scope |
| `ops/dolt-module-bump-r2` | 1 | dependency bump |
| `feat/s7-0-app-packages` | 3 (pushed 2026-10-21 from the iMac) | ARCH-03; cited in LP S6 `:41-46, 390` |
| all others | 0 | merged |
