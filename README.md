# Choir

**Early-stage, fast-moving, not a stable product yet.** Choir is a persistent
computer where AI agents do durable, inspectable, rollback-safe work.

Most agent systems start over every time. Every conversation is a fresh session;
what was believed, tried, accepted, and rolled back dies with the context
window. Choir is built around a different object: a running computer that
remembers. Work leaves versioned artifacts, provenance, accepted events, and
rollback — not a transcript.

```text
Agents keep a computer, not a conversation.
```

A human is the root of intention. Agents compound memory and execution; durable
state changes require owner-legible evidence and approval. The deeper idea — why
this architecture, what it optimizes — is in
[the vision](docs/choir-vision.md).

## What you can do today

- **Web desktop** — a persistent-computer control surface: durable writing and
  artifact editing (Texture), source windows, explicit web inspection (Web
  Lens), files, and a zot-backed repair console.
- **Native macOS app** — a Wails wrapper around the same desktop (see
  `cmd/desktop/`).
- **CLI** — headless control for agents and scripts with API-key auth
  (`cmd/choir`).

All of these are projections of the persistent-computer substrate. Publishing,
media, and the World Wire (an index of the world as reported — contested and
plural) are downstream projections, not the root.

## Status

Early. Fast-moving. APIs change. Some surfaces are code-present rather than
product-complete; if you want polished consumer software, you're early. If you
want to help build the substrate, start here.

## Try it

Local development for frontend iteration, focused unit work, or reproducing a
deployed transition:

```sh
cd frontend && pnpm install && cd ..
nix develop -c ./start-services.sh   # runs the local service stack
```

Requirements: Go 1.25+, Node.js 22+, pnpm 10+, Nix. Details live in
[docs/current-architecture.md](docs/current-architecture.md) and the
`cmd/*` package configs.

Local proof is not staging proof. Platform behavior is accepted against
staging (`https://choir.news`); vmctl, guest isolation, credentials, promotion,
rollback, and Choir-in-Choir behavior cannot be claimed from a local checkout.

## How it works, in one breath

Human intent enters through the prompt bar. The conductor routes it to an
appagent — Texture for documents, which owns the canonical artifact. When the
artifact needs execution, Texture calls super, the orchestrator, which runs
CoSuper executors — the only actors with access to containerized bash in
guest-local capsules. Risky or long-running effects freeze as proposed bundles;
an explicit owner acceptance event authorizes materialization, checkpoint, and
route projection. Everything can be reconstructed or rolled back from retained
events and receipts.

```text
prompt bar -> conductor -> appagent (Texture) -> super -> CoSuper executor
-> durable run in a capsule
-> frozen proposal + verifier certificate
-> owner approval -> materialization -> checkpoint -> rollback-safe state
```

## Concepts in five words

- **Persistent computer** — a versioned, provable, rollbackable machine.
- **Artifact** — durable owned state, like documents.
- **Accepted event** — an owner-approved change advancing state.
- **Rollback** — reconstruct state from retained events.
- **Capsule** — guest-local workspace for risky effects.

## Self-development

The essential capability, not a feature: the computer proposes changes to
itself — its own environment, tools, and operating rules — under the same
evidence-and-approval discipline as any other change. Frozen proposals,
verifier certificates, owner acceptance. That supervised loop is the product;
everything else in this repo is infrastructure for it.

## The idea behind the idea

Choir's wager: sample inefficiency is undirected learning, not architecture.
Signal density comes from a learner with standing questions and from correction
by genuinely independent others — and both have to be built into the
environment the intelligence runs in. The environment is the durable layer, not
the model. Read the argument in
[docs/signal-is-sparse-not-the-learner-2026-08-01.md](docs/signal-is-sparse-not-the-learner-2026-08-01.md).

## Contributing

Read [AGENTS.md](AGENTS.md) before using a coding agent in this repo. It is the
operating contract: mutation classes (green/yellow/orange/red/black), the
staging landing loop, and what counts as proof.

The documentation spine is [docs/README.md](docs/README.md), with authority
claims in [docs/doc-authority-manifest.yaml](docs/doc-authority-manifest.yaml)
and the normative architecture in [docs/choir-doctrine.md](docs/choir-doctrine.md).
Older docs may still use retired framing (chat, autoputer, AI workspace); treat
that as historical unless a current doctrine document promotes it.

Tests:

```sh
go test ./... -count=1
cd frontend && pnpm run build && pnpm exec playwright test --workers=1
```

Go tests that touch Dolt need ICU headers from the Nix dev shell (see
[AGENTS.md](AGENTS.md)).

## Repository shape

```text
cmd/                  service entrypoints
internal/auth/        passkey/JWT auth
internal/proxy/       auth-gated proxy and VM routing
internal/vmctl/       persistent-computer lifecycle
internal/gateway/     provider-neutral LLM/search gateway
internal/agentcore/   agent lifecycle, product APIs, evidence, control
internal/textureowner/ Texture documents, revisions, prompts, tools
internal/store/       runtime persistence (embedded Dolt)
frontend/             Svelte desktop and Playwright tests
nix/                  deployment and NixOS configuration
docs/                 architecture, doctrine, missions, evidence
```
