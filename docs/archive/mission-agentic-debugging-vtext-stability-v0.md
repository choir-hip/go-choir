# MissionGradient: Agentic Debugging And VText Stability v0

Status: draft for owner notes before execution
Date: 2026-05-31
Source draft: `agentic-debugging-vtext-stability.md` attachment plus owner
clarifications on 2026-05-31
Target environment: staging-first acceptance at `https://choir.news` for
platform-enabling changes; once Super Console exists, the repair loop itself
runs inside the affected user computer without GitHub Actions.

Doctrine note (2026-06-13): this mission records the cutover away from Trace
app and raw Terminal surfaces. Those names remain here as historical evidence,
not current product ontology.

## Goal Prompt Draft

```text
/goal Run docs/mission-agentic-debugging-vtext-stability-v0.md as a
Codex-operated MissionGradient mission. Hard-cut Choir away from visual Trace
debugging and raw Terminal usage: unship the Trace app, preserve unified
machine-readable logs/evidence, replace Terminal with a singleton Super Console
inside each user computer, and back that Super Console with out-of-process zot
so the user computer can debug and rebuild its own runtime/UI/app code when the
VText-driven MAS malfunctions.

Preserve invariants: VText remains the artifact-level surface and primary
automatic-computer interface; Super Console is repair mode, not the driver of
the computer; Super Console is singleton per user computer; zot runs as a
separate binary/process from the runtime MAS inside that computer; zot may read,
edit, command, rebuild, restart, and verify inside that computer, but it must
not become a MAS peer, appagent, scheduler worker, or VText writer; only the
VText agent writes canonical .vtext files; bug/diagnosis reports from zot are
ordinary markdown/text artifacts that VText can open; Trace app is gone with no
emergency visual route, while logs/events remain unified and processable; raw
terminal is no longer user-facing except as zot/Super Console command actuator
such as ! commands.

First reproduce and classify the current VText version-advancement regression.
Treat VText repair as state-machine inversion plus via negativa: remove
conductor-authored first drafts, prompt/classifier scaffolding,
requires-worker-grounding flags, and required-tool choreography unless the
reproduced transition proves one is essential. The simple target is conductor
routes, VText writes v1, VText sends durable co-agent messages when it needs
help, workers reply with durable updates/evidence, and VText wakes to write the
next version.

Then implement the whole first cut: Trace app hard cutover, singleton Super
Console replacing Terminal, zot session persistence in the user computer file
system, unified logs sufficient for zot to diagnose the failure, a small live
repair/rebuild/verification loop, and a markdown diagnosis report. Do not keep
Trace as an emergency UI, do not create multiple coding-agent consoles inside
one computer, do not let Super Console become a general scripting surface, and
do not claim VText is fixed without deployed/product-path version advancement
on the reproduced case.
```

## Thesis

Choir should stop building a human-facing Trace dashboard.

Trace remains as evidence: logs, structured events, JSONL streams, run bundles,
VText version history, acceptance records, build/restart/test output, diagnosis
bundles, and rollback references. But the Trace app should be unshipped as a
desktop app. No emergency visual Trace route should remain in the product path.

The replacement is not another dashboard. It is a singleton Super Console
inside each user computer:

```text
VText/MAS breaks
  -> owner or expert opens Super Console
  -> zot runs out-of-process from the runtime MAS
  -> zot reads unified logs, files, source, and process state
  -> zot forms one causal theory
  -> zot patches the smallest implicated user-computer source/runtime/UI layer
  -> zot rebuilds/restarts locally
  -> zot verifies locally
  -> zot writes a markdown diagnosis report
  -> VText can open/report on that markdown artifact
```

VText/MAS remains the primary automatic computer. Super Console/zot is the
repair tool when that system malfunctions. If Super Console becomes the normal
scripting surface or a place to run many chat agents, the product has failed.

## Product Model

Super Console is a user-computer app.

It is not a platform operator console and not a candidate promotion surface. It
runs inside the user's persistent computer and repairs that computer's own
private source/runtime/UI/app state.

Choir user computers may diverge. Their source/build state is private and
per-user. Local repair in one active user computer is distinct from later
distribution, inspiration, AppChangePackage publication, adoption, or platform
promotion. Code snippets and local fixes may inspire bespoke software elsewhere,
but they are not merged as-is into a global platform just because zot produced
them.

The important distinction:

