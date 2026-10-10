# Rerun 12: the change applied, but restore is refused and Texture tells the owner nothing changed (2026-10-10)

Status: open. Gate 2. Problem first; no fix in this commit.
Mutation class of the fixes: red (restore/rematerialize, checkpoint
replay, actor delivery of Texture emissions).
Evidence class: staging, disposable QA computer
`computer-75c31e98bdbcd8d23116922467d2329a` (VM
`vm-ce25f113b83df722248562a46b12c504`), build 19ba3034, probe receipt
`docs/evidence/m11-rerun-2026-10-10T13-21-08Z.json`. Host-side console
of a disposable computer only.

## How far it got: 14 of 17 checks

First run on the new desk prompts (79edc337) and the Phase 0 vmctl.
Primary operation `selfdev-b3c271e2…` started 13:21, reached
`awaiting_approval` 13:44, applied 13:45 (one materializer retry at
13:44:47: `route ledger: authorization evidence creation time is
required`, then it passed). Candidate B (`selfdev-68559150…`) was
rejected at 14:09. No vmctl kill touched the computer. Three checks
failed.

## Failure 1: the post-apply checkpoint replays a moving chain

`POST …/self-development/checkpoint` after apply returned 409:
`replay completeness: reconstruct event chain: computer event projection
repair required: local=seq=1860 … platform=seq=1897`.

Cause (code reading, `computerevent/appender.go` `ReconstructInto`):
replay pages to the end of the tape and then requires the platform head
to equal the replayed head. Thirty-seven events landed during the
replay. The apply hold (7ea0f66d) releases desk work when the operation
leaves `materializing`, so this checkpoint runs on a busy computer. This
is the same class as
[selfdev-apply-checkpoint-starved-by-resumed-work](selfdev-apply-checkpoint-starved-by-resumed-work-2026-10-10.md),
fix direction B: replay to a head captured at the start and compare
there. Fix A (hold work) cannot cover a checkpoint taken at an arbitrary
later moment, and every second change on a layered computer needs one.

## Failure 2: restore cannot reach a head older than the newest replay base

Restore to the pre-episode checkpoint was refused: `projection base
refused: target is not a descendant of watermark 2542`.

Cause (code reading):

- checkpointd builds replay bases for a computer as it runs and advances
  its one advertised watermark (`cmd/checkpointd/worker.go`,
  `PublishCheckpointResult`). Here it reached 2542, after the pinned
  head.
- Restore resolves its target only against that newest watermark
  (`agentcore/restore_base.go` `resolveRecoveryTarget` →
  `projectionbase.ResolveTargetSequence`), which pages forward from W. A
  target older than W never appears, so it is refused. The genesis
  fallback in `rematerialize.go` runs only when no base was ever
  advertised.
- So a rollback to any checkpoint older than the newest base is
  impossible by construction. That is every rollback after the computer
  has run a while: exactly Gate 2's rollback.
- Older bases are recorded (`computer_projection_bases`), but base GC
  keeps only the newest two plus pinned ones (`projectionBasePins`).
  A pin route exists (`/internal/computers/projection-base/pins`) and
  nothing in the tree calls it. A checkpoint does not record which base
  it rests on.

Fix shape (to decide; red):

- (a) restore picks the newest retained base at or before its target,
  and falls back to bounded genesis replay (≤ `MaxRecoveryTailEvents`,
  10000) when the target precedes every retained base. Enough for
  disposable QA computers.
- (b) a checkpoint mint pins the base under its head (reference = the
  checkpoint digest), so a long-lived computer such as the owner's can
  always restore to it. Needed before restore is routine there.

## Failure 3: Texture's document says no change was made, after it was applied

The owner-facing document's last revision (13:46:42, a minute after
apply) says: "No self-development change has been applied, frozen, or
verified … the execution assignment … ended in cancellation: a restart
revoked the assignment's capsule before it returned a result", and that
"the owner's persistent Management execution path … is not currently
available here". The operation was frozen, verified, approved and
applied. This is the Gate 2 exit's central claim (live Texture
supervision) failing: the supervision surface is false.

Trace (guest console of the disposable computer):

- 13:44:22 the engineering run that froze the candidate
  (`run:assignment-e37fe78f…`) was still running at the apply restart:
  `could not join assignment fate … connector is closed`. Boot
  passivated it and cancelled its assignment (the rerun 7 pattern: the
  producing run keeps iterating after its result).
