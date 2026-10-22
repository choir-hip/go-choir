# A store open fails with "tables changed" under load, then Dolt panics in Close

Found 2026-10-22 01:16 UTC by FP0's VM CI mirror. It ran `go test -race`
on `internal/agentcore`, with NET-01 rebased onto 7c61ce078. The VM's load
average was about 13 on 8 cores, because five workers and two mirror jobs
were running at once.

    --- FAIL: TestEngineeringRequestRefusedWhenBindingDoesNotMatch (0.13s)
        engineering_cast_request_test.go:181: open store: runtime store: open unified Dolt workspace: texture workspace: ping database: tables changed
    store: open phase=workspace-open status=failed duration=125ms err=texture workspace: ping database: tables changed
    panic: ... dolt/go/libraries/utils/errors.Fatalf
        nbs.(*NomsBlockStore).Close  store/nbs/store.go:1758
        ... engine.(*SqlEngine).Close ... driver/v2.(*Connector).Close

The panic killed the whole package run, so every later test in the shard
was lost along with this one.

What is known:
- On the same tree, the test passed three times in a row with `-count=3`
  when run alone. NET-01 touches only `podcast.go` and `podcast_test.go` in
  this package.
- CI on 7c61ce078, which shards agentcore, passed.
- `tables changed` is Dolt's `errOptimisticLockFailedTables`
  (`store/nbs/store.go:1606`). It means the store's manifest was changed by
  another writer between this store's read of it and its update.
- `openTestStore` (`internal/agentcore/store_open_test.go:26`) copies a
  per-process template workspace into each test's own `t.TempDir()` path.
  So no other test should share the manifest.

Hypotheses. None is confirmed; the trace has not been read beyond the lines
above.
1. Within one store open, two Dolt handles on the same workspace race on
   its manifest. Examples would be the workspace open's ping against a
   background writer started earlier in the open, such as the FP1
   derived-key validation or the debounced checkpoint. If so, it is a product
   defect: a real boot under load could fail its store open.
2. The test fixture's two stores interleave. The test opens fixture `f`
   and then fixture `g`, and `f`'s committer may still be writing while `g`
   opens. That would matter only if the two copies shared a path, which the
   template copy should prevent.
3. Dolt's `Close` after a failed open calls `Fatalf` rather than returning
   an error. That would explain the panic whatever the cause of the
   failure, and it is a separate defect: a failed open must not be able to
   kill the process.

Next probe: reproduce on the VM under deliberate load, with
`-count=50 -race` on this test plus `stress-ng` or a parallel package run.
Capture the goroutines that hold the workspace's handles when the error
fires, then confirm or eliminate each hypothesis before any fix.

## Update 2026-10-22 (FP10 ARCH-13 round 8): hypothesis 3 confirmed

The panic half is reproduced and explained. Dolt registers every
SQL-engine database with `SetCrashOnFatalError`; `NomsBlockStore.Close`
flushes the journal root into `.dolt/noms`, and an I/O error there
crashes the process. ARCH-13's arch13j mirror hit it after a store
workspace was removed under its engine, and
`TestCloseAfterTheWorkspaceIsRemovedReturnsAnError` reproduces it on
demand. Commit 7078b0561 makes `Store.Close` leave the engine unclosed
and return an error when the workspace's `.dolt/noms` is gone. The
"tables changed" open failure itself (hypotheses 1 and 2) is not
addressed there; a failed open whose files are intact still closes its
engine normally.
