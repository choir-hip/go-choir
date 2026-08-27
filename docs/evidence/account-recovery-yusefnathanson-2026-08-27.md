# Account Recovery Problem Evidence — yusefnathanson@me.com

**Observed at:** 2026-08-27 UTC
**Environment:** staging Node B (`https://choir.news`)
**Mutation class:** red (VM lifecycle, route authority, account availability)
**Status:** problem documented; repair not yet landed

## Problem

Owner account `yusefnathanson@me.com` (`5bd6de97-3b58-408c-bf89-c42c81b083de`) has valid identity and mailbox records but no usable primary computer surface. Its primary desktop is bound to sealed computer `computer-03335285269bdba4f94377e56879f9e6` / VM `candidate-fleet-e15cb89f25d963c220319b7b`, which is stopped under a host maintenance hold. Ordinary resolve therefore cannot make the desktop servable.

The account's email alias `000@choir.news` remains healthy. A Resend `email.received` webhook at `2026-08-27T22:59:16.256Z` persisted provider message `18d1c22b-d0cf-4146-8a2e-61e0b4a3de4c` from `ryan@refer.io`, subject `Send me api info`, to the owner's mailbox.

## Evidence

- `/var/lib/go-choir/auth/auth.db`: user and WebAuthn credential for `5bd6de97-3b58-408c-bf89-c42c81b083de` exist; API keys are present.
- `/var/lib/go-choir/mail/mail.db`: `email_aliases` maps `alias-choir-news-000` / `000` on `choir.news` to the owner.
- `/var/lib/go-choir/mail/mail.db`: the Ryan message is present in `email_webhook_events` and normalized `email_messages`.
- `/var/lib/go-choir/vm-state/ownerships.json`: primary ownership is the sealed 0333528 VM, `state: stopped`, with hold reason `protect-live-guest-during-hang-diagnosis`.
- Platform Dolt `computer_version_route_slots`: `computer:5bd6de97-3b58-408c-bf89-c42c81b083de:primary` remains generation 1 and binds an immutable ComputerVersion route. `vmctl.HandleResolve` refuses a missing or non-joining realized ownership.
- Attempts to start/refresh the 0333528 VM reached guest replay but repeatedly failed or stalled; ordinary `start`, `refresh`, `resolve`, and `resume` did not produce a servable owner computer. The latest supported hold operation reasserted the hold after diagnosis.
- Agentic consensus panel output is retained in `.agentic-consensus/agentic-consensus-20260827-191158/`. The strongest convergent finding is that the outage is a missing product-path operation for serving the sealed realization under the guest-visible maintenance fence, not an auth or mail identity failure. The panel rejected manual ownership/route edits and rejected clearing the hold without the guest fence.

## Belief update

**Before:** account recovery appeared to be a routine boot or fresh-primary choice.

**Now:** identity and mail are not the blocker. The active blocker is the absence of a supported, authorized maintenance-serve operation that can boot the existing route-bound realization with `RUNTIME_MAINTENANCE_HOLD=1` while preserving its host hold. The sealed computer must remain held until that operation is implemented and deployed, or a separately ratified route-replacement transaction exists.

## Remaining error field

- No deployed product path currently exposes `RecoverVMForDesktopMaintenance(..., replay_only=false)` / an equivalent maintenance-serve operation.
- The retained VM's current `data.img` and exact local head have not yet been re-established as a clean, guest-visible-hold boot proof.
- The account has not yet passed authenticated desktop login and mail UI acceptance.

## Safety / rollback

- Host hold was reasserted through the vmctl hold control and must remain set.
- The retained 0333528 VM, all `data.img.*` snapshots, route generation 1, canonical tape, and mailbox data must not be deleted, rewritten, or replaced.
- No route slot or auth record was changed by this receipt.
- The candidate-proof operation on the separate `new@new.com` computer remains tabled; its pre-A checkpoint and operation artifacts are preserved.

## Next safe probe

Implement and test the smallest internal-only maintenance-serve control, with explicit authorization, requiring an existing host hold and injecting the guest-visible hold. Deploy through the normal CI/staging loop. Then boot 0333528 from a verified caught-up image, prove health and route join without self-development rewarm, and only then perform owner login/mail acceptance.
