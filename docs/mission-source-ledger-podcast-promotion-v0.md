# Mission: Source-Ledger Podcast Promotion v0

Date: 2026-05-18
Method: MissionGradient
Architecture inputs:
[computer-ontology.md](computer-ontology.md),
[stable-platform-divergent-computers-architecture-2026-05-17.md](stable-platform-divergent-computers-architecture-2026-05-17.md),
[legacy-promotion-experiments-learnings.md](legacy-promotion-experiments-learnings.md)

## Real Artifact

Create Choir's first useful app migration faculty.

The real artifact is not a source ledger by itself, and not a podcast app patch
by itself. It is one continuous app-change crossing:

```text
user A candidate computer
  -> one AppChangePackage identity
  -> user B candidate computer apply/rebase/build/verify/adopt
  -> official platform computer candidate/adoption/promotion
  -> rollbackable, product-visible evidence chain
```

During that crossing, the podcast app must materially improve. The app change is
the payload and the acceptance pressure. The source ledger, per-computer builds,
verifier contracts, Trace, and promotion records are the substrate that must
carry that payload without cheating.

This mission passes only if the same migrating podcast AppChangePackage identity
travels through the whole chain, or if a precise invariant-level blocker remains
after root-cause investigation, cognitive reframing, and at least one executable
safe probe has been attempted.

## Invariants

- Platform substrate changes still go through GitHub `main`, CI, deploy,
  staging identity, and deployed acceptance proof.
- Bootstrap substrate changes are allowed, but substrate-only success is not a
  pass for this mission.
- After bootstrap, podcast app publication/adoption/promotion must use
  product-visible source-lineage, package, adoption, Trace, and run-acceptance
  paths. Do not use a host deploy to fake user-computer promotion.
- Mutable app/runtime/UI development happens in candidate computers. Active user
  computers and the official platform computer are bases and promotion targets,
  not scratchpads.
- The mission preserves one continuous migrating `AppChangePackage` identity.
  Do not complete separate checklist stages that cannot be tied back to the same
  package, trace, source refs, and rollback records.
- Use a real source ledger for source movement. For v0, prefer one
  platform-managed private GitHub repository created or verified with `gh`, with
  refs or branches namespaced by computer and candidate. Dolt remains the
  structured product/control ledger, not the primary code source store.
- App sharing transfers source deltas, manifests, contracts, and provenance. It
  never copies another user's runtime/UI binaries as the install path.
- User B and platform computer runtime/UI artifact digests must come from actual
  recipient candidate builds. Deterministic placeholder digests are acceptable
  only as blocker evidence, not as passing proof.
- Runtime Go and Svelte UI artifacts are promoted as a matched pair with a
  protocol/contract check.
- Foreground-tail source movement is recorded at candidate start and cutover,
  including merge/rebase result and conflicts.
- Private VTexts, secrets, provider credentials, uploaded files, arbitrary user
  data, auth cookies, and per-user runtime state must not enter public package
  source refs.
- User A publisher, user B adopter, verifier, reporter, orchestrator, and
  platform admin roles remain distinct in records, even if the same operator
  performs multiple v0 actions.
- The official platform computer is the public/default distribution lineage.
  Promotion to it must update product-visible route/default-base records and
  verify the strongest feasible public route or new-user base behavior.
- Product proof uses public product APIs and visible staging paths. Browser-
  public internal/test shortcuts, raw event mutation, and manual success seeding
  are forbidden.
- A partial chain is not silently successful. If B adoption succeeds but platform
  computer promotion fails, the report must say the mission did not pass and
  name the exact failed boundary and rollback status.

## Value Criterion

Minimize the distance from "Choir can autonomously improve an app in one
computer, publish it, let another divergent computer adopt it by rebuilding from
source, and promote it to the official platform computer" while preserving
authority, privacy, rollback, and inspectability.

Penalize:

- substrate work that does not carry the podcast app;
- podcast feature work that bypasses source-lineage promotion;
- copied binaries or copied full computer branches;
- fake source refs, fake transclusion panels, fake registry rows, or JSON-only
  success;
- deterministic digests used as substitutes for actual build artifact hashes;
- platform deploys used as evidence for user B or platform-computer adoption;
- unclear role authority or self-verification;
- hidden private user state in source/package artifacts;
- route/default-base records that cannot be inspected or rolled back;
- final reports that summarize over missing evidence.

