# Guests Never Use the Host Resolver: Nothing Passes choir.dns=

Date: 2026-10-22. Found by reading `internal/vmmanager` and `nix/` on `main`
(a35b3586f) for item SEC-10 of the
[security, density and architecture checklist](../security-density-architecture-checklist-2026-10-21.md).
Station: FP3 group 2
([choir-launch-fp3-groups-2-3-2026-10-21.md](../definitions/choir-launch-fp3-groups-2-3-2026-10-21.md)),
lane `lane/fp3-sec10-guest-dns`. Status: documented; the fix follows in
separate commits. Mutation class of the fix: **red** (VM lifecycle: the
kernel boot arguments of a launch, and through them the snapshot
launch-config hash).

Earlier records: item 6 of
[host-egress-floor-fails-open-2026-10-21.md](host-egress-floor-fails-open-2026-10-21.md)
(guest DNS goes to public resolvers) and the egress table in
[desk-cells-run-as-guest-root-and-egress-is-open-2026-10-14.md](desk-cells-run-as-guest-root-and-egress-is-open-2026-10-14.md)
(line 88), and row D1 of
[guest-network-inventory-2026-10-21.md](../reports/guest-network-inventory-2026-10-21.md).

## The two halves, and the missing link

Both halves of SEC-10 are on main; nothing joins them.

**Guest side** (FP2, 236e816c3). `go-choir-configure-network`
(`nix/autoputer-vm.nix:365-424`) reads `choir.dns=<ipv4>` from
`/proc/cmdline`, accepts only a dotted quad, and appends `DNS=<ip>` to
`/run/systemd/network/10-vm-runtime.network`. With no parameter it writes no
`DNS=`, as before.

**Host side** (FP3 cut 1, f916265e3 and e52a29345; live since deploy M,
8cb7e1acb, per the checklist):

- `nix/node-b.nix:93-100, 672-692`: Unbound listens on
  `127.0.0.1:8053` only; access control allows `127.0.0.0/8` and the guest
  pool (`10.200.0.0/13`, `10.208.0.0/13`) and refuses the rest; private and
  link-local answers are stripped (rebinding); the host's own resolv.conf is
  unchanged.
- `nix/node-b.nix:1451`: vmctl runs with `VMCTL_GUEST_DNS_PORT=8053`.
- `internal/vmmanager/config.go:178-184` reads it into
  `ManagerConfig.GuestDNSPort`; `hostfirewall.go:364` copies it into the
  host network state.
- `hostfirewall.go:172-195` (`tapNATSpecs`): at each VM start, nat
  PREROUTING DNATs `<tap host ip>:53` (UDP and TCP) to
  `127.0.0.1:8053`; `hostfirewall.go:577-581`: the tap's INPUT accepts the
  guest's source to that port. `TestGuestDNSReachesHostResolverOnlyWhenConfigured`
  pins both. Reattach reconciles filter rules, not nat, so a guest running
  across the cut 1 deploy has the DNAT only from its next start.
- `guest_egress.go:87-110` (`guestEgressRuleSpecs`, mangle
  CHOIR-GUEST-EGRESS): UDP and TCP 53 to the guest's own tap address
  RETURN (rules 3 and 4); later, UDP 53 and 123 to any public address
  RETURN (`-p udp -m multiport --dports 53,123`), and all public TCP
  RETURNs (`-p tcp -j RETURN`), which includes TCP 53.

**The missing link.** `buildFirecrackerConfig`
(`internal/vmmanager/manager.go:1908-1980`) writes `ip=`, the `choir.*_url`
parameters, the identity, reclaim and tunnel parameters, and never
`choir.dns=`. `grep -rn 'choir.dns' internal cmd` finds nothing. So no
guest ever gets `DNS=`, Unbound serves no guest query (only the probe's
TCP check in deploy M), and public UDP and TCP 53 must stay open in the
floor.

## What each guest resolves through today

