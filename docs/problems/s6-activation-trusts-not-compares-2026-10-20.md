# S6: activation may trust joins it does not compare (2026-10-20)

Status: **problem record, hypotheses from code reading, not trace-confirmed.**
Every claim below was read from the code at `ec2dc7b7d`; none has been
confirmed against a staging ledger. Per CLAUDE.md "Review the traces before
attributing behavior", each is marked
**hypothesis (code reading, not trace-confirmed)** until the S6-0 witness is
fetched on a minted test computer (the go-served drill) and agrees.
Mutation class of this document: **green**. No code is changed by it.

Station: [choir-lp-s6-commit-gate-2026-10-20.md](../definitions/choir-lp-s6-commit-gate-2026-10-20.md),
conjecture S6-C1 ("activation today is by trust, not comparison: at least one
of declared test exit, host-build closure against the approved patch, expected
heads at apply is recorded but never compared"). Reviewed station definition:
[choir-appdev-s6-commit-gate-full-release-2026-10-01.md](../archive/definitions/choir-appdev-s6-commit-gate-full-release-2026-10-01.md).

## Summary

The goal file states three gaps. Reading the code, S6-C1 is **partly
supported and partly narrower than stated**:

| Gap as stated in the goal file | What the code reading shows |
|---|---|
| (a) the declared test's exit status is recorded but not compared at the gate | **Half wrong.** The exit status IS compared, but at Freeze (`freezeReceiptOutcomesError`), not at decision admission or the materializer. `rlm_reduce.go:152-155` (the line the goal file cites) checks only that receipts exist, but the freeze body that the intent calls refuses a nonzero or non-done receipt before a bundle is formed. What is genuinely absent is a *declared* test: nothing records which command is the test, so any successful execution receipt (a build, `true`, a cell that did not panic) satisfies the slot. And after Freeze nothing re-reads the receipt. |
| (b) the host-build receipt/closure is not compared to the approved patch | **Mostly wrong.** The guest compares the host's signed job result to the request (operation, bundle digest, patch sha, base commit, base image) with `hostbuild.VerifyJob` before staging, and the host compares the builder's patch/base to the job's. What is only recorded: the *content* of `builder-receipt.json` (which patch and commit it names) is hash-pinned but never parsed by the guest or the updater, and the manifest's `BuilderReceiptDigest` field is a by-construction copy. |
| (c) expected heads / divergence are not derived and compared at apply | **Split.** Expected heads ARE compared at the decision (live head against the request, plus an append CAS) and are held at apply by the reducer's pending-transition lock. Not compared: the surface the candidate was frozen against against the surface the decision expected. Divergence (platform release against a computer's own release) is not derived at all; that is slice S6-4 and the vmctl `DivergenceStatus` flag. |

So S6-C1 stands as "at least one join is recorded but not compared", and the
joins that are recorded-only are named below. It does **not** stand as stated
for the declared test's exit status or the host-build patch binding. Slices
S6-1 and S6-2 must be re-scoped from the witness gap table, not from the goal
file's three sentences. This correction is itself a hypothesis until the
witness confirms it on staging.

Definition of "gate" used here: any code path that can refuse before the
release is activated: decision admission (`decideSelfDevelopment` and
`verifyFinalizedSelfDevelopmentDecision`), the materializer
(`materializeSelfDevelopmentOperation`), the host-build verification, the
reducer, and the updater's apply. Freeze is upstream of all of these and is
reported separately as "compared at freeze".

## Gap (a): the declared test's exit status

**Hypothesis (code reading, not trace-confirmed).**

Where it is recorded:

- The bundle (`internal/capsule/transaction/builder.go:38`) holds
  `TestReceipts []string`, reference strings `capsule-exec:sha256:...` or
  `capsule-go-eval:sha256:...`. The bundle digest covers the references.
  The exit code is **not** in the bundle; it lives in the execution receipt
  the capsule executor stores on disk (`capsule/executor.go:1112`,
  `OpenExecutionReceipt`, under `artifactRoot()/receipts/execution`).
