# Problem receipt: texture-delivery reconcile CommandID under-specifies identity → runtime startup crash loop

**Discovered:** 2026-09-30, while driving the M1 typed-commitments deployed proof.
**Severity:** P0 — the owner computer (`computer-03335285269bdba4f94377e56879f9e6`)
is in a ~60-second runtime crash-respawn loop; no desk cell can fire.
**Mutation class:** red (canonical lifecycle command identity).
**Supersedes/confirms:** the M0 residual (`m0-residual-texture-runtime-restart-2026-09-30.md`)
— the earlier "passivated, never re-woken" stall and this crash loop are the same
wedge class surfaced at different reconcile orders.

## Symptom

`autoputer-runtime` respawns every ~60–70s on node-b. Each boot:
`boot phase begin passivate_interrupted_activations` → passivates a pending run
(`runtime_restarted`) → completes boot phases → logs `runtime: started` → ~50s later:
`autoputer: runtime startup refused: actorruntime: reconcile Texture owner: reconcile
texture update delivery: lifecycle command digest conflict` → process exits →
supervisor respawns → repeat.

Observed PIDs cycling 55205→60662→66118→71587→77064; passivated runs
`b758ef21…`, `c9e4836a…` re-passivated each boot. Host journal
(`go-choir-vmctl-exec` framing, `[NNN]` host uptime). The M1 proof's three
prompt-bar runs each created a texture activation then passivated
`runtime_restarted` — because the runtime under them kept dying.

## Mechanism

`Handler.reconcileAgentWakeLocked` (internal/textureowner/texture_controller.go:824,
833, 842, 887) calls `reconcileTextureUpdateDelivery` on every reconcile pass,
including boot. That function builds:

- `req.CommandID = "texture-delivery:" + trajectoryID + ":" + textureAgentID + ":" + commandKey`
  where `commandKey = "bind:<targetRunID>"` or `"exhaust:<breakerRunID>:<streak>"`
  (texture_controller.go:1010-1015). **Target-slot derived — stable across boots.**
- `req.CommandDigest = ComputeReconcileUpdateDeliveryDigest(req)` — covers the full
  request, including the recomputed `Items` set (armedUpdates + exhausted)
  (texture_controller.go:1020). **Content-derived — drifts across boots** as
  packets arrive/resolve.

`Store.ReconcileUpdateDelivery` → `replayLifecycleCommand`
(internal/store/lifecycle.go:1000): if a stored `choir.lifecycle_command` receipt
exists for `commandID` with a *different* `CommandDigest`, it returns
`ErrLifecycleCommandConflict`. The reconcile propagates it as fatal
(`return nil, bindErr` at the boot-dispatch paths), `actorruntime` reconciles
Texture owner returns the error, autoputer logs `startup refused`, exits.

So: when the armed/exhausted item set differs between two boots while the target
slot (`bind:<runID>` / `exhaust:<run>:<streak>`) is unchanged, the same CommandID
is re-presented with a new digest → conflict → fatal → loop. The receipt from the
prior boot is permanent; the wedge persists until the item set happens to return
byte-identical (unlikely as runs keep arriving).

## Root cause

CommandID is the idempotency key and *is* the command identity. A reconcile whose
request content can legitimately vary must key the command on the content, not
only on the target slot. `reactivate-run` already embeds `rec.UpdatedAt`
(runtime.go:1636) for exactly this reason; `texture-delivery` keyed only on the
slot. Under-specified command identity.

## Fix direction

Make the `texture-delivery` CommandID content-derived: append the computed digest
(or a hash of the items) so identical request content → identical CommandID →
clean replay dedup, and any drifted content → a fresh CommandID → fresh receipt
→ no conflict. The stale receipt then stays orphaned but inert — its CommandID is
never re-emitted — so no data surgery is needed and the wedge clears on the next
boot after the fix deploys.

Preserves: idempotent-retry dedup (same request replays to the stored result),
tray atomicity, the digest-conflict guard for genuinely-mutated commands.

## Evidence

- Host journal (node-b, ~05:49–06:01): repeated
  `runtime startup refused: … reconcile texture update delivery: lifecycle
  command digest conflict` then PID respawn.
- `docs/reports/m0-debug-stabilize-restart-rearm-2026-09-30.md` §"Three failures"
  — the same wedge class from the texture leg.
- Internal: `internal/textureowner/texture_controller.go:824-890, 983-1025`;
  `internal/store/lifecycle.go:988-1054`; `internal/store/lifecycle_update_delivery.go:27`.
