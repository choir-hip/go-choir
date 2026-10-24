# A computer's data has more than one home, and the files tree is the largest second one (2026-10-24)

Date: 2026-10-24. Written by an FP0 read-only investigator. Status: documented; evidence and directions only. Mutation class of this record: green (docs only). No code changed, no computer or staging host touched, no credential file read.

Evidence labels: **[V]** read in code or a doc at file:line on this tree (origin/main at `204b66ff5`, plus the ledger commit `6fc61a6f0`). **[C]** read in the app-rewrite conductor's artifacts under `/Users/wiz/Dev/lanes/_runs/apprewrite/` (the v7 candidate patch and `log.md`); that candidate is **not on main**, it is frozen at awaiting_approval on one computer. **[H]** hypothesis, with what would settle it.

## The invariant this tests

Owner ruling, 2026-10-24, ledger entry 19:0x (`docs/definitions/choir-launch-first-pass-metamission-2026-10-21.ledger.md:1605`): one object graph per computer, as the Autopaper object graph is one for the world. A new app "schmail" uses the same message data as Mail; data one app writes is usable by every other app and agent; an organization can later share one graph under RBAC. "Every type of data has only one right way to store and access it." Files and Dolt must integrate.

Coordinator addition (same day): the rule that the invariant becomes is that every meaningful data kind is an object graph kind with exactly one writer, and a file is a content-addressed blob plus a path view, never a second home. The "Target" column below applies that rule to each kind.

## Short answer

The invariant is violated in five distinct ways, and the largest is not in Dolt at all.

1. The files tree (`/mnt/persistent/files`) is a second home that apps and the owner use by convention for data that is app data or computer configuration: the model policy, Calendar `.ics`, and (in the unlanded candidate) Slides `.slides.json`. It is outside the object graph, outside the replay manifest, outside the restore set, and not reachable by any desk through a verb.
2. Mail lives on the host in maild's SQLite, outside the computer entirely.
3. Several kinds live in bespoke relational tables, some on the tape and some not, beside object graph kinds that are registered for the same data and have no writer.
4. One file or document is referred to by path strings in three tables, with nothing that keeps them true.
5. Two further homes are planned or in a candidate patch and would deepen the problem: a per-package key-value store (`package_kv/v1`) in the paused S7-0 design, and browser `localStorage` for Podcast played-state and queue in the v7 candidate.

## How data is stored today (the stores)

- **VM embedded Dolt** (`RUNTIME_STORE_PATH=/mnt/persistent/state`, workspace `state.texture`; `nix/autoputer-vm.nix:1018`, `internal/store/texture.go:159`). Holds `og_objects` and `og_edges` (`internal/objectgraph/dolt_store.go:21,38`) and the bespoke tables in `internal/store/store.go` and `internal/store/texture.go`. Every `og_*` mutation goes through a mutation interceptor that appends to the tape before SQL when the tape is bound (`dolt_store.go:81-95,646-657,1141-1157`). The replay manifest classes `og_objects` and `og_edges` as `ReplayEventProjection` [V `internal/agentcore/replay_eligibility.go:93` and neighbours].
- **Files tree** (`AUTOPUTER_FILES_ROOT=/mnt/persistent/files`, `nix/autoputer-vm.nix:1022`). Served by `/api/files` and `/api/files/{path}` (GET, POST mkdir, PUT, DELETE; `internal/autoputer/files.go:140-165,455-456`). `FilesHandler` holds only a root and an observer (`files.go:50-52`): it has no store, so a write or delete updates nothing else. It also holds `Source/platform` (the capsule source root, `nix/autoputer-vm.nix:1060`), `System/model-policy.toml` (`internal/provideriface/config.go:88-93`), `.texture` shortcut files (`internal/content/source.go:12`) and a guest Maildir under `mail/` (`internal/autoputer/run.go:579`).
- **Corpus Store B** (Autopaper object graph, `og_*` plus bespoke tables, behind corpusd). Captures, publications, published Texture mirror. Outside every computer's restore set by ruling (`docs/computer-ontology.md:108-112`).
- **Host-side stores outside the computer:** maild's SQLite (`nix/node-b.nix:1035-1037`, `internal/maild/store.go:26-33,281-312`), the auth database (`internal/auth/store.go:29-138`), the file CAS chunk and root stores (`internal/platform/file_cas.go:13-22`), the capture-body artifact store (`docs/problems/platform-artifact-retention-2026-10-22.md`).
- **The tape** (computerevent chain). A projection batch carries OG mutations and the bespoke-table ops that have reducers.

### Is anything content-addressed?

Yes, in four separate places, none of them joined to the object graph [V]:

1. **File CAS** (`internal/filecas/filecas.go`, `internal/autoputer/file_sync.go`). Each regular file under the files root is split into 4 MiB chunks (`file_sync.go:25`), each chunk encrypted with a deterministic synthetic nonce so that identical plaintext gives the same ciphertext and the same SHA-256 digest (`filecas.go:129-150`). A manifest lists `path, mode, size, chunks[]` per file and has a root digest (`filecas.go:28-40,64-80`). Dedup is within one computer only ("Nothing is shared across computers or keys", `filecas.go:140-142`). The manifest holds plaintext paths and sizes (same comment). Sync runs every 15 minutes by default (`file_sync.go:27`) or on `POST /api/files/sync` (`file_sync.go:764-790`); it uploads chunks and the root to corpusd (`file_sync.go:622-646`) and appends a `file_root_committed` event citing the root (`file_sync.go:685-708`). The tape therefore cites a root at most every 15 minutes; file contents between syncs are not on the tape at all.
2. **Image blob store** (`internal/imageblob/imageblob.go:1-60`): `/mnt/persistent/state.image-blobs/sha256/<hex>`, keyed by the digest of plaintext, local to the guest, not under the files root, so not in file CAS.
3. **Artifact blobs in the corpus** (`choir.artifact_blob`, content-hash identity; `registry.go:163`, corpusd).
4. **Tape private artifacts** (event bodies and payloads, `computerevent.ArtifactRef`).

