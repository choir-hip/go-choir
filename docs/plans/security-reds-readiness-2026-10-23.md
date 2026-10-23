# Security reds: readiness for the owner's morning (2026-10-23)

Mutation class: green (a plan; no code, no deploy, nothing changed on any
computer). Written by a read-only FP0 worker from `origin/main` b1abdfb0b
(2026-10-23 13:00 UTC), the metamission ledger, the FP3, FP8, FP10 and FP2
station files, the problem records, `nix/node-b.nix`, `nix/autoputer-vm.nix`
and `scripts/proofs/`. Nothing here was run on Node B; every "staging says"
below is a ledger line, not a fresh reading. Where I could not find a
receipt I say so.

## The short version

| Item | Built | Reviewed | Live today | Next act | Wait on owner? | Work left |
|---|---|---|---|---|---|---|
| SEC-11 enforce | yes, on main (aa441f388 / 891d52ae5 era) | bunch 41, 63 | observe (`PROXY_OBJECT_GRAPH_INTERNAL_AUTH=observe`), since deploy T 10-22 17:00 | read the observe journal, flip one line | no | about 1 h plus one deploy |
| SEC-12 Dolt stage 3 | yes, one line, on `lane/host-b4-b5-on-main` (6365f329b); cherry-picks onto main cleanly | bunch 41, 63 (accept) | stage `clients` since deploy T | land alone, `--expect-dolt locked` | no | about 1 h plus one deploy |
| SEC-12 owner-act enforce | yes, one line (de193cf8d) | bunch 41, 63 (accept on a gate) | observe | run the entry gate (needs an owner session on a test computer), then land alone | a browser session as a test computer's owner | 1 to 2 h gate plus one deploy |
| SEC-04 jailer, step 2 | yes, on main, inert (deploy B 3c46d5196, FC 1.17 pin D8) | bunch 5, FP8 mirror, pin bunches 144 to 146 | `VMCTL_JAILER=off` | name one fresh test computer, run steps 4 to 6 | for steps 3 to 4 only (the global cold boot) | step 2: half a day. All four steps: 1 to 2 days elapsed |
| NET-01 host fetch | yes, on main, off (deploy N 4bc3aa3ec) | bunch 6b, 45 (all three accept) | `hostFetchVMs = ""` | list one test computer's VM id, run `net01-host-fetch.sh` | no, except two adversarial reads that want a guest root | step 1 (one test computer): 2 to 3 h. "No general guest internet": days, and it is NET-04's sequence, not NET-01's |
| SEC-10 last slice | host side live; guest `DNS=` selector live for one test computer (D11, D12) | bunch 149 (Devin approve) | T = computer-71afbf14 resolves through Unbound | owner runs step 3 in T's console; then widen; then the public-53 close | yes: step 3 needs a guest shell | the close patch is not in any ref I found; about half a day active, 1 to 2 days elapsed |
| SEC-15 | doc-tool sandbox and capability floor on main, both in observe; guest sysctls in the image; Obscura removed | bunch 39, 43 and the FP2 batch 2 deltas; bunch 100 (Obscura) | observe | no per-computer selector exists for either switch; build one, then trial | no | switches: about a day. Non-root daemon: several days, not started |

Proposed order for the morning, in one paragraph. Read first, deploy second.
Before any deploy, read the SEC-11 observe journal and have the owner run the
three-command SEC-10 step 3 in T's console (T is already booted with
`choir.dns`). Then, one deploy at a time (the activation lease, and every one
of these touches auth, vmctl or the gateway): (1) SEC-11 enforce, (2) SEC-12
Dolt stage 3, (3) SEC-04 step 2 on a freshly minted test computer. While those
run, prepare the SEC-12 owner-act gate and run it when an owner session is
free, then land cut 4 alone (4). NET-01's test-computer grant (5) is cheap and
safe and unlocks the NIC sequence, but it buys little security by itself; put it
after the jailer so the two vmctl-touching deploys do not overlap. SEC-10's
close of public 53 and SEC-15's switches wait: the first needs every running
guest cold booted with `choir.dns` (the paper and the owner's computer
included) and a patch that is not on a branch; the second needs a selector
built. SEC-15's non-root daemon is a multi-day build, not a morning item.

