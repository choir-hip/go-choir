# Choir RLM Substrate Repairs and G1 Producer — September 5, 2026

**Subject**: Close the named substrate defects, land the constrained G1 actuator producer, and restore Option B proof readiness  
**Status**: Code on `main` at `7574d899`; Node B serving that SHA; remaining gate is owner-scoped `actuator=rlm`  
**Staging**: Retained computer `computer-03335285269bdba4f94377e56879f9e6` (epoch 879, effects OFF, pre-A fence `99949fe2` untouched)  
**Proof scope**: Option B only — in-capsule read-compute-write-assign. Not Definition 3 fleet self-development.

---

## 1. What this is

The RLM target-architecture cutover had working local machinery and one live gate: Node B could not select `actuator=rlm` because the host never rendered `choir.actuator=` into the guest. A first agentic panel then named four code bugs that would have made a sealed proof dishonest even after that gate opened.

Three commits closed the gap. A third consensus panel, plus a supplementary non-Codex panel, then agreed that no further *code* blocker remains for Option B. CI run `33942777266` is green, including Deploy to Staging. `https://choir.news` reports `x-choir-build-commit: 7574d899bcd75b00824040f1684ff33a94ac3f2b`.

What is left is operational, not a patch: an owner-scoped refresh of the retained computer with `actuator=rlm`, three-way readback (cmdline, `get_actuator` / `EffectiveActuator()`, CoSuper overlay), then the sealed in-capsule trajectory with effects OFF.

Mechanical rollback remains `actuator=tools`. Git revert remains available. The pre-A checkpoint is the immutable fence.

---

## 2. What was broken

### Substrate (had to land before any honest proof)

1. **Cursor over-advance.** Reduction treated outbound intent sequences as inbox high-water. A cell that wrote mail could fence unread inbound that arrived after the snapshot.
2. **Non-idempotent reduction.** Retry of a successful cell duplicated channel envelopes.
3. **Split-brain actuator.** `HostSelectsRLM()` read env; the broker resolved cmdline. Guest and host could disagree.
4. **ChannelCast did not wake.** Addressed envelopes persisted and emitted events, but never `actor.Send`. Resident mailboxes slept.

A first repair (`7cf4050b`) closed those four and added the G1 producer. Two further panels found the wake path still incomplete: the actor handler ignored `channel_message`; sequential cells at a held cursor collided on `runID:cursor` + `tray-1`; replay skipped the wake, so a crash after persist and before `actor.Send` lost delivery forever; actor `UpdateID` hashed the envelope body, so two distinct seqs with identical content collapsed to one wake.

### G1 producer

The guest contract already existed (`choir.actuator` cmdline wins, env fallback, fail-closed `tools`). The host never rendered the param, nix never mapped it to `CHOIR_ACTUATOR`, and owner refresh had no field to preserve or update. Live proof could not leave `tools`.

---

## 3. What landed

Three commits on `main`:

| Commit | What it did |
| --- | --- |
| `7cf4050b` | Substrate repairs 1–4 plus G1 Option A (durable `VMConfig.Actuator`, `choir.actuator=` on `runtimeArgs`, nix env map, omit-on-refresh preserves) |
| `bb17d0ef` | Production `channel_message` handler, cell-key content fingerprint, replay skip of duplicate events |
| `7574d899` | Occurrence-scoped actor wakes (`channelID:seq`), destination in reducer keys, replay re-wake, blocked resume pointer, Texture mailbox no-op |

G1 is **Option A, constrained**: the actuator is durable host state rendered every boot, not an in-guest HTTP toggle, and not a `KernelParams` blob that refresh zeros. Explicit `actuator=tools` remains mechanical rollback.

### 3.1 Cursor hold — `internal/agentcore/rlm_reduce.go`

`rlmCallReduction.commit` advances only to the inbox snapshot high-water, floored at `scope.Cursor`. Outbound intent seqs live on the same log and must not become the unread fence.

### 3.2 Reducer idempotency

