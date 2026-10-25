# Recoverable changes: keep-and-skip replay, the restore pin, and the cross-version gate (SR7 design, 2026-10-24)

Status: design, docs only, no code. Station SR7 of
`docs/definitions/choir-selfdev-readiness-metamission-2026-10-24.md`. Problem:
`docs/problems/an-older-release-cannot-read-a-newer-tape-2026-10-24.md`.
Class of this doc: green. Class of what it designs: red (tape/evidence, boot
path, restore, store schema), so it is reviewed tier 2 like the change.

Owner invariant (2026-10-24): any release can boot on any tape written by
releases inside its support window (one week, ledger 18:5x), in both
directions. An older binary keeps and skips what it does not know, never fails
boot and never drops it. A new kind ships in two releases (reader first, writer
later). Store schema changes are additive or two-phase. A cross-version replay
gate in CI enforces it and must not lengthen CI wall clock.

Labels: **[code]** read in code on origin/main 5e11072c5, nothing run;
**[H]** hypothesis; **[run]** observed on a CI run.

## 0. What the code does today (the problem doc's 23:5x findings, verified)

| Claim | Verdict | Where |
|---|---|---|
| Boot replay is `Reconstruct` with no target, and any error ends boot | confirmed [code] | `internal/autoputer/run.go:764`, `:807` |
| Unknown event kind fails replay | confirmed [code] | `Event.Validate` `internal/computerevent/event.go:157-159`, called by `Reduce` `reducer.go:61` from the replay apply `appender.go:902` |
| Unknown op in a known batch fails replay | confirmed [code] | `ProjectionBatch.Validate` `projection_batch.go:202-204` via `DecodeProjectionBatch` `:226` (called at `appender.go:1136`); `projectOp` default `internal/store/project.go:105-106` |
| Unknown envelope field fails replay | confirmed, mechanism sharper than the doc said [code] | The digest is the canonical JSON of the Go struct (`event.go:119-138`), so a field the struct lacks drops out. corpusd checks the raw bytes' digest (`internal/platform/event_replay.go:74`), then decodes into its struct (`:77`) and serves the struct. The guest recomputes the struct digest in `Reduce` (`reducer.go:64`), and `sameHead` against the served next head fails (`appender.go:906`). `Store.Prepare` recomputes it again and stores the struct bytes as `event_json` (`internal/store/computer_events.go:37-43`). Writes: `PinEvent` requires the struct round trip (`event_artifacts.go:38-41`); admission decode refuses unknown fields (`decode.go:331`) |
| Unknown op-body field is dropped from the projection, kept on the tape | confirmed [code] | typed bodies (`projection_batch.go:84-170`) are decoded with `json.Unmarshal` |
| **New:** a receipt with one more kind field fails an older reader | [code] | `RequireKindFields` requires the exact count (`receipt.go:161-164`); `VerifyEventHeadReceipt` names 12 (`event_receipt.go:37-41`) |
| **New:** a field added at batch level is safe for an older reader | [code] | the batch decode ignores unknown fields (`projection_batch.go:223`), and the payload digest is over raw bytes (`payload_resolver.go:77`) |
| **New:** corpusd's page server does not validate kinds | [code] | `EventsPage` decodes and serves without `Validate` (`event_replay.go:77-109`). An older corpusd serves newer kinds; it drops envelope fields on re-marshal |
| **New:** corpusd is the head authority, and the receipt signs the next head | [code] | `Reduce` in the CAS (`internal/platform/computer_events.go:237`); signed fields (`:347-361`) |
| Restore re-converges at the next boot | **confirmed in code (static), not yet run** | see §3.1 |
| **New:** between a restore and its restart, every append is refused | [code], [H] until probed | `appendLocked` requires local head = platform head (`appender.go:741-743`). After the flip, local is at the target T and platform is at the restore event R |
| **New:** a restore restart resumes work | [code] | the marker is reason-agnostic (`internal/agentcore/planned_restart.go:200-203`; used at `engineering_admission.go:218`, `:330`) |
| Store: epoch 3, a raise-only floor, no migration framework | confirmed [code] | `internal/storeschema/schema.go:54`; `floor.go:111-121`, checked at `internal/store/store.go:938`. The updater refuses a release whose epoch is below the store's (`internal/updater/updater.go:459-461`) |
| **New:** destructive DDL runs at Open today | [code] | DROP of retired tables (`store.go:1102`, `:1113`; `texture.go:369`, `:377`); primary-key reshapes (`store.go:3597`; `texture.go:479-481`, `:2446-2448`). An older binary whose Open wants the old key would reshape it back [H] |
| **New:** the store-floor CI guard is not wired into CI | [code] | `scripts/check-store-schema-floor.sh` exists; nothing in `.github/workflows/ci.yml` calls it |
| CI room | [run] | `go-vet-build` needs only `plan` (`ci.yml:169-189`) and took 90 s and 101 s in the two latest Go runs on main (40138087758, 40132893674), whose slowest shards took 1038 s and 863 s. The account runs at most 20 jobs at once, and 24-25 are ready when plan ends (`docs/problems/ci-critical-path-race-shards-2026-10-22.md` finding 3). `plan` computes the deploy base only on main pushes and does not output it (`ci.yml:92-106`); `deploy-impact` computes it again (`ci.yml:755`) |

