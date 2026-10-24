# A reject is refused once an accept_once approval has been consumed (2026-10-24)

Status: open when written. Recorded 2026-10-24 by FP0's loop-gaps lane. Mutation
class of this record: green. Gap F of
`docs/design/app-packages-replan-2026-10-24.md` section 5. Update design
`docs/design/computer-update-architecture-rnd-2026-10-24.md` section 4.6 asks
for this record; none existed.

## What happened

On the b@c app-rewrite run (`/Users/wiz/Dev/lanes/_runs/apprewrite/log.md`, the
05:25:47 line), the owner approved the Slides operation. The guest consumed the
`accept_once` arm. The next candidate (Files, `selfdev-18e1dc56`) arrived
awaiting approval, and its reject answered

    409 "current signed mode does not authorize this decision"

The workaround was `self-dev mode set --mode propose_only --expected-generation 3`
(generation 4, a fresh receipt), after which the reject at 05:26:15 landed. Every
reject after an approval needs that extra owner act first.

## Why, from the code

- Approving under `accept_once` consumes the arm: `SetSelfDevelopmentMode` to
  `propose_only` (`internal/agentcore/api_self_development.go:439-443`), which
  mints a `ModeReceipt` with `old_mode=accept_once` and
  `consumed_operation_id=<the approved operation>`. That receipt is now the
  computer's current mode receipt.
- A later decision on any operation is authorized by `verifyDecisionModeReceipt`
  (`:680-716`). Its reject branch (`:712`) refuses when the current receipt has
  `old_mode == accept_once`, `old_mode == qualified_consensus`, or any
  `consumed_operation_id`. It never looks at which operation the receipt
  consumed.
- The rule came in with `fb0e56e33` ("close fifth G1 blockers"). The intent
  that survives in the code is that a consumption receipt is the single-use
  proof of an approval for one operation: it must not be read as authority for
  a second, opposite decision on that same operation. The check is far wider
  than that intent: it also refuses every other operation.
- The same receipt field is set on every mode transition away from a mode that
  names an operation, so an owner disarm of an `accept_once` arm for operation X
  (`propose_only` set by hand) also carries `old_mode=accept_once` and
  `consumed_operation_id=X`. Today that refuses a reject of X too, though X was
  never approved.
- Corpusd does not gate rejects. Its S1 gate runs for `EventEffectAccepted` only
  (`internal/platform/computer_events.go:248`, `owner_acts.go:328`). A reject
  event is admitted on the ordinary head and authority checks. So the guest
  rule is the only thing that blocks it, and it blocks more than corpusd would.

## Why a reject must stay available

A reject is the fail-safe decision: it ends a candidate and applies nothing. An
approval needs a current owner grant. A rejection needs none. A candidate that
cannot be rejected until the owner re-sets the mode stays `awaiting_approval`
and holds the computer's single capsule slot, and a coding agent driven by an
API key (which cannot mint `owner:acts`) cannot clear it.

## Failure modes, written before the fix

1. Reject operation Y while the current receipt is the consumption of operation
   X (X is not Y). Must be admitted. This is the bug.
2. Reject operation X while the current receipt is X's own consumption (the
   idempotency key `accept-once-consumed:X:...` or
   `qualified-consensus-consumed:X:...`). Must stay refused: X has already been
   approved, and the receipt is the approval's proof. (The operation state check
   at `:508` would also refuse, since X is no longer awaiting approval.)
3. Reject operation X under an owner disarm receipt of X's `accept_once` arm
   (`old_mode=accept_once`, `consumed_operation_id=X`, an owner-written key, not
   a consumption key). Must be admitted: X was never approved.
4. Same as 1 with `old_mode=qualified_consensus`. Must be admitted.
5. An unsigned, forged, other-computer or non-current receipt. Must stay
   refused (unchanged checks before the reject branch).
6. A decision other than `approve` or `reject`. Must stay refused.
7. Approving under a spent receipt of a different operation. Must stay refused
   (`verifyConsumedModeReceipt`, unchanged).

## Not changed here

An armed `accept_once` mode for operation X refuses a reject (`mode !=
propose_only` returns "mode receipt does not match current authority" at
`:704`). Rejecting while an arm is live needs the owner to disarm first. That is
a smaller gap and a separate decision (whether a reject should leave the arm in
place), so it is named as a residual, not folded in here.

## Classification

Fix class: red (self-development authority), tier 1, behind no switch, and it
only widens a fail-safe decision. Rollback: revert the commit.
