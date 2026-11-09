# A computer restored to base keeps serving the frontend of the day it was restored

Date: 2026-11-09. Status: open, not fixed. Reported by the owner on staging
("b@c is on old code. settings is old. the app dock is old").

## What the owner saw

b@c's desktop showed the old Settings window ("Desktop preferences", one
Account line) and the old app dock on the left. The owner also noted that
the candidates b@c builds in self-development show the current code.

## Evidence (read-only, 2026-11-09 about 22:55 UTC)

- b@c's observability reports the platform binary at the deployed commit
  (`cb149a273`), marker `genesis-baseline`, not layered, not diverged,
  source projection `base` at the same commit.
- The frontend b@c serves is a different bundle from d@e's
  (`app-h2oMqBB3.js` against `app-D7tQ9fzZ.js`). Its files carry
  `last-modified: Sun, 08 Nov 2026 01:05:40 GMT`. That is the minute of
  b@c's base restore (modular architecture ledger, 2026-11-08, "b@c
  cleaned", rollback-373448e1).
- b@c's bundle has none of the strings "Add another account",
  `add_account` or "Restart to update". d@e's bundle has all three.
- d@e serves a layered self-development release (marker `selfdev-…`), which
  is grafted onto each new base at boot. Its frontend files are dated today.

## Cause (from the code)

- The surface serves `$CHOIR_UPDATER_ROOT/current/frontend` whenever it has
  an `index.html`, and falls back to the deployed baseline only when it does
  not (`internal/autoputer/computer_surface.go`, `resolvedRoot`).
- A base restore copies the baseline of that moment into the updater store
  and points `current` at it (`internal/agentcore/selfdev_base_restore.go`,
  `updater.StageBaselineRelease`).
- At boot, `ensureServingBaseline` imports the baseline only when `current`
  is missing or invalid (`internal/agentcore/rematerialize.go:568`). A valid
  copy from an older deploy is kept as it is.

So after a base restore the binary follows every deploy, and the frontend
stays at the restore's commit until the next self-development apply or
restore. Capsules build from the source projection, which is at the current
base, so candidates look current. This matches both things the owner saw.

## Not yet checked

- Whether a computer that never ran self-development has the same stale
  copy. The same only-when-missing import would apply to it. The signed-out
  page comes from the proxy, so it does not show this.
- The owner's own computer and the paper. Read their served bundle dates.
- The on-disk state of b@c's `current` link was not read; the evidence is
  the served files and the code.

## Consequence for the account switcher report

b@c's launcher predates the switcher. After the owner added b@c as a second
account, the page ran b@c's old frontend, which has no account rows at all.
That alone explains "no ability to switch back"
([account switcher problem](account-switcher-no-switch-back-2026-11-09.md)).

## Remedy

- For b@c now (an owner act, not taken): any new base restore restages
  `current` from the deployed baseline. An applied self-development round
  does the same through the graft.
- Fix direction (not built): a `current` that is a pure base import
  (`releaseIsPureBaseImport` already tests for it) and whose content digest
  differs from the booted baseline's is restaged at boot, or the surface
  serves the booted baseline directly whenever `current` is a pure base
  import. The second deletes a copy instead of keeping it fresh. Boot path
  and serving join: red, two review seats, proved on d@e (restore to base,
  deploy, check the bundle date).
