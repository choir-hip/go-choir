# Choir-In-Choir Controller Evidence, 2026-05-14

Status: stopped on invariant-level blocker after deployed export-bridge proof.

## Landed Change

Behavior commit:

```text
43b74dbc58ae59c0f4a6ac67537a94bec475c868 Bridge worker exports into promotion evidence
```

The commit:

- inlines small worker `export_patchset` manifest and patch artifacts across the worker boundary;
- materializes inline worker artifacts into the parent runtime promotion artifact directory;
- records patchset SHA-256 in promotion candidate and run acceptance evidence;
- requires durable owner review before acceptance can rise to `promotion-level`;
- exposes vmctl lifecycle counts in `/health`;
- strengthens the deployed worker proof to assert parent-side promotion artifact evidence.

## Verification

Local verification:

```text
CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go vet ./...
CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test -count=1 ./internal/runtime -run 'TestQueuePromotionCandidates'
CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test -count=1 ./internal/runtime -run 'TestRunAcceptanceSynthesize'
go test -count=1 ./internal/vmctl
node --check frontend/tests/vtext-background-worker-demo.spec.js
git diff --check
```

Local broad runtime note: `go test -count=1 ./internal/runtime` exposed the existing timing-sensitive `TestSubmitWorkerUpdateWakeUsesSameDebouncedPath` failure under package-wide load; the same test passed when isolated. GitHub CI later passed `go test ./... -count=1`.

CI and deploy:

```text
GitHub Actions run: 25841716559
Build Frontend: passed in 11s
Go Vet + Test + Build: passed in 2m9s
Deploy to Staging (Node B): passed in 4m23s
```

Staging health after deploy:

```json
{
  "status": "ok",
  "service": "proxy",
  "upstream": "ok",
  "vmctl_routing": "enabled",
  "build": {
    "commit": "43b74dbc58ae59c0f4a6ac67537a94bec475c868",
    "deployed_at": "2026-05-14T04:31:47Z",
    "deployed_commit": "43b74dbc58ae59c0f4a6ac67537a94bec475c868"
  },
  "upstream_build": {
    "commit": "43b74dbc58ae59c0f4a6ac67537a94bec475c868",
    "deployed_at": "2026-05-14T04:31:47Z",
    "deployed_commit": "43b74dbc58ae59c0f4a6ac67537a94bec475c868"
  }
}
```

Deployed product-path proof:

```text
GO_CHOIR_RUN_BACKGROUND_WORKER_DEMO=1 GO_CHOIR_WORKER_DEMO_BASE_URL=https://draft.choir-ip.com pnpm exec playwright test tests/vtext-background-worker-demo.spec.js --workers=1 --reporter=line
1 passed (3.4m)
```

The proof used the public browser path and forbade browser requests to `/internal/*`, `/api/agent/*`, `/api/prompts`, `/api/test/*`, and `/api/events`.

## Acceptance Record

Acceptance:

```text
acceptance_id: runacc-453a102cfbfb532473c6
trajectory_id: f8e74825-e37d-446a-a19f-76bc17e9140a
acceptance_level: export-level
state: accepted
base_sha: 0096e9499cc9416fb7f9e356da8d0144095713e0
```

Checkpoints:

```text
submitted
vtext_opened
super_requested
worker_leased
worker_delegated
worker_delegated
worker_delegated
worker_delegated
export_observed
promotion_candidate_queued
rollback_available
```

Promotion evidence now includes parent-runtime artifact paths and patch digest, for example:

```text
candidate: 453b61a5-def4-4dc1-be48-39c5552ad7a1
status: queued
trace_id: f8e74825-e37d-446a-a19f-76bc17e9140a
source_loop_id: 10d4c4a9-b045-4188-950f-8ac86681109a
vm_id: vm-fd9c0912692b7ebbcf1afbba092d5817
manifest_path: /mnt/persistent/promotion-artifacts/453b61a5-def4-4dc1-be48-39c5552ad7a1/manifest.json
patchset_path: /mnt/persistent/promotion-artifacts/453b61a5-def4-4dc1-be48-39c5552ad7a1/changes.patch
patchset_sha256: d4ff2d788643ae1cf76df02561bc51658e418cee275b2d3c56ae3b52a11379ae
objective_fingerprint: ad432b1364a2a79c5c5182bb36cd6b501bace27051cce3f3422119d92d2a0235
```

The same run still queued multiple candidates from duplicate delegation/export attempts. That is now visible in acceptance evidence instead of hidden in logs.

## Blocker

Promotion-level acceptance could not be reached without violating mission invariants.

The deployed product path can now prove:

```text
prompt bar -> VText -> super -> worker VM -> export_patchset
-> parent promotion artifact materialization -> promotion candidate queued
-> rollback/discard available -> export-level acceptance
```

The deployed product path cannot yet prove:

```text
queued candidate -> verifier contract on parent-side integration repo
-> product-visible owner decision -> canonical promotion or verified discard
-> deployed promotion-level acceptance
```

The immediate blocker is structural, not just missing UI polish:

- worker exports are now parent-readable patch artifacts, but they are not tied to a parent-side integration worktree checked out at the candidate base;
- the current deployed demo candidate is a tiny worker-created repository, not a patch against the canonical `go-choir` staging checkout;
- verifier and promote operations still require internal verifier authority plus a `repo_path` on the runtime host;
- using `/internal/promotions/.../verify`, manually seeding promotion success, or calling a raw mutation path from browser-public Playwright would reward-hack the mission and violate the product-path invariant.

Therefore the run stops at an invariant-level blocker: promotion-level acceptance requires a first-class staging promotion workspace and verifier launcher that preserve the authority boundary, rather than an internal-route shortcut.

## Next Mission Gradient

The next deformation should build the missing promotion workspace/controller surface:

```text
public product signal -> worker candidate against go-choir base
-> parent promotion workspace under /var/lib/go-choir
-> verifier contract launch through bounded super/controller authority
-> product-visible owner review
-> verified discard or canonical promotion evidence
-> deployed promotion-level acceptance
```

Keep the current worker-artifact bridge and vmctl telemetry as substrate. Do not attempt continuation-level acceptance until promotion-level can be reached without internal-route bypass.
