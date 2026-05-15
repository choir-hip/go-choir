# Promotion Consent Layer Research

Date: 2026-05-15

## Executive Summary

Choir needs a typed promotion system, not one generic "merge" button.

The current sweep proved useful multi-agent substrate, but it did not produce
the original UX/onboarding improvements as account-local candidate code. The
account-local candidate changed only `README.md` with a proof marker. The real
platform improvements from the sweep were made by outer Codex directly on
`origin/main` and deployed through CI/CD.

That distinction is the design imperative:

```text
user/candidate computer change
  -> typed artifact proposal
  -> verifier evidence
  -> consent policy
  -> personal adoption, publication, or platform integration
```

The consent layer must be typed by promoted artifact kind:

- VText/publication promotion uses platform Dolt service state, provenance, and
  publication review.
- App/appagent promotion uses package, permissions, schema, install, and
  capability review.
- Personal computer promotion uses route switching, foreground-tail replay, and
  rollback.
- Source/build/platform promotion uses git patchsets, CI, staged deployment,
  security attestations, and maintainer/platform approval.

The near-term practical step is to build a source/build promotion queue that
turns worker-exported patchsets into reviewable, verifiable proposals. That
should be followed by a VText publication service backed by a platform-level
Dolt service, not embedded per-user Dolt.

## Node B State Review

### Method

I first attempted to re-query the exact QA account state through Playwright
against `https://draft.choir-ip.com`, using the saved auth state for:

```text
qa-1778850339483-ns7lyk@example.com
user_id=4f0bcb53-c775-4876-af10-7b36191fc428
```

The browser session was no longer authenticated:

```text
/auth/session -> {"authenticated": false}
/api/promotions -> 401 authentication required
/api/vtext/... -> 401 authentication required
/api/trace/... -> 401 authentication required
/api/run-acceptances/... -> 401 authentication required
```

This is an important product gap: the system does not yet have a durable
operator/product inspection route for an owner-approved promotion review after a
temporary QA browser session expires.

I then used read-only Node B inspection to review the active computer and worker
computer state. I did not edit tracked deployed files on Node B.

### Node B Ownership

`/var/lib/go-choir/vm-state/ownerships.json` shows two relevant computers:

```text
active user computer:
  vm_id=vm-cc1759bcb4816577941293c8570b1d72
  user_id=4f0bcb53-c775-4876-af10-7b36191fc428
  kind=interactive
  state=active
  sandbox_url=http://172.52.0.2:8085

worker/candidate computer:
  vm_id=vm-9c2141693c16303ea239b34e75098a77
  user_id=4f0bcb53-c775-4876-af10-7b36191fc428
  kind=worker
  worker_id=worker-4e574391cb3d8640
  trajectory_id=547ba472-c87f-4e3a-b52f-c19941e0e3b7
  purpose=Bounded Choir-in-Choir sweep substrate proof for marker
          SWEEP_837B8C7_COSUPER_1778850435886
  state=active
  sandbox_url=http://172.53.0.2:8085
```

Both sandboxes report platform commit:

```text
837b8c7bb26d195259360d9412abcb9a78f7f415
```

### Active Computer Records

The active user computer runtime DB records the promotion candidate:

```text
candidate_id=663324d3-637f-4879-9fda-56e497c02dbe
owner_id=4f0bcb53-c775-4876-af10-7b36191fc428
status=queued
trace_id=547ba472-c87f-4e3a-b52f-c19941e0e3b7
vm_id=vm-9c2141693c16303ea239b34e75098a77
base_sha=837b8c7bb26d195259360d9412abcb9a78f7f415
worker_head_sha=6ae48b2a3327eeee5e5dabdadb562dc925c5e8a8
manifest_path=/mnt/persistent/promotion-artifacts/663324d3-637f-4879-9fda-56e497c02dbe/manifest.json
patchset_path=/mnt/persistent/promotion-artifacts/663324d3-637f-4879-9fda-56e497c02dbe/changes.patch
integration_branch=agent/d6fe0776-01fc-48d8-935e-7aa2ab8f6dbe/candidate
destination_branch=main
```

The active user computer also records acceptance:

```text
acceptance_id=runacc-e61c8ae67af2568215c7
acceptance_level=export-level
state=accepted
health_commit=837b8c7bb26d195259360d9412abcb9a78f7f415
```

