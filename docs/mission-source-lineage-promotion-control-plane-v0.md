# Mission: Source-Lineage Promotion Control Plane v0

Date: 2026-05-17
Method: MissionGradient
Architecture input:
[stable-platform-divergent-computers-architecture-2026-05-17.md](stable-platform-divergent-computers-architecture-2026-05-17.md)

## Real Artifact

Implement the first product-safe source-lineage promotion control plane for
Choir: computer-owned source refs, candidate refs, app change packages, matched
runtime/UI artifact-pair records, foreground-tail accounting, verifier evidence,
adoption records, rollback profiles, Trace visibility, and run-acceptance
synthesis. Then use it to make the first real app migrate: develop the
podcasting app in a user A candidate computer, publish it as an
AppChangePackage, import/rebuild/verify/adopt it in a user B candidate
computer, and, if that succeeds, promote/adopt it into the official platform
computer.

## Invariants

- Platform substrate changes still go through GitHub `main`, CI, NixOS
  build/switch, deploy, staging identity, and deployed proof.
- User-computer changes do not require platform host deployment.
- Every user computer has a source lineage ref separate from platform `main`.
- Candidate refs fork from the target computer's active source ref.
- Mutable app/runtime/UI work happens inside candidate computers. Active user
  computers and the official platform computer are bases, authority contexts,
  and promotion targets, not scratchpads.
- App sharing transfers AppChangePackage deltas plus contracts, not another
  user's binaries or full branch.
- Runtime Go binary and Svelte UI bundle are promoted as a matched pair when an
  app crosses that boundary.
- Active foreground source updates that happen while a candidate is running are
  recorded at candidate start and cutover, with merge/rebase result and
  conflicts.
- Private VTexts, secrets, provider credentials, uploaded files, and arbitrary
  user data are never stored in source refs.
- Publishing an app change package records an artifact/provenance event; it does not
  mutate platform binaries.
- The public/logged-out surface and the new-user default base image are modeled
  as the official platform computer unless the change is true host substrate
  behavior.
- New user computers fork from the current official platform computer by
  default.
- Private user computers can later expose selected public route projections,
  but whole-computer public exposure is out of scope for v0.
- Future organization, school, community, hobbyist, or user-maintained default
  images are modeled as promoted computer distributions with their own policy.
- Automatic newspaper work should prefer user-land/candidate development and
  promotion to the platform computer or platformd publication; platform host
  deploy is reserved for shared substrate changes such as gateway APIs,
  vmctl/runtime protocols, auth/routing/security, or platformd service shape.
- Promotion requires verifier evidence, owner/platform decision, route/adoption
  record, rollback profile, Trace refs, and run-acceptance evidence.
- Product proof must use product APIs and visible staging path; no
  browser-public internal/test shortcuts.
- Worker, verifier, reporter, and orchestrator/meta-verifier roles remain
  distinct.

## Value Criterion

Minimize confusion between platform deployment, source publication, app
installation, personal promotion, platform-computer promotion, and host
substrate change while preserving source lineage, verifier evidence, rollback,
route identity, and inspectability.

Penalize:

- copying binaries across divergent computers;
- mutating platform `main` for user-local installs;
- treating an app change package as installed before recipient rebuild/verification;
- dropping foreground-tail updates during candidate cutover;
- hiding app deltas inside opaque artifacts;
- using host deploy as a proxy for computer promotion;
- evidence summaries that omit source refs, artifact digests, verifier results,
  or rollback profile.

## Quality Gradient

Expected quality: solid.

Solid means:

- additive schema/types for source-lineage and promotion/adoption evidence;
- product API inspection for the chain;
- Trace events for source refs, package manifests, builds, verifier results,
  foreground-tail merge/rebase, adoption, and rollback;
- tests for authority and cross-user isolation;
- one narrow real proof path;
- residual risks stated directly.

Substandard work:

- labels with no durable state;
- local-only fake registry;
- copied binaries as an install path;
- app install proof without matched runtime/UI pair evidence;
- platform deploy used to hide absence of user-computer promotion.

## Homotopy Parameters

Start low-resolution but real:

```text
user A active source ref
user A candidate computer
one podcast AppChangePackage manifest
user B active source ref
user B candidate computer
one matched runtime/UI artifact-pair record
one foreground-tail merge/rebase record
one verifier contract
one adoption record
one rollback profile
platform computer candidate/adoption record
```

Increase realism along these axes:

- user A package -> user B import;
- conflict requiring rebase/agentic repair;
- private, unlisted, public, and platform-blessed package visibility;
- selective public surface projection from a private computer;
- automatic newspaper app/workflow promoted from user land to platform computer;
- platform computer candidate route;
- new-user fork proof from the official platform computer;
- organization/school/community computer distribution;
- multiple packages;
- Node A/B route proof for platform computer;
- governance beyond admin approval.

## Migration Trajectory

The overnight success condition is not "records exist." It is:

```text
make the first app migrate
```

Use the existing podcasting app scaffold as the first real payload, while
keeping the source-lineage crossing as the mission center.

This section is a state-transition trajectory for one migrating
`AppChangePackage`, not a ladder of independent milestones. Do not treat
"package exists", "candidate built", "record exists", or "route changed" as
success by itself. Preserve one continuous app-change identity across source
refs, package refs, rebuilt artifacts, verifier results, authority decisions,
rollback records, Trace, and run acceptance.

1. Bootstrap any required source-lineage control-plane code through the normal
   platform deploy path. This is the only host-deploy amnesty.
2. Fork a user A candidate computer from user A's active source ref.
3. Develop the podcast app in that candidate until it has a real runtime/UI
   pair: at least one runtime endpoint, one Svelte route/window/panel, one
   persisted podcast/subscription/episode record, one protocol contract, and
   one contract test.
4. Export and publish the candidate change as a user A `AppChangePackage`
   under user A authority.
5. Import the package into a user B candidate computer forked from user B's
   active source ref.
6. Rebase/apply the package onto B's source lineage, rebuild B-specific runtime
   and UI artifacts, and run verifier contracts.
7. Before B adoption, advance B's active source ref with a tiny unrelated
   no-conflict foreground-tail change, then record merge/rebase evidence at
   cutover.
8. Adopt into user B with rollback, Trace, product-visible API records, and run
   acceptance evidence.
9. If B adoption succeeds, import/adopt/promote the same package into an
   official platform computer candidate, verifying public route profile and
   default-base profile where feasible.
10. Produce a concise promotion/adoption certificate and stop on migrated-app
   evidence or on an invariant-level blocker.

The app should be real enough that the crossing matters. It should not expand
into a full podcast product during this run: no marketplace, no broad provider
integrations, no audio upload/media pipeline, no provider credentials, and no
private user content in source refs.

## Starting Belief State

Known:

- worker source patch export/import exists;
- promotion candidate queue and server-owned workspace verification exist;
- platform VText publication has artifact/provenance/consent/route/rollback
  shape;
- recent promotion proof moved a workspace/user target, not `origin/main`.
- current product APIs already expose `/api/promotions`, `/api/trace/*`, and
  `/api/run-acceptances/*`; the v0 control plane should extend that family.

Uncertain:

- exact host path/config for the first platform-managed private bare Git source
  ledger;
- smallest schema/API slice for source-lineage and adoption records;
- first matched runtime/UI verifier contract;
- how much of app package shape should exist before the first proof.

Highest-impact uncertainty:

What minimal durable record shape can represent a computer-owned source ref, a
candidate app change package, a matched runtime/UI artifact pair, and an adoption
decision without overbuilding the app registry?

Next observation:

Inspect current promotion candidate records, platform publication records,
runtime source/build assumptions, and Trace/run-acceptance synthesis before
mutating.

## Investigation And Cognitive Reframing

When blocked, classify the blocker as tactical, target-level, invariant-level,
or external. Do not stop on tactical blockers when a safe probe remains.

Root-cause probes:

- source ref ownership and storage location;
- candidate checkout and base-ref choice;
- runtime/UI app contract boundary;
- foreground-tail merge/rebase accounting;
- artifact storage and digest identity;
- Trace and run-acceptance synthesis;
- product API authority boundary.

Apply these transforms before accepting a hard blocker:

