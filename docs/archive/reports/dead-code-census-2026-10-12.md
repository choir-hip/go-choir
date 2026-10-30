# Dead-code census, 2026-10-12

Station D0 of the re-engineering metamission
(docs/archive/definitions/choir-reeng-metamission-2026-10-12.md). Owner, 2026-10-12:
"identify all the dead code and remove it, after I learn about it and what it
does/did." Nothing is removed until the owner rules on this document.

Five read-only census agents (Sonnet) each covered one area at c30147a6. Their
raw reports (Go static reachability, frontend, scripts/tests/data, and the
50k-line per-function list) were removed in the 2026-10-14 docs prune because
the deletions they drove have landed; they remain in version control history
(see `archive/PRUNED-2026-10-14.md`). Two are kept in
[dead-code-census-2026-10-12/](dead-code-census-2026-10-12/):

| Area | File |
|---|---|
| HTTP routes with no caller | [routes.md](dead-code-census-2026-10-12/routes.md) |
| Desk verbs, prompts, metadata, env, nix | [desk-config.md](dead-code-census-2026-10-12/desk-config.md) |

This file merges the five reports by subsystem, removes the overlaps and asks
the owner the questions that a grep cannot answer.

## Owner rulings (2026-10-12)

- **Qdrant is kept, all of it** (A4 withdrawn): the package, the config,
  the nix service and the boot-time collection. "Once automatic newspaper
  is operating at scale, we're going to use Qdrant for a retrieval layer."
  It is dormant, not dead.
