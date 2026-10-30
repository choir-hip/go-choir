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

## Update 2026-10-06 (owner-guest recurrence — budget bound is time, not
tokens)

Reproduced on the owner guest post-storm-fix (`candidate-fleet-e15cb89f`,
SMG probe `smg-rlm-residue`, trajectory `ad443814`): three texture runs
(`be496a66` texture:0f181fa1, `265b9494` texture:f947cef4, `9de8e633`
texture:760b5adb) each exhausted the **45-minute elapsed** budget —
`tool loop budget exhausted: elapsed time reached max 45m0s` — not the
token cap. Iteration cadence ~20-30 s/tool with 1 tool per iteration is
consistent with transient-provider-error retries (15 s sleeps) stacking
on slow gateway calls, or a model that keeps issuing one small tool call
per turn without converging on `controls[]` emission. Either way the
desk never emits `open_persistent_super`, so Management is never opened
and the SMG legs cannot pass — the storm fix removed the wake backlog
entirely yet the legs still fail on this wall.

**The texture tool loop is the current SMG blocker on both substrates**
(fresh disposable and storm-clean owner guest). It is orthogonal to the
SA slice-1 storm work and needs a dedicated slice: either trace why
`desk_go_eval`/`ApplyTexture` turns iterate 20-40× without emitting the
control, or bound the desk to a smaller wall/token envelope sized for
its actual supervision surface.

## Impact on the SMG station boundary

SMG close requires legs 1–3 on a fresh disposable. Both failures observed so
far are different defect classes:

- Owner computer: the wake-outbox storm starves Management (fixed by
  d94ce9ef pending deployed acceptance).
- Fresh disposable: Texture budget exhaustion kills the producer report
  path before Management ever sees a bound report.

The SMG station cannot close on the disposable evidence until the texture
defect is diagnosed and fixed or ruled in scope for a different station.
