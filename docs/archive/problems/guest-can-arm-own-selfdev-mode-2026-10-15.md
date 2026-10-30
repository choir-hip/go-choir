# The Guest Can Arm Its Own Self-Development Mode

Date: 2026-10-15 (local probe run 2026-10-14 22:34 UTC). Source:
prerequisite P3 of
[docs/archive/reports/selfdev-update-architecture-2026-10-15.md](../reports/selfdev-update-architecture-2026-10-15.md)
(§1.3 "Close the guest-arming hole first", §"Unverified claims"). Code read
at origin/main 99f98e211. Status: **confirmed** by an in-process probe;
repair follows in a separate commit on branch `fix/selfdev-mode-owner-only`
(not pushed, not deployed). Mutation class of the fix: **red** (approval
and authority surface: who may authorize a self-development acceptance).

## The problem in one sentence

corpusd's self-development mode write (`POST
/internal/computers/self-development/mode`) accepts any bearer that holds
the `event:append` capability for the computer, and every guest holds that
capability, so the computer can arm `accept_once` or `qualified_consensus`
for itself and corpusd signs the result with the same platform-control key
it uses for an owner's grant.

## The trace

1. **The guest's credential.** corpusd mints the guest's computer
   capability with the scopes `event:read`, `event:pin`, `event:append` on
   every path: envelope exchange
   (`internal/platform/credential_envelope.go:211`), the post-revocation
   capability (`:227`), and renewal (`:312`). There is no narrower guest
   scope; `validCapabilityScopes` admits only those three
   (`internal/platform/event_capability.go:149-163`).
2. **The guest reaches corpusd directly.** A guest calls corpusd at its
   platform URL, not through the proxy
   (`internal/platform/authority.go:70-75`), and every
   `/internal/computers/*` route, including the mode route, is registered on
   that listener (`internal/platform/handlers.go:468`).
3. **The mode write gate.** `HandleSelfDevelopmentMode`
   (`internal/platform/self_development_modes.go:340-388`) admits a POST
   when the caller is host-internal **or**
   `eventAuth.Authorize(r, computerID, "event:append")` succeeds (`:364`).
   The capability verifier checks signature, computer, expiry, revocation
   epoch and scope (`event_capability.go:56-140`); nothing in it
   distinguishes the guest from the owner.
4. **No owner check below the gate.** `SelfDevelopmentModeCAS.Set`
   (`self_development_modes.go:123-218`) checks the generation CAS and the
   transition's shape (`validateSelfDevelopmentModeTransition`, `:220-282`),
   then signs a `ModeReceipt` with corpusd's platform-control key (`:189`).
   The receipt records no writer: an owner-armed and a guest-armed
   `accept_once` are byte-for-byte the same kind of record.
5. **The guest has a ready client.** `GuestCredentials.SetSelfDevelopmentMode`
   (`internal/selfdev/credentials.go:386-416`) posts exactly this request
   with the guest capability. The runtime wires it as `rt.selfdevControl`
   (`internal/agentcore/runtime.go:745`). Its only legitimate callers are
   the two **consumption** writes in the decision handler: `accept_once ->
   propose_only` with idempotency key `accept-once-consumed:<op>:<digest>`
   (`internal/agentcore/api_self_development.go:379`) and `qualified_consensus
   -> propose_only` with `qualified-consensus-consumed:<op>:<digest>`
   (`:437`).
6. **The owner path.** The owner's arm goes through the proxy
   (`internal/proxy/self_development.go:125-213`): a session must own the
   computer (vmctl `LookupComputerContext`, `:157`), or an API key must carry
   `computer:self_development:mode` (or `admin`) and target that computer
   (`:140-153`). The proxy then calls corpusd as a host-internal caller
   (`hostauth.MarkInternal`, `:189`). This path is correct; the hole is that
   the guest does not need it.

**There is no mode event on the tape and no mode reducer.** The mode is a
mutable corpusd row (`computer_self_development_modes`,
`internal/platform/computer_events.go:82-99`) with a signed `ModeReceipt`.
The tape binds the receipt only by digest: an `effect_accepted` decision
event carries the ModeReceipt as its first input artifact
(`internal/agentcore/self_development_decision_binding.go:71-81`). So "a
guest-appended mode event" is, concretely, a guest POST to the mode route,
and the fix belongs at that write, not in replay.

## The probe

An in-process corpusd handler test
(`TestProbeGuestCapabilityArmsSelfDevelopmentMode`, run locally and not
committed; its assertions became the regression tests in the fix commit):
a real `SignedCapabilityVerifier`, a real `SelfDevelopmentModeCAS` on the
embedded Dolt test store, a capability minted with the guest's exact scope
set, and a request from a guest tap address (`172.16.0.2`, so not a
host-internal transport) with no internal marker.

```text
guest POST accept_once: status=200 body={"computer_id":"computer-probe","mode":"accept_once","generation":1,"operation_id":"op-guest",...
  "receipt":{..."issuer":"corpusd","new_mode":"accept_once","old_mode":"off",...,"receipt_kind":"ModeReceipt",
  "signature_set":[{"key_id":"probe","signature":"PfkA...","signer_domain":"platform-control"}]}}
mode after guest accept_once: mode=accept_once generation=1 operation=op-guest
guest POST qualified_consensus: status=200
mode after guest qualified_consensus: mode=qualified_consensus generation=2 policy=bbbbbbbb consensus=cccccccc
--- PASS: TestProbeGuestCapabilityArmsSelfDevelopmentMode (0.44s)
```

Verdict: **confirmed.** A guest-class bearer arms `accept_once` from `off`,
and then `qualified_consensus` with a policy digest and consensus receipt
digest of its own choosing, and corpusd signs both.

Staging was not touched. The probe is local proof of the corpusd handler
and CAS, which is the whole of the authority decision; it does not prove
that a live guest can reach corpusd's port, which item 2 above reads in
code and which the guest's own `PublishCheckpoint` and event-append calls
exercise daily.

## Severity

**High today, fatal once P4 lands.**

- **Today no acceptance happens on a guest-armed mode alone.** The decision
  route still requires a host-sourced authenticated user
  (`authenticateUser`, `internal/agentcore/api.go:164-173`, used for the
  decision at `api_self_development.go:179`), and the CLI's owner approval
  arms `accept_once` itself before deciding (`cmd/choir/commands.go:580`).
- **But the guest forges the host-side owner witness.** The ModeReceipt is
  the only record signed outside the guest that says "the owner authorized
  this exact operation, bundle, heads and commitments". A guest-armed one is
  indistinguishable from an owner-armed one, so the receipt bound into every
  `effect_accepted` event proves nothing about the owner.
- **The guest can switch self-development on.** `off` and `audit_only` do
  not open candidates; the guest can move itself to `propose_only` and open
  them.
- **The guest can disarm or churn the owner's grant.** Any guest write bumps
  the generation, so an owner arm racing it loses with a CAS conflict, and
  the guest can set `off` under the owner.
- **With the standing autonomous mode and an actuator outside the guest
  (P4), the guest would arm the mode the actuator trusts and supply the
  policy and consensus digests it trusts.** That is a self-approval loop. The
  report names this as the precondition of P4.

## Failure modes the fix must handle

Refused (guest bearer, no owner provenance):

1. guest arms `accept_once` (from any mode);
2. guest arms `qualified_consensus` (from any mode);
3. guest raises `off` or `audit_only` to `propose_only`;
4. guest lowers or churns the mode (`propose_only -> off`, `-> audit_only`,
   re-sets the same mode) — owner-only means every set, not only arming;
5. guest "consumes" a grant that is not current: `propose_only ->
   propose_only` under a consumption key, or a key naming an operation other
   than the armed one;
6. guest consumption under the wrong kind of key (`accept-once-consumed:`
   for an armed `qualified_consensus`, and the reverse).

Still allowed:

7. the guest's real consumption writes (`accept_once -> propose_only` and
   `qualified_consensus -> propose_only`, each under its consumption key
   naming the armed operation), because the decision handler depends on
   them and the consumed receipt is what the decision event binds;
8. an idempotent retry of a guest consumption that already committed (it
   must return the stored receipt, not a refusal, or a retried decision
   breaks);
9. every owner set through the proxy (session that owns the computer, or an
   API key with `computer:self_development:mode` or `admin`), which arrives
   at corpusd as a host-internal caller;
10. corpusd's own expiry write inside `Get`;
11. the guest's GET of the mode (`event:read`).

Compatibility:

12. existing `ModeReceipt`s on staging lack any writer field and must keep
    verifying and keep satisfying `verifyDecisionModeReceipt` /
    `verifyConsumedModeReceipt`;
13. replay of existing tapes must not change: decision events bind a
    ModeReceipt by digest, and replay never re-reads the mode row, so the fix
    must not add a replay-time rule.

## Mutation class, protected surfaces, rollback

- Class: **red**. Protected surface: approval authority for self-development
  (who may produce the host-signed witness that an owner authorized an
  acceptance). Not touched: Texture canonical writes, the event append and
  replay path, checkpoint/route projection, vmctl, gateway, proxy.
- Admissible evidence: in-process corpusd handler and CAS tests on the
  embedded Dolt store (this is where the decision is made), plus the
  existing agentcore decision tests that drive the consumption writes. A
  staging proof is the station-A negative leg in the report ("a mode write
  with the guest's own credential is refused (P3)").
- Rollback: `git revert` the fix commit. The fix changes only the mode POST
  gate and the CAS write; it adds a `mode_writer` field to new ModeReceipts
  but no schema column, so a revert leaves stored rows readable.

## Heresy delta

- discovered: the guest's `event:append` capability is also the authority to
  write the self-development mode, so the host-signed owner witness can be
  minted by the subject it is meant to restrain.
- discovered (residual, not this fix): the guest appends canonical events,
  including `effect_accepted`, with its own capability; the owner gate on
  the decision event is enforced by the guest's own code (the decision
  route's host-sourced check). Corpusd does not re-verify the decision's
  authority at append. Once mode writes are owner-only, the ModeReceipt
  bound in the decision is a host-verifiable owner witness, which is the
  hook a later corpusd-side check (or P4's actuator) needs.
- introduced: none intended.
- repaired: none in this commit.

## Residuals

- The guest can still consume (and so burn) an armed grant without an owner
  decision, by naming the armed operation. That is fail-closed: the owner
  re-arms. It cannot raise authority.
- The proxy accepts an API key with the `admin` scope in place of
  `computer:self_development:mode`. That is the owner's own key, so it is in
  bounds, but it is broader than the scope the report names.

## Repair (branch `fix/selfdev-mode-owner-only`, not pushed)

- `HandleSelfDevelopmentMode` POST: a host-internal caller (the proxy after
  it authenticated the owner) writes through `SelfDevelopmentModeCAS.Set`;
  a guest capability writes only through `ConsumeAsGuest`.
- `ConsumeAsGuest` admits exactly failure-mode list items 7 and 8: current
  mode `accept_once` or `qualified_consensus`, next mode `propose_only`,
  idempotency key `<that grant's consumption prefix><armed operation>:<sha256>`.
  The idempotent-retry short-circuit runs before the check, so a retried
  consumption returns its stored receipt. Anything else is
  `ErrSelfDevelopmentModeOwnerRequired`, served as 403.
- Every new ModeReceipt carries `mode_writer` (`host-internal`,
  `guest-consumption`, `corpusd-expiry`). Old receipts lack it and still
  verify; nothing requires the field.
- The consumption key prefixes are exported from `internal/platform` and
  used by agentcore's decision handler and consumed-receipt verifier, so the
  two spellings cannot drift apart.
- Replay is untouched: no mode event exists on the tape, so there is no
  replay rule to tighten, and existing tapes fold as before.

Tests (`internal/platform/self_development_modes_owner_test.go`): a guest
bearer from a tap address is refused for every arm, raise, lower and stray
consumption from `off`, `propose_only` and an armed grant, and the mode and
generation stay unchanged; the guest's real consumption of `accept_once` and
`qualified_consensus` succeeds and retries idempotently; the owner (host
internal) sets every mode; expiry is recorded as a corpusd write. With the
handler change reverted, the guest test fails (a guest arm returns 200).

Staging proof still owed: the station-A negative leg (a guest-credential
mode write is refused on a live computer) after this lands through the
Landing Loop.