The acceptance checkpoints include:

```text
submitted
vtext_opened
super_requested
worker_leased
worker_delegated
export_observed
promotion_candidate_queued
rollback_available
```

The active computer trace has:

```text
event_count=144
channel.message=9
tool.invoked=37
tool.result=37
promotion.candidate.queued=1
vtext.document_revision.created=4
```

The VText-to-super channel shows the intended flow: VText requested the worker
VM sweep, super returned grounded findings, and VText stored the canonical proof
revision. A later attempt to add another revision failed with:

```text
Unable to store a new canonical revision: the VText mutation window is already completed.
```

That is not catastrophic, but it is a real workflow signal: VText finalization
can close before late worker details arrive. Promotion review should not depend
on "edit one more VText revision" after closure.

### Worker/Candidate Computer Filesystem

The worker VM persistent disk contains:

```text
/files/go-choir-candidate
/files/patchsets/SWEEP_837B8C7_COSUPER_1778850435886
```

The exported patch is:

```diff
diff --git a/README.md b/README.md
index f47182f..ab110b8 100644
--- a/README.md
+++ b/README.md
@@ -218,3 +218,8 @@ frontend/            Svelte desktop and Playwright tests
 nix/                 deployment and NixOS configuration
 docs/                architecture, missions, proofs, and historical notes
 ```
