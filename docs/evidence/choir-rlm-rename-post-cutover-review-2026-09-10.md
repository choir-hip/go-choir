# Post-Cutover Review — RLM Versioned Rename

Date: 2026-09-10. Status: adjudicated review evidence for
`docs/definitions/choir-rlm-versioned-rename-2026-09-09.md` at base `ae62fc82`.
Evidence class: static analysis plus independent panel review. Not deployed
proof, not completion.

This artifact exists because the consensus outputs under `.agentic-consensus/`
are gitignored process diagnostics. It carries the adjudicated findings, the
panel identity, and the local verifications, so the problem record survives
without the transcripts.

## 1. Why this review exists

The mission's terminal receipt (`versioned-rename-deployed-proof-2026-09-10`)
recorded landing-order step 6 complete with deployed proof, on the strength of
green CI, a staging computer active at epoch 896, and a `/api/model-policy/resolve`
probe showing ten V2 roles resolving 200 and V1/unknown names refusing 400.
That receipt is real but narrower than the claim placed on it. Independent
review found that the cutover's migration and serving fence do not cover the
persistence carrier that actually serves, and that historic V1 values are now
refused by live read paths.

## 2. Method

Five read-only review slices (writer cutover, fence and migration, authority
boundaries, mechanical residual sweep, evidence audit), then a 13-model
convergent consensus panel. Panel: codex, claude/opus, cursor, opencode,
omp-gpt56-sol, omp-gpt56-terra, omp-gpt56-luna, omp-gemini38,
omp-cursor-grok46, omp-muse-spark, omp-nemotron-3-ultra, omp-glm53-flash ran
(12); omp-hy3 failed (`401 Model hy3-free is not supported`). Raw outputs under
`.agentic-consensus/rlm-rename-completion-2026-09-10/` (not durable; superseded
by this file).

Claims marked **verified here** were re-read by the orchestrator in the source,
not accepted from review prose.

## 3. Adjudicated findings

### F1 — Migration and serving fence are vacuous against the live carrier

`internal/store/vocab_migrate.go:147-156` (`vocabValueTargets`) and `:569-582`
(`servingRoleQueries`) enumerate only `agents`, `runs`, `channel_messages`,
`inbox_deliveries`, `work_items`, `worker_updates`. **Verified here:** no
production code inserts into any of those six tables; live reads go through the
object graph (`internal/store/store.go:1269` `GetRun` → `GetRunOG`), and
role-bearing values are written to `og_objects`
(`internal/store/graph_store.go:526-527, 671-672, 1641, 1717, 1872, 1919`).

Consequence: a V1-stamped `og_objects` row is neither migrated nor refused.
V2-keyed live lookups silently miss it (`graph_store.go:911-915`), and
read-modify-write paths re-emit the token they decoded (`graph_store.go:671-672`).
The terminal receipt's claim that "the serving fence held: boot would have
failed closed had unmigrated V1 rows remained" is therefore circular: the fence
never inspects the carrier that serves.

`vocab_migrate.go:18-22` records object-graph objects/edges and grant
attestation rows as deferred **implementation** obligations. The panel split on
whether that deferral excuses the residual (11 blocker, 1 deferred-residual).
The deferral is honest; the completion claim built on top of it is not.

### F2 — Historic V1 values are refused by live read paths

`agentprofile.Super` is now `"management"` and `CoSuper` is `"engineering"`
(`internal/agentprofile/agentprofile.go:9-16`), but several readers compare
**raw historic** values against those constants. **Verified here:**
`internal/platform/checkpoints.go:217` (decoded historic verification event's
ActorProfile) and `internal/agentcore/self_development_decision_binding.go:54`
(historic decision event). Also cited and not re-read by the orchestrator:
`internal/types/cosuper_assignment.go:94-98` (`Validate()` demands
`management:<owner>` and `engineering` on rows that still hold
`super:<owner>` / `co-super`), and `internal/store/graph_store.go:911-915`
(passivated persistent-Super runs carrying `super:<owner>` are skipped).

