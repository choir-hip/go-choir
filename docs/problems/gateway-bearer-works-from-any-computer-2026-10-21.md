# A Gateway Bearer Works From Any Computer (SEC-05)

Date: 2026-10-21. Source: the computer isolation security review
(finding F9, section 4.6), the learning document
[03-gateway-and-providers.md](../learn/03-gateway-and-providers.md) §2.3,
and a re-read of the code at origin/main 26c4e1ed0. Tracked as SEC-05 in the
[checklist](../security-density-architecture-checklist-2026-10-21.md). Station:
[FP7](../definitions/choir-launch-fp7-gateway-identity-2026-10-21.md).
Mutation class of any fix: **red** (gateway credentials, auth). Problem
first; no fix in the commit that wrote this record. Status: fix built in
station FP7 (modes off, observe, enforce, strict; nix stages observe); deploy
and enforcement status are in the station file.

## The problem in one sentence

A computer's gateway bearer is accepted from any Choir guest address (and
from loopback), so a bearer taken from one computer, for example from a
forked or snapshot-cloned credential disk, or by a guest-root process that
reads its own disk and hands it to a sibling, spends model, search and
source-API budget under the first computer's name from another computer,
and nothing in the gateway can tell.

## Evidence (code, read 2026-10-21 at 26c4e1ed0)

- `authenticateAutoputer` (`internal/gateway/handlers.go:677-700`) checks the
  bearer, then only `isAuthorizedRuntimePeer`: loopback, or any address
  that `isChoirGuestPeer` accepts (`172.x.0.2`, `10.200-215.x.2`,
  `handlers.go:753-794`). It never compares the peer with the computer the
  bearer belongs to. Every gateway route that serves a computer goes through
  it: inference, the OpenAI-compatible adapter (chat completions and
  models), search, source-api and Jev judgments (`handlers.go:350, 816`,
  `openai_compat.go:223, 235`, `source_api.go:678`, `jev.go:268`).
- Only the Jev route pins a bearer to an address (`BindJevPeer`,
  `config.go:437-456`, called from `jev.go:274`). The pin is the first
  address that used it; only the host's credential-ensure route, with the
  address vmctl attests, moves it (`EnsureCredentialForPeer`,
  `config.go:336-398`; `HandleEnsureCredential`, `handlers.go:624-672`).
- The hash comparison is `hex.EncodeToString(hash[:]) != identity.TokenHash`
  (`config.go:424-426`) and `existing.TokenHash != hashText`
  (`config.go:357`): not constant time. Both sides are SHA-256 of a 32-byte
  random token, so a timing leak yields bits of a hash, not of the token;
  the fix costs nothing and is the habit to keep.
- Lifetime: the default is one hour (`DefaultAutoputerTokenTTL`,
  `config.go:64`); staging sets `GATEWAY_AUTOPUTER_TOKEN_TTL=720h`
  (`nix/node-b.nix:1929`) because nothing rotates tokens in place.
  `RotateCredential` is `IssueCredential` (`config.go:474-476`): it revokes
  and replaces, but nothing delivers the new bearer to a running guest.
  The bearer is delivered at a cold boot (`issueGatewayTokenAt`,
  `internal/vmctl/ownership.go:1675`, called from `startAdmittedVM` and
  `platform_computer.go:365`) on the root-only credential disk
  (`RUNTIME_GATEWAY_TOKEN_FILE=/run/choir-bootstrap/gateway-token`,
  `nix/autoputer-vm.nix:890`). A recovery or snapshot resume passes an empty
  token and keeps the old one (`startAdmittedVM`, `ownership.go:1431-1458`).
- The source address is trustworthy: the host drops any packet entering a
  guest tap whose source is not that guest's address (`tapInputDropSpec`,
  `internal/vmmanager/hostfirewall.go:114-116`, installed by `ensureTap`;
  S1a anti-spoofing), and the gateway is reached through the tap's
  host-side address, so `RemoteAddr` is the tap's guest address.
- The stale doc claim: `docs/learn/03-gateway-and-providers.md` §2.3
  says the documentation test ranges are admitted in production. They are
  admitted only inside a test binary (`runtimePeerAllowed(ip, testing.Testing())`,
  `handlers.go:765-770`; `runtime_peer_test.go`).

## Why the repair is not a one-liner (the Jev incident)

