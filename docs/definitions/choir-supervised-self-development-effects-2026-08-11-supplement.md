# Supplement — Supervised Self-Development with Effects

**Status:** non-executable supplement to
[choir-supervised-self-development-effects-2026-08-11.md](choir-supervised-self-development-effects-2026-08-11.md).
**Mutation class:** green (documentation only).
**Purpose:** carry the reasoning, retired approaches, research findings, and
panel records that shaped the Definition, so the Definition itself stays
executable. Nothing here is a work item. If this document and the Definition
disagree, the Definition wins.

---

## 1. Retired: model policy as the content axis

The first draft made computer-scoped model policy the content of the first
effect, on the theory that it was the smallest real change to the computer's
working state. Two findings retired it.

**It had no path to its target.** The materializer's only apply surface stages
the bundle's runtime files into the updater's release tree, swaps the `current`
pointer, restarts, and health-probes. Model policy is read from a separate files
root on every activation. A model-policy bundle would have produced a complete
green trajectory — staged, promoted, restarted, health-probed, signed — while
changing nothing about resolution. A fully green proof of a change that did not
happen is the exact failure the Definition exists to prevent.

*Evidence:* `internal/agentcore/self_development_materializer.go:199`;
`internal/updater/updater.go:145-188`;
`internal/modelpolicy/model_policy.go:89-118`;
`internal/provideriface/config.go:95-101`.

**The surface is system-owned and already scheduled for replacement.** Promoted
direction holds that model selection, fallback, and cycling are system-owned
processes rather than agent-edited configuration, and `internal/modelpolicy` is
slated for generalization into broker-mediated multi-call selection, with live
overlays retired once the eval substrate lands. Teaching the computer to own
that surface first would have been a step in a direction already decided against.

*Evidence:* `docs/memo-persistent-rlm-actors-2026-08-09.md` (integration table,
`internal/modelpolicy` row; Multi-Model Execution);
`docs/memo-live-retrospective-evals-2026-08-09.md` (overlays are not
counterfactual isolation).

**Why source code instead.** The frozen `CapsuleEffectBundle` cannot validate
without a source tree ref, a capsule-executed build recipe, a runtime artifact
ref, test receipts, and dependency toolchain refs. The envelope was designed
around one story: a capsule took a source tree, ran a recorded build with a
recorded toolchain, ran tests, and produced a runtime artifact. Configuration
would have required stubbing five fields whose purpose is to attest to a build.
The updater's restart-and-health-probe loop is likewise a binary-deployment
safety loop that a configuration file exercises not at all.

*Evidence:* `internal/capsule/transaction/builder.go:95-111`.

---

## 2. Corrected: authority is effect-specific consensus, not reversibility

Per-candidate owner approval is encoded across the substrate — the decision
binding strips an `external-owner:` prefix from the decision event and fails
without it, and `accept_once` binds exact owner presence. It is a fail-closed
pre-consensus gate, not the intended product authority. The intent is an
automatic computer: one input, autonomous work for weeks, durable consequence
receipts, and correction without continuous human operation.

The first 2026-08-11 correction removed universal owner approval but made a
second category error: it treated reversibility as the boundary of autonomy and
routed irreversible effects to a separate human decision. The owner's
2026-08-13 correction supersedes that inference. Reversibility governs recovery;
effect-specific multiagent consensus governs authority. Send, publish, pay, and
third-party writes remain inside the autonomy window when a stronger policy
authorizes their exact subject. A human is a policy-selected participant, not a
universal gate.

Most existing machinery still survives: exact head and state commitments,
joins across operation, verifier, bundle, and heads, expiry, and idempotency are
transaction invariants. The replacement must add a versioned policy reference,
frozen eligible-seat and independence manifests, quorum, dissent,
abstention/timeout/recusal/replacement semantics, actuator constraints, and
consequence receipts. Preserve the fail-closed current gate until that complete
policy-bound replacement passes deployed acceptance.

*Evidence:* `internal/agentcore/self_development_decision_binding.go:30-33`;
`internal/platform/self_development_modes.go:226-242`;
`docs/problems/irreversible-effects-human-gate-drift-2026-08-13.md`;
`docs/choir-vision.md`.

---

## 3. Why rollback today is not reversibility

