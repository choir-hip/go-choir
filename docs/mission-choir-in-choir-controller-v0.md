# MissionGradient: Choir Self-Development Controller v0

Status: next 8-24h staging mission, revised after export-level acceptance
Date: 2026-05-14

## Real Artifact

Choir self-development controller: the durable control layer that turns run
memory, mission docs, promotion queue state, verifier results, product gaps,
VM/gateway health, and user constraints into the next bounded run, then drives
candidate-world work through verification, promotion, compaction, and
continuation.

The artifact is not "more docs", a local patch, a Playwright script, a single
worker run, or a UI feature by itself. Those are projections. The artifact is a
staging-proven loop:

```text
prompt/product signal -> VText/super objective -> worker/candidate world
-> export/promotion candidate -> verifier contract -> owner decision
-> canonical landing -> deployed acceptance -> compaction -> next run
```

The previous long run proved this loop through `export-level` acceptance on
staging. This mission should advance the same object toward `promotion-level`
and, if the evidence stays stable, `continuation-level`.

## Current Evidence

Deployed staging evidence from the preceding run:

- behavior commit deployed on staging: `5aa5110cce9c0d0870e3b409c05ca67ac07c8712`;
- CI run for that behavior commit: `25835456007`, green;
- staging health reported proxy and sandbox deployed commit `5aa5110cce9c0d0870e3b409c05ca67ac07c8712`;
- deployed Playwright proof passed:
  `GO_CHOIR_RUN_BACKGROUND_WORKER_DEMO=1 GO_CHOIR_WORKER_DEMO_BASE_URL=https://draft.choir-ip.com pnpm exec playwright test tests/vtext-background-worker-demo.spec.js --workers=1 --reporter=line`;
- accepted run record reached `export-level`, not `promotion-level`;
- docs-only proof commit `5eb232c` is on `origin/main` and intentionally did not trigger CI.

Implemented primitives already present:

- durable `RunAcceptanceRecord` synthesis from trace, run, promotion, rollback,
  gateway, and deployment evidence;
- run memory entries, compaction, context-overflow recovery, and continuation
  records;
- deterministic run-control continuation synthesis from promotion candidates or
  mission-doc fallback;
- authenticated public continuation APIs and Trace controls;
- promotion queue store/runtime APIs, public owner review, internal verify and
  promote primitives, verifier contracts, divergence blocking, and rollback
  reports;
- worker VM request/delegation/export and objective-fingerprint lease reuse;
- vmctl idle sweeper on Node B with staging reclaim;
- desktop app icons, bottom-left Start menu, Files upload UI/API, editable
  theme presets/config, first-pass Podcast/Radio surface, and Web Lens /
  Candidate Desktop Viewer split.

## Residual Error Field

The mission should choose work from the current error field rather than replay
old objectives:

- Promotion-level acceptance is not deployed-proven. We have queued candidates
  and owner review, but need an end-to-end verifier/promotion/rollback evidence
  path tied to a real landed product patch.
- Continuation-level acceptance is not deployed-proven. We have compaction and
  continuation primitives, but the deployed product proof did not record a
  selected/started next objective after a promoted result.
- Worker recurrence is partially controlled. vmctl can reuse worker leases by
  objective fingerprint, but the latest acceptance trace still observed
  duplicate delegation/export attempts. The next verifier should enforce or
  explicitly explain one-active-worker-per-objective behavior.
- vmctl lifecycle is only coarsely observable from `/health`. The next run
  should not rely on log archaeology to understand active, hibernated, failed,
  idle, worker, and interactive VM pressure.
- Product pressure should harden real existing features, not re-add them:
  launcher, desktop icons, uploads, themes, podcast/radio, and web surfaces now
  exist. The next product patch should be the smallest real improvement that
  exercises promotion and continuation.
- Candidate Desktop Viewer is intentionally same-Svelte routing, but it is not
  yet connected to promotion candidate records. Users still need to type a
  desktop id for candidate preview.
- Gateway/model/search credentials must stay host-mediated and scoped for
  worker/candidate worlds. Any expansion of candidate authority must prove that
  provider credentials are not copied into unbounded foreground/user space.

## Invariants

- Staging is the acceptance environment for behavior-changing claims.
- Behavior-changing missions land through:

```text
commit -> push origin/main -> monitor CI -> monitor deploy
-> verify staging commit identity -> run deployed acceptance proof
```

- Documentation-only commits remain exempt from automatic CI path filters.
- Foreground desktop and canonical repo state remain stable unless a verified
  promotion occurs.
- Candidate work mutates only in background VM, worker sandbox, or
  integration-branch boundaries.