- A job receipt carries `ExitCode`, `JobState` and the start/end worktree
  digests (`capsule/job_receipt.go:60-110`).

Where it is compared:

- `internal/agentcore/rlm_reduce.go:152-155`: the Freeze intent requires only
  non-empty `BuildRecipeRef`, `TestReceipts`, `DependencyToolchainRefs`.
- `internal/agentcore/rlm_reduce.go:951` and `:1405-1420`: the freeze intent
  commits through `freezeCapsuleEffectBundle`.
- `internal/agentcore/tools_capsule.go:251` calls `freezeReceiptOutcomesError`
  (`:765`), which refuses a receipt with a nonzero exit, a job not `done`, or
  (build and test slots) a workspace that changed while the job ran
  (`capsule/job_receipt.go:130-140`, `JobReceiptOutcomeRefusal`). Added in
  `e8f9688df` (2026-10-14). The refusal text is
  `a cited execution receipt records an execution that did not succeed`.
- A nonzero exit also settles a say-do precommitment "no"
  (`types/commitment_rules.go:283-298`, fed from
  `engineering_precommit_rules.go:204`). That settles a record; it refuses
  nothing.

Where it is not compared (the refusal that would fire if it were):

- `verifyFinalizedSelfDevelopmentDecision` (`self_development_decision_binding.go`)
  joins operation identity, bundle digest, verifier ref, authority, receipt
  and heads. It never opens a test receipt.
- `materializeSelfDevelopmentOperation` (`self_development_materializer.go:315`)
  checks the bundle digest and `Validate(true)` (which checks that the receipt
  references are well formed, `builder.go:113`), not the exit codes.
- `releaseDraft` (`api_release.go:103`) records `Declarations.TestCommand` from
  a query parameter, unverified.

