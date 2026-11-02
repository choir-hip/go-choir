# Store Header/Body Split — Ledger

Append-only. One terse entry per pass.

- 2026-11-01T19:00-0500 drafted at 3529f400d after five reread fixes on 2026-11-01 that were one defect (filter fields after the body).
- 2026-11-01T19:37-0500 executable on the owner's word ("Make it a throughline now, superseding"); supersedes the guest isolation goal; now.status working, slice S0 (page reads per statement, three layouts).
- 2026-11-01T19:42-0500 S0: split beats body-last and today on every header read (census 19 vs 214 vs 737 MiB; retire slice 0.23 vs 2.68 vs 9.8); body reads equal. Evidence docs/evidence/store-layout-reads-2026-11-01.md. Next S1.
- 2026-11-01T19:59-0500 S1 design landed (Opus planner, adjudicated accept). Paper image 128 GiB allocated, store ~21 GB: the 43 GB migration peak fits; preflight refuses otherwise. Next S2 slice 0.
- 2026-11-01T21:10-0500 S2 integrated on split/s2-integrate (f3e5f7d2b): slices 0, A-E, cmd/store-split-migrate, 15 test-fixture fixes, epoch 33. CI mirror: gates, vet, build, store guards, light -race green; heavy -race (agentcore, store) left to GitHub CI. Mirror defect: its RESULT greps the whole lane file, so an earlier run's FAIL (6aa7ed5c) marked this head FAIL; the three objectgraph tests pass at f3e5f7d2b.
- 2026-11-01T22:45-0500 S3 on a reflink copy of the paper's store (Node B, nice/idle, MemoryHigh 4G): 22.30 GB -> 21.31 GB in 8m33s (copy ~3 min, index builds 5m44s, verify 15 s), og 3,403,389 rows, choir_events 0. Header table 2.7 GB, og_bodies 7.2 GB. Differential: 13 checks, 12 green; witness.tables differed only in store_meta, whose sole row is schema_version 1 -> 2 (the intended bump; design §5 said "every table digest equal", store_meta excepted). Diff tool now exempts exactly that row. The diff took 84 min and 2.4 TB of logical reads: its retire check runs 30 days x 3 modes over the old layout, which reads bodies (the defect itself). Paper data disk 128 GB, ~104 GB free (superblock): preflight need ~25 GB.