- **PII redaction is kept** (B5): `internal/pii` and `trace/redact.go` are
  built but not wired, and the trace store they wrap is off. Cut 0 answers
  "are exported traces redacted?" before it ships trace export. B4 (the
  trace store) is decided in Cut 0 too. Owner, later the same day: PII
  redaction will run on a Jev-style private decision model, never an
  external API ("we can't call Jev over API for PII redaction; that would
  be the opposite of what we want"); the Ollama small-model redactor is not
  the path. Recorded in Cut 0 unit (1).
- **Auth recovery is kept and noted** (B8): production readiness needs an
  auth redesign, after Autopaper's functional requirements are met. The
  broken recovery flow, the passkey and session routes no UI calls, and
  the unreachable root status are inputs to that redesign, recorded in
  [docs/problems/auth-redesign-inputs-2026-10-12.md](../../problems/auth-redesign-inputs-2026-10-12.md).
- **Trusted email outbox** (B7): explained to the owner, who agreed with
  the recommendation: delete it (decisionpolicy stays; it is live in
  self-development approval). The receipt design is kept in
  docs/evidence/effects-trusted-outbox-2026-08-16.md.
- The owner explicitly and categorically approved all of the deletions
  above (2026-10-12).
- **Everything else: the recommendations stand** (Part A; B1, B2, B3, B6
  recovery and maild spool, B9 to B16; Part C). Mission probes (A11) still
  wait for Cut 0's tracer.

## Result (2026-10-12): landed

- **What landed.** 33 D0 commits, febe1df8..6505fa33. In that range, 383
  files changed: +702, -50,103 lines. The 32.8 MB binary is not counted in
  those lines.
- **How it ran.** Four removal batches, each written in its own worktree:
  - R1: hygiene, scripts and CI;
  - R2: frontend;
  - R3: Go outside the desk runtime;
  - R4: the desk runtime and prompts.

  Each batch was rebuilt and tested on main, then pushed and passed CI.
- **Final state.** CI run 38324849111 passed. Staging health and the
  acceptance computer's guest build are both 6505fa33.
- **Acceptance PASS.** `scripts/m0_pools_acceptance.mjs` ran on a fresh
  test computer (computer-695920eae4e77b5324166e20d3bc510b); artifact
  [docs/archive/evidence/d0-dead-code-acceptance-2026-10-12.json](../evidence/d0-dead-code-acceptance-2026-10-12.json).
  - Four documents authored in 24 to 45 s.
  - Both pool members used, 4 threads each.
  - Every thread kept one model, and nothing was refused.
- **Defects repaired:**
  - D1: compaction now names `choir.RunMemoryEntry` to research only.
  - D5: a run with an unknown profile gets no tool registry.
  - D6: no comprehensive-tagged tests remain; they were revived into CI or
    deleted.
  - Texture PUT and DELETE no longer write outside the Texture desk.
- **Defects discovered:**
  - D4 had already been repaired by b445fd21.
  - The live yaegi Session could report an output overflow as a timeout.
    Recorded and fixed: 86630cf3, then 5f135d8e.
  - The nix Go module hash went stale when imports were removed; fixed in
    f96ead40.
- **Introduced:** none known.
- **Moved into the cuts:**
  - Cut 0: the mission probes and the trace store decision.
  - Cut 1: the 17 legacy tables, management-open, control-records, the
    processor metadata keys, D3, and the spawn_agent flags.
  - Cut 2: the Universal Wire and texture merge frontend, the sourcecycled
    daemon, the 24 Store B tables, and D2.
- **Residuals (closed in the leftovers commit):**
  - `go.mod` tidied: gonso, go-keyring, wincred and the explicit yaml.v3
    dropped; the nix vendorHash was recomputed on Node B (x86_64).
  - `capsule.Executor.ListOwned`, `DestroyOwned`, `ControlHandle` and
    `InspectOwned` had no callers; deleted with their stubs and
    `CapsuleControlSummary`.
  - `SupervisionInterval` and `RUNTIME_SUPERVISION_INTERVAL` deleted from
    the config and every test.
  - Node B's gateway env now sets `GATEWAY_CHATGPT_MODELS=gpt-6-luna`
    (M0 residual closed).
  - Kept on purpose: the decision-policy JSON still names the solitaire
    tables in `blast_radius`. That JSON is hash-pinned and stored self-dev
    receipts cite its digests, so editing it would orphan those receipts.
  - Leftovers landed in 964bdd25; acceptance PASS on fresh computer
    computer-779ea55381c44b0bcbf4409c3c97351d (guest build 964bdd25):
    `docs/archive/evidence/d0-leftovers-acceptance-2026-10-12.json`.

## Size

- Go: 667 functions (about 11,200 lines) unreachable from any `main` on the
  linux build. 320 of them are dead even counting tests; 347 are kept alive only
  by their own tests. Five `internal/` packages have no importer at all, and
  five `cmd/` binaries are built by nothing.
- Routes: about 335 endpoints. About 69 are dead (no caller, an
  always-error stub, blocked by the proxy, or never mounted), and about 20 are
  called only by tests or probes.
- Frontend: about 670 lines of unreachable components, 15 dead exports,
  about 38 dead CSS classes, and about 7,500 lines of Playwright specs that
  click UI removed on 2026-05-29. Nothing runs them, because Playwright is not
  in CI.
- Scripts: about 12,800 of the 14,984 lines in `scripts/` serve missions that
  have landed or tools that are retired.
- Tests: 6,000 lines behind a `comprehensive` build tag nothing sets. Two of
  those files no longer compile.
- Schema: 24 Store B tables that no code reads or writes, and 17 legacy runtime
  tables with no production writer.
- A 32.8 MB Linux binary (`capsule-broker`) and a nix `result` symlink are
  committed at the repo root.

Rough total: 25,000 to 30,000 lines can go before Cut 0 without losing
anything that works.

## How to read the rest

- **Part A** is what I propose to remove without asking. Nothing works today
  that would stop working. Read it to learn what each piece was.
- **Part B** holds the owner questions: built for a purpose that might come
  back. Each has a recommendation.
- **Part C** lists items the cuts already delete, where the cut text misses
  part of them.
- **Part D** lists live defects found along the way. They are not dead code,
  but they are wrong today. Each needs a problem record before its fix.

---

## Part A: proposed removals (no working behavior lost)

Each entry: what it was, why it is dead, and how big it is.

### A1. Accidents in the repo root

- `capsule-broker`: a 32.8 MB Linux build of `cmd/capsule-broker`, committed by
  accident in 19e2c256 (2026-09-27). Nix builds the real one from source. The
  32 MB stays in git history unless history is rewritten, which is not
  proposed.
- `result`: a nix build symlink into `/nix/store`, committed 2026-04-11 and
  dangling everywhere else.
- Both names get added to `.gitignore`.

### A2. The wire processor and reconciler leftovers

Universal Wire's ingestion pipeline (May to October 2026) worked like this:
sourcecycled fetched items, a "processor" desk decided whether an item was
news, and a "reconciler" desk merged items into stories. 3c963edf (2026-10-05)
deleted the pipeline from end to end, but it left function bodies behind.

The leftovers:
- The typed processor-decision work items (`wire_publication.go`, 10 dead
  functions).
- The source-item helpers, and the internal-submission fingerprints in
  `api.go`.
- `POST /internal/runtime/runs`, which now always answers 400 ("internal
  ingestion profiles are retired"), and its status and cancel routes.
- The completion gate: before a run could finish, it waited up to 5 minutes
  for `required_child_runs` and `required_texture_revisions`. No code writes
  those keys, so the gate always passes at once.
- The batch-executor guards for tools that no longer exist: `bash`,
  `write_file`, `update_coagent`, and a management `spawn_agent`.
- About 1,400 lines in all. Cut 1 deletes the live remainder (autonomous
  publish and claim recovery). Removing these now shrinks Cut 1.

### A3. The coagent source-packet layer

Builders for `update_coagent` payloads. Under the old protocol, a desk
reported findings by sending a typed "source packet" to its supervisor.
`update_coagent` registration was deleted in 3c963edf, so these builders have
no caller. They exist twice, once in `agentcore` and once in `textureowner`.
That is about 665 lines, plus Texture's media-source prompt enrichment (about
260 lines with no caller). Cut 1 deletes the packet types themselves.

### A4. The Qdrant and Ollama semantic dedup

Mission 3b (June 2026) dropped near-duplicate source items before processor
dispatch. It embedded each item with Ollama and searched a Qdrant collection.

- The dedup call has no caller.
- What still runs is a best-effort "create the Qdrant collection" at every
  guest boot, which guests cannot even reach (Qdrant listens on Node B's
  127.0.0.1).
- Ollama is provisioned nowhere.

Removal covers `internal/qdrant` (2,006 lines), `qdrant_dedup.go`,
`qdrant_runtime.go`, four config fields, the readiness probes,
`docker-compose.qdrant.yml` and the qdrant service in `nix/node-b.nix`.

Removing the nix service is a Node B deploy change; it ships through the normal
deploy, not by hand.

### A5. The typed-tool registries that are built and then thrown away

Before desks ran as Go cells, each desk got typed JSON tools: `read_file`,
`glob`, `grep`, nine research tools, the evidence tools,
`get_run_memory_entry` and `verify_model_capability`. Today
`InstallDefaultAgentTools` still builds those registries for management,
Texture, email and research, then overwrites each one with the cell-only
registry.

The leftover management registry survives as the fallback for an unknown
profile. If such a run ever existed, it would get full typed-tool authority
(see D5).

Removal: the registrations, the `read_file/glob/grep` tools, the `Allow*Tools`
policy flags and their owner-visible settings. The tool constructors stay,
because the cell verbs reuse them.

### A6. Four capsule tools no desk can call

`spawn_capsule`, `destroy_capsule`, `list_capsules` and `inspect_capsule`
(July 2026, capsule runtime v14) let the conductor manage isolated capsules.
Their registrar's last caller was removed on 2026-07-19. Engineering's
`capsule_go_eval` stays. About 170 lines.

### A7. Yaegi kernel prototypes

Phases 0 to 3 of the in-cell interpreter (August 2026):
- an in-process evaluator;
- a sidecar subprocess per activation;
- durable actor state with rewarm;
- receipt transclusion into Texture.

`desk_session_worker` replaced all four. They are kept alive only by their own
tests (about 960 lines). The export table that desks see is static and does
not reference them.

### A8. Older dead Go, grouped by origin

- **SQLite-era row scanners in `internal/store`** (41 functions, 1,030 lines):
  orphaned when storage moved to the object graph (63dca14d, b307dfab).
- **`provider/bridge.go`** (570 lines): the April 2026 Bedrock and Z.AI bridge,
  replaced by `gatewayruntime`.
- **`gateway/client.go`** (303 lines): the April 2026 host-side gateway client,
  replaced by `gatewayruntime` and `hostlink`.
- **`trace/query.go`**: a trace HTTP query API that was never mounted (added
  2026-06-28). Its `/api/trace/` prefix is still in management's ProductAPI
  allowlist and prompt, pointing at nothing.
- **`run_acceptance.go` helpers** (10 functions), **`value_helpers.go`** (12),
  **`capsule` diagnostics and namespaces** (overlayfs era), and about 120 stray
  helpers across `agentcore`, `textureowner`, `platform`, `vmctl` and others
  (listed per file in go-static.md).
- **Five unused tool-loop options**: the typed-tool-era "force the first tool
  call" and "require a write tool" options.

### A9. Dead routes

- **Solitaire** (`/api/solitaire/*`, 911 lines): a Klondike engine the
  engineering desk built as "Candidate A" for the August self-development
  proof. Nothing calls it. Its two tables must also leave
  `replay_eligibility.go` in the same commit. See B9.
- **Self-development genesis import** (about 290 lines): the proxy has
  answered 409 for it since 2026-08-16.
- **The Texture document routes (PUT, DELETE, diff, blame)**: a title rename, a
  delete, and the line diff and blame views from the April editor. The
  frontend functions that called them have no importer. PUT and DELETE also
  write a Texture outside the Texture desk.
- **Three 404 tombstones**: `tell`, `correct` and `agent-revision`.
- **Mail**: `ingress-events`, `source-packet`, and `completion-email`. The
  last one linked to the Features app, deleted 2026-07-19.
- **Content and browser read routes with no caller**: `import-file`, the
  content list, and the browser session list and get.
- **Internal runtime routes**: `channel-casts`, the internal run status and
  cancel routes, and `GET /internal/source-service/health`.
- **Proxy tombstones**: `/api/terminal/ws` (410) together with the guest's
  unregistered terminal route, `/api/system/*` (404), and
  `HandlePublicTexture`.
- **Gateway**: `credentials/revoke` and `credentials/rotate`, which have never
  had a caller.

### A10. Frontend

- **Unreachable components:**
  - `DesktopIcons.svelte`, the April left rail.
  - `Window.svelte`, the first window component, replaced by `FloatingWindow`
    the next day.
  - Six `apps/App*` UI primitives that no app adopted.
  - About 670 lines in all.
- **Dead exports and fields:**
  - 15 exports with no importer (mostly `texture.js` clients for the routes in
    A9).
  - The `minimizedWindows` store.
  - The registry fields `auth.requiresAuthFor` and `launcher.mobileSwitcher`.
- **Dead CSS:** about 38 classes in `app.css` for removed apps (trace,
  features, swimlane, the old mail rail).
- **Dead page code:** the `App.svelte` legacy public-reader block (about 225
  lines), which only prints "Legacy publication links were removed".
- **Debug scripts:** 11 April debug `.mjs` scripts in `frontend/` that call
  routes that no longer exist.

### A11. Scripts, tools and tests

- **Retired gates:**
  - The V1 vocabulary inventory gate (4 scripts). CI dropped it the day it
    was added.
  - `scripts/lint/cts-receipt-lint.py`, which reads a deleted file.
  - The one-time staging-recovery workflow from the July 23 incident.
  - `deploy-impact-classify` arms for packages that no longer exist.
- **Runtime ratchet:** `cmd/runtime-ratchet` and
  `docs/runtime-dissolution-inventory.yaml` (2,700 lines plus a 474 KB
  baseline). This was the July ratchet for dissolving `internal/runtime`. The
  package is gone, and the tool fails at HEAD with 2,849 mismatches. CI does
  not run it.
- **Experiment tests:**
  - Two Dolt branch tests, still run in CI, whose stated conclusions were
    retracted.
  - The `experiment`-tagged concurrency test.
- **Empty fixtures:** the ten empty `testdata/rlm_replay` directories.
- **Packages nothing imports:**
  - `internal/diskinstantiation`, the audited ext4 realization contract
    (orphaned 2026-07-19).
  - `internal/base/testkit`.
  - `internal/pii` together with `trace/redact.go`, a PII redaction pipeline
    for traces that was never constructed in production. See B5 first.
- **Mission probes** (about 3,800 lines; `s0m_*`, `smg_*`, `m3`, `m4`, `m9a`,
  `s1a`, `s2_*` and the cadence probe): acceptance probes for stations that
  have landed. They exercise packets, `update_coagent` and `/api/trace`.
  - Remove them after Cut 0's tracer lands.
  - Before removing, read `s0m_*` and `smg_*`: they hold the shortest recorded
    form of each desk handshake.
  - Removal waits on the tracer, but the decision to remove is made now.
- **Report generators:** the 19 `generate_*.py` PDF generators, each with one
  fixed input; their reports are done.

### A12. Config and env with no effect

- **Unused config fields:** `ResearchCount` (logs "researchers=3", a Universal
  Wire topology), `SupervisionInterval` and `TextureWakeDebounce`.
- **Flag with no setter:** `RUNTIME_DISABLE_TOOLS`, never set, so its branch
  never runs.
- **Env with no reader:**
  - `RUNTIME_SUPER_FOREGROUND_MUTATION_MODE`, `RUNTIME_LOCAL_WORKER_MODE` and
    `RUNTIME_SELF_URL` in `start-services.sh`.
  - The two `SOURCE_SERVICE_AGENT_DISPATCH_*` vars.
  - `CHOIR_COMPUTER_KIND` (and its kernel parameter) and
    `CHOIR_KERNEL_CAPABILITY_PROBE`.
  - `RUNTIME_ZAI_MODEL` and `RUNTIME_BEDROCK_MODEL`.
- **Event kinds:** `desktop.state.updated` and `computer.status.updated` are
  never emitted, along with the frontend branches that wait for them. The
  continuation-event case in run acceptance is dead too.

### A13. Prompt text describing things that do not exist

These fixes change prompt text (green or yellow class). They are corrections,
not tuning.

- **`run_memory.go`:** compaction checkpoints tell every desk to call
  `get_run_memory_entry`, a tool no desk has. Only research has a similar
  verb, `choir.RunMemoryEntry`. See D1.
- **`skill_context.go`:** injects "Use `mission-gradient`…" into four desk
  prompts. That skill was archived on 2026-10-04.
- **`researchtools.go`:** search results carry a `next_instruction` telling
  research to "submit a concise first update_coagent source packet".
- **Texture's revision policy:** a template branch for
  `explicit_researcher_request`, a key nothing writes.
- **An overlay file** that nothing renders.
- **ProductAPI allowlist:** the prefixes `/api/trace/`, `/api/continuations`
  and `/api/universal-wire/`, and the management-prompt sentence that lists
  them.
- **Most other dead prompt text** goes with Cut 1's prompt rewrite (Part C).

---

## Part B: owner questions

Each of these was built on purpose and might return. A grep cannot decide them.

1. **Base file sync** (June 2026): a Dropbox-like sync product.
   - **Parts:** the three-tree planner, the journal, a blob store, the
     `/api/base/*` REST API, the Wails desktop sync engine, a macOS Finder
     File Provider (`macos/`, 7 files), and three harness CLIs.
   - **How it died:** the desktop engine was unregistered on 2026-07-10. The
     API lost its last deployed host on 2026-07-19. The 2026-08-29 review
     already classed it safe to delete. About 2,400 lines plus 2,300 of tests.
   - **Keep:** `internal/base/{blob,journal,model,tree}`. `computerversion`
     still imports them (see B2).
   - **Recommendation:** remove, unless Finder integration is on the roadmap.
2. **Proof machinery in `computerversion`** (July 2026): the "audited
   computer" proof line.
   - **What it did:** it generated a filesystem from a version tree, checked
     "user isomorphism" across substrates, and issued promotion certificates.
   - **Status:** frozen 2026-07-19. About 1,300 lines are kept alive only by
     tests. `diskinstantiation` (A11) belongs to the same line.
   - **Recommendation:** remove, and keep the ComputerVersion ref types.
3. **sourcecycled** (cmd, `internal/cycle`, `wire/processorkey`; about 3,500
   lines plus 1,500 of tests): the source-polling ledger daemon.
   - **Status:** built and deployed, but deliberately off since 2026-10-05.
     Autopaper's source schedules (`/api/autopaper/sources/schedules`,
     corpusd captures) appear to replace it. Its name is retired, and it reads
     every setting under two names (`SOURCE_SERVICE_*` and `SOURCECYCLED_*`).
   - **Recommendation:** remove the 41 statically dead functions now. Remove
     the daemon when Cut 2 confirms captures and schedules are the only
     ingestion path.