## 1. Failure modes (written before the design)

Each failure mode names the section that answers it.

- **F1. A skipped event carried authority.** Examples: a skipped
  `restore_requested`, an approval, an owner act. The older binary then acts
  as if the act never happened; an owner may approve twice and apply twice.
  Answer: transitions are recognised from the signed head (§2.2). Authority
  kinds and ops are declared `refuse` (§2.1), and the never-skip list is in
  §2.6.
- **F2. A skip leaves the projection wrong, not just missing data.** A known op
  in a later event depends on rows a skipped op would have written, so it
  fails (poison, a boot loop) or writes the wrong thing. Answer: `safe`
  requires independence (§2.1). The gate's projection oracle compares the old
  and new projections on what both know (§4.3).
- **F3. A skip hides a corrupt event.** Answer: the digest and the signature
  are checked on every event and never skipped. Only a writer can declare a
  kind skippable (§2.2), so an undeclared unknown is refused.
- **F4. The digest breaks on an unknown envelope field** (§0). Answer: the
  digest is taken from the raw bytes (§2.3).
- **F5. A receipt field is added, and every older reader refuses every event
  that carries it** (`receipt.go:162`). Answer: older readers accept extra
  signed fields first, and only receipts of kinds that older readers already
  refuse carry the stamp (§2.2).
- **F6. Readers that skipped events never project them.** After a forward
  update, the newer binary starts replay from the local head, which is past
  the skipped events. Answer: catch-up at boot (§2.5).
- **F7. Replay is not deterministic across binaries.** A newer binary
  rebuilding a restore target projects events the target release skipped, so
  the checkpoint witness mismatches, or the restored store holds data its
  binary cannot read. Answer: the projection is a function of the tape, the
  reader manifest and the pins, and a restore rebuilds with the target's
  manifest (§3.4).
- **F8. The support-window boundary.** A release far older than the tape
  writer skips its way to a projection that is silently thin. Answer: such a
  reader refuses loudly (§2.6).
- **F9. Re-convergence: a restore lasts only until the next boot** (§3.1).
- **F10. Restore pins break history.** Applying exclusion to the restores
  already on tapes would remove events that later events were written
  against, because those computers re-converged and kept working. Answer:
  only restores written as pinned are pins (§3.2).
- **F11. The pinned computer replays T-state obligations.** A restore restart
  is planned, so it resumes work. With a pin, the work open at T would be
  dispatched again, and it already ran (and may have sent mail) inside the
  window. Answer: §3.5.
- **F12. Idempotency collides across the window.** An act retried after the
  restore derives the same key. corpusd returns the receipt of the excluded
  event (`internal/platform/computer_events.go:221`, `:230`), so the act is a no-op in the restored
  timeline. Answer: §3.5.
