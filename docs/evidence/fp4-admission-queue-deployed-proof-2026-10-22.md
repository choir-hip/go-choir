# FP4 engineering admission queue: deployed proof on a test computer (2026-10-22)

Run by the self-development monitor (lane/fp4-selfdev-monitor) for FP0,
on deploy J (a84b39bac, FP4 round 2). The target was the test computer
`computer-71afbf1461dffabcce8eeebe558d03bd` (vm-c5c801f8…, warmness
`test`), and no other computer. Calls went through the product path,
`https://choir.news`, with the test computer's key. The host-internal
token was sent to no guest address. There was no SQL and no tape edit,
and no apply was approved. The drill is the `next_action` of
docs/definitions/choir-launch-fp4-selfdev-architecture-2026-10-21.md.
Judged steps use `scripts/proofs/fp4-selfdev-admission.sh` where it has
a step, and otherwise the rows, events and trace.

## Plan and prompts (written before any cast)

- A is held live for at least five minutes with a `sleep 300` job, then
  `go vet`, then a normal release build and a Freeze. A should end
  `completed` with its candidate `awaiting_approval`, and is not
  approved.
- B is cast while A is live. It should queue behind A ("waits behind"),
  bind within seconds of A's terminal_at (the nudge), and then make one
  cheap Freeze. With A's candidate open, that freeze should be refused
  at the operation open ("cannot freeze until the open candidate
  settles").
- B reports `partial`. B's freeze is refused at
  `OpenFreezeCandidate` (internal/agentcore/freeze_candidate.go:57), so
  it opens no operation.

Request A (sent verbatim):

```
Engineering request A (FP4 admission drill FP4AQ-20261022, test computer only): through the engineering desk, make one small, real, reversible source change. Keep this assignment live for at least five minutes: a second engineering request will be cast while it works and must queue behind it.
1. In internal/selfdev/operations.go add the comment line "// Operation is one self-development change, from freeze to its decision." directly above "type Operation struct".
2. Start the job `sh -c 'sleep 300'` and wait for it to end (the drill needs A live for five minutes).
3. Check the change with `go vet ./internal/selfdev`.
4. Build the release as usual (bin/autoputer and the frontend), then Freeze the candidate for my review.
5. Await my decision; do not apply it.
Management: cast this once; a second request B will arrive and should wait in the engineering queue, not replace this one.
```

Request B (sent verbatim):

```
Engineering request B (FP4 admission drill FP4AQ-20261022, test computer only): through the engineering desk, in one assignment. This request is expected to wait in the engineering queue behind request A, which is already working; that is the drill.
1. In internal/selfdev/operations.go add the comment line "// Store keeps self-development operations." directly above "type Store struct".
2. Record three cheap receipts, each in its own cell: the toolchain (`go version`, `node --version`, `npm --version`); the check `gofmt -l internal/selfdev/operations.go` (no output expected); and, as the build stand-in, `sh -c 'test -s internal/selfdev/operations.go'`.
3. Do not run go vet, go build, npm or vite.
4. Call Freeze exactly once, citing those three receipts. It is expected to be refused while A's candidate awaits my decision.
5. When Freeze is refused, do not retry and do not work around it: Complete the assignment as partial, quoting the refusal text.
Management: cast this once and do not re-cast, cancel or replace request A; report B's outcome back to me.
```

## Step 3 (planned refresh while A is live): unreachable by design, on any computer

- The guest's planned-restart endpoint (`HandleInternalPlannedRestart`)
  refuses whenever `plannedRestartWork.unsafe()` holds:
  `SelfdevActive > 0 || LiveCapsules > 0`
  (internal/agentcore/planned_restart.go:348). Its comment says such work
  "always refuse[s]", whatever the host asks (`allow_busy` included).
- A bound, non-terminal assignment whose capsule is active, freezing,
  frozen-undecided or being revoked counts as a live capsule
  (`plannedRestartLiveCapsule`, :388).
- So while A is live, no host refresh can get a marker-backed restart,
  `critical_protected` or not.
- The other marker writer is the self-development apply
  (`markPlannedRestart`, :298). It needs an approved candidate, and this
  drill approves nothing.
- vmctl's deploy refresh of a non-protected computer also defers on a
  live capsule.
- So "B kept and resumed on a planned boot" can only be reached when B is
  queued and no capsule is live:
  - A is terminal and B not yet bound. The nudge closes that window in
    seconds.
  - Or A is open but not yet bound, which lasts about 20 s in practice.
- Neither is forced here, and step 3 is not run.

## Steps

