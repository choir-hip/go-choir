# vsock control plane and identity (NET-03), then no guest NIC (NET-04)

Status: design, 2026-10-22. Nothing built. Mutation class of this document:
green. Every build step below is red (vmmanager, vmctl, gateway auth, the
guest image, guest authority predicates) and carries its own ceremony when
built. Problem: [guest-host-identity-rides-tap-addresses-2026-10-22.md](../problems/guest-host-identity-rides-tap-addresses-2026-10-22.md).
Station: [choir-launch-fp10-net-03-04-2026-10-21.md](../definitions/choir-launch-fp10-net-03-04-2026-10-21.md).
Checklist: [SDC NET-03, NET-04](../security-density-architecture-checklist-2026-10-21.md).

Basis: main d1c60ad87, read; FP8 (`lane/fp8-image-jailer` ee2dcf0a3), NET-01
(`lane/fp10-net01-host-fetch`), NET-02 (`lane/fp10-net02-mirrors`) and NET-10
(`lane/fp10-net10-app-network`) read with `git show`, not merged. vmmanager,
vmctl and the image are FP8's while it runs, so nothing here is built.
Line numbers are pointers to read the code at d1c60ad87.

## 1. Decision in five lines

1. Add one Firecracker vsock device per VM. On the host side it is a unix
   socket in the VM's jail directory (Firecracker's "hybrid" vsock), so the
   host needs no `AF_VSOCK`, no `vhost_vsock` and no `vsock_loopback`.
2. **Identity is the endpoint, not an address.** A guest connection arrives
   on that VM's socket and nowhere else. One small host relay,
   `choir-vsockd`, accepts on every VM's sockets and forwards to the host
   service with a typed peer record (computer, realization, CID) that
   vmctl wrote outside the jail. No host service reads identity from a
   peer address string for a vsock caller.
3. **Host-authority predicates never pass for a vsock caller.** The
   relay's upstream is a dedicated unix socket per service, and the
   service's listener wrapper rewrites the peer before any handler runs,
   so today's "unix socket or empty `RemoteAddr` is the host" checks
   (`internal/server/host_caller.go:35,73`; `internal/vmctl/handlers.go:1221`)
   cannot see it. This is the invariant the first red commit tests.
4. The gateway accepts a bearer only when the bearer's computer equals the
   relay's computer (SEC-05's binding, moved from "any guest address" to
   the endpoint).
5. The NIC goes last (§9), in a fixed order, each step reversible, and
   only after every capability in "Networking must enable, not starve"
   has a host route that does not cross the NIC.

## 2. Inventory of TCP paths between guest and host

From [guest-network-inventory-2026-10-21.md](../reports/guest-network-inventory-2026-10-21.md)
section C, re-read, plus the two neighbour ports and the host-to-guest
side. "Identity today" is how the receiver decides who called.

### 2.1 Guest to host (tap, DNAT to `127.0.0.1:<port>`, `internal/vmmanager/manager.go:481-490, 3542-3555`; `hostfirewall.go:150-163`)

| Port | Service | Guest callers | Identity today | On vsock |
|---|---|---|---|---|
| 8084 | gateway | inference, Jev, web search, keyed source APIs, zot base URL; NET-01's host fetch verb rides here | bearer, plus "any Choir guest address" (`gateway/handlers.go:677-699, 787`); Jev pins the address | bearer **and** relay computer must match (§6) |
| 8086 | corpusd | events, releases, credentials, files, key escrow and delivery, projection base, captures and verdicts (no bearer), published search | bearer computer capability; captures and verdicts unauthenticated | bearer and relay computer must match; captures and verdicts take the relay computer as their author (closes the unauthenticated rows) |
| 8082 | proxy | story publish, a plain POST (`agentcore/story_publish.go:406-411`) | none | relay computer |
| 8083 | vmctl | route resolve and apply, effective-release report, memory lease | tap source address mapped to a computer (`vmctl/handlers.go:1141-1201`, `client.go:451`) | relay computer replaces the address lookup |
| 8088 | goproxy | capsule Go module fetches (`capsule/egress.go:52-86`) | none; source address logged | relay computer logged |
| 8089 | pkgmirror (NET-02, on its lane) | capsule npm, PyPI, crates, git | per-computer grant via the credential disk | relay computer checks the grant |
| 8087 | maild | nothing in the guest (inventory Fact 2) | n/a | **not carried**: no vsock port |

### 2.2 Host to guest (`http://<guest-ip>:8085`, `vmmanager/manager.go:953, 1183`)

