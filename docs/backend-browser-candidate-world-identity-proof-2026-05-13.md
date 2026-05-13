# Backend Browser Candidate-World Identity Proof - 2026-05-13

## Mission Pressure

Bounded host-process CDP control proved Choir can drive a backend browser session, but the session still had no candidate-world identity. For Choir-in-Choir, a Browser window must eventually be able to represent a background VM/candidate world without letting browser callers name raw vmctl handles or invoke vmctl control routes.

This slice adds the narrow identity bridge: a Browser session can bind to an owner-scoped promotion candidate by `promotion_candidate_id`, and the server derives VM/snapshot/source identity from the candidate queue record.

## Change

- Browser session records now persist candidate-world identity fields:
  - `world_kind`
  - `promotion_candidate_id`
  - `vm_id`
  - `snapshot_id`
  - `source_loop_id`
  - `candidate_trace_id`
- `POST /api/browser/sessions` accepts `promotion_candidate_id`.
- The public Browser API still rejects arbitrary `vm_id` fields because request decoding disallows unknown fields.
- The server looks up the promotion candidate under the authenticated owner before binding identity.
- Other owners cannot bind a Browser session to someone else's promotion candidate.
- Candidates without VM identity are rejected.
- Browser session Trace payloads include the candidate-world identity.
- Trace summarizes candidate-world Browser session creation with VM identity.
- Browser app exposes stable data attributes for world/candidate/VM/snapshot/source identity.

## Verification

- `gofmt -w internal/types/browser.go internal/store/store.go internal/store/browser.go internal/runtime/browser.go internal/runtime/api_trace.go internal/runtime/api_test.go`
- `CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test -count=1 ./internal/runtime ./internal/store -run 'TestBrowser|TestPromotion|TestHandleTraceTrajectory|TestRegisteredTrace|TestTraceRunGeometry|TestAppendAndListEvents|TestListEventsByOwner|TestListEventsByTrajectory'`
- `cd frontend && pnpm build`

The focused API proof queues a promotion candidate with VM and snapshot identity, proves a forged `vm_id` browser-session request is rejected, proves another owner cannot bind to the candidate, creates a Browser session by `promotion_candidate_id`, and verifies the persisted session plus Trace payload contain the derived candidate-world identity.

## Residual Risk

This is identity binding, not VM-local browser execution. The Obscura CDP process still runs in host-process scope. The next deformation should attach the browser substrate to the candidate VM lease or route a browser controller through the candidate sandbox, while keeping vmctl control endpoints internal-only.