Consequence (hypothesis): leg 3 as written ("a declared test that exits
nonzero in the frozen bundle, then attempted approval and activation") is
**unreachable through the product path**, because Freeze refuses to form that
bundle. The deployed negative proof for leg 3 should assert the Freeze
refusal on the tape/operation record (`recordOperationIntentError`,
`rlm_reduce.go:1434-1439`), and a *post-freeze* mutation (receipt file
replaced or removed) is the only way to exercise a gate-side compare. Two
further consequences: the test is not *declared* (any success satisfies the
slot), and the receipt bytes the freeze read are a guest-local file, so a
gate that wants the exit status must re-read a store the tape holds only by
digest.

Refusal that would fire at the gate today: none. At freeze:
`freezeFailedReceipts`.

## Gap (b): the host build against the approved patch

**Hypothesis (code reading, not trace-confirmed).**

Where it is recorded:

- The bundle holds `SourcePatchSHA256` and `SourcePatchBaseCommit`
  (`builder.go:55-57`), both inside the digest the owner (or standing
  consensus) approves. The patch bytes sit in `incoming/<bundle>/source.patch`.
- The host build is requested after acceptance, only when the bundle touches
  Go input or a consensus decision accepted it
  (`self_development_materializer.go:343-347`).
- The job result (`hostbuild/types.go:123-145`) names operation, bundle
  digest, patch sha, base commit, base image digest, code commit, closure
  digest, builder receipt digest.
- The layered manifest copies those fields (`layeredSelfDevelopmentManifest`,
  `self_development_host_build.go:357-368`). The decision event and
  `MaterializationStarted` carry the bundle digest only
  (`self_development_materializer.go:374-385`).

Where it is compared:

- Guest, before staging and before every reuse of a staged build:
  `hostbuild.VerifyJob` (`self_development_host_build.go:180, 284`;
  `hostbuild/service.go:632-659`) refuses a result that does not bind this
  request's operation, bundle digest, patch sha, base commit and base image,
  and verifies the host's signed receipt. The patch handed to the host is read
  through `readBoundSourcePatch` (`:134`), which refuses bytes that do not
  hash to the bundle's `SourcePatchSHA256`.
- Host: `hostbuild/service.go:310` refuses a patch that does not hash to the
  declared sha; `:574` refuses a builder that built a different patch or base;
  `:577` refuses a moved base image.
- Guest: `fetchVerifiedBlob` (`self_development_host_build.go:253`) re-hashes
  `closure.nar` and `builder-receipt.json` against the signed result.
- Updater: `closure.go:448` compares `closure.nar` to `manifest.ClosureDigest`;
  `closure.go:505` compares the binary's own build commit to
  `manifest.CodeCommit` before the pointer swap; `updater/runtime.go:193-196`
  compares the served release digest, marker, schema, reducer and (layered)
  build commit after the swap; `updater.go:911` requires the manifest's
  accepted head to equal the request's.

Where it is only recorded (the residual):

- `builder-receipt.json`'s content, which names `source_patch_sha256`,
  `source_patch_base_commit` and `code_commit` (`builder/builder.go:67-70`),
  is hash-pinned and copied into the release but never parsed by the guest or
  the updater. The binding of the closure to the approved patch rests on the
  host's signed job result and the host's own compare (`service.go:574`), not
  on a guest-side compare of the receipt.
- `manifest.BuilderReceiptDigest` is carried through the mint
  (`updater/updater.go:89-92`); only the file entry for `builder-receipt.json`
  is hashed.
- The tree of `CodeCommit` against `PatchedTree(base, patch)` is not compared
  on the guest (the host admits the patched tree, `hostbuild/types.go:225`).
- The owner approves the patch and bundle digest, not the closure: the
  closure is bound afterwards. Under standing consensus the closure is what
  corpusd votes on (`platform/standing_consensus.go:417, 531`), so there it is
  approved.

Refusal that would fire at the gate today for a different patch: `host build
failed (receipt_refused)` (`VerifyJob`). For a builder receipt that names a
different patch than the job: none on the guest.

## Gap (c): expected heads at apply, and divergence

**Hypothesis (code reading, not trace-confirmed).**

Where it is recorded:

- The decision request carries `ExpectedDesiredEventHead`,
  `ExpectedEffectiveEventHead`, `ExpectedPendingTransitionRef` and both state
  commitments (`api_self_development.go:40-44` (the request type)); the decision event carries
  them (`:509-516`).
- The operation row stores `BaseHead` (canonical head at start) and, at start,
  the desired/effective surface pin; both desired/effective columns are
  overwritten at decision with the decision's resulting heads
  (`api_self_development.go:582`, `self_development_materializer.go:287`).
- Vmctl holds a manual `DivergenceStatus` (`internal/vmctl/ownership.go:139-151`);
  CI writes `divergence_status: tracking` into every offer.

Where it is compared:

- At decision: `decideSelfDevelopment` refuses `decision head binding changed`
  when the live desired/effective/pending/commitments differ from the request
  (`api_self_development.go:491-496`), and the append is a CAS
  (`RequireExpectedHead`, `:516`, `computerevent/appender.go:640`). In
  accept_once and qualified_consensus modes the armed mode's expected heads
  must equal the request's (`:385-392, :403-410`).
- At apply: the reducer requires `materialization_started` and
  `materialization_applied` to bind the pending transition equal to the
  decision (`computerevent/reducer.go:92-98`), and refuses a second accepted
  effect or research update while one is pending (`reducer.go:84, :112`). So desired and
  effective cannot move between decision and apply.

Where it is not compared:

- The surface the candidate was frozen against (the start pin, bundle
  `BaseEventHead`) against the surface the decision expected. The decision
  checks the request against the live head and never against the candidate's
  pin (`grep` of `operation.DesiredHead` in `api_self_development.go` finds no
  read before decision). A candidate frozen before another operation applied
  can be approved if the card re-reads the live head. Not derivable after the
  fact, because the pin is overwritten at decision; the witness reports this
  pair as unknown, not as bound.
- The applied event's `ResultingEffectiveCommitment` is taken from the head's
  own desired commitment (`self_development_materializer.go:592`), so the
  reducer compare at `reducer.go:96-98` proves chain integrity, not content.
- There is no `divergent_release` refusal anywhere. Whether the computer's
  effective release carries an owner-approved patch is not derived from the
  tape; vmctl's flag is manual and default-unsafe. Slice S6-4.

Refusal that would fire at the gate today: `decision head binding changed`
(request against live head); `ErrPendingTransition` (second effect while one
is pending). For a stale candidate pin or a diverged release: none.

## Gap table

`compared at gate` means a code path can refuse before activation (see the
definition above). `compared at freeze` is reported separately.

| Pair (witness id) | Recorded where | Compared at gate | Code ref | Refusal that fires |
|---|---|---|---|---|
| `decision_operation`: decision event against operation (identity, bundle digest, verifier, authority, receipt, heads) | decision event on the tape; operation row | yes | `self_development_decision_binding.go` `verifyFinalizedSelfDevelopmentDecision` | `decision binding: ... mismatch` |
| `bundle_operation`: frozen bundle file against operation | `incoming/<digest>/bundle.json` | yes | `self_development_materializer.go:318-326` | `materializer: frozen bundle unavailable` / `invalid frozen bundle` |
| `test_receipts_exit`: test receipts' exit status against the bundle's cited refs | executor receipt store (guest disk); refs in bundle | **no** at gate; **yes** at freeze | `tools_capsule.go:251,765` (freeze only) | at freeze: `freezeFailedReceipts`; at gate: none |
| `bundle_source_patch`: source.patch bytes against bundle `SourcePatchSHA256` | `incoming/<digest>/source.patch` | yes (host-build path) | `api_self_development_patch.go:132`, `self_development_host_build.go:134` | `host build failed (patch_unavailable)` |
| `bundle_host_build`: host job result against operation, bundle, patch, base, base image | `incoming/host-build-<job>/host-build.json` | yes | `self_development_host_build.go:180,284`; `hostbuild/service.go:632` | `host build failed (receipt_refused)` |
| `builder_receipt_patch`: `builder-receipt.json` content against the approved patch | `incoming/host-build-<job>/builder-receipt.json` | **no** (guest hashes the file, never parses it) | `self_development_host_build.go:253`; host compare `hostbuild/service.go:574` | none on the guest |
| `manifest_closure_commit`: applied manifest closure, code commit, base image and commit against the host result | `releases/<digest>/release-manifest.json` | yes (by the updater) | `updater/closure.go:448,505`; `updater/runtime.go:196` | `updater: closure.nar digest ... != manifest` / `health identity mismatch` |
| `manifest_builder_receipt`: manifest `BuilderReceiptDigest` against the host result | manifest field | **no** (by-construction copy) | `self_development_host_build.go:366` | none |
| `manifest_decision`: applied manifest accepted head and content digest against the decision and operation | manifest; operation `ReleaseDigest` | yes | `updater/updater.go:911`; `updater.ReadPinnedManifest` | `updater: incomplete or mismatched apply request` |
| `decision_pending_transition`: applied event against the head's pending transition | `materialization_*` events | yes | `computerevent/reducer.go:92-98` | `ErrInvalidTransition` |
| `frozen_surface_decision_heads`: candidate's frozen surface against the surface the decision expected | start pin (overwritten at decision) | **no** | `api_self_development.go:491-496` compares the request to the live head only | none |
| `checkpoint_event`: checkpoint ref against the checkpoint-published event | operation `CheckpointRef`; event on the tape | yes (checkpoint authority) | `PublishCheckpoint` joins certificate fields (`self_development_materializer.go:661-690`) | authority refusal |
| `route_event`: route certificate and generation against the route-projection event | operation `RouteCertificate`; event on the tape | yes (route CAS) | `routeledger.TransitionCommand` ExpectedGeneration (`self_development_materializer.go:796`) | route CAS refusal |
| `effective_release_operation`: running release against the applied operation's release | `current` manifest; operation `ReleaseDigest` | yes (at apply) | `updater/runtime.go:193-196` | `health identity mismatch` |
| `divergence_derived`: effective release against the platform's | none derived; vmctl manual flag | **no** | `internal/vmctl/ownership.go:139-151` | none (`divergent_release` does not exist) |

Counting the not-compared pairs (the S6 value criterion's quantity): at gate,
four (`test_receipts_exit`, `builder_receipt_patch`, `manifest_builder_receipt`,
`frozen_surface_decision_heads`) plus the underived `divergence_derived`. Each
becomes a measured number once the witness is fetched on staging.

## What this changes for the slices (to confirm against the witness)

- S6-1 ("declared tests are a gate"): the exit-status compare exists at
  Freeze. The work is to declare the test (name the command in the bundle's
  digest), bind the receipt's exit status into what is approved, and compare
  at admission and the materializer. The leg 3 negative must be rewritten
  around a post-freeze mutation, since a failing test cannot reach a bundle.
- S6-2 ("bind and compare the build"): `VerifyJob` already binds patch and
  base. Re-scope to the builder receipt content, the closure tree against the
  patched tree, and the frozen-surface against decision-heads join.
- S6-4 is unchanged: divergence is genuinely not derived.

## Heresy delta

- discovered: the goal file's claim that no code reads a test receipt's exit
  status is false (freeze reads it); a recorded-only join (builder receipt
  content) exists on the host-build path; the applied event's resulting
  commitment is self-sourced from the head.
- introduced: none.
- repaired: none.

## The release witness and how it could fail (S6-0)

The witness is `GET /api/computers/<id>/self-development/operations/<op>/release-witness`
(`internal/agentcore/selfdev_release_witness.go`). It is a read of the tape, the
operation row and the updater store; it is never an authority and has no store
of its own. Mutation class: **orange** (a read, no canonical write). Rollback:
revert the commit; nothing reads the witness back. Failure modes, written down
before the tests that cover them:

1. **Wrong join.** (a) A staged host build of another attempt or operation is
   taken for this one: matched on operation id and bundle digest, preferred by
   the job id the gate itself derives, `candidates` reported. (b) The checkpoint,
   route and applied events are found by the idempotency key derived from the
   decision event, never "the latest event". (c) The effective release is
   compared to an operation that has been superseded or restored: that is
   `bound: null` with the reason, never `false`. (d) Another computer's
   operation: the route requires `X-Authenticated-Computer` to equal the path's
   computer and the lookup is scoped to it.
2. **Stale read.** The stores are read one after another, not atomically. The
   operation row is read again after the sections; if its state or `updated_at`
   moved the read repeats (three tries) and `read_consistent: false` reports a
   read that never settled (an apply in flight). `read_at` and
   `Cache-Control: no-store` are set; nothing is cached by the witness.
3. **Leaks private data.** The witness returns digests, refs, exit codes,
   counts and fixed reason text. No patch bytes, no file paths, no command text
   (a command is its sha256), no reject reason, no event payload, no decision
   actor, no keys; error strings are truncated. It sits behind the same owner
   authentication as the other self-development reads.
4. **Claims compared when not.** `compared_at_gate` is read from the constant
   table `selfdevGatePairs`; the code that evaluates a pair can only attach an
   observation and cannot set it. `TestReleaseWitnessGateTableMatchesTheGateCode`
   ties every row to the gate source: a compared row needs its comparison text
   present, a recorded-only row needs the text that would make it compared to
   stay absent (or under its count), so a gate change fails the test until the
   row is updated. `bound` is the witness's own observation and is separate: a
   join can be bound without being compared, and the summary counts
   `recorded_only_unbound` apart.
5. **Writes or becomes an authority.** GET only; the route appends nothing,
   transitions nothing and reads nothing back; the test asserts the operation
   row and the event head are unchanged by a read.
6. **Partial rows.** An operation in `requested` through `awaiting_approval`
   has no decision, release, checkpoint or route: every dependent pair is
   `bound: null` with a reason, never an error or a panic.
7. **Unbounded reads.** At most 256 staged host-build directories are opened
   per read; the effective-release source tree resolves in the background as
   `/health` already does.

What the witness does not do: it does not read the vmctl route (a network call
to another service), the event payload bodies, or the Store B release records.
Those stay with their own routes.
