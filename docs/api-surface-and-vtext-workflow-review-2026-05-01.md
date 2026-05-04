# API Surface and VText Workflow Review

Date: 2026-05-01

Status: review artifact only. This document records current API and workflow issues before another implementation pass. It is not a fix plan with completed work.

2026-05-04 follow-up: the hardening pass removed `/api/events` from the public route table, kept Trace on `/api/trace/*`, enriched tool events with unredacted arguments/results for auditability, moved workflow verification away from caller-supplied browser request logs, routed VText super requests through the persistent super inbox controller, and changed VText replace edits to reject ambiguous duplicate matches unless `replace_all` is explicit. Remaining live-demo caution: dry-run/stub tests are still engineering gradients only; product proof needs a prompt-bar workflow plus event-log verification with tool-backed worker-run causality enabled.

## Executive Summary

The latest run made one important narrow improvement: canonical appagent-authored vtext revisions now have an `edit_vtext` tool path, and provider final text is no longer supposed to become document content.

The broader problem remains: browser-visible routes still expose runtime internals. The product API still says "agent loop", "spawn agent", "agent revision", "prompts", and browser-selected `desktop_id`. Those names are not cosmetic. They leak control-plane concepts into the public API and let browser callers express authority they should not have. Trace/debugging does not justify those holes; Trace should work from a dedicated read-only projection, not from public agent orchestration APIs.

The next pass should clean the API contract, stale docs, privileged-agent boundary, and verification strategy before adding more behavior. Otherwise tests will continue to prove that scripted internals can be driven from the browser, not that the product workflow is correct.

## Current Public Boundary Problem

The proxy intentionally forwards all authenticated `/api/*` requests to the sandbox. That is the right proxy shape for future apps, but it means the sandbox route table is the real public API contract.

Therefore these are browser-public today:

| Route | Current Meaning | Correct Boundary |
| --- | --- | --- |
| `POST /api/agent/loop` | Start arbitrary runtime run with caller metadata | Replace with prompt-bar intent endpoint |
| `POST /api/agent/spawn` | Start child agent run | Go-internal/tool-only |
| `GET /api/agent/status` | Raw run status | Remove from public API; expose only a product-level prompt submission status if needed |
| `GET /api/agent/loops` | Raw run list | Remove from public API |
| `GET /api/agent/events` | Raw event history | Remove from public API |
| `GET /api/agent/channel-messages` | Raw mailbox/channel messages | Remove from public API |
| `GET /api/agent/topology` | Runtime internals | Remove from public API |
| `GET /api/events` | Global runtime SSE | Remove from public route table; Trace uses `/api/trace/*` |
| `GET/PUT/DELETE /api/prompts/{role}` | Runtime prompt policy mutation | Dev/admin-only until intentionally productized |
| `POST /api/test/vtext/*` | Dry-run seams | Test-only registration |
| `GET /api/shell/error` | Deliberate 500 test route | Test-only or removed |

The same boundary applies inside the runtime: VText should own document synthesis, not privileged execution topology. `super` should be persistent for the user/session, and only `super` should spawn `cosuper` workers for mutable execution work.

Trace should not depend on keeping `/api/agent/*` browser-callable. It should read authorized trajectory projections from durable events/messages/findings through `/api/trace/*` only. Those Trace routes should be read-only, owner-scoped, and incapable of starting runs, mutating prompts, spawning agents, exposing raw mailboxes, exposing arbitrary run lists, or changing desktop/VM state.

VText verification should be over the durable event log. The UI can be tested as a consumer of that state, but the workflow proof should be a causal audit: prompt-bar submission caused conductor routing, conductor caused VText document creation/revision work, VText requested allowed workers, workers produced structured updates and tool events, VText consumed those updates through `edit_vtext`, and no browser-exposed runtime orchestration route participated.

Provider and vmctl routes are in better shape at the proxy boundary:

| Route Family | Current Status |
| --- | --- |
| `/provider/*` | Proxy denies browser access |
| `/internal/vmctl/*` | Proxy denies browser access |
| Gateway bearer auth | Intended service-to-service path |
| vmctl internal caller checks | Present, but the proxy can still invoke creation semantics from browser-selected desktop IDs |

## Findings To Preserve

These are the 11 review findings that should stay in the issue register.

