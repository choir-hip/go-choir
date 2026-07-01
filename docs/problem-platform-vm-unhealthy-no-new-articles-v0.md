# Problem: Platform VM Unhealthy — No New Articles Since June 27

**Date:** 2026-06-30
**Status:** active paradoc
**Discovery method:** staging evidence via CLI + SSH to Node B

## Problem Statement

The Universal Wire platform VM (`vm-universal-wire-platform`) is in a
continuous unhealthy state. The VM boots and the runtime starts (`runtime:
started (sandbox=vm-universal-wire-platform)`, `sandbox: starting server
on 0.0.0.0:8085`) but immediately fails health checks from the vmmanager.
Health checks fail every 15 seconds with no recovery. The VM has been
rebooted 1048 times (epoch=1048) and is stuck in a crash loop.

**Consequence:** No new articles have been synthesized since June 27
(3 days). The 41 stories on the wire are all old, single-source rewrites
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
- `internal/vmctl/platform_computer.go` — platform VM ownership
- `cmd/sourcecycled/main.go` — sourcecycled dispatch logic
- `nix/node-b.nix` — VM and service configuration
