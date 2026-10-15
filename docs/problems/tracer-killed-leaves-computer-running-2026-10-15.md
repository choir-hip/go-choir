# A killed tracer leaves no receipt and its computer running

Found 2026-10-15 ~05:30 UTC, when the owner stopped the background task that
was running tracer `selfdev-jobs-0458` (staging e4cdc2ba6, test computer
computer-d7c52c2bb443f461ad666f06309f090b). The owner treats it as a useful
chaos-monkey observation: "Important to see how things resolve after
inadvertent shutoff."

## What happened

- The `node scripts/reeng_tracer.mjs` process died with its parent shell.
  `~/.config/choir-qa/traces/selfdev-jobs-0458/` stayed empty: no
  `receipt.json`, no partial phases. Thirty-two minutes of evidence
  (mint, story, casts, Freeze/Verify progress) exist only in the guest.
- The tracer never ran `stopMinted`, so the test computer kept running and
  holding Node B capacity until the lead stopped it by hand through
  `POST /api/computers/{id}/lifecycle/stop` (201, `stopped`).
- Just before that stop, `GET /api/computers/{id}/self-development/operations`
  answered 502, so the operation's state at the kill is unknown. Cause of the
  502 not investigated (the computer was busy; the proxy may have timed out).

## Why it matters

Tracers are the acceptance proof for self-development. A tracer that dies
silently loses its evidence and leaks a computer, and the next run cannot
tell whether the previous one left an operation half-applied.

## Repair shape (first three done 2026-10-15, below)

- Write the receipt incrementally (append each phase as it happens), so a
  kill leaves a partial receipt with `interrupted: true`.
- On SIGINT/SIGTERM/SIGHUP, record the interruption, call `stopMinted`, and
  exit non-zero.
- On start, find computers minted by earlier tracer labels that are still
  running (the credential files under ~/.config/choir-qa/test-computers/)
  and report or stop them.
- Investigate the 502 from the operations list on a busy computer.

## Repair (2026-10-15)

Mutation class yellow: the tracer script only. There is no product change
and no deploy.

Ways it could fail, written before the code (K1–K8 in
scripts/reeng_tracer_receipt_test.mjs):
- K1: SIGKILL leaves nothing behind.
- K2: a partial receipt reads as a pass.
- K3: an interrupt before mint names a computer the run never had.
- K4: an interrupt records no signal or exit code.
- K5: the startup report misses a computer left behind by a killed run.
- K6: the report names stopped or finished runs.
- K7: kept computers are reported.
- K8: a two-computer stop counts as stopped when only one stopped.

What changed:
- **scripts/reeng_tracer_receipt.mjs** (pure, 8 node tests) builds the
  receipt states.
- **scripts/reeng_tracer.mjs** uses them:
  - It rewrites `receipt.json` at every phase, with `in_progress: true`
    and `exit_code: null`.
  - It binds the stop to the attempt before the `mint` phase.
  - On SIGINT, SIGTERM or SIGHUP it stops the minted computer (bounded at
    90 s), closes the receipt with `interrupted`, and exits 128 + the
    signal number. A second signal exits at once.
  - At start it reports, and never stops, computers that earlier killed
    or interrupted runs left running.

### Proof on staging (b244507f4), 21:45–21:47 UTC

Both runs used `--skip-export`, and each acted only on the computer it
minted.
- **Run A** (`kill-proof-a-1015`) was sent `kill -9` after
  `story_submitted`. Its receipt was on disk with `in_progress: true`,
  `exit_code: null`, `computer_id: computer-f4158479…`, and phases `mint`
  and `story_submitted`. (K1, K2)
- **Run B** (`kill-proof-b-1015`) logged at start: `stranded: tracer
  kill-proof-a-1015 left computer-f4158479… (killed, …)`. No older trace
  was reported. (K5, K6)
- B was sent SIGTERM after `story_submitted`. It logged `stopped
  computer-ef58510f…: 201 stopped` and exited 143. Its receipt has
  `in_progress: false`, `exit_code: 143`, and `interrupted: {signal:
  SIGTERM, computer_stop: {status: 201, state: stopped}}`. (K4)
- Run A's computer was then stopped by the operator through
  `POST /api/computers/{id}/lifecycle/stop` (201 stopped). A's receipt
  was closed as interrupted with that stop, so later runs no longer
  report it.
- The receipts are in `~/.config/choir-qa/traces/kill-proof-{a,b}-1015/`.

**Still open:** the 502 from the operations list on a busy computer.