4. **The Dolt trace store** (`internal/trace`, June 2026): an append-only
   observability store for runtime events.
   - **Status:** `RUNTIME_TRACE_PERSISTENCE_ENABLED` is never set, so it is off
     everywhere. Its query API was never mounted.
   - **Why it matters now:** Cut 0 builds trace export.
   - **Recommendation:** let Cut 0 decide whether it is the export's store. If
     not, delete it.
5. **PII redaction of traces** (`internal/pii`): tied to B4. Is redaction a
   requirement before traces are exported? Recommendation: answer that in Cut
   0. Delete if no.
6. **`internal/recovery`** (August 2026: recovery capsule, restore drill,
   integrity scrub) and **`maild` spool and drain** (August 2026: an fsync'd
   host mail queue). Both were built and never wired. They look like future
   storage and mail infrastructure. Recommendation: delete; git keeps them.
7. **`internal/trustedoutbox` and `decisionpolicy`**: the irreversible-email
   decision policy from the effects rehearsal (August 2026). Its only caller is
   a test. Recommendation: delete, unless effects rehearsal returns.
8. **Account recovery, passkey and session management**:
   - **Routes:** 7 auth routes and `/api/auth/root/status`.
   - **Why broken:** recovery creates a token but never emails it, so it cannot
     work. No UI ever called these routes. Root status is unreachable, because
     Caddy sends `/api/*` to the proxy.
   - **Why it matters:** they are unauthenticated-reachable routes.
   - **Recommendation:** delete recovery and root status now. Keep the
     passkey and session list only if a Settings panel for a second device is
     planned.
