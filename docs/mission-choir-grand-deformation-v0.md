# MissionGradient: Choir Grand Deformation v0

Status: proposed grand 8-24h+ mission
Date: 2026-05-13

## Real Artifact

Choir as a durable self-developing computer: a learning-control system that receives user/world signals, maintains semantic artifacts, speculates in candidate worlds, verifies deltas, promotes canonical improvements, compacts operational memory, and automatically continues toward higher-value objectives without relying on Codex to choose the next step.

This is one continuous deformation from today's Codex-operated repo to Choir developing Choir inside Choir.

The mission must not collapse into a checklist of disconnected features. App
launcher, file uploads, themes, podcast/radio, Obscura-backed browser, skills,
run memory, typed app-change adoption, vsuper, and continuation are all
projections of one object:

```text
signal -> evidence -> objective -> candidate world -> verifier contract -> promotion -> compaction -> next objective
```

The order of operations is intentionally deformable. The route should change when evidence changes. The identity of the mission is preserved by invariants, verifier meaning, authority boundaries, rollback, and product-path continuity, not by completing named steps in a fixed sequence.

This is homotopy, not ladder. A smaller slice is valid only when it preserves the same trust boundaries, artifact ownership, event semantics, rollback meaning, and verifier shape that the fuller system will use.

## Invariants

- Foreground desktop and canonical repo state remain stable unless a verified promotion occurs.
- Candidate worlds mutate in background VM or integration-branch boundaries.
- Every candidate has owner, source run, candidate run, VM/snapshot identity,
  base ref, candidate ref, typed package/adoption record, verifier contracts,
  report, promotion decision, and rollback point.
- Super owns orchestration and promotion. Vsuper owns one candidate world. Cosupers are subordinate helpers. Researchers write epistemic state. Appagents own semantic artifacts.
- Workers never promote. Verifiers are contracts/phases, not a privileged agent caste.
- Every automatic continuation has objective, reason, authority profile, lease, verifier target, stop condition, and durable source evidence.
- Context-limit behavior must be defined: compaction is an operational sufficient statistic, not a prose chat summary.
- No circular synchronous waits; every worker lease expires; every durable message is idempotent.
- Product features must land through the same candidate-world promotion path as architecture changes.
- Playwright/Codex bootstrapping is allowed only when continuously deformable into Choir driving itself through the product path.
- Once Choir can drive a loop through its own prompt/product path, Codex should step back into observer/auditor/repair mode unless the loop blocks, violates an invariant, or needs an external bootstrap capability.

## Value Criterion

Maximize verified artifact improvement over time while minimizing:

- canonical-state corruption;
- hidden state and context loss;
- human monitoring burden;
- verifier Goodharting;
- authority leakage;
- deadlock;
- rollback cost;
- product regressions;
- epistemic drift.

The run should always choose the next refinement from the current error field: the weakest invariant, least observed transition, most valuable product pressure, or most ambiguous recovery path.

A completed local improvement is not sufficient reason to stop when the next safe deformation is visible. The run should either continue into that deformation or record a durable continuation with enough evidence for the next agent to resume without redefining the mission.

## Error Field

At every step, choose the next move from observed error rather than from a fixed checklist:

- if rollback is unproved, shrink toward branch-per-VM and rollback proof;
- if verification is thin, add a verifier contract before adding feature surface;
- if product-path evidence is missing, drive the next slice through Choir UI/API rather than direct Codex edits;
- if context continuity is undefined, implement compaction/run memory before longer autonomy;
- if the desktop surface is blocking dogfood, prioritize launcher/uploads/themes/files affordances;
- if Choir can already prompt or launch its own worker loop, move Codex toward observer/auditor/repair mode;
- if evidence changes the best path without violating invariants, update the mission doc and continue.

## Homotopy Parameters

Increase realism continuously along these axes:

- Codex operator -> Codex drives Choir through Playwright -> Choir super drives worker VMs -> Choir-in-Choir self-development.
- Codex direct edits -> Codex observes Choir product-path runs -> Codex repairs only failed invariants or missing bootstrap surfaces.
- Local unit proof -> runtime integration proof -> product-path proof -> live VM proof.
- Manual continuation -> metadata continuation -> deterministic controller -> semantic controller from run memory, queue state, vtexts, and research packets.
- Single candidate -> parallel candidate portfolio -> integration candidate queue.
- Marker product patch -> real launcher/uploads/themes/files slice -> podcast/radio semantic app -> Obscura browser-in-VM surface.
- Static prompt roles -> native skills for super/cosuper/vsuper/appagents -> user-editable skill/theme/onboarding flows.
- Git-only rollback -> branch-per-VM rollback -> VM snapshot rollback -> product-visible recovery packet.

## Dense Feedback Channels

- Go tests for store, runtime, AppChangePackage/adoption, vmctl, vmmanager,
  gateway, and server.
- Frontend build and focused Playwright tests for every product surface touched.
- Product-path Playwright dogfood: prompt bar -> app/super -> worker VM ->
  AppChangePackage publication -> adoption -> verification -> promotion ->
  continuation.
- Trace assertions for roles, tool calls, candidate records, verifier results,
  package/adoption events, continuation events, and compactions.
- Git/source-lineage assertions for base ref, candidate ref, dirty state,
  divergence, and rollback command.
- VM assertions for worker identity, snapshot/lease metadata, export artifacts, and disposal/recovery.
- Run memory assertions for compaction before continuation and defined behavior near context limits.
- UI screenshots only when they prove real UI state, not cosmetic presence.
- Dogfood reports and next-frontier reports written in repo docs after every major loop.

## Forbidden Shortcuts

- Do not use Codex file edits as proof of Choir-in-Choir unless the change is also driven through candidate-world promotion evidence.
- Do not keep Codex as the main actor after Choir can use Playwright/product-path prompting to start its own super/vsuper loop.
- Do not hard-code a single next goal and call it self-development.
- Do not create fake APIs, test-only stores, or manually seeded success records that bypass production topology.
- Do not promote without verifier contracts and explicit approval semantics.
- Do not mutate canonical state from a worker, vsuper, researcher, or appagent.
- Do not build one-off launcher/uploads/themes/podcast patches that cannot become user-editable systems.
- Do not let theme presets replace theme creation/editing flow.
- Do not use frontend iframes where the architecture requires backend Obscura/browser execution.
- Do not treat compaction as optional when approaching long-run or context-limit behavior.
- Do not hide failed candidates; failure must leave diagnostics, learning, and the next safe probe.

## Product Pressure Sequence

The product path is not separate from architecture. Use visible product improvements as pressure to exercise the control system.

This sequence is a pressure field, not a ladder. Reorder it when feedback shows that a different slice exposes more invariant error, improves verification density, or better preserves the path toward Choir-in-Choir.

1. App launcher and desktop icons:
   - bottom-left control becomes a start/app launcher;
   - desktop icons exist for installed apps;
   - prompt bar routing to apps is manually and Playwright tested.

2. Files upload:
   - Files app gets upload UI;
   - backend upload path enforces per-user file-root boundaries;
   - uploaded content opens through existing app routing.

3. Theme creation/editing:
   - theme schema becomes user-editable UI;
   - presets include NeXT, classic Mac, Aqua, Frutiger Aero, Linux/GTK, Y3K, and current Windows-like style as examples;
   - users can promote/share custom themes through onboarding/demo flows.

4. Podcast/radio:
   - podcast app becomes a durable content artifact system;
   - feeds, episodes, clips, narration beats, sources, and listen paths connect to vtext semantics;
   - radio becomes a screenless traversal of promoted meaning.

