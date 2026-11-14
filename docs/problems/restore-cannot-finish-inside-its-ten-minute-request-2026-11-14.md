# A restore on d@e cannot finish inside its ten-minute request

Recorded 2026-11-14T22:57Z. Not repaired.

## Problem

`choir computer restore` is one blocking request with no async form and no
status route. Every budget on its path is ten minutes (proxy
`PROXY_REPLAY_COMPLETENESS_TIMEOUT=10m` in `nix/node-b.nix:961`; the guest's
write deadline in `internal/agentcore/api_self_development.go:31`). The work
is the scan, the install of the newest snapshot at or below the checkpoint,
the replay of the tail at about 113 events a second, then the witness check,
then the flip. A checkpoint that sits just below a snapshot falls back to the
snapshot before it. On d@e that was 74,000 events: 16 to 20 minutes of work.
The client gives up, the cancel aborts the replay, nothing is flipped, and
the caller is told nothing.

Three smaller defects seen on the way:

- A witness mismatch is not in `rematerializeRefusalStatus`
  (`rematerialize.go:374`), so it would answer 500, and only after the whole
  replay. Read from code, not observed.
- The restore holds `currentReleaseMu` for its whole run; the hourly cadence
  checkpoint waited on it and logged `held appends 6m17s`. That figure is the
  bind's wall time, not an append hold (`checkpoint_cadence.go:295`).
- The recovery bound is in events (720,000, `recoveryplan/recovery_plan.go:22`)
  with no matching time budget.

## Evidence

- Step 7 of the step 2 proof, run by an operator agent on d@e under N7.
  Checkpoint 189a91ed..., 21:34:02Z, sequence 108,784, `owner_recovery`,
  verdict `unverified`/`no_base`. d@e's own key carries `owner:acts`.
- Request 1, 22:38:33Z: Caddy status 499 after 74.9 s (the CLI's default
  timeout). Request 2, 22:43:29Z with a nine-minute timeout: 499 after 539.9 s.
- Guest log: staged store opened 22:47:35Z; `replay applied 33332 events
  through seq=68048 in 4m54s` at 22:52:29Z, the second the client gave up;
  the replay started after sequence 34,716. No flip, no witness line.
- After: d@e active, realization epoch 13706 unchanged, the revision written
  after the checkpoint still present, same process since 22:28:21Z.
- Guest memory read 1.2 to 1.7 GB against a 1,024 MiB ceiling during the
  second request; vmctl reclaimed each time; no kill.

## Belief

The two requests did nothing durable beyond the restore intent event
(idempotent per checkpoint; not checked on the tape). A third request with
the same checkpoint would fail the same way. A checkpoint at or above the
newest snapshot would replay a short tail and could finish; the cadence bind
published one at sequence 110,778 at 22:52:48Z.

Even with time, the restore's witness is the strict comparison, and d@e's
replay differs from live on the event index
(`replay-of-de-differs-on-the-event-index-only-2026-11-14.md`). The restore
may refuse on that. Not observed.

## Remaining error

- Whether the staging directory was removed on the guest; what request 1
  reached before the cancel.
- The repair: restore as an accepted act with a status to poll, its budget
  tied to the events to replay, and a witness mismatch answered as a refusal.
  Not designed.
- Steps 7d to 7f of the step 2 proof are not done.