+
+
+
+
+SWEEP_837B8C7_COSUPER_1778850435886
```

The manifest reports:

```text
run_id=42ef3a7b-13c5-45fc-9527-f09e2172ab00
trace_id=547ba472-c87f-4e3a-b52f-c19941e0e3b7
vm_id=vm-9c2141693c16303ea239b34e75098a77
base_sha=837b8c7bb26d195259360d9412abcb9a78f7f415
expected_head_sha=6ae48b2a3327eeee5e5dabdadb562dc925c5e8a8
verification=grep marker in README.md passed
```

Direct read-only inspection of the candidate checkout confirms:

```text
README.md line 225 contains SWEEP_837B8C7_COSUPER_1778850435886
.git/HEAD = 6ae48b2a3327eeee5e5dabdadb562dc925c5e8a8
main ref = 837b8c7bb26d195259360d9412abcb9a78f7f415
```

### What Actually Changed

Account-local candidate change:

- One README marker line plus blank lines.
- No onboarding change.
- No prompt bar layout fix.
- No UX/aesthetic work.
- No podcast app improvement.
- No VText publication service.
- No app/appagent publication system.
- No platform Dolt service.

Platform changes already deployed globally by outer Codex:

- `f04877a` kept worker status reads observable with a separate read DB pool.
- `fd84355` enforced `request_worker_vm -> delegate_worker_vm` handoff.
- `837b8c7` exposed worker delegation topology evidence and tightened vsuper
  prompt behavior.

The mission achieved a substrate proof, not the original UX sweep objective.

## Cognitive Transforms

Current uncertainty:

How should Choir convert private computer changes into shared/public/platform
changes without collapsing personal freedom, security, consent, provenance, and
platform stability into one brittle GitHub-like flow?

Selected transforms:

1. **Depth Extraction**: "promotion" is not merge. The deep object is typed
   state transition under evidence and authority.
2. **Audience Translation**: the system must make sense to a solo maintainer
   today, a nontechnical author publishing VTexts tomorrow, and a future
   community with plural consent later.
3. **Adversarial/Security Lens**: every promotion is a supply-chain boundary.
   Assume the candidate computer may be compromised, confused, stale, or merely
   optimizing a local preference that should not become global.
4. **Information-Theoretic Lens**: promotion candidates are compressed claims
   about large state changes. The proposal must retain enough bits to recreate,
   verify, compare, and roll back the transition.
5. **Mechanism-Design Lens**: consent is not one thing. Benign fiat, maintainer
   approval, user opt-in, delegated voting, and public governance have different
   failure modes.

Route-changing insights:

- The core record should be a typed **PromotionProposal**, not a patchset.
- A patchset is only one possible delta payload.
- Consent policy is part of the artifact type.
- Verification contracts should be typed and reproducible, not just prose in a
  VText.
- Product UX should make the blocking requirement visible, like Gerrit submit
  requirements, not hide it in logs.
- "Promote globally" should often mean "publish as opt-in package" first.

## Prior Art

### GitHub Pull Requests

GitHub protected branches can require PR reviews, status checks, and up-to-date
branches before merge. That is the standard source/build gate for many projects.
The useful lesson is the split between proposal, review, checks, and protected
canonical branch. The limitation for Choir is that GitHub PRs only model source
repositories, not VText/Dolt/app/computer state.

Source: [GitHub protected branches](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches/about-protected-branches?apiVersion=2022-11-28)

### Gerrit

Gerrit is closer to what Choir needs for source/build proposals because its UI
centers submit requirements and labels. A change is not merely "reviewed"; it is
submittable only when configured requirements are satisfied, such as Code-Review
or Verified labels. The useful lesson is that blockers should be first-class
state and UI, not hidden CI details.

Sources:

- [Gerrit submit requirements](https://gerrit-review.googlesource.com/Documentation/config-submit-requirements.html)
- [Gerrit review labels](https://gerrit-review.googlesource.com/Documentation/config-labels.html)

### Dolt and DoltHub

Dolt is directly relevant because Choir already uses embedded Dolt for user
computer state and plans platform-level Dolt service for publication. Dolt
exposes Git-like branch, diff, merge, and history operations through SQL. DoltHub
also has pull request APIs for database branches.

The lesson is not "use DoltHub as-is"; it is that database state can have PR-like
review semantics if branches, diffs, conflicts, and merge operations are
first-class.

Sources:

- [Dolt version control features](https://docs.dolthub.com/sql-reference/version-control)
- [DoltHub database pull request API](https://docs.dolthub.com/products/dolthub/api/database)
- [DoltHub pull request workflow design](https://www.dolthub.com/blog/2023-08-18-design-pull-request-workflow/)

### Kubernetes Admission Control

Kubernetes admission controllers validate or mutate requests before persistence.
OPA/Gatekeeper turns policy into a first-class admission decision. The lesson for
Choir is architectural: promotion should pass through admission controllers
before it mutates canonical state. Some controllers validate, some default or
annotate, and side effects require reconciliation.

Sources:

- [Kubernetes admission controllers](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/)
- [Kubernetes dynamic admission control](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/)
- [OPA for Kubernetes admission control](https://www.openpolicyagent.org/docs/kubernetes)

### Supply Chain Security: SLSA, in-toto, TUF, Sigstore

Source/build promotion is a software supply-chain operation. SLSA and in-toto
frame attestations and provenance. TUF frames trusted update metadata, delegated
roles, threshold signatures, hashes, and versioned metadata. Sigstore frames
artifact signing and verification with transparency logs.

Choir should not copy all of these wholesale for v0, but should adopt their
shape: subject artifact, provenance, builder identity, verification predicates,
signatures/approvals, immutable refs, and revocation/rollback behavior.

Sources:

- [SLSA attestation model](https://slsa.dev/spec/v1.1/attestation-model)
- [GitHub artifact attestations](https://docs.github.com/en/enterprise-cloud%40latest/actions/concepts/security/artifact-attestations)
- [TUF metadata roles](https://theupdateframework.io/docs/metadata/)
- [TUF specification](https://github.com/theupdateframework/specification/blob/master/tuf-spec.md)
- [Sigstore verification](https://docs.sigstore.dev/cosign/verifying/verify/)

### Progressive Delivery

LaunchDarkly distinguishes deployment from release and supports canary and
percentage rollout strategies. The lesson for Choir is that "deployed to
platform" and "released to all users" should be distinct states. A platform
runtime change can be deployed behind a flag or limited to selected computers
before broad rollout.

Source: [LaunchDarkly deployment and release strategies](https://launchdarkly.com/docs/fed-docs/guides/infrastructure/deployment-strategies)

### App and Extension Stores

Chrome Web Store and WordPress plugin review demonstrate a separate class of
promotion: packaging code for many users with permissions, policy, privacy, and
malware review. This maps to Choir apps and appagents more than to platform core.

Sources:

- [Chrome Web Store review process](https://developer.chrome.com/docs/webstore/review-process/)
- [WordPress plugin guidelines](https://developer.wordpress.org/plugins/wordpress-org/detailed-plugin-guidelines/)
- [VS Code extension publishing](https://code.visualstudio.com/api/working-with-extensions/publishing-extension)

### Standards and Governance

IETF rough consensus and running code is useful because it distinguishes proof
from unanimous voting. Mechanism-design literature such as quadratic voting and
liquid democracy is useful later, but probably too heavy for Choir v0. The near
term should support maintainer fiat as a policy mode while recording it as an
explicit approval artifact. Later the same approval table can support delegated
or weighted mechanisms.

Sources:

- [IETF standards process](https://www.ietf.org/process/process/)
- [IETF RFCs and rough consensus](https://www.ietf.org/process/rfcs/)
- [RFC 7282 on rough consensus](https://www.ietf.org/ietf-ftp/rfc/rfc7282.txt.pdf)
- [Quadratic Voting, AEA](https://www.aeaweb.org/articles?id=10.1257%2Fpandp.20181002)
- [Liquid Democracy experiments](https://arxiv.org/abs/2212.09715)

## Proposed Architecture

### Core Object: PromotionProposal

```text
promotion_id
promotion_kind:
  vtext_publication
  app_package
  appagent_package
  personal_computer
  source_build_patchset
  platform_runtime