The Jev pin is the pattern to generalize, and it already failed once in
production. A computer keeps its gateway token across boots but takes a new
guest address from each boot's host port. The pin was the first address
that used the bearer, for the credential's whole life, so after a cold boot
onto a new port every Jev call from the paper was refused ("Jev bearer is
not authorized for this VM") and the paper labelled nothing for hours
([l2a-merge-recall-on-the-paper-2026-10-14.md](../archive/problems/l2a-merge-recall-on-the-paper-2026-10-14.md),
finding 2). The repair moved the pin from the host: vmctl names the current
guest address on every credential ensure
(`ensureExistingGatewayCredential`, `ownership.go:1746-1822`).

Pinning every route turns a stale pin from "Jev labels stop" into "the
computer's model calls stop". The paper is always on; a pin that goes stale
at a boot, or an ensure that fails after one, would take it down. So the
repair needs a rollout that can be watched before it enforces, and a way
for the host to move a stale pin.

## Ways the repair can fail (written before the code)

1. A bearer pinned to computer A is accepted from computer B's address
   (the defect), on any route.
2. The computer's own calls are refused: after a boot onto a new address
   the pin is stale until vmctl re-ensures; after a gateway restart the pin
   is lost; the pin is checked against the wrong form of the address
   (IPv4-mapped IPv6).
3. The guest, not the host, moves the pin (a pin any caller can move
   protects nothing). Only the host-internal ensure route may move it.
4. Unpinned identities (every identity that predates the change except those
   Jev pinned) are refused at the deploy, so every running computer loses
   the gateway until vmctl's reconcile reaches it.
5. An unpinned bearer's first use pins an attacker's address (trust on first
   use) and the legitimate computer is then refused. The host's ensure must
   be able to move it back, and a strict mode must be able to refuse
   unpinned bearers once the host has pinned every running computer.
6. A pin write that fails (disk) leaves the in-memory pin and the file
   disagreeing, or accepts the call without a pin in enforce mode.
7. Two first uses from different addresses at once both succeed.
8. A loopback process presents a guest's bearer and passes because loopback
   is always a runtime peer.
9. The mismatch log floods the journal under a retry storm, or names the
   token.
10. The constant-time comparison is applied to one path and not the other
    (`ValidateCredential` and the ensure conflict check).
11. A rollback of the change strands the pin file in a shape the old code
    cannot read.
12. The change widens a route: nothing here may make a call that was refused
    before succeed.

## Fix direction (this station)

- Bind the bearer to its computer's address on every route: an identity
  carries one pin; a call from any other address is refused; the host moves
  the pin on each boot (already true); an unpinned bearer binds on first use
  in `enforce` mode, is refused in `strict` mode, and stays unpinned in
  `observe` mode.
- Roll out in modes, the way host-internal auth did
  ([hostauth-enforce-flip-2026-10-20.md](hostauth-enforce-flip-2026-10-20.md)):
  `observe` first (log and count "would refuse", refuse nothing), read the
  paper's and the test computers' counts, then `enforce`, later `strict`.
- Constant-time hash comparison on both paths.
- Rotation: see the station's design. A live rotation needs a way into the
  running guest that does not exist; the repair designs it and does not
  build it.
- Later the binding moves from the source address to the vsock context ID
  (NET-03), which the guest cannot spoof at all.

## Review finding, batch 1 (2026-10-21, candidate b80a9bcac)

Found by the batch 1 panel (Gemini 4 Argon, blocking), confirmed by reading
the code; no deployed evidence, the candidate never left the lane.

13. **The Jev route writes the shared pin in every mode.** The candidate
    unified the Jev-only pin into the identity's one pin
    (`AutoputerIdentity.PeerIP`, JSON key `JevPeerIP`), which every route now
    reads. `HandleJudgment` still calls `BindPeer`, which is
    `CheckPeer(..., bindIfUnpinned=true)`, before the body is read. So:
    - in `strict`, an unpinned bearer refused on inference can send one `{}`
      to `/provider/v1/judgments`, pin itself to its caller's address, and
      then pass every other route (strict's whole point, refusing a pin the
      host did not attest, is bypassed);
    - in `observe`, a Jev call writes a pin although observe promises to
      write none, so the observe counts no longer measure what enforce
      would do.
    The must-preserve line "the Jev route's strict pin is unchanged in every
    mode" was written when the Jev pin was Jev's alone; once the pin is
    shared, an unchanged first-use write on one route is a write for all of
    them. Repair: the Jev route keeps its own refusal (403) for a mismatch
    in every mode, and writes a first-use pin only where the mode allows
    one (`enforce`, and `off`, which is the pre-SEC-05 behavior); in
    `observe` an unpinned bearer passes and nothing is written; in `strict`
    it is refused. Residual this accepts: in `observe`, between a cold boot
    and the host's ensure, an unpinned Jev bearer is accepted from any
    guest address, where before the first caller's address won the pin.
    The host's ensure pins it right after the boot, and observe is a
    measured, temporary mode.

## Review finding, bunch 4b (2026-10-21, candidate 6dafee596)

14. **The finding 13 repair opened Jev in observe** (GPT-6.1-sol, blocking;
    Gemini 4 Argon accepted). Accepted. Evidence: `jevPeerAllowed` at
    6dafee596 passes `bind = mode == enforce || mode == off`, and returns
    true for `PeerUnpinned` in observe; the candidate's own test
    (`TestJevRouteWritesAPinOnlyWhereTheModeAllowsOne/observe`) asserts an
    unpinned call passes with no pin written. So in observe (the staged
    mode) guest A's first Jev call no longer pins, and a copied bearer from
    guest B is served until the host's ensure. Before the batch, A's call
    pinned and B was refused. That is new provider access, not observation,
    and breaks "nothing here makes a call succeed that was refused before".
    Finding 13's "observe writes no pin" was the wrong reading: the Jev
    route's first-use pin predates SEC-05, so keeping it is not a write
    observe adds. Repair: the Jev route pins on first use in off, observe
    and enforce, as it always did; strict alone refuses an unpinned bearer
    (finding 13's strict bypass stays closed). A Jev-written pin in observe
    is read by the other routes only to count `would_refuse`, which is the
    signal observe exists for. The finding 13 residual (an unpinned Jev
    bearer accepted from any address in observe) is withdrawn.

## Heresy delta

- Discovered: the Jev pin's trust-on-first-use is an unplanned authority
  (whoever calls first owns the pin until the host's next ensure).
- Introduced: none yet.
- Repaired: none yet.