5. Obscura/browser-in-VM:
   - browser app runs through backend Obscura/background VM control, not frontend iframe;
   - Choir desktop can view/control a background VM browser window;
   - this becomes the bridge for Choir developing Choir through its own product path.

6. Skills and mission gradient:
   - import/superset MissionGradient into Choir;
   - super/cosuper/vsuper/appagents support native skills;
   - skills are versioned, reviewable, and bounded by authority profile.

## Codex Role Transition

Codex may bootstrap the system, write missing substrate, and use Playwright to operate Choir through its own product path. That is acceptable only as a continuous deformation toward Choir driving the loop itself.

Once Choir can initiate candidate-world work through its own prompt/product path, Codex should stop acting as the main developer. Codex should instead observe traces, audit verifier contracts, repair broken bootstrap surfaces, and intervene only when Choir blocks, violates an invariant, or needs an external capability it does not yet own.

## Control-System Work

Build the controller that makes the product sequence self-propelling:

- run memory compaction near context limits and before continuation;
- objective synthesis from mission docs, queue state, verifier failures, app obligations, research packets, and product gaps;
- continuation queue with bounded authority and leases;
- promotion queue UI/API for candidate inspection and decisions;
- branch-per-background-VM and rollback proof;
- failed-candidate recovery path;
- portfolio selection over small tasks, probes, and risky branches.

## Rollback Policy

Git:

- every candidate records base SHA and worker head;
- integration branches are disposable until verified;
- destination branch promotion is explicit and blocks divergence;
- rollback command and report are saved.

VM:

- every worker has VM/snapshot/lease identity;
- failed worlds can be discarded;
- successful worlds publish typed packages and evidence before teardown;
- live rollback proof is required before claiming full VM safety.

Database/runtime:

- migrations are additive;
- run memory entries are append-only;
- adoption/promotion/continuation records are state machines with audit trail;
- context-limit failures become blocked/recoverable states, not undefined behavior.

Product:

- UI features require Playwright or component coverage;
- upload paths enforce owner/root boundaries;
- theme editing validates before application;
- user-promoted themes/skills are reviewable artifacts.

## Learning Side-Channel

Every loop must write durable learning:

- mission doc updates;
- dogfood proof reports;
- next-frontier reports;
- adoption/promotion reports;
- failed-candidate reports;
- run compactions;
- verifier contract improvements.

Classify discoveries:

- Tactical learning: apply immediately.
- Target-level learning: update mission doc or next-frontier report.
- Invariant-level learning: stop and ask before changing authority, trust boundary, promotion semantics, rollback, or canonical mutation rules.

## Dogfood Evidence

- 2026-05-13 product-pressure, context-limit, and podcast/radio proof snapshots
  were mined into `docs/old-docs-review-2026-06-06.md`.
- Legacy patchset-promotion proof files were pruned. Their durable lessons are
  preserved in `docs/legacy-promotion-experiments-learnings.md`.
- Old Trace promotion-candidate artifact proof was pruned with the legacy
  patchset queue.
- Backend browser proof shards were pruned. Their durable lessons are preserved
  in `docs/backend-browser-substrate-learnings.md`.

## Stopping Condition

Stop only when one of these is true:

- Choir has made a real product improvement through its own product path,
  candidate-computer execution, verifier contracts, typed package/adoption,
  explicit promotion, compaction, and automatic continuation into the next
  bounded objective; or
- the run is blocked by an invariant-level issue, with rollback point, failed evidence, and the next smallest safe probe documented.

Completion requires a next-state decision. If a goal finishes and a safe next objective exists, the system should continue or record the next continuation instead of ending as a dead stop.

## One-Line Goal

`/goal Use MissionGradient to execute docs/mission-choir-grand-deformation-v0.md end to end as one continuous Choir-in-Choir deformation, verifying and promoting only through candidate-world invariants and typed AppChangePackage/adoption records, and continuing until real self-development works or an invariant-level blocker is documented.`
