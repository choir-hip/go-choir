# MissionGradient: Platform Promotion Substrate v0

Status: ready for execution
Date: 2026-05-17
Operator: outer Codex supervising Choir through staging, git, CI, deploy, Playwright, Trace, VText, promotion, continuation, and worker-VM evidence

## One-Line Goal String

```text
/goal Run docs/mission-platform-promotion-substrate-v0.md as a Codex-operated MissionGradient mission: build the missing product-safe platform promotion substrate for Choir self-development. Start from the current export-level worker path and create a first-class promotion workspace/controller that can take a queued worker export, apply it against the canonical go-choir base, run verifier contracts, expose owner/platform review evidence through product APIs and Trace, then land through git/CI/deploy or precisely block without internal/test route shortcuts. Use a narrow Podcast discovery/playback improvement only as the product-pressure candidate after promotion substrate is real enough: server-side provider-agnostic podcast search with Apple/gpodder/configured provider fallback, durable feed import/subscription, one playable episode path, and VText radio-brief continuity. Finish with VText, Trace, run-acceptance, promotion or blocker evidence, rollback refs, staging identity, screenshots/DOM metrics, residual risks, and next objective.
```

## Mission Shift

The current Choir-in-Choir path can reach `export-level`: visible staging prompt
bar work can route to VText, super, worker VM vsuper, co-super channels,
`export_patchset`, a queued promotion candidate, rollback refs, run acceptance,
and continuation evidence.

That is not yet platform promotion. The missing object is a product-safe
promotion workspace/controller that imports a queued worker export against the
canonical `go-choir` base, runs verifier evidence, records owner/platform review
state, and either prepares a real platform landing path or blocks precisely.

Podcast work is the pressure workload, not the priority. Do not build podcast
search/playback directly in foreground platform code until the promotion
substrate is realistic enough to review a queued candidate.

## Current Belief State

Deployed baseline entering this mission:

- latest landed commit: `d32aabae9c2fdc50b77cfd4dd625c4a63039ab05`
- staging: `https://draft.choir-ip.com`
- latest accepted trajectory: `f8ab9962-1822-432c-ae7e-e80d2d720bec`
- latest accepted run acceptance: `runacc-4e24b55ad445c580e3fc`
- level reached: `export-level`
- continuation proof exists through run-memory event-ledger compaction, but
  platform promotion remains unproved.

Existing substrate:

- `delegate_worker_vm` can queue parent-visible promotion candidates from worker
  exports.
- `/api/promotions` lists owner-scoped candidates.
- `/api/promotions/{candidate_id}` exposes candidate detail.
- `/api/promotions/{candidate_id}/approve` and `/reject` record product-visible
  owner review.
- Trace can link promotion queue events to candidate artifacts.

Known gap:

- verifier/import/promotion actions still depend on internal routes and an
  arbitrary caller-supplied `repo_path`;
- queued worker exports usually carry no product-safe verifier contract;
- there is no server-owned integration workspace rooted in canonical
  `go-choir` source identity;
- there is no public product path for "verify this queued candidate against the
  canonical base";
- a browser-visible API must not run arbitrary worker-supplied shell commands or
  accept a repo path from the client.

## Real Artifact

The artifact is a deployed, reviewable platform-promotion substrate:

```text
visible staging prompt bar
-> VText mission/report
-> conductor -> super
-> worker VM vsuper
-> implementation/verifier co-super channels
-> export_patchset
-> queued promotion candidate
-> server-owned promotion workspace at canonical go-choir base
-> safe verifier evidence
-> owner/platform review through /api/promotions and Trace
-> promotion-ready patch or precise blocker
-> run acceptance, rollback refs, continuation evidence
```

The podcast improvement is a later candidate that must flow through this shape
rather than bypassing it.

## Invariants

- Staging `https://draft.choir-ip.com` is the acceptance environment for worker
  VM, promotion, Trace, VText, run acceptance, auth/session, gateway/model, and
  deploy claims.
- Platform behavior changes complete:

```text
commit -> push origin main -> monitor CI -> monitor staging deploy
-> verify staging commit identity -> run deployed acceptance proof
```

- Browser/public acceptance uses visible product surfaces and public
  authenticated product APIs only: `/api/prompt-bar`,
  `/api/prompt-bar/submissions/*`, `/api/vtext/*`, `/api/trace/*`,
  `/api/promotions/*`, `/api/continuations/*`, and
  `/api/run-acceptances/*`.
- Do not use browser-public internal/test success paths: `/api/agent/*`,
  `/api/prompts`, `/api/test/*`, `/internal/*`, raw event mutation endpoints,
  direct service ports, or manually seeded success records.