9. **Solitaire** (A9): the self-development proof artifact from August.
   Recommendation: delete. The proof receipts stay in docs.
10. **Operator break-glass routes:**
    - **Routes:** vmctl `hold`, `divergence` and `maintenance-serve`; gateway
      search health, breakers and `/health/{service}`; platform key-escrow
      unwrap and the transparency head.
    - **How they're used:** nothing calls them, by design. An operator curls
      them.
    - **The tension:** the standing "no-SSH operability" question argues
      against them, but removing the key-escrow unwrap would remove the only
      key recovery path.
    - **Recommendation:**
      - Keep key escrow, and write its runbook.
      - Delete vmctl hibernate, resume, recover, logout, prune and
        `runtime-package`. These are April lifecycle endpoints with no caller,
        and guests no longer fetch a runtime package.
      - Lock the gateway ops routes to a caller check (open problem
        `docs/archive/problems/search-plane-cooldown-on-caller-cancel-2026-10-09.md`).
11. **The `comprehensive` test tag** (14 files, 6,000 lines). Recommendation:
    - Delete the two files that do not compile.
    - Drop the tag from the ones that pass (`apihandler`, `content`,
      `mediastate`, most of `agentcore`), so CI runs them.
12. **The stale Playwright specs** (about 7,500 lines across 17 files):
    - **What's wrong:** they click a Start menu, window switcher and logout
      button that were removed on 2026-05-29.
    - **Why nobody noticed:** Playwright is not in CI.
    - **The decision:** is Playwright the acceptance mechanism? CLAUDE.md says
      "highly prefer E2E".
    - **Recommendation:** delete the stale tests. Cut 0's tracer becomes the
      acceptance harness, and the deployed `*-deployed.spec.js` specs stay.
