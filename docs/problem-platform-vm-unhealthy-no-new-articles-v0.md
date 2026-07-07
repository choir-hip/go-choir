# Problem: World Wire VM Unhealthy — No New Articles Since June 27

**Date:** 2026-06-30 (updated 2026-07-07)
**Status:** active paradoc; escalated to substrate-symptom cluster
**Discovery method:** staging evidence via CLI + SSH to Node B

## Problem Statement

The World Wire platform VM (`vm-universal-wire-platform`, formerly
"Universal Wire") is in a continuous unhealthy state. The VM boots and the
runtime starts (`runtime: started (sandbox=vm-universal-wire-platform)`,
`sandbox: starting server on 0.0.0.0:8085`) but immediately fails health
checks from the vmmanager. Health checks fail every 15 seconds with no
recovery. The VM has been rebooted 1048+ times (epoch=1048) and is stuck in
a crash loop.

As of 2026-07-07, the wire is still down: `choir wire stories` returns
`context deadline exceeded` against `https://choir.news/api/universal-wire/stories`.
The platform VM has been in `recovery_failed` state since July 3.

**Consequence:** No new articles have been synthesized since June 27
(10+ days). The 41 stories on the wire are all old, single-source rewrites
from June 26-27. sourcecycled is running and fetching new items (22 RSS,
25 Telegram per cycle) but:
- Processor dispatch is stuck: `processor_submitted=1
  processor_skipped=61` per drain — only 1 item processed per 5-minute
  cycle, and the processor runs can't complete because the platform VM
  is unresponsive.
- Web captures fail: `context deadline exceeded` talking to
  `http://unix/internal/vmctl/sandbox-proxy/universal-wire-platform/...`
- The articles that do exist are low-quality single-source rewrites, not
  multi-source syntheses of important current events.

## Evidence

