# Engineer spins no-op cells for many minutes without freezing (2026-11-04)

Mutation class of this doc: green. The fix (orange; rollback: revert) lands
in the next commit and references this record.

## Evidence

Live8, crouton test arm `computer-848b21d55df1dd1a953ae798b8edc696`,
trajectory `4a80ca21-871a-5680-88f9-9b3c4d2ac220`, engineering run
`run:assignment-9b225b8fa74b99ebffeed9ab`, engineer on openrouter
`stealth/crouton-alpha`. Read with `choir trace export` (run_events layer,
771 events for this run), read-only.

- 04:43:35 to 05:10:05: real work. Probes, shots, contrast fixes, check-packages,
  screen cleanup, a toolchain check (`go version; node --version`).
- From 05:10:15 every cell is the same `capsule_go_eval`:
  `old := "x"; _ = old; _ = choir.Context(); println("advance-aN")`, N = 40 at
  iteration 187, 2.5 s per cell, no job, no write, no Freeze. The trace
  holds 236 cells of this shape in total (the name `advance` first appears at
  04:57:13, numbered from `advance2` at 04:57:32).
- Iteration 186 to 384 in 8 minutes (05:10:05 to 05:18:22) at 123k to 130k
  input tokens each, `cached_input_tokens` 0 on every call (the route
  reports no cache).
- It ended only when an owner note ("The build, the shots and the checks are
  done; call choir.Freeze now ... Stop no-op cells.") was relayed at 05:18:21;
  the engineer then called choir.Freeze and Complete in one cell at 05:18:22.
- No platform wait was involved: the last `loop.retry` (`capsule_job_ended`)
  is 05:09:56, the last job ended before 05:10:05, and no `capsule_job_park`
  event follows.

## What the trace shows about why

Findings (read from the ledger):

1. The spin cells are not waiting on a job. No job runs after 05:09:56, no
   cell polls one (`jobs_awaited` and `job_park` are absent from every spin
   result), and no `capsule_job_park` event exists after 05:09:56.
2. Nothing in the harness told it to "advance". The word is the model's own:
   its text at 04:57:45 calls two earlier no-op cells "placeholder advance
   jobs". No harness line in the run carries the word.
3. `choir.Context()` is called and discarded (`_ =`); the cell prints only its
   own counter, so the model never reads what Context says.
4. The engineering desk has no `Wait` verb (deskmanifest: WriteFile, Exec,
   Start, Poll, Stop, Complete, Freeze, Precommit, Report, Ask, Reply, Note).
   The only ways to pass time are a Poll that parks on a running job, ending
   the turn, or a cell.
5. The loop has no check on cells that change nothing. The existing guards
   (completion guard, job park, `maxCompletionGuardWaits` 128) bound waits and
   narrative endings, not a stream of tool calls.

Hypothesis (not confirmed by the trace; the model's reasoning is not in the
ledger): the model uses a no-op cell as a tick, believing a cell lets
something "interleave" or land. Its own texts at 05:01 to 05:07 say
"the interleaved writer/reader share the capsule" and "my pending edit was
already applied by the time the reader returned", a mental model in which the
next cell flushes pending work. At 05:10:05 its plan was "record the
toolchain, Freeze, and Complete"; the toolchain cell ran, and instead of the
Freeze it ticked. Whether it lacked a receipt it believed Freeze needed, or
waited on a flush that never comes, the trace does not say.

## Failure modes of a detector

- False positive on real polling: a bare `choir.Poll(id)` loop is a no-op by
  source and by effect but it parks the turn on the job. Cells that return
  `job_park` or `jobs_awaited` must reset the count.
- False positive on repeated real work: the same `choir.Exec` retried after
  a fix (identical source, different file state). Only cells that stage no
  intent, run no exec and write no file are no-ops; an Exec cell's receipt
  differs from a bare print, so the detector compares source modulo numbers
  and string literals AND requires no staged intents, no job, no fate, no
  error.
- Counter never resets: any differing cell (different normalized source) or
  any cell with an effect resets it.
- Nudge repeated every cell: the reminder fires once at K, not at each cell
  after it.
- Unbounded after the nudge: at 2K the run must end with a visible fate, not
  continue.
- Ending a frozen computer: a Freeze is a terminal fate and ends the loop
  before any further cell, so a frozen candidate cannot spin here.
- Management and Texture cannot see it: the run's error becomes the run's
  fate; the assignment fate joins from the failed run.

## Plan

K = 6 identical no-effect cells: one harness line names the count and the
next safe acts (Freeze, Poll a job, Report/Ask, or change something). 2K: end
the run with `engineer spun N no-op cells`.
