# go-choir

`go-choir` is the implementation repo for Choir, my personal writing,
research, and publishing system.

Choir helps me work with notes, drafts, source material, code, traces, and
local app state as durable artifacts instead of one-off chats. The current
product is a web desktop with apps for writing, files, browsing, terminal work,
traces, media, and agent-assisted workflows.

The short version:

```text
Choir is a personal AI-assisted publishing workspace built around durable artifacts.
```

The project is still early and fast-moving. For now, the goal is practical:
make it easier to read, write, build, verify, and publish from one persistent
workspace.

The larger direction is to make artifacts easier to compose, verify, publish,
revisit, and render into other formats such as briefings, streams, and audio.
That deeper product vocabulary belongs in the docs and in public writing rather
than in the top-level README.

## What Choir Does Today

Choir currently runs as a web desktop backed by Go services and a Svelte
frontend. It includes apps such as:

- VText for durable writing and artifact editing
- Files for workspace state
- Browser for source gathering
- Trace for run and tool evidence
- Terminal for local/runtime work
- Podcast and media apps for source and playback workflows
- Compute Monitor, Settings, and supporting desktop APIs

The system is designed around a basic rule:

```text
Work should leave artifacts, evidence, and recoverable state.
```

AI is part of the workflow, but the canonical product object is not a chat
transcript. The important objects are files, drafts, traces, versions, source
references, candidate changes, and promoted state.

A chat-style input may appear where useful, but it is only an affordance. The
output of the system is durable work.

## Why This Repo Exists

This repo is the implementation side of Choir: runtime services, APIs,
frontend, tests, deployment, and control boundaries.

The public writing side currently lives at [Mosiah.org](https://mosiah.org/).
The long-term direction is for Choir itself to carry more of that writing and
publishing process directly.

For the deeper design frame, see:

- [docs/mission-geometry.md](docs/mission-geometry.md)
- [docs/computer-ontology.md](docs/computer-ontology.md)
- [docs/project-goals.md](docs/project-goals.md)

## Runtime Model

The implementation centers on persistent user computers and controlled
candidate work.

At a high level:

- each user computer is a persistent, stateful object;
- canonical state stays stable unless a change is promoted;
- risky or speculative mutation happens in candidate computers or worker worlds;
- appagents own durable app artifacts;
- workers produce evidence, deltas, candidates, or reports;
- AppChangePackages carry source changes between divergent computers;
- recipient computers rebuild and verify adopted changes themselves;
- promotion requires verification, owner acceptance, and rollback evidence;
- compaction preserves what a run learned for future inference.

A compact operating invariant:

```text
Evidence enters through researchers.
Meaning is owned by appagents.
Computation is orchestrated by super.
Mutation happens in candidate worlds.
Computers diverge.
Canonical state changes only by promotion.
```

The current self-development path is roughly:

```text
prompt bar -> conductor -> appagent/VText -> super
-> vmctl worker or candidate computer
-> AppChangePackage
-> recipient adoption and rebuild
-> verifier evidence
-> owner decision
-> promotion or rollback
```

The objective is to improve artifacts over time while minimizing corruption,
deadlock, human monitoring burden, and loss of understanding.

## Services

The stack has five Go services:

| Service | Port | Role |
| --- | --- | --- |
| `auth` | 8081 | Email/passkey registration, login, JWT access/refresh sessions |
| `proxy` | 8082 | Auth-gated HTTP/WebSocket proxy, user-context injection, VM routing |
| `vmctl` | 8083 | Desktop and worker VM ownership/lifecycle, with host-process fallback where Firecracker is unavailable |
| `gateway` | 8084 | Provider-neutral LLM/search gateway reachable by host/guest callers, not the public browser edge |
| `sandbox` | 8085 | Runtime service for desktop APIs, VText, Trace, files, terminal, browser sessions, and the agent/tool loop |

Every service exposes `/health`.

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
- ICU headers/libs for Go tests that touch Dolt-backed packages

Install frontend dependencies:

```sh
cd frontend
pnpm install
cd ..
```

Start the local stack when local iteration is appropriate:

```sh
./start-services.sh
```

The script uses local auth keys and service ports. For detailed manual service
startup, inspect `start-services.sh` and the relevant `cmd/*` package configs.

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
go test -count=1 ./internal/store ./internal/runtime
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
- Appagents own durable app artifacts; `vtext` is the current canonical semantic surface.
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
- `continuation-level`

Do not claim `promotion-level` without AppChangePackage adoption verifier
contract evidence plus owner review and promote/rollback evidence. Do not claim
`continuation-level` without run-memory/compaction and continuation evidence.

## Documentation Map

Start here:

- [AGENTS.md](AGENTS.md): repository agent operating contract.
- [docs/mission-geometry.md](docs/mission-geometry.md): high-level mission geometry and product ontology.
- [docs/computer-ontology.md](docs/computer-ontology.md): persistent computer, ledger, promotion, and update ontology.
- [docs/project-goals.md](docs/project-goals.md): current goal continuum and absorbed historical mission signal.
- [docs/glossary.md](docs/glossary.md): canonical vocabulary.
- [docs/README.md](docs/README.md): documentation index and cleanup status.
- [docs/current-architecture.md](docs/current-architecture.md): current architecture memo.
- [docs/frontend-app-building-api.md](docs/frontend-app-building-api.md): current frontend app registry, preview, theme, and shell contract.
- [docs/runtime-invariants.md](docs/runtime-invariants.md): implementation invariants.
- [docs/adr-dolt-as-canonical-state.md](docs/adr-dolt-as-canonical-state.md): Dolt/SQLite state-boundary decision.
- [docs/legacy-promotion-experiments-learnings.md](docs/legacy-promotion-experiments-learnings.md): consolidated lessons from pruned patchset-promotion experiments.
- [docs/implementation-scope.md](docs/implementation-scope.md): near-term scope and non-goals.
- [docs/north-star.md](docs/north-star.md): longer product direction.

Many stale dated proof files have been pruned. Preserve their reusable lessons
in consolidated docs instead of keeping obsolete success paths alive.

## Repository Shape

```text
cmd/                 service entrypoints
internal/auth/       passkey/JWT auth
internal/proxy/      auth-gated proxy and VM routing
internal/vmctl/      VM ownership/lifecycle API
internal/gateway/    LLM/search gateway
internal/runtime/    agent runtime, product APIs, VText/Trace/browser/control surfaces
internal/store/      runtime persistence plus embedded VText/Dolt workspace
frontend/            Svelte desktop and Playwright tests
nix/                 deployment and NixOS configuration
docs/                architecture, missions, proofs, and historical notes
```
