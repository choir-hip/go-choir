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

## Why the retries never end (20:57Z, code reading)

Only Texture runs get a tool-loop budget (`textureActorToolLoopBudget`,
`agentcore/runtime.go`); engineering runs have none. And the engineering
completion guard refuses to let a turn end without a committed terminal
fate ("Finish the assignment inside a capsule_go_eval cell … Narrative
text alone does not end the run."). Fates are committed from inside a
capsule cell. With the worker dead, the run can neither do the work nor
record that it cannot: a deadlock, held open by the guard.

Fix direction 3, sharpened: give engineering a tool-loop budget, and let
a run whose capsule worker is unavailable end with a runtime-recorded
blocker fate instead of a model-committed one.

## Cause confirmed (21:34Z, M11 rerun on 7922bd56 with the stderr diagnostic)

Every `capsule_go_eval` on the fresh disposable `computer-c1f4b153…`
returned:

    session worker landlock: failed to apply Landlock restrictions:
    missing kernel Landlock support. Got Landlock ABI v0, wanted
    {Landlock V5; FS: all; Net: ∅; Scoped: ∅}

H1 holds, in its first layer: the guest kernel has no Landlock, and the
S1 floor (a578bbc8, 2026-10-04) fails closed by design, so the session
worker can never start. Self-development on staging has been unable to
run a capsule cell since that floor landed.

Fix direction: enable Landlock (ABI v5 needs kernel 6.10 or later) in the
guest kernel and its LSM list. Not chosen: best-effort Landlock, which
would silently drop the floor this design makes mandatory.

## Correction (21:36Z): the kernel has Landlock; the worker inherits a filter that refuses it

The guest kernel is 6.18 with `lsm=landlock,yama,bpf` in the params vmctl
passes, and the broker applies its own Landlock successfully before it
spawns anything (`cmd/capsule-broker/main.go`; it would `log.Fatalf`
otherwise). It then loads `LoadBrokerFilter`, a seccomp allowlist that
equals the workload list and has no `landlock_create_ruleset`,
`landlock_add_rule` or `landlock_restrict_self`. Seccomp filters are
inherited across fork and exec, so the session worker's Landlock call
gets EPERM, which go-landlock reports as "missing kernel Landlock
support … ABI v0". The earlier "guest kernel has no Landlock" reading was
an inference the code does not support.

The next layer is safe: the worker's capability drop skips the
bounding-set drop when CAP_SETPCAP is not effective (moby/sys/capability
`Apply`), and the broker's bounding set already holds only five caps.

Fix: the broker's filter allows the three Landlock syscalls (Landlock
can only remove access). The worker applies its Landlock, drops caps,
then stacks the workload filter, which still refuses Landlock to model
code.

## Budget verified on staging (7922bd56)

The second M11 rerun (`evidence/m11-rerun-2026-10-09T21-32-17Z.json`,
5/16 again) carried the worker's stderr into every result (the
diagnostic works) and its engineering run stopped at tool-loop iteration
200 at 21:43:37Z, eleven minutes after it started, and ended
`cancelled`; the probe then finished instead of waiting out its hour.

## Third rerun (21:50Z, e3225820): Landlock passes; the worker's own seccomp load is refused

`computer-b9722092…`: every result now reads `session worker seccomp: failed
to load workload seccomp filter: failed loading seccomp filter: operation
not permitted`. Same mechanism one layer later: the inherited broker
allowlist has no `seccomp` syscall, so the worker cannot stack its own
filter. (Adding a seccomp filter can only restrict.) The e3225820 test
checked the Landlock syscalls alone; the floor should be tested as a
sequence, the worker's layers applied under the broker filter.

## Repaired on staging (22:04Z, 733bec77)

Fourth M11 rerun, `computer-beb952a2…`: the first `capsule_go_eval`
returned the cell's stdout (slot, activation, capsule computer id) with
exit 0; later cells show the floor working as designed (`import "os"
refused: direct OS manipulation must route through capsule broker`) and
ordinary compile errors, then directory listings. The session worker
starts hardened (Landlock, capability drop, workload seccomp) under the
broker's filter. Whether the episode reaches approval, apply and restore
is the rest of this rerun.
