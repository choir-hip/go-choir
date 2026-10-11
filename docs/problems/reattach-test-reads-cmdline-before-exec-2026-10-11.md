# The reattach test reads a child's cmdline before the child has exec'd

Found 2026-10-11 in CI run 38134435618 (commit 8574a8e1, which does not
touch vmmanager). Evidence class: CI log and code reading.

## What happened

`TestReattachVMAdoptsByIdentity` failed in the standard non-runtime shard:
"healthy adopt = <nil>, pid 3468 does not identify VM vm-reattach;
refusing to adopt". The race shard of the same commit passed.

## Cause

The test starts a fake Firecracker (`/bin/sh <script> --id vm-reattach`)
with `cmd.Start()` and immediately calls `ReattachVM`, which reads
`/proc/<pid>/cmdline` to confirm the process names the VM. `Start` returns
after the fork; the child may not have exec'd yet, so its cmdline is still
the test binary's. The product check is right; the test does not wait for
the identity it then asserts.

## Test

The fixture waits, with a deadline, until the child's cmdline names the VM
before it saves the pid.