| Caller | What | Identity today (guest side) | On vsock |
|---|---|---|---|
| vmctl | health, recovery authorities (`vmctl/recovery_authorities.go:60-110`), restart handoff, route projection | `HostSourcedCaller` / `HostPeerCaller`: peer equals the default gateway from `/proc/net/route` (`server/host_caller.go:31-100`) | guest accepts on `AF_VSOCK` port 8085; peer CID must be 2 (host) |
| proxy | owner HTTP, `/api/ws`, `/api/super-console/ws` (`proxy/handlers.go:1236, 1258`), content import (`proxy/platform_publish.go:721`) | same | same; websockets are a byte stream and need nothing else |
| maild | inbound mail push (`autoputer/maildir.go:153-212`) | same | same |
| diag oracle | `/internal/diag/tcp-dial` (`autoputer/boot_timeline.go:585-633`) | `HostPeerCaller` | same; the oracle itself is a NIC tool and goes with the NIC (§9 step N6) |

### 2.3 Not on the tap and not changed

Guest-internal unix sockets (signer, updater, capsule broker, egress
forwarder, delegate, desk-worker socketpair) and the updater's loopback
`127.0.0.1:8085` calls (`cmd/choir-updater/main_linux.go:43-44`) stay as
they are. The updater's loopback calls are in-guest and keep working with
no NIC (the guest keeps `lo`).

## 3. The device and the binding

**Firecracker hybrid vsock.** The config gains
`"vsock": {"guest_cid": <cid>, "uds_path": "/v.sock"}` (inside the jail,
so the host path is `<jail>/root/v.sock`). Host to guest: connect to
`v.sock`, write `CONNECT 8085\n`, read `OK <n>\n`, then the stream.
Guest to host: the guest connects to CID 2 port P; Firecracker connects to
`<jail>/root/v.sock_P` on the host. If nothing listens there, Firecracker
answers with a reset. Cloud Hypervisor implements the same hybrid
protocol, so RES-20's seam carries this unchanged.