scope:
  personal
  org
  public_package
  platform
owner_id
source_computer_id
candidate_computer_id
source_run_id
trace_id
artifact_refs
delta_refs
base_refs
head_refs
verifier_contracts
verifier_results
consent_policy
approval_records
apply_plan
rollback_plan
status
created_at
updated_at
```

The proposal is not the artifact. It is the control record that binds artifact,
lineage, verification, consent, and apply semantics.

### Kind-Specific Promotion

#### VText Publication

Target:

- platform publication service;
- platform Dolt service, not embedded user Dolt;
- immutable publication revision graph.

Input:

- VText document id;
- selected revision id;
- author identity;
- citations/provenance;
- license/visibility;
- content hash;
- optional editorial review.

Verification:

- selected revision exists and is immutable;
- author/owner approved;
- citations or source refs satisfy publication policy;
- no private-only refs leak;
- content hash stored;
- publication preview rendered.

Consent:

- author approval for personal publication;
- editorial/platform approval for platform featured/public canon;
- later community curation signals.

Apply:

- copy/commit selected revision into platform Dolt service;
- create public publication record;
- do not mutate the user's working VText.

Rollback:

- unpublish, supersede, or retract;
- preserve historical record unless legally required to erase.

Platform Dolt design note:

VText publication is also the beginning of Choir's retrieval and citation
economy. The platform Dolt service should not only store publication metadata;
it should make citation, retrieval, provenance, attribution, reuse, and review
queryable as first-class platform facts. Promotion records should preserve how a
candidate used sources, what claims are citation-backed, which artifacts became
retrieval-visible, and how later work cites or transforms earlier work. This
needs a dedicated research/design pass before the platform Dolt service and
promotion system harden their schemas.

#### App Package

Input:

- app source/assets;
- manifest;
- permissions;
- storage schema/migrations;
- install/uninstall hooks.

Verification:

- package builds;
- app opens in clean computer;
- migrations apply and roll back;
- permission manifest is minimal;
- no forbidden network/filesystem/tool access;
- UI smoke tests pass.

Consent:

- user can install into personal computer;
- org admin can install for org;
- platform curator can publish to app catalog.

Apply:

- install package into target computer or publish to package registry.

Rollback:

- uninstall or disable package;
- schema rollback/compatibility plan.

#### Appagent Package

Input:

- prompts;
- tool permissions;
- authority profile;
- memory/state schema;
- evaluation suite.

Verification:

- prompt/tool boundary review;
- capability least privilege;
- evals for refusal, data exfiltration, action scope, and recovery;
- traceability of generated actions.

Consent:

- user opt-in for personal use;
- platform review for default agents.

#### Personal Computer Promotion

Input:

- candidate computer id;
- base active computer refs;
- typed deltas across ledgers.

Verification:

- active foreground tail identified;
- candidate delta replay/merge succeeds or conflicts explicitly;
- state being switched to was actually verified;
- old route recorded.

Consent:

- owner approval.

Apply:

- atomic route pointer switch.

Rollback:

- route pointer back to previous active computer for a TTL.

#### Source/Build Patchset

Input:

- base SHA;
- worker head SHA;
- patchset hash;
- manifest;
- trace;
- verifier results;
- candidate VM identity.

Verification:

- patch applies to base;
- changed files match declared scope;
- tests/build/security checks pass;
- worker did not push directly;
- provenance retained.

Consent:

- maintainer approval for integration branch or PR;
- platform owner approval for direct main merge in early phase.

Apply:

- create integration branch or GitHub PR;
- never mutate `origin/main` directly from a user computer.

Rollback:

- close PR, delete integration branch, or revert commit after merge.

#### Platform Runtime

Input:

- source/build patchset plus deploy plan;
- migration plan;
- compatibility statement for divergent user computers.

Verification:

- CI;
- staging deploy identity;
- deployed Playwright/product acceptance;
- canary and rollback proof.

Consent:

- maintainer/platform approval.

Apply:

- merge to `origin/main`;
- CI/CD deploy.

Rollback:

- revert/redeploy previous known-good platform release.

## Source/Build Promotion UX

The first useful product surface should be a Promotion Queue app/panel with:

- candidate summary;
- kind/scope badge;
- owner and source computer;
- base SHA and worker head;
- changed files;
- rendered diff;
- trace/VText links;
- verifier contract status;
- risk classification;
- approval/reject/archive buttons;
- "propose as GitHub PR" for platform code;
- "apply to my computer" only for personal promotion kinds;
- explicit "this will affect all staging users" warning for platform scope.

The UX should copy Gerrit's strongest idea: show exactly what is blocking
submission. Examples:

```text
Blocked:
- missing verifier: frontend build
- missing approval: platform-maintainer
- stale base: origin/main moved from base SHA
- unsupported scope: candidate kind=personal_computer cannot target platform
```

## Mechanism Design

### Phase 0: Benign Fiat

For now, your approval is enough, but it must be a recorded approval object:

```text
approval_id
promotion_id
approver_id
authority_basis=owner|maintainer|platform_admin
decision=approve|reject|request_changes
comment
created_at
signature_or_session_ref
```

This avoids magical fiat. It is explicit, inspectable, and replaceable later.

### Phase 1: Maintainer Review

Use required approval roles:

```text
source_build_patchset/platform:
  required:
    - tests_passed
    - security_scan_passed
    - platform_maintainer_approval

