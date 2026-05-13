# Backend Browser VM-Local Execution Blocker - 2026-05-13

## Current Evidence

Choir now has:

- backend Obscura snapshots;
- persisted HTML/link/text/screenshot artifacts;
- persistent host-process CDP sessions;
- bounded `fill` and `click` controls;
- Browser session candidate-world identity derived from owner-scoped promotion candidates.

The remaining target is stronger: a Browser window that actually runs in, or is routed through, a background candidate VM.

## Blocker

Implementing VM-local Browser execution directly from the current Browser API would cross an authority boundary.

Unsafe shortcuts would include:

- accepting `vm_id`, `worker_sandbox_url`, or raw vmctl handles from browser callers;
- exposing vmctl lookup/control routes through `/api/browser`;
- letting frontend code choose a worker sandbox URL;
- treating host-process Obscura CDP as if it were running inside the candidate VM.

The current safe Browser API can bind to a promotion candidate by `promotion_candidate_id`, but that is only metadata binding. It does not give the runtime a safe server-to-server browser controller inside the candidate VM.

The missing internal substrate is:

- a durable worker lease reference on promotion/candidate records, including typed `worker_id` and lease/epoch identity;
- an internal-only resolver from candidate/worker identity to the active worker sandbox/controller endpoint;
- an internal Browser controller route in the worker sandbox, or a vmctl-mediated browser controller, with server-to-server authority rather than browser-public authority;
- verifier contracts proving the foreground Browser session cannot route to another owner's worker and cannot mint/modify VM ownership.

## Why This Stops The Current Deformation

This is an invariant-level risk because the next naive patch could let browser clients name internal execution targets. That would violate the mission rule that browser-public routes express product semantics only and must not expose vmctl/control-plane authority.

Stopping here is better than pretending candidate-world identity equals VM-local browser execution.

## Next Smallest Safe Probe

The next coding run should implement an internal VM browser lease resolver, not a public Browser control shortcut.

Minimum safe probe:

1. Add typed `worker_id` and lease/epoch identity to candidate-world records when a worker export queues a promotion candidate.
2. Add an internal-only vmctl lookup by worker ID or VM ID that returns ownership state and sandbox URL only to internal callers.
3. Add a runtime-side resolver that, given an owner-scoped promotion candidate, resolves a currently active worker lease without exposing the sandbox URL to the browser.
4. Add tests proving:
   - browser-public requests cannot pass `vm_id`, `worker_id`, or `worker_sandbox_url`;
   - another owner cannot resolve a candidate worker;
   - inactive or mismatched worker leases fail closed;
   - no vmctl route becomes browser-public.
5. Only after that, add an internal Browser controller route for the worker sandbox or a vmctl-mediated browser controller.

## Verification Already Completed

- `git diff --check`
- `bash -n start-services.sh`
- `cd frontend && pnpm build`
- `CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test -count=1 ./cmd/sandbox ./internal/runtime ./internal/store ./internal/promotion ./internal/vmctl`
- `GO_CHOIR_RUN_OBSCURA_CDP=1 CHOIR_OBSCURA_BIN=/Users/wiz/obscura/target/release/obscura CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test -count=1 ./internal/runtime -run 'TestCaptureObscuraCDPScreenshotLive|TestRuntimeReusesObscuraCDPSessionLive|TestRuntimeControlsObscuraCDPSessionLive' -v`
- `CHOIR_OBSCURA_BIN=/Users/wiz/obscura/target/release/obscura CHOIR_OBSCURA_CDP_SCREENSHOTS=1 CHOIR_SERVICES_FOREGROUND=1 ./start-services.sh`
- `cd frontend && GO_CHOIR_RUN_OBSCURA_CDP_BROWSER=1 npx playwright test browser-backend-obscura-cdp.spec.js --workers=1 --timeout=120000`