- **F13. Monotone facts are undone.** A key rotation or revocation inside the
  window must survive the restore (update design §4.2, "Some facts are
  monotone across restore"). Answer: exclusion never touches the head
  reducer, and monotone kinds stay applied (§3.3).
- **F14. A projection base straddles a pin.** A base taken inside an excluded
  window cannot seed a rebuild past the restore. Answer: §3.4.
- **F15. Adding a field without `omitempty` changes every historic digest.**
  Answer: the static check (§4.4).
- **F16. The gate lengthens CI.** An extra job takes one of the 20 slots
  before a critical shard, or a per-SHA binary cache evicts the Go caches
  (cache use was 10.54 GB against the 10 GB limit, finding 4 there). Answer:
  §4.1.
- **F17. The gate tests one step only.** Release k reading k+1 does not prove
  that k reads k+2. Answer: the age rule for `refuse` kinds (§4.4); the rest
  is residual R3.
- **F18. An older Open destroys a newer store.** It drops a table or reshapes
  a key back. Answer: §5.

## 2. Keep-and-skip

### 2.1 The reader manifest and declarations

- One checked-in file, `internal/computerevent/reader-manifest.json`. It is
  append-only and lists every kind this release can *read*:
  - event kinds;
  - projection op kinds;
  - envelope field names;
  - typed op-body field names, per op.
- Each new event kind or op kind declares `skip: safe` or `skip: refuse`.
  - `safe` means an older reader may skip it without behaving differently on
    anything it does know. Its rows live in tables or rows that no known op
    reads or writes, and no authority decision (approve, apply, publish,
    send, restore) reads them.
  - `refuse` is required for anything that carries authority or changes the
    meaning of known state.
- A Go test (it runs in the existing shards) asserts that the file equals the
  registries, and that typed op-body fields are reflected from the body
  structs. The file therefore cannot drift from the code.
- Today one closed map serves both reading and writing (`validEventKinds`,
  `projectionOpKinds`). This splits it into a *reader* set (the manifest) and
  a *writer* set (what `AppendNew` and the batch writer admit). The writer set
  must be a subset of the reader set.
- The two-release rule is then mechanical: the candidate's writer set must be
  a subset of the base release's reader set (§4.4).
- Retired kinds are never removed from the manifest. They stay readable as
  history (frozen V1 decode, `decode.go:9-12`).

### 2.2 Per level: skip or refuse, and who declares it

**Never skipped, at every level:**

- the raw digest of the event and the payload;
- the corpusd signature and the receipt binding;
- sequence and previous-head continuity.

Failing any of these refuses, as it does today.

| Level | Rule | Declared by | Recorded |
|---|---|---|---|
| **Unknown event kind** | Skip iff (a) the receipt verifies, (b) the signed next head moves only sequence and canonical head (desired, effective, pending, both state commitments and the credential epoch all equal current), and (c) the receipt carries `compat.skippable=true`. If (b) fails, the kind is a transition: refuse. If (c) is absent, refuse: either the event is corrupt or the reader predates the stamp. The head advances to the signed next head; nothing is projected | corpusd, in the receipt (it always knows the kind: `PinEvent` validates it, `event_artifacts.go:34-41`). Only events of kinds new after S4a carry the field, so older readers that refuse unknown kinds anyway see no new failure (F5) | an index row with `projection_status=skipped_kind`, plus the skip ledger |
| **Unknown op in a known batch** | Apply the known ops. Skip an unknown op iff its kind is listed in the batch's `compat.skippable_ops`. An unlisted unknown op refuses | the guest writer, in a new batch-level field `compat {skippable_ops, writer_base_commit, writer_base_time}`, filled from its manifest (`safe` ops only). An older reader ignores the field (§0) | a skip-ledger row per (sequence, op index, kind) |
| **Unknown envelope field** | Ignore it. The tape keeps the raw bytes, and the local index stores raw bytes, not the struct (`computer_events.go:41`). A meaning-changing envelope field is not allowed: it must be a new event kind declared `refuse` | static check: a new field must be `omitempty` and listed by the base manifest, and S3 must have been in the base for at least the window | nothing (no data lost) |
| **Unknown receipt kind field** | Accept extra signed fields (relax `RequireKindFields` to "required ⊆ present") | corpusd | nothing |
| **Unknown op-body field** | Dropped from the projection and kept on the tape (today's behaviour). Accepted loss: an older writer that upserts the row writes it without the field. Two-release rule on body fields, so the reader-first release keeps the field on upsert | static check over the manifest's body fields | nothing |
| **Unknown version** (`SchemaVersion`, `ReducerVersion`, batch `Version`/`ProjectorVersion`) | Refuse, as today (`event.go:141`, `:170`; `projection_batch.go:183-187`). Versions are not bumped inside a window; changes are additive | review | typed boot refusal |

**Why the writer declares and the reader stays fail-closed.** The owner
invariant ("keeps and skips what it does not know") holds as policy: every new
kind defaults to `safe` and is stamped skippable. The reader still refuses
anything no writer declared, and that is what keeps a garbage or truncated
event from being skipped as "newer" (F3). A writer cannot stamp an op it does
not know itself: the batch dry run (`internal/store/computer_events.go:97`)
validates against the writer's own reader.

### 2.3 The raw digest (prerequisite for any envelope growth)

- **corpusd.** `EventsPage` adds `request.event_raw`: the pinned bytes, as
  read from the tape store and digest-checked (`event_replay.go:70-76`). The
  JSON field is additive; older guests ignore it, because
  `DecodeHistoricDurableEvents` does not disallow unknown fields
  (`decode.go:134-141`).
- **The guest.** Replay verifies `sha256(event_raw) == event_digest ==
  receipt.event_digest`, decodes the known fields from `event_raw`, and calls
  a `ReduceVerified(current, event, digest)` that takes the digest instead of
  recomputing it from the struct.
- **The local index.** `Store.Prepare` checks and stores the raw bytes.
- **Live appends.** These are unchanged: the writer knows every field.

### 2.4 The skip ledger and its surface

- A new projection table, `computer_event_skips`, keyed by computer and
  sequence. Columns: op index, kind, reason (`skipped_kind`, `skipped_op`,
  `excluded_by_restore`), `writer_base_commit`.
- It is tape-derived: any replay with the same manifest and pins rebuilds it
  byte for byte.
- `/health` and the runtime observability endpoint show a summary. For
  example: `tape_compat: {skipped: [{kind, count, first_seq, last_seq,
  needs: <writer_base_commit>}], excluded: [{restore_seq, target_seq,
  count}]}`. The host's release catalog turns `needs` into "needs release ≥
  X".

### 2.5 Catch-up after a forward update (F6)

1. At boot, before the tail replay, the binary reads the skip ledger for kinds
   its manifest now knows.
2. It projects those events (only the ops it skipped before), in sequence
   order, then deletes their ledger rows.
3. This is correct only because a `safe` kind is independent: no known op
   that ran in between could have touched its rows.
4. Any error falls back to a staged full rebuild (the rematerialize path),
   never to a partial store.

### 2.6 Refusal, the window boundary, and the never-skip list

- **What a refusal is.** It uses the existing typed boot refusal on `/health`
  (`internal/autoputer/boot_refusal.go:65-73`) with a new kind,
  `tape_needs_newer_reader`. The refusal names the kind, the sequence, and
  the writer release.
- **The window boundary (F8).** A reader refuses to *project* content whose
  `writer_base_time` is more than the window (7 days, a compiled constant)
  after its own base commit time.
  - Events inside an excluded restore window are not projected, so they never
    trigger this.
  - Default and alternative: open decision D2.
- **Never skippable:**
  - `genesis_imported`;
  - every kind that moves the head: `effect_accepted`,
    `materialization_*`, `rollback_*`, `researcher_update`. This follows
    mechanically from rule (b);
  - `key_rotated` and `key_revoked`, and any new security or monotone kind
    (declared `refuse`);
  - pinned `restore_requested` (§3.2), which a pin-unaware reader cannot
    honour; the restore preflight handles it (§3.6);
  - any op whose rows an authority decision reads, such as a new
    self-development or approval op.

## 3. The restore pin

### 3.1 Re-convergence is real in code

The restore sequence today:

1. `restoreComputer` appends a causal `restore_requested` R: `DecisionRef` =
   target head T, `AuthorityRef` = `platform-control:restore`
   (`internal/agentcore/rematerialize.go:363`, `:391-419`).
2. `RematerializeFromTape` rebuilds a staged store through T
   (`rematerialize.go:165`, via `appender.go:1163-1193`), flips it in, and
   reopens it (`:244-286`).
3. If the binary changes, a planned marker is written with the checkpoint
   digest as its target (`self_development_restore_restart.go:76`,
   `:123-149`).

What happens after:

- **At the next boot.** The marker is consumed (`internal/autoputer/run.go:536`),
  but nothing reads its target for replay. `Reconstruct` runs from the local
  head T to the platform head (`run.go:764`, `appender.go:854-856`). It
  applies every event in (T, R] with the restored binary, and it must end
  with local head = platform head (`appender.go:1045-1047`). The boot-time
  projection-base planner also targets the platform head
  (`internal/autoputer/projection_base.go:121-125`).
- **So the restore is undone at boot,** and an unknown event in (T, R]
  crashes the boot instead. This matches the 03:39 drill (the operation went
  back to applied after the restart).
- **Structurally, the current model cannot hold a pin.** The local projection
  must be a prefix replay of the whole tape (`appender.go:741-743`, `:1045`).
  So a pin needs the replay change below; a boot flag alone would fail boot
  with `ErrNeedsProjectionRepair`.
- **Before the restart [H].** The flipped store sits at T while corpusd sits
  at R, so every append is refused until the restart (`appender.go:741-743`).
  A restore that does not change the binary schedules no restart
  (`rematerialize.go:372`), so that computer cannot write until some
  later restart.

S5 step 0 runs the probe on a test computer (§6) to confirm both points
before any code.

### 3.2 The pin is a tape fact, not a boot flag

- **A pinned restore.** It is a `restore_requested` with `AuthorityRef =
  platform-control:restore-pinned`. That is a value inside the frozen V1
  envelope, so no reader breaks. `DecisionRef` = T, as today.
- **Older history keeps today's meaning (F10).** A historic restore (the old
  `AuthorityRef`) re-converged, and later events were written against that
  state, so it is not a pin.
- **Why not a file or a marker.** A durable pin in a file or a marker would be
  a second home for the state (standing question 5, I27). It would also be
  lost by every rebuild from genesis or from a base. The restore event is
  already on the tape and names its target; the pin is that event read
  differently.
- **Nothing to consume or clear.** A pin is a range of history. A later owner
  act is simply a new event after R and is projected normally. A later
  restore adds a new pin.
- **The planned-restart marker.** It keeps its one job: resume versus close
  (§3.5).

### 3.3 Effective history

The projection is `P(tape, manifest, pins)`.

**The effective set.** `effective(x)` is a pure, unit-tested function over the
pins (Rj, Tj) by sequence:

- let Rj be the last pin with seq(Rj) ≤ x;
- if there is none, `effective(x) = [1..x]`;
- otherwise `effective(x) = effective(Tj) ++ (Rj..x]`.

So nested restores compose, and so does a restore to a checkpoint that lies
inside an earlier excluded window.

**Every event is still processed.** The digest and signature are verified,
the head reducer runs (so the credential epoch and the head stay right, F13),
and the index row is written. Only an event in effective(head) is
*projected*, plus the monotone kinds, which are always projected.

**Excluded events:**

- they get `projection_status=excluded_by_restore` and a skip-ledger row;
- they are never caught up (§2.5);
- they never trigger a refusal for an unknown kind or the window, unless
  their kind is monotone.

**The index readers** must use effective history. Today they include:

- the recovery query over `effect_accepted`/`effect_rejected`
  (`internal/store/computer_event_recovery.go:16`);
- `EventByIdempotency` (`computer_events.go:338`).

The S5 code inventories every `computer_event_index` reader (`computer_events.go`:
50, 299, 338, 357, 435, 471, 493).

**Where `P` must be applied.** All four use `reconstruct` (`appender.go:869`):
boot replay, `ReconstructThrough` (restore), replay completeness
(`replay_completeness.go:318-320`), and the projection-base rebuilder
(`internal/projectionbase/rebuilder.go:364`). So the rule goes into
`reconstruct` once, never into its callers.

**The lookahead.** Replay meets events in (T, R] before it meets R, so it
needs the pins ahead of time.

- corpusd keeps an additive index, `computer_restore_pins` (restore seq,
  restore digest, target head, target seq). It is written in the CAS
  transaction when a pinned restore is admitted, and the CAS refuses a target
  that is not on the chain.
- A page endpoint serves the index. Replay fetches it once per reconstruct.
- The index is verified against the tape in the same pass: at each R, the
  event must match its entry, and a pinned R missing from the index refuses.
  The tape stays the authority.
- If the index is unavailable, replay refuses. This adds no new fate-sharing:
  replay already reads its pages from corpusd.

### 3.4 Determinism and restore rebuilds (F7, F14)

- **The rebuild uses the target's manifest.** `RematerializeFromTape` rebuilds
  through T with `manifest = current ∩ target release's manifest`. The target
  release's manifest digest comes from its release manifest (S8 stamps it).
  This yields what the target binary would have projected.
- **The witness check stays the verifier.** A mismatch refuses the restore
  loudly (`rematerialize.go:197`).
- **Base selection.** A base at S may seed a rebuild to T only if
  `effective(S) == effective(T) ∩ [1..S]`. Otherwise use an earlier base or
  genesis. This applies to `planRestoreTarget` (`internal/agentcore/restore_base.go:87`)
  and to the boot planner.

### 3.5 Work, restarts and idempotency (F11, F12)

**A restore restart is planned, but it is not an update.**

- With the pin on, a restore restart closes every open obligation in the
  restored projection: run, wake, control, directive, assignment and work
  item. Their visible fate is "superseded by a restore to <checkpoint>".
- This is the crash rule of "Restarts End Work (Crash) Or Resume It (Planned
  Update)" applied to a planned act that is not an update. Re-dispatching
  T-state work would duplicate effects that already happened in the window.
