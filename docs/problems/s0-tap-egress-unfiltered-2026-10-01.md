# S0 finding: guest tap egress and tap-to-tap reachability are unfiltered

Date: 2026-10-01
Discovered by: S0a probe (s0a-tap-reachability-2026-10-01.json)
Station receiving it: S1-security-floor

## Evidence

From inside a guest VM (via `/internal/diag/tcp-dial`), a computer at
`10.200.139.2` reached all of:

- another guest's autoputer listener — `10.200.136.2:8085` → `ok` (tap→tap
  forwarding between computers is open);
- the host's vmctl control plane — `10.200.139.1:8083` → `ok`;
- an external address — `1.1.1.1:443` → `ok` (guest egress to the internet
  is unfiltered).

`iptables -L FORWARD` shows unconditional `ACCEPT` rules per VM tap in both
directions. There is no isolation between guest computers, between a guest
and host control-plane ports, or between a guest and the public internet.

## Why it matters

S1's security floor is specified against isolation boundaries. The current
runtime provides none: any compromised or prompt-injected guest can reach
other owners' computers, the vmctl internal API surface, and arbitrary
internet endpoints. The `spawn_capsule` "networkless" claim applies to
the in-guest capsule netns, not the guest itself — the guest's own egress
is wide open.

## Not a fix here

S0a is read-only observation. S1 owns the isolation decision
(pf/nftables rules, FORWARD policy, proxy-only egress via a recorded
capsule proxy). This document only records the observed posture.
