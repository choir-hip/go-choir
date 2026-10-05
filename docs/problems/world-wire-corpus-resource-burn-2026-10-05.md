# World Wire corpus: 99 GB store, 15-day CPU burn, dead processing pipeline

**Status:** documented for director-agent triage. Not fixed.
**Date:** 2026-10-05
**Context:** surfaced while diagnosing S0 ops-substrate resource anomalies; owner
confirmed corpus-dolt is where sourcecycled output lands and that WW
rearchitecture needs deep consideration — this doc is the evidence package.

## Evidence (all observed on Node B, 2026-10-05)

### corpus-dolt (`go-choir-corpus-dolt.service`, Store B)

- `dolt sql-server` PID 2661900, elapsed **15d 21h**, RSS **10.2 GB**, host store
  `/var/lib/go-choir/corpus-dolt` = **99 GB**.
- Sustained CPU: ~112 jiffies / 5s ≈ >1 core continuous, `top` avg 34–52%.
  `utime=64530800 stime=8157215` — ~8.2 CPU-days. This is steady-state burn, not
  a crash loop: only routine query log lines (`nothing to commit` warnings).
- Cgroup `memory.pressure`: some/full total counters large but current averages
  near zero — it is not currently thrashing, it is *steadily* busy.
- Database `corpus` is entirely World Wire schema: `sources` (211 rows),
  `cycles` (34,802), `fetches` (3,279,910), `items` (2,329,949),
  `ingestion_events` (4,246,438), `og_objects` (**8,562,794**),
  `processor_requests`, `reconciler_requests`, `publications`,
  `publication_*` (9 tables), `provenance_*` (4), `citation_edges`, etc.
- Per `computer-ontology.md` "Dolt Store Taxonomy", this same sql-server also
  carries narrow platform control tables (`computer_event_heads`, lifecycle,
  route slots) "beside" WW objects — **the platform event-head CAS authority
  fate-shares with a 99 GB WW ingestion store.** This is the entanglement that
  makes the burn a platform problem, not just a WW problem.

### sourcecycled (`go-choir-sourcecycled.service`)

- Was `active`, ~99 MB RSS, ~2 RSS cycles / 10 min — normal cadence, NOT a
  doom loop. Its 211 sources produce routine fetch churn: 14 distinct error
  types / 10 min (403s — bot-blocked publishers; 429s; parse failures).
- **The downstream pipeline is dead**: `processor_requests` status census:
  `superseded: 208,320 · dispatch_failed: 2,582 · submitted: 181 · queued: 70 ·
  completed: 127`. 83 `transient runtime unavailable: 502 Bad Gateway`
  ingestion-handoff errors in a single 30-min window. Fetches accumulate;
  almost nothing processes. The fetch loop was pure resource debt.
- **Interim action taken (logged, reversible):** `systemctl stop
  go-choir-sourcecycled` at ~01:15Z 2026-10-05. Stops fetch accumulation while
  the processor path is dead. Rollback: `systemctl start`. NOTE: a host deploy
  (`switch-to-configuration`) will restart it — durable disable requires a
  `nix/node-b.nix` change (deferred to director).

### What it is not

- The CPU spike seen alongside (`nix build`, `go mod vendor`, `nix store gc`)
  was the in-flight CI deploy + Nix store GC — unrelated to WW.
- `corpusd` (the Go service) itself is not the burner — the burn is inside the
  `dolt sql-server` process corpusd fronts. corpusd restarted cleanly during
  the f20f3a51 deploy.

## Open questions for the director

1. What drives dolt's sustained >1-core CPU with ingestion running? Candidates:
   noms-level compaction/GC churn on a bloated store; a hot query loop from
   corpusd source-API handlers serving choir.news; the dead-but-retrying
   processor/reconciler dispatch path. Needs `dolt sql` processlist + a few
   minutes of query sampling — host lacks tooling (see tools doc).
2. How much of 99 GB is WW objects vs platform control tables vs un-collected
   garbage? `og_objects` at 8.5M rows dominates; whether its `.bin` bodies in
   `platform-artifacts/sha256/og/` (~421K files, 813 MB) are live is unresolved
   (see GC hazard below).
3. Is `sourcecycled`'s fetch set worth preserving at all, or does the
   rearchitecture replace source ingestion wholesale?

## Hazard discovered: platform artifact-GC vs WW og blobs

