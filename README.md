# go-choir

**Scope:** orientation only. For authority and claim scope, start with
[`docs/README.md`](docs/README.md) and
[`docs/doc-authority-manifest.yaml`](docs/doc-authority-manifest.yaml).

`go-choir` is the implementation repo for Choir, a human-improving,
machine-compounding mainframe: a persistent-computer system for owned learning
over artifacts, evidence, provenance, and promotion history.

Choir runs apps, agents, traces, source material, code, candidate worlds, and
promotion flows inside persistent computers. It is not a personal note app, an
AI workspace, or a one-off chat surface (chat is a deprecated framing). It is a substrate where work can be
created, inspected, revised, verified, forked, and promoted.

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
spine: it points to current architecture, the active Definition, the minimal
mission graph, assertion register, and heresy detector manifest.

Older docs and code comments may still describe Choir as a personal writing
system, publishing system, AI workspace, sandbox, or workflow app. Treat that as
historical or surface-specific language unless a current doctrine document
explicitly promotes it. The current framing is human-improving,
machine-compounding mainframe, persistent computers, durable artifacts,
evidence, trajectories, candidate worlds, and promotion.
Where those older terms appear below, they are contrast classes or transition
labels, not endorsed root framing.

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

Choir-in-Choir is Choir's implementation of Level 5: Choir uses its own
persistent computers, Texture narratives, Trace evidence, candidate worlds,
AppChangePackages, verifier contracts, and promotion path to improve Choir
itself. Choir leaves alpha when this self-development loop is reliable enough
that failed runs improve the system instead of merely consuming attention.

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

The implementation centers on persistent user computers. The target contract
for controlled candidate work is ahead of the currently wired product path.

### Live today

At a high level:

- each user computer is a persistent, stateful object;
- appagents own durable app artifacts;
- Super can delegate mutable work into worker/background VMs and repo
  checkouts; those VMs are transitional execution substrates, not semantic
  candidate ComputerVersions;
- workers produce evidence, source deltas, AppChangePackages, or reports;
- AppChangePackages carry source changes between divergent computers;
- recipient computers rebuild and verify adopted changes themselves;
- compaction preserves what a run learned for future inference.

The usable self-development path is:

```text
prompt bar or `choir run start`
-> conductor -> Texture
-> optional Super delegation to a worker/background VM
-> repo edit, build, tests, evidence
-> AppChangePackage
-> Features import and recipient build/verification
-> owner approval and lineage/adoption record
```

This path does **not** perform a served runtime/UI route cutover and does not
land shared Choir source. A shared platform change still uses the Git landing
loop: commit, push, CI, deploy, deployed identity, and product-path proof.

### Target contract

- canonical state stays stable unless a change is promoted;
- candidate state is a ComputerVersion fork — `(CodeRef, ArtifactProgramRef)` —
  not a VM;
- risky effects execute in capsules: ephemeral, capability-scoped chambers
  whose typed transactions append to the candidate;
- substrates such as Firecracker, containers, and host processes materialize a
  ComputerVersion but do not define its identity;
- promotion is an atomic route flip between ComputerVersions after verifier
  evidence, owner acceptance, and a recorded rollback target.

A compact **target** operating invariant:

```text
Evidence enters through researchers.
Meaning is owned by appagents.
Computation is orchestrated by super.
Mutation happens in capsules against forked ComputerVersions.
Computers diverge.
Canonical state changes only by promotion.
```

The target self-development path is:

```text
prompt bar -> conductor -> appagent/Texture -> super
-> capsule execution against a forked ComputerVersion
-> AppChangePackage
-> recipient adoption and rebuild
-> verifier evidence
-> owner decision
-> promotion (atomic route flip between ComputerVersions) or rollback
```

The objective is to improve artifacts over time while minimizing corruption,
deadlock, human monitoring burden, and loss of understanding.