Rules that shape the order (ledger 13:35 and 00:20, 10-22): a deploy carries at
most two live reds, only if they touch different services or binaries, have
their own proofs and are independently revertible, and neither touches the
store schema, the boot path, auth/session or deploy routing. Auth and
store-auth changes keep a deploy to themselves. A config-only flip whose
enforce proof has passed on a test computer and whose rollback is the same
one-line flip may ride as inert. An activation lease runs from a deploy to the
end of its proof. So SEC-11, SEC-12 (both halves) are four separate deploys, or
at best SEC-11 and Dolt stage 3 together as deploy T did before the 13:35 rule;
I recommend separate.

---

## SEC-11. Proxy object-graph route trusts loopback

Class: red. Protected surfaces: auth and session (host-service authority),
the proxy, corpusd's internal authority (`/internal/platform/objects*`, edges,
captures, published search, story verdicts).

What is built. `requirePlatformObjectGraphCaller`
(`internal/proxy/guest_authority.go`) no longer accepts a loopback peer plus
`X-Internal-Caller: true` alone: it also calls
`hostauth.AuthorizeAtMost(r, h.objectGraphInternalAuth())`, a route-level mode
read from `PROXY_OBJECT_GRAPH_INTERNAL_AUTH` (unset means enforce).
TEST-NET-1 (`192.0.2.*`) is admitted only inside a Go test binary. The
credential is sent to port 8082 as well now (`hostauth` `defaultPorts`).
Commits aa441f388 on main (originally 891d52ae5 on `lane/fp3-cuts-on-main`).

Reviewed. Flash and Devin accepted the originals; DeepSeek accepted in bunch 41
and on the replay in bunch 63.

Landed. Deploy T (f0b1ee313, 10-22 17:00 UTC). Behind a switch: yes,
`nix/node-b.nix:959` is `PROXY_OBJECT_GRAPH_INTERNAL_AUTH=observe`. Step 2 PASS
at 17:05: marker alone answers 200 and logs "hostauth: observe: would refuse
... missing credential"; marker plus credential answers 200 and logs nothing.
The credential alone answers 403 "internal caller required" (the marker is
still required).