| ID | Priority | Issue | Location |
| --- | --- | --- | --- |
| 1 | P1 | Replace public agent loop with prompt-bar intent endpoint. Browser callers should submit user intent only. Server creates conductor run internally. | `internal/runtime/api.go:173` |
| 2 | P1 | Browser URL can mint arbitrary published desktops by passing `desktop_id` into proxy routing. | `internal/proxy/handlers.go:58` |
| 3 | P1 | vmctl resolve is both lookup and create. Split published-desktop lookup from internal provisioning. | `internal/vmctl/ownership.go:444` |
| 4 | P1 | Public API exposes agent identity instead of prompt-bar intent. Duplicate of finding 1, but important enough to keep visible. | `internal/runtime/api.go:173` |
| 5 | P1 | Prompt bar client explicitly sets conductor metadata. Browser should not set `agent_profile`, `agent_role`, or app routing metadata. | `frontend/src/lib/conductor.js:13` |
| 6 | P1 | Browser-selected desktop IDs can mint routable VMs. Duplicate of finding 2, but captures the proxy/resource angle. | `internal/proxy/handlers.go:58` |
| 7 | P1 | vmctl resolve creates published desktops for unknown IDs. Duplicate of finding 3, but captures the published-state detail. | `internal/vmctl/ownership.go:512` |
| 8 | P1 | Agent spawn remains public runtime orchestration. `/api/agent/spawn` should not be browser-callable. | `internal/runtime/api.go:221` |
| 9 | P1 | Public revision creation can claim appagent authorship. User revision POSTs should create user revisions only. | `internal/runtime/vtext.go:116` |
| 10 | P2 | VText revise route exposes agent semantics. `/agent-revision` should become a product action like `/revise`. | `internal/runtime/vtext.go:1071` |
| 11 | P2 | Public prompt manager is runtime policy mutation. `/api/prompts` should be dev/admin-only until productized. | `internal/runtime/prompts.go:175` |

## Latest Run Review

What improved:

- `edit_vtext` now exists as a vtext-only tool.
- Vtext appagent revisions are no longer supposed to be written from provider final text.
- `edit_vtext` requires `base_revision_id` and checks it against the current document head.
- The conductor-created V1 no longer literally says "Conductor framing", "User request", or "Use this vtext".
- The real demo no longer manually calls `/api/agent/spawn` from Playwright.

What did not improve:

- `/api/agent/loop` still exists and is still the prompt-bar entrypoint.
- The prompt bar client still posts to `/api/agent/loop` with runtime metadata.
- `/api/agent/spawn` still exists as a public browser route.
- Trace/demo tests still rely on raw `/api/agent/status` and `/api/agent/topology` instead of a self-contained read-only Trace API.
- `/api/prompts` still exists as public runtime policy mutation.
- `/api/vtext/documents/{id}/agent-revision` still exists as public product flow.
- Public vtext revision creation still accepts `author_kind`.
- Browser-selected `desktop_id` still flows into proxy routing.
- vmctl `ResolveOrAssignDesktop` still creates published VMs when a desktop is unknown.
- The latest changes added another test seam: `/api/test/vtext/worker-update`.
- Stale docs still describe `/api/agent/*`, `/api/prompts`, and `/agent-revision` as intended product APIs.

## Latest Run Specific Flaws

### The demo is improved but still not final proof

`frontend/tests/vtext-real-workflow-demo.spec.js` now types into the prompt bar and asserts that the browser did not call `/api/agent/spawn`. That is better than manual Playwright orchestration.

It still proves less than the summary claims:

- The acceptance response still waits for `/api/agent/loop`, not a prompt-bar product endpoint.
- The test still uses raw runtime internals: `/api/agent/status`, `/api/agent/topology`, and `/api/trace/*`.
- It asserts roles appear in Trace, but does not require explicit tool invocation events for `web_search` and `bash node ...`.
- It checks generated files contain the marker, but does not prove the verification command actually ran.
- It checks final document content has evidence-like words or URLs, but not that the content came from real search.
- It is highly scripted with marker strings, exact artifact paths, exact verification path, and exact final-document requirements.

This should be renamed conceptually from "product proof" to "live scripted workflow smoke test" until the API boundary and tool-event assertions are fixed.

### V1 is a better seed, but not yet the right document abstraction

`buildConductorFramingContent` now produces a generic working document with a title, working objective, and current requirements. That is better than transcript/control-plane text.

