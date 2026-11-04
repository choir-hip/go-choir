# Custody inventory: who reads what across desk boundaries (origin/main 416b30293)

Read-only inventory. Source: `git show` / `git grep origin/main`. The working tree (cutover/data-system) was not read.
Marking: **[V]** = verified at the cited file:line on origin/main. **[H]** = hypothesis (inferred, or a negative result from grep that I could not prove exhaustively).
Paths are repo-relative. "OG" = object graph (`og_objects`), "tape" = `choir_events`.

---------------------------------------------------------------------------
## 0. The shape of the system that matters for a read-custody rule

1. **One store, no per-kind isolation.** The guest has one SQLite file: relational tables in `internal/store/schema.go` (events :78, run_memory_entries :94, texture_* :356-424, agent_evidence :441, desktop_* :195-237, self_development_* :300-308) plus the object graph (`og_objects`, `og_bodies`, `og_edges`: `internal/objectgraph/sql_store.go:230,257,288`) plus the tape (`choir_events`: schema.go:33). Every Go service holds a `*store.Store` with full read access. [V]
2. **Cells have no store handle.** A desk cell runs in a worker with its own uid, empty netns, Landlock and seccomp; "every effect is a frame on the inherited fd 3 socket that the host answers" (`internal/yaegikernel/desk_cell_confinement.go:15-28`). So a desk's *cell-level* reads are exactly: (a) the preloaded frame bindings (`Records`, `Pack`, `ReadDoc`, `StoryAnalysis`, `Calibration`, `Context`), (b) host-served actions from `deskmanifest.verbDispatch` (`internal/deskmanifest/manifest.go:~120-141`), (c) `ReadFile/ListDir`, which are local and bounded to `deskWorkerRoot` = run `WorkingDir` else `os.TempDir()` (`internal/agentcore/tools_desk.go:1022-1027`). `tool_cwd` is read at `tool_profiles.go:51` but I found no writer anywhere in `internal` or `cmd`, so desk ReadFile does not reach the owner's files root. [V for the code, H for "no writer"]. This makes cell-level custody enforceable at one choke point (the `FrameActions` table), but *Go-service-on-behalf* reads are the large part of the surface and are not gated by it.
3. **Write custody exists, read custody does not.** `objectgraph.KindRegistration.Writer` names one writer per kind (`internal/objectgraph/registry.go:36-52`, `writer.go:44-65`). The gate default is *observe* (log and allow), not enforce (`writer.go:111-145`). `deskmanifest.Desk.OwnedKinds` is "declared and parity-pinned; no consumer reads it yet" (`manifest.go:~96-100`). [V]
4. **Most kinds have no desk as writer.** In `registry.go`, only these have a desk writer: Texture (`choir.texture_document` :97, `choir.texture_revision` :98, `choir.texture_decision` :99, `choir.source_entity/source_ref` :76-77, `choir.citation_check` :149) and Research (`choir.agent_evidence` :100). Everything else in the VM store is `WriterRuntime` ("the VM store acting for the desks"): commitment_record :150, work_item :94, trajectory :93, assignments :166-175, capture_label :120, world_event :123, source_schedule :108, etc. Captures/publications are `WriterCorpusd` (:78, :178-222). So a custody rule needs a decision for every `WriterRuntime` kind about *which desk is custodian*. [V]
5. **Mail is not in the guest at all.** There is no mail verb, tool, or store call in `agentcore`, `yaegikernel`, `textureowner`, `store`, `objectgraph`, `search`, `deskmanifest`, `runtimeprompts` (case-insensitive grep for `mail`/`maildir`/`/api/email` returns only `internal/autoputer/run.go:651-655`). The `email` desk exists in the manifest with `CarrierNone` and an empty tool registry (`manifest.go` Email row; `tool_profiles.go:411,421`). [V for the matches, H that grep was exhaustive]. Mail custody is already de facto a host service (maild).

---------------------------------------------------------------------------
## 1. Data kinds: storage and writer

### 1.1 Mail (maild, host side)

| Item | Where stored | Writer | Readers (see 2.1) |
|---|---|---|---|
| Inbound/outbound messages, recipients, attachments, source packets, ingress events | maild per-owner mailbox SQLite: `email_messages` store.go:680, `email_message_recipients` :704, `email_attachments` :711, `email_source_packets` :724, `email_ingress_events` :733 (`internal/maild/store.go`) | maild ingest (`internal/maild/ingest.go:43`, webhook.go) | owner via `/api/email/*` |
| Drafts, approvals, edit requests, risk alerts | same mailbox DB: `email_drafts` :743, `email_draft_approval_events` :765, `_tokens` :775, `email_risk_alerts` :789, `email_draft_edit_requests` :800 | maild drafts.go / send.go / approval_reply.go; owner act | owner via `/api/email/drafts*` |
| Addresses, aliases, policies, whitelist, webhook index, owner addresses | maild global routing DB: `email_aliases` :339, `email_receive_policies` :326, `email_sender_whitelist` :353, `email_webhook_events` :362, `email_owner_addresses` :388; per-owner isolation stated at store.go:24-27 | maild (public_addresses.go, numbers.go, user_numbers.go); auth service posts sign-in/recovery/assign-numbers (`internal/auth/*_mail.go`, `number_reconcile.go`) | owner; auth over loopback |
| Digest state | `digest_subscriptions/_runs/_sent` (`internal/maild/digest_store.go:32,38,57`) | maild | owner `/api/email/digest` |
| Guest Maildir (raw .eml under files root `mail/`) | files root, `ownerfiles.MailDir = "mail"` (`internal/ownerfiles/ownerfiles.go:41`) | `POST /api/mail/inbound` (`internal/autoputer/maildir.go:215`); I found **no caller** of that route [H] | Files HTTP route lists it (no carve-out, `autoputer/files.go:607-608`) [V]; engineering capsule snapshot excludes it (`capsule/computer_state.go:45`, `computer_state_linux.go:40`) [V] |