The frozen interpreter already exists — `internal/computerevent/decode.go:94`
`CanonicalActorProfile` — and is not wired into these sites. Frozen V1 decode
was never deferrable; this is a permanent-compatibility regression.

### F3 — Compound ID migration mislabels the engineering desk

`internal/store/vocab_migrate.go:95` lists `{"-super-", "-management-"}` before
`{"-co-super-", "-engineering-"}` and matches with `strings.Contains` first.
**Verified here:** `-super-` is a substring of `-co-super-`, so
`run-co-super-*` / `work-co-super-*` become `run-co-management-*`. Invertible
but semantically wrong; latent while the migrated tables have no live writers.

### F4 — Migration provenance is not crash-atomic

Row writes and the provenance sidecar are separate steps; a crash between them
can destroy exact-token inverse provenance and mint spurious `engineering` /
`research` provenance, degrading the byte-fidelity guarantee the rollback path
depends on.

### F5 — A management spawner still persists arbitrary child tokens

`internal/agentcore/rlm_reduce.go:82-92` returns `true` for any child under
`management` while its own comment claims "No V1 token is accepted in any
position". **Verified here:** the intent persists `Role: in.Role` raw into the
spawn envelope (`:178-186`) and the model-authored `ToDesk` into durable
mailbox state. Violates the mission's `not_done_when` writer clause.

### F6 — Internal channel cast persists an unvalidated role

`internal/agentcore/api.go:755-769` writes a caller-supplied `req.Role`
straight into durable channel state with no canonicalization. An unknown token
there trips the fence on the next boot; a V1 token is silently rewritten by the
next migration pass, mutating a live audit field outside the frozen decoder.

### F7 — Verifier role spelling splits the live vocabulary

`internal/modelpolicy/model_policy.go:176-189` returns the hyphenated
`verifier-multimodal` verbatim while `Policy.Roles` is keyed by
`MultimodalVerifierRole = "verifier_multimodal"` (`:28`, `:389`). **Verified
here.** A 200 response can therefore resolve defaults instead of the configured
entry; `Canonical` accepts the hyphen spelling and rejects the underscore one
while the fence accepts both.

### F8 — Empty profile defaults to the most permissive desk

`internal/agentcore/tool_profiles.go:75-90, 101-116` defaults a missing/empty
profile to Management on wake, recovery, injection, and prompt selection, while
the fence deliberately skips empty values. Pre-existing behavior and bounded by
raw-token re-checks at the privileged tool. The panel split: 7 rejected it as a
rename defect, 5 treated it as residual or blocker. Record as a separate
hardening item, not a rename blocker.

### F9 — TOML `[roles.owner]` is refused

`internal/modelpolicy/model_policy.go:194-207` resolves `owner` to empty and
fails the whole policy parse. `owner` is a frozen non-desk protocol value and
must not be refused. Reachability requires a computer-owned section that has
not been observed. The panel leaned toward rejecting this as a defect (owner is
not a model-execution role); record it, do not block on it.

### F10 — The mission is not recorded complete

`now.status` is `working`, `next_action` still says "Record terminal receipt",
and the three registries still describe the mission as the live entrypoint. No
fetched terminal evidence artifact exists under `docs/evidence/` (both
predecessor missions published one), and the cited refresh receipt IDs
`01a08ce9` / `01a08d03` resolve to nothing in the repository. Acceptance items
3, 5, 6 have no dedicated receipts and the cutover commits' CI runs are
uncited. Standing question 6 — complete when the artifact exists and is
fetched, not when a log narrates success — is unmet.

### F11 — Computer-authored V1 prompt files unobserved

Recorded by the mission itself. The purity gate scans only in-repo files, so a
runtime-authored prompt surface is outside its coverage. Panel split (5
blocker, 7 recorded residual); it is an unpaid proof obligation, not an
authorized deferral.

### F12 — The writer-purity gate cannot see runtime writes

`scripts/check-v1-writer-purity.sh` greps quoted literals over `internal/`,
`cmd/`, `frontend/src`. It cannot see runtime-derived values, does not cover
`frontend/tests`, `configs/`, or `scripts/`. Findings F5 and F6 are exactly the
class it cannot catch.

