# Lifecycle Simplification — Agentic Consensus (2026-08-29)

Panel run `.agentic-consensus/agentic-consensus-20260829-141131/` — 11/11 ok
(codex, cursor, opencode, **claude**, gpt-5.6-sol, gpt-5.6-luna, gemini-3.7,
grok-4.6, muse-spark, nemotron-3-ultra, hy3). Prompt:
`.agentic-consensus/prompt-lifecycles-simplification-2026-08-29.md`. Mode:
convergent. Grounding: four read-only scouts (lifecycle inventory, RLM
architecture + web research on original RLM/Prime Agent, account isolation,
dead-code sweep) at main `a9178b76`.

## Panel corrections to the scout inventory (verified by panelists)

- `internal/yaegikernel` is **not** wholly dead: `cmd/capsule-broker` imports
  `ExecuteWorkerStdin`/`DefaultSafeStdlibPackages` (packaged in flake/nix VM).
  Only the in-memory actor sub-stack (`actor_state.go`, `broker.go`,
  `broker_protocol.go`, `profiles.go`, `handles.go`) is orphaned. Delete the
  sub-stack, keep the evaluator. (claude, grok, luna, muse — overriding the
  scout's "zero importers" claim.)
- The P0 pending-forever path is **confirmed in code**, not just inferred:
  occurrence identity is content-hashed with no epoch/generation
  (`adapter.go:216`), the actor returns `nil` on `!appended`
  (`actor/actor.go:229-234`), and the boot migration **discards the `inserted`
  flag** (`adapter.go:372`) while the run has already been set pending
  (`runtime.go:2338`) — an unguarded path where a dropped duplicate occurrence
  leaves the run pending forever. (claude; consistent with grok/sol/luna.)
- "Zero-caller export" ≠ dead: `RegisterCapsuleTools` and
  `RegisterTerminalRoutes` are registration functions — built-but-unwired, the
  exact false-complete pattern this repo has been burned by twice. Per-symbol
  triage mandatory; no bulk-106 deletion. (claude, cursor, opencode, sol.)
- `computerversion.SQLInputCatalog` (PinCode/PinArtifactProgram/RouteAuthority)
  is live — the cross-computer gap is candidate-bundle import + target-owner
  acceptance, not the route input catalog. (AccountIsolation scout addendum.)
- `docs/computer-ontology.md:194-220` already names this seam: published
  package/change is a typed sharing artifact, but import/adoption that
  materializes routes is a distinct, currently missing authority.

## Consensus

### C1 — Lifecycle authority consolidation: ADOPT (unanimous, keep two stores)

SQLite (actor log) owns consumption: `processed_at`, snapshots, mailbox
dedup. Dolt owns semantic state: runs, packets, assignments, dispositions.
Merging couples the provider loop to OLTP and rewrites recovery. The real
defect is **predicate drift**: `Terminal()` excludes passivated globally, the
selfdev wake treats passivated+blocked as replaceable
(`api_self_development.go:1187-1192`), boot texture uses `Active()`, and
`!Terminal()` is used as "resumable" in places. Fixes (all panelists):
a named predicate family — `Terminal()` (work verdict), `Active()` (live
slot), `Replaced()` (`Terminal()||passivated`) for wake/remint policy — one
owner package, ban `!Terminal()` as resumable; generation-stamped occurrence
identity so replay and a genuinely new wake are distinguishable; a standing
detector for `pending ∧ processed ∧ !Replaced` (the dual-authority deadlock).

### C2 — RLM: not built today; delete the duplicate now; build after candidate A

Ground truth (scout + panel verification): the original RLM (arXiv
2512.24601; reference `alexzhang13/rlm`; Prime Intellect prime-agent) requires
the prompt/context bound as a REPL variable the model programmatically
inspects/slices, plus recursive sub-LLM calls from within the REPL. Choir has
neither: yaegi is a per-call `interp.New` sandboxed eval tool
(`GoEvalRequest{Source,Cwd,AllowedPackages,TimeoutMS}`), context is harness
prompt-stuffing from durable run-memory entries, and the doctrine memo's
model→Go-cell→observation loop with `models.Call/Parallel/Map` is unbuilt.
The standalone `actor_state`/`broker` duplicate is a false-complete (the
"accepted/deployed 53f80af4" now-card is contradicted by zero production
callers).

Verdict: **defer the real RLM until after candidate A pays**; delete the
duplicate sub-stack now; when built, make it the primary surface behind
parity acceptance — persistent **per activation** (prompt/context prebound,
model-authored cells, recursive model modules), with **typed continuations +
artifact refs across wakes**, never a serialized interpreter heap (sol/luna,
matching the doctrine memo's disposable-activation model).

### C3 — Clean-account development: ADOPT NOW; cross-owner install mechanism: ADOPT design, DEFER build

Unanimous on the operating pattern: primary development moves to
**new@new.com** (historically provisioned, own computer, no mail/legacy
state); `a@b.com` has a registered passkey but historical boot incidents;
**yusefnathanson/0333528 is rejected as first install target by every
panelist** (sealed, host hold, 132k-event tape). Sol/cursor/luna: prove the
publish→install loop on a **second disposable clean owner** before ever
touching the main account.

Honest gap (claude, confirmed by all): until the mechanism exists, tested
code still reaches the main account via `git push` → CI → platform deploy —
the product loop is not closed, and the operating doc should say so plainly.

The mechanism, when built (post-candidate-A): one **signed candidate-package
manifest + target-issued install capability** — frozen bundle + content-
addressed artifacts + receipts under source owner; target verifies digests,
CodeClosure/ArtifactProgram, route freshness, then stages into its local
`updater incoming` and reuses the existing decision/materializer path.
Reject: File-CAS generalization (drags computer-local DEK escrow into
red/black territory), general package registry, residue import as transport.

### C4 — Deletion: two waves, per-symbol triage (no blanket 106)

Safe now (pre-A, disjoint from the execution path): yaegikernel actor
sub-stack; `internal/base` API + `cmd/baseobserve`/`cmd/baseharness` —
**keeping** `blob/journal/model/tree` (computerversion reads them — grok
correction vs gemini's whole-orphan delete); unused adapter options
(`WithInboxCapacity/WithSendTimeout/WithBackpressure/WithOnActorFailure`);
`RegisterCapsuleTools` + its unreachable legacy constructors;
`capsule/diagnostics.go`; `store/json.go`; `qdrant_dedup.go` (then reassess
`qdrant_runtime.go` wiring); trivial dead wrappers after test migration
(`continuations.go`, `cosuper_assignment_seed.go`).

Needs caller-migration first (post-A): texture helper exports, dead lifecycle
query wrappers, `ListCapsules` stub, raw terminal shell path (proxy already
410s it), VM enum convergence (`vmmanager` → `vmctl.VMState`, red).

Unknown actor kinds: unanimous — the current silent mark-processed is a bug,
but a hard error would poison-loop the FIFO backlog; make it a **durable
dead-letter/reject disposition + mark processed** (sol/grok/claude/luna).

### C5 — Packet envelope: REJECT full retirement this cycle (majority)

`actor.Update` is the transport and carries `initial_dispatch`/`cancel` which
are not Coagent packets; ChannelMessage is still live in Texture prompt
history (grok), not pure audit. Two-step adaptation: typed
`CoagentSourcePacket` becomes the sole semantic payload for `coagent_result`
(killing the field duplication inside opaque content); ChannelMessage/
ChannelCast producers demote to event-sourced audit only after every durable
consumer migrates. The consumption identity fix (C1's `OccurrenceKey` with
generation) is the unifying piece, not envelope removal.

### C6 — Sequencing (unanimous shape)

1. **P0**: capture the live actor SQLite row → problem receipt → recovery-
   generation occurrence identity fix (recovery-scoped, not global — a global
   change risks double delivery against the live mailbox) + the two missing
   tests → deployed proof the Super executes.
2. **Candidate A** on the clean-account path (effects OFF), immediately after
   P0; no RLM/packet/VM/package semantics change in the window.
3. **Parallel with 1–2**: Wave-1 deletions (disjoint files), registry
   reconciliation (ACTIVE.md/NOW.md/Overhauls Track F false-complete),
   candidate-package threat/contract design (paper only).
4. **Post-A**: Wave-2 deletions, cross-owner candidate-package build (prove on
   two clean owners first), packet semantic-payload unification, VM enum
   convergence.
5. **Last**: real RLM as primary surface behind parity acceptance.

## Dissent / Disagreements

- C1 single predicate vs predicate family: cursor rejects one boolean
  (`IsTerminalForScope`) as re-bricking; sol/luna accept one named remint
  predicate plus the family. Resolution: family, with each member named and
  owned.
- C4 `internal/base` scope: gemini "delete ~4k LOC whole subsystem" vs grok
  "API+cmds only, keep blob/journal/model/tree." Grok's is grounded in the
  computerversion read path — adopted.
- C3 timing: gemini/opencode defer even the design; sol/luna/cursor allow
  contract design in parallel. Adopted: paper design parallel, build post-A.

## Raw Outputs

- `/Users/wiz/go-choir/.agentic-consensus/agentic-consensus-20260829-141131/`
  (11 ok; a first attempt with `--include claude` alone ran one agent at
  `...-140833` — claude's solo verdict there contributed the yaegikernel-live
  and P0-confirmed corrections).
- Scout briefings: lifecycle inventory, RLM architecture (+ web research:
  arXiv 2512.24601, alexzhang13/rlm, PrimeIntellect prime-agent), account
  isolation, dead-code sweep (106 zero-caller exports; whole-file candidates
  `capsule/diagnostics.go`, `store/json.go`, `qdrant_dedup.go`).