13. **TLA+ specs** (`specs/`): they model passivation and packet delivery,
    which Cut 1 replaces. Their own README calls them historical, and CI still
    downloads TLC for them. Recommendation: delete them with the CI job.
    After Cut 1, write one spec for cursor delivery if it earns its keep.
14. **Store schema:**
    - **Store B:** 24 tables (relational publication, provenance, consent and
      retrieval) that no code reads or writes.
    - **Legacy runtime:** 17 tables with no production writer.
    - **What removal touches:** the replay-eligibility manifest (a
      hash-checked contract) and existing guest stores. This is red class.
    - **Recommendation:** remove the Store B DDL now (after an empty-count
      check). Fold the 17 runtime tables into Cut 1, which deletes their last
      readers.
15. **Smaller scripts:**
    - the Node B storage report family (2,927 lines, last relied on in July);
    - the mail provider setup scripts (Resend and Gandi, May);
    - the Dolt authority-split tools (September);
    - `cmd/choir-rebuild-base` (an operator tool, edited 2026-10-08);
    - `scripts/go-test-local`.

    Recommendation: keep `choir-rebuild-base` and the mail scripts, delete the
    rest.
16. **Ledger acts with no consumer:** Precommit, Resolve, Disagreement and
    Escalate. These are the precommitment mechanism (the product), but no
    scorer exists. Texture and engineering never teach them, and the
    supervision-pack readers are dead. Recommendation: not dead code. Cut 1's
    prompt rewrite either teaches them or stops exporting them to desks that
    do not use them.