- The cancel report woke Texture. Its turn waited for the apply
  checkpoint (fix A working), then at 13:46:42 wrote the revision from
  that cancel report.
- Both Texture turns in the episode (13:41:47 and 13:46:42) logged
  `deferred … discards 15 (13) emitted update(s)` and `Texture
  activation returned without disposing exact trigger`, then on retry
  `consumed without a turn: terminal (… head already consumed by a
  Texture turn)`.

Findings and hypotheses:

- **Finding (code, `actor/dispatcher.go`):** a deferred event's
  emissions are dropped on the promise that "the event re-fires and the
  handler re-emits them". For Texture the re-fire is consumed without a
  turn, because the turn already consumed the document head. So
  whatever Texture emitted in those turns is lost for good. Which
  updates those were (dispatches, controls, wakes) is not yet read.
- **Hypothesis H1:** Texture renders from desk reports and never sees
  the operation's own state (`frozen`, `verified`, `applied`), so a
  cancel report of the producing run reads to it as "nothing happened".
- **Hypothesis H2:** "Management path not available" is Texture's
  reading of a dispatch whose effect it never saw, because the emission
  was discarded.

Next probe: read the two turns' run events and the discarded update ids
on this disposable computer (internal run-events read) before any fix.

## Probe bug (fixed with this record)

The M11 probe's `mark()` sets any leg it is named after. Its progress
mark `post_apply_checkpoint` set that leg true on a 409. The receipt is
corrected (false, with a note) and the probe now marks
`post_apply_checkpoint_failed` unless a checkpoint was minted.

## Order of work

1. Restore (failure 2, fix a): smallest, and it blocks the rollback leg.
2. Texture emissions (failure 3 finding): a substrate loss, not prompt
   tuning. Read the trace first.
3. Post-apply checkpoint (failure 1, direction B).
4. Rerun 13, then the trace-review and prompt-tuning loop.

## Fix (a) for failure 2: restore replays from genesis below the newest base

Red ceremony (restore/rematerialize).

- Conjecture delta: the tape, not the newest base, decides whether a
  restore target is reachable. A target older than the base is replayed
  from genesis, within the same 10000-event bound the fresh-computer path
  already uses.
- Change: `planRestoreTarget` (`agentcore/restore_base.go`) replaces the
  branch in `RematerializeFromTape`. If the base path refuses, it resolves
  the target from genesis. It replays from genesis only when no base was
  advertised, or when the target's sequence is below the base's. A
  target at or past the base keeps the base's refusal, so a stale or
  corrupt base is never bypassed. A target off the chain, or past the
  bound, refuses.
- Tests first: `TestPlanRestoreTargetBeforeNewestBase` (older than the
  base, past it, off the chain, a base that refused a target it covers,
  past the bound, absent base, watermark outage). The genesis replay
  itself is the path the existing "absent watermark" test exercises.
- Protected surfaces: restore target resolution. The base install path,
  the tail receipt and the witness check are unchanged.
- Admissible evidence: rerun 13's restore leg returns 200 with the
  witness matched, on a computer whose watermark passed the pinned head.
- Rollback: git revert; no persistent state changes.
- Heresy delta: discovered "rollback impossible below the newest base";
  repaired for computers under 10000 events, on staging proof only.
  Residual `restore-long-lived`: the owner's computer is past the bound,
  so it needs fix (b), a checkpoint that pins its base.

## Failure 3 trace read (15:00Z): a read-only cell consumed the engineering result

Read-only reads on the disposable computer: the two Texture runs' events
(`022cc498…`, `498dbc0e…`) and the trajectory's lifecycle log.

- 13:39:14 engineering's implementation assignment (`7b33b8ab…`) froze its
  candidate and reported `execution_result`. Delivery bound it to Texture
  run `022cc498…`, whose wake message says the payload is "already bound
  inside your cell as choir.Updates()".
- 13:39:17 the model's first cell failed to compile. 13:39:20 its second
  cell ran and printed nothing. At 13:39:20.93 the lifecycle log shows
  `texture_turn_committed` with reason **"desk cell completed with no
  authoring act; consuming the owner revision"**, then `update_applied`
  for the engineering report. From 13:39:23 every `choir.Updates()`
  returned 0. The model searched for the report (Help, Context, Pack, even
  /tmp files), never found it, and at 13:41:47 wrote a document with no
  result in it.
