# RLM Option B actuator refresh — 2026-09-05

**Boundary:** execute (owner-authorized refresh of the retained computer). Not sealed-proof complete.
**Parent:** `docs/definitions/choir-rlm-target-architecture-cutover-2026-09-04.md`
**Host:** `https://choir.news` `x-choir-build-commit` `7574d899bcd75b00824040f1684ff33a94ac3f2b`
**Computer:** `computer-03335285269bdba4f94377e56879f9e6`

## Actuation

Owner authorized SSH to `node-b`. Refresh itself used the product path (G1 owner-scoped actuator write), not a hand edit of `ownerships.json`.

```
POST /api/computers/computer-03335285269bdba4f94377e56879f9e6/lifecycle/refresh
{"idempotency_key":"rlm-actuator-cutover-2026-09-05T1405Z","actuator":"rlm"}
```

LifecycleReceipt `01a071e3-e430-7073-b95f-9bddd7f0e74d`: epoch **879 → 880**, `prior_lifecycle_state=active` → `resulting_lifecycle_state=active`, HTTP 201 in 19.1s.

Effects mode unchanged: `propose_only` generation 1. Pre-A fence `99949fe2` not touched. Mechanical rollback remains `actuator=tools`.

## Three-way readback (partial)

Observed on Node B after refresh (`vm_id` `candidate-fleet-e15cb89f25d963c220319b7b`, guest `http://10.200.13.2:8085`):

| Leg | Result |
| --- | --- |
| Durable ownership | `/var/lib/go-choir/vm-state/ownerships.json` `actuator=rlm`, epoch 880 |
| Guest cmdline | `fc-config.json` `boot_args` contains `choir.actuator=rlm` and `choir.realization_id=...-epoch-880` |
| Guest identity | `/health` `status=ready`, `deployed_commit=7574d899bcd75b00824040f1684ff33a94ac3f2b` (`deployed_at` 2026-09-05T14:05:43Z). Pre-refresh guest was still `8c410a0d` at epoch 879 with **no** `choir.actuator=` (fail-closed tools) |
| `get_actuator` / CoSuper overlay | **not observed.** No CoSuper capsule ran. Overlay/schema is derived in-guest from `HostSelectsRLM()` at CoSuper start. |

Nix still maps `choir.actuator=*` → `CHOIR_ACTUATOR` in `/run/go-choir-autoputer.env`. That mapping is mechanical given the live cmdline; it is not a live `get_actuator` RPC receipt.

## Option B attempt (not complete)

Assignment vehicle used: Texture create + tell, then prompt-bar as a second product path.

| Artifact | Identity |
| --- | --- |
| Texture doc (create+tell) | `d599c4b1-a265-5545-b073-9fb7b51d5ce5` |
| Trajectory | `6c835b61-8743-51db-9996-64d39ef77ff1` |
| Texture run | `4025ccc5-e2ed-4efe-9b62-3291171e9825` — first turn committed rev `b7640fe6-...`; later resume **failed**: `tool loop: required write tool did not succeed after 2 retries` |
| Super work item (still open) | `a4a7fc58-5876-41f0-8d14-c370200ea52c` |
| Pending Super control | update `f8eb36b5-236d-4db4-aaa1-3bc6b5dcc583` disposition `pending` |
| Prompt-bar doc | `72e7f345-bb8f-5cb4-9ecb-d67fe7c69216` trajectory `0de03b4f-3733-5d80-a917-0e931d711f1c` |
| Prompt-bar Super work (still open) | `f91588f0-56ca-4ef4-b1dd-f5ff0c1e4ec7` pending update `d7822d0d-cb20-4689-a151-6722b5559833` |

Two Super runs started after those control queues, but each bound a **different** trajectory's worker update (injected `worker_update_ids` were not the Option B controls):

1. Super `ee8b34b6-3ff7-49bd-a14c-cc9f3cbe57a0` bound `texture:e4d4ab5c-...` / trajectory `bb5b3544-...` / update `054cc420-...`. Result: inspection blocked (no `texture_span`).
2. Super `91f96e20-8334-47c6-9063-033e805e76f1` bound `texture:8d106704-...` / trajectory `3fa254bc-...` / update `00fb0042-...`. Result: CoSuper preflight failed: **`/mnt/persistent/files/Source/platform` does not exist**. No capsule opened.

That Source path is `CHOIR_CAPSULE_SOURCE_ROOT` (`nix/autoputer-vm.nix`). `assign_co_super` always calls `PreflightSourceSnapshot` on it, including for this in-capsule Option B cell. Product `/api/files` lists only `System/`, `mail/`, and `self-development-supervision.texture`. Guest persist is otherwise healthy (`used_bytes` ~3.8GiB of ~31GiB). Pre-upgrade `data.img.*` copies exist on the host and were not mounted or restored.

Mailbox scale (first 15 `/api/trajectories` rows, 2026-09-05T14:26Z): hundreds of still-pending Super controls on older Texture docs (examples: `d8ccd11b` 112 pending, `24693e87` 187, `5242ca03` 127, `91492b4e` 269, `3fa254bc` 175). FIFO Super activation binds the oldest global pending control, not the Texture-opened Option B work item. Draining that mailbox would consume unrelated owner Texture work and cannot be the proof path.

## What this does and does not prove

**Proved:** owner-scoped `actuator=rlm` refresh works. The retained guest booted epoch 880 on `7574d899` with `choir.actuator=rlm` on the kernel command line, matching durable ownership.

**Not proved:** live `get_actuator` route=rlm, sealed CoSuper overlay (no `capsule_exec`/`capsule_read_file`/`capsule_write_file`/`capsule_list_dir`), or in-capsule read-compute-write-assign. Super did not consume the Option B control packets. A later Super on this computer refused an implementation assignment because `Source/platform` is missing. Both the Super-mailbox targeting gap and the missing source tree independently block Option B on this computer.

## Next

Do not drain the Super mailbox and do not restore `data.img.pre-*`. Remaining gates, in order:

1. Populate `/mnt/persistent/files/Source/platform` as a clean git tree of the current deployed commit (not a pre-A restore) so `assign_co_super` preflight can succeed. This is a guest workspace repair, not the sealed cell.
2. Make Super activation bind the Texture-queued control that woke it, rather than the oldest global pending Super packet.
3. Then live `get_actuator` + sealed overlay + one in-capsule read-compute-write-assign with effects `propose_only`.

Rollback remains `actuator=tools`. Fence `99949fe2` stays untouched.
