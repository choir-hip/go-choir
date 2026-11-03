# luna-arm trace review (2026-11-03, computer-2bf3cab1...)

Delegate-internal behaviour is not in the export (events live in the capsule /tmp); only `wait` status and final text are visible.

## Rounds (bind to Freeze, outcome, judge reason)
Files (no delegate, no shots): ops 1a7a1195 2.8m, 1b52f0ff 5.6m, 6038b25b 4.5m, 7708bffc 2.6m, 00592bf5 16.9m, e55e1cf8 2.5m, all failed: partial shell, no browser run, no shots or measurements. About 12 later assignments blocked in 0.5-1.5m; two were cancelled by gateway 400 (11:23, 11:28).
Slides:
- R1a 0efb6e06 3.1m failed: engineer itself said shots not done. R1b c403e37c 26.5m failed: shots 11,12 were 1100x750. R2 0ef4ade2 3.4m failed: only 2 shots frozen, verifier could not see PNGs.
- R3 3a83afac 14.5m failed: shot 03 showed an Autopaper error, states not exercised.
- R4-R5 (12:48-13:11) three assignments blocked in 0.4-1.7m: "no .git", "files already exist", "no slides-next in workspace". No op.
- R6 69de017b 3.6m failed: one boot-screen shot. Five more assignments (13:24-13:53) blocked.
- R7-R9 (14:01-16:14): 58a33e94 blocked (autoputer died with its job); d96b46c1 and ef3891f1 cancelled after 46s and 45s ("circuit open (model unhealthy)"); 9c22d795, 6210d4f5 blocked in under 1m.
- R10 7bb175e0 10.4m, op 3d4b74c0 failed: mocked decks (/computer withheld, problem 26).
- R11a 898728a4 2.1m, op cd23e16b failed: "unknown paths ... 2 paths". R11b b04f47e3 2.7m, op 85772169 failed: no shot definitions or screenshots.
- R12 00d5d68f 17.4m, op d3ce7dbc applied (judge pass).
- Calendar R1 5294355b 12.1m: Complete blocked, no Freeze, no op; package written, zero shots.

## Delegate runs
R10 44 turns/43 tools, 8.5m, $0.77, mocked shots. R11a 11/10, 47s, $0.11, no edits. R11b 12/11, 67s, $0.17, reapply only. R12-1 41/48, 13.4m, $1.30, `error_max_turns` with shots 01-10 done. R12-resume failed in 0s ("No conversation found with session ID"). R12-2 17/16, 89s, $0.13, shots 11-12 and a CSS fix. Calendar 21/20, 5.1m, $0.69, five files, never built or ran.

## Findings (ranked by rounds failed / minutes lost)
1. **The delegate sees only `objective`, and luna's brief drops the request.** Symptom: R11a delegate: "I searched the repo ... found no list" of shots; Calendar delegate: "I never received the owner's request text or the list of twelve shots. I picked twelve myself"; R10 spec said "make the appropriate polished slides workflow changes" with no feature list; R11b spec told it "Do not create shot definitions". Four rounds. Cause: `Prompt(spec)` is the objective alone (spec.go:196); the overlay says "Write the objective from management's Cast" (rlm_engineering_runtime.yaml:73), which luna compresses to about 700 characters. Change: the broker writes the Cast body to `<run dir>/cast.md` and the system context says "read it first"; the overlay says to paste owner_directions verbatim (limit is 16 KB). Status: open.
2. **Delegate env strips every CHOIR_ variable and it cannot start jobs.** Symptom: "no $CHOIR_FRONTEND_DEPS or $CHOIR_BROWSER" (R11a); "choir-shot stopped: CHOIR_FRONTEND_DEPS is not set" and "npx vite refused by the registry (403)" (Calendar), so the Svelte was never compiled by the delegate, and luna spent 6.5m doing shots itself. Cause: `deniedEnvPrefixes` includes "CHOIR_" (spec.go:204). Change: pass CHOIR_FRONTEND_DEPS and CHOIR_BROWSER (store paths, not secrets) and say so in SystemContext. Status: open.
3. **Early Freeze with an admitted gap.** About nine ops (all Files rounds, R1a, R6, R11b) froze at 2-5m with "no screenshots" in the Complete text, then died at the judge. R11b ledger: delegate told to reapply only; luna wrote an empty `/tmp/shots.mjs` (`export default []`), noted "Cannot responsibly fake", and froze. Hypothesis: the Cast's "rather than ending the round empty" read as licence to freeze. Change: Freeze output prints the screen count in /var/lib/artifact/screens, and warns when the Cast named screenshots and there are none. Status: open.
4. **A gateway circuit-open cancels the assignment at loop iteration 3 and nothing retries.** Cancelled at 14:21:51 and 15:54:08 (and two 400s at 11:23, 11:28). Engineering lost under 3 minutes, but the arm had no useful attempt from 14:21 to 16:03 (100 min); Texture heard at 14:44. Change: treat circuit-open like the host-memory wait (6e4658891): park and retry to the deadline. Status: open.
5. **Exec cwd is `/`.** R11a ran `git apply /selfdev/carried/source.patch` there; `ls` at 17:55:59 shows `/frontend/src/packages/slides-next` beside the real one; Freeze printed FROZEN, then failed on "2 paths" at commit (App.svelte, manifest, hypothesis). Change: default Exec cwd to /workspace/platform; report unknown paths when Freeze is called. Status: partly fixed (70160304a names the paths, only at commit).
6. **Carried source inconsistency (R4-R9).** Six blocks: "no .git", "files already exist", "workspace lacks slides-next". Change: one line in the engineer's context saying whether the source is in place. Status: owner notes only.
7. **max_turns copied from the overlay example (40).** R12-1 died at turn 41 of a 40-minute budget, status "failed" with 10 shots made. Change: example 120 for shot jobs. Status: open.
8. **resume_session cannot work.** Each run gets `HOME=<run>/home` (server.go:265), yet overlay line 75 recommends resume. Status: open.
9. **choir-shot has no open-app step.** Calendar: four runs timed out waiting for `[data-calendar-next]` after `page.goto('/')`; luna read App.svelte and Desktop.svelte, then blocked. The launcher recipe exists only in lane C's slides-r6 text. Change: `choir-shot --open-app <id>`. Status: open.
10. **Small costs.** Go slips `[] ,` (5x), `import os/exec` (2x), a mistyped receipt hash, `npm run build` in a cell (OOM exit 134 twice, R10), Freeze "no frozen runtime artifacts" (R10, R12). Each costs a model turn. Fix: omittable slice arguments, and a Freeze message that names the cp command.

## Keep
- R12: whole job to the delegate, one parked wait (13.4m, zero model turns), then `ls` of the screens directory to learn that 10 shots existed despite error_max_turns.
- Fresh delegate for shots 11-12 after resume failed: 89s, $0.13, fixed a real 27px overflow.
- Luna re-ran check-packages and the build itself, and the summary named 3 delegate runs, the failure, and "shots 01-10 not rerun after the mobile CSS change".
- R10: one delegate call, a full package in 8.5m for $0.77. Total delegate spend about $3.16.