`CastEnvelope` carries `rlm:{cellID}:{localID}:{sha256(to + content)[:6]}`. `AppendChannelMessage` replays the same key with the same content; same key with different content or destination conflicts. Sequential cells at a held cursor with different bodies or different destinations persist separately. Same body+dest retry returns the original seq.

### 3.3 One EffectiveActuator

`capsule.EffectiveActuator()` is `ReadGuestActuator()` (cmdline wins, env fallback, fail-closed tools). `HostSelectsRLM()` and broker `resolveActuatorRoute()` both call it. The Linux executor always forwards `CHOIR_ACTUATOR=<EffectiveActuator()>`.

### 3.4 ChannelCast → actor.Send

Addressed inserts wake the recipient with kind `channel_message`. Broadcasts do not. The dispatch seed is `channelID:seq`, not the envelope body: two persisted messages with identical content still produce two actor `UpdateID`s. Replay skips the channel event and re-issues the wake; `actor.Send` collapses the same `UpdateID` (`ON CONFLICT DO NOTHING`), so a crash after persist and before the durable actor append recovers on retry without double-executing.

`actorHandler.HandleUpdate` handles `channel_message`:

- Parked/active non-Texture runs resume via `ExecuteActivationSyncChecked` (`request_source=channel_message`).
- No parked run: no-op. Does not mint a run.
- Terminal run: no-op. Does not `ReconcileCoagentWake`.
- Texture mailbox prefix (`texture:`) and `textureRunRecord`: no-op. Texture stays on `coagent_result`.
- `memoryFromRunState` keeps the resume pointer for passivated and still-active (including blocked) runs.

### 3.5 G1 Option A, constrained

- `VMConfig.Actuator` is rendered every boot as `choir.actuator=<ParseActuator>` in microvm `runtimeArgs` and legacy boot args. **Not** in `KernelParams` (`refreshConfigForCurrentDeploy` still zeros that field and does not touch Actuator).
- `nix/autoputer-vm.nix` maps `choir.actuator=*` → `CHOIR_ACTUATOR=`.
- `mergeVMConfigOverrides`: non-empty override parses and sets; empty preserves. Contrast `MaintenanceHold`, which is authoritative per call.
- Owner refresh: `SetActuatorForDesktop` then `RefreshVMForDesktop`. Explicit actuator updates `ownerships.json`; omit preserves. Proxy `HandleComputerLifecycle` refresh body may pass `actuator`. Client: `RefreshDesktopContextWithActuator`. Explicit `tools` is rollback.

---

## 4. Tests

Focused suites used as the verification bar (full-package `agentcore` hits a pre-existing `SQLITE_BUSY` flake and is not the bar):

- `TestCommitAdvancesOnlyInboxHighWater`
- `TestReduceCellIntentsIdempotentReplay`
- `TestReduceCellIntentsSequentialCellsAtSameCursor`
- `TestReduceCellIntentsSequentialCellsDistinctDestinations`
- `TestReduceCellIntentsReplaySkipsEventAndWake` (one event; wake re-issued on replay)
- `TestChannelCastWakesAddressedActor` (`chan-wake:1` then `chan-wake:2` for identical bodies; broadcast silent)
- `TestAppendChannelMessageIdempotentReplay`
- EffectiveActuator / `HostSelectsRLM` / cmdline-vs-env agreement
- VMConfig merge omit/preserve, refreshConfig keeps Actuator, boot args
- `TestOwnershipRegistry_ActuatorPersistsAcrossUnflaggedRefresh`
- `TestHandlerChannelMessageResumesBlockedRun`
- `TestHandlerChannelMessageWithoutParkedRunDoesNotMintRun`
- `TestHandlerChannelMessageIgnoresTerminalRun`
- `TestHandlerChannelMessageIgnoresTextureMailbox`
- `TestChannelMessageUpdateIdentityIsStableAndScoped`
- `TestMemoryFromRunStateRetainsBlockedPointer`

---

