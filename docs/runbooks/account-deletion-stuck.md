# Runbook: an account deletion that is stuck

Date: 2026-11-24. Mutation class: **black** (it deals with a purge that
deletes personal data in production and cannot be undone once begun).
Design: [account-create-delete-2026-11-23.md](../design/account-create-delete-2026-11-23.md).
Change entry: [changes.md](../changes.md), "Delete an account".

A deletion has two stages and the state, not the clock, is the boundary:

| State | What was removed | Cancel |
| --- | --- | --- |
| `pending` (even past its due time) | nothing: access revoked, computers stopped, takes held back, gateway identity forgotten | the person signs in with the passkey and chooses Cancel deletion |
| `purging` | the steps recorded in `steps_done`, in the order mail, data, computers, gateway, tape | none |

The runner (`internal/accountpurge`, started by `cmd/auth`) looks at every
open deletion once a minute. A pass that fails is retried the next minute and
never skips a step. Three alarms name a stuck deletion in the auth journal:

- `ALERT job=... preflight keeps failing, stuck for <age> past the deadline;
  the account is untouched and still cancellable`: the deletion is due, state
  `pending`, and vmctl's preflight does not pass. Nothing was removed. The
  person can still cancel. Fix the cause below.
- `ALERT job=... purge halted ..., stuck for <age> past the deadline`: state
  `purging`; a step failed or refused. Data may be partly removed. The step is
  in the log line and in `last_error`.
- `ALERT job=... the deletion ledger could not be written`: the purge stops
  before it removes anything (or before it deletes the user row) because the
  restore-from-backup guarantee needs the ledger line first. Fix the disk or
  the permissions of the directory that holds `auth.db`.

Both age alarms start one hour past the due time (or after five attempts).

## Look

```bash
ssh node-b 'sudo -u go-choir auth deletion-ops list -db /var/lib/go-choir/auth/auth.db'
```

One line per open deletion, by job id (never by person): state, how long past
due, attempts, the steps done and the last error. The journal line for a step
names the step and the computers (`journalctl -u go-choir-auth | grep job=del-...`).

## What each refusal means, and what to do

The tape step refuses by naming a computer and a reason. A refusal HALTS the
purge: the user row is never deleted past it, because it is what lets anyone
find the account to finish it.

| Reason in the log | Cause | Clears by itself? | Action |
| --- | --- | --- | --- |
| `head moved within the quiet window` | the computer's tape was written in the last window | yes, when the window closes | wait; if a computer keeps writing, find why (a stopped computer should not) |
| `has route slot` | a route slot still names the computer (the data step deletes the account's slots) | no | confirm the data step is done (`auth deletion-ops list`); if the slot belongs to a computer vmctl no longer lists, remove it with the platform's route tool, then `rerun -steps tape` |
| `in registry list` | the computer is still in the computer registry | no | vmctl's destroy did not remove its registry entry: check `vmctl` state for the computer; once vmctl lists none, `rerun -steps computers,tape` |
| `ownership record exists` | an ownership row remains for the computer | no | same as the registry; the destroy step failed halfway: `rerun -steps computers,gateway,tape` |
| `protected computer: hard refusal list` | the account owns the paper's or the owner's computer | never | the purge must not run for this account. Do not edit the list. Cancel is impossible (state purging); this is an owner decision: stop and ask. The preflight should have refused earlier; record how it was missed |
| `fork lineage unrecorded; not acknowledged` | the sweep wants the fork-lineage acknowledgement | no | a sweep-side change: ask the owner; do not acknowledge by hand for a deletion |
| `chain unreadable: ...` | the tape head cannot be read | no | read the file named; do not delete it by hand |
| `no proof for <id>` or `destroyed sweep refused: ...` | the sweep did not answer for a named computer | maybe | read corpusd's log; fix and `rerun -steps tape` |

A preflight failure (`preflight: ...`) names vmctl's refusal:

| Reason | Action |
| --- | --- |
| vmctl cannot read the deletion record (`ReadOnlyPaths`) | the read of `auth.db` from vmctl is unproven on Node B (see the change entry): check `nix/node-b.nix` `ReadOnlyPaths` and `VMCTL_RETENTION_AUTH_DB_PATH`. The person can still cancel until this is fixed and the purge begins |
| a computer is held or has a live capsule | wait for the hold or capsule to end; do not release it for the deletion |
| vmctl is down | restart vmctl through its unit |

## Run steps again

```bash
ssh node-b 'sudo -u go-choir auth deletion-ops rerun -db /var/lib/go-choir/auth/auth.db -job del-... -steps tape'
```

This forgets the named steps of one job (`mail`, `data`, `computers`, `gateway`,
`tape`) so the runner does them again on its next pass; every step is
idempotent. It refuses a job that is not `purging`, an unknown step and an
unknown job, and changes nothing then.

There is no command that finalizes a deletion past a refusal or deletes a
record. Forcing a purge past a refusal is an **owner act**: it leaves personal
data (a computer's tape) in place with no way left to find the account. If the
owner decides to do it, record the decision and the evidence in the change log
first.

## After a restore from backup

The ledger travels off-host with every backup run (`scripts/off-host-backup.sh`).
Read the newest ledger against the chosen snapshot's `auth.db` before the
restored host serves anyone:

```bash
bash scripts/off-host-restore-drill.sh <chosen snapshot> deletions        # reports; exit 1 if accounts would come back
auth deletion-restore-check -db <restored auth.db> -ledger <newest ledger> -reapply   # applies
```

## Not proved

The ledger copy and the drill piece are proved by `scripts/off-host-backup-ledger-test.sh`
and `scripts/off-host-restore-drill-deletions-test.sh` against fakes. Neither
has run against the real repository on Node B; the deployed restore drill has
not been run for this change.