Reward:

- one package identity with source refs, manifest hashes, real build hashes,
  verifier results, foreground-tail accounting, Trace links, run acceptance, and
  rollback records;
- podcast app capability that a human can actually use on staging;
- a certificate that makes the chain reviewable without reading raw logs.

## Quality Gradient

Expected quality: solid.

Solid means:

- the source ledger exists or its creation is precisely blocked;
- source refs resolve through product-visible records;
- user A candidate changes are exported as one `AppChangePackage`;
- user B applies the package to a candidate source checkout, rebuilds recipient
  runtime/UI artifacts, verifies contracts, and adopts with rollback;
- the official platform computer adopts/promotes the same package if B adoption
  succeeds;
- the podcast app gains a real capability, not only a label or fixture;
- tests, Trace, run acceptance, and staging proof all point to the same chain;
- residual risks are narrow and named.

Substandard work:

- creating a private repo but not using it in the product path;
- adding podcast UI directly to platform `main` and calling that app migration;
- adding only placeholder provider rows or fixture-only search;
- accepting "digests differ" without hashing real recipient build outputs;
- using local tests as proof for staging behavior;
- treating platform lineage record updates as public route proof when no route
  or default-base behavior was observed.

If the chain succeeds early, use remaining time for verification density,
certificate quality, route/default-base proof, and one focused podcast usability
or reliability improvement. Do not branch into unrelated marketplace, payments,
social, or broad registry work.

## Podcast Payload

The podcast app should become more useful during the migration. The target is
not a full podcast product, but it must be real enough that the crossing matters.

Preferred feature slice:

- server-side provider-agnostic podcast search with an Apple/iTunes, gpodder, or
  configured-provider adapter, plus explicit fallback behavior;
- search UI in the podcast surface;
- import or subscribe from a search result into durable user content/state;
- one playable episode path using real feed metadata where possible;
- VText/radio-brief continuity for an imported/subscribed feed;
- tests or Playwright proof for search, import/subscribe, playback visibility,
  and VText continuity.

If external podcast providers are unavailable, do not fake success. Record the
provider blocker, then preserve the topology by proving the same provider
interface against a configured local or seeded feed source and mark external
provider verification as a residual risk.

## Homotopy Parameters

Increase realism continuously along these axes while preserving the same object:

- source ledger:
  `recorded source refs -> private GitHub ledger branches -> worker candidate
  checkouts -> actual apply/rebase/build refs`;
- app payload:
  `existing feed display -> search -> import/subscribe -> playback -> VText
  continuity`;
- build proof:
  `recorded intended digests -> actual Go/Svelte build outputs -> artifact
  hashes stored in adoption records`;
- authority:
  `single operator acting as A/B/admin -> distinct records -> distinct sessions
  or accounts where feasible`;
- promotion:
  `user B adoption -> platform computer adoption -> route/default-base proof ->
  rollback proof`;
- evidence:
  `unit/API tests -> Trace/run acceptance -> Playwright screenshots/DOM metrics
  -> human-readable certificate`.

A lower-resolution proof is valid only if it is a projection of the full system.
A fake island that cannot deform into real source-ledger app migration is not
progress.

## Belief State

Starting belief:

- deployed control-plane records exist for source lineage, packages, adoptions,
  verifier summaries, rollback profiles, Trace, and run acceptance;
- `gh` can create a private GitHub source-ledger repo under the authenticated
  account if one does not already exist;
- current runtime adoption logic records recipient-specific digests, but they
  are not yet produced by real per-computer Go/Svelte builds;
- platform computer promotion updates product-visible lineage/default-base
  records, but has not yet proven logged-out route switching or new-user fork
  behavior;
- the podcast surface already has feed display, audio controls, import from RSS,
  and VText/radio-brief continuity scaffolding, but lacks a real search/import
  path over a general podcast index.

Main uncertainties:

- exact source-ledger repo name, branch/ref convention, token placement, and
  deploy-time access pattern;
- how candidate computer checkouts should apply source deltas without mutating
  active computers;
- whether real Go/Svelte builds fit inside the current worker/candidate runtime
  budget;
