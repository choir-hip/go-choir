# The Host Egress Floor for Guest Taps Fails Open

Date: 2026-10-21. Found by re-reading `internal/vmmanager` on `main`
(b489ed48f) for items SEC-03, SEC-09, SEC-10 and NET-07 of the
[security, density and architecture checklist](../security-density-architecture-checklist-2026-10-21.md).
Station: FP3 groups 2 and 3
([choir-launch-fp3-groups-2-3-2026-10-21.md](../archive/definitions/choir-launch-fp3-groups-2-3-2026-10-21.md)).
Status: documented; the fix follows in separate commits. Mutation class of
the fix: **red** (host firewall for guest taps, vmctl).

The earlier records stay the history:
s0-tap-egress-unfiltered-2026-10-01.md
(the first probe),
[tap-deny-rule-covers-one-of-sixteen-guest-subnets-2026-10-14.md](../archive/problems/tap-deny-rule-covers-one-of-sixteen-guest-subnets-2026-10-14.md)
(guest to guest, fixed by efbd69e27), and
[desk-cells-run-as-guest-root-and-egress-is-open-2026-10-14.md](desk-boundary-residuals-2026-10-21.md)
Problem 2 (the egress inventory and the mangle design, code 61f4da20f on
`origin/land/desk-cell-confinement`, never landed).

## Evidence (code, main at b489ed48f)

1. **A rule that fails to install only logs.** `setupHostNetworking`
   (`internal/vmmanager/manager.go:3476-3512`) logs a warning when
   `ensureTap` or `ensureNAT` fails and returns nil; `prepareVMNetwork`
   (`:2129-2145`) logs again and returns nothing; boot
   (`launchFirecracker`, `:2103-2109`) and snapshot restore
   (`snapshot_lifecycle.go:522, 534`) start Firecracker anyway. A guest
   whose isolation rules did not install runs with whatever the host
   tables hold.
2. **Replies are not the only traffic towards a guest.** The per-tap
   FORWARD rule is `-o <tap> -j ACCEPT` (`hostfirewall.go`,
   `tapForwardReturnSpec`): anything the host forwards to the tap is
   accepted, not just replies to the guest's own connections (review F8,
   ACTIVE item 1's unbuilt half).
3. **The guest reaches host ports beyond its services.** Only the INPUT
   rules for the tap service ports are source-bound; there is no rule that
   drops the rest. `nix/node-b.nix:524-530` opens 22, 80 and 443 on every
   interface, so a guest reaches sshd and Caddy on its tap address and on
   the host's public address. The service ACCEPTs also match any
   destination, so guest A is admitted to tap B's host address on a
   service port (nothing listens there today because services bind
   loopback; the rule should not depend on that).
4. **Guest egress to private, link-local, CGNAT and metadata ranges is
   open.** POSTROUTING masquerades the guest /30 to any destination
   (`tapNATSpecs`). Only the guest pool itself is dropped (CHOIR-GUEST-FWD).
5. **IPv6:** guests get no routable IPv6 and forwarding is off, but sshd
   listens on `::` and a guest root can bring up link-local on its NIC;
   nothing on the host drops guest IPv6.
6. **Guest DNS goes to public resolvers** (SEC-10). The guest network unit
   written by `go-choir-configure-network` (`nix/autoputer-vm.nix:357-366`)
   has no `DNS=`, so resolved falls back to its compiled-in public servers;
   the host has no resolver for guests and the floor would have to leave
   UDP 53 open to the world.
7. **KSM** (SEC-09): nothing sets `/sys/kernel/mm/ksm/run`; the last
   reading (`run=1`, nothing merged) predates the 10-15 host reboot.
   Comments still say KSM dedups guests: `internal/vmmanager/manager.go:259-262`,
   `nix/autoputer-vm.nix:290-292, 303-309`, `flake.nix:525, 608`. KSM is a
   cross-guest side channel (review N4); the comments invite turning it on.
8. **`/dev/kvm` is mode 0666** (NET-07): systemd's default udev rule gives
   every local user KVM. Only vmctl's direct VMMs (root) and the jailer's
   per-VM device nodes need it.

## Capabilities the fix must keep (checklist "Networking must enable")

Research reads any public URL; web search; source ingestion (RSS,
Telegram MTProto over TCP, GDELT, podcasts, YouTube, content import);
engineering's goproxy (tap 8088) and the L2 proxy; Mail through maild
(guest caller: `choir.maild_url`, tap 8087, maild draft persistence, so
8087 stays); the gateway (8084), vmctl (8083), corpusd (8086), proxy
publish path (8082). Public internet TCP egress stays open until NET-01.
Nothing in the guest calls the host's own 80/443 (checked: no guest code
dials `choir.news`); the cost of dropping them is that a guest cannot read
`https://choir.news` pages through the public address.

## Decided defaults (most restrictive that keeps the list above)

- Host egress floor: the mangle chain from 61f4da20f, enforcing, with
  host 80/443 removed, and a DNS exception added for the guest's own host
  resolver (UDP and TCP 53 on the tap host address; public UDP 53 stays
  until the guest-side line lands); replies only on `-o <tap>`; INPUT
  service ACCEPTs bound to the DNAT'd loopback destination.
- Every install step (filter, nat, mangle, IPv6) fails the VM start. A
  cold boot tears the tap and its rules down; a snapshot restore does not
  start the VMM and falls back to a cold boot, which reuses the tap, re-runs
  the install, and fails (and tears down) if the floor still does not
  install. Reattach of an already-running guest logs
  and keeps it running (a crash-free computer is not stopped for a rule).
- Guest DNS: a host-side Unbound on loopback, reached at the tap host
  address port 53 through DNAT, answering only the guest pool, with
  private answers stripped (rebinding). The guest-side `DNS=` line belongs
  to FP2 (guest side of `nix/autoputer-vm.nix`); vmctl passes the resolver
  in the kernel `ip=` parameter's dns0 field so the guest change is one
  line. Until it lands, guests keep resolving through public UDP 53, which
  the floor still admits.
- KSM: written to 2 (stop and unmerge) at boot; comments corrected.
- `/dev/kvm`: 0660 root:kvm. SMT stays on (owner ruling, NET-07: until
  strangers share the host); core scheduling and `nosmt` are recorded as
  owed, not built.

## Residuals

- Public TCP egress stays open (NET-01 owns the host fetch service).
- SMT, core scheduling (DEC-06).
- Guest-side DNS line (FP2), then public UDP 53 dropped.
- Local Nix builds that need the `kvm` feature (node-a takes those; a
  local fallback build would lose `/dev/kvm` unless nixbld is in `kvm`).
