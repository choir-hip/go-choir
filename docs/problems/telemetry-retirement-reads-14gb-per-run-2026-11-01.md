# Telemetry retirement reads about 14.5 GB of the paper's store per run

Date: 2026-11-01. Found by the paper memory sampler during the guest memory
cap benchmark (docs/definitions/choir-memory-optimization-2026-11-01.md).
Mutation class of any fix: orange (store queries).

## Evidence

The paper booted at 16:11:21Z with its derived-key marks trusted (no
validation pass). Its guest cache held flat at about 2.45 GiB and host RSS at
3.3 GiB until 16:21. Telemetry retirement starts 10 minutes after every boot
(`telemetryRetireFirstDelay`, `internal/store/telemetry_retire.go`) and then
every 24 h. It ran 16:21:49 to 16:25:31Z: 24 retire ops of a few hundred to
1,337 rows each. Over the same minutes the firecracker process's rchar rose
from 2.5 GB to 17.1 GB, the guest cache from 1.4 to 7.2 GiB and the host RSS
from 3.1 to 8.1 GiB. About 600 MB read per op.

The 24 ops were real work (slices newly past the 14-day trace and 7-day tool
result windows), not a re-walk.

## Hypothesis (not yet confirmed by a query plan)

Each op's selection carries `retireKeepCitedPredicate`: a `NOT IN` subquery
that runs `json_each` over the `evidence_refs` of every `run.acceptance`
object, and `json_extract` on every candidate row's metadata and body. The
existence probe, the og_objects delete, the choir_events delete and the
partition pass each evaluate it, so the run-acceptance bodies are read
several times per op.

## Directions (not started)

- Materialize the cited event ids once per run (or keep them in a derived-key
  family, as og_key_index does), not once per statement.
- Measure with EXPLAIN QUERY PLAN on a copy of a store, never on the paper.

Until then, a guest memory cap turns this into a daily disk read on the host
instead of 5 GiB of held guest cache.