The S0-1 `internal/platform/artifact_gc.go` sweeps `sha256/og/` by
`og_objects.body_ref` liveness. Dry-run classified **421,167 of 421,370 og
files as unreachable → 813 MB "deletable"**, because only ~203 `og_objects`
rows have a non-empty `body_ref`. If the WW corpus uses inline `body` storage
(below `ogBodyExternalizeMinBytes`) this is correct-but-scary; if `body_ref`
population is buggy this deletes live article/claim bodies. **Do not run the
`og` namespace in active mode until a director-level decision classifies WW
corpus data as keep/archive/delete.** Other namespaces (file-cas-chunks,
platform-update, projection-base) are unaffected — 39.2 GB dry-run reclaimable
there, almost all file-cas-chunks.

## Related

- S0-1 storage lifecycle: docs/problems/s0-storage-lifecycle-gaps-2026-10-05.md
- GC live-set code: internal/platform/artifact_gc.go:150-213
- Rearchitecture analysis: docs/world-wire-rearchitecture-2026-10-05.md
- WW stack: docs/world-wire-mission-stack-2026-09-22.md (Phase 5)

## Director review (2026-10-05) — corrections, one confirmed bug, decisions

Source-checked against `main@ee013c96`. Nothing here was re-observed on Node B.

### Correction 1 — the event-head CAS does NOT share corpus-dolt's process

There is a deployed Store A / Store B split:
- `nix/node-b.nix:484-488`: corpus-dolt is "separate process from the
  canonical event/control store (Store A, port 13306) so a corpus GC-OOM or
  bulk scan cannot take the canonical log down".
- corpusd's primary DSN is `127.0.0.1:13306/platform` (`node-b.nix:539`).
- `computer_event_heads` is created on that platform pool
  (`internal/platform/computer_events.go:18`).
- The WW tables ride `Store.corpusDB` when `corpus-dsn.env` is present
  (`internal/platform/store.go:441-470`).
- The observed `corpus` database listing contains only WW tables, which is
  consistent with this.

The remaining coupling is at the **corpusd process** level: one Go service
fronts both pools, so a corpus-side stall can still slow corpusd handlers.
There is no sql-server or CAS fate-sharing. The misleading sentence is the
pre-split wording in `docs/computer-ontology.md` "Dolt Store Taxonomy",
corrected in the same commit as this review. Severity is downgraded from
"platform authority entangled" to "corpusd process shared".

### Correction 2 — the host does have dolt and jq

`environment.systemPackages` includes `dolt` and `jq` (`nix/node-b.nix`,
around line 1063). If they were missing from an agent shell, that is a PATH
or session issue (`/run/current-system/sw/bin`), not missing packages. Only
sysstat, iotop and e2fsprogs-on-PATH remain to add.

### Confirmed bug — artifact GC reads og liveness from the wrong store

- og bodies are written and read through `o.store.corpus()` — Store B
  (`internal/platform/objectgraph_store.go:101,132`).
- The GC's og liveness query uses `s.store.db` — Store A
  (`internal/platform/artifact_gc.go:202`).
- Store A's `og_objects` holds only pre-split leftovers, hence about 203
  refs against 421k files.

So the "421,167 deletable" figure is a wrong-database artifact, not a data
classification question. **Active og GC would delete live WW bodies.** Fix:
query `s.store.corpus()` for og liveness (and audit every other liveness
query for its owning store), add a test with a split two-pool store, and
keep `og` in dry-run until the fixed dry-run shows a plausible live set.
Owner: SO storage lifecycle (orange; the sweep deletes data).

### CPU burn — one discriminating observation

`sourcecycled` (the only fetch, processor and reconciler dispatcher) has
been stopped since about 01:15Z.
- Sample corpus-dolt CPU now, then run `dolt sql -q "show processlist"`
  against :13307 for a few minutes.
- **Burn gone:** it was ingestion writes and commits on 8.5M-row tables
  (corpus `doltbatch` commits). The stop already fixes it for now.
- **Burn persists:** it is the read side (choir.news publication and source
  API queries via corpusd) or Dolt background work on the bloated store.
  Then sample queries before changing anything.

### Decisions (director, reversible, under continuous authority)

1. **sourcecycled stays off, durably.** Gate the unit behind an option that
   defaults to disabled in `nix/node-b.nix`, so a deploy cannot restart it.
   The processor path is dead and every fetch is debt. Re-enable only as
   part of the WW rearchitecture. Owner may override.
2. **WW corpus data class = frozen and retained.** No deletion and no
   migration. Treat it as first-attempt residue to be read by the
   rearchitecture. og GC stays dry-run until the bug above is fixed, and
   even then WW bodies are excluded until the rearchitecture classifies
   them.
3. **No corpus-dolt GC or compaction yet.** A 99 GB store needs comparable
   scratch space Node B does not have. Revisit after SO's headroom work.
4. **The rearchitecture is not started now.** It becomes the consuming
   application of this metamission (see the metamission "World Wire as the
   consuming application" section). Containment items 1-3 and the GC fix
   ride SO.
