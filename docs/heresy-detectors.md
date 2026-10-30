# Heresy Detector Manifest

Status: doctrine-level manifest, baseline captured 2026-06-13. Updated 2026-07-09.

This file is the current detector inventory for Choir Doctrine. The CI check
(`scripts/check-heresies.sh` and `.github/workflows/ci.yml`) reports every
family's discovery count and fails rows individually promoted with
`enforce: zero`. Generated ledgers and broader typed allowlist syntax remain
deferred per phase.

Reduction rule: counts are evidence, not ontology. A count decrease supports a
`repaired` claim only when the remaining hits are classified as historical
evidence, doctrine detector text, or explicit transitional compatibility. A
count increase is `introduced` unless the mission records an explicit
conjecture delta and acceptance of the new debt. Discovery of a new detector or
site is `discovered`, not `repaired`.
Pattern strings quoted below are detector vocabulary, not endorsed product
framing.

## Detector Manifest

| ID | Detector family | Grep patterns | Target | Notes |
| --- | --- | --- | --- | --- |
| H001-H005 | parent/child control residue | `ParentRunID`, `parent_id`, `parent_loop_id`, `StartChildRun`, `spawned_child_run`, `spawned_child:` | 0 active control hits | Historical provenance fields need explicit quarantine. |
| H006-H008/H014 | continuation residue | `RunContinuation`, `run_continuations`, `/api/continuations`, `continuation-level`, `"request_source": "run_continuation"` | 0 target-doctrine hits | `continuation-level` is transitional until M4 deletes or re-points it. |
| H009/H022 | semantic next-tool forcing | `next_required_tool`, `next_tool`, `required_next_tool`, `delegation_required`, `chained_required_tool`, `next_tools` | 0 semantic forcing hits | Mechanical protocol envelopes need explicit allowlisting. |
| H010/H024/H026 | Texture/prompt forcing | `requiredContinuationAfterTextureEdit`, `explicit_researcher_request`, `durableMetadataKeys`, `textureEditResearcherIntentText`, `initialTextureToolChoice`, `WithInitialToolChoice`, `buildAgentRevisionRequest`, `call spawn_agent now` | 0 semantic forcing hits | Long-running Texture work must keep exact tool choice mechanical only; detector hits need typed allowlist context. |
| H011/H012 | role keyword oracles | `texturePromptNeedsSuperExecution`, `promptBarExplicitResearcherIntent`, `texturePromptExplicitlyRequestsResearcher`, super keyword lists | 0 routing-control hits | Role mentions may inform Texture; they must not route or force tools; enforce: zero; exclude: docs/**, *_test.go |
| H019 | lease vocabulary drift | `lease`, `leased`, `lease_seconds`, `worker lease` | 0 actor-control hits | Capacity/QoS or historical usage must be labeled explicitly. |
| H027 | Trace app residue | `Trace app`, `Trace UI`, `Open Trace`, `appId: "trace"`, `data-trace-app` | 0 current product-surface hits | Trace evidence remains valid; Trace app/dashboard direction is retired. |
| H028 | raw Terminal app residue | `Terminal app`, `raw Terminal`, `manual terminal`, `/api/terminal/ws`, `appId: "terminal"` | 0 current product-surface hits | The Super Console/zot repair surface that replaced Terminal is removed too (2026-10-22, `docs/problems/remove-zot-2026-10-22.md`), with `internal/autoputer/terminal.go`. The proxy `410` tombstone for `/api/terminal/ws` was removed in D0 (2026-10-12); `/api/super-console/ws` answers `404` at the proxy. |
| H029 | Browser source-gathering residue | `Browser app`, `BrowserApp`, `browser_sessions`, `AppHint: "browser"`, `open_surface: "browser"` | 0 current product-surface hits | Browser names may remain only as transitional implementation names for Web Lens/source work. |
| H023 | synchronous control-plane polling | `pollInternalWorkerRun`, `time.After(500 * time.Millisecond)` | 0 active control hits | Foreground supervision should receive a durable handle; blocking poll loops are forbidden. |
| H024a | trivial first patch as hidden work-state | `delta_chars`, first-revision metadata after an owner work request | 0 trivial-patch hits | Long-running Texture work must record honest work-state; review-level detector until machine-checkable. |
| H024b | model-invented legacy packet IDs | `update_id` required in model-facing schema | 0 model-invented hits | The four desks use semantic acts and must not author delivery IDs. Packet machinery remains only for deferred roles until their phase. |
| H025 | dead parent/child result-channel API | `PostChildResult`, `PostChildError`, `WaitForChildResult` | 0 production hits | Unused compatibility surfaces must be deleted, not documented around. |
| H030 | actor runtime database polling | `log.Unprocessed` | 0 active warm-loop hits | Repaired 2026-06-27; remaining hits should be cold-start replay, post-drain overflow, or Sweep boot recovery, not warm-loop polling. Registry row update only. |
| H031 | route resolves to VM/desktop identity | `UniversalWirePlatformOwnerID`, `UniversalWirePlatformDesktopID`, `ResolveDesktopContext`, `route_profile` | 0 product-route hits | See `docs/choir-doctrine.md` H031; Banned Patterns list #16. The `route_profile` hits need allowlist context for the parser implementation itself and tests. |
| H032 | host-global computer surface | `frontend-current` taught as the computer UI / restore set | review-level until serving envelope | See doctrine `C15`/`I25`/`H032` and `docs/memo-per-computer-frontend-2026-08-13.md`. Do not enforce zero against the live Caddy hop before the successor lands. |
| structural | logic-level residue families | (no grep; review-level) | n/a | H013 overclaim, H015 residency short-circuit, H016 active-run fallback, H017 blockers not durable, H018 assignment not materialized, H020 mixed current/target, H021 stale doctrine. These need model-checked specs or review detectors, not string patterns. |
| H033 | Texture writer/cadence drift | (review-level) | 0 authority/cadence violations | Two writer classes: `AuthorUser` owner CAS edits; `AuthorAppAgent` Texture sole agent writer. `texture_turn_committed` is not a revision; synthetic wait-turns cannot create versions; the v0→v8 table is a non-normative example. |
| H034 | Management-desk singleton ontology drift | (review-level) | 0 claims that the per-computer management desk is a concurrency limiter | Management is the one-per-computer coherence, error-correction, and resource-arbitration authority; it is never a throughput cap and never a document/computer mutator. |
| H035 | Assignment scheduling/supersession drift | (review-level) | 0 ping-pong/duplicate assignment paths | Enforce computer arrival ordinal, FIFO among non-expired requests, one live engineering assignment per computer in Mission A, expiry/supersession/deadline failure, and retryable admission refusal; stale terminal requests settle as late evidence. |
| H036 | Engineering-desk/capsule cardinality drift | (review-level) | 0 doctrinal claims that one engineering assignment has exactly one capsule | N capsules per engineering assignment is the general shape with union-bundle provenance; 1:1 is only the candidate-A transitional implementation. |
| H037 | pipeline-stage ontology drift | (review-level) | 0 normative v0→v8/fixed-stage claims | Pipeline tables are non-normative examples; stage names must not imply prescribed Texture revision semantics or ordering. |
| H038 | second homes for data (`I27`) | (review-level) | report only until the file object lands | Every meaningful kind of data is an object graph kind with one writer and one home; the files tree is a path view over blobs by digest; a new app reuses existing kinds and extends them in its own namespace, never copies them. Inventory: docs/problems/data-has-more-than-one-home-2026-10-24.md. |
| I4 | destructive embedded rollback guard | `CALL DOLT_RESET`, `DoltPromotionAdapter` | 0 production hits | The obsolete adapter is deleted; destructive embedded-main reset or adapter-symbol reintroduction is inadmissible. One typed exception (2026-10-09): `internal/projectionbase/compact.go` soft-resets to the root commit to squash a stopped projection store's history (a cache of the tape, O21). It is history-only, runs after a backup, and refuses unless the content witness is identical before and after (docs/archive/problems/guest-store-history-bloat-and-memory-shape-2026-10-09.md). The same reset moved into the shared `internal/projectionbase/squash.go` (`SquashAndGC`, 2026-10-20), which both the guest compaction and Store A's weekly squash call (owner notes §23, §27 item 7; docs/archive/problems/store-a-weekly-squash-2026-10-20.md): history-only, offline, after a reflink backup, refused unless the HEAD and WORKING roots after equal the WORKING root before. Any other hit stays inadmissible. (Status 2026-10-30: Dolt, Store A and `internal/projectionbase` were deleted in the data system cutover; the exclusions below are inert.) enforce: zero; exclude: docs/**, README.md, AGENTS.md, scripts/**, specs/**, *_test.go, internal/projectionbase/compact.go, internal/projectionbase/squash.go |
| framing | retired root ontology | `personal writing system`, `publishing system`, `AI workspace`, `workflow app`, `StoryGraph`, `chat` | 0 current-root hits | Surface or historical usage is acceptable when explicitly labeled. |
| C1E | work items, delivery packets and the cut-1 retired vocabulary (docs/archive/definitions/choir-reeng-c1e-engineering-on-records-2026-10-13.md) | `WorkItem`, `workItem`, `work_item`, `workitem`, `CoagentSourcePacket`, `worker_update`, `update_coagent`, `RequireNoOpenWorkItems`, `"Assign"`, `choir.Message`, `choir.Emit`, `textureSupervision`, `MaybeAutonomousPublishWireArticle`, `wire_publication`, `textureResearchRoundsPerOwnerRequest`, `ToolLoopBudget`, `EgressBudgetLedger` | 0 production hits at C1e B5 | Report mode from C1e B0: the count is the deletion progress measure; the row is promoted to zero enforcement in B5. Literal substrings, one per name; the live Report body was renamed SourcePacket so `CoagentSourcePacket` matches only the delivery object. exclude: docs/**, *.md, *_test.go, *_test.mjs, frontend/tests/**, internal/vocabmigrate/families.go; docs and tests are history and instruments; families.go is the frozen kind-membership table for historical vocabulary and decodes no work-item or packet object into a live type. |

## Baseline Counts

Captured with fixed-string `rg` across `README.md`, `AGENTS.md`, `docs`,
`internal/agentcore`, `internal/runtimeprompts`, selected frontend/source/test
directories, and selected runtime/store/type/proxy directories on 2026-06-13.
(The historical `internal/runtime/prompt_defaults` path was dissolved in
`c791a0ae`; agent prompts now live in `internal/runtimeprompts/overlays`.)

| Pattern | Count |
| --- | ---: |
| `Trace app` | 41 |
| `Trace UI` | 13 |
| `Open Trace` | 7 |
| `Terminal app` | 10 |
| `raw Terminal` | 15 |
| `manual terminal` | 2 |
| `Browser app` | 16 |
| `BrowserApp` | 46 |
| `browser_sessions` | 19 |
| `AppHint: "browser"` | 1 |
| `continuation-level` | 74 |
| `RunContinuation` | 91 |
| `/api/continuations` | 14 |
| `lease` | 224 |
| `leased` | 18 |
| `lease_seconds` | 7 |
| `personal writing system` | 2 |
| `publishing system` | 10 |
| `AI workspace` | 5 |
| `workflow app` | 3 |
| `StoryGraph` | 302 |
| `chat` | 118 |

These counts intentionally include historical docs and detector text. The next
paramission must add typed allowlists before any fail-on-increase check can be
trusted.

## Deferred Enforcement


Done (Phase A):

- create a structured detector manifest consumed by a script;
- wire the discovery check into the `check` CI gate.

In progress (Phase B/E): classify allow contexts
(`historical-evidence`, `doctrine-detector`, `explicitly-deprecated`,
`implementation-transitional`, `current-violation`); generate a heresy ledger
with `discovered`, `introduced`, and `repaired` deltas; fail on unaccepted count
increases for protected detector families; promote `scripts/check-heresies.sh`
from `--report` to `--fail-on-regression` after the baseline is reviewed.

H011/H012 is the first enforced family: doctrine/Definition text and tests are
explicit allow-contexts, while any production reintroduction fails
`--fail-on-regression`. Other families remain discovery-only until their Phase
B/E cluster closes.
