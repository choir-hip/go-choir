# A server can miss its own shutdown signal (lost wakeup on done)

Date: 2026-10-23. Found by FP0 while landing D20 (the self-development
stamp heal), when its combined-tree CI hung twice in the race test shard
that runs `internal/server`. Status: documented; the fix follows in
separate commits. Mutation class of the fix: **orange** (runtime behavior:
how every service built on `internal/server` exits). Rollback: revert.

## Evidence

- PR #90, run 40002926558, attempt 1: "Go Test (race, non-runtime shard 6)"
  ran 60 minutes and was cancelled at the job timeout. The one test still
  running at the cancel was `internal/server`
  `TestHealthHandlerIncludesAddrAfterStart`, started 18:25:46 UTC; its last
  line was "test-addr-service: server stopped".
- Attempt 2 (the failed-jobs rerun) hung in the same shard; FP0 cancelled
  it after 33 minutes. The one test still running was
  `TestServerStartAndAcceptsRequests`, started 19:28:19 UTC, which uses the
  same Start and Shutdown pair.
- On main the same shard passes in 3 to 7 minutes (runs 39999634689 and
  39992068135), and on PR #87 (the heal on an older base) in 6. D20 touches
  no file in `internal/server` and `internal/server` imports none of the
  packages D20 changes, so the D20 tree most likely only changed what runs
  beside these tests and how loaded the runner is [H].

## Cause

`Server.done` is an unbuffered channel (`internal/server/server.go`,
`NewServer`). `Start` returns from `httpServer.Serve` with
`http.ErrServerClosed` and then blocks on `<-s.done`. `Shutdown` signals it
with a non-blocking send:

    select {
    case s.done <- struct{}{}:
    default:
    }

If `Shutdown` reaches that send before `Start` has reached `<-s.done`,
the send takes the `default` branch, the signal is lost, and `Start`
blocks forever. The window is between `httpServer.Shutdown` returning and
`Start` getting from `Serve`'s return to the receive. Under race-detector
load the losing order is likely enough to hang a shard. A `Shutdown`
called before `Start` runs at all (it returns `ErrServerClosed` from
`Serve` at once) always loses.

The same path runs in production: a service's main goroutine is in
`Start`, and SIGTERM calls `Shutdown` from the signal goroutine. When the
signal is lost, the process stays up until systemd's stop timeout kills
it, so the clean exit after the shutdown hooks never happens. That has not
been observed on Node B [H]; it is the same code.

## Fix shape

Make `done` buffered with capacity one. The first `Shutdown`'s send then
always lands, and `Start` takes it whenever it reaches the receive; a
second `Shutdown` still drops its send through `default`, as now. Test
first: a `Shutdown` before `Start` must let `Start` return within a bound
(it hangs on today's code).
