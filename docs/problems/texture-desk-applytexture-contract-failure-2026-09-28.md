# Texture desk burns a full activation failing to call `choir.ApplyTexture` — `undefined: choir` then invented packet fields

**Status:** DIAGNOSED 2026-09-28 on staging (deployed `54c96c98`); fix
commit follows this documentation commit per
`docs/memo-problem-documentation-first.md`.

## Symptom

Every texture desk run activated by an owner document revision fails without
committing an app-agent revision. Doc `12f0e3fc-aa9a-5e2b-971d-0e7df77f28e0`,
trajectory `e97bce0e-7458-5105-b5aa-ed3772a4ea03`, user
`a0c289f5-5957-4dd4-8236-09965ab94acb` (probe VM `vm-a540b76d64518053`):

- runs `20cc71fb`, `34f840a5`, `6dbad585` — all `failed`;
- `b1868404` — passivated.

Run `20cc71fb` event tape shows the desk looping on its own write path:

- iters 1–2: `choir.ApplyTexture({...})` → compile rejection
  `1:28: undefined: choir` (model did not write `import "choir"`);
- iters 3–5: after adding `import "choir"`, the cell compiled but reduce
  rejected the staged body —
  `texture cell author: decode staged body: json: unknown field "type"`.
  The model guessed the controls packet shape:
  `packet: {"type": "coagent_source_packet.v1", "kind": "research_request"}`.
  Neither field is valid: `packet` has no `type` key (`schema_version` is
  runtime-stamped by `PrepareTextureControlPacket`), and
  `research_request` is not a valid kind (valid: `evidence_update`,
  `execution_request`, `execution_result`, `blocker`, `question`,
  `proposal`, `decision_request`; the test-pinned research opener uses
  `kind="question"`).
- Net: ~49k activation input tokens spent, zero commits, run died.

## Why it is a contract bug, not a model-quality bug

`desk_go_eval` is the desk's only tool — there is no provider tool schema
for `choir.*` verbs. The **system prompt is the entire API surface**, and
`internal/textureprompts/overlays/run_system.yaml` never names the packet's
fields; it says only "an objective, and a packet". The model filled the gap
from generic OpenAPI habits (`type` discriminator, `research_request` kind)
and the strict `DisallowUnknownFields` decode killed it — then returned an
opaque `unknown field "type"` with no field list to self-correct against.

Two gaps:

1. `import "choir"` is required but nothing guarantees it — the first cell
   of every activation risks a wasted compile cycle.
2. The packet schema (`kind`/`summary`/`claims`/`sources`/`actions`/
   `questions`/`notes`) is undocumented in the prompt, and the decode error
   does not teach it.

## Fix shape (separate commit)

- `yaegikernel.NewSession` predeclares `choir` when `choir/choir` symbols
  and the allowlist permit it — no import needed, dedupe still guards a
  model-authored re-import.
- `run_system.yaml` gains the exact packet field contract + `choir` marked
  predeclared; management/research/engineering overlays aligned.
- `CommitCellTextureAuthor` decode errors append the valid field list and
  valid `kind` values so the desk self-corrects in-band in one iteration.
- Texture role reasoning effort `low` → `xhigh` in `fallbackPolicy` +
  `defaultPolicyText`: instruction-following at `low` demonstrably cannot
  hold this contract.
