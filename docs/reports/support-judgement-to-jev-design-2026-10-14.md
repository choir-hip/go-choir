# Support Judgement Leaves Management: Jev Judges On Commit

Date: 2026-10-14. Problem:
[management-serial-turn-blocks-paper-2026-10-14](../problems/management-serial-turn-blocks-paper-2026-10-14.md)
(one management turn spent 38 minutes hand-checking a Texture's
citations; no story moved meanwhile). Owner direction, 03:55 UTC: "we
can't do serial … source checking can probably be done by jev." This
note covers the Jev half only; management delegating to sub-RLMs is a
separate change.

Mutation class: **red** (Texture evidence surfaces: citation check and
verdict memos, records to `texture:<doc>`, the management desk's module
set and prompts, Jev calls through the gateway).

## Today (64278ca6)

1. A Texture revision commit (`ApplyTextureTurn`) writes a
   `choir.citation_check` memo per new hash pair and, in the same
   transaction, one system `support_pairs` record to `management:<owner>`
   that wakes management.
2. Management's cell calls `choir.Judge(r.Support.Pairs)` (host:
   `judgeSupportEgress`), which asks Jev in one call, or passes its own
   `{key, verdict, why}` maps. The prompt offers both.
3. The cell's records commit (`CommitInboxRecords` with `Judge`,
   `Verdicts`) writes a `choir.support_verdict` memo per pair and, per
   document with failures, one record from `management:<owner>` to
   `texture:<doc>`, which the Texture's frame shows as unsupported
   citations to correct with Write. A pass writes only its memo.

## Change

1. **The commit lists, the runtime judges.** `ApplyTextureTurn` keeps
   writing the check memos atomically with the revision, but no longer
   mints a `support_pairs` record. After the commit returns, the Texture
   commit path hands the revision's citation pairs to
   `Runtime.JudgeRevisionSupport`, which returns at once. A background
   job, bounded by a runtime-wide semaphore like intake labels
   (`supportJudgeConcurrency`), judges every pair of that revision that
   has a check memo and no verdict, with the existing judging code: the
   memo first, the committed sentence from the check memo, the captured
   block re-read and hash-checked, the remaining pairs to Jev in one
   call. A pair already in flight is skipped.
2. **Verdicts are a system act.** The job records verdicts through the
   same store command as today (`CommitInboxRecords` with `Verdicts`),
   with `Judge = system:support` instead of `management:<owner>`. The
   verdict memos are the same objects; a failure is one record per
   document to `texture:<doc>` from `system:support` (emitter kind
   system), which wakes the Texture. A pass writes only its memo. The
   store accepts verdicts only from `system:support` now. Published
   verdict overlays are fed the same way (`publishStoryVerdicts`).
3. **Management loses support judgement.** `choir.Judge` is deleted from
   the kernel (scope method, broker action, tray intent, surface entry,
   module sets) and from the reducer; the prompt paragraph, the
   overlay's Judge bullet and the records-wake sentence go. Management
   never receives a new `support_pairs` record.
4. **Legacy records.** A `support_pairs` record already in management's
   inbox is shown to management closed: kind `support_pairs`, no pairs,
   body "Closed: Jev judges citation support off your turn now; this
   record needs no answer." Its pairs are handed to the new path when a
   live management turn reads it (once per record per process), so they
   are judged and the record still advances the cursor normally. It
   never wedges management (there is nothing to answer and no Judge to
   call).
5. **Cost.** A successful Jev call appends a run-progress event to the
   committing Texture run (`phase: jev_judgment`, `llm_model`,
   `input_tokens`, `output_tokens` from the answer's `usage`, agent
   `system:support`), and `llmcost.ExtractCostEntries` reads that phase
   like `tool_loop`. Support judgements then appear in `/api/costs`
   (priced once the Jev rate is in the table; tokens count either way).
   Intake-label calls stay unrecorded (no run to attach to); named
   residual.

