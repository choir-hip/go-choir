# MissionGradient: Features Hard Cutover v0

Status: draft_for_review
Date: 2026-05-28

## Goal String

```text
/goal Run docs/mission-features-hard-cutover-v0.md as a Codex-operated MissionGradient mission: hard-cut Choir's Apps & Changes surface into a deletion-first, video-first Features import flow with no backwards-compatible product skin. First document and preserve the problem: the current UI asks users to do QA/review/candidate work instead of watching a demo, importing, waiting while Choir builds/verifies in the background, and activating with visible rollback. Then make the smallest coherent breaking product cut: rename the launcher-facing app to Features, treat short demo video/evidence as the primary catalog proof, replace Try in candidate with Import, run recipient apply/build/verify automatically after import, replace Install/Promote with Activate only after verification and rollback refs exist, and expose pending/active/rolled-back change controls through a glowing Desk menu affordance with Activate, Roll back, Roll forward, Later, Watch demo, and View details. Add the notification contract for unattended work: ordinary research/coding trajectories and especially feature import trajectories that reach a final, ready, or blocker state while the owner is not actively observing must send an email to the account signup email for now, with on-screen copy before/during the run saying Choir will email when ready; make this configurable later but do not require settings UI in this mission. Delete or demote candidate preview, review queue, visible Verify build, Settings promotion controls, stale launcher code, raw package/adoption IDs, and ordinary UI references to AppChangePackage/adoption/promotion/candidate. Preserve source-level package and recipient rebuild invariants internally; do not mutate the active computer before activation; do not hide technical evidence from expert details; do not claim activation without verifier evidence and rollback refs; do not email secrets, raw logs, or private content beyond a concise title/status/link. Verify on staging with desktop and mobile Computer Use/Playwright proof: catalog video-first UI, one-click Import creates a background import, on-screen email-ready notice appears, unattended completion sends an email to the signup address, Desk menu visual state changes when ready, Activate advances the computer only after verification, Roll back works, Roll forward works, and technical evidence remains accessible but out of the happy path. Stop at the highest honest evidence level with exact commits, CI/deploy identity, screenshots/video/email observations, API IDs, rollback refs, deleted-code diffstat, residual risks, and the next mission string.
```

## Problem Record

The current Apps & Changes path exposes too much implementation machinery to the
owner. It asks a normal user to think like a QA operator: candidate preview,
manual Verify build, Trace/acceptance, review queue, package/adoption ids, and
technical rollback panels. That is the wrong product shape.

The desired product is simpler:

```text
Features catalog
  -> watch short demo video
  -> Import
  -> Choir applies/builds/verifies in the background
  -> screen says Choir will email when ready if the owner leaves
  -> Desk menu glows when ready
  -> email goes to the account signup address if the owner is not observing
  -> Activate / Roll back / Later
  -> after rollback, Roll forward remains available
```

Video is the main persuasive proof. Recipient build and verifier evidence are
the safety proof. The owner should not need to perform QA work before deciding
whether the feature is interesting.

There is also a notification problem. Choir runs can finish while the owner is
not watching. A useful automatic computer should not require the owner to keep a
tab open and poll the UI. For v0, when an ordinary research/coding trajectory or
feature import reaches a final, ready, or precise blocker state while the owner
is not actively observing, Choir should email the account signup email address.
The email should be a concise notification with title, status, and a product
link back into Choir. It must not include secrets, raw traces, long logs, or
private source/content dumps. The destination can become configurable later.

## Cognitive Transform Pass

Current obstacle:

```text
The source package/adoption substrate is real but the product asks users to
operate substrate controls directly. The vocabulary has become part of the bug:
Apps & Changes, Try in candidate, Verify build, Install, AppChangePackage,
adoption, promotion, human proof, and candidate preview all leak local
implementation steps into the owner experience.
```

Selected transforms:

1. **Via negativa** - define the product by what users no longer have to do:
   no package ids, no candidate desktops, no manual verify button, no Trace
   reading, no adoption/promotion vocabulary, no disappearing rollback.
2. **Supply-chain split** - demo video proves appeal; recipient build/verifier
   evidence proves safety. Do not make either proof pretend to be the other.
3. **System affordance transform** - rollback is not a catalog detail. It is a
   visible computer-state affordance surfaced through the Desk menu.
4. **State-machine compression** - ordinary UI should show a few useful states:
   Available, Importing, Verifying, Ready, Active, Rolled back, Blocked. The
   implementation may keep richer package/adoption records under details.
