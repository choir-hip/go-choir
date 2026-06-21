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

## Coagent update delivery (2026-06-17)

`update_coagent` is the sole agent-to-agent wake primitive. Delivery semantics are
uniform across Texture, super, researcher, vsuper, and co-super activations.

### Typed packets, not inferred routing

- Every delivered update becomes a **typed user turn** in the target activation's
  context window: a `coagent_update` JSON packet with `packet_type`,
  `delivery_phase` (`activation_mailbox_turn`, `cold_activation`,
  `mid_activation`, `final_checkpoint`), and structured update records.
- **Warm activations** inject pending updates between tool-loop iterations.
- **Texture activation wakes** append pending updates as the first durable
  mailbox turn in run memory, not as prompt-prefix reconstruction.
- **Cold activation** packet prepending is compatibility behavior for
  non-Texture actors that do not yet have the durable thread substrate.
- `update_id` is a runtime-owned idempotency and delivery handle. Model-facing
  prompts and tool schemas should not require an LLM to invent a globally unique
  checkpoint key. The runtime may return `update_id` after persistence for Trace,
  delivery accounting, and debugging.
- Runtime must **not** traverse spawned-by / parent-run edges to decide who
  receives an update. Provenance fields (`RequestedByRunID`, `requested_by_run_id`)
  are audit-only.

### One Texture coagent per article

- Each Texture document/article has a durable Texture coagent id:
  `texture:<doc_id>`.
- Researchers spawned for that article must address **that exact id** on every
  `update_coagent` call via the required `agent_id` argument.
- Spawn metadata (`requested_by_agent_id`, run-context overlay) names the
  delivery target so the researcher can copy it; runtime does not infer the
  target when the caller is a researcher.
- Super and other roles may still use explicit `agent_id` or documented
  non-researcher resolution paths; researchers may not omit `agent_id`.

### Texture wake path

- `wakeUpdatedCoagent` uses the same `reconcileUpdatedCoagentActor` entry path
  for all addressed agents, including `texture:<doc_id>`.
- Texture integrate runs (`integrate_worker_findings`) start when pending
  updates exist and no conflicting pending mutation blocks; worker content
  arrives through injected packets, not a separate channel-only prompt embed.
- Failed Texture integrate runs must **not** advance the worker-update
  checkpoint or mark updates delivered without a canonical revision.

### Required tests

- researcher `update_coagent` rejects missing or non-texture `agent_id`;
- model-facing `update_coagent` can be called without a model-invented
  `update_id`;
- retries of one delivery dedupe by runtime-derived identity, while distinct
  deliveries cannot collide because the model reused a local label such as
  `checkpoint-1`;
- typed packet builder and Texture warm/cold injection paths;
- coagent rewarm and resident-activation injection behavior;
- Texture wake after researcher delivery produces a revision when the model
  patches.

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

## Researcher delivery addressing

Each Texture article has its own Texture coagent (`texture:<doc_id>`). Researchers
report to exactly one Texture coagent per activation.

- Every researcher `update_coagent` call must set `agent_id` to that Texture
  coagent id. Runtime rejects researcher deliveries without an explicit texture
  agent id.
- Spawn/run context names the delivery target (`requested_by_agent_id` and the
  run-context overlay). The researcher copies that value into each tool call;
  runtime does not infer the recipient from spawned-by lineage or channel alone.
- See [texture-agentic-invariants-2026-06-13.md](texture-agentic-invariants-2026-06-13.md)
  for the full coagent update delivery contract (typed packets, warm injection,
  Texture wake).

## Enforcement

- Seeded defaults live in `internal/runtime/prompt_defaults/*.yaml` and
  `internal/runtime/textureprompts/texture.yaml`.
- Per-run runtime overlays live in `internal/runtime/runtimeprompts/overlays/*.yaml`
  (temporal grounding, conductor routing, researcher saturation, super/vsuper
  boundaries, worker repo bootstrap, run context).
- Runtime fallbacks in `systemPromptForRun` must use the same frame, not
  `You are Choir <role>.`
- Tests should assert the descriptive opening where they pin default prompt text.
