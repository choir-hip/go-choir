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
- `/var/lib/go-choir/vm-state/ownerships.json`: primary ownership is the sealed 0333528 VM, initially `state: stopped`, with hold reason `protect-live-guest-during-hang-diagnosis`. After the repair it is `state: active`, epoch `804`, with the hold still present.
- Platform Dolt `computer_version_route_slots`: `computer:5bd6de97-3b58-408c-bf89-c42c81b083de:primary` remains generation 1 and binds an immutable ComputerVersion route. `vmctl.HandleResolve` refuses a missing or non-joining realized ownership.
- Attempts to start/refresh the 0333528 VM reached guest replay but repeatedly failed or stalled; ordinary `start`, `refresh`, `resolve`, and `resume` did not produce a servable owner computer. The hold was reasserted through the supported vmctl hold control.
- A first maintenance-serve attempt failed before guest boot with Firecracker `TapOpen(IfreqExecuteError(...))`. The VM manager killed the orphaned Firecracker PID but left stale tap device `vm-lackej5pmie2`. This separate substrate defect was documented before repair.
- Source repair `c4b7a9a581fbc190dfe291c6b196f9c71b3a7c5a` cleans stale VM-specific tap devices during orphan reclamation. Deployed CI run `33129455918` activated it on Node B at `2026-08-28T00:43:40Z`.
- Source repair `716850f5bc19eac6123c1dc24a8f2b3794034683` exposes internal-only `POST /internal/vmctl/maintenance-serve`, requiring the existing host hold and invoking `RecoverVMForDesktopMaintenance(..., replay_only=false)`. The deployed endpoint booted 0333528 under the guest maintenance hold.
- Agentic consensus panel output is retained in `.agentic-consensus/agentic-consensus-20260827-191158/`. The strongest convergent finding is that the outage is a missing product-path operation for serving the sealed realization under the guest-visible maintenance fence, not an auth or mail identity failure. The panel rejected manual ownership/route edits and rejected clearing the hold without the guest fence.

**Now:** identity and mail were not the blocker. The account is restored to an active, route-joined, guest-ready computer through the maintenance-serve path. The host hold remains intentionally set, so self-development and automatic lifecycle mutation remain fenced.

## Recovery proof

- Node B `/health`: `status: ok`, `vmctl_status: ok`, deployed commit `c4b7a9a581fbc190dfe291c6b196f9c71b3a7c5a`.
- Maintenance-serve response: computer `computer-03335285269bdba4f94377e56879f9e6`, VM `candidate-fleet-e15cb89f25d963c220319b7b`, epoch `804`, `state: active`, `held: true`.
- Guest health at the resolved URL: `status: ready`, `runtime_health: ready`, `running_runs: 0`, matching computer ID and deployed commit.
- Authenticated CLI status: `state: active`, epoch `804`.
- Authenticated `GET /api/shell/bootstrap`: HTTP `200`, owner `5bd6de97-3b58-408c-bf89-c42c81b083de`, matching computer ID.
- Authenticated `GET /api/email/messages?folder=inbox`: 50 messages returned; Ryan's message appears with sender `ryan@refer.io`, subject `Send me api info`, received `2026-08-27T22:59:14.921Z`.

## Remaining error field

- The host maintenance hold remains by design; ordinary resolve must not auto-start or mutate the computer.
- Full interactive WebAuthn browser login was not exercised in this shell session; authenticated product API and shell bootstrap acceptance passed.
- Mail remains host-side `maild` data; guest Maildir synchronization is not claimed because Track M drain is not wired on the live path.

## Safety / rollback

- Host hold was reasserted through the vmctl hold control and must remain set.
- The retained 0333528 VM, all `data.img.*` snapshots, route generation 1, canonical tape, and mailbox data must not be deleted, rewritten, or replaced.
- No route slot or auth record was changed by this receipt.
- The candidate-proof operation on the separate `new@new.com` computer remains tabled; its pre-A checkpoint and operation artifacts are preserved.

## Next safe probe

Exercise the browser WebAuthn sign-in and Email app surface against the active held
computer. Do not clear the hold; do not claim guest Maildir delivery.