- which podcast provider gives the most reliable free search path on staging;
- what product route/default-base switch can be proven in this platform slice.

Highest-impact uncertainty:

Whether a worker/candidate path can apply one package to B's source checkout,
produce real runtime/UI build artifacts, and feed those hashes back into
product-visible adoption records.

Next observation that reduces uncertainty:

Create or verify the private source ledger, push/fetch a candidate branch from a
controlled checkout, then run the smallest product-integrated apply/build/hash
loop for one package before expanding podcast capability.

## Investigation & Cognitive Reframing

Use investigate discipline before patching: reproduce, inspect evidence,
hypothesize root cause, patch the implicated layer, and verify the expected
state transition.

Before stopping on a nontrivial blocker, apply at least three route-changing
cognitive transforms:

- Atomicity transform: ask which single boundary in the A package -> B adoption
  -> platform promotion chain failed, and change the next probe to isolate that
  boundary instead of splitting the mission.
- Substrate-payload transform: if source-ledger work and podcast work diverge,
  force the next action to make the podcast package traverse the source-ledger
  path.
- Contamination transform: verify that no binary, secret, private data, or full
  user branch crossed the boundary by accident.
- Authority-shadow transform: separate publisher, recipient, verifier, reporter,
  orchestrator, and platform-admin authority in records even when operated by
  one Codex session.
- Route/default-base transform: distinguish "platform lineage says active" from
  "public route or new-user fork actually uses it," then choose the missing
  verifier.

Tactical blockers should trigger another autonomous probe when the next probe is
inside current authority. Examples: missing branch, build script mismatch,
provider timeout, absent API field, Trace omission, or verifier result too weak.

Invariant-level or external blockers require a stop with evidence. Examples:
cannot create or access any source ledger without credentials; promotion would
leak private source/data; route switch requires an authorization boundary not
available to the run; or repeated build attempts show candidate runtime cannot
execute the required toolchain without lower-level platform repair.

If a blocker defines an executable next probe inside the mission authority, run
that probe instead of ending.

## Receding-Horizon Control

Operate in 30 to 60 minute control intervals.

At each interval:

1. state the current belief about the chain;
2. choose the next mutation that most reduces chain divergence;
3. predict which evidence should change;
4. mutate one bounded subsystem or candidate state;
5. observe tests, logs, Trace, source refs, build outputs, product API records,
   and staging behavior;
6. update belief state;
7. continue, narrow, rollback, or stop only under the stopping condition.

Prefer the shortest probe that strengthens the full chain. Avoid isolated
polish that does not improve source-ledger migration, real builds, podcast
capability, or platform promotion evidence.

## Dense Feedback Channels

Use:

- `gh repo view` or `gh repo create` for private source-ledger existence;
- `git ls-remote`, `git fetch`, `git push`, branch/ref inspection, and commit
  hash checks for source-ledger truth;
- product APIs for source-lineage, packages, adoptions, promotions,
  continuations, Trace, and run acceptances;
- worker/candidate logs for apply/rebase/build;
- actual `go build` or repo-standard Go test/build commands for runtime output;
- actual Svelte/frontend build commands for UI output;
- artifact hashing commands over generated runtime/UI outputs;
- unit and API tests for source-ledger, package, adoption, verifier, rollback,
  and provider behavior;
- Playwright screenshots/DOM metrics for podcast search, import/subscribe,
  playback visibility, VText continuity, Trace readability, and route/default
  proof;
- staging `/health` and build identity after platform deploys;
- CI and deploy logs for platform substrate changes.

## Evidence Ledger

For every nontrivial claim, record:

```text
claim
evidence source
command or observation
artifact path or URL
result
uncertainty/caveat
promotion relevance
```

The final certificate must include:

- source-ledger repo name or URL and access mode;
- source refs for user A active/candidate, user B active/candidate/cutover, and
  platform computer active/candidate/cutover;
- AppChangePackage id, manifest hash, source delta hashes, and visibility;
- build commands and actual runtime/UI artifact hashes for user B and platform
  computer;
- verifier contracts and results;
- foreground-tail merge/rebase result;
- Trace trajectory id and relevant moment/event ids;
- run-acceptance id and level;
- podcast app feature evidence with screenshots/DOM metrics;
- route/default-base evidence or exact blocker;
- rollback refs and rollback command/API path;
- residual risks and next executable probe.