1. **CLI proof:** `choir wire stories` returns 41 stories, all dated
   2026-06-26/27, all `source_state: universal-wire-edition-texture`.
   Content is single-source rewrites ("A further update adds detail on
   telegram post from metropoles telegram. [3]... [4]... [5]...").
2. **sourcecycled logs:** `processor_submitted=1 processor_skipped=61`
   consistently. Web captures fail with context deadline exceeded.
3. **vmctl logs:** `vmmanager: health check failed for VM
   vm-universal-wire-platform at http://10.201.73.2:8085` every 15
   seconds. VM was booted at epoch=1048 but immediately went unhealthy.
4. **VM serial console:** Runtime starts successfully (`runtime: started`,
   `sandbox: starting server on 0.0.0.0:8085`) but then becomes
   unresponsive. Qdrant connection refused (best-effort, non-fatal).
5. **Direct health check:** `curl http://10.201.73.2:8085/health` times
   out — the VM is not responding on its HTTP port.
6. **July 7 confirmation:** `choir wire stories` returns
   `context deadline exceeded` against `https://choir.news/api/universal-wire/stories`.
   The wire has been down for 10+ days. The platform VM has been in
   `recovery_failed` state since July 3.
7. **Codebase verification (2026-07-07):** The VM-embedded Dolt and corpusd
   platform Dolt are two separate databases. The sync path (HTTP POST
   `/internal/platform/texture/sync`) transfers only texture document
   metadata and revision content — no runtime state. 37 tables exist in
   the VM-embedded store; only 2 are synced to corpusd. The host-side
   corpusd is healthy (640 artifact manifests, 640 blobs, 103 texture
   documents). The failure is isolated to the VM-embedded Dolt workspace.

## Root Cause Hypotheses

- **H1 (VM runtime crash after start):** The sandbox service starts but
  crashes during initialization (e.g., loading the Dolt database, which
  is 223 GiB on the host). No crash logs visible in the serial console,
  but the service stops responding immediately after starting.
- **H2 (Network routing):** The VM boots and the service starts, but the
  network path between the host and the VM's tap device is broken. The
  health check can't reach the service.
- **H3 (Resource exhaustion):** The VM doesn't have enough memory/CPU to
  run the sandbox service, causing it to hang or crash silently.
- **H4 (Disk/database corruption):** The VM's Dolt database is corrupted
  or too large to load, causing the service to hang during startup.

## Next Probes

1. Check the VM's resource limits (memory, CPU) in the vmctl configuration
2. Try to restart the platform VM and watch the serial console for errors
   after the "starting server" log line
3. Check if the host sandbox service can serve as a fallback for the
   platform computer (bypassing the VM)
4. Check the VM's disk usage and Dolt database size
5. Check the health check timeout — it might be too short for the VM's
  startup time

## Substrate vs Symptom Classification

**Substrate:** The platform VM lifecycle (vmctl/vmmanager) — the VM
boots but can't keep its sandbox service healthy. This is a foundational
layer issue.

**Symptom:** The lack of new articles on the wire is a symptom of the
platform VM being down. The edition bootstrap fix (Track B) made existing
articles visible, but the synthesis pipeline can't produce new ones
because the runtime that runs it is on the dead VM.

## Root Cause Clustering (added 2026-07-07)

This is the **second** embedded-Dolt boot failure in one month on the same
substrate:

1. **June 9, 2026** — Operator VM boot failure. Guest disk hit 100%.
   `state.vtext/` was 2.5G with suspected Dolt compaction debt. Recovery
   pruned caches but **left the 2.5G Dolt in place**. See
   `docs/archive/incident-vm-bootstrap-stale-route-2026-06-09.md`.
2. **June 30 / July 3, 2026** — World Wire platform VM boot failure.
   Runtime starts but immediately becomes unresponsive. H4 (Dolt
   corruption/bloat) is the leading hypothesis. VM entered
   `recovery_failed` on July 3.

Per AGENTS.md Root Cause Clustering: 2+ Dolt boot failures in one month
on the same substrate (embedded Dolt inside Firecracker VM with opaque
`data.img`) means the next action is substrate-level, not symptom-level.

**Substrate-level assessment:**

The embedded Dolt inside the VM is the substrate problem. The VM-embedded
Dolt and corpusd platform Dolt are two separate databases stitched by an
app-level HTTP POST that syncs only texture documents (verified 2026-07-07).
The VM's embedded Dolt carries 37 tables (26 runtime + 11 texture) with no
version-control features in use (no branches, merges, AS OF, or history
queries). It is used as MySQL + per-write commit markers in the platform
store only; the VM-embedded store uses no DOLT_COMMIT at all.

The host-side corpusd platform Dolt is healthy (640 artifact manifests, 640
blobs, 103 texture documents). The failure is isolated to the VM-embedded
Dolt workspace.

**Heresy alignment:** The proxy wire route hard-codes
owner="universal-wire-platform", desktop="platform" → vmctl VM resolve
(`internal/proxy/handlers.go:970`). This is H031 (candidate-computer-as-VM)
expressing itself — a route pointing at a VM identity, exactly what
route-over-computer-version forbids. The 180s hang was this heresy
expressing itself (per `docs/mission-og-dolt-heresy-hard-cutover-v0.md`
Phase 4b).

**Recovery direction:** Rebuild the VM's embedded Dolt from corpusd + typed
inputs, not by patching the corrupted workspace. The corrupted embedded
workspace is superseded by (corpusd head + code ref + blob root). Do not
`e2fsck` a third time. This is the deletion-first heuristic applied to a
database.

However, per I7 (problem documentation first), this problem doc is the
first commit. The fix commit(s) come second, referencing this documentation.

**SIAC alignment:** This failure is the first staging proof target for
SIAC Gate 7. The World Wire service must serve from a materialized computer
whose route resolves to ComputerVersion, not to a VM identity. The fix
converges with OG/Dolt Phase 4b (candidate-VM residue elimination) and
the H031 heresy eradication.

## Impact on Mission Conjectures

- **C5 (News fix):** NOT settled. The edition bootstrap made stories
  appear, but the synthesis pipeline is broken because the platform VM
  is down. The stories that appear are old single-source rewrites, not
  new multi-source syntheses. Reverting C5 to undecided.
- The Track B fix (edition alias bootstrap) is correct but insufficient —
  it makes existing articles visible but doesn't fix the synthesis
  pipeline that creates new ones.

## References

- `docs/mission-news-live-pr-merge-model-default-v0.md` — parent mission
- `docs/problem-universal-wire-edition-alias-not-bootstrapped-v0.md` —
  the edition alias problem (fixed, but insufficient)
- `docs/archive/incident-vm-bootstrap-stale-route-2026-06-09.md` —
  June 9 operator VM boot failure (first in the Dolt boot-failure cluster)
- `docs/definitions/substrate-independent-audited-computer-2026-07-04.md` —
  SIAC definition (Gate 7 staging proof target)
- `docs/definitions/heresy-eradication-2026-07-07.md` — H031 heresy
  (candidate-computer-as-VM; the proxy route is this heresy expressing itself)
- `docs/mission-og-dolt-heresy-hard-cutover-v0.md` — Phase 4b
  (candidate-VM residue elimination; wire fix converges with route-over-version)
- `internal/vmctl/platform_computer.go` — platform VM ownership
- `internal/proxy/handlers.go:970` — proxy route hard-coded to VM identity
- `cmd/sourcecycled/main.go` — sourcecycled dispatch logic
- `nix/node-b.nix` — VM and service configuration