---

## Part C: cuts that miss part of what they delete

- **Cut 2, Universal Wire:** also delete the `App.svelte` legacy reader (or
  remove it now, per A10), the `read_owner` plumbing in `texture.js` and
  `TextureEditor`, and the two Universal Wire specs.
- **Cut 2, texture merge:** also delete the merge wiring in
  `TextureEditor.svelte` (about 190 lines), `TextureToolbar.svelte` (about
  80), and the three `texture.js` fetchers. If the panel goes alone, the build
  breaks.
- **Cut 1, `spawn_agent`:** Cut 1 says spawn_agent is registered for "the
  conductor and the base engineering registry". It is registered only for the
  conductor, because engineering's policy has no spawn targets. Also delete
  `AllowedSpawnTargets`, `CanSpawn`, the `allow_coagent_tools` flag and the
  `cancel_agent` registration.
- **Cut 1, work items and packets:** the relational `work_items`,
  `worker_updates`, `inbox_deliveries` and `run_continuations` tables (B14),
  and `POST /api/texture/management-open` and
  `GET /internal/runtime/control-records`. Both serve the control accumulator
  Cut 1 replaces with Casts.
- **Cut 1, processor metadata keys:** the three metadata allowlists
  (`durableMetadataKeys`, `api.go`, `texture_task_types.go`) and the handoff
  copy in `coagent_route.go`.