No object graph kind references a file by digest. A `content_item` carries `ContentHash(text)`, the hash of the extracted text, not of the file bytes, and a `file_path` string [V `internal/content/content.go:255,265`].

### Restore and replay of the files tree

A computer restore addresses an event head and restores the release pointer plus VM-local embedded state [V `docs/computer-ontology.md:108-110`]. The files tree is not embedded Dolt state. The only code that reads a stored file root back is `HydrateIfNeeded`, which copies the latest root into the tree only when the tree has no regular files or the volume is fresh (`file_sync.go:182`: "a local file always wins over a remote snapshot"), and it is called only at boot (`internal/autoputer/run.go:680,700`). `file_root_committed` is a causal event kind with no reducer that rebuilds anything from it (`internal/computerevent/reducer.go:174`). So restoring a computer to an earlier head does not rewind a file. The replay equivalence check observes only Dolt tables (`replay_eligibility.go:256-272`, keys `dolt:texture:table:*`). [V for the code paths; H that nothing outside these (for example the projection base) carries file content: the base is described as a snapshot of projected state, `docs/problems/platform-artifact-retention-2026-10-22.md` section 1.]

### Can an agent reach it?

The management desk is the only desk with `ProductAPI` in its verb list [V `internal/deskmanifest/manifest.go:210`; `ProductAPI` appears only at `:183` and `:210`]. Its route allowlist is `/api/prompt-bar`, `/api/texture/` (GET), `/api/computers/` (GET), `/api/run-acceptances`, `/api/autopaper/`, plus `POST /api/model-policy/verify`; `/internal/`, `/api/prompts` and others are refused [V `internal/agentcore/tools_product_api.go:177-222`]. Not on it: `/api/files`, `/api/email`, `/api/media`, `/api/preferences`, `/api/podcast`, `/api/content`, `/api/desktop`, `/api/computer-settings`, model-policy reads and writes. Every desk cell has `ReadFile`, `WriteFile` and `ListDir`, but they are jailed to `execCtx.WorkingDir`, falling back to `os.TempDir()` (`internal/agentcore/tools_desk.go:917-922`), and nothing sets the `tool_cwd` run metadata that feeds `WorkingDir` (`tool_profiles.go:27,51`; the constant has no setter on this tree). So the owner's files tree is not a place a desk reads or writes through a verb. [V for the allowlist and the jail; H that no other path (a capsule mount, a shell tool for engineering) reaches the files tree: engineering runs in a capsule with its own `/workspace`, `internal/agentcore/engineering_assignment_runtime.go:482`.] The app verb registry that was meant to give apps and desks one verb set per app has no callers and is off by default (`internal/appverbs/appverbs.go:1-12`).

## Inventory

Columns: **Home** is where the live bytes are. **Replay** is the class in `replay_eligibility.go` (EP event projection, EU empty-until-supported, RA retired-absent, none = outside the manifest). **Restore**: in the restore set (VM embedded state) or not. **2+ homes** is yes when the same kind is live in more than one place. **Target** is the og kind and writer under the rule, "already conforms", or "exempt (reason)". Writer names follow `objectgraph.Writer` (`internal/objectgraph/writer.go`): `desk:<name>`, `system:runtime`, `system:store`, `owner`.

### Texture and documents

| Data kind | Home | Writers | Readers | Replay / restore | 2+ homes | Target |
|---|---|---|---|---|---|---|
| Texture document | OG `choir.texture_document` (`registry.go:97`; `graph_store.go:2306`) | `desk:texture`; owner edits admitted (`OwnerEdits`) | Texture app via `/api/texture`; desks by GET | EP via `og_objects`; in restore | **yes**: also a `.texture` shortcut file in Files (`textureowner/texture_import.go:116-123,510-518`), alias table `texture_document_aliases` (`store/texture.go:107,772-797`), a preserved original content item for opened files (`textureowner/texture.go:653`), a host mirror `platform_texture_documents` in corpusd (`platform/store.go:117,522`, `platform/service.go:154`), and a browser draft (`TextureEditor.svelte:282-315`) | already conforms for the document. Shortcut file becomes a path view of the document object; alias becomes the registered `document_alias` edge (`registry.go` edge list) or a path view; corpusd mirror is a publication copy and stays only as published `choir.publication_version`. Draft: exempt (per-device unsent text, cache) |
| Texture revision | OG `choir.texture_revision` (`registry.go:98`; `graph_store.go:2482`) | `desk:texture`; owner edits | same | EP; in restore | **yes**: dead bespoke `texture_revisions` DDL (`store/texture.go:56`, no INSERT; replay EU `replay_eligibility.go:82`) and host `platform_texture_revisions` (`platform/store.go:126,530`) | already conforms. Drop the dead table DDL and its manifest entry. Host mirror as above |
| Texture source entity / ref | OG `choir.source_entity`, `choir.source_ref` (`registry.go:76-77`) | `desk:texture` [H: written from `textureowner/tools_texture.go:211,380`; `PutTextureSourceEntityOG` has no caller] | Texture | EP; in restore | dead tables `texture_source_entities/refs` (`store/texture.go:74,87`; EU) | already conforms; drop dead DDL |
| Texture agent mutation | bespoke `texture_agent_mutations` (`store/texture.go:124`), tape-bound path `store/project.go:540`, unbound seam direct SQL `store/texture.go:1868` | texture runtime | texture runtime | EP (`replay_eligibility.go:86`); in restore | edge kind `document_mutation` registered with no object kind | og kind `choir.texture_mutation`, writer `system:runtime` (bookkeeping). Low priority |
| Texture decision | OG `choir.texture_decision` | `desk:texture` | Texture | EP; in restore | table retired (RA) | already conforms |
| Agent evidence (research `SaveEvidence`) | **bespoke `agent_evidence`**, direct SQL (`store/texture.go:2263-2285`) via the broker tool (`agentcore/tools_evidence.go:78-135`, `tools_desk.go:1201`) | research desk (`deskmanifest/manifest.go:242`) | research desk (`ReadEvidence`, `ListEvidence`) | **EU** (`replay_eligibility.go:89`): one saved evidence row makes the computer checkpoint-ineligible, the same defect as `docs/problems/stock-ui-writes-make-a-computer-checkpoint-ineligible-2026-10-24.md`; not on the tape | **yes**: OG `choir.agent_evidence` is registered (`registry.go:100`, writer `system:runtime`) and `CreateEvidenceOG` has no caller | og `choir.agent_evidence`, writer `desk:research` (the manifest already lists `evidence` as its owned kind). Highest-value small migration |
| Run memory | **bespoke `run_memory_entries`** (`store/store.go:531`, insert `store/run_memory.go:134`) | runtime | runtime, `RunMemoryEntry` verb | EP (`:60`); in restore | **yes**: OG `choir.run_memory_entry` registered (`registry.go:92`); `AppendRunMemoryEntryOG` has no caller | og `choir.run_memory_entry`, writer `system:runtime`; retire the table once the projection is an object |

