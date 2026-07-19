# go-choir

**Scope:** orientation only. For authority and claim scope, start with
[`docs/README.md`](docs/README.md) and
[`docs/doc-authority-manifest.yaml`](docs/doc-authority-manifest.yaml).

`go-choir` is the implementation repo for Choir, a human-improving,
machine-compounding mainframe: a persistent-computer system for owned learning
over artifacts, evidence, provenance, accepted events, and checkpoints.

Choir runs apps, agents, traces, source material, code, and disposable effect
capsules inside stable persistent computers. It is not a personal note app, an
AI workspace, or a one-off chat surface (chat is a deprecated framing). It is a
substrate where work can be created, inspected, revised, verified, accepted,
materialized, reconstructed, and rolled back.

The short version:

```text
Choir is a human-improving, machine-compounding mainframe made of persistent computers.
```

The project is still early and fast-moving. The practical goal is to make
computers durable enough to improve their own artifacts, applications, runtime,
and doctrine without losing provenance or owner control.

Publishing, writing, reading, coding, review, media, and Wire-style news are
important surfaces, but they are projections of the persistent-computer
substrate. They are not the root ontology.

## Doctrine Snapshot

The canonical doctrine and architecture source is
[docs/choir-doctrine.md](docs/choir-doctrine.md). This README gives the
orientation; Choir Doctrine wins on architectural conflicts.
Use [docs/README.md](docs/README.md) as the documentation index and truth
spine: it points to current architecture, the active-work registry, the minimal
mission graph, assertion register, and heresy detector manifest.

Older docs and code comments may still describe Choir as a personal writing
system, publishing system, AI workspace, sandbox, or workflow app. Treat that as
historical or surface-specific language unless a current doctrine document
explicitly promotes it. The current framing is human-improving,
machine-compounding mainframe, stable persistent computers, durable artifacts,
evidence, trajectories, capsules, accepted checkpoints, and event-derived
rollback. Where older terms appear below, they are contrast classes or
transition labels, not endorsed root framing.

Choir is not trying to optimize for chat smoothness (a deprecated framing), local test passage, or a
short-term product demo. The architecture is currently optimizing for:

- truth from facts: naming a real heresy is progress even before the code is
  fixed;
- correct ontology: the product object is a persistent computer, not a sandbox
  or retired chat session;
- durable causality: work leaves trajectories, work items, evidence, versions,
  provenance, and promotion history;
- evidence-bounded claims: smoke proof, architectural proof, export proof,
  promotion proof, and settlement proof are different claims;
- deletion of retired control paths: dual paths are bugs unless explicitly
  frozen, gated, and on a deletion clock;
- safe self-improvement: architecture changes require explicit conjecture
  deltas, not silent pivots to satisfy probes.

The current rearchitecture target is durable actors: the database remembers, Go
delivers, actors passivate and rewarm, and old parent/child run control,
continuation synthesis, latest-active-run fallbacks, and semantic tool forcing
are removed rather than worked around.

Texture is the canonical document/versioning core. It is an agentic appagent in a
multi-agent system, not a workflow runner. Runtime may expose affordances and
durable obligations, but it must not force Texture to call researcher, super,
verifier, or another semantic appagent merely because text, metadata, or an
acceptance probe mentions that role.

## Autonomy Readiness

Choir's readiness marker is not "humans approve less." It is whether autonomous
mutation produces stronger proof, better doctrine, and lower future supervision
burden.

The autonomy ladder:

| Level | System capability |
| --- | --- |
| 0 | Human writes code and doctrine directly. |
| 1 | Agent writes code; human reviews the diff. |
| 2 | Agent writes code; CI catches obvious failures. |
| 3 | Staging runtime probes catch behavior failures. |
| 4 | An invariant-checking layer catches architectural failures. |
| 5 | Self-development: failed and successful runs update the system's implementation, memory, tests, docs, and future behavior. |
| 6 | The system improves its own improvement machinery under the same discipline it applies to ordinary changes. |

Level 5 is self-development. It means the system can change its own
implementation, doctrine, tests, and operating process while preserving evidence,
rollback, and owner legibility. A Level 5 run does not merely end with a diff,
commit, or deployment. It leaves reusable learning: the system's model of itself
gets sharper, tests are corrected or added, docs stop lying to future agents,
and the next run starts from a better state.

