# Design: Choir Headless Surface — Mental Model Before MCP

**Status:** design / paradoc
**Date:** 2026-07-07
**Scope:** the mental model and verb design for `cmd/choir` (Phase 1 shipped)
and the future MCP server. No code in this doc; it gates the MCP work.
**Doctrine anchor:** [choir-doctrine.md](./choir-doctrine.md) — vocabulary and
invariants below defer to it.

---

## The Question This Doc Answers

"What can Choir do for me?" — asked by an agent or script holding a
`choir_sk_` key, not by a person looking at the web desktop.

The web desktop answers this question with apps (Texture, Source, Super
Console). The headless surface has no apps to point at, so it must answer with
a mental model. Today's CLI answers with an endpoint list, which is the wrong
altitude: `wire stories` and `texture read` tell you what routes exist, not
what the computer is for.

## The Mental Model

Choir is a persistent computer you can hand work to, with three properties
that ordinary computers do not have:

1. **It tries things without breaking itself.** Risky or speculative
   computation runs in candidate computers and capsules; canonical state
   changes only by promotion. (User-facing shorthand: "sandboxing
   technology." Doctrine vocabulary: candidate worlds, promotion, rollback.
   Per the Framing Doctrine, "sandbox" is detector vocabulary, not the
   product ontology — the headless surface should teach the promoted terms.)
2. **It ships.** Work that survives verification becomes durable artifacts,
   publications, and AppChangePackages that other computers can adopt,
   rebuild, and verify themselves. The output is production state, not a chat
   transcript.
3. **It is audited.** Every run leaves trajectories, revisions with
   authorship, evidence, and promotion history. Claims about what the
   computer did are checkable, not asserted.

One sentence for docs and MCP tool descriptions:

> Choir is a persistent computer that tries things in candidate worlds,
> promotes only what survives verification, and can prove what it did.

The three pillars are not independent features; they are one loop —
**try → verify → promote → prove** — observed from three angles. The headless
surface should present them as one loop.

## What Exists Today (Phase 1 inventory)

The proxy forwards any authenticated `/api/*` route to the sandbox by default
(`internal/proxy/handlers.go` `HandleAPI`), so exposure is a curation
decision, not a plumbing project. Routes relevant to each pillar:

| Pillar | Route(s) | CLI verb today |
| --- | --- | --- |
| Try (submit work) | `POST /api/prompt-bar`, `GET /api/prompt-bar/submissions/{id}` | `run start`, `run status` |
| Try (isolation) | candidate/capsule lifecycle — internal to super/vmctl, no public route | none (correct for now; see Gate) |
| Ship (artifacts) | `/api/texture/documents/*` incl. `/revisions` | `texture read/history/revisions` |
| Ship (adoption) | `POST /api/app-change-packages/pull`, review-evidence routes | none |
| Ship (publish) | `/api/platform/publications/*`, `/api/platform/texture/publications` | none |
| Prove (causality) | `/api/trajectories`, `/api/trajectories/{id}` | `trajectories`, `trajectory` |
| Prove (computer identity) | `GET /api/compute/status` (roles, epochs, warmness, protection) | none |
| Prove (public health) | `GET /api/pulse/summary` (aggregate-only, privacy-bounded) | none |
| Retrieval | `GET /api/platform/retrieval/search` | `search` |
| Feed | `GET /api/universal-wire/stories` | `wire stories/diagnostics` (server currently hangs; known issue) |
| Keys | `/auth/api-keys` | `api-key list/create/revoke` |

Observed loop, verified live 2026-07-07: `run start` → conductor decision
(routed app + doc id) → appagent revision within ~10s → `texture revisions`
returns the content → the run appears as a trajectory. The loop works; it is
just not narrated as a loop anywhere headless callers can see.

## Verb Design

Principle: **verbs follow the loop, not the route table.** Target verb
families for the CLI (and, one-to-one, MCP tools):

```text
choir run start|status         # try: hand work to the conductor
choir texture ...              # artifacts: what the work produced
choir package pull|evidence    # ship: adopt AppChangePackages + review evidence
choir publications ...         # ship: resolve/export published artifacts
choir computer status          # prove: which computers exist, epochs, protection
choir trajectory ...           # prove: causality for one piece of work
choir pulse                    # prove: public aggregate health
```

Near-term additions are `computer status`, `package pull`, `pulse`, and
`publications` — all are thin wrappers over routes that already exist and
already enforce scopes.

### The Gate: no candidate-lifecycle verbs yet

The obvious missing verbs — `choir candidate fork|promote|rollback` — are
deliberately **not** in scope. Per
[mission-suite-autoputer-autopaper-spec-first-v0.md](./mission-suite-autoputer-autopaper-spec-first-v0.md),
the promotion protocol is the gate: a persistent computer is not an autoputer
until candidate promotion is model-checked (`specs/promotion_protocol.tla`
rewrite). Exposing promotion verbs on the public surface before the spec
lands would freeze today's unverified semantics into an external contract.
The CLI observes the loop now; it gets to drive the loop's dangerous half
after the spec does. Until then, mutation enters only through `run start`,
where the conductor and appagents own the semantics.

## Scopes Map to Pillars

The existing scope set already encodes the pillars; the surface should say so:

- `read:texture, read:base, read:runtime` — the **prove** surface: audit
  anything, mutate nothing. Safe default for chat models and third parties.
- `write:runtime` (+ read scopes) — the **try** surface: submit runs, adopt
  packages.
- `write:texture, write:base` — direct artifact mutation; agents that own
  meaning.
- `admin` — key management and, eventually, lifecycle verbs behind the gate.

MCP configuration guidance follows directly: issue the narrowest key for the
client class, and let scope errors (403s) teach the model where the
boundaries are.

## MCP Shape (deferred until this doc is accepted)

- One tool per verb family, not per endpoint; descriptions state the mental
  model sentence first.
- One composed tool, `choir_ask`: run start → poll status → fetch appagent
  revision content → return the artifact text plus its doc id and trajectory
  id. This makes the try→prove loop legible to chat models in a single call,
  with the provenance ids attached so the audit trail is one follow-up away.
- Implementation: Go, same repo, reusing the `client.do` pattern from
  `cmd/choir` — either `cmd/choir-mcp` or `choir mcp serve` on the existing
  binary (one auth path, one release artifact).

## Acceptance

This design is accepted when:

1. the mental-model sentence and pillar table are reviewed by the owner;
2. the near-term verbs (`computer status`, `pulse`, `package pull`,
   `publications`) are agreed as Phase 1.5 CLI scope;
3. the candidate-lifecycle gate (no fork/promote/rollback verbs before
   `promotion_protocol.tla` lands) is confirmed;
4. MCP work is unblocked and scoped to the shape above.
