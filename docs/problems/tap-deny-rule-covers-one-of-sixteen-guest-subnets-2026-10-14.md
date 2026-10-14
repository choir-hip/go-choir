# The tap-to-tap deny rule covers 1 of the 16 guest subnets (2026-10-14)

Status: open; found by reading code. **Not confirmed on Node B**: no host
state was read. Found while researching
[reports/self-hosting-and-desktop-2026-10-14.md](../reports/self-hosting-and-desktop-2026-10-14.md).
Mutation class of the fix: red (vmctl, guest isolation; the S1a security
floor).

## Findings (code, `main` f54f05c42)

**The guest address pool spans 16 /16s.**
- `internal/vmmanager/manager.go:370-373` defines:
  - `vmSubnetSecondOctetBase = 200`
  - `vmSubnetSecondOctetCount = 16`
  - `vmSubnetCapacity = 4096`
- `guestAndHostIP` (`manager.go:402-411`) maps host-port ordinal `n` to
  `10.(200 + n/256).(n%256).2`.
- So guests live anywhere in `10.200.0.0`-`10.215.255.x`.

**The tap-to-tap deny names only the first /16.**
- `ensureTapIsolationRules` (`manager.go:3379`) inserts
  `FORWARD -i <tap> -d 10.200.0.0/16 -j DROP`.
- Its comment (`:3347`) calls `10.200.0.0/16` "the shared guest space".
- Cross-tap traffic to destinations in `10.201`-`10.215` therefore falls
  through to the per-tap `FORWARD ACCEPT` rules appended by
  `setupHostNetworking`.

**Ordinals of 256 or more are reachable in practice.**
- `allocateHostPortLocked` (`manager.go:456-470`) advances `nextPort`
  monotonically through the whole capacity.
- It reuses a port only when its scan comes back around to it.
- *Hypothesis:* on a long-lived vmctl process, after more than 256
  allocations, new guests get `10.201+` addresses.
- An archived record shows a live guest at `10.201.201.2` in July 2026
  ([archive/choir-audited-autoputer-construction-2026-07-15.md](../archive/choir-audited-autoputer-construction-2026-07-15.md)).
  That was under an older scheme and does not prove the current one.

**The anti-spoof rules are unaffected.** They are per-tap `/32` source
checks (`manager.go:3376-3377`).

## Likely effect (hypothesis until host rules are read)

- A guest at `10.200.x` can open connections to guests at `10.201+`.
- Guests at `10.201+` can reach each other.
- They cannot reach `10.200.x` guests.
- This breaks the S1a boundary "no guest may forward into the shared guest
  space" ([definitions/choir-appdev-s1-security-floor-2026-10-01.md](../definitions/choir-appdev-s1-security-floor-2026-10-01.md):328).
- Whether it is live depends on whether any current guest holds a
  `10.201+` address, and on whether other host rules drop the traffic
  first.

## Evidence to collect before a fix (read-only on Node B)

- `iptables -S FORWARD`
- `ip -4 addr show | grep 'inet 10\.2'`
  - Do any taps carry `10.201+`?
  - Does any rule other than the 10.200 deny stop cross-tap forwarding?

## Fix shape (not applied)

Pick one:
- Deny the pool's real extent: `10.200.0.0/12` covers `10.192-10.207`, so
  it is not exact. Either use two rules (`10.200.0.0/13` plus
  `10.208.0.0/13`), or derive the CIDRs from the constants.
- Change the deny from destination-based to egress-interface-based:
  `-i <tap> -o tap+ -j DROP`.

The self-hosting report also asks for a configurable base, so that guest
subnets do not collide with LANs and VPNs on desktops and clouds. Any
fix should derive the deny from the same constants.
