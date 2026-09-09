# Restore-Zero post-completion consensus — 2026-09-09

Class: green (adjudication; no runtime mutation).
Authority: `docs/definitions/choir-rlm-restore-zero-2026-09-08.md`.
Panel: `.agentic-consensus/restore-zero-post-completion-20260909/` (gitignored transcripts).
Prompt: `.agentic-consensus/restore-zero-post-completion-review-prompt.md`.
HEAD at review: `6145b60c`. Snapshotting binary: `9341b5d1`.

This is **not** a second completion claim. It is the missing post-completion gate on the claim already recorded in `docs/evidence/choir-rlm-restore-zero-retained-boot-2026-09-09.md`. The 01:09 panel (`agentic-consensus-20260909-010928`) was the snapshotting **design** review, not this gate. The 00:41 panel that accepted W=1/W=13 remains demoted.

## Panel

Default convergent panel, 1200s, `--keep-going`. 13 routes attempted, 12 ok, 1 failed (`omp-hy3`: `401 Model hy3-free is not supported`).

| Agent | Lens | Verdict |
| --- | --- | --- |
| Devin | correctness | Accept-with-conditions |
| Codex | skeptic | Reject |
| Claude | deployed-proof | Accept-with-conditions |
| Cursor | test-gaps | Accept-with-conditions |
| OpenCode | rollback | Accept-with-conditions |
| OMP GPT-5.6 Sol | historian | Reject |
| OMP GPT-5.6 Luna | operator | Reject |
| OMP Gemini 3.8 Flash | security | Reject |
| OMP Cursor Grok 4.6 | maintainability | Accept-with-conditions |
| OMP Muse Spark | completion-gate | Accept-with-conditions |
| OMP Nemotron 3 Ultra | prefix-proof | Accept-with-conditions |
| OMP GLM 5.3 Flash | rebase-gap | Accept-with-conditions |
| OMP HY3 | residual-scope | failed to start |

Tally among completed routes: **8 Accept-with-conditions / 4 Reject / 0 Accept**.

## Orchestrator adjudication

**Accept-with-conditions.** Do not flip `now.status` back to `working`. Do not treat this panel as an unconditional Accept of the original nine-action deployed-proof list.

The skip heresy that demoted the first completion is closed on the retained owner computer: owner-scoped refresh booted `9341b5d1`, `PlanRecovery` chose **resume** at `local=W=H=148431 tail=0`, genesis/refuse/rebase did not fire, the 15.5 GiB disk was retained. That is the owner-set remaining-error for snapshotting.

The Reject camp is right about **coverage**, not about a live lifetime-replay path. Staging never observed `RecoveryRebase`, a nontrivial `(W,H]` rematerialize/restore, or `recover_current` under the new contract. Those are conditions, not a reopen of nonempty-store skip.

## Locally verified findings (not panel hearsay)

1. **The retained-boot receipt overclaimed observer proof.** `computer event appender: replay page fetch after=` (`internal/computerevent/appender.go:798-799`) logs only when a page takes **> 2s**. Absence of that line is not a fetch count. Boot `runReplayPhase` (`internal/autoputer/run.go:542-578`) installs **no** `ReplayObserver`; that observer exists on rematerialize/replay-completeness only. Prefix=0 on this boot is a **structural** claim: reconstruct seeds `after = localHead.Sequence` (`appender.go:725-727`); platform replay is strictly-after; at `after=148431` the page is empty and seq ≤ W cannot be reduced.

2. **The quoted reconstruct line is the wrong line.** `autoputer: computer event authority reconstructed` (`run.go:332`) is emitted when the appender is wired, **before** deferred replay. Completion is `autoputer: computer event authority reconstructed (replay complete)` (`run.go:578`). Guest `/health` `ready` is corroboration that `gate.setPending(false)` ran after `Reconstruct` returned, not the acceptance line.

3. **Resume is a legal `PlanRecovery` branch, and rebase was not applicable at these numbers.** `local ≥ W` and tail ≤ 10000 → `RecoveryResume` (`internal/projectionbase/recovery_plan.go:108-128`). `local=W=H` is that branch. The store reached 148431 under the *old* skip, then W was advertised at H. That is not a live lifetime replay on `9341b5d1`. It is also not a staging observation of `RebaseRetainedStore`.

4. **Resume compares sequences, not descriptor ancestry.** `PlanRecovery` does not bind blob digest / `vocabulary_version` / witness on the resume branch. At tail=0, `sameHead` never runs because no tail event is applied. Content-equivalence of the retained store to blob `6099cf69…` was not checked by this boot.

5. **Action 7 is unmet as written; Action 8 is discharged as a recorded blocked prerequisite.** `finish.acceptance` still asks for a staging rematerialize/restore + owner-scoped `recover_current` with a **nontrivial tail** and receipts. Action 8 says: if scoped failure-injection controls do not exist, record a blocked prerequisite — already done (`restore-zero-blocked-prerequisites-2026-09-09`). Do not convert Action 8 into a silent drop, and do not reject completion solely because it is blocked.

6. **Gemini’s ENOSPC residual is real for a future rebase, not for this boot.** `InstallVerifiedBase` downloads the full blob to `.base-download-*` then unpacks beside it (`internal/projectionbase/install.go:82-108`). Blob `6099cf69…` is 15,884,999,168 bytes. Guest free space at the constructed-skip observation was ~16.9 GiB. A behind-W rebase on this disk would need streaming unpack or more free space; it was not executed. Do not invent that this boot failed ENOSPC.

7. **S1-B is availability, not a lifetime-replay hole.** Once `H − start > 10000`, boot `log.Fatalf`s (`run.go:263-265`). Without periodic near-head publication, the owner computer **refuses** rather than replaying 128k events.

## Conditions (do not silently drop)

- **C1 — evidence wording:** cite structural `after=local` + resume decision, not absence of the >2s page-fetch log; quote `(replay complete)` or `/health` after reconstruct, not `run.go:332`.
- **C2 — nontrivial tail:** one staging product-path drill with `0 < H−W ≤ 10000` (rematerialize/restore receipts, or a behind-W retained boot that actually rebases). Local tests already pin rebase arithmetic; this is coverage of the interesting branch.
- **C3 — S1-B cadence:** publish W before live H exceeds W+10000, or retained boots fail closed.
- **C4 — first rebase disk budget:** do not assume a 15G blob plus unpack fits in ~16.9 GiB guest free; streaming unpack or host-side install-into-scratch remains follow-on if rebase is ever required on this disk.

## What NOT to reopen as Restore-Zero remaining error

- Reload-app / host-guest Vite asset graph (`dc291240`).
- Proxy 502 `failed to resolve user autoputer`.
- G4 constructed-computer CI skip / `active_computers.status=empty`.
- Demoted W=1 / W=13 HTTP 200 completion.
- Genesis guest `computer-bb0f4fa5`.
- Host/proxy SHA `9341b5d1` without the guest resume line.
- Weakening G4 or SSH `curl` to `/internal/vmctl/refresh`.
- Using `recover_current` (blank 32 GiB successor) to “force” a rebase photograph.

## Confidence

Medium-high on the skip heresy being gone on this computer. Medium on `completed` as the registry word for the original action-7 deployed-proof list. High that the receipt’s log-absence inference needed correction.
