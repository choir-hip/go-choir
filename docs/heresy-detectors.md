# Heresy Detector Manifest

Status: doctrine-level manifest, baseline captured 2026-06-13.

This file is the current detector inventory for Choir Doctrine. It is not yet a
CI-enforced check. Failing enforcement, generated ledgers, and allowlist syntax
are deferred to the next code-bearing detector paramission because those changes
would alter repo process behavior.

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
| H011/H012 | role keyword oracles | `texturePromptNeedsSuperExecution`, `promptBarExplicitResearcherIntent`, `texturePromptExplicitlyRequestsResearcher`, super keyword lists | 0 routing-control hits | Role mentions may inform Texture; they must not route or force tools. |
| H019 | lease vocabulary drift | `lease`, `leased`, `lease_seconds`, `worker lease` | 0 actor-control hits | Capacity/QoS or historical usage must be labeled explicitly. |
| H027 | Trace app residue | `Trace app`, `Trace UI`, `Open Trace`, `appId: "trace"`, `data-trace-app` | 0 current product-surface hits | Trace evidence remains valid; Trace app/dashboard direction is retired. |
| H028 | raw Terminal app residue | `Terminal app`, `raw Terminal`, `manual terminal`, `/api/terminal/ws`, `appId: "terminal"` | 0 current product-surface hits | Super Console/zot is the repair surface; PTY terms may remain hidden implementation detail. |
| H029 | Browser source-gathering residue | `Browser app`, `BrowserApp`, `browser_sessions`, `AppHint: "browser"`, `open_surface: "browser"` | 0 current product-surface hits | Browser names may remain only as transitional implementation names for Web Lens/source work. |
| H030 | actor runtime database polling | `log.Unprocessed` | 0 active warm-loop hits | Repaired 2026-06-27; remaining hits should be cold-start replay, post-drain overflow, or Sweep boot recovery, not warm-loop polling. Registry row update only. |
| H031 | route resolves to VM/desktop identity | `UniversalWirePlatformOwnerID`, `UniversalWirePlatformDesktopID`, `ResolveDesktopContext`, `route_profile` | 0 product-route hits | See `docs/choir-doctrine.md` H031; Banned Patterns list #16. The `route_profile` hits need allowlist context for the parser implementation itself and tests. |
| framing | retired root ontology | `personal writing system`, `publishing system`, `AI workspace`, `workflow app`, `StoryGraph`, `chat` | 0 current-root hits | Surface or historical usage is acceptable when explicitly labeled. |

## Baseline Counts

Captured with fixed-string `rg` across `README.md`, `AGENTS.md`, `docs`,
`internal/runtime/prompt_defaults`, selected frontend/source/test directories,
and selected runtime/store/type/proxy directories on 2026-06-13:

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


Required work:

- create a structured detector manifest consumed by a script;
- classify allow contexts (`historical-evidence`, `doctrine-detector`,
  `explicitly-deprecated`, `implementation-transitional`, `current-violation`);
- generate a heresy ledger with `discovered`, `introduced`, and `repaired`
  deltas;
- fail on unaccepted count increases for protected detector families;
- wire the check into docs/process CI only after the baseline is reviewed.
