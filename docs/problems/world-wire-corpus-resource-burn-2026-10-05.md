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
