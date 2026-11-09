# The paper's runs fail together when every model is out of quota (2026-11-09)

Status: documented, fix pending. Mission: docs/definitions/choir-autopaper-improvement-2026-11-09.md (S1).

## Evidence

Read only, from the paper's /api/runtime/observability at build 1f2493123,
2026-11-09 16:18-16:28Z (two samples, about eight minutes of recent_log; copies
in the session scratchpad audit folder).

- The management queue held 1763 queued intake tasks, the oldest 253,321 s
  (70 h), growing about 11 an hour (1618 at 03:10Z). Four task slots, newest
  first.
- At 16:18:22-16:18:40 texture, research, management and management-task runs
  logged together "every model is out of quota: waiting for the OpenRouter quota
  to reset (about 2-6 min)". Six failed within three seconds at 16:24:00-02:
  "tool loop iteration 25 (24-29, and 65 for management): every model is out of
  quota after 8 retry rounds over 5m28s-5m36s". The Synthetic hold ended
  14 s after one of them gave up. The rest cleared at 16:24:22-16:26:10 when
  "a model answered again"; incidents lasted six to eight minutes.
- About a third of the 33 runs with model calls in the window entered an
  incident. Two management-task runs that slept through one were then
  cancelled: "activation budget exceeded: progress deadline reached".
- The dispatcher logged "activation wait ... slots=12/12 due_agents=40-48" with
  waits of one to five minutes, ending when the incidents ended.
- 227 model calls from 33 runs in about eight minutes; crouton-alpha served
  about 80 percent, Synthetic the rest. One tool per loop iteration.
  run_memory_agent_list returned 20,510 rows at each management desk start.

## Reading of the code (hypotheses until a trace confirms)

1. The incident is bounded by retry rounds, not by elapsed time. The wait is
   clamped to the soonest ledger reset, and each clamped wait spends a round,
   so short clamps burn the eight rounds in about five and a half minutes
   (internal/toolregistry/toolloop.go:539, :1089, :1103).
2. A failed run loses its loop state; the round closes without a Report and
   the caster is woken, so the work is redone from the start
   (inbox_records.go:1391).
3. A run sleeping in an incident keeps its dispatcher slot and its progress
   deadline keeps counting, so twelve sleeping runs stall every other desk and
   some are then cancelled.
4. All runs retry in lockstep against one per-computer quota ledger and the
   gateway's per-model circuit.
5. The ledger's holds are not visible in observability, so which members were
   held, and why, is not on any trace.

## Why it matters

Throughput follows model availability, not the four task slots: the queue
cannot drain while a third of runs sleep, fail and redo their work. This is a
code problem (owner: too many model calls is a code problem to fix, not a limit
to impose), not a reason to add a cap.

## Fix order

First, the round accounting and the progress deadline (1, 3 in part): bound an
incident by elapsed time with a wait floor at the earliest known reset, and do
not count incident sleep against the progress deadline. Expose the ledger's
holds in observability (5). Then a design for parking an incident run off its
dispatcher slot (3) and keeping its loop state (2). Then calls per activation.