Next step. Flip `observe` to `enforce` (one line). The gate is a journal read
over the roughly 20 hours since T: `journalctl -u go-choir-proxy --since
"2026-10-22 17:00 UTC"` for "hostauth: observe: would refuse" lines on
`/internal/platform/`. Zero lines apart from FP0's own probe at 17:05 is the
pass. The FP3 station (`choir-launch-fp3-groups-2-3-2026-10-21.md`, cut 2 step
4) says "after a clean day". Ledger 13:35 allows hours, "once the traffic that
matters has passed through"; the traffic that matters is the paper's publish
and any host-side object-graph caller. I found no receipt of this read, so it
is owed. Proof script: none. The proof is a curl pair on Node B (marker alone
now 403; marker plus the host credential 200), plus the paper's next story
publish. Do not send the credential to 8082 by hand: `scripts/lib/choir_internal.sh`
deliberately does not (the proxy's own hostauth tests cover that half).

Needs from the owner. Nothing. The FP3 station states "no owner question blocks".

What could go wrong. The paper: any host-side process that calls the route over
loopback with the marker but without the credential would start getting 403.
Guest callers are unaffected (they arrive from tap addresses, which take the
other branch). I did not enumerate the host callers; the journal is the oracle.
Owner computers: no direct effect. Rollback is the same line back to `observe`
and a deploy; the proxy restarts.

Estimate. About an hour to read the journal and prepare, plus one deploy cycle
(CI, deploy, proof), so 1 to 1.5 h.

## SEC-12a. Dolt auth stage 3 (root locked)

Class: red. Protected surfaces: the object store's access (auth), both Dolt
servers (platform 13306, corpus 13307), corpusd, checkpointd, vmctl's route DB.

What is built. `doltAuthStage` `clients` to `locked`
(`nix/node-b.nix:147`; commit 6365f329b on `lane/host-b4-b5-on-main`, also
9b47d4a31 on `lane/fp3-cuts-on-main`): the dolt-users units set root's password
from `/var/lib/go-choir/secrets/dolt-root-password` (the ensure script's
`lock_root` path); passwordless root is refused. Root clients that remain
present the password: `scripts/off-host-backup.sh` tries root with the password
first, the liveness probe and squash checks use the app user, local `dolt sql`
in the data directory uses Dolt's local-user file. I replayed both commits
(6365f329b and de193cf8d) onto current main in a scratch cherry-pick: they
apply with no conflict (I then reset; nothing was kept).

Reviewed. DeepSeek accept on the original (bunch 41) and on the replay (bunch
63, 15:58 on 10-22). Flash and Devin accepted the originals.

Landed. Stages "users" and "clients" are on main and live: stage 2 in deploy T
(44c50bb7b), "no Access denied since 16:55" (ledger 17:05). Stage 3 is not on
main. Progress JSON: slice at stage 3 ("accepted"). `verify-host-hardening.py`
already takes `--expect-dolt locked`.

Next step. Land 6365f329b alone as its own deploy (auth and store-auth keep a
deploy to themselves). Gate first: `--expect-dolt clients` healthy, and no
dbcred or "Access denied" line since T. Proof after: `--expect-dolt locked`
PASS; passwordless root over TCP refused on 13306 and 13307; the next
off-host backup run succeeds (the backup's `dolt_sync` and snapshot-hash reads
are the root-with-password consumers); corpusd, checkpointd and vmctl active.
Script: `scripts/verify-host-hardening.py`; no `scripts/proofs/` file for it.

Needs from the owner. Nothing. Owner notes section 27 item 3 authorizes
security switch flips after a clean window.

What could go wrong. Anything that still connects as passwordless root now
fails. By grep of `nix`, `scripts`, `cmd`, `internal` the remaining root
clients are `scripts/off-host-backup.sh` (has the password path),
`scripts/dolt-data-to-subvolume.sh` (has the password path), the dolt-users
ensure script, and the three root-looking DSNs in `node-b.nix` (1213, 1323,
1500) that `internal/dbcred` rewrites to the app user at connect. The weekly
squash runs offline on files, not over SQL. I did not audit ad hoc operator
runbooks or `_runs` scripts. The unlanded `lane/fp0-platform-dolt-oldgen`
restore drill is a new consumer; check it against a locked root when it lands.
The paper: a Dolt auth failure would stop corpusd's platform store, which the
paper publishes through. Rollback is not a pure one-line flip: reverting the
nix line to `clients` does not unlock root (the password is stored in Dolt);
only if something needs passwordless root, on Node B as root:
`ALTER USER 'root'@'localhost' IDENTIFIED BY ''` using the root password from
the secrets file. Nothing that I found needs it, so the revert alone restores
the previous client wiring.

Estimate. One deploy cycle plus proof: 1 to 1.5 h.

## SEC-12b. Owner-act enforce

Class: red. Protected surfaces: Texture canonical writes (owner mode), run
acceptance (`effect_accepted` provenance), auth.

What is built. `ownerActMode` `observe` to `enforce` (`nix/node-b.nix:126`;
commit de193cf8d, 7a7a34228 on `lane/fp3-cuts-on-main`). With enforce, corpusd
refuses an owner-mode write that lacks the proxy's signed act.

Reviewed. Bunch 41 and bunch 63: DeepSeek accepts with edits; the bunch 63
note says the observe-window gate "is still unmet, the same as in bunch 41".

Landed. No. Observe is live. The 10-20 observe window had no real owner acts,
so its zero was not evidence (the commit message says so).

Next step. The entry gate in the FP3 station, cut 4, steps a to f, on a
`class:test` computer (never the paper or the delegate computer): the proxy
holds the signing key and corpusd names the same key id; one real owner mode
write through the proxy (arm self-development accept_once, a PUT to
`/api/computers/<computer>/self-development/mode`); one accepted effect; one
Texture owner edit as a control; zero "would refuse" lines in the window; and
rows in `owner_act_receipts` and `self_development_mode_receipts`. Land only
when a to f hold. Then deploy alone, and repeat b to d; a direct corpusd
owner-mode write with the internal credential but no act must be refused.

Needs from the owner. A browser session as a test computer's owner on
choir.news (steps b and d are UI actions). If the QA account from
`scripts/qa_test_computer.mjs` can be driven by a spec, FP0 can run it
without the owner; I did not check whether such a spec exists for the
self-development approve. This is the only item on the list that needs a
signed-in human act, not a guest shell.

What could go wrong. A legitimate owner path that does not carry the act
(conjecture of the commit: "every legitimate owner-mode write carries the
proxy's act") would be refused: an owner could be unable to arm
self-development or accept an effect. The paper has no owner acts in normal
running. The owner's computer: this is where a miss would show. Rollback is
the one line back to `observe` and a deploy.

Estimate. Gate 1 to 2 h including the owner's session, plus one deploy cycle.
Do it after Dolt stage 3 so a failure is not confused with a store-auth fault.

## SEC-04. Firecracker jailer, rollout step 2 (one test computer)

Class: red. Protected surfaces: vmctl (launch, kill, reattach), VM lifecycle,
snapshot validity, the host firewall (one OUTPUT owner rule), the vmctl unit's
privileges and cgroup, tenant isolation.

What is built. Slice 1 of the jailer (9ac656298, 10e9f18e1): per-VM uid,
chroot, new pid namespace, cgroup leaf under
`system.slice/go-choir-vmctl.service/vms`, empty VMM environment, no IP for VMM
uids. FP8 (`lane/fp8-image-jailer`, 11 commits, replayed onto main as deploy B
3c46d5196) added RES-05 (jails hard-link one root-owned 0444 store image per
source identity, so the host page cache stays shared), a per-computer selector
(`VMCTL_JAILER_ONLY` takes ids, `class:<class>`, `*`), and the proof script
`scripts/proofs/fp8-image-jailer.sh` (steps 4 to 6 are SEC-04, step 5 is
RES-05). The jailer binary is now the pinned Firecracker 1.17.0 jailer
(`VM_JAILER_BIN`, D8, 10-23 03:55). Verifier: `scripts/vmm_confinement_verify.py`.

Reviewed. Bunch 5 (Gemini flash accept; Devin accept); the FP8 mirror; the 1.17
pin in bunches 144 (Gemini adopt, Devin approve, DeepSeek conditional), 145
(DeepSeek: changes required) and 146 (DeepSeek: pass). The slice-1 design had a
real jailer run in a privileged container (`TestRealJailerLaunch`), not a
guest boot.

Landed. Deploy B (inert) and D8 (the 1.17 pin). `VMCTL_JAILER=off`
(`nix/node-b.nix:1526`), `VMCTL_JAILER_ONLY` unset. FP8 step 1 PASS (a
control's `fc-config.json` unchanged across the deploy). Progress JSON: "jailer
on one test computer" stage 0, "not run; FP8 step 2 blocked by host admission
03:16". The vmctl unit changes (no `SYS_PTRACE`, the wider bounding set,
`Delegate`) shipped with slice 1; I found no receipt that
`vmm_confinement_verify.py --expect direct` ran on a newly launched direct VMM
(only the 10-14 baseline exists). It is cheap to run first and I recommend it.

Next step. FP8 definition "RES-05 and SEC-04" steps 1 to 5, in order:
1. Mint a fresh test computer (`node scripts/qa_test_computer.mjs --label
   fp8-jailer`). Read its VM id from `GET /internal/vmctl/list`. Set
   `VMCTL_JAILER=on` and `VMCTL_JAILER_ONLY=<its computer id>`; deploy. vmctl
   log: `jailer ready`, not `JAILER UNAVAILABLE`.
2. Stop/start it. `scripts/proofs/fp8-image-jailer.sh --computer <id> --step 4`,
   then 6 (`vmm_confinement_verify.py --ssh node-b --vm <vm> --expect jailer`).
   Boot timeline `confinement.jail_files` shows `store.img=shared(reflink)`.
3. Mint a second, add it to the list, start it; script step 5 (same
   `dev:inode`, root 0444, link count 3 or more).
4. Manual lifecycle (the script prints it as MANUAL): hibernate by snapshot and
   wake (`snapshot_resume`), stop/start, `systemctl restart go-choir-vmctl`
   and reattach, then the assumed-breach probe (review section 7.2: as the VMM
   uid, in its namespaces, other computers' `data.img`,
   `gateway-provider.env`, `auth-signing/ed25519-key`, `127.0.0.1:8083` all
   refused). That probe has no script; it is manual commands.
5. `VMCTL_JAILER_ONLY=class:test`, then (owner) dropping the list: every
   computer moves at its next launch and every hibernated computer cold boots
   once.

Use a freshly minted computer, not computer-9648031f (it carries tunnel enforce,
DAMON and discard selectors and has not woken since 10-22 05:04) and not
kf-canary 254b5cfe (it is SEC-10's unresumed control). Host admission refused
a new test computer for 20 minutes on 10-23 03:16 (about 5 GB available
against 4779 MiB plus 2048 MiB needed); check headroom before minting. The
steps 4 to 6 of the proof script can only be run after the nix edit and the
deploy.

Needs from the owner. Steps 1 to 4: nothing; the rollout was authorized (owner
notes section 27 item 3). Step 5 (drop the list): a deploy-shape choice, one
cold boot of every hibernated computer, plus the paper and the owner's
computer at their next launch. Per the resource rule (owner 10-22) a cold boot
that durably improves the posture is acceptable, but the paper's window should
be chosen. Not for this morning.

What could go wrong. The test computer fails to launch (rollback: remove the
id, redeploy; the next launch reclaims ownership to root and cold boots). A
vmctl fault would reach the paper: vmctl restarts on every deploy already, and
direct launches are unchanged when the selector is empty or names others. Open
risks the docs themselves name: whether the `MemorySwapMax=0` guarantee holds
for a jailed process (the hibernate problem record says "repeat R2 once the
jailer is on"); snapshot wake and hotplug/balloon under the jail are unproven on
staging; the 1.17 jailer has not been run jailed. Per-VM network namespaces
(slice 2) are not built; a VMM escape still sees the host network namespace.

Estimate. Steps 1 to 4: half a day with a test computer and a clean memory
reading. Step 5 and the global cold boot: another half day plus the owner's
call. Slice 2 (network namespaces): not estimated here; the review sized it
medium-high.

## NET-01. Host fetch service

Class: red. Protected surfaces: gateway (a new route that dials public
addresses), vmctl credential disk, guest egress.

What is built. `POST /provider/v1/fetch` on the gateway
(`internal/gateway/host_fetch.go`, `internal/hostfetch`), the guest route
(`sourcefetch.SetRoute`, set at `autoputer.Run`), vmctl writes
`host-fetch.json` on a listed VM's credential disk at cold boot, and podcast
search moved onto the guarded client. The gateway admits only VM ids in
`CHOIR_HOST_FETCH_VMS`; empty refuses everyone (fix commit 60f3f12c7; the
first version served any guest's bearer). Budget 600 hops a minute and 16 in
flight per computer. Proof tool `scripts/host-fetch-proof.py`; proof script
`scripts/proofs/net01-host-fetch.sh` (8 steps).

Reviewed. Devin accept (bunch 6b), flash accept, DeepSeek accept (bunch 45), so
all three; sent back once at 00:20 on 10-22 for the open verb and fixed.

Landed. Deploy N (4bc3aa3ec, 10-22 11:29), off. `hostFetchVMs = ""`
(`nix/node-b.nix:28`). "No fetch or grant logged." The progress JSON has
"one test computer granted, proof" at stage 0.

Next step. Put one test computer's VM id (not computer id; the list is VM ids
because the gateway sees the credential identity) in `hostFetchVMs`, deploy,
cold boot it (a snapshot resume keeps the old credential disk), then run
`net01-host-fetch.sh` steps 1 to 8 (1 and 2 before the grant, the rest after;
step 6 adversarial reads are FP0's). Rollback drill is step 8's.

Needs from the owner. Nothing for step 1. The first half of step 5 calls the
verb directly "from an unlisted computer's guest root (or the gateway's test
harness with its bearer)"; the harness alternative avoids a guest shell. The
paper's grant is a later, separate step after step 7 counts its peak hops
against 600 a minute.

What could go wrong. The paper: nothing until it is granted. A granted
computer never falls back to direct egress, so a gateway fault (or the gateway
restart a deploy causes) fails that computer's reads, imports and polls
closed. The gateway runs as root with no network hardening and the verb makes
it dial guest-chosen public addresses (a named residual in the station file; a
dedicated unprivileged fetch user is the durable shape). Gateway restarts also
interact with the Synthetic gate work running on the same process. Class
tokens are not honored (VM ids only), and a re-realized computer gets a new VM
id, so the list can silently stop matching.

Estimate. Test-computer grant and proof: 2 to 3 h, one gateway-and-vmctl
deploy. The paper's grant: another half day (peak count, budget). The stated
goal, "no general guest internet", is not NET-01's code: it is NET-04, after
NET-01, NET-02 (mirrors), NET-03 (vsock) and the DNS and time routes. The
design (`docs/design/vsock-control-plane-and-nic-removal-2026-10-22.md`, steps
V0 to V6 and N1 to N7) is days of reds, each per computer. NET-01's security
gain on its own is that the address guard and the fetch record move to the
host; the guest still has open TCP egress until N6.

## SEC-10. Guest DNS, the last slice (no public resolver)

Class: red. Protected surfaces: the host egress floor (a host-wide rule), vmctl
launch configuration (kernel args and snapshot validity), Unbound.

What is built. Host side live since deploy M (8cb7e1acb): Unbound on
127.0.0.1:8053, a DNAT of each tap host address :53 to it (`VMCTL_GUEST_DNS_PORT`
= 8053). Guest side: the image line (`choir.dns=`, 236e816c3) and vmctl's
per-computer selector `VMCTL_GUEST_DNS_ONLY` (`internal/vmmanager/guest_dns.go`;
ids, `class:`, `*`). D11 landed the selector inert, D12 (7ca537e01, 10-23
08:08) named test computer 71afbf14. `nix/node-b.nix:1497` names it today.

Reviewed. Bunch 149 (Devin approve: inert when unset, fails closed, the guest
cannot enable it). Flash/Devin accepted FP3 groups 2-3 in bunch 5 for the host
side.

Landed. Steps 2 PASS (DNAT for UDP and TCP 53, Unbound listening); step 4 PASS
in part (the UDP DNAT counter moved 0 to 2 packets after T's cold boot); step 6
PASS so far (the paper and kf-canary carry no `choir.dns=`); kf-canary has not
resumed since D12, so its resume half is open. Step 3 (resolv.conf and
`resolvectl status` inside the guest) and step 5 (a capsule git fetch, a
research read, a source poll) are open. The ledger notes step 5 is weak: an
engineering capsule in `ecosystem_proxy` mode resolves through the runtime's
egress proxy, not the guest resolver, so a capsule git fetch does not prove
the guest resolver. A model call through T worked throughout.

Next step.
1. Owner, now, in T's Management Console as T's owner (T = computer-71afbf14,
   woken through vmctl resolve if needed): `cat /etc/resolv.conf` (expect
   `nameserver 127.0.0.53`), `resolvectl status` (the link's DNS Servers are
   exactly T's tap host address, no fallback in use), `cat
   /run/systemd/network/10-vm-runtime.network` (ends with `DNS=<tap host ip>`).
   If resolv.conf is anything else, stop: resolved is not the guest's resolver
   and the line does nothing (record it; do not go to the close).
   Alternative with no shell: add `/etc/resolv.conf` and the resolved link
   status to the isolation self-test (a small orange change). The owner
   decided to remove the console eventually (N4: "just remove zot"), so a
   self-test field outlives it.
2. Widen `VMCTL_GUEST_DNS_ONLY`. Public 53 can leave the floor only when every
   running guest has cold booted with `choir.dns=`, because the floor is
   host-wide: a guest still on public resolvers loses DNS entirely. Adding
   `choir.dns` voids a computer's snapshot once (launch-config hash), so each
   hibernated computer cold boots once; the paper and the owner's computer need
   a planned restart. Use `*` only after the probe.
3. Close public 53 (drop UDP and TCP 53 to non-local destinations before the TCP
   RETURN, `internal/vmmanager/guest_egress.go:92-102`; the floor also leaves UDP
   123 open for timesyncd). The FP3 station says this exists as
   `sec10-step2-close-public-dns.patch`, an uncommitted file in the lane
   worktree root. I could not find it: the lane worktree is gone and the file
   is on no ref I searched (`git ls-tree` of `origin/lane/fp3-sec10-guest-dns`,
   a filesystem search under `~/Dev`). Treat it as needing a rewrite: two
   rules plus tests, `verify-host-hardening.py` and `host_floor_probe.py`
   updates. A day of Unbound traffic and the probe first (the patch header's
   gate).

Needs from the owner. Step 1: a guest shell (or the self-test change). Step 2:
a time for the paper's and the owner's cold boot.

What could go wrong. The paper and every computer share Unbound's fate once
public 53 is closed: an Unbound outage (each deploy that touches it) fails all
lookups, with no fallback by design. Rolling back out of order strands a
guest: deselect first, then take away the port or Unbound; dropping Unbound or
the DNAT under a guest that has `DNS=` leaves it with no resolver until its next
cold boot. DNS over HTTPS on 443 stays possible until NET-01 and N6 close
public TCP, so closing 53 reduces but does not remove resolver freedom. T's
tap address moved after a crash boot (10.200.42.x to 10.200.43.x); the DNAT
followed, which is good, but it shows the rule is per tap, not per computer.

Estimate. Step 3 of the proof: minutes of owner time. Widening and the close:
about half a day of active work (rewrite the patch, review, floor deploy,
verify), one to two days elapsed with a day of traffic and the cold-boot window.

## SEC-15. Non-root guest runtime, guest sysctls, then required and enforce

Class: red. Protected surfaces: the guest image, the guest daemon (every desk,
capsule and cell), capsule seccomp, document extraction.

What is built.
- Guest sysctls: `boot.kernel.sysctl` in `nix/autoputer-vm.nix:275-296`
  (kptr_restrict, dmesg_restrict, unprivileged BPF off, kexec off, sysrq off,
  perf_event_paranoid, protected links, rp_filter, redirects, syncookies). I
  found no receipt that reads them from a running guest. `user.max_user_namespaces`
  and `io_uring_disabled` were left out on purpose (capsules need user
  namespaces). `route_localnet=1` per tap is host-side
  (`internal/vmmanager/manager.go:3634`), needed by the DNAT to loopback; it
  goes with the vsock work (V6), not this item.
- Document-tool sandbox: `internal/doctool`, uid 61010, empty network
  namespace, Landlock V5, seccomp, own cgroup. `CHOIR_DOCUMENT_TOOL_SANDBOX =
  "observe"` (`nix/autoputer-vm.nix:956`). Observe receipt after real reads:
  runs 2, equal 2, diverged 0, sandbox_failed 0 (deploy P, 10-22 12:52). Two
  runs is a thin sample; "required" is ready when diverged, sandbox_failed,
  setup_failed and over_budget stay zero over real reads.
- Capability floor: `internal/capfloor`, 16 capabilities kept (floor
  `00000000032c15ff`, with `CAP_SYS_TIME` for hibernate resume).
  `CHOIR_RUNTIME_CAPABILITY_FLOOR = "observe"` (line 965). Step 1 re-run PASS
  on deploy Q: cap_bnd `000001ffffffffff`, 25 extra, none missing.
- Obscura removed from the guest image (D2, 10-22 20:57; closure has no
  obscura).
- Not built: the non-root daemon. The FP2 design is an unprivileged API and
  orchestration process plus a small privileged helper over a unix socket with
  peer credentials. The capsule executor is called in process from 55 sites in
  `internal/agentcore` and `internal/autoputer`, so the split cannot be
  carved as one slice.

Reviewed. Bunch 39 (DeepSeek: accept observe; found the `CAP_SYS_TIME` problem),
bunch 43 (flash accepts round 2), Devin accepted the FP2 batch 2 deltas
(bunch 9); Obscura removal bunch 100 (flash, DeepSeek).

Landed. Deploy P (966d3f519, 10-22 12:35), observe. Progress JSON: required and
enforce switches stage 0 ("observe only; owner or FP0 decides when"); non-root
daemon stage 1 ("design residual in the FP2 station file only").

Next step. There is no per-computer selector for either switch (grep of
`internal`, `cmd`, `nix`, `scripts`: the two env names are set only in
`nix/autoputer-vm.nix`). SEC-06 shows the pattern: a host-side selector that
the boot script turns into an environment file. Without one, flipping
`enforce` or `required` is an image change and reaches every computer at its
next guest refresh, the paper included, which the FP2 doc's "separate
decision, each after its own probe" forbids. So:
1. Build a host-side per-computer selector for both switches (red, reviewed
   as additive and behind a selector).
2. Enforce trial on a test computer, FP2 doc "staging step 4": the daemon
   logs `capfloor: enforced`, the confinement probe passes, an engineering
   assignment with egress completes, a management, research and Texture turn
   each complete, then hibernate and resume (a fast resume, no cold-boot
   fallback). Required: more real PDF reads until the four counters are zero.
3. Widen per computer; the paper last.
4. The non-root daemon: a design review first, then the helper split.

Proof scripts: `scripts/proofs/fp2-desk-boundary.sh` (steps 2 and 4 to 6) and
`scripts/desk_cell_confinement_probe.py`.

Needs from the owner. Nothing blocks; the FP2 doc leaves "required or enforce
on the paper" to FP0 or the owner after a probe.

What could go wrong. A dropped capability that something needs (git, go,
node or a library path no source scan found) breaks capsules or desks on that
computer; the enforce trial exists to find it, and the documented fallback is
to add the capability back to `capfloor.Kept`. Under `required`, a document
tool that cannot be sandboxed falls back to the pure-Go extraction and warns,
which can degrade imports on a computer that cannot use Landlock V5.
The daemon is still root until the split exists; the floor bounds what root can
do, it does not make it non-root.

Estimate. Selector and trial for both switches: about a day, active, with a
test computer and some real document reads. Non-root daemon: several days of
design and build (my estimate, not a measured figure; the unit is a privileged
helper behind 55 call sites, with the cell spawn and cgroup writes), so not a
morning item and not on this week's critical path unless the owner wants it
there.

---

## Proposed order, with reasons

Before any deploy (read-only, parallel):
- A. Read the SEC-11 observe journal and the paper's publish cadence over it.
- B. Owner runs SEC-10 step 3 in T's console (three commands, minutes). It has
  no dependency and unblocks the DNS widening.
- C. Check Node B headroom for a fresh test computer (the 03:16 admission
  refusal) and run `vmm_confinement_verify.py --expect direct` on one new
  direct VMM, to close the unrecorded step 1 reading.
- D. Prepare the owner-act gate (a to f) on a test computer.

Deploys, one at a time, each its own lease:
1. SEC-11 enforce. P0, the smallest change with a clear oracle (the journal),
   one-line rollback, no owner input.
2. SEC-12 Dolt stage 3. One line, stage 2 has run clean for about 20 hours,
   cherry-picks cleanly, DeepSeek accepted the replay. Higher blast radius than
   SEC-11 if a root client was missed, so it goes second, with the squash and
   backup consumers checked first.
3. SEC-04 step 2 (jailer on one fresh test computer, steps 1 to 4). It
   touches vmctl, a different surface from 1 and 2, but it changes only a test
   computer's launch. It is the largest security gain on the list once rolled
   out, and the long pole (the lifecycle checks and the global cold boot), so
   start it early. Stop at step 4; step 5 is the owner's call.
4. SEC-12 owner-act enforce, once the gate holds and an owner session is
   available. Alone.
5. NET-01 test-computer grant. Safe and cheap; low direct gain; sequenced after
   the jailer so two vmctl-touching deploys do not overlap.

Waiting on something else:
- SEC-10 close of public 53: waits on step 3 (owner), on every guest being
  cold booted with `choir.dns`, on a rewritten patch, and on a day of traffic.
- SEC-04 steps 3 to 4 (global): wait on step 2 and an owner decision about the
  cold-boot window.
- NET-01 beyond one test computer, and any "no general guest internet": wait on
  the paper's peak-hop count, NET-02 enforce, NET-03 (vsock) and the DNS and
  time routes (NET-04's order).
- SEC-15 switches: wait on a per-computer selector, then a trial. The non-root
  daemon waits on a design review.

Guest-shell steps on the list (owner): SEC-10 step 3 here; the copied-token
refusal (SEC-05 step 5) and the oversized-body test (gateway caps step 11)
are outside this plan but listed in the overnight letter. NET-01 step 5's first
half has a no-shell alternative.

## What I did not check

- No receipt exists for the SEC-11 observe read, the owner-act window, or
  guest sysctls on a running guest; I list each as owed rather than assume a pass.
- Host-side callers of the SEC-11 route were not enumerated from code; the
  journal is the oracle.
- The Dolt root-client list is from grep, not from Node B.
- Node B memory headroom now is unknown to me.
- The `docs/definitions/choir-launch-first-pass-progress.json` snapshot is
  from 02:35Z and is older than the ledger; where they differ I used the
  ledger and `nix/node-b.nix` on main.
