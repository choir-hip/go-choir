# Choir data architecture selection (2026-10-28)

```yaml
readiness: drafted
review: none yet (tier: an architectural design is reviewed like the change
  it designs; red store substrate, so Devin + DeepSeek before executable)

start:
  observed_at: 2026-10-28 12:3x UTC, main 0bf89ed84 (staging deployed)
  facts:
    - Three Dolt stores: per-computer embedded (the paper 26.5 GB), corpus
      server (12.6 GiB), platform Store A server (50.9 GiB, 120,335 commits).
      docs/design/three-dolt-stores-2026-10-28.md
    - Dolt versioning is load-bearing nowhere found: Texture history is an
      explicit parent chain of content-addressed rows
      (internal/store/texture.go:1235); restore treats the Dolt head as an
      audit receipt (internal/selfdevprotocol/restore.go:28); no production
      DOLT_BRANCH/CHECKOUT/MERGE or AS OF read. Used: the SQL engine and
      DOLT_HASHOF_TABLE (S3 witness v2).
    - Costs paid for unused versioning: commit-per-write history, Dolt GC
      memory (Store A squash OOM at 10G), online GC holding the write
      connection (81% of the paper's non-200 health samples), compaction,
      two-hour capture with ~72 GiB clone growth.
  dirty_work: none in scope

finish:
  deliver: >-
    A ratified data architecture for Choir, chosen by measurement against
    owner budgets, with a migration plan by shadow projection.
  artifact: >-
    docs/design/data-architecture-decision-<date>.md holding the census, the
    budgets (owner-edited), the candidate set, the bake-off scorecard from
    tape replay, the panel adjudication and the shadow-migration plan;
    plus the replay bake-off harness in the repo as a permanent storage
    regression test.
  acceptance: >-
    The owner ratifies the decision; the scorecard is reproducible from the
    harness on the paper's captured base and tape.
  rollback: docs and an offline harness only; nothing live changes.
  landing: harness code follows the landing loop; no live store changes in
    this goal.

value:
  better_means: >-
    Minimize per-computer memory, disk, write amplification and checkpoint
    cost, and reader blocking, at equal or better durability, security and
    witnessability, measured on the real tape.
  goodharting_would_be: >-
    Choosing by a synthetic benchmark or by preference; or a candidate that
    wins the bake-off by dropping durability, the witness or per-owner
    isolation.

homotopy:
  realism_axis: >-
    Census (read-only) -> budgets -> candidates on paper -> replay of a
    d@e tape -> replay of the paper's 4.5M-event tape -> shadow projector
    on d@e (a later goal).

boundaries:
  mutation_class: green/yellow here (docs, read-only measurement, offline
    harness); the migration it plans is red and is its own goal.
  invariants:
    - the tape is never modified; candidates are projectors over it
    - no SQL against production stores beyond owner-approved read-only
      census queries; census runs on captured bases where possible
    - protected computers are never used for adversarial steps

conjecture:
  claim: >-
    Most of the stores' bytes and load are immutable content duplicated
    from the tape or the corpus, and a non-versioned engine plus a
    content-addressed store meets every budget with less memory and disk.
  test: the census (lifecycle classes by bytes) and the replay bake-off.
  edge: missing_oracle (no per-table census exists yet)
  delta_o: the census itself
  scope: Node B workloads as of 2026-10

now:
  status: drafted
  slice: 1, the census
  next_action: >-
    Per-table bytes, rows, write and read rates, query shapes and lifecycle
    class for the paper (captured base 5a241cb0), Store A and the corpus;
    then a budget draft for the owner to edit (memory and disk per
    computer, checkpoint and restore time, kill -9 durability, no reader
    blocking, no GC pause on the serving path, per-owner encryption and
    crypto-erase, witnessable state hash, no cgo if wanted).

plan:
  1: census (read-only)
  2: budgets (owner edits)
  3: candidates, divergent panel (SQLite WAL; CAS + SQLite; Postgres per
     host; bbolt/LMDB + own indexes; append logs + in-memory indexes; Dolt
     small with history off); cut on paper against the budgets
  4: replay bake-off harness (the rebuilder with a pluggable projector);
     scorecard on d@e then the paper's tape; kill -9 drill per candidate
  5: convergent panel on the scorecard; owner ratifies
  6: migration goal: shadow projector on d@e fed from the tape,
     differential answers and witnesses, widen by ring, reads last;
     rollback = drop the shadow
```