- Every candidate has owner, source run, candidate/worker identity, VM/sandbox
  identity where applicable, objective fingerprint, base SHA, worker head or
  patchset, verifier contracts, evidence refs, and rollback refs.
- Workers and vsupers produce candidates. They do not promote.
- Owner review is explicit. Verification is a contract over evidence, not a
  privileged agent species.
- Browser-public product tests may use authenticated product APIs only. They
  must not call `/internal/*`, `/api/test/*`, raw event mutation endpoints, or
  hidden orchestration APIs to manufacture success.
- Automatic continuation can start only bounded profiles with lease, source
  evidence, stop condition, and rollback/recovery semantics.
- Candidate desktop viewing uses the same Svelte app and routed `/api/*` stack;
  it does not introduce VNC, WebRTC, MJPEG, framebuffer streaming, or a browser
  running inside the candidate VM merely to view Choir.
- Gateway/model/search credentials stay behind the gateway or scoped VM tokens.
- Product work must preserve existing auth/session renewal, desktop routing,
  files root boundaries, theme validation, trace causality, and deploy health.

## Value Criterion

Maximize verified Choir self-improvement per human review minute while
minimizing:

- canonical-state corruption;
- verifier bypass and Goodharting;
- hidden VM/gateway/runtime state;
- duplicate worker effort;
- context loss and undefined continuation;
- authority leakage;
- rollback cost;
- staging/product regressions;
- local-only proofs for staging-only properties.

The controller is better when it selects the same next high-value objective a
careful operator would select, records why, runs it in the right authority
boundary, proves it through deployed evidence, lands it safely, and leaves the
next bounded objective ready without Codex having to invent it again.

## Homotopy

This is one object at increasing realism, not a checklist ladder. The agent may
change route when evidence changes, but it must preserve the invariants above.

### Lambda 0: Review And Classify The Residuals

Start by checking local `main`, `origin/main`, GitHub Actions, staging health,
latest acceptance records, promotion candidates, run continuations, and vmctl
capacity. Classify each gap as tactical, target-level, or invariant-level.

### Lambda 0.2: Strengthen Observability Before Larger Autonomy

If vmctl or acceptance records hide state needed for safe long runs, add the
smallest durable telemetry or verifier field first. Prefer structured health,
trace, or acceptance evidence over deploy-log archaeology.

### Lambda 0.35: Promotion-Level Bridge

Advance from export-level to promotion-level by proving:

- exported candidate patchset becomes a durable promotion candidate;
- verifier contracts run on an integration candidate;
- owner review is recorded through product-visible state;
- canonical mutation is impossible before verification and approval;
- rollback or discard evidence is recorded;
- the landed behavior change is committed, pushed to `origin/main`, deployed to
  staging, and tied back to the candidate evidence.

At this resolution, Codex may still be the bootstrap operator that commits and
pushes after verifying the candidate. Do not call that full autonomous
promotion unless Choir itself performs the landing action through a bounded
authority path.

### Lambda 0.55: Product Pressure

Use one narrow real product patch as pressure. Choose from current residuals,
not stale gaps. Good candidates include:

- candidate preview list: connect Candidate Desktop Viewer to promotion
  candidates so a user can open a candidate without typing `desktop_id`;
- promotion review panel: expose verifier/rollback details more clearly in
  Settings or Trace;
- vmctl/run dashboard: show active worker/candidate lifecycle in Trace or
  Settings;
- theme onboarding: turn editable presets into a user-promotable theme artifact
  without allowing arbitrary CSS;
- Files upload hardening: add size/error/progress evidence if staging behavior
  shows a weakness.

Avoid broad podcast/radio work until promotion and continuation acceptance are
more durable. Podcast/radio remains a major semantic target, not the smallest
next controller proof.

### Lambda 0.75: Continuation-Level Bridge

After a verified product patch lands, synthesize and record the next objective
from durable signals. If safe under staging isolation, start the bounded next
run. If not safe, stop at a selected continuation with explicit reason,
authority, verifier target, and blocked-start rationale.

The continuation must use compaction/run memory and objective fingerprints, not
transient chat memory.

### Lambda 1: Choir-In-Choir Observer Mode

Use Playwright/Codex to prompt Choir to initiate the loop through its own
desktop. Once Choir has initiated candidate work, Codex should observe, collect
evidence, and repair invariant failures rather than directly steering every
step. If Choir blocks, Codex can patch the missing substrate, but that repair
must itself be landed with the staging-first loop.

## Dense Feedback Channels

