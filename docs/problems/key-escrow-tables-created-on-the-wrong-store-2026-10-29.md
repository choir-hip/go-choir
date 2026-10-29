# Key-escrow tables are created on the wrong store (2026-10-29)

Status: open; documented, not fixed. Class of a fix: red (key escrow,
store schema). Found by the Store A pre-split liveness investigation
(docs/evidence/store-a-pre-split-tables-2026-10-29.md).

`computer_key_escrows`, `computer_key_escrow_transparency`,
`computer_key_unwrap_requests` and `computer_key_unwrap_approvals` are
created by `corpusSchemaDDL` (`internal/platform/store.go:181-210`), which
bootstraps on the Store B (corpus) pool. `internal/platform/key_escrow.go`
reads and writes them through `s.db`, Store A. On Node B Store A has them
only because they pre-existed the A/B split; Store B holds empty copies.

Consequence: any fresh or rebuilt Store A (a history-free rebuild, a move to
PostgreSQL, a restore on a new host) would not get these tables from the
bootstrap, and key escrow, the second copy of every computer's privacy key,
would break or silently start empty. Every rebuild or migration keep-set
must carry them until the DDL runs on the store that serves them.

Fix direction (not started): move their DDL to the Store A bootstrap (or
make `key_escrow.go`'s store own its DDL), with a test that a fresh Store A
can escrow and unwrap.