Choir-in-Choir is Choir's implementation of Level 5: one persistent computer
uses durable runs, Texture narratives, Trace evidence, guest-local capsules,
canonical computer events, external verification, scoped owner decisions,
checkpoint publication, and event-derived rollback to improve Choir itself.
Choir leaves alpha only after that complete deployed loop is repeatable.

Level 6 is the perennial beta target: the system can improve its own improvement
machinery. Choir's version uses conjectures, heresies, proof objects, verifier
contracts, promotion, and rollback. Other Level 4+ systems need not use those
names, but they likely need something isomorphic to a proof-theoretic
approximation: explicit claims, protected invariants, admissible evidence,
falsification, and disciplined acceptance or retraction.

## What Choir Does Today

Choir currently presents as a web desktop backed by Go services and a Svelte
frontend. That desktop is a control surface for the mainframe. It includes apps
such as:

- Texture for durable writing and artifact editing
- Source windows for transcluded expansion and long-form source rendering
- Web Lens for explicit live/original web-source inspection
- Files for workspace state
- Super Console for zot-backed diagnosis and repair
- Podcast and media apps for source and playback workflows
- Compute Monitor, Settings, and supporting desktop APIs

### Native macOS App

Choir also has a native macOS desktop app built with
[Wails v3](https://v3.wails.io/). The app wraps the Svelte frontend in a
native macOS window with a transparent title bar, tetramark app icon, and
`ASWebAuthenticationSession` for passkey auth via Safari. It launches in
cloud mode by default (connecting to `choir.news`) and can optionally run
the full local service stack via `CHOIR_MODE=local`.

```bash
cd cmd/desktop
task package    # build .app bundle
task sign       # ad-hoc sign for local testing
```

See [cmd/desktop/README.md](cmd/desktop/README.md) for setup, build, auth bridge,
and the maintained desktop contract.

### Choir CLI

`cmd/choir` is the headless control surface: a pure-Go binary that wraps the
public `/api/` and `/auth/` routes with API key auth (`choir_sk_...`) so
agents and scripts can drive Choir without a browser.

**Current status:** code-present, buildable Phase 1. It has no supported binary
distribution or recorded staging acceptance in this document; build it from
this checkout. It can submit and observe work, but it has no package, adoption,
verifier, run-acceptance, promote, or rollback verbs. `/goal <definition.md>` is
an invocation understood by compatible external agent harnesses; it is not a
Choir CLI command, prompt-bar command, or end-to-end runner implemented by
Choir today.

```bash
go build -o choir ./cmd/choir
export CHOIR_API_KEY=choir_sk_...   # or --api-key; host defaults to https://choir.news

choir run start "prompt text"        # submit to the conductor (same path as the prompt bar)
choir run status <submission_id>     # conductor decision, routed app, doc ids
choir texture read <doc_id>          # document metadata
choir texture revisions <doc_id>     # revisions with full content bodies
choir trajectories                   # recent trajectory state
choir search "query"                 # corpus search
choir wire stories                   # World Wire feed
choir api-key list|create|revoke     # key management
```

All output is JSON on stdout; errors go to stderr (exit 0/1/2 for
success/API error/usage error). See
[skills/choir-cli/SKILL.md](skills/choir-cli/SKILL.md) for the full command
reference and architecture notes.

Trace is not a normal user-facing app. It is the causal/evidence substrate for
agentic tracing, run bundles, acceptance records, and diagnosis artifacts. Raw
Terminal is not a user app either; shell-like repair access is mediated through
Super Console. Web Lens is not the primary source-gathering workflow; source
reading should move from Texture transclusion to a Source window and only then to
Web Lens when the original web page itself needs inspection.

The system is designed around a basic rule:

```text
Work should leave artifacts, evidence, and recoverable state.
```

AI is part of the workflow, but the canonical product object is not a retired chat
transcript. The important objects are files, drafts, traces, versions, source
references, candidate changes, and promoted state.

A retired chat-style input may appear where useful, but it is only an affordance. The
output of the system is durable work.

## Why This Repo Exists

This repo is the implementation side of Choir: runtime services, APIs,
frontend, tests, deployment, doctrine, and control boundaries.

Choir may publish writing and media, but publishing is one projection of the
system. The deeper object is the owned computer and its durable artifacts.

For the deeper design frame, see:

- [docs/choir-doctrine.md](docs/choir-doctrine.md)
- [docs/computer-ontology.md](docs/computer-ontology.md)
- [docs/semantic-registry.md](docs/semantic-registry.md)

## Runtime Model

The implementation centers on persistent user computers. The active
self-development cutover keeps effects disabled by default while exposing one
audited path:

```text
prompt bar -> conductor -> Texture/Super
-> durable implementation run in a guest-local capsule
-> frozen effect bundle and independent verifier certificate
-> canonical proposal event
-> external scoped owner decision through the public API/CLI
-> root guest updater materialization
-> checkpoint publication and route projection
-> restart/reconstruction or event-derived rollback
```

The invariants are:

- one stable `ComputerID` owns the history; realizations are replaceable;
- exactly one guest-core appender sequences semantic computer events;
- risky effects run only in capability-scoped capsules;
- a speculative change is a frozen capsule effect bundle, never a VM, route,
  mutable branch, package, or lineage record;
- accepted desired state is distinct from effective materialized state;
- private event payloads use authenticated encryption with a guest-owned key;
- guest-core and verifier private keys remain in separate typed signer services
  and are inaccessible to the updater and runtime;
- checkpoint publication and route projection follow effective application;
- rollback is derived from retained events and receipts, not host-local state.

Shared platform source still uses the Git landing loop: commit, push, CI,
deploy, deployed identity, then product-path proof. That deployment path is
separate from the reviewed source-candidate identity bound at the G1 gate.

Storage direction (owner decision, 2026-07-08): exactly two product-state Dolt
stores remain distinct: the corpusd world-wire sql-server store and each
computer's VM-local embedded app-state store. Narrow route-slot and transition
receipt tables live on corpusd with vmctl as sole CAS writer; they are route
control, not a third product-state store. The current `DoltPromotionAdapter`
remains tag-only and non-conformant. These nonconformant paths remain blocked. The completed
[audited-construction Definition](docs/definitions/choir-audited-autoputer-construction-2026-07-15.md)
retains the evidence and deletion boundary; no current product `/goal` authorizes
further replacement or deletion.

## Services

The core edge/runtime topology has five Go services:

| Service | Port | Role |
| --- | --- | --- |
| `auth` | 8081 | Email/passkey registration, login, JWT access/refresh sessions |
| `proxy` | 8082 | Auth-gated HTTP/WebSocket proxy, user-context injection, VM routing |
| `vmctl` | 8083 | Persistent computer realization lifecycle and ComputerVersion route projection |
| `gateway` | 8084 | Provider-neutral LLM/search gateway reachable by host/guest callers, not the public browser edge |
| `sandbox` | 8085 | Runtime service for desktop APIs, Texture, files, source/Web Lens sessions, Super Console, trace evidence APIs, and the agent/tool loop |

Every service exposes `/health`.

Additional deployed or deploy-wired host services are deliberately narrower:

| Service | Role | Current boundary |
| --- | --- | --- |
| `corpusd` | Public publication and World Wire object-graph API/store boundary | Writes/serves public projections; platform-computer agents remain semantic owners. |
| `sourcecycled` | Experimental source polling and handoff adapter | Cycle/queue state is in memory and lost on restart; it is not canonical article authority. |
| `maild` | Host email ingress/adapter | Transport boundary, not private appagent authority. |

`capsule-host`, Choir Base harnesses, materializers, evidence tools, and other
`cmd/*` binaries are implementation utilities or partial substrates unless a
current domain contract explicitly labels them deployed product services.

The `sandbox` service name is an implementation name, not the product ontology.
The product object is a persistent computer. The sandbox health response
includes build/deploy identity used by staging verification.

## Self-Hosting And Local Development

The repo is still a fast-moving system, but the intended local shape is
straightforward: run the Go services plus the Svelte frontend, backed by local
service configuration and runtime state.

Requirements:

- Go 1.25+
- Node.js 22+
- pnpm 10+
- Nix for reproducible Linux builds, deployment configuration, and dev shells

Install frontend dependencies:

```sh
cd frontend
pnpm install
cd ..
```

Start the local stack when local iteration is appropriate:

```sh
nix develop -c ./start-services.sh
```

The script uses local auth keys and service ports, and requires the repo dev
shell so Dolt/ICU compiler and linker paths come from the declared Nix
environment. For detailed manual service startup, inspect `start-services.sh`
and the relevant `cmd/*` package configs.

Local development is useful for frontend iteration, focused unit shaping, and
reproducing transitions identified by deployed evidence. It is not sufficient
proof for live `vmctl`, guest isolation/key custody, provider credentials,
self-development materialization, route projection, rollback, or production
deployment.

## Tests And Dev Shells

Go tests that touch Dolt need ICU headers. In a developer checkout, use the repo
dev shell so compiler and linker paths come from Nix instead of hand-entered
`CGO_*` flags:

```sh
nix develop
go test -count=1 ./internal/store ./internal/agentcore ./internal/textureowner
```

If you use `direnv`, run `direnv allow` once and the same environment will load
automatically when you enter the repo.

The guest image provides the direct toolchain used by guest-local capsules:
`git`, `go`, `gcc`, `pkg-config`, `node`, and ICU libraries. A capsule must use
that pinned offline toolchain; it must not invoke `nix develop`, `nix build`, or
`nix-store`.

Full local test shaping:

```sh
go test ./... -count=1

cd frontend
pnpm run build
pnpm exec playwright test --workers=1
```

Documentation-only changes intentionally do not run automatic CI. The GitHub
workflow ignores `docs/**` and top-level `*.md` for push and pull-request CI.
Do not weaken those path filters just to make docs-only commits run CI.

## Deployment And Staging Proof

Platform behavior-changing work uses staging as the acceptance environment. A
platform behavior-changing mission is not complete because local tests pass. It
is complete when the pushed commit is running on staging and the deployed
product path is verified there.

Personal-computer evolution is different. A user's stable computer develops a
new Go runtime, Svelte UI, package set, app, or prompt inside a guest-local
capsule. The frozen effect bundle receives verifier evidence and explicit owner
acceptance before the updater materializes it as an immutable checkpoint and
the serving route advances to that accepted checkpoint. The canonical computer
event chain supplies typed deltas, causal history, reconstruction, and rollback;
no candidate/worker computer, mutable branch, host repair path, or global
platform deploy participates.

Required landing loop for behavior changes:

```text
commit -> push origin main -> monitor CI -> monitor staging deploy
-> verify staging commit identity -> run deployed acceptance proof
```

Keep staging-host specifics in deployment docs, mission reports, or environment
configuration rather than making this README depend on one temporary domain.

## Agent Contract

Read [AGENTS.md](AGENTS.md) before using an agent to modify this repo.

The short version:

- `conductor` routes exogenous user/app input and does not own semantic outcomes.
- Appagents own durable app artifacts; `texture` is the canonical semantic surface.
- `super` orchestrates durable runs and guest-local capsules but cannot mutate
  accepted state directly.
- CoSuper performs bounded implementation or verification through capsule tools.
- Canonical computer state changes only through the event appender, external
  scoped approval, materialization, checkpoint, and route certificates.
- Verification is a contract and evidence record, not a separate agent caste.

## Self-Development Proof

The public `choir self-dev` commands expose mode, genesis, proposal, inspection,
approval/rejection, rollback, wait, and kernel-capability operations for one
explicit `ComputerID`. Effects default to `off`; `accept_once` authorizes only
the exact canonical approval request bound into its consumption receipt.

No individual artifact proves self-development. Completion requires joined
event heads, verifier and materialization receipts, checkpoint and route
certificates, deployed build identity, restart/reconstruction evidence,
rejection evidence, and exercised rollback through the active Definition's
G3 gate.

## Documentation Map

Start with [docs/README.md](docs/README.md). It defines the bounded current
packet: doctrine, operating contract, semantic registry, current state, domain
contracts, and any currently promoted product Definition. The coherent terminal
state has none; completed Definitions remain working-tree evidence or Git history.

## Repository Shape

```text
cmd/                 service entrypoints
cmd/desktop/         native macOS app (Wails v3)
internal/auth/       passkey/JWT auth
internal/proxy/      auth-gated proxy and VM routing
internal/vmctl/      VM ownership/lifecycle API
internal/gateway/    LLM/search gateway
internal/agentcore/  generic agent lifecycle, product APIs, evidence, and control surfaces
internal/textureowner/ Texture documents, revisions, prompts, tools, sources, and wake behavior
internal/coagentowner/ co-agent spawn and handoff ownership
internal/store/      runtime persistence plus embedded Texture/Dolt workspace
frontend/            Svelte desktop and Playwright tests
nix/                 deployment and NixOS configuration
docs/                architecture, missions, proofs, and historical notes
```
