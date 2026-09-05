# RLM Option B exact-bind and go_eval attempts — 2026-09-05T18:48Z

**Boundary:** execute on staging. Not sealed-proof complete.
**Parent:** `docs/definitions/choir-rlm-target-architecture-cutover-2026-09-04.md`
**Host:** `https://choir.news` `x-choir-build-commit` `3724db1abb9c1538dfa8f32c0974102814c00df1`
**Computer:** `computer-03335285269bdba4f94377e56879f9e6`
**Guest:** `3724db1a` deployed_at `2026-09-05T17:36:04Z`, epoch **885**, `choir.actuator=rlm`, effects `propose_only`, fence `99949fe2` untouched. Source/platform remains frozen at `7574d899`.

## Three-way actuator (still holds)

| Leg | Result |
| --- | --- |
| Durable ownership | `/var/lib/go-choir/vm-state/ownerships.json` `actuator=rlm`, epoch 885, `vm_id` `candidate-fleet-e15cb89f25d963c220319b7b` |
| Guest cmdline | `fc-config.json` `choir.actuator=rlm` and `choir.realization_id=...-epoch-885` |
| Guest identity | `http://10.200.18.2:8085/health` ready, `deployed_commit=3724db1a` |

## Exact Super bind (proved)

Hung Super `782d6ed1` occupied the persistent slot in a no-tool Luna loop (325–351 messages, `tool_calls=0`). Owner cancelled that occupant only; mailbox was not drained.

Live Texture→Super hashed occurrence then bound the named control, not FIFO `updates[0]`:

| Super run | Bound control | Work item |
| --- | --- | --- |
| `e3306a66-f85f-4015-baa6-fe24ee9b7fbc` | `96212cc3-6d48-4125-8353-e84a018800e6` | `52c703dd-64f0-4cec-8294-b04d9f4cee67` |
| `da34787a-abaf-4031-b7c3-b348fbd37ba7` | `948a9e0a-2731-432b-a597-695eda971d3e` | `8a934abe-08ed-43e8-aa9d-2df32dc4af11` |

## Sealed overlay (proved)

CoSuper inference used `tools=6` (RLM overlay). Super kept `tools=24`. Observed on gateway for `assignment-60bdda62` and `assignment-b8fad426`.

## Freeze after 3724db1a (proved; eval still failed)

Both fresh assignments reached `capsule_disposition=frozen` with fate acks. That is the 3724db1a/9ff717d0 bind path working. `record_assignment_result` did not land a successful sealed receipt because `capsule_go_eval` never returned exit 0.

| Assignment | Capsule | CoSuper result |
| --- | --- | --- |
| `assignment-60bdda62` | `capsule-ce184f10-...` frozen | parse error before execution (`deepseek-v4-flash`) |
| `assignment-b8fad426` | `capsule-c6e9f846-...` frozen | invalid octal-literal during parse (`deepseek-v4-flash`, 18636/1154 tokens) |


## Verbatim cell retry (2026-09-05T18:57Z)

Cancelled hung Super `da34787a` only. Owner tell `owner-instruction-b1b60919` on doc `d599c4b1` queued at cursor 120.

| Step | Receipt |
| --- | --- |
| Texture turn | version 9, `8105bd86`, 18:59:45Z |
| Super exact bind | `6fc6c937` bound control `5d01c158` work `7855459f` (not FIFO) |
| CoSuper | `assignment-e7107631` capsule `capsule-900854c2` tools=6, prompt contained verbatim `package main` + `func main()` |
| Eval | Yaegi parsed and ran. `choir.ReadFile("/workspace/platform/AGENTS.md")` succeeded. `choir.WriteFile("/workspace/platform/rlm-option-b-proof-2026-09-05.txt")` failed `permission denied`. Capsule froze. `record_assignment_result` did not land a pass. CoSuper completed 19:03:33Z. |

**Cause:** capsule Landlock directory access included `WRITE_FILE`/`MAKE_DIR`/`MAKE_SYM` but not `MAKE_REG`. Creating a new regular file is EACCES. Existing-file reads still work.

**Not proved:** in-capsule write of the proof file, granted receipts, live `get_actuator` RPC.

## Landlock MAKE_REG retry (2026-09-05T19:55Z)

`48c5c5b1` added Landlock `AccessFSMakeReg`. CI run 33986200851 succeeded after SQLITE_BUSY flake reruns, including Deploy to Staging. Owner refresh LifecycleReceipt `01a07323-c4fa-718a-96cf-8d8291e3c8ec` epoch **885→886**, actuator=rlm.

| Leg | Result |
| --- | --- |
| Host | `https://choir.news` `x-choir-build-commit` `a281f1c0` |
| Guest | `http://10.200.19.2:8085/health` ready, `deployed_commit=a281f1c0`, `deployed_at` 2026-09-05T19:55:16Z |
| Ownership | epoch 886 `actuator=rlm` |
| Cmdline | `choir.actuator=rlm` `...-epoch-886` |
| Source/platform | HEAD `7574d899` |

Owner tell `owner-instruction-5540d699` cursor 138. Super `8b001023` exact-bound control `f75e21b1` work `8ea7fad9`.

| Assignment | Outcome |
| --- | --- |
| `assignment-5a5dbb9d` | Super cancelled in 46s; objective omitted `package main` |
| `assignment-241bb9a1` | Verbatim `package main` cell. `capsule_go_eval` **exit 0**. Wrote `/workspace/platform/rlm-option-b-proof-2026-09-05.txt`. Receipt `capsule-go-eval:sha256:7fe0432dcb0600ba03ba4bbbe15bc5026902fea196c1675e3a442b1d88f5a166`. `choir.Complete` receipt `rlm:complete:1`. `record_assignment_result` failed: `executor receipt unavailable`. No freeze event. Super `8b001023` completed. |

**Proved:** three-way actuator=rlm on epoch 886; exact Super bind; sealed overlay; Yaegi file eval; in-capsule read-compute-write of the proof file (Landlock MAKE_REG).
**Not proved:** granted/freeze bind of the go_eval receipt into `record_assignment_result`.