- `git status`, `git log`, `gh run list`, and staging `/health` commit identity.
- Store/runtime tests for promotion candidates, owner review, continuation
  selection/start, run acceptance synthesis, objective fingerprint dedupe, and
  checkpoint causal order.
- vmctl tests for worker reuse, idle reclaim, lifecycle telemetry, and
  owner-scoped candidate/worker identity.
- Promotion tests for verifier contract failure, verified approval, divergence
  blocking, rollback report, and promotion/discard evidence.
- Frontend build plus focused Playwright for any touched product surface.
- Deployed Playwright proof through visible prompt bar and authenticated product
  APIs only.
- Trace assertions for request_super_execution, request_worker_vm,
  delegate_worker_vm, export_patchset, promotion candidate events, verifier
  events, review/promotion events, compaction, continuation, and acceptance
  synthesis.
- Acceptance record assertions for `promotion-level` and, if reached,
  `continuation-level`.
- Artifact evidence: trace id, run id, candidate id, continuation id,
  acceptance id, base SHA, promoted SHA, CI run id, deploy id, health commit,
  rollback refs, and test artifacts.

## Forbidden Shortcuts

- Do not rerun old local-only tests and claim staging proof.
- Do not manually seed run acceptance checkpoints or promotion success records.
- Do not use browser-public internal/test routes to bypass the product path.
- Do not suppress duplicate workers by ignoring one successful result; prove
  dedupe or record the recurrence as a blocker.
- Do not call a UI summary "owner review" unless the review state is durable.
- Do not call a verified candidate "promoted" unless canonical landing,
  deployment, and rollback semantics are evidenced.
- Do not let theme presets replace editable theme validation.
- Do not expand candidate VM or gateway credential authority without proving the
  trust boundary.
- Do not treat a selected continuation as a started continuation.
- Do not start an unbounded continuation just to reach `continuation-level`.
- Do not change docs-only CI filters to force CI for documentation commits.

## Rollback Policy

Git:

- every behavior-changing commit lands on `origin/main`;
- every candidate records base SHA, worker head, integration branch, destination
  branch, and rollback/discard instructions;
- promotion blocks on dirty repo or destination divergence;
- rollback for a landed commit is either revert commit or documented reset path,
  never silent history rewrite.

VM/runtime:

- worker/candidate leases expire;
- idle reclaim remains active on staging;
- failed worlds remain as diagnostics, queue records, and trace evidence;
- candidate VM preview must be owner-scoped.

Database/product:

- migrations are additive;
- run memory is append-only;
- promotion/continuation records are state machines with audit trail;
- file uploads stay under the authenticated user's files root;
- theme configs validate before application.

## Learning Side-Channel

Write durable learning to:

- this mission document if the target or residual field changes;
- a proof report named for the completed slice;
- run acceptance records;
- promotion reports;
- continuation records;
- Trace artifacts and Playwright artifacts;
- README/AGENTS/runtime-invariants only if the operating contract changes.

Classify surprises:

- Tactical learning: adjust implementation, tests, verifiers, or product slice.
- Target-level learning: update this mission or create a next-frontier report.
- Invariant-level learning: stop and escalate before changing authority,
  promotion, rollback, gateway credentials, or candidate/canonical boundaries.

## Stopping Condition

Stop only when one of these is true:

- staging proves `promotion-level` acceptance for one real Choir product patch:
  candidate export, verifier contracts, owner review, explicit promotion or
  rollback evidence, landed commit on `origin/main`, green CI/deploy, matching
  staging health, and a synthesized acceptance record with structured evidence;
- staging additionally proves `continuation-level` acceptance: compaction and a
  selected or started bounded next objective tied to the promoted result; or
- an invariant-level blocker is documented with rollback state, failed evidence,
  and the next smallest safe probe.

Completion requires a next-state decision. If the mission reaches
promotion-level and a safe next continuation exists, record or start it rather
than ending as a dead stop.

## Review Questions

- What transition prevented the previous proof from reaching promotion-level?
- What product patch is smallest while still exercising that transition?
- Which verifier would a reward-hacking implementation try to satisfy without
  preserving topology?
- What evidence proves staging, gateway, vmctl, product UI, git, and CI all
  agree about the same artifact state?
- If the run stops, is the next objective discoverable from durable records
  rather than chat memory?

## One-Line Goal

`/goal Use MissionGradient. Execute docs/mission-choir-in-choir-controller-v0.md as one staging-first Choir-in-Choir controller mission: advance export-level proof to promotion-level and, if safe, continuation-level acceptance for one real product patch, with worker dedupe, vmctl observability, verifier contracts, rollback, origin/main CI/deploy, and deployed evidence.`
