# The paper's live writes are refused (or rewritten) by the deposit upcast: prose leaves with "-super-" read as identities (2026-10-16)

Status: fixed in b8feaa147 and deployed 16:30; outcome below. Third symptom of the class in
the clustering assessment of
`paper-boot-refused-vocabulary-migration-capture-label-override-2026-10-16.md`.
**A boot with a replay tail can take the paper down again.**

## Evidence

Paper console (`console.log.1`), 15:38:21–15:39:21 UTC, four refused
writes, all through `projection dry-run refused batch: deposit upcast:
op 0 (object_recorded): og migrate: <kind> <id> has migrated identity
fields but no verified identity formula`:

| Time | Kind | What was lost |
|---|---|---|
| 15:38:21 | `choir.commitment_record` | an rss:ars capture arrival ("3 captures pending for the next poll") |
| 15:38:21 | `choir.source_schedule` (twice) | the rss:ars poll's save ("3 pending captures not saved"; the poll failed) |
| 15:39:21 | `choir.event_membership` | the event membership of one web capture |

There are no refusals before 15:38 or after 15:39 in the logs since 13:08.
The paper kept running; the other sources polled.

## Mechanism (code-confirmed; the activation time is inferred)

1. **The upcaster turns on for live writes once a ledger exists.**
   `depositUpcastForBatch` → `resolve(replay, fresh, cutover)` activates
   on a replay against a cut-over store and creates the ledger sidecar
   (`vocab-deposit-upcast.jsonl`). A store with a migration report is
   cut over from the moment it opens (`store.go:658`). After that, the
   live finalize path upcasts **every** deposit, for the store's life.
2. **The paper's only replaying boot today was 12:49.** "projection
   recovery resume … tail=1" at 12:49:17; every other boot today (00:58,
   01:43, 02:14, 02:28, 02:44, 02:54, 07:30, 10:50, 13:06) logged tail=0.
   An event membership resolved at 09:05 still carries the URL
   `…/egypts-climate-resilient-super-corals-…` unrewritten, so live
   writes were not upcast then. Inference: the 12:49 tail replay created
   the ledger.
3. **The upcast applies the V1→V2 rules to every string leaf.**
   `ogLeafMigrate` says "Whitespace-bearing leaves are prose, never IDs
   or tokens", but it checks only leading and trailing whitespace
   (`trimmed != v`). A title such as "a super-fast …" or a URL slug such
   as `…-super-corals-…` passes, and `migrateIDForward` rewrites
   `-super-` → `-management-` inside it.
4. **Then one of two things happens.** A kind with a verified identity
   formula (or a generic `*_id` candidate that verifies) is **accepted
   with its prose rewritten**: silent corruption of titles and URLs.
   A kind without one is **refused**, and the write fails. The four
   15:38 refusals are the second case. No instance of the first case has
   been observed on the paper's event and schedule surfaces (0
   `-management-` hits), but the code path is unconditional.

## Correction to the 12:49 boot doc

That doc said "every boot runs the migration" and that any boot after
10:50 would have failed. That is wrong. `MigrateAndFenceServingVocabulary`
takes its fast path when the boot replayed nothing and the report is
fenced. The 12:49 boot failed because its tail=1 replay forced the full
rescan (the S0 finding, `s0-vocab-rescan-fires-on-any-replay-2026-10-01.md`).
The 13:07 boot replayed nothing (tail=0) and took the fast path, so
3d669e89b's formulas were not what let the paper boot. The open
hypothesis in that doc's clustering section is answered: the paper's
report is fenced; the 12:49 boot was a replay.

## Risk now

The next boot with a replay tail (any deploy whose planned restart
lands between an append and its projection) runs the full rescan again.
The 09:05 event membership above has a `-super-` URL leaf; event
memberships have no identity formula, and the live path already refuses
the same shape (15:39). **That boot would refuse, as at 12:49.** A deploy
of any commit is the trigger, so the fix must ship in the binary that
runs that boot.

## Class

| Incident | Path | Kind | Effect |
|---|---|---|---|
| 2026-10-15 | checkpointd replay | `choir.commitment_record` | two QA computers' checkpoints never rebuild |
| 2026-10-16 12:49 | boot rescan (replay tail) | `choir.capture_label_override` | the paper down 18 min |
| 2026-10-16 15:38 | live deposit upcast (ledger present) | `commitment_record`, `source_schedule`, `event_membership` | an rss:ars poll and one membership lost |

All three are one defect: prose leaves pass `ogLeafMigrate`. Adding
formulas kind by kind (3d669e89b) moved the failure to the next kind.

## Fix direction (substrate, red)

Make `ogLeafMigrate` keep its documented contract: a leaf with any
whitespace, or with a URL scheme (`://`), is prose or a locator, never
an ID or desk token, and is never rewritten. This is option 1 of the
clustering assessment, narrowed to what the code already claims.

- V1 identities (`super:…`, `run-super-assignment-…`, desk tokens) never
  contain whitespace or `://`, so the one-time migration of genuine V1
  rows is unchanged.
- The serving fence (`VerifyServingVocabulary`) checks role-bearing keys
  as whole values, not prose, so it is unaffected.
- Replay and live use the same function, so they stay equal going
  forward. A row accepted with rewritten prose between 12:49 and the
  fix would replay differently on a fresh reconstruction; none has been
  observed.
- It does not cover a bare slug field with no whitespace and no scheme
  (`climate-resilient-super-corals`). That stays a residual.

Not chosen: freezing the boot migration (option 2) leaves the live path
and checkpointd replay broken; making a refusal per-object (option 3)
keeps the loss.

## Outcome (16:30–16:40)

- **The fix held under the risky case.** The deploy's planned refresh
  of the paper booted with a replay tail ("projection recovery resume
  … tail=1", 16:29:59). The full rescan ran and passed. No "identity
  formula" refusal appears in the console since the boot. The paper
  reports `ready` on b8feaa147.
- **The deploy job failed anyway, but not on the fix.** The full rescan
  took 4 minutes (16:30:00 → 16:34:00, "replay complete"), and vmctl
  logged `outcome=refreshed` at 16:34:00. The deploy's refresh request
  got an empty reply (curl 52), because vmctl's HTTP write timeout was
  the 120 s default. (An earlier version of this note blamed the 5 s
  diagnostic curl; that curl only ran after the failure.) Problem doc:
  `deploy-refresh-empty-reply-vmctl-write-timeout-2026-10-16.md`. Fix:
  173d02b68, deployed 16:57. The job's rerun at 16:45 succeeded. It
  deployed main's head (028af566e, docs only), so it restarted the
  paper once more: a planned restart, tail=0, 37 s.
- **Still open:**
  - the full rescan on any replaying boot costs about 4 minutes on the
    paper (the S0 finding, `s0-vocab-rescan-fires-on-any-replay-2026-10-01.md`);
  - checkpointd's two refusing QA computers (2026-10-15) are in failed-job
    backoff. Whether this fix clears them shows at their next retry.
