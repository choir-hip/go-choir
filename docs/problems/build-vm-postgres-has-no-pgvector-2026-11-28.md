# The build VM's Postgres has no pgvector, so nothing near internal/platform can land

Standing: governing | kind: happened | settled-by: two landing-queue runs on 2026-11-28 whose suites failed on choir-agents, result file ~/mirror/land-b5ebfbe3d7.result | note: the VM was read only; when it lost the extension, or whether it ever had it, was not established

## Problem

The landing queue runs, on the build VM (`choir-agents`), the whole suite of
every changed Go package and of every package that imports one. For
`internal/platform` that suite needs the pgvector extension. The VM's Postgres
does not have it: 30 tests fail with `extension "vector" is not available
(SQLSTATE 0A000)`. So a landing that changes `internal/platform`, or any
package it imports, fails at the suites step and nothing is pushed.

## Evidence

- A docs move that changed only the path of a document inside comments in
  eight Go files was set aside twice. First with two files of
  `internal/platform` changed, then with none of them changed but a package
  that `internal/platform` imports changed. Each time
  `FAIL github.com/yusefmosiah/go-choir/internal/platform` and the message
  above, 30 tests.
- CI on main is green on the same commits, so CI's Postgres has the
  extension.
- The move landed (20e3e0e1c) only after every change under `internal/` was
  taken out of it.

## Belief

The VM's Postgres package set lacks pgvector. The message names
`go-choir-postgresql-roles` as what creates the extension on a host; the VM
has no such step. Not checked: the VM's nix configuration.

## Remaining error

- Until the VM has pgvector, a real change to `internal/platform` cannot land
  through the queue. Nobody has needed to since the vector work landed, or
  they landed another way; which was not checked.
- Eight comments in Go files still cite the old paths of documents archived
  on 2026-11-28: `internal/agentcore/event_vector_shortlist_test.go:3`,
  `internal/autoputer/diag_drill.go:6`, `internal/autoputer/diag_drill_test.go:19`,
  `internal/capsule/manifest_whiteout_test.go:4`,
  `internal/platform/capture_rank.go:4`, `internal/platform/vector_store.go:4`,
  `internal/provider/bedrock_cache_ttl_test.go:11`,
  `internal/vmmanager/io_write_cap.go:16`. Each needs `docs/` changed to
  `docs/archive/` once such a change can land.