- Boundary transform: platform substrate, user computer, candidate computer,
  platform computer, or package registry?
- State-machine transform: which transition is missing?
- Invariant transform: which object must not be mutated?
- App-pair transform: does runtime and UI still match?
- Value-of-information transform: what probe reduces uncertainty fastest?

If the blocker names an executable next probe inside current authority, run that
probe instead of ending.

## Receding-Horizon Control

Operate in short loops:

1. identify the next state transition to make inspectable;
2. predict the durable evidence expected;
3. implement within the smallest mutation radius;
4. run focused tests;
5. inspect Trace/API/run-acceptance output;
6. update belief state and choose the next transition.

Prefer additive records and product-visible inspection before broad registry or
binary deployment changes.

## Dense Feedback

Use:

- Go tests for source-lineage/promotion/adoption records;
- API tests for product-visible inspection;
- Trace assertions for package/build/verifier/adoption events;
- run-acceptance synthesis;
- staging prompt-bar proof after any platform behavior deploy;
- screenshots/DOM metrics only if UI behavior changes.

## V0 Source Backend

Use one platform-managed private bare Git repository or equivalent source
ledger for v0:

```text
source-ledger.git
  refs/choir/platform/main
  refs/computers/<computer_id>/active
  refs/computers/<computer_id>/candidates/<candidate_id>
  refs/platform-computers/default/active
  refs/platform-computers/default/candidates/<candidate_id>
```

Do not make user GitHub forks the default source backend. Mirroring/export can
come later.

## V0 Lifecycle States

App change package:

```text
draft -> exported -> published_private | published_unlisted | published_public
-> adoption_proposed -> candidate_applied -> built -> verified
-> owner_approved -> adopted -> rolled_back | archived
```

Platform computer promotion:

```text
proposal -> platform_candidate -> built -> public_route_verified
-> new_user_fork_verified -> approved -> active -> rolled_back | archived
```

Do not collapse these into one generic "installed" status.

## V0 Verifier Contracts

Required verifier contracts:

- source refs resolve and candidate forks from target active ref;
- package manifest hashes match retrieved artifacts;
- runtime_source_delta and ui_source_delta are present when the package changes
  app behavior;
- no private content, secrets, provider credentials, uploads, or arbitrary user
  data appear in source refs;
- runtime Go build passes;
- Svelte/UI build passes;
- runtime/UI protocol contract hash matches;
- schema migrations are dry-run checked or explicitly marked no-op;
- capability requests are recorded and not silently granted;
- no copied runtime/UI binaries from another user's computer;
- user B adopted runtime/UI digests are rebuilt for B and are not user A's
  artifact digests unless a future cache verifier explicitly proves a matching
  base/profile;
- adoption record names rollback source ref, runtime digest, UI digest, and
  route profile;
- foreground-tail merge/rebase result is recorded.

## Product API Sketch

Build on the existing product-visible family:

- `/api/promotions`
- `/api/trace/*`
- `/api/run-acceptances/*`

Minimal additive API shape:

```text
GET  /api/computers/{computer_id}/source-lineage
GET  /api/app-change-packages/{package_id}
POST /api/app-change-packages
POST /api/computers/{computer_id}/adoptions
GET  /api/adoptions/{adoption_id}
POST /api/adoptions/{adoption_id}/verify
POST /api/adoptions/{adoption_id}/promote
POST /api/adoptions/{adoption_id}/rollback
GET  /api/promotions/{promotion_id}
```

If this is too broad, implement the inspectable read side plus one command path.

## Role Separation

- Worker: produces candidate changes, build artifacts, and self-report.
- Verifier: independently inspects candidate state and evidence.
- Reporter: writes the human-readable promotion/adoption report.
- Orchestrator/meta-verifier: decides whether the evidence chain is adequate.

Worker self-report is evidence, but not sufficient verification.

## Threat Model

Primary threats:

- cross-user source or binary contamination;
- accidental publication of private source/data/files/prompts/uploads;
- capability escalation through package install;
- verifier spoofing or self-verification;
- route switch without rollback;
- platform substrate deploy used to hide user-local promotion gaps;
- new-user base image forked from an unverified platform computer;
- foreground-tail updates lost during candidate promotion.

