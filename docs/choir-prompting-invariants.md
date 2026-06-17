# Choir prompting invariants

## Status

Operating invariant for seeded agent system prompts and YAML prompt specs.
Companion: [choir-role-free-actor-protocol-2026-06-11.md](choir-role-free-actor-protocol-2026-06-11.md).

## No false identity assignment

Choir prompts must **not** open with persona assignment such as:

- `You are a researcher.`
- `You are Choir super.`
- `You are one agent inside Choir…` (when that line assigns a self-image)

Those lines induce role theater: the model performs an identity instead of
advancing proof state under an authority envelope.

## Required opening frame

Each role-specific prompt body should open with a **descriptive frame**, not an
identity claim:

```text
This is the system prompt for the <role> agent in Choir.
```

`<role>` is the functional role name (`texture`, `researcher`, `super`,
`conductor`, `vsuper`, `co-super`, `processor`, `reconciler`, …).

## Choir context

After the opening frame, explain Choir in third person or as system context:

- multiagent writing, research, and execution;
- one product, one runtime, one standard of truth;
- durable documents, agents, coordination channels, event and revision history;
- agents as participants in a workflow, not isolated chatbots.

The shared `core` prompt carries the cross-role Choir explanation. Role prompts
carry the opening frame plus role-specific theory and operational morphisms.

## Obligation over persona

Prefer obligation, authority envelope, and morphism class over persona:

- what evidence or artifact state must change;
- which tools/oracles are admissible;
- when to checkpoint, incorporate, delegate, or stop;
- stop when **marginal returns diminish**, not when a role “feels done.”

## Research cadence

Multi-revision Texture work stalls when **researchers stop early**, not when
Texture refuses to revise. While researchers keep searching and sending
`update_coagent` checkpoints, Texture should keep incorporating with
`patch_texture` and, when helpful, keep addressing researchers with follow-up
questions via `update_coagent` or additional `spawn_agent` probes—until depth
no longer materially improves the artifact.

Researchers should prefer **parallel saturation**: in the same tool-call block,
combine `update_coagent` with the next `web_search`, `source_search`, `fetch_url`,
or import probe; repeat for multiple rounds in one run until further searches
mostly repeat prior findings and no longer add marginal grounded material.

## Enforcement

- Seeded defaults live in `internal/runtime/prompt_defaults/*.yaml` and
  `internal/runtime/textureprompts/texture.yaml`.
- Runtime fallbacks in `systemPromptForRun` must use the same frame, not
  `You are Choir <role>.`
- Tests should assert the descriptive opening where they pin default prompt text.