**CID.** With hybrid vsock the host never sees the guest's CID: a
connection's identity is the jail it came through. CIDs are still unique
per host (3 + the VM's slot index, the same index that picks its /30),
because unique CIDs cost nothing, make logs readable and keep a later move
to `vhost-vsock` or another VMM honest. "Bound to the VMM-assigned CID"
(the checklist's wording) is met by binding to the endpoint the VMM
created for that CID.

**The binding record.** At VM create, vmmanager writes
`<vm-state>/vsock-binding.json` (computer ID, realization ID, CID, jail
path, boot ID), root-owned, **outside the chroot**: the jailed VMM uid owns
its jail directory and could rewrite anything in it (FP8
`jail_shared.go` makes the same distinction for shared images). The relay
reads the binding, never anything inside the jail except the sockets.
Wiring `VsockPort`/`BaseVsockPort` (`manager.go:116-117, 275-276, 375`):
delete them; ports are fixed per service (§2), not per VM.

## 4. The relay, `choir-vsockd`

A host unit of its own, not part of vmctl or any service:

- **Fate-sharing.** vmctl restarts on most deploys; a relay inside it would
  cut every guest's gateway and corpusd calls for the restart. The relay is
  a byte pump with no Choir dependencies, so its binary changes rarely.
- **Single state authority.** vmctl stays the only writer of which VM is
  which computer (the binding file). The relay is a reader.

What it does, per VM in the binding directory:

1. Listens on `<jail>/root/v.sock_<P>` for each carried port P (§2.1).
   The VMM must connect to it (that is the device), so the socket is
   owned by the relay's uid, group-owned by that VM's jail gid, mode 0660.
   Each jail has its own uid and gid and sees only its own chroot, so one
   VMM cannot reach another VM's relay socket. The relay needs write
   access to a directory the VMM uid owns, so it runs with
   `CAP_DAC_OVERRIDE` and `CAP_CHOWN` only. A compromised VMM can unlink
   its own sockets: that cuts its own guest off and impersonates nobody.
   Ownership under FP8's jailer is verified by the step V1 test, not
   assumed.
2. For each accepted connection, dials the service's relay socket
   `/run/choir/vsock-up/<service>.sock` (mode 0600, relay uid only) and
   writes a PROXY protocol v2 header whose TLVs carry computer ID,
   realization ID, CID and boot ID. Then copies bytes both ways.
3. Holds the guest connection while the upstream dial is refused, with
   backoff, up to `hostlink.RestartWait` (3 min): the relay absorbs a
   service restart that hostlink absorbs today. A dial that never
   succeeds closes the guest side before reading a byte.
4. Bounds: connections per VM (default 256), per-VM bytes per second
   logged, idle timeout. A flood from one guest costs that guest.
5. Removes a VM's listeners when its binding goes away; a new boot ID is a
   new set of listeners.

**Service side.** One shared package (`internal/vsockpeer`, a name to keep
or change) gives each service a `net.Listener` wrapper for its relay
socket. It parses the PROXY header, refuses a connection without one,
and returns a `net.Conn` whose `RemoteAddr()` is a typed `vsockpeer.Addr`
with `Network() == "vsock"` and `String() == "vsock:<computer-id>"`.
Handlers read `vsockpeer.FromRequest(r)` (set through `ConnContext`).
Every host-authority predicate is changed in the same commit to refuse a
request whose connection is a `vsockpeer.Addr` before its other checks.

**Alternatives considered.**
- *Each service listens on the per-VM sockets itself.* No relay, but
  every service then tracks VM lifecycle and the binding, five copies of
  one authority. Rejected.
- *Relay forwards to the service's existing TCP port on loopback.* The
  service would see `127.0.0.1`: loopback is a credential today, so every
  guest call becomes a host call. **Forbidden**; the V1 test asserts the
  relay has no TCP upstream at all.
- *Relay signs an identity header (HMAC) over HTTP.* Works only for HTTP
  and puts a parser in the relay. PROXY v2 is protocol-blind, standard,
  and parsed before any HTTP byte.

## 5. Guest side

- **Listener.** The runtime adds an `AF_VSOCK` listener on port 8085
  (`golang.org/x/sys/unix`, already a dependency at v0.48.0; no new
  module). Each accepted connection's peer CID must be 2; anything else is
  closed. The connection context carries "host peer, via vsock".
- **Predicates.** `HostSourcedCaller` and `HostPeerCaller` gain "the
  connection is the vsock listener's, peer CID 2" as their first and, after
  step V6, only path. Today they need the NIC's default route and refuse
  closed without it (problem record, fact 2), so this must land before the
  NIC can go.
- **No in-guest vsock reach.** The guest kernel config drops
  `vsock_loopback`, so no guest process can reach the guest's own vsock
  listener as CID 1; the CID-2 check stands even so. Units that need no
  host link get `RestrictAddressFamilies` without `AF_VSOCK` (SDC SEC-15);
  the runtime, the updater and the capsule egress forwarder keep it.
- **Dialer.** Host URLs keep their `http://` form so no caller changes; the
  host name becomes a fixed token (`host.vsock`, never resolved) that one
  dialer in `hostlink` maps to `CID 2, port P`. `hostlink.Transport` today
  resends only `ECONNREFUSED` (`internal/hostlink/hostlink.go:59`). A vsock
  connect to a port with no listener fails with `ECONNRESET` instead; the
  rule becomes "resend an error from the dial phase" (an `*net.OpError`
  whose `Op` is `dial`), which keeps its meaning: nothing reached the host.
  That change is tested first (the cases: refused TCP, reset at vsock
  connect, reset after the request was written — the last is not resent).
- **Environment.** `CHOIR_PLATFORM_URL`, the gateway URL and the rest are
  set by vmmanager from kernel parameters (`manager.go:1806-1844`); each
  switches from `http://<tap-host>:<port>` to `http://host.vsock:<port>`
  per service, by a per-computer selector (§7).

## 6. Credentials bound to the endpoint (SEC-05)

`authenticateAutoputer` (`gateway/handlers.go:677-699`) already returns the
bearer's computer ID from `ValidateCredential`. For a vsock caller it then
requires `vsockpeer.FromRequest(r).ComputerID == computerID`, else 403 and
a log line naming both. corpusd does the same for its bearer routes, and
its unauthenticated capture and verdict routes take the relay computer as
the caller (they stop being unauthenticated for vsock callers; TCP callers
keep today's behaviour until step V6). The result is the isolation review's
"bind every bearer to its computer" (§4.6) without a registry write per
boot and without depending on a firewall rule. Rotation (SEC-05's other
half) is unchanged by this design.

## 7. Restarts and snapshots

- **Host deploy.** The relay outlives service restarts (§4 item 3). A relay
  restart drops every guest's open connections once; hostlink resends
  dial-phase failures, and long streams (the owner websocket) reconnect as
  they do after a proxy restart today.
- **vmctl restart.** No effect on existing connections; vmctl reattaches to
  VMs as today and rewrites nothing in the binding.
- **Snapshot and restore.** Firecracker resets the vsock transport when a
  snapshot is loaded: the guest sees `ECONNRESET` on every open vsock
  connection, and listeners survive. The `uds_path` is part of the
  snapshot, so the restore runs with the same jail-relative `/v.sock`
  (FP8's restore args keep the jail layout,
  `vmmanager/confinement.go:309-310`), and the stale socket file is removed
  before load. **A snapshot taken without a vsock device restores without
  one**: the device is added at cold boot only, never on wake, so no wake
  turns into a crash boot. A hibernated computer gets vsock at its next
  cold boot, the same rule FP8's step 4 uses for the jailer.
  *To verify at the pin (1.16.1):* the reset event, and whether restore
  re-binds `uds_path` or needs a `vsock_override`; the V1 local test on a
  KVM host reads this, not the docs.
- **Planned restart marker.** Unchanged; nothing here resumes work.

## 8. CVE review

| Item | What it is | Applies to this design? | Action |
|---|---|---|---|
| Vghost, CVE-2026-74580 (fixed 6.18.45) | local root through `/dev/vhost-net` or `/dev/vhost-vsock` | **No new exposure**: Firecracker's vsock is a userspace device; the host uses unix sockets, never `/dev/vhost-vsock` | blacklist `vhost_vsock` on Node B in the V1 nix change, so no host user can open it; the host kernel patch is SDC's own item |
| VsockDrop, CVE-2026-53365 (2026-08-18) | local root over the vsock loopback transport | Host: no, the host never opens `AF_VSOCK`; blacklist `vsock_loopback` and `vsock` on Node B. Guest: the guest gains `AF_VSOCK`, but a guest-root bug is guest root, which the guest runtime already is | blacklist on the host; drop `vsock_loopback` from the guest kernel; keep the guest kernel patched |
| Firecracker vsock busy-spin (#6031, #6041, #6077), fixed after 1.15.1 | a guest pins a VMM thread | yes: host CPU, one VM's VMM | confirm the fixes are in the pinned 1.16.1 (**unverified**) before V1; the VMM's CPU cgroup bounds it |
| Isolation-runtimes E2 row, "vsock flood" | guest-root device fuzzing | yes, new device surface | relay per-VM connection cap (§4); run the E2 probe on a test computer before V3 |

Net surface: V1 to V6 add one virtio device; N7 removes virtio-net, the
larger and older device, and the tap, DNAT and anti-spoofing rules with
it. The end state has less surface than today.

## 9. The order: vsock first (V), then the NIC (N)

Each step is its own red commit (or nix change), off or observe by default,
selected per computer the way FP8 selects its jailer and VMM build
(cold boot only), reverted by `git revert` or by clearing the selector.
"Cap." names the capabilities of "Networking must enable, not starve" the
step touches; each is proved on a test computer before the step widens.

| Step | Change | Default | Cap. | Proof before widening | Rollback |
|---|---|---|---|---|---|
| V0 | `hostlink` resends any dial-phase error (§5); `vsockpeer` package; predicates refuse a `vsockpeer.Addr` — tests first, no caller | inert | none | unit tests: the three hostlink cases; every host-authority predicate in host and guest refuses a vsock peer | revert |
| V1 | vmmanager adds the vsock device and writes the binding; `choir-vsockd` unit; Node B blacklists `vhost_vsock`, `vsock_loopback`, `vsock` | off (`VM_VSOCK_COMPUTERS` empty) | none | one test computer cold-boots with the device; relay logs its listeners; `/health` over the host-to-guest socket; snapshot, wake and a vmctl restart on that computer | clear the selector; the next cold boot has no device |
| V2 | host to guest over vsock: vmmanager's `HostURL` dialer, guest vsock listener with the CID-2 check | observe: both paths, TCP serves, vsock result compared and logged | owner HTTP, websockets, mail push, recovery | a day of zero mismatches on the test computer | selector |
| V3 | guest to host per service, gateway first: relay sockets, `host.vsock` URLs, the §6 binding check | per service per computer; TCP still open | C1-C3 (gateway: inference, search, host fetch, keyed APIs), C5 (goproxy, mirrors) | the four capability probes the NET-01 and NET-02 stations already wrote (`scripts/host-fetch-proof.py`, `scripts/capsule-mirror-probe.sh`), plus a bearer taken from computer A refused when sent from computer B's endpoint | selector per service |
| V4 | corpusd, vmctl, proxy, then goproxy and pkgmirror onto vsock | as V3 | events, releases, self-development, story publish, capsules | the paper's schedules and polls before and after on its own selector step; an autopaper computer moves last | selector |
| V5 | every computer selected at its next cold boot | on | all | per-port tap counters (nft) read zero for control ports over a week | selector |
| V6 | tap DNAT for 8082-8088 removed; TCP paths in the host predicates and the guest predicates deleted | — | all | the counters stay zero; deployed acceptance of the seven capabilities | revert (the rules come back) |
| N1 | NET-01's host fetch verb on for every computer, guest-side fetch route deleted | per NET-01 | C1, C3 | NET-01's proof | NET-01's |
| N2 | NET-02 mirrors only for capsules; L2 CONNECT to the internet off | per NET-02 | C5 | NET-02's probe | NET-02's |
| N3 | the delegate's model API through the gateway (SEC-08 Option K) | per SEC-08 | C5 | delegate one-round build | SEC-08's |
| N4 | the Management Console's zot and shell (inventory A9): **removed** (owner, 2026-10-22: "just remove zot") | — | none of the seven | — | — |
| N5 | time without NTP: kvm-clock plus `ptp_kvm` and chrony reading `/dev/ptp0` (**unverified** on Firecracker 1.16.1 x86_64; fallback: a host time verb over vsock); resolver none (nothing in the guest resolves once N1-N3 hold) | per computer | all, indirectly (TLS, token expiry) | clock offset logged before and after on a test computer for a day | selector |
| N6 | the tap's forward chain drops everything for selected computers; the NIC is present and dead; the diag oracle is removed | per computer | all | a week of the paper and the owner computer with zero dropped packets that matter (drops logged by destination) | delete the rule |
| N7 | no `network-interfaces` block, no tap, no `/30`, no `ip=` parameters for selected computers; cold boot only | per computer, test first, then all; hibernated computers cold-boot once | all | the seven capabilities on staging; the paper a full day | selector; the next cold boot has a NIC |

**Why this order.** V2 before V3 because the guest's host predicates need
the NIC's default route (problem record fact 2), and moving the host
side's calls first keeps every guest-authority route alive. V0 before V1
because the relay invariant (§1 item 3) is the whole security value and
must exist before any byte crosses. N1 to N5 before N6 because "moving a
path to the host moves where the check runs, not what is reachable": each
egress row needs its host route before the NIC stops carrying it. N6
before N7 because a dead NIC is reversible by one rule and finds the path
nobody listed; removing the device is reversible only at the next cold
boot.

**C4 and C6, C7.** Web Lens live inspection (C4) no longer fetches from
the guest (Obscura deleted, FP6); its Source Viewer snapshot is C3's
content import. Mail (C6) needs no guest egress (inventory Fact 2) and its
push is host to guest (V2). App agents (C7): NET-10 rules that ingress
never connects into the guest and egress is a declared grant enforced at
the host, so it rides N1 and NET-05's host proxy; no NIC needed.

## 10. What this design does not settle

- **A9, the console's zot and shell** (N4): settled by the owner
  (2026-10-22 02:25 UTC): "For the zot question, we should actually just
  remove zot." zot is deleted (the Console's repair window, its
  `/api/super-console/ws` route, `internal/zot`, `cmd/zot`, the
  `zot-session` mode), so N4 needs no egress row.
- **Unverified at the pin**, each read on a KVM host in V1 before it is
  relied on: the snapshot reset and `uds_path` re-bind (§7), the
  busy-spin fixes in 1.16.1 (§8), `ptp_kvm` (N5), the vsock socket's
  ownership under the jailer's uid and gid (§4 item 1).
- **Relay restarts** drop every guest's open streams at once. If the relay
  binary changes often enough to matter, it gets a socket-activation
  handoff; measure first.
- **Within-guest separation** is unchanged: any guest-root process can
  open `AF_VSOCK` to the host, as it can open TCP today. Identity is per
  computer, not per process; desk-cell confinement and SEC-15 own the rest.
- **Rotation** of the gateway bearer (SEC-05) stays open; this design
  binds it, it does not shorten it.

## 11. Proof steps for FP0 when built (summary)

Per step, on a test computer selected by its selector, never the paper
first: (1) cold boot and read the relay's listener log and the binding
file's owner and mode; (2) host-to-guest `/health` through the socket and
the owner's desktop websocket open; (3) the gateway, host fetch and mirror
probes the neighbour stations wrote; (4) the cross-computer bearer test —
read computer A's bearer through an owner-authorised route only if one
exists, otherwise a test computer's own token replayed through a second
test computer's socket by FP0 on the host; (5) snapshot, wake, and a
vmctl restart, then (2) again; (6) the paper's schedules and polls before
and after every widening.