Custodian today: maild (host process). No desk, no yaegi verb.

### 1.2 Texture state

| Item | Where stored | Writer |
|---|---|---|
| Documents, revisions, decisions | OG kinds `choir.texture_document/_revision/_decision` (registry.go:97-99). Legacy relational DDL `texture_revisions` (schema.go:356), `texture_document_aliases` :407, `texture_agent_mutations` :424 still exist; reads go through OG (`store/texture.go:143-158` lists `ogKindTexDoc`) [H that the relational tables are residual] | Agent: Texture desk (`desk:texture`) via `texture_turn.go:323` `WithWriter(WriterTexture)`; owner: `OwnerEdits` path (registry.go:97-98) |
| Lineage / lifecycle (trajectory-bound docs, `GetLifecycleDocument`) | same OG kinds, lifecycle getters in `store/lifecycle*.go` | runtime store for Texture |
| Evidence/citations/source graph | `choir.source_entity`, `choir.source_ref` (registry.go:76-77), `choir.citation_check` (:149), `choir.support_verdict` (:163), schema.go:374,387 | Texture / runtime |
| Texture root marker | `choir.texture_root` (registry.go:115) | runtime |
| Search index | derived in-memory BM25 over doc heads (`store/search_index.go:15-29`), "derived, holds no authority" | n/a (cache) |
| Files-tree shortcut | `ensureTextureManifest` writes a `{doc_id,title,source_path}` JSON into the files root (`textureowner/texture_import.go:117-123,491-523`): titles only, not content | Texture handler |

### 1.3 Commitment ledger and inboxes

| Item | Where stored | Writer |
|---|---|---|
| Commitment records | OG `choir.commitment_record` (registry.go:150), `store/commitment.go:85,121,258` | `WriterRuntime` (reducer, `lifecycle_commit_act.go`) |
| Inbox head/cursor/command, conversation head, lifecycle events/commands/sequence, actor wake outbox | `choir.inbox_*`, `choir.conversation_head`, `choir.lifecycle_*`, `choir.actor_wake_outbox` (registry.go:147-158) | `WriterRuntime` |
| Channel messages / coagent mailbox | `choir.channel_message` :90, `choir.coagent_mailbox` :107 | `WriterRuntime` |

### 1.4 Captures

| Item | Where stored | Writer |
|---|---|---|
| Web captures (shared content) | Store B via corpusd: `choir.web_capture` (registry.go:78); research writes through `researchtools.NewCorpusdCaptureSink` (`agentcore/tool_profiles.go:382`) | `system:corpusd` |
| Capture labels / overrides (Jev) | `choir.capture_label(_override)` (registry.go:120-121) | `WriterRuntime` |
| Capture dispositions (management) | `choir.capture_disposition` (:148), `store/management_tasks.go:41-43` | `WriterRuntime` |

### 1.5 Work items, trajectories, runs, run memory

| Item | Where | Writer |
|---|---|---|
| Runs, agents, events, trajectories, work items, run acceptance, continuations | OG `choir.run/agent/event/trajectory/work_item/run_acceptance/run_continuation` (registry.go:87-96); relational `events` (schema.go:78) | `WriterRuntime` |
| Run memory entries | OG `choir.run_memory_entry` (:92) + relational `run_memory_entries` (schema.go:94) | `WriterRuntime` |
| Research evidence | OG `choir.agent_evidence` (:100) + relational `agent_evidence` (schema.go:441) | **Research desk** (`store/graph_home.go:90` `WithWriter(WriterResearch)`) |

### 1.6 Engineering assignments and capsules

| Item | Where | Writer |
|---|---|---|
| Assignments, reports, subject candidates, capability/capsule/run claims, slots | `choir.co_super_assignment*`, `_subject_candidate`, `_capability_claim`, `_capsule_claim`, `_slot`, `_run_claim` (registry.go:166-175) | `WriterRuntime` (manifest says management owns `assignment`,`admission`, engineering owns `capsule_effect_bundle`: manifest.go Management/Engineering rows, undeclared at write sites) |
| Engineering lineage (handoff) | `choir.engineering_lineage` (:112) | `WriterRuntime` |
| Self-development operations | relational `self_development_operations` (schema.go:308) | runtime |
| Capsule filesystem, store copy, evidence bundles | capsule state dir; `/computer/state.db`, `/computer/files` (`capsule/computer_state.go:17-40`) | capsule executor |