`ComputerVersion` — what a checkpoint pins — is `CodeRef` plus
`ArtifactProgramRef`. Code and artifacts, no state. State commitments exist on
the head, but `DoltStateExtractor` is read-only by explicit design ("does NOT
commit, branch", "does not mutate the database") and `ProjectionMaterializer` is
marked non-runtime. Together they prove state changed and cannot put it back.
`rollbackSelfDevelopmentOperation` re-applies a prior release, so rollback means
reinstalling the previous binary while every row written under it remains.

Three stores call `DOLT_COMMIT` and nothing in production calls checkout,
branch, or reset — a complete history is written and never read back. Those
commits carry a free-text message with no binding to an event head, so even the
history that exists cannot be indexed by the point one would want to return to.

The substrate is favourable despite that: Dolt is already a versioned database
committing on every write, and the extractor's per-table schema and content
hashes are exactly what a post-revert re-extraction must reproduce — a drift
observer that becomes an acceptance test for return.

*Evidence:* `internal/computerversion/types.go:40-48`;
`internal/computerversion/dolt_state_extractor.go:27-83`;
`internal/computerversion/projection_materializer.go:9-22`;
`internal/agentcore/self_development_materializer.go:224-273`;
`internal/platform/store.go:481-499`; `internal/store/store.go:119`;
`internal/cycle/storage.go:56`.

---

## 4. Agentic consensus: multi-Dolt revert consistency

**Panel:** divergent 7/8 (omp-cursor-grok45 failed, no API key), convergent 8/8
(codex, cursor, opencode, gpt-5.6-sol, gpt-5.6-terra, gemini, deepseek, devin).
**Evidence:** `.agentic-consensus/dolt-revert-consistency-2026-08-11/{divergent,convergent}/`.

The panel retired the question as posed. "Do the computer's Dolt databases
revert atomically or in a proven-consistent order" was wrong in three ways.

**The restore set is much smaller than assumed.** A user-computer revert must not
move the shared platform store — that is service state, and rewinding it would
rewind other people's computers. Cycle state is out until proven to be private
computer behavior, which today it is not. What remains is the release pointer
and one VM-local embedded workspace, at which point the multi-database problem
mostly dissolves. This corrected a real scope error in the Definition draft,
which had said "every Dolt database together."

**The address is the event head, not a database commit.** The event chain is the
semantic authority and VM-local Dolt is a projection, so the natural restore is
to rebuild the projection through a target head rather than check out a
database. The VM-local Dolt HEAD is still bound into the checkpoint as an audit
witness joined to that head — never as restore authority.

**"Atomic" means an acceptance fence, not a distributed transaction.** Stage,
verify with the extractor, flip visibility only on exact match. Partial never
greens. Physical step order is mechanical once the fence exists.

**Where the panel split.** A majority (sol, terra, cursor, codex, devin,
opencode hybrid) favored rematerialization from the event head — rebuild
VM-local rows from the immutable chain and receipts, with the Dolt commit as
witness rather than restore operand. A minority (deepseek, gemini) favored
checking out the single VM-local pin. Ontology sides with rematerialization:
reconstruction verifies the event chain and deterministically rebuilds embedded
state, events are never deleted, and VM-local Dolt is a projection rather than
an alternate head authority.

**Dissent retained.** DeepSeek's objection is fair and is carried as an
implementation contingency, not an authority redesign: `ProjectionMaterializer`
is explicitly non-runtime today, so rematerialization has no runtime path yet,
while pin checkout is implementable against existing Dolt state. If step 2's
probe or step 4's build shows rematerialization is not product-ready, restore
may use a single-workspace pin checkout on an interim basis while the event head
remains the sole semantic authority.

**The surviving question.** Not atomicity — replay completeness. Is every
behavior-bearing VM-local write a deterministic function of the event chain plus
pinned receipts? If yes, rebuilding from the tape reproduces state exactly. If
not, some rows exist only in the database, replay silently loses them, and the
checkpoint must fail closed until those writes are event-derived. This is
empirical: rematerialize current state, diff against a live extractor reading,
and the answer is the diff.

---

## 5. Why CoSuper was isolated, and what reconnection must preserve

Persistent Super decides whether a packet is a work order by reading
`packet.kind` — a field the sending model writes. Any actor holding
`update_coagent` can therefore set `kind=execution_request`, address it to Super,
and open privileged execution on its own supervisor. Sender identity is honest
(the delivery envelope is runtime-derived, and the tool description states that
target and envelope authority must not appear in model arguments), so this is
not forgery. It is authority derived from a model-supplied label instead of from
the sender's authorization.

CoSuper is the actor most exposed to content that could steer it — it runs in a
capsule chewing on source, test output, and whatever the work drags in — and the
lowest-privilege actor in the chain. Denying it the tool closed the hole by
amputating the channel, and `TestSurvivorContract_GenericCoSuperCannotAuthorPersistentSuperPackets`
pins that amputation.

The repair relocates the decision rather than restoring the old wiring:
executability derives from whether the sender is authorized to open execution,
and `kind` describes content without granting authority. This is what the
promoted invariants already say — free text cannot grant authority, and
receiving a message cannot expand the recipient's capabilities. The pinned test
is replaced by a stronger assertion (a CoSuper packet declaring
`kind=execution_request` must not open Super execution) rather than deleted,
because the property is worth keeping even though the mechanism was blunt.

Reconnection stays minimal because the RLM/Yaegi rebase is expected to rewrite
this layer; a general authorization framework built now would be built to be
deleted.

*Evidence:* `internal/agentcore/super_controller.go:784-796`;
`internal/agentcore/tools_worker_update.go:46`;
`internal/textureowner/texture_turn_runtime.go:110`;
`internal/agentcore/tool_profiles.go:309-315,386`;
`internal/agentcore/cosuper_assignment_tools_overlay.go:57-64`;
`internal/agentprofile/agentprofile.go:52-65`;
`internal/agentcore/update_coagent_survivor_contract_test.go:193-199`.

---

## 6. Supervision: no observation subsystem is needed

An earlier draft recorded Texture's lack of any `selfdev` reference as a
supervision gap and called wiring it the mission's largest new build. That was
wrong, inferred from a missing grep hit. Supervision flows upward through the
path that already exists: the capsule reports to its CoSuper, CoSuper to Super
via `update_coagent`, Super wakes Texture, Texture incorporates the update into
an immutable canonical revision. Texture never needs to know what a
self-development operation is.

The real risk is narrower and has a structural home. Identity must ride the
revision's metadata blob and its typed source citations, never the prose. A
revision that recites operation ids and bundle digests at the owner is worse for
human supervision, not better; a revision whose metadata omits them cannot be
joined to the authority it describes. Texture already separates these layers —
revisions carry a metadata JSON blob with durable carry-forward keys, and typed
coagent packet sources become `textureSourceEntity` citations — so the work is
populating those layers, not inventing them.

CTS separately observed that Texture's production registry omits
`update_coagent` despite its profile allowing coagent tools. Whether that gap is
still present on the deployed build is an open check.

*Evidence:* `internal/agentcore/tools_worker_update.go:176`;
`internal/textureowner/texture.go:2303`;
`internal/agentcore/super_controller.go:1632`;
`internal/textureowner/texture_revision_metadata.go:20-60`;
`internal/textureowner/texture_evidence_sources.go`.

---

## 7. Research findings that set scope

**The capsule can build.** The capsule lower layer is the whole guest root
(`CHOIR_CAPSULE_LOWER_ROOT=/`), the platform source is a git repo snapshotted by
commit into the capsule (`CHOIR_CAPSULE_SOURCE_ROOT=/mnt/persistent/files/Source/platform`,
`copyImmutableCommitTree`), and the runtime PATH and systemPackages carry go,
gcc, git, gnumake, nodejs, pkg-config, and icu, with `GOPATH`/`GOMODCACHE`/
`GOCACHE` on the persistent volume, `GOTOOLCHAIN=local`, and `CGO_CFLAGS`/
`PKG_CONFIG_PATH` set for Dolt's ICU dependency. The guest was provisioned for
exactly this.
*Evidence:* `nix/autoputer-vm.nix:675-700,717-745,777-798`;
`internal/autoputer/capsule_executor_linux.go:14-56`;
`internal/capsule/executor.go:113-136,224-266`.

**The UI cannot ship in the effect.** Caddy on Node B serves `/` and `/assets/*`
from `/var/www/go-choir/frontend-current` — a host directory, outside the guest
and outside the updater-controlled release. A UI change inside a bundle would
land where the browser never reads. Solitaire is API-only until frontend serving
moves inside the computer's own release, which is a separate mission.
*Evidence:* `nix/node-b.nix:23-24,161,193-207`.

**Freeze and propose are wired to nothing.** An assigned CoSuper's registry is
built fresh by `buildAssignedCoSuperRegistry` → `RegisterCapsuleLocalTools` and
contains exactly five tools: `capsule_exec`, `capsule_read_file`,
`capsule_write_file`, `capsule_list_dir`, `record_assignment_result`. The
freeze/propose/verify tools live in `registerHostSelfDevelopmentTools`, which
has no production call site — only a test. `RegisterCapsuleTools` (spawn,
destroy, list, inspect) also has no production call site. The source comment is
explicit: "No host profile registry wires this future surface today."
*Evidence:* `internal/agentcore/tool_profiles.go:309-315,386`;
`internal/agentcore/tools_capsule.go:61-102`.

**Supersession cannot be an operation state.** The selfdev state machine is
strictly linear per operation and terminal at `rolled_back`/`failed`/`degraded`;
no transition links two operations. "B supersedes A" is therefore carried by
ordering plus the proposal event's prompt artifact. The decision verifier admits
exactly one input artifact ref (the mode receipt) and exactly one verifier ref,
so the supersession citation must ride B's *proposal* event, not its decision
event. The convention is writable today with no schema change; an enforced
`supersedes` field would be a schema change on a red surface.
*Evidence:* `internal/selfdev/operations.go:423-446`;
`internal/agentcore/self_development_decision_binding.go:45,53`.

**Schema changes are additive-only and survive revert of code alone.** There is
no migration framework; stores create tables with `CREATE TABLE IF NOT EXISTS`
at startup, so an additive table is created by a new binary and ignored by the
prior binary. This is why a code-only rollback leaves rows behind, and why the
mission's revert must move state as well.
*Evidence:* `internal/platform/computer_events.go:18,39,63`.

---

## 8. Corrections made during authoring

Recorded so the reasoning trail is inspectable rather than tidied away.

1. **"Texture cannot observe self-development" was wrong** — inferred from a
   missing grep hit; supervision already flows upward through `update_coagent`.
   Retracted in section 6.
2. **"The complete authoring surface exists" was too strong** — the tools exist
   as code, but the live wiring is deliberately absent with source comments
   saying so. Corrected in section 7.
3. **"Revert every Dolt database together" was a scope error** — it would have
   swept in the shared platform store and rewound other computers. Corrected by
   the panel in section 4.
4. **The fixed two-seat conclusion was under-specified and is now superseded** —
   Texture and Super do hold meaningfully different contexts, but role labels do
   not by themselves prove independence and no one pair is the universal
   authority. Each effect policy must predeclare eligible seats, independence
   domains, quorum, dissent, failure, recusal, and replacement semantics.

---

## 9. Pre-mission haunted-authority cutover (2026-08-11)

Lateral consensus
(`.agentic-consensus/mission-success-lateral-2026-08-11/`) found that waiting
until after success to demote competing constitutions would regenerate the
pre-envelope world mid-mission. Pre-mission (green) changes landed:

- Roadmap demoted to historical migration receipt; ACTIVE points at this
  Definition's restore and effect-policy proof, not D1/`accept_once` sequencing;
  Mission 0 and Invocation demoted so ACTIVE owns no second schedule.
- `docs/mission-graph.yaml` active-node note rewritten to replay probe + disposed
  Mission 0 (no longer schedules headed-browser then E2 as the live path).
- Ontology restore-set boundary named (VM-local + release IN; platform/cycle/
  host frontend OUT); ComputerVersion clarified as code identity not full restore.
- Agent-product doctrine: model policy is config, not self-dev content axis;
  effects OFF is pre-gate, not destination.
- Doctrine Product Path: policy-relative permission after gates; restore
  vocabulary preferred for reversible product undo. The 2026-08-13 correction
  further replaces the envelope-as-authority model with effect-specific
  consensus across reversible and irreversible effects.
- AGENTS Landing Loop: product restore ≠ failed deploy / git revert; runtime-
  bearing `completion_cutover` items re-enter Landing Loop before `goal.complete`.
- `SEM-03` transitional wording is superseded by decision-policy / qualified
  consensus / restore; full runtime cutover stays in completion obligations.
- Definition YAML made subset-parser safe (`skills/definition` dashboard).

Post-success obligations remain in `finish.completion_cutover` (doctrine
promote, lexicon, registries, detectors, owner surfaces, ops identity,
frontend ownership, successor preconditions, residual risks).