- Public promotion verification must not accept `repo_path` from the browser and
  must not run arbitrary worker-supplied shell commands.
- Canonical state changes only after verifier evidence plus explicit review.
- Do not claim `promotion-level` without verifier contract evidence, owner
  review, promotion or rollback/discard evidence, and durable product refs.
- Do not claim `continuation-level` without run-memory/compaction and
  continuation evidence.
- No fake-island placeholders, fake transclusion panels, fake candidate refs,
  fake verifier transcripts, or summaries that launder missing evidence into
  success.

## Value Criterion

Minimize:

```text
promotion ambiguity + trust-boundary leakage + local-only proof
+ hidden candidate artifacts + unsafe verifier execution + UX/product drift
```

while preserving canonical state, rollback, staging evidence, and product-path
review.

The mission moves uphill when:

- queued worker exports can be imported into a server-owned integration
  workspace derived from canonical `go-choir` source identity;
- product APIs expose verification status, report JSON, integration branch,
  rollback refs, and owner decisions without internal shortcuts;
- Trace and run acceptance can explain why a candidate is queued, verified,
  rejected, promoted, or blocked;
- verifier execution is useful but bounded by the public API trust boundary;
- a narrow podcast search/playback patch can be attempted as a worker export
  only after the promotion substrate is real enough.

The mission moves downhill when:

- `export-level` is described as platform promotion;
- a public API accepts a caller-provided repo path;
- public verification executes worker-authored shell without an additional
  sandbox/trust boundary;
- local tests are claimed as staging worker/candidate proof;
- podcast UX patches land directly and bypass the candidate/promotion path.

## Receding-Horizon Control

1. Inspect the current promotion queue, internal verifier path, Trace
   projection, and run-acceptance level rules.
2. Add the mission doc and patch the smallest product-safe promotion workspace
   controller that can verify a queued export against canonical `go-choir`.
3. Keep older internal verifier/promote diagnostics available, but make the
   public product verifier server-resolved and bounded.
4. Add focused tests for owner scope, no browser `repo_path`, workspace import,
   verifier report persistence, promotion events, and run-acceptance evidence.
5. Commit, push main, monitor CI/deploy, and verify staging identity.
6. Rerun a visible staging prompt-bar Choir-in-Choir workload and use the
   product promotion API to verify/review the queued candidate if available.
7. Only then use a podcast discovery/playback improvement as the pressure
   candidate, or record a precise blocker if candidate promotion is still not
   safe.

## Podcast Pressure Candidate

When the promotion substrate is ready enough, the narrow candidate should be:

- server-side provider-agnostic podcast search endpoint;
- configured provider first, then Apple/gpodder fallback when no configured
  provider is available;
- durable feed import/subscription using existing content/feed records;
- one episode path that renders playable audio through the product UI;
- VText radio-brief continuity for imported/subscribed feeds.

Do not treat the specific phrase "Podcast Index" as a SaaS requirement. The
requirement is a general podcast index/search capability with a provider
abstraction.

## Dense Feedback

- Local: focused Go tests for runtime promotion API/workspace behavior, store
  queue behavior, run acceptance, and Trace artifact projection.
- CI: GitHub Actions run for pushed SHA.
- Deploy: `/health` build identity for proxy and sandbox.
- Product proof: Playwright against `https://draft.choir-ip.com` using visible
  prompt bar and public product APIs.
- Evidence artifacts: VText doc/revisions, Trace snapshot, promotion candidate
  detail/report, run acceptance record, rollback refs, desktop/mobile
  screenshots and DOM metrics.

## Rollback Policy

- Platform rollback: revert pushed commits or deploy the previous known-good
  SHA.
- Candidate rollback: reject/discard the candidate and preserve the report as
  evidence.
- Promotion workspace rollback: delete only server-owned candidate workspaces
  under the configured promotion workspace root; never mutate canonical source
  as part of verification.
- If a verifier result is wrong, append corrected verification/review evidence;
  do not erase the old evidence.

## Stopping Condition

Stop when one of the following is true:

- `promotion-level`: a queued candidate is verified through the product-safe
  promotion workspace, reviewed through product APIs, exposed in Trace/VText,
  and run acceptance records verifier/owner/rollback evidence honestly.
- `platform landing`: a reviewed candidate or substrate patch lands through
  git/CI/deploy with staging identity and deployed acceptance proof.
- `hard blocker`: after root-cause probes and cognitive search-space
  transforms, an external/invariant blocker remains, with exact evidence,
  rollback refs, residual risks, and the next executable probe.