### F13 — The decoder matrix was not exercised on staging

Acceptance item 8 requires the published matrix proven on staging with effects
OFF. The record evidences two routes (boot replay+migration+fence, and
`/api/model-policy/resolve`), not the enumerated row set.

## 4. What is genuinely complete

V2 desk constants and `CurrentVocabularyVersion = v2`; alias retirement in the
live canonicalizers with a typed fail-closed unknown-profile error; frozen V1
decoder roots and the widened `{v1, v2}` known set; the frozen V1 inventory
artifact (337 rows) with corpus manifest and CI gate; the per-function mapping
tables; TOML V1 desk-section decode; migration/fence wiring invoked on boot,
rematerialization, and rebuild; `owner` preserved as a frozen non-desk protocol
value across the fence, migrator, agent-ID rewrite, and frozen decoder; and the
reported CI runs and staging resolve probe.

## 5. Panel dissent and limits

- **F1**: one panelist (nemotron) read the recorded deferral as authorizing the
  residual. The majority reading — the deferral scopes drill round 1, and
  `not_done_when` still binds — is the one recorded here.
- **F8/F9**: panels split as noted; neither was promoted to blocker.
- **Not verified**: whether staging's `og_objects` currently holds V1-stamped
  rows. F1 is a structural claim about gate coverage and does not depend on
  that dump; the dump would convert it from structural to present-tense.
- **Correction to one panel finding**: claude reported the admission decode root
  (`internal/computerevent/decode.go:143-148`) as an unexamined hole. On
  reading, the main append path enforces V2-only
  (`internal/computerevent/appender.go:571-576`) and the recovered-prepared-event
  path deliberately accepts either vocabulary with a stated reason
  (`appender.go:670-678`). The residual issue is a stale comment claiming a
  future flip, not a proven hole. It remains the cheapest unconfirmed unknown.
- **Highest-value unexamined surface**: roughly 150 live
  `== agentprofile.Super/CoSuper/Researcher` joins over object-graph-backed
  fields; reviewers sampled four sites.

## 6. Disposition

`blocked-incomplete` per the panel's dominant recommendation, implemented in
the goal file as `working` with the reopened acceptance items, because the
required repairs are inside the mission's own authority and no external
prerequisite is missing. Revert is rejected: reverting a landed vocabulary flip
on a live computer without an object-graph inverse is the charter's own
known-broken state.

Repair order recorded in the Definition: object-graph vocabulary boundary →
historic read acceptance → close live write roots → migration correctness →
verifier key unification → gate breadth → full deployed matrix and fetched
artifact → settle.

## 7. Repair-wave census addendum (2026-09-10, post-review)