Hypothesis, not verified on a live guest (no guest shell on this worker,
and no nixpkgs source here to read the module default):
the guest image sets `systemd.network.enable = true` and no nameserver
(`nix/autoputer-vm.nix:1090-1091`); the desk-cells inventory says
systemd-resolved, and with no link `DNS=` resolved uses its compiled-in
`FallbackDNS` list (public resolvers such as 1.1.1.1, 8.8.8.8, 9.9.9.9).
Every guest resolver user goes through it: Go's `net.DefaultResolver` in
the runtime (`sourcefetch/policy.go:49`, the capsule `EgressProxy` at
`capsule/egress.go:221`), timesyncd's pool names, git and curl in the
guest. FP0's proof step 4 reads `/etc/resolv.conf` and `resolvectl status`
in a guest and settles this.

The one way the fix could be a silent no-op: if resolved is **not** the
guest's resolver (resolv.conf is not `nameserver 127.0.0.53`), networkd's
`DNS=` reaches nothing and the guest keeps whatever resolv.conf names. The
proof step checks this before step 2 (public 53 closed) is considered.

## Who needs DNS in a guest, and what breaks if public 53 closes

Needs no guest DNS (IP literals or host-side names):

- model calls, search, keyed source APIs: `http://<tap host ip>:8084`
  (gateway);
- corpusd, proxy, vmctl, maild: `<tap host ip>:8086/8082/8083/8087`;
- capsule Go modules: `goproxy.choir.internal` is a route inside the
  capsule egress proxy, mapped to `<tap host ip>:8088`
  (`capsule/egress.go:52-86`), not a DNS name; the package mirror is
  `<tap host ip>:8089`;
- host fetch (`internal/hostfetch`) resolves on the host.

Needs guest DNS, and keeps working through Unbound once `DNS=` is set:

- research Read, Autopaper source polls, content import, podcast feeds
  (`sourcefetch`, guest resolver, then the address guard);
- capsule L2 egress (git over HTTPS CONNECT, pip, npm, Hugging Face,
  arXiv): the capsule has no NIC; the runtime's `EgressProxy` resolves the
  name with the guest resolver;
- timesyncd (`*.nixos.pool.ntp.org`);
- the Management Console's zot and shell, curl and git run by hand.

Breaks if public 53 closes (step 2), or breaks under Unbound:

- anything that names a resolver itself instead of using resolv.conf:
  none in the code (`grep` for 8.8.8.8, 1.1.1.1, 9.9.9.9 and `:53"` finds
  only an isolation selftest dialing 1.1.1.1:443); a guest root or a tool
  that does it by hand loses it, which is the point;
- names whose answers are private or link-local: Unbound strips them; the
  guest's `sourcefetch` refuses those addresses anyway;
- domains with broken DNSSEC: Unbound validates (the NixOS module's root
  trust anchor); the public resolvers in FallbackDNS validate too, so
  no change expected;
- every guest's DNS shares Unbound's fate: an Unbound outage or restart
  (each deploy that touches it) fails all name lookups on the guests that
  use it, with no fallback (resolved uses FallbackDNS only when the link
  has no `DNS=`). Standing question "fate-sharing": the floor admits only
  the own-tap resolver after step 2, so there is no second path by design;
- DNS over HTTPS to a public resolver on 443 stays possible (public TCP is
  open until NET-01); step 2 closes plain DNS, not DoH;
- Tor: nothing in the code or the guest image runs Tor; nothing to break.

Not a break: closing public **UDP** 53 alone leaves public **TCP** 53 open
through `-p tcp -j RETURN`, so step 2 must drop TCP 53 to non-local
destinations explicitly, before the TCP RETURN.

## Snapshots

Kernel boot arguments are fixed at cold boot; a resumed snapshot keeps the
command line it was booted with. Boot arguments are part of the
Firecracker config hashed into `SnapshotMeta.LaunchConfigSHA256`
(`snapshot.go:366`), and `validateSnapshot` rebuilds the expected config
with the current `ManagerConfig` (`expectedLaunchConfig`). So adding
`choir.dns=` to a computer's launch voids its snapshot once
(`snapshotReasonLaunchConfig`) and it cold boots with the resolver; a
computer whose args do not change must keep resuming. That is why the
change must be per computer: `VMCTL_GUEST_DNS_PORT` is already set on every
staging launch, so keying the argument on the port alone would void every
computer's snapshot and switch every guest's resolver in one deploy.