```text
VText-driven MAS:
  primary automatic computer; artifact-level, multi-agent, more powerful,
  more complex, currently less stable.

Super Console/zot:
  singleton repair mode; single-threaded causal debugging; faster OODA when the
  VText/MAS layer breaks.
```

## Cognitive Transforms

Current uncertainty or obstacle:

```text
VText keeps regressing after near-success. The old response is to inspect Trace
or Terminal manually, which slows the OODA loop and creates another brittle
surface. The new route must increase repair frequency without turning Choir
back into "humans multitasking with chat agents."
```

Selected transforms:

1. **Depth Extraction** - the banal version is "add an AI terminal." The deep
   version is "increase OODA frequency for a sick user computer without making
   the MAS debug itself." The load-bearing variable is time-to-causal-repair
   inside the affected computer.
2. **Boundary Correction** - the trust/ownership boundary is one persistent
   user computer. zot is outside the runtime MAS process but inside the same
   computer, with authority over that computer's files and processes.
3. **Anti-Antipattern** - Choir exists to replace multitasking with many chat
   agents. Super Console must be singleton and exceptional; multiple "Claude
   Code" style sessions inside Choir would recreate the failure mode.
4. **Artifact Preservation** - zot reports are markdown/text artifacts. VText
   can open them, but zot does not write `.vtext`. The VText agent remains the
   canonical VText writer.
5. **Hard Cutover** - keeping a hidden Trace app preserves the old failure
   mode. Trace app goes away; unified logs/evidence stay.

Route-changing insights:

- The VText fix is deletion-led: remove the state-machine/control-flow cruft
  before adding another classifier, prompt rule, or hidden continuation.
- Conductor must route/open only. VText writes the first canonical appagent
  version through the normal VText edit path.
- The first artifact is a user-computer repair loop, not a platform operation
  surface.
- zot should not talk to the MAS as a peer/tool/agent. It should inspect and
  repair from outside: files, logs, commands, rebuilds, restarts, smoke checks.
- Unified logs are more important than visual Trace because zot can search and
  process them better than humans.
- Raw terminal should disappear as a user-facing app. `!` commands through zot
  are enough terminal escape for expert repair.
- The first proof must fix or precisely diagnose the VText regression because
  more VText regression is the failure mode this architecture exists to stop.

Changed plan:

- implementation: unship Trace app; replace Terminal launch with singleton
  Super Console; run zot out-of-process inside the active user computer; persist
  session logs/files locally.
- verifier/evidence: require VText failure transition ledger, unified logs,
  zot session log, local patch/rebuild/test output, markdown diagnosis report,
  and deployed/product proof for platform-enabling changes.
- scope: user-computer-level repair. Platform promotion/distribution is
  adjacent documentation context, not the inner loop.
- stopping condition: stop when the Trace app is gone, Super Console/zot can
  repair or diagnose the reproduced VText failure inside a user computer, and
  the result is captured as a markdown report openable by VText.

Next high-information action:

```text
Reproduce one current VText version-advancement failure and record its transition
ledger: document head, revisions, pending mutations, run states, worker
messages, controller checkpoint, unified logs/events, and frontend dirty/head
state. Use that exact failure as the first Super Console/zot debugging target.
```

## VText Via Negativa Repair Direction

The current VText failure pattern should be treated as too much machinery, not
too little. The repair direction is to invert the state-machine instinct.

Delete or collapse:

- conductor-authored first appagent versions;
- `create_initial_version` as a conductor content-writing contract;
- prompt taxonomy as runtime control flow;
- `requires_worker_grounding` and similar hidden route flags;
- `next_required_tool` as the primary way to force multi-step VText workflow;
- tests that bless conductor-written `v1` as the product contract.

Preserve and strengthen:

- conductor as exogenous router;
- VText as the only canonical document writer;
- user revisions as user-authored versions;
- durable co-agent messages as the coordination substrate;
- worker updates/evidence as inputs, never patches;
- VText wake/revision behavior after eligible worker messages;
- unified evidence showing which messages each VText revision consumed.

The desired first-draft path is:

```text
prompt
  -> conductor routes/opens VText with user seed
  -> VText writes v1
  -> VText sends co-agent messages only when help is needed
  -> co-agents reply with durable updates/evidence
  -> VText writes v2/v3/... from those updates
```

If a future implementation needs coordination policy, put it in durable
messages, revision metadata, and evidence that zot can inspect. Do not rebuild
the hidden state machine out of prompt wording, keyword classifiers, or
tool-choice obligations.

