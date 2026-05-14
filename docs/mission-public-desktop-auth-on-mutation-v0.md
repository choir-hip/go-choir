# MissionGradient: Public Desktop And Auth-On-Mutation

**Status:** active next mission
**Created:** 2026-05-14

## Real Artifact

Optimize the deployed web desktop access model so `https://draft.choir-ip.com`
shows a real public desktop surface to signed-out visitors, while every mutable
move crosses an explicit auth-on-mutation boundary and then continues through
the normal product path.

The low-resolution artifact is still the real app:

```text
public desktop view
-> user mutation intent
-> auth overlay without losing intent
-> authenticated active/candidate computer
-> product-path mutation
-> visible progress state
```

The target is not a marketing landing page, a separate demo shell, or a local
mock. It is the production desktop topology with read access separated from
mutation authority.

## Invariants

- Public viewing does not require login.
- Anonymous visitors cannot perform durable mutation, LLM-backed writes,
  candidate creation, Dolt writes, file writes, promotion actions, or account
  changes.
- Login/register appears at the mutation boundary, not as a prerequisite to see
  the desktop.
- The user's typed mutation intent survives register/login/session renewal and
  is replayed only after the user explicitly completes auth.
- Mutation routes use the same product APIs and authority boundaries used by
  signed-in users. The overlay must not call browser-public internal routes or
  test-only endpoints.
- The signed-out public desktop resolves to platform/public read state, not to a
  private active computer.
- After auth, Choir creates or resumes the user's active computer, or a
  candidate computer when the mutation targets public/platform state.
- Platform/public mutation is not direct anonymous mutation of the platform
  computer. It becomes a user-owned fork/proposal/promotion path.
- Loading animation reflects real causal state when available: queued,
  authenticating, assigning computer, routing through conductor, waiting on LLM,
  writing artifact, verifying, done/error. Do not fake success progress.
- Logout remains reachable during loading or failure.
- Staging is the acceptance environment for behavior-changing work:
  commit -> push main -> monitor CI/deploy -> verify deployed identity -> run
  deployed product-path proof.
- Custom domains remain a compatibility pressure, not an implementation target
  for this mission. See [public-identity-and-custom-domains.md](public-identity-and-custom-domains.md).

## Value Criterion

Minimize user-visible friction from first page load to meaningful mutation while
preserving authority, state ownership, and verifier semantics.

Useful movement decreases:

- time before a signed-out visitor sees the actual desktop surface;
- number of auth prompts before clear mutation intent exists;
- lost input across auth/session transitions;
- black-screen or dead-end states during bootstrap, LLM calls, or errors;
- hidden mutable side effects before identity is known.

Penalties:

- bypass penalty for any mutable anonymous write path;
- regression penalty for breaking existing signed-in desktop, logout, passkey,
  or vmctl routing behavior;
- hidden-state penalty for auth overlays or animations that obscure real
  request/run state;
- Goodhart penalty for tests that assert only UI copy while the product path is
  bypassed or unobserved.

## Homotopy Parameters

Increase realism along these axes while preserving the same topology:

- Visitor identity: signed-out visitor -> newly registered user -> returning
  existing account -> expired-session returning account.
- Mutation intent: prompt-bar text -> app launch requiring saved state ->
  file/upload intent -> platform/public proposal intent.
- State target: read-only public platform state -> user's private active
  computer -> user candidate computer -> platform/public proposal.
- Provider realism: fake provider deterministic proof -> local stack product
  proof -> deployed staging proof with live routing and session renewal.
- Loading realism: static spinner -> stateful progress labels -> progress driven
  by submission/run/trace evidence.
- Failure realism: bootstrap delay -> auth renewal failure -> vmctl assignment
  delay -> LLM error -> verifier/replay failure.
- Routing realism: `draft.choir-ip.com` root -> future `choir-ip.com/:handle`
  public route -> verified custom domain route.

## Dense Feedback Channels

Use feedback that reveals where the topology is wrong:

- backend tests proving mutable APIs reject anonymous writes;
- auth tests proving register/login preserves and resumes pending prompt intent;
- desktop-state tests proving signed-out public state does not leak private
  user state;
- vmctl/proxy tests proving anonymous reads do not allocate private mutable
  computers, while post-auth mutation does create/resume the right computer;
- Playwright local tests for signed-out desktop visibility, auth overlay,
  prompt preservation, logout reachability, and loading/error states;
- deployed Playwright proof on `draft.choir-ip.com` covering new account and
  existing account flows;
- health/build identity checks proving staging is running the tested commit;
- trace/submission/run records for LLM-backed mutations, when the slice reaches
  actual conductor submission.

## Forbidden Shortcuts

- Do not replace the desktop with a marketing or placeholder page.
- Do not make anonymous mutable APIs work by assigning a shared anonymous user.
- Do not write signed-out prompt text into Dolt, SQLite, files, or run records
  before auth.
- Do not use browser-public internal routes such as `/api/agent/*`,
  `/api/prompts`, `/api/test/*`, `/internal/*`, or raw event mutation endpoints.
- Do not clear cookies or local state as the fix for returning-user failures.
- Do not hide bootstrap or LLM errors behind an infinite spinner.
- Do not make the auth overlay a separate app that loses the current desktop
  context.
- Do not implement custom domains in this mission unless the mission is
  explicitly reparameterized.
- Do not claim deployed proof from local tests.

## Rollback Policy

Every behavior-changing implementation commit must be revertable by git commit
SHA. Staging proof must record:

- pushed commit SHA;
- GitHub Actions run and deploy job;
- `/health` deployed proxy and sandbox commit identity;
- deployed acceptance command and result;
- any created test account/handle/domain-like route data;
- whether any durable prompt/run/desktop state was created;
- residual risk and rollback recommendation.

If the public desktop path exposes mutable anonymous state, leaks private state,
or blocks existing-account login/logout, stop and revert or patch forward before
continuing.

## Learning Side-Channel

Classify discoveries during the mission:

- Tactical learning: update implementation and tests directly.
- Target-level learning: update this mission doc with a better homotopy
  parameter or verifier.
- Invariant-level learning: stop and escalate before changing public/private
  state boundaries, anonymous write policy, platform/public promotion semantics,
  or auth trust boundaries.

Durable learnings should be folded into:

- this mission doc;
- [current-architecture.md](current-architecture.md);
- [runtime-invariants.md](runtime-invariants.md);
- focused tests that encode the discovered boundary.

## Stopping Condition

The mission is complete when staging proves:

- signed-out root shows the real desktop shell without login;
- signed-out mutation intent opens auth overlay instead of mutating;
- prompt-bar text survives register/login and resumes as an authenticated
  product-path submission;
- existing signed-in account still opens the desktop without black screen;
- logout remains reachable during loading/failure;
- relevant anonymous write APIs reject without side effects;
- loading states for at least one LLM-backed path are visible and tied to real
  request/submission/run state;
- deployed health reports the expected commit;
- residual risks are named plainly.

## Short Goal Prompt

Use MissionGradient. Complete
`docs/mission-public-desktop-auth-on-mutation-v0.md` by optimizing the real
deployed desktop access model under its invariants and verification criteria.
Preserve topology, avoid forbidden shortcuts, and stop/escalate on
invariant-level surprises.
