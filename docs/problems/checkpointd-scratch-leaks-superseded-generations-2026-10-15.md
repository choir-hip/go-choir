# checkpointd keeps every generation's scratch: 6.6 GB in a week (2026-10-15)

Status: documented; fix follows in a separate commit. Mutation class of
the fix: orange (checkpointd housekeeping; no change to what is replayed
or published). Rollback: git revert.

## Evidence (Node B, 2026-10-15 about 19:05 UTC)

- `/var/lib/go-choir/platform-artifacts/checkpoint-scratch` holds 6.6 GB
  in 41 directories, one per `<computer>-<generation>`.
- By modification date: 10-09: 1; 10-11: 6; 10-12: 11; 10-13: 12;
  10-14: 7; 10-15: 4.
- The largest directories are about 660 MB each. One computer
  (`1355b83b…`) keeps generations 14, 15, 16, 17 and 19.
- Host root is 78% used (368 of 476 GB). The 48 GB pre-2.4.2 Dolt backup
  dominates, and it stays until the soak ends. The scratch grows by
  roughly 1 GB a day, and nothing removes it.

## Cause

`cmd/checkpointd/worker.go` removes the scratch directory only after a
publish succeeds. On any failure it is kept, and that is deliberate within
a generation: `projectionbase.Rebuilder` resumes from it on retry (the
`runtime.db` marker and seed meta). But once a computer's job moves to a
new generation, the earlier generations' directories are never read
again, and nothing deletes them.

## Failure modes the fix must avoid (written before the code)

- S1: deleting the current generation's directory, which breaks the resume
  of a `retry` job.
- S2: deleting another computer's directory. Computer ids are prefixes of
  nothing, but a name match must be exact on `<computer>-<int>`.
- S3: deleting anything not shaped `<computer>-<int>` under the scratch
  root.
- S4: a cleanup error failing the job. Cleanup is best effort and logged.
- S5: deleting a newer generation's directory, if generations ever run
  out of order.

## Fix direction

When the worker claims generation N for computer C, it removes the
scratch directories `C-<g>` with g < N, and keeps C-N and anything newer.
The leak is then bounded to one directory per computer.