### Runtime ledger

| Data kind | Home | Writers | Readers | Replay / restore | 2+ homes | Target |
|---|---|---|---|---|---|---|
| Agents, runs, trajectories, work items, run acceptances, continuations, channel messages, worker updates, events | OG `choir.agent`, `choir.run`, `choir.trajectory`, `choir.work_item`, `choir.run_acceptance`, `choir.run_continuation`, `choir.channel_message`, `choir.worker_update`, `choir.event` (`registry.go:85-98`; `graph_store.go:358-2250`) | `system:runtime` | runtime, `/api/runs`, `/api/trajectories` | EP via `og_objects`; in restore | no (their old tables are RA) | already conforms |
| Commitment records, acting pack, precommits and resolutions | OG `choir.commitment_record` (`registry.go:133`); precommit routes ride it (`agentcore/api_precommit.go`) | `system:runtime` on behalf of every desk | desks via the ledger pack (`tools_desk.go:934`) | EP; in restore | no | already conforms |
| Engineering slot claims | **bespoke `co_super_slots`**, direct SQL (`store/store.go:2009-2074`) | runtime | runtime | **EU** (`replay_eligibility.go:68`); not on the tape | the code comment says "co_super_slots table not yet in OG" (`store.go:2245`); engineering assignments and claims are OG | og `choir.co_super_run_claim`-style kind exists (`registry.go` co_super_*); fold slot claims into it, writer `system:runtime`. [H] whether the table is empty at checkpoint time (rows are deleted on release at `store.go:2074`; whether every path releases is unchecked) |
| Wire publish debounce | **bespoke** `wire_publish_debounce_entries/state` (`store/store.go:552,562`) | none found outside DDL | none found | EU | n/a | exempt if dead: delete DDL. [H] confirm no writer on a build with the Autopaper code |
| Computer settings | OG `choir.computer_setting` (`registry.go:109`; `agentcore/computer_settings.go`) | owner act via `/api/computer-settings` | runtime | EP; in restore | no | already conforms (not reachable by desks, by design, `computer_settings.go:15-17`) |
| Source schedules | OG `choir.source_schedule` (`registry.go:108`; `store/source_schedule.go:136`) | owner, runtime | Autopaper poller | EP; in restore | no | already conforms |
| Self-development operations and start intents | bespoke `self_development_operations`, `self_development_start_intents` (`store/store.go:739,747`; `store/project.go:371,440`) | selfdev runtime | selfdev, `/api/computers/*` | EP; in restore | no | og kinds `choir.selfdev_operation`, `choir.selfdev_start_intent`, writer `system:runtime`. Low priority; already on the tape |

### Apps

