# SMG probe on a fresh disposable: Texture desk burns the full token budget per run

Written 2026-10-06 during SA slice 1 (wake-outbox storm fix). Mutation class:
docs-only record; the defect is orange/red (model-loop behavior + trace
evidence). Distinct from `sa1-wake-outbox-rearm-storm-2026-10-06.md` — this is
not a re-arm storm; there are no persistent obligations on a fresh disposable
to re-arm. Do not merge it into the storm record.

## Symptom (staging, disposable computer `computer-423134e8`, vm `vm-d79a1916`)

SMG acceptance probe `scripts/smg_rlm_acceptance_probe.mjs` legs 1–3 against
the fresh disposable (keydriver-provisioned, genesis minted at 12:33Z):

- `prompt_bar_submit` succeeded: trajectory `b3447097`, doc `d37849d1`.
- The probe then watched trajectory events for the three activation legs
  (unbound-report refusal, assignment cancel, bound report). Timeout 30 min.
- **All three legs failed**: no bound producer report, no cancel verb
  exercise, no unbound refusal surfaced in a report packet, no Management
  signal. Probe verdict `ok=false`, "no bound producer report observed".
  Evidence: `docs/evidence/smg-rlm-acceptance-disposable-2026-10-06.json`.

## Guest console evidence (`vm-d79a1916…/console.log`)

The Texture desk — not Management — is where the work died:

- `texture:d640dd27` run `815a436c` — 22+ tool-loop iterations,
  `failed: tool loop budget exhausted: total tokens 1250479 > max 1200000`
  at 12:03:25Z.
- `texture:d640dd27` run `d7c64d4d` — same, 1225654 tokens, failed
  12:33:02Z; the trigger disposed `durably invalid occurrence: Texture
  activation terminated without disposing exact trigger`.
- `texture:d37849d1` (the SMG prompt's own doc) run `f2823dd6` — 1251222
  tokens, failed 12:40:21Z; same durable-invalid disposal.

Each Texture activation iterated the tool loop 22–36+ times executing 1–3
tools per iteration and accumulated the entire 1.2M-token budget before
failing. Whatever the loop was reading/calling grew the transcript to the
cap on every attempt — then the occurrence poisoned, so the desk's next
trigger could not cleanly fire either.

## Root-cause hypotheses (unconfirmed — no trace read yet)

1. **Unbounded context growth in the Texture tool loop.** A per-iteration
   cost of ~50k tokens × 22-36 iterations = the whole budget. Candidates:
   mailbox backlog injected every iteration, repeated doc/context re-reads,
   or a tool returning the full trajectory history each call.
2. **A retry loop at the tool level.** Each iteration executing exactly 1
   tool then "continuing" is consistent with a single tool that keeps
   being re-issued (e.g. a read that never resolves, or a write rejected
   with a retryable error the model keeps re-attempting).
3. **The 1.2M budget is the wrong shape for the desk role.** If the texture
   desk legitimately needs more context for a supervision pass, the budget
   itself is the defect — but the rate (50k+/iteration) argues leak, not
   load.

This needs a trace read (`run_events` for run `815a436c`/`f2823dd6` on the
disposable, or any owner's texture run) before fix selection. The probe's
`docs/evidence/` JSON is the acceptance record of the failure, not the
mechanism.

## Why this is a separate record

The owner guest's storm (`sa1-wake-outbox-rearm-storm`) could not have caused
this: the disposable has ~zero history, the migration minted nothing
there (console grep finds no `wake outbox` lines), and the failing surface
is Texture's tool loop, not the wake drain. The SA slice-1 storm fix
(d94ce9ef) does not touch this path.

## Impact on the SMG station boundary

SMG close requires legs 1–3 on a fresh disposable. Both failures observed so
far are different defect classes:

- Owner computer: the wake-outbox storm starves Management (fixed by
  d94ce9ef pending deployed acceptance).
- Fresh disposable: Texture budget exhaustion kills the producer report
  path before Management ever sees a bound report.

The SMG station cannot close on the disposable evidence until the texture
defect is diagnosed and fixed or ruled in scope for a different station.
