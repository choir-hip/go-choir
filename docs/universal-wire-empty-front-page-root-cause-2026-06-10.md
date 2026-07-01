# Universal Wire Empty Front Page Root Cause — 2026-06-10

## Problem

Authenticated Universal Wire windows on staging (`https://choir.news`) show:

- `0 articles`
- `No Wire edition articles yet`

This occurs on new and existing accounts after 30+ minutes. The earlier operator conclusion that news production was restored was wrong: sourcecycled accepted processor submissions, but accepted processor runs are not proof of completed article VTexts, platform publication, edition transclusion, or front-page visibility.

## Observed live evidence

Staging checks on 2026-06-10 after the platform VM disk expansion and stale env-file cleanup found:

1. Platform ownership still points at the expected platform computer:
   - owner: `universal-wire-platform`
   - desktop: `platform`
   - VM: `vm-universal-wire-platform`
   - state in ownership registry: `active`
   - sandbox URL: `http://10.203.146.2:8085`
   - data image: `17179869184` bytes (16 GiB)

2. Direct platform sandbox requests hang:
   - `curl --max-time 15 http://10.203.146.2:8085/health` timed out with no bytes.
   - `curl --max-time 20 http://10.203.146.2:8085/api/universal-wire/stories` with internal/auth headers connected but timed out with no response headers.

3. vmctl health checks repeatedly mark the platform VM unhealthy:
   - `vmmanager: health check failed for VM vm-universal-wire-platform at http://10.203.146.2:8085`
   - `vmmanager: VM vm-universal-wire-platform is unhealthy`
   - repeated every 15 seconds from roughly `17:42` through at least `18:00` UTC.

4. Host process state shows the platform Firecracker process saturated:
   - `firecracker` child of `vmctl` around PID `2338778`
   - ~`240%` CPU
   - ~`6.4%` host memory
   - elapsed ~56 minutes at inspection time.

5. sourcecycled is fetching sources successfully, so source ingestion itself is not the immediate blocker:
   - latest source service API response had `success_fetch_count=198`, `failed_fetch_count=13`, `item_producing_source_count=150`, `item_count=4241`.
   - sourcecycled logs show cycles fetching/deduping hundreds of new items (`655`, `559`, `723`).

6. sourcecycled submitted too much processor work and then could not submit more:
   - `17:05:36`: `processor_submitted=32 ... errors=0`
   - `17:05:39`: `processor_submitted=32 ... errors=0`
   - later attempts timed out through the UDS proxy: `Post "http://unix/internal/vmctl/sandbox-proxy/universal-wire-platform/internal/runtime/runs": context deadline exceeded (Client.Timeout exceeded while awaiting headers)`.

7. Corpusd has no synced/published VText rows:
   - `SELECT COUNT(*) FROM platform_vtext_documents` returned `0`.
   - `SELECT COUNT(*) FROM platform_vtext_revisions` returned `0`.
   - corpusd logs show restarts only; no publish/sync activity.

## Root causes

### Root cause 1 — Processor admission is mistaken for news production

The visible front page depends on this full chain:

```text
sourcecycled fetches items
-> queues processor handoffs
-> platform sandbox accepts runtime runs
-> processor/VText work creates canonical article revisions
-> autonomous publication posts through proxy/corpusd
-> edition `universal-wire/Wire.vtext` transcludes article doc ids
-> `/api/universal-wire/stories` reads the edition and article heads
-> Universal Wire app renders cards
```

The prior verification stopped at `processor_submitted=32`. That only proves the sandbox accepted run requests. It does not prove article revisions, corpusd sync, edition mutation, or `/api/universal-wire/stories` visibility.

### Root cause 2 — The processor dispatch limit is a per-drain batch size, not a concurrency/backpressure limit

`cmd/sourcecycled/main.go` defines and reads `SOURCE_SERVICE_AGENT_DISPATCH_MAX_PROCESSORS` into `ingestionRuntimeDispatcher.maxProcessorRequests`, with default `32`.

But the dispatch path does not enforce it as live concurrency:

- queued requests are listed from storage;
- each eligible request is submitted;
- submitted requests are marked `submitted` immediately;
- the next queue drain can submit another batch while the first batch is still running;
- there is no check of platform sandbox `running_runs`, no cap by active processor profile, and no “wait for completions before admitting more”.

Observed result: two drain passes admitted `32 + 32 = 64` processor runs into the platform VM within seconds. The VM then saturated and stopped answering health/story/run-submit requests.

### Root cause 3 — The platform sandbox can wedge while ownership still says `active`

vmctl ownership reports `state=active`, but repeated health checks mark the VM unhealthy and direct HTTP requests time out. The product route can therefore resolve to the correct platform VM and still hang or return stale/empty behavior depending on request timing.

The status model needs a user/product-facing distinction between:

- ownership active;
- HTTP reachable;
- runtime accepting new runs;
- processors completing;
- article publications visible.

### Root cause 4 — Platform publication never happened

