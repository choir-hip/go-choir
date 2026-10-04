# S0 finding: any guest inherits host-internal authority — cross-tenant control-plane reach and a frontend-injection chain

**Date:** 2026-10-04
**Status:** open. Source-traced. The reachability leg is confirmed on staging
(S0a). The authority legs are confirmed in source only: no request was replayed
against another tenant. Owner direction 2026-10-04: record in full (prerelease;
considerable hardening follows the metamission).
**Mutation class of this record:** green. The fix is red (VM networking,
vmctl, corpusd platform-control signing, auth identity propagation).
**Station receiving it:** S1-security-floor, as a new first slice **S1a
host-boundary hotfix**, ordered ahead of S0b. See the metamission `now` card.
**Supersedes the scope of:** `s0-tap-egress-unfiltered-2026-10-01.md` (reachability)
and `s0-internal-surface-forgeable-caller-2026-10-01.md` (boot-timeline endpoints
only). This record covers the general authority consequence.

## Evidence

### Leg 1: reachability (staging, S0a)

`docs/evidence/s0a-tap-reachability-2026-10-01.json`: from inside a guest,
TCP dials succeeded to another guest's `:8085`, the host's vmctl `:8083`, and
`1.1.1.1:443`.

### Leg 2: guest traffic to host services arrives with the REAL guest source IP (CORRECTED 2026-10-04, deployed-observed)

`internal/vmmanager/manager.go` `setupHostNetworking`:
- PREROUTING DNATs `hostIP:<port>` to `127.0.0.1:<port>` for every port in
  `tapReachableHostServicePorts()`: 8082 proxy, 8083 vmctl, 8084 gateway,
  8085 host autoputer, 8086 corpusd, 8087 maild, 8787 source service.
- `route_localnet=1` is set on the guest tap so the DNAT'd packet to a local
  destination is delivered via INPUT rather than dropped as martian.
- The packet therefore takes the **INPUT path, not POSTROUTING**: the
  per-tap `-o lo -j MASQUERADE` rules exist but have 0 packets/0 bytes and
  cannot fire; **no source NAT occurs**.
- Deployed proof (`docs/evidence/s1a-guest-to-host-addr-preservation-2026-10-04.json`):
  a guest-originated SYN to `hostIP:8086` was captured at host INPUT with
  `SRC=10.200.207.2 DST=127.0.0.1` — the real guest tap IP, not loopback.
- `FORWARD -i tap -j ACCEPT` / `-o tap -j ACCEPT` plus a general outbound
  MASQUERADE leave tap->tap and tap->internet open; FORWARD has no drop
  chain (policy ACCEPT). The INPUT tail does jump to `nixos-fw`
  (log-refuse), so guest->host port reach is already limited to the 7
  DNAT'd service ports; the exposure is **authority inside those
  services**, not port reach.

Consequence for the fix: `RemoteAddr` at host services is the guest's real
tap IP, so tap-sourced requests are distinguishable at the application
layer (`isLocalhost`-style checks already exclude guests; only
header-only checks pass). The earlier inference that traffic "arrives as
loopback" is withdrawn.

### Leg 3: host-internal authority checks accept caller-controlled inputs (source)

| Service | Check | Location |
|---|---|---|
| vmctl `:8083` | `isInternalCaller`: `X-Internal-Caller: true`, OR `Host` header is `localhost`, OR loopback `RemoteAddr` | `internal/vmctl/handlers.go:1442` |
| corpusd `:8086` platform-update mint | `X-Internal-Caller == "true"` only; signs under platform-control | `internal/platform/platform_update.go` `HandlePlatformUpdateOfferMint` |
| guest autoputer `:8085` `/internal/runtime/*` | `X-Internal-Caller == "true"` only | `internal/agentcore/api.go:295` `requireInternalRuntimeCaller`; `/internal/runtime/platform-update` at `internal/apihandler/routes.go:50` |
| guest autoputer (user routes) | trusts proxy-set `X-Authenticated-User` | `internal/proxy/computer_lifecycle.go:203` sets it; guests reachable directly |
| maild `:8087` | owner from `X-Authenticated-User` + `X-Internal-Caller` | `internal/maild/api.go:165-176` |
| gateway `:8084` internal | `isLocalhost(r) && X-Internal-Caller` — with leg 2 corrected, guest RemoteAddr is a tap IP, so guests already fail `isLocalhost`; guest->gateway internal is closed by network position | `internal/gateway/handlers.go:787` |

