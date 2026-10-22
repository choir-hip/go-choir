# FP2 batch 1 turned CI red: the desk isolation self-test needs root, and the worker's stdout buffer races

Found 2026-10-22 00:50 UTC by CI run 39701731511 on 564840fcf (FP2 batch 1
plus the ride-along batch). The run failed before the deploy, so staging
still serves ca21f1ce7. Neither failure ran on the Mac before landing: one
test is Linux-only, and the other fails only under `-race`.

## 1. `TestIsolationSelfTestDeskCellsThroughTheRealWorker` assumes a root spawner

`internal/agentcore/isolation_selftest_linux_test.go:30`:

    self-test through the real worker failed: [texture: read /dev/shm/choir-cell-management-…/selftest-canary succeeded
    research: read /dev/shm/choir-cell-texture-…/selftest-canary succeeded]

The self-test checks that each desk's cell can't read another desk's
canary. Separate uids per desk keep the canaries apart, and only a root
spawner can switch uids. The GitHub runner, and the node-a agents VM, spawn
as an ordinary user, so every desk runs under one uid and the read succeeds.
The FP2 batch 2 worker saw the same failure in the VM and recorded it as
"the first-batch test failure, still open"; FP0 did not catch that before
landing.

The yaegikernel suite already has both forms: `TestDeskCellWorkerSandboxed`
(root) and `TestDeskCellWorkerSandboxedWithoutRoot` (the CI and dev form,
which skips as root). This agentcore test asserts the cross-desk read
refusal without that gate.

Is it a product defect? Not on the guest. The autoputer daemon spawns as
root, so desks get distinct uids. The deployed evidence is FP2's proof
step 2, which runs this same self-test on a staging test computer.

There is one real residual. Under a non-root spawner (dev and CI only), the
cross-desk read is not refused, so Landlock does not separate desks'
`/dev/shm` cells by itself. Desk separation depends on the uid switch.

## 2. The desk worker's stdout and stderr are unsynchronized `bytes.Buffer`s

`internal/yaegikernel/desk_session_worker.go:215` gives the child's stdout
and stderr to `exec.Cmd` as plain `bytes.Buffer`s. `exec` copies into them
from its own goroutine while the process runs. Two readers can run during
that time:
- the test helper `workerProbes`, in `desk_cell_confinement_linux_test.go:248`, which CI's race detector caught;
- in production, the spawn error path (`:259`) and the exit cause (`:750`), which read the buffers too.

The exit-cause read happens after reaping, so it is safe. The spawn error
path is not: it can read while the copy goroutine still writes. It is a
data race on crash diagnostics only, never on the frame transport.

## Repair

- The agentcore self-test test skips under a non-root spawner, with the
  reason. The root form still runs wherever tests run as root, and the
  deployed self-test is the acceptance.
- One small lock-guarded buffer type serves the worker's stdout and
  stderr. The spawn error path, the exit cause and the test helper all read
  through it.

Rollback: revert the repair commit. CI goes red again on the same two
tests; no product behavior changes.