| Data kind | Home | Writers | Readers | Replay / restore | 2+ homes | Target |
|---|---|---|---|---|---|---|
| Mail message, draft, approval event, risk alert | **host** maild per-owner SQLite `<StorageRoot>/users/<owner>/mail.db` (`maild/store.go:283-312,677-800`; node-b `mailDir=/var/lib/go-choir/mail`, `node-b.nix:38,1035`) | maild (webhook ingest, drafts, send); Mail app via proxy `/api/email/*` (`proxy/handlers.go:977`) | Mail app; **no desk** (management allowlist excludes `/api/email`; `docs/reports/agentic-email-desk-2026-10-14.md` section 1.2) | none: outside the tape, the manifest and the restore set | **yes**: guest Maildir under `files/mail` with `POST /api/mail/inbound` still registered (`autoputer/run.go:579-583`, `maildir.go:206-220`; the producer was removed, `agentic-email-desk` section 1.2) | og kinds `choir.mail_message`, `choir.mail_draft`, `choir.mail_thread`, writer `desk:mail` (the mail app desk, sole agent writer). Transport and routing (aliases, receive policies, webhook receipts, provider index) are exempt: platform-level routing shared across owners |
| Mail attachment | **host** files `<StorageRoot>/attachments/outbound/<owner>/<id>` (`maild/attachments.go:24,215-230`); inbound bodies in the mailbox DB | maild; "attach from Files" copies bytes in (`MailApp.svelte:958`) | Mail app | none | **yes**: copy of a file in Files, and of the original in the mailbox | blob (content-addressed) plus `choir.mail_attachment` object referencing the digest; no copy |
| Calendar event | main: **in memory only**; `.ics` is read from Files via `sourceKey` and merged client-side (`CalendarApp.svelte:343-360,405-425`); events are never persisted. Candidate [C]: edited events are written back into the source `.ics` with `PUT /api/files` (`v7/patch.diff:865-867,978-992`) | the Calendar window (main: nobody; candidate: the window, as a file) | Calendar window only | none; Files tree, not restored | candidate adds the `.ics` file as the home | og `choir.calendar_event`, writer `desk:calendar`; `.ics` is import/export, a path view of a blob. The candidate should not land as written |
| Slides deck | main: viewer of `.pptx`, `.pdf`, `.html` fetched from `/api/files` or a content item (`SlidesApp.svelte:245-290`); no deck is authored. Candidate [C]: authored decks saved as `<title>.slides.json` by `PUT /api/files`, autosave plus Ctrl+S (`v7/patch.diff:7668, 10823-10828`) | the Slides window | Slides window; Files "Play as slides" | none; Files tree | candidate makes the file the home | og `choir.slides_deck`, writer `desk:slides`; `.pptx` stays an imported blob |
| Podcast subscription | OG `choir.podcast_subscription` (`registry.go:102`; `store/podcast.go:30`) | `/api/podcast/subscriptions` handler (runtime) | Podcast app | EP; in restore | no (the table is RA) | already conforms; writer should become `desk:podcast` when that desk exists |
| Podcast episode / imported media | OG `choir.content_item` (`registry.go:101`; `graph_store.go:2793`) with extracted `text_content` in the object | content owner (`/api/content/*`) | apps, desks do not (not on allowlist) | EP; in restore | **yes** for uploaded or file-backed items: file bytes are in Files, the item holds `file_path` | already conforms for the item; becomes the file object (below) |
| Episode progress | **bespoke `media_progress`**, event projection (`store/store.go:585`, `store/media_projection.go:30-50`) | `/api/media/progress` (`mediastate/handler.go`) | media apps | EP (`replay_eligibility.go:71`); in restore | edge kind `media_progress` still registered as an "edge-only table" with no object | og `choir.media_progress`, writer `system:runtime` now, `desk:media` later; keep the upsert reducer shape |
| Media recents | **bespoke `media_recents`**, event projection (`store/store.go:598`, `media_projection.go:57`) | `/api/media/recents` | Files, media apps | EP; in restore | holds `file_path` and `content_id` pointers | og `choir.media_recent`, same writer; references the file object, not a path string |
| Theme and user preferences | **bespoke `user_preferences`**, event projection (`store/store.go:613`, `media_projection.go:78`); browser boot cache `THEME_BOOT_CACHE_KEY` (`App.svelte:386-398`) | `/api/preferences/theme` (Settings) | Settings, App shell | EP; in restore | cache copy is a first-paint hint ("server preference remains authoritative", `App.svelte:399`) | og `choir.user_preference`, writer `owner`. Browser cache: exempt (rebuildable cache) |
| Desktop window layout | **bespoke** `desktop_workspaces` (legacy mirror, `windows_json`), `desktop_app_instances`, `desktop_window_placements`, `desktop_sessions` (`store/store.go:633-675`; written together `store/desktop_live.go:96,149,179`; tape path `store/project.go:141-218`) | Desktop via `/api/desktop/state` (`desktopstate/handler.go`) | Desktop | EP for four tables; RA for `desktop_state` (`replay_eligibility.go:74-78`); in restore | **yes**: the same window list is stored in two shapes in one transaction; OG kinds `choir.desktop_session` and `choir.desktop_app_instance` are registered (`registry.go:104-105`) and `SaveDesktopStateOG` has no caller | og `choir.desktop_session`, `choir.desktop_app_instance`, writer `system:runtime`; drop the `windows_json` mirror. Window `app_context` holds file paths: reference file objects |
| Browser session | OG `choir.browser_session` (`store/browser.go:34`) | runtime | Browser app [H] | EP; in restore | no | already conforms |
| Autopaper front, publications, stories | corpusd Store B: `choir.publication`, `choir.publication_version`, others (`registry.go:111-135`) | `system:corpusd` | Autopaper app via `/api/platform/publications/*`; desks via `Stories` verb | outside the computer restore set by ruling | no | already conforms (this is the world graph) |
| Captures | Store B `choir.web_capture` metadata (`registry.go:78`) plus bodies in the host `capture-bodies` artifact store (`docs/problems/platform-artifact-retention-2026-10-22.md`); VM-side `choir.capture_label`, `choir.capture_disposition` (`registry.go:110-111,131`) | corpusd; runtime for labels | desks via `ReadCapture`; `/api/captures` | Store B outside restore; VM labels EP | metadata and body are two stores, by design | already conforms: body is a blob referenced by digest [H: confirm the object carries the digest] |

### Files, blobs and configuration

| Data kind | Home | Writers | Readers | Replay / restore | 2+ homes | Target |
|---|---|---|---|---|---|---|
| Uploaded or created file | files tree path (`files.go:455-456`), snapshotted into file CAS every 15 minutes | Files app `PUT /api/files`; Texture import (`texture_import.go:510-515`); model-policy seeding (`modelpolicy/model_policy.go:540-591`); engineering and capsule apply into `Source/` | Files, Mail attach, media apps | none; not rewound by restore (`file_sync.go:182`); cited by `file_root_committed` | **yes**: file bytes, `content_item` (type, title, text), `texture_document_aliases.source_path`, `media_recents.file_path`, window `app_context` paths. Delete leaves all four dangling (`files.go:396-430`, no store hook) | og `choir.file` (path, blob digest, media type, title), writer `owner` (Files service acting for the owner); bytes are a content-addressed blob in one CAS. Other tables reference the file object id |
| File metadata (type, title, extracted text) | OG `choir.content_item` keyed by uuid, with a `file_path` string | content owner | apps | EP | see above | fold into `choir.file` |
| Model policy | `System/model-policy.toml` in Files, hand-parsed TOML (`provideriface/config.go:88-93`, `modelpolicy/model_policy.go`); the "owner-edited" marker is a comment line in the file (`model_policy.go:49` `ownerEditedLine`); host-level policy is separate (`docs/provider-vault-and-model-policy-design-2026-10-22.md`) | owner or an operator via `PUT /api/files`; the runtime rewrites an older default (`model_policy.go:540-548`) | runtime on resolve; `/api/model-policy/resolve`; no desk may read or write it | none; not rewound by restore | **yes**: a second writer (platform default seeding) and a host policy | og `choir.model_policy`, writer `owner`; TOML is a rendered view. Real use: the app-rewrite conductor switched engineering's model by editing this file twice, saved a copy beside it and restored it by hand (`apprewrite/log.md:94,186,197,200`) |
| Prompt overrides | files `/mnt/persistent/prompts/users/<owner>/<role>.md` (`promptstore/store.go:131,206-208`; root `provideriface/config.go:176-181`), not under the files root | owner via `/api/prompts` (blocked to desks, `tools_product_api.go:177-182`) | runtime | none; not synced to file CAS; not restored | **yes**: a third file home outside Files and Dolt | og `choir.prompt_override`, writer `owner`. Embedded defaults: exempt (they ship in the binary) |
| Image blobs given to models | `/mnt/persistent/state.image-blobs/sha256/<hex>` (`imageblob/imageblob.go:20-35`) | desks and owner through the asset path (`agentcore/api_assets.go`, `tools_capsule_visual.go`) | gateway bridge, `/api/assets/` | none; not in file CAS | **yes**: a second content-addressed store, unjoined to the first | one CAS; objects reference digests. Until then exempt as a blob store |
| App package installs | not on main. Paused branch: `choir.app_package_event` and `choir.app_package_datum` (`origin/feat/s7-0-app-packages:docs/design/s7-0-app-packages-2026-10-14.md` sections D2, 5.4); retired tables `app_adoptions`, `app_change_packages` (RA); `choir.release_adoption` OG (`registry.go:117`) | owner act | runtime | OG kinds would be EP | the planned `package_kv/v1` store is keyed `(package, key)`, one silo per package | each package's kinds namespaced `choir.app.<pkg>.<kind>` per `app-package-carries-app-desk` section 3; **do not** ship `package_kv` as the app data model (see violation 9) |

