# Capsule Go session worker dies at start; engineering retries forever (2026-10-09)

Found by the Gate 2 reality slice: the M11 self-development episode probe
on staging (build 19b7ef48), fresh disposable `computer-c995c31b…` (VM
`vm-1034d6a9…`), operation `selfdev-9d955cb3…` started 20:31:27Z.
Problem first; no fix in this commit. Written 20:55Z.

## Evidence

- The probe passed fresh_owner, bootstrap_chain, pre_episode_checkpoint,
  propose_only_armed and primary_started (20:31:27Z), then waited for the
  engineering desk to reach `awaiting_approval`.
- The engineering run (`run:assignment-5395fba2…`) called
  `capsule_go_eval` from its first iteration. Every call, starting with
  `println("hello")`, returned
  `session worker unavailable: session worker ready: frame: read header: EOF`
  about 7 ms after launch (49 of 49 results in the first event page).
- The desk kept trying (`println("retry-40")`, `"retry-41"`, …) and was at
  tool-loop iteration 398 at 20:53Z, one call every 3–4 s. Nothing bounds
  it.

## What this shows

1. **The capsule's Go session worker exits before its ready handshake.**
   `cmd/capsule-broker/session_worker.go` starts
   `--isolation-stage exec-go-session` with `hardened: true` (always, in
   production). With hardening on, the worker applies Landlock,
   capability drop and seccomp and calls `log.Fatalf` on any failure
   (`cmd/capsule-broker/main.go`). A fatal exit closes the socket, which
   the broker reads as EOF.
2. **The cause is invisible.** The broker captures the worker's stderr
   into a buffer and discards it; the error names only the EOF.
3. **Engineering has no failure budget for a dead tool.** A worker that
   cannot start costs ~400 model calls in 22 minutes and the operation
   never reaches a terminal state on its own.

## Hypotheses (not yet confirmed; the stderr will decide)

- H1: the hardening floor fails or kills the worker at start (Landlock
  ruleset, capability drop, or a seccomp filter that blocks a syscall the
  Go runtime or Yaegi init needs). The floor landed 2026-10-04
  (a578bbc8, gated by eb9c5b19), after M11's last green run (2026-09-29).
- H2: something in the worker's startup after hardening (allowed root,
  inherited socket fd 3) fails and exits fatally.

## Fix directions

1. Return the worker's stderr (bounded) in the "session worker
   unavailable" error and log it, so the next run names the cause.
   Diagnostic only.
2. Fix the cause the stderr names.
3. Bound identical failing tool calls in the engineering loop (a dead
   worker is a terminal blocker, not something to retry 400 times).
