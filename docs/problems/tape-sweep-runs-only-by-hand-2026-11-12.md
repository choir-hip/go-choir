# The tape sweep runs only when a person drives it (2026-11-12)

Date: 2026-11-12. Investigation record. Nothing in it is built. Station S7 of
[choir-storage-solutions-2-2026-11-12](../definitions/choir-storage-solutions-2-2026-11-12.md).
The design of the sweep is in
[tape-sweeps-destroyed-and-orphans-2026-10-20](tape-sweeps-destroyed-and-orphans-2026-10-20.md).

## Problem

Unreferenced tape (orphan pins) and the tape of destroyed computers are
reaped only when a person posts a dry-run to
`/internal/platform/artifact-gc/tape`, reads the manifest, and posts the
apply with the run id and manifest hash. Nothing schedules it. Tonight the
first dry-run of the destroyed sweep (`ts-20261112T202150Z-a31549a9`) also
died at the 20:53Z deploy, because the references phase lived in memory and
took more than three hours.

## Evidence

- Code read: `internal/platform/tape_sweep.go` (dry-run, `clampTapeSweepRequest`),
  `tape_sweep_apply.go` (apply), `tape_purge.go` (protected list, purge),
  `cmd/corpusd/main.go` (the only periodic job is the artifact `GCRunner`; the
  tape sweep comment says "on demand only"), `nix/node-b.nix` (the tape
  sweep's mode is a setting, `PLATFORM_TAPE_SWEEP_MODE`, `active` since
  2026-10-20).
- The branch `lane/sweep-survives-restart` (commits 1 and 2) makes the
  references phase durable and incremental, which removes the reason a
  scheduled dry-run could not finish. A dry-run now walks the events since the
  last sweep (plus the rows after the first unreadable event of a computer);
  it still scans the three namespace directories and sorts the whole
  reference set, so it is not free. A resumed sweep does not re-verify the
  media of ranges its stored progress covers: a corrupt segment or a vanished
  event file inside them does not raise the unreadable refusal that a
  from-scratch walk would (references are unaffected, so nothing is wrongly
  deleted). `chain_events_resumed` in the report says how much was not
  re-read; run a from-scratch walk (delete `tape_sweep_ref_chunks`) when
  media is in doubt.

## What the sweep guards today, and what each guard would mean unattended

| Guard | Today | Unattended |
| --- | --- | --- |
| Manifest hash | A person reads the manifest; apply must repeat the run id and sha256, and re-reads the file and its sidecar. | The reviewer disappears. The hash then only proves apply runs the manifest its own dry-run wrote, so a corrupt or swapped file is caught, but a wrong proof is not. The review has to be replaced by rules the code checks (below), not by the hash. |
| Registered and protected computers | The destroyed proof needs: no ownership record, not in the registry list, no route slot, head quiet for 7 days (never under grace), not on the hard-coded protected list (the paper, the owner computer). Apply re-proves all of it at delete time and also checks the head did not move. | These hold unattended. They are code, not review. The weak point is the registry: any registry or route failure refuses the whole destroyed sweep (fail closed), which is right, but an unattended run must report the refusal where a person sees it, not retry silently. |
| `accept_unrecorded_fork_lineage` | The caller asserts that no fork-lineage record exists to check. Without it the destroyed sweep lists nothing. | An unattended run would assert it every time, forever. Forked computers are siblings with their own ComputerID and chain (docs/computer-ontology.md); a destroyed parent's payloads that a live fork still names are kept because the live set names them, but there is still no lineage store to prove a parent is safe to reap. Either build the lineage record the proof already names, or keep the destroyed sweep out of the schedule. Setting the flag in a timer is the one thing that must not be done. |
| Archive | Default on: files and a rows dump go to a verified `tar.zst` under `tape-sweep-archive` before any unlink; a tombstone then refuses reads and appends. `no_archive` is test computers only and must be repeated on apply. | The archive stays on Node B, on the same disk that is short of space. An unattended run needs a free-space guard before it writes an archive, and must never use `no_archive`. `prune-archives` deletes the archive after 7 days; that is a drop of history, and stays by hand (D1). |
| Caps | 2M deletes and 8 GiB per apply; a capped run defers the rest. | Keep them. They bound a wrong manifest. |
| Delta re-check at apply | Apply reads every event appended since the dry-run snapshot and every extra record, and keeps anything newly referenced. | Unchanged, and a timer makes it more valuable. |
| Mode | `off`, `dry-run`, `active`. | The setting is a switch in the sense of CLAUDE.md "No switches": an unattended collector that runs only if the setting says so. See the open question. |

## What it would take

Split by what the sweep deletes, because the two sweeps differ in risk.

1. Orphans (unreferenced pins past a 24 hour grace, legacy namespaces only;
   tape segments are never in this sweep). A scheduler beside `GCRunner`:
   every hour start a dry-run through `StartTapeSweep` (the run lock already
   makes a second start a refusal, `errTapeSweepBusy`), then apply it with
   the manifest hash the job returned, when: no refusal in the report
   (`OrphansRefused`, warnings), the receipt check matched, and the eligible
   count and bytes are under a bound. This deletes nothing a chain, a
   closure, a program, a checkpoint or a certificate names, and archives
   first. It never reaches a registered computer's sealed segments.
2. Destroyed computers. Everything in the proof exists already. What does not:
   a fork-lineage record (so the flag is not asserted blind), a free-space
   guard for the archive, an alarm on any refusal, and an owner decision that
   the archive of a destroyed computer is the retention and not a drop.
3. Observability: a log line and the existing hourly report carry run id,
   hash, counts, refusals, and the growth meter before and after, so a
   person can see an unattended run without driving it.
4. Restart behaviour (CLAUDE.md Restarts): a crash or deploy mid-run does not
   resume. The next tick starts a new dry-run; the references walk now finds
   its stored chunks, and an interrupted apply is already idempotent (the
   tombstone states `archiving`, `archived`, `deleted` and the GC deletion
   log record intent before each unlink). No boot-time resumer.

Forbidden and not proposed: an automatic apply that deletes the sealed tape
of a registered computer. The proof refuses one by construction (ownership,
registry, route, protected list), and a schedule must not add a path around
that. Owner ruling D1 stands: no tape segment leaves Node B and old segments
are not dropped yet.

## Belief and remaining error

- Belief (0.8): the orphan sweep can run on a timer with the existing
  protections and a bound on the manifest, with no change to what a person
  reviewed on 2026-10-20 beyond replacing the reader with bounds. It reaps
  pins only.
- Belief (0.7): the destroyed sweep is not ready for a timer until the
  lineage record and the archive free-space guard exist.
- Unknown: how many bytes the hourly orphan sweep would find once the
  watermark makes the dry-run cheap; the 2026-11-12 figure of 35 GiB for
  destroyed computers is the larger prize, so item 2 above is where S7's
  value is, and it is the part gated on the flag.
- Unknown: whether an unattended dry-run every hour is itself a cost
  (a full scan of three namespace directories); measure on the first
  incremental dry-run before choosing the interval.

## Open question for the owner (no safe default that removes it)

`PLATFORM_TAPE_SWEEP_MODE` is an existing setting. Under "No switches" a
scheduled collector should not exist behind it. The conservative reading is
to leave the setting as it is for the on-demand endpoint and add the
scheduler without a new switch for the orphan sweep only, since it deletes
nothing a chain names. Recorded as a residual; not decided here.
