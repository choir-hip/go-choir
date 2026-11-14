# The agentcore race suite runs at its own timeout

Recorded 2026-11-14T03:00Z.

## Problem

A landing runs `go test -race -count=1 -timeout 45m -p=1 ./internal/agentcore`
as one package. The suite now takes about as long as its timeout, so a busy
host turns a passing suite into a failed landing.

## Evidence

- 2026-11-14, coalescing landing on the agents VM: `ok ... internal/agentcore
  2559.942s`, 42.7 minutes of the 45 allowed.
- 2026-11-14, the store-table reading's landing on the new runner VM
  (`~/mirror/land-249ea176ad.result` there): `panic: test timed out after
  45m0s`, `FAIL ... 2700.140s`. No test had failed before the timeout. Both
  VMs were running suites at once on Node A's 12 cores (8 vCPU each, plus
  builders' verification runs on the agents VM).

## Belief

The timeout is the failure, not a test. The suite has grown past what one
45-minute run holds on a shared host.

## What was done

`scripts/land` now gives the heavy packages 90 minutes. That removes the
false failure and does nothing about the hour a landing takes.

## Remaining error

The real problem is the suite's length: every landing that touches
`internal/agentcore` or anything it imports pays about 45 minutes, serially.
`scripts/go-test-runtime-shards` already splits the package into six shards
for builders. The landing could run those shards two at a time across the
two VMs. Not done; it needs a measurement of peak memory per shard under
`-race` first, because Node A is overcommitted by about 1 GiB.