### Host-side and system (mostly exempt)

| Data kind | Home | Target |
|---|---|---|
| API keys, sessions, passkeys, recovery tokens, owner root-key wraps | host auth DB (`internal/auth/store.go:29-138`; Settings `/auth/api-keys`) | exempt: secrets and credentials |
| Gateway token, privacy key / DEK | `/mnt/persistent/gateway-token`, `/mnt/persistent/choir-credentials/privacy-key` (`autoputer/boot_timeline.go:451-452`) | exempt: secrets; never in a graph |
| Provider keys, host model policy | host (`docs/provider-vault-and-model-policy-design-2026-10-22.md`) | exempt: secrets; the host policy is platform-level |
| Tape, event index, projection heads, `choir_events` | `computer_event_index`, `computer_event_projection_heads` (`store/store.go:693,707`), `choir_events` (`store/event_table.go:52`) | exempt: this is the tape and its index |
| Platform control (Store A): heads, receipts, checkpoints, route slots | platform Dolt (`platform/computer_events.go:18-144`, `routeledger/sql_ledger.go`) | exempt: platform control plane, outside any computer's graph |
| Actor recovery log | SQLite `actor_updates`, `actor_snapshots`, `actor_heads` beside the store (`actorruntime/adapter.go:111-120`, `actor/log_sqlite.go:20-40`) | exempt: raw recovery log, "recovery/replay only" (`docs/computer-ontology.md` ledger split) |
| Derived keys, lookup indexes, capture index | `og_derived_keys` (`objectgraph/derived_keys.go:62`), `og_lookup_keys` (`platform/retrieval_schema.go:51`), `proj_capture_index` (`platform/capture_index.go:59`), Qdrant | exempt: rebuildable derived indexes |
| Source tree, builds, capsule state | `files/Source/platform`, `/mnt/persistent/capsule-state`, `capsule-artifacts` (`nix/autoputer-vm.nix:1056-1060`) | exempt: source/build state (`docs/computer-ontology.md` "The filesystem is not one thing"). Note `Source/platform` sits inside the file CAS root, so it is synced and encrypted with the owner's files; [H] its size is large |
| Choir Base (journal, tree, blob) | `internal/base/*` | exempt: no product wiring found; only `computerversion` reads it (`computerversion/base_current_state_loader.go:36`) |
| Browser per-device UI state | `localStorage`: auth hint, theme boot cache, Autopaper section and visits (`autopaper.ts:376-420`), Texture drafts; candidate adds Podcast played map, queue and speed (`v7/patch.diff:10753-10790,5377`) | exempt for caches and per-device convenience. **Not exempt**: Podcast played-state and queue, which are owner data that would be invisible to every other device, app and agent |

## Patterns (how data is stored today)

1. **Object graph kind, one registered writer, on the tape.** Runtime ledger, Texture documents and revisions, commitment records, content items, podcast subscriptions, computer settings, source schedules. This is the target pattern and it works: writes pass a kind gate and the interceptor, and replay rebuilds them.
2. **Bespoke relational table, on the tape as an event projection.** `media_progress`, `media_recents`, `user_preferences`, `desktop_*`, `texture_document_aliases`, `texture_agent_mutations`, `run_memory_entries`, `self_development_*`. Replayable, but not a graph kind: no kind gate, no writer declaration, a second query API, and unreachable by a generic reader of the graph.
3. **Bespoke relational table, direct SQL, off the tape.** `agent_evidence`, `co_super_slots`. Replay class `EmptyUntilSupported`: the first row makes the computer checkpoint-ineligible.
4. **File in the files tree, by convention.** The model policy, `.ics`, (candidate) `.slides.json`, `.texture` shortcuts, `Source/`, a guest Maildir, and uploads. Content-addressed only in the 15-minute whole-tree snapshot; outside the manifest and the restore set; unreachable by desk verbs.
5. **Host-side store outside the computer.** maild's mailboxes and attachments, auth, platform control, the corpus. Shared services or secrets for some; for Mail it is owner data.
6. **Browser storage.** Caches and drafts today; owner data in the candidate.

A seventh, smaller one: **file outside the files root** on the guest (`/mnt/persistent/prompts`, `state.image-blobs`), which is neither Dolt nor file CAS.

## Violations, ranked by consequence

