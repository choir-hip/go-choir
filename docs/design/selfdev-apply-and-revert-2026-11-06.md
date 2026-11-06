# Self-development applies on automated acceptance; the owner reverts

Date: 2026-11-06, revision r2. Status: design, for build. Mutation class: red
(self-development decision authority, run acceptance, owner-act surface).
Mission: [modular architecture](../definitions/choir-modular-architecture-2026-11-04.md).
Review: r0 SEND BACK from both seats (Devin; Sonnet second seat). r1 answers
every finding (section 9); Devin r1 ACCEPT with six build-time precision
items, folded in as r2 (section 9, r1 items).

## 0. Owner rulings this design carries out

- 2026-11-06: "the point is to revert rather than approve".
- 2026-11-06: "auto apply and revert? should apply everywhere for self-dev. i
  guess autopaper no, but its not doing self-dev (currently)".
- 2026-11-04: never block on human approval where it can be avoided; clean
  rebases apply without an owner act (55e80f612).
- 2026-11-03, "No switches": computers diverge by what they run and adopt,
  never by a platform flag.

These supersede the 2026-11-03 ruling that standing consensus is "refused on
primary-tier computers". Platform computers (the paper) stay refused
(section 3.3).

## 1. What exists today (origin/main 7bc0dfbd8)

- **Six modes**: `off`, `audit_only`, `propose_only`, `accept_once`,
  `qualified_consensus`, `standing_consensus`
  (`internal/platform/self_development_modes.go:20-31`). Stored by corpusd,
  signed (ModeReceipt), generation CAS, written only by an owner act. A
  computer with no row reads `off` (`:136-137`). No creation site writes a
  mode: the owner sets each computer's mode by `choir self-dev mode set`
  (`cmd/choir/commands.go:1204-1229`). `audit_only` refuses at freeze exactly
  like `off` (`internal/agentcore/freeze_candidate.go:133-141`).
- **Most computers park.** Under `propose_only` an operation that passes
  management's Verify stops at `awaiting_approval` and waits for an owner
  `approve` (`api_self_development.go:564,661`). Nothing times it out:
  `selfdev-ccf792fc` on d@e has waited since 2026-11-03. A crash boot
  deliberately leaves `awaiting_approval` alone
  (`self_development_materializer.go:109-111,125,185-191`).
- **The automatic path works but is fenced.** After Verify,
  `triggerStandingConsensus` (`rlm_reduce.go:1874`, the only call site) runs
  `runStandingConsensus` (`self_development_standing_consensus.go:140`): the
  host builds the candidate, then corpusd's `ConsensusActuator.Decide` mints a
  decision under policy `reversible-selfdev-v1` (no human seat). It applied
  `selfdev-152fe036` on 2026-11-03. The arm that admits it needs an owner act
  (`selfdev.autonomy.arm`), a test-tier computer, not a platform computer,
  and an expiry within 24h (`standing_consensus.go:128-145,225-244`). The arm
  also feeds the decision's policy digest, `not_after` cap, the decisions
  table's generation key, the admission gate's input artifact 0 and
  generation check (`:47-91,326,589-651`), and the guest verifier's expiry
  check (`self_development_standing_consensus.go:337-415`).
- **What the votes check, honestly.** The reviewer ballot is signed
  unconditionally by the guest's own core key (`:338-357`); the poster pins
  that key on first use. The only check independent of the guest is the host
  build of the source patch on the booted base (`standing_consensus.go:487-534`).
  Under the arm, an owner act, a test tier and a 24h window bounded that.
- **Revert works and is CLI-only.** `choir self-dev rollback --to-operation
  <op>` is an owner act (`commands.go:661-675`, `cliOnly`); the first owner
  rollback settled `rolled_back` in under 15s (`rollback-fe946ac2`,
  2026-10-27). The route accepts any prior `applied` or `rolled_back` target
  (`api_self_development.go:875,915-933`) and refuses a genesis or
  platform-update prior (`:918-926`). The CLI builds seven bindings from
  several reads and keys idempotency `cli-rollback:<target>:<latest landed>`
  (`cmd/choir/selfdev.go:340,370-415`). `frontend/src/` has no rollback
  control; the Texture operation embed is a read-only GET
  (`texture-embeds.ts:202-204`). No rollback of a consensus-applied operation
  is recorded.