## 5. Consensus until convergence

Three convergent panels on this cut, plus a supplementary non-Codex panel after the Codex family hit a usage limit. `.agentic-consensus/` is gitignored session diagnostics.

### Panel 1 — `7cf4050b` (`.agentic-consensus/agentic-consensus-20260904-225024/`)

Split **3 approve / 1 approve-with-changes / 4 block / 1 timeout**.

Unanimous: cursor over-advance closed; HostSelectsRLM/broker split-brain closed; G1 Option A correctly constrained.

Block camp (sol, terra, luna, codex): handler ignored `channel_message`; `runID:cursor`+`tray-1` collided across sequential cells; replay still emitted and re-woke. Claude: CellID collision was the one blocker before Option B.

### Panel 2 — `bb17d0ef` (`.agentic-consensus/agentic-consensus-20260904-232519/`)

| Agent | Verdict |
| --- | --- |
| cursor, grok, gemini38 | approve |
| sol, opencode | approve-with-changes |
| codex, terra, luna | block |
| claude | no verdict (plan-mode stop) |

Remaining named defects, closed in `7574d899`:

1. Content-body actor identity collapsed distinct seqs (lost second wake).
2. Blocked production transitions cleared actor memory.
3. Reducer key omitted destination.
4. Replay skipped wake, so persist-then-crash lost delivery.
5. Texture mailbox prefix unguarded.

### Panel 3 — `7574d899` (`.agentic-consensus/agentic-consensus-20260904-234730/`)

| Agent | Verdict | Notes |
| --- | --- | --- |
| cursor | approve | Four 232519 defects closed; Option B gated on landing-loop + proof procedure |
| grok (`omp-cursor-grok46`) | approve | Claimed tests green locally; no Option B code blocker |
| gemini38 | approve | High confidence (0.95); verified at HEAD |
| opencode | approve | Independent re-run of the named tests; high confidence (0.86) |
| claude | approve-with-changes | All four defects closed; two non-blocking hardening asks (see §6) |
| codex, sol, terra, luna | failed | OpenAI Codex usage limit until 2026-09-05 01:54 AM |

A Codex-family rerun at `.agentic-consensus/agentic-consensus-20260905-000617/` failed the same quota. That is metadata, not dissent.

### Supplementary panel — same SHA (`.agentic-consensus/agentic-consensus-20260905-000659/`)

| Agent | Verdict | Notes |
| --- | --- | --- |
| muse-spark | approve | Traced each claim to source; 11 focused tests green |
| glm-5.3-flash | approve | Same four questions yes; no new defect |
| devin | failed | Non-interactive permission-mode rejected a tool call |
| hy3 | failed | Model not supported |
| nemotron-3-ultra | failed | Empty / hung |

### Convergence

Successful independent verdicts on `7574d899`: **7 approve, 1 approve-with-changes, 0 block**.

The 232519 named code blockers are closed. Claude's two requested changes are real and recorded below; they do not intersect the Option B surface. Partial-panel Codex quota is not a withheld block.

Do not reopen G1 Option A vs ephemeral B. Do not treat Option B as Def 3 fleet self-development. Do not touch pre-A fence `99949fe2`.

---

## 6. Residuals (not Option B blockers)

Carried from earlier panels and still true:

- `ChannelRead` still uses empty ownerID (unscoped `ListChannelMessagesOG`). Safe for the single-owner in-capsule proof; must close before multi-owner exposure.
- Poison-pill DLQ after N retries; a mutated tray at retry writes a new key instead of conflicting.
- 48-bit truncated digest collision is theoretical. Distinct `(to, content)` that collide fail-close as a store conflict.
- Identical body+destination sequential cells at the same cursor still replay (intentional retry collapse, not a per-cell UUID).
- `cgroup.kill` on unreaped workers.
- Proxy lifecycle idempotency commitment is still `computer_id`+`action`+`idempotency_key` (omits actuator).
- First-boot assign path omits Actuator (fail-closed `tools` until explicit write).
- `SetActuator` before a failed refresh can diverge ownership vs guest; retry refresh.
- Fingerprint is a proxy for cell identity: two different cells at the same cursor with byte-identical tray-1 to the same dest still collapse.

