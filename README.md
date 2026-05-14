# go-choir

`go-choir` is the implementation repo for Choir: an artifact-native learning system built around persistent computers, versioned artifacts, appagents, candidate worlds, verification, promotion, and public memory.

The short version:

```text
Choir is a durable learning control system over versioned artifacts.
```

The product vector is:

```text
automatic computer -> automatic newspaper -> automatic radio -> automatic capital
```

- **Automatic computer**: the private persistent computer where users and agents work over durable artifacts, local apps, user-specific runtime state, and candidate branches.
- **Automatic newspaper**: the public memory layer where selected artifacts become citeable, disputable, forkable, and reusable.
- **Automatic radio**: the embodied traversal layer where artifact graphs become source-grounded audio experiences.
- **Automatic capital**: the later capital-formation layer where future-relevant contribution can route resources and upside.

This repo is the left brain: runtime, services, APIs, tests, deployment, and control boundaries. [Mosiah.org](https://mosiah.org/) is currently the right brain: public writing, artifact-native theory, and the visible field of ideas that Choir is meant to operationalize. The long-term work is to unify them: make the repo's substrate capable of carrying the Mosiah-style artifact graph directly.

For the higher-level frame, read [docs/mission-geometry.md](docs/mission-geometry.md).

## What Choir is

Choir is not a chat app. It is not even a chat-shaped product with better tools behind it. Chat may appear as an input/control affordance where useful, but the product output is not a discussion thread. The user works with documents, artifacts, sources, versions, candidate worlds, and promoted state.

A CMS, agent queue, desktop, publication tool, or radio app can be a projection of Choir. Chat is not the canonical projection; `vtext` and artifact state are.

The core science object is:

```text
durable learning control over versioned artifacts
```

The deployed product is currently a web desktop served from a user computer, with apps such as VText, Files, Browser, Trace, Terminal, Podcast, and Settings. Behind the desktop is a control system:

```text
prompt bar -> conductor -> VText/appagent -> super -> vmctl worker/candidate computer
-> worker export -> promotion candidate -> verification/owner decision -> promotion or rollback
```

The objective is to maximize verified artifact improvement over time while minimizing corruption, deadlock, human monitoring burden, and loss of understanding.

## Runtime model

The implementation centers on a few invariants:

- each user computer is a persistent, divergent stateful object;
- canonical state stays stable inside that computer unless promoted;
- risky or speculative mutation happens in background/candidate computers;
- appagents own durable app artifacts;
- workers produce evidence, deltas, candidates, or reports;
- personal computer state changes through local promotion after verification and rollback evidence;
- platform/public state changes through higher-ceremony promotion after verification and owner acceptance;
- compaction preserves what a run learned for future inference.

A compact operating invariant:

```text
Evidence enters through researchers.
Meaning is owned by appagents.
Computation is orchestrated by super.
Mutation happens in candidate worlds.
Computers diverge.
Canonical state changes only by promotion.
Radio is a traversal of promoted meaning.
```

## Services

The stack has five Go services:

| Service | Port | Role |
| --- | --- | --- |
| `auth` | 8081 | Email/passkey registration, login, JWT access/refresh sessions |
| `proxy` | 8082 | Auth-gated HTTP/WebSocket proxy, user-context injection, VM routing |
| `vmctl` | 8083 | Desktop and worker VM ownership/lifecycle, host-process fallback where Firecracker is unavailable |
| `gateway` | 8084 | Provider-neutral LLM/search gateway reachable by host/guest callers, not the public browser edge |
| `sandbox` | 8085 | Runtime service currently named sandbox: desktop APIs, VText, Trace, files, terminal, browser sessions, agent/tool loop |

Every service exposes `/health`. The sandbox service name is an implementation name, not the product ontology; the product object is a persistent computer. The sandbox health response includes build/deploy identity used by staging verification.

## Self-hosting and local development

The repo is still a fast-moving system, but the intended local shape is straightforward: run the Go services plus the Svelte frontend, backed by local service configuration and runtime state.

Requirements:

- Go 1.25+
- Node.js 22+
- pnpm 10+
- Nix for reproducible Linux builds and deployment configuration
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

The script uses local auth keys and service ports. For detailed manual service startup, inspect `start-services.sh` and the relevant `cmd/*` package configs.

Local development is useful for frontend iteration, focused unit shaping, and reproducing a transition identified by deployed evidence. It is not sufficient proof for claims about live vmctl behavior, provider credentials, background/candidate computers, platform promotion, rollback, or production deployment.

## Tests

Focused Go tests on macOS need ICU flags:

```sh
CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' \
CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' \
CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' \
go test -count=1 ./internal/store ./internal/runtime
```

Full local test shaping:

```sh
CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' \
CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' \
CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' \
go test ./... -count=1

cd frontend
pnpm run build
pnpm exec playwright test --workers=1
```

Documentation-only changes intentionally do not run automatic CI. The GitHub workflow ignores `docs/**` and top-level `*.md` for push and pull-request CI. Do not weaken those path filters just to make docs-only commits run CI.

## Deployment and staging proof

Platform behavior-changing work uses staging as the acceptance environment. A platform behavior-changing mission is not complete because local tests pass; it is complete when the pushed commit is running on staging and the deployed product path is verified there.

Personal-computer evolution is different. A user should be able to fork their own computer, build a new Go runtime or Svelte UI, install packages, add apps, change prompts, and promote that candidate back into their own active computer without waiting for a global platform deploy. That path still needs lineage, typed deltas, verifier evidence, and rollback, but its target is the user's active computer rather than `origin/main`.

Required landing loop for behavior changes:

```text
commit -> push origin main -> monitor CI -> monitor staging deploy
-> verify staging commit identity -> run deployed acceptance proof
```

The current staging host is an implementation detail for this deployment, not the conceptual center of the project. Keep staging-host specifics in deployment docs, mission reports, or environment configuration rather than making the README read like a product page for one temporary domain.

## Agent contract

Read [AGENTS.md](AGENTS.md) before using an agent to modify this repo. The short version:

- `conductor` routes exogenous user/app input and does not own semantic outcomes.
- Appagents own durable app artifacts; `vtext` is the current canonical semantic surface.
- `super` is the foreground orchestration root and mints bounded execution authority.
- Worker/candidate mutation belongs in background/candidate computers or isolated worker worlds.
- Canonical state changes only through explicit promotion after verification and owner acceptance.
- Verification is a contract and evidence record, not a separate agent caste.

## Run acceptance

The durable verifier is the Run Acceptance System:

- `POST /api/run-acceptances/synthesize` derives a `RunAcceptanceRecord` from existing runs, Trace tool results, worker exports, promotion candidates, build identity, and owner-scoped state.
- `GET /api/run-acceptances?trajectory_id=...` lists acceptance records for a trajectory.
- `GET /api/run-acceptances/{acceptance_id}` fetches one record.

Acceptance levels are explicit so the system does not overclaim:

- `docs-level`
- `staging-smoke-level`
- `export-level`
- `promotion-level`
- `continuation-level`

The current target for staging proof is at least `export-level`: prompt bar to VText to super to vmctl worker to worker export to promotion candidate, with rollback evidence and structured evidence refs.

## Documentation map

Start here:

- [AGENTS.md](AGENTS.md): repository agent operating contract.
- [docs/mission-geometry.md](docs/mission-geometry.md): high-level mission geometry and product ontology.
- [docs/computer-ontology.md](docs/computer-ontology.md): persistent computer, ledger, promotion, and update ontology.
- [docs/README.md](docs/README.md): documentation index and cleanup status.
- [docs/current-architecture.md](docs/current-architecture.md): current architecture memo.
- [docs/runtime-invariants.md](docs/runtime-invariants.md): implementation invariants.
- [docs/mission-choir-in-choir-controller-v0.md](docs/mission-choir-in-choir-controller-v0.md): current MissionGradient mission.
- [docs/mission-run-acceptance-verification-v0.md](docs/mission-run-acceptance-verification-v0.md): completed export-level run acceptance mission.
- [docs/implementation-scope.md](docs/implementation-scope.md): near-term scope and non-goals.
- [docs/north-star.md](docs/north-star.md): longer product direction.

Many dated `docs/*-proof-2026-05-13.md` files are evidence artifacts from earlier runs. Keep them as history unless a cleanup mission explicitly deletes or archives them.

## Repository shape

```text
cmd/                 service entrypoints
internal/auth/       passkey/JWT auth
internal/proxy/      auth-gated proxy and VM routing
internal/vmctl/      VM ownership/lifecycle API
internal/gateway/    LLM/search gateway
internal/runtime/    agent runtime, product APIs, VText/Trace/browser/control surfaces
internal/store/      runtime persistence plus embedded VText/Dolt workspace
internal/promotion/  candidate-world integration and promotion helpers
frontend/            Svelte desktop and Playwright tests
nix/                 deployment and NixOS configuration
docs/                architecture, missions, proofs, and historical notes
```