## Evidence Ledger

For each nontrivial claim, record:

```text
claim
evidence source
command or observation
artifact path or API route
result
uncertainty/caveat
promotion relevance
```

Required final evidence:

- source refs;
- package manifest refs;
- runtime/UI artifact digests or explicit blocker;
- verifier contracts/results;
- route/adoption record;
- rollback profile;
- Trace refs;
- run acceptance id and level;
- staging identity if platform substrate code changed.

## Promotion/Adoption Certificate

The final report should be a certificate first and an appendix second.

Certificate fields:

```text
what changed
source refs
package refs
runtime digest
UI digest
verifier results
foreground-tail merge/rebase result
route/default-base changes
Trace refs
run acceptance refs
rollback path
residual risks
next executable probe
```

A useful failure has the same discipline:

```text
blocked at
reason
evidence
next probe
rollback/no-active-state-changed status
```

## Forbidden Shortcuts

- Do not copy user A's runtime/UI binaries into user B's computer.
- Do not develop the app directly in user A active, user B active, or the active
  platform computer. Fork candidates first.
- Do not require a GitHub account for default user-computer source lineage.
- Do not store private content, secrets, or uploads in Git refs.
- Do not mutate platform `main` to prove a user-local app install.
- Do not use internal/test routes as acceptance proof.
- Do not call a package installed before recipient rebuild, verification, and
  adoption.
- Do not promote without recording target active ref at candidate start and at
  cutover, plus merge/rebase result.
- Do not update the host platform when platform-computer promotion is the real
  target.
- Do not treat a computer distribution as safe for new users until forkability,
  source lineage, and rollback are verified.
- Do not expose a private computer publicly except through explicit route
  projection with visibility, provenance, and rollback.
- Do not use host deploy as the default path for automatic newspaper features
  that can be built and promoted in user/platform-computer land.
- Do not summarize missing evidence as success.

## Rollback Policy

For computer promotions, rollback names:

- previous active source ref;
- previous runtime artifact digest;
- previous UI artifact digest;
- previous route profile;
- candidate ref;
- package manifest ref;
- target active source ref at candidate start;
- target active source ref at cutover;
- foreground-tail merge/rebase evidence;
- verifier result that justified adoption;
- product action or command to restore the prior profile.

For platform substrate deploys, rollback remains git/deploy/NixOS generation
rollback.

## Goal String

```text
/goal Run docs/mission-source-lineage-promotion-control-plane-v0.md.
Build the simplest real implementation of divergent-computer app promotion.
Implement enough source-lineage, AppChangePackage, candidate adoption, matched
runtime/UI rebuild, verifier evidence, rollback, Trace, and product inspection
to support one real app crossing between computers. Preserve one continuous
migrating AppChangePackage identity rather than completing independent checklist
stages. Use the existing podcasting app scaffold as the first real payload:
develop it in a user A candidate computer, publish it as an AppChangePackage,
import/rebase/build/verify/adopt it in a user B candidate computer, then
promote/adopt it into the official platform computer if B adoption succeeds. Do
not build a marketplace, copy binaries between computers, use platform deploy
to fake user-computer promotion, mutate active computers directly, or stop at
labels/JSON-only records. Finish with a concise promotion/adoption certificate:
what changed, source refs, package refs, artifact digests, verifier results,
route/default-base changes, rollback path, residual risks, and next executable
probe.
```

## Learning Side-Channel

Record target-level learning in this mission doc or a follow-up architecture doc.

Classify surprises:

- tactical: adjust implementation and continue;
- target-level: update mission parameterization and continue if invariant holds;
- invariant-level: stop and escalate before changing architecture;
- external: stop with evidence and next safe probe.

## Stopping Condition

Stop only when:

- product APIs and Trace show the podcast AppChangePackage moving from user A
  candidate to user B candidate/adoption and, if feasible, into the platform
  computer candidate/promotion, as one continuous app-change trajectory with
  rollback, or
- a precise invariant-level blocker remains after root-cause probes and
  cognitive reframing, or
- an external authority boundary is hit.

Do not stop merely because the first implementation path is blocked if a safe
next probe exists.