**Promotion claim ceiling today:** Features **Activate** and **Roll back** are
adoption/source-lineage protocol transitions. Nothing in the ordinary personal
computer path currently consumes `RouteProfile` to switch the served runtime or
UI binary. Current API `promotion-level` records therefore prove bounded
package/adoption protocol evidence only; do not cite them as doctrine-level
ComputerVersion promotion without an observed route/build cutover and rollback
proof.

Storage direction (owner decision, 2026-07-08): exactly two product-state Dolt
stores remain distinct: the corpusd world-wire sql-server store and each
computer's VM-local embedded app-state store. Narrow route-slot and transition
receipt tables live on corpusd with vmctl as sole CAS writer; they are route
control, not a third product-state store. The current `DoltPromotionAdapter`
remains tag-only and non-conformant. These nonconformant paths remain blocked;
the active
[audited-construction Definition](docs/definitions/choir-audited-autoputer-construction-2026-07-15.md)
exclusively assigns and sequences replacement or deletion.

## Services

The core edge/runtime topology has five Go services:

| Service | Port | Role |
| --- | --- | --- |
| `auth` | 8081 | Email/passkey registration, login, JWT access/refresh sessions |
| `proxy` | 8082 | Auth-gated HTTP/WebSocket proxy, user-context injection, VM routing |
| `vmctl` | 8083 | Desktop and worker VM ownership/lifecycle, with host-process fallback where Firecracker is unavailable |
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
proof for claims about live `vmctl` behavior, provider credentials,
background/candidate computers, platform promotion, rollback, or production
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

Worker and candidate VMs are different. They should not run `nix develop`,
`nix build`, or `nix-store`; the guest image is expected to provide direct PATH
tools such as `git`, `go`, `gcc`, `pkg-config`, `node`, and ICU libraries. If a
worker VM cannot run Dolt-backed Go tests with plain `go test`, treat that as a
VM image/runtime regression.

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

Personal-computer evolution is different. A user should be able to fork their
own computer, build a new Go runtime or Svelte UI, install packages, add apps,
change prompts, and promote that candidate back into their own active computer
without waiting for a global platform deploy. That path still needs lineage,
typed deltas, verifier evidence, owner acceptance, and rollback, but its target
is the user's active computer rather than `origin/main`.

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
- Appagents own durable app artifacts; `texture` is the current canonical semantic surface.
- `super` is the foreground orchestration root and mints bounded execution authority.
- Worker/candidate mutation belongs in background/candidate computers or isolated worker worlds.
- Canonical state changes only through explicit promotion after verification and owner acceptance.
- Verification is a contract and evidence record, not a separate agent caste.

## Run Acceptance

The durable verifier is the Run Acceptance System:

- `POST /api/run-acceptances/synthesize` derives a `RunAcceptanceRecord` from
  existing runs, Trace tool results, AppChangePackage/adoption records, build
  identity, verifier evidence, and owner-scoped state.
- `GET /api/run-acceptances?trajectory_id=...` lists acceptance records for a
  trajectory.
- `GET /api/run-acceptances/{acceptance_id}` fetches one record.

Acceptance levels are explicit so the system does not overclaim:

- `docs-level`
- `staging-smoke-level`
- `export-level`
- `promotion-level`
- `continuation-level` (transitional H008/H014 residue until M4 re-points or
  deletes it; not a target permanent doctrine term)

Do not claim `promotion-level` without AppChangePackage adoption verifier
contract evidence plus owner review and promote/rollback evidence. Do not claim
`continuation-level` without run-memory/compaction and continuation evidence.
Do not introduce new continuation-shaped proof as doctrine; the target evidence
class is trajectory/work-item settlement.

In the current implementation, a recorded rollback reference can satisfy the
API checkpoint without an exercised rollback, and an adoption promotion event
does not prove the served runtime/UI changed. Treat current `promotion-level`
as package/adoption protocol evidence unless route identity, deployed build,
and rollback behavior were independently observed.

## Documentation Map

Start with [docs/README.md](docs/README.md). It defines the bounded current
packet: doctrine, operating contract, semantic registry, current state, domain
contracts, and the one active product Definition. Historical missions and raw
evidence are available through Git history, not the working-tree search corpus.

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