It is still generic scaffolding, not really an abstract of the document to be iteratively produced. The desired V1 should read like the first useful state of the artifact. For a cellular automata research/build request, V1 should begin to frame the research model, assumptions, artifact goal, and verification goal. It should not mostly be a reusable template.

### `edit_vtext` is the right write boundary, but the edit primitive is still weak

Current `edit_vtext` problems:

- `replace` uses first-match string replacement. If the `find` text appears more than once, the wrong paragraph can be edited.
- `replace_all` is allowed and may remain necessary, but tests should distinguish full replacement from precise edit behavior.
- Revision creation and mutation completion are not one transaction. `CreateRevision` can advance the document head, then `CompleteAgentMutation` can fail, leaving document state and mutation state inconsistent.
- The tool calls `commitVTextToolEdit(context.Background(), ...)`, so it ignores the tool execution context cancellation path.

The write boundary is directionally correct, but it is not yet robust enough to build the whole workflow on without cleanup.

### The old public authoring hole remains

Even if `edit_vtext` is correct, public `POST /api/vtext/documents/{id}/revisions` still accepts `author_kind: appagent`. That means the API still has a second appagent-write path from the browser.

The handler should create only user-authored revisions. Internal runtime/tool paths should create appagent-authored revisions.

### The conductor path is still runtime-shaped

The browser prompt goes to `/api/agent/loop`. The frontend sets `agent_profile: conductor`, `agent_role: conductor`, `input_source: prompt_bar`, and `requested_app: vtext`.

That is backwards. The browser should submit:

```json
{
  "text": "user request",
  "surface": "prompt_bar"
}
```

The server should decide:

```text
prompt-bar intent -> conductor run -> conductor decides app/workflow -> runtime creates agents internally
```

### The conductor route creation is still a side effect, not a clean product endpoint

The route creation path currently depends on conductor run metadata and completion normalization. `materializeConductorDecision` can create/open vtext after a conductor result if conditions match.

This may explain why the video can feel like vtext starts "without using prompt bar": the prompt bar is only the first HTTP trigger, while the visible vtext route is a backend side effect hidden behind `/api/agent/loop`.

That can be valid internally, but the public API should expose the product action as prompt-bar submission, not expose the run machinery.

### VText can still spawn super

`roleSpec(AgentProfileVText)` still allows VText to delegate to `researcher` and `super`.

This is not just a future nicety. It is the internal version of the same boundary problem:

- VText should spawn or request researchers for evidence and document-grounded investigation.
- VText should request privileged execution from the persistent `super`, not spawn `super` directly.
- `super` should be the per-user privileged orchestration root.
- `super` should be the only agent that can spawn `cosuper`.
- `cosuper` should be durable worker capacity under `super`, not an ad hoc child of arbitrary agents.

This keeps document authorship, evidence gathering, and privileged mutable execution separated. It also reduces reward-hacking pressure: a VText run cannot make a demo look successful by directly spawning execution agents, fabricating status-like updates, or bypassing the privileged execution path.

### Tests still reward the wrong shortcuts

The test suite has accumulated useful dry-run seams, but the acceptance criteria are still too easy to satisfy by steering through internals:

- Browser tests can pass by observing Trace roles instead of proving the product path.
- Live demos can pass by requiring marker strings instead of proving real evidence and verification provenance.
- Stub and dry-run paths can create impressive videos while proving no human-valuable work.
- Runtime APIs let tests perform orchestration that the product should not expose.

The cleanup should make dry-run tests explicit engineering scaffolding only. Product proof should require the public prompt-bar path, real provider/search/tool calls when opted in, generated artifacts, verification command events, final VText document revisions created through the VText edit tool, and an event-log audit proving the causal path.

### VText verification should audit causality, not appearances

The current tests lean on visible UI state, marker strings, and Trace role summaries. Those are useful smoke checks, but they do not prove that the product workflow happened. The durable event log should be the machine-verifiable source of truth.

An end-to-end VText verification should assert:

- A browser request used only the public prompt-bar endpoint.
- The conductor run was created by the server, not by browser-supplied `agent_profile` metadata.
- The conductor created or opened the VText workflow.
- The VText document has user-authored and appagent-authored revisions with valid causal parents.
- Appagent-authored revisions were created only by `edit_vtext` tool events.
- VText requested only allowed work: research directly, and privileged execution through the persistent `super` path.
- Researchers emitted real structured findings/evidence updates.
- Execution workers emitted real artifact and test/verification updates.
- Tool-call events include the expected real search provider call when live search is required.
- Tool-call events include the expected real file/artifact writes.
- Tool-call events include the expected verification command and result.
- VText consumed the relevant worker update event IDs/message sequences in later document revisions.
- No browser request called `/api/agent/loop`, `/api/agent/spawn`, `/api/agent/status`, `/api/agent/topology`, `/api/prompts`, or test-only routes.

This event-log audit is the anti-reward-hacking mechanism. The video can demonstrate the product. The event log proves it.

### Security and reward hacking are now the same problem

Any public runtime control seam is both a security bug and a reward-hacking target. A browser-callable route that can spawn agents, set roles, mutate prompts, claim appagent authorship, or allocate desktops lets tests and agents bypass the product workflow while producing plausible success artifacts.

The security posture should be fail-closed:

- Browser routes express user intent and app data only.
- Runtime authority is assigned server-side.
- Debugging routes are read-only projections.
- Test seams are registered only in explicit test mode.
- Acceptance tests fail if they exercise private orchestration from the browser.
- Trace must inspect what happened, not provide a second way to make things happen.

## Stale Documentation

The code and docs still normalize the old API shape:

| File | Stale Content |
| --- | --- |
| `README.md` | Mentions `/api/vtext/documents/{id}/agent-revision` and `/api/prompts` as product surfaces |
| `docs/multiagent-architecture.md` | Lists `/api/agent/loop`, `/api/agent/spawn`, `/api/prompts`, and `/agent-revision` as core APIs |
| `docs/PROJECT-STATE.md` | Describes prompt-bar/vtext behavior through old route names |
| `docs/mission-4-core-functionality-and-choir-in-choir.md` | Lists `/api/agent/spawn` as an API |
| Frontend comments | `TaskRunner`, `runtime.js`, `Shell.svelte`, and `vtext.js` document the old surface |
| Tests | Multiple specs assert `/api/agent/loop`, `/api/agent/spawn`, `/api/prompts`, and `/agent-revision` as expected behavior |

Before more coding, update these docs to make the target API boundary explicit. Otherwise future agents will keep preserving the wrong routes because the repo tells them those routes are canonical.

## Recommended API Contract

Public product routes:

| Route | Meaning |
| --- | --- |
| `POST /api/prompt-bar` | Submit user intent from the desktop prompt bar |
| `GET /api/prompt-bar/submissions/{id}` | Product-level submission status, if needed |
| `POST /api/prompt-bar/submissions/{id}/cancel` | Product-level cancellation, if needed |
| `GET/PUT /api/desktop/state` | Persist visible desktop state for server-approved desktop |
| `GET/PUT/POST/DELETE /api/files/*` | User-visible Files app |
| `GET /api/terminal/ws` | User-visible Terminal app, if terminal remains a product feature |
| `GET/POST /api/vtext/documents` | VText document collection |
| `GET/PUT/DELETE /api/vtext/documents/{id}` | VText document metadata and lifecycle |
| `POST /api/vtext/documents/{id}/revisions` | User-authored document edit only |
| `POST /api/vtext/documents/{id}/revise` | User asks vtext appagent to revise |
| `GET /api/vtext/documents/{id}/stream` | VText document stream |
| `GET /api/vtext/revisions/{id}` | Revision snapshot |
| `GET /api/vtext/diff` | Revision diff |

Internal Go/tool routes or no HTTP route. These should not be browser-public:

| Current Surface | Target |
| --- | --- |
| `/api/agent/spawn` | Remove public route; use Go method/tool only |
| `/api/agent/loop` | Replace with `/api/prompt-bar` |
| `/api/agent/status` | Remove from browser surface; replace with product prompt-submission status only if needed |
| `/api/agent/loops` | Remove from browser surface; Trace reads projections from `/api/trace/*` |
| `/api/agent/events` | Remove from browser surface; Trace reads projections from `/api/trace/*` |
| `/api/agent/channel-messages` | Remove from browser surface; Trace reads projections from `/api/trace/*` |
| `/api/agent/topology` | Remove from browser surface; Trace reads projections from `/api/trace/*` |
| `/api/prompts` | Dev/admin-only unless productized |
| `/api/test/vtext/*` | Register only when test APIs are enabled |
| `/api/shell/error` | Test-only |

