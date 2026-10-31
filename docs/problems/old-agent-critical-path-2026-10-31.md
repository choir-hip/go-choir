# Old parts of the agent critical path still live

Date: 2026-10-31. Status: recorded, parked until after the Mac app (owner priority rule). Read-only audit (Sonnet) of origin/main at abbb0a1ac, code and git history only. Companion to docs/problems/desk-runs-seeded-from-prior-run-2026-10-31.md.

# Old-era parts of the agent critical path (read of origin/main abbb0a1ac, 2026-10-31)

Method: code and git history only. Traced executeActivation -> executeWithToolLoop
(runtime.go) -> systemPromptForRun (tool_profiles.go) -> runContextForRun
(run_context.go) -> runMemoryManager.initialize (run_memory.go) -> RunToolLoop
(toolregistry/toolloop.go) -> run end (runtime.go, run completion, outcome binding).
Dated with `git log -S`. No builds, tests, run data or staging touched.

Baseline of what is already modern (so it is not re-flagged): desks have exactly one
tool (desk_go_eval; capsule_go_eval for engineering), see buildDeskCellRegistry /
InstallDefaultAgentTools. The typed tool channels are gone. The system prompt is a
byte-stable per-desk prefix, with the time and run facts moved into a run-context user
turn (3f142fe2f, 2026-10-14). The wake turn for update packets is already
"pointer, not payload". Texture's frame, role prompts for research/management and the
texture.yaml core are October text. A sweep (48a4608ce, 2026-10-12, "stop prompts and
tool results naming tools and routes that do not exist") already ran once; the items
below survived it.

Run flow on the default path, for orientation:
1. Run created (lifecycle control activation or Texture reconcile); first user message is
   `rec.Prompt`.
2. executeWithToolLoop builds [run-context turn, prompt], then `memory.initialize`.
   If the run has no memory it calls the seed (finding 1) before appending those.
3. Texture frame and calibration injectors add turns.
4. RunToolLoop appends the tool catalog to the system prompt, calls the provider, executes
   desk_go_eval cells, injects mailbox turns after each batch, compacts at a token
   threshold.
5. End: end_turn, completion guard, park or passivate, then
   handleRunCompletion/afterRunCompleted/bindTerminalRunOutcome.

## Ranked findings, confirmed (live on a default path)

### 1. Prior-run seed (the model finding) - context interplay worth adding
- What: `seedActorMemorySnapshot` (run_memory.go) writes the prior run's digest into the
  new run. Already documented; not re-derived here.
- Commit: a7b43100b, 2026-06-13 (after the durable actor, before Yaegi/RLM). Texture
  carry: 5f1f056a3 2026-06-18, then the Oct 25 hold_trajectory carry.
- Additional facts found:
  - Applies to every stable agent id: `management:<owner>`, `research:<doc>`,
    `texture:<doc>`. It does not apply to engineering (id per assignment) or research
    children (unique ids).
  - The REPL session already carries state across runs. desk_go_eval's description says
    "The desk's session continues across its runs", and the first cell says whether it is
    new or continuing (tools_desk.go:425). So the seed is a second, lossy continuity
    channel beside the real one. When the session was killed, the seed still says
    "continue the same run".
  - The research prompt (promptstore/defaults/research.yaml) relies on both: "what you
    learned earlier still counts", "your Go session usually continues between runs".
    Removing the seed needs the REPL path to be real for research and management first.
  - Only the research desk can read raw prior entries back (`choir.RunMemoryEntry`,
    `runMemoryRetrievalVerb`). Management and Texture have no on-demand lookup, so
    "available in the REPL" is not true for them today.

### 2. Compaction checkpoint: typed-JSON format from before the durable actor and Yaegi
- What it does: when estimated prompt pressure crosses a context-window ratio,
  `compactIfNeeded` asks the model for a JSON checkpoint with 15 fixed fields
  (`buildRunMemoryCompactionPrompt`). `renderRunMemoryCheckpointSummary` turns it into a
  user message "Context checkpoint summary. Compacted prior provider context estimated at N
  tokens". On LLM failure `deterministicEmergencyRunMemoryCompaction` is used.
- File/function: internal/agentcore/run_memory.go - `buildRunMemoryCompactionPrompt`,
  `renderRunMemoryCheckpointSummary`, `compactionSummaryMessage`,
  `deterministicEmergencyRunMemoryCompaction`.
- Commits: compactionSummaryMessage e759ce3f1 2026-05-13 (before the durable actor);
  LLM checkpoint aa8bee5fe 2026-06-08 "Add LLM run-memory compaction" (before the durable
  actor, before Yaegi, before the RLM cutover).
- Why made: keep long tool-calling runs inside the context window.
- Live: yes. Every run that crosses the threshold, every desk.
- What it puts in context: the whole prior conversation is replaced by a summary shaped
  as a tool-loop agent's state: "tool obligations", "raw_tool_result_handles",
  "Source/evidence/artifact handles", "Files/docs/resources". It does not name REPL
  session variables or choir handles.
- Why wrong now: it never mentions the REPL session (which survives), so the model can
  restate what is already defined in the cell. For management and Texture the prompt says
  "This desk cannot retrieve compacted entries; keep everything the next turn needs in the
  checkpoint fields", which pushes all the content into context. The list "Raw entry
  handles" gives ids that are unreadable except on research.
- Replacement: none wired. The REPL session plus an on-demand run-memory verb for every
  desk is the natural one.

### 3. Tool catalog appended to the system prompt on every provider call
- What: `RunToolLoop` does `systemPrompt = BuildSystemPrompt(systemPrompt, registry)`,
  appending "Available tools:\n- desk_go_eval - <description cut to 80 chars>..." to the
  system prompt, in addition to sending the tool definition.
- File/function: internal/toolregistry/toolloop.go:369, `BuildSystemPrompt`/`Catalog` in
  toolregistry.go:171-194.
- Commit: the loop is from 9f9e8fe52, 2026-04-11; moved to toolregistry
  b98531cd1 2026-06-27 and fd049fc54 2026-07-14. Pre-actor, pre-Yaegi.
- Live: yes, every call of every desk with a registry. It is part of the cached prefix.
- Puts in context: a truncated one-line copy of the tool description, ending mid-sentence
  (the description is cut at 80 chars of "Evaluate model-authored Go source for this desk in a
  killable host subprocess..."). It says "tools" (plural catalog) for a desk that has one
  surface.
- Why wrong: with one tool, the catalog repeats the schema and truncates it; the real
  description is in the tool definition. Pure noise per turn.
- Replacement: tool definitions already carry it. Delete the call.

### 4. Skill context in the system prompt, from the vsuper/shell era
- What: for Conductor, Texture, Management and Engineering, `skillContextForProfile`
  appends "Available repo skills (natural-language use; no slash commands):" and a
  1200-byte extract of skills/cognitive-transform-portfolio/SKILL.md, with the line
  "Source: <absolute path>", then "Use `cognitive-transform-portfolio` before stopping on a
  blocker...".
- File/function: internal/agentcore/skill_context.go - `skillContextForProfile`,
  called from `systemPromptForRun`.
- Commit: 5af8828e4, 2026-05-15, "Harden sweep substrate for vsuper workers" (before the
  durable actor and Yaegi; agents had shell and file tools then).
- Why made: tell file-capable workers where the repo skills are.
- Live: yes wherever RUNTIME_SKILLS_ROOT is set; nix/autoputer-vm.nix:122 and :1059 and
  nix/node-b.nix:175 set it.
- Puts in context: a repo path and an extract about "natural-language use; no slash
  commands", aimed at desks that cannot read that path. The Yaegi allowlist forbids `os`,
  `os/exec`, `io/ioutil` (yaegikernel/allowlist.go:37-41); I did not enumerate every
  choir.* file verb, so "cannot reach it" is a strong hypothesis, not proven.
- Why wrong: points at a host path from desks that run in a restricted cell or capsule;
  the instruction "Use cognitive-transform-portfolio" is an instruction the desk cannot
  follow by name. Also it is in the system prompt of three desks and not of research.
- Replacement: none. Mission skills belong in the throughline/doctrine, not the desk
  prompt; if wanted, as a REPL-readable item.

### 5. Core prompt still describes the Conductor-era world
- What: `corePrompt()` (run_context.go:34) loads promptstore/defaults/core.yaml and puts it
  first in every desk's system prompt. It says "Conductor routes top-level intent",
  "When outside facts, validation, or real system work are needed, use the right tools or
  delegate to the right functional role", "The runtime may thread that delivery into the
  loop as a normal next user turn. Do not poll for inbox state manually", "durable agents,
  explicit coordination edges".
- File: internal/promptstore/defaults/core.yaml (body); `corePrompt`.
- Commits: the conductor line dates from 41cad630d 2026-04-16 ("vtext attempt") and
  ec829c53d 2026-04-16 "Route prompt bar through conductor"; current file last changed
  a14abf055 2026-09-10 (V2 writer cutover; last touch).
- Live: yes, every desk, every call.
- Why wrong: the Conductor makes no model call now. `submitPromptBar` ->
  `CompletePromptBarDecision` records a deterministic decision (texture_model_api.go:
  "records a deterministic conductor decision"). The desks are system and app desks
  (I26); "tools" is not what a desk uses. "Do not poll for inbox state" teaches a delivery
  model that records and choir.Records() replaced.
- Replacement: none; rewrite to the desk ontology.

### 6. The update-packet wake turn still speaks the June 2026 mailbox
- What: `buildCoagentUpdateUserMessages` emits a user turn: a preamble "Choir coagent
  update packet (cold activation backlog)" plus a JSON envelope with owner_id,
  computer_id, target_run_id, channel_id, trajectory_id, an `update_refs` list, a
  redundant `updates` id stub array, `source_entities` and `source_instruction`, and an
  instruction string "New update_coagent records arrived ...". update_coagent is not a
  tool on any registry (grep: only metadata strings and error text remain).
- File/function: internal/agentcore/coagent_update_packet.go - `buildCoagentUpdateUserMessages`,
  `coagentUpdateInstruction`, `coagentUpdatePacketPreamble`;
  management_controller.go - `coagentUpdateTurnInjectorWithInitialPhase`,
  `prependInitialCoagentUpdatePackets`, `coagentParkWaiter`.
- Commit: 2561c917f, 2026-06-17 "runtime: unify coagent update delivery" (after the
  durable actor, before Yaegi). Slimmed to pointer-not-payload later.
- Live: yes whenever a desk has a pending packet (cold, mid, final-checkpoint). The final
  checkpoint injection runs at the end of every turn that has packets.
- Puts in context: ids and routing fields (computer id, owner id) the model has no use
  for, and a retired tool name. The `sourceInstruction` text is Texture-document-era
  ("Texture source entities/transclusion refs").
- Why wrong: Cut 1 (Oct 12) moved conversations to the records inbox; its wake "carries no
  work" and the desk reads choir.Records() (handler.go handleInboxRecord comment). The
  packet path remains for what "stays on its packet path" (self-development reports,
  bound producer reports); I did not enumerate every remaining sender.
- Replacement: records inbox (Cut 1 L3a/L3b/C1e). Partly wired; the packet turn is the
  residual second channel.

### 7. First user message of a lifecycle activation
- What: `lifecycleControlActivationPromptForProfile` makes the run prompt
  "Continue assigned actor work. Process the coagent update packets in context." plus
  `buildAssignedWorkItemPromptForCarrier` (work items, step/token budget, "Final text and
  RunRecord completion do not settle work").
- File/function: management_controller.go:1759, runtime.go:3217.
- Commit: 2561c917f 2026-06-17 (first line); 23672e587 2026-07-23 (work-item block).
- Live: yes for lifecycle Research (profile==Research takes the choir branch). The
  non-carrier branch, which tells the model "call update_coagent with
  work_disposition=completed", is reachable only for profiles that are not Management or
  Research. management_controller.go:2008 returns early for Management, Email, Conductor,
  and runtime.go:3150-3160 returns nil for Texture, so that branch is near dead.
- Why wrong: the "coagent update packets" are pointer stubs now, and the true state lives in
  records; "work_disposition" survives as a Report field. The dead branch names a tool that
  does not exist.
- Replacement: records inbox plus choir.Report; delete the update_coagent branch.

### 8. Run-context overlay wording from the tool era
- What: run_context.yaml and `runContextForRun` tell research runs "Texture delivery
  scope: ... use only a report or message module already exposed by your desk" and give
  every desk "Current coordination channel: <id>".
- File: internal/runtimeprompts/overlays/run_context.yaml; `runContextForRun`.
- Commit: 3f142fe2f 2026-10-14 moved it into the run-context turn; the wording is the
  2026-06 report/message-module era.
- Live: yes, research with a Texture requester; the channel line for every run.
- Why wrong: a desk's only surface is choir.* verbs. "module exposed by your desk" has no
  referent; the coordination channel is a legacy channel id (channelIDForRun falls back to
  the agent id or the run id).

### 9. Run end: final prose still becomes a packet for the requester
- What: `afterRunCompleted` -> `bindTerminalRunOutcome` binds a delegated child's final
  safe packet to the terminal RunRecord and re-drives stranded lifecycle controls.
  `normalizeCompletedRunResult`/`handleRunCompletion` still carry conductor decision
  normalization and the "legacy-protocol recovery" of pending Texture mutations.
- Files: research_checkpoint_fallback.go (misnamed; holds `bindTerminalRunOutcome`),
  runtime.go:4216, 4594-4650.
- Commits: f46b6bd18 2026-09-25 and 66981cef6 2026-09-24 touched it; origin 2026-06-17
  (2561c917f).
- Live: yes for research children and persistent desks.
- Inherits to the next run: the requester gets the child's final text through the packet
  path (finding 6), and rec.Result is stored. Not a context-shaping issue by itself, but it
  is what keeps the packet path alive.

## Dead or nearly dead code in the loop (not shaping context, still on the path)
- Required-next-tool and initial-tool-choice machinery in RunToolLoop
  (`extractRequiredNextTool`, `requiredNextToolProtocolAllowed` returns false always,
  `WithInitialToolChoice` has no non-test caller). Commit 820fa74d3 2026-07-12.
  `requiredInitialToolChoiceReminderText` still names `patch_texture`. Dead; delete.
- `executeWithProvider`/`providerPromptForRun`/Conductor and Email empty registries/
  `conductor.yaml`/`ConductorRunOverlay`/PeerCoordination non-cell branch (the "report or
  message module exposed in the current tool catalog" text). Only reached by a model run of
  profile Conductor or Email with an empty registry. The prompt bar does not run one;
  Email is only referenced as an exclusion and in model policy. Caveat:
  textureowner/api_texture_prompt_eval.go builds a Conductor-profile run, so I could not
  rule out an eval path that still runs a model; treat as "appears dead".

## Suspicions (not confirmed live or not shown wrong)
- promptstore user overrides: `Load(ownerID, role)` returns an owner-saved role prompt
  (users/<owner>/<role>.md) in place of the default, wholesale. An owner who saved a prompt
  before the RLM cutover would have a stale desk prompt, and defaults being rewritten each
  boot would not reach it. I did not read any owner content, so I do not know if any exist.
  Check the Save/Reset API callers.
- Run-per-wake model: every wake mints a new RunRecord with its own run memory, which is
  why a bridge (the seed) was needed. After the durable actor (06-11) memory is conceptually
  per agent, so an alternative is one resident memory per agent with the REPL as continuity.
  This is the cause behind finding 1 and 2, not a separate bug; recorded for the
  substrate-level question under "Convergence Before Patching".
- textureRunRevision has a dual store branch: `LifecycleVersion > 0` reads lifecycle
  documents, else legacy `GetDocument`/`GetRevision` (texture_frame.go). A pre-lifecycle
  run could still show a frame from the legacy store. Untested.
- Calibration digest and intake.labels/event_wakes injectors are arm- or setting-gated and
  default false (computer_setting.go:229-231). They are October code, not old-era, but they
  conflict with the "on, never off everywhere" rule. Noted only.
- Compaction input uses `rec.Prompt` as original_prompt (1400 chars); for lifecycle runs
  that is the finding-7 text, so the checkpoint's "objective" can be "Continue assigned
  actor work". Hypothesis from code reading; needs a trace to confirm.

## Not found / checked and clean
- No legacy tool-call channel is exposed beside the REPL for management, texture,
  research or engineering (desk_go_eval / capsule_go_eval only).
- Provider call itself (ToolLoopRequest, fallback, transient retry, cache key) is generic and
  current. `stripProviderReplay` and reasoning_content passthrough are provider plumbing.
- Texture's prompt core (texture.yaml v2), frame injector and research/management overlays
  are October text and reference choir.* verbs.