### 1.7 Autopaper sources and stories

| Item | Where | Writer |
|---|---|---|
| Source schedules | `choir.source_schedule` (registry.go:108), `store/source_schedule.go:18` | `WriterRuntime`; owner via `POST /api/autopaper/sources/schedules` (`apihandler/routes.go:31`) |
| World events / membership | `choir.world_event`, `choir.event_membership` (registry.go:123-124) | `WriterRuntime` |
| Stories | Texture lifecycle docs (1.2) + `choir.story_library_head` (:162) | Texture |
| Published stories, entities, relations | corpusd: `choir.publication*`, `choir.entity*`, edges `related_story`, `mentions_entity` (registry.go:178-222, :306-310) | `system:corpusd` (guest exports via `story_publish.go`, proxy `types.StoryPublishPath` handlers.go:1791) |

### 1.8 App package object kinds

| Item | Where | Writer |
|---|---|---|
| `choir.app.<pkg>.<kind>` objects | OG, registered per package (`objectgraph/package_kinds.go:44`); store seam `store/app_package_objects.go:51-98` | Declared writer `desk:<pkg>`; the only live write path is the **owner** route `/api/app-packages/{pkg}/objects/{kind}` (`autoputer/app_package_objects.go:71-76,170,190`, setting `app_packages.objects`, store/app_package_objects.go:30). App-desk verbs cannot read them (`deskmanifest.appDeskVerbs`, manifest.go:471-475) [V]. Only a test fixture declares kinds (`apppackage/testdata/p2btest.choir-package.json`) [H] |

### 1.9 Other owner data

| Item | Where | Writer |
|---|---|---|
| Files (owner files root), file objects/paths | files root; `choir.file`, `choir.file_path` (registry.go:133-134) | owner via `/api/files`; `ownerfiles` is the single write path (`ownerfiles.go:1-22`) |
| Content items, podcast subs, browser sessions, media progress/recents, theme | OG (registry.go:101-103) + media tables (schema.go:147,160,175) | runtime on owner acts |
| Desktop state | schema.go:195-237 | runtime |
| Computer settings | `choir.computer_setting` (:109) | owner |

---------------------------------------------------------------------------
## 2. Readers of each kind

Classes: **CUST** custodian itself; **VERB** another desk through a yaegi verb; **SVC** Go service on a desk's behalf (agentcore/textureowner/reducers); **HTTP** route for the front end; **HOST** vmctl/proxy/gateway/maild/auth/corpusd; **MGT/ENG** management or engineering; **RES** research.

### 2.1 Mail readers

| Reader | Class | file:line | Note |
|---|---|---|---|
| Mail app (`MailApp.svelte`) -> proxy -> maild | CUST (owner via host) | `frontend/src/lib/MailApp.svelte:282-1017` (aliases, messages, drafts, digest, attachments); `proxy/email.go:14-66` | API-key sessions refused for send/approve (`proxy/email.go:30`) |
| maild digest reads published front | HOST | `maild/digest.go:100-107` (corpusd `/internal/platform/publications/front`) | public data, not mail |
| auth -> maild (sign-in, recovery, number assignment) | HOST | `auth/email_signin_mail.go:19`, `recovery_mail.go:24`, `number_reconcile.go:28` | writes only |
| Files route over `mail/` Maildir | HTTP | `autoputer/files.go:607-608` (no mail exclusion) | **gap** if Maildir is ever populated [H: no caller of `/api/mail/inbound`] |
| Engineering capsule files snapshot | ENG | excluded: `capsule/computer_state.go:45`, `computer_state_linux.go:40` | already a custody carve-out |
| Any desk, any verb, any Go service in the guest | none found | grep negative (see 0.5) | [H] |

Verdict: mail already satisfies a hard custody rule in the guest. No desk reads it. Only the Files/Maildir path needs a carve-out.

### 2.2 Texture readers (documents, revisions, lineage, evidence, citations)