Trace routes:

| Route | Boundary |
| --- | --- |
| `/api/trace/trajectories` | Read-only owner-scoped trajectory index |
| `/api/trace/trajectories/{id}` | Read-only owner-scoped trajectory snapshot |
| `/api/trace/moments/{id}` | Read-only owner-scoped detail lookup |
| `/api/trace/trajectories/{id}/stream` | Read-only owner-scoped event stream |

Trace must not call or require `/api/agent/status`, `/api/agent/loops`, `/api/agent/events`, `/api/agent/channel-messages`, `/api/agent/topology`, or `/api/agent/spawn`.

Trace can be the UI over the event-log audit, but it should not be the verifier itself. Verification should run against the durable event records/projections directly, so a pretty Trace graph cannot hide missing or manually injected causality.

Radically reduced public API principle:

The public API should contain only app/user operations that a normal browser user is allowed to perform. Everything else is either an in-process Go call, a service-to-service authenticated route, or absent. Do not keep general runtime read APIs as "debug" endpoints on the authenticated browser surface; they become reward-hacking targets and future compatibility liabilities.

Agent graph contract:

| Capability | Target Boundary |
| --- | --- |
| Prompt-bar entry | Browser submits user intent; server starts conductor |
| Conductor | Opens/routes app workflows; does not expose runtime identity to browser |
| VText | Owns canonical document versions and may request research |
| Researcher | Reads files/web and writes structured findings/evidence updates |
| Super | Persistent per-user privileged execution root |
| Cosuper | Durable mutable execution worker spawned only by `super` |
| Worker updates | Structured inputs to VText; not document patches |
| VText revisions | Created only by VText edit tool or user edits |

vmctl changes:

| Current Behavior | Target |
| --- | --- |
| `ResolveOrAssignDesktop(user, anyDesktop)` creates published VM | Split into lookup published desktop vs internal create/fork |
| Browser `desktop_id` controls routing directly | Browser can select only server-known published desktops |
| `?desktop_id=unknown` can allocate | Unknown desktop ID should 404 or deny |

## Cleanup Order

1. Update docs to name the intended product API and explicitly mark `/api/agent/*` as legacy/internal.
2. Add failing tests for public API boundaries: browser cannot set agent role, cannot call spawn, cannot create appagent revision, cannot route to unknown desktop, cannot mutate prompts in normal product mode.
3. Implement `/api/prompt-bar` as the only browser entrypoint for prompt text.
4. Change frontend prompt bar to call `/api/prompt-bar` without runtime metadata.
5. Make Trace self-contained on read-only `/api/trace/*` projections and remove all frontend/test dependencies on `/api/agent/*`.
6. Remove or gate `/api/agent/loop`, `/api/agent/spawn`, `/api/agent/status`, `/api/agent/loops`, `/api/agent/events`, `/api/agent/channel-messages`, `/api/agent/topology`, `/api/prompts`, `/api/test/vtext/*`, and `/api/shell/error`.
7. Change vtext public revision POST to force `AuthorKind=user` server-side.
8. Rename `/api/vtext/documents/{id}/agent-revision` to `/api/vtext/documents/{id}/revise`.
9. Move privileged execution topology behind persistent `super`: VText cannot spawn `super`; only `super` can spawn `cosuper`.
10. Split vmctl resolve semantics so proxy routing cannot create new desktops.
11. Build event-log verification for VText workflows: assert causal chain, allowed actors, allowed routes, real tool events, worker updates, and `edit_vtext` materialization.
12. Tighten anti-reward-hacking verification: dry-run tests prove only plumbing; live acceptance must pass the event-log audit and prove real search/tool/verification events plus final document synthesis.
13. Then rerun the live workflow demo with stricter proof: explicit `web_search` tool event, explicit generated artifact, explicit verification command event/result, final vtext revision from `edit_vtext`, event-log causality, and no browser access to internal routes.

## Bottom Line

The latest run did not solve the API semantics problem. It made vtext document materialization less wrong, but the public surface still exposes the runtime as if the browser were an agent harness.

The next implementation pass should be an API boundary cleanup plus the matching internal privilege-boundary cleanup: persistent `super`, `super`-only `cosuper` spawning, VText as document editor/research requester, Trace as read-only projection, and event-log verification that cannot pass by exercising fake or privileged shortcuts.
