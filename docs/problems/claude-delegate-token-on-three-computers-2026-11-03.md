# The owner's Claude token sits on three computers, one of them an engineering arm (2026-11-03)

Date: 2026-11-03. Mutation class of any fix: red (credential custody, the
guest credential disk, gateway and provider calls, capsule egress). Problem
first; no fix in this commit. It extends SEC-08,
[claude-delegate-key-sits-on-the-guest-disk-2026-10-21.md](claude-delegate-key-sits-on-the-guest-disk-2026-10-21.md),
with today's deployment and the capsule overlay.

Owner, 2026-11-03: "there shouldn't be credentials on computers anyway."

## The problem in one sentence

The only credential found on any computer that unlocks something outside
it is the owner's Claude OAuth token, which vmctl writes onto the
credential disk of every computer in `CHOIR_CLAUDE_DELEGATE_COMPUTERS`
(three today, one of them the Claude arm of the self-development apps
goal), and which a `claude`-mode capsule receives with egress to
`api.anthropic.com`, so the gateway neither holds the token nor meters the
calls.

## Evidence

- The inventory of d@e for the capsule overlay
  ([capsule-overlay-s0-measurement-2026-11-03.md](../evidence/capsule-overlay-s0-measurement-2026-11-03.md),
  "Secret-shaped material"): names, owners, modes and sizes only. d@e is
  not on the delegate list and has no such token; its credential disk holds
  `gateway-token`, `host-fetch.json` and `capsule-mirrors.json`.
- `nix/node-b.nix` (go-choir-vmctl Environment):
  `CHOIR_CLAUDE_DELEGATE_COMPUTERS` names three computers, the third added
  2026-11-03 for `selfdev-arm-claude`; `CHOIR_CLAUDE_DELEGATE_EXPIRES` is
  2026-11-17.
- `internal/vmmanager/claude_delegate.go` (`claudeDelegateMaterial`)
  writes `claude-delegate.json` and `claude-oauth-token` (0400, root) onto
  the credential disk at each cold boot of a listed computer;
  `internal/capsule/claude_delegate.go` hands the token to a `claude`-mode
  capsule's broker and allows CONNECT to `api.anthropic.com` with a
  512 MiB upload budget.
- The capsule overlay design
  (`docs/design/capsule-overlay-computer-state-2026-11-03.md`) gives
  engineering capsules a copy of the computer's files and store. On a
  listed computer, a capsule that holds both that copy and the token would
  have an upload path, through the token, to a host outside Choir's meter.

## Why it matters

- It is the one place the platform rule "provider keys live in the
  gateway" does not hold; every other provider call goes through the
  gateway, which holds the keys and meters the calls.
- A guest-root process on a listed computer can read the token (SEC-08).
- The overlay makes the capsule a holder of owner data; the token's egress
  is the widest upload path from such a capsule.

## Proposed removal (not built here)

Move the delegate call behind the gateway, as every other provider call
is: the gateway holds the token, the capsule's `claude` CLI talks to a
gateway route with the computer's gateway identity, and the credential
disk carries only the grant record. SEC-08's station (FP7) has the gateway
half built and dark; the guest half is the remaining work.

Until it lands, the overlay and the delegate meet on the listed computers:
`engineeringCastNetwork` (`internal/agentcore/engineering_assignment_runtime.go:497`)
makes an engineering Cast `claude`-mode whenever the delegate is available
and the Cast does not ask for the ecosystem proxy, so an engineering
capsule on the Claude arm would hold the computer-state copy, the token
and `api.anthropic.com` egress together. The overlay reaches those
computers only by the ring walk; this record is the reason to settle the
delegate before that ring.
