# What still grows on Node B after the sweep

Date: 2026-11-13. Status: open. Found by a read-only survey of Node B
(growth-meter ledger, `du`, `pg_class`), 16:00Z to 17:00Z.

## Problem

With the destroyed computers gone and the snapshot loop fixed, Node B still
grows about 10 GiB a day against 135 GiB free. Four items have no bound or a
bound that does not hold.

## Evidence

| item | size | per day | bound today |
|---|---|---|---|
| `platform-artifacts/capture-bodies` | 24 GiB, 387,000 files, 11 GiB older than 7 days | +1.7 GiB | none: not in the collector's namespaces, and the writer never prunes |
| the paper's data image | 50.8 GiB allocated; the store inside is 21 to 22 GiB | +2.1 GiB | discard and an hourly guest fstrim are configured, yet about 30 GiB is not returned |
| d@e's data image | 28.6 GiB; its tape is 3.5 GiB | +2.0 GiB | the same |
| `computer_event_append_receipts` | 27.7 GiB (17.6 heap, 10.0 indexes), 12.7 million rows | +0.8 GiB | none; compaction saved 5% on the paper |
| the paper's tape segments | 71.8 GiB | +2.6 GiB | none for a live computer (station S5 cuts the rate) |
| `platform-update` | 7.7 GiB | +1.0 GiB | the collector, 168 hour grace |

Bounded and healthy: guest releases (8.6 GiB, retention every 30 minutes),
`/nix/store` (25.9 GiB, collected every 4 hours), journald, chunks.
One-time: b@c's hibernated image 19.2 GiB (never deleted, owner rule), two
stopped images 7.7 GiB, old work directories 2.5 GiB.

corpusd has no memory ceiling; its unit peaked at 9.7 GiB, almost all file
cache (268 MB of its own memory). The host never fell below 11.6 GiB
available in 24 hours. Not urgent.

## Belief

Capture bodies are the only large store with no bound at all. The two data
images are the largest possible release (up to about 50 GiB) and the least
understood: nobody has looked at what fills the volume beside the store.
Receipts need fewer rows, not smaller rows (station S6, after S5).

## Remaining error

The images were measured from the host only. The share of capture bodies
that nothing references is unknown. One index on the receipts table
(`receipt_id_idx`, 2.2 GiB, 2,184 scans) may be droppable; its readers are
not yet identified.
