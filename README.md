# go-choir

Choir is a deployed web desktop for durable multiagent work over versioned artifacts. The visible product is an authenticated desktop with apps such as VText, Files, Browser, Trace, Terminal, Podcast, and Settings. The core product path is a control system:

```text
prompt bar -> conductor -> VText/appagent -> super -> vmctl worker/candidate world
-> worker export -> promotion candidate -> verification/owner decision -> promotion or rollback
```

The near-term goal is Choir developing Choir through that product path, with staging evidence strong enough that a reviewer can trust what happened without reading raw logs.

## Operating Model

`https://draft.choir-ip.com` is the staging acceptance environment. Behavior-changing work is not complete because local tests pass; it is complete when the pushed commit is running on staging and the deployed product path is verified there.

The required landing loop for behavior changes is:

```text
commit -> push origin main -> monitor CI -> monitor staging deploy
-> verify staging commit identity -> run deployed acceptance proof
```

Documentation-only commits intentionally do not trigger automatic CI/deploy. The GitHub workflow ignores `docs/**` and top-level `*.md` for push and pull-request CI. Do not remove those path filters just to make docs-only commits run CI. If docs need a check, run the specific check directly or use an explicit manual workflow when one exists.

Local development is for fast frontend iteration, narrow unit shaping, and reproducing a staging failure after staging evidence identifies the failing transition. Local proof does not satisfy claims about live vmctl behavior, gateway credentials, model/search calls, auth/session renewal, background/candidate VMs, promotion, rollback, or Choir-in-Choir product behavior.

## Services

The deployed stack has five Go services behind Caddy:

| Service | Port | Role |
| --- | --- | --- |
| `auth` | 8081 | Email/passkey registration, login, JWT access/refresh sessions |
| `proxy` | 8082 | Auth-gated HTTP/WebSocket proxy, user-context injection, VM routing |
| `vmctl` | 8083 | Desktop and worker VM ownership/lifecycle, host-process fallback where Firecracker is unavailable |
| `gateway` | 8084 | Provider-neutral LLM/search gateway reachable by host/guest callers, not the public browser edge |
| `sandbox` | 8085 | Runtime, desktop APIs, VText, Trace, files, terminal, browser sessions, agent/tool loop |

Every service exposes `/health`. The sandbox health response includes build/deploy identity used by staging verification.

## Agent Contract

Read [AGENTS.md](AGENTS.md) before using an agent to modify this repo. The short version:

- `conductor` routes exogenous user/app input and does not own semantic outcomes.
- Appagents own durable app artifacts; `vtext` is the current canonical semantic surface.
- `super` is the foreground orchestration root and mints bounded execution authority.
- Worker/candidate mutation belongs in background VMs or isolated worker worlds.
- Canonical state changes only through explicit promotion after verification and owner acceptance.
- Verification is a contract and evidence record, not a separate agent caste.

## Run Acceptance

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

## Development Setup

Requirements:

- Go 1.25+
- Node.js 22+
- pnpm 10+
- Nix for Node B deployment config and reproducible Linux builds
- ICU headers/libs for local Go tests that touch Dolt-backed packages

Install frontend dependencies:

```sh
cd frontend
pnpm install
cd ..
```

Start the local stack only when local iteration is appropriate:

```sh
./start-services.sh
```

The script uses local auth keys and service ports. For detailed manual service startup, inspect `start-services.sh` and the relevant `cmd/*` package configs.

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

Opt-in deployed worker acceptance proof:

```sh
cd frontend
GO_CHOIR_RUN_BACKGROUND_WORKER_DEMO=1 \
GO_CHOIR_WORKER_DEMO_BASE_URL=https://draft.choir-ip.com \
pnpm exec playwright test tests/vtext-background-worker-demo.spec.js --workers=1
```

That Playwright proof must go through the visible prompt bar and authenticated product APIs. It must not call browser-public internal orchestration routes such as `/api/agent/*`, `/api/prompts`, `/api/test/*`, `/internal/*`, or raw event mutation endpoints.

## Deploy

Push behavior-changing commits to `origin/main`. GitHub Actions runs Go vet/test/build, frontend build, then deploys Node B via NixOS rebuild and health checks.

After CI/deploy, verify:

```sh
curl -s https://draft.choir-ip.com/health
```

The reported commit/deployed commit must match the pushed behavior-changing commit before staging acceptance evidence can count.

## Documentation Map

Start here:

- [AGENTS.md](AGENTS.md): repository agent operating contract.
- [docs/README.md](docs/README.md): documentation index and cleanup status.
- [docs/current-architecture.md](docs/current-architecture.md): current architecture memo.
- [docs/runtime-invariants.md](docs/runtime-invariants.md): implementation invariants.
- [docs/mission-choir-in-choir-controller-v0.md](docs/mission-choir-in-choir-controller-v0.md): current MissionGradient mission.
- [docs/mission-run-acceptance-verification-v0.md](docs/mission-run-acceptance-verification-v0.md): completed export-level run acceptance mission.
- [docs/implementation-scope.md](docs/implementation-scope.md): near-term scope and non-goals.
- [docs/north-star.md](docs/north-star.md): longer product direction.

Many dated `docs/*-proof-2026-05-13.md` files are evidence artifacts from earlier runs. Keep them as history unless a cleanup mission explicitly deletes or archives them.

## Repository Shape

```text
cmd/                 service entrypoints
internal/auth/       passkey/JWT auth
internal/proxy/      auth-gated proxy and VM routing
internal/vmctl/      VM ownership/lifecycle API
internal/gateway/    LLM/search gateway
internal/runtime/    agent runtime, product APIs, VText/Trace/browser/control surfaces
internal/store/      SQLite runtime store plus embedded VText workspace
internal/promotion/  candidate-world integration and promotion helpers
frontend/            Svelte desktop and Playwright tests
nix/                 Node B NixOS configuration
docs/                architecture, missions, proofs, and historical notes
```