| Reader | Class | file:line | What it reads |
|---|---|---|---|
| Texture cell `ReadDoc` | CUST | `yaegikernel/choir.go:1195-1200`; frame built `agentcore/texture_frame.go:73-111` | own doc |
| Texture cell `ReadCapture` | CUST (captures are corpusd) | `choir.go:665-676`; `texture_frame.go:160` | captures its sources name |
| Texture HTTP API (documents, revisions, history, export, stream, events, source-open, research, revise, restore) | CUST/HTTP | `textureowner/router.go:15-69`; front end `frontend/src/lib/texture.js:5-19` | owner reads/writes |
| Owner edit path / prompt bar continue | CUST | `textureowner/prompt_bar.go:150` -> `reviseOwnerDocument` | owner revise |
| Research `SearchWireCorpus` | VERB (RES) | `researchtools/researchtools.go:71` -> `store/texture.go:125-160` `SearchDocuments` -> `searchableTextureDocuments` lists **every** `choir.texture_document` of the owner (`:143-158`), heads via `search_index.go` | all doc head text, not just stories |
| Research `System1` `context_refs: story:<rev>` | VERB (RES) | `agentcore/system1_verb.go:461-475` (`GetLifecycleRevision(owner, computer, revisionID)`, no doc scoping) | any revision of the owner |
| Research `StoryAnalysis` | SVC for RES (frame preload) | `agentcore/story_analysis.go:142,200-220` (doc + head content) | its bound story |
| Research `Relate` | VERB (RES) | `agentcore/story_entities_verb.go:329-341` (existence check only) | doc exists |
| Research inbox start reads doc | SVC | `agentcore/inbox_records.go:1048-1053` (only `doc.TrajectoryID`) | metadata |
| Management `ProductAPI` GET `/api/texture/*` | VERB (MGT) | allowlist `agentcore/tools_product_api.go:215-222`; writes refused `:199-204` (only `/api/prompt-bar` POST creates) | whole Texture HTTP surface, read |
| Management `Stories` | VERB (MGT) | `agentcore/intake_view.go:67-130` (`ListStoryDocuments`, `StorySummariesOf`: headline, lede, version) | story library |
| Management intake labeler (Jev) | SVC for MGT | `agentcore/intake_labels.go:~240-300` (candidate stories' title/lede in the judge prompt) | story summaries |
| Engineering run context (owner words) | SVC for ENG | `agentcore/engineering_owner_words.go:17-24,152-165` (`ListTextureRevisionsByScopeOG`), re-read each render | owner revisions of the bound doc |
| Cast reducer attaches `owner_directions` | SVC (reducer) | `agentcore/texture_owner_directions_cast.go:19-27,466-478` | owner revisions |
| Engineering assignment open | SVC for ENG | `agentcore/engineering_assignment_runtime.go:297-304` (`doc.TrajectoryID`) | doc metadata |
| Engineering capsule store copy | ENG | `capsule/computer_state_linux.go:135-177`, `autoputer/capsule_executor_linux.go:144-156` -> `store.SnapshotTo` (`store/snapshot_to.go`) | **entire store** incl. all Texture docs |
| Wake/rebase/stack/override readers | SVC | `selfdev_owner_act_wake.go:57`, `rebase_notice.go:499,523,598`, `stack_base_wake.go:172`, `intake_overrides.go:102`, `story_publish.go:148`, `tools_desk.go:1244` | doc heads/metadata |
| `/api/trajectories/*` | HTTP | `agentcore/api_trajectory.go:425` + lifecycle snapshot/events/capsule evidence | doc + lifecycle + engineering evidence, used by the Texture UI (`frontend/src/lib/lifecycle.js:17,88`, `texture.js:143,268,282`) |
| Texture `/diagnosis` | HTTP | `textureowner/texture.go:~1975-2005` | runs, events, **all agents' evidence** for the owner |
| Proxy story expansion | HOST | `proxy/platform_expand.go:407,450,597` (HTTP to the paper computer's `/api/texture/...`, as owner) | already a request-to-custodian |
| Guest -> corpusd publication | HOST | `story_publish.go:148`, proxy `handlers.go:1791` | Texture's own act |
| vmctl/witness | HOST | `store/witness_tables.go:21-24` (hashes `og_objects`, `choir_events`), `vmctl/recovery_admission.go:639-668` (heads, watermark), `vmctl/data_image_capture.go:45` | digests and an opaque image |

### 2.3 Commitment ledger and inboxes

| Reader | Class | file:line | What it reads |
|---|---|---|---|
| Any desk `Records()` | CUST (self-addressed) | `choir.go:1099-1116`; inbox by addressee | records addressed to the desk |
| Any desk `Pack()` | CUST (self-scoped, filtered in Go) | `tools_desk.go:1039-1077` loads up to 256 claim records for the owner (`:1051`), `types/commitment_views.go:497-533` keeps own+addressed acts | own acts + issuer tallies |
| `Pack.Waits` for management | MGT via SVC | `tools_desk.go:1062-1068` (assignment waits, candidate decision waits, verify owed) | engineering assignment state |
| `Pack.Waits` for engineer | ENG | `tools_desk.go:1069-1072` | own assignment |
| `Pack.ProviderWaits` for every desk | SVC | `tools_desk.go:1074-1076` | every run in a quota incident on the computer |
| Texture evidence from its own addressed records | CUST/SVC | `textureowner/texture_evidence_sources.go:691-699` (`addressee = textureAgentID`) | records addressed to Texture |
| Engineering/management Go services reading casts/notices | SVC | e.g. `engineering_assignment_runtime.go:265,415,601,712`, `engineer_clock.go:87`, `round_receipt.go:83-126`, `rebase_*`, `calibration_e1.go:*`, `verify_owed.go:371` | the ledger is the shared PSA substrate |
| Calibration/precommit/release HTTP | HTTP | `apihandler/routes.go:23-24,34-36`; `calibration_api.go:160`, `calibration_claims_api.go:194-199`, `api_release.go:43` | owner reads |
| `/api/inboxes/{inbox}/records` | HTTP | `agentcore/api_inbox_records.go:29-87` (owner can read any desk inbox) | owner root |
| Management `ProductAPI` `/api/computers/*`, `/api/run-acceptances` | VERB (MGT) | `tools_product_api.go:196-198,215-222` | computer lifecycle, acceptances (read-only) |

### 2.4 Captures

| Reader | Class | file:line |
|---|---|---|
| Texture, management, research, app desks `ReadCapture` | VERB | `deskmanifest` verb lists (Management, Texture, Research, ResearchChild rows; `appDeskVerbs` :471-475); host action via `researchtools/host_egress.go:67-78` |
| Research `SearchWireCorpus` (captures list) | VERB (RES) | `researchtools/researchtools.go:96,107,114` (also events and published) |
| Texture citations require captures | SVC | `agentcore/tool_profiles.go:385-390`, `texture_frame.go:160` |
| Support judge, story publish, System1 | SVC | `support_judge.go:240`, `story_publish.go:278`, `system1_verb.go:477-490` |
| `GET /api/captures` | HTTP | `agentcore/api_captures.go:15-50` |
| Management intake (arrival records, labels) | MGT via SVC | `capture_arrival.go:17-40`, `intake_labels.go` |

Captures are a **shared world-content kind** (corpusd Store B, not owner-private); every desk reading them is expected.

### 2.5 Work items, trajectories, runs, run memory, evidence

| Reader | Class | file:line |
|---|---|---|
| `/api/trajectories`, `/api/runs`, `/api/run-acceptances`, `/api/runtime/*` | HTTP | `apihandler/routes.go:21-22,47-56` |
| Research `RunMemoryEntry` | VERB (RES) | `tools_evidence.go:35-76`: `GetRunMemoryEntry(ctx, ownerID, entryID)`, **no run/agent scoping** [V]; intended for the run's own compaction summary (description at :40) so exploitability is [H] |
| Research `ReadEvidence/ListEvidence` | CUST (research owns evidence) | `tools_evidence.go:143-215`; `ListEvidence` accepts any `agent_id` for the owner (`:195-199`) |
| Texture reads evidence refs | SVC for Texture | `textureowner/texture_evidence_sources.go:147-155` (`GetEvidence`), `:156-157` execution-evidence refs become source entities **without a store read** |
| Obligations/census | HTTP | `obligations_surface.go`, `census_surface.go` |
| Management `Pack.Waits` | see 2.3 | |

### 2.6 Engineering assignments and capsules

| Reader | Class | file:line |
|---|---|---|
| Management waits | MGT via SVC | `tools_desk.go:1062-1068` |
| Engineering own wait | ENG | `tools_desk.go:1069-1072` |
| `/api/trajectories` capsule evidence | HTTP | `api_trajectory.go` (`GetEngineeringCapsuleEvidence`) |
| Texture shows execution evidence | SVC (ref only) | `texture_evidence_sources.go:156-157` |
| Judge, screens, shot review | SVC | `selfdev_screens.go`, `selfdev_shot_review.go`, `engineering_owner_words.go:193-199` |

### 2.7 Autopaper sources and stories

| Reader | Class | file:line |
|---|---|---|
| Management `ProductAPI` GET/POST `/api/autopaper/*` | VERB (MGT), read **and write** | `tools_product_api.go:221` (the `/api/texture` write ban at :199-204 has no `/api/autopaper` equivalent) |
| Source schedule poller | SVC | `agentcore/source_schedule.go` |
| Research `PollSource`, `SourceAPI` | VERB (RES) | `deskmanifest` Research row; `host_egress.go:67-78` |
| Research corpus search of events | VERB (RES) | `researchtools.go:107` |
| Front end Autopaper app | HTTP | `frontend/src/lib/autopaper.ts:134`, `autopaper-expand.ts:35,92`, `texture-related-stories.ts:49` (`/api/platform/publications/*`, corpusd via proxy) |
| maild digest | HOST | `maild/digest.go:100-107` |

### 2.8 App package objects

Readers: the owner route only (`autoputer/app_package_objects.go:170,190`). No desk verb reads package objects; app desks have only semantic-act verbs plus `ReadCapture` (`manifest.go:471-475`). Compliant already. [V]

---------------------------------------------------------------------------
## 3. The specific cross reads asked about

| # | Cross read | file:line | Verified |
|---|---|---|---|
| X1 | Research reads all Texture doc heads (corpus search) | `researchtools/researchtools.go:71`; `store/texture.go:125-160` | V |
| X2 | Research reads any Texture revision text via System1 `story:<rev>` | `system1_verb.go:461-475` | V |
| X3 | Research gets its story's head text in the frame | `story_analysis.go:142,200-220` | V |
| X4 | Engineering reads Texture owner revisions into its run context | `engineering_owner_words.go:152-165` | V |
| X5 | Reducer reads Texture revisions to attach to a Cast | `texture_owner_directions_cast.go:466-478` | V |
| X6 | Engineering capsule gets the full store copy and the files tree | `capsule/computer_state_linux.go:135-177`; `autoputer/capsule_executor_linux.go:144-156` | V |
| X7 | Management reads Texture (HTTP allowlist) | `tools_product_api.go:215-222` | V |
| X8 | Management reads story library | `intake_view.go:67-130` | V |
| X9 | Management intake labeler sends story summaries to the judge | `intake_labels.go:~240-300` | V |
| X10 | Management reads mail | none found | H (negative) |
| X11 | Conductor/prompt bar reads anything | classifier uses only the submitted text (`textureowner/prompt_bar.go:206-246`); conductor has an empty tool registry (`tool_profiles.go:410,419`); it reads its own submission run (`prompt_bar.go:265`); `continue_doc_id` goes to the Texture owner-revise path (`prompt_bar.go:150`) | V |
| X12 | `choir.Context` | identity only (`choir.go:1157-1188`) | V |
| X13 | `choir.Records` | self-addressed records only (`choir.go:1099-1116`) | V |
| X14 | `choir.Pack` | own and addressed acts, score-free (`tools_desk.go:1039-1077`, `commitment_views.go:497-533`); management additionally gets engineering waits (`tools_desk.go:1062-1068`); all desks get `ProviderWaits` (`:1076`) | V |
| X15 | Search indexes spanning kinds | `search_wire_corpus` spans Texture docs + world events + captures + published (`researchtools.go:71,96,107,114`). `yaegikernel/search_store.go` and `internal/search` are **web search**, not store indexes. `store/search_index.go` indexes Texture only | V |
| X16 | Texture reads research output | via inbox records carrying `ReportPacket` (claims, excerpts, source refs: `yaegikernel/report_packet.go:12-44`); refs resolved by Texture's handler: `texture_evidence_sources.go:147-155` (evidence_id), `:138-146` (content item), captures via `ReadCapture` | V |
| X17 | Texture reads the ledger | only records addressed to Texture (`texture_evidence_sources.go:695`) | V |
| X18 | Research reads other desks' run memory | `tools_evidence.go:57` unscoped by run or agent | V code / H exploit |
| X19 | Casts embedding data vs refs | Embedding: Texture Cast packet carries owner words verbatim (`owner_directions`, `texture_owner_directions_cast.go:19-27`, bound 64 KiB :39); Report packets carry claims and excerpts (`report_packet.go`); capture labels carry headline. Refs: `Feed(doc, captureIDs, reason)` (`choir.go:763-777`), `owner_revision` ids (`texture_owner_directions_cast.go:29`), evidence_id and execution-evidence refs in report sources (`texture_evidence_sources.go:147-157`), engineering `story:<rev>` and capture ids in System1 refs (`system1_verb.go:446`) | V |

---------------------------------------------------------------------------
## 4. Is each cross read needed? (a) carry in cast/report, (b) scoped read capability on a delegation, (c) request to the custodian, or real broad read

| # | Cross read | Verdict | Reasoning |
|---|---|---|---|
| X1 | Research corpus search over Texture docs | **(c)** | Dedupe against "what the paper already holds" is a question Texture can answer ("do you have a story on X?"). Today it indexes all docs including non-story owner docs, which is more than needed. A Texture-owned search service answering with `{doc_id,title,snippet}` keeps the behavior. The capture/event/published parts of the same tool are not Texture data |
| X2 | System1 `story:<rev>` | **(a)** | The calling cell can be handed the text (Texture's Cast/frame) or (c) System1 call carries `text` instead of a ref; `system1Context{Kind:"story"}` just needs the text bound (`system1_verb.go:471-475`) |
| X3 | StoryAnalysis | **(a)** already | Host reads head and preloads it. Under custody, Texture (or a Texture-owned export) supplies the text; research never touches the revision |
| X4 | Engineering run context owner words | **(a)** already half-done | The Cast already carries `owner_directions` (`texture_owner_directions_cast.go`). The re-read "at every render" (`engineering_owner_words.go:17-24`) is the extra read; replace with reading the data attached to the Cast record (record is PSA substrate) |
| X5 | Reducer attaches owner words | **custodian-side** | This is the reducer acting for Texture's Cast. Treat the reducer as Texture's agent for this one read (exception E3) or have Texture's turn write the words |
| X6 | Capsule store copy + files | **real broad read, shape (b)** | Engineering must reproduce and debug against realistic state ("a copy is not access": `computer_state.go:13-16`). Keep as a delegation-scoped capability: choose kinds per assignment; default exclude mail (already), Texture bodies, ledger text except the assignment's chain |
| X7 | Management GET /api/texture/* | **(c)** | It is literally a request to Texture's API, but read-only and unfiltered. Narrow to list/summary routes; stop at the Texture desk for content |
| X8 | Management Stories | **(c)/(a)** | Routing a capture needs candidate story headlines. The Jev label already carries `EventHeadline`/`TopicHeadline` (`intake_view.go:42-49`), so after labeling it is (a). The candidate listing itself is (c): Texture publishes a story-index projection |
| X9 | Jev judge sees story summaries | **(c)** | Same as X8; the labeler is a Go service on management's behalf reading Texture summaries |
| X10 | Mail from any desk | not present | no change |
| X11 | Conductor | none | no change |
| X14 | Pack.Waits for management | **custodian read** | Management owns `assignment`,`admission` per `manifest.go` Management row (`OwnedKinds`). Keep, but declare the writer |
| X14b | ProviderWaits to every desk | **(a)** | host-supplied scalar, no data kind crossed |
| X15 | Cross-kind search | **split** | Texture part -> (c); captures/events/published are shared world content (no owner custody) |
| X16 | Texture reads evidence refs | **(a)** | Report packets already carry `excerpt` and claims. The evidence_id read (`texture_evidence_sources.go:147-155`) is a second path; the packet can carry the evidence content, or research (custodian) exposes `ReadEvidence` as a request |
| X18 | Research `RunMemoryEntry` | **self-scope it** | A run's own entries only; a cross-run read has no stated need |
| E-HTTP | `/diagnosis`, `/api/trajectories` joins | **owner root; keep** | owner-authenticated debug/UI surfaces; these are the supervision surface, not desk reads |
| E-proxy | Proxy -> paper Texture HTTP | **(c) already** | |

**Where research or engineering truly need broad reads**
- Research: the *world* (web, sources, captures, published stories, events) yes; owner Texture bodies no, only via (a)/(c). Cross-run run-memory no.
- Engineering: the platform source repo (`/workspace/platform`, `engineering_assignment_runtime.go:1135`) and, for realistic repro, a point-in-time copy of computer state (X6). That is the one honest broad read; scope it per assignment as (b).
- Management: the cross-desk view of assignments, admissions and ledger health is a **supervisory** read (manifest: "management owns computer-wide coherence"); reading the ledger/assignment kinds broadly is part of its charter. Reading Texture and mail is not.

---------------------------------------------------------------------------
## 5. Front end: which app reads which kinds through the API

| App / surface | API calls | Kinds |
|---|---|---|
| Mail (`MailApp.svelte`) | `/api/email/{aliases,messages,drafts,attachments,digest}` (:282-1017) | mail only, via proxy to maild |
| Texture (`TextureEditor`, `texture.js`, `lifecycle.js`) | `/api/texture/*` (texture.js:5-19), `/api/trajectories/*` (lifecycle.js:17,88; texture.js:143,268,282), `/api/platform/*`, `/api/content/*`, `/api/model-policy/resolve` (TextureEditor:1400), `/api/computers/<id>/self-development/operations/*` (texture-embeds.ts:203), `/api/assets/sha256/` | Texture, lifecycle/work items, engineering evidence, publications, content items |
| Files (`packages/files/App.svelte`) | `/api/files*` only (:121-712) | files root. **Does not read Texture.** Texture docs appear only as shortcut JSON files (`texture_import.go:117-123`); opening one goes through `/api/texture/files/open` (texture.js:7). It would list `mail/` if populated (`files.go:607-608`) |
| Reader / Image packages | `/api/media/*`, `/api/files/*` (reader/App.svelte:568,592; open-file.js:98) | files, media |
| Slides, Calendar, PDF, EPUB, Audio, Video apps | no app API of their own; fetch the file path they were launched with (`SlidesApp.svelte:270`, `CalendarApp.svelte:351`, `PdfApp.svelte:248`, `EpubApp.svelte:275`) + `/api/media/*` | files, media. No Slides/Calendar object kinds in use [H] |
| Podcast / Browser / ContentViewer | `/api/podcast/*`, `/api/content/*` | content items, podcast subs |
| Autopaper | `/api/platform/publications/{front,resolve,expansion,expand,related}` | corpusd publications |
| Prompt surface / conductor | `/api/prompt-bar` (`conductor.js:19,43`) | conductor submissions, hands off to Texture |
| Desktop shell | `/api/shell/bootstrap`, `/api/desktop/state`, `/api/ws`, `/api/packages/installed` | desktop state, package list |
| Settings / Compute monitor | `/api/compute/{status,recovery}`, `/auth/api-keys`, `/api/preferences/theme` | computer/health |
| Library (`packages/library-next`) | `/api/catalog/*`, `/api/computers/<id>/self-development/adoptions` | releases, adoptions |
| Pulse | `/api/pulse/summary` (public, aggregate: proxy/handlers.go:1006-1025) | none per-owner |

No front-end app crosses a boundary except Texture's UI (which is the Texture custodian's own client but reads lifecycle and engineering evidence kinds) and Files (mail/ listing).

---------------------------------------------------------------------------
## 6. Summary

### 6.1 Cross reads by class (non-custodian reading another custodian's kind; each row of section 3 counted once)

| Class | Count | Items |
|---|---|---|
| VERB: desk reads another desk's kind through a host-served verb | 7 | X1 (research corpus search), X2 (System1 story ref), X7 (management ProductAPI to Texture), X8 (Stories), X18 (RunMemoryEntry), Relate existence check (`story_entities_verb.go:335`), management ProductAPI `/api/computers`,`/api/run-acceptances`,`/api/autopaper` |
| SVC: Go service reads on a desk's behalf | 8 | X3 StoryAnalysis, X4 owner words, X5 reducer owner_directions, X9 intake labeler, engineering assignment open (`engineering_assignment_runtime.go:298`), Pack.Waits (management), Texture evidence ref read (X16), wake/rebase/override readers |
| MGT/ENG broad | 1 | X6 capsule store copy + files |
| HTTP (owner-authenticated front-end or debug routes spanning kinds) | 4 | `/diagnosis`, `/api/trajectories`, `/api/inboxes/*`, Files over `mail/` |
| HOST | 3 | proxy -> paper Texture HTTP, vmctl/witness/data image, maild <- corpusd (public) |
| Research | (counted in VERB: X1, X2, Relate, X18) | |
| **Total** | **23** | |

Self-scoped reads that are already compliant: `Context`, `Records`, `Pack` (own part), `ReadDoc`, Texture's own HTTP API, app-package objects, mail.

### 6.2 Would break under a hard read-custody invariant

1. **X1** research corpus search over all Texture docs.
2. **X2** System1 story refs.
3. **X7, X8, X9** management reading Texture content and story summaries (also breaks the intake pipeline's candidate selection until a Texture index exists).
4. **X4/X5** engineering context and Cast attach, if the reducer is not treated as the custodian's agent.
5. **X6** capsule store copy and files tree, unless it becomes a kind-filtered capability.
6. **X16** Texture reading evidence records written by research.
7. **X18** research reading arbitrary run-memory entries (probably unintended).
8. Management ProductAPI `/api/autopaper` write path touches source schedules (`WriterRuntime`, no desk custodian) and would need a custodian decision.
9. `/diagnosis` and `/api/trajectories` (HTTP) join kinds; they survive only as owner-root surfaces (exception E1).
10. Structural: every `WriterRuntime` kind (ledger, inbox, work items, runs, assignments, source schedules, world events, labels) has no custodian desk. The rule needs a table mapping these to desks or to a "runtime commons" exemption. Today the writer gate is also only in observe mode.

Not broken: mail, app package objects, conductor, `Context`, `Records`, `Pack` (own), app desks (verbs are only semantic acts and `ReadCapture`).

### 6.3 Minimal exceptions

- **E1 Owner root.** Every owner-authenticated HTTP read (Files, Texture, trajectories, inboxes, diagnosis, captures) stays. The owner is above the desks.
- **E2 Runtime commons.** The ledger, inboxes, wake outbox, run/event/work-item bookkeeping are the PSA substrate itself. Desks see them only as self-scoped `Records`/`Pack`; Go services read them freely. Management gets supervisory read of assignments, admissions and ledger health.
- **E3 Reducer as custodian's agent** for the one read in X5 (Cast owner-direction attach), or move that read into Texture's turn.
- **E4 Shared world content.** Captures, corpusd publications, world events and web/source results are readable by research, Texture and management (they are not owner-private custody).
- **E5 Engineering capsule snapshot** as a delegation-scoped capability (b): per-assignment kind list; mail and (proposed) Texture bodies excluded by default; the existing leave-out mechanism (`capsule/computer_state.go:42-52`) is the place.
- **E6 Host integrity services** (vmctl, witness, data-image capture, heads) keep opaque or digest-level access.

### 6.4 Facts that make the rule cheaper than it looks

- Cell-level host-served reads are a short list: management `product_api`, `read_capture`, `stories`; Texture `read_capture` only; research `search_wire_corpus`, `read_evidence`, `list_evidence`, `get_run_memory_entry`, `system1`, `relate*`, `read`, `poll_source`, `source_api`; app desks `read_capture` only (`deskmanifest` verb rows + `FrameActions`). The big tool is `product_api` (management); narrowing its allowlist and the three research reads (X1, X2, X18) closes the desk-verb class.
- The "carry" pattern already exists (`owner_directions`, `ReportPacket`, labels with headlines, `StoryAnalysis` preload). The SVC class mostly needs the carried data to be sourced from the custodian rather than re-read from the store.
- Mail needs almost nothing: add the `mail/` exclusion to the Files route and decide whether the Maildir stays.

### 6.5 Not checked (gaps)

- Gateway: it sees prompt contents for every desk (H, not inspected).
- `cmd/desk-worker`, checkpointd, corpusd internals beyond registry/exports.
- Whether `/api/mail/inbound` has any external caller (none found in this repo).
- Whether the relational `texture_*` tables are written at all, versus the OG kinds.
- Exhaustive enumeration of every `rt.store.*` call in `agentcore` (about 175 files); I sampled by kind via grep of the getters.