- Standing intents (Autopaper schedules) continue, as after a crash.
- The marker reason `self_development_restore` selects this; today the reason
  is not read (`planned_restart.go:200-203`).

**Idempotency.**

- Runtime-minted keys (I2c) include the lineage: the digest of the latest
  pinned restore in effective history, or ZeroHead.
- An act retried after a restore then gets a new key and a new event, never
  the excluded one's receipt.
- The lineage is empty until the first pinned restore, so existing keys do
  not change.

**Projection gaps left by the window:**

- External effects inside the window (mail sent, a paper published) are not
  undone. Their events stay on the tape, which is the audit.
- Rendering a summary of the excluded window in Texture or the Updates app is
  residual R5.

### 3.6 Restore preflight

A restore is refused before it mutates anything when:

- the target release's manifest lacks a monotone kind present in (T, R]; or
- the target release cannot honour pins (it predates S5).

For the second case, the default for the first week is D3.

## 4. The cross-version gate

### 4.1 Where it runs: inside `go-vet-build`, not a new job

- **No new job.** A new job that needs only `plan` competes for the 20 slots
  at the moment 24-25 jobs are ready, and can push a critical shard back by
  one slot wait (F16).
- **Slack.** `go-vet-build` needs only `plan`, feeds `check`, ran 90-101 s,
  and the critical shard ran 863-1038 s. That leaves more than 12 minutes of
  slack.
