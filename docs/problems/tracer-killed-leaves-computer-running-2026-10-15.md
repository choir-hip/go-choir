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

## Repair shape (not done)

- Write the receipt incrementally (append each phase as it happens), so a
  kill leaves a partial receipt with `interrupted: true`.
- On SIGINT/SIGTERM/SIGHUP, record the interruption, call `stopMinted`, and
  exit non-zero.
- On start, find computers minted by earlier tracer labels that are still
  running (the credential files under ~/.config/choir-qa/test-computers/)
  and report or stop them.
- Investigate the 502 from the operations list on a busy computer.
