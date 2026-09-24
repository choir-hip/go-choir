# Choir

**The automatic computer.**

Choir is a general-purpose technology built to serve a real demand: a
better way to do complex R&D, and to consume and communicate ideas. It
is a persistent computer made of agents — not a chat-interfaced agent
handed a computer, as most agent products are.

Most agent systems start over every time. Every conversation is a fresh
session; what was believed, tried, accepted, and rolled back dies with the
context window. Choir is built around a different object: a running computer
that remembers. Work leaves versioned artifacts, provenance, accepted events,
and rollback — not a transcript.

```text
Agents keep a computer, not a conversation.
```

Choir rests on divisive bets — against chat-shaped agents, against the
agent-as-replacement-worker framing. They could be wrong; if they're
right, the computer is the product.

## What it is

A persistent, self-supervised, multi-agent computer. An RLM (recursive
language model) coordinates many communicating agents — persistent and
ephemeral — over long horizons. Every change is a typed event; every surface
is a deterministic projection. A human is the root of intention; durable
state changes require owner-legible evidence and approval.

The same computer, projected outward, is a media product: an automatic
newspaper (the proof) and an automatic radio (the adoption UX). The
provenance-linked record those produce — who said what, and whether they were
right — is the data product. The learning mechanism underneath is
**precommitment records**: agents commit typed predictions before acting, the
outcome resolves and scores them, and the accumulated log is the computer's
world model — the thing humans supervise and the moat that compounds.

## What you can do today

- **Web desktop** — a persistent-computer control surface: durable writing and
  artifact editing (Texture), source windows, explicit web inspection (Web
  Lens), files, and a repair console.
- **Native macOS app** — a Wails wrapper around the same desktop (see
  `cmd/desktop/`).
- **CLI** — headless control for agents and scripts with API-key auth
  (`cmd/choir`).

All of these are projections of the persistent-computer substrate. Publishing,
media, and the record are downstream projections, not the root.

## Status

Early. Fast-moving. APIs change. Some surfaces are code-present rather than
product-complete; if you want polished consumer software, you're early. If you
want to help build the substrate, start here.

## How it works, in one breath

Human intent enters through the prompt bar and deferred world-wire ingress
materializes it in Texture, the sole agent writer of the canonical artifact.
Texture, management, engineering, and research are persistent root RLM desks
running yaegi Go cells in killable subprocesses. When an artifact needs
execution, Texture sends a semantic act to management; management admits
engineering through delegated `choir.Cast`, and engineering mutates only through
capsule-bound per-assignment sub-RLM cells. Research has read-only world and
message authority. Risky or long-running effects freeze as exact proposals.
An effect-specific policy evaluates a qualified multiagent consensus, optionally
including a human, before a trusted actuator executes the decision. Commitment
objects live on the tape, reports resolve them, and Texture renders their
material supervision state under editorial discretion. Reversible state can be
reconstructed or restored from retained events and receipts; irreversible
consequences retain receipts and recover through compensation or a new forward
action.

```text
prompt bar -> world-wire ingress (deferred) -> Texture desk
-> semantic act -> management desk -> delegated choir.Cast
-> engineering sub-RLM in capsule -> frozen proposal + verifier evidence
-> policy-governed multiagent consensus -> audited actuator
-> commitment/report ledger -> Texture revision -> restore, correction, or compensation
```

## Concepts in five words

- **Persistent computer** — a versioned, provable, evolving machine.
- **Artifact** — durable owned state, like documents.
- **Accepted event** — a policy-authorized change advancing state.
- **Restore** — a forward transaction reconstructing prior reversible state.
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
[docs/signal-is-sparse-not-the-learner-2026-08-01.md](docs/signal-is-sparse-not-the-learner-2026-08-01.md),
and the deeper architecture in [the vision](docs/choir-vision.md).

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