- **The added steps fit in it:**
  - The candidate harness `cmd/tapecompat` is compiled by the existing
    `go build ./cmd/...` (`ci.yml:185-186`).
  - The base harness is built from a `git worktree` at the base SHA on the
    same job's `GOCACHE`. The candidate build has just compiled Dolt and the
    other dependencies there, so only packages changed since the base
    compile.
  - [H] The added time is 2-4 minutes warm and under 6 cold, so the job stays
    under 10 minutes. A step timeout of 10 minutes bounds it.
- **No per-SHA binary cache.** It would add about 100 MB per main SHA to a
  cache that is already over its limit, and evict the Go caches (F16).
- **Proof of no lengthening:** the job timings of the first 5 Go runs after
  landing, against the 2 baselines above. The `vet-build` end must stay
  before the slowest shard's end.

### 4.2 Which reader, which tape

- **The base.** It is the live staging commit, picked by
  `deploy-base-select`, the same rule `plan` uses on main. `plan` gains an
  output `reader_base` for every event kind; on a pull request it falls back
  to the PR base with a notice.
- **Bootstrap.** Until the base contains `cmd/tapecompat`, the gate prints
  "base predates the harness" and passes. It fails closed once
  `git merge-base --is-ancestor <harness commit> $reader_base` holds.