5. **Attention-boundary transform** - if the owner is watching, the product can
   update in place; if the owner is away, the product owes them a concise email
   notification. Do not make unattended completion depend on polling.
6. **Hard-cutover deletion** - prefer removing old UI and tests over preserving
   compatibility shells. Compatibility here would preserve the wrong ontology.

Route-changing insights:

- `Features` is the right user-facing noun for apps, workflows, agents, UI
  improvements, prompt policies, and integrations.
- `Import` should create the recipient candidate and start verification.
- `Activate` should be unavailable until recipient verification and rollback
  refs exist.
- Candidate preview is secondary expert/debug evidence, not the primary path.
- The Desk button should become a system-level signal for pending computer
  state transitions.
- The screen should explicitly tell the owner that Choir will email when the
  import or long run is ready, using the signup email address for v0.

## Real Artifact

The artifact is a hard-cutover product path:

```text
Features app
  -> video-first feature catalog
  -> one-click Import
  -> background recipient apply/build/verify
  -> unattended completion email to account signup address
  -> Desk menu pending/active/rolled-back affordance
  -> Activate / Roll back / Roll forward
  -> technical evidence hidden from happy path but available
  -> desktop and mobile staging proof
```

The artifact is not:

- a renamed AppChangePackage admin console;
- an app store with ratings/payments/social discovery;
- a mandatory live candidate desktop preview;
- a Trace/acceptance browser for normal users;
- a backwards-compatible UI preserving old language;
- a fake video placeholder without proof meaning;
- an activation path that skips recipient build/verifier/rollback evidence;
- a run that silently completes while the owner is away.

## Naming Cutover

User-facing names:

| Old | New |
| --- | --- |
| Apps & Changes | Features |
| Change | Feature |
| Try in candidate | Import |
| Verify build | automatic Building / Verifying state |
| Install / Promote | Activate |
| Rollback | Roll back |
| Candidate preview | Live preview, secondary/expert |
| Technical refs | Evidence |

Internal names may remain temporarily only where renaming would obscure the
substrate or inflate the mission. Product UI and docs should not expose
`AppChangePackage`, `adoption`, `promotion`, or `candidate` in ordinary flows.

## Deletion Targets

Prefer deletion or demotion over compatibility:

- Remove candidate preview from the main happy path.
- Remove visible Verify build as a user action.
- Remove review queue from ordinary UI.
- Remove Settings promotion controls, or move them behind a developer-only
  diagnostic if deletion would hide essential recovery.
- Remove stale launcher code if it is unused.
- Remove raw package/adoption ids from ordinary UI.
- Remove ordinary UI copy that says AppChangePackage, adoption, promotion,
  candidate, human proof, or machine receipt.
- Delete tests that assert old product language and replace them with tests
  for the new state machine.

Deletion budget:

```text
Target: deletions >= additions.
Hard warning: if additions exceed deletions by more than 500 lines before
staging proof, stop and explain why the hard cutover is adding structure.
```

## Value Criterion

Maximize:

```text
watchable feature understanding
+ one-click import
+ background verification
+ obvious reversible computer state
+ rollback and roll-forward confidence
+ hidden-but-available technical evidence
```

while minimizing:

```text
QA work imposed on users
+ raw package/adoption/promotion language
+ manual verification buttons
+ candidate desktop dependence
+ review cockpit UI
+ compatibility shells
+ unverified activation
```

## Hard Invariants

- Features are source-level packages rebuilt for the recipient computer.
- Import must not mutate the active computer.
- Activation requires recipient build/verifier evidence and rollback refs.
- Roll back must have an actual previous active source/route profile.
- Roll forward must only be available when a rolled-back activation has enough
  preserved candidate/activation evidence to re-activate safely.
- Technical evidence remains available under details or developer surfaces.
- Public/demo video is persuasive evidence, not safety evidence.
- Recipient verification is safety evidence, not marketing proof.
- Completion emails are notifications, not authorization. Email links must
  still require auth for private state or mutation.
- Completion emails must not include secrets, raw traces, long logs, or private
  source/content dumps.
- The notification destination is the account signup email for v0; future
  configurability must not block this mission.
- No manual deploy shortcuts.
- Platform behavior changes follow the repo landing loop.
- New problems discovered during staging proof are documented before fixes.

## Control Intervals

### P0 - Problem Documentation And Inventory