1. **The files tree is a second home for app data and configuration, outside graph, tape, restore and desk reach.** A restore does not rewind it (`file_sync.go:182`; no reducer for `file_root_committed`). A desk cannot read or write it (allowlist, jail). The model policy that routes every desk's model lives only here, with state encoded in a comment line, and it was edited and restored by hand twice during the app-rewrite run [C `log.md:94,186,197,200`]. If the v7 candidate is approved, Calendar and Slides add two more file-as-database conventions. Consequence: self-development cannot claim reversibility over data it cannot see, and no other app or agent can use what these apps write.
2. **Mail is owner data on the host, outside the computer.** Messages, drafts, approvals and attachments are in maild's SQLite and files under `/var/lib/go-choir/mail`. They are not on the tape, not in the manifest, not in the restore set, and not reachable by any desk (management allowlist; email desk report). A "schmail" cannot share this data, and the Mail desk cannot be the sole writer of a kind that is not in the graph. A dead guest Maildir route is still registered beside it.
3. **Two bespoke tables are written by direct SQL, off the tape.** `agent_evidence` (research `SaveEvidence`) and `co_super_slots` (engineering slot claims). The first row makes the computer ineligible for any checkpoint, restore or effects transition, the defect recorded for media state on the same day, still open for these two. Evidence is a live desk verb (`tools_desk.go:1201`). [H] `co_super_slots` is empty at checkpoint time only if every claim is released.
4. **One file is named by path string in four places with nothing keeping them true.** `content_items.file_path`, `texture_document_aliases.source_path`, `media_recents.file_path` and window `app_context`. The Files handler has no store hook and offers no rename or move (`files.go:140-165`), so delete leaves all four dangling. There is no object that is "the file".
5. **A Texture document can have five live or planned homes** (graph, `.texture` shortcut, alias table, original content item, corpus mirror), plus a browser draft and dead tables. The graph copy is canonical and conforms; the others are views that were built as homes.
6. **Names registered for data the code stores elsewhere.** OG kinds `choir.run_memory_entry`, `choir.agent_evidence`, `choir.desktop_session`, `choir.desktop_app_instance` have no writer while live tables hold that data; edge kinds `media_progress`, `media_recent`, `user_preference`, `desktop_*` name tables as "edge-only". `TestEveryRegisteredKindHasAWriter` (`registry_coverage_test.go:260`) exempts only `Reserved`; [H] it counts a writer function that exists, not one that is called (the `...OG` functions exist but have no non-test callers). The registry reads as if the migration were done.
7. **Desktop layout is stored twice in one transaction** (`windows_json` mirror plus `desktop_app_instances` and `desktop_window_placements`), so any reader must pick one.
8. **Two guest-local files homes outside the files root** (`prompts/`, `state.image-blobs/`) are neither graph nor file CAS, and are not restored. Prompt overrides are owner configuration.
9. **A planned silo.** The paused S7-0 design stores app data in `choir.app_package_datum` keyed `(package, key)` with a 64 KiB JSON value (design section 5.4). Built as written, every app would own an opaque key-value namespace, which is the opposite of "schmail uses Mail's message data". The design's own namespacing rule (`choir.app.<pkg>.<kind>`) is closer; `package_kv` is not.
10. **Browser storage as the home of owner data** in the candidate (Podcast played, queue, speed). Not on main.
11. **A second content-addressed store** (image blobs) that no object references, and a file CAS whose chunks no object references.

## What this does not settle

- Whether the owner's computer, the paper's computer or b@c actually hold rows in `agent_evidence` or `co_super_slots` (no computer was read). Settled by one `replay-completeness` report per computer.
- Which of the registered-but-unwritten OG kinds have a writer I missed through a different path (for example source entities, `textureowner/tools_texture.go`). Settled by a runtime census of `og_objects` by kind on a real computer.
- Whether `projection base` snapshots carry any file content. I read them as Dolt projection only.
- Whether engineering's capsule, or any shell tool, reaches the files tree by a path I did not trace.
- How much of the files tree is `Source/platform`, and what that does to sync cost.
- Whether the file-per-document convention is a good view (it is, for interop) versus a home (it is not); the doctrine text must say which direction a conflict resolves. I did not test a conflict between a `.texture` shortcut and its document.
- The `choir.mail_*` schema, the desk boundaries for Calendar and Slides, and whether an organization graph under RBAC changes where Store B ends. Those are design questions, not inventory.
- Trace evidence for any behavior claim. This record is code-reading; no run ledger was consulted, so the consequences stated (ineligibility, dangling pointers, lost restore) are derived from code, not observed, except the model-policy episodes, which are from the conductor's own log.

## Directions (one paragraph, not a design)

Make the rule enforceable at the same gate the registry already uses: a kind may be registered only with a live writer, a bespoke table that holds a registered kind's data is a defect, and a path or a blob digest is a field of an object, not a key in a table. Give the files tree one object (`choir.file`: path, blob digest, media type, writer `owner`) so the four path pointers reference an id and delete and rename update the object; give the graph, not the file, the lifecycle of Calendar, Slides, model policy and prompt overrides, with the file as an import or export view; and move Mail's owner data into the computer behind a `desk:mail` writer while leaving transport on the host.

## Exceptions the doctrine must name

1. **Secrets and keys:** API keys, sessions, passkeys, the gateway token, the privacy key and DEK, provider keys. They are never graph objects; they have their own vault and escrow.
2. **The tape and its index:** event chain, `computer_event_index`, projection heads, `choir_events`. The graph is a projection of the tape; the tape is not a kind.
3. **Platform control and routing shared across owners:** Store A heads, receipts, route slots, mail routing and policy tables, the auth user table. They belong to no one computer's graph.
4. **Raw store files and recovery logs:** the embedded Dolt workspace itself, the actor recovery SQLite, Choir Base internals.
5. **Rebuildable caches and derived indexes:** derived keys, lookup and capture indexes, Qdrant, browser boot caches and unsent drafts, per-device UI choices.
6. **Source, build and capsule state:** `Source/`, builds, overlay uppers, per `docs/computer-ontology.md`. They are blob and machine state, promoted by the effect path, not by the graph.
7. **Blob bytes themselves:** the rule is "object plus content-addressed blob", so bytes sit in a CAS and are referenced by digest. The doctrine should say there is one CAS per computer for owner bytes, and name the two present ones (file CAS, image blobs) as the unification target.
8. **The shared world graph (Store B) versus the computer graph:** two stores with the same schema and different restore rules. The doctrine should say they are one model with two scopes, not two models.