vtext_publication/public:
  required:
    - author_approval
    - publication_policy_passed
```

### Phase 2: Opt-In Distribution

Most non-core changes should publish as opt-in packages before becoming
defaults. Adoption signals become evidence, not automatic governance.

### Phase 3: Plural Governance

Add voting only after the artifact and verification substrate is strong.
Quadratic voting can express intensity but is sybil-sensitive. Liquid democracy
can delegate expertise but can form capture/cycle problems. Rough consensus is
often better for technical standards because it asks whether objections are
technically sustained, not merely numerous.

## Security Model

Promotion is the boundary between local agency and shared trust.

Threats:

- compromised worker VM exports malicious patch;
- confused agent exports unrelated changes;
- stale candidate applies over newer active/platform state;
- owner accidentally approves platform-wide change;
- malicious appagent requests broad tools;
- VText publication leaks private citations;
- provenance evidence is local-only and unrecoverable;
- UI makes global apply look like personal apply.

Controls:

- typed promotion kinds;
- scope-specific consent;
- immutable artifact hashes;
- verifier contracts with reproducible commands;
- base/head lineage;
- branch protection / PR for platform code;
- no direct worker push;
- policy admission before apply;
- role/threshold approvals for higher-risk scopes;
- staged rollout/canary for platform runtime changes;
- durable platform inspection APIs.

## Information-Theoretic Frame

A promotion proposal is a lossy compression of a large candidate world. The
record must preserve enough information to answer:

- What changed?
- From what base?
- In which ledger?
- Who or what made the change?
- What evidence says it works?
- Who consented?
- What state will be mutated?
- How do we reverse it?
- Can another verifier recompute the same conclusion?

If the proposal cannot answer those questions, it is not promotable. It can
still be a note, draft, or local artifact.

## Priority Correction: Embedded Dolt First

The promotion system depends on typed ledgers. The most important current ledger
mismatch is inside the user computer itself.

The canonical docs already say the right thing:

- per-user embedded Dolt owns private computer/appagent state;
- platform Dolt owns platform-visible state;
- SQLite is tolerated only for narrow hot runtime, auth/session, cache, local
  compatibility, or transitional roles;
- the sandbox should converge on one embedded Dolt database for VText, work
  graph, sessions, events, app state, desktop state, artifact metadata, and
  promotion records.

The current implementation has only partially reached that state:

- `vtext` document state is already backed by an embedded Dolt workspace;
- runtime/control state is still a SQLite store with tables for runs, agents,
  events, channel messages, run memory, promotion candidates, run acceptances,
  browser sessions, desktop state, findings, and worker updates;
- deployed VM sandboxes put that SQLite file on `/mnt/persistent`, so the Node B
  issue is not that live product state is disappearing into `/tmp`;
- nevertheless, the semantic problem remains: user-computer product state is
  split across SQLite plus Dolt, so promotion, history, branch, merge, and
  publication semantics are fractured.

This should now outrank the Promotion Queue UI as the next substrate mission.
The queue is easier to build on top of the correct ledger than to build once on
SQLite and migrate immediately afterward.

### Mission Gradient For The Migration

Real artifact:

- one embedded Dolt workspace per user computer owns durable sandbox product
  state, including current runtime tables and existing VText tables;
- this is one Dolt workspace/ledger per user computer, not separate per-user
  Dolt workspaces for VText and runtime/control state;
- host-side auth/vmctl SQLite remains allowed until platform Dolt exists;
- filesystem roots remain for source/build trees, uploaded/generated blobs, and
  materialized aliases, not as hidden canonical product ledgers.

Invariants:

- no loss of existing runs, traces, VTexts, promotion candidates, acceptances,
  desktop state, or channel messages during cutover;
- public product APIs keep their shape unless a change is intentionally
  versioned;
- staging proof must inspect Node B active and worker computers after deploy;
- host auth/session and vmctl routing are not casually pulled into embedded
  per-user Dolt;
- large binary content stays in filesystem/blob storage with Dolt metadata.

Value criterion:

```text
Minimize semantic state split inside the user computer while preserving deployed
staging behavior, product API compatibility, restart recovery, traceability, and
rollback to the previous SQLite-backed store.
```

Homotopy axes:

- start by making the existing runtime store SQL dialect portable and tested
  against both SQLite and Dolt;
- move the runtime schema into the existing embedded Dolt workspace;
- either make the existing VText workspace the unified per-computer workspace or
  migrate VText into a new unified workspace and retire the old VText-only
  workspace after verification;
- run dual-read or import verification only as a temporary cutover aid;
- delete SQLite as a sandbox runtime source of truth once staging proves parity;
- only then build promotion/publication workflows on top of Dolt-native state.

Dense feedback:

- unit tests for every store method against Dolt;
- migration/import tests from an existing SQLite runtime DB into Dolt;
- restart tests proving runs/events/VText/promotions survive process restart;
- Playwright/API staging proof for active and worker computers;
- Node B disk/DB inspection proving the active computer no longer creates
  runtime `state-wal`/`state-shm` SQLite files for sandbox product truth.

Forbidden shortcuts:

- do not rename a SQLite file or wrap it behind an interface and call that Dolt;
- do not keep writing new durable product facts to runtime SQLite;
- do not move host auth/vmctl into per-user embedded Dolt as part of this slice;
- do not store blobs directly in Dolt rows when content-addressed file/blob
  storage plus metadata is the correct boundary;
- do not claim local-only proof for staging VM persistence.

Rollback:

- keep a SQLite export/import path during the cutover;
- retain the previous deployed commit as platform rollback;
- keep old active VM state untouched until the migration verifier succeeds;
- if migration fails, route users back to the previous runtime store and preserve
  the failed Dolt workspace for diagnosis.

Stopping condition:

- staging is deployed at the migration commit;
- health reports that commit;
- product APIs for VText, Trace, run acceptances, promotions, desktop state, and
  worker delegation pass on Node B;
- read-only Node B inspection confirms the migrated user computer state is in
  embedded Dolt and any remaining SQLite is explicitly host-side or transitional;
- one quality pass has removed duplicate store paths or documented any remaining
  transitional state with owner, reason, and deletion condition.

## Recommended Build Plan

### Slice 0: Collapse Sandbox Runtime State Into Embedded Dolt

Goal:

Make the user computer's durable product state live in one embedded Dolt
workspace before adding more promotion ceremony.

Work:

- add runtime/control tables and store operations to the unified per-computer
  Dolt workspace, keeping SQLite readable only as a migration/rollback source
  during cutover;
- port current runtime tables into Dolt-compatible DDL;
- replace SQLite-specific DDL, pragmas, and upserts with portable or
  Dolt-specific equivalents;
- keep VText tables in the same embedded workspace or establish a clear
  same-workspace logical database/table layout;
- add migration/import from existing runtime SQLite files;
- run the runtime store test suite against Dolt;
- update defaults so local dev no longer implies durable sandbox state under
  `/tmp` unless explicitly ephemeral.

Acceptance:

- Node B product APIs still pass for VText, Trace, promotion candidates, run
  acceptances, desktop state, and worker delegation;
- active and worker computer inspection shows sandbox product truth in embedded
  Dolt, with no runtime SQLite WAL pair as the semantic source of truth;
- any remaining SQLite is host-side auth/vmctl or an explicitly named
  transitional compatibility artifact.

### Slice 1: Evidence-Correct Source/Build Promotion Queue

Goal:

Make worker-exported patchsets reviewable from the product.

Work:

- improve `/api/promotions` detail to include parsed manifest, patch diff,
  verifier results, candidate JSON, rollback refs;
- add product UI detail view;
- show base/head/changed files;
- show explicit status requirements;
- preserve owner-scoped access;
- add Playwright proof that a queued worker export can be inspected after the
  initial browser session renews.

Acceptance:

- Node B product UI displays the same candidate currently verified by direct
  DB/filesystem inspection.

### Slice 2: Integrate Source/Build Candidate To Branch

Goal:

Turn queued patchset into an integration branch or GitHub draft PR, not main.

Work:

- connect `internal/promotion` library to runtime/product route;
- apply patchset to clean integration workspace;
- run verifier contracts;
- update candidate status to `verified` or `blocked`;
- allow owner/platform approver to create a GitHub PR.

Acceptance:

- worker candidate becomes PR with trace/manifest links and no direct push from
  worker VM.

### Slice 3: VText Publication Service

Goal:

Publish selected VText revisions into a platform service backed by Dolt server.

Work:

- introduce platform publication service;
- provision platform Dolt service/database;
- define publication schema;
- design retrieval/citation/provenance tables before hardening the schema;
- add selected-revision publication proposal;
- render public preview;
- record author approval and publication hash.

Acceptance:

- a private VText revision is published as immutable public artifact without
  mutating the authoring VText.

### Slice 4: App/Appagent Packages

Goal:

Package app/appagent candidates for install or catalog publication.

Work:

- app manifest;
- permissions model;
- install/uninstall hooks;
- eval contracts for appagents;
- package registry records.

### Slice 5: Platform Runtime Promotions

Goal:

Promote source/build candidates into platform releases through CI/CD and staged
rollout.

Work:

- PR creation;
- required checks;
- staging deploy proof;
- canary/rollback;
- compatibility notes for divergent computers.

## Immediate Implication For The UX Sweep

We did not yet produce UX/onboarding changes inside the account candidate. To
achieve the original sweep aims, the next run should use the now-working
substrate to create real candidates for:

- first-load VText explaining Choir/VText;
- prompt bar window-crowding fix;
- login/onboarding flow;
- desktop aesthetics;
- podcast search/index behavior;
- native skill-context support.

But the sweep should not try to make those global automatically. It should
produce one or more typed promotion proposals. Then we approve or reject them
through the promotion system.

## Bottom Line

The central design principle is:

```text
Personal computers can diverge freely.
Shared changes must become typed, reviewable, verifiable, consented artifacts.
```

The current code has the beginning of this system for source/build patchsets,
but not the durable product UX, not kind-specific promotion semantics, and not
the platform Dolt publication service. Building that bridge is now more urgent
than another ambitious UX sweep, because otherwise good work produced inside
Choir has no principled path from private candidate to shared reality.
