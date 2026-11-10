# X6: a vsock echo spike on d@e, what can be measured now (2026-11-10)

Mutation class: green (evidence only). Mission item X6 of
[choir-autopaper-overnight-2026-11-09.md](../definitions/choir-autopaper-overnight-2026-11-09.md):
"a vsock echo spike on d@e". The design questions it serves:
[vsock-control-plane-and-nic-removal-2026-10-22.md](../design/vsock-control-plane-and-nic-removal-2026-10-22.md)
(NET-03, identity from the endpoint; NET-04, no guest NIC),
[outboard-capsules-arch-11-12-2026-10-22.md](../design/outboard-capsules-arch-11-12-2026-10-22.md)
(a relay over vsock is the only path a leased machine has), and the
[resource fabric](../design/resource-fabric-2026-10-31.md) (the server-side
vsock control plane's shape is the model for the fabric session). No secret,
key, token or address of the owner appears here. Acts: read-only on d@e
(one prompt-bar run, nine read-only commands, no delegate); read-only on
node-a (one small socket client, nothing installed); nothing on Node B.

## Summary in plain language

**A real echo measurement on d@e is not possible today, and the reason is not
a missing tool: d@e has no vsock device.** The guest's Firecracker config has no
`vsock` section at all. The vsock design is "nothing built". The host reaches a guest over a
tap device and TCP (`http://<guest-ip>:8085`, a per-VM /30), and the guest
reaches host services through DNAT to loopback ports. So there is no
host-guest vsock path on d@e to time.

What I could establish instead:

1. **From inside d@e (a capsule, through the product):** the guest kernel
   (6.18.57) has the whole vsock stack loaded (`vsock`,
   `vmw_vsock_virtio_transport`, `vmw_vsock_virtio_transport_common`,
   `vsock_loopback`), and `AF_VSOCK` is registered. But the capsule cannot use
   it: `socket(AF_VSOCK)` fails with `EPERM` (seccomp, 2 filters, no new
   privs), `/dev/vsock` does not exist, `/sys` is empty, and even netlink is
   refused. [M]
2. **The only live host-guest vsock path I can reach is the agents VM** (cloud
   hypervisor on node-a, hybrid vsock, `ssh choir-agents` goes through it).
   Measured against it, host to guest: opening a vsock connection to a
   listening guest port and getting the guest kernel's accept takes a median of
   about **0.24 to 0.29 ms** (min about 0.11 ms, p99 2 to 4 ms), over three runs of
   about 270 connections each. [M]
3. **Throughput could only be bounded from below**, because the only way to
   push bytes through is ssh from my Mac over the internet to node-a. The
   vsock hop kept up with that internet path (about 27 to 32 MB/s at best,
   same as the direct ssh to node-a). The vsock ceiling is above that and
   unmeasured. [M for the floor; the ceiling is not measured]
4. **The tap-and-TCP path on d@e was not measured**, so there is no
   side-by-side number. Doing so needs the host (Node B), which this session
   may not touch. [not possible]

## Method

### What is possible without changing Node B or the guest image

Read from code (`internal/vmmanager`, `nix/`):

- The VM config sections are `boot-source`, `drives`, `machine-config`,
  `network-interfaces` and `balloon` (`internal/vmmanager/confinement.go`,
  `confinement_test.go`, `balloon_test.go`). There is no `vsock`. The only
  vsock references in the tree are the struct fields `VsockPort` and
  `BaseVsockPort: 6000` in `internal/vmmanager/manager.go` (declared, unused), and a design that says
  "Nothing built".
- The host to guest path is `http://<guest-ip>:8085` over the tap
  (`vmmanager/manager.go`); guest to host is the tap with DNAT to
  `127.0.0.1:<port>` for gateway 8084, corpusd 8086, proxy 8082, vmctl 8083 and
  others (design section 2.1, 2.2).
- The agents VM is a different thing: node-a, cloud hypervisor with a hybrid
  vsock socket `/var/lib/microvms/agents/notify.vsock` (mode 0700 microvm), and
  the local helper `choir-agents-vsock`, a 20-line python program that
  connects to that unix socket, writes `CONNECT 22`, waits for `OK <port>`, and
  hands the stream to socat. That is the "reached through node-a over vsock"
  path.

### On d@e, through the product

One prompt-bar run (`POST /api/prompt-bar`, command id
`x6-vsock-capsule-facts-2026-11-10`, trajectory
`78fe45a0-a451-5f37-af23-1c3b3a1ce84e`): Texture, then management, then an
engineering assignment (`run:assignment-5ef1855976324f0b0b9df590`) that ran
nine read-only commands with `choir.Exec` in its capsule, plus a labelled
supplemental read-only set to explain why `/sys` and `/dev` looked empty. No
delegate, no source change, no freeze, no apply. 11:37:25 to 11:40:09 UTC (2.7
minutes). The engineer also recorded two precommit claims on d@e's tape.

### On the agents VM

`vsock_rtt.py` (run over ssh stdin on node-a with the existing python from the
`choir-agents-vsock` shebang; nothing written or installed): for 300
iterations, 20 ms apart, open a unix connection to `notify.vsock`, send
`CONNECT 22`, time until `OK <port>`, then time until the sshd banner line.
Three runs. In cloud hypervisor and Firecracker alike, the `OK` is returned
when the guest kernel answers the vsock connection request, so
`connect_to_OK` is a host to guest to host round trip through the virtio
queues and the guest's vsock driver. It is **not** an application echo: the
guest kernel answers it, not a user process. `OK_to_sshd_banner` is dominated
by sshd's per-connection startup and says nothing about the transport.

Throughput (`tp.py`, from my Mac): `ssh -o Compression=no HOST 'head -c 100000000 /dev/zero'`
timed (download), and 100 MB piped to `wc -c` (upload), against `node-a`
directly and against `choir-agents` (which adds the vsock hop and a second ssh
session on top of the first). Alternated, three downloads and two uploads
each. ssh crypto, my uplink and the internet are all in the number.

## Results

### Host to guest connect round trip over vsock, agents VM [M]

| Run | n ok (of 300) | uds connect p50 | connect to OK min / p50 / p90 / p99 / max |
|---|---|---|---|
| 1 | 278 | 43 us | 109 / 237 / 1,277 / 4,009 / 10,272 us |
| 2 | 270 | 49 us | 113 / 276 / 717 / 2,481 / 2,867 us |
| 3 | 269 | not kept | 117 / 294 / 692 / 2,022 / 2,232 us |

`OK_to_sshd_banner` p50 was 33 to 47 ms and p99 over 1.6 s: that is sshd, not
the transport. 22 to 31 of 300 connects per run failed within 5 s, all in the
banner phase (consistent with sshd's startup throttling under 20 ms spacing; I
did not find the cause and did not tune).

### Throughput through the agents VM vsock, lower bound [M]

| Direction | node-a direct (MB/s) | via choir-agents, vsock hop (MB/s) |
|---|---|---|
| download, 100 MB x3 | 29.3, 31.3, 32.0 | 4.0, 3.7, 27.4 |
| upload, 100 MB x2 | 26.5, 28.2 | 6.8, 22.9 |
| `ssh true` wall time (5 each) | 0.36 to 0.48 s | 0.77 to 0.92 s |

The via-agents numbers swung from 4 to 27 MB/s between consecutive runs
(the slow ones overlapped other work of mine on the same link and the agents
VM runs other agents' jobs). The best run equals the direct path, so the vsock
hop is not the limit at about 30 MB/s; the number says nothing about its
ceiling. The extra 0.4 s on `ssh true` is the second ssh handshake (key
exchange and auth inside the tunnel), not vsock.

### Inside d@e's capsule [M]

| Check | Result |
|---|---|
| kernel | 6.18.57 |
| `/proc/net/protocols` | `AF_VSOCK 1248 ...` registered |
| `/proc/modules` | `vsock`, `vmw_vsock_virtio_transport`, `vmw_vsock_virtio_transport_common`, `vsock_loopback`, all Live (the `(E)` flag is the kernel's unsigned-module taint mark; whether they were loaded at boot or autoloaded later is not known) |
| `/dev/vsock`, `/dev/vhost-vsock` | absent (`/dev` holds only null, zero, random, urandom) |
| `/sys/bus/virtio/devices`, `/sys/class/vsock` | absent (`/sys` is an empty directory) |
| `python3 socket.socket(AF_VSOCK)` | `PermissionError: [Errno 1] Operation not permitted` at create |
| local CID ioctl | not reachable: no `/dev/vsock` |
| `ip -brief addr`, `ip route` | `Cannot open netlink socket: Operation not permitted` |
| capsule hardening (supplemental) | `Seccomp: 2`, 2 filters, `NoNewPrivs: 1`, `CapEff 0x2011cb`, `/proc` hidepid=invisible |

Read: the guest kernel can speak vsock; the capsule is denied it by seccomp,
and the guest (not the capsule) has no virtio vsock device to connect to
because the VM config declares none. I could not read the guest's own
`/sys/bus/virtio` (a capsule sees an empty `/sys`), so "no vsock device in the
guest" is from the VM config in code, not from a guest-side listing.

## Measured vs estimated vs not possible

| Quantity | Status |
|---|---|
| d@e host-guest vsock RTT | **not possible** (no device; creating one changes the Node B VM config and the guest listener) |
| d@e host-guest vsock throughput | not possible, same reason |
| d@e host-guest TCP-over-tap RTT and throughput | not possible without Node B; estimate only: the tap is a same-host virtio-net path, so expect tens of microseconds RTT and gigabit-class throughput [E, no measurement] |
| agents VM vsock connect RTT | measured: p50 0.24-0.29 ms, min 0.11 ms, p99 2-4 ms |
| agents VM vsock throughput | bound only: at least about 30 MB/s through ssh; ceiling unknown |
| whether d@e's guest kernel supports vsock | measured: yes (modules loaded) |
| whether a capsule or cell can open AF_VSOCK | measured: no (EPERM) |

The comparison "vsock versus tap TCP" that X6 asked for is therefore not
available. The agents VM numbers are on a different hypervisor (cloud
hypervisor, not Firecracker), a different host (node-a) and a busy guest, so
they bound what a hybrid vsock costs, not what d@e would see.

## What a d@e spike would need

Nothing here is a product path today; each item is a build or an owner act.

1. **A vsock device on d@e's VM.** Add `"vsock": {"guest_cid": 3+slot, "uds_path": "/v.sock"}`
   to the generated Firecracker config in `internal/vmmanager` (red: vmmanager,
   guest boot path). This requires a Node B deploy and a d@e refresh. The
   design's step is V1 of NET-03.
2. **A guest listener.** A small `AF_VSOCK` echo server on a fixed port in the
   guest. It must run in the guest runtime, not a capsule (the capsule's
   seccomp forbids `AF_VSOCK`, and the design says units without a need get
   `RestrictAddressFamilies` without `AF_VSOCK`). The cleanest owner is a
   spike-only goroutine behind a debug route on d@e's own image, deleted after.
3. **A host peer.** Firecracker connects the host side to the unix socket
   `<jail>/root/v.sock` (host initiated: write `CONNECT <port>\n`, read `OK`,
   then the stream). The peer is a ~40-line client, run as root on Node B (the
   jail's directory is owned by the jailed uid). Authority: the Node B
   operator (owner or an agent with explicit Node B authority), because the
   socket is under the jail directory. The one command (hypothetical, after 1
   and 2 exist):
   `choir-vsock-echo-bench --uds <jail>/root/v.sock --port 5001 --rtt 2000 --bytes 1073741824`
   printing RTT percentiles for 64-byte round trips and sustained MB/s one way
   each direction.
4. **The same bench over TCP**, to the guest's `<ip>:8085`-style port, from the
   same host process, so the two numbers share a clock and a CPU.

The numbers it would settle: (a) per-frame RTT of a vsock relay, the cost the
outboard-capsule and fabric designs hang on; (b) whether a hybrid-vsock
bytes-per-second ceiling is above the websocket and corpus traffic the proxy
and corpusd already push over the tap; (c) the connect cost that
`hostlink.Transport`'s dial-phase retry will pay per resend.

A cheaper partial spike needs no Node B change: a cloud-hypervisor guest on
node-a already has hybrid vsock. A throwaway echo service on the agents VM
(its own owner's call) would give the RTT and throughput of the same
mechanism with no change to d@e or the paper. I did not do it: it needs a
process started on the agents VM, which is outside "no installs" for a
borrowed machine and not d@e.

## Limits

- The connect round trip is the vsock handshake the guest kernel answers; an
  application echo adds a scheduler wakeup on each side. Expect the real echo
  RTT above these numbers by tens of microseconds.
- Throughput was measured end to end through ssh over the internet; the bound
  is weak.
- The agents VM is shared and was in use; the p90 and p99 tail is likely
  contention.
- The guest's own device list was not read: capsule `/sys` is empty.
- One host (node-a), one hypervisor (cloud hypervisor), one afternoon.

## Artifacts left on staging

- d@e trajectory `78fe45a0-a451-5f37-af23-1c3b3a1ce84e`: Texture document
  `1fe0255c-f7e5-53e0-ab5e-0abb235fa2d0`, engineering assignment
  `run:assignment-5ef1855976324f0b0b9df590` (completed, finding, no change to
  apply). Two precommit claims on d@e's tape. No capsule file survives.
- node-a: nothing written, no process left.
- Node B: untouched.

## What it suggests

- The NET-03 design's "the guest adds an AF_VSOCK listener" is the one place
  the design must also decide the capsule rule: the capsule seccomp denies
  `AF_VSOCK` today, which is right for the capsule and means the vsock
  relay can never be reached by a cell or a capsule job. The first d@e echo is
  a guest-runtime job.
- The guest image already carries the vsock modules, so the guest needs no
  kernel change; the work is the VM config device, the listener, and the
  host relay. `vsock_loopback` is loaded in d@e's guest, which the design
  (section 5) wants dropped so no guest process can reach its own listener as
  CID 1; that is a cheap, separate finding to act on.
- The measurable cost of the transport is small (about a quarter millisecond
  to connect on a busy cloud-hypervisor guest). It is not the reason to
  choose or reject vsock; identity and the removal of the NIC are.
