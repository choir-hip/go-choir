# Self-development apply: the checkpoint never sees a quiet computer (2026-10-10)

Found by the seventh Gate 2 reality rerun (M11 probe) on staging, build
2a16a6db, disposable `computer-520c29ba…` (VM `vm-1a9c51e0…`), operation
`selfdev-72caae58…`. Written 01:19Z, while the probe is still waiting.
Problem first; no fix in this commit. Mutation class of a fix: red
(checkpoint, run lifecycle after a planned restart).

## How far it got

For the first time since S2: implementation froze a candidate
(01:07:18Z), the independent verifier recorded a **pass** (the 3e69567b
mirror fix held), the operation reached `awaiting_approval`, the probe
approved it, and materialization began with a planned restart
(`restart kind=planned reason=self_development_apply`, 01:13:51Z).

## What happens next (guest console)

- The verifier run (`run:assignment-f59a636d…`) was still iterating
  after recording its verdict (as in rerun 5). The apply restart cut it;
  boot passivated it and its assignment was cancelled; the cancel report
  (`blocker`, 01:13:54Z) reached Texture (the 028446a5 binding fix lets
  it through) and a Texture supervision run (`280c9590…`) started and
  was still iterating at 01:18Z (iteration 34+).
- Every ~47 s from 01:14:41Z the materializer retries and fails:
  `self-development checkpoint: replay completeness: reconstruct event
  chain: computer event projection repair required`, with the replayed
  head 10–24 events behind the platform head (local seq 1368 → 1541,
  platform 1392 → 1551 across retries).

## Cause (code reading)

`ReplayCompleteness` (`agentcore/replay_completeness.go`) replays the
chain into a disposable workspace and then requires the platform head to
equal the replayed head (`computerevent/appender.go`
`ReconstructInto`), and it separately refuses if the live head moves
during the probe. It is designed for a quiet computer. After a planned
restart, pre-boot work resumes by the owner's rule ("if we're rebooting
to do an update … we can resume work"), and any desk that appends events
during the checkpoint starves it.

Two contributing facts: the verifier keeps working after its verdict
(so the apply restart always cuts a live run), and a cancel report now
wakes a Texture supervision turn.

## Fix directions (to decide; red)

- A: the materializer holds new desk activations (the actor dispatcher)
  from the planned-restart boot until the checkpoint and route
  projection finish, then releases them. Work still resumes after the
  update, as the owner's rule says; it just waits for the checkpoint.
- B: the checkpoint replays up to a head captured at its start and
  compares the live state at that head, tolerating later appends. Larger
  change to a protected verifier.
- Also: the verifier's run ends once it has recorded its verdict, so the
  apply restart does not cut a live verifier.

