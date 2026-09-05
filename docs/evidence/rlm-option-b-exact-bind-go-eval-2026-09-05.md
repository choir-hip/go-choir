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

## Remaining gap

Super work items carried the exact Go program. `assign_co_super` paraphrased it. CoSuper then invented source:

1. English / snippet without `package main` + `func main()` (Yaegi eval is a file, not a statement list).
2. Unquoted date fragments such as `2026-09-05` parse as `2026 - 09 - 05`; Go rejects `09` as an octal literal.

The first exact-Go tell's source itself omitted `package main`/`func main()`. The 18:42Z tell included a complete program; Super still did not copy it into the CoSuper objective.

**Not proved:** live `get_actuator` RPC, in-capsule read-compute-write of `rlm-option-b-proof-2026-09-05.txt`, or a successful `record_assignment_result`.

## Next

Do not drain the Super mailbox. Super must place the verbatim `package main` cell into `assign_co_super` (structured field, not paraphrase). Then one `capsule_go_eval` exit 0 and one `record_assignment_result`, effects `propose_only`.
