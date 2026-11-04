# Modular architecture: the consolidated plan (2026-11-04)

Status: design, green (this document). It is the one plan the owner reads before
any building starts. Nothing here has been built, reviewed or landed. The slices
carry their own mutation classes.

Evidence base: `origin/main` at 416b30293. Every code claim below carries a
file and line read at that commit, or is marked **hypothesis** (not verified) or
**unverified** (cited from an earlier design that Devin verified at an older
commit, to be re-grepped by the builder as the slice's first step). Line numbers
drift; builders grep by name.

Inputs consolidated here:

- the front-end and system design, Devin ACCEPT with non-blocking amendments:
  `docs/design/modular-architecture-2026-11-04.md` and
  `docs/research/plugin-architectures-2026-11-04.md` (branch `sysmod`, not on
  main);
- the app-agent desks design, Devin SEND BACK with blockers B1 to B4:
  `docs/design/app-agent-desks-2026-11-04.md` (branch `appdesk`, not on main);
- the four-seat red-team of community app agents;
- the Bayesian trust draft, copied in as
  [bayesian-trust-2026-11-04.md](bayesian-trust-2026-11-04.md);
- the custody inventory, copied in as
  [../evidence/custody-inventory-2026-11-04.md](../evidence/custody-inventory-2026-11-04.md);
- the D0 protocol-lock build (branch `d0-protocol`, three unpushed commits:
  632d98385, ad80f2330, e63e2f536), which is re-cut in Phase 1;
- the goal file `docs/definitions/choir-modular-architecture-2026-11-04.md` and its precommitment ledger `docs/definitions/choir-modular-architecture-2026-11-04.precommit.md` (on main; round 0 holds 15 records committed 2026-11-04 19:13 UTC, before this plan's evidence existed; this plan does not edit it);
- the owner rulings of 2026-11-04 (section 1).

How to read the slice ids. `MA<phase>.<n>` is new to this plan. The earlier ids
(K0, G0, P0, S1 to S7, W1, U1, C1 to C13, H1 to H4, T1, M1 to M5, R1, X1, F1,
L1 to L4 from the system design; D0 to D12 from the desks design; BT1 to BT7
from the trust draft) are named where a slice keeps their content. Review
tiers follow CLAUDE.md: tier 0 none; tier 1 Devin, one seat; tier 2 Devin plus
DeepSeek v4.1 flash. Code review never uses luna (owner). Cheap gates run first
on every slice (`check-v1-writer-purity`, `check-decode-roots`,
`check-test-hang-guards`, `selftest-guest-token`, `check-switch-registry`, the
CI mirror). Every slice lands enabled on d@e and widens by ring; none adds a
switch.

## 0. Principles that decide close calls

1. Security first. Then the expressivity of the RLM (full Go with module calls).
   Then low resource overhead. (Owner.)
2. Pre-release: no interim layers, no switches, no compatibility shims. A
   slice goes straight to the real mechanism. Where a mechanism has a
   short-duration form (a per-task lease), that form is the same mechanism, not
   an interim.
3. One trust model for stranger code and generated code (owner). Privilege comes
   from grants and tier, never from identity or from being first-party.
4. A crash closes work; a planned-restart marker may resume (CLAUDE.md).
6. Terminology (owner, 2026-11-04): "PSA" (prospective self-alignment) names the theory and the science. "Precommitment records" names the practice and the engineering: the schema, the protocol, the code. This plan uses the words that way. Owner quotes that say "PSA" are left as spoken.
7. **A new round of predictions at every phase boundary.** The ledger's rule 7 ("Re-predict at every phase boundary (roadmap phases 0 to 8), citing what was learned") is a gate of this plan: before a phase's first slice lands, FP0 commits a new round of records to the ledger (claim, probability or 80% interval, falsifier, resolving observation, settler other than the committer), before that phase's evidence exists, citing the discrepancies of the round before. A phase does not start until its round is committed; a round is a new commit, and committed records are never edited (a revision is a new record citing the old). The settlements and the Brier, log and interval scores are part of each phase's exit report.
8. Security process (R27): defence in depth first; agentic red teaming from the inside once the functionality is satisfying; then iterate. Section 12.2 places the red-team round.
5. Defence in depth: per-user VMs, tape restore, process confinement, the
   frame check, custody, leases, trust.

## 1. Decisions table

Quotes are the owner's words as recorded in the rulings file or the earlier
designs. Where only a ruling summary exists it is marked *(recorded summary)*.

| # | Date | Owner ruling | Applies to |
|---|---|---|---|
| R1 | 2026-11-04 | "The code line limits are arbitrary." Drop the 800/500 budgets; replace with structural gates: the kernel imports no package, app or desk code; no desk names or app ids in the kernel; an enumerated responsibility list. | MA2.1, MA2.2 (supersedes modular section 1 size budget, goal file artifact list) |
| R2 | 2026-11-04 | "More modular is better." Maximal modularity serves self-development and, above all, security. | whole plan |
| R3 | 2026-11-04 | "we should aim to make all desks modular. Better to make the kernel small and make every desk function the same way. No special system desks or apps." | Phases 2, 4, 5 |
| R4 | 2026-11-04 | "A locked, versioned protocol between desks (casts, delegation, the commitment ledger)." Amended by R5: the lock must not record MessageTargets. | MA1.4 |
| MA1.5 | Every sub-RLM call is a Cast carrying the parent's precommitment (section 4.4): delete the raw `choir.Research` result path, so research children are Casts to a `research_child` role whose Reports are records; give `Delegate` Casts typed questions with probabilities and make the task's final Report a scored observation; add the rule kinds `report_schema_valid` and `citations_resolve`; the kernel judge service as settler (blind System 1 question, batched per fan-out); self-score records and the self-favor gap view; management sampling of material disagreements; the weight floor from grants and the weight-zero audit sample | `internal/yaegikernel/choir.go:580-634`, `broker_protocol.go:32-34, 163-188`, `internal/agentcore/{tools_desk.go:1336,research_children.go,management_tasks.go:316+,inbox_records.go:640-650,system1_verb.go}`, `internal/types/{commitment.go,commitment_rules.go,management_task.go:136-139}` | MA1.3 | red / 2 (tape type, reducer, judge as settler) | every child call produces a Cast with at least one typed question unless marked weight zero, and a Report record; a fan-out of N children is scored by one judge call; a rule failure (a citation that does not resolve) contradicts regardless of the judge; a parent cannot settle its own record (negative test); the self-favor gap is computed on replayed d@e records; the research acceptance (Texture asks, research answers) unchanged | revert; records written stay decodable |
| R5 | 2026-11-04 | "Desks all need to message each other, but only via the PSA protocol." Delete MessageTargets allowlists (app desk default-empty, management-only). Any desk may message any desk; every message is a PSA record (request with precommitment, typed, settled by rule or by the recipient), never free text or a call. Authority never travels with a message: the recipient acts on its own grants; untrusted-origin requests carry provenance and need admission for privileged effects. | Phase 1 |
| R6 | 2026-11-04 | Roadmap: "remove raw messaging/allowlists ASAP". | Phase 1 first |
| R7 | 2026-11-04 | Progressive delivery closed (done, cb1a6cba9). | none |
| R8 | 2026-11-04 | "No spend ceiling. But do monitor costs." | MA3.9, Phase 7; supersedes desks 8.1 spend class, D10 spend ceiling, red-team quota items that cap spend |
| R9 | 2026-11-04 | Investigate wazero; "gvisor has bad perf characteristics but maybe that doesn't apply in our case"; "We definitely need desks to be able to call our go modules... that's the whole point of the system. Not looking for security theater." Later: "Skip wasm." Yaegi in wasm measured 45x slower on cells: wasm is dropped. | Phase 6; supersedes desks 4.3 tiers T1a/T1b/T2 and red-team item "wasm" |
| R10 | 2026-11-04 | "there's no difference between 'strangers code' and 'generated code'." One trust model. "We should force a code review before installing, and also track user reputation of publishers in library." | Phases 2, 5, 7 |
| R11 | 2026-11-04 | "PSA is a real security technique. We need to lean on it. It's a way to create trust from reputation... perhaps the most important aspect of it. Keeping track records of resident agents; bayesian-trust (not zero trust)." Also "We need to judge precommitments by materiality." | Phase 7, MA1.3 |
| R12 | 2026-11-04 | "Don't use luna for code review. Let's slow down with the design. I want to understand all the details and decide this myself." No design revisions, reviews or landings without the owner walking through them. | Phase 0 gate; all review tiers |
| R13 | 2026-11-04 | Expressivity of the RLM is a priority after security (full Go, calling Go modules); low resource overhead next. Defence in depth (per-user VMs, tape restore). *(recorded summary)* | MA6.5 | Code mode with per-desk import sets (R26): each desk runs model cells in yaegi with its own least-privilege import set, derived from its grants and carried in the manifest capability list (MA2.3). All desks import the precommitment-records protocol package, `math` and the other pure packages; most import nothing with effects. The point is tool calling as code: loops, fan-out and a hundred tool calls in one cell with one model round trip. The mechanism is the per-activation `NewAllowlist` (`internal/yaegikernel/allowlist.go:45-75`) with the static import check that fails closed (`eval.go:101-122`). Known limit to resolve in this slice: a cell may stage at most `MaxIntentsPerCell = 16` intents (`internal/yaegikernel/intent.go:171`), so code mode's many calls are frame calls (reads and host verbs), not staged acts; the slice measures whether frame calls need their own per-cell bound (hypothesis: they are bounded only by the cell deadline) and sets the intent quota deliberately. Third-party Go modules: see open question 9 | `internal/yaegikernel/{allowlist.go,eval.go,intent.go}`, `internal/deskmanifest`, `internal/apppackage` | MA6.4, MA2.3 | red / 2 | a cell makes a hundred read-verb calls in one model round trip and the trace shows one round trip; a cell importing a package outside its desk's set is refused at the static check; each desk's computed set is stable under the behaviour-surface hash | revert |
| R14 | 2026-11-04 | Data custody: email state only to the email app agent, texture state only to the texture desk; research and engineering may have broad read; engineering may request web grants to arbitrary sites (privileged). Note desk capabilities; limit what users can publish by default. A new user app agent desk (e.g. video editing) is very limited and uses precommitment records to ask other desks (read a specific email, make a video texture consumes). *(recorded summary)* | Phases 2, 3 |
| R15 | 2026-11-04 | Labels and provenance propagate. The model provider is chosen by the user's VM, always configurable and modular; no rule constraining models. "No cross reads!" is a hard invariant. gVisor looks good; test on our x86. Skip wasm. Run the custody inventory. *(recorded summary)* | MA3.5, MA3.6, Phase 6 |
| R16 | 2026-11-04 | Defaults 1 to 10 accepted ("Sounds good"), except: remove raw messaging ASAP; keep full publishing until the iframe sandbox ships (no known-publishers interim). "We are still pre-release so don't do excess building only to make more changes. Just go straight to real solution." More testing before 100% committed. | all; Phase 6 |
| R17 | 2026-11-04 | Roadmap Phases 0 to 8 accepted in the order of sections 3 to 11. | this document |
| R18 | 2026-11-04 | "Mail as conductor? That doesn't sound right. Conductor is its own concern. It does need help though. Conductor should be system 1 router I think." Mail absorbs only the email desk rows. The conductor becomes a System 1 (Jev, typed choice question) router, a kernel-adjacent package, not a desk with custody. | MA4.8 | **The prompt router** (the conductor), a decided design, section 9.2: each route a frozen System 1 typed choice record, settled by rule from the owner's next acts, calibrated per pinned model, with an in-context pack of similar settled routes, abstain and disambiguate below a tuned threshold, scorer audit, dogfooded | `internal/textureowner/prompt_bar.go:206-243`, `internal/content/content.go:1639`, `internal/agentcore/texture_model_api.go:31-42`, `internal/agentcore/system1_verb.go`, `internal/deskmanifest/manifest.go:300-304`, router package, settlement rule over desktop events | MA3.6, MA4.5, MA1.3 | red / 2 (the owner's only door) | section 9.2 | revert to the deterministic default route |
| R19 | 2026-11-04 | "research should maybe get a texture research grant? And/or search through guarded sql based api rather than texture files?" **Answered (Q3, same day):** use the RLM pattern properly. A research RLM fans out to sub-RLMs, one for the web and one for Texture; the Texture sub-RLM holds the `texture.search` lease, the web sub-RLM holds `world.read`, and the parent receives only their reports, as precommitment records. *(recorded summary)* | MA3.7, MA4.6, section 8.6 |
| R20 | 2026-11-04 | "Then other desks, like a video editing appagent desk can also get this scoped grant. And the grants can be durable if the desk has good trust score." Two-layer authority: ceiling in the manifest, leases issued by the custodian; trust sets lease duration and scope inside the ceiling. | MA3.4, MA7.x, sections 8 and 10 |
| R23 | 2026-11-04 | "PSA" (prospective self-alignment) names the theory and science; "precommitment records" names the practice and engineering. A real paper will be written at external launch, so the evidence it needs is built in from Phase 0. | section 0 item 6, MA0.6, MA7.9, section 18 |
| R24 | 2026-11-04 | (Q8) Build the sandboxes before community desks. Phase 6 (iframe sandbox, broker audit and fuzzing, gVisor) lands before any desk-bearing package from a non-first-party publisher can be published or installed. (Q12) There is no third-party front-end package that reads routes before the iframe sandbox. Front-end-only packages stay under R16. *(recorded summary)* | Phase 6, MA2.3, section 12.1 |
| R25 | 2026-11-04 | (Q9) The router is the conductor. "the key is to make it a system which is learning via PSA ... you can figure this one out yourself, how to make sure the system one model is getting increasingly calibrated over time ... we'll dogfood it and calibrate it ourselves ... good enough for users when we really launch." | MA4.8, section 9.2 |
| R26 | 2026-11-04 | (Q10) Each desk has its own least-privilege yaegi import set; all desks import the precommitment-records protocol, math and similar; most import nothing with effects. The point is code mode: tool calling as code, loops, fan-out, a hundred tool calls in one cell with one model round trip. Yaegi runs inside gVisor, performance permitting; engineering already runs in capsules. Third-party Go modules are allowed in principle, since generated code is no different from third-party code, but: "if you import a million lines of code but then don't review that, that's a huge vulnerability surface". | MA2.3, MA6.5, open questions 2 and 9 |
| R27 | 2026-11-04 | Security process: defense in depth first; then agentic red teaming from the inside once the functionality is satisfying; then iterate. *(recorded summary)* | section 12.2 |
| R28 | 2026-11-04 | "the sub RLM calls shouldn't be raw. They should also be wrapped in the PSA protocol. So when you're making a sub RLM call, you're making a pre-commitment as to the results that you get from it. And then that pre-commitment needs to be scored at some point ... any kind of sub-agent delegation needs to be through PSA." The owner is unsure who scores: management, the parent itself, or a self-score that is in turn scored. | MA1.5, section 4.4, open question 16 |
| R21 | 2026-11-04 | (earlier, system design) Every system app moves to an app package, written by us on main, not by self-development. The Library distributes service providers, themes and command packs. No feature switches. Publisher identity is the maild numbered address. App updates are a per-user per-app setting, manual by default; forced update and yank are platform-only. Install is the only name (adopt is deleted). | Phases 2, 5 |
| R22 | 2026-11-04 | (earlier, desks design) An app agent is an app package that ships an RLM desk, the sole writer of its app's object kinds. Texture is the precedent. | Phases 3, 4 |

## 2. Where this plan supersedes the earlier designs

| Earlier text | Fate | Reason |
|---|---|---|
| modular 1, "Size budget: 800 and 500 lines" | superseded by MA2.1/MA2.2 structural gates | R1 |
| desks 4.3 tiers T0/T1a/T1b/T2 for desk code | superseded: one runtime, yaegi with full Go modules, inside the Phase 6 sandbox. The Library tier names T0/T1/T2 for *front-end* packages (modular 1) stay | R9, R13 |
| desks 8.1 spend class, 8.5 ceiling, D10 spend ceiling | superseded: cost attribution and monitoring only | R8 |
| desks 4A.1 `address(<desk>)` grant, 8.2 item 5 MessageTargets | superseded: no address grant; any desk may send a request record to any desk | R5 |
| desks 5.2 root 7 (the address table) | removed from the lock | R5 |
| desks 9.2 migration order | superseded by section 7 order (research, Mail, management, engineering last; Texture with its app in Phase 5) | R17 |
| desks 9.4 "Mail absorbs conductor and email" (decision A) | superseded: Mail absorbs only the email rows | R18 |
| desks 8.2 containing a malicious app agent, layers 1 to 9 | kept and extended by Phases 3, 6, 7 | |
| modular 6 gate G | superseded by Phase 8 entry criteria (section 11) | |
| modular 8 decisions 1 to 12 | kept, except 7 (iframe) which is Phase 6 and decision 3 which stands as written | |
| modular 4 app migration table | kept (Phase 5), with C13 Mail moved to MA4.7 | R18 |
| trust draft "Trust never adds a capability" | revised: trust never exceeds the ceiling and governs leases within it (section 10) | R20 |

## 3. Phase 0: evidence

Goal: decide gVisor on measured numbers, then freeze this plan. No product code
changes.

Measured so far (reported by the coordinator; not re-measured by this author):
yaegi in wasm is 45x slower on cells, so wasm is dropped. gVisor systrap on x86
costs about 1.8x per warm cell, about 2x on allocation-heavy work, +20% on
compute, verb IPC 22 to 60 microseconds, and about 30 ms to start. The KVM
platform is unavailable because Node B sets nested=0, so systrap is the only
platform. Resident memory per sandbox and layering with the existing
Landlock/seccomp worker confinement are not yet measured.

What the layering test has to answer, from the code as it is. The worker is a
re-execution of the daemon binary over a socketpair (`internal/yaegikernel/desk_worker_spawn_linux.go:12`
`socketpairFDs`). The spawner, as root, switches the worker to the desk's own
uid, gives it an empty network and IPC namespace and a private tmp dir; the
worker then sets no_new_privs, Landlock and a deny-by-default seccomp filter
before serving a cell (`internal/yaegikernel/desk_cell_confinement.go:11-25`).
In the guest image the mode is "required": Landlock V5 is mandatory and the
spawner refuses to start an unconfined worker, with no opt-out
(`desk_cell_confinement.go:38-60`).

| Slice | Work | Files | Deps | Class / tier | Proof | Rollback |
|---|---|---|---|---|---|---|
| MA0.1 | Resident memory per gVisor sandbox: RSS and PSS of sentry plus gofer at 1, 5, 20 concurrent sandboxes running the desk worker binary idle and under the cell benchmark, on Node-B-class x86 | `docs/evidence/gvisor-memory-2026-11-0x.md` (new) | none | green / 0 | a table with method and raw numbers; the figure per sandbox and the figure for the five-desk case | delete the file |
| MA0.2 | Layering: run the existing desk worker under `runsc` with the fd 3 socketpair passed in. Answer: does the worker's seccomp filter load inside gVisor; does Landlock (hypothesis: gVisor does not implement it, so "required" mode would refuse to start) load; does the uid switch and netns survive; where the spawner sits (host process starting `runsc`, or the worker starting itself inside); whether the broker socket and its slot token cross the boundary unchanged | same evidence file | MA0.1 harness | green / 0 | each question answered yes/no with the command and output | delete the file |
| MA0.3 | This plan | `docs/design/modular-architecture-plan-2026-11-04.md`, the two copied docs | none | green / 0 | committed on branch `modplan`, not pushed | revert |
| MA0.4 | Devin review of this plan (one seat; the plan is a design, so it is reviewed like the change it designs) | none | MA0.1 to MA0.3 | n/a / 1 | a verdict file | n/a |
| MA0.7 | Commit round 1 of the ledger (predictions for Phase 1) before MA1.1 lands; each later phase repeats this as its first step (rule 7 of the ledger, section 0 item 7) | `docs/definitions/choir-modular-architecture-2026-11-04.precommit.md` (append a round; no earlier record changes) | the previous round's settlements | green / 0 | the commit that adds the round precedes the phase's first slice commit | n/a |
| MA0.6 | Pre-register the hypotheses before any data exists: H4 (`docs/precommitment-records.md` line 878, section 8.4: "a prior, domain-specific history of resolved records predicts future performance better than success rates, benchmarks, popularity or uncalibrated ratings") and the trust conjecture of the trust draft section 11 (per-class, materiality-weighted posteriors predict a resident agent's next material failure better than a fixed allowlist or a popularity signal, at a usable owner-tap rate), each with its test, baselines, metrics, stopping rule and the edge cases the draft names (`missing_oracle`). Committed with a date before MA1.3 lands | `docs/evidence/preregistration-2026-11-0x.md` (new) | MA0.3 | green / 1 (a design) | the file is committed and later edits are append-only with dates | revert |
| MA0.5 | Owner walkthrough and sign-off | none | MA0.4 | n/a | owner statement recorded in the goal file | n/a |

Nothing in Phases 1 to 8 starts before MA0.5 (R12). The gVisor decision
criteria are an open question (section 18 item 2).

Supersedes: nothing. Keeps: the desks design 1.3 account of confinement.

## 4. Phase 1: precommitment records are the only inter-desk channel

Goal: after this phase every sub-agent call is a Cast with a precommitment (R28) and a desk reaches another desk only by a typed request record
record, every request carries probability of success, materiality weight and
provenance, and protocol 1.0.0 is locked without any address allowlist.

### 4.1 What exists (verified at 416b30293)

Raw channels and allowlists:

- `MessageTargets` is a field of every desk row: declared at
  `internal/deskmanifest/manifest.go:136-137`, set in the system rows at
  `:214, :229, :247, :263, :279, :293, :302, :307`, empty for app desks at
  `:512`, copied at `:338`, compared at `:591`. `Addresses` (the inbox
  allowlist) is declared at `:138-141` and set in the same rows.
- `agentprofile.PolicyFor` copies `MessageTargets` into
  `AllowedMessageTargets` (`internal/agentprofile/agentprofile.go:32, 56-57`);
  `CanMessage` checks it (`:119-135`). Its callers are the reducer's note check
  (`internal/agentcore/rlm_reduce.go:2285`) and the worker-update tool
  (`internal/agentcore/tools_worker_update.go:168`); the field is also carried
  in `internal/agentcore/prompts.go:44, 61`.
- A second allowlist is hidden in routing. `recordsDesk` names the four desks
  that converse on records (`internal/agentcore/inbox_records.go:33-40`) and
  `recordsRoute` is a per-sender `switch` fixing which inbox each desk may
  address (`:49-120`): Texture to research and management, research to Texture,
  management to Texture, engineering and capture-source inboxes, engineering to
  management, a management task to stories and sources. The Texture dispatch
  fold refuses any cast not aimed at research or management
  (`internal/agentcore/texture_dispatch_fold.go:50-52`).
- Free text that is not a precommitment record:
  - **The emit channel.** `choir.Emit(toDesk, kind, body)` is a retired verb
    that returns an error (`internal/yaegikernel/choir.go:538-556`), but the
    host side is intact: `BrokerAction` `emit` (`broker_protocol.go:20`),
    `EmitPayload`/`EmitResult` (`:143-151`), a dedicated stream pair
    (`internal/yaegikernel/transport.go:41-45`), the worker sidecar sender
    (`sidecar.go:140-154`), the host handler `deskEmitSignal`
    (`internal/agentcore/tools_desk.go:1085-1119`), which writes a free-text
    `body` and a caller-chosen `msg_kind` as an envelope to any destination
    desk it is given. That handler performs no `CanMessage` check (it only
    requires a non-empty destination, `:1091-1093`). **New finding:** the
    channel is nevertheless unreachable from any desk today, because the frame
    check serves only the actions that a desk's verbs dispatch
    (`deskmanifest.FrameActions`, `manifest.go:392-401`, "Never emit, never
    message") and no entry of `verbDispatch` (`:167-189`) is `emit` or `message`.
    `admitFrame` calls `FrameAllowed` (`desk_session_worker.go:629`). So the
    emit and message actions are dead code that a future manifest edit could
    revive; deleting it removes a latent bypass.
  - **The retired message intent.** `IntentMessage` (`intent.go:22`, staged at
    `:407`) keeps reducer branches "for one release so a tray staged before the
    change still replays" (`choir.go:540-542`; reducer `rlm_reduce.go:138, 388,
    1127`). Because a crash never resumes work, a replaying tray exists only
    across a planned restart (hypothesis: no planned restart has carried a
    pre-P2c tray; the build checks the tape).
  - **Note.** `IntentNote` is "raw unscored transport" (`intent.go:44-48`,
    staged at `:521` and, with a disposition, `:543`). It is the directive
    subtype `note` (`internal/types/commitment.go:60`). Management's capture
    `Decline`/`Feed` ride on it (`intent.go:543`; management row verbs
    `manifest.go:278`).
  - **Ask/Reply/Escalate/Cast/Report** carry typed acts but their payloads are
    free-text strings (`choir.go:1431, 1444, 1453, 1462, 1532`). They are
    records on the ledger, so they are precommitment records already; what they lack is a
    required probability and weight on each request (see 4.2).
- What a precommitment record has today. A `CommitmentRecord` has a required
  `Provenance` of agent id, model id, context ref and timestamps, plus acting
  task and run for delegated sends (`commitment.go:162-175`); typed questions
  carry `Probabilities` and `Weight` (`:78-85`); casts carry hypotheses with
  probabilities (`:330-345`); the closed rule-settled precommit kinds are at
  `internal/types/commitment_rules.go:107-170`. There is no required
  probability or weight on a Cast, Ask, Report or Escalate, and no origin or
  label on provenance.

- **Sub-agent paths that are not precommitment records today (verified).**
  Of the verbs on `ChoirScope`, only two spawn agents; the rest are acts, reads
  or the judge:
  - `choir.Research(children)` (`internal/yaegikernel/choir.go:580`) sends
    `ActionResearch` (`broker_protocol.go:34`) and the host serves it with
    `runResearchChildren` (`internal/agentcore/tools_desk.go:1336`,
    `internal/agentcore/research_children.go:132`). It is **raw**: a child is a
    `research_child` desk loop whose ledger reduction is "inert (no run record,
    no channel)", it "reports" nothing, and its result is a return value,
    `ResearchChildResult{Objective, Model, Findings, Error, ...}`
    (`broker_protocol.go:177-188`). The parent states no prediction about the
    result and nothing scores it. Children share the computer-wide call spacer
    (`research_children.go:93-106`, `runtime.go:3841-3843`).
  - `choir.Delegate` (`choir.go:838`) stages `IntentDelegate`; the reducer
    plans one management task and one Cast into the task's inbox per group
    (`inbox_records.go:640-650`, `management_tasks.go:316-320`). This is
    already a Cast record (the task is a bounded sub-RLM of management,
    `internal/types/management_task.go:11`), but the Cast carries an objective
    and leased captures only (`management_tasks.go` near `:413-418`, no
    hypotheses or probabilities), and the task's final Report is minted by the
    host from its dispositions under a system provenance
    (`management_task.go:136-139`): no prediction was made, so none is scored.
  - Neighbours that are not agent spawns and stay as they are: `choir.System1`
    is a judge call, not an agent; capsule `IntentJob` (`internal/yaegikernel/jobs.go`)
    starts a process; the Claude delegate (`internal/claudedelegate/spec.go:1-13`,
    reached through `cmd/choir-delegate`) is an experiment that is deleted with
    the experiment; if it survives it is a sub-agent call and takes the rule.
  - I found no `llm_query`-style verb in the cell surface, and the reducer
    (`rlm_reduce.go`) turns staged intents into acts; it spawns no agent
    except through the Delegate branch above (hypothesis: only the verb list
    and the reducer's intent switch were read for this).

### 4.2 Design

**A message is a request record.** The act set is closed and typed: cast
(asks for work), ask (asks for an answer), report (asserts a claim and names its
resolver), reply, escalate, precommit, resolve, disagreement. Each *request*
(cast, ask, escalate, and any new request kind) carries:

- `p_success`: the sender's stated probability, in [0,1], that the request
  achieves its stated outcome; required, validated;
- `weight`: the sender's proposed materiality (precommitment-records 3.1: probability times
  impact times relevance), required for requests whose impact class is I1 or
  above; the recipient and scorers judge it independently, and the sender's
  figure is scored, not trusted;
- `settlement`: how it settles, a rule kind from the closed vocabulary
  (`commitment_rules.go:107-170`) or "recipient";
- `provenance`: stamped by the reducer, never supplied by a cell (agent, model,
  desk, package and version, run, context ref, origin class). Labels join in
  Phase 3.

Free text survives only as a bounded content field of a typed request
(`objective`, `question`, `claim`), never as a message by itself. Note is
deleted. `Decline` and `Feed`, which ride on Note today, become typed request
kinds (`decline`, `feed`) owned by management's intake grant (section 6, B1).

**Authority never travels.** Any desk may send a request to any desk. The
recipient decides on its own grants, as follows.

- The recipient orders and admits inbound requests by its own rules; trust
  (Phase 7) will weight them by the sender's record. A request from an
  untrusted-origin run carries provenance and cannot cause a privileged effect
  without admission.
- A Cast to `engineering` today opens an assignment
  (`engineeringCastIntent`, `inbox_records.go:122+`; hypothesis: line moved,
  grep the name). With any desk allowed to cast, an assignment must still be
  opened only by the holder of `admit.assignment`. The plan: a Cast to
  engineering from a desk without that grant is *forwarded* to management's
  inbox as an admission request, and management's admission ledger decides
  (open question, section 18 item 5).

**One resolver, no sender switch.** `recordsRoute` and `recordsDesk` become one
data-driven resolver: the recipient name is looked up in the desk registry; the
inbox shape comes from the recipient's declared `instance` (per owner, per
subject, per run). The non-desk inboxes `recordsRoute` serves today
(`captureSourceDesk:<subject>`, `inbox_records.go:77-83, 107-110`, and the
open-new-subject address `texture:new`, `:113`) become registered inbox
classes owned by the service that reads them, and "open a new subject" becomes
a request kind that the custodian of the subject kind handles (Texture opens
documents). This fixes Devin's B1 non-desk routing item.

### 4.3 Slices

| Slice | Work | Files (verified) | Deps | Class / tier | Proof | Rollback |
|---|---|---|---|---|---|---|
| MA1.1 | Delete the emit and message channels: `ActionEmit`, `ActionMessage`, `EmitPayload`, `EmitResult`, `StreamBrokerEmit` and its result stream, the sidecar sender, the host `deskEmitSignal`, the `emit` envelope kind, the `Emit` and `Message` stubs, `IntentMessage` and its three reducer branches. Bump `ProtocolVersion` | `internal/yaegikernel/broker_protocol.go:9, 20, 143-151, 326`; `transport.go:41-45`; `sidecar.go:140-154`; `desk_session_worker.go:55-56, 448-451`; `worker_mux.go:78`; `frame_selftest.go:31, 96, 170, 195`; `choir.go:538-556`; `intent.go:22, 407`; `internal/agentcore/tools_desk.go:515-519, 869, 1085-1119`; `rlm_reduce.go:138, 388, 1127` | none | orange / 1 (deletes unreachable code; one-act revert) | frame self-test asserts every action in the new set and none else; a grep gate for the deleted names; a test that a manifest naming `emit` is refused at validation; the existing D0 branch corpus is re-captured | revert |
| MA1.2 | One data-driven address resolver; delete `MessageTargets`, `Addresses`, `CanMessage`, `AllowedMessageTargets`, `recordsDesk`/`recordsRoute` sender switch, the Texture cast restriction. Non-desk inbox classes become registrations. Engineering casts from non-holders forward to management | `deskmanifest/manifest.go:136-141, 214-307, 338-339, 512, 591`; `internal/agentprofile/agentprofile.go:32, 56-57, 119-135`; `internal/agentcore/prompts.go:44, 61`; `inbox_records.go:33-120`; `texture_dispatch_fold.go:46-58`; `rlm_reduce.go:2285`; `tools_worker_update.go:168` | MA1.1 | red / 2 (routing, concurrency, ledger) | contract test: every desk to every desk resolves to its registered inbox; a request to an unregistered name is refused; a cast to engineering from research lands in management's inbox, not as an assignment; the existing Texture, research, management and engineering product-path acceptance unchanged on d@e | revert |
| MA1.3 | The request record: required `p_success`, `weight` and `settlement` on requests; reducer-stamped provenance with an origin class; delete Note and the `note` directive subtype; `Decline` and `Feed` as typed request kinds | `internal/yaegikernel/intent.go:44-48, 521, 543`; `choir.go:1431-1462, 1532`; `internal/types/commitment.go:43-65, 162-175, 572`; `rlm_reduce.go:237, 262, 404, 1218, 1265, 1297, 1357`; `inbox_records.go:391, 442, 595`; `tools_desk.go:1178`; `commitment_rules.go:107-170` | MA1.2 | red / 2 (tape type, reducer) | golden fixtures for every request kind; the reducer refuses a request with missing or out-of-range `p_success`; provenance cannot be set by a cell (negative test); a precommit with `weight` 0 on an I1+ class is refused | revert; tape records written stay decodable (new fields optional on read, required on write) |
| MA1.4 | Re-cut D0: lock protocol 1.0.0 without allowlists. Cherry-pick the extractor, checker, contract tests and the transition table from the `d0-protocol` branch; remove `MessageTargets`/`Addresses` from the surface (`internal/deskprotocol/surface.go:77-78, 277-278` on that branch) and the `note` and `emit` golden fixtures; re-capture the corpus after MA1.3; add the roots below | `internal/deskprotocol/**`, `cmd/deskprotocol-lock`, `.github/workflows/ci.yml` (job `desk-protocol`), `internal/store/lifecycle.go` and `engineering_assignments.go` (transition table reads, from commit e63e2f536) | MA1.3, MA1.5 | yellow tooling, orange reducer table / 1 | the checker passes against its own lock; negative tests (field removed, required field added, enum value added to a closed type) classify correctly; the transition contract test runs against current store behaviour (every transition the store performs today is in the table), not only table-versus-store symmetry; a build with a deliberately breaking change fails the job | revert |

### 4.4 Sub-RLM calls are Casts, and who scores them (R28)

**Rule.** Any call that makes a child agent do work for a desk is a Cast. That
covers research children, management's delegated tasks and any other desk's
sub-call, present or future, including the Claude delegate if it stays. There
is no raw sub-agent verb. A Cast to a child carries the parent's **precommitment
about the result**: one or more typed questions with probabilities
(`TypedQuestion{Probabilities, Weight}`, `internal/types/commitment.go:78-85`)
and, where the parent has them, hypotheses (`CastHypothesis{Statement,
Probability, Kind, Criterion}`, `commitment.go:330-345`; precommitment-records
4.4), for example: "will the child find a primary source for X (yes/no, p=0.7)";
"how many independent sources (0, 1 or 2, 3 or more)". The child's **Report** is
the observation (precommitment-records 4.2); it names the hypotheses it bears
on. The child is a desk or role of the registry, so the existing address
resolver (MA1.2) and the request fields of MA1.3 apply unchanged: `p_success`,
`weight`, `settlement` and reducer-stamped provenance. The reducer links the
child's records to the parent's by the existing nested-commitment edges
(`ParentID`/`ChildIDs`, `commitment.go:257`). The child's lease and grants are
the role's (section 6, B1; section 8.6): the Cast cannot widen them.

**Scoring** follows precommitment-records 5.1 and 5.2: the committer never
settles its own record; scorers are several and each is scored. Layers, in
order:

1. **Rules first, where the result is checkable.** Schema well-formedness of
   the Report; every cited source resolves to a capture or evidence record;
   tests pass for engineering-style children. The rule vocabulary is the
   closed `PrecommitRuleKind` set (`internal/types/commitment_rules.go:107-170`,
   engineering-oriented today); research-style checks (`report_schema_valid`,
   `citations_resolve`) are added to it, a minor bump of the lock (MA1.5).
   A rule settlement is final for the facts it covers.
2. **A blind System 1 typed question as the default independent scorer.** The
   judge (Jev, `system1_verb.go:20-40`) is asked, blind to who asserted and to
   the parent's probabilities (the Jev row of precommitment-records 5.2): did the
   report answer the question, and is each prediction confirmed, qualified or
   contradicted. The settler identity is the kernel judge service, not a desk,
   so no desk is a bottleneck. Calibrate per question type and pinned model
   (5.3).
3. **Downstream settlement.** When the parent's own output that relied on the
   child is later confirmed or contradicted (a Texture revision, a Report to
   management, an owner approval), that outcome flows back along the
   `ParentID`/`ChildIDs` edges and the reliance recorded in the report refs
   (report packets already carry source refs, `yaegikernel/report_packet.go`),
   and re-scores the child's prediction as downstream evidence. Credit is split
   by recorded reliance; where reliance is not recorded the flow-back is skipped
   rather than guessed (hypothesis: reliance is recorded for Texture's evidence
   sources, `textureowner/texture_evidence_sources.go:147-157`, unverified).
4. **The parent self-scores as a learning signal.** The parent writes its own
   score of each child result; it never settles. The **self-favor gap**,
   the self-score against the independent score, is computed and tracked per
   agent and model: that is the self-score being scored (5.2). A self-score
   never gates.
5. **Management is not the default scorer**, because of cost and bottleneck. It
   takes the **material disagreements** between the self-score and the
   independent score, sampled and weighted by materiality, as the third
   opinion (5.2: "Material disagreements go to a third opinion").

**Cost.** One System 1 call per child is the upper bound. Batching helps the
call count, not the tokens: the judge request already carries many questions
(`system1_verb.go:516-546`, `intake_labels.go` `callIntakeJudge`), so one fan-out
of N children is scored in one call with N questions; input tokens still scale
with the bounded report text. Judge price is about $0.042 per million input
tokens at the 2026-10-11 measurement, and its rate limit is about 60 calls a
minute per computer (precommitment-records 5.3, to be rechecked), shared with
the intake labeller and the support judge; children themselves queue on the
40-per-minute sub-RLM spacer (`research_children.go:31-37`). There is no spend
ceiling (R8); the cost is attributed per desk and watched. Scoring runs after
the report is delivered and is not on the critical path of the cell.

**Trivial sub-calls.** A sub-call whose materiality is zero (relevance zero or
impact class I0: a lookup, a format, a computation) skips independent scoring,
and **the record is kept** with its prediction, report and weight zero, so the
track record exists and can be audited. A kept record earns no trust evidence
(trust draft 3.2: I0 earns nothing). Two guards against a parent declaring
weight zero to dodge scoring: the kernel sets a floor on `weight` from the
child's grants (a child that holds a lease or `world.read`, or whose report is
later cited, is at least I1), and a small random fraction of weight-zero
records is scored anyway as the scorer audit (the fraction is a starting
parameter committed in the Phase 1 ledger round).

**Who scores is not fully settled.** The owner is unsure whether management,
the parent or a scored self-score. This plan implements the layering above
(rules, then blind judge, then downstream outcome, with the scored self-score
and management on disagreements) because it needs no desk to be the scorer; the
remaining choice is open question 16.

Roots added to the lock (Devin's D0 amendments): `IntentJob` and `JobOp*`
(`internal/yaegikernel/jobs.go`), `PendingUpdate` and `types.SourcePacket`
(desk-to-desk delivery shapes), the request record of MA1.3, the module
surface per desk after MA2.5, and the broker wire minus the deleted emit
actions. The lock does not record who may message whom: that is now a registry
fact and a recipient decision. The checker's root-count floor (an empty surface
must not match) follows the SDK precedent `MIN_NAMES`
(`frontend/scripts/sdk-surface.mjs:36`, not `:33`).

Supersedes: desks 5 (D0 as written), desks 8.2 item 5. Keeps: desks 5.3 to 5.6
(artifact, CI gate, contract tests, skew rules). Folds in Devin's D0
amendments and the transition-table extraction risk.

## 5. Phase 2: kernels and manifest

Goal: two small kernels with structural gates; manifest v2 where desks are
members, capabilities are declared and typed, publishing levels exist and a
behaviour-surface hash is computed; adopt is deleted.

### 5.1 Design

**Structural gates replace line limits (R1).** For each kernel:

1. *Imports.* The Go kernel (`internal/kernel`) imports only the standard
   library and itself; the front-end kernel (`frontend/src/lib/kernel/`)
   imports only itself. A gate script lists the import graph (`go list -deps`;
   a small module-graph walk for the front end) and fails otherwise.
2. *Names.* The kernel source names no desk and no app id. The gate builds the
   name list from the registries at check time (`deskmanifest.Names()`, the
   app registry, the package manifests) plus the legacy literals, and fails on
   any occurrence in kernel files.
3. *Responsibility list.* A checked-in file lists the kernel's exported
   symbols per responsibility (scope tree, effects, service resolution,
   pipelines, snapshot, supervisor for Go). A test fails when the exported
   surface and the list differ, so growth is a deliberate edit that review
   sees.

**Manifest v2** (`choir.package/v2`, modular 2.1 kept) with these changes:

- `contributes.desks` is decoded. It replaces the opaque `Desk json.RawMessage`
  (`internal/apppackage/manifest.go:64`) and the refusal "desk must be null
  before P4" (`:196-198`).
- A **declared capability list**. `Grants []any` is refused non-empty today
  (`:202-204`). It becomes typed: a list of capability *requests* with a
  closed name per entry from the grant catalogue (section 6.1). This is the
  **ceiling** of section 10: what the package may ever request. Namespacing by
  enforcement domain (`ui.*` for front-end bridge checks, `desk.*` for frame
  checks) closes the red-team overlap between the front-end and desk grant
  vocabularies.
- **Publishing levels.** Three, set by the grants a package declares:
  - *default*: own object kinds, the act set, `captures.read`, requests to
    other desks. Anyone may publish; install shows the declared list.
  - *extended*: adds ceilings the owner must approve per grant at install
    (for example `texture.search`, `mail.read(ref)`, `net.egress(host)`,
    `read.kind(k)`); a platform review record is required for the listing
    to appear, and the Library shows the sentences of section 9 of the trust
    draft.
  - *privileged*: the first-party-only grants (`capsule.mutate`,
    `selfdev.*`, `platform.update`, `product.api`, `commit.<kind>`,
    `delegate.task`, `admit.assignment`, `world.read`, and the front-end
    `ui.shell`). Refused for any origin but first-party, at validation and
    again at the daemon's frame check.
  A package with a desk is never classed below *default* with review: the
  red-team result that a community desk is an autonomous agent even without
  code ("abolish T0 for desks") is adopted; a desk's prompt is behaviour.
  **Desk-bearing packages from a non-first-party publisher can be neither
  published nor installed until Phase 6 has landed (R24).** The validator
  refuses `contributes.desks` from a non-first-party origin unless the running
  platform reports the isolation it requires (iframe sandbox for the UI, the
  hardened and fuzzed broker, and the worker sandbox Phase 0 selects). That is
  a correctness refusal tied to a capability the platform either has or lacks,
  not a switch. Front-end-only packages: see section 12.1.
- **Per-desk import set.** The capability list also carries the desk's yaegi
  import set (R26), derived from its grants by the host and not authored: every
  desk gets the precommitment-records protocol package (`choir`), `math`,
  `math/big`, `sort`, `strings`, `strconv`, `unicode`, `encoding/json` and the
  other pure packages of the present default set
  (`internal/yaegikernel/allowlist.go:11-28`); a grant adds only the
  packages it implies; nothing with effects is in the base set (`os`, `os/exec`,
  `net`, `net/http` stay banned for cells, `allowlist.go:32-43`; engineering's
  effects run in capsules through the broker). The set is part of the
  behaviour-surface hash and is shown in the Library. The mechanism exists:
  `NewAllowlist(packages...)` builds a per-activation list and the session
  worker is given `DefaultSafeStdlibPackagesList()` plus `choir`
  (`allowlist.go:45-75`); the plan makes the list per desk and derived.
- **Behaviour-surface hash.** The host computes, at publish and install, a
  digest over the prompt file, desk `code`, schedules, owned kinds and
  schemas, migrations, tool artifacts and egress methods, stored in the
  release record (`internal/pkgrelease/record.go`). Any change of the hash is
  an update that needs the owner, even when grants are unchanged (red-team item
  2). The trust draft's change discount `d` (section 4.2 there) is computed
  from the same hash.
- **Source-only releases.** A published release binds source digest,
  toolchain and artifact digest; the platform builds. No opaque binaries from
  strangers (red-team item 7).
- The manifest `kinds` rename and the desk-name grammar changes of Phase 4
  (B3) land here with the validator.

**Delete adopt.** Install is the only name. Scope and file list are those of
forced modularity M1 (`docs/design/forced-modularity-build-2026-11-04.md`
lines 544 to 562): proxy route `adoptions` (`internal/proxy/self_development.go:53, 77, 94, 459`),
`handleReleaseAdoptions` (`internal/agentcore/api_release.go:69-132`),
`api_self_development.go:264-268`, `package_install.go`, `store.ReleaseAdoption`
(`internal/store/release_adoption.go`), the object kind `choir.release_adoption`
(`internal/objectgraph/registry.go:128`), the Library front end. The vmctl and
vmmanager uses of "adopt a surviving VM" are a different concept and stay.

### 5.2 Slices

| Slice | Work | Files | Deps | Class / tier | Proof | Rollback |
|---|---|---|---|---|---|---|
| MA2.1 | Go kernel: scope, effects (LIFO, awaited), provider and inject, supervisor with restart intensity, snapshot, `GET /api/plugins/state`; structural gates (imports, names, responsibility list) in the repo gates and CI. Registers no existing service. (G0, with R1 replacing the size limit) | `internal/kernel/**` (new), `internal/apihandler/routes.go`, `scripts/check-kernel-*`, `.github/workflows/ci.yml` | MA0.5 | orange / 1 | Go tests: scope cancel, effect order, supervisor intensity (two crashes in a window close with the cause), boot-fold rebuild with no occurrence replayed; the three gates, with a negative fixture each; `GET /api/plugins/state` on d@e with no SSH | revert |
| MA2.2 | Front-end kernel: same five responsibilities; gates as above; K0 leak differential. The d@e renderer-swap drill moves to S4 because it needs the S4 slot (Devin amendment 4) | `frontend/src/lib/kernel/**`, `frontend/scripts/check-kernel.mjs`, `frontend/tests/kernel.spec.js` | MA0.5 | orange / 1 | contract tests for scope disposal order, swap ordering, ranking and failover; the leak differential (install, activate, exercise, uninstall; `snapshot()` equals baseline) | revert |
| MA2.3 | Manifest v2 and validators: `contributes.desks` decoded, typed capability list with levels, namespaced grants, behaviour-surface hash, source-only release record, kind inference. Rewrite every in-tree manifest; delete the v1 reader in the same slice (no shim; the exposure is published v1 releases, section 18 item 12). Includes P0 | `internal/apppackage/{manifest,schema}.go`, testdata, `frontend/scripts/{check-packages,gen-package-kinds}.mjs`, every `frontend/src/packages/*/choir-package.json`, `internal/pkgrelease/{listing,record}.go` | none | red / 2 (install path) | Go tests for strict unknown-key refusal, path containment, publisher-declared refusal, kind inference, each privileged grant refused from a non-first-party origin, hash stability and change on each covered input; `check-packages` on all in-tree packages; `runtime-install.spec.js` unchanged | revert |
| MA2.4 | Delete adopt (M1). Brings the kind rename `release_adoption` to `install` with its tape conversion as designed in forced modularity | files above plus `cmd/choir/package.go`, `frontend/src/packages/library-next/**`, `scripts` | none (disjoint from kernels) | orange / 1 | the forced-modularity M1 proof; a grep gate for `adopt` outside the vmctl exceptions | revert |
| MA2.5 | Grant-driven desk registry (D1): desk becomes a data type loaded through `Register`; the closed lists become lookups (F1 to F6, F11, F12 of the desks design); `scripts/check-desk-names` gate with an allowlist that must reach zero; fix `deskModuleSets` excluding `KindApp` (`internal/yaegikernel/choir.go:328-331`); fix `isTextureAgentID`'s duplicated check (`internal/agentcore/tool_profiles.go:170-173`) and the `management-task`/`management_task` dual spelling (`internal/types/management_task.go:20`) | `internal/deskmanifest/manifest.go`, `agentprofile/agentprofile.go:86-115`, `modelpolicy/model_policy.go:519-531, 639-662`, `promptstore/store.go`, `runtimeprompts/prompts.go:57-123`, `agentcore/{tool_profiles,runtime,tools_desk,inbox_records}.go`, `yaegikernel/choir.go`, `capsule/roles.go` (all unverified beyond the spot checks above; re-grep) | MA1.2, MA2.1, MA2.3 | red / 2 | the parity tests pin each row unchanged; a test asserts every built-in desk's computed verb set equals today's literal byte for byte; all four desks run an ordinary day on d@e with identical routing; the names gate passes with the allowlist at its starting size | revert |

Parallel group: MA2.1, MA2.2, MA2.3, MA2.4 share no files. MA2.5 follows them
and MA1.2.

Supersedes: modular 1 (size budget), P0, G0, K0 wording; desks D1 wording.
Keeps: modular 2 (manifest shape), forced modularity M1.

Folded Devin amendments on the system design: add the arrow `check-packages.mjs`
P0 (MA2.3), then S2, S3, S5, S6, S7, U1; add the missing items to the slice
tables of Phase 5 (list in 8.x there).

## 6. Fixes to Devin's desk-design blockers B1 to B4

These are Phase 4 slices (MA4.1 to MA4.4) and are listed here because they fix
text the owner is reading. All four land before any desk moves.

**B1 — the grant catalogue was not complete.** The catalogue in the desks
design 4A.1 lacked these powers. Each gets a grant (names provisional):

| Power today | Where | Grant |
|---|---|---|
| `CancelAct`, `CancelAssignment` | desk verb rows `deskmanifest/manifest.go:167-189, 203-310`; `intent.go` `IntentCancel`, `IntentCancelAssignment` | `act.cancel`; `cancel.assignment` (privileged) |
| Intake verbs `Stories`, `Feed`, `Decline` | management rows; `internal/agentcore/intake_view.go:67` | `intake.read`, `intake.dispose` (privileged) |
| Engineering rule-settled precommit | `internal/agentcore/rlm_reduce.go:132-135`; commit path `:1118-1119` | `precommit.rule(<kinds>)` over the closed kinds of `commitment_rules.go:107-170` |
| Task model floor | `ManagementTaskModelAllowed`, `internal/modelpolicy/model_policy.go:445-450` | `model.floor(<tier>)` |
| Task run deadline | `managementTaskDeadline`, `internal/agentcore/runtime.go:732` (run-level, not per cell) | `run.deadline(<duration>)` beside `cell_deadline` |
| A role holds `Done` that its parent lacks | role row `manifest.go:278` against `appDeskVerbs` `:471-475` | the role-narrow rule is corrected: a role may only narrow, except the lifecycle module (`done`) which a role marked `ends_run` receives |
| Non-desk routing, open-new-subject | `inbox_records.go:77-83, 107-110, 113` | registered inbox classes and a custodian-handled `open_subject` request (MA1.2) |

The user test stays: every power a desk has today appears as a grant, and no
privilege keys on a desk name. The names gate (MA2.5) enforces it.

**B2 — "never cuts a run" was violated.** The frame check resolves the desk row
by name against the live registry (`deskmanifest.FrameAllowed`, `manifest.go:425-430`,
called from `admitFrame`, `internal/yaegikernel/desk_session_worker.go:605-631`).
After an update that narrows a verb set, an in-flight old-epoch worker's legal
frame would be refused. Rule: the check is `FrameAllowedAt(name, deskEpoch,
action)` and resolves the row *retained for the run's desk epoch*; rows are
retained until the last activation on that epoch ends (refcount of live
workers). `BrokerRequest.Epoch` exists (`broker_protocol.go:59`) and
`admitFrame` compares it with the worker's session epoch (`desk_session_worker.go:622`);
that value counts worker generations, so a separate `DeskEpoch` (the install
event sequence) is added (hypothesis: the existing epoch is not the install
sequence; the builder confirms). Desk sessions are reused across runs under
`agent:<id>` (`internal/agentcore/tools_desk.go:206-213`), so `DeskEpoch` joins
the session key; a post-update activation then starts a new worker.

**B3 — candidate shadow kinds could not register.** The kind grammar
`^[a-z][a-z0-9_]{0,47}$` (`internal/objectgraph/package_kinds.go:60`) and
`ValidPackageKindName` (`:122-126`) refuse a dot inside the kind part, so
`choir.app.<pkg>.candidate.<kind>` is refused. Fix by changing the package
segment, not the grammar: the candidate is the package named `<pkg>-candidate`,
its desk `app:<pkg>-candidate` (valid under the desk-name grammar
`^app:[a-z][a-z0-9-]{0,31}$`, `manifest.go:465`) and its shadow kinds
`choir.app.<pkg>-candidate.<kind>`. The suffix `-candidate` is reserved and
refused for any installed package name, and the package name is limited so the
desk name fits 32 characters. Hypothesis: `ValidPackageKindName` accepts a dash
in the package segment; the builder confirms before the slice starts.

**B4 — required-set re-seed on restore was unspecified.** Rule, written for
the boot path: the *desired state is the fold of the tape's install events
union the required set of the running base image*. The required set is image
configuration (it ships with the base build), not a tape fact. At every boot
the reconcile compares and, for a required package absent from the fold (a
restore to a head before it was installed) or older than the image's version,
appends a new install event after the restored head and then proceeds. That is
an install, not a resumer of work: it does not touch open obligations, which
the boot closes as "interrupted by a restart" in the same pass. A required
package cannot be disabled or uninstalled. The set is: the management package,
the research package, the engineering package, the Texture package and the
prompt router (MA4.8). The Mail package is not required. Red, boot path,
tier 2.

## 7. Phase 3: custody and labels

Goal: sole reader and writer per kind, enforced at the cell frame, the store
and the service layer; leases for cross-desk reads; provenance labels that
propagate; the disclosure path; app UIs reading only their own desk's kinds.

### 7.1 What exists (verified, from the inventory and spot-checked)

- The guest has one SQLite file; every Go service holds a `*store.Store` with
  full read access (inventory 0.1: `internal/store/schema.go`,
  `internal/objectgraph/sql_store.go:230, 257, 288`).
- Cells have no store handle. A cell reads only what the frame serves:
  preloaded bindings, host-served actions from `verbDispatch`, and local
  `ReadFile`/`ListDir` bounded to the worker root
  (`internal/agentcore/tools_desk.go:1022-1027`, inventory 0.2). This is the
  one choke point for cell reads; it is where leases are enforced for cells.
- Write custody exists as a registry field `Writer`
  (`internal/objectgraph/registry.go:36-52`); the gate defaults to **observe**:
  an unknown or empty `CHOIR_OG_WRITER_GATE` reads as observe, a wrong writer
  is counted and allowed (`internal/objectgraph/writer.go:134-148`; mode
  setter `:156`; `writerAdmits` `:204`; a `WriterRuntime` kind admits every
  declared writer, `:206`). The writer is a label the caller puts on the
  context (`WithWriter`, `:97`); it is a claim, not a credential. Production
  declarations: `store/texture_turn.go:323` (Texture), `store/graph_home.go:90`
  (research), `store/file_object.go:388` (runtime), `store/app_package_objects.go:66`
  (owner).
- 52 registry lines name `WriterRuntime` (`registry.go`, count at 416b30293).
  Only Texture kinds (`:76-77, 97-99, 149`) and research evidence (`:100`) have
  a desk writer. `deskmanifest.Desk.OwnedKinds` is declared and consumed by
  nothing (`manifest.go:147-149`, inventory 0.3).
- There is no read gate anywhere.
- Mail never touches the guest store; maild is a host process with per-owner
  mailbox databases (inventory 1.1, 2.1). The guest Maildir route
  `POST /api/mail/inbound` has no caller found, and the Files route lists
  `mail/` without a carve-out (`internal/autoputer/files.go:607-608`).

### 7.2 Design

**What can and cannot be enforced, stated plainly.** Hostile code never runs
in the daemon process: community desk code runs in a worker whose only route to
data is a frame the daemon serves. So the *hard* boundary for untrusted code
is the frame check plus leases. First-party Go services run in the daemon and
are trusted; for them the store reader/writer gate and a static handle gate
catch bugs and drift, they do not stop an attacker who already runs in the
daemon. The plan says so and builds both layers.

Layers:

1. **Frame layer (cells).** A frame that reads a kind not the desk's own is
   served only against a live lease (below). The frame handler resolves the kind
   from the request, not from the cell's claim.
2. **Store layer.** `CheckReadAs` beside `CheckWriteAs`: a reader label on the
   context, a per-kind `Readers` rule in the registry (custodian, plus
   named kernel readers, plus lease holders), enforced at the object-graph list
   and get paths. The writer gate moves from observe to **enforce**, after the
   `WriterGateCounts` counters read zero across a d@e drill; the environment
   variable stays only as a test seam (`SetWriterGateMode`).
3. **Service layer.** Go services stop holding the whole `*store.Store`. A
   scoped handle type `store.As(custodian)` exposes only the methods for the
   kinds the holder is custodian of, plus the commons views. A static gate
   (`check-store-handles`) fails when a package outside the allowlist imports
   the unscoped constructor. Conversion is by service, in the order of the
   cross-read table (8.4).
4. **Route layer (app UIs).** HTTP routes carry a kind-ownership table; a route
   serving a kind is callable by its owning app. Until the iframe sandbox
   (Phase 6) this is keyed on an app identity the SDK bridge supplies and is
   *not* unforgeable (same-origin app code can lie); Phase 6 makes it real.
   First-party routes are already one-app-per-kind by construction. The Texture
   UI's reads of lifecycle and engineering evidence kinds
   (`frontend/src/lib/texture.js:143, 268, 282`, `lifecycle.js:17, 88`,
   `texture-embeds.ts:203`) become Texture-custodian endpoints that return
   projections.

**Custodians.** Section 8 assigns one custodian per kind. The owner's question
whether the custodian is a separate desk: recommendation, no. The custodian is
the owning app desk; the kernel owns the commons (ledger, inboxes, runs); there
is no custodian desk and management gets a supervisory read of assignments and
ledger health only. The owner's System 1 question: System 1 is the existing
judge (Jev, `CandidateJudge`) generalised to a kernel service (MA3.6).

**Leases (R20).** Authority has two layers.

- **Ceiling (manifest, install time).** The manifest's capability list
  (MA2.3) names what the package may ever *request*, for example
  `texture.search`, `mail.read(ref)`, `read.kind(video_asset)`. Install review
  covers the ceiling and the Library shows it. A desk can obtain nothing the
  ceiling does not name.
- **Leases (custodian-issued, runtime).** A desk asks the custodian through a
  request record (a typed `lease_request` kind of MA1.3's record): the scope
  (kinds, refs, or a query surface), a duration, and a purpose, stated in the
  request's precommitment with its `p_success` and weight. The custodian's
  service decides by its rules and issues a **lease record on the tape**:
  `{lease_id, holder desk and epoch, custodian, scope, labels it admits,
  not_before, expires, purpose, request ref, precommit ref}`. A lease is
  - the generalisation of the delegation-scoped read (E5, inventory 6.3): a
    per-assignment capsule snapshot kind list is a lease;
  - visible to the owner (an owner view over lease records, rendered as
    sentences) and **revocable by the owner**;
  - revoked immediately by the trust circuit breaker (Phase 7) and on
    uninstall, yank and desk retirement;
  - enforced at the frame layer and the store reader gate: a read presents
    the lease id (from the run's record, never from the cell), the gate checks
    scope, expiry and revocation, and every use is logged as a frame record.
- **Trust sets duration and scope inside the ceiling.** A desk with a strong
  record in that action class gets a durable lease that renews without
  asking; a thin or new desk gets a per-task lease (lifetime = the run or
  assignment it was asked for); a fidelity event revokes at once. **Until
  Phase 7 lands, every lease is per-task.** That is not an interim layer:
  per-task is the same lease mechanism with the shortest duration, and Phase 7
  only sets the policy function that picks duration and scope.

The request to the custodian is itself a precommitment record, so it is scored (did the
lease's purpose match the reads that followed?) and the custodian's grant
decision is evidence for the requester's track record (trust draft E5, E6).

**Labels (R15).** Every record, object body, capture-derived text and model
context segment carries a label set. The lattice: `owner_private:<kind>`,
`world_untrusted`, `platform`, `publisher:<id>`, and for derived data the union
of the inputs. A run carries the union of the labels of everything it read (a
per-run taint accumulator in the run record); the reducer stamps every act and
object write with the run's labels, never the cell. The context builder frames
labelled-untrusted segments as data (red-team item 4). Labels join the record's
provenance as an optional field, a minor protocol bump (1.1.0).

**Disclosure path.** When a run holding label L tries a sink that L restricts
(publication, mail send, egress to a destination, a message to a desk that is not
a custodian admitted for L, a Library release), the decision runs in order:

1. *Kernel rules*: a deterministic table of label by sink to allow, deny or
   ask. Most decisions end here.
2. *System 1 typed question*: a closed choice question to the judge ("is this
   disclosure within what the owner's request implies: yes, no, unsure"),
   returning the full distribution, recorded as a precommit. The desk or the
   rule decides from the distribution; System 1 never decides alone
   (`internal/agentcore/system1_verb.go:20-40` states this contract today).
3. *The custodian's model* (the custodian desk's own cell decides) or *an
   owner tap*.

The **model provider is chosen by the user's VM policy**
(`internal/modelpolicy/model_policy.go`, per-role sections `:639-663`), logged
per call, and never constrained by a rule (R15). A model call is therefore not
a disclosure sink that the rules gate; it is logged with the resolved provider
and the run's labels. This contradicts the trust draft's I3 listing "a
provider call carrying owner-private data" (draft 3.2); the plan follows the
ruling and the draft is amended in MA7.1 (open question, section 18 item 7).
System 1's own call caps (`CHOIR_SYSTEM1_MAX_CALLS_PER_RUN`/`_PER_DAY`,
`system1_verb.go:38-40, 56`) make it "off until configured": a switch and a
ceiling that MA3.6 removes.

### 7.3 Slices

| Slice | Work | Files | Deps | Class / tier | Proof | Rollback |
|---|---|---|---|---|---|---|
| MA3.1 | Assign a custodian to every `WriterRuntime` kind (section 8.2); declare it in the registry; `OwnedKinds` becomes the source and gets a consumer; kernel-owned `system:kernel` writer for the commons | `internal/objectgraph/registry.go` (52 lines), `deskmanifest/manifest.go:147-149`, `internal/store/*` write sites | MA2.5 | red / 2 | a registry test: no kind has `WriterRuntime`; every kind has exactly one custodian; a drill writing each kind from its custodian succeeds and from another desk is counted | revert |
| MA3.2 | Writer gate to enforce: declare the writer at the reducer from the run record (not the cell) for package kinds from day one; built-in kinds after the d@e counters read zero; delete the env default | `internal/objectgraph/writer.go:134-156, 204-216`; `store/{texture_turn.go:323,graph_home.go:90,file_object.go:388,app_package_objects.go:66}` | MA3.1 | red / 2 (tape, store) | a wrong-writer write is refused; counters zero over a recorded d@e day before the default flips | revert |
| MA3.3 | Reader gate and scoped store handles: `CheckReadAs`, per-kind reader rules, `store.As(custodian)`, `check-store-handles` gate; convert services in the order of 8.4 | `internal/objectgraph/{writer.go,sql_store.go}`, `internal/store/**`, `internal/agentcore/**`, `internal/textureowner/**` | MA3.1 | red / 2 | per-kind negative tests (a non-custodian read is refused); the gate fails on an unscoped import; the desks run an ordinary day | revert |
| MA3.4 | Leases: the `lease_request` kind, the custodian-issued lease record on the tape, owner view and revoke, frame-layer and store-layer enforcement, revocation on uninstall, yank and retirement; every lease per-task until Phase 7. Converts the capsule store copy to a lease | new `internal/lease/**`; `internal/capsule/computer_state.go:13-16, 42-52`, `computer_state_linux.go:135-177`; `internal/autoputer/capsule_executor_linux.go:144-156`; frame handlers in `tools_desk.go` | MA3.3, MA1.3 | red / 2 | a desk without a lease is refused; a lease expires at its run's end; the owner revokes and the next read fails; a lease request is a scored precommitment record; a hostile fixture cannot read a kind outside the ceiling even with a lease for another | revert |
| MA3.5 | Provenance labels that propagate: label set on records and objects, per-run accumulator, reducer stamping, context framing | `internal/types/commitment.go:162-175`, `internal/agentcore/{rlm_reduce.go,run_context.go}`, run record | MA1.3 | red / 2 | a run that reads an owner-private capture carries the label into every act it commits; a derived summary carries the union; a cell cannot clear or set a label | revert |
| MA3.6 | The disclosure path: kernel rules table, System 1 as a kernel service (generalised from research-only; caps and switch removed), custodian-model and owner-tap steps | `internal/agentcore/system1_verb.go`, `internal/yaegikernel/system1.go` (unverified), kernel service, owner act surface | MA3.5, MA2.1 | red / 2 (authority path) | a labelled run attempting each sink gets the table's decision; an `ask` reaches System 1 and the distribution is recorded; the owner tap is a tape fact; model calls log provider and labels | revert |
| MA3.7 | Custody resolutions (section 8.4): the `texture.search` surface, Texture story index for management, self-scoped `RunMemoryEntry`, `Mail` carve-out from the Files route, ProductAPI narrowing | `internal/researchtools/researchtools.go:71`; `store/texture.go:125-160`; `agentcore/{system1_verb.go:461-475,tools_evidence.go:57,intake_view.go:67,tools_product_api.go:196-222}`; `autoputer/files.go:607-608` | MA3.3, MA3.4 | red / 2 | each former cross read has a test: before, it worked by raw store access; after, it works through its fate or is refused | revert |
| MA3.8 | Object verbs and the reducer-set writer for package desks (`Put`, `Get`, `List`, `Retract`; `IntentObjectPut`), schema versions per kind | `internal/yaegikernel/{intent.go,choir.go}`, `deskmanifest`, `rlm_reduce.go`, `internal/apppackage/register.go`, `objectgraph/package_kinds.go` | MA3.2 | red / 2 | the desks design D2 proofs: a cell cannot supply a writer; wrong kind, oversize body, schema violation and another desk's kind are refused; protocol minor bump | revert |
| MA3.9 | Per-desk write and storage quotas at the reducer, wake and schedule rate bounds, and cost attribution and monitoring per desk (no ceiling, R8) | `rlm_reduce.go`, `internal/llmcost`, gateway attribution | MA3.8 | orange / 1 | a desk past its object quota is refused; the owner view groups spend by desk and package | revert |
| MA3.10 | App UIs read only their own desk's kinds: route-to-kind table, app identity check (advisory until Phase 6), Texture projections endpoints | `internal/apihandler/routes.go`, `internal/agentcore/api_trajectory.go`, `internal/textureowner/router.go`, front-end calls above | MA3.7 | red / 2 | a route test per app; the Texture UI works against the projection endpoints; Files no longer lists `mail/` | revert |

Supersedes: desks 7.1 (ownership service, enforce schedule), desks 4.6 and D2,
desks 8.1 Read class (now ceiling plus lease). Keeps: desks 7.2 and 7.3 (schema
versions, uninstall and replace) in Phase 4.

## 8. Custody section

### 8.1 The invariant and its exceptions

Hard invariant (R15): "No cross reads!" No desk reads a kind it is not the
custodian of except through (a) a lease it asked for through a precommitment-record request, (b) data a
custodian carried to it in a record, or (c) a request answered by the
custodian. Minimal exceptions (inventory 6.3):

- **E1 owner root.** Every owner-authenticated HTTP read (Files, Texture,
  trajectories, inboxes, diagnosis, captures) stays; the owner is above the
  desks.
- **E2 commons.** The ledger, inboxes, wake outbox and run bookkeeping are the
  precommitment-record substrate: desks see them only as self-scoped `Records` and `Pack`
  (`internal/yaegikernel/choir.go:1099-1116`, `internal/agentcore/tools_desk.go:1039-1077`);
  kernel services read them freely; management gets a supervisory read of
  assignments, admissions and ledger health.
- **E3 reducer as the custodian's agent** for the one read that attaches owner
  directions to a Cast (X5); or move that read into Texture's turn.
- **E4 shared world content.** Captures (corpusd Store B), publications, world
  events and web results are not owner-private custody; research, Texture and
  management read them under `captures.read` and `world.read`.
- **E5 engineering capsule snapshot** becomes a lease (7.2).
- **E6 host integrity services** (vmctl, witness, data-image capture) keep
  digest-level or opaque access.

### 8.2 A custodian per kind

Proposals; the first step of MA3.1 lists all 52 `WriterRuntime` lines and
confirms. Kinds the inventory enumerated are named; the remainder are placed by
the same categories and marked unverified.

| Kind group (registry lines, verified in the inventory) | Custodian | Readers (besides the custodian) |
|---|---|---|
| Mail: messages, recipients, attachments, source packets, drafts, approvals, risk alerts, aliases, digest (maild stores, host side) | Mail desk, through the maild service | the owner through the Mail app; nothing else; no guest verb reads mail today |
| Texture documents, revisions, decisions, root, source entity and ref, citation check, support verdict, story library head (`registry.go:76-77, 97-99, 115, 149, 162-163`) | Texture desk | owner (E1); research through `texture.search` lease; management through a Texture-published story index; engineering and reducers only by carried data (8.4) |
| Research evidence (`registry.go:100`) | research desk | Texture reads carried excerpts in report packets, not the store |
| Assignments, subject candidates, admission, capture disposition (`registry.go:148, 166-175` in part) | management | engineering sees its own assignment through its inbox |
| Capability, capsule and run claims, slots, engineering lineage, self-development operations, capsule effect bundle (`registry.go:112, 166-175` in part; `manifest.go:270`) | engineering | management supervisory |
| Capture labels and overrides, world events, event membership, source schedules (`registry.go:108, 120-124`) | management (intake and the Autopaper source schedules, until Autopaper is an app agent; excluded from this mission) | research polls through `PollSource` requests |
| Commitment records, inboxes, conversation heads, lifecycle events and commands, wake outbox, channel messages, coagent mailbox, runs, agents, events, trajectories, work items, run acceptances, run continuations, run memory entries (`registry.go:87-96, 90, 107, 147-158`) | kernel (`system:kernel`), the commons | self-scoped views per desk; management supervisory; owner (E1) |
| Install records (was release adoption, `registry.go:128`) | kernel | owner, Library |
| Captures, publications, entities, subjects (`registry.go:78, 178-222`) | `system:corpusd` | all desks per E4 |
| Files, file objects (`registry.go:133-134`) | `ownerfiles` (owner) | the owner's apps; Files route excludes `mail/` |
| Computer settings, content items, podcast subscriptions, browser sessions, media progress, desktop state, theme (`registry.go:101-103, 109`; `schema.go:195-237`) | shell and owner-act services | owner |
| Package kinds `choir.app.<pkg>.*` | that package's desk | none without a lease |

### 8.3 Reads that already comply

`Context`, `Records`, `Pack` (own part), `ReadDoc` for the Texture desk's own
document, Texture's own HTTP API, app package objects (only the owner route reads
them, `autoputer/app_package_objects.go:170, 190`), mail, and the conductor,
which reads only the submitted text (inventory X11).

### 8.4 The fate of every cross read

| # | Cross read (file:line) | Fate | Slice |
|---|---|---|---|
| X1 | Research corpus search over all Texture documents: `researchtools.go:71` calls `Store.SearchDocuments` (`store/texture.go:125`), which lists every owner Texture document (`searchableTextureDocuments`, `:143-158`) and uses the in-memory head index (`store/search_index.go:15-29`) | **Decided (Q3): a `texture.search` lease held by a Texture sub-RLM of research**, section 8.6. The capture, event and published parts of the same tool are shared world content (E4) and stay | MA3.7 |
| X2 | `System1` context ref `story:<rev>` reads any revision of the owner: `agentcore/system1_verb.go:461-475` | the cell passes the text, or the call carries `text`; the ref form is deleted | MA3.7 |
| X3 | `StoryAnalysis` preloads a story's head (`agentcore/story_analysis.go:142, 200-220`, unverified line numbers) | Texture supplies the text in the Cast or a Texture-owned export; research never reads the revision | MA4.6 | Research as a package (D6) with the fan-out of section 8.6: a research parent and two child roles, `research_web` (holds `world.read`, no lease) and `research_texture` (holds the `texture.search` lease, no `world.read`); children report to the parent as precommitment records | `internal/agentcore/research_children.go` (existing child mechanism: `runResearchChildren` serves `choir.Research`, children are `research_child` desks with read-only world verbs, no further `choir.Research`, no messaging, reporting or writes, an inert ledger reduction, one level of fan-out, `:132-145`), `internal/deskmanifest/manifest.go:287-298` (the `research_child` role row), `internal/yaegikernel/{search_store.go,system1.go,relate*.go,story_analysis.go}`, `internal/agentcore/desk_search.go` (unverified) | MA4.5, MA3.4, MA3.7, MA1.5 | red / 2 | the deployed Texture acceptance unchanged (research answers its casts); each child call is a Cast with the parent's typed prediction and its Report is scored (MA1.5); a run in which one child tries the other's grant is refused at the frame check; the web child's transcript contains no Texture-derived text; a third-party manifest asking `world.read` is refused | revert |
| X4 | Engineering run context re-reads owner words each render (`agentcore/engineering_owner_words.go:152-165`) | read the data attached to the Cast record (the record is the precommitment-record substrate) | MA4.10 |
| X5 | Reducer attaches `owner_directions` to a Cast (`agentcore/texture_owner_directions_cast.go:466-478`) | E3: the reducer acts as Texture's agent for this one read, or Texture's turn writes the words | MA3.7 |
| X6 | Capsule store copy and files tree: `capsule/computer_state_linux.go:135-177`, `autoputer/capsule_executor_linux.go:144-156` | a lease: kinds chosen per assignment; default excludes mail (already carved out at `capsule/computer_state.go:45`) and Texture bodies | MA3.4 |
| X7 | Management `ProductAPI` GET `/api/texture/` (allowlist `agentcore/tools_product_api.go:215-222`; writes refused `:199-204`) | narrow to list and summary routes; content through a request to Texture | MA3.7 |
| X8, X9 | Management `Stories` (`agentcore/intake_view.go:67`) and the intake labeler's judge prompt (`intake_labels.go` ~`:240-300`, unverified) read story summaries | Texture publishes a story-index projection; management reads the projection | MA3.7 |
| X10 | Management reads mail: none found | no change | none |
| X11 | Conductor reads anything: no | no change; router (MA4.8) keeps it so | none |
| X14 | `Pack.Waits` for management (`tools_desk.go:1062-1068`) | custodian read; management is custodian of assignments and admission (`manifest.go:219`) | MA3.1 |
| X14b | `ProviderWaits` to every desk (`tools_desk.go:1074-1076`) | a host-supplied scalar; no kind crosses | none |
| X15 | Cross-kind search (`researchtools.go:71, 96, 107, 114`) | split: Texture part is X1, the rest is E4 | MA3.7 |
| X16 | Texture reads research evidence by id (`textureowner/texture_evidence_sources.go:147-155`, unverified) | report packets carry the excerpts and claims; evidence by id becomes a request to research | MA4.6 |
| X18 | Research `RunMemoryEntry` is unscoped by run or agent (`agentcore/tools_evidence.go:57`) | self-scope: own run's entries only | MA3.7 |
| 6.2-8 | Management `ProductAPI` `/api/autopaper/` write path touches source schedules (no desk custodian; allowlist `tools_product_api.go:221`) | management is custodian of source schedules (8.2) and the write ban at `:199-204` is extended to `/api/autopaper/` for kinds it is not custodian of | MA3.7 |
| 6.2-9 | `/diagnosis`, `/api/trajectories`, `/api/inboxes/*` join kinds | E1 owner root; keep | none |
| 6.2-10 | Every `WriterRuntime` kind lacks a custodian | table 8.2 | MA3.1 |
| Files/mail | The Files route lists `mail/` (`autoputer/files.go:607-608`); the inbound route has no caller | carve-out; delete the route if no caller exists at build time | MA3.10 |
| UI | Texture UI reads lifecycle and engineering evidence kinds | Texture-custodian projection endpoints | MA3.10 |
| Proxy | Proxy to the paper computer's Texture HTTP (`proxy/platform_expand.go:407, 450, 597`, unverified) | already a request to the custodian | none |

### 8.5 The two layers in the custody section (R20)

Custody has a ceiling and leases. The manifest ceiling names what a package may
request (`texture.search`, `mail.read(ref)`). A lease is a custodian-issued,
tape-recorded, owner-visible, owner-revocable grant of a scope for a duration
for a stated purpose, enforced at the frame check and the store gate. The video
editing app agent of the owner's example gets exactly this: its ceiling may name
`mail.read(ref)` and `texture.search`; at runtime it asks the Mail desk through
a precommitment-record request to read a specific email and the Mail desk issues a per-task lease for that
ref; it asks Texture for a lease on a video-texture surface. With a strong
record in the action class (Phase 7) the same request is answered with a
durable lease that renews without asking; a fidelity event revokes it at once.
Delegation-scoped reads (X6, the capsule snapshot) are leases.

### 8.6 Research's corpus search over Texture (R19, settled by Q3)

Decided: use the RLM pattern properly. The research desk fans out to
sub-RLMs, one for the web and one for Texture, and its parent receives only
their reports. This is the structural taint split (option (a) of the earlier
draft), realised as child RLMs.

**What exists.** Research already fans out. `choir.Research` is served by
`runResearchChildren` (`internal/agentcore/research_children.go:132`): each
child is a `research_child` desk (a role row, `internal/deskmanifest/manifest.go:287-298`)
on its own model loop with read-only world verbs, no further `choir.Research`
(one level of fan-out, "rounds over recursion", owner 2026-10-11 PD1), no
messaging, reporting or writes, and an inert ledger reduction; concurrency and
the computer-wide call spacer bound the tree (`:22-48, :93-106`); the whole
tree ends with the calling cell's deadline. Results come back as return
values. The present Texture search is not a child: the tool `search_wire_corpus`
calls `Store.SearchDocuments` from whatever session holds it
(`internal/researchtools/researchtools.go:34, 42-48, 71`; implementation
`internal/store/texture.go:125`, listing every owner document at `:143-158`,
head index `store/search_index.go:15-29`).

**The design.**

- Two child roles of the research desk, each a role with a subset of the
  desk's grants (a role can only narrow; section 6, B1):
  - `research_web` holds `world.read` (search, read, poll sources) and no
    lease. It reads untrusted web content.
  - `research_texture` holds a `texture.search` lease and no `world.read`.
    It reads owner-private text and has no outbound channel.
- `texture.search` is a first-party grant whose lease the Texture service
  issues (section 7.2). It grants a guarded, typed search API that Texture's
  service owns: ranked refs plus bounded snippets labelled
  `owner_private:texture`, backed by SQL or full-text search, with limits on
  results, snippet length and rate. Every query is logged as a visible
  precommitment record. It grants no raw documents, files or arbitrary SQL. A
  full document body is never returned: it comes only by a ref Texture hands
  over in a record or a lease on that ref, or by a request to Texture.
- The parent spawns both children in one fan-out call. Their objectives are
  written from the parent's own, unlabelled context, so no Texture-derived text
  can reach a web query. Each child returns a **Report precommitment record**
  (claims, excerpts, source refs, `p_success`, provenance, labels), and the
  parent receives only those. This changes the present child contract (results
  as return values, no reporting): each child call becomes a Cast carrying the
  parent's precommitment about the result (section 4.4, R28), and a child's
  single permitted act becomes a Report addressed to its parent, stamped by the
  reducer and scored; everything else in the child contract stays.
- The Texture child's report carries `owner_private:texture`, so the parent
  run now carries that label (MA3.5). Any later outbound act by the parent,
  including spawning another web child with an objective it wrote after reading
  that report, meets the disclosure path (section 7.2): the kernel table denies
  or asks, System 1 may be asked, the owner may tap. Rounds after the first are
  therefore the controlled case, not the default.
- Raw SQL is not offered: one query can dump everything and is hard to guard.
  Read-only views are a possible later middle ground (the custodian defines a
  view per purpose and the lease names it).

Settled by this: the option between a separate-children split and a post-read
trust gate (the trust gate remains as the control on the later rounds above).
The grant name and the guarded API shape are decided; the snippet, result and
rate limits are parameters set when MA3.7 is built and logged with the
lease.

## 9. Phase 4: every desk is a package

Goal: every desk except Texture's (which moves with its app in Phase 5) is a
package desk loaded by the one path; the desk-name switches are deleted.

Order, by the roadmap: Devin's B1 to B4 first; then research, Mail,
management, engineering last. Texture follows in Phase 5 (section 18 item 6).

### 9.1 Slices

| Slice | Work | Files | Deps | Class / tier | Proof | Rollback |
|---|---|---|---|---|---|---|
| MA4.1 | B1: complete the grant catalogue (section 6) and the desks design 4A table | `internal/deskmanifest`, desks doc text | MA2.5 | red / 2 | a test that the union of grants held by the four built-in desks reproduces today's verbs, tools, commit paths and run budgets, and the names gate | revert |
| MA4.2 | B2 and D3: reconcile, update, epochs, retire; `FrameAllowedAt`; `DeskEpoch` in the session key; boot closes open obligations in one pass keyed on the registry | `internal/appdesk/appdesk.go`, `deskmanifest/manifest.go:425-430`, `yaegikernel/desk_session_worker.go:605-631`, `agentcore/tools_desk.go:206-213`, `internal/kernel`, `internal/autoputer/run.go` | MA4.1, MA2.1 | red / 2 (boot, concurrency) | an update that narrows a verb set during a live run: the run finishes under its epoch; a leak differential (goroutines and registry sizes return to baseline); a crash between event and reconcile | revert |
| MA4.3 | B3: candidate namespace `<pkg>-candidate`; reserved suffix; candidate desks write the shadow kinds; no inbound routing, no schedules, no egress | `internal/appdesk`, `internal/objectgraph/package_kinds.go`, `internal/selfdev` | MA4.2 | red / 2 | a candidate runs beside the installed desk on d@e, writes only shadow kinds, a write to the installed kind is refused, acceptance supplants and drops the shadow | revert |
| MA4.4 | B4: required set is image config, desired state is fold union required; re-seed on boot | `internal/appdesk`, boot path in `internal/autoputer/run.go` | MA4.2 | red / 2 | restore to a pre-install head: the computer boots with the required packages re-installed by a new event and open obligations closed; a required package refuses disable | revert |
| MA4.5 | `contributes.desks` wired for first-party registration: delete `CHOIR_APP_DESKS` (`appdesk.go:328`), the `app_packages.objects` setting (it defaults ON when absent, `internal/autoputer/app_package_objects.go:96-104`, against its own comment), and the P4 gate; `Activate` gets its first production caller | `internal/appdesk/appdesk.go:327-340`, `internal/autoputer/app_package_objects.go`, `internal/apppackage/manifest.go:196-198`, `internal/store/app_package_objects.go` | MA4.2, MA2.3 | red / 2 | the staging health lists the derived desk set; a third-party manifest asking for a privileged grant is refused | revert |
| MA4.6 | Research as a package (D6): the world tools as a module; the `texture.search` lease (MA3.7) and the document-search child per 8.6(a) | `internal/yaegikernel/{search_store.go,system1.go,relate*.go,story_analysis.go}`, `internal/agentcore/{desk_search.go,research_children.go}` (unverified) | MA4.5, MA3.7 | red / 2 | the deployed Texture acceptance unchanged (research answers its casts); a third-party manifest asking `world.read` is refused; a search-then-web run in one run is refused | revert |
| MA4.7 | **Mail app agent** (new): the Mail package ships the Mail UI (modular C13, moved here) and a Mail desk that absorbs the email desk rows only (`deskmanifest.Email`, a carrier-less row with an empty registry: `manifest.go:305-309`, `agentcore/tool_profiles.go:411, 421`). Mail data stays in maild, reached through a maild service the Mail desk is custodian of; objects it writes are package kinds written by the reducer. Not the conductor | `frontend/src/packages/mail/**`, `frontend/src/lib/MailApp.svelte`, `internal/maild` service seam, `deskmanifest`, `agentcore/tool_profiles.go` | MA3.8, MA4.5, S1, S2, S7 (the C13 deps) | red / 2 | `email-app-state.spec.js`, `mail-digest-toggle.spec.js`, `mail-attachments-deployed.spec.js`; a video-editing-style fixture desk (a test-only first-party fixture, since community desks wait for Phase 6) asks Mail through a precommitment-record request for one email by ref, gets a per-task lease, and is refused a second email | revert both halves together |
| MA4.8 | **The prompt router**: the conductor as a System 1 router (section 9.2) | `internal/textureowner/prompt_bar.go`, `internal/content/content.go:1639`, `internal/agentcore/texture_model_api.go:31-42`, `internal/agentcore/system1_verb.go`, `internal/deskmanifest` (conductor row), router package | MA3.6, MA4.5, MA1.3 | red / 2 (the owner's only door) | section 9.2 | revert to the deterministic default route |
| MA4.9 | Management as a package (D9): admission ledger, intake controller, platform update, `product.api`; persistent per-owner identity as `instance: per_owner` | `internal/agentcore/{management_controller.go,management_tasks.go,platform_update.go:184,229,337,tools_worker_update.go,intake_view.go,calibration_feedback.go}`, `internal/actorruntime/**` | MA4.6, MA4.7, MA4.1 | red / 2, two seats, re-reviewed after any blocking-real finding | a platform update applied from d@e by the packaged desk; engineering admission FIFO unchanged; owner escalation reaches the owner | revert |
| MA4.10 | Engineering as a package, **last** (D8): capsule driver registered, `store/engineering_assignments.go` profile checks (`Profile != Engineering`, desks doc `:728-787, 845, 1289, 1512`, unverified) become grant checks, generic assignment ledger | `internal/capsule/**`, `internal/agentcore/{engineering_assignment_runtime.go,engineering_waits.go,self_development_materializer.go}`, `internal/store/engineering_assignments.go`, `internal/selfdev` | MA4.9 and a week of MA4.6 to MA4.9 runs on staging | red / 2, two seats | a full self-development operation (capsule, freeze, verify, apply) on d@e from the packaged desk; a crash during a capsule closes it "interrupted by a restart"; no planned restart while a capsule is live | revert; in-flight assignments finish under their epoch |
| MA4.11 | Delete the desk-name switches: the names gate (MA2.5) allowlist, which holds only Texture's remaining uses, is cut to its Phase 5 residue; `agentprofile.Canonical`/`IsLive`, `NormalizeRole`, `promptRoles`, `recordsDesk` and the per-desk overlays are registry lookups | as in desks 1.4 and 1.5 | MA4.10 | red / 2 | the names gate; the grep count (586 references in 104 files at the Devin measurement) is a bound that the gate drives to the Texture residue, not an enumeration | revert |

The desks design's stale counts and cites are corrected: `WriterRuntime`
registry entries are 52, not about 21; `deskWorkerArg` is
`internal/agentcore/tools_desk.go:36`; `ErrWriterConflict` is
`objectgraph/writer.go:88`.

Supersedes: desks 9.2 and slices D1 to D9 wording, desks 10 D4 to D9 order,
desks decision A. Keeps: desks 4.5 (schedules vs occurrences), 6.2 to 6.5,
7.2, 7.3 (schema versions; uninstall and replace).

### 9.2 The prompt router (MA4.8), a decided design

**The router is the conductor.** It is not folded into Mail (R18) and it is not
a desk with custody. It is a kernel-adjacent package: first party, required
(B4), providing the service `prompt.route` to the Go kernel; it owns no object
kinds, reads no kind but the submitted text and the bounded context named
below, and writes only its route records.

**What the conductor does today.**

- The owner's text arrives at the prompt-bar route and `submitPromptBar`
  (`internal/textureowner/prompt_bar.go:206`). The default app is Texture
  (`:207`).
- A deterministic classifier, `ClassifyPromptBarContentIntent`
  (`internal/content/content.go:1639`), fires only when the text is exactly one
  whitespace-delimited token that normalises to an http(s) URL; it maps the
  media type to an app hint ("files" becomes "browser", `:1653-1660`).
  Everything else, every sentence, goes to Texture.
- It records a "conductor decision" run through `CompletePromptBarDecision`
  (`internal/agentcore/texture_model_api.go:31-42`) with the action `open_app`
  (`prompt_bar.go:235-238`) and, for Texture, hands off with caller profile
  conductor (`:240-244`). The decision shape is `ConductorDecision`
  (`internal/textureowner/texture_handoff.go:40-52`). A `continue_doc_id` goes
  to the owner-revise path (`continuePromptBarDocument`, ~`:150`).
- Management reaches the same route to create Textures
  (`internal/agentcore/tools_product_api.go:215-216`).
- The conductor desk row has no verbs, targets or tools
  (`internal/deskmanifest/manifest.go:300-304`;
  `internal/agentcore/tool_profiles.go:409, 419`). A `conductor` model role
  exists on the "low" pool (`internal/modelpolicy/model_policy.go:679`) and a
  prompt overlay exists (`internal/runtimeprompts/prompts.go:48`); whether
  any run uses them is a hypothesis (the decision is recorded as a
  deterministic run).

So today nearly everything routes to Texture. The router matters once there
are more app agents (MA4.7 is the first, Mail), and the owner's instruction is
to make it a system that learns by the practice of precommitment records and
is dogfooded and calibrated by us before launch (R25).

**The design.** One owner submission is one route, and one route is one
precommitment record:

1. *Candidates* come from the installed packages' `accepts` declarations
   (cast kinds, file media types, the existing `opens` routes of the
   manifest, `internal/apppackage/manifest.go:88-94`) plus the standing
   choices (new Texture, continue the focused document, open an app). The
   choice set is therefore open and derived from the registry. This is decided
   from the code: the set of destinations is not enumerable in Go source, since
   packages install at runtime; the standing choices guarantee a default.
2. *The question.* A System 1 typed choice question (`system1_verb.go:20-40`,
   request construction `:516-546`): "which destination does this input want?",
   with the owner's text and a bounded context (the focused app and the title
   of the focused document) as the evidence. The judge returns a probability
   per candidate. The record is frozen with the full distribution, the pinned
   model id, the context pack ids and the chosen route **before the app
   opens**. The deterministic URL classifier stays as a rule route: it also
   emits a route record with its own calibration, so rule routes and judge
   routes are scored on the same footing.
3. *Decision.* Take the top candidate when its calibrated probability clears
   the threshold `theta`; otherwise abstain and disambiguate (a one-tap
   choice among the top candidates, defaulting to Texture, today's behaviour).
   A judge failure or timeout takes the default route. The router performs no
   effect itself; it hands off by a request record to the destination's inbox.
4. *Settlement by rule from the owner's next acts.* The settler is a rule over
   desktop and window events (the desktop state surface,
   `internal/desktopstate`; hypothesis: the focus and input signals needed
   exist, and the build's first step is to confirm them):
   - **confirmed** if the owner stays and works in the chosen app or desk
     within window `W` (focus held and input received);
   - **contradicted** on a redirect (the owner opens a different app or desk
     within the window), a close-and-reopen elsewhere, or an explicit
     correction;
   - **unresolved** otherwise.
   Corrections are first-class one-tap acts ("not here", with the right
   destination); a correction both settles the route and labels the right
   answer. Unresolved routes are reported as a rate and excluded from the
   calibration fit, with that exclusion stated in every report because
   unresolved is not random.
5. *Calibration.* Fit a per-type temperature, or isotonic regression where
   the data allows, on settled routes, per pinned model
   (`docs/precommitment-records.md` section 5.3: miscalibration sign depends on
   question type; calibrate per type; clamp probabilities before logging).
   Report reliability and sharpness together, since calibration can improve
   simply by becoming vaguer. A new pinned model starts uncalibrated and
   abstains more.
6. *In-context learning.* A small context pack (k similar settled routes, with
   their text, the distribution given, the settled outcome and any correction)
   is retrieved from the owner's own route records into the System 1 prompt.
   This is the ICRL loop of the practice. Retrieval is lexical over route text
   (the head-index style of `store/search_index.go:15-29`); the pack carries the
   label `owner_private:router` and goes only to the judge the VM's model policy
   chose.
7. *Abstain and disambiguate.* `theta` is tuned to a target error rate `E*`
   from the settled data: the smallest threshold at which the settled error
   above it is at or below `E*`, recomputed per window of routes. `E*`, `W` and
   `k` are starting parameters committed in the ledger round for Phase 4 and
   tuned by dogfooding, not open questions.
8. *Scorer audit.* A sample of routes, weighted toward disagreements and low
   margins, is scored by a second-opinion scorer (a different pinned model, or
   the owner's correction), and the self-favor gap is computed: how much more
   the judge favours its own routes than the second scorer does
   (trust draft 4.1, calibration beside the posterior).
9. *Measures and dogfooding.* Routing accuracy (of settled routes), calibration
   (reliability, Brier, log score, with skill against the "always Texture" base
   rate), sharpness, the tap rate (disambiguations plus corrections per day)
   and the unresolved rate, over time, on the owner's own computers once app
   agents exist. "Good enough for users" is set by the owner at launch from
   these curves. The route records and their settlements feed the paper
   harness (MA7.9) and the Phase 4 ledger round.

A route is also a request-record sender in the trust sense: the router has a
track record in the action class `route/I1`, and a destination desk may use it.

## 10. Trust: how the plan revises the draft

The draft [bayesian-trust-2026-11-04.md](bayesian-trust-2026-11-04.md) is copied
in unchanged except for a header note. The plan changes three things in it.

1. **Two layers of authority, not "trust never adds a capability".** The draft
   says "Trust never adds a capability" (1, 5). Revised by R20: **trust never
   exceeds the ceiling; it governs leases within it.** The ceiling is the
   manifest's declared capability list, reviewed at install and shown in the
   Library. Leases are custodian-issued scoped grants (section 7.2). Trust sets
   a lease's duration and scope: a strong record in the action class gets a
   durable lease that renews without asking, a thin or new desk gets a
   per-task lease, a fidelity event (E3) or platform event (E7) revokes the
   lease at once (the draft's circuit breaker, 5.2, becomes lease revocation
   plus gating). The draft's "grants are the ceiling, trust is the dial"
   table stays true: the dial now also moves lease length.
2. **The act gate comes last** (Phase 7). Until then leases are per-task and
   non-first-party I3 acts are gated by the label rules and the owner tap
   (the draft's own conservative default, 10).
3. **Model provider calls are not I3.** Per R15 no rule constrains models;
   the draft's I3 example "a provider call carrying owner-private data" (3.2)
   is removed; model calls are logged with provider and labels. The draft's
   spend language already follows R8 (no ceiling, monitor).

## 11. Phase 5: system apps and services become packages

Goal: every system app, shell surface and service is an installed package; the
Library distributes services; Texture last.

The slices are the system design's, kept with its content and with these
changes: the K0, G0 and P0 slices are Phase 2; C13 is MA4.7; the order is
S1, then {S2, S3, S5, S6, S7, U1} in parallel on S1's tip, then W1, then S4,
then the C batches and H1 to H4, then the forced-modularity slices, then the
Library kinds, then Texture.

| Group | Slices (modular section 5, kept) | Notes for this plan | Class / tier |
|---|---|---|---|
| Services | S1 SDK 1.1.0 and shell functions; S2 commands and shortcuts; S3 one file-route table; S4 context menu on the slot; S5 theme; S6 settings; S7 deep links; W1 window and launch API; U1 `uses` extraction | S1 owns `Desktop.svelte`, `AppHost.svelte` and the SDK lock (serialized); S4 is where the renderer-swap drill from K0 runs | orange / 1 (U1 yellow / 0) |
| App batches | C1 Calendar, C2 Pulse, C3 Audio, C4 Video, C5 Slides, C6 Podcast, C7 PDF and EPUB deleted; C8 Autopaper, C9 Compute Monitor, C10 Source, C11 Web Lens; C12 Settings | Mail (C13) moved to MA4.7. Each deletes only its registry row. Audio and Video need the five media helpers named in SDK 1.1.0 | orange / 1 (tier 0 for Pulse, Calendar) |
| Shell surfaces | H1 launcher, desktop-icons, overview; H2 `menu-default`; H3 `prompt-bar`; H4 `notifications` and `Desktop.svelte` reduced to the window host with the last-resort surface | H3's front end pairs with MA4.8's router service. The window manager and context-menu event capture stay host (modular 1 items 3 and 4) | red / 2 |
| Forced modularity | M2+M3 together, M4, M5, X1 (SDK 2.0.0), F1 (update policy and platform yank) | M1 is MA2.4. F1 compares extracted `uses`, grants and the behaviour-surface hash | red / 2 (M4, M5, X1 orange / 1) |
| Library distributing services | L1 kind, slots, tier, uses, displaces, and now level and ceiling; L2 selection records; L3 tiered review; L4 watchdog failover and yank reaching the kernel | L3's T2 refusal stays; the Library block shows the ceiling, the levels and the lease policy; L3 is the install-time forced review of R10 for the mechanical part | L1, L2, L4 orange / 1; L3 red / 2 |
| Texture | T1 Texture app, with the Texture desk (D7): committer registered with the kernel, `textureCellAuthorizer` field removed, `/pub/texture/` as a `contributes.links` public route, `TEXTURE_APP_ID` leaves `registry.ts` | last; app and desk land together; the names gate allowlist reaches zero here | red / 2 |

New slices this plan adds to Phase 5 (Devin's unassigned-work amendments and
the owner's "context menu with full coverage"):

| Slice | Work | Files | Deps | Class / tier | Proof |
|---|---|---|---|---|---|
| MA5.1 | Assign the unowned work: `SettingsApp.svelte` renders `contributes.settings` (S6 must list it: `frontend/src/lib/SettingsApp.svelte`); the logged-out preview-window seed (`Desktop.svelte:275-291`) gets a `links`/preview rule owned by S7; `notification.post` pipeline wiring owned by H4; the SDK names for W1 (`focus`, `restore`, `suspendBackgroundHeavyWindows`, `isHeavyAppId`, used by `ComputeMonitorApp.svelte:3`) and H3 (`submitPrompt`, `windowAction`, `showOverview`, `showDesktop`, `logout`, used at `PromptSurface.svelte:95, 135, 150, 276-280`) enumerated in the 1.1.0 lock | the listed files | S1 | orange / 1 | the SDK lock lists the names; each slice's proof covers its added file |
| MA5.2 | Context menu with full coverage: every window, desktop, file, app and text target contributes through `contributes.menus` and the `contextMenu.build` pipeline; a sweep spec right-clicks every registered surface; the `essential` items stay | `lib/context-menu.ts`, `ContextMenu.svelte`, each package | S4, C batches | orange / 1 | a Playwright sweep that every registered target shows a menu; `context-menu-open-with.spec.js` unchanged |
| MA5.3 | Window management as a service: the `windows` provider contract and the API of W1; the window host stays in the shell | `lib/stores/desktop.js`, `Desktop.svelte` | W1, H4 | orange / 1 | `desktop-window-manager.spec.js`, `desktop-state-persistence.spec.js` unchanged |

Folded system-design amendments (Devin ACCEPT): the collision arrows (below);
unassigned work (MA5.1); v1 manifest reader lifetime (MA2.3, section 18 item 12); K0 drill moved to S4; factual slips (the Texture family is 21 other files:
`PublicationResult`, `SourcePanel`, `Toolbar` and the rest; S1 does not name
`packages/library-next/App.svelte` as a clipboard site because it has no
`navigator.clipboard` at this commit); doc inconsistencies resolved: provider
resolution order is declared `priority` first, recency as the tiebreak
(modular 2.4 governs; the "most recently installed" wording at modular 1 is
wrong); `rank.high` is added to the grant vocabulary of 2.5 and `ui.shell`
versus `ui.shell-menus` is settled as `ui.shell` for the whole shell and
`ui.shell-menus` as a narrower grant for menu renderers; `keys.global` is bound
to the keybinding resolver or dropped; "build fails when `uses` exceeds
`grants`" reads "grant-implying `uses`"; third-party access to
`commands.resolve`, `files.openWith` and `notification.post` is refused until
the sandbox, like decision 3; S4 depends on S3 for Open with.

## 12. Phase 6: isolation

Goal: front-end sandbox; hardening and fuzzing of the worker and broker; gVisor
for desk workers if Phase 0 holds; one runtime for all desks.

### 12.1 What Phase 6 gates (R24)

- **Desk-bearing packages.** No desk-bearing package from a non-first-party
  publisher can be published or installed until MA6.1 to MA6.5 have landed
  (the iframe sandbox, the broker audit and fuzzing, and the worker sandbox
  Phase 0 selects; if Phase 0 rejects gVisor, the gate is the iframe sandbox,
  the audit, the fuzzing and the alternative the owner chooses). The
  validator enforces it as a capability check (MA2.3). First-party desks are
  unaffected, so Phase 4 proceeds.
- **Front-end-only packages.** The earlier ruling (R16) stands: publishing
  stays open until the iframe sandbox ships, with no known-publishers interim.
  The two rulings fit together this way, as the plan's reading:
  - *Publishing* a front-end-only package by any publisher is open before
    MA6.1. A release lands in the Library catalogue.
  - *Installing a data-only package* (themes, command packs, settings
    schemas, the T0 kinds of modular 2) from another publisher is allowed
    before MA6.1 because it runs no code.
  - *Installing a code-bearing front-end package* (app windows, pipeline
    handlers, providers) from another publisher is refused before MA6.1,
    because before the sandbox it would run in the shell realm with the session
    and could read routes. This is R24's "no third-party front-end package that
    reads routes before the iframe sandbox"; it matches the system design's
    standing statement that the Library does not offer T1 code from other
    publishers before S13 (modular section 1). After MA6.1, such packages
    install and read only the routes their ceiling names, through the bridge.
  - First-party code-bearing packages are not affected.

### 12.2 Security process (R27)

1. Defence in depth, built in the order of this plan: per-user VMs, tape
   restore, process confinement and the frame check (exist), custody and leases
   (Phase 3), the sandboxes (this phase), trust (Phase 7).
2. Once the functionality is satisfying (after Phase 5 for the product path and
   MA6.1 to MA6.5 for isolation), agentic red teaming from the inside: agents
   with the community-desk threat model attack d@e only, with planted
   known-bad packages (the drills of MA7.9). Never the paper or an owner's
   computer.
3. Iterate: each finding is a problem record first, then a fix, then a new
   drill; the red-team round repeats after each hardening slice and before any
   widening of what a non-first-party publisher may publish.

### 12.3 Slices

| Slice | Work | Files | Deps | Class / tier | Proof | Rollback |
|---|---|---|---|---|---|---|
| MA6.1 | Iframe sandbox for app UIs: opaque-origin frame with a message bridge (modular decision 7); the SDK in the frame is stubs that post messages; every call grant-checked against the ceiling; the `fetchWithRenewal` session never crosses. After it ships, code-bearing front-end packages from other publishers can install (section 12.1) and the MA3.10 app-identity check becomes unforgeable. Until it ships, publishing stays open (R16) and code-bearing installs from others are refused | `frontend/src/lib/PackageAppFrame.svelte`, `internal/pkgruntime/build-module.mjs`, `app-sdk.ts` | Phase 5 | red / 2 | a hostile fixture app cannot read the shell DOM, the session, or another app's routes; the contract runner passes in sandboxed mode | revert; renderers stay T2 |
| MA6.2 | Worker and broker hardening audit (red-team items 3 and 5): explicit denials in the seccomp profile (io_uring, keyctl, bpf, perf events, userfaultfd, ptrace, namespace clones), a mount namespace without /proc, /sys and /dev, fd closure across the worker's execve, rlimits, a unique uid per activation, restart intensity for crash loops. Each is checked against `desk_cell_confinement_linux.go` (not read by this author: hypothesis that some are already present) | `internal/yaegikernel/desk_cell_confinement*.go`, `desk_worker_spawn_linux.go`, `desk_cgroup_linux.go` | none | red / 2 | the profile test enumerates what each is; an escape test per denied class; a selftest at boot | revert |
| MA6.3 | Broker protocol fuzzing: native Go fuzz targets for `BrokerRequest` decode, the frame multiplexer (`worker_mux.go`) and each frame handler's payload; bound every field; audit `FrameAllowed` and verb handlers for confused-deputy use (a verb that acts with the daemon's authority on a cell-supplied reference) | `internal/yaegikernel/{broker_protocol.go,worker_mux.go,desk_session_worker.go}`, `agentcore/tools_*` handlers | none | orange / 1 | a fuzz corpus in CI for a fixed time; each handler has a field-bound test | revert |
| MA6.4 | gVisor for desk workers, if Phase 0 holds (section 18 item 2): the spawner starts the worker under `runsc` with systrap; the fd 3 socketpair and slot token cross unchanged; "required" confinement mode restated for the new layering. Yaegi runs inside gVisor, performance permitting; engineering's mutation already runs in capsules. How many sandboxes (one per desk, or shared per trust class) waits for the running memory experiment (MA0.1) | `internal/yaegikernel/desk_worker_spawn_linux.go`, `desk_cell_confinement*.go`, guest image | MA0.1, MA0.2, MA6.2 | red / 2 | the cell benchmark within the accepted ratio; a hostile cell cannot reach the host; a boot selftest | revert to the process-only profile only by an owner act, since it lowers isolation |
| MA6.5 | One runtime, yaegi with full Go modules, for all desks: every desk, first-party or published, runs model cells and package-shipped Go modules in the same yaegi worker; the import allowlist (16 stdlib packages, `internal/yaegikernel/allowlist.go:11-28`, banned list `:32-43`, fail-closed static check `eval.go:101-122`) widens by expressivity under the sandbox boundary (still no `unsafe`, `syscall`, `os/exec`, raw sockets); package Go modules are vendored by the platform build (source only) and loaded with their declared imports | `internal/yaegikernel/{allowlist.go,eval.go}`, `internal/pkgruntime`, `internal/apppackage` | MA6.4 | red / 2 | a desk cell calls a package Go module and the host verbs; a module importing a banned package is refused at publish; the cell benchmark | revert |

Supersedes: desks 4.3 (T1a, T1b), D11. Keeps: desks 8.2 layers.

## 13. Phase 7: Bayesian trust

Per the trust draft, with the revisions of section 10. The act gate comes last.

| Slice | Work | Class / tier | Deps | Proof |
|---|---|---|---|---|
| MA7.1 | Amend the draft: lease policy, ceiling wording, provider calls not I3, impact table as locked data (BT1: every grant mapped to an impact class) | yellow / 0 | MA3.4 | the table has a row for every catalogue grant |
| MA7.2 | Evidence extraction E1 to E8 (settled sub-RLM casts of MA1.5 are E1 and E5 evidence for the parent and the child role) and the derived trust view per subject and class, never stored (BT2) | orange / 1 | MA1.3, MA3.5 | replay on d@e's settled records reproduces a view; excluded classes (crash-closed, self-fulfilling, same-publisher scorer) excluded |
| MA7.3 | Hierarchical prior and change discount from the behaviour-surface hash (BT3) | orange / 1 | MA2.3 | hash change moves `d` as specified |
| MA7.4 | Lease policy: duration and scope from the posterior's lower bound inside the ceiling; revocation by the breaker | red / 2 | MA3.4, MA7.2 | a strong-record fixture gets a durable lease that renews; a thin one stays per-task; a fidelity event revokes at once |
| MA7.5 | Install review pipeline with reviewer precommitments (BT5): mechanical checks, then seats sized by risk (Devin, DeepSeek; never luna) | orange / 1 | MA2.3 | a package with a widened ceiling cannot install without the verdict |
| MA7.6 | Library publisher reputation from signed settled outcomes (BT6) | orange / 1 | MA7.2 | the block shows sentences with n, not a score |
| MA7.7 | Cost attribution and anomaly notice (BT7) | orange / 1 | MA3.9 | an anomaly produces an owner notice |
| MA7.8 | **The act gate and circuit breaker** (BT4), last: material acts (I2 and up) outside autonomy go to a gate; the first fidelity event drops the class | red / 2 | all above | a planted-bad fixture package on d@e is gated within the measured time; replay AUC and tap rate reported (draft section 11 test) |

| MA7.9 | **Evidence for the paper.** A harness that, from the instrumented records, produces the pre-registered tests of MA0.6: (0) the mission's own precommitment ledger (every round's records, their settlements by rule, Devin or the owner, and the Brier, log and interval scores with calibration and sharpness), reported beside the desk records as evidence that the practice works when applied to ourselves; (a) replay of settled records on d@e and the arms, scoring whether the lower-bound ordering predicts the next contradicted or fidelity event (AUC) and how many owner taps per day the gate would have asked for; (b) baselines computed on the same data, a fixed allowlist (the grants alone) and a popularity signal (install counts), plus the success-rate and uncalibrated-rating baselines H4 names; (c) measured outcomes: lease durations and revocations, gate rates, time to gate; (d) red-team drills that plant known-bad packages on d@e only (never the paper or owner computers) and measure time to gate; every run writes a dated, reproducible artifact | orange / 1 | MA7.2, MA7.4, MA7.8 | the harness reproduces a stored result byte for byte from the stored inputs; drills write their artifact; results append to the pre-registration file whatever they show |

Instrumentation that earlier phases must already emit, so the harness has data: the golden record corpus (MA1.4), reducer-stamped provenance and `p_success` on every request (MA1.3), lease records and their use logs (MA3.4), label-stamped acts and disclosure decisions (MA3.5, MA3.6), the router's decision distribution and settlement (MA4.8), per-call model attribution (MA3.9), and the ledger rounds committed at each phase boundary (section 0 item 7). Missing oracles are recorded as such (the draft's `missing_oracle`: failures nobody settles never enter the posterior), not dropped.

The impact numbers, thresholds and decay are open (item 1 below).

## 14. Phase 8: resume self-development

Entry criteria (replacing modular gate G): Phases 1 to 7 landed on main;
MA4.11's names gate at zero; every system app and shell surface a package;
every desk a desk package; the leak differential and contract runner green on
CI; the package-only operation path (M3) run end to end on d@e; the hostile
fixture package (throws, over budget, asks for a privileged grant, writes
another app's kind, asks Mail for a second email) contained on d@e;
`GET /api/plugins/state` shows only the base image's default packages on a
fresh computer. Then the paused self-development goal resumes; Files and the
context-menu rounds run first as in-place updates with before and after
screenshots. Class: not applicable (a mission restart); the goal file is
repointed per the goal-station handoff rule.

## 15. Parallelism map

Rules: no phase starts before MA0.5. Within a phase, slices that share a file
serialize. Review runs in the background while the next slice builds.

| Phase | Run at once | Then | Hot-file collisions |
|---|---|---|---|
| 0 | MA0.1, MA0.2 harness; MA0.3 | MA0.4, MA0.5 | none |
| 1 | MA1.1 (alone first); the D0 tooling cherry-pick (extractor, checker, contract harness) can run beside MA1.1 | MA1.2, then MA1.3, then MA1.5, then MA1.4 | `deskmanifest/manifest.go`, `agentcore/inbox_records.go`, `rlm_reduce.go`, `yaegikernel/intent.go`, `choir.go`, `types/commitment.go`, `.github/workflows/ci.yml` |
| 2 | MA2.1, MA2.2, MA2.3, MA2.4 | MA2.5 after MA1.2 and MA2.1/MA2.3 | `check-packages.mjs` (MA2.3 then S2, S3, S5, S6, S7, U1), `ci.yml` (kernel gates, desk-names gate, desk-protocol job: serialize edits), `objectgraph/registry.go` (MA2.4 kind rename then MA3.1) |
| 3 | MA3.1, then MA3.2 and MA3.3 in sequence (shared `writer.go`, `sql_store.go`); MA3.8 beside MA3.3 after MA3.2; MA3.5 beside MA3.3 | MA3.4, MA3.6, MA3.7, MA3.9, MA3.10 | `objectgraph/{registry,writer}.go`, `store/**`, `agentcore/{tools_desk,tools_product_api,system1_verb}.go`, `rlm_reduce.go` |
| 4 | MA4.1, then MA4.2 to MA4.4 in sequence (shared `appdesk.go`, boot path), MA4.5 | MA4.6 and MA4.7 in parallel; MA4.8 beside them after MA3.6; MA4.9; MA4.10; MA4.11 | `deskmanifest/manifest.go`, `agentcore/{runtime,tool_profiles,tools_desk,inbox_records}.go`, `store/engineering_assignments.go` |
| 5 | S1; then S2, S3, S5, S6, S7, U1 in parallel; then W1; then S4; then C1 to C12 and H1 to H3 in parallel (disjoint directories); H4; the forced-modularity slices overlap the C batches (disjoint); L1, then L2 and L3, then L4; Texture last | | `Desktop.svelte`, `AppHost.svelte` (S1, W1, X1, then H1 to H4, then T1), `src/App.svelte` (S5 at `:391-402, :499-500` and S7 at `:236-296`: serialize), `apps/registry.ts` (row-local), `public-preview-data.ts`, `app-sdk.ts` and `sdk-surface/*` (one lock per minor, S1 owns), `check-packages.mjs`, `library-next` (M1/MA2.4, F1, L1 to L4 in order) |
| 6 | MA6.1 (front end) beside MA6.2 and MA6.3 (Go) | MA6.4, then MA6.5 | `desk_cell_confinement*.go`, `desk_worker_spawn_linux.go` |
| 7 | MA7.1, MA7.2, MA7.3, MA7.7 | MA7.4, MA7.5, MA7.6, then MA7.8 | |
| 8 | | | |

Cross-phase overlap that is safe: Phase 5's front-end services (S1 to S7, U1,
C batches) touch only front-end files and may start when Phase 2's front-end
kernel (MA2.2) and MA2.3 have landed, in parallel with Phases 3 and 4's Go
slices. The owner's roadmap order is the order of *completion*; the entry
conditions above are file and data dependencies, and the owner decides whether
to take the overlap. Phase 6's iframe slice depends on Phase 5's packages
being loadable as modules.

## 16. Review items folded in, by origin

| Item | Source | Where |
|---|---|---|
| B1 catalogue completeness | Devin desks | section 6, MA4.1 |
| B2 epoch-resolved frame check, session key | Devin desks | section 6, MA4.2 |
| B3 candidate kind grammar | Devin desks | section 6, MA4.3 |
| B4 required-set re-seed | Devin desks | section 6, MA4.4 |
| D0 roots incomplete; emit/update shapes; transition table against current behaviour | Devin desks | MA1.4 |
| Inventory undercount (586 in 104); missing mechanisms (`deskModuleSets`, Texture fold, `isTextureAgentID`, emit, dual spelling) | Devin desks | MA2.5, MA1.1, MA1.2, MA4.11 as a bound |
| Third-party `address` asymmetry | Devin desks | moot: no address grant (R5) |
| `app_packages.objects` defaults ON | Devin desks | MA4.5 |
| Stale counts and cites | Devin desks | section 9 note |
| Collisions and unassigned work, v1 reader lifetime, K0 drill, factual slips, inconsistencies | Devin system | MA2.2, MA2.3, MA5.1, section 11, section 15 |
| Red-team 1: install-time grant-composition flow check | red team | label flow rules (MA3.5/3.6), the Library block (L1); dual-authority exclusion as a hard manifest rule was **considered and not adopted** because it conflicts with R11 (trust, not zero trust) and R15 (labels propagate) |
| Red-team 2: executable content in `uses` | red team | behaviour-surface hash, MA2.3 |
| Red-team 3 and 5: broker as hostile input; confinement audit | red team | MA6.2, MA6.3 |
| Red-team 4: provenance stamping, authority-threshold acts | red team | MA1.3, MA3.5, MA7.8 |
| Red-team 6: wasm | red team | rejected by R9; the audit and sandboxes before any community desk become MA6.2 to MA6.5 and the section 12.1 gate |
| Red-team 7: no opaque binaries | red team | MA2.3 |
| Red-team 8: resource envelope, outbound ledger | red team | MA3.9 without any spend ceiling (R8); owner-visible outbound ledger from lease and frame records |
| Gateway monopoly: a package never supplies a provider, model or URL | red team | MA3.6; the local-first default routing is **not adopted** (R15: the VM chooses, no rule) |
| Standing-intent decay and throttling | red team | section 18 item 14 |
| Luna's findings | red team | considered as one seat's opinion; no review use of luna |

## 17. Docs to write once the design is committed

Terms: PSA (prospective self-alignment) is the theory and science;
precommitment records are the practice and engineering (R23). Two docs are
written after this plan is signed off (MA0.5), not before:

1. **A theory doc, PSA** (`docs/psa-theory-<date>.md`, extending
   `docs/precommitment-records.md` sections 1 and 3, or beside them). Contents:
   Bayesian trust from materiality-weighted track records; custody and leases
   as earned authority (ceiling in the manifest, leases within it, trust sets
   duration and scope); messages as predictions; generated code equals stranger
   code (one trust model); the hypotheses H4 and the trust conjecture and what
   would falsify them. This is the source the owner's paper starts from.
2. **A practice doc, precommitment records** (`docs/precommitment-records-practice-<date>.md`).
   Contents: the record schema (with the request fields of MA1.3), the
   inter-desk protocol and its lock (MA1.4), leases (MA3.4), the disclosure
   path (MA3.6), the trust view (MA7.2) and the act gate (MA7.8), with the
   engineering of each (files, invariants, failure modes, how to extend it).
   The existing `docs/precommitment-records.md` keeps its role until the
   practice doc replaces its engineering sections; the split of its text is
   decided when the docs are written.

Slice **MA8.0** (green, tier 0): write the two docs and repoint
`docs/ACTIVE.md` and the goal file; deps MA0.5; the practice doc is revised at
each phase that changes the protocol (a protocol bump is not complete until its
section is). The owner writes the paper at external launch from the theory doc
and the artifacts of MA7.9.

## 18. Open questions

Only what remains after the rulings (R19, R24, R25, R26, R27 closed the earlier
items on research search, community desks before Phase 6, the router and the
Go-module scope), plus what the plan found.

1. **Trust parameters.** The impact table (I0 to I4 = 0, 1, 3, 10, 30), the
   thresholds tau (0.80, 0.95, 0.99), the severity multipliers (1, 5, 20), the
   decay (0.995 per day), the prior shrinkage and the change-discount range
   (0.1 to 0.9) are the draft's starting numbers and policy. With leases: what
   lower bound earns a durable lease, how long durable is, and how scope
   widens. Also the draft's questions: does the owner see the trust view or only
   its effects; cross-computer aggregation on or off by default.
2. **gVisor decision criteria and sandbox count.** Measured: about 1.8x warm
   cell, about 2x on allocation-heavy work, +20% compute, 22 to 60 microsecond
   verb IPC, about 30 ms start. For the owner to set: the accepted overhead
   ratio; the resident memory per sandbox ceiling (from MA0.1); **how many
   sandboxes: one per desk, or shared per trust class** (waits for the running
   memory experiment); whether gVisor wraps all desk workers or only those of
   published origin; the layering outcome of MA0.2 (hypothesis: if Landlock
   cannot run inside gVisor, the "required" mode in
   `desk_cell_confinement.go:38-60` needs a new definition); and the fallback
   when the platform is unavailable (KVM is off on Node B because nested=0;
   development hosts without x86 systrap).
3. **The typed request vocabulary.** Which request kinds exist beyond cast,
   ask, report, reply, escalate, precommit, resolve and disagreement
   (`lease_request`, `open_subject`, `decline`, `feed`, `inform`?); what
   replaces Note's information-only use; who sets materiality weight (the
   sender proposes, recipient and scorers judge: confirm); whether all requests
   are scored or only material ones.
4. **Custodian shape and System 1** (owner questions, answered as
   recommendations): the custodian is the owning app desk, not a separate desk
   and not management; System 1 is the Jev judge generalised to a kernel
   service. Confirm.
5. **Engineering casts from non-management desks.** Recommended: forwarded to
   management's admission ledger. Alternative: any desk may open an assignment
   if it holds `admit.assignment`.
6. **Where the Texture desk moves.** The roadmap lists Texture's app last in
   Phase 5 and the desks in Phase 4; this plan moves Texture's desk with its
   app in Phase 5 (app and desk never land separately), so MA4.11's names gate
   keeps a Texture residue until then.
7. **Provider calls and the trust draft.** The draft lists a provider call
   carrying owner-private data as I3. R15 says no rule constrains models. Plan:
   remove it and log. Confirm.
8. **Interplay reading, front-end packages before the iframe.** Section 12.1
   reads R16 (publishing open) and R24 (no third-party front-end package reads
   routes before the iframe) as: publishing open, data-only installs open,
   code-bearing installs from others refused until MA6.1. Confirm this reading.
9. **Third-party Go modules in desk code (R26).** Allowed in principle, since
   generated code is no different from third-party code. The owner flags the
   tension: importing a million lines of unreviewed code is a huge
   vulnerability surface. Open, not decided: the gating of such imports and
   which desks may import them (engineering, maybe others). Considerations:
   - *Platform-built and pinned.* Source only, vendored and built by the
     platform at a pinned version and digest, no opaque binaries (MA2.3), with
     the module digest in the behaviour-surface hash.
   - *Review depth versus line count.* A review sized by lines of imported code
     is impractical for large modules; sizing by the capability the module can
     reach (its import set, which stays inside the desk's derived set) is
     cheaper but weaker. Whether a size bound or a transitive-dependency bound
     applies.
   - *Trust record per module publisher.* The trust draft's hierarchy extends to
     module publishers: a module is a subject with a track record and a
     change discount.
   - *Curated set versus open.* A curated allowlist of modules reviewed once for
     everyone, or any module reviewed per install.
   - *Containment independent of review.* The module runs inside the same
     sandbox and under the same per-desk import set as the cell; an import
     outside the desk's set is refused at the static check whatever the module
     is.
10. **Out-of-process tool servers** (the backend protocol of modular section
    1). What sandbox they get: gVisor like desk workers, or a separate VM as an
    earlier ruling suggested. Not needed for Phases 1 to 5.
11. **Planned-restart trays.** MA1.1 deletes the retired message intent whose
    branches exist for a pre-change tray that "still replays". Verify on the
    tape that no planned-restart marker has carried one.
12. **Published v1 manifests.** MA2.3 deletes the v1 reader with no shim. The
    exposure is any release published before P0 becoming uninstallable. Check
    the listing count at build time; if non-zero, republish them on their
    tracks or accept the loss (pre-release).
13. **System 1 caps and switch.** `system1_verb.go:38-40, 56` make System 1 off
    until configured, with per-run and per-day caps. Under R8 and "on, never
    off", MA3.6 removes both. Confirm no ceiling is wanted for the judge, given
    that the router calls it on every submission.
14. **Standing-intent throttling** for packages with no owner interaction (red
    team): a bound on schedule frequency is in MA3.9; the decay after seven
    days idle is not adopted without the owner.
15. **Unread by this author.** The `d0-protocol` branch was inspected by file
    list and commit subjects only; `desk_cell_confinement_linux.go` and the
    desk-worker spawn were not read in full; the desk-design file lists for
    D6 to D9 are unverified at this commit and must be re-grepped by each
    slice's first step; the desktop focus and input signals the router's
    settlement rule needs are a hypothesis until MA4.8's first step.
16. **Who scores sub-RLM Casts (R28).** The owner is unsure: management, the
    parent itself, or a self-score that is in turn scored. **Recommended:**
    layered scoring, as in section 4.4: rules first, then a blind System 1
    judge as the default independent scorer, then downstream settlement, with
    the parent's self-score as a learning signal whose gap against the
    independent score is tracked (the scored self-score), and management only
    for sampled material disagreements. Parameters to set in the Phase 1 ledger
    round: the fraction of weight-zero records scored as an audit, the sampling
    rate and materiality weighting for management's third opinion, and the
    judge-call budget per fan-out (batched; one call per fan-out, N questions).
    Also open: the judge's per-computer rate limit (about 60 calls a minute,
    shared with intake labelling and support judging) against fan-outs of many
    children.
