# The Claude Delegate's Key Sits on the Guest Disk and Bypasses the Gateway (SEC-08)

Date: 2026-10-21. Source: the delegate's own design
([claude-delegate-slice1-2026-10-14.md](../design/claude-delegate-slice1-2026-10-14.md),
section 7, "Introduced"), the research report
([claude-agent-sdk-in-choir-2026-10-14.md](../reports/claude-agent-sdk-in-choir-2026-10-14.md),
sections 2.2, 2.3 and 3.3, Options R, T and K), owner notes section 36
(2026-10-21), and a re-read of the code at origin/main 26c4e1ed0. Tracked
as SEC-08 in the [checklist](../security-density-architecture-checklist-2026-10-21.md).
Station: [FP7](../definitions/choir-launch-fp7-gateway-identity-2026-10-21.md).
Mutation class of any fix: **red** (credential custody, gateway and provider
calls, the guest credential disk, capsule egress). Problem first; no fix in
the commit that wrote this record. Status: the gateway half is built and dark
(station FP7); the guest half and the owner's Console API key are open, so the
key still sits on the disk (Option T) until both land.

## The problem in one sentence

The owner's Claude token lives on the credential disk of the one test
computer, readable by any guest-root process on it, and the delegate calls
`api.anthropic.com` straight from the capsule, so the gateway (which holds
every other provider key and meters every other call) neither holds this
key nor sees these calls.

## Evidence (code and config, read 2026-10-21)

- `nix/node-b.nix:1371-1389`: `CHOIR_CLAUDE_DELEGATE_COMPUTERS` names one
  computer (`computer-ad1b46d4...`), `CHOIR_CLAUDE_DELEGATE_EXPIRES=2026-11-03`,
  `CHOIR_CLAUDE_DELEGATE_TOKEN_FILE` is vmctl's private copy.
- `internal/vmmanager/claude_delegate.go` (`claudeDelegateMaterial`): at a
  cold boot of an allowlisted computer vmctl writes `claude-delegate.json`
  and, when the token is usable, `claude-oauth-token` onto the VM's
  credential disk (`credential.img`, root-only inside the guest).
- `internal/capsule/claude_delegate.go` (`ReadClaudeDelegate`,
  `ClaudeEgressPolicy`): the guest core reads that file, hands the token to a
  `claude`-mode capsule's broker, and the capsule's egress allows
  `api.anthropic.com` only (CONNECT, so the proxy sees ciphertext), with a
  512 MiB upload budget.
- The delegate design records the residual: "a guest-root process (the
  owner's terminal, Zot) can read every file on the credential disk, the
  gateway token included", and "code that runs inside `claude`'s own process
  tree can still read the parent's environment through `/proc`". It was
  ratified for one computer and one window as an `I19` exception (owner,
  2026-10-14), then extended to 2026-11-03 (owner notes section 28).
- Owner ruling, 2026-10-21 (owner notes section 36, checklist DEC-03): the
  delegate stays on the one test computer until desk-cell confinement
  (SEC-01) lands, and before any wider use the key is held by the gateway
  (Option K). SEC-01 alone does not unlock wider use.
- Owner request, 2026-10-25 (in session: "We should set it up on b@c.com
  as well"): a second test computer, b@c's
  (computer-6c50d6e566143ae32cdb660faa60e24d), joins the allowlist with the
  same token and expiry (2026-11-03). This widens the exposure this record
  names to two guest disks; the 10-21 ruling's gateway condition (Option K)
  is not met, and the owner was told so in session. Review 252 (Devin)
  approved the one-line change with this note. Rollback: drop the id,
  deploy, cold-boot b@c.

## What Option K can and cannot be

Anthropic's terms (report section 2.2, verbatim there) forbid a service
that "collects, stores, or intermediates" a Claude.ai subscription token,
and point developers to API keys. The delegate's token is the owner's
`claude setup-token` subscription token. So:

- A gateway relay cannot hold that token (Option R is the shape the terms
  forbid for a subscription).
- Option K is a relay with a **Claude Console API key**, which the terms
  explicitly allow a platform to hold. It bills at API rates, and the key is
  the owner's to create and place. Until they do, there is nothing for the
  gateway to hold, and the delegate continues under Option T.

So the repair has a code half (the gateway can hold a key and relay the
delegate's calls, authenticated, bounded and metered) and an owner half (a
Console key, in `gateway-provider.env` on Node B, never in the repository or
a chat). This record does not decide the second half.

## Why it matters now

More agents can run on Node B only inside computers, and the delegate may
leave its one test computer only after SEC-01 and after the key is
gateway-held. Both gates exist because the token on the disk is reachable by
any process that gets guest root, and one such process today is an
unconfined desk cell.

## Ways the repair can fail (written before the code)

1. The relay forwards a caller-supplied credential (`x-api-key`,
   `Authorization`) upstream, so the caller's own key is billed or the
   gateway's key leaks in a header echo.
2. The relay is open: any computer with a bearer, not just the delegate's
   computer, can spend the key.
3. The relay is a general Anthropic proxy: it forwards paths, models or
   methods the delegate does not need (batches, files, admin, other
   models).
4. The key reaches a log, an error body, a response header, or the guest.
5. The delegate window ends and the relay keeps serving.
6. A streamed (server-sent events) response is buffered whole, cut, or
   re-framed, so the delegate stalls or mis-parses.
7. An unbounded request body or a hung upstream pins gateway memory or
   goroutines, and one computer's long streams starve everyone's inference
   (the relay shares the gateway with every model call).
8. A request is relayed before the caller is authenticated and bound to its
   computer's address (SEC-05).
9. The relay is configured but the key is missing, and the call falls back
   to another credential instead of refusing.
10. Moving the key changes who may run the delegate or where. It must not.

## Review findings, batch 1 (2026-10-21, candidate b80a9bcac)

Found by the batch 1 panel (GPT-6.1-sol reproduced both through the
candidate's own handler code with an overlay probe; Gemini 4 Argon found the
first independently). No deployed evidence: the relay is dark on staging
(no key, no identities, no window), which bounds today's exposure and does
not satisfy failure mode 8.

11. **The relay trusts the general binding's rollout mode.**
    `HandleAnthropicRelay` authenticates with `authenticateAutoputer`, whose
    peer check follows `GATEWAY_BEARER_BINDING`. In `observe` (the mode nix
    stages) or `off`, a bearer copied from an allowlisted computer is served
    from any guest address: the probe pinned the bearer to `10.200.0.2`,
    called from `10.200.1.2` in observe, and got 200 with one upstream
    request while the log said "would refuse". Failure mode 8 is therefore
    open whenever the relay is enabled before enforce, or binding is rolled
    back after. Repair: the relay checks the pin itself, in every mode, and
    serves only a bearer whose identity is pinned (host-attested or already
    bound) to the caller's address; an unpinned or mismatched identity is
    refused and nothing is relayed.
12. **A disabled or unlisted relay call writes the shared pin.**
    `authenticateAutoputer` runs before the relay checks whether it is
    configured, whether the identity is listed and whether the window is
    open; in `enforce` it pins an unpinned bearer on first use. The probe:
    relay off, an unlisted identity, answer 403, and its pin was now the
    caller's address, which every other route then honors. Repair: the relay
    never writes identity state; bearer validation is non-mutating, and the
    configuration, allowlist and window are checked before the pin is read.

## Heresy delta

- Discovered: the subscription token cannot be intermediated, so "the
  gateway holds the key" is two different projects depending on the credential
  type.
- Introduced: none yet.
- Repaired: none yet.
