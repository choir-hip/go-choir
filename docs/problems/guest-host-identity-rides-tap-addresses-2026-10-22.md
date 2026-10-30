# Guest and host identity rides tap addresses and firewall rules (2026-10-22)

Status: open. Documented for SDC NET-03 and NET-04
([checklist](../security-density-architecture-checklist-2026-10-21.md)).
Mutation class of this record: green. Design:
[vsock-control-plane-and-nic-removal-2026-10-22.md](../design/vsock-control-plane-and-nic-removal-2026-10-22.md).

## The problem

Every guest-to-host service call and every host-to-guest call is TCP over the
per-VM tap. Who is calling is decided from the packet's source address, and
the source address is only trustworthy while the host's per-tap
anti-spoofing and DNAT rules are installed and correct. Three facts follow,
all read from the code on main d1c60ad87:

1. **Host services read guest identity from `RemoteAddr`.** vmctl maps a tap
   source address to a computer (`internal/vmctl/handlers.go:1141-1201`,
   `client.go:451`, `memory_hotplug_routes.go:136`); the gateway admits a
   bearer from any Choir guest address (`internal/gateway/handlers.go:787`,
   SDC SEC-05); goproxy logs the source address as attribution
   (`internal/goproxy/server.go:232`). A rule that was wrong for one of
   sixteen subnets already happened once
   ([tap-deny-rule-covers-one-of-sixteen-guest-subnets-2026-10-14.md](../archive/problems/tap-deny-rule-covers-one-of-sixteen-guest-subnets-2026-10-14.md)).
2. **The guest reads host identity from its default route.**
   `HostSourcedCaller` and `HostPeerCaller` compare the peer with the
   gateway read from `/proc/net/route` (`internal/server/host_caller.go:31-100`).
   Removing the NIC (NET-04) removes the default route, and these
   predicates then refuse every host call, closed. NET-04 cannot happen
   before the host has another way to prove itself to the guest.
3. **Unix-socket and empty peers are treated as host authority** on both
   sides (`host_caller.go:35, 73`; `vmctl/handlers.go:1221`). Any new
   transport that terminates on a unix socket (a vsock relay does) and
   forwards to these services would be read as the host, or as loopback,
   which is a credential today
   ([loopback-is-a-credential-2026-10-14.md](loopback-is-a-credential-2026-10-14.md)).
   A naive vsock relay would turn every guest call into a host call.

`VsockPort` and `BaseVsockPort` exist and are never read
(`internal/vmmanager/manager.go:116-117, 275-276, 375`); the Firecracker
config has no `vsock` block (`manager.go:1714-1890`).

## Why it matters

- A stolen gateway bearer works from any other computer (SEC-05); the
  binding to a computer is a firewall property, not a credential property.
- The guest NIC carries both the control plane and general internet egress.
  The control plane cannot leave the NIC until it has another transport,
  so egress cannot be closed by removing the NIC (NET-04).

## Evidence class

Code reading only. No packet capture, no staging probe. The inventory of the
TCP paths is [guest-network-inventory-2026-10-21.md](../reports/guest-network-inventory-2026-10-21.md)
section C, re-read for this record.

## Not the fix

Moving bytes from TCP to vsock alone fixes nothing if the receiving side
still reads identity from a peer address string that a relay controls. The
fix is a typed peer identity set by the one component that knows which VM a
socket belongs to, and host-authority predicates that a vsock-origin
request can never satisfy.
