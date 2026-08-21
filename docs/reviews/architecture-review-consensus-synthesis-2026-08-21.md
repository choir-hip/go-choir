# Consensus Panel Synthesis: Architecture Review v2

Date: 2026-08-21
Panel: 8 models (codex, devin, cursor, opencode, gpt-5.6-sol, gpt-5.6-luna, gemini-3.7, grok-4.6); deepseek failed
Source: .agentic-consensus/architecture-review-v2-20260821/panel/

## Verdict tally

| Model | Verdict |
|---|---|
| codex | NOT-READY |
| gpt-5.6-sol | NOT-READY |
| gpt-5.6-luna | NOT-READY |
| devin | READY-WITH-CHANGES |
| cursor | READY-WITH-CHANGES |
| gemini-3.7 | READY-WITH-CHANGES |
| grok-4.6 | READY-WITH-CHANGES |
| opencode | (no explicit verdict; ran out mid-analysis) |

Split: 3 NOT-READY / 4 READY-WITH-CHANGES / 1 void. The NOT-READY voters agree with the direction; they object that specific findings are factually wrong or under-specified and would become doctrine if §5 executed as written.

## Convergent findings (all or most models agree)

### C1. My revision-cadence diagnosis was WRONG (blocker — codex, luna, sol, gemini)
`texture_turn_committed` is a lifecycle event emitted for **every** turn outcome — revision, wait, block, no-change. Only `TextureTurnRevision` outcomes create a version and advance the head (`texture_turn.go:452-510`). The ~250-events-vs-v2 gap I reported is not a bug: those were wait/control turns that correctly preserved the head. The real selfdev defect is different: the texture join path projects a synthetic deterministic Texture run and commits `TextureTurnWait` directly (`selfdev_texture_join.go:386`), bypassing genuine Texture-agent authoring turns entirely. **Action: retract the "version-stall detector" framing from the mission sequence; replace with an evidence receipt counting turn outcomes and revisions separately before any cadence repair.**

### C2. Single-writer vs owner edits is unresolved (blocker — cursor, luna, codex)
Doctrine today says owner edits create immediate `AuthorUser` canonical versions (I2b, current-architecture:456, direct API path). My §0 said all inputs arrive as updates Texture incorporates — which would delete the existing owner-authored-head mechanism. That's a new heresy either way it lands. **Decision needed from you:** (a) owner edit = typed correction instruction, Texture incorporates into next revision (pure single-writer), or (b) two writer classes: `AuthorUser` (owner, immediate head CAS) + `AuthorAppAgent` (Texture). Doctrine supports (b) today; your statement supports (a).

### C3. FIFO across trajectories is undefined and unsafe as stated (blocker — luna, sol, gemini)
`ReducerSeq` is trajectory-local — there is no computer-global arrival order to implement FIFO against. Worse, head-of-line blocking: a hung trajectory starves every other one forever, and an owner correction on doc A must tombstone A's queued requests or Super executes them after finishing B. **Action: add durable computer-scoped enqueue ordinal; per-request execution deadlines; owner-instruction settlement of stale requests; FIFO within trajectory + explicit arbitration across trajectories (round-robin or priority).**

### C4. Memory overcommit policy is physically wrong without swap (blocker — codex, gemini, grok)
With swap=0 (current Firecracker config), `cgroup.freeze` stops CPU but does not reclaim anonymous memory — a paused capsule holds its RSS, so PSI-pause cannot relieve pressure. Also my table was internally inconsistent: admission at 1.5× while allowing each capsule 2× means aggregate limits of 3× physical. **Action options: enable zram/swap in the guest kernel, OR drop the pause/resume tier and treat requested memory as a scheduling weight with retryable admission refusal (work stays pending, never deadlocks), OR both.** Codex's framing is cleanest: separate concurrency admission from per-capsule containment; candidate A needs neither PSI scheduling nor overcommit — it needs the one-live-slot invariant plus retryable refusal.

### C5. Mission supersession mechanics unauthorized (blocker — codex)
A review document cannot supersede an owner-ratified Definition. Also the live authority is candidate-proof-2026-08-20 (not effects-2026-08-11, already superseded); registries (ACTIVE.md, mission-graph.yaml, doc-authority-manifest.yaml) need atomic update, and ACTIVE.md still promises an email leg the compact definition dropped. **Action: successor definition requires your ratification + registry hygiene in the same commit series.**

### C6. Smaller convergents
- Singleton Super rationale must not claim event-chain arbitration authority (codex): justify by coordination/scheduling only.
- "Any actor's authoritative update" wording conflicts with I1 — inputs may trigger/wake Texture turns but never authorize revisions (luna).
- texture-live-supervision-architecture.md §§1/3/4 contain more normative pipeline claims than just §2 — all need rewrite, not just the table demotion (codex, luna).
- N-capsule generalization touches bundle provenance contracts, not just ontology notes (codex).
- Before candidate A resumes: resolve the CoSuper 200-tool-loop budget exhaustion and decide whether checkpoint 99949fe2 remains valid after steps 2–4 (gemini) or re-baseline.

## Divergences

- Severity calibration: NOT-READY camp treats C1–C4 as doctrine-blocking because §5 would execute them; R-W-C camp treats them as fixable-before-definition details. Substantively they agree on what's wrong.
- Memory end-state: codex/sol lean "scheduling weight + retryable refusal, defer PSI entirely"; gemini/grok lean "keep graduated tiers but require swap/zram." Not contradictory — a sequencing question.
- gemini uniquely flags storage/export cost of hundreds of self-contained snapshots; nobody else disputes but nobody else raises it.

## My synthesis recommendation

The panel converged on: direction correct, four specific findings must be fixed before this becomes mission authority. Proposed v3 changes:

1. Retract cadence-bug claim; replace mission step 2 with "evidence receipt: turn-outcome census → then decide whether any repair is needed."
2. Ask you to decide the owner-edit question (C2) — this is the one genuinely yours to make.
3. Rewrite FIFO as: enqueue ordinal (computer-scoped) + per-trajectory FIFO + cross-trajectory arbitration + deadlines + owner-settlement.
4. Replace memory table with codex's minimal form: one-live-slot invariant + memory.high/max split + retryable admission refusal; explicitly defer PSI pause/resume and zram to a later mission.
5. Drop review-as-supersession-authority; v3 becomes input to a successor Definition you ratify, with registry updates bundled.
