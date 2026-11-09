# System prompt review: simplification with behavior held constant (2026-11-09)

Read-only. Repo: /Users/wiz/Dev/go-choir/.claude/worktrees/followup-token (main 5dc98bcf5). Prompt texts are the built prompts the earlier audit saved in `$SCR/eff/prompt-<desk>.txt` (built from a5103d5a0, close to this main). `SCR=/private/tmp/claude-501/-Users-wiz-Dev-go-choir/f298736e-cabb-51ff-9cea-34588a4989cb/scratchpad`. Labels: (M) measured, command named or obvious; (I) inferred; (H) hypothesis. Tokens = chars / 4.3 (the audit's calibration). Traces used: `$SCR/eff/{research,texture,management}-events.json` (14 research, 22 texture, ~19 management-task runs, 2026-11-09 evening), `$SCR/eff/runs.json` (500 paper runs). No model call, no live POST.

## 0. Where prompts come from (so the edits land in the right file)

`systemPromptForRun` (internal/agentcore/tool_profiles.go:176-262) assembles, per desk: `core.yaml` (1,099 chars, same on every desk) + "Role-specific instructions:" + role prompt (promptstore default or per-owner override) + overlay (`internal/runtimeprompts/overlays/rlm_<desk>_runtime.yaml`) + generated verb surface (`yaegikernel.DeskSurface`) + `peer_coordination.yaml` trailer (195 chars). A management task is built separately (`managementTaskSystemPrompt`, tool_profiles.go:461: core + task overlay + surface, no trailer). Time, ids, assignment and the capsule browser recipe are in the run-context user turn (run_context.go:56-168), not the system prompt.

Files that carry model-facing standing text:

| source | chars | goes to |
|---|---|---|
| promptstore/defaults/core.yaml | 1,261 | all desks (first 1,099 chars of every prompt) |
| promptstore/defaults/{management 6,695, research 5,586, engineering 2,699, conductor 1,495}.yaml | | role part (per-owner override possible via PUT /api/prompts/{role}) |
| textureprompts/texture.yaml | 8,780 | Texture role + overlay in one file |
| textureprompts/texture_root.txt / texture_root_context.txt | 1,989 / 710 | master-document run's request turn only |
| runtimeprompts/overlays/rlm_management_runtime.yaml | 27,610 | management |
| .../rlm_management_task_runtime.yaml | 5,177 | management task |
| .../rlm_research_runtime.yaml | 13,100 | research (and both research child roles) |
| .../rlm_engineering_runtime.yaml | 30,076 | engineering |
| .../{conductor_run, peer_coordination, run_context, temporal_context}.yaml | 1,388 / 554 / 768 / 600 | see sections |
| agentcore/research_children.go:65-66 | ~600 each | child preamble, prepended to the 38k research prompt |
| agentcore/run_context.go capsuleBrowserRecipe + computer-state | ~6.3k | engineering run's first user turn |
| claudedelegate/spec.go SystemContext + Prompt | ~2.2k code | Claude delegate `--append-system-prompt` and stdin |
| agentcore/coagent_update_packet.go | | wake-turn JSON envelope (every update wake) |
| Jev: intake_labels.go:313-333, event_layer.go, frontimportance/scorer.go:340, gateway/jev.go | tens to hundreds of chars per question | classifier questions, not agent prompts |
| appdesk/firstparty_prompts/mail-reader-drill.md | 523 | drill desk only. Mail itself (maildesk) has no model prompt. |
| pii/slm_redactor.go:97, modelpolicy/tools_model_verify.go:209, gatewayruntime/provider.go:128 | one-liners | utility calls, out of scope |

Sizes by desk (M, from the saved builds; production adds intake and event-wake text to management, so 49,908 on the gateway line vs 42,593 built):

| desk | chars | tokens (I) | verb surface (M) | surface share |
|---|---|---|---|---|
| research | 37,727 (gateway 37,943) | 8.8k | 18,729 | 50% |
| management (conductor in the audit's naming) | 42,593 built / 49,908 prod | 9.9k / 11.6k | 11,470 (+ intake verbs) | 27% |
| engineering | 35,980 | 8.4k | 8,826 | 25% |
| texture | 15,541 built / 17,637 prod | 3.6k / 4.1k | 7,553 | 49% |
| management task | 11,836 (gateway 11,986) | 2.8k | 6,111 | 52% |

(The `conductor` desk in deskmanifest is a different thing, see bug B5.)

Verb documentation is the other investigator's. I only note two structural facts about it: the same `req`/p_success paragraph is printed in every Ask/Cast/Escalate/Decline/Feed doc (3 copies, 1,473 chars on management, task and research; 2 copies, 982 on texture) and again in the overlay prose (5 mentions on management); and the generic per-verb docs leak other desks' text (Texture sees "A management task's Done ends the task...", every desk sees Pack's "Waits (management)" and Context's "An engineer's also has elapsed_min"). Pack alone is 912 chars on every desk.

## 1. Per-prompt breakdown and classification

Legend: LB load-bearing; DUP duplicated; CON contradicted; STALE; PoP patch-on-patch; EMP over-emphatic; HOST host could do it.

### 1.1 Research (37.7k chars; 710 calls/h on the paper per the audit)

Section sizes (M, offsets in prompt-research.txt):

| section | chars | class | note |
|---|---|---|---|
| core | 1,099 | LB/DUP | see 1.6 |
| role text (research.yaml) | 3,465 | mostly LB | "Report early and pack everything", "Read widely", "8 independent sources" added 2026-11-08 by 8d53b58b5 ("research drops most sources and reports late", Ring-Expedite). "Work in rounds", "Send a promising lead" restate it. |
| "Read before you cite..." + children | 1,011 | LB, DUP | read-before-cite appears in role text, here, and Evidence Doctrine (3 times) |
| "Searching is asynchronous" | 661 | DUP | again in "Search (async)" bullet (916) and the verb docs (SearchStart 481, SearchWait 230, Searches 607) |
| "Research Authority & Doorway" | 469 | STALE | "no typed tools at all... a tool name outside the cell does not exist" describes a removed regime (SR 2026-10-05 deleted the typed surface, prompts.go:114-118). The model has one tool; this tells it about the absence of others. |
| "Go REPL Workbench" | 812 | DUP | same paragraph on management (797), engineering, task; includes "one desk_go_eval per turn" which is also in the tool description |
| "Research Cell Surface" intro + Observation list | 153+ | DUP | lists verbs already in the generated surface |
| Search (async) bullet | 916 | DUP | |
| Closed questions (System 1) | 819 | DUP | verb doc System1 is 1,339 |
| Related stories | 1,243 | DUP | verb doc RelateStories is 1,338; commit test `TestRLMResearchOverlayListsRelateStoriesWithItsContract` pins both |
| What a story is about (entities) | 2,317 | DUP + HOST | verb doc Relate is 1,887. The procedure is: read StoryAnalysis (host preloaded System 1's answers), copy role/kind answers into Relate("", entities) before the final Report. The final Report is refused until done. Mechanical. |
| Coordination, Reporting & commitment | 527+1,354 | DUP | ReportPacket packet schema is spelled here and in the ReportPacket verb doc and in Records paragraph (1,544) |
| Records paragraph | 1,544 | LB (partly DUP) | restates "interim packet" a 4th time and adds the final-Report rules and `p_complete` |
| Evidence Doctrine (unchanged) | 1,942 | DUP | restates corpus-first, async search, read-before-cite, untrusted data. Nothing here is new; "(unchanged)" is a leftover label. |
| Error Recovery Doctrine | 666 | LB/DUP | compile/panic semantics also in the shared REPL manual that every desk repeats |
| generated surface | 18,534 | other investigator | 36 verbs; Cast, Escalate, Precommit, PollSource, SourceAPI listed though "Research does not cast work to other desks" |

Cadence stated four ways (CON). Quote them:
1. "Report early and pack everything. Send an interim packet as soon as the first source is captured; do not wait for the search to finish."
2. "Work in rounds. After each round, send Texture everything you found with choir.ReportPacket, then keep going on what is still open."
3. "Send the first interim packet as soon as you have read one new source you can cite, and another each time you have read more" (Records paragraph).
4. "A search arrival never wakes you: read the results you need before you end your turn."
Plus "read at least 8 independent sources across distinct hosts before your final Report". Behavior in traces: see bug B1.

History: the role text was grown by dated yellow/orange fixes (git log -S): interim policy 73b84cb78 (2026-10-21, "Texture waited minutes for it"), children text 01b9457e1 (2026-11-05), the 8-source floor and "Searches.Summary host-deduped" 8d53b58b5 (2026-11-08). The 8d53b58b5 fix also added the host nudge (read_ledger.go:179), so the prompt sentence and the nudge now say the same thing. The causes it fixed are code-enforced where they can be (ReportPacket pack warning, Reads ledger); the prompt lines that remain are policy.

### 1.2 Texture (15.5k built, 17.6k on the paper; 460 calls/h)

| section | chars | class | note |
|---|---|---|---|
| core | 1,099 | | |
| role + "Each turn begins with your frame" | 946 | LB | |
| "Write with choir.Write..." (directives, cite grammar, image/file/video/audio) | 1,371 | DUP | the Write verb doc (1,892) restates the directives, cite grammar and the four embed forms |
| "Close each turn" | 1,006 | LB/DUP | Done/Wait/Reject docs (313/296/190) repeat; the "Write alone is refused while the owner's request has no next actor" is host-enforced (it is a refusal, the error teaches it) |
| "The owner's intention comes first" | 1,114 | LB | all five bullets guard real failures (summary-casting, re-cast, rebase notice); keep, tighten |
| "Revision shape" | 2,381 | LB, EMP | 8 bullets, 18 "never"; Source-description ban list is useful but spends 400 chars of examples |
| "Stop asking for research when its answers stop changing the document." | 71 | LB | |
| generated surface | 7,358 | other investigator | |
| peer-coordination trailer | 195 | STALE/CON | "Coordinate with peers through addressed acts (choir.ReportPacket, choir.Cast, choir.Ask)": Texture has no ReportPacket in its surface (verified in the surface list above) |

Contradiction to fix (CON): V1 rule 1, "V1, your first revision on a new request, is a fast acknowledgment: ... the plan at the level of ideas ... No code, no answer from model priors", then bullet 2, "A story (its Cast says publish: auto) publishes every revision, V1 included, so its V1 is news, not a plan." Two V1 definitions, one conditional; make the condition the first line.

Nothing in the Texture prompt tells it to wait or batch interim packets. "choir.Wait(why) when the document waits on something named" applies to the open-research case, and in the 22 sampled runs it is paired with the Cast in the same cell (3 Cast cells; 2 Waits, 21 Dones, M). Ruling 1 (never batch) is not contradicted by the text. What does cost Texture calls is the frame: sources are "shown without their words" (prompt line "choir.ReadCapture("r1.c1", 0, 0) reads a source shown without its words"), so 14 of 22 runs open with a read-only cell, with 33 ReadCapture and 21 CellOut cells in 22 runs (M, texture-events.json). That is a frame design issue (put the new record's capture text, bounded, in the appended wake message; appended bytes are cache-safe), not prompt text.

### 1.3 Management (42.6k built / 49.9k production; 240 calls/h on the paper)

| section | chars | class | note |
|---|---|---|---|
| core + role intro | 1,099+731 | | "Generic Engineering spawn and capsule lifecycle authority do not exist" (STALE-ish negative) |
| "Answering a Texture request" | 3,148 | LB | bullets on report shape, receipts, "cast must be executable" (925ce5a64, 2026-10-10) |
| "Living stories (Autopaper)" | 2,643 | LB/DUP | the p_success paragraph (also in Requests bullet and verb docs); Decline/Feed rules overlap the task overlay |
| Authority + REPL Workbench + Cell Surface intro | 386+797+352 | DUP/STALE | "not a host tool loop... every affordance is an in-cell choir function" |
| Delegation bullet | 4,233 | LB, PoP | 2.0k is the `network` field essay (claude mode vs ecosystem_proxy vs forbidden, which of them shuts the delegate out, loopback for choir-shot). The default is "leave network out"; the host picks. Plus the predictions schema (required, refusal teaches it), plus the rebase_notice recipe (rebase text appears again in Texture's prompt). |
| Verification bullet | 1,206 | LB | |
| Requests / Calibration | 896 + 1,290 | DUP | Requests = the p_success paragraph for the 3rd time; Calibration = hypothesis probability + kinds, which the task overlay repeats nearly word for word |
| Intake labels / Delegate capture intake / Event records (production only) | 735+1,200+1,707 | LB, conditional | text switched on by two computer settings (see S1) |
| Product API | 575 | LB | |
| Screens | 2,677 | DUP + HOST | the six screenshot questions |
| Shots owed | 3,731 | DUP + HOST | the six questions again, plus ShotStart/ShotWait/ShotVerdict choreography |
| Install review owed | 1,566 | LB but rare | |
| Cast exact desk name | 283 | HOST | refusal already says it |
| Records / Updates paragraph | 1,772 | LB/DUP | |
| "Delegating Engineering" | 2,480 | LB | includes the "first act is an Ask to the bound engineer" rule |
| Cancellation | 236 | DUP | CancelAssignment verb doc says it |
| Error Recovery Doctrine | 1,022 | LB | the `x := recs[0]` rule (d2324a551) is a real compiler quirk; keep |
| generated surface | 11,275 | other investigator | |

Screens + Shots owed + Install review owed = 7,974 chars (17% of the production prompt, 1.85k tokens) for events that are absent from the paper's normal cycle. They are record-triggered ("A 'Shots owed on <operation>' record..."), so the instruction can ride the record: the owed record is appended to the transcript when it arrives (cache-safe), carrying the choreography and the one copy of the six questions.

CON inside management (quote): Screens bullet, "(5) desktop 1100x750, 1280x800 or 1440x900, phone 390x844 at any DPR" is listed among the questions "each must answer yes to", while Shots owed says "q5 is never yours: the platform checked each shot's size before showing it to you, so never fail a shot on its size". And Screens says "A Verify "pass" on a candidate with screens runs the six questions over every shot itself", while Shots owed opens "you shoot and judge a platform candidate yourself; the platform judges nothing". Two procedures, one text. Which applies depends on package versus platform candidates; say so once.

### 1.4 Management task (11.8k; 400 calls/h)

Overlay 5,177 chars + surface 6,111. Sections: job and deadline (LB, 400), "Your records" (LB, 530), req paragraph (DUP, ~700; also 3 verb docs), disposition rules (LB, 3,100, of which the hypotheses/forecast spec is ~1.4k), Ending (LB, 640). The rules that guard host-refused behavior (second disposition refused, open from stale stories refused) are enforced by refusals that name the fix, so the prose needs one clause each.

### 1.5 Engineering (36.0k; rare on the paper, the self-development path)

Overlay 30.1k chars, role 2.7k, surface 8.8k, plus ~6.3k recipe in the first user turn, plus the delegate's own context (spec.go SystemContext, 2.2k).

| section | chars | class |
|---|---|---|
| Authority & Doorway | 934 | STALE-ish ("No other JSON tool exists", "Static host reads, glob, grep... unavailable", repeated in engineering.yaml too) |
| Worked examples (3 cells) | ~1,700 | CON, see B6 |
| Terminal Recording + Two Kinds of Identifier | 1,640 + ~600 | LB |
| Error recovery + backtick note | ~1,100 | LB |
| Capsule jobs, Edit-do-not-rewrite, Bound your jobs, Where commands run | 1,192+571+845+588 | LB (each tied to a trace-found waste; keep as one "capsule facts" list) |
| Freeze (evidence slots, refusal not a blocker) | 850+1,479+562 | LB, partly HOST: the Freeze refusal text names the unmet precondition |
| Frontend check job command (60), Go check (61) | 1,509+666 | HOST: a `choir.CheckFrontend()` / `choir.CheckGo()` verb returns the job, no 1.5k shell string |
| Precommit before you build | 1,507 | HOST candidate: scored predictions; the rule kinds list is verb doc |
| Platform screens shot plan + Screenshots | 851 + 3,402 | DUP (3 places) |
| Claude Delegate (4 steps) | 1,605+1,307+1,079+927+688 | CON with ruling 7, see B8 |
| Reporting & Completion | 929 | LB |

Screenshot rules exist in three places: `SystemContext` (spec.go:327-329, "Screenshots are required..."), `capsuleBrowserRecipe` in the run-context turn (~5k), and the overlay "Screenshots" bullet (3.4k), and the recipe's own header says "authoritative" because "four assignments made six statements about the toolchain, three of them false" (run_context.go comment). Three authorities is the failure the recipe was created to cure. Freeze refusals (`screens_required`, `before_after_required`) already carry the rule at the moment it matters.

### 1.6 Common text (all desks)

core.yaml 1,099 chars: "multiagent writing, research, and execution system", user cares about progress, "The runtime owns delivery... Do not poll for inbox state manually" (LB for polling, but the host does it), and the four-role map "Conductor routes top-level intent. Texture owns canonical document versions. Research gathers... Management orchestrates... Engineering performs...". The role map is DUP (each desk's own role text says what it does) and "Conductor routes" is STALE for the paper (B5). Could be 500 chars.

`peer_coordination.yaml` trailer: 195 chars at the end of every prompt, wrong for Texture (B3), duplicates the req paragraph.

Wake turn (`coagent_update_packet.go`): a JSON envelope with `schema`, `packet_type`, `delivery_phase`, owner, computer, target run, channel, trajectory ids, the sentence "New update_coagent records arrived while this activation was running. Treat this packet as the next user turn." and "The payloads are already bound inside your cell as choir.Updates() - read them there; do not re-request them." About 700 to 900 chars (I), appended per wake (cache-safe), but: `update_coagent` is a retired tool name (STALE); the ids are already in the run-context; and a pointer-only wake forces a read cell on every wake (see L6).

Untrusted wrapper (`run_context.go:222-232`): 215-char preamble plus "[End of data]" on every data result (M: 330 chars with the cell envelope per the audit). Appended, not prefix-breaking. Shorten to a 60-char marker; saving ~40 tokens per data result, ~18 per research run: small (I).

Cell-result envelope: `{"diag","duration","error","intents","receipts","reuse","stderr","stdout"...}` about 206 chars per result and a 200-char "new session: nothing from this desk's earlier runs is defined..." sentence in the first result (M, audit 1.5). Omit empty fields: ~40 tokens per result.

### 1.7 Conductor (front door; carrier-less)

conductor.yaml 1,495 + conductor_run.yaml 1,388. Tells the model to "Prefer spawn_agent with role=texture", write "patch_texture", return `{"action":"toast"...}`. Registry is empty (`conductorRegistry := toolregistry.MustNewToolRegistry()`, tool_profiles.go:402, installed at :410), bare-URL routing is server-owned (`completePromptBarDecisionRun`, runtime.go:1537-1541), and 0 of the 500 sampled paper runs are conductor runs (M: `Counter(agent_profile)` on runs.json = texture 323, management 98, research 79). Last prompt edit 2026-09-12. This prompt names two verbs that no longer exist.

### 1.8 Research children

`researchChildPreamble` (research_children.go:65-66) is prepended to the full 38k research prompt (`researchChildSystemPrompt`, :425-437). The Texture child's preamble ends with "The world verbs and the research procedure in the prompt below are not yours; ignore them": 37.7k chars of prompt loaded to be told to ignore it (M from code). Prepending also breaks byte-sharing with the parent's own prompt (the child's first bytes differ), the opposite of the cache goal. `choir.Research` was not called in any of the 14 sampled runs (M, verb counts), so frequency is low; fix is cheap (own short prompt per child role; the web child needs the read-only verbs but not Relate/Report/StoryAnalysis).

### 1.9 Claude delegate brief

`SystemContext` (spec.go:325-343) is the capsule rules (network, screenshots, never commit) and `Prompt` (:346-370) is objective + shots + checks + owner words. The engineering desk is told to "brief it" with a spec object and "never squeeze the request into the objective". That is the pasted-brief model ruling 7 wants to move away from. Proposed shape in section 5.6.

### 1.10 Jev judgment prompts, Mail, other desks

Jev questions (intake event/topic, front importance 1-10, support, cast, inert) are 300-900 chars each, choice-shaped, with the editor's corrections as state. They are the right size; nothing to simplify except the front importance rubric text (clip(story.Dek,600)) which is out of scope. Mail has no model prompt. Package desks get the generic role line "This is the system prompt for the X agent in Choir." plus core plus their package instructions; the drill desk is 523 chars. Nothing to trim.

## 2. Stale / switch residue (retired things in model-facing text)

| text | where | state |
|---|---|---|
| spawn_agent, patch_texture, toast JSON | conductor.yaml:14-16, conductor_run.yaml:9,13 | no such tools (empty registry) |
| update_coagent | coagent_update_packet.go:228-234 | retired tool name in every update wake |
| `choir.SearchWireCorpus` | research.yaml:17, overlay:20,22,30, verb | "Wire" is a retired product name (CLAUDE.md); verb rename is the §9 TODO in autopaper-sw-design; the prompt words "world events" |
| "intake labels on/off", "event wakes" | `RLMManagementOverlayOptions{IntakeLabels, EventWakes}`, rlm_management_runtime.yaml:26-34; "Living stories" text "(it does when intake labels are on)... Otherwise answer all of them in this turn" | two computer settings change the prompt text per computer. CLAUDE.md (2026-11-03): "New behaviour ships on, with no runtime switch, ever". The paper has both on. Text for the off case is dead on production; the conditional makes management's prompt differ per computer and defeats prompt-level cache sharing between computers. |
| "tool catalog is the complete authority... No other JSON tool exists", "Do not search for non-existent tools", "a tool name outside the cell does not exist", "legacy catalog text is replaced" | engineering, research, management overlays | describes removed regimes |
| `Available tools:` line | toolregistry.go:176 | the one tool is already in the tool list |
| "texture:new", "PD3 Texture root" strings | fine, live | |

## 3. Behavioral bugs, ranked by harm

Evidence labels in brackets.

**B1 (high). Research interim packet is late, against the prompt, the nudge and ruling 1.** The prompt states the cadence four ways (1.1) and the host adds a nudge after the first capture (read_ledger.go:179). In the 12 sampled runs that sent packets (M, research-events.json), the first Read sits at cell 1-8 and the first ReportPacket at cell 4-27: gaps after the nudge fired were 7, 3, 7, 9, 3, 8, 16 cells in 7 of 12 runs (e.g. run b36d97e4 nudge at result 3, first packet at 10; 8da78c22 nudge at 5, packet at 12; 4e0afcd2 nudge at 14, packet at 27). Probable mechanism (H): a packet quote must be "copied exactly from one block", so the model must see the blocks first, which takes a full turn, and one cell per turn means read, see, then write the packet. Instructions cannot fix a data-dependency. Fix is code: let packet sources name `capture_id` plus block ordinal and have the host fill `quote`/`excerpt` from the capture (and fail only on a bad id), or add `choir.ReportCaptures("texture", summary, ids...)` that builds the packet deterministically. Then search, read and send is one cell (ruling 4) and the four cadence sentences collapse to one.

**B2 (medium-high). Management task burns cells on shapes the prompt does not give.** 13 of ~75 sampled cells error (M): `undefined method: Type/Task/To/TaskID/Objective/ID/Title` on `choir.Records()` items and `Items` on a StoriesPage (the field is Stories), plus four `CellOut` slice-out-of-bounds (`out[20000:]` on a shorter output). The overlay names `r.Cast.Captures` and `r.Captures.Items` but not the item fields (CaptureID, Title, URL), not `StoriesPage.Stories/Next`, and not `InboxRecord` fields. Also 3 runs call `choir.Help("Feed")`. Fix: one example cell with the real field names in the task overlay (~300 chars) or, better, HOST: every task starts with the same Records, ReadCapture and Stories cells (all 13 listed runs begin `Rec`, then ReadCapture x1-3, Stories); put the records and the first Stories page in the task's first user turn.

**B3 (medium). Wrong verb in every Texture prompt.** The trailer tells Texture to coordinate via `choir.ReportPacket`, which is not in its surface. No attempt appears in 22 sampled runs (M), so the harm is latent.

**B4 (medium). Final-Report refusal as a procedure.** Research's final Report is refused until the entities are recorded (overlay "What a story is about"). Mechanical, and every story research run reads StoryAnalysis in cell 1 (12 of 14 runs start with it, M). The host already asked System 1 the role and kind questions; apply the top answers by default and let the desk override.

**B5 (medium, cheap). Conductor prompt for tools that do not exist** (1.7). If a conductor model call ever happens it would be told to call spawn_agent/patch_texture with an empty registry. Verify one thing before deleting: whether any path still runs the model for the conductor (none seen in 500 runs, M).

**B6 (medium). Engineering's own example contradicts its rule.** Worked example cell 2 teaches `src, _ := choir.ReadFile(target); patched := strings.Replace(src, "oldExpr", "newExpr", 1); choir.WriteFile(target, patched)` while a later bullet says "a file you already wrote or read is changed with choir.Edit(path, old, new)... never re-emit a longer file" because "Output tokens are the slowest part of a round (about 95 a second)". Models copy the worked example. Replace the example with choir.Edit.

**B7 (medium). Engineering file-delivery contradiction.** engineering.yaml says deliverables go in `/var/lib/artifact/deliverables/`, "not in /computer/files", and "the host files them in the owner's Files at Reports/<assignment id>/<name>"; two paragraphs later "save it in the owner's Files, not only in the capsule: Reports/<name>.png ... Name the exact Files paths in your completion". Two destinations; the second names a path shape the host does not produce. One paragraph.

**B8 (medium, ruling 7). Delegate-first text asks for the opposite of "delegate more, explore yourself".** The delegate is told to be briefed with objective, paths, tests, shots, checks and owner words (pasted brief); the engineer must "read those files, then re-run the test in your own cell". Only part of that re-run is required (Freeze needs distinct own receipts for build, test, toolchain). The sentence "Verify, never trust... read those files" is optional re-verification layered on a rule that is host-enforced. Also "Screenshots: take your own screenshots" (overlay) vs "Delegate first: ... the screenshots with choir-shot.mjs ... are the delegate's" in the same prompt.

**B9 (medium, tokens). "read what you need with choir.CellOut(handle) before concluding."** (research "Go REPL Workbench", all desks) instructs reading cut output in full. Run 57942a (audit W3) spent nine paging cells on one capture set. The output cut (8.6k chars) already exists; a prompt line "search or slice in the cell and print only matches" is the fix that keeps results untrimmed (ruling 3) and cuts the paging.

**B10 (low-medium). Contradicting screen rules in management** (1.3): q5 as a judged criterion vs "never yours"; "Verify pass runs the questions itself" vs "the platform judges nothing". A model fails shots on size or double-judges.

**B11 (low). Texture V1 rule is given twice with a conditional second** (1.2).

**B12 (low). "once per run" nudge fires more than once.** readNoteLines sets `l.nudged` once per scope, but the nudge text appears in 2-3 results in 8 of 12 runs (M). Adjacent indices (14,15) suggest a session restart creates a new scope (H). Harmless but the comment is wrong.

**B13 (info). 8-source floor vs token budget.** "read at least 8 independent sources" (8d53b58b5) is an owner-wanted floor; it conflicts with the cost goal only through carried results, which ruling 3 forbids trimming. Keep; the cost answer is cache, not text.

Not a bug but checked: Texture does not wait for batches; ReadDoc/Records are not needed before acting (frame says "No cell is needed to read it"), though 14 of 22 runs start with a read cell because sources are shown wordless; management does read `choir.Records()` first in 100% of sampled task runs, by design of the pointer wake.

## 4. Proposed simplified outlines (behavior held constant)

Method: delete restatements (keep the one place that is actually authoritative), move record-triggered procedures to the record, move mechanical procedure to the host, keep every rule that guards a dated failure. Keep verbatim: the p_success/weight/settlement contract (once), the quote-exactness rule, the Write directive grammar and citing grammar (Texture), the cell-shape compile rules, "Never write that work was dispatched unless its Cast committed", the receipts rule, "first act is an Ask to the bound engineer", the Freeze-evidence slot rules.

### 4.1 Shared block (new, byte-identical on all desks, right after core)

"Choir REPL manual" (target ~1,800 chars): desk_go_eval is a stateful Go notebook; fragments, no package/main/import choir; one eval per turn; print summaries, keep records in variables; a large print is cut with a handle (slice or search in the cell, CellOut only for the part you need); a failed or panicking cell loses the session; the req contract (p_success, weight, settlement) once; "choir.Help() lists verbs, choir.Help(name) documents one". Replaces: REPL Workbench (4 copies, ~800 each), Error Recovery (3 copies ~700-1,000), cell-shape sentences, the req paragraphs in overlays (5 copies) and the verb docs (3 copies). Honest caveat (I): 1,099 core + 1,800 manual is ~700 tokens, below the minimum cacheable block of Anthropic models (I recall 1-4k tokens; not verified here), so the gain is code maintenance and fewer copies, not cross-desk cache hits. Per-agent threads already cache each desk's own prefix; Texture documents already share one prefix across documents (the system prompt is byte-stable, prompt_cache_prefix_test.go). Net: do not engineer around cross-desk sharing.

### 4.2 Research: 37.7k to about 12-13k chars (overlay and role ~6.5k, surface per the other investigator ~5k, core+manual 2.9k)

Keep: role (job, corpus first, read before you cite, quote exactness, status per claim, authority of Texture), cadence (one sentence), packet rules, 8-source floor, "a page that refuses your fetch is not missing".
Cut: Research Authority (469), Workbench (812, into manual), Cell Surface header and Observation list (153+), the Search (async) bullet (916) and "Searching is asynchronous" (661; keep 2 sentences: start several, read what landed, results never wake you), Evidence Doctrine (1,942), Closed questions / Related stories / entity bullets (4,379, the verb docs and `Help` carry them), duplicate ReportPacket schema (keep in the verb doc), "Coordination" bullet. Move: entity recording to the host (B4); first interim packet to a host verb (B1).
Savings about 18k chars of overlay (4.2k tokens) per call before surface work; 710 calls/h gives about 3.0M tokens/h (I), 6% of the paper's 50M/h.

### 4.3 Management: 49.9k to about 20k chars (overlay and role ~8k, surface 7k with intake verbs, core+manual 2.9k)

Cut or move: Screens + Shots owed + Install review (7,974) to the owed records (about 2.5k appended on arrival, once per event); Calibration and Requests bullets (2,186, covered by the manual and the Cast doc); the `network` essay (2,000) to default host behavior with the exception stated in one sentence; the rebase_notice recipe to the notice record; Authority/Workbench/Cell Surface intro (1,535); Cancellation (236); "Cast exact desk name" (283); conditional intake/event text un-conditioned and merged into "Living stories" with the labels bullet (3.6k to about 1.6k). Rewrite "Answering a Texture request" and "Delegating Engineering" (5.6k) to about 3.5k.
Savings about 24k chars including conditionals (5.6k tokens) per call; 240 calls/h = 1.3M tokens/h (I). It falls with the wedge fix (calls/h), and is larger per call than any other desk.

### 4.4 Texture: 15.5k to about 11k built (about 12.5k production)

Keep: role, frame sentence, owner-intention bullets, revision shape, Done/Wait/Reject rule. Cut: the Write/cite/embed grammar from the overlay (1,371) or from the Write verb doc (1,892), not both; "Close each turn" shortened (the refusal teaches the next-actor rule); trailer (B3); V1 rule merged into one conditional; source-description ban examples halved. Remove cross-desk leaks from the Done, Pack, Context docs. Savings about 3k chars (0.7k tokens) per call, 460 calls/h = 0.32M/h. The real Texture lever is the wake path, not text (L2).

### 4.5 Management task: 11.8k to about 8k

Cut the req paragraph (the manual has it), the "With intake labels on" clause, trim the hypothesis/forecast spec to a table; add the record field names (B2). Savings about 2.5k chars (0.6k tokens) per call, 400 calls/h = 0.23M/h; the host-delivered first cells (L5) save more.

### 4.6 Engineering: 36k to about 17k, and the delegate

Cut Authority negatives, the three-copy screenshot rules to one (Freeze refusals name the rest; the host owns `choir-shot`), the frontend and Go check strings to verbs, the worked examples fixed (B6), the delegate section to: "hand source edits to the delegate with the Cast's request and the owner's words verbatim; it explores the repo itself; you own the receipts Freeze cites". Ruling 7: the delegate's `Prompt` becomes objective plus the Cast text verbatim plus owner words; drop `paths`, `shots` list and `checks` fields from the required brief (optional constraints). MCP bridge (open question, ruling 7): the brief stays small if the delegate can ask the desk instead of being pre-briefed. A `choir-desk` MCP server inside the capsule exposing `records()`, `report(text)`, `ask(desk, q)` and `read_capture(id)` to Claude Code would let it be a desk-peer; the capsule is networkless except the model API and loopback (spec.go:327), so the server must be a local stdio process talking to the host broker the way `choir-delegate` already does (`--strict-mcp-config --mcp-config {"mcpServers":{}}` today, spec.go:306-307, so this is a deliberate opening). I did not test it.
Savings are small on the paper (engineering is rare); the value is behavior.

### 4.7 Conductor: delete both files and `ConductorRunOverlay` once the one-grep check in B5 passes (2,883 chars, zero live calls). Child roles: own short prompts (1.8).

## 5. Ranking by tokens saved x paper call frequency

All savings are on raw input tokens (I). On a caching endpoint the dollar value is about 10x smaller (audit §2); the benefit that survives is latency and attention (fewer contradicting instructions). A prompt edit changes the system hash once per desk and per document, one cold prefix each.

| rank | change | chars/call | tokens/call | calls/h | tokens/h | note |
|---|---|---|---|---|---|---|
| 1 | Research overlay dedupe (4.2) | 18,000 | 4.2k | 710 | 3.0M | plus B1 fix for behavior |
| 2 | Management overlay (4.3) | 24,000 | 5.6k | 240 | 1.3M | falls with wedge fix |
| 3 | Management-task host-delivered first cells (L5) | | ~8k carried per avoided call x 3.3 calls | 41 runs | ~1.1M | 400 calls/h / 9.8 per run = 41 runs/h |
| 4 | Texture overlay (4.4) | 3,000 | 0.7k | 460 | 0.32M | |
| 5 | Task overlay (4.5) | 2,500 | 0.6k | 400 | 0.23M | |
| 6 | Wake envelope + untrusted wrapper + empty-field envelope (1.6) | ~600 | 0.14k | ~1,800 appended turns/results | ~0.25M | I |
| 7 | Conductor, child prompts, engineering (4.6, 4.7) | | | rare | <0.1M | behavior value |

Total about 6.3M tokens/h of the paper's about 50M (12%), at about 25% fewer fixed-prompt tokens on the three high-frequency desks. The prompt trim alone is not a 10x lever; B1 plus the progressive-disclosure surface plus host-done procedure are.

### Levers (non-text)

- L2 (Texture): put the new record's capture text (bounded) in the appended wake message instead of "shown without words", removing the read-only first cell in 14 of 22 runs (M) without touching earlier bytes.
- L5 (task): first user turn carries the Records and a first Stories page.
- L6 (all desks using Updates): an update wake that is pointer-only forces one read cell. Where a desk reads `choir.Updates()` first in 100% of runs, inline the payload in the wake turn (appended).
- Host-done: entity recording (B4), interim packet (B1), frontend/Go check verbs, install/shot choreography.

## 6. How to hold behavior constant

What pins prompts today (M, grep):
- ~140 `strings.Contains` pins on prompt text: runtimeprompts tests (51 in prompts_test.go, 5 freeze_order, 5 frontend_build_job, 6 one_round, 2 partial_trap, 1 rebase, 2 request_examples, 2 research_interim, 5 texture_request), textureprompts/prompts_test.go (24), promptstore/store_test.go (4). Examples that will fail on edits: `TestRLMResearchOverlayListsSystem1WithItsContract`, `...ListsRelateStoriesWithItsContract`, `...ListsRelateWithItsContract`, `TestOtherDesksOverlaysDoNotTeachSystem1`, `TestResearchOverlaySendsTheFirstPacketAfterTheFirstSource`.
- `TestDeskPromptPrefixIsByteStableAcrossRunsAndTime` and its siblings (prompt_cache_prefix_test.go:128,164,190,245): the cache-prefix contract. Must stay green; they also guard against my shared-block moving volatile text.
- `TestVersionIsStablePerDeskAndOnlyChangesWithItsOverlays` (prompts_test.go:470): a hash version per desk.
- No golden prompt files and no behavioral eval. `api_texture_prompt_eval.go` is a model-arm sweep driver (POST /api/evals/texture-prompt pins a model-policy overlay), usable as the driver for prompt arms too.

Plan: replace each wording pin by a behavior pin before cutting: keep the pin only where the text is a contract (quote exactness, Never write dispatched, receipts). Keep the removed sentences in a new `docs/evidence` table (old text, where the rule now lives, test).

Cheap A/B on d@e (computer-59c902c5...; adversarial/drill computer, never the paper): run the old and new commit on the same inputs and compare per-run counters from `/api/runs/{id}/events` (the same events used here).
1. Task (deterministic oracle): replay N=30 capture batches that the paper's tasks disposed (their dispositions are on the tape). Compare disposition per capture (Feed/Cast/Decline plus target doc), cells per run, error cells (`undefined method`), input tokens. Target: same dispositions at least as often as old-vs-old noise (run old twice to get the noise floor), fewer cells.
2. Research: 10 Cast objectives from the 14 sampled research runs via the prompt-bar route on d@e. Compare: cells before first ReportPacket (old median ~9, M), number of sources cited, number of packets, final-Report `p_complete`, hosts count (8-source floor), cells with CellOut. Pass: first packet no later, source count not lower.
3. Texture: replay 10 documents with a research packet sequence (3 packets each); compare revisions written per packet (must be 1:1, no batching), cites accepted vs refused, `Wait` vs `Done` mix, input tokens at each wake.
4. Management: owner-request path and Shots/Install owed records on d@e (they are red paths with their own drills; run the existing drills with the moved text in the owed record).
Per-owner role-prompt override (PUT /api/prompts/{role}) is a no-build way to A/B only the role part (research.yaml, management.yaml, texture.yaml) on d@e; the overlay edits need the commit. Stage by ring as usual; no switch.

## 7. Open questions

- Is there any code path that still invokes the conductor model (B5)? One grep for `FrontDoorName()` callers that construct provider requests settles it.
- Does the production paper really run intake labels and event wakes both on? (Prompt-size 49,908 vs built 42,593 suggests yes, I.) If so, delete the conditionals.
- The 8-source floor and "report early" must be held together by B1's host fix, otherwise shortening the prompt will look like a regression.