- **Pre-existing**: the guest appends `rollback_requested` with no
  corpusd-side owner check (`owner_acts.go:366-385` gates only
  `effect_accepted`), so a compromised guest can already roll itself back.

## 2. The foundational question

What does the owner's approval buy? Judgment before use, on a change the
platform cannot judge. What the platform can check before use is one thing:
the patch builds on the booted base. Management's Verify ran inside the guest
and its ballot is the guest's own signature, so it is a record, not an
independent check. What the owner uniquely holds is judgment after use, and a
revert is cheap because every apply leaves its prior checkpoint.

So the ruling trades a pre-use gate the owner rarely exercised (candidates
parked for days) for a post-use revert one tap away. The bound on harm is
this: a consensus apply changes only that computer's own guest, which the
guest already controls; nothing crosses to another computer except by
publish and adopt, which are separate acts. The design states that bound and
does not pretend there are two independent checks.

## 3. Design

### 3.1 Two modes: `off` and `apply`

- The stored modes collapse to **`off`** (this computer does no
  self-development) and **`apply`** (propose, verify, host build, decide by
  consensus, apply). `off` remains because the owner choosing what their
  computer does is the divergence the no-switches rule allows.
- **No row decodes to `apply`.** Every computer self-develops unless its
  owner wrote `off`, per the owner's "everywhere". Platform computers are
  refused regardless of mode (3.3).
- **One normaliser**, `platform.CanonicalSelfDevelopmentMode(stored string)
  string`: `off` and `audit_only` give `off`; `propose_only`, `accept_once`,
  `qualified_consensus`, `standing_consensus` and no row give `apply`.
  Rows are not rewritten. Every reader of a mode goes through it, including:
  - `readSelfDevelopmentMode` and `Get` (before any expiry logic; the lazy
    expiry is deleted, so an expired standing arm reads `apply`, not `off`);
  - `clean_rebase.go:96-113`, which reads the raw column in SQL: read the
    raw value and normalise in Go; the allowlist becomes `== apply`;
  - `verifyDecisionModeReceipt` (`api_self_development.go:756-799`): the
    receipt's `new_mode` is normalised before it is compared, so a legacy
    `propose_only` receipt matches `apply`;
  - `freeze_candidate.go:133-141`, `api_self_development.go:121,457-545,770,781`,
    `runStandingConsensus` (`:146`), the CLI's `mode` display
    (`commands.go:704`).
- **Writes**: `validateSelfDevelopmentModeTransition` accepts `off` and
  `apply` only, with no bindings. `OwnerActKindForMode` maps both to
  `ActSelfDevModeSet`.
- **Deleted**: `audit_only`, `propose_only`, `accept_once`,
  `qualified_consensus` and `standing_consensus` as writable values;
  `lazyExpiringMode` and `ModeWriterCorpusdExpiry`; `guestConsumptionAllowed`
  and `ConsumeAsGuest` (so the guest can write no mode at all); the
  consumption key prefixes; `standingArmRefusal`'s owner-act and tier tests;
  `ActSelfDevAutonomyArm` (historic `owner_act_receipts` rows naming it stay
  as replay records); `StandingConsensusMaxWindow`; the proxy's tier logic
  in `mintModeOwnerAct` (`proxy/self_development.go:319-332`); the CLI arm
  flow and the deleted modes' bindings (`commands.go:634-645,1152-1172,1209-1228`);
  the owner `approve` decision (the CLI verb and the proxy's approve
  acceptance at `proxy/self_development.go:100-102,517-523`). The owner
  `reject` stays for a candidate not yet decided; its receipt check admits
  `apply` (`api_self_development.go:756-799`).
- `scripts/reeng_tracer.mjs:1452-1481` stops arming `propose_only`.

### 3.2 The poster and the admission gate without the arm

The mode row is the authority the arm was.

- **Policy** is a constant, `reversible-selfdev-v1`; its digest no longer
  comes from the row.
- **Generation** stays the revocation: a decision records the mode's
  generation, and the admission gate refuses it if the generation changed.
  An owner writing `off` therefore revokes every decision in flight. A
  no-row computer has generation 0.
- **Input artifact 0** is the mode receipt's digest, or for a no-row computer
  `artifact:sha256:<D0>` where `D0 = sha256("selfdev-mode:default-apply:0")`
  (a 64-hex constant, since the gate compares `ArtifactRef.Digest()`,
  `computerevent/artifact_ref.go:10-27`). The decisions table's
  `standing_digest` column holds the same 64-hex value.