Claude's non-blocking asks, verified locally:

1. `(channel_id, seq)` uniqueness is a mutex plus a maxSeq scan. `DispatchWorkerUpdate` (`internal/store/store.go` around 2846) can force a caller-supplied `MessageSeq`. That path is Texture/worker-update, not RLM `CastEnvelope` (`AppendChannelMessage` always assigns `maxSeq+1`). Residual, not Option B.
2. The 10k-object scan window in `AppendChannelMessage` bounds both maxSeq and idempotency replay detection, while `ListChannelMessagesOG` uses 100000. Past 10k messages a replay can miss its own key. Pre-existing; the sealed proof will not approach that scale.
3. `wakeChannelCastRecipient` swallows dispatch errors. Recovery for that path needs a CastEnvelope retry. The persist-before-append crash is the window this cut actually closes.
4. Actor `UpdateID` also hashes live owner/computer resolution. A retry with a different computer fallback would mint a second UpdateID. Same-cell retry on one computer is stable.

---

## 7. Landing receipts

| Gate | Identity |
| --- | --- |
| Source | `main@7574d899` (`7574d899bcd75b00824040f1684ff33a94ac3f2b`) |
| CI `7cf4050b` | run `33940087417` success, including Deploy to Staging |
| CI `bb17d0ef` | run `33941797450` (superseded / cancelled by the next push) |
| CI `7574d899` | run `33942777266` success, including Deploy to Staging |
| Node B | `https://choir.news` `x-choir-build-commit: 7574d899bcd75b00824040f1684ff33a94ac3f2b` at 2026-09-05T04:32:50Z |
| Panel 3 | `.agentic-consensus/agentic-consensus-20260904-234730/` |
| Alt panel | `.agentic-consensus/agentic-consensus-20260905-000659/` |

---

## 8. Now / next

**Code-ready. Not proof-complete.**

**Next action (owner):** owner-scoped refresh of `computer-03335285269bdba4f94377e56879f9e6` with `actuator=rlm`. Prove three-way agreement: guest cmdline `choir.actuator=rlm`, `get_actuator` / `EffectiveActuator()`, CoSuper overlay without ambient JSON tools. Then run the sealed in-capsule read-compute-write-assign trajectory with effects OFF.

Mechanical rollback remains `actuator=tools`. Git revert remains available. The pre-A checkpoint is the immutable fence.

---

## 9. Files

- `internal/agentcore/rlm_reduce.go` — cursor hold, destination-aware keys
- `internal/agentcore/channel_store.go` — CastEnvelope, replay, `channelID:seq` wake
- `internal/store/store.go` — AppendChannelMessage idempotency + `Replayed`
- `internal/types/task.go` — `IdempotencyKey`, `Replayed`
- `internal/capsule/roles.go`, `actuator.go`, `executor.go` — EffectiveActuator
- `cmd/capsule-broker/main.go` — `resolveActuatorRoute`
- `internal/vmmanager/manager.go` — `VMConfig.Actuator`, runtimeArgs, omit-preserve
- `internal/vmctl/ownership.go`, `handlers.go`, `client.go`
- `internal/proxy/computer_lifecycle.go`
- `nix/autoputer-vm.nix`
- `internal/actorruntime/handler.go`, `adapter.go`
- Definition: `docs/definitions/choir-rlm-target-architecture-cutover-2026-09-04.md`
- Design: `docs/designs/rlm-target-architecture-2026-09-04.md`

*Report committed to repository: `docs/reports/choir-rlm-substrate-repairs-and-g1-producer-2026-09-05.md`*  
*Rendered and archived in iCloud Drive Choir Reports: `choir-rlm-substrate-repairs-and-g1-producer-2026-09-05.pdf`*