## Real Artifact

The real artifact is the singleton user-computer repair loop:

```text
Super Console app
  -> one zot session inside the current user computer
  -> unified log/source/file/process inspection
  -> local patch/rebuild/restart/verify
  -> markdown diagnosis report
  -> VText opens or incorporates the report through normal artifact paths
```

This is not the main automation surface. The main automation surface remains
VText plus the multi-agent system. Super Console exists because the MAS can
break, and debugging often needs one coherent causal story.

## Hard Invariants

- VText is the artifact interface to agentic computation.
- Super Console is singleton per user computer.
- Super Console is repair mode, not the normal user surface and not a general
  scripting/product driver.
- zot runs inside the user computer but out-of-process from the runtime MAS.
- zot may do anything inside that computer for repair: read, edit, run commands,
  rebuild, restart, inspect logs, and verify.
- zot must not become part of the MAS: no agent role, no appagent authority, no
  scheduler peer, no VText writer, no multiple concurrent chat-agent sessions.
- Only the VText agent writes canonical `.vtext` files.
- Conductor does not write appagent VText document text; VText writes the first
  canonical appagent version.
- zot diagnosis reports are ordinary markdown/text files. VText can open them.
- Trace app is unshipped. Logs/events/evidence remain durable and unified.
- Raw Terminal is not user-facing. Shell access exists through zot/Super
  Console command actuation, for example `!` commands.
- Inner repair loop must not require GitHub Actions.
- Platform-enabling changes still require the normal landing loop before claims
  about staging/product behavior.
- VText repairs must document the failing transition before code fixes.

## Unified Logs

Unified logs are the replacement for human Trace spelunking.

Functional requirements:

- low resource usage;
- append-only or otherwise hard to corrupt accidentally;
- simple for zot to search with ordinary file tools;
- covers frontend events, backend/runtime events, agent/tool events, process
  restarts, build/test output, VText revisions/mutations, and errors;
- can be bundled into diagnosis reports and acceptance records.

Preferred v0 shape:

```text
user-computer filesystem
  logs/
    runtime.jsonl
    frontend.jsonl
    agents.jsonl
    tools.jsonl
    builds.jsonl
    super-console/
      <session-id>.jsonl
      <session-id>.md
```

This can later be indexed or mirrored into Dolt if needed, but v0 should favor
simple files because zot can read, grep, summarize, and attach them with low
overhead.

## Source And Repair Semantics

User computers diverge. A user computer has private source/build state for its
runtime, frontend bundle, installed app code, and local artifacts.

Super Console repair mutates that user computer's active private source/runtime
state. That is intentionally dev-in-prod inside the user's own computer.

Distribution is separate:

- A local repair may remain local.
- A local repair may later be packaged or rewritten as an AppChangePackage.
- A local repair may inspire a platform fix.
- A local repair is not automatically a platform merge or shareable package.

The first mission should make this explicit in docs if current source-lineage
docs are too easy to misread.

## Value Criterion

Minimize time from owner-observed VText/MAS breakage to causal diagnosis and
local rollback-safe repair inside the affected user computer, while preserving
VText as the primary artifact surface and eliminating human Trace archaeology.

The mission moves uphill when:

- VText version-advancement failures can be classified by transition edge;
- humans stop scanning Trace as a normal debugging loop;
- zot can search/process unified logs better than a human;
- Super Console can patch/rebuild/restart the user computer without CI;
- bug reports become markdown artifacts that VText can open;
- raw Terminal disappears as a user-facing app;
- only one Super Console/zot session exists per user computer;
- normal users see less debugging machinery, not more.

## Quality Gradient

Expected quality level: **solid first cut**.

Solid means:

- Trace app is unshipped from the desktop/product path.
- Logs/events/evidence remain available as unified machine-readable substrate.
- Terminal is replaced by singleton Super Console.
- zot launches out-of-process inside the user computer.
- Super Console streams zot text/tool/command/result/error/done events.
- zot can run `!`-style raw commands when needed.
- zot session logs persist as files inside the user computer.
- The current VText failure is reproduced, classified, and used as the first
  repair/diagnosis target.
- zot produces a markdown diagnosis report.
- The inner loop can patch/rebuild/restart/test without GitHub Actions.