- 13:45:26 the verifier's cancel report woke run `498dbc0e…`. Its first
  cell printed the report at 13:45:31, and the same auto-commit consumed it
  at 13:45:31.41. Later cells saw nothing. The model's disposition of the
  report was refused ("the snapshot has no update with this id"). Its four
  attempts to send an execution request were refused on schema and target
  (`apply_change` unsupported, safety fields required, target not found).
  It then wrote "blocked … Management path not available".

**Finding:** `consumeIdleTextureTrigger` (`agentcore/tools_desk.go`,
a08defc0, defect #5) runs after **every** Texture cell that stages no
`texture_apply`, not once per activation. Its decide turn uses the
consume-at-commit default (`textureTurnPendingInbound`), which marks every
pending producer report delivered. So the model's first read-only cell
consumes the reports the activation exists to handle. This confirms H1 as
a substrate defect, not a prompt or model failure.

It also explains the discarded emissions. A producer-report occurrence is
handled only when the report's disposition names the run's own revision
(`TextureActorOccurrencePostcondition`). The early decide named the base
revision, so every such activation deferred and its emissions were
dropped. H2 is refuted as stated: "Management path not available" was the
model's reading of its own refused controls.

Fix shape (red; Texture canonical writes):

- A cell only records what it staged.
- When a Texture activation ends with completed cells and no
  `texture_apply`, the runtime commits the idle decide once, in
  `ExecuteActivationSyncChecked`. That keeps the defect #5 guarantee (an
  untouched owner revision is still answered), and reports stay visible to
  every cell of the activation.

Prompt-side residuals from the same trace, for the tuning loop:
`texture-control-schema-friction` (four refused control attempts) and
`texture-truncated-update-id`.

## Fix for failure 3: the activation, not the cell, answers an idle trigger

Red ceremony (Texture canonical writes, actor occurrence disposal).

- Conjecture delta: a pending report belongs to the whole activation. Only
  the model's own turn, or the activation's end, may consume it.
- Change: `noteTextureCell` records each completed Texture cell's staged
  intents. `answerIdleTextureTrigger` runs once at the end of
  `ExecuteActivationSyncChecked`. It commits the decide turn
  (`no_worker_needed`, or `delegation_skipped` after a staged act) only
  when the activation completed at least one cell, applied nothing, and
  did not fail or cancel. An activation retried as unprocessed drops its
  record.
- Tests first (`texture_idle_trigger_test.go`): a read-only cell commits
  nothing; the end answers once and states what happened; an applied
  activation, a cell-less one, a failed one, and a non-Texture one are not
  answered. The existing desk, textureowner, actorruntime and actor suites
  pass.
- Protected surfaces: when a Texture decide turn is committed. The turn
  itself, its consume-at-commit default, and the postcondition are
  unchanged.
- Admissible evidence: rerun 13's Texture document names the frozen,
  verified candidate, and the console shows no "discards … emitted
  update(s)" for Texture.
- Rollback: git revert.
- Heresy delta: discovered "a read-only cell consumes the activation's
  inputs"; repaired on staging proof only; introduced none.

## Failure 1 disposition: measured with retries; product fix is a named residual

The owner's own apply checkpoint is taken inside the apply hold and
passed in rerun 12. Only the probe's extra post-apply checkpoint, taken
while the desks resume, replays a moving chain. The probe now retries it
once a minute for up to ten minutes and records every attempt.
Residual `checkpoint-at-captured-head`: a checkpoint should replay to a
head captured at its start and compare live state at that head, so a busy
computer can mint one. That needs a live-state snapshot at the head, a
red change to the replay-completeness verifier, and it is scheduled after
rerun 13.

## Open (H3): operation outcomes never reach Texture

Code reading: no path delivers a self-development operation's transitions
(verified, awaiting approval, approved, applied, rejected, rolled back) to
the trajectory's Texture agent. Texture hears only desk reports. With the
idle fix it should see engineering's freeze result and the verifier's
report, but it still cannot tell the owner that the change was approved,
applied or taken back. Rerun 13's trace decides the shape. Either each
transition becomes a ledger report that wakes Texture, or Texture's cell
can read the operation's state. The first is live supervision; the second
is only a read.