- **The fixture.** It is written by the candidate: `tapecompat write-fixture`
  drives the real append path, meaning the appender plus the platform CAS and
  `EventArtifactService` over a temp platform store with a fixture key, as
  `internal/platform/event_replay_test.go` does.
  - It emits the corpusd wire format: `DurableEvent` pages (with
    `event_raw` once S3 lands) and payload blobs. That format is the stable
    protocol every guest already reads.
  - It must cover every kind and op kind in the candidate's *writer* set. A
    unit test fails when a writer kind has no fixture generator.
  - It also covers a pinned restore across a few events (once S5 lands) and
    one `refuse` sub-fixture per new `refuse` kind.

### 4.3 What it asserts

1. **It boots.** The base harness replays the fixture to the fixture head:
   `Reconstruct` returns nil and the local head equals the platform head.
2. **It skips and preserves only what it may.** Every event the base does not
   know is in the skip ledger with a reason, and its kind is `safe` in the
   candidate manifest. The processed count equals the fixture's event count.
   The base's index digests equal the fixture's raw digests.
3. **It refuses what it must.** Each `refuse` sub-fixture ends in the typed
   refusal, not in a skip.
4. **The projection oracle (F2).** Both harnesses dump per-table digests. On
   the tables and ops the base knows, the base's projection must equal the
   candidate's.
5. **The store round trip** (§5).

### 4.4 The static check (seconds; same job, first step)

`scripts/check-tape-compat.sh --base $reader_base`. It reads
`git show $base:internal/computerevent/reader-manifest.json`, as
`check-store-schema-floor.sh` reads the epoch, and fails closed when the base
cannot be read. It fails when:

- the candidate's writer set is not a subset of the base's reader set (the
  two-release rule: event kinds, op kinds, envelope fields, op-body fields);
- the manifest lost an entry, or an entry lacks `skip`;
- a candidate writes a `refuse` kind whose reader entry is less than 7 days
  old on main (the commit time of the manifest line, by `git log -S`) (F17);
- an envelope field is new and S3 has been in the base for less than the
  window, or the struct field is not `omitempty` (F15);
- a new event kind is written before S4b.

**Override.** A `Tape-Compat-Override: <reason>` trailer, as with
`Store-Floor-Override`.

**Before the dynamic gate is enforcing,** the static check runs report-only
for one week.

## 5. Store schema: additive only, two-phase removal

- **Static check:** `scripts/check-store-additive.sh --base $reader_base`.
  It runs in the same job and scans the lines the diff adds under
  `internal/store/**`. It fails on:
  - `DROP COLUMN`, `MODIFY`/`CHANGE COLUMN`, `RENAME`;
  - `DROP PRIMARY KEY` and other key reshapes;
  - `NOT NULL` without a default on an added column;
  - a `DROP TABLE` of a table that is not in a `retiringTables` list at the
    base, dated at least 7 days earlier.