Minimal is not enough if it leaves Trace app or Terminal as fallback surfaces.
Excellent is not required; do not build a broad debugger product or generalized
scripting layer before proving the VText repair loop.

## Homotopy Parameters

Increase realism along these axes while preserving topology:

- Super Console: local visible app -> singleton app state -> persisted sessions
  -> diagnosis report history;
- zot authority: inspect-only -> command execution -> small patch -> rebuild
  runtime/UI -> restart/smoke -> rollback;
- logs: raw local files -> unified JSONL -> report bundle -> optional index;
- debugging target: static source issue -> VText local repro -> staging/active
  user-computer VText repro -> long-running self-development regression;
- Trace cutover: remove launcher/route -> delete app code -> evidence-only log
  projections;
- Terminal cutover: remove launcher -> Super Console command actuator -> raw
  PTY only as non-product implementation detail;
- source divergence: local repair -> local persistence -> optional package or
  upstream inspiration.

A simplification is valid only if it keeps the same topology: singleton Super
Console inside a user computer, out-of-process zot repairing that computer from
outside the MAS.

## Starting Belief State

Believed current state:

- VText version advancement is currently owner-visible broken again.
- Recent source review found likely recurrence points: pending mutation gates,
  worker wake/addressing, stale-head safety without retry, frontend dirty-head
  behavior, and prompt/tool choreography accretion.
- The Trace app is a brittle debugging surface and must be unshipped.
- Terminal is too raw and should be replaced by Super Console.
- User computers have or are intended to have private divergent source/build
  state; Super Console repair belongs at that level.
- zot has the right shape for an out-of-process maintainer/debugging agent.

Evidence:

- `docs/vtext-regression-review-2026-05-31.md`.
- Attached draft `agentic-debugging-vtext-stability.md`.
- Current architecture docs on active/candidate computers, personal promotion,
  and source/build divergence.
- Current code paths around VText revisions, pending mutations, worker wake,
  autosave, and tool continuations.

Main uncertainties:

- Which exact VText transition is failing now.
- Whether zot is already installed/available inside the user computer image.
- Exact v0 unified-log file layout.
- Exact rollback affordance for local zot mutations.
- How to expose singleton Super Console while removing Terminal cleanly.
- How much Trace app code can be deleted immediately versus only unregistered
  from product routes in the first landing loop.

Highest-impact uncertainty:

```text
Can singleton Super Console/zot inside the affected user computer reproduce,
classify, and repair the current VText version-advancement bug faster than the
current Codex + Trace + Terminal + GitHub Actions loop?
```

Next observation:

Run a product-path VText reproduction and build the transition ledger before
implementing the repair surface.

## Dense Feedback And Evidence Ledger

Required evidence for mission execution:

- VText failing document ID, current head, revision list, and owner/computer
  identity.
- `vtext_agent_mutations` rows for that document.
- run records for mutation run IDs.
- channel messages addressed to `vtext:<docID>`.
- controller checkpoint row.
- unified log excerpts for the failing window.
- frontend state: dirty editor vs latest head vs `New version available`.
- Super Console singleton/session record.
- zot session JSONL and markdown diagnosis report.
- local patch/rebuild/restart/test output.
- staging/deployed proof for platform-enabling changes.
- rollback refs or revert notes for local zot mutations.

Evidence quality rule:

Do not claim the Super Console works unless it launches an actual out-of-process
zot session inside the user computer and persists the session artifacts. Do not
claim VText is fixed unless the reproduced product path advances versions.

## Forbidden Shortcuts

- Do not keep visual Trace as an emergency UI.
- Do not expose raw Terminal as a user-facing app.
- Do not make Super Console a general scripting surface.
- Do not run multiple zot/coding-agent sessions inside one user computer.
- Do not make zot an agent role, appagent, scheduler worker, or VText writer.
- Do not have zot write `.vtext` files.
- Do not make zot talk to the MAS as part of normal debugging.
- Do not skip VText failure reproduction and jump to a speculative fix.
- Do not require GitHub Actions for the inner repair loop.
- Do not turn local user-computer repair into automatic platform promotion.
- Do not add more VText prompt taxonomy, hidden classifiers, or required-tool
  choreography as the primary fix unless transition evidence proves that is the
  smallest correct layer.
- Do not preserve conductor-authored `v1` as a compatibility shortcut.

## Receding-Horizon Execution

Control interval 1: reproduce and classify VText failure, then delete the
state-machine surface implicated by the evidence.