## Proposed migration order

Start with kinds that have two live homes, and stop the growth first.

0. **Hold the two new file conventions.** The v7 candidate (awaiting approval on b@c) would add `.ics` write-back, `.slides.json`, and localStorage Podcast state. Decide before approval whether they land as file homes or are rebuilt on graph kinds. This costs nothing to do first and everything later.
1. **The file object.** `choir.file` with blob digest and path view, and repoint `content_item.file_path`, `texture_document_aliases`, `media_recents.file_path` and window contexts at it. Every later step needs it, and it resolves violation 4.
2. **Mail.** Two live homes (host mailbox and a registered guest Maildir route) and the largest owner data outside the computer. Introduce `desk:mail` kinds, import from maild, remove the Maildir route. Transport stays on the host.
3. **Agent evidence and engineering slot claims** (`agent_evidence`, `co_super_slots`). Small; the OG kind already exists and the desk manifest already names `evidence`. Removes the ineligibility class.
4. **Model policy and prompt overrides** to owner-written objects with a rendered view. The model policy is the one config file every desk's behavior depends on.
5. **Desktop layout:** one set of kinds, drop the `windows_json` mirror.
6. **Run memory:** swap the table for the registered kind.
7. **Media progress, recents and preferences** to registered kinds. Already on the tape, so this is a re-homing, not a repair; do it with the file object so recents reference ids.
8. **Texture views:** turn the `.texture` shortcut and alias into path views of the document object; keep the corpus mirror as publication only.
9. **Calendar and Slides** born on graph kinds with `.ics`, `.pptx` and `.slides.json` as import and export.
10. **Clean-up:** drop dead DDL (`texture_revisions`, `texture_source_*`, `wire_publish_debounce_*`), the edge-only vestiges, and the unused `...OG` functions; make registering a kind without a live writer fail, not pass.

Heresy delta: discovered, the files tree as a second home for configuration and app data; Mail as owner data outside the computer; `agent_evidence` and `co_super_slots` as off-tape direct-write tables of the media-state defect class; registered kinds with no writer beside live tables. Introduced: none. Repaired: none.

## Step 3 design: agent evidence and engineering slot claims (2026-10-24, lane `lane/fp0-evidence-slots`)

Mutation class: red, tier 2 (store write path with a new concurrency edge, the
slot claim, and tape-bearing writes). The SQL schema is untouched: no table is
added, altered or dropped in this slice (the tables stay as the legacy home
until a later clean-up step, and `agent_evidence` keeps its manifest entry).
What is added is one registered object kind and two computer settings. Both
settings default off, so a deploy changes nothing until the owner turns one on
for one computer.

### What the code supports (checked, not assumed)

- `choir.agent_evidence` is registered (`registry.go`), and `CreateEvidenceOG`
  / `GetEvidenceOG` / `ListEvidenceByAgentOG` exist with no non-test caller.
  Every OG write on a bound store already rides the projection interceptor to
  the tape and is replayed as `og_objects`, so graph evidence is replay-class
  `ReplayEventProjection` with no new op kind, no reducer and no manifest
  change. The desk manifest's `evidence` owned kind is a label and is not
  tied to the registry; the registered writer is `system:runtime`.