## Restart rule

The job is in-memory and holds no durable obligation: no wake row, no
work item, no record. A crash kills it; nothing resumes it at boot, and
there is nothing to close. The pairs it held stay **pending** (a check
memo without a verdict), the same visible state a pair has between commit
and verdict. A later **live** Texture commit whose citations still
include a pending pair judges it, because that commit is new work that
cites it; a pair the Texture edited away is never judged (its sentence is
gone). There is **no boot-time sweep**: judging pending pairs at boot
would be resuming work after a crash.

## Failure modes, written first

1. The commit waits for Jev (Jev latency or a gateway outage stalls the
   Texture). Guard: the hand-off returns before any store read or Jev
   call; tested with a Jev that blocks until released.
2. A burst of commits starts unbounded goroutines or Jev calls. Guard:
   one semaphore for the runtime; a job that cannot get a slot within the
   queue wait leaves its pairs pending.
3. Two commits judge the same pair concurrently and call Jev twice or
   fail the Texture twice. Guard: an in-flight set by pair key; the
   verdict memo makes a second record a no-op (store refuses nothing,
   records nothing).
4. Jev judges text other than the committed sentence and the captured
   block. Guard: unchanged code path (check memo + capture re-read with
   hash check).
5. A failure does not reach the Texture, or reaches it without waking it
   (system records do not wake, D5). Guard: `inboxRecordWakes` wakes a
   system record carrying support failures; tested through the Texture
   commit end to end.
6. A pass sends the Texture a record or mints a revision. Guard and test.
7. Management still receives `support_pairs` records or still sees Judge
   in its surface or prompt. Guard: tests on the commit's records, the
   module sets and the prompts.
8. A legacy `support_pairs` record wedges management (nothing to call,
   cursor never advances) or is silently dropped. Guard: shown closed with
   a body; its pairs judged.
9. One bad pair (cited block gone or changed, Jev omits its answer) blocks
   the verdicts of the others. Guard: such a pair is left pending and
   logged; the rest are recorded.
10. A crash mid-judgement resumes at boot. Guard: no durable obligation,
    no boot sweep.
11. Verdicts from a desk (a forged management verdict) still commit.
    Guard: the store accepts only `system:support`.
12. The cost event lands with no tokens or breaks the run's trace.
    Guard: emitted only with tokens; a distinct phase so it is not read as
    a model iteration of the Texture run.

## Ceremony

- **Conjecture delta.** Before: support judgement needed management's
  reasoning and serial turn. After: judging a listed pair is a typed
  Jev question with no desk judgement; management's turn is for
  dispositions. Falsified if Jev's verdicts are worse than management's
  (watch the failure-to-correction ratio on Texture's support failures).
- **Protected surfaces.** `internal/store/support.go`, `inbox.go`,
  `texture_turn.go` (records minted by a Texture commit, verdict
  authority, wake rule); `internal/agentcore/support_judge.go`,
  `inbox_records.go`, `rlm_reduce.go`, `tools_desk.go`;
  `internal/yaegikernel` module sets; management prompts;
  `internal/textureowner/texture_cell_author.go` (post-commit hand-off).
- **Admissible evidence.** Local store-backed tests through the real
  Texture commit and a fake Jev; staging acceptance is a deployed paper
  revision whose pairs gain verdicts with no management run reading
  `support_pairs` (not claimed here: the branch is not deployed).
- **Rollback.** Revert the commits. Pending pairs written while this was
  live are judged by nothing after a revert (the old code lists only new
  pairs); acceptable, they stay pending.
- **Heresy delta.** Discovered: a desk performed a typed judgement a
  decision model was already wired to do, inside the one serial turn
  every paper action needs. Introduced: none intended; the system act
  writes records from `system:support`, a non-desk sender (as capture
  arrivals come from `source:<id>`). Repaired: support judgement off
  management's turn; the "judge yourself" path deleted.