vmctl internal routes behind that check include resolve, list (every
ownership: computer IDs, VM IDs, guest URLs), refresh, stop/start/recover,
maintenance hold, divergence status, cold-recover, boot-timeline and the
autoputer proxy.

## Consequence (source-traced chain, not exercised)

An account on choir.news gets a computer. The owner has a root shell in their
own guest via the terminal app (`internal/autoputer/terminal.go`). From there:

1. **Lifecycle control over every computer.** vmctl list enumerates all
   computers. Refresh, stop, hold or cold-recover can be applied to any of
   them.
2. **Cross-tenant data access.** maild draft and mail endpoints, and every
   guest user route, accept a forged `X-Authenticated-User` from a guest.
3. **Cross-tenant frontend injection.**
   (a) POST corpusd `/internal/computers/platform-updates/offer` with
   `X-Internal-Caller: true`, the victim's computer and realization IDs, and
   a base head. corpusd builds and signs a genuine platform-control offer,
   staging the attacker's files.
   (b) POST it to the victim guest's `10.200.Y.2:8085/internal/runtime/platform-update`
   with `X-Internal-Caller: true`.
   (c) The victim verifies the real signature. A `tracking` / `auto` lineage
   (the default) applies it and promotes the route. The victim's computer
   surface (`current/frontend`) is now attacker-authored and served at
   choir.news inside the victim's authenticated session.
   The owner computer is `divergence_status: canary` and refuses offers by
   policy. Ordinary tracking computers do not.
   Unverified link: how the attacker obtains the victim's current base event
   head. The surrounding corpusd and vmctl read surfaces are similarly
   header-gated, so this is presumed reachable.
4. **Unfiltered egress.** Any compromised or prompt-injected guest can reach
   the internet and other guests directly.

Related exposures already on record: the gateway token is on the guest kernel
cmdline and readable inside the guest
(`s0-gateway-token-on-kernel-cmdline-2026-10-01.md`). The guest runtime unit
runs as unconfined root, and non-capsule shells (zot, terminal) inherit the
runtime environment
(`docs/reports/nixos-agent-platform-redhat-deepseek-audit-2026-10-01.md` §1.3).

## Root cause (substrate)

Host-internal authority is inferred from **network position plus a
self-asserted header**. The VM network makes every guest able to
TCP-connect to the host's loopback-bound services through the DNAT
bridge, while carrying a forgeable internal-caller header — and guest
traffic to other guests and the internet is unfiltered. Identity for
user routes is a header assertion that the proxy adds, but the proxy is
not the only path to a guest. This is one substrate defect, not several
endpoint bugs. Patching individual handlers would leave the class open.

## Fix shape (S1a, red — design to be settled in the S1 station file)

1. **Network (smallest, first).**
   - Drop tap->tap forwarding.
   - Default-deny guest->internet, with research egress through the gateway
     or a recording proxy.
   - Keep guest->host restricted to the service ports each guest flow
     actually needs (the INPUT rules already scope this; drop the
     `lo` MASQUERADE rules which provably never fire).
   - Set `networking.firewall.filterForward = true` on Node B (or an
     equivalent explicit FORWARD policy in vmmanager).
2. **Authority.**
   - Host-internal endpoints refuse any tap-sourced request, or bind a
     per-realization credential (the existing credential disk / receipt
     signer is the candidate) instead of `X-Internal-Caller`.
   - corpusd's platform-control mint is callable only by a host-local
     authority, never from a guest path.
   - Guest `/internal/runtime/*` requires a vmctl-issued credential, not a
     header.
   - `X-Authenticated-User` is honored only on a proxy-authenticated
     channel: a signed assertion or a unix-socket transport.
3. **Verification (deployed, two disposable accounts).** Each leg above is
   refused from guest A against guest B and against the host services, while
   the legitimate guest→gateway/maild/corpusd flows still work. Record
   pre/post `iptables-save` and the refusal receipts.

## Rollback

Network rules are applied by vmmanager per tap. Reverting the commit and
refreshing restores the prior rules. Authority changes revert by git revert
+ redeploy. Before landing, the legitimate guest→host flows (gateway
inference, maild drafts, corpusd event CAS, wire publish, source service)
must each be named and tested, or the fix will break product paths.