## Forbidden Shortcuts

- Do not split the mission into "source ledger first" and "podcast app later"
  and call the first half complete.
- Do not declare success when only substrate records changed.
- Do not declare success when only podcast code changed in platform `main`.
- Do not copy runtime/UI binaries from user A to user B or platform computer.
- Do not mutate active user computers directly.
- Do not use platform deploy to simulate user B adoption or platform-computer
  promotion.
- Do not use browser-public internal/test endpoints, raw event mutation, or
  manual seeded records as acceptance evidence.
- Do not use deterministic digest strings as passing build artifacts.
- Do not hide provider failures behind fixture-only podcast search.
- Do not build a marketplace, package browsing, ratings, payments, social graph,
  or broad registry governance.
- Do not include private user data, uploads, provider credentials, secrets, auth
  cookies, or unrelated source in package refs.
- Do not produce a chat-log-style success report that launders missing evidence.

## Rollback Policy

For platform substrate deploys:

- keep pushed commit SHA;
- monitor CI/deploy;
- verify deployed SHA;
- preserve git revert target and NixOS/deploy rollback evidence.

For source-ledger state:

- keep prior active refs for every computer;
- create candidate refs without deleting prior refs;
- record package source refs and manifest hashes;
- avoid force-pushing shared active refs unless the rollback record proves the
  previous ref and the operation is explicitly safe.

For user B and platform adoptions:

- promotion records must name previous active source ref, runtime digest, UI
  digest, route profile, default-base profile where applicable, and rollback API
  path;
- failed candidates must remain inspectable until evidence is captured;
- route/default-base switches must have a product-visible rollback profile.

## Learning Side-Channel

Record tactical learnings in tests, Trace annotations, or mission report.

Record target-level learnings in this mission doc or the broader architecture
doc if they change source-ledger, package, build, or promotion parameterization.

Stop and escalate invariant-level learnings before changing the architecture
identity. Examples: app sharing cannot be represented as source deltas, route
identity cannot be rollbackable, or privacy requires a different trust boundary.

## Stopping Condition

Success requires all of:

- one podcast AppChangePackage developed in a user A candidate computer;
- material podcast app improvement in that package;
- package source stored or referenced through the private source ledger;
- user B candidate applies/rebases the package from source;
- user B runtime/UI artifacts are actually built and hashed in the recipient
  context;
- user B verifies, adopts, and has rollback evidence;
- the same package is adopted/promoted into the official platform computer if B
  adoption succeeds;
- platform route/default-base behavior is verified at the strongest feasible
  product level, or a precise blocker is recorded after root-cause probes;
- Trace, VText where relevant, run acceptance, screenshots/DOM metrics, source
  refs, verifier results, build hashes, rollback refs, and certificate all agree.

A hard-blocker stop is valid only when:

- the failed boundary is named;
- root-cause probes and cognitive transforms changed the search route at least
  once;
- no safe executable probe remains inside the current authority boundary;
- mutated state is rolled back or explicitly left as inspectable failed
  candidate state;
- the next executable probe is stated.

Substrate-only progress, app-only progress, or B-only adoption without platform
promotion is not a passing outcome.

## Goal String

```text
/goal Run docs/mission-source-ledger-podcast-promotion-v0.md as a Codex-operated MissionGradient mission: make the first useful app migrate atomically. Create or verify the private GitHub source ledger, then use one continuous podcast AppChangePackage identity to develop a real podcast search/import/playback/VText-continuity improvement in a user A candidate computer, publish it, import/rebase/build/verify/adopt it in a user B candidate computer with real recipient Go/Svelte artifact hashes and rollback, then promote/adopt the same package into the official platform computer with product-visible route/default-base evidence. Do not split substrate and app into separate passes, copy binaries, mutate active computers directly, use platform deploy to fake user-computer promotion, stop at labels/JSON-only records, or hide provider/build/route failures. Bootstrap platform substrate through git/CI/deploy only when required, then prove the app crossing through product APIs, Trace, run acceptance, screenshots/DOM metrics, and a concise promotion certificate. Stop only on full-chain success or a named invariant-level blocker after root-cause probes, cognitive reframing, rollback refs, residual risks, and the next executable probe.
```