- **Two-phase removal.** Release N stops reading and writing the table and
  lists it in `retiringTables` with a date. Release N+k, at least a week
  later, moves it to the drop list.
- **Dynamic round trip** (gate step 5, F18):
  1. the candidate harness opens a fresh store and replays the fixture;
  2. the base harness opens that same workspace (an older Open on a newer
     store), and its schema dump must show no drop and no reshape;
  3. the candidate reopens it.

  This would have caught the primary-key reshapes in §0 if they were new [H].
- **Wire the existing guard.** `scripts/check-store-schema-floor.sh`
  (lowering `storeschema.Version`) is added as a step in the same job. It is
  written and self-tested, but not called today.
- **The epoch's new meaning.** Under SR7, `storeschema.Version` moves only for
  the second phase of a two-phase change, after the window. The updater's
  rule against an older epoch (`updater.go:459-461`) stays as the
  out-of-window fence.

## 6. Slices

Every slice is off by default and can land on its own.

Proofs run on a staging **test** computer: test class, not b@c, not
`critical_protected` or `public_platform`. The canary is a dedicated op kind,
`tape_compat_probe_recorded` (`safe`). Its reader ships in one release; its
writer ships in a later one, behind an internal endpoint enabled only for
test-class computers. The proof is the two-release rule run live.

**S1. The manifest, split registries, static check (report-only).**

- Change: §2.1 and §4.4.
- Switch: report-only in CI.
- Proof: the selftest of the script, with throwaway repos as in
  `selftest-store-schema-floor`; the manifest equality test.
- Rollback: revert.
- Class: yellow, with orange-light for the registry split, which has no
  behaviour change.
- Review: tier 1.

**S2. Keep-and-skip for ops (§2.2 ops, §2.4, §2.5).**

- Change: the batch `compat` stamp; the reader skips listed ops; the skip
  ledger; `/health`; catch-up.
- Switches: `CHOIR_TAPE_STAMP` (writer) and `CHOIR_TAPE_SKIP` (reader), both
  off.
- Proof: on the test computer, enable the probe writer and write probe ops.
  Then restore with the updater to the reader-less release, or restore to a
  checkpoint on it. Read `/health`: `skipped [{kind:
  tape_compat_probe_recorded, count: n}]`, and boot succeeds. Update forward:
  the ledger empties and the rows appear.
- Rollback: switches off. Batches already stamped are harmless, because older
  readers ignore the field.
- Class: red (tape, boot path).
- Review: tier 2.

**S3. The raw digest (§2.3).**

- Change: corpusd sends `event_raw` (additive, always on); the guest uses it
  behind `CHOIR_TAPE_RAW_DIGEST`.
- Proof:
  - CI: the fixture has an event with a test-only envelope field, and only
    the harness accepts it.
  - Staging: the test computer boots and its replay completeness probe
    passes with the switch on.
- Rollback: switch off.
- Class: red (tape/evidence, platform).
- Review: tier 2.

**S4a. The reader for unknown kinds (§2.2 kinds).**

- Change: the `Validate` split; head advance trusted to the receipt for causal
  unknown kinds; `RequireKindFields` relaxed.
- Switch: `CHOIR_TAPE_SKIP` again.
- Proof: CI fixture only (no new kind exists).
- Class: red.
- Review: tier 2.

**S4b. The corpusd receipt stamp for new kinds.**

- Switch: on the corpusd side, off.
- Ships with the first new event kind and no earlier.
- Class: red (gateway/evidence).
- Review: tier 2.

**S5. The restore pin (§3).**

- **Step 0, documented first:** the probe on the test computer, before any
  code.
  1. Checkpoint C at head T.
  2. Three user-preference writes.
  3. Restore to C, then read the preference and the head.
  4. Write once (H2 predicts a refusal).
  5. Owner restart, then read again.
  6. Record the result in the problem doc.
- Then the code:
  - the pinned `AuthorityRef`;
  - the corpusd pin index and its endpoint;
  - `effective()` in `reconstruct`;
  - the index-reader filter;
  - restore restarts that close work;
  - the lineage idempotency salt;
  - the preflight.
- Switch: `CHOIR_RESTORE_PIN`, off. While off, restores are written unpinned,
  as today.
- Proof: the step 0 probe repeated with the switch on. The head is at R+, the
  preference stays at T across two restarts, a write after the restore
  succeeds, a probe kind inside the window shows as `excluded`, not skipped,
  and the boot does not crash.
- Rollback: switch off for new restores. Pins already on the tape stay facts
  that S5 readers honour. Reverting the code re-converges, as today.
- Class: red (restore, Trace, boot).
- Review: tier 2.

**S6. The dynamic gate (§4.1-4.3).**

