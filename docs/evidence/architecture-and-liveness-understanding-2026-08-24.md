# Architecture and Liveness Understanding — 2026-08-24

## 1. The Core Architecture: Boot, Liveness, and Differential Replay

### Intended Architecture
1. **Live State is Kept Online**: In normal operation, a user's computer VM stays active and warm. When a user opens the UI (e.g. `choir.news`) or issues a CLI command, requests are routed directly to the running in-memory/in-store runtime on the guest. **No VM boot, no replay, and no state reconstruction occur during active UI/CLI usage.**
2. **Boot Semantics ($O(\Delta)$, not $O(\text{history})$)**:
   - A VM only boots on cold start (first start or after an intentional stop/sleep).
   - On boot, the runtime inspects its local Dolt projection head (`localHead = a.projection.Head(...)`).
   - It queries the platform tape for events after `localHead.Sequence`.
   - **If $\Delta = 0$ (current disk)**: The query returns 0 events (`len(page) == 0`). `reconstruct()` returns `nil` in ~10–20ms. The runtime opens immediately with zero replay delay.
   - **If $\Delta > 0$ (events arrived while stopped)**: The guest replays *only* the new events since its last checkpoint to catch up to the platform head.
3. **Emergency Recovery vs Normal Boot**:
   - The full tape rebuild from event 0 to head (132,539 events) executed on 2026-08-23/24 was a one-off rescue operation because the stopped computer's disk had been corrupted and had no valid base. It was never intended as the standard boot path.
   - The upcoming **Durable Substrate Overhauls (Track F)** introduces periodic `ProjectionBase` Merkle watermarks and content-addressed File-CAS, formally guaranteeing that full disaster recovery is bounded by $O(\Delta)$ rather than $O(\text{history})$.

---

## 2. Root Cause of the Live Computer `0333528` Restart Loop

During investigation on 2026-08-24, live logs from `computer-03335285269bdba4f94377e56879f9e6` (`candidate-fleet-e15cb89f…`) revealed why all apps (Texture, Files, Podcast, Calendar) returned HTTP 502:

### Exact Sequence Observed in Guest Console
1. VM boots in ~7 seconds.
2. Local Dolt store opens in ~4 seconds (`fresh=false`, 11 GiB store).
3. Delta replay completes in ~1 second (`autoputer: computer event authority reconstructed (replay complete)`).
4. Runtime server binds port 8085.
5. `a.Runtime.Start(ctx)` runs and passivates stale lifecycle runs from prior sessions:
   `runtime: passivated stale lifecycle run fec9df18-dd1e-5941-965c-6f6d628d1492 before restart dispatch`
6. `a.textureOwner.Start(ctx)` runs immediately after (`internal/textureowner/texture_controller.go:108-115`).
   - It loads the document's active run from `subject.ActiveRunID`, which points to `fec9df18-dd1e-5941-965c-6f6d628d1492`.
   - It evaluates `if run.State.Terminal()`.
   - Because `fec9df18` was just passivated, `run.State.Terminal()` evaluates to **true**.
   - It fails with:
     `autoputer: runtime startup refused: actorruntime: reconcile Texture owner: boot Texture run fec9df18-dd1e-5941-965c-6f6d628d1492 is not exact canonical authority`
7. `autoputer/run.go` executes `log.Fatalf(...)`, killing the process.
8. Systemd restarts the service; the exact same error recurs; `vmctl` times out waiting for port 8085 to stay healthy and reboots the Firecracker VM.

### Resolution
In `internal/textureowner/texture_controller.go`:
When `subject.ActiveRunID` points to a terminal/passivated run, `texture_controller` must not treat it as a fatal unrecoverable authority error. A passivated run represents completed or superseded work; the controller should clear/bypass the stale candidate and allow startup reconciliation to proceed cleanly.
