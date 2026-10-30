# The tap-to-tap deny rule covers 1 of the 16 guest subnets (2026-10-14)

Status: fix committed on branch `fix/tap-deny-guest-range`; not pushed and
not deployed (see "Fix" at the end). Found by reading code. **Not confirmed on Node B**: no host
state was read. Found while researching
[reports/self-hosting-and-desktop-2026-10-14.md](../../reports/desktop-sync-and-local-vms-2026-10-14.md).
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

## Live check on Node B (lead, 2026-10-14 ~15:55 UTC, read-only)

- Three guest taps exist (vm-dhdbzl4tzqi3, vm-vh3spxieztky,
  vm-lackej5pmie2) with host addresses 10.200.112.1/30, 10.200.113.1/30
  and 10.200.114.1/30. Each has its two DROP rules (destination
  10.200.0.0/16; source other than its own guest) ahead of its ACCEPT
  rules, so **today's guests are isolated**: all live addresses are inside
  10.200.0.0/16. The gap is latent; it opens when an allocation reaches
  10.201.0.0 or above.
- Separately: FORWARD holds 122 stale ACCEPT rules for interfaces of an
  older naming scheme (`vm-vm-<hash>-tap`), none of which exists now. They
  match nothing today, but rules for vanished taps are never removed —
  a hygiene leak to fix with the deny-range change (remove a tap's rules
  when the tap is torn down; sweep orphans at vmctl start).
- The FORWARD policy is ACCEPT, so isolation rests entirely on per-tap
  rules.

## Second live read and code read before the fix (2026-10-14 ~15:53 UTC, read-only)

Read with `sudo iptables -S`, `sudo iptables -t nat -S`,
`sudo ip6tables -S FORWARD`, `ip -o addr show`, and `sysctl` on Node B.
Nothing on the host was changed.

**The pool's upper /16s have been used.** Stale nat rules remain for two
vanished taps of the older naming scheme: `vm-vm-5b0c1-tap` held
10.202.16.1 and `vm-vm-unive-tap` held 10.202.17.1. So allocations have
reached 10.202 before. That was under the older scheme, but it shows that
reaching 10.201+ is not hypothetical.

**What is stale, by chain.** None of these interfaces exists any more.
- FORWARD: 61 untagged `-i`/`-o` ACCEPT pairs (122 rules) for
  `vm-vm-<prefix>-tap`.
- INPUT: 15 rules. Fourteen are tagged
  `go-choir-vm-vm-vm-{5b0c1,unive}-tap`; one is untagged
  `-i vm-vm-eefb4-tap --dport 8084`.
- nat: 20 rules (PREROUTING DNAT, OUTPUT DNAT, POSTROUTING MASQUERADE,
  with the retired `-o lo` masquerade among them), tagged for the same
  two taps. They include DNATs to 127.0.0.1:8085 and :8787, ports that
  have since been retired.

**A related ordering defect in INPUT.** For each live tap, the six
tap-to-host-service ACCEPTs (8082-8088) come *before* that tap's INPUT
anti-spoof DROP. The cause is that `setupHostNetworking` inserts the DROP at
INPUT 1 first, and `ensureTapHostServiceInputRules` then inserts each
ACCEPT at INPUT 1, which puts them ahead of it. The ACCEPTs have no source
match. So a packet entering tap T with a forged source reaches host
service ports, against the S1a rule "only the guest's own /30 source may
enter this tap". Blind TCP spoofing cannot finish a handshake, because the
SYN-ACK goes to the forged address, so this is hard to exploit. It still
breaks a stated invariant.

NixOS's reverse-path filter narrows the exposure further. mangle
PREROUTING jumps to `nixos-fw-rpfilter`, which ends `-m rpfilter ... -j
DROP` with no `--accept-local`. A forged source whose return route does not
leave through the same tap (another guest, an outside address) is dropped
there. So is a source that is a local host address, such as the tap's own
.1. What gets through is a forged source inside the tap's own /30 other
than the guest's .2.

**IPv6 is not a path today.** `net.ipv6.conf.all.forwarding = 0`, and the
taps carry only kernel `fe80::` link-local addresses, which are never
forwarded. The ip6tables FORWARD chain is empty, with policy ACCEPT. No
IPv6 rule is needed now. If guests ever get routable v6, the deny must be
mirrored in ip6tables.

**Teardown gaps found by reading the code** (`main` dd88b1767):
- A Firecracker exit that vmctl did not cause (a crash, or a guest
  poweroff) only marks the instance failed (`startFirecrackerProcess`'s
  wait goroutine). The tap and all its rules stay until a later stop,
  recover or boot of the same VM.
- If `launchFirecracker` fails after `prepareVMNetwork` (for example, the
  Firecracker spawn fails), `bootVM` returns with the tap and its rules
  still installed.
- `deleteIPTablesRulesForTap` matches rules by substring, and only in the
  six built-in chains.
- Rules for a tap that disappears while vmctl is not running are never
  removed. Nothing sweeps at start, which is how the 157 stale rules above
  built up.

**Fail-open posture.** The FORWARD policy is ACCEPT, so guest-originated
forwarding is allowed unless a per-tap DROP happens to match. A tap whose
rules were never installed, or were lost, can forward anywhere, including
into other guests.

## Fix (branch `fix/tap-deny-guest-range`, not deployed)

`internal/vmmanager/hostfirewall.go`:

- **Guest forwarding is default-deny.** FORWARD gets one jump, at position
  1: `-i vm-+ -j CHOIR-GUEST-FWD`. That chain:
  - drops every destination in the guest pool (`10.200.0.0/13` and
    `10.208.0.0/13`, computed from `vmSubnetSecondOctetBase` and
    `vmSubnetSecondOctetCount`);
  - jumps to `CHOIR-GUEST-TAPS`, which holds one ACCEPT per live tap,
    `-s <guest>/32 -i <tap>`;
  - ends in DROP.

  A guest interface without its per-tap rule therefore fails closed, and
  so does a packet with a forged source. The host's FORWARD policy is not
  changed.
- **INPUT service ACCEPTs now match `-s <guest>/32`.** A forged source can
  no longer match them, wherever they sit relative to the DROP.
- **Teardown.** Every teardown path removes all of a tap's rules, matched
  by `go-choir-vm-<tap>` comment or by exact owned interface name. Teardown
  now also runs on a Firecracker exit that vmctl did not cause, and on a
  spawn failure after the network was prepared.
- **Start-up.** vmctl's `Manager.Start` sweeps rules of vanished taps in
  both naming schemes. It then seeds live taps' egress ACCEPTs from their
  addresses, and only then installs the base chain. Reattach migrates a
  live tap's older rule shapes, adding the new rules before deleting the
  old ones.
- **Not done.** There is no ip6tables counterpart (guests have no routable
  v6). A crashed *reattached* guest (adopted by PID, so it has no wait
  goroutine) keeps its tap until its next lifecycle action, or until the
  next vmctl start if the tap is gone by then; under default-deny, that
  leftover state cannot forward.