- **`not_after`** is `MaxBallotTTL` from minting, with no arm cap. The stale
  `fields["computer_tier"]` stamp (`standing_consensus.go:328-329`) goes.
- The gate's `!found || mode.Generation != row.generation`
  (`standing_consensus.go:641`) becomes: no row reads generation 0 and
  canonical `apply`; refuse only when the generation differs or the canonical
  mode is not `apply`.
- `standingArmFromMode` becomes `applyAuthorityFromMode`: canonical mode is
  `apply`, the computer is not a platform computer, and (if a row exists) the
  receipt verifies. The admission gate (`consensusDecisionRefusal`,
  `standing_consensus.go:589-651`) keeps every other check: computer,
  operation, bundle, heads and commitments, expiry, single admission, the
  decision cited.
- The guest's `verifyStandingConsensusDecision` drops the mode `ExpiresAt`
  parse (`:372-375`) and keeps the decision's own `not_after` check
  (`:392-395`); it checks canonical `apply` and the generation.
- **No-row on the guest side.** Three guards assume a receipt and must take
  "no row" as canonical `apply`, generation 0, no receipt:
  `runStandingConsensus` (`self_development_standing_consensus.go:149-151`),
  `decideSelfDevelopment` (`api_self_development.go:445-448`, before the mode
  switch), and `verifyStandingConsensusDecision`'s
  `currentMode.Receipt.CanonicalBytes()` (`:384`, a nil dereference today).
  `standingConsensusRequest` signs the ballot with the constant policy
  digest, not `mode.PolicyDigest` (`:338`).
- The owner `reject` needs a mode receipt today (`api_self_development.go:445,556,756`);
  on a no-row computer it takes the same no-row rule. With decisions taking
  one host build, reject is rarely reachable; that is accepted.

### 3.3 Platform computers refused in corpusd

The paper does no self-development (owner, 2026-11-06). The refusal is in
corpusd, the side of the trust boundary the guest cannot skip:

- `Decide` refuses a platform computer (`platformComputer()`,
  `standing_consensus.go:120-126`, using corpusd's own tier file, not a
  guest's embedded copy).
- `consensusDecisionRefusal` refuses one too.
- The guest's freeze check also refuses, for an early and readable reason:
  "a platform computer does not self-develop (owner ruling 2026-11-06)".

Opening the paper later is an owner ruling and a one-line change.

### 3.4 Nothing parks: every outcome closes

`awaiting_approval` now lasts one host build. Every exit from the attempt
that does not decide closes the operation `failed` (the existing
`awaiting_approval -> failed` edge, `selfdev/operations.go:707`) with the
reason on the operation (`RecordIntentError`) and in Texture. No new state.

| attempt outcome | today | r1 |
| --- | --- | --- |
| poster approves | decide approve, apply | same |
| host build failed (dissent), including `base_moved` | `rejected` with reason | same |
| poster refuses: no source patch or base commit, no job, bundle does not decode, package paths, platform computer, mode `off` | parks forever | `failed`, the refusal as reason |
| decision names another host build | parks | `failed` |
| head contention exhausts `standingConsensusAttempts` | parks | `failed`, "decision attempt exhausted" |
| attempt budget exceeded | parks | `failed` |

A closed candidate keeps its bundle and kept ref; management may propose
again on the current base, which re-runs Verify.

Package-path bundles close with the poster's existing refusal until D7's
package decision lands (forced-modularity-build M3 item 1). Plain-release
bundles with no source patch close too: the host cannot build them, so
nothing independent of the guest would have checked them.

**Boots.** Every boot, crash or planned, closes each `awaiting_approval`
operation with no finalized decision `failed`, reason "interrupted by a
restart before its decision". It first asks
`FinalizedDecisionForOperation` (as
`markSelfDevelopmentOperationsInterruptedAfterCrash` does,
`self_development_materializer.go:185-193`): an operation whose decision is
on the tape but not projected goes through the existing settlement path, not
this close. On a planned boot, a decided operation keeps today's path,
`recoverSelfDevelopmentDecision` (`self_development_materializer.go:525-560`),
and applies; the crash mark (`markSelfDevelopmentOperationsInterruptedAfterCrash`)
stays crash-only. There is no resumer, so no loop. This same rule is the first-boot
sweep: the deploy that carries r1 boots every computer, and every parked
candidate (`selfdev-ccf792fc` first, any parked operation on the paper too)
closes. The comments at `self_development_materializer.go:109` and
`self_development_standing_consensus.go:16` change with it.

### 3.5 Revert in reach

- **A server-side revert.** `POST /api/computers/<id>/self-development/operations/<X>/revert`,
  an owner act from the signed-in session (the proxy already requires the
  cookie session for rollback, `proxy/owner_acts.go:25-40`). The guest
  assembles the seven bindings itself from its current head, so a stale tap
  cannot fail on bindings the browser read earlier. Idempotency key
  `revert:<X>:<latest landed operation>`, the CLI's shape, so a failed revert
  can be retried after the head moves and a second revert after later applies
  is a new act, while a double tap is one act.
- **Target.** Reverting X restores the state before X. If X's prior is an
  applied or rolled-back operation, that is the existing rollback to it. If
  X's prior is a genesis or platform-update checkpoint, which is the case for
  every computer's first apply and for the first apply after every deploy,
  revert restores the booted base release with no self-development overlay.
  The route refuses this today (`api_self_development.go:918-926`) because
  the prior has no operation, verifier refs or verification evidence to
  record the rollback against (`self_development_materializer.go:986-989,1030-1035,1121-1141`).
  The swap itself is proven: the M3 boot migration puts `current` back on the
  booted baseline under the floor check (`servingBaselineManifest`,
  `StageBaselineRelease`, `RestagePinnedRelease`, `recordStartupManifest`;
  `package_migration.go:690-749,1086-1093`, `rematerialize.go:594-613`).
  What R3 designs is the evidence: the rollback operation for a base target
  carries the booted base manifest digest as its target and the reverted
  operation as its subject, and its applied tail binds the baseline release,
  not a verifier certificate. This is the hard part of R3; proof step 4
  is required.
- **Texture.** Every consensus apply writes an entry on the candidate's
  Texture document: the operation's summary and paths, the decision digest,
  and a **Revert** action calling the route above. Reverting an operation
  that is not the latest says, before the tap, that later applies are undone
  with it (the route already accepts any prior target).
- The CLI rollback verb stays. The revert route is also an MCP tool if the
  owner-act tools are exposed there; rollback today is CLI-only and this
  design does not change that.

### 3.6 Harm, and what bounds it

| failure | answer |
| --- | --- |
| a passing candidate breaks the product | owner reverts from Texture |
| a prompt-injected or compromised desk proposes a harmful patch that builds | applies to that computer only; owner reverts; this is the bound the ruling accepts (section 2) |
| a harmful apply disables the guest's own rollback | residual R7 below; today the code-rollback ring guard and a crash boot are the only outside levers |
| an apply fails mid-way | existing: materializer marks it failed, Texture notified |
| a decision on a stale base | host build dissents `base_moved`; `rejected` |
| owner writes `off` mid-attempt | generation changes; admission refuses; the attempt closes `failed` |
| the platform has frequent apply defects (2026-11-03 blockers) | each is an operation record with its reason; a cluster is a problem doc, not a reason to park |

## 4. Slices

| slice | content | class | review |
| --- | --- | --- | --- |
| R1 | modes and authority (3.1, 3.2, 3.3): normaliser, two modes, arm deleted, poster and admission gate on the mode row, corpusd platform refusal, clean-rebase allowlist, CLI and proxy deletions; security-epoch bump | red | tier 2 |
| R2 | nothing parks (3.4): close on every attempt outcome; boot close with the finalized-decision check | red | tier 2 |
| R3 | revert (3.5): server-side revert route, base restore for a first apply, Texture entry and action | red | tier 2 (base restore is new boot-path behaviour) |
| R4 | d@e proof (section 5) | | |
| R5 later | automatic revert when a post-apply health check fails | red | tier 2 |
| R6 later | the host runs the candidate's declared test command in its build, so the host ballot checks behaviour, not only that it builds | red | tier 2 |
| R7 later | a host-side revert (vmctl pins the prior release) for a guest that will not roll back | red | tier 2 |

R1 and R2 are `internal/platform` and `internal/agentcore`, one builder in
sequence. R3 is the guest route, the proxy route and the frontend, a second
builder in parallel.

## 5. Proof

Unit, over the real store: the normaliser on every legacy value and no row;
a decision on a legacy `propose_only` row's receipt; a no-row computer
decides end to end, guest and corpusd (the three guest guards, artifact 0,
the gate's generation 0); corpusd `Decide` and the admission gate refuse a platform computer;
owner `off` mid-attempt revokes; each refused attempt outcome closes
`failed`; the boot close skips a decided-but-unprojected operation; a second
revert to the same target after a later apply is accepted.

On d@e, after the deploy:

1. d@e's mode reads `apply` (its row says `propose_only`).
2. `selfdev-ccf792fc` is `failed`, "interrupted by a restart before its
   decision".
3. A self-development round passes Verify, the host builds, the poster
   decides, the operation reaches `applied` with no owner act. Record the
   decision digest.
4. This is d@e's first apply since the deploy, so its prior is the platform
   checkpoint: Revert from Texture in a signed-in session restores the booted
   base and settles `rolled_back`. A second tap starts nothing.
5. A second round applies; a third applies; Revert on the second (not the
   latest) undoes both, with the warning shown.
6. A reboot keeps the reverted state.

Then the release walks the rings like any other.

## 6. Rollback of this design

Revert the R commits. Stored mode rows were never rewritten, so the old code
reads them as before. Operations closed `failed` stay closed; their bundles
and kept refs remain. Computers whose owner never wrote a row return to `off`.

## 7. Heresy delta

- discovered: owner approval parks candidates until stale
  (`selfdev-ccf792fc`, three days); the autonomy arm was a per-computer
  switch in effect; an unbuildable candidate parks forever under the
  standing path; the reviewer ballot is self-attestation.
- introduced: none intended; the platform refusal is a named authority rule.
- repaired: the approval queue; revert reachable from the product.

## 8. Open items (named, not blocking)

- How long Texture keeps a Revert action live (while the target's checkpoint
  exists, by default).
- When the paper opens to self-development: an owner ruling.

## 9. Review adjudication (r0)

Both seats SEND BACK; findings adopted unless noted.

- No creation site writes a mode (both, blocking): adopted; no row decodes to
  `apply` (3.1).
- `superseded` does not exist (both, blocking): adopted; closes use the
  existing `failed` edge (3.4).
- Reviewer ballot is self-attestation (Sonnet blocking, Devin non-blocking):
  adopted; stated in sections 1, 2 and 3.6, with R6 and R7 named as the
  upgrades. Not built now: the owner ruled revert over pre-use gating.
- Platform refusal guest-side only (both, blocking): adopted; corpusd
  `Decide` and admission (3.3).
- Arm dependencies in the poster and gate (both, blocking): adopted (3.2).
- Legacy receipt `new_mode` mismatch and mode-string comparisons (both,
  blocking): adopted; one normaliser, enumerated readers (3.1).
- Clean-rebase allowlist (Devin, blocking): adopted (3.1).
- Unbuildable candidates park forever (Devin, blocking): adopted; the
  outcome table (3.4).
- Sweep could close a decided operation (Sonnet, blocking): adopted;
  finalized-decision check first (3.4).
- First apply cannot be reverted (Sonnet, blocking): adopted; base restore
  in R3 and proof step 4.
- Target-only idempotency (both): adopted; target plus latest landed (3.5).
- Crash-close is new behaviour; no resumer exists (both): adopted; no
  resumer, close on every boot (3.4).
- `audit_only` description wrong (Devin): adopted; deleted, decodes to `off`.
- Contention is not staleness (Sonnet): adopted; contention closes as
  "attempt exhausted", staleness is the host's `base_moved` dissent.
- Owner `off` mid-attempt parks (Devin): adopted; closes `failed`.
- Server-side bindings (Sonnet): adopted (3.5).
- Hold durations 2h and 6h (Sonnet): noted; holds unchanged.
- MCP rollback is CLI-only (Sonnet): adopted (3.5).
- Proof gaps (both): adopted (section 5).

Devin r1 (ACCEPT) precision items, folded in as r2:

- N1 artifact 0 must be a 64-hex digest: adopted (3.2).
- N2 three guest-side nil-receipt guards and the ballot's policy digest:
  adopted (3.2) and a guest-side no-row proof (section 5).
- N3 the gate's `!found` clause: adopted (3.2).
- N4 the crash mark stays crash-only; planned boots keep
  `recoverSelfDevelopmentDecision`: adopted (3.4).
- N5 base restore's evidence synthesis named: adopted (3.5).
- N6 keep the decision's `not_after` check; reject on no-row stated; the
  stale tier stamp removed: adopted (3.2).