- There is no kind for slot claims. `choir.co_super_run_claim` is keyed by
  `run_id` and written inside the lifecycle bind command; a slot is keyed by
  `(trajectory, slot)` and claimed by the older runtime path
  (`ClaimEngineeringSlot`). A slot row is not deleted on release: it is
  advanced to the next run when the previous owner is terminal, and deleted
  only when run creation fails after the claim. So the table keeps one row per
  `(trajectory, slot)` ever claimed, and **every computer that has ever run an
  engineering trajectory through this path is checkpoint-ineligible**, not only
  one mid-claim. (This settles the doc's [H] about emptiness: it is not empty.)

### The choice per table

- `agent_evidence`: the graph kind (the doc's target). The kind and its
  functions already exist, no reducer or op kind is needed, and the one
  writer is the research desk. An event projection would add an op kind and a
  rollback floor for data the graph already models.
- `co_super_slots`: a new graph kind `choir.co_super_slot`, external key
  `(trajectory, slot)`, body the slot record. Not an event projection: the
  claim is a compare-and-set on `(trajectory, slot)` whose other half
  (the run's state) is already graph data, and an op kind would be the third
  bespoke projection for a record that the graph models directly. A new kind
  is replay-safe for an older binary (replay projects objects without
  consulting the registry), so unlike a new op kind it adds no rollback floor.

### The switch and the transition

Two computer settings (`/api/computer-settings`, read live per call, default
off): `evidence.graph` and `engineering.slots_graph`. A setting is read by the
store at each write, from the computer setting object, with no restart and no
deploy. Off: writes go to the table as today. On: writes go to the graph.

- **Reads always union both homes**, whatever the switch says, so turning it
  on or off never hides a record. Evidence: by id from the graph, else the
  table; lists merge by `evidence_id`, newest first. Slots: per
  `(trajectory, slot)` the newer `updated_at` wins.
- **Migration** (I27: a one-time import that empties the old home): before any
  graph-on write, and when the setting is turned on, the store moves every
  table row into the graph (the object is written first, then the row is
  deleted; the object is keyed by the row's identity so a crash between the two
  re-runs idempotently). A slot row that is newer than its graph object wins
  the import, which covers off then on again. A check that the table is
  non-empty runs per graph-on write, so a rollback that wrote rows to the
  table is re-imported on the next write or the next turn-on.
- **Rollback**: turn the setting off. New writes return to the table; the
  graph objects already written stay on the tape and stay readable. The
  checkpoint-eligibility gain is lost only for rows written after the
  rollback. No binary revert is needed, and no op kind exists to strand a
  restore.

### Failure modes, written before the code

1. A graph-on write that errors must not also write the table (two homes
   again) and must not report success.
2. A crash between writing the graph object and deleting the row must leave
   the record readable exactly once and the next import must finish it.
3. Two concurrent claims of one `(trajectory, slot)` must give exactly one
   winner; a claim of a slot whose owner is terminal must advance it; a
   claim of a slot whose owner is live must return the live run.
4. A release must clear only the matching run's claim, in whichever home holds
   it.
5. Rows written to the table with the switch off, after the switch was on,
   must not shadow or duplicate the graph record: the newer `updated_at` wins
   and the next import folds it in.
6. With the switch on and the table empty, a computer with evidence and a
   claimed slot replays eligible and equivalent; with legacy rows and the
   switch on, it becomes eligible after the import and not before.
7. The setting read failing (store error) reads as off and logs; it never
   blocks a write.
8. An unbound store (the test seam) keeps the table whatever the setting says,
   because the graph there is not on a tape.
9. The import must not run unbounded inside a request: it moves in bounded
   pages, and a failure leaves the setting stored, the unmoved rows readable,
   and the error visible in the response.
10. Evidence content can be large; a row too large for the tape is refused
    with the evidence id in the error and the row stays in the table (the
    computer stays ineligible, visibly) instead of being dropped.

Tests are written first and fail before the code
(`internal/agentcore/evidence_slots_graph_test.go`; the review round added
`internal/store/graph_home_race_test.go`). Failure modes 2, 7, 9 and 10 have
no dedicated test yet (residual, named in the review round below).

### As built

- Settings: `evidence.graph` and `engineering.slots_graph`
  (`store/computer_setting.go`), both in `KnownComputerSettings`, default off.
  The store reads them per write through `graphHomeOn` (`store/graph_home.go`),
  keyed by the tape's computer id; an unbound store is always off. Turn one on
  for one computer with `POST /api/computer-settings {"key":..., "value":true}`;
  the response carries `import: {moved, error}`. The import runs under a 25 s
  deadline and continues on the next write or a repeat POST.
- Evidence writes `choir.agent_evidence` through the existing
  `CreateEvidenceOG` under the research writer; the kind's registered writer is
  now `desk:research` (was `system:runtime`) and its identity mode
  `external_key` (it always was written with a key).
- Slots: the new kind `choir.co_super_slot` (`registry.go`), one object per
  `(trajectory, slot)`. The claim is a conditional write (`PutBatchConditional`,
  expected content hash, or absent) under a store mutex, mirroring the SQL
  insert-or-advance. A release writes the object with no run (a slot with no
  run reads as absent), because the tape has no delete.
- Reads union both homes for every slot and evidence read; a slot's newer
  record wins at the table's one-second DATETIME resolution, a tie going to
  the table (the later write in a rollback). The table claim path refuses a
  slot the graph holds for a live run, so a rolled-back setting never hands a
  held slot out twice.
- Not done: the tables and their manifest entries stay (empty with a setting
  on); dropping the DDL is step 10. `CreateEvidenceOG`'s agent edge is
  best-effort as before.

**Rollback.** Turn the setting off: new writes return to the tables, graph
records stay on the tape and stay readable through the new binary. No op kind
was added, so a binary revert strands no restore. One caution for a binary
revert while `engineering.slots_graph` is on: an older binary does not read the
graph slot objects, so it would not see a live claim. Turn the setting off and
let live engineering runs finish before reverting a binary. The same holds for
`evidence.graph`: an older binary reads only the `agent_evidence` table, so
imported evidence would read as empty (`read_evidence`, `list_evidence`) though
it stays on the tape. Turning the setting off does not move the records back;
do not revert the binary while a live run needs imported evidence.

### Review round (bunch 206, tier 2: Devin and DeepSeek v4.1 flash, sent back)

Discovered, before the fix:

- **Two claims across the two homes could both be told `claimed=true`.** The
  table path's graph-holder check ran outside `graphHomeMu`, and the graph path
  checked `co_super_slots` before taking it, so a claim that read the setting
  on and a claim that read it off could each pass the other home's check while
  the other's write was still in flight (failure mode 3, silent dual
  authority). Fix: `ClaimEngineeringSlot` holds `graphHomeMu` across the whole
  decision (setting read, other-home check, import, write) in both homes, and
  release holds it too.
- **The table home never excluded a second claim on this driver** (found while
  testing the above, on the pre-existing path, not introduced by this lane).
  `INSERT ... ON DUPLICATE KEY UPDATE run_id = run_id` reports `RowsAffected=1`
  for a matched duplicate, so a second claim of a slot a live run held returned
  `claimed=true` (the table kept the first run; the second claimer ran as a
  co-holder). Evidence: a two-claim sequential test on the table home returned
  claimed=true for both. Fix: read the row first and insert only when absent.
  This changes default-off behavior toward the documented contract (a held
  slot returns the live run, `spawn_reused`).
- Import deleted by key only, so a row advanced between its read and the delete
  would be lost; the delete now names the `run_id` it read. A release in the
  graph home cleared the graph record even when a newer table claim held the
  slot; it now clears only when the union record still names the run.
  `GetEvidence` treated any graph read error as not-found; it now falls back to
  the table only on not-found.

Residual: the graph claim's conditional write is not atomic with its tape
append inside the object store; `graphHomeMu` plus one live store per computer
is what serializes it. A second process would lose with an error
(`ErrNeedsProjectionRepair` or head CAS), not receive the holder; production
runs one autoputer per computer.

What only staging proves: the setting flip on a real computer, the import of
real rows, a real checkpoint accepted after, and the research and engineering
desks using the graph home in a live run.