Corpusd DoltDB contains zero `platform_vtext_documents` and zero `platform_vtext_revisions`. That means no successful autonomous publication/sync completed after the recent runs. The empty front page is therefore not a frontend-only issue.

Potential explanations still requiring deeper inspection after the VM is recovered:

- the 64 processor runs are still hung and never reached VText article edits;
- VText article edits happened but did not meet `wirepublish.EligibleForAutonomousPublish` metadata/content gates;
- publish attempts failed inside the wedged guest before reaching proxy/corpusd;
- edition mutation failed after platform publish, though corpusd zero rows makes this less likely for the current incident.

## Miswired architecture

The currently miswired part is **backpressure and completion semantics**, not just a URL:

- sourcecycled treats “submitted runtime run” as dispatch success;
- sourcecycled has no durable feedback from runtime completion, VText revision creation, autonomous publish, or edition update;
- the frontend correctly renders empty when `/api/universal-wire/stories` returns no edition articles;
- the platform VM becomes the bottleneck and single point of failure for both write-side production and read-side front-page serving.

## Solutions

### Immediate operational recovery

1. Stop admitting new processor work temporarily:
   - set `SOURCE_SERVICE_AGENT_DISPATCH_MAX_PROCESSORS` to `1` or disable queue drains until the VM recovers.
   - Because current code does not enforce active concurrency, this is only partial; it prevents large new bursts once deployed/restarted but does not clean already submitted runs.

2. Restart or refresh `vm-universal-wire-platform` to clear the wedged guest.

3. After recovery, verify the complete chain, not just run submission:
   - sandbox `/health` returns quickly;
   - `/api/universal-wire/stories` returns non-empty `stories` or a precise empty source;
   - corpusd `platform_vtext_documents` and `platform_vtext_revisions` counts increase after article publication;
   - Universal Wire app shows article cards.

4. If no article rows appear after recovery, inspect platform runtime run records/revisions for the submitted processor run IDs and classify them as:
   - not started;
   - running/hung;
   - completed without VText edit;
   - VText edit created but ineligible for publish;
   - publish failed;
   - edition transclusion failed.

### Durable code fixes

1. Enforce real backpressure in sourcecycled:
   - distinguish configured batch size from live concurrency;
   - before submitting queued processor requests, query platform runtime health/status for active processor runs;
   - submit at most `max(0, concurrency_limit - active_processor_count)`;
   - do not submit another batch merely because queued rows remain.

2. Make runtime run submission reject overload:
   - platform sandbox `/internal/runtime/runs` should return `429 Too Many Requests` when active runs exceed role/profile limits;
   - sourcecycled already treats 429 as transient.

3. Track end-to-end production state, not just submission:
   - store runtime completion status back into sourcecycled or a shared ledger;
   - record article doc id, revision id, platform publication id, and edition revision id per processor request;
   - expose counters: submitted, running, completed, article_created, published, edition_visible, failed.

4. Split read path from wedged write path:
   - `/api/universal-wire/stories` should read the durable corpusd edition/index once publication sync is the invariant;
   - write-side platform sandbox saturation should not make the public front page empty or unavailable;
   - if corpusd has no current edition, report a diagnostic source such as `universal-wire-corpusd-empty`, not a generic honest empty state.

5. Add a deployed acceptance test that fails on the exact false positive:
   - start from sourcecycled dispatch;
   - wait for at least one article publication to corpusd;
   - assert `/api/universal-wire/stories` returns `source=universal-wire-edition-vtext` and `stories.length > 0`;
   - assert the Universal Wire app renders `[data-universal-wire-story]` cards;
   - assert corpusd has synced full VText revision history for the first story.

6. Add vmctl health semantics for `active_but_unhealthy`:
   - ownership should not report an operator-usable `active` state when guest health has failed for many consecutive checks;
   - sourcecycled should stop dispatching to an unhealthy VM and leave queued work queued.

## Proposed implementation order

1. **Backpressure guard:** enforce `maxProcessorRequests` as active concurrency in sourcecycled/runtime. This prevents the recurring VM wedge.
2. **Runtime overload response:** return 429 from sandbox when active run limit is reached.
3. **Completion ledger:** connect sourcecycled processor requests to runtime completion and publish/edition evidence.
4. **Read-path cutover:** serve Universal Wire stories from corpusd durable state once corpusd edition sync is complete.
5. **Acceptance proof:** staging Playwright/API test that requires visible article cards and corpusd VText revision history.

## Open questions for follow-up inspection

These require recovering the platform VM or mounting its Dolt disk safely while stopped:

- Did any of the 64 submitted processor runs create VText documents?
- Did any created revision fail `wirepublish.EligibleForAutonomousPublish` because metadata lacked `source=edit_vtext`, `source_network_cycle_id`, `revision_role=canonical`, `article_version`, or `vtext_edit_kind`?
- Does `universal-wire/Wire.vtext` still exist in the platform sandbox after the disk resize/reboot?
- Are submitted run records still marked running forever, or did provider/tool calls hang inside the processors?