- Change: the harness and steps in `go-vet-build`, fail-open until the base
  has the harness. It can land right after S1 and assert today's behaviour; it
  will then correctly fail any candidate that writes a kind the base lacks.
- Proof: CI timings (§4.1), and a deliberate red PR that writes a new op kind
  with no reader in the base, which must fail.
- Class: yellow (CI and proof tooling).
- Review: tier 1, because it is a gate.

**S7. Store additive check, round trip, and the floor guard wired (§5).**

- Class: yellow.
- Review: tier 1.

**S8. The apply and restore preflight against the computer's tape.**

- Change:
  - the builder stamps the manifest digest and the writer set into
    `release.Record` and the updater manifest;
  - the approval surface shows "this release would skip N events of kind K"
    and refuses `refuse` kinds the candidate lacks.
- This is the "proved reversible" input SR8 needs, and it covers
  self-development candidates on older bases (the selfdev-79dc4bf4 receipt),
  which the CI gate cannot see.
- Class: red (run acceptance).
- Review: tier 2.

**S9. Doctrine.** An I-number for the invariant and its two rules, after S2,
S5 and S6 are proved. Green, reviewed like the change it records.

**Order:** S1, then S6 and S7 (cheap fences against new hazards), then S3 and
S2 (the D39 class), then S5 (its step 0 now, because it bears on the
metamission's "restore still rewinds"), then S8, then S4a and S4b when the
first new kind is needed, then S9.

## 7. Residuals and open decisions

**Open decisions, each with a recommended default (No Blocking Asks):**

- **D1.** Skip a batch per op, or skip the whole batch when any op is unknown.
  Default: per op, limited to the writer's `skippable_ops`. Whole-batch skip
  turns a missing new kind into missing known data (F2).
- **D2.** The window test: base commit time (7 days) or a release counter.
  Default: commit time of the platform base. It is reproducible from the
  commit and survives self-development lineages, where a global counter would
  collide. Alternative: count deployed releases from vmctl's receipts.
- **D3.** Restoring to a release that predates S5, during the first week.
  Default: allow it, with the restore answer and Texture saying "this restore
  lasts until the next restart" (today's behaviour, made visible). After one
  window the case cannot arise, because every checkpoint is pin-aware.
- **D4.** The pin lookahead: a corpusd index (red) or guest-only state.
  Default: the corpusd index. A guest-only pin row covers the
  restart-after-restore path, but no rebuild from genesis or a base.
- **D5.** The gate base for PRs: the deployed commit, or the PR base.
  Default: the deployed commit, where readable; otherwise the PR base, with a
  notice.
- **D6.** How the pinned restore is encoded: an `AuthorityRef` value, or a new
  kind. Default: the `AuthorityRef` value. A new kind would make every reader
  older than S4 refuse the restore event itself.

**Residuals:**

- **R1.** Restore does not move the reducer's effective head or state
  commitment back (`restore_requested` is causal, `reducer.go:166-177`). So
  after a restore, the signed head still names the code of the restored-away
  release. Health and identity must join the pin (CLAUDE.md "Product restore
  vs platform deploy"). This is a separate station.
- **R2.** An older writer drops op-body fields it does not know on upsert.
  The two-release rule on body fields narrows this; it does not close it.
- **R3.** The gate tests the latest deployed reader only. Replaying the oldest
  in-window release as well (a second base build) is left until a
  multi-release miss is seen.
- **R4.** Projector behaviour for a known op that changes between releases
  (a bug fix) still breaks witness equality on restore. The witness refuses
  loudly; nothing hides it.
- **R5.** Show the excluded window (runs dropped, effects that were not
  undone) in Texture and in the read-only Updates app.
- **R6.** Older corpusd on a newer tape (a platform rollback below the floor)
  is covered by `event_raw` for reads. The CAS of a newer kind by an older
  corpusd is refused, which is correct and loud. Tape-segment decode was not
  read.
- **R7.** The primary-key reshapes at Open (§0) predate this rule. Whether an
  older Open reshapes them back on today's stores is unprobed [H]. The S7
  round trip decides it.

## Class, heresy delta

- **Class:** green (this doc); the slices are classed in §6.
- **Heresy delta:**
  - discovered:
    - restore re-converges at boot [code];
    - appends are refused between a restore and its restart [code, H];
    - a restore restart resumes work it should close;
    - the receipt exact-count rule blocks any receipt extension;
    - the store-floor guard is not in CI;
    - destructive DDL at Open;
  - introduced: none;
  - repaired: none (design only).
- **Conjecture delta:** "restore rewinds" was true only until the next boot.
  Under this design it becomes a property of the tape (effective history),
  not of the store that happens to be on disk.