- Submit or reopen the failing VText product path.
- Capture transition ledger and unified log window.
- Name the failed edge.
- Identify which existing state-machine surface can be removed or collapsed:
  conductor seed, classifier, required-tool continuation, pending gate, or wake
  checkpoint coupling.
- Update this mission doc with evidence before code fixes.

Control interval 2: unship Trace app and replace Terminal affordance.

- Remove Trace from desktop launcher/default routes with no emergency visual
  fallback.
- Preserve logs/events/evidence records as machine-readable substrate.
- Replace Terminal entry with singleton Super Console.
- Keep raw shell as implementation detail behind zot command actuation.

Control interval 3: implement singleton Super Console/zot.

- Launch one zot RPC/session inside the user computer.
- Enforce singleton session behavior.
- Stream text/tool/command/result/error/done events.
- Persist session JSONL and markdown report under the user computer filesystem.
- Provide `!` raw command escape through zot.

Control interval 4: apply zot to VText.

- Ask Super Console to diagnose the captured VText failure.
- Let zot inspect unified logs/files/source/process state.
- Patch the smallest implicated layer or produce a precise blocker.
- Rebuild/restart runtime/UI as needed inside the computer.
- Run focused verification.
- Write markdown diagnosis report.

Control interval 5: product proof and documentation.

- Complete the normal landing loop for platform-enabling changes.
- Verify staging identity.
- Re-run the VText acceptance path.
- Confirm Trace app is gone and logs/evidence remain available.
- Confirm Terminal is replaced by Super Console.
- Update architecture docs and residual risks.

## Acceptance Criteria

Draft acceptance for the first mission:

1. A current VText version-advancement failure is reproduced or precisely
   classified as not reproducible on current staging.
2. The failed transition is documented before code fix.
3. Conductor no longer writes the first appagent document version; VText writes
   `v1` through the VText edit path.
4. Prompt/classifier/required-tool VText workflow scaffolding is deleted or
   reduced to the smallest evidence-proven invariant.
5. Trace app is unshipped from the product desktop/app routes; no emergency
   visual Trace route remains.
6. Unified logs/evidence remain available for zot and acceptance records.
7. Terminal app is replaced by singleton Super Console.
8. Super Console launches one out-of-process zot session inside the user
   computer.
9. Super Console streams zot text, tool/command calls, results, errors, usage,
   and done state.
10. zot can inspect VText-related logs/files/source/process state.
11. zot can run command-actuation including `!` raw commands.
12. zot either makes a small VText patch and runs focused verification, or
    produces a precise diagnosis/blocker with the smallest safe next probe.
13. zot writes a markdown diagnosis report that VText can open.
14. No GitHub Actions are required for the inner diagnose/patch/rebuild/test
    loop inside the user computer.
15. Platform-enabling behavior changes complete the landing loop.
16. Final report includes diagnosis, evidence refs, accepted/rejected
    hypotheses, rollback refs, residual risks, and the next realism axis.

## Non-Goals

- Full debugger product polish.
- Broad log indexing infrastructure.
- Keeping or redesigning the visual Trace app.
- Replacing VText with Super Console.
- Making coding-agent UX the main product.
- New automatic newspaper/media generation before VText stability is proven.
- General multi-agent repair orchestration.
- Automatic platform promotion of local zot patches.
- Telegram zot connection. Useful later, not this mission.
- Cron/scheduled Super Console usage.

## Rollback Policy

- Trace app unshipping should preserve a git rollback ref until replacement log
  evidence access is proven.
- Super Console/zot integration should be feature-gated at first if needed, but
  Terminal should not remain a normal user-facing fallback.
- zot sessions that mutate files must record diff summary and revert notes.
- If a local zot mutation breaks the user computer, rollback should be a local
  source checkout revert, previous build restore, process restart, or active
  computer snapshot restore depending on what was changed.
- VText code changes require focused verification and product-path proof.

## Learning Side-Channel

Record durable learning in:

- this mission doc for run state and decisions;
- `docs/vtext-regression-review-2026-05-31.md` if new VText recurrence patterns
  are discovered;
- architecture docs for the accepted Super Console/zot/Trace/Terminal boundary;
- markdown diagnosis reports for individual debugging sessions.

## Run Checkpoint And Resumption State