Direct evidence gathered on Node B against the retained computer's store
(quarantine image `data.img.quarantine-1-2864e5d885aa3f56`, Sep 9, static;
the live image's `vocab-migration-report.json` reads `{"provenance":{},
"counts":{}}` — the migration ran and found zero rows).

### C1 — Present-tense F1 confirmation

`og_objects` in the computer store holds V1 desk tokens on the live carrier:

| object_kind | V1-token rows | dominant carriers |
|---|---|---|
| choir.event | 23,139 | metadata/body `agent_id`, `channel_id` (`co-super:`/`super:` prefixes) |
| choir.lifecycle_command | 2,732 | nested StoredResult role/ID fields |
| choir.lifecycle_event | 2,732 | body `AgentID`, refs |
| choir.run | 1,871 | `agent_profile`/`agent_role` = `super`/`co-super`/`researcher`; `agent_id` |
| choir.agent | 803 | `agent_id` = `co-super:assignment-*` (800), `super:` (1), `researcher:` (2) |
| choir.co_super_assignment | 800 | binding IDs, nested grant attestation `Role` |
| choir.channel_message | 55 | `role` = `co-super` |
| choir.worker_update | 52 | `role` = `co-super` |

All six migrated relational tables (`agents`, `runs`, `channel_messages`,
`inbox_deliveries`, `work_items`, `worker_updates`) contain **zero rows** in
the computer store. The fence verified an empty set.

### C2 — New finding: `run_memory_entries` is a live carrier

34,781 rows; `agent_id` prefixes: `super` 26,949, `co-super` 7,208,
`texture` 516, `researcher` 76. The table is live — written by
`internal/store/run_memory.go:131` and `project.go:316`, read at
`run_memory.go:55-131` and `residue_import.go:434`. It was absent from both
the migration targets and the review's F1 table list. Its `role` column is
message-role vocabulary (`assistant`/`user`/`runtime_injection`), not desk
vocabulary — migrate `agent_id`, do not fence `role`.

### C3 — New finding: rematerialize witness ordering defect

`internal/agentcore/rematerialize.go:150-167` extracts the staged replay
witness and compares it against the checkpoint's `VMLocalContentWitness`
**before** `MigrateAndFenceServingVocabulary` runs at :171. Once the live
store is OG-migrated, a staged V1 replay can never match the live witness —
every rematerialize would fail. The migration must precede witness
extraction on the staged side. (`projectionbase/rebuilder.go:91-114` already
has the correct order: ReconstructThrough → MigrateAndFence → extract.)

### C4 — Tombstone reads are inconsistent; in-place migration required

`GetObject` (objectgraph/dolt_store.go:140), `ListObjectsByMetadataPage`
(:479), and `ListObjectsByOwnerAndBody` (:511) return tombstoned rows. A
copy-forward + tombstone migration would leave V1 rows serving through
direct-ID and metadata lookups — non-compliant. The OG migration must be an
in-place rewrite: body/metadata fields, recomputed `content_hash`, rewritten
`canonical_id` where the identity key is desk-bearing, `og_edges`
`from_id`/`to_id` rewrite plus `edge_id` recomputation, and body-embedded
canonical references (ArtifactRefs, ReportRefs, CandidateID, source-graph
refs) rewritten to the new IDs. Because referencing objects' own
content-derived IDs change when their embedded refs are rewritten, the
canonical-ID map is a bounded fixpoint computation, not a two-pass rewrite.

### C5 — Replay emits raw V1; migration covers it positionally

`projectObject`/`projectObjectEdge` (store/project.go:208-280) write
tape-supplied `canonical_id`/`content_hash`/`body`/`metadata` verbatim — no
recomputation. Replayed OG rows arrive V1-stamped; the post-replay
`MigrateAndFenceServingVocabulary` call sites (autoputer/run.go:520,569;
rematerialize.go:171; rebuilder.go:112) are correctly positioned to cover
them once the migration covers OG.

### C6 — No tape-digest equality on OG bodies

No code compares `og_objects.content_hash` or OG body digests against tape
event/artifact digests. Tape integrity is a separate digest domain
(computer_events.go:34-36, payload_resolver.go:73-80). Migrating OG event
bodies does not invalidate tape digests. Witness equality operates on
whole-table hashes (`dolt_state_extractor.go:342-398`), so live and replayed
stores converge when both run the same migration.

### C7 — `computer_event_index.event_json` is a tape mirror — do not migrate

257 rows contain V1 tokens inside `event_json`; this column mirrors
immutable tape event bytes and must stay V1. The fence must not scan it.

### C8 — Comparison-site classification (item 11 scope)

~150 `== agentprofile.*` sites classified: nearly all are OG-backed or
live-request comparisons correct post-migration. Only two sites read
immutable historic bytes and need the frozen V1 interpreter:
`platform/checkpoints.go:217` and
`agentcore/self_development_decision_binding.go:54`. Mixed/unknown sites
(runtime.go:342,385,1332; tools_worker_update.go:468; texture_turn.go:728)
resolve to V2-domain post-migration.

### C9 — Owner TOML verifier spelling

The computer-owned `model-policy.toml` uses `[roles.verifier_multimodal]`
(underscore). Canonical spelling for item 14: `verifier_multimodal`;
`NormalizeRole` maps both spellings to it.