## Fix shape (next commits)

1. Tests (yellow): with the port set and the computer named by
   `VMCTL_GUEST_DNS_ONLY`, both boot paths carry `choir.dns=<tap host ip>`
   once; with the port 0/unset, or the computer unnamed, the fc-config is
   byte-identical to today's and an existing snapshot still validates.
2. Code (red): `guest_dns.go`, the same shape as
   `capsule_tunnel_identity.go`; off unless both the port and the
   selector hold.
3. Not in this lane: remove public 53 from `guestEgressRuleSpecs` after
   one cold-boot cycle and a probe (patch file in the lane worktree,
   uncommitted, for FP0).

Rollback order matters: deselect first (the computer cold boots without
`DNS=`), then drop the port or Unbound. Dropping Unbound or the DNAT under
a guest that has `DNS=` leaves it with no resolver until its next cold
boot.

## Observed on T, 2026-10-23: the fallback stays current beside DNS=

FP0 ran `scripts/guest_diag_probe.py --probe all --expect-udp53 open` from
Node B against T (computer-71afbf1461dffabcce8eeebe558d03bd, the one
computer in `VMCTL_GUEST_DNS_ONLY`) at 17:27 UTC, on a guest that cold
booted at 17:27:30 on c53237618 (D17). Eight checks matched and one did
not:

- matched: `/etc/resolv.conf` is the resolved stub (127.0.0.53); the
  runtime network unit ends with `DNS=10.200.55.1` (the tap host);
  resolved's own server list and the link's DNS servers are exactly the
  tap host; the system resolver answers (example.com, 22 ms); direct UDP 53
  to 1.1.1.1 is open (expected until the SEC-10 close);
- did not match: `resolvectl status` shows the **Global** scope with
  `Current DNS Server: 1.1.1.1#one.one.one.one`, beside the link's
  `Current DNS Server: 10.200.55.1`. The Global section lists resolved's
  compiled-in `Fallback DNS Servers` (1.1.1.1, 8.8.8.8, 9.9.9.9, 1.0.0.1,
  8.8.4.4, 149.112.112.112 and six IPv6 addresses); the guest image's
  `/etc/systemd/resolved.conf` sets no `FallbackDNS=` line
  (`nix eval` of `environment.etc."systemd/resolved.conf".text`).

So the sentence above, "resolved uses FallbackDNS only when the link has
no `DNS=`", is false as a statement about a booted guest: the link has
`DNS=` and a fallback server is still the Global scope's current server.

Hypothesis, not confirmed by a packet trace (Node B has no tcpdump, and
the host floor has no per-port counter for public 53): resolved picks a
fallback as the Global current server when a lookup runs before
networkd has given the link its `DNS=` (early boot, timesyncd), and keeps
it, since its server selection only falls back when no current server is
set. A Global scope with a current server takes part in every lookup that
no routing domain claims, beside the link scope, so plain DNS from a
selected guest still reaches a public resolver today. The SEC-10 close
(public 53 dropped at the floor) would turn those queries into timeouts on
the Global scope, not into failures, because the link scope still answers.
Either way the property SEC-10 step 3 asserts, that no fallback is in use,
does not hold.

Why not set `FallbackDNS=` empty in the image: every guest except T has no
`choir.dns=` and no link `DNS=`, so the compiled-in fallback is its only
resolver today (the paper included). Emptying it image-wide would leave
those guests with no DNS. The fix follows the selector instead:
`go-choir-configure-network` writes a resolved drop-in with an empty
`FallbackDNS=` only when it accepted a `choir.dns=` address, so only a
guest that has the host resolver loses the public fallback. Mutation class:
red (the guest boot path), gated by the existing selector; rollback is
removing T from `VMCTL_GUEST_DNS_ONLY` (it cold boots without `choir.dns=`
and without the drop-in) or reverting the commit.

Evidence: `_runs/d17-proof-guestdiag.json` and `_runs/d17-proof-dns-raw.json`
in FP0's run folder (the probe report and the raw `/internal/diag/dns`
answer, with the full `resolvectl status` text).