Document the current problem and inventory the code to delete or rename before
code changes.

Expected inventory:

- `frontend/src/lib/AppsChangesApp.svelte`
- `frontend/src/lib/ChangePreviewFrame.svelte`
- `frontend/src/lib/SettingsApp.svelte`
- `frontend/src/lib/BottomBar.svelte`
- `frontend/src/lib/stores/desktop.js`
- stale launcher code, if unused
- frontend tests asserting old Apps & Changes behavior
- runtime API names that leak into product copy

### P1 - Product Language Hard Cut

Rename the launcher-facing surface to Features and update product copy.

Acceptance:

- Desk menu shows Features, not Apps & Changes.
- Ordinary UI does not say AppChangePackage, adoption, promotion, candidate,
  Verify build, Try in candidate, or Install.
- Existing technical API names may remain if hidden from ordinary UI.

### P2 - Import State Machine

Replace manual Try/Verify/Install flow with:

```text
Available -> Importing -> Building/Verifying -> Ready -> Active
Active -> Rolled back
Rolled back -> Roll forward available
Blocked -> View details
```

Acceptance:

- Import creates the recipient candidate/import record and starts verification
  without a separate user Verify click.
- Activate is disabled until verification and rollback refs exist.
- Blocked imports explain the blocker without asking the user to inspect Trace.

### P3 - Desk Menu Affordance

Make the Desk button/menu reflect pending computer state transitions.

Acceptance:

- Desk button changes color/glow/badge when a feature is ready to activate.
- Desk menu shows the current pending/active/rolled-back feature state.
- Desk menu offers Watch demo, Activate, Roll back, Roll forward, Later, and
  View details where valid.
- The same controls remain available in Features.

### P4 - Evidence And Video

Make video the primary catalog proof when available.

Acceptance:

- Feature cards/details prioritize a watchable demo video or an honest missing
  video state.
- Technical evidence is available under details.
- Video does not substitute for recipient verification before activation.

### P4.5 - Unattended Completion Email

Notify the owner when work finishes away from their attention.

Acceptance:

- Feature import UI says Choir will email the signup address when the import is
  ready or blocked.
- Ordinary research/coding trajectories that reach a final version while the
  owner is not actively observing send a concise email notification.
- Feature imports that reach Ready, Active-ready, Rolled back, Roll-forward
  available, or Blocked while the owner is not actively observing send a concise
  email notification.
- Email destination is the account signup email for v0.
- Email contains title/status/link, not raw traces, secrets, long logs, or
  private content dumps.
- If delivery cannot be proven end-to-end in this mission, record the precise
  blocker and keep the on-screen promise honest.

### P5 - Deletion And Test Rewrite

Delete old surfaces and rewrite tests to assert the hard cutover.

Acceptance:

- stale launcher code removed if unused;
- Settings promotion controls removed or developer-gated;
- Candidate preview demoted or removed from happy path;
- tests assert Features/Import/Activate/Roll back/Roll forward language;
- tests do not preserve old product compatibility.

### P6 - Staging Proof

Land through the normal loop and prove on staging.

Required proof:

- CI passes;
- deploy identity matches the pushed commit;
- desktop and mobile screenshots show Features catalog;
- Import starts background verification;
- Desk button/menu changes state when ready;
- on-screen copy says Choir will email when ready;
- unattended final/ready/blocker completion sends email to the signup address,
  or the precise email delivery blocker is recorded;
- Activate advances the computer only after verifier/rollback evidence;
- Roll back works;
- Roll forward works or is precisely blocked by missing preserved activation
  evidence;
- technical evidence is accessible but out of the happy path.

## Dense Feedback

- `rg` evidence for deleted old names in ordinary UI.
- Frontend build.
- Focused frontend tests for Features and Desk menu.
- Focused Go tests for import/activation/rollback if backend behavior changes.
- Computer Use or Playwright screenshots on desktop and mobile.
- Product API responses for import, verify, activate, roll back, and roll
  forward.
- Email evidence: provider/test capture/logged delivery metadata proving the
  signup address notification for unattended completion, without leaking private
  content in the report.
- Diffstat with deletion/addition ratio.

## Rollback Policy

- Code rollback uses git revert through CI/deploy.
- Feature activation rollback uses recorded source/route rollback refs.
- UI cutover rollback should not restore old product ontology unless the new
  path is unusable; prefer fix-forward within Features language.
- If roll-forward cannot be implemented safely, expose the missing evidence as a
  precise blocker and keep rollback working.

