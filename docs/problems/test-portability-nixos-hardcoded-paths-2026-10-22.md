# Tests hard-code FHS paths and lose the Nix loader path on NixOS (2026-10-22)

Mutation class: green (problem record); fix is yellow (test code only).

## Evidence (reproduced on the Linux worker VM, NixOS)

1. `go test ./internal/vmmanager -run TestLaunchThreadPolicyReachesOnlyItsChild`
   fails: `fork/exec /bin/cat: no such file or directory`.
2. `go test ./internal/yaegikernel` fails 10 tests (CellOutput*, DeskSessionWorker*,
   Egress*, RunningCellSeesALaterArrival, ArrivalsBetweenCells): the spawned
   worker, a cgo test binary, dies with `error while loading shared libraries:
   libicui18n.so.76`. `DeskSessionWorker.Spawn` builds the worker env from
   scratch (`PATH`, `TMPDIR`, session vars, `ExtraEnv`), so `LD_LIBRARY_PATH` is
   dropped. That scrub is intentional production behavior.
3. `go test ./cmd/capsule-broker` fails `TestHandleExecDirectNoShell` and
   `TestHandleExecDirectTimeoutReapsGroup`: `/bin/echo` and `/bin/sleep` do not
   exist. (The "seccomp: load filter" log line in that run is emitted by a fake
   worker script in session_worker_stderr_test.go and is not a failure.)

These hide real failures because workers learn to ignore red runs that pass in CI.

## Fix (test code only; production unchanged)

- Resolve `cat`, `echo`, `sleep` via `exec.LookPath` in the tests.
- `deskWorkerConfig` passes the test process's own `LD_LIBRARY_PATH`/`LD_PRELOAD`
  through `ExtraEnv`. The production env scrub is untouched, so what the tests
  assert about the scrub (secrets not leaked) is unchanged.

Heresy delta: discovered 0 new, introduced 0, repaired 3 portability cases.
Rollback: revert the commit.