```text
status: accepted_on_staging
last checkpoint: deployed behavior commit 84c8c4f005db913cf47f5bc66e1bf55c10bfb224
  passed CI run 26706511492 and deployed to Node B at 2026-05-31T07:29:25Z.
current artifact state: conductor routes/opens VText without writing an
  appagent first draft; VText writes the first appagent revision through
  edit_vtext; worker evidence moves through durable co-agent updates; visual
  Trace is unregistered/deleted as an app; raw Terminal is gone from the product
  path; Super Console is the singleton desktop repair surface backed by
  out-of-process zot.
what shipped:
  - 22f8a8f docs: record vtext simplification mission
  - 1735aa1 docs: record vtext staging repro checkpoint
  - 9519606 unship trace and simplify vtext flow
  - f1f63bc fix sandbox zot session id test
  - 84c8c4f include zot in sandbox deploy package
what was proven before patch: two staging prompt-bar probes on ba49f0f advanced
  backend VText versions, so the owner-visible regression was not reproduced as
  a total backend version-advancement failure. The old source still contained
  the cruft surface that could keep reintroducing the failure.
what was locally verified: focused Go tests for VText first-writer and
  Super Console/zot PTY paths passed; full local runtime shard suite passed;
  `pnpm build` passed and emitted no Trace chunk; direct zot proof wrote
  `.choir/zot/sessions/proof-1/session.jsonl`, executed `!printf zot_ok`, and
  wrote `diagnosis.md`. Local `nix build .#packages.x86_64-linux.sandbox`
  could not compile on the aarch64-darwin workstation because no x86_64-linux
  builder was available; the same packaging path was verified by CI/Node B.
deployed VText proof: Playwright against https://choir.news with prompt
  "Write and run one tiny shell command that prints the SHA256 of the word
  choir..." passed. Submission a63c8c8e-b229-4f88-934f-6ab2a357b382 created doc
  1f74f922-106f-44da-a118-2528f56d48a2. Conductor decision had
  create_initial_version=false and initial_revision_id equal to the user
  revision 2b682fd9-a262-449e-a667-2cbb405c2b93. VText wrote appagent revision
  dc134b56-f118-44b3-b7fa-6c6acf1332f4, super returned durable command
  evidence, and VText woke to write appagent revision
  d5a51a53-7d41-48b4-9886-d13b3f9d2d95 containing command
  `echo -n choir | sha256sum` and output
  `1be0686a785a469ecfeba5a30f06d591c4e1f2135e0f5559a51e6cd4173f5327  -`.
deployed Super Console proof: staging health reported proxy and sandbox commit
  84c8c4f005db913cf47f5bc66e1bf55c10bfb224. Temporary Playwright proof showed
  no Trace desktop icon, no Terminal desktop icon, /api/terminal/ws returned
  410 "terminal app has been replaced by Super Console", one Super Console
  window opened, and zot persisted `.choir/zot/sessions/zot-1/session.jsonl`
  plus `diagnosis.md` inside the user computer. The session log contained
  start, diagnosis_report, command, and command_result records for
  `!printf super_console_ok`.
belief-state changes: the current staging acceptance supports the simple path:
  route -> VText v1 -> durable co-agent evidence -> VText v2. The historical
  state-machine/control-flow surface was removed rather than patched with more
  prompt taxonomy. Visual Trace is no longer a human debugging route; unified
  machine-readable trace/log/evidence APIs remain.
remaining error field: owner-visible UI head-follow/version list behavior has
  not been separately isolated beyond the backend product-path revision proof.
  zot is a first-cut REPL-style repair session, not yet a full structured
  rebuild/restart/verifier planner. Rollback for local zot mutations remains
  policy-level rather than typed product UI.
highest-impact remaining uncertainty: whether long-running mixed research/code
  VText documents keep the same clean revision cadence after many worker
  updates and compactions.
next realism axis: run a longer VText research/code document through the same
  product path and make zot consume the resulting unified evidence bundle
  without using a visual Trace UI.
evidence artifact refs:
  - test-results/vtext-version-advancement-final-20260531T073750Z/repro-2026-05-31T07-38-28-871Z.json
  - test-results/super-console-staging-proof-20260531T074326Z/proof-2026-05-31T07-43-37-592Z.json
rollback refs: revert 84c8c4f, f1f63bc, and 9519606 to restore the prior Trace
  app/Terminal/VText-control-flow behavior. Prefer a forward fix unless a
  staging blocker appears, because rollback restores the regression surface.
```