## Stopping Conditions

Stop as `complete` only when:

- old user-facing product language is gone from ordinary UI;
- Features import flow works on staging;
- Desk state affordance exists and is proven;
- Activate, Roll back, and Roll forward are proven or roll-forward is precisely
  blocked with rollback still available;
- unattended completion notification is proven for at least one long run or
  feature import, or a precise delivery blocker is documented with no false UI
  promise;
- deletion-first cleanup is complete with diffstat recorded;
- technical evidence remains available but not in the happy path.

Stop as `checkpoint_incomplete` if:

- product language and deletion cutover land, but a backend substrate blocker
  prevents full activation or roll-forward proof;
- Desk affordance works, but live import/verification remains blocked with
  precise evidence;
- roll-forward needs a separate substrate mission while rollback is safe.

Stop as `blocked_incomplete` only after root-cause probes and cognitive
transforms show that continuing requires external authority or would violate an
invariant.

## Suggested Follow-On Mission If Needed

```text
/goal Run a Codex-operated MissionGradient mission to repair the lowest substrate blocker discovered by the Features hard cutover. Preserve the new Features/Import/Activate/Roll back/Roll forward product language, do not restore Apps & Changes or manual QA controls, and focus only on the backend state transition that prevented staging proof.
```

## Staging Evidence Checkpoint: Import Contract Mismatch

Timestamp: 2026-05-28 06:10 UTC

Commit under test:

```text
7706c220afd9d415e14080707fd40d173744b324
```

Staging proof established:

- `https://choir.news` served frontend/proxy/sandbox build identity
  `7706c220afd9d415e14080707fd40d173744b324`.
- A fresh authenticated staging account could open the Desk menu and launch
  `Features`.
- The launcher showed `Features`, not `Apps & Changes`.
- The Features app rendered on desktop and mobile without horizontal overflow.
- Old ordinary product labels were absent from the new surface:
  `Apps & Changes`, `Try in candidate`, `Install`, `Promote`, and
  `AppChangePackage`.
- A product API-created private source-level package rendered as a video-first
  catalog item with `Import`, `Watch demo`, `Later`, `View details`, and signup
  email notification copy.

Blocker discovered:

```text
Clicking Import failed before adoption creation with:

invalid app adoption request
```

Root cause hypothesis:

The new `FeaturesApp` sends
`target_active_source_ref_at_candidate_start` in the create-adoption request,
but `createAppAdoptionInput` does not accept that field and the runtime decoder
uses `DisallowUnknownFields`. The backend already computes and records the
target active source ref at candidate start. The frontend should not send this
server-owned field during import.

Why this matters:

The hard cutover UI is live, but the central state transition is not. Until the
request contract is fixed, `Import` cannot create an adoption, verification
cannot start, no completion email can be sent from the import flow, and Desk
ready/activate/rollback/roll-forward controls cannot be proven on staging.

## Staging Evidence Checkpoint: Verification Is Not Background

Timestamp: 2026-05-28 06:21 UTC

Commit under test:

```text
ed0568a70b9c5f27306104e3a0b139900c6017de
```

Staging proof established after the request contract fix:

- Clicking `Import` created a real adoption record:
  `feature-feature-proof-1779949266247-8f71f7a4-1aa5-433e-9de7-34559b4f677c`.
- The adoption entered `verifying`.
- The verifier results recorded:
  - `source-refs-resolve`: passed;
  - `source-ledger-reference`: passed;
  - `actual-recipient-runtime-ui-build`: running.
- The rollback profile already recorded previous active source ref
  `origin/main`.
- Mobile continued to render the importing state without horizontal overflow.

Blocker discovered:

```text
The browser-triggered verify request remained open during the recipient build
and hit the deployed proxy timeout after roughly 122 seconds. The UI still
showed Importing, and the adoption remained in status "verifying" with only the
"recipient build started" verifier result.
```

Root cause hypothesis:

`POST /api/adoptions/:id/verify` performs recipient materialization/build inside
the request context. That contradicts the mission invariant that `Import` starts
background apply/build/verify and returns control to the owner. A long build
should not depend on an open browser request, and request cancellation should
not leave a durable adoption stuck in `verifying`.

Why this matters:

Until verification is detached from the request, the Features flow cannot prove
Ready, Desk glow, Activate, Roll back, Roll forward, or completion email. The
UI may say Choir will work in the background, but the substrate is still a
synchronous request path.