Leaning A (smallest, keeps the checkpoint's quiet-computer contract).

## After the computer went quiet (01:25Z)

The Texture supervision run hit its tool-loop budget at 01:19:40Z. The
next retries show the checkpoint stepping through three failures:

1. 01:20:29 `projection base refused: tail application is not
   exactly-once (1680 events, [1,1680], want (0,1673])`: the budget
   failure's own events landed during that replay.
2. 01:21:24 onward, with the head now stable: `replay is ineligible:
   reconstructed projection is not equivalent to live state`.

The replay-completeness report (read-only, owner-bound guest route,
01:24Z) names two differing keys: `dolt:texture:table:og_objects` and
the content root that hashes it. No missing tables, no schema drift, run
memory equal. So some object-graph rows on this computer were written
outside the event chain, and replay cannot reproduce them.

What is new on this computer compared with the September 29 pass: boot
passivated a run that was still working at the apply restart (the
verifier), its assignment was cancelled, and a Texture run failed on its
tool-loop budget. Hypotheses, not findings:

- H1: boot passivation's `store.UpdateRun` (`agentcore/runtime.go`)
  writes the run object without a reducer event.
- H2: the tool-loop budget failure path writes the run's terminal state
  directly.
- H3: the assignment cancel at restart writes outside the chain.

The report hashes whole tables, so it cannot say which rows differ.
Next: a row-level og_objects diff in the replay-completeness report (by
object kind and key), then name the writer. The operation stays in
`materializing`, retrying.

## Fix plan A (01:31Z; red, ceremony)

- Conjecture delta: work may resume after an update restart (owner
  rule), but not until the apply's checkpoint has read a still chain.
- Change: after a boot whose planned-restart marker is
  `self_development_apply`, the actor handler defers work-kind
  occurrences (`restartWorkKinds`: dispatches, coagent results, channel
  messages, owner revisions, assigned work, spawn and resume deadlines)
  with `actor.ErrDeferUnprocessed` while a self-development operation is
  `materializing`, for at most 10 minutes after boot. Cancels, fail-closed
  deadlines and `selfdev_materialization_retry` are never held. After
  the operation leaves `materializing`, or the 10 minutes pass, work
  proceeds as before. The bound keeps deferrals far below the
  dispatcher's 64-deferral poison limit, so a stalled apply delays work
  and never destroys it.
- Failure modes pinned in a handler test: held while materializing; not
  held when nothing is materializing; not held after the window; cancels
  and the materializer's retry never held; crash boots unchanged; a
  planned platform update boot never held.
- Protected surfaces: run lifecycle after a planned restart (red).
  Admissible evidence: the next M11 rerun shows no desk turn between the
  apply restart and the checkpoint. Rollback: git revert. Heresy delta:
  discovered "resume during apply"; repaired only on staging proof.

## Rerun 8 on 5704ace6: the hold works; the diagnostic names the rows (02:45Z)

Disposable `computer-14bec738…` (VM `vm-419eeec3…`), operation
`selfdev-87882911…`. Implementation and verification took 16 minutes;
the apply restart came at 02:43:13Z. From then the engineering and
Texture desks logged "waits for the self-development apply checkpoint"
(fix A, 7ea0f66d). The first checkpoint (02:44:02Z) read a quiet chain:
no head-moving or tail errors, straight to the equivalence test.

The equivalence test still fails, and now says why:

`og_objects live_only=26 replay_only=26 changed=3`, by kind:
`choir.lifecycle_command` 24, `choir.lifecycle_event` 24, `choir.event` 4,
`choir.co_super_assignment` 2 (content and body differ),
`choir.co_super_assignment_report` 1 (content and body differ).

The live-only rows were written at 02:27:58, 02:28:12, 02:40:18–24 and
02:43:15: the engineering assignment's open, its report and freeze, and
its cancel at the apply restart. The matching 26/26 counts mean replay
builds the same number of lifecycle commands and events under different
keys. **Finding:** the engineering assignment lifecycle (open, report,
cancel) is not replay-deterministic. Either the keys include a value the
replay does not see the same way (a sequence, a time, a run id), or the
live write and the reducer build different objects. H1–H3 above are
refuted as stated: the divergence is the assignment lifecycle, not run
passivation or the budget path.

Next: read how `store/engineering_assignments.go` mints lifecycle
command and event keys and bodies, and how the reducer replays the same
event, before any fix.

## Rerun 8: the different keys are vocabulary spellings (02:55Z)

The 26/26 key difference is not nondeterminism. It is the vocabulary
upcast, and the trace and code agree on how.

- Every live store is cut over at boot
  (`autoputer/run.go` calls `MigrateAndFenceServingVocabulary`). After
  that the write guard (`vocabWriteGuard`) checks **role fields only**.
- Live writers still mint IDs that the frozen migration rule
  (`ogLeafMigrate` → `migrateIDForward`) treats as V1. The engineering
  assignment path mints lifecycle command IDs `co-super-open:`,
  `co-super-open-failed:`, `co-super-bind:`, `co-super-capsule:`,
  `co-super-cancel:`, `co-super-system-cancel:`,
  `co-super-restart-open-cancel:`, `co-super-restart-cancel:`,
  `co-super-report:` and `co-super-orphan:`, plus the digest refs
  `co-super-grant:sha256:`, `co-super-execution:sha256:` and
  `co-super-fate:sha256:`. Each contains the infix `-super-`.
- The live store writes those bytes. The probe's from-genesis replay
  opens a fresh store. A fresh store activates the deposit upcaster
  (`vocab_upcast_deposit.go`, `fresh`), which rewrites
  `co-super-open:…` to `co-management-open:…` and re-derives the
  canonical id from `command_id`. That gives the same number of
  lifecycle commands and events under different keys, and assignment
  bodies that differ only in the digest-ref leaves. That matches the
  diagnostic exactly.
- The boot fast path (`!replayed && FencedAt != ""`) skips the rescan on
  the claim that "a fenced store … cannot contain non-V2 rows". That claim
  is false for IDs, because the guard never checks them.

**Heresy (discovered):** the serving fence and the migration disagree
about what V2 means. The guard accepts any ID spelling. The migration and
upcast rewrite every leaf. So a live store is not a fixed point of its own
migration, and any replay that upcasts diverges from it.

The same flaw would break restore. `rematerialize.go` says the staged
store "was reconstructed from the V1 tape byte-identically" and compares
its witness with the checkpoint's. With no advertised base it replays
from genesis in a fresh store, which upcasts, so the witness would not
match.

Fix options:

- (a) Writers mint IDs that are fixed points of the frozen rule. This is
  correct by construction for new events and needs an inventory of
  writers. Digest refs that are recomputed and compared must accept the
  legacy spelling.
- (b) Probe and restore replay in the live store's deposit mode. A
  live store without an upcast ledger keeps its own bytes, so the staged
  replay must keep them too. That matches rematerialize's stated design.
  Its weak spot: a store with mixed spellings after a boot rescan.
- (c) Extend the write guard to refuse leaves that are not fixed points.
  This enforces (a) but would fail live writes for any writer left out of
  the inventory, so it can only come after (a).

Decision (conservative, per no-blocking-asks): (a) for the engineering
assignment path, with a test guard that every deposit the path writes is
an upcast fixed point. Then (b) only if the rerun still shows spelling
divergence from other writers. (c) is a named residual,
`vocab-guard-ids`.

## Fix (a): engineering writers mint serving-vocabulary ids

Red ceremony (checkpoint/route projection replay; engineering lifecycle).

- Conjecture delta: a live store is replayable only if every leaf its
  writers mint is a fixed point of the frozen vocabulary rule. The engineering
  path broke that with thirteen `co-super-*` spellings. With them minted
  as `engineering-*`, the from-genesis upcast is a no-op for new deposits.
  Probe and restore then match live byte for byte.
- Change: lifecycle command ids `engineering-{open,open-failed,bind,capsule,
  cancel,system-cancel,restart-open-cancel,restart-cancel,report,orphan}:`.
  Digest refs are `engineering-{grant,execution,fate}:sha256:`.
  Compatibility:
  - Validation accepts the same digest under the legacy `co-super-` spelling.
  - Report replay uses the report's recorded command id.
  - The legacy report fallback keeps `co-super-report:`.
  - The `choir:co-super-report:v1` hash domain is unchanged, so report ids
    stay stable.
- Tests first:
  - `requireServingVocabularyDeposits` scans every object-graph leaf after
    five real minting flows (boot sweep, deadline cancel, restart recast,
    stranded proposal resume, revoked resume). It failed on exactly the
    production spellings before the change.
  - Store tests pin legacy digest refs and legacy report replay.
  - The assignment seed fixture now uses `engineering-` ids so it cannot
    mask a writer.
- Protected surfaces: engineering assignment lifecycle commands, attestation
  and fate validation, and replay equivalence.
- Admissible evidence: an M11 rerun whose apply checkpoint reports
  replay-equivalent, and no `co-super-*` leaf in the replay diagnostic.
- Rollback: git revert. Records minted as `engineering-*` stay valid only
  under this code, so a revert after new assignments would refuse their
  refs. Revert only on a fresh QA computer.
- Heresy delta: discovered "serving fence accepts V1-spelled ids";
  repaired for the engineering path only on staging proof; introduced none.
- Named residuals:
  - `vocab-guard-ids`: the write guard still checks role fields only.
  - `vocab-recovery-prefix`: `persistent-super-recovery:v2:` is still
    minted. It is stored as update content and parsed by prefix, so a
    replayed store would not recognize it. Renaming it changes the
    recovery occurrence identity, which is a separate red change.
  - `upcast-legacy-refs`: on a store that was already upcast, legacy refs
    were hashed over V1 payloads, so their digests no longer validate.

## Rerun 9 on 77406673: engineering replays exactly; recorded content still diverges (04:36Z)

Disposable `computer-3af55828…` (VM `vm-fc88c237…`), operation
`selfdev-6a5317f4…`. Implementation reported at 04:17Z, after a longer
build than rerun 8. Verification and approval followed, and the apply
restart came at 04:32:14Z. The hold logged "waits for the
self-development apply checkpoint" again.

Replay-completeness at 04:34Z is still ineligible, but the diff shrank
from `live_only=26 replay_only=26 changed=3` to
`live_only=2 replay_only=2 changed=0`, all `choir.event`. The
engineering lifecycle now replays exactly (f9531177 confirmed on
staging).

The two live-only events were written at 03:56:19 and 04:06:49, each an
ordinary tool-loop iteration (8 and 70). `choir.event` uses
content-hash identity, and its body is the recorded tool call or result.
**Finding (hypothesis until the payload is read):** a whitespace-free
string the model sent or received, such as a search pattern or path
containing `-super-`, is upcast on replay. That changes the event's
content hash and so its key. Rerun 8 had the same 2/2 pair.

This is model-authored content. No writer can make it a fixed point,
so fix (a) cannot close it. The frozen leaf rule rewrites history the
model actually produced. **Decision:** add fix (b). Probe and restore
replay in the live store's deposit mode: a live store without an upcast
ledger never upcast its deposits, so the staged replay keeps their bytes.
That is what `rematerialize.go` already assumes ("reconstructed from
the V1 tape byte-identically", then `MigrateAndFenceServingVocabulary`).
Fix (a) stays: after that migration, ids the runtime recomputes must
already be fixed points, or lookups miss.
