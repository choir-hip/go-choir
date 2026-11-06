# Interpreted goroutines race on yaegi's own frames (2026-11-06)

## Problem

Since MA6.5 (5c83a08ff) a desk cell may import `sync` and `context` and start
goroutines, and MA6.6 code mode encourages fan-out. The Go interpreter that
runs cells, github.com/traefik/yaegi (v0.16.1 in go.mod; v0.16.3 is the same),
is not safe for that: every interpreted closure started as a goroutine races
on the interpreter's own frame bookkeeping, whatever locks the cell's code
takes.

## Evidence

- CI run 42237103762 (fa6713c44), race non-runtime shard 6:
  `TestSessionWidenedStdlib` (internal/yaegikernel/go_modules_test.go) fails
  with DATA RACE, write at yaegi interp/run.go:1922 (`getFrame(f, l).data[i] = o`
  after a closure call) against read at run.go:1890 (`o := getFrame(f, l).data[i]`,
  unlocked, as the loop makes the next `go func(n int){...}(i)` closure).
  Intermittent in that test; it passed the CI of 5c83a08ff.
- A Sonnet investigation (read-only) reproduced it with a loop of 50 `go func`
  closures that each take a user mutex: 30 of 30 runs fail under `-race` on
  v0.16.1 and v0.16.3.
- Without the race detector the same defect is wrong behaviour: a finishing
  goroutine restores the parent's slot to its old (nil) func after the next
  iteration stored its closure there, and the call panics with
  `reflect.Value.Call: call of nil function` at run.go:1310. A torn
  `reflect.Value` could in principle crash the worker with a fatal error.
- Related upstream, open: PR #1739 (`_select` shares one `[]reflect.SelectCase`
  across goroutines running the same select, so goroutines can swap channels)
  and issue #1658. No upstream issue names this race; no maintained fork fixes it.

## Blast radius

One desk worker process: a wrong cell result, a panic, or a worker crash. Each
desk worker runs in its own process under gVisor (MA6.4), and the interpreter
shares frames only within a process; no cross-desk or host path was found. A
crash closes the open work as interrupted (restart rule).

## Fix options (cheapest first)

1. Patch yaegi: delete the slot read at run.go:1890 and the restore at
   1921-1923 (the closure runs on `fr`, a clone taken before the slot holds
   `fct`, so nothing needs restoring). 0 of 300 runs fail with the patch.
   Carry it in a fork under the org with a go.mod `replace` (the repo does
   not vendor third-party code), cherry-pick #1739 into the same fork, and
   send both upstream. Recommended.
2. Refuse `go` in cells and offer a host fan-out helper. Partial (closures
   still share parent frames) and removes the MA6.6 fan-out.
3. One interpreter lock around every cell callback. Costs the concurrency
   and risks deadlock with channels and WaitGroup.

## State

Documented; not fixed. Option 1 needs a public fork repository under the org,
an outward step the owner is asked about. Until then the race shard can fail
on `TestSessionWidenedStdlib`; a failure there is this defect, not a flake.

## Update, same day

The owner authorized a fork with the fix and wants it merged upstream.
A second Sonnet investigation confirmed the defect is yaegi's: Choir builds
one interpreter per session, calls `Use` once and evaluates sequentially,
and every racing frame is yaegi's. The write-back exists for the #1618
memory-leak fix (commit 3fbebb36); the patch keeps it, reading the slot
under the frame lock and restoring only if the slot still holds this
closure. Upstream PR: https://github.com/traefik/yaegi/pull/1752 (draft
until the fork has run on staging; the CLA is signed). Fork:
github.com/choir-hip/yaegi branch `choir` = v0.16.3 + #1752 + #1739 (the
select race). Standalone repro: 96/100 races on stock v0.16.3, 0/100 on
the fork. Devin reviewed the pin and both patches: ACCEPT.

Pre-existing upstream residuals Devin named, unchanged by the fork:
`defer g()` where `g` holds an interpreted function literal can deadlock
the interpreter (the deferred call runs under `f.mutex` and the closure's
restore takes it again); and a cell can reassign non-constant process
globals such as `io.EOF` or `time.Local` inside its own worker process.