---

## Part D: live defects found along the way

Each needs a problem record before any fix (CLAUDE.md: problem documentation
first). The records, with evidence and owners, are in
[docs/problems/dead-code-census-live-defects-2026-10-12.md](../../problems/dead-code-census-live-defects-2026-10-12.md)
(D1 to D6). D7 is in the M0 acceptance section of
docs/model-pools-design-2026-10-12.md.

1. **Compaction points at a missing tool.** After a context rollover, a desk
   is told to call `get_run_memory_entry` for raw content. No desk has it, and
   Texture, management and engineering have no retrieval verb at all.
   Hypothesis, unconfirmed by traces: compacted desks lose detail they were
   told they could recover.
2. **The proxy calls a deleted route.** Every reader proposal
   (`POST /api/platform/publications/{id}/proposals`) is forwarded to the
   author's guest `/internal/texture/proposals`, which was deleted in R3d. The
   call always fails, and the delivery state is recorded as a constant.
3. **Research can Cast.** The research module set exports `Cast`, and
   `castOpensEngineeringAssignment` does not check who the caster is, while
   the research prompt says research does not cast (Study G E4 and R2). This
   is an authority gap.
4. **Gateway ops routes have no caller check.** Search health and reset and
   breaker reset can be reached from any computer's tap network (open problem
   doc, 2026-10-09).
5. **An unknown profile gets the full typed-tool registry.** A run with an
   unrecognised profile falls back to the leftover management registry (A5).
   No such run is created today, so this is latent. It should fail closed.
6. **The `comprehensive` tests no longer compile**, and Playwright never runs.
   The test suite reports less than it appears to.
7. **The Node B gateway env still lists retired models**
   (`GATEWAY_CHATGPT_MODELS=gpt-6-luna,gpt-5.5,gpt-5.4,gpt-5.4-mini`).
   Harmless, because the catalog refuses them, but stale. Rerunning
   `nix/deploy-provider-creds.sh` (an owner host step) rewrites it. This was
   found during M0 acceptance, not by the census.

---

## Removal plan (after the owner rules)

The commits are grouped so each one can be reverted alone. Each is compiled,
vetted and tested, and goes through the Landing Loop where runtime behavior is
touched.

1. Root accidents and `.gitignore` (A1). Green.
2. Spent scripts, gates, workflow, ratchet, frontend debug scripts and empty
   fixtures (A11, without the probes). Green or yellow.
3. Dead Go with no runtime effect (A2, A3, A6, A7, A8, plus the Part B
   packages ruled "delete"), one commit per subsystem. Orange: they compile
   out, but `agentcore` and `textureowner` are protected files.
4. Dead routes and their frontend clients (A9, A10). Orange.
5. Qdrant and the nix service (A4), config and env (A12). Orange, and a Node B
   deploy.
6. Prompt-text corrections (A13). Yellow, and a deployed check that the desks
   still run.
7. Problem records for Part D, then the fixes, one at a time.

The acceptance is the M0 probe (`scripts/m0_pools_acceptance.mjs`) on a fresh
test computer after steps 3 to 6: documents still get authored, and nothing
regresses.
