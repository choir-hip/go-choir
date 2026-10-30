# Owner decision: one `effective_release` block, two definitions (P5 vs P6)

Date 2026-10-15. Class: red surface (`/health`, identity), decision only, no code
changed. Found by reading `main` (2eae77de6 and later) against
`feat/p6-p8-release-adopt`; the trial merge was done with `git merge-tree`, so no
worktree was created.

## The problem in plain words

Both branches add a block called `effective_release` to a computer's `/health` and
`/api/runtime/observability`. They answer different questions with the same name.

| | P5 on main | P6 on the branch |
|---|---|---|
| Go type | `effectiveReleaseIdentity` (private), 7 fields, `api.go` | `EffectiveRelease` (public), about 20 fields, `effective_release.go` |
| Fields | release_digest, layered, code_commit, base_commit, base_image_manifest_digest, closure_digest, builder_receipt_digest | the same names, plus serving_release_digest, marker, code_ref, artifact_program_ref, build_commit, operation_id, source patch, tree_hash, adopted_release_id, event_head |
| Who fills it | `effectiveReleaseFromManifest`, from the manifest read **once at process start** (`runtime.go`) | `Runtime.EffectiveRelease()`, from the manifest `current` points at **now**, plus the store's live event head and the applied operation |
| Who reads it | `restoredExecDiffers` (restore-restart decision); `scripts/reeng_tracer.mjs` (`layered_serving`, post-apply and post-restore checks) | the same tracer (adopt scenario); vmctl via a guest report (`reported_release`, a narrower copy) |

Not readers: the frontend, the deploy workflow and the updater do not read
`effective_release` (the updater compares `build.commit` with the manifest's
`code_commit` itself). Docs that name it: the P5 and P6 reports and two overnight
reports.

Three facts matter. P6's field names are a superset of P5's, so the JSON is wire
compatible. P6 was written to absorb P5: its report says "P5 should read
`Runtime.EffectiveRelease` instead of adding a second block". The two differ in
meaning, though: P5 says "what this process booted with", P6 says "what `current`
holds". They disagree between a pointer swap and the restart, and after a restore.
P5's own report says the block is "read from `current`", but its code reads the
startup manifest.

## Why this matters (doctrine)

CLAUDE.md, "Product restore vs platform deploy": deploy SHA and a computer's
effective identity diverge on purpose, and health must eventually join event head,
CodeRef, content witness and route, not SHA alone. P5 identity carries code and
closure only. P6 adds head and CodeRef (and the source tree as witness), and
leaves route out on purpose (vmctl owns route). P5 also notes
`/api/acceptance/execution-identity` returns 503 on a layered computer, so this
block is the only identity such a computer has.

## The six conflicting files (trial merge)

1. `internal/agentcore/api.go`: both sides edit the `effective_release` field in the observability struct (type differs); main also adds a memory block there.
2. `internal/agentcore/api_self_development.go`: P6 adds release-draft and adoptions routes where main changed the same switch region.
3. `internal/platform/handlers.go`: main registers the host-build routes; P6 registers the releases routes at the same spot. Keep both.
4. `internal/proxy/self_development.go`: P6 widens the allowed `operations/<id>/...` suffixes and adds an `adoptions` rule; main edited the same condition.
5. `internal/vmctl/handlers.go`: main adds memory fields to the list entry; P6 adds `ReportedRelease` at the same place. Keep both.
6. `scripts/reeng_tracer.mjs`: two hunks, the header comment and the USAGE line (P6 adds the `adopt` scenario, main adds the `--autonomous` notes). Textual only.

Files 3 and 5 are plain keep-both merges. Files 1, 2, 4 and 6 are small and
mechanical. The only real decision is the type behind the one JSON key.

## Options

**(a) P6's block supersedes P5's; P5's fields are mapped in. Smallest and cleanest.**
Delete `effectiveReleaseIdentity`. Keep P5's startup manifest as a private runtime
field, not JSON, so `restoredExecDiffers` still compares the booted layer with the
restored one. The tracer keeps reading `layered`, `code_commit` and `build.commit`
(same names). Readers that change: `runtime.go`, `self_development_host_build.go`,
`self_development_restore_restart(_test).go` (type rename), and the tracer's
`layered_serving` (it must now mean `code_commit == build_commit` on the live
manifest; before the restart it is false, which is the truthful answer). Risk:
P5's post-restore assertion changes timing, because `layered` goes false when
`current` is restaged, not when the process restarts. Re-run the tracer once.
Size: about 1 day, one red merge plus one tracer run.

**(b) One merged block with an explicit `serving` sub-block.**
Take P6's block and put P5's boot-time facts under `effective_release.serving`
(release_digest, layered, code_commit), alongside live `current`. Both meanings
stay visible and nothing is lost. Readers: as (a), plus the tracer reads
`serving.*`. Risk: two sets of near-identical fields in one block; this
duplicates a fact the manifest already holds, and a reader can pick the wrong one.
Size: (a) plus about half a day.

**(c) Separate keys: `effective_release` for P5, a new key for P6.**
Keep P5 untouched; rename P6's to something like `adopted_release` or
`release_identity` and update its tests, vmctl report, tracer and report doc. Risk:
two blocks answer "what does this computer run" and can disagree, which is the
failure this doctrine is meant to prevent; vmctl would report one and `/health`
show another. Size: about 1 day, plus a permanent two-block cost.

## Recommendation

Choose (a): the manifest `current` points at is already the single authority for
what a computer runs, and only P6's block joins it to the event head and CodeRef
(and the source tree as witness); a second or sub-block copy adds a competing
state authority for no reader that needs it.

Owner act needed: say yes to (a), or pick (b) or (c). Merge P6-P8 only after P5's
deployed proof (its tracer shows 17 of 18 checks) is stable, and re-run the tracer
engineering scenario after the merge, since the restore check changes meaning.
